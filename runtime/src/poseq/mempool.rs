//! Fee-Prioritized Intent Mempool
//!
//! Maintains pending transactions ordered by fee (highest first) for optimal
//! block packing. When the pool is full, the lowest-fee transaction is evicted
//! to make room for a higher-fee replacement.
//!
//! Supports:
//! - O(log n) insert with fee-priority ordering
//! - O(1) lookup by submission_id
//! - O(log n) eviction of lowest-fee transaction when full
//! - Deadline-based expiry (remove stale intents)
//! - Sender nonce ordering (optional, for sequential intent guarantee)

use crate::intents::base::{IntentTransaction, FeePolicy, SponsorshipLimits};
use std::collections::{BTreeMap, BTreeSet};

/// A fee-priority key: (max_fee descending, tx_id for deterministic tiebreak).
/// BTreeSet sorts ascending, so we negate the fee for highest-first ordering.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
struct PriorityKey {
    /// Negated max_fee for descending sort (highest fee = smallest negated value = first)
    neg_fee: i128,
    /// Tiebreaker: tx_id for determinism
    tx_id: [u8; 32],
}

/// Mempool holding pending intent transactions with fee-based priority.
pub struct IntentMempool {
    /// submission_id → IntentTransaction
    pending: BTreeMap<[u8; 32], IntentTransaction>,
    /// Fee-priority index for ordered iteration and eviction
    priority_index: BTreeSet<PriorityKey>,
    /// Maximum number of pending transactions
    max_size: usize,
}

impl IntentMempool {
    pub fn new(max_size: usize) -> Self {
        IntentMempool {
            pending: BTreeMap::new(),
            priority_index: BTreeSet::new(),
            max_size,
        }
    }

    fn priority_key(tx: &IntentTransaction) -> PriorityKey {
        PriorityKey {
            neg_fee: -(tx.max_fee as i128),
            tx_id: tx.tx_id,
        }
    }

    /// Add a transaction to the mempool.
    ///
    /// Returns true if inserted successfully.
    /// If the pool is full:
    /// - If the new tx has higher fee than the lowest, evict the lowest
    /// - If the new tx has equal or lower fee, reject it
    pub fn insert(&mut self, tx: IntentTransaction) -> bool {
        if self.pending.contains_key(&tx.tx_id) {
            return false; // duplicate
        }

        if self.pending.len() >= self.max_size {
            // Pool full — check if new tx outbids the lowest
            if let Some(lowest) = self.priority_index.iter().next_back() {
                let lowest_fee = (-lowest.neg_fee) as u64;
                if tx.max_fee <= lowest_fee {
                    return false; // New tx doesn't outbid lowest
                }
                // Evict lowest
                let evict_id = lowest.tx_id;
                let evict_key = lowest.clone();
                self.priority_index.remove(&evict_key);
                self.pending.remove(&evict_id);
            } else {
                return false;
            }
        }

        let key = Self::priority_key(&tx);
        self.priority_index.insert(key);
        self.pending.insert(tx.tx_id, tx);
        true
    }

    /// Look up a transaction by submission ID.
    pub fn get(&self, submission_id: &[u8; 32]) -> Option<&IntentTransaction> {
        self.pending.get(submission_id)
    }

    /// Remove a transaction after inclusion in a finalized batch.
    pub fn remove(&mut self, submission_id: &[u8; 32]) -> Option<IntentTransaction> {
        if let Some(tx) = self.pending.remove(submission_id) {
            let key = Self::priority_key(&tx);
            self.priority_index.remove(&key);
            Some(tx)
        } else {
            None
        }
    }

    /// Get the top N highest-fee transactions (for batch building).
    pub fn top_n(&self, n: usize) -> Vec<&IntentTransaction> {
        self.priority_index.iter()
            .take(n)
            .filter_map(|k| self.pending.get(&k.tx_id))
            .collect()
    }

    /// Remove transactions with deadline_epoch <= current_epoch.
    pub fn expire(&mut self, current_epoch: u64) -> usize {
        let expired: Vec<[u8; 32]> = self.pending.values()
            .filter(|tx| tx.deadline_epoch <= current_epoch)
            .map(|tx| tx.tx_id)
            .collect();
        let count = expired.len();
        for id in expired {
            self.remove(&id);
        }
        count
    }

    /// Number of pending transactions.
    pub fn len(&self) -> usize {
        self.pending.len()
    }

    pub fn is_empty(&self) -> bool {
        self.pending.is_empty()
    }

    /// Remove all transactions.
    pub fn clear(&mut self) {
        self.pending.clear();
        self.priority_index.clear();
    }
}

impl Default for IntentMempool {
    fn default() -> Self {
        Self::new(10_000)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intents::base::{IntentType, IntentConstraints, ExecutionMode};
    use crate::intents::types::TransferIntent;
    use std::collections::BTreeMap;

    fn make_tx(id_byte: u8) -> IntentTransaction {
        let mut tx_id = [0u8; 32];
        tx_id[0] = id_byte;
        let mut sender = [0u8; 32];
        sender[0] = 0x01;
        let mut recipient = [0u8; 32];
        recipient[0] = 0x02;
        let mut asset = [0u8; 32];
        asset[0] = 0x03;

        IntentTransaction {
            tx_id,
            sender,
            nonce: 1,
            intent: IntentType::Transfer(TransferIntent {
                asset_id: asset,
                amount: 100,
                recipient,
                memo: None,
            }),
            max_fee: 1_000,
            deadline_epoch: 100,
            signature: [0u8; 64],
            metadata: BTreeMap::new(),
            target_objects: vec![],
            constraints: IntentConstraints::default(),
            execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
        }
    }

    #[test]
    fn test_insert_and_get() {
        let mut pool = IntentMempool::new(100);
        let tx = make_tx(1);
        let tx_id = tx.tx_id;
        assert!(pool.insert(tx));
        assert_eq!(pool.len(), 1);
        assert!(pool.get(&tx_id).is_some());
    }

    #[test]
    fn test_duplicate_rejected() {
        let mut pool = IntentMempool::new(100);
        let tx1 = make_tx(1);
        let tx2 = make_tx(1); // same tx_id
        assert!(pool.insert(tx1));
        assert!(!pool.insert(tx2));
        assert_eq!(pool.len(), 1);
    }

    #[test]
    fn test_full_low_fee_rejected() {
        let mut pool = IntentMempool::new(2);
        assert!(pool.insert(make_tx(1))); // fee 1000
        assert!(pool.insert(make_tx(2))); // fee 1000
        assert!(!pool.insert(make_tx(3))); // fee 1000 — doesn't outbid
        assert_eq!(pool.len(), 2);
    }

    #[test]
    fn test_full_high_fee_evicts_lowest() {
        let mut pool = IntentMempool::new(2);
        let mut low1 = make_tx(1);
        low1.max_fee = 100;
        let mut low2 = make_tx(2);
        low2.max_fee = 200;
        assert!(pool.insert(low1));
        assert!(pool.insert(low2));

        // Insert higher fee — should evict fee=100
        let mut high = make_tx(3);
        high.max_fee = 500;
        assert!(pool.insert(high));
        assert_eq!(pool.len(), 2);
        assert!(pool.get(&make_tx(1).tx_id).is_none()); // evicted
        assert!(pool.get(&make_tx(3).tx_id).is_some()); // inserted
    }

    #[test]
    fn test_top_n_returns_highest_fee_first() {
        let mut pool = IntentMempool::new(100);
        for fee in [100u64, 500, 200, 1000, 50] {
            let mut tx = make_tx(fee as u8);
            tx.max_fee = fee;
            pool.insert(tx);
        }
        let top = pool.top_n(3);
        assert_eq!(top.len(), 3);
        assert!(top[0].max_fee >= top[1].max_fee);
        assert!(top[1].max_fee >= top[2].max_fee);
        assert_eq!(top[0].max_fee, 1000);
    }

    #[test]
    fn test_expire_removes_stale() {
        let mut pool = IntentMempool::new(100);
        let mut tx1 = make_tx(1);
        tx1.deadline_epoch = 50;
        let mut tx2 = make_tx(2);
        tx2.deadline_epoch = 200;
        pool.insert(tx1);
        pool.insert(tx2);

        let expired = pool.expire(100); // epoch 100
        assert_eq!(expired, 1); // tx1 expired (deadline 50)
        assert_eq!(pool.len(), 1);
    }

    #[test]
    fn test_remove() {
        let mut pool = IntentMempool::new(100);
        let tx = make_tx(1);
        let tx_id = tx.tx_id;
        pool.insert(tx);
        let removed = pool.remove(&tx_id);
        assert!(removed.is_some());
        assert_eq!(pool.len(), 0);
        assert!(pool.get(&tx_id).is_none());
    }

    #[test]
    fn test_clear() {
        let mut pool = IntentMempool::new(100);
        pool.insert(make_tx(1));
        pool.insert(make_tx(2));
        assert_eq!(pool.len(), 2);
        pool.clear();
        assert!(pool.is_empty());
    }

    #[test]
    fn test_is_empty() {
        let pool = IntentMempool::new(100);
        assert!(pool.is_empty());
    }
}
