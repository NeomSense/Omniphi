// poseq/tests/test_network_hardening.rs
//
// Phase 7D — Testnet Readiness & Network Hardening
// Soak tests and network fault injection scenarios.

use omniphi_poseq::networking::peer_manager::{PeerConnState, PeerManager};
use omniphi_poseq::networking::messages::{NodeId, NodeRole, PoSeqMessage, WirePeerStatus};
use omniphi_poseq::config::node_config_file::{NodeConfigFile, NodeRoleConfig};

fn nid(b: u8) -> NodeId {
    let mut id = [0u8; 32];
    id[0] = b;
    id
}

fn make_status(node_id: NodeId, addr: &str, epoch: u64, slot: u64) -> WirePeerStatus {
    WirePeerStatus {
        node_id,
        listen_addr: addr.to_string(),
        current_epoch: epoch,
        current_slot: slot,
        latest_finalized_batch_id: None,
        is_leader: false,
        in_committee: true,
        role: NodeRole::Attestor,
        protocol_version: Some("1.0.0".to_string()),
    }
}

fn valid_config_file() -> NodeConfigFile {
    NodeConfigFile {
        id: "0".repeat(64),
        listen_addr: "0.0.0.0:7001".into(),
        peers: vec!["127.0.0.1:7002".into()],
        quorum_threshold: 2,
        slot_duration_ms: 2000,
        data_dir: "./data".into(),
        role: NodeRoleConfig::Attestor,
        key_seed: None,
        metrics_addr: None,
        seed_peers: vec![],
        slots_per_epoch: Some(10),
    }
}

// ─── Test 1: Peer health check degrades silent peer ───────────────────────────

#[test]
fn test_peer_health_check_degrades_silent_peer() {
    let mut pm = PeerManager::new(nid(0), 100);

    // Register a peer via update_from_status — last_seen_at is set to Instant::now()
    pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 1));
    assert_eq!(pm.connected_count(), 1, "peer should be Connected after update");

    // Wait a tiny bit so that Instant::now() > last_seen_at
    std::thread::sleep(std::time::Duration::from_millis(5));

    // Call tick_health_check with silence_ms=1 — the 5ms wait ensures the peer
    // has been silent longer than the 1ms threshold.
    let now = std::time::Instant::now();
    pm.tick_health_check(now, 1);

    assert_eq!(pm.degraded_count(), 1, "peer should be Degraded after silence threshold exceeded");
    assert_eq!(pm.connected_count(), 0, "no peers should remain Connected");
}

// ─── Test 2: Peer eviction after extended silence ─────────────────────────────

#[test]
fn test_peer_eviction_after_extended_silence() {
    let mut pm = PeerManager::new(nid(0), 100);
    pm.update_from_status(&make_status(nid(2), "127.0.0.1:7002", 1, 1));
    assert_eq!(pm.connected_count(), 1);

    let now = std::time::Instant::now();

    // Degrade immediately with silence_ms=0 (any nonzero elapsed time triggers)
    pm.tick_health_check(now, 0);
    assert_eq!(pm.degraded_count(), 1, "peer should be Degraded");

    // Evict immediately with max_silence_ms=0
    pm.evict_dead_peers(now, 0);
    assert_eq!(pm.degraded_count(), 0, "no peers should remain Degraded");
    assert_eq!(pm.disconnected_count(), 1, "peer should be Disconnected after eviction");

    let peer = pm.get_peer(&nid(2)).unwrap();
    assert_eq!(peer.state, PeerConnState::Disconnected);
}

// ─── Test 3: Reconnect candidates only includes Disconnected peers ─────────────

#[test]
fn test_reconnect_candidates_only_disconnected() {
    let mut pm = PeerManager::new(nid(0), 100);

    // Peer A: Connected (via update_from_status, which sets state = Connected)
    pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 1));

    // Peer B: Disconnected (registered but never heard from)
    pm.register_peer(nid(2), "127.0.0.1:7002".into());

    // Peer C: Degraded (connected then health-checked with zero threshold)
    pm.update_from_status(&make_status(nid(3), "127.0.0.1:7003", 1, 1));
    let now = std::time::Instant::now();
    pm.tick_health_check(now, 0);

    // After tick_health_check, both nid(1) and nid(3) were Connected.
    // Both should now be Degraded. We need only nid(1) connected.
    // Let's reconstruct: re-register nid(1) as Connected.
    pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 2, 2));

    assert_eq!(pm.connected_count(), 1, "only nid(1) should be Connected");
    assert_eq!(pm.degraded_count(), 1, "nid(3) should be Degraded");
    assert_eq!(pm.disconnected_count(), 1, "nid(2) should be Disconnected");

    let candidates = pm.reconnect_candidates();
    assert_eq!(candidates.len(), 1, "reconnect_candidates should return exactly 1 Disconnected peer");

    let candidate_ids: Vec<NodeId> = candidates.iter().map(|(id, _)| *id).collect();
    assert!(candidate_ids.contains(&nid(2)), "the Disconnected peer nid(2) must be a reconnect candidate");
    assert!(!candidate_ids.contains(&nid(1)), "Connected peer nid(1) must not be a reconnect candidate");
    assert!(!candidate_ids.contains(&nid(3)), "Degraded peer nid(3) must not be a reconnect candidate");
}

// ─── Test 4: Backoff doubles on failure ──────────────────────────────────────

#[test]
fn test_backoff_doubles_on_failure() {
    let mut pm = PeerManager::new(nid(0), 100);
    pm.register_peer(nid(1), "127.0.0.1:7001".into());

    // Initial backoff should be 1000ms
    assert_eq!(pm.backoff_ms(&nid(1)), 1000, "initial backoff must be 1000ms");

    // 3 failures trigger Degraded and first doubling: 1000 -> 2000
    pm.record_send_failure(&nid(1));
    pm.record_send_failure(&nid(1));
    pm.record_send_failure(&nid(1));
    let backoff_after_3 = pm.backoff_ms(&nid(1));
    assert!(backoff_after_3 > 1000, "backoff must have doubled after 3 failures, got {}", backoff_after_3);

    // Continue to 10 total failures
    for _ in 0..7 {
        pm.record_send_failure(&nid(1));
    }
    let backoff_after_10 = pm.backoff_ms(&nid(1));
    assert!(
        backoff_after_10 <= 60_000,
        "backoff must be capped at 60000ms, got {}",
        backoff_after_10
    );
}

// ─── Test 5: Reconnect success resets backoff ─────────────────────────────────

#[test]
fn test_reconnect_success_resets_backoff() {
    let mut pm = PeerManager::new(nid(0), 100);
    pm.register_peer(nid(1), "127.0.0.1:7001".into());

    // Apply 5 failures to degrade and increase backoff
    for _ in 0..5 {
        pm.record_send_failure(&nid(1));
    }
    assert_eq!(
        pm.get_peer(&nid(1)).unwrap().state,
        PeerConnState::Degraded,
        "peer should be Degraded after 5 failures"
    );
    assert!(pm.backoff_ms(&nid(1)) > 1000, "backoff should have increased after 5 failures");

    // Record a successful reconnect
    pm.record_reconnect_success(&nid(1));

    let peer = pm.get_peer(&nid(1)).unwrap();
    assert_eq!(peer.state, PeerConnState::Connected, "state must be Connected after reconnect success");
    assert_eq!(peer.reconnect_backoff_ms, 1000, "backoff must be reset to 1000ms after reconnect success");
    assert_eq!(peer.failure_count, 0, "failure_count must be reset to 0 after reconnect success");
}

// ─── Test 6: Config validation rejects bad peer address ──────────────────────

#[test]
fn test_config_validation_rejects_bad_peer_addr() {
    let mut cfg = valid_config_file();
    cfg.peers = vec!["not_a_valid_addr".into()];

    let result = cfg.validate();
    assert!(result.is_err(), "validate() must return Err for invalid peer address");

    let err_msg = result.unwrap_err();
    assert!(
        err_msg.contains("invalid peer address"),
        "error message should mention 'invalid peer address', got: {:?}",
        err_msg
    );
}

// ─── Test 7: Config validation: slots_per_epoch bounds ───────────────────────

#[test]
fn test_config_validation_slots_per_epoch_bounds() {
    // slots_per_epoch = 0 must fail
    let mut cfg = valid_config_file();
    cfg.slots_per_epoch = Some(0);
    let err = cfg.validate();
    assert!(err.is_err(), "slots_per_epoch=0 must be rejected");
    assert!(
        err.unwrap_err().contains("slots_per_epoch"),
        "error must mention 'slots_per_epoch'"
    );

    // slots_per_epoch = 10001 must fail
    let mut cfg = valid_config_file();
    cfg.slots_per_epoch = Some(10001);
    let err = cfg.validate();
    assert!(err.is_err(), "slots_per_epoch=10001 must be rejected");
    assert!(
        err.unwrap_err().contains("slots_per_epoch"),
        "error must mention 'slots_per_epoch'"
    );

    // slots_per_epoch = 100 must be accepted
    let mut cfg = valid_config_file();
    cfg.slots_per_epoch = Some(100);
    assert!(
        cfg.validate().is_ok(),
        "slots_per_epoch=100 must be valid, got: {:?}",
        cfg.validate()
    );
}

// ─── Test 8: Protocol version survives WirePeerStatus encode/decode roundtrip ─

#[test]
fn test_version_in_peer_status_roundtrip() {
    let status = WirePeerStatus {
        node_id: nid(1),
        listen_addr: "127.0.0.1:7001".to_string(),
        current_epoch: 3,
        current_slot: 15,
        latest_finalized_batch_id: None,
        is_leader: false,
        in_committee: true,
        role: NodeRole::Attestor,
        protocol_version: Some("1.0.0".to_string()),
    };

    let msg = PoSeqMessage::PeerStatus(status);
    let enc = msg.encode().expect("encode must succeed");

    // First 4 bytes are the length prefix — skip them for decode
    let decoded = PoSeqMessage::decode(&enc[4..]).expect("decode must succeed");

    match decoded {
        PoSeqMessage::PeerStatus(s) => {
            assert_eq!(
                s.protocol_version,
                Some("1.0.0".to_string()),
                "protocol_version must survive encode/decode roundtrip"
            );
        }
        other => panic!("expected PeerStatus, got {:?}", other.kind()),
    }
}
