//! State sync engine for PoSeq nodes.
//!
//! Provides:
//! - Periodic checkpoint creation at epoch boundaries
//! - Catch-up detection: node knows if it's behind
//! - Sync status for operator diagnostics
//! - Bridge delivery state persistence

#![allow(dead_code)]

use std::collections::BTreeSet;

use sha2::{Sha256, Digest};

use crate::recovery::{NodeRecoveryManager, RecoveryCheckpoint};
use crate::checkpoints::{CheckpointPolicy, CheckpointStore, CheckpointMetadata, PoSeqCheckpoint};
use crate::bridge_recovery::{BridgeRecoveryStore, BridgeRetryPolicy, BridgeDeliveryState};
use crate::finality::FinalityCheckpoint;

// ---------------------------------------------------------------------------
// SyncStatus
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize)]
pub struct SyncStatus {
    pub local_epoch: u64,
    pub peer_max_epoch: u64,
    pub lag: u64,
    pub is_catching_up: bool,
    pub latest_checkpoint_epoch: Option<u64>,
    pub bridge_backlog: usize,
    pub bridge_acked: usize,
    pub bridge_failed: usize,
}

// ---------------------------------------------------------------------------
// StateSyncEngine
// ---------------------------------------------------------------------------

pub struct StateSyncEngine {
    recovery_mgr: NodeRecoveryManager,
    checkpoint_store: CheckpointStore,
    bridge_store: BridgeRecoveryStore,
    local_epoch: u64,
    peer_max_epoch: u64,
}

impl std::fmt::Debug for StateSyncEngine {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("StateSyncEngine")
            .field("local_epoch", &self.local_epoch)
            .field("peer_max_epoch", &self.peer_max_epoch)
            .finish()
    }
}

impl StateSyncEngine {
    /// Create a new `StateSyncEngine` with the given policies.
    pub fn new(checkpoint_policy: CheckpointPolicy, bridge_policy: BridgeRetryPolicy) -> Self {
        StateSyncEngine {
            recovery_mgr: NodeRecoveryManager::new(),
            checkpoint_store: CheckpointStore::new(checkpoint_policy),
            bridge_store: BridgeRecoveryStore::new(bridge_policy),
            local_epoch: 0,
            peer_max_epoch: 0,
        }
    }

    /// Create a checkpoint if `checkpoint_store.should_checkpoint(epoch)` returns true.
    ///
    /// Returns the checkpoint_id on success, or `None` if no checkpoint was created.
    pub fn create_checkpoint(
        &mut self,
        epoch: u64,
        slot: u64,
        latest_finalized: Option<[u8; 32]>,
        committee_hash: [u8; 32],
        exported_epochs: &BTreeSet<u64>,
        node_id: [u8; 32],
    ) -> Option<[u8; 32]> {
        if !self.checkpoint_store.should_checkpoint(epoch) {
            return None;
        }

        // Build finality checkpoint
        let batch_id = latest_finalized.unwrap_or([0u8; 32]);
        let fc = FinalityCheckpoint::compute(epoch, batch_id, slot);

        // Build recovery checkpoint
        let bridge_hash = {
            // Use a simple hash of the exported epochs set as the bridge_state_hash
            let mut h = Sha256::new();
            Digest::update(&mut h, b"bridge:state:");
            for ep in exported_epochs {
                Digest::update(&mut h, ep.to_le_bytes());
            }
            Digest::finalize(h).into()
        };

        let rc = RecoveryCheckpoint::compute(
            epoch,
            slot,
            batch_id,
            fc.checkpoint_hash,
            committee_hash,
            bridge_hash,
        );
        // Store in recovery manager (ignore errors — don't fail checkpoint creation)
        let _ = self.recovery_mgr.store_checkpoint(rc);

        // Build PoSeqCheckpoint
        let meta = CheckpointMetadata {
            version: 1,
            epoch,
            slot,
            created_seq: epoch, // use epoch as a monotone sequence proxy
            node_id,
        };
        let checkpoint_id = PoSeqCheckpoint::compute_id(&meta, &fc.checkpoint_hash);
        let cp = PoSeqCheckpoint {
            metadata: meta,
            finality_checkpoint: fc,
            epoch_state_hash: committee_hash,
            bridge_state_hash: bridge_hash,
            misbehavior_count: 0,
            checkpoint_id,
        };

        match self.checkpoint_store.store(cp) {
            Ok(_) => Some(checkpoint_id),
            Err(_) => None,
        }
    }

    /// Update `peer_max_epoch` to the max of current and `peer_epoch`.
    pub fn update_peer_epoch(&mut self, peer_epoch: u64) {
        if peer_epoch > self.peer_max_epoch {
            self.peer_max_epoch = peer_epoch;
        }
    }

    /// Update `local_epoch`.
    pub fn update_local_epoch(&mut self, epoch: u64) {
        self.local_epoch = epoch;
    }

    /// Returns true if `peer_max_epoch > local_epoch + threshold`.
    pub fn is_behind(&self, threshold: u64) -> bool {
        self.peer_max_epoch > self.local_epoch + threshold
    }

    /// Returns a snapshot of current sync status.
    pub fn sync_status(&self) -> SyncStatus {
        let lag = self.peer_max_epoch.saturating_sub(self.local_epoch);
        let is_catching_up = self.is_behind(0);

        let latest_checkpoint_epoch = self.checkpoint_store
            .latest()
            .map(|cp| cp.metadata.epoch);

        // Bridge stats from consistency_check
        let check = self.bridge_store.consistency_check();
        let bridge_backlog = check.batches_pending + check.batches_in_retry;
        let bridge_acked = check.batches_acked;
        let bridge_failed = check.batches_failed;

        SyncStatus {
            local_epoch: self.local_epoch,
            peer_max_epoch: self.peer_max_epoch,
            lag,
            is_catching_up,
            latest_checkpoint_epoch,
            bridge_backlog,
            bridge_acked,
            bridge_failed,
        }
    }

    /// Register a batch in the bridge store.
    pub fn register_batch_for_bridge(&mut self, batch_id: [u8; 32]) {
        self.bridge_store.register_batch(batch_id);
    }

    /// Call `try_export` on the batch. Returns `true` on success.
    pub fn mark_bridge_exported(&mut self, batch_id: &[u8; 32]) -> bool {
        self.bridge_store.export_batch(batch_id).is_ok()
    }

    /// Call `acknowledge` on the batch. Returns `true` on success.
    pub fn mark_bridge_acked(&mut self, batch_id: &[u8; 32], ack_hash: [u8; 32]) -> bool {
        self.bridge_store.acknowledge(batch_id, ack_hash).is_ok()
    }

    /// Returns batch_ids in Rejected or RetryPending state.
    pub fn bridge_retry_candidates(&self) -> Vec<[u8; 32]> {
        self.bridge_store
            .all_records()
            .into_iter()
            .filter(|r| {
                matches!(
                    r.state,
                    BridgeDeliveryState::Rejected | BridgeDeliveryState::RetryPending { .. }
                )
            })
            .map(|r| r.batch_id)
            .collect()
    }

    /// Prune old checkpoints per policy. Returns number pruned.
    pub fn prune_checkpoints(&mut self) -> usize {
        self.checkpoint_store.prune_old()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn make_engine() -> StateSyncEngine {
        StateSyncEngine::new(
            CheckpointPolicy::default_policy(),
            BridgeRetryPolicy::default_policy(),
        )
    }

    fn batch_id(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn node_id() -> [u8; 32] {
        [0x01u8; 32]
    }

    fn committee_hash() -> [u8; 32] {
        [0xBBu8; 32]
    }

    #[test]
    fn test_sync_status_initial() {
        let engine = make_engine();
        let status = engine.sync_status();
        assert_eq!(status.local_epoch, 0);
        assert_eq!(status.peer_max_epoch, 0);
        assert_eq!(status.lag, 0);
        assert!(!status.is_catching_up);
        assert!(status.latest_checkpoint_epoch.is_none());
        assert_eq!(status.bridge_backlog, 0);
        assert_eq!(status.bridge_acked, 0);
        assert_eq!(status.bridge_failed, 0);
    }

    #[test]
    fn test_create_checkpoint_at_interval() {
        // Default policy: interval=10. Epoch 10 should create; epoch 11 should not.
        let mut engine = make_engine();
        let exported = BTreeSet::new();

        let cp_id = engine.create_checkpoint(10, 100, None, committee_hash(), &exported, node_id());
        assert!(cp_id.is_some(), "should create checkpoint at epoch 10");

        // Epoch 11 is not at a checkpoint boundary (last was 10, interval=10, so next is 20)
        let cp_id2 = engine.create_checkpoint(11, 110, None, committee_hash(), &exported, node_id());
        assert!(cp_id2.is_none(), "should not create checkpoint at epoch 11");
    }

    #[test]
    fn test_is_behind_detects_lag() {
        let mut engine = make_engine();
        engine.update_local_epoch(5);
        engine.update_peer_epoch(10);

        assert!(engine.is_behind(0), "lag=5, threshold=0 => behind");
        assert!(engine.is_behind(4), "lag=5, threshold=4 => behind");
        assert!(!engine.is_behind(5), "lag=5, threshold=5 => NOT behind");
        assert!(!engine.is_behind(10), "lag=5, threshold=10 => NOT behind");
    }

    #[test]
    fn test_bridge_lifecycle_register_export_ack() {
        let mut engine = make_engine();
        let bid = batch_id(1);
        let ack_hash = [0xAAu8; 32];

        engine.register_batch_for_bridge(bid);
        assert!(engine.mark_bridge_exported(&bid), "export should succeed");
        assert!(engine.mark_bridge_acked(&bid, ack_hash), "ack should succeed");

        let status = engine.sync_status();
        assert_eq!(status.bridge_acked, 1);
        assert_eq!(status.bridge_backlog, 0);
    }

    #[test]
    fn test_bridge_retry_candidates() {
        let mut engine = StateSyncEngine::new(
            CheckpointPolicy::default_policy(),
            BridgeRetryPolicy { max_attempts: 3, backoff_base_seq: 10 },
        );

        let bid1 = batch_id(10);
        let bid2 = batch_id(11);

        // bid1: export then force-retry path via reject_and_retry
        engine.register_batch_for_bridge(bid1);
        engine.bridge_store.export_batch(&bid1).unwrap();
        engine.bridge_store.reject_and_retry(&bid1).unwrap();

        // bid2: just register (Pending — not a retry candidate)
        engine.register_batch_for_bridge(bid2);

        let candidates = engine.bridge_retry_candidates();
        assert!(candidates.contains(&bid1), "bid1 should be a retry candidate");
        assert!(!candidates.contains(&bid2), "bid2 in Pending should not be a retry candidate");
    }

    #[test]
    fn test_prune_checkpoints() {
        let mut engine = StateSyncEngine::new(
            CheckpointPolicy {
                checkpoint_interval_epochs: 1,
                max_checkpoints_retained: 3,
            },
            BridgeRetryPolicy::default_policy(),
        );

        let exported = BTreeSet::new();
        // Create 6 checkpoints at epochs 1–6
        for epoch in 1u64..=6 {
            engine.create_checkpoint(epoch, epoch * 10, None, committee_hash(), &exported, node_id());
        }

        let pruned = engine.prune_checkpoints();
        assert_eq!(pruned, 3, "should have pruned 3 checkpoints");
    }
}
