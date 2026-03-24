//! Phase 7 — Enhanced settlement result model for chain anchoring.
//!
//! Extends the base SettlementResult with cryptographic commitments,
//! batch linkage, and chain-digestible fields for state root anchoring.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use crate::objects::base::ObjectId;

/// Transfer record generated during settlement.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransferRecord {
    pub from: [u8; 32],
    pub to: [u8; 32],
    pub asset_id: [u8; 32],
    pub amount: u128,
}

/// Object state update record.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ObjectUpdate {
    pub object_id: ObjectId,
    pub old_version: u64,
    pub new_version: u64,
    pub update_type: ObjectUpdateType,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ObjectUpdateType {
    Modified,
    Created,
    Destroyed,
}

/// Solver reward allocation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SolverRewardEntry {
    pub solver_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub reward_amount: u128,
    pub fee_bps: u64,
}

/// Validator fee allocation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ValidatorFeeEntry {
    pub validator_id: [u8; 32],
    pub fee_amount: u128,
}

/// Gas consumption summary.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GasSummary {
    pub total_gas_used: u64,
    pub total_gas_limit: u64,
    pub per_intent_gas: Vec<(u64, [u8; 32])>, // (gas_used, intent_id)
}

/// Settlement status.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SettlementStatus {
    /// All intents settled successfully.
    FullySettled,
    /// Some intents failed, batch partially settled.
    PartiallySettled,
    /// All intents failed.
    Failed,
}

/// Enhanced settlement result for chain anchoring and accountability.
///
/// This structure links a PoSeq finalized batch to its runtime execution outcome.
/// The `hash()` is deterministic and can be verified on-chain.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainSettlementResult {
    /// Hash of the PoSeq finalized batch this settlement applies to.
    pub batch_hash: [u8; 32],
    /// State root before execution.
    pub pre_state_root: [u8; 32],
    /// State root after execution.
    pub post_state_root: [u8; 32],
    /// Hash of all execution receipts.
    pub execution_receipt_hash: [u8; 32],
    /// Transfer records generated during settlement.
    pub transfer_records: Vec<TransferRecord>,
    /// Object state updates.
    pub object_updates: Vec<ObjectUpdate>,
    /// Solver reward allocations.
    pub solver_rewards: Vec<SolverRewardEntry>,
    /// Validator fee allocations.
    pub validator_fees: Vec<ValidatorFeeEntry>,
    /// Gas consumption summary.
    pub gas_summary: GasSummary,
    /// Overall settlement status.
    pub settlement_status: SettlementStatus,
    /// Epoch of this settlement.
    pub epoch: u64,
    /// Sequence number within epoch.
    pub sequence_number: u64,
    /// Number of intents settled.
    pub settled_count: u32,
    /// Number of intents that failed.
    pub failed_count: u32,
}

impl ChainSettlementResult {
    /// Compute the deterministic hash of this settlement result.
    ///
    /// settlement_hash = SHA256(
    ///     batch_hash ‖ pre_state_root ‖ post_state_root ‖
    ///     execution_receipt_hash ‖ epoch ‖ sequence_number
    /// )
    pub fn hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.batch_hash);
        hasher.update(self.pre_state_root);
        hasher.update(self.post_state_root);
        hasher.update(self.execution_receipt_hash);
        hasher.update(self.epoch.to_be_bytes());
        hasher.update(self.sequence_number.to_be_bytes());
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }

    /// Verify this settlement result was produced from the given batch.
    pub fn verify_against_batch(&self, expected_batch_hash: &[u8; 32]) -> bool {
        self.batch_hash == *expected_batch_hash
    }

    /// Compute execution receipt hash from individual receipt hashes.
    pub fn compute_receipt_hash(receipt_hashes: &[[u8; 32]]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for h in receipt_hashes {
            hasher.update(h);
        }
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }

    /// Build from a base SettlementResult plus batch metadata.
    pub fn from_base(
        base: &crate::settlement::engine::SettlementResult,
        batch_hash: [u8; 32],
        pre_state_root: [u8; 32],
        sequence_number: u64,
    ) -> Self {
        let receipt_hashes: Vec<[u8; 32]> = base.receipts.iter()
            .map(|r| {
                let mut hasher = Sha256::new();
                hasher.update(r.tx_id);
                hasher.update([if r.success { 1u8 } else { 0u8 }]);
                hasher.update(r.gas_used.to_be_bytes());
                let result = hasher.finalize();
                let mut h = [0u8; 32];
                h.copy_from_slice(&result);
                h
            })
            .collect();

        let status = if base.failed == 0 {
            SettlementStatus::FullySettled
        } else if base.succeeded == 0 {
            SettlementStatus::Failed
        } else {
            SettlementStatus::PartiallySettled
        };

        ChainSettlementResult {
            batch_hash,
            pre_state_root,
            post_state_root: base.state_root,
            execution_receipt_hash: Self::compute_receipt_hash(&receipt_hashes),
            transfer_records: Vec::new(), // populated by caller
            object_updates: Vec::new(),   // populated by caller
            solver_rewards: Vec::new(),   // populated by caller
            validator_fees: Vec::new(),   // populated by caller
            gas_summary: GasSummary {
                total_gas_used: base.receipts.iter().map(|r| r.gas_used).sum(),
                total_gas_limit: 0,
                per_intent_gas: base.receipts.iter().map(|r| (r.gas_used, r.tx_id)).collect(),
            },
            settlement_status: status,
            epoch: base.epoch,
            sequence_number,
            settled_count: base.succeeded as u32,
            failed_count: base.failed as u32,
        }
    }
}

/// Data package for submitting settlement results to the control chain.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SettlementAnchor {
    /// Settlement result hash.
    pub settlement_hash: [u8; 32],
    /// Batch this settlement is for.
    pub batch_hash: [u8; 32],
    /// Post-execution state root.
    pub post_state_root: [u8; 32],
    /// Execution receipt hash.
    pub execution_receipt_hash: [u8; 32],
    /// Epoch.
    pub epoch: u64,
    /// Sequence number.
    pub sequence_number: u64,
    /// Settled / failed counts.
    pub settled_count: u32,
    pub failed_count: u32,
}

impl SettlementAnchor {
    pub fn from_result(result: &ChainSettlementResult) -> Self {
        SettlementAnchor {
            settlement_hash: result.hash(),
            batch_hash: result.batch_hash,
            post_state_root: result.post_state_root,
            execution_receipt_hash: result.execution_receipt_hash,
            epoch: result.epoch,
            sequence_number: result.sequence_number,
            settled_count: result.settled_count,
            failed_count: result.failed_count,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_result(batch_byte: u8, epoch: u64) -> ChainSettlementResult {
        let mut batch_hash = [0u8; 32]; batch_hash[0] = batch_byte;
        ChainSettlementResult {
            batch_hash,
            pre_state_root: [0xAA; 32],
            post_state_root: [0xBB; 32],
            execution_receipt_hash: [0xCC; 32],
            transfer_records: vec![],
            object_updates: vec![],
            solver_rewards: vec![],
            validator_fees: vec![],
            gas_summary: GasSummary { total_gas_used: 5000, total_gas_limit: 10000, per_intent_gas: vec![] },
            settlement_status: SettlementStatus::FullySettled,
            epoch,
            sequence_number: 1,
            settled_count: 3,
            failed_count: 0,
        }
    }

    #[test]
    fn test_settlement_hash_deterministic() {
        let r = make_result(1, 1);
        assert_eq!(r.hash(), r.hash());
        assert_ne!(r.hash(), [0u8; 32]);
    }

    #[test]
    fn test_settlement_hash_differs_by_batch() {
        let r1 = make_result(1, 1);
        let r2 = make_result(2, 1);
        assert_ne!(r1.hash(), r2.hash());
    }

    #[test]
    fn test_settlement_hash_differs_by_epoch() {
        let r1 = make_result(1, 1);
        let r2 = make_result(1, 2);
        assert_ne!(r1.hash(), r2.hash());
    }

    #[test]
    fn test_verify_against_batch() {
        let r = make_result(1, 1);
        let mut expected = [0u8; 32]; expected[0] = 1;
        assert!(r.verify_against_batch(&expected));
        assert!(!r.verify_against_batch(&[0xFF; 32]));
    }

    #[test]
    fn test_receipt_hash_computation() {
        let hashes = vec![[1u8; 32], [2u8; 32]];
        let h1 = ChainSettlementResult::compute_receipt_hash(&hashes);
        let h2 = ChainSettlementResult::compute_receipt_hash(&hashes);
        assert_eq!(h1, h2);
        // Different order → different hash
        let h3 = ChainSettlementResult::compute_receipt_hash(&[[2u8; 32], [1u8; 32]]);
        assert_ne!(h1, h3);
    }

    #[test]
    fn test_settlement_anchor() {
        let r = make_result(1, 5);
        let anchor = SettlementAnchor::from_result(&r);
        assert_eq!(anchor.batch_hash, r.batch_hash);
        assert_eq!(anchor.settlement_hash, r.hash());
        assert_eq!(anchor.epoch, 5);
    }

    #[test]
    fn test_from_base_settlement_result() {
        use crate::settlement::engine::{SettlementResult, ExecutionReceipt};
        use crate::objects::base::ObjectId;

        let base = SettlementResult {
            epoch: 1,
            total_plans: 3,
            succeeded: 2,
            failed: 1,
            receipts: vec![
                ExecutionReceipt { tx_id: [1u8; 32], success: true, affected_objects: vec![], version_transitions: vec![], error: None, gas_used: 1000, events: vec![] },
                ExecutionReceipt { tx_id: [2u8; 32], success: true, affected_objects: vec![], version_transitions: vec![], error: None, gas_used: 2000, events: vec![] },
                ExecutionReceipt { tx_id: [3u8; 32], success: false, affected_objects: vec![], version_transitions: vec![], error: Some("fail".into()), gas_used: 500, events: vec![] },
            ],
            state_root: [0xDD; 32],
            ibc_hooks: vec![],
            schedules: vec![],
            balance_bindings: vec![],
            all_events: vec![],
        };

        let result = ChainSettlementResult::from_base(&base, [0xAA; 32], [0xBB; 32], 1);
        assert_eq!(result.batch_hash, [0xAA; 32]);
        assert_eq!(result.pre_state_root, [0xBB; 32]);
        assert_eq!(result.post_state_root, [0xDD; 32]);
        assert_eq!(result.settlement_status, SettlementStatus::PartiallySettled);
        assert_eq!(result.gas_summary.total_gas_used, 3500);
        assert_eq!(result.settled_count, 2);
        assert_eq!(result.failed_count, 1);
    }
}
