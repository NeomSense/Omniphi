use sha2::{Sha256, Digest};
use crate::proposals::batch::ProposedBatch;
use crate::attestations::collector::AttestationQuorumResult;

/// Incremental SHA256-based builder for batch commitments.
pub struct BatchHashBuilder {
    submission_ids: Vec<[u8; 32]>,
    header_fields: Vec<(String, Vec<u8>)>, // (key, value_bytes)
}

impl BatchHashBuilder {
    pub fn new() -> Self {
        BatchHashBuilder {
            submission_ids: Vec::new(),
            header_fields: Vec::new(),
        }
    }

    pub fn add_submission_id(&mut self, id: [u8; 32]) -> &mut Self {
        self.submission_ids.push(id);
        self
    }

    pub fn add_header_field(&mut self, key: &str, value: &[u8]) -> &mut Self {
        self.header_fields.push((key.to_string(), value.to_vec()));
        self
    }

    pub fn finalize(self) -> CanonicalBatchDigest {
        let mut sorted_ids = self.submission_ids.clone();
        sorted_ids.sort();

        let mut hasher = Sha256::new();
        for id in &sorted_ids {
            hasher.update(id);
        }
        let subs_result = hasher.finalize();
        let mut submissions_root = [0u8; 32];
        submissions_root.copy_from_slice(&subs_result);

        // Sort header fields by key for determinism
        let mut fields = self.header_fields.clone();
        fields.sort_by(|a, b| a.0.cmp(&b.0));

        let mut hasher2 = Sha256::new();
        for (key, val) in &fields {
            hasher2.update(key.as_bytes());
            hasher2.update(val);
        }
        let header_result = hasher2.finalize();
        let mut header_hash = [0u8; 32];
        header_hash.copy_from_slice(&header_result);

        let mut hasher3 = Sha256::new();
        hasher3.update(&submissions_root);
        hasher3.update(&header_hash);
        let full_result = hasher3.finalize();
        let mut digest = [0u8; 32];
        digest.copy_from_slice(&full_result);

        CanonicalBatchDigest {
            submissions_root,
            header_hash,
            digest,
        }
    }
}

impl Default for BatchHashBuilder {
    fn default() -> Self {
        Self::new()
    }
}

/// Output of BatchHashBuilder::finalize().
#[derive(Debug, Clone)]
pub struct CanonicalBatchDigest {
    pub submissions_root: [u8; 32],
    pub header_hash: [u8; 32],
    pub digest: [u8; 32],
}

/// Full commitment record for a finalized batch.
#[derive(Debug, Clone)]
pub struct BatchCommitment {
    pub ordered_submissions_root: [u8; 32],
    pub header_hash: [u8; 32],
    pub policy_version: u32,
    pub parent_reference: [u8; 32],
    pub attestation_summary_hash: [u8; 32],
    pub commitment_hash: [u8; 32],
}

impl BatchCommitment {
    /// Compute a BatchCommitment from a ProposedBatch and the quorum result.
    pub fn compute(proposal: &ProposedBatch, quorum: &AttestationQuorumResult) -> Self {
        // Compute ordered submissions root
        let ordered_submissions_root = ProposedBatch::compute_root(&proposal.ordered_submission_ids);

        // Compute header hash
        let mut header_hasher = Sha256::new();
        header_hasher.update(&proposal.slot.to_be_bytes());
        header_hasher.update(&proposal.epoch.to_be_bytes());
        header_hasher.update(&proposal.leader_id);
        header_hasher.update(&proposal.policy_version.to_be_bytes());
        let header_result = header_hasher.finalize();
        let mut header_hash = [0u8; 32];
        header_hash.copy_from_slice(&header_result);

        // Attestation summary hash = quorum_hash
        let attestation_summary_hash = quorum.quorum_hash;

        // Final commitment hash
        let mut commitment_hasher = Sha256::new();
        commitment_hasher.update(&ordered_submissions_root);
        commitment_hasher.update(&header_hash);
        commitment_hasher.update(&proposal.policy_version.to_be_bytes());
        commitment_hasher.update(&proposal.parent_batch_id);
        commitment_hasher.update(&attestation_summary_hash);
        let commitment_result = commitment_hasher.finalize();
        let mut commitment_hash = [0u8; 32];
        commitment_hash.copy_from_slice(&commitment_result);

        BatchCommitment {
            ordered_submissions_root,
            header_hash,
            policy_version: proposal.policy_version,
            parent_reference: proposal.parent_batch_id,
            attestation_summary_hash,
            commitment_hash,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::proposals::batch::ProposedBatch;
    use crate::attestations::collector::AttestationQuorumResult;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_quorum() -> AttestationQuorumResult {
        AttestationQuorumResult {
            reached: true,
            approvals: 3,
            rejections: 0,
            total_votes: 3,
            quorum_hash: [7u8; 32],
        }
    }

    #[test]
    fn test_batch_commitment_determinism() {
        let batch = ProposedBatch::new(1, 1, make_id(10), vec![make_id(1), make_id(2)], [0u8; 32], 1, 100);
        let quorum = make_quorum();
        let c1 = BatchCommitment::compute(&batch, &quorum);
        let c2 = BatchCommitment::compute(&batch, &quorum);
        assert_eq!(c1.commitment_hash, c2.commitment_hash);
        assert_eq!(c1.ordered_submissions_root, c2.ordered_submissions_root);
    }

    #[test]
    fn test_batch_commitment_changes_with_different_input() {
        let b1 = ProposedBatch::new(1, 1, make_id(10), vec![make_id(1)], [0u8; 32], 1, 100);
        let b2 = ProposedBatch::new(1, 1, make_id(10), vec![make_id(2)], [0u8; 32], 1, 100);
        let quorum = make_quorum();
        let c1 = BatchCommitment::compute(&b1, &quorum);
        let c2 = BatchCommitment::compute(&b2, &quorum);
        assert_ne!(c1.commitment_hash, c2.commitment_hash);
    }

    #[test]
    fn test_hash_builder_determinism() {
        let mut b1 = BatchHashBuilder::new();
        b1.add_submission_id(make_id(1));
        b1.add_submission_id(make_id(2));
        b1.add_header_field("slot", &1u64.to_be_bytes());
        let d1 = b1.finalize();

        let mut b2 = BatchHashBuilder::new();
        b2.add_submission_id(make_id(2)); // different order
        b2.add_submission_id(make_id(1));
        b2.add_header_field("slot", &1u64.to_be_bytes());
        let d2 = b2.finalize();

        // Same submissions root despite different insertion order
        assert_eq!(d1.submissions_root, d2.submissions_root);
        assert_eq!(d1.digest, d2.digest);
    }

    #[test]
    fn test_hash_builder_changes_with_different_ids() {
        let mut b1 = BatchHashBuilder::new();
        b1.add_submission_id(make_id(1));
        let d1 = b1.finalize();

        let mut b2 = BatchHashBuilder::new();
        b2.add_submission_id(make_id(2));
        let d2 = b2.finalize();

        assert_ne!(d1.digest, d2.digest);
    }
}
