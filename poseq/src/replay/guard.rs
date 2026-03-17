use std::collections::{BTreeMap, BTreeSet};

/// Generic FIFO-eviction replay guard.
///
/// `K` must be `Ord` so we can use `BTreeSet` and `BTreeMap`.
/// When `capacity > 0`, oldest entries are evicted once the set exceeds capacity.
/// When `capacity == 0`, the guard retains all entries (unlimited).
pub struct ReplayGuard<K: Ord + Clone> {
    seen: BTreeSet<K>,
    insertion_order: BTreeMap<u64, K>, // seq → key for FIFO eviction
    next_seq: u64,
    capacity: usize,
}

impl<K: Ord + Clone> ReplayGuard<K> {
    pub fn new(capacity: usize) -> Self {
        ReplayGuard {
            seen: BTreeSet::new(),
            insertion_order: BTreeMap::new(),
            next_seq: 0,
            capacity,
        }
    }

    /// Returns `true` if this is the first time we've seen `key` (allowed),
    /// `false` if it's a replay (denied).
    pub fn check_and_record(&mut self, key: K) -> bool {
        if self.seen.contains(&key) {
            return false;
        }

        // Evict oldest if at capacity
        if self.capacity > 0 && self.seen.len() >= self.capacity {
            if let Some((&oldest_seq, _)) = self.insertion_order.iter().next() {
                let oldest_key = self.insertion_order.remove(&oldest_seq).unwrap();
                self.seen.remove(&oldest_key);
            }
        }

        let seq = self.next_seq;
        self.next_seq += 1;
        self.insertion_order.insert(seq, key.clone());
        self.seen.insert(key);
        true
    }

    pub fn contains(&self, key: &K) -> bool {
        self.seen.contains(key)
    }

    pub fn len(&self) -> usize {
        self.seen.len()
    }

    pub fn is_empty(&self) -> bool {
        self.seen.is_empty()
    }
}

/// Replay guard specialized for proposal IDs ([u8; 32]).
pub struct ProposalReplayGuard {
    inner: ReplayGuard<[u8; 32]>,
}

impl ProposalReplayGuard {
    pub fn new(capacity: usize) -> Self {
        ProposalReplayGuard {
            inner: ReplayGuard::new(capacity),
        }
    }

    pub fn check_and_record(&mut self, proposal_id: [u8; 32]) -> bool {
        self.inner.check_and_record(proposal_id)
    }

    pub fn contains(&self, proposal_id: &[u8; 32]) -> bool {
        self.inner.contains(proposal_id)
    }
}

/// Replay guard specialized for ack delivery IDs ([u8; 32]).
pub struct AckReplayGuard {
    inner: ReplayGuard<[u8; 32]>,
}

impl AckReplayGuard {
    pub fn new(capacity: usize) -> Self {
        AckReplayGuard {
            inner: ReplayGuard::new(capacity),
        }
    }

    pub fn check_and_record(&mut self, delivery_id: [u8; 32]) -> bool {
        self.inner.check_and_record(delivery_id)
    }

    pub fn contains(&self, delivery_id: &[u8; 32]) -> bool {
        self.inner.contains(delivery_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_first_submission_allowed() {
        let mut guard: ReplayGuard<[u8; 32]> = ReplayGuard::new(0);
        assert!(guard.check_and_record(make_id(1)));
    }

    #[test]
    fn test_second_submission_rejected() {
        let mut guard: ReplayGuard<[u8; 32]> = ReplayGuard::new(0);
        assert!(guard.check_and_record(make_id(1)));
        assert!(!guard.check_and_record(make_id(1)));
    }

    #[test]
    fn test_different_keys_both_allowed() {
        let mut guard: ReplayGuard<[u8; 32]> = ReplayGuard::new(0);
        assert!(guard.check_and_record(make_id(1)));
        assert!(guard.check_and_record(make_id(2)));
        assert_eq!(guard.len(), 2);
    }

    #[test]
    fn test_fifo_eviction_at_capacity() {
        let mut guard: ReplayGuard<[u8; 32]> = ReplayGuard::new(3);
        // Fill to capacity
        assert!(guard.check_and_record(make_id(1)));
        assert!(guard.check_and_record(make_id(2)));
        assert!(guard.check_and_record(make_id(3)));
        assert_eq!(guard.len(), 3);

        // Adding make_id(4) should evict make_id(1) (oldest)
        assert!(guard.check_and_record(make_id(4)));
        assert_eq!(guard.len(), 3);
        // make_id(1) was evicted, so it can be added again
        assert!(guard.check_and_record(make_id(1)));
        // make_id(2) was evicted by the make_id(1) insertion, so it can come back
        // (make_id(2) was evicted when make_id(1) re-entered)
    }

    #[test]
    fn test_capacity_zero_means_unlimited() {
        let mut guard: ReplayGuard<u64> = ReplayGuard::new(0);
        for i in 0..1000u64 {
            assert!(guard.check_and_record(i));
        }
        assert_eq!(guard.len(), 1000);
        // All still seen
        for i in 0..1000u64 {
            assert!(!guard.check_and_record(i));
        }
    }

    #[test]
    fn test_proposal_replay_guard() {
        let mut guard = ProposalReplayGuard::new(10);
        let pid = make_id(5);
        assert!(guard.check_and_record(pid));
        assert!(!guard.check_and_record(pid));
    }

    #[test]
    fn test_ack_replay_guard() {
        let mut guard = AckReplayGuard::new(10);
        let did = make_id(7);
        assert!(guard.check_and_record(did));
        assert!(!guard.check_and_record(did));
    }

    #[test]
    fn test_contains() {
        let mut guard: ReplayGuard<[u8; 32]> = ReplayGuard::new(0);
        assert!(!guard.contains(&make_id(1)));
        guard.check_and_record(make_id(1));
        assert!(guard.contains(&make_id(1)));
    }
}
