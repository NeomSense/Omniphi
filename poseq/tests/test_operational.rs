//! Operational integration tests for Phase 7B — Network Activation.
//!
//! Covers:
//! 1. Snapshot import — valid acceptance and tamper rejection
//! 2. Export deduplication — same epoch exported only once
//! 3. Multiple epoch exports — each epoch exported independently
//! 4. Snapshot duplicate rejection — same epoch rejected after first import
//! 5. 3-node cluster smoke test — nodes bind, handle NextSlot without panic
//! 6. SnapshotImporter unit — hash verification and duplicate rejection

use omniphi_poseq::networking::{NetworkedNode, NodeConfig, NodeControl, NodeRole};
use omniphi_poseq::chain_bridge::snapshot::{
    ChainCommitteeSnapshot, ChainCommitteeMember, SnapshotImporter,
};
use tokio::time::{sleep, Duration};

// ─── Helpers ─────────────────────────────────────────────────────────────────

fn nid(b: u8) -> [u8; 32] {
    let mut id = [0u8; 32];
    id[0] = b;
    id
}

fn nid_hex(b: u8) -> String {
    hex::encode(nid(b))
}

fn make_config(node_id: [u8; 32], addr: &str) -> NodeConfig {
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos();
    NodeConfig {
        node_id,
        listen_addr: addr.to_string(),
        peers: vec![],
        quorum_threshold: 1,
        slot_duration_ms: 200,
        data_dir: format!("/tmp/poseq_optest_{}_{}", node_id[0], ts),
        role: NodeRole::Attestor,
        slots_per_epoch: 10,
    }
}

/// Build a valid `ChainCommitteeSnapshot` with the given epoch and member IDs.
fn make_snapshot(epoch: u64, member_ids: &[[u8; 32]]) -> ChainCommitteeSnapshot {
    // Sort for canonical ordering
    let mut sorted = member_ids.to_vec();
    sorted.sort();
    let members: Vec<ChainCommitteeMember> = sorted.iter().map(|id| ChainCommitteeMember {
        node_id: hex::encode(id),
        public_key: hex::encode(id), // use same bytes as mock pubkey
        moniker: format!("node-{}", id[0]),
        role: "Sequencer".to_string(),
    }).collect();
    let hash = ChainCommitteeSnapshot::compute_hash(epoch, &sorted).to_vec();
    ChainCommitteeSnapshot {
        epoch,
        members,
        snapshot_hash: hash,
        produced_at_block: 100,
    }
}

// ─── Test 1: valid snapshot import is accepted ───────────────────────────────

#[tokio::test]
async fn test_snapshot_import_valid_accepted() {
    let config = make_config(nid(1), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let snap = make_snapshot(1, &[nid(10), nid(11), nid(12)]);
    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, Some(1),
        "valid snapshot should update latest_snapshot_epoch");

    let logged = s.slog.entries().iter().any(|e| e.event == "snapshot.imported");
    assert!(logged, "slog should contain snapshot.imported");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 2: tampered snapshot is rejected ───────────────────────────────────

#[tokio::test]
async fn test_snapshot_import_tampered_rejected() {
    let config = make_config(nid(2), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let mut snap = make_snapshot(2, &[nid(10), nid(11)]);
    // Tamper: flip first byte of hash
    if !snap.snapshot_hash.is_empty() {
        snap.snapshot_hash[0] ^= 0xFF;
    }

    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, None,
        "tampered snapshot must not update latest_snapshot_epoch");

    let rejected = s.slog.entries().iter().any(|e| e.event == "snapshot.rejected");
    assert!(rejected, "slog should contain snapshot.rejected for tampered hash");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 3: duplicate snapshot epoch is rejected ────────────────────────────

#[tokio::test]
async fn test_snapshot_import_duplicate_epoch_rejected() {
    let config = make_config(nid(3), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Import epoch 5 twice
    ctrl.send(NodeControl::ImportSnapshot(Box::new(make_snapshot(5, &[nid(10), nid(11)])))).await.unwrap();
    sleep(Duration::from_millis(30)).await;
    ctrl.send(NodeControl::ImportSnapshot(Box::new(make_snapshot(5, &[nid(10), nid(11)])))).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    // First accepted, second rejected
    let rejected_count = s.slog.entries().iter()
        .filter(|e| e.event == "snapshot.rejected")
        .count();
    assert!(rejected_count >= 1, "duplicate epoch should produce snapshot.rejected");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 4: export epoch is persisted and deduped ───────────────────────────

#[tokio::test]
async fn test_export_epoch_deduplication() {
    let config = make_config(nid(4), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Export epoch 3 twice
    ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap();
    sleep(Duration::from_millis(50)).await;
    ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    assert!(s.exported_epochs.contains(&3), "epoch 3 must be in exported_epochs");

    // export.completed exactly once
    let completed = s.slog.entries().iter()
        .filter(|e| e.event == "export.completed" && e.epoch == 3)
        .count();
    assert_eq!(completed, 1, "epoch 3 must be exported exactly once");

    // export.skipped exactly once (from second call)
    let skipped = s.slog.entries().iter()
        .filter(|e| e.event == "export.skipped" && e.epoch == 3)
        .count();
    assert_eq!(skipped, 1, "second export attempt should produce export.skipped");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 5: multiple epochs exported independently ──────────────────────────

#[tokio::test]
async fn test_export_multiple_epochs() {
    let config = make_config(nid(5), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    ctrl.send(NodeControl::ExportEpoch(1)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(2)).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap();
    sleep(Duration::from_millis(100)).await;

    let s = state.lock().await;
    assert!(s.exported_epochs.contains(&1));
    assert!(s.exported_epochs.contains(&2));
    assert!(s.exported_epochs.contains(&3));

    let completed = s.slog.entries().iter()
        .filter(|e| e.event == "export.completed")
        .count();
    assert_eq!(completed, 3, "all 3 distinct epochs should each produce export.completed");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 6: 3-node cluster smoke test ───────────────────────────────────────

#[tokio::test]
async fn test_three_node_cluster_smoke() {
    let mut node1 = NetworkedNode::bind(make_config(nid(10), "127.0.0.1:0")).await.unwrap();
    let mut node2 = NetworkedNode::bind(make_config(nid(11), "127.0.0.1:0")).await.unwrap();
    let mut node3 = NetworkedNode::bind(make_config(nid(12), "127.0.0.1:0")).await.unwrap();

    for node in [&mut node1, &mut node2, &mut node3] {
        let mut s = node.state.lock().await;
        s.committee = vec![nid(10), nid(11), nid(12)];
        s.in_committee = true;
        s.current_epoch = 1;
    }

    let ctrl1 = node1.ctrl_tx.clone();
    let ctrl2 = node2.ctrl_tx.clone();
    let ctrl3 = node3.ctrl_tx.clone();
    let state1 = node1.state.clone();

    tokio::spawn(async move { node1.run_event_loop().await });
    tokio::spawn(async move { node2.run_event_loop().await });
    tokio::spawn(async move { node3.run_event_loop().await });
    sleep(Duration::from_millis(30)).await;

    ctrl1.send(NodeControl::NextSlot).await.unwrap();
    ctrl2.send(NodeControl::NextSlot).await.unwrap();
    ctrl3.send(NodeControl::NextSlot).await.unwrap();
    sleep(Duration::from_millis(100)).await;

    let s = state1.lock().await;
    assert!(s.current_slot >= 1, "slot should have advanced after NextSlot");

    ctrl1.send(NodeControl::Shutdown).await.ok();
    ctrl2.send(NodeControl::Shutdown).await.ok();
    ctrl3.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 7: SnapshotImporter unit — hash verification ───────────────────────

#[test]
fn test_snapshot_importer_accepts_valid_rejects_invalid() {
    let mut importer = SnapshotImporter::new();
    let epoch = 10;
    let snap = make_snapshot(epoch, &[nid(1), nid(2), nid(3)]);

    // Valid snapshot
    assert!(importer.import(snap).is_ok(), "valid snapshot must be accepted");

    // Duplicate epoch
    let snap2 = make_snapshot(epoch, &[nid(1), nid(2), nid(3)]);
    assert!(importer.import(snap2).is_err(), "duplicate epoch must be rejected");

    // Tampered hash
    let mut snap3 = make_snapshot(11, &[nid(1), nid(2)]);
    snap3.snapshot_hash[0] ^= 0xFF;
    assert!(importer.import(snap3).is_err(), "tampered hash must be rejected");
}

// ─── Test 8: snapshot import then export in sequence ─────────────────────────

#[tokio::test]
async fn test_import_then_export_sequence() {
    let config = make_config(nid(6), "127.0.0.1:0");
    let mut node = NetworkedNode::bind(config).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Import committee snapshot for epoch 7
    let snap = make_snapshot(7, &[nid(10), nid(11)]);
    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Export epoch 7
    ctrl.send(NodeControl::ExportEpoch(7)).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, Some(7));
    assert!(s.exported_epochs.contains(&7));

    ctrl.send(NodeControl::Shutdown).await.ok();
}
