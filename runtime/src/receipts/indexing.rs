//! Receipt indexing and enhanced receipt types — Section 10.1-10.4.
//!
//! Provides intent-aware receipt structure and multi-key indexing for
//! queries by receipt_id, intent_id, solver_id, and batch_id.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use crate::objects::base::ObjectId;
use crate::settlement::engine::ExecutionReceipt;

/// Enhanced execution receipt with intent and solver metadata.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentExecutionReceipt {
    // Identity
    pub receipt_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub sequence_slot: u64,

    // Execution results
    pub amount_in: u128,
    pub amount_out: u128,
    pub fee_paid_bps: u64,
    pub recipient: [u8; 32],
    pub fill_fraction_bps: u64,  // 0-10000 (10000 = full fill)

    // Status
    pub execution_status: ExecutionStatus,
    pub failure_reason: Option<String>,
    pub gas_used: u64,

    // State proofs
    pub state_root_before: [u8; 32],
    pub state_root_after: [u8; 32],
    pub objects_read: Vec<(ObjectId, u64)>,    // (object_id, version_read)
    pub objects_written: Vec<(ObjectId, u64)>,  // (object_id, version_written)

    // Proof
    pub proof_hash: [u8; 32],

    // Timing
    pub block_height: u64,
    pub batch_window: u64,
    pub epoch: u64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ExecutionStatus {
    Succeeded,
    Failed,
    PartialFill,
}

impl IntentExecutionReceipt {
    /// Compute deterministic receipt_id = SHA256(intent_id ‖ bundle_id ‖ batch_id).
    pub fn compute_receipt_id(intent_id: &[u8; 32], bundle_id: &[u8; 32], batch_id: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(intent_id);
        hasher.update(bundle_id);
        hasher.update(batch_id);
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Build from a base ExecutionReceipt plus intent metadata.
    pub fn from_base(
        base: &ExecutionReceipt,
        intent_id: [u8; 32],
        bundle_id: [u8; 32],
        solver_id: [u8; 32],
        batch_id: [u8; 32],
        sequence_slot: u64,
        amount_in: u128,
        amount_out: u128,
        fee_paid_bps: u64,
        recipient: [u8; 32],
        state_root_before: [u8; 32],
        state_root_after: [u8; 32],
        block_height: u64,
        batch_window: u64,
        epoch: u64,
    ) -> Self {
        let receipt_id = Self::compute_receipt_id(&intent_id, &bundle_id, &batch_id);
        let proof_hash = Self::compute_proof_hash(&base);

        let execution_status = if base.success {
            ExecutionStatus::Succeeded
        } else {
            ExecutionStatus::Failed
        };

        IntentExecutionReceipt {
            receipt_id,
            intent_id,
            bundle_id,
            solver_id,
            batch_id,
            sequence_slot,
            amount_in,
            amount_out,
            fee_paid_bps,
            recipient,
            fill_fraction_bps: if amount_out > 0 { 10_000 } else { 0 },
            execution_status,
            failure_reason: base.error.clone(),
            gas_used: base.gas_used,
            state_root_before,
            state_root_after,
            objects_read: base.affected_objects.iter().map(|id| (*id, 0)).collect(),
            objects_written: base.version_transitions.iter().map(|(id, _, new)| (*id, *new)).collect(),
            proof_hash,
            block_height,
            batch_window,
            epoch,
        }
    }

    fn compute_proof_hash(receipt: &ExecutionReceipt) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(receipt.tx_id);
        hasher.update(if receipt.success { [1u8] } else { [0u8] });
        hasher.update(receipt.gas_used.to_be_bytes());
        for (id, old, new) in &receipt.version_transitions {
            hasher.update(id.as_bytes());
            hasher.update(old.to_be_bytes());
            hasher.update(new.to_be_bytes());
        }
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }
}

/// Multi-key receipt index for efficient lookups.
pub struct ReceiptIndex {
    /// Primary: receipt_id → receipt
    by_receipt_id: BTreeMap<[u8; 32], IntentExecutionReceipt>,
    /// Index: intent_id → [receipt_id]
    by_intent_id: BTreeMap<[u8; 32], Vec<[u8; 32]>>,
    /// Index: solver_id → [receipt_id]
    by_solver_id: BTreeMap<[u8; 32], Vec<[u8; 32]>>,
    /// Index: batch_id → [(sequence_slot, receipt_id)]
    by_batch_id: BTreeMap<[u8; 32], Vec<(u64, [u8; 32])>>,
}

impl ReceiptIndex {
    pub fn new() -> Self {
        ReceiptIndex {
            by_receipt_id: BTreeMap::new(),
            by_intent_id: BTreeMap::new(),
            by_solver_id: BTreeMap::new(),
            by_batch_id: BTreeMap::new(),
        }
    }

    /// Insert a receipt into all indexes.
    pub fn insert(&mut self, receipt: IntentExecutionReceipt) {
        let receipt_id = receipt.receipt_id;

        self.by_intent_id.entry(receipt.intent_id).or_default().push(receipt_id);
        self.by_solver_id.entry(receipt.solver_id).or_default().push(receipt_id);
        self.by_batch_id.entry(receipt.batch_id).or_default().push((receipt.sequence_slot, receipt_id));
        self.by_receipt_id.insert(receipt_id, receipt);
    }

    /// Lookup by receipt_id.
    pub fn get(&self, receipt_id: &[u8; 32]) -> Option<&IntentExecutionReceipt> {
        self.by_receipt_id.get(receipt_id)
    }

    /// Lookup all receipts for an intent.
    pub fn by_intent(&self, intent_id: &[u8; 32]) -> Vec<&IntentExecutionReceipt> {
        self.by_intent_id.get(intent_id)
            .map(|ids| ids.iter().filter_map(|id| self.by_receipt_id.get(id)).collect())
            .unwrap_or_default()
    }

    /// Lookup all receipts for a solver.
    pub fn by_solver(&self, solver_id: &[u8; 32]) -> Vec<&IntentExecutionReceipt> {
        self.by_solver_id.get(solver_id)
            .map(|ids| ids.iter().filter_map(|id| self.by_receipt_id.get(id)).collect())
            .unwrap_or_default()
    }

    /// Lookup all receipts in a batch, sorted by sequence_slot.
    pub fn by_batch(&self, batch_id: &[u8; 32]) -> Vec<&IntentExecutionReceipt> {
        self.by_batch_id.get(batch_id)
            .map(|entries| {
                let mut sorted = entries.clone();
                sorted.sort_by_key(|(slot, _)| *slot);
                sorted.iter().filter_map(|(_, id)| self.by_receipt_id.get(id)).collect()
            })
            .unwrap_or_default()
    }

    /// Check for double-fill: same intent with multiple successful receipts.
    pub fn is_double_fill(&self, intent_id: &[u8; 32]) -> bool {
        self.by_intent(intent_id).iter()
            .filter(|r| r.execution_status == ExecutionStatus::Succeeded)
            .count() > 1
    }

    pub fn len(&self) -> usize {
        self.by_receipt_id.len()
    }

    pub fn is_empty(&self) -> bool {
        self.by_receipt_id.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::objects::base::ObjectId;
    use crate::settlement::engine::ExecutionReceipt;

    fn make_base_receipt(tx_id: [u8; 32]) -> ExecutionReceipt {
        ExecutionReceipt {
            tx_id,
            success: true,
            affected_objects: vec![ObjectId::new([10u8; 32])],
            version_transitions: vec![(ObjectId::new([10u8; 32]), 1, 2)],
            error: None,
            gas_used: 5000,
            events: vec![],
        }
    }

    fn make_intent_receipt(intent_byte: u8, solver_byte: u8, batch_byte: u8) -> IntentExecutionReceipt {
        let intent_id = { let mut id = [0u8; 32]; id[0] = intent_byte; id };
        let solver_id = { let mut id = [0u8; 32]; id[0] = solver_byte; id };
        let batch_id = { let mut id = [0u8; 32]; id[0] = batch_byte; id };
        let bundle_id = { let mut id = [0u8; 32]; id[0] = intent_byte; id[1] = solver_byte; id };
        let base = make_base_receipt(intent_id);

        IntentExecutionReceipt::from_base(
            &base, intent_id, bundle_id, solver_id, batch_id,
            0, 1000, 950, 30, [0u8; 32],
            [0u8; 32], [1u8; 32], 100, 1, 1,
        )
    }

    #[test]
    fn test_receipt_id_deterministic() {
        let id1 = IntentExecutionReceipt::compute_receipt_id(&[1u8; 32], &[2u8; 32], &[3u8; 32]);
        let id2 = IntentExecutionReceipt::compute_receipt_id(&[1u8; 32], &[2u8; 32], &[3u8; 32]);
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_receipt_index_insert_and_query() {
        let mut index = ReceiptIndex::new();
        let r1 = make_intent_receipt(1, 10, 100);
        let r2 = make_intent_receipt(2, 10, 100); // same solver, same batch
        let r1_id = r1.receipt_id;

        index.insert(r1);
        index.insert(r2);

        assert_eq!(index.len(), 2);

        // By receipt_id
        assert!(index.get(&r1_id).is_some());

        // By solver
        let solver_id = { let mut id = [0u8; 32]; id[0] = 10; id };
        assert_eq!(index.by_solver(&solver_id).len(), 2);

        // By batch
        let batch_id = { let mut id = [0u8; 32]; id[0] = 100; id };
        assert_eq!(index.by_batch(&batch_id).len(), 2);
    }

    #[test]
    fn test_double_fill_detection() {
        let mut index = ReceiptIndex::new();
        let r1 = make_intent_receipt(1, 10, 100);
        let intent_id = r1.intent_id;

        index.insert(r1);
        assert!(!index.is_double_fill(&intent_id));

        // Insert another receipt for the same intent
        let mut r2 = make_intent_receipt(1, 20, 101);
        // Change bundle_id to avoid dedup
        r2.bundle_id[1] = 99;
        r2.receipt_id = IntentExecutionReceipt::compute_receipt_id(&r2.intent_id, &r2.bundle_id, &r2.batch_id);
        index.insert(r2);
        assert!(index.is_double_fill(&intent_id));
    }
}
