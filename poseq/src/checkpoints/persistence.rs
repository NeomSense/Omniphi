//! Persist PoSeq checkpoints to sled for crash-safe recovery.
//!
//! Checkpoints are stored under the `snapshot:` prefix keyed by epoch (big-endian).
//! A metadata index under `snapshot_meta:` tracks available checkpoints.

use std::sync::Arc;
use tokio::sync::Mutex;

use crate::checkpoints::{
    CheckpointError, CheckpointMetadata, CheckpointPolicy, PoSeqCheckpoint, SnapshotExportRecord,
};
use crate::persistence::durable_store::DurableStore;
use crate::persistence::keys::prefix;

/// Persisted checkpoint store backed by sled.
pub struct PersistedCheckpointStore {
    store: Arc<Mutex<DurableStore>>,
    policy: CheckpointPolicy,
}

impl PersistedCheckpointStore {
    pub fn new(store: Arc<Mutex<DurableStore>>, policy: CheckpointPolicy) -> Self {
        PersistedCheckpointStore { store, policy }
    }

    /// Save a checkpoint to sled. Verifies integrity before writing.
    pub async fn save_checkpoint(
        &self,
        checkpoint: &PoSeqCheckpoint,
    ) -> Result<SnapshotExportRecord, CheckpointError> {
        if !checkpoint.verify_id() {
            return Err(CheckpointError::IdMismatch(checkpoint.checkpoint_id));
        }

        let epoch = checkpoint.metadata.epoch;
        let key = Self::checkpoint_key(epoch);
        let meta_key = Self::meta_key(epoch);

        let data = bincode::serialize(checkpoint)
            .map_err(|e| CheckpointError::SerializationFailed(e.to_string()))?;

        let size_bytes = data.len() as u64;

        let meta = bincode::serialize(&checkpoint.metadata)
            .map_err(|e| CheckpointError::SerializationFailed(e.to_string()))?;

        let mut ds = self.store.lock().await;
        ds.engine.put_raw(&key, data);
        ds.engine.put_raw(&meta_key, meta);

        Ok(SnapshotExportRecord {
            checkpoint_id: checkpoint.checkpoint_id,
            export_seq: epoch,
            format_version: 1,
            size_bytes,
        })
    }

    /// Load the latest checkpoint from sled.
    pub async fn load_latest(&self) -> Option<PoSeqCheckpoint> {
        let ds = self.store.lock().await;
        let entries = ds.engine.prefix_scan_raw(prefix::SNAPSHOT);

        // Keys are `snapshot:<epoch_be>`, so last in sorted order = highest epoch
        let (_, data) = entries.into_iter().last()?;
        bincode::deserialize(&data).ok()
    }

    /// Load checkpoint by epoch.
    pub async fn load_by_epoch(&self, epoch: u64) -> Option<PoSeqCheckpoint> {
        let key = Self::checkpoint_key(epoch);
        let ds = self.store.lock().await;
        let data = ds.engine.get_raw(&key)?;
        bincode::deserialize(&data).ok()
    }

    /// List available checkpoint metadata.
    pub async fn list_available(&self) -> Vec<CheckpointMetadata> {
        let ds = self.store.lock().await;
        let entries = ds.engine.prefix_scan_raw(prefix::SNAPSHOT_META);
        entries
            .into_iter()
            .filter_map(|(_, data)| bincode::deserialize(&data).ok())
            .collect()
    }

    /// Prune checkpoints older than policy allows.
    pub async fn prune_old_checkpoints(&self) -> usize {
        let metas = self.list_available().await;
        if metas.len() <= self.policy.max_checkpoints_retained {
            return 0;
        }

        let to_prune = metas.len() - self.policy.max_checkpoints_retained;
        let mut epochs: Vec<u64> = metas.iter().map(|m| m.epoch).collect();
        epochs.sort();

        let mut pruned = 0;
        let mut ds = self.store.lock().await;
        for &epoch in epochs.iter().take(to_prune) {
            let key = Self::checkpoint_key(epoch);
            let meta_key = Self::meta_key(epoch);
            ds.engine.delete_raw(&key);
            ds.engine.delete_raw(&meta_key);
            pruned += 1;
        }
        pruned
    }

    fn checkpoint_key(epoch: u64) -> Vec<u8> {
        let mut key = prefix::SNAPSHOT.to_vec();
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    fn meta_key(epoch: u64) -> Vec<u8> {
        let mut key = prefix::SNAPSHOT_META.to_vec();
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::checkpoints::make_test_checkpoint;
    use crate::persistence::backend::InMemoryBackend;
    use crate::persistence::engine::PersistenceEngine;

    async fn make_store() -> (PersistedCheckpointStore, Arc<Mutex<DurableStore>>) {
        let backend = InMemoryBackend::new();
        let engine = PersistenceEngine::new(Box::new(backend));
        let ds = Arc::new(Mutex::new(DurableStore::new(engine)));
        let policy = CheckpointPolicy {
            checkpoint_interval_epochs: 5,
            max_checkpoints_retained: 3,
        };
        (PersistedCheckpointStore::new(ds.clone(), policy), ds)
    }

    #[tokio::test]
    async fn test_save_and_load_by_epoch() {
        let (store, _) = make_store().await;
        let cp = make_test_checkpoint(10, 0);
        store.save_checkpoint(&cp).await.unwrap();

        let loaded = store.load_by_epoch(10).await.unwrap();
        assert_eq!(loaded.checkpoint_id, cp.checkpoint_id);
        assert_eq!(loaded.metadata.epoch, 10);
    }

    #[tokio::test]
    async fn test_load_latest() {
        let (store, _) = make_store().await;
        store.save_checkpoint(&make_test_checkpoint(5, 0)).await.unwrap();
        store.save_checkpoint(&make_test_checkpoint(10, 1)).await.unwrap();
        store.save_checkpoint(&make_test_checkpoint(15, 2)).await.unwrap();

        let latest = store.load_latest().await.unwrap();
        assert_eq!(latest.metadata.epoch, 15);
    }

    #[tokio::test]
    async fn test_load_nonexistent_returns_none() {
        let (store, _) = make_store().await;
        assert!(store.load_by_epoch(999).await.is_none());
    }

    #[tokio::test]
    async fn test_list_available() {
        let (store, _) = make_store().await;
        store.save_checkpoint(&make_test_checkpoint(5, 0)).await.unwrap();
        store.save_checkpoint(&make_test_checkpoint(10, 1)).await.unwrap();

        let metas = store.list_available().await;
        assert_eq!(metas.len(), 2);
    }

    #[tokio::test]
    async fn test_prune_old() {
        let (store, _) = make_store().await;
        // Policy retains 3; add 5
        for i in 1u64..=5 {
            store.save_checkpoint(&make_test_checkpoint(i * 5, i)).await.unwrap();
        }

        let pruned = store.prune_old_checkpoints().await;
        assert_eq!(pruned, 2); // 5 - 3 = 2 pruned

        let remaining = store.list_available().await;
        assert_eq!(remaining.len(), 3);
    }

    #[tokio::test]
    async fn test_tampered_checkpoint_rejected() {
        let (store, _) = make_store().await;
        let mut cp = make_test_checkpoint(10, 0);
        cp.metadata.epoch = 999; // tamper without recomputing id
        let result = store.save_checkpoint(&cp).await;
        assert!(matches!(result, Err(CheckpointError::IdMismatch(_))));
    }

    #[tokio::test]
    async fn test_serialization_roundtrip() {
        let (store, _) = make_store().await;
        let cp = make_test_checkpoint(20, 5);
        store.save_checkpoint(&cp).await.unwrap();

        let loaded = store.load_by_epoch(20).await.unwrap();
        assert_eq!(loaded.metadata.version, cp.metadata.version);
        assert_eq!(loaded.epoch_state_hash, cp.epoch_state_hash);
        assert_eq!(loaded.bridge_state_hash, cp.bridge_state_hash);
        assert!(loaded.verify_id());
    }
}
