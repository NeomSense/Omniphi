//! Phase 7B Proof Layer — Runtime, Restart, Replay, Consistency, and Cross-Lane tests.
//!
//! This file is the primary evidence that Omniphi's PoSeq side is operationally ready:
//!
//! - **Restart safety**: exported_epochs and snapshot state survive node restart
//! - **Replay resilience**: duplicate imports/exports produce no double effects
//! - **Delay/ordering**: stale and out-of-order snapshots handled correctly
//! - **Multi-node consistency**: nodes don't diverge under supported scenarios
//! - **Cross-lane handoff**: Rust ExportBatch → Go-compatible JSON payload roundtrip

use std::collections::BTreeSet;

use omniphi_poseq::networking::{NetworkedNode, NodeConfig, NodeControl, NodeRole, PeerEntry};
use omniphi_poseq::chain_bridge::snapshot::{
    ChainCommitteeSnapshot, ChainCommitteeMember, SnapshotImporter,
};
use omniphi_poseq::chain_bridge::exporter::ExportBatch;
use tokio::time::{sleep, Duration};
use tempfile::TempDir;

// ─── Test helpers ─────────────────────────────────────────────────────────────

fn nid(b: u8) -> [u8; 32] {
    let mut id = [0u8; 32]; id[0] = b; id
}

fn make_config_at(node_id: [u8; 32], addr: &str, data_dir: &str) -> NodeConfig {
    NodeConfig {
        node_id,
        listen_addr: addr.to_string(),
        peers: vec![],
        quorum_threshold: 1,
        slot_duration_ms: 200,
        data_dir: data_dir.to_string(),
        role: NodeRole::Attestor,
        slots_per_epoch: 10,
    }
}

fn make_config(node_id: [u8; 32], addr: &str) -> NodeConfig {
    // Use a fresh TempDir-style path via a process-unique counter so tests that
    // don't explicitly test restart don't accumulate state across runs.
    // We embed node_id[0] plus a simple counter derived from the current instant.
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos();
    let dir = format!("/tmp/poseq_p7b_{}_{}", node_id[0], ts);
    make_config_at(node_id, addr, &dir)
}

fn make_snapshot(epoch: u64, member_ids: &[[u8; 32]]) -> ChainCommitteeSnapshot {
    let mut sorted = member_ids.to_vec();
    sorted.sort();
    let members: Vec<ChainCommitteeMember> = sorted.iter().map(|id| ChainCommitteeMember {
        node_id: hex::encode(id),
        public_key: hex::encode(id),
        moniker: format!("node-{}", id[0]),
        role: "Sequencer".to_string(),
    }).collect();
    let hash = ChainCommitteeSnapshot::compute_hash(epoch, &sorted).to_vec();
    ChainCommitteeSnapshot { epoch, members, snapshot_hash: hash, produced_at_block: 100 }
}

// ─── PART 1: RESTART SAFETY ──────────────────────────────────────────────────

/// After exporting epochs 1 and 2, restart the node with the same data dir.
/// The restarted node must NOT re-export those epochs (dedup survives restart).
#[tokio::test]
async fn test_restart_export_dedup_survives() {
    let tmp = TempDir::new().unwrap();
    let dir = tmp.path().to_str().unwrap().to_string();

    // --- First boot: export epochs 1 and 2 ---
    {
        let mut node = NetworkedNode::bind(
            make_config_at(nid(20), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(1)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(2)).await.unwrap();
        sleep(Duration::from_millis(80)).await;

        let s = state.lock().await;
        assert!(s.exported_epochs.contains(&1));
        assert!(s.exported_epochs.contains(&2));
        ctrl.send(NodeControl::Shutdown).await.ok();
    }

    // Brief pause to let sled flush
    sleep(Duration::from_millis(50)).await;

    // --- Second boot (restart): dedup must block re-export ---
    {
        let mut node = NetworkedNode::bind(
            make_config_at(nid(20), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        // Before event loop: check restored state
        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&1),
                "epoch 1 must be in exported_epochs after restart");
            assert!(s.exported_epochs.contains(&2),
                "epoch 2 must be in exported_epochs after restart");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Try to re-export epoch 1 — must be skipped
        ctrl.send(NodeControl::ExportEpoch(1)).await.unwrap();
        sleep(Duration::from_millis(50)).await;

        let s = state.lock().await;
        let skipped = s.slog.entries().iter()
            .filter(|e| e.event == "export.skipped" && e.epoch == 1)
            .count();
        assert_eq!(skipped, 1,
            "re-export of epoch 1 after restart must produce export.skipped");
        let completed = s.slog.entries().iter()
            .filter(|e| e.event == "export.completed" && e.epoch == 1)
            .count();
        assert_eq!(completed, 0,
            "re-export of epoch 1 after restart must NOT produce export.completed");

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

/// Restart near an epoch boundary: partial export in boot N, complete in boot N+1.
/// Epoch 3 exported in first boot, epoch 4 not. After restart, epoch 3 blocked,
/// epoch 4 succeeds.
#[tokio::test]
async fn test_restart_epoch_boundary_behavior() {
    let tmp = TempDir::new().unwrap();
    let dir = tmp.path().to_str().unwrap().to_string();

    // First boot: export epoch 3 only
    {
        let mut node = NetworkedNode::bind(
            make_config_at(nid(21), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let ctrl = node.ctrl_tx.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap();
        sleep(Duration::from_millis(60)).await;
        ctrl.send(NodeControl::Shutdown).await.ok();
    }

    sleep(Duration::from_millis(50)).await;

    // Second boot: epoch 3 blocked, epoch 4 succeeds cleanly
    {
        let mut node = NetworkedNode::bind(
            make_config_at(nid(21), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        // Pre-loop: epoch 3 present, epoch 4 absent
        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&3));
            assert!(!s.exported_epochs.contains(&4));
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap(); // must skip
        ctrl.send(NodeControl::ExportEpoch(4)).await.unwrap(); // must succeed
        sleep(Duration::from_millis(80)).await;

        let s = state.lock().await;
        assert!(!s.exported_epochs.contains(&3) || {
            // epoch 3 was already in set — check it didn't re-export
            let completed = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 3)
                .count();
            completed == 0
        });
        assert!(s.exported_epochs.contains(&4), "epoch 4 must export successfully after restart");

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

/// Snapshot import state (latest_snapshot_epoch) survives restart.
#[tokio::test]
async fn test_restart_snapshot_epoch_survives() {
    let tmp = TempDir::new().unwrap();
    let dir = tmp.path().to_str().unwrap().to_string();

    // First boot: import snapshot for epoch 7
    {
        let mut node = NetworkedNode::bind(
            make_config_at(nid(22), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ImportSnapshot(Box::new(
            make_snapshot(7, &[nid(10), nid(11)])
        ))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        let s = state.lock().await;
        assert_eq!(s.latest_snapshot_epoch, Some(7));
        ctrl.send(NodeControl::Shutdown).await.ok();
    }

    sleep(Duration::from_millis(50)).await;

    // Second boot: latest_snapshot_epoch must be restored
    {
        let node = NetworkedNode::bind(
            make_config_at(nid(22), "127.0.0.1:0", &dir)
        ).await.unwrap();
        let s = node.state.lock().await;
        assert_eq!(s.latest_snapshot_epoch, Some(7),
            "latest_snapshot_epoch must be restored from persistence after restart");
    }
}

// ─── PART 2: REPLAY & DELAY RESILIENCE ───────────────────────────────────────

/// Same snapshot delivered twice: second delivery is rejected as duplicate.
/// No double effect in state or logs.
#[tokio::test]
async fn test_replay_duplicate_snapshot_no_double_effect() {
    let mut node = NetworkedNode::bind(make_config(nid(30), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let snap = make_snapshot(10, &[nid(1), nid(2)]);
    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    sleep(Duration::from_millis(30)).await;
    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    let imported = s.slog.entries().iter()
        .filter(|e| e.event == "snapshot.imported" && e.epoch == 10).count();
    let rejected = s.slog.entries().iter()
        .filter(|e| e.event == "snapshot.rejected" && e.epoch == 10).count();
    assert_eq!(imported, 1, "snapshot should be imported exactly once");
    assert_eq!(rejected, 1, "duplicate replay should produce exactly one rejection");
    // latest_snapshot_epoch is set once and not corrupted by replay
    assert_eq!(s.latest_snapshot_epoch, Some(10));

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Same epoch export triggered three times: only first succeeds, rest are skipped.
#[tokio::test]
async fn test_replay_export_triple_trigger_idempotent() {
    let mut node = NetworkedNode::bind(make_config(nid(31), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
    sleep(Duration::from_millis(100)).await;

    let s = state.lock().await;
    let completed = s.slog.entries().iter()
        .filter(|e| e.event == "export.completed" && e.epoch == 5).count();
    let skipped = s.slog.entries().iter()
        .filter(|e| e.event == "export.skipped" && e.epoch == 5).count();
    assert_eq!(completed, 1, "epoch 5 must be exported exactly once regardless of replay count");
    assert_eq!(skipped, 2, "two duplicate triggers must produce two export.skipped entries");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Tampered snapshot: hash mismatch is detected and state unchanged.
#[tokio::test]
async fn test_replay_tampered_snapshot_rejected_with_reason() {
    let mut node = NetworkedNode::bind(make_config(nid(32), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let mut tampered = make_snapshot(11, &[nid(1), nid(2)]);
    tampered.snapshot_hash[0] ^= 0xFF; // corrupt hash

    ctrl.send(NodeControl::ImportSnapshot(Box::new(tampered))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, None, "tampered snapshot must not update state");
    let rejected = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.rejected" && e.epoch == 11);
    assert!(rejected, "tampered snapshot must produce snapshot.rejected log entry");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Delayed (stale) snapshot: epoch 5 arrives after epoch 7 is already the current.
/// Both must be accepted independently (each epoch has its own slot).
#[tokio::test]
async fn test_replay_delayed_stale_snapshot_handling() {
    let mut node = NetworkedNode::bind(make_config(nid(33), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Import newer epoch first
    ctrl.send(NodeControl::ImportSnapshot(Box::new(
        make_snapshot(7, &[nid(1), nid(2)])
    ))).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Now the "delayed" older epoch arrives
    ctrl.send(NodeControl::ImportSnapshot(Box::new(
        make_snapshot(5, &[nid(1), nid(2)])
    ))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    // Both snapshots valid for their respective epochs
    let imported_7 = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.imported" && e.epoch == 7);
    let imported_5 = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.imported" && e.epoch == 5);
    assert!(imported_7, "epoch 7 (early arrival) must be accepted");
    assert!(imported_5, "epoch 5 (delayed arrival) must be accepted — different epoch slot");
    // latest_snapshot_epoch tracks the most recently processed, not highest epoch
    // (it's updated on each successful import)
    assert!(s.latest_snapshot_epoch.is_some());

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Stale snapshot with wrong hash for its epoch: rejected, does not affect
/// valid already-imported snapshots.
#[tokio::test]
async fn test_replay_stale_tampered_does_not_corrupt_valid_state() {
    let mut node = NetworkedNode::bind(make_config(nid(34), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Import valid epoch 3
    ctrl.send(NodeControl::ImportSnapshot(Box::new(
        make_snapshot(3, &[nid(1), nid(2)])
    ))).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Deliver tampered version of epoch 4
    let mut bad = make_snapshot(4, &[nid(1), nid(2)]);
    bad.snapshot_hash[0] ^= 0xAB;
    ctrl.send(NodeControl::ImportSnapshot(Box::new(bad))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    // Epoch 3 still accepted, epoch 4 rejected
    let imported_3 = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.imported" && e.epoch == 3);
    let rejected_4 = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.rejected" && e.epoch == 4);
    assert!(imported_3, "valid epoch 3 must remain imported");
    assert!(rejected_4, "tampered epoch 4 must be rejected");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Crash + rejoin: node re-announces after rejoin, export dedup still holds.
#[tokio::test]
async fn test_replay_crash_rejoin_export_dedup_intact() {
    let mut node = NetworkedNode::bind(make_config(nid(35), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    ctrl.send(NodeControl::ExportEpoch(6)).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    // Simulate crash then rejoin (in-process)
    ctrl.send(NodeControl::Crash).await.unwrap();
    sleep(Duration::from_millis(30)).await;
    ctrl.send(NodeControl::Rejoin).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Attempt duplicate export after rejoin — must still be blocked
    ctrl.send(NodeControl::ExportEpoch(6)).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    let completed = s.slog.entries().iter()
        .filter(|e| e.event == "export.completed" && e.epoch == 6).count();
    assert_eq!(completed, 1, "epoch 6 must be exported exactly once even after crash+rejoin");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── PART 3: MULTI-NODE CONSISTENCY ──────────────────────────────────────────

/// A minimal comparable view of a node's operational state.
#[derive(Debug, Clone, PartialEq, Eq)]
struct NodeStateView {
    latest_snapshot_epoch: Option<u64>,
    exported_epochs: BTreeSet<u64>,
    current_epoch: u64,
}

async fn collect_view(node_state: &std::sync::Arc<tokio::sync::Mutex<omniphi_poseq::networking::NodeState>>) -> NodeStateView {
    let s = node_state.lock().await;
    NodeStateView {
        latest_snapshot_epoch: s.latest_snapshot_epoch,
        exported_epochs: s.exported_epochs.clone(),
        current_epoch: s.current_epoch,
    }
}

/// All 3 nodes import the same snapshot → all show identical latest_snapshot_epoch.
#[tokio::test]
async fn test_consistency_shared_snapshot_import() {
    let mut n1 = NetworkedNode::bind(make_config(nid(40), "127.0.0.1:0")).await.unwrap();
    let mut n2 = NetworkedNode::bind(make_config(nid(41), "127.0.0.1:0")).await.unwrap();
    let mut n3 = NetworkedNode::bind(make_config(nid(42), "127.0.0.1:0")).await.unwrap();

    let (c1, c2, c3) = (n1.ctrl_tx.clone(), n2.ctrl_tx.clone(), n3.ctrl_tx.clone());
    let (s1, s2, s3) = (n1.state.clone(), n2.state.clone(), n3.state.clone());

    tokio::spawn(async move { n1.run_event_loop().await });
    tokio::spawn(async move { n2.run_event_loop().await });
    tokio::spawn(async move { n3.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let snap = make_snapshot(15, &[nid(40), nid(41), nid(42)]);
    for ctrl in [&c1, &c2, &c3] {
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    }
    sleep(Duration::from_millis(80)).await;

    let (v1, v2, v3) = tokio::join!(
        collect_view(&s1),
        collect_view(&s2),
        collect_view(&s3),
    );

    assert_eq!(v1.latest_snapshot_epoch, Some(15));
    assert_eq!(v1.latest_snapshot_epoch, v2.latest_snapshot_epoch,
        "node 1 and node 2 must agree on latest_snapshot_epoch");
    assert_eq!(v2.latest_snapshot_epoch, v3.latest_snapshot_epoch,
        "node 2 and node 3 must agree on latest_snapshot_epoch");

    c1.send(NodeControl::Shutdown).await.ok();
    c2.send(NodeControl::Shutdown).await.ok();
    c3.send(NodeControl::Shutdown).await.ok();
}

/// All 3 nodes export the same epochs → all show identical exported_epochs sets.
#[tokio::test]
async fn test_consistency_shared_epoch_export() {
    let mut n1 = NetworkedNode::bind(make_config(nid(43), "127.0.0.1:0")).await.unwrap();
    let mut n2 = NetworkedNode::bind(make_config(nid(44), "127.0.0.1:0")).await.unwrap();
    let mut n3 = NetworkedNode::bind(make_config(nid(45), "127.0.0.1:0")).await.unwrap();

    let (c1, c2, c3) = (n1.ctrl_tx.clone(), n2.ctrl_tx.clone(), n3.ctrl_tx.clone());
    let (s1, s2, s3) = (n1.state.clone(), n2.state.clone(), n3.state.clone());

    tokio::spawn(async move { n1.run_event_loop().await });
    tokio::spawn(async move { n2.run_event_loop().await });
    tokio::spawn(async move { n3.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    for epoch in [10u64, 11, 12] {
        for ctrl in [&c1, &c2, &c3] {
            ctrl.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        }
    }
    sleep(Duration::from_millis(120)).await;

    let (v1, v2, v3) = tokio::join!(
        collect_view(&s1),
        collect_view(&s2),
        collect_view(&s3),
    );

    assert_eq!(v1.exported_epochs, v2.exported_epochs,
        "node 1 and node 2 must have identical exported_epochs");
    assert_eq!(v2.exported_epochs, v3.exported_epochs,
        "node 2 and node 3 must have identical exported_epochs");
    assert!(v1.exported_epochs.contains(&10));
    assert!(v1.exported_epochs.contains(&11));
    assert!(v1.exported_epochs.contains(&12));

    c1.send(NodeControl::Shutdown).await.ok();
    c2.send(NodeControl::Shutdown).await.ok();
    c3.send(NodeControl::Shutdown).await.ok();
}

/// Duplicate snapshot delivery to all 3 nodes: each node accepts once, rejects once.
/// No divergence in rejection behavior.
#[tokio::test]
async fn test_consistency_duplicate_snapshot_all_nodes_reject_consistently() {
    let mut n1 = NetworkedNode::bind(make_config(nid(46), "127.0.0.1:0")).await.unwrap();
    let mut n2 = NetworkedNode::bind(make_config(nid(47), "127.0.0.1:0")).await.unwrap();
    let mut n3 = NetworkedNode::bind(make_config(nid(48), "127.0.0.1:0")).await.unwrap();

    let (c1, c2, c3) = (n1.ctrl_tx.clone(), n2.ctrl_tx.clone(), n3.ctrl_tx.clone());
    let (s1, s2, s3) = (n1.state.clone(), n2.state.clone(), n3.state.clone());

    tokio::spawn(async move { n1.run_event_loop().await });
    tokio::spawn(async move { n2.run_event_loop().await });
    tokio::spawn(async move { n3.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let snap = make_snapshot(20, &[nid(46), nid(47), nid(48)]);
    // Deliver twice to each node
    for ctrl in [&c1, &c2, &c3] {
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    }
    sleep(Duration::from_millis(100)).await;

    for (i, state) in [&s1, &s2, &s3].iter().enumerate() {
        let s = state.lock().await;
        let imported = s.slog.entries().iter()
            .filter(|e| e.event == "snapshot.imported" && e.epoch == 20).count();
        let rejected = s.slog.entries().iter()
            .filter(|e| e.event == "snapshot.rejected" && e.epoch == 20).count();
        assert_eq!(imported, 1, "node {} should import epoch 20 exactly once", i + 1);
        assert_eq!(rejected, 1, "node {} should reject the duplicate exactly once", i + 1);
    }

    c1.send(NodeControl::Shutdown).await.ok();
    c2.send(NodeControl::Shutdown).await.ok();
    c3.send(NodeControl::Shutdown).await.ok();
}

/// Multi-epoch rollover: import snapshots and export across epochs 1-3.
/// All nodes maintain consistent state at each epoch boundary.
#[tokio::test]
async fn test_consistency_multi_epoch_rollover() {
    let mut n1 = NetworkedNode::bind(make_config(nid(50), "127.0.0.1:0")).await.unwrap();
    let mut n2 = NetworkedNode::bind(make_config(nid(51), "127.0.0.1:0")).await.unwrap();
    let mut n3 = NetworkedNode::bind(make_config(nid(52), "127.0.0.1:0")).await.unwrap();

    let (c1, c2, c3) = (n1.ctrl_tx.clone(), n2.ctrl_tx.clone(), n3.ctrl_tx.clone());
    let (s1, s2, s3) = (n1.state.clone(), n2.state.clone(), n3.state.clone());

    tokio::spawn(async move { n1.run_event_loop().await });
    tokio::spawn(async move { n2.run_event_loop().await });
    tokio::spawn(async move { n3.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let members = [nid(50), nid(51), nid(52)];

    // Simulate 3 epoch rollovers: import snapshot then export each epoch
    for epoch in 1u64..=3 {
        let snap = make_snapshot(epoch, &members);
        for ctrl in [&c1, &c2, &c3] {
            ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        }
        sleep(Duration::from_millis(30)).await;
        for ctrl in [&c1, &c2, &c3] {
            ctrl.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        }
        sleep(Duration::from_millis(40)).await;
    }

    let (v1, v2, v3) = tokio::join!(
        collect_view(&s1),
        collect_view(&s2),
        collect_view(&s3),
    );

    // All nodes must agree on exported epochs
    assert_eq!(v1.exported_epochs, v2.exported_epochs);
    assert_eq!(v2.exported_epochs, v3.exported_epochs);
    assert!(v1.exported_epochs.len() >= 3, "all 3 epochs must be exported");

    // All nodes must agree on latest snapshot
    assert_eq!(v1.latest_snapshot_epoch, v2.latest_snapshot_epoch);
    assert_eq!(v2.latest_snapshot_epoch, v3.latest_snapshot_epoch);

    c1.send(NodeControl::Shutdown).await.ok();
    c2.send(NodeControl::Shutdown).await.ok();
    c3.send(NodeControl::Shutdown).await.ok();
}

// ─── PART 4: CROSS-LANE HANDOFF ──────────────────────────────────────────────

/// Verify that a Rust ExportBatch serializes to JSON that is schema-compatible
/// with the Go `types.ExportBatch` struct.
///
/// This test is the "Rust side" of the cross-lane handoff proof. It:
/// 1. Triggers a real ExportBatch via the node event loop
/// 2. Reads the persisted JSON from sled
/// 3. Deserializes it as the Rust ExportBatch type (schema roundtrip)
/// 4. Asserts required fields are present and correctly typed
/// 5. Writes the payload to a shared fixture file for the Go integration test
///    to consume (see chain/x/poseq/keeper/keeper_test.go TestCrossLane_*)
#[tokio::test]
async fn test_cross_lane_export_payload_schema() {
    let tmp = TempDir::new().unwrap();
    let dir = tmp.path().to_str().unwrap().to_string();

    let mut node = NetworkedNode::bind(
        make_config_at(nid(60), "127.0.0.1:0", &dir)
    ).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let store = node.store.clone();

    // Set a committee so the export hash is non-trivial
    {
        let mut s = node.state.lock().await;
        s.committee = vec![nid(60), nid(61), nid(62)];
        s.current_epoch = 9;
    }

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    ctrl.send(NodeControl::ExportEpoch(9)).await.unwrap();
    sleep(Duration::from_millis(80)).await;

    // Read the persisted JSON back from sled
    let key = b"export:epoch:9";
    let json_bytes = {
        let locked = store.lock().await;
        locked.engine.get_raw(key)
    };
    assert!(json_bytes.is_some(), "export:epoch:9 must be persisted to sled");
    let json_bytes = json_bytes.unwrap();

    // Parse as serde_json::Value for schema inspection
    let val: serde_json::Value = serde_json::from_slice(&json_bytes)
        .expect("persisted export batch must be valid JSON");

    // Required top-level fields (matching Go types.ExportBatch)
    assert!(val.get("epoch").is_some(), "epoch field must be present");
    assert_eq!(val["epoch"].as_u64(), Some(9), "epoch must be 9");
    assert!(val.get("evidence_set").is_some(), "evidence_set field must be present");
    assert!(val.get("escalations").is_some(), "escalations field must be present");
    assert!(val.get("suspensions").is_some(), "suspensions field must be present");
    assert!(val.get("epoch_state").is_some(), "epoch_state field must be present");

    // evidence_set sub-fields
    let ev_set = &val["evidence_set"];
    assert!(ev_set.get("epoch").is_some(), "evidence_set.epoch must be present");
    assert!(ev_set.get("packets").is_some(), "evidence_set.packets must be present");
    assert!(ev_set.get("set_hash").is_some(), "evidence_set.set_hash must be present");

    // epoch_state sub-fields
    let ep_state = &val["epoch_state"];
    assert!(ep_state.get("epoch").is_some(), "epoch_state.epoch must be present");
    assert!(ep_state.get("committee_hash").is_some(), "epoch_state.committee_hash must be present");
    assert!(ep_state.get("finalized_batch_count").is_some(),
        "epoch_state.finalized_batch_count must be present");

    // Deserialize as typed ExportBatch (full roundtrip proof)
    let batch: ExportBatch = serde_json::from_slice(&json_bytes)
        .expect("persisted JSON must deserialize as Rust ExportBatch");
    assert_eq!(batch.epoch, 9);

    // Write the JSON to a shared fixture for Go consumption
    let fixture_path = "/tmp/poseq_crosslane_epoch9.json";
    std::fs::write(fixture_path, &json_bytes)
        .expect("must write cross-lane fixture file");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Duplicate cross-lane export: delivering the same ExportBatch JSON twice to
/// the Rust side via ExportEpoch must produce exactly one persisted payload.
/// (The Go side dedup is tested in keeper_test.go TestCrossLane_IngestDedup)
#[tokio::test]
async fn test_cross_lane_duplicate_export_single_payload() {
    let tmp = TempDir::new().unwrap();
    let dir = tmp.path().to_str().unwrap().to_string();

    let mut node = NetworkedNode::bind(
        make_config_at(nid(61), "127.0.0.1:0", &dir)
    ).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let store = node.store.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Send ExportEpoch(8) three times — only first must produce a payload
    ctrl.send(NodeControl::ExportEpoch(8)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(8)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(8)).await.unwrap();
    sleep(Duration::from_millis(100)).await;

    // sled must have exactly one payload for epoch 8
    let key = b"export:epoch:8";
    let stored = store.lock().await.engine.get_raw(key);
    assert!(stored.is_some(), "epoch 8 export payload must be in sled");

    // The payload is a valid JSON object (not corrupted by concurrent writes)
    let val: serde_json::Value = serde_json::from_slice(&stored.unwrap()).unwrap();
    assert_eq!(val["epoch"].as_u64(), Some(8));

    // Check log: exactly 1 completed, 2 skipped
    let s = state.lock().await;
    let completed = s.slog.entries().iter()
        .filter(|e| e.event == "export.completed" && e.epoch == 8).count();
    let skipped = s.slog.entries().iter()
        .filter(|e| e.event == "export.skipped" && e.epoch == 8).count();
    assert_eq!(completed, 1);
    assert_eq!(skipped, 2);

    ctrl.send(NodeControl::Shutdown).await.ok();
}

/// Snapshot → Export sequence: import a chain snapshot, then export the epoch.
/// The exported payload's epoch must match the imported snapshot epoch.
/// This is the basic cross-lane activation proof: snapshot from chain consumed,
/// epoch evidence prepared for chain ingestion.
#[tokio::test]
async fn test_cross_lane_snapshot_then_export_sequence() {
    let mut node = NetworkedNode::bind(make_config(nid(62), "127.0.0.1:0")).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();
    let store = node.store.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Step 1: chain → PoSeq (import committee snapshot for epoch 25)
    ctrl.send(NodeControl::ImportSnapshot(Box::new(
        make_snapshot(25, &[nid(62), nid(63)])
    ))).await.unwrap();
    sleep(Duration::from_millis(40)).await;

    // Step 2: PoSeq → chain (export epoch 25 batch)
    ctrl.send(NodeControl::ExportEpoch(25)).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    // Verify handoff trace
    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, Some(25), "chain snapshot must be consumed");
    assert!(s.exported_epochs.contains(&25), "epoch must be exported toward chain");

    // Both events must be logged with the same epoch
    let imported = s.slog.entries().iter()
        .any(|e| e.event == "snapshot.imported" && e.epoch == 25);
    let exported = s.slog.entries().iter()
        .any(|e| e.event == "export.completed" && e.epoch == 25);
    assert!(imported, "snapshot.imported must appear in log");
    assert!(exported, "export.completed must appear in log");

    // The persisted export payload is valid JSON with correct epoch
    let key = b"export:epoch:25";
    let bytes = store.lock().await.engine.get_raw(key).unwrap();
    let val: serde_json::Value = serde_json::from_slice(&bytes).unwrap();
    assert_eq!(val["epoch"].as_u64(), Some(25));

    ctrl.send(NodeControl::Shutdown).await.ok();
}
