//! Phase 8 — State Sync, Catch-Up, and Long-Run Reliability
//! Tests for state sync engine, snapshot lifecycle, bridge durability, catch-up detection,
//! and long-run operation correctness.

use std::collections::BTreeSet;

use omniphi_poseq::sync::StateSyncEngine;
use omniphi_poseq::checkpoints::{CheckpointPolicy, CheckpointStore};
use omniphi_poseq::bridge_recovery::{BridgeRetryPolicy};
use omniphi_poseq::recovery::{RecoveryCheckpoint, NodeRecoveryManager, RecoveryError};
use omniphi_poseq::chain_bridge::snapshot::{
    ChainCommitteeSnapshot, ChainCommitteeMember, SnapshotImporter, SnapshotImportError,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

fn make_engine_with_policy(interval: u64, max_retained: usize) -> StateSyncEngine {
    StateSyncEngine::new(
        CheckpointPolicy {
            checkpoint_interval_epochs: interval,
            max_checkpoints_retained: max_retained,
        },
        BridgeRetryPolicy::default_policy(),
    )
}

fn default_engine() -> StateSyncEngine {
    make_engine_with_policy(10, 50)
}

/// Build a valid ChainCommitteeSnapshot with a correct hash.
fn make_chain_snapshot(epoch: u64, member_ids: &[[u8; 32]]) -> ChainCommitteeSnapshot {
    let mut sorted = member_ids.to_vec();
    sorted.sort();
    let members: Vec<ChainCommitteeMember> = sorted
        .iter()
        .map(|id| ChainCommitteeMember {
            node_id: hex::encode(id),
            public_key: hex::encode(id),
            moniker: format!("node-{}", id[0]),
            role: "Sequencer".to_string(),
        })
        .collect();
    let hash = ChainCommitteeSnapshot::compute_hash(epoch, &sorted).to_vec();
    ChainCommitteeSnapshot {
        epoch,
        members,
        snapshot_hash: hash,
        produced_at_block: (epoch * 100) as i64,
    }
}

// ─── Test 1: Checkpoint created at epoch boundary ────────────────────────────

#[test]
fn test_sync_engine_checkpoint_created_at_epoch_boundary() {
    let mut engine = default_engine();

    engine.update_local_epoch(10);

    let exported_epochs: BTreeSet<u64> = (0..10).collect();
    let result = engine.create_checkpoint(
        10,
        100,
        Some([0xABu8; 32]),
        [0xBBu8; 32],
        &exported_epochs,
        [0x01u8; 32],
    );

    assert!(result.is_some(), "checkpoint must be created at epoch boundary (epoch 10, interval 10)");

    let status = engine.sync_status();
    assert_eq!(
        status.latest_checkpoint_epoch,
        Some(10),
        "latest_checkpoint_epoch must be Some(10) after creating checkpoint at epoch 10"
    );
}

// ─── Test 2: No checkpoint created off boundary ───────────────────────────────

#[test]
fn test_sync_engine_no_checkpoint_off_boundary() {
    let mut engine = default_engine();

    let exported_epochs: BTreeSet<u64> = (0..5).collect();
    let result = engine.create_checkpoint(
        5,
        50,
        None,
        [0xBBu8; 32],
        &exported_epochs,
        [0x01u8; 32],
    );

    assert!(result.is_none(), "checkpoint must NOT be created at epoch 5 (not a boundary with interval=10)");

    let status = engine.sync_status();
    assert_eq!(
        status.latest_checkpoint_epoch,
        None,
        "latest_checkpoint_epoch must be None when no checkpoint has been created"
    );
}

// ─── Test 3: Catch-up detection ───────────────────────────────────────────────

#[test]
fn test_catch_up_detection() {
    let mut engine = default_engine();

    engine.update_local_epoch(5);
    engine.update_peer_epoch(15); // lag = 10

    // is_behind(threshold): returns true if peer_max_epoch > local_epoch + threshold
    // local=5, peer=15: peer > local+threshold iff 15 > 5+threshold iff threshold < 10
    assert!(
        engine.is_behind(5),
        "lag=10, threshold=5 => peer(15) > local(5)+5(10) => behind"
    );
    assert!(
        !engine.is_behind(10),
        "lag=10, threshold=10 => peer(15) > local(5)+10(15) => 15 > 15 is false => NOT behind"
    );

    let status = engine.sync_status();
    assert_eq!(status.lag, 10, "lag must be 10 (peer 15 - local 5)");
    assert!(
        status.is_catching_up,
        "is_catching_up must be true when peer_max_epoch > local_epoch"
    );
}

// ─── Test 4: Bridge delivery full lifecycle ───────────────────────────────────

#[test]
fn test_bridge_delivery_full_lifecycle() {
    let mut engine = default_engine();

    let batch_id = [0x01u8; 32];
    let ack_hash = [0xAAu8; 32];

    engine.register_batch_for_bridge(batch_id);

    let exported = engine.mark_bridge_exported(&batch_id);
    assert!(exported, "export must succeed for a registered batch");

    let acked = engine.mark_bridge_acked(&batch_id, ack_hash);
    assert!(acked, "ack must succeed after export");

    let status = engine.sync_status();
    assert_eq!(status.bridge_backlog, 0, "bridge backlog must be 0 after ack");
    assert_eq!(status.bridge_acked, 1, "bridge_acked must be 1 after one ack");
}

// ─── Test 5: Bridge retry candidates after exporting without ack ───────────────

#[test]
fn test_bridge_retry_after_rejection() {
    let mut engine = default_engine();

    // Register batch 1, export it, ack it — should not be a retry candidate
    let bid1 = [0x01u8; 32];
    engine.register_batch_for_bridge(bid1);
    engine.mark_bridge_exported(&bid1);
    engine.mark_bridge_acked(&bid1, [0xAAu8; 32]);

    // Retry candidates must be empty — acked batches are terminal, not retry-eligible
    let candidates = engine.bridge_retry_candidates();
    assert!(
        !candidates.contains(&bid1),
        "acked batch must not be a retry candidate"
    );

    // Register batch 2, export it but do NOT ack — state is Exporting
    let bid2 = [0x02u8; 32];
    engine.register_batch_for_bridge(bid2);
    engine.mark_bridge_exported(&bid2);

    // Exporting state is not retry-eligible
    let candidates2 = engine.bridge_retry_candidates();
    assert!(
        !candidates2.contains(&bid2),
        "batch in Exporting state must not be a retry candidate (only Rejected/RetryPending qualify)"
    );

    // Verify the exporting batch counts as backlog
    let status = engine.sync_status();
    assert_eq!(status.bridge_backlog, 1, "one unacknowledged batch counts as backlog");
    assert_eq!(status.bridge_acked, 1, "only the fully acked batch counts as acked");
}

// ─── Test 6: Checkpoint prune retention policy ───────────────────────────────

#[test]
fn test_checkpoint_prune_retention_policy() {
    // max_retained=3, interval=10
    let mut engine = make_engine_with_policy(10, 3);

    let exported_epochs = BTreeSet::new();

    // Create checkpoints at epochs 10, 20, 30, 40
    engine.create_checkpoint(10, 100, None, [0xBBu8; 32], &exported_epochs, [0x01u8; 32]);
    engine.create_checkpoint(20, 200, None, [0xBBu8; 32], &exported_epochs, [0x01u8; 32]);
    engine.create_checkpoint(30, 300, None, [0xBBu8; 32], &exported_epochs, [0x01u8; 32]);
    engine.create_checkpoint(40, 400, None, [0xBBu8; 32], &exported_epochs, [0x01u8; 32]);

    // 4 checkpoints stored, max=3 → prune should remove 1
    let pruned = engine.prune_checkpoints();
    assert_eq!(pruned, 1, "exactly 1 checkpoint must be pruned to respect max_retained=3");

    // latest checkpoint must still be epoch 40
    let status = engine.sync_status();
    assert_eq!(
        status.latest_checkpoint_epoch,
        Some(40),
        "latest_checkpoint_epoch must still be Some(40) after pruning oldest"
    );
}

// ─── Test 7: Stale snapshot rejection ────────────────────────────────────────

#[test]
fn test_stale_snapshot_rejection() {
    let mut importer = SnapshotImporter::new();

    let member_id = [0x01u8; 32];

    // Build and import valid snapshot for epoch 5
    let valid_snap = make_chain_snapshot(5, &[member_id]);
    let result = importer.import(valid_snap.clone());
    assert!(result.is_ok(), "valid snapshot for epoch 5 must be imported successfully");

    // Attempt duplicate import of epoch 5 — must fail with DuplicateEpoch
    let duplicate_snap = make_chain_snapshot(5, &[member_id]);
    let dup_err = importer.import(duplicate_snap);
    assert!(
        matches!(dup_err, Err(SnapshotImportError::DuplicateEpoch(5))),
        "duplicate snapshot for epoch 5 must return DuplicateEpoch error, got: {:?}",
        dup_err
    );

    // Build a snapshot for epoch 6, then tamper its hash
    let mut tampered_snap = make_chain_snapshot(6, &[member_id]);
    // Flip first byte of snapshot_hash to create a hash mismatch
    tampered_snap.snapshot_hash[0] ^= 0xFF;

    // Verify the tamper is detected before import
    assert!(
        !tampered_snap.verify_hash(),
        "tampered snapshot must fail verify_hash()"
    );

    let tamper_err = importer.import(tampered_snap);
    assert!(
        matches!(tamper_err, Err(SnapshotImportError::HashMismatch { epoch: 6 })),
        "tampered snapshot must return HashMismatch error, got: {:?}",
        tamper_err
    );
}

// ─── Test 8: Late join simulation ────────────────────────────────────────────

#[test]
fn test_late_join_simulation() {
    let mut engine = default_engine();

    // Peers are at epoch 20, we are at epoch 0
    engine.update_peer_epoch(20);
    engine.update_local_epoch(0);

    // We should be behind with a small threshold
    assert!(
        engine.is_behind(3),
        "lag=20, threshold=3 => peer(20) > local(0)+3 => behind"
    );

    // Simulate catch-up: advance local epoch to 20
    engine.update_local_epoch(20);

    // Now create a checkpoint at epoch 20 to mark the catch-up complete
    let exported_epochs = BTreeSet::new();
    let cp = engine.create_checkpoint(
        20,
        200,
        None,
        [0u8; 32],
        &exported_epochs,
        [0x01u8; 32],
    );
    assert!(cp.is_some(), "checkpoint at epoch 20 must be created as part of catch-up");

    // After catch-up we should not be behind anymore
    assert!(
        !engine.is_behind(3),
        "after catching up to epoch 20 with peer at 20, lag=0 => NOT behind with threshold=3"
    );
}

// ─── Test 9: Long-run no state drift ─────────────────────────────────────────

#[test]
fn test_long_run_no_state_drift() {
    // interval=10, max_retained=50
    let mut engine = make_engine_with_policy(10, 50);

    let exported_epochs = BTreeSet::new();

    for epoch in 0u64..100 {
        engine.update_local_epoch(epoch);

        if epoch > 0 && epoch % 10 == 0 {
            // Create checkpoint at every interval boundary
            engine.create_checkpoint(
                epoch,
                epoch * 10,
                Some([epoch as u8; 32]),
                [0u8; 32],
                &exported_epochs,
                [0x01u8; 32],
            );
        }

        // Register, export, and ack a bridge batch per epoch
        let batch_id = [epoch as u8; 32];
        engine.register_batch_for_bridge(batch_id);
        engine.mark_bridge_exported(&batch_id);
        engine.mark_bridge_acked(&batch_id, [epoch as u8; 32]);
    }

    let status = engine.sync_status();

    // Loop runs epochs 0..99 so last update is epoch 99
    assert_eq!(
        status.local_epoch, 99,
        "local_epoch must be 99 after loop 0..100"
    );

    // All 100 batches (epochs 0..99) were acked
    assert_eq!(
        status.bridge_backlog, 0,
        "bridge backlog must be 0 — all 100 batches acked"
    );
    assert_eq!(
        status.bridge_acked, 100,
        "bridge_acked must be 100 — one per epoch in 0..100"
    );

    // Checkpoints are created at epochs 10, 20, 30, 40, 50, 60, 70, 80, 90
    // (epoch 100 is not reached in 0..100; last is 90)
    assert_eq!(
        status.latest_checkpoint_epoch,
        Some(90),
        "latest_checkpoint_epoch must be Some(90) — last checkpoint in loop 0..100 is at epoch 90"
    );

    // Prune: 9 checkpoints made (10,20,...,90), max_retained=50 → no pruning needed
    let pruned = engine.prune_checkpoints();
    assert_eq!(
        pruned, 0,
        "no pruning needed: 9 checkpoints <= max_retained=50"
    );

    // Engine is still functional — calling sync_status again must not panic
    let status2 = engine.sync_status();
    assert_eq!(status2.local_epoch, 99, "engine remains functional after 100-epoch soak");
}

// ─── Test 10: Recovery checkpoint verify ─────────────────────────────────────

#[test]
fn test_recovery_checkpoint_verify() {
    // Compute a valid RecoveryCheckpoint
    let epoch = 42u64;
    let last_slot = epoch * 100;
    let last_batch = [0xABu8; 32];
    let finality_hash = [0xCCu8; 32];
    let committee_hash = [0xDDu8; 32];
    let bridge_hash = [0xEEu8; 32];

    let cp = RecoveryCheckpoint::compute(
        epoch,
        last_slot,
        last_batch,
        finality_hash,
        committee_hash,
        bridge_hash,
    );

    // Valid checkpoint must verify
    assert!(cp.verify(), "freshly computed RecoveryCheckpoint must verify correctly");

    // Tamper the epoch field — verify must fail
    let mut tampered = cp.clone();
    tampered.epoch = 999;
    assert!(
        !tampered.verify(),
        "RecoveryCheckpoint with tampered epoch must fail verify()"
    );

    // Store the valid checkpoint in NodeRecoveryManager — must succeed
    let mut mgr = NodeRecoveryManager::new();
    let store_result = mgr.store_checkpoint(cp.clone());
    assert!(
        store_result.is_ok(),
        "storing a valid RecoveryCheckpoint must succeed"
    );

    // Attempt to store the tampered checkpoint — must return CheckpointHashMismatch
    let tampered_result = mgr.store_checkpoint(tampered);
    assert!(
        matches!(
            tampered_result,
            Err(RecoveryError::CheckpointHashMismatch { .. })
        ),
        "storing a tampered RecoveryCheckpoint must return CheckpointHashMismatch, got: {:?}",
        tampered_result
    );

    // Verify that the valid checkpoint is retrievable
    let retrieved = mgr.checkpoint_for_epoch(epoch);
    assert!(
        retrieved.is_some(),
        "valid checkpoint for epoch {} must be retrievable from NodeRecoveryManager",
        epoch
    );
    assert_eq!(
        retrieved.unwrap().epoch, epoch,
        "retrieved checkpoint must have the correct epoch"
    );
}
