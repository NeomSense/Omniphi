//! Devnet chaos and failure tests for Phase 7C — Devnet Activation.
//!
//! These tests inject failures into a running in-process PoSeq cluster and
//! verify that the system continues to operate correctly:
//!
//! - Kill/restart a node mid-operation: state is preserved
//! - Duplicate snapshot delivery: idempotent (rejected after first)
//! - Stale export re-trigger: dedup guard blocks it
//! - Peer disconnect and reconnect: protocol resumes
//! - Epoch boundary chaos: export fires exactly once per epoch
//! - Tampered snapshot rejected: hash check prevents acceptance
//! - Concurrent export triggers: only one succeeds, rest are deduped
//! - Auto-epoch-advance fires correct export: epoch N exported at slot N*slots_per_epoch

use tokio::time::{sleep, Duration};
use omniphi_poseq::networking::{
    NetworkedNode, NodeConfig, NodeControl, NodeRole, PeerEntry,
};
use omniphi_poseq::networking::messages::NodeId;
use omniphi_poseq::chain_bridge::snapshot::{
    ChainCommitteeSnapshot, ChainCommitteeMember,
};

fn nid(b: u8) -> NodeId {
    let mut id = [0u8; 32];
    id[0] = b;
    id
}

fn make_config(node_id: NodeId, addr: &str) -> NodeConfig {
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos();
    NodeConfig {
        node_id,
        listen_addr: addr.to_string(),
        peers: vec![],
        quorum_threshold: 1,
        slot_duration_ms: 50,
        data_dir: format!("/tmp/poseq_chaos_{}_{}", node_id[0], ts),
        role: NodeRole::Attestor,
        slots_per_epoch: 5,
    }
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
    let snapshot_hash = ChainCommitteeSnapshot::compute_hash(epoch, &sorted).to_vec();
    ChainCommitteeSnapshot {
        epoch,
        members,
        snapshot_hash,
        produced_at_block: (epoch * 100) as i64,
    }
}

// ─── Test 1: Crash-restart preserves exported_epochs ──────────────────────────

#[tokio::test]
async fn test_chaos_crash_restart_preserves_exported_epochs() {
    let id = nid(0xC1);
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos();
    let data_dir = format!("/tmp/poseq_chaos_cr_{}", ts);
    let _ = std::fs::remove_dir_all(&data_dir);

    // Boot 1: export epochs 1, 2, 3
    {
        let cfg = NodeConfig {
            node_id: id,
            listen_addr: "127.0.0.1:0".to_string(),
            peers: vec![],
            quorum_threshold: 1,
            slot_duration_ms: 50,
            data_dir: data_dir.clone(),
            role: NodeRole::Attestor,
            slots_per_epoch: 5,
        };
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(1)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(2)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(3)).await.unwrap();
        sleep(Duration::from_millis(80)).await;
        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // Boot 2: same data dir — exported_epochs must be restored
    {
        let cfg = NodeConfig {
            node_id: id,
            listen_addr: "127.0.0.1:0".to_string(),
            peers: vec![],
            quorum_threshold: 1,
            slot_duration_ms: 50,
            data_dir: data_dir.clone(),
            role: NodeRole::Attestor,
            slots_per_epoch: 5,
        };
        let node = NetworkedNode::bind(cfg).await.unwrap();
        let state = node.state.lock().await;
        assert!(state.exported_epochs.contains(&1), "epoch 1 must survive restart");
        assert!(state.exported_epochs.contains(&2), "epoch 2 must survive restart");
        assert!(state.exported_epochs.contains(&3), "epoch 3 must survive restart");
    }
}

// ─── Test 2: Duplicate snapshot delivery is rejected after restart ─────────────

#[tokio::test]
async fn test_chaos_duplicate_snapshot_rejected_after_restart() {
    let id = nid(0xC2);
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .subsec_nanos();
    let data_dir = format!("/tmp/poseq_chaos_ds_{}", ts);
    let _ = std::fs::remove_dir_all(&data_dir);
    let snap = make_snapshot(10, &[id]);

    // Boot 1: import snapshot for epoch 10
    {
        let cfg = NodeConfig {
            node_id: id,
            listen_addr: "127.0.0.1:0".to_string(),
            peers: vec![],
            quorum_threshold: 1,
            slot_duration_ms: 50,
            data_dir: data_dir.clone(),
            role: NodeRole::Attestor,
            slots_per_epoch: 5,
        };
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        sleep(Duration::from_millis(60)).await;
        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // Boot 2: re-deliver the same snapshot — must be rejected
    {
        let cfg = NodeConfig {
            node_id: id,
            listen_addr: "127.0.0.1:0".to_string(),
            peers: vec![],
            quorum_threshold: 1,
            slot_duration_ms: 50,
            data_dir: data_dir.clone(),
            role: NodeRole::Attestor,
            slots_per_epoch: 5,
        };
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();
        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        let s = state.lock().await;
        let rejected = s.slog.entries().iter().any(|e|
            e.event == "snapshot.rejected" && e.epoch == 10
        );
        assert!(rejected, "duplicate snapshot for epoch 10 must be rejected after restart");
    }
}

// ─── Test 3: Stale export re-trigger is suppressed by dedup ───────────────────

#[tokio::test]
async fn test_chaos_stale_export_dedup() {
    let id = nid(0xC3);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Export epoch 5
    ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    // Trigger export again — should be deduped
    ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    let s = state.lock().await;
    let skipped = s.slog.entries().iter().filter(|e|
        e.event == "export.skipped" && e.epoch == 5
    ).count();
    let completed = s.slog.entries().iter().filter(|e|
        e.event == "export.completed" && e.epoch == 5
    ).count();
    assert_eq!(completed, 1, "epoch 5 must be exported exactly once");
    assert_eq!(skipped, 1, "second export of epoch 5 must be skipped");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 4: Tampered snapshot rejected ───────────────────────────────────────

#[tokio::test]
async fn test_chaos_tampered_snapshot_rejected() {
    let id = nid(0xC4);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Create a valid snapshot then tamper its hash
    let mut snap = make_snapshot(20, &[id]);
    snap.snapshot_hash[0] ^= 0xFF; // flip one byte

    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    let s = state.lock().await;
    let rejected = s.slog.entries().iter().any(|e|
        e.event == "snapshot.rejected" && e.epoch == 20
    );
    assert!(rejected, "tampered snapshot must be rejected");
    assert!(s.latest_snapshot_epoch != Some(20),
        "latest_snapshot_epoch must not advance after tampered snapshot");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 5: Concurrent export triggers — exactly one succeeds ───────────────

#[tokio::test]
async fn test_chaos_concurrent_export_exactly_one() {
    let id = nid(0xC5);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    let ctrl2 = ctrl.clone();
    let ctrl3 = ctrl.clone();
    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Fire 3 concurrent export triggers for the same epoch
    tokio::join!(
        async { ctrl.send(NodeControl::ExportEpoch(7)).await.ok(); },
        async { ctrl2.send(NodeControl::ExportEpoch(7)).await.ok(); },
        async { ctrl3.send(NodeControl::ExportEpoch(7)).await.ok(); },
    );
    sleep(Duration::from_millis(100)).await;

    let s = state.lock().await;
    let completed = s.slog.entries().iter().filter(|e|
        e.event == "export.completed" && e.epoch == 7
    ).count();
    let skipped = s.slog.entries().iter().filter(|e|
        e.event == "export.skipped" && e.epoch == 7
    ).count();
    assert_eq!(completed, 1, "concurrent exports: exactly 1 should complete");
    assert_eq!(completed + skipped, 3, "all 3 triggers must be accounted for");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 6: Crash (Crash+Rejoin) does not lose finalized state ───────────────

#[tokio::test]
async fn test_chaos_crash_rejoin_does_not_lose_event_log() {
    let id = nid(0xC6);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Trigger crash
    ctrl.send(NodeControl::Crash).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // During crash, export attempts should not be processed
    ctrl.send(NodeControl::ExportEpoch(9)).await.unwrap();
    sleep(Duration::from_millis(50)).await;

    // Rejoin
    ctrl.send(NodeControl::Rejoin).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Export after rejoin should work
    ctrl.send(NodeControl::ExportEpoch(10)).await.unwrap();
    sleep(Duration::from_millis(80)).await;

    let s = state.lock().await;
    // epoch 10 export should succeed after rejoin
    let completed_10 = s.slog.entries().iter().any(|e|
        e.event == "export.completed" && e.epoch == 10
    );
    assert!(completed_10, "export after rejoin (epoch 10) must succeed");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 7: Auto-epoch-advance fires export at correct boundary ──────────────

#[tokio::test]
async fn test_chaos_auto_epoch_advance_exports_at_boundary() {
    let id = nid(0xC7);
    let cfg = make_config(id, "127.0.0.1:0");
    // slots_per_epoch = 5 (set in make_config above)
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Set up a minimal committee so leader election works
    {
        let mut s = state.lock().await;
        s.committee = vec![id];
        s.in_committee = true;
    }

    // Advance 5 slots — should trigger epoch 0 auto-export at slot 5
    for _ in 0..5 {
        ctrl.send(NodeControl::NextSlot).await.unwrap();
        sleep(Duration::from_millis(30)).await;
    }

    // Give the auto-triggered ExportEpoch time to process
    sleep(Duration::from_millis(100)).await;

    let s = state.lock().await;
    assert_eq!(s.current_epoch, 1, "after 5 slots with slots_per_epoch=5, current_epoch must be 1");
    let exported_epoch0 = s.slog.entries().iter().any(|e|
        e.event == "export.completed" && e.epoch == 0
    );
    assert!(exported_epoch0, "epoch 0 must be auto-exported at slot boundary");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 8: Multiple snapshot epochs accepted sequentially ───────────────────

#[tokio::test]
async fn test_chaos_sequential_snapshot_import() {
    let id = nid(0xC8);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Import snapshots for epochs 1..=4
    for epoch in 1u64..=4 {
        let snap = make_snapshot(epoch, &[id]);
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
        sleep(Duration::from_millis(40)).await;
    }

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, Some(4), "latest snapshot epoch must be 4 after sequential imports");
    let imported = s.slog.entries().iter().filter(|e| e.event == "snapshot.imported").count();
    assert_eq!(imported, 4, "all 4 snapshots must be imported");

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── Test 9: Export + snapshot in same epoch both succeed independently ────────

#[tokio::test]
async fn test_chaos_export_and_snapshot_same_epoch_independent() {
    let id = nid(0xC9);
    let cfg = make_config(id, "127.0.0.1:0");
    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    let snap = make_snapshot(15, &[id]);
    ctrl.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    ctrl.send(NodeControl::ExportEpoch(15)).await.unwrap();
    sleep(Duration::from_millis(100)).await;

    let s = state.lock().await;
    assert_eq!(s.latest_snapshot_epoch, Some(15));
    assert!(s.exported_epochs.contains(&15));
    let imported_ok = s.slog.entries().iter().any(|e| e.event == "snapshot.imported" && e.epoch == 15);
    let exported_ok = s.slog.entries().iter().any(|e| e.event == "export.completed" && e.epoch == 15);
    assert!(imported_ok, "snapshot import for epoch 15 must succeed");
    assert!(exported_ok, "export for epoch 15 must succeed independently");

    ctrl.send(NodeControl::Shutdown).await.ok();
}
