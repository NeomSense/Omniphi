//! Dispute and fraud proof types — Section 11.3 of the architecture specification.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Types of fraud proofs that can be submitted.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum FraudProofType {
    /// Solver's reveal did not match their commitment hash.
    InvalidReveal,
    /// Solver charged more than intent.max_fee.
    FeeViolation,
    /// User received less than intent.min_amount_out.
    MinOutputViolation,
    /// Same intent filled by two different bundles.
    DoubleFill,
    /// Solver used a pool that doesn't exist or invalid route.
    InvalidRoute,
    /// Post-execution state root is incorrect.
    StateCorruption,
    /// Assets were created or destroyed (conservation violation).
    ConservationViolation,
}

impl FraudProofType {
    /// Whether this proof type can be auto-verified on-chain.
    pub fn is_auto_verifiable(&self) -> bool {
        matches!(self,
            Self::InvalidReveal |
            Self::FeeViolation |
            Self::MinOutputViolation |
            Self::DoubleFill |
            Self::InvalidRoute
        )
    }

    /// Slash basis points for this fraud type.
    pub fn slash_bps(&self) -> u64 {
        match self {
            Self::InvalidReveal => 10_000,      // 100%
            Self::FeeViolation => 500,           // 5%
            Self::MinOutputViolation => 1_000,   // 10%
            Self::DoubleFill => 10_000,          // 100%
            Self::InvalidRoute => 2_500,         // 25%
            Self::StateCorruption => 5_000,      // 50% (governance-resolved)
            Self::ConservationViolation => 5_000, // 50%
        }
    }
}

/// A submitted fraud proof.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FraudProof {
    pub proof_id: [u8; 32],
    pub proof_type: FraudProofType,
    pub challenger: [u8; 32],
    pub receipt_id: [u8; 32],
    pub evidence: Vec<u8>,
    pub bond_amount: u128,
    pub submitted_at_block: u64,
    pub signature: Vec<u8>,
}

impl FraudProof {
    /// Compute deterministic proof_id.
    pub fn compute_proof_id(challenger: &[u8; 32], receipt_id: &[u8; 32], proof_type: &FraudProofType) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(challenger);
        hasher.update(receipt_id);
        let type_bytes = bincode::serialize(proof_type).unwrap_or_default();
        hasher.update(&type_bytes);
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Verify the proof_id matches computed value.
    pub fn verify_proof_id(&self) -> bool {
        self.proof_id == Self::compute_proof_id(&self.challenger, &self.receipt_id, &self.proof_type)
    }
}

/// State of a dispute.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum DisputeState {
    Pending,
    Accepted,
    Rejected,
    Appealed,
}

/// Record of a dispute and its resolution.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DisputeRecord {
    pub proof: FraudProof,
    pub state: DisputeState,
    pub resolved_at_block: Option<u64>,
    pub challenger_reward: u128,
    pub solver_slash_amount: u128,
    pub resolution_reason: Option<String>,
}

impl DisputeRecord {
    pub fn new(proof: FraudProof) -> Self {
        DisputeRecord {
            proof,
            state: DisputeState::Pending,
            resolved_at_block: None,
            challenger_reward: 0,
            solver_slash_amount: 0,
            resolution_reason: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_proof_id_deterministic() {
        let challenger = [1u8; 32];
        let receipt_id = [2u8; 32];
        let proof_type = FraudProofType::FeeViolation;

        let id1 = FraudProof::compute_proof_id(&challenger, &receipt_id, &proof_type);
        let id2 = FraudProof::compute_proof_id(&challenger, &receipt_id, &proof_type);
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_auto_verifiable() {
        assert!(FraudProofType::InvalidReveal.is_auto_verifiable());
        assert!(FraudProofType::FeeViolation.is_auto_verifiable());
        assert!(!FraudProofType::StateCorruption.is_auto_verifiable());
    }
}
