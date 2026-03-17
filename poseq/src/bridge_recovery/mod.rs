#![allow(dead_code)]

use std::collections::BTreeMap;
use std::fmt;

// ---------------------------------------------------------------------------
// BridgeDeliveryState
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum BridgeDeliveryState {
    Pending,
    Exporting,
    Exported,
    Acknowledged,
    Rejected,
    RetryPending { attempt: u32 },
    Failed,
    RecoveredAck,
}

impl fmt::Display for BridgeDeliveryState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

impl BridgeDeliveryState {
    pub fn can_retry(&self, max_attempts: u32) -> bool {
        match self {
            BridgeDeliveryState::Rejected => true,
            BridgeDeliveryState::RetryPending { attempt } => *attempt < max_attempts,
            _ => false,
        }
    }

    pub fn is_terminal(&self) -> bool {
        matches!(
            self,
            BridgeDeliveryState::Acknowledged
                | BridgeDeliveryState::Failed
                | BridgeDeliveryState::RecoveredAck
        )
    }
}

// ---------------------------------------------------------------------------
// BridgeRetryPolicy
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BridgeRetryPolicy {
    pub max_attempts: u32,
    pub backoff_base_seq: u64,
}

impl BridgeRetryPolicy {
    pub fn default_policy() -> Self {
        BridgeRetryPolicy {
            max_attempts: 3,
            backoff_base_seq: 10,
        }
    }
}

impl Default for BridgeRetryPolicy {
    fn default() -> Self {
        Self::default_policy()
    }
}

// ---------------------------------------------------------------------------
// BridgeRecoveryError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum BridgeRecoveryError {
    BatchNotFound([u8; 32]),
    AlreadyTerminal([u8; 32]),
    MaxRetriesExceeded([u8; 32]),
    InvalidStateForOperation {
        batch_id: [u8; 32],
        state: String,
        op: String,
    },
    /// FIND-011: a duplicate ack arrived with a different hash — evidence of
    /// message corruption or a deliberate tampering attempt.
    AckHashConflict {
        batch_id: [u8; 32],
        stored_hash: [u8; 32],
        new_hash: [u8; 32],
    },
}

// ---------------------------------------------------------------------------
// BridgeRecoveryRecord
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BridgeRecoveryRecord {
    pub batch_id: [u8; 32],
    pub state: BridgeDeliveryState,
    pub attempts: u32,
    pub last_attempt_seq: u64,
    pub ack_hash: Option<[u8; 32]>,
}

impl BridgeRecoveryRecord {
    pub fn new(batch_id: [u8; 32]) -> Self {
        BridgeRecoveryRecord {
            batch_id,
            state: BridgeDeliveryState::Pending,
            attempts: 0,
            last_attempt_seq: 0,
            ack_hash: None,
        }
    }

    pub fn try_export(&mut self) -> Result<(), BridgeRecoveryError> {
        if self.state.is_terminal() {
            return Err(BridgeRecoveryError::AlreadyTerminal(self.batch_id));
        }
        match &self.state {
            BridgeDeliveryState::Pending | BridgeDeliveryState::RetryPending { .. } => {
                self.state = BridgeDeliveryState::Exporting;
                self.attempts += 1;
                Ok(())
            }
            _ => Err(BridgeRecoveryError::InvalidStateForOperation {
                batch_id: self.batch_id,
                state: format!("{:?}", self.state),
                op: "try_export".into(),
            }),
        }
    }

    pub fn mark_acknowledged(&mut self, ack_hash: [u8; 32]) -> Result<(), BridgeRecoveryError> {
        if self.state.is_terminal() {
            return Err(BridgeRecoveryError::AlreadyTerminal(self.batch_id));
        }
        match &self.state {
            BridgeDeliveryState::Exporting | BridgeDeliveryState::Exported => {
                self.state = BridgeDeliveryState::Acknowledged;
                self.ack_hash = Some(ack_hash);
                Ok(())
            }
            _ => Err(BridgeRecoveryError::InvalidStateForOperation {
                batch_id: self.batch_id,
                state: format!("{:?}", self.state),
                op: "mark_acknowledged".into(),
            }),
        }
    }

    pub fn mark_rejected(&mut self) -> Result<(), BridgeRecoveryError> {
        if self.state.is_terminal() {
            return Err(BridgeRecoveryError::AlreadyTerminal(self.batch_id));
        }
        match &self.state {
            BridgeDeliveryState::Exporting | BridgeDeliveryState::Exported => {
                self.state = BridgeDeliveryState::Rejected;
                Ok(())
            }
            _ => Err(BridgeRecoveryError::InvalidStateForOperation {
                batch_id: self.batch_id,
                state: format!("{:?}", self.state),
                op: "mark_rejected".into(),
            }),
        }
    }

    pub fn retry(&mut self, policy: &BridgeRetryPolicy, seq: u64) -> Result<(), BridgeRecoveryError> {
        if self.attempts >= policy.max_attempts {
            self.state = BridgeDeliveryState::Failed;
            return Err(BridgeRecoveryError::MaxRetriesExceeded(self.batch_id));
        }
        match &self.state {
            BridgeDeliveryState::Rejected | BridgeDeliveryState::RetryPending { .. } => {
                // FIND-010: use the current attempts count (already incremented by try_export)
                // as the attempt field in RetryPending so it matches reality.
                let attempt = self.attempts;
                self.state = BridgeDeliveryState::RetryPending { attempt };
                self.last_attempt_seq = seq;
                Ok(())
            }
            _ => Err(BridgeRecoveryError::InvalidStateForOperation {
                batch_id: self.batch_id,
                state: format!("{:?}", self.state),
                op: "retry".into(),
            }),
        }
    }
}

// ---------------------------------------------------------------------------
// RuntimeAckReconciliationResult
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RuntimeAckReconciliationResult {
    pub batch_id: [u8; 32],
    pub was_duplicate_ack: bool,
    pub ack_hash_match: bool,
    pub action_taken: String,
}

// ---------------------------------------------------------------------------
// DeliveryConsistencyCheck
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct DeliveryConsistencyCheck {
    pub batches_pending: usize,
    pub batches_acked: usize,
    pub batches_failed: usize,
    pub batches_in_retry: usize,
    pub inconsistencies: Vec<String>,
}

// ---------------------------------------------------------------------------
// BridgeRecoveryStore
// ---------------------------------------------------------------------------

pub struct BridgeRecoveryStore {
    records: BTreeMap<[u8; 32], BridgeRecoveryRecord>,
    policy: BridgeRetryPolicy,
    seq: u64,
}

impl BridgeRecoveryStore {
    pub fn new(policy: BridgeRetryPolicy) -> Self {
        BridgeRecoveryStore {
            records: BTreeMap::new(),
            policy,
            seq: 0,
        }
    }

    pub fn register_batch(&mut self, batch_id: [u8; 32]) {
        self.records
            .entry(batch_id)
            .or_insert_with(|| BridgeRecoveryRecord::new(batch_id));
    }

    pub fn export_batch(&mut self, batch_id: &[u8; 32]) -> Result<(), BridgeRecoveryError> {
        let record = self
            .records
            .get_mut(batch_id)
            .ok_or(BridgeRecoveryError::BatchNotFound(*batch_id))?;
        record.try_export()
    }

    pub fn acknowledge(
        &mut self,
        batch_id: &[u8; 32],
        ack_hash: [u8; 32],
    ) -> Result<RuntimeAckReconciliationResult, BridgeRecoveryError> {
        let record = self
            .records
            .get_mut(batch_id)
            .ok_or(BridgeRecoveryError::BatchNotFound(*batch_id))?;

        // Detect duplicate ack
        if record.state.is_terminal() {
            let was_acked = matches!(
                record.state,
                BridgeDeliveryState::Acknowledged | BridgeDeliveryState::RecoveredAck
            );
            let stored = record.ack_hash;
            let hash_match = stored.map(|h| h == ack_hash).unwrap_or(false);

            // FIND-011: if the hashes differ, this is not a benign duplicate —
            // it is evidence of message corruption or a tampering attempt.
            if was_acked && !hash_match {
                if let Some(stored_hash) = stored {
                    return Err(BridgeRecoveryError::AckHashConflict {
                        batch_id: *batch_id,
                        stored_hash,
                        new_hash: ack_hash,
                    });
                }
            }

            return Ok(RuntimeAckReconciliationResult {
                batch_id: *batch_id,
                was_duplicate_ack: was_acked,
                ack_hash_match: hash_match,
                action_taken: "ignored_duplicate".into(),
            });
        }

        // Check if this is a recovery ack (was previously Recovered state)
        let is_recovery = matches!(record.state, BridgeDeliveryState::RetryPending { .. });
        record.mark_acknowledged(ack_hash)?;
        if is_recovery {
            record.state = BridgeDeliveryState::RecoveredAck;
        }
        Ok(RuntimeAckReconciliationResult {
            batch_id: *batch_id,
            was_duplicate_ack: false,
            ack_hash_match: true,
            action_taken: "acknowledged".into(),
        })
    }

    /// Read-only access to a record (used by DurableStore for serialization).
    pub fn get_record(&self, batch_id: &[u8; 32]) -> Option<&BridgeRecoveryRecord> {
        self.records.get(batch_id)
    }

    /// All records (for full-scan persistence on startup).
    pub fn all_records(&self) -> Vec<&BridgeRecoveryRecord> {
        self.records.values().collect()
    }

    pub fn reject_and_retry(&mut self, batch_id: &[u8; 32]) -> Result<(), BridgeRecoveryError> {
        let seq = self.seq;
        self.seq += 1;
        let record = self
            .records
            .get_mut(batch_id)
            .ok_or(BridgeRecoveryError::BatchNotFound(*batch_id))?;
        record.mark_rejected()?;
        record.retry(&self.policy.clone(), seq)
    }

    pub fn consistency_check(&self) -> DeliveryConsistencyCheck {
        let mut pending = 0;
        let mut acked = 0;
        let mut failed = 0;
        let mut in_retry = 0;
        let mut inconsistencies = Vec::new();

        for record in self.records.values() {
            match &record.state {
                BridgeDeliveryState::Pending | BridgeDeliveryState::Exporting | BridgeDeliveryState::Exported => {
                    pending += 1;
                }
                BridgeDeliveryState::Acknowledged | BridgeDeliveryState::RecoveredAck => {
                    acked += 1;
                    if record.ack_hash.is_none() {
                        inconsistencies.push(format!(
                            "batch {:?} acked but no ack_hash",
                            &record.batch_id[..4]
                        ));
                    }
                }
                BridgeDeliveryState::Failed => {
                    failed += 1;
                }
                BridgeDeliveryState::RetryPending { .. } => {
                    in_retry += 1;
                }
                BridgeDeliveryState::Rejected => {
                    inconsistencies.push(format!(
                        "batch {:?} stuck in Rejected",
                        &record.batch_id[..4]
                    ));
                }
            }
        }

        DeliveryConsistencyCheck {
            batches_pending: pending,
            batches_acked: acked,
            batches_failed: failed,
            batches_in_retry: in_retry,
            inconsistencies,
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn batch(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn ack_hash(n: u8) -> [u8; 32] {
        let mut h = [0xAAu8; 32];
        h[0] = n;
        h
    }

    fn default_store() -> BridgeRecoveryStore {
        BridgeRecoveryStore::new(BridgeRetryPolicy::default_policy())
    }

    #[test]
    fn test_register_and_export() {
        let mut store = default_store();
        store.register_batch(batch(1));
        store.export_batch(&batch(1)).unwrap();
        assert_eq!(store.records[&batch(1)].state, BridgeDeliveryState::Exporting);
    }

    #[test]
    fn test_export_not_found() {
        let mut store = default_store();
        assert!(matches!(
            store.export_batch(&batch(99)),
            Err(BridgeRecoveryError::BatchNotFound(_))
        ));
    }

    #[test]
    fn test_acknowledge_happy_path() {
        let mut store = default_store();
        store.register_batch(batch(2));
        store.export_batch(&batch(2)).unwrap();
        let result = store.acknowledge(&batch(2), ack_hash(2)).unwrap();
        assert!(!result.was_duplicate_ack);
        assert!(result.ack_hash_match);
    }

    #[test]
    fn test_duplicate_ack_detected() {
        let mut store = default_store();
        store.register_batch(batch(3));
        store.export_batch(&batch(3)).unwrap();
        store.acknowledge(&batch(3), ack_hash(3)).unwrap();
        let result = store.acknowledge(&batch(3), ack_hash(3)).unwrap();
        assert!(result.was_duplicate_ack);
    }

    #[test]
    fn test_reject_and_retry() {
        let mut store = default_store();
        store.register_batch(batch(4));
        store.export_batch(&batch(4)).unwrap();
        store.reject_and_retry(&batch(4)).unwrap();
        assert!(matches!(
            store.records[&batch(4)].state,
            BridgeDeliveryState::RetryPending { .. }
        ));
    }

    #[test]
    fn test_max_retries_exceeded() {
        let mut store = BridgeRecoveryStore::new(BridgeRetryPolicy {
            max_attempts: 1,
            backoff_base_seq: 5,
        });
        store.register_batch(batch(5));
        store.export_batch(&batch(5)).unwrap();
        // attempts is now 1, which equals max_attempts=1
        let result = store.reject_and_retry(&batch(5));
        // mark_rejected succeeds, but retry should fail
        assert!(matches!(result, Err(BridgeRecoveryError::MaxRetriesExceeded(_))));
        assert_eq!(store.records[&batch(5)].state, BridgeDeliveryState::Failed);
    }

    #[test]
    fn test_terminal_state_prevents_export() {
        let mut store = default_store();
        store.register_batch(batch(6));
        store.export_batch(&batch(6)).unwrap();
        store.acknowledge(&batch(6), ack_hash(6)).unwrap();
        assert!(matches!(
            store.export_batch(&batch(6)),
            Err(BridgeRecoveryError::AlreadyTerminal(_))
        ));
    }

    #[test]
    fn test_consistency_check_all_acked() {
        let mut store = default_store();
        store.register_batch(batch(10));
        store.register_batch(batch(11));
        store.export_batch(&batch(10)).unwrap();
        store.export_batch(&batch(11)).unwrap();
        store.acknowledge(&batch(10), ack_hash(10)).unwrap();
        store.acknowledge(&batch(11), ack_hash(11)).unwrap();
        let check = store.consistency_check();
        assert_eq!(check.batches_acked, 2);
        assert_eq!(check.batches_pending, 0);
        assert!(check.inconsistencies.is_empty());
    }

    #[test]
    fn test_consistency_check_with_pending() {
        let mut store = default_store();
        store.register_batch(batch(20));
        let check = store.consistency_check();
        assert_eq!(check.batches_pending, 1);
    }

    #[test]
    fn test_failed_state_counted() {
        let mut store = BridgeRecoveryStore::new(BridgeRetryPolicy {
            max_attempts: 1,
            backoff_base_seq: 1,
        });
        store.register_batch(batch(30));
        store.export_batch(&batch(30)).unwrap();
        let _ = store.reject_and_retry(&batch(30));
        let check = store.consistency_check();
        assert_eq!(check.batches_failed, 1);
    }

    #[test]
    fn test_bridge_delivery_state_can_retry() {
        assert!(BridgeDeliveryState::Rejected.can_retry(3));
        assert!(BridgeDeliveryState::RetryPending { attempt: 1 }.can_retry(3));
        assert!(!BridgeDeliveryState::RetryPending { attempt: 3 }.can_retry(3));
        assert!(!BridgeDeliveryState::Acknowledged.can_retry(3));
    }

    #[test]
    fn test_bridge_delivery_state_is_terminal() {
        assert!(BridgeDeliveryState::Acknowledged.is_terminal());
        assert!(BridgeDeliveryState::Failed.is_terminal());
        assert!(BridgeDeliveryState::RecoveredAck.is_terminal());
        assert!(!BridgeDeliveryState::Pending.is_terminal());
        assert!(!BridgeDeliveryState::Rejected.is_terminal());
    }

    #[test]
    fn test_mark_rejected_wrong_state() {
        let mut record = BridgeRecoveryRecord::new(batch(40));
        // Pending → can't be rejected directly
        let result = record.mark_rejected();
        assert!(matches!(
            result,
            Err(BridgeRecoveryError::InvalidStateForOperation { .. })
        ));
    }

    #[test]
    fn test_duplicate_ack_with_different_hash_returns_error() {
        // FIND-011: conflicting ack hash must return an error, not Ok
        let mut store = default_store();
        store.register_batch(batch(50));
        store.export_batch(&batch(50)).unwrap();
        store.acknowledge(&batch(50), ack_hash(50)).unwrap();

        // Second ack with a different hash
        let mut different_hash = ack_hash(50);
        different_hash[0] ^= 0xFF;
        let result = store.acknowledge(&batch(50), different_hash);
        assert!(
            matches!(result, Err(BridgeRecoveryError::AckHashConflict { .. })),
            "conflicting ack hash must return AckHashConflict error"
        );
    }

    #[test]
    fn test_duplicate_ack_with_same_hash_is_ok() {
        // Identical duplicate ack (e.g. network retry) is still accepted silently
        let mut store = default_store();
        store.register_batch(batch(51));
        store.export_batch(&batch(51)).unwrap();
        store.acknowledge(&batch(51), ack_hash(51)).unwrap();
        let result = store.acknowledge(&batch(51), ack_hash(51));
        assert!(result.is_ok());
        assert!(result.unwrap().was_duplicate_ack);
    }
}
