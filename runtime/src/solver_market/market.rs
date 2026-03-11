use crate::capabilities::checker::Capability;
use crate::objects::base::ObjectId;
use crate::plan_validation::validator::ValidationReasonCode;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// The semantic type of a plan action.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PlanActionType {
    DebitBalance,
    CreditBalance,
    SwapPoolAmounts,
    LockBalance,
    UnlockBalance,
    /// Extension point for future action types.
    Custom(String),
}

/// A single atomic action within a candidate plan.
#[derive(Debug, Clone)]
pub struct PlanAction {
    pub action_type: PlanActionType,
    pub target_object: ObjectId,
    pub amount: Option<u128>,
    pub metadata: BTreeMap<String, String>,
}

/// Proof that a candidate plan satisfies one of the intent's constraints.
///
/// Currently a structured declaration. Future versions may include ZK proofs.
#[derive(Debug, Clone)]
pub struct PlanConstraintProof {
    pub constraint_name: String,
    pub declared_satisfied: bool,
    /// Supporting numeric value, e.g. actual output amount for a slippage check.
    pub supporting_value: Option<u128>,
    pub explanation: Option<String>,
}

/// A fully described plan submitted by a solver for a specific intent.
#[derive(Debug, Clone)]
pub struct CandidatePlan {
    pub plan_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub actions: Vec<PlanAction>,
    pub required_capabilities: Vec<Capability>,
    /// Objects this plan will read (for conflict detection).
    pub object_reads: Vec<ObjectId>,
    /// Objects this plan will write (for conflict detection).
    pub object_writes: Vec<ObjectId>,
    pub expected_output_amount: u128,
    pub fee_quote: u64,
    /// 0–10000 bps, solver self-reported, validated by runtime.
    pub quality_score: u64,
    pub constraint_proofs: Vec<PlanConstraintProof>,
    /// SHA256 of deterministic encoding of this plan (excludes plan_hash, explanation, metadata).
    pub plan_hash: [u8; 32],
    pub submitted_at_sequence: u64,
    /// AI/agent explainability field.
    pub explanation: Option<String>,
    pub metadata: BTreeMap<String, String>,
}

impl CandidatePlan {
    /// Compute the canonical SHA256 hash of this plan.
    ///
    /// Fields excluded from hash: `plan_hash`, `explanation`, `metadata`.
    pub fn compute_hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();

        // plan_id
        hasher.update(&self.plan_id);
        // intent_id
        hasher.update(&self.intent_id);
        // solver_id
        hasher.update(&self.solver_id);

        // actions: encode each action deterministically
        for action in &self.actions {
            let type_tag: u8 = match &action.action_type {
                PlanActionType::DebitBalance => 0,
                PlanActionType::CreditBalance => 1,
                PlanActionType::SwapPoolAmounts => 2,
                PlanActionType::LockBalance => 3,
                PlanActionType::UnlockBalance => 4,
                PlanActionType::Custom(s) => {
                    hasher.update(s.as_bytes());
                    255
                }
            };
            hasher.update([type_tag]);
            hasher.update(action.target_object.as_bytes());
            match action.amount {
                None => hasher.update([0u8]),
                Some(a) => {
                    hasher.update([1u8]);
                    hasher.update(&a.to_le_bytes());
                }
            }
        }

        // required_capabilities
        for cap in &self.required_capabilities {
            // Encode capability as a stable byte tag
            let cap_tag = capability_tag(cap);
            hasher.update([cap_tag]);
        }

        // object_reads
        let read_count = self.object_reads.len() as u64;
        hasher.update(&read_count.to_le_bytes());
        for id in &self.object_reads {
            hasher.update(id.as_bytes());
        }

        // object_writes
        let write_count = self.object_writes.len() as u64;
        hasher.update(&write_count.to_le_bytes());
        for id in &self.object_writes {
            hasher.update(id.as_bytes());
        }

        // expected_output_amount
        hasher.update(&self.expected_output_amount.to_le_bytes());

        // fee_quote
        hasher.update(&self.fee_quote.to_le_bytes());

        // quality_score
        hasher.update(&self.quality_score.to_le_bytes());

        // constraint_proofs
        for proof in &self.constraint_proofs {
            hasher.update(proof.constraint_name.as_bytes());
            hasher.update([proof.declared_satisfied as u8]);
            match proof.supporting_value {
                None => hasher.update([0u8]),
                Some(v) => {
                    hasher.update([1u8]);
                    hasher.update(&v.to_le_bytes());
                }
            }
        }

        // submitted_at_sequence
        hasher.update(&self.submitted_at_sequence.to_le_bytes());

        hasher.finalize().into()
    }

    /// Verify the stored `plan_hash` matches a fresh computation.
    pub fn validate_hash(&self) -> bool {
        self.compute_hash() == self.plan_hash
    }
}

fn capability_tag(cap: &Capability) -> u8 {
    match cap {
        Capability::ReadObject => 0,
        Capability::WriteObject => 1,
        Capability::TransferAsset => 2,
        Capability::SwapAsset => 3,
        Capability::ProvideLiquidity => 4,
        Capability::WithdrawLiquidity => 5,
        Capability::MintAsset => 6,
        Capability::BurnAsset => 7,
        Capability::ModifyGovernance => 8,
        Capability::UpdateIdentity => 9,
    }
}

/// The runtime's evaluation result for a single candidate plan.
#[derive(Debug, Clone)]
pub struct PlanEvaluationResult {
    pub plan_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub passed: bool,
    pub reason_codes: Vec<ValidationReasonCode>,
    /// 0–10000 bps composite score.
    pub normalized_score: u64,
    pub validated_object_reads: Vec<ObjectId>,
    pub validated_object_writes: Vec<ObjectId>,
    /// Estimated resource cost for settlement.
    pub settlement_footprint: u64,
    pub validated_output_amount: u128,
}
