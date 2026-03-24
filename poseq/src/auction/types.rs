//! Commit-reveal auction types — Section 5 of the architecture specification.
//!
//! Defines BundleCommitment, BundleReveal, ExecutionStep, and supporting structures
//! for the two-phase solver competition system.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use crate::intent_pool::types::AssetId;

// ─── Resource Access Declarations (Phase 3) ─────────────────────────────────

/// Type of access to a resource.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub enum AccessType {
    Read,
    Write,
}

/// A declared resource access for parallel execution scheduling.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct ResourceAccess {
    pub resource_id: [u8; 32],
    pub access_type: AccessType,
}

impl ResourceAccess {
    pub fn read(resource_id: [u8; 32]) -> Self {
        ResourceAccess { resource_id, access_type: AccessType::Read }
    }

    pub fn write(resource_id: [u8; 32]) -> Self {
        ResourceAccess { resource_id, access_type: AccessType::Write }
    }
}

/// Check if two bundles conflict based on their resource declarations.
///
/// Two bundles conflict if they access the same resource and at least one is a Write.
/// Read/Read on the same resource is NOT a conflict.
pub fn bundles_conflict(a: &[ResourceAccess], b: &[ResourceAccess]) -> bool {
    for ra in a {
        for rb in b {
            if ra.resource_id == rb.resource_id {
                // Write/Write, Write/Read, Read/Write all conflict
                if ra.access_type == AccessType::Write || rb.access_type == AccessType::Write {
                    return true;
                }
            }
        }
    }
    false
}

// ─── Execution Step ─────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum OperationType {
    Debit,
    Credit,
    Swap,
    Lock,
    Unlock,
    Create,
    Destroy,
    Transfer,
    /// Contract state transition — custom_data carries schema_id, method, proposed_state.
    ContractCall,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct OperationParams {
    pub asset: Option<AssetId>,
    pub amount: Option<u128>,
    pub recipient: Option<[u8; 32]>,
    pub pool_id: Option<[u8; 32]>,
    pub custom_data: Option<Vec<u8>>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ExecutionStep {
    pub step_index: u16,
    pub operation: OperationType,
    pub object_id: [u8; 32],
    pub read_set: Vec<[u8; 32]>,
    pub write_set: Vec<[u8; 32]>,
    pub params: OperationParams,
}

// ─── Predicted Output ───────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PredictedOutput {
    pub intent_id: [u8; 32],
    pub asset_out: AssetId,
    pub amount_out: u128,
    pub fee_charged_bps: u64,
}

impl PredictedOutput {
    /// Compute the effective user outcome score for ranking.
    /// user_outcome_score = amount_out * (10000 - fee_bps) / 10000
    pub fn user_outcome_score(&self) -> u128 {
        let fee_factor = 10_000u128.saturating_sub(self.fee_charged_bps as u128);
        self.amount_out.saturating_mul(fee_factor) / 10_000
    }
}

// ─── Fee Breakdown ──────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FeeBreakdown {
    pub solver_fee_bps: u64,
    pub protocol_fee_bps: u64,
    pub total_fee_bps: u64,
}

// ─── Liquidity Source ───────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum LiquidityType {
    Pool,
    Vault,
    External,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LiquiditySource {
    pub source_type: LiquidityType,
    pub source_id: [u8; 32],
    pub asset: AssetId,
    pub amount_used: u128,
}

// ─── Bundle Commitment ──────────────────────────────────────────────────────

/// Sealed commitment submitted during the commit phase.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BundleCommitment {
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub batch_window: u64,
    pub target_intent_count: u16,
    pub commitment_hash: [u8; 32],
    pub expected_outputs_hash: [u8; 32],
    pub execution_plan_hash: [u8; 32],
    pub valid_until: u64,
    pub bond_locked: u128,
    pub signature: Vec<u8>,
}

impl BundleCommitment {
    /// Verify that the commitment hasn't expired.
    pub fn is_valid_at(&self, block: u64) -> bool {
        block <= self.valid_until
    }
}

// ─── Bundle Reveal ──────────────────────────────────────────────────────────

/// Full reveal submitted during the reveal phase.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BundleReveal {
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub batch_window: u64,

    // Execution plan
    pub target_intent_ids: Vec<[u8; 32]>,
    pub execution_steps: Vec<ExecutionStep>,
    pub liquidity_sources: Vec<LiquiditySource>,
    pub predicted_outputs: Vec<PredictedOutput>,
    pub fee_breakdown: FeeBreakdown,

    // Resource declarations (Phase 3) — for parallel execution scheduling
    pub resource_declarations: Vec<ResourceAccess>,

    // Commitment link
    pub nonce: [u8; 32],
    pub proof_data: Vec<u8>,
    pub signature: Vec<u8>,
}

impl BundleReveal {
    /// Compute the commitment hash that should match the BundleCommitment.
    ///
    /// commitment_hash = SHA256(bundle_id ‖ solver_id ‖ execution_steps_bytes ‖
    ///                          predicted_outputs_bytes ‖ fee_breakdown_bytes ‖ nonce)
    pub fn compute_commitment_hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.bundle_id);
        hasher.update(self.solver_id);
        let steps_bytes = bincode::serialize(&self.execution_steps).unwrap_or_default();
        hasher.update(&steps_bytes);
        let outputs_bytes = bincode::serialize(&self.predicted_outputs).unwrap_or_default();
        hasher.update(&outputs_bytes);
        let fee_bytes = bincode::serialize(&self.fee_breakdown).unwrap_or_default();
        hasher.update(&fee_bytes);
        hasher.update(self.nonce);
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }

    /// Compute the expected outputs hash.
    pub fn compute_expected_outputs_hash(&self) -> [u8; 32] {
        let bytes = bincode::serialize(&self.predicted_outputs).unwrap_or_default();
        let result = Sha256::digest(&bytes);
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }

    /// Compute the execution plan hash.
    pub fn compute_execution_plan_hash(&self) -> [u8; 32] {
        let bytes = bincode::serialize(&self.execution_steps).unwrap_or_default();
        let result = Sha256::digest(&bytes);
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }

    /// Verify that this reveal matches a given commitment.
    pub fn verify_against_commitment(&self, commitment: &BundleCommitment) -> Result<(), RevealValidationError> {
        // Bundle ID must match
        if self.bundle_id != commitment.bundle_id {
            return Err(RevealValidationError::BundleIdMismatch);
        }
        // Solver ID must match
        if self.solver_id != commitment.solver_id {
            return Err(RevealValidationError::SolverIdMismatch);
        }
        // Batch window must match
        if self.batch_window != commitment.batch_window {
            return Err(RevealValidationError::BatchWindowMismatch);
        }
        // Commitment hash must match
        let computed = self.compute_commitment_hash();
        if computed != commitment.commitment_hash {
            return Err(RevealValidationError::CommitmentHashMismatch {
                expected: commitment.commitment_hash,
                computed,
            });
        }
        // Outputs hash must match
        let outputs_hash = self.compute_expected_outputs_hash();
        if outputs_hash != commitment.expected_outputs_hash {
            return Err(RevealValidationError::OutputsHashMismatch);
        }
        // Plan hash must match
        let plan_hash = self.compute_execution_plan_hash();
        if plan_hash != commitment.execution_plan_hash {
            return Err(RevealValidationError::PlanHashMismatch);
        }

        Ok(())
    }

    /// Best user outcome score across all predicted outputs.
    pub fn best_user_outcome(&self) -> u128 {
        self.predicted_outputs.iter()
            .map(|o| o.user_outcome_score())
            .max()
            .unwrap_or(0)
    }

    /// Total user outcome score (sum across all intent fills).
    pub fn total_user_outcome(&self) -> u128 {
        self.predicted_outputs.iter()
            .map(|o| o.user_outcome_score())
            .sum()
    }
}

// ─── Reveal Validation Errors ───────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RevealValidationError {
    BundleIdMismatch,
    SolverIdMismatch,
    BatchWindowMismatch,
    CommitmentHashMismatch { expected: [u8; 32], computed: [u8; 32] },
    OutputsHashMismatch,
    PlanHashMismatch,
    NoMatchingCommitment,
    RevealPhaseNotOpen,
    RevealPhaseClosed,
    SolverNotActive,
    SolverInsufficientBond,
    IntentNotFound([u8; 32]),
    IntentExpired([u8; 32]),
    FeeExceedsMax { intent_id: [u8; 32], charged: u64, max: u64 },
    OutputBelowMin { intent_id: [u8; 32], output: u128, min: u128 },
    TooManySteps { count: usize, max: usize },
    DuplicateReveal,
}

impl std::fmt::Display for RevealValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::BundleIdMismatch => write!(f, "bundle_id mismatch"),
            Self::SolverIdMismatch => write!(f, "solver_id mismatch"),
            Self::BatchWindowMismatch => write!(f, "batch_window mismatch"),
            Self::CommitmentHashMismatch { expected, computed } => {
                write!(f, "commitment hash mismatch: expected {}, computed {}",
                    hex::encode(&expected[..8]), hex::encode(&computed[..8]))
            }
            Self::OutputsHashMismatch => write!(f, "outputs hash mismatch"),
            Self::PlanHashMismatch => write!(f, "plan hash mismatch"),
            Self::NoMatchingCommitment => write!(f, "no matching commitment"),
            Self::RevealPhaseNotOpen => write!(f, "reveal phase not open"),
            Self::RevealPhaseClosed => write!(f, "reveal phase closed"),
            Self::SolverNotActive => write!(f, "solver not active"),
            Self::SolverInsufficientBond => write!(f, "solver insufficient bond"),
            Self::IntentNotFound(id) => write!(f, "intent not found: {}", hex::encode(&id[..4])),
            Self::IntentExpired(id) => write!(f, "intent expired: {}", hex::encode(&id[..4])),
            Self::FeeExceedsMax { intent_id, charged, max } => {
                write!(f, "fee {} exceeds max {} for intent {}", charged, max, hex::encode(&intent_id[..4]))
            }
            Self::OutputBelowMin { intent_id, output, min } => {
                write!(f, "output {} below min {} for intent {}", output, min, hex::encode(&intent_id[..4]))
            }
            Self::TooManySteps { count, max } => write!(f, "too many steps: {} > {}", count, max),
            Self::DuplicateReveal => write!(f, "duplicate reveal"),
        }
    }
}

impl std::error::Error for RevealValidationError {}

// ─── Sequenced Bundle (output of ordering) ──────────────────────────────────

/// A bundle that has been selected and placed in the canonical sequence.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SequencedBundle {
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub target_intent_ids: Vec<[u8; 32]>,
    pub execution_steps: Vec<ExecutionStep>,
    pub predicted_outputs: Vec<PredictedOutput>,
    pub resource_declarations: Vec<ResourceAccess>,
    pub sequence_index: u32,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intent_pool::types::{AssetId, AssetType};

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32]; id[0] = b;
        AssetId { chain_id: 0, asset_type: AssetType::Token, identifier: id }
    }

    fn make_bundle_pair() -> (BundleCommitment, BundleReveal) {
        let bundle_id = [1u8; 32];
        let solver_id = [2u8; 32];
        let nonce = [3u8; 32];

        let steps = vec![ExecutionStep {
            step_index: 0,
            operation: OperationType::Debit,
            object_id: [10u8; 32],
            read_set: vec![[10u8; 32]],
            write_set: vec![[10u8; 32]],
            params: OperationParams {
                asset: Some(make_asset(1)),
                amount: Some(1000),
                recipient: None,
                pool_id: None,
                custom_data: None,
            },
        }];

        let outputs = vec![PredictedOutput {
            intent_id: [5u8; 32],
            asset_out: make_asset(2),
            amount_out: 950,
            fee_charged_bps: 30,
        }];

        let fee = FeeBreakdown {
            solver_fee_bps: 20,
            protocol_fee_bps: 10,
            total_fee_bps: 30,
        };

        let reveal = BundleReveal {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_ids: vec![[5u8; 32]],
            execution_steps: steps,
            liquidity_sources: vec![],
            predicted_outputs: outputs,
            fee_breakdown: fee,
            nonce,
            resource_declarations: vec![],
            proof_data: vec![],
            signature: vec![1u8; 64],
        };

        let commitment = BundleCommitment {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_count: 1,
            commitment_hash: reveal.compute_commitment_hash(),
            expected_outputs_hash: reveal.compute_expected_outputs_hash(),
            execution_plan_hash: reveal.compute_execution_plan_hash(),
            valid_until: 100,
            bond_locked: 50_000,
            signature: vec![1u8; 64],
        };

        (commitment, reveal)
    }

    #[test]
    fn test_commitment_hash_deterministic() {
        let (_, reveal) = make_bundle_pair();
        let h1 = reveal.compute_commitment_hash();
        let h2 = reveal.compute_commitment_hash();
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_reveal_matches_commitment() {
        let (commitment, reveal) = make_bundle_pair();
        assert!(reveal.verify_against_commitment(&commitment).is_ok());
    }

    #[test]
    fn test_tampered_reveal_fails() {
        let (commitment, mut reveal) = make_bundle_pair();
        // Tamper with amount
        reveal.predicted_outputs[0].amount_out = 999;
        let result = reveal.verify_against_commitment(&commitment);
        assert!(result.is_err());
    }

    #[test]
    fn test_mismatched_solver_fails() {
        let (commitment, mut reveal) = make_bundle_pair();
        reveal.solver_id = [99u8; 32];
        assert_eq!(
            reveal.verify_against_commitment(&commitment).unwrap_err(),
            RevealValidationError::SolverIdMismatch
        );
    }

    #[test]
    fn test_user_outcome_score() {
        let output = PredictedOutput {
            intent_id: [0u8; 32],
            asset_out: make_asset(1),
            amount_out: 10_000,
            fee_charged_bps: 30,
        };
        // 10000 * (10000 - 30) / 10000 = 10000 * 9970 / 10000 = 9970
        assert_eq!(output.user_outcome_score(), 9970);
    }
}
