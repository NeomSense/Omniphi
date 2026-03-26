//! Intent transaction mempool.
//!
//! Maps submission_id -> IntentTransaction for lookup during batch execution.
//! Submissions are added via RPC/gossip before sequencing; the ingestion
//! layer looks up real payloads by submission_id after PoSeq orders them.

use crate::intents::base::IntentTransaction;
use std::collections::BTreeMap;

/// Mempool holding pending intent transactions indexed by their submission ID.
pub struct IntentMempool {
    /// submission_id -> IntentTransaction
    pending: BTreeMap<[u8; 32], IntentTransaction>,
    /// Maximum number of pending transactions (prevents memory exhaustion)
    max_size: usize,
}

impl IntentMempool {
    pub fn new(max_size: usize) -> Self {
        IntentMempool {
            pending: BTreeMap::new(),
            max_size,
        }
    }

    /// Add a transaction to the mempool. Returns false if full or duplicate.
    pub fn insert(&mut self, tx: IntentTransaction) -> bool {
        if self.pending.len() >= self.max_size {
            return false;
        }
        if self.pending.contains_key(&tx.tx_id) {
            return false; // duplicate
        }
        self.pending.insert(tx.tx_id, tx);
        true
    }

    /// Look up a transaction by submission ID. Returns None if not found.
    pub fn get(&self, submission_id: &[u8; 32]) -> Option<&IntentTransaction> {
        self.pending.get(submission_id)
    }

    /// Remove a transaction after it has been included in a finalized batch.
    pub fn remove(&mut self, submission_id: &[u8; 32]) -> Option<IntentTransaction> {
        self.pending.remove(submission_id)
    }

    /// Number of pending transactions.
    pub fn len(&self) -> usize {
        self.pending.len()
    }

    pub fn is_empty(&self) -> bool {
        self.pending.is_empty()
    }

    /// Remove all transactions (used on epoch boundary or reset).
    pub fn clear(&mut self) {
        self.pending.clear();
    }
}

impl Default for IntentMempool {
    fn default() -> Self {
        // Default max size: 10,000 pending transactions
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
    fn test_full_rejected() {
        let mut pool = IntentMempool::new(2);
        assert!(pool.insert(make_tx(1)));
        assert!(pool.insert(make_tx(2)));
        assert!(!pool.insert(make_tx(3)));
        assert_eq!(pool.len(), 2);
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
