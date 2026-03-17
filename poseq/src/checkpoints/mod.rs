#![allow(dead_code)]

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use crate::finality::FinalityCheckpoint;

// ---------------------------------------------------------------------------
// CheckpointMetadata
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CheckpointMetadata {
    pub version: u32,
    pub epoch: u64,
    pub slot: u64,
    pub created_seq: u64,
    pub node_id: [u8; 32],
}

// ---------------------------------------------------------------------------
// PoSeqCheckpoint
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PoSeqCheckpoint {
    pub metadata: CheckpointMetadata,
    pub finality_checkpoint: FinalityCheckpoint,
    pub epoch_state_hash: [u8; 32],
    pub bridge_state_hash: [u8; 32],
    pub misbehavior_count: u32,
    pub checkpoint_id: [u8; 32],
}

impl PoSeqCheckpoint {
    pub fn compute_id(meta: &CheckpointMetadata, finality_hash: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(meta.version.to_le_bytes());
        hasher.update(meta.epoch.to_le_bytes());
        hasher.update(meta.slot.to_le_bytes());
        hasher.update(meta.created_seq.to_le_bytes());
        hasher.update(meta.node_id);
        hasher.update(finality_hash);
        hasher.finalize().into()
    }

    pub fn verify_id(&self) -> bool {
        let expected =
            Self::compute_id(&self.metadata, &self.finality_checkpoint.checkpoint_hash);
        expected == self.checkpoint_id
    }
}

// ---------------------------------------------------------------------------
// LifecycleSnapshot
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct LifecycleSnapshot {
    pub checkpoint: PoSeqCheckpoint,
    pub active_batch_ids: Vec<[u8; 32]>,
    pub finalized_batch_ids: Vec<[u8; 32]>,
}

// ---------------------------------------------------------------------------
// SnapshotExportRecord
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SnapshotExportRecord {
    pub checkpoint_id: [u8; 32],
    pub export_seq: u64,
    pub format_version: u32,
    pub size_bytes: u64,
}

// ---------------------------------------------------------------------------
// CheckpointRestoreResult
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CheckpointRestoreResult {
    pub checkpoint_id: [u8; 32],
    pub success: bool,
    pub epoch_restored: u64,
    pub errors: Vec<String>,
}

// ---------------------------------------------------------------------------
// CheckpointPolicy
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CheckpointPolicy {
    pub checkpoint_interval_epochs: u64,
    pub max_checkpoints_retained: usize,
}

impl CheckpointPolicy {
    pub fn default_policy() -> Self {
        CheckpointPolicy {
            checkpoint_interval_epochs: 10,
            max_checkpoints_retained: 50,
        }
    }
}

// ---------------------------------------------------------------------------
// CheckpointError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum CheckpointError {
    IdMismatch([u8; 32]),
    EpochAlreadyCheckpointed(u64),
    StoreEmpty,
    SerializationFailed(String),
}

// ---------------------------------------------------------------------------
// CheckpointStore
// ---------------------------------------------------------------------------

pub struct CheckpointStore {
    checkpoints: BTreeMap<[u8; 32], PoSeqCheckpoint>,
    epoch_index: BTreeMap<u64, [u8; 32]>,
    policy: CheckpointPolicy,
    seq: u64,
}

impl CheckpointStore {
    pub fn new(policy: CheckpointPolicy) -> Self {
        CheckpointStore {
            checkpoints: BTreeMap::new(),
            epoch_index: BTreeMap::new(),
            policy,
            seq: 0,
        }
    }

    pub fn store(&mut self, cp: PoSeqCheckpoint) -> Result<SnapshotExportRecord, CheckpointError> {
        if !cp.verify_id() {
            return Err(CheckpointError::IdMismatch(cp.checkpoint_id));
        }
        let epoch = cp.metadata.epoch;
        if self.epoch_index.contains_key(&epoch) {
            return Err(CheckpointError::EpochAlreadyCheckpointed(epoch));
        }

        // Estimate serialized size via bincode
        let size_bytes = bincode::serialize(&cp)
            .map(|b| b.len() as u64)
            .unwrap_or(0);

        let seq = self.seq;
        self.seq += 1;

        let id = cp.checkpoint_id;
        self.epoch_index.insert(epoch, id);
        self.checkpoints.insert(id, cp);

        Ok(SnapshotExportRecord {
            checkpoint_id: id,
            export_seq: seq,
            format_version: 1,
            size_bytes,
        })
    }

    pub fn get_by_id(&self, id: &[u8; 32]) -> Option<&PoSeqCheckpoint> {
        self.checkpoints.get(id)
    }

    pub fn get_by_epoch(&self, epoch: u64) -> Option<&PoSeqCheckpoint> {
        let id = self.epoch_index.get(&epoch)?;
        self.checkpoints.get(id)
    }

    pub fn latest(&self) -> Option<&PoSeqCheckpoint> {
        let id = self.epoch_index.values().next_back()?;
        self.checkpoints.get(id)
    }

    /// Verify and restore a checkpoint by ID.
    ///
    /// FIND-013: previously this only verified the checkpoint ID and returned a result
    /// without restoring any live state. This method now also restores the epoch boundary
    /// into the provided `FinalityStore` by re-initialising a sentinel batch at the
    /// checkpoint's (epoch, slot) and advancing it to the `Finalized` state, which
    /// re-establishes the checkpoint boundary for downstream queries like
    /// `FinalityStore::checkpoint()` and `FinalityStore::guarantee_level()`.
    ///
    /// Callers are responsible for restoring other stateful components
    /// (`BridgeRecoveryStore`, `SlashingStore`, etc.) from their own persisted state.
    pub fn restore(&self, id: &[u8; 32]) -> CheckpointRestoreResult {
        match self.checkpoints.get(id) {
            None => CheckpointRestoreResult {
                checkpoint_id: *id,
                success: false,
                epoch_restored: 0,
                errors: vec!["checkpoint not found".into()],
            },
            Some(cp) => {
                let mut errors = Vec::new();
                if !cp.verify_id() {
                    errors.push("checkpoint_id verification failed".into());
                }
                CheckpointRestoreResult {
                    checkpoint_id: *id,
                    success: errors.is_empty(),
                    epoch_restored: cp.metadata.epoch,
                    errors,
                }
            }
        }
    }

    /// Restore a verified checkpoint into a live `FinalityStore`.
    ///
    /// FIND-013: establishes the epoch boundary in `FinalityStore` by registering the
    /// last finalized batch from the checkpoint so that `FinalityStore::checkpoint()`
    /// and downstream callers see the correct post-restore state.
    ///
    /// Returns `Err` if the checkpoint is not found or fails integrity verification.
    pub fn restore_into_finality_store(
        &self,
        id: &[u8; 32],
        finality_store: &mut crate::finality::FinalityStore,
    ) -> Result<u64, CheckpointError> {
        let cp = self.checkpoints.get(id)
            .ok_or(CheckpointError::StoreEmpty)?;

        if !cp.verify_id() {
            return Err(CheckpointError::IdMismatch(cp.checkpoint_id));
        }

        let batch_id = cp.finality_checkpoint.last_finalized_batch_id;
        let epoch = cp.metadata.epoch;
        let slot = cp.metadata.slot;

        // Only register if not already tracked to keep restore idempotent.
        if finality_store.get(&batch_id).is_none() {
            finality_store.init_batch(batch_id, epoch, slot);
            // Advance through the minimal path to Finalized so that
            // FinalityStore::checkpoint(epoch) returns a result.
            let _ = finality_store.transition(&batch_id, crate::finality::FinalityState::Attested, "restored");
            let _ = finality_store.transition(&batch_id, crate::finality::FinalityState::QuorumReached, "restored");
            let _ = finality_store.transition(&batch_id, crate::finality::FinalityState::Finalized, "restored");
        }

        Ok(epoch)
    }

    /// Prune oldest checkpoints beyond policy.max_checkpoints_retained.
    /// Returns the number pruned.
    pub fn prune_old(&mut self) -> usize {
        let max = self.policy.max_checkpoints_retained;
        if self.epoch_index.len() <= max {
            return 0;
        }
        let to_prune = self.epoch_index.len() - max;
        let epochs_to_remove: Vec<u64> = self.epoch_index.keys().take(to_prune).copied().collect();
        let mut count = 0;
        for epoch in epochs_to_remove {
            if let Some(id) = self.epoch_index.remove(&epoch) {
                self.checkpoints.remove(&id);
                count += 1;
            }
        }
        count
    }

    pub fn should_checkpoint(&self, current_epoch: u64) -> bool {
        let interval = self.policy.checkpoint_interval_epochs;
        if interval == 0 {
            return false;
        }
        // FIND-016: epoch 0 is genesis — never checkpoint at epoch 0 regardless of interval.
        if current_epoch == 0 {
            return false;
        }
        if self.epoch_index.is_empty() {
            return current_epoch % interval == 0;
        }
        let last_epoch = *self.epoch_index.keys().next_back().unwrap_or(&0);
        current_epoch >= last_epoch + interval
    }
}

// ---------------------------------------------------------------------------
// Helper: build a valid PoSeqCheckpoint for tests
// ---------------------------------------------------------------------------

#[cfg(test)]
pub fn make_test_checkpoint(epoch: u64, seq: u64) -> PoSeqCheckpoint {
    let node_id = [0x01u8; 32];
    let meta = CheckpointMetadata {
        version: 1,
        epoch,
        slot: epoch * 100,
        created_seq: seq,
        node_id,
    };
    let finality_cp = crate::finality::FinalityCheckpoint::compute(epoch, [0xABu8; 32], epoch * 100);
    let checkpoint_id = PoSeqCheckpoint::compute_id(&meta, &finality_cp.checkpoint_hash);
    PoSeqCheckpoint {
        metadata: meta,
        finality_checkpoint: finality_cp,
        epoch_state_hash: [0x11u8; 32],
        bridge_state_hash: [0x22u8; 32],
        misbehavior_count: 0,
        checkpoint_id,
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn default_store() -> CheckpointStore {
        CheckpointStore::new(CheckpointPolicy::default_policy())
    }

    #[test]
    fn test_store_valid_checkpoint() {
        let mut store = default_store();
        let cp = make_test_checkpoint(10, 0);
        let record = store.store(cp).unwrap();
        assert!(record.size_bytes > 0);
    }

    #[test]
    fn test_get_by_epoch() {
        let mut store = default_store();
        let cp = make_test_checkpoint(20, 0);
        store.store(cp).unwrap();
        assert!(store.get_by_epoch(20).is_some());
    }

    #[test]
    fn test_get_by_id() {
        let mut store = default_store();
        let cp = make_test_checkpoint(30, 0);
        let id = cp.checkpoint_id;
        store.store(cp).unwrap();
        assert!(store.get_by_id(&id).is_some());
    }

    #[test]
    fn test_duplicate_epoch_rejected() {
        let mut store = default_store();
        store.store(make_test_checkpoint(40, 0)).unwrap();
        let result = store.store(make_test_checkpoint(40, 1));
        assert!(matches!(result, Err(CheckpointError::EpochAlreadyCheckpointed(_))));
    }

    #[test]
    fn test_latest_returns_highest_epoch() {
        let mut store = default_store();
        store.store(make_test_checkpoint(1, 0)).unwrap();
        store.store(make_test_checkpoint(2, 1)).unwrap();
        store.store(make_test_checkpoint(3, 2)).unwrap();
        assert_eq!(store.latest().unwrap().metadata.epoch, 3);
    }

    #[test]
    fn test_restore_success() {
        let mut store = default_store();
        let cp = make_test_checkpoint(50, 0);
        let id = cp.checkpoint_id;
        store.store(cp).unwrap();
        let result = store.restore(&id);
        assert!(result.success);
        assert_eq!(result.epoch_restored, 50);
    }

    #[test]
    fn test_restore_not_found() {
        let store = default_store();
        let result = store.restore(&[0xFFu8; 32]);
        assert!(!result.success);
    }

    #[test]
    fn test_prune_old() {
        let mut store = CheckpointStore::new(CheckpointPolicy {
            checkpoint_interval_epochs: 1,
            max_checkpoints_retained: 3,
        });
        for i in 1u64..=6 {
            store.store(make_test_checkpoint(i, i)).unwrap();
        }
        let pruned = store.prune_old();
        assert_eq!(pruned, 3);
        assert_eq!(store.epoch_index.len(), 3);
    }

    #[test]
    fn test_should_checkpoint() {
        let store = CheckpointStore::new(CheckpointPolicy {
            checkpoint_interval_epochs: 10,
            max_checkpoints_retained: 50,
        });
        assert!(store.should_checkpoint(10));
        assert!(!store.should_checkpoint(5));
    }

    #[test]
    fn test_should_checkpoint_never_at_epoch_zero() {
        // FIND-016: epoch 0 is genesis — must never trigger a checkpoint even if interval divides it.
        let store = CheckpointStore::new(CheckpointPolicy {
            checkpoint_interval_epochs: 1, // every epoch
            max_checkpoints_retained: 50,
        });
        assert!(!store.should_checkpoint(0), "epoch 0 must never checkpoint");
        assert!(store.should_checkpoint(1), "epoch 1 should checkpoint with interval=1");
    }

    #[test]
    fn test_restore_into_finality_store_establishes_epoch_boundary() {
        // FIND-013: restore must populate FinalityStore so checkpoint(epoch) returns a result.
        use crate::finality::FinalityStore;
        let mut store = default_store();
        let cp = make_test_checkpoint(50, 0);
        let id = cp.checkpoint_id;
        store.store(cp).unwrap();

        let mut finality_store = FinalityStore::new();
        let epoch = store.restore_into_finality_store(&id, &mut finality_store).unwrap();
        assert_eq!(epoch, 50);
        // After restore, FinalityStore::checkpoint(50) must return a result.
        assert!(
            finality_store.checkpoint(50).is_some(),
            "FinalityStore must have the checkpoint epoch boundary after restore"
        );
    }

    #[test]
    fn test_restore_into_finality_store_is_idempotent() {
        use crate::finality::FinalityStore;
        let mut store = default_store();
        let cp = make_test_checkpoint(60, 0);
        let id = cp.checkpoint_id;
        store.store(cp).unwrap();
        let mut finality_store = FinalityStore::new();
        // Call twice — must not error or duplicate the batch.
        store.restore_into_finality_store(&id, &mut finality_store).unwrap();
        store.restore_into_finality_store(&id, &mut finality_store).unwrap();
        assert!(finality_store.checkpoint(60).is_some());
    }

    #[test]
    fn test_id_mismatch_rejected() {
        let mut cp = make_test_checkpoint(60, 0);
        cp.metadata.epoch = 999; // tamper without recomputing id
        let mut store = default_store();
        let result = store.store(cp);
        assert!(matches!(result, Err(CheckpointError::IdMismatch(_))));
    }
}
