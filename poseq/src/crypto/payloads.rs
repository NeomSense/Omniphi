#![allow(dead_code)]

use sha2::{Sha256, Digest};

// Domain tags — each exactly 32 bytes.
// "POSEQ_PROPOSAL_V1"  = 17 chars → 15 null bytes
pub const DOMAIN_TAG_PROPOSAL: [u8; 32] = *b"POSEQ_PROPOSAL_V1\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0";
// "POSEQ_ATTEST_V1"    = 15 chars → 17 null bytes
pub const DOMAIN_TAG_ATTESTATION: [u8; 32] = *b"POSEQ_ATTEST_V1\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0";
// "POSEQ_EVIDENCE_V1"  = 17 chars → 15 null bytes
pub const DOMAIN_TAG_EVIDENCE: [u8; 32] = *b"POSEQ_EVIDENCE_V1\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0";
// "POSEQ_FINALIZED_V1" = 18 chars → 14 null bytes
pub const DOMAIN_TAG_FINALIZED: [u8; 32] = *b"POSEQ_FINALIZED_V1\0\0\0\0\0\0\0\0\0\0\0\0\0\0";

/// Signing payload for a leader's batch proposal.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProposalPayload {
    pub leader_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub batch_root: [u8; 32],
    pub submission_count: u32,
}

impl ProposalPayload {
    pub fn to_payload_hash(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&DOMAIN_TAG_PROPOSAL);
        h.update(&self.leader_id);
        h.update(&self.slot.to_be_bytes());
        h.update(&self.epoch.to_be_bytes());
        h.update(&self.batch_root);
        h.update(&self.submission_count.to_be_bytes());
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

/// Signing payload for an attestor's vote on a batch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AttestationPayload {
    pub attestor_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub batch_id: [u8; 32],
    pub vote_accept: bool,
}

impl AttestationPayload {
    pub fn to_payload_hash(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&DOMAIN_TAG_ATTESTATION);
        h.update(&self.attestor_id);
        h.update(&self.slot.to_be_bytes());
        h.update(&self.epoch.to_be_bytes());
        h.update(&self.batch_id);
        h.update(&[self.vote_accept as u8]);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

/// Signing payload for a double-voting evidence report.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EvidencePayload {
    pub reporter_id: [u8; 32],
    pub accused_id: [u8; 32],
    pub slot_a: u64,
    pub epoch_a: u64,
    pub batch_id_a: [u8; 32],
    pub slot_b: u64,
    pub epoch_b: u64,
    pub batch_id_b: [u8; 32],
}

impl EvidencePayload {
    pub fn to_payload_hash(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&DOMAIN_TAG_EVIDENCE);
        h.update(&self.reporter_id);
        h.update(&self.accused_id);
        h.update(&self.slot_a.to_be_bytes());
        h.update(&self.epoch_a.to_be_bytes());
        h.update(&self.batch_id_a);
        h.update(&self.slot_b.to_be_bytes());
        h.update(&self.epoch_b.to_be_bytes());
        h.update(&self.batch_id_b);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

/// Signing payload for a finalized-batch certificate.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FinalizedBatchPayload {
    pub finalizer_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub batch_id: [u8; 32],
    pub finalization_hash: [u8; 32],
}

impl FinalizedBatchPayload {
    pub fn to_payload_hash(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&DOMAIN_TAG_FINALIZED);
        h.update(&self.finalizer_id);
        h.update(&self.slot.to_be_bytes());
        h.update(&self.epoch.to_be_bytes());
        h.update(&self.batch_id);
        h.update(&self.finalization_hash);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn sample_proposal() -> ProposalPayload {
        ProposalPayload {
            leader_id: make_id(1),
            slot: 10,
            epoch: 2,
            batch_root: make_id(42),
            submission_count: 5,
        }
    }

    fn sample_attestation() -> AttestationPayload {
        AttestationPayload {
            attestor_id: make_id(1),
            slot: 10,
            epoch: 2,
            batch_id: make_id(42),
            vote_accept: true,
        }
    }

    fn sample_evidence() -> EvidencePayload {
        EvidencePayload {
            reporter_id: make_id(1),
            accused_id: make_id(2),
            slot_a: 10,
            epoch_a: 2,
            batch_id_a: make_id(42),
            slot_b: 11,
            epoch_b: 2,
            batch_id_b: make_id(43),
        }
    }

    fn sample_finalized() -> FinalizedBatchPayload {
        FinalizedBatchPayload {
            finalizer_id: make_id(1),
            slot: 10,
            epoch: 2,
            batch_id: make_id(42),
            finalization_hash: make_id(99),
        }
    }

    // ── Domain tag length sanity ────────────────────────────────────────────

    #[test]
    fn test_domain_tags_are_32_bytes() {
        assert_eq!(DOMAIN_TAG_PROPOSAL.len(), 32);
        assert_eq!(DOMAIN_TAG_ATTESTATION.len(), 32);
        assert_eq!(DOMAIN_TAG_EVIDENCE.len(), 32);
        assert_eq!(DOMAIN_TAG_FINALIZED.len(), 32);
    }

    // ── Determinism ─────────────────────────────────────────────────────────

    #[test]
    fn test_proposal_hash_is_deterministic() {
        let p = sample_proposal();
        assert_eq!(p.to_payload_hash(), p.to_payload_hash());
    }

    #[test]
    fn test_attestation_hash_is_deterministic() {
        let p = sample_attestation();
        assert_eq!(p.to_payload_hash(), p.to_payload_hash());
    }

    #[test]
    fn test_evidence_hash_is_deterministic() {
        let p = sample_evidence();
        assert_eq!(p.to_payload_hash(), p.to_payload_hash());
    }

    #[test]
    fn test_finalized_hash_is_deterministic() {
        let p = sample_finalized();
        assert_eq!(p.to_payload_hash(), p.to_payload_hash());
    }

    // ── Domain separation ───────────────────────────────────────────────────

    #[test]
    fn test_domain_separation_proposal_vs_attestation() {
        // Construct proposal and attestation sharing the same numeric fields where possible.
        let p = ProposalPayload {
            leader_id: make_id(1),
            slot: 10,
            epoch: 2,
            batch_root: make_id(42),
            submission_count: 5,
        };
        let a = AttestationPayload {
            attestor_id: make_id(1),
            slot: 10,
            epoch: 2,
            batch_id: make_id(42),
            vote_accept: true,
        };
        assert_ne!(p.to_payload_hash(), a.to_payload_hash());
    }

    #[test]
    fn test_domain_separation_evidence_vs_finalized() {
        let e = sample_evidence();
        let f = sample_finalized();
        assert_ne!(e.to_payload_hash(), f.to_payload_hash());
    }

    #[test]
    fn test_all_four_types_produce_distinct_hashes() {
        let h1 = sample_proposal().to_payload_hash();
        let h2 = sample_attestation().to_payload_hash();
        let h3 = sample_evidence().to_payload_hash();
        let h4 = sample_finalized().to_payload_hash();
        // All four must differ.
        assert_ne!(h1, h2);
        assert_ne!(h1, h3);
        assert_ne!(h1, h4);
        assert_ne!(h2, h3);
        assert_ne!(h2, h4);
        assert_ne!(h3, h4);
    }

    // ── Field sensitivity ───────────────────────────────────────────────────

    #[test]
    fn test_proposal_field_sensitivity_leader_id() {
        let mut p = sample_proposal();
        let h1 = p.to_payload_hash();
        p.leader_id[0] ^= 1;
        assert_ne!(h1, p.to_payload_hash());
    }

    #[test]
    fn test_proposal_field_sensitivity_slot() {
        let mut p = sample_proposal();
        let h1 = p.to_payload_hash();
        p.slot += 1;
        assert_ne!(h1, p.to_payload_hash());
    }

    #[test]
    fn test_proposal_field_sensitivity_epoch() {
        let mut p = sample_proposal();
        let h1 = p.to_payload_hash();
        p.epoch += 1;
        assert_ne!(h1, p.to_payload_hash());
    }

    #[test]
    fn test_proposal_field_sensitivity_batch_root() {
        let mut p = sample_proposal();
        let h1 = p.to_payload_hash();
        p.batch_root[0] ^= 1;
        assert_ne!(h1, p.to_payload_hash());
    }

    #[test]
    fn test_proposal_field_sensitivity_submission_count() {
        let mut p = sample_proposal();
        let h1 = p.to_payload_hash();
        p.submission_count += 1;
        assert_ne!(h1, p.to_payload_hash());
    }

    #[test]
    fn test_attestation_field_sensitivity_vote_accept() {
        let mut a = sample_attestation();
        let h1 = a.to_payload_hash();
        a.vote_accept = false;
        assert_ne!(h1, a.to_payload_hash());
    }

    #[test]
    fn test_attestation_field_sensitivity_batch_id() {
        let mut a = sample_attestation();
        let h1 = a.to_payload_hash();
        a.batch_id[0] ^= 1;
        assert_ne!(h1, a.to_payload_hash());
    }

    #[test]
    fn test_evidence_field_sensitivity_accused_id() {
        let mut e = sample_evidence();
        let h1 = e.to_payload_hash();
        e.accused_id[0] ^= 1;
        assert_ne!(h1, e.to_payload_hash());
    }

    #[test]
    fn test_evidence_field_sensitivity_slot_b() {
        let mut e = sample_evidence();
        let h1 = e.to_payload_hash();
        e.slot_b += 1;
        assert_ne!(h1, e.to_payload_hash());
    }

    #[test]
    fn test_finalized_field_sensitivity_finalization_hash() {
        let mut f = sample_finalized();
        let h1 = f.to_payload_hash();
        f.finalization_hash[0] ^= 1;
        assert_ne!(h1, f.to_payload_hash());
    }
}
