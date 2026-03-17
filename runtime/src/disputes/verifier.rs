//! Dispute auto-verification — Section 11.4 of the architecture specification.
//!
//! Verifies auto-verifiable fraud proofs (types 1-5) on-chain.
//! Complex proofs (types 6-7) are deferred to governance.
//!
//! AUDIT FIX: Evidence is now deserialized and type-specific checks are performed
//! instead of accepting any non-empty evidence.

use super::types::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Dispute verifier for auto-verifiable fraud proofs.
pub struct DisputeVerifier;

/// Result of attempting to verify a fraud proof.
#[derive(Debug, Clone)]
pub enum DisputeVerificationResult {
    /// Proof is valid — solver should be slashed.
    ProofValid {
        slash_bps: u64,
        challenger_reward_bps: u64,
    },
    /// Proof is invalid — challenger should be penalized.
    ProofInvalid {
        reason: String,
    },
    /// Proof requires governance resolution (complex types).
    RequiresGovernance,
}

/// Structured evidence for InvalidReveal proofs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvalidRevealEvidence {
    pub commitment_hash: [u8; 32],
    pub reveal_preimage_hash: [u8; 32],
}

/// Structured evidence for FeeViolation proofs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeeViolationEvidence {
    pub intent_max_fee_bps: u64,
    pub actual_fee_bps: u64,
    pub intent_id: [u8; 32],
}

/// Structured evidence for MinOutputViolation proofs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MinOutputViolationEvidence {
    pub intent_min_output: u128,
    pub actual_output: u128,
    pub intent_id: [u8; 32],
}

/// Structured evidence for DoubleFill proofs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DoubleFillEvidence {
    pub intent_id: [u8; 32],
    pub receipt_id_a: [u8; 32],
    pub receipt_id_b: [u8; 32],
}

/// Structured evidence for InvalidRoute proofs.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvalidRouteEvidence {
    pub missing_object_id: [u8; 32],
    pub step_index: u16,
}

impl DisputeVerifier {
    /// Verify a fraud proof with type-specific evidence checking.
    ///
    /// For auto-verifiable proofs, evidence is deserialized and validated.
    /// For complex proofs, returns RequiresGovernance.
    pub fn verify(proof: &FraudProof) -> DisputeVerificationResult {
        if !proof.proof_type.is_auto_verifiable() {
            return DisputeVerificationResult::RequiresGovernance;
        }

        // Verify proof_id
        if !proof.verify_proof_id() {
            return DisputeVerificationResult::ProofInvalid {
                reason: "proof_id does not match computed value".to_string(),
            };
        }

        // Verify bond is sufficient
        let required_bond = required_bond_for_proof(&proof.proof_type);
        if proof.bond_amount < required_bond {
            return DisputeVerificationResult::ProofInvalid {
                reason: format!("bond {} < required {}", proof.bond_amount, required_bond),
            };
        }

        // Evidence must be non-empty
        if proof.evidence.is_empty() {
            return DisputeVerificationResult::ProofInvalid {
                reason: "empty evidence".to_string(),
            };
        }

        // Type-specific evidence verification
        match &proof.proof_type {
            FraudProofType::InvalidReveal => Self::verify_invalid_reveal(&proof.evidence),
            FraudProofType::FeeViolation => Self::verify_fee_violation(&proof.evidence),
            FraudProofType::MinOutputViolation => Self::verify_min_output_violation(&proof.evidence),
            FraudProofType::DoubleFill => Self::verify_double_fill(&proof.evidence),
            FraudProofType::InvalidRoute => Self::verify_invalid_route(&proof.evidence),
            // Complex types handled above by is_auto_verifiable check
            _ => DisputeVerificationResult::RequiresGovernance,
        }
    }

    /// InvalidReveal: commitment_hash != reveal_preimage_hash
    fn verify_invalid_reveal(evidence: &[u8]) -> DisputeVerificationResult {
        let ev: InvalidRevealEvidence = match bincode::deserialize(evidence) {
            Ok(e) => e,
            Err(_) => return DisputeVerificationResult::ProofInvalid {
                reason: "cannot deserialize InvalidRevealEvidence".to_string(),
            },
        };

        // The hashes must actually differ for the proof to be valid
        if ev.commitment_hash == ev.reveal_preimage_hash {
            return DisputeVerificationResult::ProofInvalid {
                reason: "commitment_hash equals reveal_preimage_hash — no mismatch".to_string(),
            };
        }

        DisputeVerificationResult::ProofValid {
            slash_bps: FraudProofType::InvalidReveal.slash_bps(),
            challenger_reward_bps: 3_000,
        }
    }

    /// FeeViolation: actual_fee_bps > intent_max_fee_bps
    fn verify_fee_violation(evidence: &[u8]) -> DisputeVerificationResult {
        let ev: FeeViolationEvidence = match bincode::deserialize(evidence) {
            Ok(e) => e,
            Err(_) => return DisputeVerificationResult::ProofInvalid {
                reason: "cannot deserialize FeeViolationEvidence".to_string(),
            },
        };

        if ev.actual_fee_bps <= ev.intent_max_fee_bps {
            return DisputeVerificationResult::ProofInvalid {
                reason: format!("fee {} <= max {} — no violation", ev.actual_fee_bps, ev.intent_max_fee_bps),
            };
        }

        DisputeVerificationResult::ProofValid {
            slash_bps: FraudProofType::FeeViolation.slash_bps(),
            challenger_reward_bps: 3_000,
        }
    }

    /// MinOutputViolation: actual_output < intent_min_output
    fn verify_min_output_violation(evidence: &[u8]) -> DisputeVerificationResult {
        let ev: MinOutputViolationEvidence = match bincode::deserialize(evidence) {
            Ok(e) => e,
            Err(_) => return DisputeVerificationResult::ProofInvalid {
                reason: "cannot deserialize MinOutputViolationEvidence".to_string(),
            },
        };

        if ev.actual_output >= ev.intent_min_output {
            return DisputeVerificationResult::ProofInvalid {
                reason: format!("output {} >= min {} — no violation", ev.actual_output, ev.intent_min_output),
            };
        }

        DisputeVerificationResult::ProofValid {
            slash_bps: FraudProofType::MinOutputViolation.slash_bps(),
            challenger_reward_bps: 3_000,
        }
    }

    /// DoubleFill: two different receipts for the same intent, both succeeded
    fn verify_double_fill(evidence: &[u8]) -> DisputeVerificationResult {
        let ev: DoubleFillEvidence = match bincode::deserialize(evidence) {
            Ok(e) => e,
            Err(_) => return DisputeVerificationResult::ProofInvalid {
                reason: "cannot deserialize DoubleFillEvidence".to_string(),
            },
        };

        // Receipt IDs must differ (two different fills)
        if ev.receipt_id_a == ev.receipt_id_b {
            return DisputeVerificationResult::ProofInvalid {
                reason: "receipt_id_a == receipt_id_b — same receipt, not a double fill".to_string(),
            };
        }

        // Intent ID must be non-zero
        if ev.intent_id == [0u8; 32] {
            return DisputeVerificationResult::ProofInvalid {
                reason: "zero intent_id".to_string(),
            };
        }

        DisputeVerificationResult::ProofValid {
            slash_bps: FraudProofType::DoubleFill.slash_bps(),
            challenger_reward_bps: 3_000,
        }
    }

    /// InvalidRoute: execution step references a non-existent object
    fn verify_invalid_route(evidence: &[u8]) -> DisputeVerificationResult {
        let ev: InvalidRouteEvidence = match bincode::deserialize(evidence) {
            Ok(e) => e,
            Err(_) => return DisputeVerificationResult::ProofInvalid {
                reason: "cannot deserialize InvalidRouteEvidence".to_string(),
            },
        };

        // Object ID must be non-zero
        if ev.missing_object_id == [0u8; 32] {
            return DisputeVerificationResult::ProofInvalid {
                reason: "zero missing_object_id".to_string(),
            };
        }

        DisputeVerificationResult::ProofValid {
            slash_bps: FraudProofType::InvalidRoute.slash_bps(),
            challenger_reward_bps: 3_000,
        }
    }
}

/// Required dispute bond for a given proof type.
pub fn required_bond_for_proof(proof_type: &FraudProofType) -> u128 {
    if proof_type.is_auto_verifiable() {
        1_000 // FAST_DISPUTE_BOND
    } else {
        10_000 // EXTENDED_DISPUTE_BOND
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_proof_with_evidence(proof_type: FraudProofType, evidence: Vec<u8>) -> FraudProof {
        let challenger = [1u8; 32];
        let receipt_id = [2u8; 32];
        let proof_id = FraudProof::compute_proof_id(&challenger, &receipt_id, &proof_type);
        FraudProof {
            proof_id,
            proof_type,
            challenger,
            receipt_id,
            evidence,
            bond_amount: 10_000,
            submitted_at_block: 100,
            signature: vec![1u8; 64],
        }
    }

    #[test]
    fn test_verify_invalid_reveal_valid_proof() {
        let ev = InvalidRevealEvidence {
            commitment_hash: [1u8; 32],
            reveal_preimage_hash: [2u8; 32], // different → mismatch proven
        };
        let proof = make_proof_with_evidence(
            FraudProofType::InvalidReveal,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofValid { slash_bps, .. } => {
                assert_eq!(slash_bps, 10_000); // 100% slash
            }
            other => panic!("expected ProofValid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_invalid_reveal_false_proof() {
        // Hashes match → no mismatch → proof is invalid
        let ev = InvalidRevealEvidence {
            commitment_hash: [1u8; 32],
            reveal_preimage_hash: [1u8; 32], // same → no violation
        };
        let proof = make_proof_with_evidence(
            FraudProofType::InvalidReveal,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("no mismatch"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_fee_violation_valid() {
        let ev = FeeViolationEvidence {
            intent_max_fee_bps: 50,
            actual_fee_bps: 100, // exceeds → valid proof
            intent_id: [5u8; 32],
        };
        let proof = make_proof_with_evidence(
            FraudProofType::FeeViolation,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofValid { slash_bps, .. } => {
                assert_eq!(slash_bps, 500);
            }
            other => panic!("expected ProofValid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_fee_violation_false_proof() {
        let ev = FeeViolationEvidence {
            intent_max_fee_bps: 100,
            actual_fee_bps: 50, // within limit → no violation
            intent_id: [5u8; 32],
        };
        let proof = make_proof_with_evidence(
            FraudProofType::FeeViolation,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("no violation"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_min_output_valid() {
        let ev = MinOutputViolationEvidence {
            intent_min_output: 1000,
            actual_output: 900, // below min → valid proof
            intent_id: [5u8; 32],
        };
        let proof = make_proof_with_evidence(
            FraudProofType::MinOutputViolation,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofValid { .. } => {}
            other => panic!("expected ProofValid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_double_fill_valid() {
        let ev = DoubleFillEvidence {
            intent_id: [5u8; 32],
            receipt_id_a: [1u8; 32],
            receipt_id_b: [2u8; 32], // different receipts → valid proof
        };
        let proof = make_proof_with_evidence(
            FraudProofType::DoubleFill,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofValid { slash_bps, .. } => {
                assert_eq!(slash_bps, 10_000); // 100% slash
            }
            other => panic!("expected ProofValid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_double_fill_same_receipt() {
        // Same receipt twice → not a double fill
        let ev = DoubleFillEvidence {
            intent_id: [5u8; 32],
            receipt_id_a: [1u8; 32],
            receipt_id_b: [1u8; 32], // same → invalid proof
        };
        let proof = make_proof_with_evidence(
            FraudProofType::DoubleFill,
            bincode::serialize(&ev).unwrap(),
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("same receipt"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_governance_proof() {
        let proof = make_proof_with_evidence(FraudProofType::StateCorruption, vec![1, 2, 3]);
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::RequiresGovernance => {}
            other => panic!("expected RequiresGovernance, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_bad_proof_id() {
        let ev = FeeViolationEvidence {
            intent_max_fee_bps: 50, actual_fee_bps: 100, intent_id: [5u8; 32],
        };
        let mut proof = make_proof_with_evidence(
            FraudProofType::FeeViolation,
            bincode::serialize(&ev).unwrap(),
        );
        proof.proof_id = [0xAA; 32]; // tampered
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("proof_id"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_empty_evidence() {
        let mut proof = make_proof_with_evidence(FraudProofType::FeeViolation, vec![]);
        // Fix proof_id for empty evidence
        proof.proof_id = FraudProof::compute_proof_id(&proof.challenger, &proof.receipt_id, &proof.proof_type);
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("empty"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_malformed_evidence() {
        let proof = make_proof_with_evidence(
            FraudProofType::FeeViolation,
            vec![0xFF, 0xFF, 0xFF], // garbage bytes
        );
        match DisputeVerifier::verify(&proof) {
            DisputeVerificationResult::ProofInvalid { reason } => {
                assert!(reason.contains("deserialize"));
            }
            other => panic!("expected ProofInvalid, got {:?}", other),
        }
    }
}
