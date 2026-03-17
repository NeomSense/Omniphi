use std::collections::{BTreeMap, BTreeSet, VecDeque};
use crate::validation::validator::ValidatedSubmission;
use crate::errors::PoSeqError;

/// Replay/duplicate guard using seen submission IDs.
///
/// Bounded mode evicts by **insertion order** (FIFO) so that the oldest
/// submissions are forgotten first, not those with the lexicographically
/// smallest ID (FIND-008).
pub struct ReplayGuard {
    /// O(log n) membership test.
    seen_ids: BTreeSet<[u8; 32]>,
    /// Preserves insertion order for FIFO eviction.
    insertion_order: VecDeque<[u8; 32]>,
    max_history: usize,  // 0 = unlimited
}

impl ReplayGuard {
    pub fn new(max_history: usize) -> Self {
        ReplayGuard {
            seen_ids: BTreeSet::new(),
            insertion_order: VecDeque::new(),
            max_history,
        }
    }

    pub fn check_and_record(&mut self, id: [u8; 32]) -> Result<(), PoSeqError> {
        if self.seen_ids.contains(&id) {
            return Err(PoSeqError::Duplicate(id));
        }
        self.seen_ids.insert(id);
        self.insertion_order.push_back(id);
        // Evict the oldest-inserted entry when capacity is exceeded.
        if self.max_history > 0 && self.insertion_order.len() > self.max_history {
            if let Some(oldest) = self.insertion_order.pop_front() {
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

#[cfg(test)]
mod tests {
    use super::*;

    fn id(b: u8) -> [u8; 32] {
        let mut v = [0u8; 32];
        v[0] = b;
        v
    }

    #[test]
    fn test_replay_guard_unlimited_rejects_duplicate() {
        let mut g = ReplayGuard::new(0);
        assert!(g.check_and_record(id(1)).is_ok());
        assert!(matches!(g.check_and_record(id(1)), Err(PoSeqError::Duplicate(_))));
    }

    #[test]
    fn test_replay_guard_bounded_evicts_oldest_not_smallest_hash() {
        // FIND-008: capacity=2; insert id(0xFF) first (large hash), then id(0x01),
        // then id(0x02). The evicted entry must be id(0xFF) (oldest), not id(0x01)
        // (smallest hash). After eviction, re-submitting id(0xFF) must succeed.
        let mut g = ReplayGuard::new(2);

        let first = [0xFFu8; 32];
        let second = id(1);
        let third = id(2);

        g.check_and_record(first).unwrap();   // inserted first; queue=[first]
        g.check_and_record(second).unwrap();  // inserted second; queue=[first, second]
        // Inserting third evicts `first` (oldest, FIFO), not `second` (smaller hash)
        g.check_and_record(third).unwrap();   // queue=[second, third]; first evicted

        // `first` must have been evicted — re-submit succeeds.
        // Note: re-inserting `first` with capacity=2 causes `second` to be evicted next.
        assert!(g.check_and_record(first).is_ok(), "`first` must have been evicted (FIFO)");
        // queue is now [third, first]; `second` was evicted when `first` was re-added
        assert!(matches!(g.check_and_record(third), Err(PoSeqError::Duplicate(_))), "third still present");
        assert!(g.check_and_record(second).is_ok(), "`second` evicted when first was re-added");
    }

    #[test]
    fn test_replay_guard_bounded_capacity_respected() {
        // With capacity=3, inserting 5 entries evicts the 2 oldest.
        // After 5 inserts: queue = [id(3), id(4), id(5)]; ids 1 and 2 evicted.
        let mut g = ReplayGuard::new(3);
        for i in 1u8..=5 {
            g.check_and_record(id(i)).unwrap();
        }
        // Verify FIFO eviction: oldest two (id(1), id(2)) were evicted
        assert!(g.check_and_record(id(1)).is_ok(), "id(1) must be evicted");
        assert!(g.check_and_record(id(2)).is_ok(), "id(2) must be evicted");
        // id(3) is still present (was not evicted after initial 5 inserts)
        // Note: check before re-inserting id(1)/id(2) to avoid cascading evictions
        // Reset and re-test to isolate the invariant
        let mut g2 = ReplayGuard::new(3);
        for i in 1u8..=5 {
            g2.check_and_record(id(i)).unwrap();
        }
        // id(3), id(4), id(5) must be in the guard after 5 inserts into capacity-3
        assert!(matches!(g2.check_and_record(id(3)), Err(PoSeqError::Duplicate(_))), "id(3) not evicted");
        assert!(matches!(g2.check_and_record(id(4)), Err(PoSeqError::Duplicate(_))), "id(4) not evicted");
        assert!(matches!(g2.check_and_record(id(5)), Err(PoSeqError::Duplicate(_))), "id(5) not evicted");
    }
}
