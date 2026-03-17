//! Phase 3 — Finality Commitment bridging PoSeq ordering with HotStuff consensus.
//!
//! FinalityCommitment is the canonical structure that validators vote on.
//! It binds the auction ordering output, intent references, fairness log,
//! and DA checks into a single deterministic commitment root.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// The finality commitment that validators vote on during HotStuff consensus.
///
/// This structure bridges PoSeq ordering output with HotStuff consensus.
/// Validators must verify all fields before casting their vote.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FinalityCommitment {
    /// Monotonic sequence identifier for this batch.
    pub sequence_id: u64,
    /// Bundle IDs in canonical execution order.
    pub ordered_bundle_ids: Vec<[u8; 32]>,
    /// Intent IDs referenced by all bundles.
    pub intent_ids: Vec<[u8; 32]>,
    /// SHA256 root over the ordered bundle sequence (from auction/ordering).
    pub ordering_root: [u8; 32],
    /// SHA256 root over all bundle commitment hashes.
    pub bundle_root: [u8; 32],
    /// Merkle root over fairness log entries.
    pub fairness_root: [u8; 32],
    /// Commitment root of the previous finalized sequence.
    pub previous_commitment: [u8; 32],
    /// Validator proposing this commitment.
    pub proposer: [u8; 32],
    /// Block height at time of proposal.
    pub timestamp: u64,
}

impl FinalityCommitment {
    /// Compute the deterministic commitment root.
    ///
    /// commitment_root = SHA256(
    ///     sequence_id ‖ ordering_root ‖ bundle_root ‖ fairness_root ‖ previous_commitment
    /// )
    pub fn commitment_root(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.sequence_id.to_be_bytes());
        hasher.update(self.ordering_root);
        hasher.update(self.bundle_root);
        hasher.update(self.fairness_root);
        hasher.update(self.previous_commitment);
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// Compute the bundle root from a list of bundle commitment hashes.
    ///
    /// bundle_root = SHA256(commitment_hash[0] ‖ ... ‖ commitment_hash[n])
    pub fn compute_bundle_root(commitment_hashes: &[[u8; 32]]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for hash in commitment_hashes {
            hasher.update(hash);
        }
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// Verify internal consistency: ordering_root and bundle_root match the declared lists.
    pub fn verify_roots(
        &self,
        expected_ordering_root: [u8; 32],
        expected_bundle_root: [u8; 32],
    ) -> Result<(), CommitmentVerificationError> {
        if self.ordering_root != expected_ordering_root {
            return Err(CommitmentVerificationError::OrderingRootMismatch {
                expected: expected_ordering_root,
                actual: self.ordering_root,
            });
        }
        if self.bundle_root != expected_bundle_root {
            return Err(CommitmentVerificationError::BundleRootMismatch {
                expected: expected_bundle_root,
                actual: self.bundle_root,
            });
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CommitmentVerificationError {
    OrderingRootMismatch { expected: [u8; 32], actual: [u8; 32] },
    BundleRootMismatch { expected: [u8; 32], actual: [u8; 32] },
    FairnessRootMismatch { expected: [u8; 32], actual: [u8; 32] },
    MissingBundle([u8; 32]),
    MissingIntent([u8; 32]),
    DuplicateBundle([u8; 32]),
    DACheckFailed(String),
    InvalidPreviousCommitment,
    SequenceGap { expected: u64, actual: u64 },
    InvalidProposer([u8; 32]),
}

impl std::fmt::Display for CommitmentVerificationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::OrderingRootMismatch { expected, actual } => {
                write!(f, "ordering root mismatch: expected {}, got {}",
                    hex::encode(&expected[..4]), hex::encode(&actual[..4]))
            }
            Self::BundleRootMismatch { expected, actual } => {
                write!(f, "bundle root mismatch: expected {}, got {}",
                    hex::encode(&expected[..4]), hex::encode(&actual[..4]))
            }
            Self::FairnessRootMismatch { expected, actual } => {
                write!(f, "fairness root mismatch: expected {}, got {}",
                    hex::encode(&expected[..4]), hex::encode(&actual[..4]))
            }
            Self::MissingBundle(id) => write!(f, "missing bundle {}", hex::encode(&id[..4])),
            Self::MissingIntent(id) => write!(f, "missing intent {}", hex::encode(&id[..4])),
            Self::DuplicateBundle(id) => write!(f, "duplicate bundle {}", hex::encode(&id[..4])),
            Self::DACheckFailed(msg) => write!(f, "DA check failed: {}", msg),
            Self::InvalidPreviousCommitment => write!(f, "invalid previous commitment"),
            Self::SequenceGap { expected, actual } => {
                write!(f, "sequence gap: expected {}, got {}", expected, actual)
            }
            Self::InvalidProposer(id) => write!(f, "invalid proposer {}", hex::encode(&id[..4])),
        }
    }
}

impl std::error::Error for CommitmentVerificationError {}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_commitment(seq_id: u64) -> FinalityCommitment {
        FinalityCommitment {
            sequence_id: seq_id,
            ordered_bundle_ids: vec![[1u8; 32], [2u8; 32]],
            intent_ids: vec![[10u8; 32], [20u8; 32]],
            ordering_root: [0xAA; 32],
            bundle_root: [0xBB; 32],
            fairness_root: [0xCC; 32],
            previous_commitment: [0u8; 32],
            proposer: [0xFF; 32],
            timestamp: 100,
        }
    }

    #[test]
    fn test_commitment_root_deterministic() {
        let c = make_commitment(1);
        let root1 = c.commitment_root();
        let root2 = c.commitment_root();
        assert_eq!(root1, root2);
        assert_ne!(root1, [0u8; 32]); // non-trivial
    }

    #[test]
    fn test_commitment_root_changes_with_sequence_id() {
        let c1 = make_commitment(1);
        let c2 = make_commitment(2);
        assert_ne!(c1.commitment_root(), c2.commitment_root());
    }

    #[test]
    fn test_commitment_root_changes_with_ordering_root() {
        let mut c1 = make_commitment(1);
        let mut c2 = make_commitment(1);
        c2.ordering_root = [0xDD; 32];
        assert_ne!(c1.commitment_root(), c2.commitment_root());
    }

    #[test]
    fn test_bundle_root_computation() {
        let hashes = vec![[1u8; 32], [2u8; 32], [3u8; 32]];
        let root1 = FinalityCommitment::compute_bundle_root(&hashes);
        let root2 = FinalityCommitment::compute_bundle_root(&hashes);
        assert_eq!(root1, root2);

        // Different order → different root
        let hashes_rev = vec![[3u8; 32], [2u8; 32], [1u8; 32]];
        assert_ne!(root1, FinalityCommitment::compute_bundle_root(&hashes_rev));
    }

    #[test]
    fn test_verify_roots_success() {
        let c = make_commitment(1);
        assert!(c.verify_roots([0xAA; 32], [0xBB; 32]).is_ok());
    }

    #[test]
    fn test_verify_roots_ordering_mismatch() {
        let c = make_commitment(1);
        match c.verify_roots([0xFF; 32], [0xBB; 32]) {
            Err(CommitmentVerificationError::OrderingRootMismatch { .. }) => {}
            other => panic!("expected OrderingRootMismatch, got {:?}", other),
        }
    }

    #[test]
    fn test_chain_of_commitments() {
        let c1 = make_commitment(1);
        let root1 = c1.commitment_root();

        let mut c2 = make_commitment(2);
        c2.previous_commitment = root1;
        let root2 = c2.commitment_root();

        assert_ne!(root1, root2);
        assert_eq!(c2.previous_commitment, root1);
    }
}
