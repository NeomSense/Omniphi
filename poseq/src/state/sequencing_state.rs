use std::collections::BTreeMap;
use crate::receipts::receipt::BatchAuditRecord;

/// Tracks the current sequencing state in memory.
pub struct SequencingState {
    pub current_height: u64,
    pub last_batch_id: Option<[u8; 32]>,
    pub policy_version: u32,
}

impl SequencingState {
    pub fn new(policy_version: u32) -> Self {
        SequencingState {
            current_height: 0,
            last_batch_id: None,
            policy_version,
        }
    }
    pub fn advance(&mut self, batch_id: [u8; 32]) {
        self.current_height += 1;
        self.last_batch_id = Some(batch_id);
    }
}

/// Append-only ledger of processed batches.
pub struct BatchLedger {
    pub records: BTreeMap<u64, BatchAuditRecord>,   // height → record
}

impl BatchLedger {
    pub fn new() -> Self { BatchLedger { records: BTreeMap::new() } }
    pub fn append(&mut self, record: BatchAuditRecord) { self.records.insert(record.height, record); }
    pub fn get(&self, height: u64) -> Option<&BatchAuditRecord> { self.records.get(&height) }
    pub fn latest(&self) -> Option<&BatchAuditRecord> { self.records.values().next_back() }
    pub fn len(&self) -> usize { self.records.len() }
}

impl Default for BatchLedger {
    fn default() -> Self { Self::new() }
}
