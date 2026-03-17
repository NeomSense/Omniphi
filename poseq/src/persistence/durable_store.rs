//! `DurableStore` — serialization + flush layer over `PersistenceEngine`.
//!
//! This module is the boundary between domain types and raw bytes.  Every
//! write path that must survive a crash goes through here:
//!
//! 1. Serialize the domain value with `bincode`.
//! 2. Write raw bytes via `PersistenceEngine`.
//! 3. Optionally flush (callers control durability / throughput trade-off).
//!
//! # Key namespaces added here
//!
//! | Prefix              | Contents                                      |
//! |---------------------|-----------------------------------------------|
//! | `finality:`         | `BatchFinalityStatus` per batch_id            |
//! | `bridge_rec:`       | `BridgeRecoveryRecord` per batch_id           |
//! | `checkpoint:`       | `PoSeqCheckpoint` per checkpoint_id           |
//! | `replay:`           | Replay guard entries (`u64` seq per id)       |
//! | `receipt:`          | `SequencingReceipt` per batch_id              |
//! | `meta:schema_ver`   | Schema version (u32, big-endian)              |
//!
//! Existing namespaces from `PersistenceEngine` (`proposals:`, `finalized:`,
//! `attestations:`, `jail:`, `slashing:`, `rotation:`, `fairness:`) are
//! also used through the engine's typed helpers.

use serde::{Serialize, de::DeserializeOwned};

use crate::persistence::engine::PersistenceEngine;
use crate::finality::BatchFinalityStatus;
use crate::bridge_recovery::BridgeRecoveryRecord;
use crate::checkpoints::PoSeqCheckpoint;
use crate::receipts::receipt::SequencingReceipt;

// ─── Schema versioning ───────────────────────────────────────────────────────

pub const CURRENT_SCHEMA_VERSION: u32 = 1;

const META_SCHEMA_VER_KEY: &[u8] = b"meta:schema_ver";

// ─── Key helpers for new namespaces ──────────────────────────────────────────

mod prefix {
    pub const FINALITY:   &[u8] = b"finality:";
    pub const BRIDGE_REC: &[u8] = b"bridge_rec:";
    pub const CHECKPOINT: &[u8] = b"checkpoint:";
    pub const REPLAY:     &[u8] = b"replay:";
    pub const RECEIPT:    &[u8] = b"receipt:";
    pub const CKPT_EPOCH: &[u8] = b"ckpt_epoch:";  // epoch -> checkpoint_id index
}

fn finality_key(batch_id: &[u8; 32]) -> Vec<u8> {
    let mut k = prefix::FINALITY.to_vec();
    k.extend_from_slice(batch_id);
    k
}

fn bridge_rec_key(batch_id: &[u8; 32]) -> Vec<u8> {
    let mut k = prefix::BRIDGE_REC.to_vec();
    k.extend_from_slice(batch_id);
    k
}

fn checkpoint_key(checkpoint_id: &[u8; 32]) -> Vec<u8> {
    let mut k = prefix::CHECKPOINT.to_vec();
    k.extend_from_slice(checkpoint_id);
    k
}

fn ckpt_epoch_key(epoch: u64) -> Vec<u8> {
    let mut k = prefix::CKPT_EPOCH.to_vec();
    k.extend_from_slice(&epoch.to_be_bytes());
    k
}

fn replay_key(submission_id: &[u8; 32]) -> Vec<u8> {
    let mut k = prefix::REPLAY.to_vec();
    k.extend_from_slice(submission_id);
    k
}

fn receipt_key(batch_id: &[u8; 32]) -> Vec<u8> {
    let mut k = prefix::RECEIPT.to_vec();
    k.extend_from_slice(batch_id);
    k
}

// ─── Codec ───────────────────────────────────────────────────────────────────

fn encode<T: Serialize>(value: &T) -> Vec<u8> {
    bincode::serialize(value).expect("bincode::serialize failed — value must be serializable")
}

fn decode<T: DeserializeOwned>(bytes: &[u8]) -> Result<T, bincode::Error> {
    bincode::deserialize(bytes)
}

// ─── DurableStore ─────────────────────────────────────────────────────────────

/// Serialization + flush layer over `PersistenceEngine`.
///
/// Obtain via [`DurableStore::new`] passing a `PersistenceEngine` already
/// wired to a `SledBackend` (or `InMemoryBackend` for tests).
pub struct DurableStore {
    pub engine: PersistenceEngine,
}

impl DurableStore {
    pub fn new(engine: PersistenceEngine) -> Self {
        DurableStore { engine }
    }

    // ─── Schema version ──────────────────────────────────────────────────────

    /// Write the current schema version.  Call once on first open.
    pub fn write_schema_version(&mut self) {
        self.engine.put_raw(META_SCHEMA_VER_KEY, CURRENT_SCHEMA_VERSION.to_be_bytes().to_vec());
    }

    /// Read the stored schema version, or `None` if the DB is brand new.
    pub fn read_schema_version(&self) -> Option<u32> {
        let bytes = self.engine.get_raw(META_SCHEMA_VER_KEY)?;
        if bytes.len() != 4 { return None; }
        Some(u32::from_be_bytes(bytes.try_into().unwrap()))
    }

    /// Validate schema version; return `Err` if migration is needed or DB is corrupt.
    pub fn validate_schema(&self) -> Result<(), StorageError> {
        match self.read_schema_version() {
            None => Ok(()), // brand new — writer will stamp it
            Some(v) if v == CURRENT_SCHEMA_VERSION => Ok(()),
            Some(v) => Err(StorageError::SchemaMismatch { stored: v, expected: CURRENT_SCHEMA_VERSION }),
        }
    }

    // ─── Finality status ─────────────────────────────────────────────────────

    pub fn put_finality_status(&mut self, status: &BatchFinalityStatus) {
        let key = finality_key(&status.batch_id);
        self.engine.put_raw(&key, encode(status));
    }

    pub fn get_finality_status(&self, batch_id: &[u8; 32]) -> Option<BatchFinalityStatus> {
        let key = finality_key(batch_id);
        let bytes = self.engine.get_raw(&key)?;
        decode(&bytes).ok()
    }

    pub fn scan_finality_statuses(&self) -> Vec<BatchFinalityStatus> {
        self.engine
            .prefix_scan_raw(prefix::FINALITY)
            .into_iter()
            .filter_map(|(_, v)| decode(&v).ok())
            .collect()
    }

    // ─── Bridge recovery records ──────────────────────────────────────────────

    pub fn put_bridge_record(&mut self, record: &BridgeRecoveryRecord) {
        let key = bridge_rec_key(&record.batch_id);
        self.engine.put_raw(&key, encode(record));
    }

    pub fn get_bridge_record(&self, batch_id: &[u8; 32]) -> Option<BridgeRecoveryRecord> {
        let key = bridge_rec_key(batch_id);
        let bytes = self.engine.get_raw(&key)?;
        decode(&bytes).ok()
    }

    pub fn scan_bridge_records(&self) -> Vec<BridgeRecoveryRecord> {
        self.engine
            .prefix_scan_raw(prefix::BRIDGE_REC)
            .into_iter()
            .filter_map(|(_, v)| decode(&v).ok())
            .collect()
    }

    // ─── Checkpoints ─────────────────────────────────────────────────────────

    pub fn put_checkpoint(&mut self, checkpoint: &PoSeqCheckpoint) {
        let key = checkpoint_key(&checkpoint.checkpoint_id);
        self.engine.put_raw(&key, encode(checkpoint));
        // Secondary index: epoch → checkpoint_id (for latest-by-epoch lookup)
        let eidx = ckpt_epoch_key(checkpoint.metadata.epoch);
        self.engine.put_raw(&eidx, checkpoint.checkpoint_id.to_vec());
    }

    pub fn get_checkpoint(&self, checkpoint_id: &[u8; 32]) -> Option<PoSeqCheckpoint> {
        let key = checkpoint_key(checkpoint_id);
        let bytes = self.engine.get_raw(&key)?;
        decode(&bytes).ok()
    }

    pub fn get_checkpoint_id_for_epoch(&self, epoch: u64) -> Option<[u8; 32]> {
        let eidx = ckpt_epoch_key(epoch);
        let bytes = self.engine.get_raw(&eidx)?;
        if bytes.len() != 32 { return None; }
        let mut id = [0u8; 32];
        id.copy_from_slice(&bytes);
        Some(id)
    }

    pub fn get_checkpoint_for_epoch(&self, epoch: u64) -> Option<PoSeqCheckpoint> {
        let id = self.get_checkpoint_id_for_epoch(epoch)?;
        self.get_checkpoint(&id)
    }

    pub fn scan_checkpoints(&self) -> Vec<PoSeqCheckpoint> {
        self.engine
            .prefix_scan_raw(prefix::CHECKPOINT)
            .into_iter()
            .filter_map(|(_, v)| decode(&v).ok())
            .collect()
    }

    /// Return the checkpoint with the highest epoch, if any.
    pub fn latest_checkpoint(&self) -> Option<PoSeqCheckpoint> {
        // CKPT_EPOCH keys are sorted by epoch (big-endian) — scan all, take last.
        self.engine
            .prefix_scan_raw(prefix::CKPT_EPOCH)
            .into_iter()
            .last()
            .and_then(|(_, id_bytes)| {
                if id_bytes.len() != 32 { return None; }
                let mut id = [0u8; 32];
                id.copy_from_slice(&id_bytes);
                self.get_checkpoint(&id)
            })
    }

    // ─── Replay protection ────────────────────────────────────────────────────

    /// Record a submission ID as seen.  The value is a big-endian u64 sequence
    /// number so crash-recovery can reconstruct insertion order.
    pub fn put_replay_entry(&mut self, submission_id: &[u8; 32], seq: u64) {
        let key = replay_key(submission_id);
        self.engine.put_raw(&key, seq.to_be_bytes().to_vec());
    }

    pub fn get_replay_seq(&self, submission_id: &[u8; 32]) -> Option<u64> {
        let key = replay_key(submission_id);
        let bytes = self.engine.get_raw(&key)?;
        if bytes.len() != 8 { return None; }
        Some(u64::from_be_bytes(bytes.try_into().unwrap()))
    }

    pub fn delete_replay_entry(&mut self, submission_id: &[u8; 32]) {
        let key = replay_key(submission_id);
        self.engine.delete_raw(&key);
    }

    /// Scan all replay entries, sorted by seq (insertion order).
    pub fn scan_replay_entries(&self) -> Vec<([u8; 32], u64)> {
        let mut entries: Vec<([u8; 32], u64)> = self.engine
            .prefix_scan_raw(prefix::REPLAY)
            .into_iter()
            .filter_map(|(k, v)| {
                if k.len() != prefix::REPLAY.len() + 32 { return None; }
                let mut id = [0u8; 32];
                id.copy_from_slice(&k[prefix::REPLAY.len()..]);
                if v.len() != 8 { return None; }
                let seq = u64::from_be_bytes(v.try_into().unwrap());
                Some((id, seq))
            })
            .collect();
        // Sort by insertion seq so callers can reconstruct FIFO order.
        entries.sort_by_key(|(_, seq)| *seq);
        entries
    }

    // ─── Sequencing receipts ─────────────────────────────────────────────────

    pub fn put_receipt(&mut self, batch_id: &[u8; 32], receipt: &SequencingReceipt) {
        let key = receipt_key(batch_id);
        self.engine.put_raw(&key, encode(receipt));
    }

    pub fn get_receipt(&self, batch_id: &[u8; 32]) -> Option<SequencingReceipt> {
        let key = receipt_key(batch_id);
        let bytes = self.engine.get_raw(&key)?;
        decode(&bytes).ok()
    }

    pub fn scan_receipts(&self) -> Vec<SequencingReceipt> {
        self.engine
            .prefix_scan_raw(prefix::RECEIPT)
            .into_iter()
            .filter_map(|(_, v)| decode(&v).ok())
            .collect()
    }
}

// ─── StorageError ─────────────────────────────────────────────────────────────

#[derive(Debug)]
pub enum StorageError {
    /// Stored schema version does not match binary's expected version.
    SchemaMismatch { stored: u32, expected: u32 },
    /// Deserialization failed for a stored value.
    DecodeFailed { key: String, detail: String },
    /// A required record was not found.
    NotFound(String),
}

impl std::fmt::Display for StorageError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            StorageError::SchemaMismatch { stored, expected } =>
                write!(f, "schema mismatch: stored={stored} expected={expected}"),
            StorageError::DecodeFailed { key, detail } =>
                write!(f, "decode failed for key={key}: {detail}"),
            StorageError::NotFound(key) =>
                write!(f, "record not found: {key}"),
        }
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::persistence::backend::InMemoryBackend;
    use crate::persistence::engine::PersistenceEngine;
    use crate::finality::{FinalityStore, FinalityState};
    use crate::bridge_recovery::{BridgeRecoveryStore, BridgeRetryPolicy};
    use crate::checkpoints::{CheckpointStore, CheckpointPolicy};

    fn make_store() -> DurableStore {
        DurableStore::new(PersistenceEngine::new(Box::new(InMemoryBackend::new())))
    }

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    // ── schema version ──

    #[test]
    fn test_schema_version_roundtrip() {
        let mut ds = make_store();
        ds.write_schema_version();
        assert_eq!(ds.read_schema_version(), Some(CURRENT_SCHEMA_VERSION));
    }

    #[test]
    fn test_validate_schema_empty_db_passes() {
        let ds = make_store();
        assert!(ds.validate_schema().is_ok());
    }

    #[test]
    fn test_validate_schema_wrong_version_fails() {
        let mut ds = make_store();
        // Manually write a different version
        ds.engine.put_raw(META_SCHEMA_VER_KEY, 99u32.to_be_bytes().to_vec());
        assert!(matches!(ds.validate_schema(), Err(StorageError::SchemaMismatch { stored: 99, .. })));
    }

    // ── finality status ──

    #[test]
    fn test_finality_status_roundtrip() {
        let mut ds = make_store();
        let mut fs = FinalityStore::new();
        let bid = make_id(1);
        fs.init_batch(bid, 1, 10);
        fs.transition(&bid, FinalityState::Attested, "test").unwrap();
        let status = fs.get(&bid).unwrap().clone();
        ds.put_finality_status(&status);
        let loaded = ds.get_finality_status(&bid).unwrap();
        assert_eq!(loaded.current_state, FinalityState::Attested);
    }

    #[test]
    fn test_scan_finality_statuses() {
        let mut ds = make_store();
        let mut fs = FinalityStore::new();
        for i in 1u8..=3 {
            let bid = make_id(i);
            fs.init_batch(bid, 1, i as u64);
            let status = fs.get(&bid).unwrap().clone();
            ds.put_finality_status(&status);
        }
        let statuses = ds.scan_finality_statuses();
        assert_eq!(statuses.len(), 3);
    }

    // ── bridge records ──

    #[test]
    fn test_bridge_record_roundtrip() {
        let mut ds = make_store();
        let bid = make_id(10);
        let mut brs = BridgeRecoveryStore::new(BridgeRetryPolicy::default());
        brs.register_batch(bid);
        brs.export_batch(&bid).unwrap();
        let record = brs.get_record(&bid).unwrap().clone();
        ds.put_bridge_record(&record);
        let loaded = ds.get_bridge_record(&bid).unwrap();
        assert_eq!(loaded.batch_id, bid);
    }

    #[test]
    fn test_scan_bridge_records() {
        let mut ds = make_store();
        let mut brs = BridgeRecoveryStore::new(BridgeRetryPolicy::default());
        for i in 1u8..=3 {
            let bid = make_id(i);
            brs.register_batch(bid);
            let record = brs.get_record(&bid).unwrap().clone();
            ds.put_bridge_record(&record);
        }
        assert_eq!(ds.scan_bridge_records().len(), 3);
    }

    // ── checkpoints ──

    #[test]
    fn test_checkpoint_roundtrip() {
        let mut ds = make_store();
        let cp = crate::checkpoints::make_test_checkpoint(5, 1);
        ds.put_checkpoint(&cp);
        let loaded = ds.get_checkpoint(&cp.checkpoint_id).unwrap();
        assert_eq!(loaded.metadata.epoch, 5);
    }

    #[test]
    fn test_checkpoint_epoch_index() {
        let mut ds = make_store();
        let cp = crate::checkpoints::make_test_checkpoint(7, 1);
        ds.put_checkpoint(&cp);
        let id = ds.get_checkpoint_id_for_epoch(7).unwrap();
        assert_eq!(id, cp.checkpoint_id);
    }

    #[test]
    fn test_latest_checkpoint_returns_highest_epoch() {
        let mut ds = make_store();
        for epoch in [3u64, 7, 5] {
            let cp = crate::checkpoints::make_test_checkpoint(epoch, 1);
            ds.put_checkpoint(&cp);
        }
        let latest = ds.latest_checkpoint().unwrap();
        assert_eq!(latest.metadata.epoch, 7);
    }

    // ── replay entries ──

    #[test]
    fn test_replay_entry_roundtrip() {
        let mut ds = make_store();
        let sid = make_id(20);
        ds.put_replay_entry(&sid, 42);
        assert_eq!(ds.get_replay_seq(&sid), Some(42));
    }

    #[test]
    fn test_replay_entry_delete() {
        let mut ds = make_store();
        let sid = make_id(21);
        ds.put_replay_entry(&sid, 1);
        ds.delete_replay_entry(&sid);
        assert!(ds.get_replay_seq(&sid).is_none());
    }

    #[test]
    fn test_replay_entries_sorted_by_seq() {
        let mut ds = make_store();
        ds.put_replay_entry(&make_id(3), 30);
        ds.put_replay_entry(&make_id(1), 10);
        ds.put_replay_entry(&make_id(2), 20);
        let entries = ds.scan_replay_entries();
        assert_eq!(entries.len(), 3);
        // Must be sorted by seq
        assert_eq!(entries[0].1, 10);
        assert_eq!(entries[1].1, 20);
        assert_eq!(entries[2].1, 30);
    }

    // ── receipts ──

    #[test]
    fn test_receipt_roundtrip() {
        let mut ds = make_store();
        let bid = make_id(30);
        let receipt = SequencingReceipt::build(bid, 100, &[], &[], 1);
        ds.put_receipt(&bid, &receipt);
        let loaded = ds.get_receipt(&bid).unwrap();
        assert_eq!(loaded.batch_id, bid);
        assert_eq!(loaded.height, 100);
    }
}
