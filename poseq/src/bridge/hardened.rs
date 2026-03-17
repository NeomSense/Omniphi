use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::finalization::engine::FinalizedBatch;
use crate::errors::BridgeError;

/// Wraps a FinalizedBatch for delivery to the runtime.
#[derive(Debug, Clone)]
pub struct RuntimeDeliveryEnvelope {
    pub delivery_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub batch_root: [u8; 32],
    pub attempt_count: u32,
    pub delivery_hash: [u8; 32],
}

impl RuntimeDeliveryEnvelope {
    fn compute_delivery_id(batch_id: &[u8; 32], attempt_count: u32) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(batch_id);
        hasher.update(&attempt_count.to_be_bytes());
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    fn compute_delivery_hash(delivery_id: &[u8; 32], batch_id: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(delivery_id);
        hasher.update(batch_id);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    pub fn build(batch: &FinalizedBatch, attempt_count: u32) -> Self {
        let delivery_id = Self::compute_delivery_id(&batch.batch_id, attempt_count);
        let delivery_hash = Self::compute_delivery_hash(&delivery_id, &batch.batch_id);
        RuntimeDeliveryEnvelope {
            delivery_id,
            batch_id: batch.batch_id,
            slot: batch.slot,
            epoch: batch.epoch,
            ordered_submission_ids: batch.ordered_submission_ids.clone(),
            batch_root: batch.batch_root,
            attempt_count,
            delivery_hash,
        }
    }
}

/// Runtime acknowledgment of a delivered envelope.
#[derive(Debug, Clone)]
pub struct RuntimeExecutionAck {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub accepted: bool,
    pub epoch: u64,
    pub ack_hash: [u8; 32],
}

impl RuntimeExecutionAck {
    pub fn compute_ack_hash(
        batch_id: &[u8; 32],
        delivery_id: &[u8; 32],
        accepted: bool,
        epoch: u64,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(batch_id);
        hasher.update(delivery_id);
        hasher.update(&[accepted as u8]);
        hasher.update(&epoch.to_be_bytes());
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    pub fn new(batch_id: [u8; 32], delivery_id: [u8; 32], accepted: bool, epoch: u64) -> Self {
        let ack_hash = Self::compute_ack_hash(&batch_id, &delivery_id, accepted, epoch);
        RuntimeExecutionAck { batch_id, delivery_id, accepted, epoch, ack_hash }
    }
}

/// Runtime rejection of a delivered envelope.
#[derive(Debug, Clone)]
pub struct RuntimeExecutionRejection {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub reason: String,
    pub epoch: u64,
}

/// Delivery tracking record per batch.
#[derive(Debug, Clone)]
pub struct BridgeDeliveryRecord {
    pub delivery_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub delivered: bool,
    pub acked: bool,
    pub accepted: bool,
    pub attempt_count: u32,
}

/// Hardened bridge with idempotent delivery and ack replay protection.
pub struct HardenedRuntimeBridge {
    records: BTreeMap<[u8; 32], BridgeDeliveryRecord>,   // keyed by batch_id
    ack_replay_guard: BTreeMap<[u8; 32], bool>,            // keyed by delivery_id
    rejections: Vec<RuntimeExecutionRejection>,
}

impl HardenedRuntimeBridge {
    pub fn new() -> Self {
        HardenedRuntimeBridge {
            records: BTreeMap::new(),
            ack_replay_guard: BTreeMap::new(),
            rejections: Vec::new(),
        }
    }

    /// Idempotent delivery: always returns the original delivery envelope (same delivery_id
    /// and delivery_hash), but increments `attempt_count` in the tracking record on every call
    /// so that observers can detect flooding/abuse. (FIND-009)
    pub fn deliver(&mut self, batch: &FinalizedBatch) -> RuntimeDeliveryEnvelope {
        if let Some(record) = self.records.get_mut(&batch.batch_id) {
            // Increment attempt counter to reflect actual delivery attempts.
            record.attempt_count = record.attempt_count.saturating_add(1);
            // Return the original envelope (same delivery_id from attempt 1).
            return RuntimeDeliveryEnvelope::build(batch, 1u32);
        }

        let attempt_count = 1u32;
        let envelope = RuntimeDeliveryEnvelope::build(batch, attempt_count);

        let record = BridgeDeliveryRecord {
            delivery_id: envelope.delivery_id,
            batch_id: batch.batch_id,
            delivered: true,
            acked: false,
            accepted: false,
            attempt_count,
        };
        self.records.insert(batch.batch_id, record);
        envelope
    }

    /// Record an ack. Returns Err(BridgeError::AckReplay) if the delivery_id was already acked.
    pub fn record_ack(&mut self, ack: RuntimeExecutionAck) -> Result<(), BridgeError> {
        if self.ack_replay_guard.contains_key(&ack.delivery_id) {
            return Err(BridgeError::AckReplay);
        }
        self.ack_replay_guard.insert(ack.delivery_id, true);

        if let Some(record) = self.records.get_mut(&ack.batch_id) {
            record.acked = true;
            record.accepted = ack.accepted;
        }
        Ok(())
    }

    /// Record a rejection from the runtime.
    pub fn record_rejection(&mut self, rej: RuntimeExecutionRejection) {
        if let Some(record) = self.records.get_mut(&rej.batch_id) {
            record.acked = true;
            record.accepted = false;
        }
        self.rejections.push(rej);
    }

    pub fn is_delivered(&self, batch_id: &[u8; 32]) -> bool {
        self.records.get(batch_id).map(|r| r.delivered).unwrap_or(false)
    }

    pub fn is_acked(&self, batch_id: &[u8; 32]) -> bool {
        self.records.get(batch_id).map(|r| r.acked).unwrap_or(false)
    }

    pub fn get_record(&self, batch_id: &[u8; 32]) -> Option<&BridgeDeliveryRecord> {
        self.records.get(batch_id)
    }
}

impl Default for HardenedRuntimeBridge {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::finalization::engine::FinalizedBatch;
    use crate::attestations::collector::AttestationQuorumResult;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_finalized_batch(proposal_byte: u8, batch_byte: u8) -> FinalizedBatch {
        let qr = AttestationQuorumResult {
            reached: true,
            approvals: 3,
            rejections: 0,
            total_votes: 3,
            quorum_hash: [5u8; 32],
        };
        let fh = [batch_byte; 32];
        FinalizedBatch {
            batch_id: fh,
            proposal_id: make_id(proposal_byte),
            slot: 1,
            epoch: 1,
            leader_id: make_id(10),
            ordered_submission_ids: vec![make_id(1)],
            batch_root: [1u8; 32],
            parent_batch_id: [0u8; 32],
            finalized_at_height: 100,
            quorum_summary: qr,
            finalization_hash: fh,
        }
    }

    #[test]
    fn test_idempotent_delivery() {
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        let env1 = bridge.deliver(&batch);
        let env2 = bridge.deliver(&batch);
        // Delivery IDs remain the same (idempotent envelope)
        assert_eq!(env1.delivery_id, env2.delivery_id);
        assert_eq!(env1.delivery_hash, env2.delivery_hash);
    }

    #[test]
    fn test_repeated_delivery_increments_attempt_count_in_record() {
        // FIND-009: attempt_count must reflect actual delivery count
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        bridge.deliver(&batch);
        assert_eq!(bridge.get_record(&batch.batch_id).unwrap().attempt_count, 1);
        bridge.deliver(&batch);
        assert_eq!(bridge.get_record(&batch.batch_id).unwrap().attempt_count, 2);
        bridge.deliver(&batch);
        assert_eq!(bridge.get_record(&batch.batch_id).unwrap().attempt_count, 3);
    }

    #[test]
    fn test_is_delivered_after_deliver() {
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        assert!(!bridge.is_delivered(&batch.batch_id));
        bridge.deliver(&batch);
        assert!(bridge.is_delivered(&batch.batch_id));
    }

    #[test]
    fn test_valid_ack_accepted() {
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        let env = bridge.deliver(&batch);
        let ack = RuntimeExecutionAck::new(batch.batch_id, env.delivery_id, true, 1);
        assert!(bridge.record_ack(ack).is_ok());
        assert!(bridge.is_acked(&batch.batch_id));
    }

    #[test]
    fn test_ack_replay_rejected() {
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        let env = bridge.deliver(&batch);
        let ack1 = RuntimeExecutionAck::new(batch.batch_id, env.delivery_id, true, 1);
        let ack2 = RuntimeExecutionAck::new(batch.batch_id, env.delivery_id, true, 1);
        bridge.record_ack(ack1).unwrap();
        let result = bridge.record_ack(ack2);
        assert_eq!(result, Err(BridgeError::AckReplay));
    }

    #[test]
    fn test_rejection_recorded() {
        let mut bridge = HardenedRuntimeBridge::new();
        let batch = make_finalized_batch(1, 2);
        let env = bridge.deliver(&batch);
        let rej = RuntimeExecutionRejection {
            batch_id: batch.batch_id,
            delivery_id: env.delivery_id,
            reason: "invalid state".to_string(),
            epoch: 1,
        };
        bridge.record_rejection(rej);
        let record = bridge.get_record(&batch.batch_id).unwrap();
        assert!(record.acked);
        assert!(!record.accepted);
    }
}
