use std::collections::{BTreeMap, BTreeSet};
use crate::validation::validator::ValidatedSubmission;
use crate::errors::PoSeqError;

/// Replay/duplicate guard using seen submission IDs.
pub struct ReplayGuard {
    seen_ids: BTreeSet<[u8; 32]>,
    max_history: usize,  // 0 = unlimited
}

impl ReplayGuard {
    pub fn new(max_history: usize) -> Self {
        ReplayGuard { seen_ids: BTreeSet::new(), max_history }
    }
    pub fn check_and_record(&mut self, id: [u8; 32]) -> Result<(), PoSeqError> {
        if self.seen_ids.contains(&id) {
            return Err(PoSeqError::Duplicate(id));
        }
        self.seen_ids.insert(id);
        // If max_history > 0 and we exceeded it, evict the smallest (oldest) key
        if self.max_history > 0 && self.seen_ids.len() > self.max_history {
            if let Some(oldest) = self.seen_ids.iter().next().copied() {
                self.seen_ids.remove(&oldest);
            }
        }
        Ok(())
    }
    pub fn contains(&self, id: &[u8; 32]) -> bool {
        self.seen_ids.contains(id)
    }
}

/// In-memory pending submission queue ordered by (received_at_sequence → normalized_id).
/// BTreeMap keyed by (received_at_sequence, normalized_id) for deterministic FIFO within intake order.
pub struct SubmissionQueue {
    entries: BTreeMap<(u64, [u8; 32]), ValidatedSubmission>,
    max_size: usize,
}

impl SubmissionQueue {
    pub fn new(max_size: usize) -> Self {
        SubmissionQueue { entries: BTreeMap::new(), max_size }
    }
    pub fn push(&mut self, sub: ValidatedSubmission) -> Result<(), PoSeqError> {
        if self.max_size > 0 && self.entries.len() >= self.max_size {
            return Err(PoSeqError::QueueFull { capacity: self.max_size });
        }
        let key = (sub.envelope.received_at_sequence, sub.envelope.normalized_id);
        self.entries.insert(key, sub);
        Ok(())
    }
    /// Drain up to `count` entries in deterministic order.
    pub fn drain(&mut self, count: usize) -> Vec<ValidatedSubmission> {
        let keys: Vec<_> = self.entries.keys().take(count).copied().collect();
        keys.into_iter().filter_map(|k| self.entries.remove(&k)).collect()
    }
    pub fn len(&self) -> usize { self.entries.len() }
    pub fn is_empty(&self) -> bool { self.entries.is_empty() }
}
