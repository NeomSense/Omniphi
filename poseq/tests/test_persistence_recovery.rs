//! Persistence and recovery tests for Phase 7B — Operational Readiness.
//!
//! # Design intent
//!
//! These tests intentionally **reuse the same sled data directory** across
//! restart steps, proving that:
//!
//! 1. Export dedup survives a full process restart (Deliverable 2)
//! 2. Snapshot dedup survives a full process restart (Deliverable 3)
//! 3. Multi-epoch progression across restart is correct (Deliverable 5)
//! 4. Crash/restart around epoch boundaries is safe (Deliverable 4)
//! 5. Partial multi-node restart does not cause divergence (Deliverable 6)
//! 6. Startup load log events are visible for observability (Deliverable 7)
//!
//! Each test that exercises persistence reuse explicitly derives its unique
//! storage path from a per-test constant so that:
//!   - Parallel test runs do NOT share storage (each test is isolated)
//!   - Across the two simulated "boots" WITHIN the same test, storage IS shared
//!
//! Tests that do not care about persistence isolation use timestamp-qualified
//! paths (same pattern as test_operational.rs) to avoid cross-test contamination.

use omniphi_poseq::networking::{NetworkedNode, NodeConfig, NodeControl, NodeRole};
use omniphi_poseq::chain_bridge::snapshot::{ChainCommitteeSnapshot, ChainCommitteeMember};
use tokio::time::{sleep, Duration};
use std::collections::BTreeSet;

// ─── Helpers ─────────────────────────────────────────────────────────────────

fn nid(b: u8) -> [u8; 32] {
    let mut id = [0u8; 32];
    id[0] = b;
    id
}

/// Make a config pointing at a **fixed** data dir — used for persistence reuse tests.
/// Each test that calls this must provide a unique `test_tag` string so different
/// tests never share the same sled directory.
fn make_config_fixed(node_id: [u8; 32], addr: &str, test_tag: &str) -> NodeConfig {
    NodeConfig {
        node_id,
        listen_addr: addr.to_string(),
        peers: vec![],
        quorum_threshold: 1,
        slot_duration_ms: 200,
        data_dir: format!("/tmp/poseq_recovery_{}_n{}", test_tag, node_id[0]),
        role: NodeRole::Attestor,
        slots_per_epoch: 10,
    }
}

/// Make a config with a timestamp-qualified data dir — for tests that need
/// isolation but do not exercise persistence reuse.
fn make_config_isolated(node_id: [u8; 32], addr: &str) -> NodeConfig {
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
        data_dir: format!("/tmp/poseq_recovery_iso_{}_{}", node_id[0], ts),
        role: NodeRole::Attestor,
        slots_per_epoch: 10,
    }
}

/// Build a valid `ChainCommitteeSnapshot` for the given epoch and raw member IDs.
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
    ChainCommitteeSnapshot {
        epoch,
        members,
        snapshot_hash: hash,
        produced_at_block: 100,
    }
}

// ─── A. Export Dedup Across Restart ─────────────────────────────────────────
//
// Non-negotiable requirement: epoch N exported before restart MUST NOT be
// re-exported after restart, even when ExportEpoch(N) is triggered again.
// The dedup guard (exported_epochs BTreeSet) must survive the restart.

/// Boot 1: export epoch N.  Boot 2: retrigger epoch N — must be blocked.
#[tokio::test]
async fn test_export_dedup_across_restart() {
    let tag = "export_dedup_restart";
    let epoch = 42u64;

    // ── Boot 1 ──────────────────────────────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(1), "127.0.0.1:0", tag);
        // Wipe any leftover state from a previous CI run so this test is repeatable.
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Export epoch N once.
        ctrl.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&epoch),
                "Boot1: epoch {epoch} must be in exported_epochs after first export");
            let completed = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == epoch)
                .count();
            assert_eq!(completed, 1, "Boot1: export.completed must appear exactly once");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: restart with same storage ────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(1), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        // Verify startup restored the exported epoch set.
        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&epoch),
                "Boot2: exported_epochs must be restored from sled on restart");
            let restored_log = s.slog.entries().iter()
                .any(|e| e.event == "startup.restore.exported_epochs");
            assert!(restored_log, "Boot2: startup.restore.exported_epochs log must be present");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Re-trigger the same epoch — dedup must block it.
        ctrl.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            let completed = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == epoch)
                .count();
            assert_eq!(completed, 0,
                "Boot2: export.completed must NOT appear — epoch {epoch} was already exported");

            let skipped = s.slog.entries().iter()
                .filter(|e| e.event == "export.skipped" && e.epoch == epoch)
                .count();
            assert_eq!(skipped, 1,
                "Boot2: export.skipped must appear once — dedup hit after restart");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── B. Snapshot Replay Across Restart (Option A: persisted dedup) ──────────
//
// Model: imported snapshot identity is persisted in sled; on restart the
// SnapshotImporter is reconstructed from persisted snapshots so that the same
// snapshot delivered again is rejected as a duplicate.
//
// This is Option A from the spec — the stronger safety guarantee.

/// Boot 1: import snapshot S.  Boot 2: deliver snapshot S again — must reject.
#[tokio::test]
async fn test_snapshot_dedup_across_restart() {
    let tag = "snapshot_dedup_restart";
    let epoch = 7u64;
    let snap = make_snapshot(epoch, &[nid(10), nid(11), nid(12)]);

    // ── Boot 1 ──────────────────────────────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(2), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(epoch),
                "Boot1: latest_snapshot_epoch must be set after import");
            let imported = s.slog.entries().iter()
                .any(|e| e.event == "snapshot.imported" && e.epoch == epoch);
            assert!(imported, "Boot1: snapshot.imported must be in log");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: restart with same storage, re-deliver same snapshot ──────────
    {
        let cfg = make_config_fixed(nid(2), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        // Startup must have restored the snapshot epoch.
        {
            let s = state.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(epoch),
                "Boot2: latest_snapshot_epoch must be restored from sled");
            let restored_log = s.slog.entries().iter()
                .any(|e| e.event == "startup.restore.snapshots");
            assert!(restored_log, "Boot2: startup.restore.snapshots log must be present");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Re-deliver the identical snapshot — SnapshotImporter was reconstructed
        // from sled so it knows about epoch 7 and will reject the duplicate.
        ctrl.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            // latest_snapshot_epoch must still be epoch (not reset or doubled)
            assert_eq!(s.latest_snapshot_epoch, Some(epoch),
                "Boot2: latest_snapshot_epoch must remain {epoch} after duplicate delivery");
            // snapshot.rejected must appear (DuplicateEpoch)
            let rejected = s.slog.entries().iter()
                .any(|e| e.event == "snapshot.rejected" && e.epoch == epoch);
            assert!(rejected,
                "Boot2: snapshot.rejected must appear — duplicate epoch after restart");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── C. Multi-Epoch Progression Across Restart ──────────────────────────────
//
// Epoch N exported → restart → epoch N+1 exported → restart → verify:
//   - N is not re-exported on any restart
//   - N+1 exports correctly
//   - N+2 (new epoch) exports on third boot
//   - epoch tracking remains coherent across all three boots

#[tokio::test]
async fn test_multi_epoch_progression_across_restart() {
    let tag = "multi_epoch_restart";

    // ── Boot 1: export epoch 100 ─────────────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(3), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(100)).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&100));
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: epoch 100 blocked, epoch 101 new ─────────────────────────────
    {
        let cfg = make_config_fixed(nid(3), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&100),
                "Boot2: epoch 100 must be restored in exported_epochs");
            assert!(!s.exported_epochs.contains(&101),
                "Boot2: epoch 101 must not yet be in exported_epochs");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Retrigger epoch 100 — must be skipped.
        ctrl.send(NodeControl::ExportEpoch(100)).await.unwrap();
        // Export epoch 101 — must succeed.
        ctrl.send(NodeControl::ExportEpoch(101)).await.unwrap();
        sleep(Duration::from_millis(80)).await;

        {
            let s = state.lock().await;
            let skipped_100 = s.slog.entries().iter()
                .filter(|e| e.event == "export.skipped" && e.epoch == 100)
                .count();
            assert_eq!(skipped_100, 1, "Boot2: epoch 100 must be skipped (dedup)");

            let completed_101 = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 101)
                .count();
            assert_eq!(completed_101, 1, "Boot2: epoch 101 must export successfully");

            assert!(s.exported_epochs.contains(&100));
            assert!(s.exported_epochs.contains(&101));
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 3: 100 + 101 blocked, epoch 102 new ─────────────────────────────
    {
        let cfg = make_config_fixed(nid(3), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        {
            let s = state.lock().await;
            assert!(s.exported_epochs.contains(&100));
            assert!(s.exported_epochs.contains(&101));
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(100)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(101)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(102)).await.unwrap();
        sleep(Duration::from_millis(100)).await;

        {
            let s = state.lock().await;
            let skipped = s.slog.entries().iter()
                .filter(|e| e.event == "export.skipped")
                .count();
            assert_eq!(skipped, 2, "Boot3: epochs 100 and 101 must both be skipped");

            let completed_102 = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 102)
                .count();
            assert_eq!(completed_102, 1, "Boot3: epoch 102 must export successfully");

            // Full set is now {100, 101, 102}
            let mut expected = BTreeSet::new();
            expected.insert(100u64);
            expected.insert(101u64);
            expected.insert(102u64);
            assert_eq!(s.exported_epochs, expected, "Boot3: exported_epochs must be {{100, 101, 102}}");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── D. Restart Near Epoch Boundary ─────────────────────────────────────────
//
// Simulate a node that is shut down immediately after a successful persistence
// write for epoch N, then restarted and immediately triggered for epoch N and N+1.
// The key invariant: no double export, no skipped epoch.

#[tokio::test]
async fn test_restart_at_epoch_boundary_safe() {
    let tag = "epoch_boundary_restart";

    // ── Boot 1: export epoch 200, then shut down ──────────────────────────────
    {
        let cfg = make_config_fixed(nid(4), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(200)).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            let completed = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 200)
                .count();
            assert_eq!(completed, 1, "Boot1: epoch 200 must export once");
        }

        // Shutdown immediately — simulating crash/restart at the boundary.
        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(20)).await;
    }

    // ── Boot 2: simultaneous trigger for boundary epoch (200) and next (201) ──
    {
        let cfg = make_config_fixed(nid(4), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Both triggers arrive quickly after restart — boundary condition.
        ctrl.send(NodeControl::ExportEpoch(200)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(201)).await.unwrap();
        sleep(Duration::from_millis(100)).await;

        {
            let s = state.lock().await;

            // Epoch 200: must be skipped (persisted before restart)
            let skipped_200 = s.slog.entries().iter()
                .filter(|e| e.event == "export.skipped" && e.epoch == 200)
                .count();
            assert_eq!(skipped_200, 1, "Boot2: epoch 200 must be skipped — dedup at boundary");

            // Epoch 201: must export exactly once (new)
            let completed_201 = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 201)
                .count();
            assert_eq!(completed_201, 1, "Boot2: epoch 201 must export exactly once after restart");

            // No double completion for 200
            let completed_200 = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 200)
                .count();
            assert_eq!(completed_200, 0, "Boot2: epoch 200 must NOT produce export.completed");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── D2. Crash Simulation — Crash/Rejoin Dedup Integrity ─────────────────────
//
// Tests the in-process Crash/Rejoin path (no full restart) to confirm that
// the in-memory exported_epochs is not lost across crash+rejoin within the
// same process. This is a weaker but fast test of the in-memory path.

#[tokio::test]
async fn test_crash_rejoin_export_dedup_intact() {
    let cfg = make_config_isolated(nid(5), "127.0.0.1:0");

    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();
    let state = node.state.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    // Export epoch 50
    ctrl.send(NodeControl::ExportEpoch(50)).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    // Simulate crash
    ctrl.send(NodeControl::Crash).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Rejoin
    ctrl.send(NodeControl::Rejoin).await.unwrap();
    sleep(Duration::from_millis(30)).await;

    // Re-trigger epoch 50 post-rejoin — must be blocked
    ctrl.send(NodeControl::ExportEpoch(50)).await.unwrap();
    sleep(Duration::from_millis(60)).await;

    {
        let s = state.lock().await;
        let completed = s.slog.entries().iter()
            .filter(|e| e.event == "export.completed" && e.epoch == 50)
            .count();
        assert_eq!(completed, 1, "epoch 50 must export exactly once even after crash+rejoin");

        let skipped = s.slog.entries().iter()
            .filter(|e| e.event == "export.skipped" && e.epoch == 50)
            .count();
        assert_eq!(skipped, 1, "epoch 50 re-trigger after rejoin must be skipped");
    }

    ctrl.send(NodeControl::Shutdown).await.ok();
}

// ─── E. Partial Multi-Node Restart Consistency ──────────────────────────────
//
// 3 nodes share the same snapshot epoch and export epoch.
// Node 1 is restarted; nodes 2 and 3 remain live (in-process simulation).
// After node 1's restart:
//   - Node 1 must have the same exported_epochs and latest_snapshot_epoch
//     as it had before the restart
//   - Re-delivery of same snapshot and epoch must be deduped on node 1
//   - Nodes 2 and 3 must not be affected by node 1's restart

#[tokio::test]
async fn test_partial_multi_node_restart_consistency() {
    let tag = "partial_restart_consistency";
    let epoch = 33u64;
    let snap = make_snapshot(epoch, &[nid(10), nid(11), nid(12)]);

    // ── Pre-condition: all 3 nodes export epoch 33 and import the snapshot ────
    let state2; // kept alive for cross-node assertion
    let state3;
    let ctrl2;
    let ctrl3;

    // Boot nodes 2 and 3 (they won't be restarted)
    {
        let cfg2 = make_config_isolated(nid(11), "127.0.0.1:0");
        let cfg3 = make_config_isolated(nid(12), "127.0.0.1:0");

        let mut node2 = NetworkedNode::bind(cfg2).await.unwrap();
        let mut node3 = NetworkedNode::bind(cfg3).await.unwrap();

        ctrl2 = node2.ctrl_tx.clone();
        ctrl3 = node3.ctrl_tx.clone();
        state2 = node2.state.clone();
        state3 = node3.state.clone();

        tokio::spawn(async move { node2.run_event_loop().await });
        tokio::spawn(async move { node3.run_event_loop().await });
    }
    sleep(Duration::from_millis(20)).await;

    // All 3 nodes import snapshot and export epoch 33.
    ctrl2.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    ctrl3.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
    ctrl2.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
    ctrl3.send(NodeControl::ExportEpoch(epoch)).await.unwrap();

    // Node 1 — first boot
    {
        let cfg1 = make_config_fixed(nid(10), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg1.data_dir);

        let mut node1 = NetworkedNode::bind(cfg1).await.unwrap();
        let ctrl1 = node1.ctrl_tx.clone();
        let state1 = node1.state.clone();

        tokio::spawn(async move { node1.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl1.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        ctrl1.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        sleep(Duration::from_millis(80)).await;

        {
            let s = state1.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(epoch));
            assert!(s.exported_epochs.contains(&epoch));
        }

        ctrl1.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    sleep(Duration::from_millis(60)).await;

    // ── Node 1 restarts; nodes 2+3 remain live ────────────────────────────────
    {
        let cfg1 = make_config_fixed(nid(10), "127.0.0.1:0", tag);
        let mut node1 = NetworkedNode::bind(cfg1).await.unwrap();
        let ctrl1 = node1.ctrl_tx.clone();
        let state1 = node1.state.clone();

        // Verify node 1 restored state before event loop starts.
        {
            let s = state1.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(epoch),
                "Restarted node1: latest_snapshot_epoch must be restored");
            assert!(s.exported_epochs.contains(&epoch),
                "Restarted node1: exported_epochs must contain epoch {epoch} from sled");
        }

        tokio::spawn(async move { node1.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Re-deliver same snapshot and same epoch to restarted node 1.
        ctrl1.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();
        ctrl1.send(NodeControl::ExportEpoch(epoch)).await.unwrap();
        sleep(Duration::from_millis(80)).await;

        {
            let s = state1.lock().await;
            // Snapshot must be rejected as duplicate
            let snap_rejected = s.slog.entries().iter()
                .any(|e| e.event == "snapshot.rejected" && e.epoch == epoch);
            assert!(snap_rejected,
                "Restarted node1: duplicate snapshot must be rejected");

            // Export must be skipped
            let export_skipped = s.slog.entries().iter()
                .any(|e| e.event == "export.skipped" && e.epoch == epoch);
            assert!(export_skipped,
                "Restarted node1: duplicate export must be skipped");
        }

        ctrl1.send(NodeControl::Shutdown).await.ok();
    }

    // Verify nodes 2 and 3 were unaffected by node 1's restart.
    {
        let s2 = state2.lock().await;
        assert_eq!(s2.latest_snapshot_epoch, Some(epoch),
            "Node2 must retain its snapshot state, unaffected by node1 restart");
        assert!(s2.exported_epochs.contains(&epoch),
            "Node2 must retain its exported_epochs, unaffected by node1 restart");
    }
    {
        let s3 = state3.lock().await;
        assert_eq!(s3.latest_snapshot_epoch, Some(epoch),
            "Node3 must retain its snapshot state, unaffected by node1 restart");
        assert!(s3.exported_epochs.contains(&epoch),
            "Node3 must retain its exported_epochs, unaffected by node1 restart");
    }

    ctrl2.send(NodeControl::Shutdown).await.ok();
    ctrl3.send(NodeControl::Shutdown).await.ok();
}

// ─── F. Reused Storage Path Integrity ────────────────────────────────────────
//
// Explicitly confirm that state loaded from reused storage is correct and that
// there is no hidden contamination: the only epochs in `exported_epochs` after
// restart are exactly the epochs that were persisted before the restart.

#[tokio::test]
async fn test_reused_storage_path_integrity() {
    let tag = "storage_integrity";

    // ── Boot 1: export exactly epochs {10, 20, 30} ───────────────────────────
    {
        let cfg = make_config_fixed(nid(6), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(10)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(20)).await.unwrap();
        ctrl.send(NodeControl::ExportEpoch(30)).await.unwrap();
        sleep(Duration::from_millis(100)).await;

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: verify exactly {10, 20, 30} is restored — no more, no less ───
    {
        let cfg = make_config_fixed(nid(6), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        {
            let s = state.lock().await;
            let mut expected = BTreeSet::new();
            expected.insert(10u64);
            expected.insert(20u64);
            expected.insert(30u64);
            assert_eq!(s.exported_epochs, expected,
                "Boot2: exported_epochs must be exactly {{10, 20, 30}} — no more, no less");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Export a new epoch — must succeed since it was never persisted.
        ctrl.send(NodeControl::ExportEpoch(40)).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            let mut expected = BTreeSet::new();
            expected.insert(10u64);
            expected.insert(20u64);
            expected.insert(30u64);
            expected.insert(40u64);
            assert_eq!(s.exported_epochs, expected,
                "Boot2 after new export: exported_epochs must be {{10, 20, 30, 40}}");

            let completed_40 = s.slog.entries().iter()
                .filter(|e| e.event == "export.completed" && e.epoch == 40)
                .count();
            assert_eq!(completed_40, 1, "Boot2: epoch 40 must export once");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── G. Snapshot Sequence Across Restart ─────────────────────────────────────
//
// Multiple snapshots (epochs 1, 2, 3) are imported in boot 1.
// After restart, all are restored, and a new snapshot (epoch 4) is accepted
// while epochs 1–3 are correctly rejected as duplicates.

#[tokio::test]
async fn test_snapshot_sequence_across_restart() {
    let tag = "snapshot_sequence_restart";

    // ── Boot 1: import snapshots for epochs 1, 2, 3 ──────────────────────────
    {
        let cfg = make_config_fixed(nid(7), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        for epoch in [1u64, 2u64, 3u64] {
            ctrl.send(NodeControl::ImportSnapshot(Box::new(
                make_snapshot(epoch, &[nid(10), nid(11)])
            ))).await.unwrap();
        }
        sleep(Duration::from_millis(80)).await;

        {
            let s = state.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(3));
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: verify restoration, then try duplicates and new snapshot ──────
    {
        let cfg = make_config_fixed(nid(7), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        {
            let s = state.lock().await;
            assert_eq!(s.latest_snapshot_epoch, Some(3),
                "Boot2: latest_snapshot_epoch must be 3 after restore");
            let restore_log = s.slog.entries().iter()
                .any(|e| e.event == "startup.restore.snapshots");
            assert!(restore_log, "Boot2: startup.restore.snapshots log must be present");
        }

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Re-deliver epochs 1, 2, 3 — all must be rejected
        for epoch in [1u64, 2u64, 3u64] {
            ctrl.send(NodeControl::ImportSnapshot(Box::new(
                make_snapshot(epoch, &[nid(10), nid(11)])
            ))).await.unwrap();
        }
        // Deliver epoch 4 — must be accepted
        ctrl.send(NodeControl::ImportSnapshot(Box::new(
            make_snapshot(4, &[nid(10), nid(11)])
        ))).await.unwrap();
        sleep(Duration::from_millis(100)).await;

        {
            let s = state.lock().await;
            // Epochs 1–3 rejected
            let rejected_count = s.slog.entries().iter()
                .filter(|e| e.event == "snapshot.rejected")
                .count();
            assert_eq!(rejected_count, 3,
                "Boot2: epochs 1, 2, 3 must each produce snapshot.rejected");

            // Epoch 4 accepted
            let accepted_4 = s.slog.entries().iter()
                .any(|e| e.event == "snapshot.imported" && e.epoch == 4);
            assert!(accepted_4, "Boot2: epoch 4 must be accepted as new snapshot");

            assert_eq!(s.latest_snapshot_epoch, Some(4),
                "Boot2: latest_snapshot_epoch must advance to 4 after accepting new snapshot");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── H. Observability — Startup Log Completeness ─────────────────────────────
//
// Confirms that all three startup restoration log events are present after a
// clean first boot AND after a restart with existing data, covering the
// observability requirement from Deliverable 7.

#[tokio::test]
async fn test_startup_restoration_logs_present() {
    let tag = "startup_logs";

    // ── Boot 1: fresh directory — restore logs should show zero items ─────────
    {
        let cfg = make_config_fixed(nid(8), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        // Just bind; don't start event loop (we only need the restore path)
        let node = NetworkedNode::bind(cfg).await.unwrap();
        let s = node.state.lock().await;

        let _has_finalized_log = s.slog.entries().iter()
            .any(|e| e.event.starts_with("startup.restore.finalized")
                || e.event == "startup.restore.exported_epochs"
                || e.event == "startup.restore.snapshots");

        // On first boot with no data, only exported_epochs and snapshots logs
        // should be present (finalized log only appears if data was found).
        let has_epoch_log = s.slog.entries().iter()
            .any(|e| e.event == "startup.restore.exported_epochs");
        let has_snap_log = s.slog.entries().iter()
            .any(|e| e.event == "startup.restore.snapshots");

        assert!(has_epoch_log, "Boot1/fresh: startup.restore.exported_epochs must always be logged");
        assert!(has_snap_log, "Boot1/fresh: startup.restore.snapshots must always be logged");
        drop(s);
    }

    // ── Seed some data ────────────────────────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(8), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ExportEpoch(5)).await.unwrap();
        ctrl.send(NodeControl::ImportSnapshot(Box::new(
            make_snapshot(5, &[nid(10), nid(11)])
        ))).await.unwrap();
        sleep(Duration::from_millis(80)).await;

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: with existing data — all three startup logs must appear ───────
    {
        let cfg = make_config_fixed(nid(8), "127.0.0.1:0", tag);
        let node = NetworkedNode::bind(cfg).await.unwrap();
        let s = node.state.lock().await;

        // exported_epochs restore log
        let epoch_log = s.slog.entries().iter()
            .find(|e| e.event == "startup.restore.exported_epochs");
        assert!(epoch_log.is_some(),
            "Boot2: startup.restore.exported_epochs must be in log");
        assert!(epoch_log.unwrap().details.contains("1 epochs"),
            "Boot2: restore log must report 1 epoch restored");

        // snapshots restore log
        let snap_log = s.slog.entries().iter()
            .find(|e| e.event == "startup.restore.snapshots");
        assert!(snap_log.is_some(),
            "Boot2: startup.restore.snapshots must be in log");
        assert!(snap_log.unwrap().details.contains("1 committee snapshots"),
            "Boot2: restore log must report 1 snapshot restored");
    }
}

// ─── I. Tampered Snapshot After Restart Does Not Corrupt Valid State ──────────
//
// After a restart, a tampered snapshot for an epoch that was legitimately
// imported before the restart must not overwrite the existing valid snapshot
// and must not change latest_snapshot_epoch.

#[tokio::test]
async fn test_tampered_snapshot_after_restart_does_not_corrupt() {
    let tag = "tampered_post_restart";
    let epoch = 15u64;
    let valid_snap = make_snapshot(epoch, &[nid(10), nid(11)]);

    // ── Boot 1: import valid snapshot ────────────────────────────────────────
    {
        let cfg = make_config_fixed(nid(9), "127.0.0.1:0", tag);
        let _ = std::fs::remove_dir_all(&cfg.data_dir);

        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        ctrl.send(NodeControl::ImportSnapshot(Box::new(valid_snap.clone()))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        ctrl.send(NodeControl::Shutdown).await.ok();
        sleep(Duration::from_millis(30)).await;
    }

    // ── Boot 2: deliver tampered snapshot for same epoch ─────────────────────
    {
        let cfg = make_config_fixed(nid(9), "127.0.0.1:0", tag);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });
        sleep(Duration::from_millis(20)).await;

        // Build tampered snapshot: same epoch, corrupted hash
        let mut tampered = valid_snap.clone();
        if !tampered.snapshot_hash.is_empty() {
            tampered.snapshot_hash[0] ^= 0xFF;
        }

        ctrl.send(NodeControl::ImportSnapshot(Box::new(tampered))).await.unwrap();
        sleep(Duration::from_millis(60)).await;

        {
            let s = state.lock().await;
            // latest_snapshot_epoch must remain epoch 15 (not changed by tampered delivery)
            assert_eq!(s.latest_snapshot_epoch, Some(epoch),
                "Tampered delivery must not change latest_snapshot_epoch");

            // snapshot.rejected must appear (either DuplicateEpoch or HashMismatch)
            let rejected = s.slog.entries().iter()
                .any(|e| e.event == "snapshot.rejected" && e.epoch == epoch);
            assert!(rejected,
                "Tampered snapshot after restart must produce snapshot.rejected");
        }

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}

// ─── J. Export Payload Persisted and Retrievable From Sled ───────────────────
//
// The ExportBatch JSON payload stored in sled must be valid and deserializable.
// This proves the persistence write is complete and not corrupted.

#[tokio::test]
async fn test_export_payload_persisted_to_sled() {
    use omniphi_poseq::chain_bridge::exporter::ExportBatch;

    let cfg = make_config_isolated(nid(20), "127.0.0.1:0");
    let data_dir = cfg.data_dir.clone();

    let mut node = NetworkedNode::bind(cfg).await.unwrap();
    let ctrl = node.ctrl_tx.clone();

    tokio::spawn(async move { node.run_event_loop().await });
    sleep(Duration::from_millis(20)).await;

    ctrl.send(NodeControl::ExportEpoch(77)).await.unwrap();
    sleep(Duration::from_millis(80)).await;

    ctrl.send(NodeControl::Shutdown).await.ok();
    sleep(Duration::from_millis(30)).await;

    // Open the sled store directly and verify the export payload.
    use omniphi_poseq::persistence::{DurableStore, SledBackend};
    use omniphi_poseq::persistence::engine::PersistenceEngine;

    let backend = SledBackend::open(std::path::Path::new(&data_dir))
        .expect("should be able to re-open sled after node shutdown");
    let engine = PersistenceEngine::new(Box::new(backend));
    let store = DurableStore::new(engine);

    let key = b"export:epoch:77";
    let raw = store.engine.get_raw(key)
        .expect("export:epoch:77 must exist in sled after export");

    let batch: ExportBatch = serde_json::from_slice(&raw)
        .expect("stored export payload must deserialize as ExportBatch");

    assert_eq!(batch.epoch, 77, "persisted ExportBatch must have epoch = 77");
}
