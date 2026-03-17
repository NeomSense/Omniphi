use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};
use crate::crypto::signer::SignatureEnvelope;

/// A single attestation vote from an attestor node.
#[derive(Debug, Clone)]
pub struct BatchAttestationVote {
    pub vote_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub attestor_id: [u8; 32],
    pub approved: bool,
    pub vote_hash: [u8; 32],
    pub epoch: u64,
    /// Optional cryptographic signature authenticating this vote.
    pub signature: Option<SignatureEnvelope>,
}

impl BatchAttestationVote {
    pub fn new(
        proposal_id: [u8; 32],
        attestor_id: [u8; 32],
        approved: bool,
        epoch: u64,
    ) -> Self {
        let vote_hash = Self::compute_vote_hash(proposal_id, attestor_id, approved, epoch);
        let vote_id = vote_hash; // vote_id = vote_hash for simplicity (unique per tuple)
        BatchAttestationVote {
            vote_id,
            proposal_id,
            attestor_id,
            approved,
            vote_hash,
            epoch,
            signature: None,
        }
    }

    /// Construct a vote pre-signed with the attestor's key.
    pub fn new_signed(
        proposal_id: [u8; 32],
        attestor_id: [u8; 32],
        approved: bool,
        epoch: u64,
        slot: u64,
        batch_id: [u8; 32],
        key: &crate::crypto::node_keys::NodeKeyPair,
    ) -> Result<Self, crate::errors::Phase4Error> {
        let mut vote = Self::new(proposal_id, attestor_id, approved, epoch);
        let payload = crate::crypto::payloads::AttestationPayload {
            attestor_id,
            slot,
            epoch,
            batch_id,
            vote_accept: approved,
        };
        let envelope = key.sign_attestation(&payload)?;
        vote.signature = Some(envelope);
        Ok(vote)
    }

    pub fn compute_vote_hash(
        proposal_id: [u8; 32],
        attestor_id: [u8; 32],
        approved: bool,
        epoch: u64,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&proposal_id);
        hasher.update(&attestor_id);
        hasher.update(&[approved as u8]);
        hasher.update(&epoch.to_be_bytes());
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Record of a conflicting pair of votes from the same attestor.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConflictingAttestationRecord {
    pub attestor_id: [u8; 32],
    pub first_vote_id: [u8; 32],
    pub second_vote_id: [u8; 32],
    pub conflict_type: String,
}

/// Minimum fraction threshold for quorum (in basis points, e.g. 6667 = 2/3).
#[derive(Debug, Clone)]
pub struct AttestationThreshold {
    pub min_approvals: usize,
    pub min_fraction_bps: u32,
}

impl AttestationThreshold {
    pub fn two_thirds(committee_size: usize) -> Self {
        let min_approvals = (committee_size * 2 + 2) / 3; // ceiling of 2/3
        AttestationThreshold {
            min_approvals,
            min_fraction_bps: 6667,
        }
    }
}

/// Result of a quorum check.
#[derive(Debug, Clone)]
pub struct AttestationQuorumResult {
    pub reached: bool,
    pub approvals: usize,
    pub rejections: usize,
    pub total_votes: usize,
    pub quorum_hash: [u8; 32],
}

impl AttestationQuorumResult {
    fn compute_hash(proposal_id: &[u8; 32], approvals: usize, rejections: usize, total: usize) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(proposal_id);
        hasher.update(&approvals.to_be_bytes());
        hasher.update(&rejections.to_be_bytes());
        hasher.update(&total.to_be_bytes());
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Collects votes for a single proposal and detects conflicts.
#[derive(Debug, Clone)]
pub struct AttestationCollector {
    pub proposal_id: [u8; 32],
    pub votes: BTreeMap<[u8; 32], BatchAttestationVote>, // keyed by attestor_id
    pub conflicts: Vec<ConflictingAttestationRecord>,
}

impl AttestationCollector {
    pub fn new(proposal_id: [u8; 32]) -> Self {
        AttestationCollector {
            proposal_id,
            votes: BTreeMap::new(),
            conflicts: Vec::new(),
        }
    }

    /// Add a vote from a validated committee member.
    ///
    /// FIND-017: validates that the attestor is in the active committee before accepting
    /// the vote. Returns `Err` if the attestor is not a committee member (sybil guard).
    pub fn add_vote_from_committee(
        &mut self,
        vote: BatchAttestationVote,
        committee: &BTreeSet<[u8; 32]>,
    ) -> Result<bool, ConflictingAttestationRecord> {
        if !committee.contains(&vote.attestor_id) {
            // Non-committee node — record as a conflict so callers can track the violation.
            let conflict = ConflictingAttestationRecord {
                attestor_id: vote.attestor_id,
                first_vote_id: [0u8; 32],
                second_vote_id: vote.vote_id,
                conflict_type: "NonCommitteeMember".to_string(),
            };
            self.conflicts.push(conflict.clone());
            return Err(conflict);
        }
        self.add_vote(vote)
    }

    /// Add a vote. Detects duplicate (same attestor, same approval) and
    /// conflicting votes (same attestor, different approval decision).
    /// Returns Ok(true) if added, Ok(false) if exact duplicate, Err if conflicting.
    pub fn add_vote(&mut self, vote: BatchAttestationVote) -> Result<bool, ConflictingAttestationRecord> {
        if let Some(existing) = self.votes.get(&vote.attestor_id) {
            if existing.approved == vote.approved {
                // Exact duplicate — idempotent, no conflict
                return Ok(false);
            } else {
                // Conflicting vote — same attestor voted both ways
                let conflict = ConflictingAttestationRecord {
                    attestor_id: vote.attestor_id,
                    first_vote_id: existing.vote_id,
                    second_vote_id: vote.vote_id,
                    conflict_type: "ApprovalFlip".to_string(),
                };
                self.conflicts.push(conflict.clone());
                return Err(conflict);
            }
        }
        self.votes.insert(vote.attestor_id, vote);
        Ok(true)
    }

    /// Check whether quorum has been reached.
    pub fn check_quorum(
        &self,
        threshold: &AttestationThreshold,
        committee_size: usize,
    ) -> AttestationQuorumResult {
        let approvals = self.votes.values().filter(|v| v.approved).count();
        let rejections = self.votes.values().filter(|v| !v.approved).count();
        let total_votes = self.votes.len();

        // Basis-point fraction check: approvals / committee_size >= min_fraction_bps / 10000
        // Rearranged to avoid floating point: approvals * 10000 >= min_fraction_bps * committee_size
        let fraction_ok = if committee_size == 0 {
            false
        } else {
            (approvals as u64) * 10000 >= (threshold.min_fraction_bps as u64) * (committee_size as u64)
        };
        let count_ok = approvals >= threshold.min_approvals;
        let reached = fraction_ok && count_ok;

        let quorum_hash = AttestationQuorumResult::compute_hash(
            &self.proposal_id,
            approvals,
            rejections,
            total_votes,
        );

        AttestationQuorumResult {
            reached,
            approvals,
            rejections,
            total_votes,
            quorum_hash,
        }
    }

    /// Like `add_vote_from_committee` but additionally verifies the cryptographic signature
    /// when one is present. Rejects the vote with a `ConflictingAttestationRecord` if the
    /// signature fails verification.
    ///
    /// `slot` and `batch_id` are the proposal's slot and batch identifier used to reconstruct
    /// the `AttestationPayload` for signature verification.
    pub fn add_verified_vote(
        &mut self,
        vote: BatchAttestationVote,
        committee: &BTreeSet<[u8; 32]>,
        verifier: &crate::crypto::verifier::PoSeqVerifier,
        slot: u64,
        batch_id: [u8; 32],
    ) -> Result<bool, ConflictingAttestationRecord> {
        // Committee membership check first (sybil guard).
        if !committee.contains(&vote.attestor_id) {
            let conflict = ConflictingAttestationRecord {
                attestor_id: vote.attestor_id,
                first_vote_id: [0u8; 32],
                second_vote_id: vote.vote_id,
                conflict_type: "NonCommitteeMember".to_string(),
            };
            self.conflicts.push(conflict.clone());
            return Err(conflict);
        }

        // Cryptographic signature check (if a signature is present).
        if let Some(ref env) = vote.signature {
            let payload = crate::crypto::payloads::AttestationPayload {
                attestor_id: vote.attestor_id,
                slot,
                epoch: vote.epoch,
                batch_id,
                vote_accept: vote.approved,
            };
            if verifier.verify_attestation(&payload, env).is_err() {
                let conflict = ConflictingAttestationRecord {
                    attestor_id: vote.attestor_id,
                    first_vote_id: [0u8; 32],
                    second_vote_id: vote.vote_id,
                    conflict_type: "InvalidSignature".to_string(),
                };
                self.conflicts.push(conflict.clone());
                return Err(conflict);
            }
        }

        self.add_vote(vote)
    }

    pub fn has_conflicts(&self) -> bool {
        !self.conflicts.is_empty()
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

    fn make_vote(proposal: [u8; 32], attestor: u8, approved: bool) -> BatchAttestationVote {
        BatchAttestationVote::new(proposal, make_id(attestor), approved, 1)
    }

    #[test]
    fn test_quorum_reached() {
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        for i in 1..=4u8 {
            let vote = make_vote(proposal_id, i, true);
            collector.add_vote(vote).unwrap();
        }
        let threshold = AttestationThreshold::two_thirds(5);
        let result = collector.check_quorum(&threshold, 5);
        assert!(result.reached);
        assert_eq!(result.approvals, 4);
    }

    #[test]
    fn test_quorum_not_reached() {
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        // Only 2 out of 6 — not enough
        for i in 1..=2u8 {
            let vote = make_vote(proposal_id, i, true);
            collector.add_vote(vote).unwrap();
        }
        let threshold = AttestationThreshold::two_thirds(6);
        let result = collector.check_quorum(&threshold, 6);
        assert!(!result.reached);
    }

    #[test]
    fn test_duplicate_vote_ignored() {
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        let v1 = make_vote(proposal_id, 1, true);
        let v2 = make_vote(proposal_id, 1, true);
        assert_eq!(collector.add_vote(v1), Ok(true));
        assert_eq!(collector.add_vote(v2), Ok(false)); // duplicate, not added again
        assert_eq!(collector.votes.len(), 1);
    }

    #[test]
    fn test_conflicting_vote_detected() {
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        let v_approve = make_vote(proposal_id, 1, true);
        let v_reject = make_vote(proposal_id, 1, false);
        assert!(collector.add_vote(v_approve).is_ok());
        let result = collector.add_vote(v_reject);
        assert!(result.is_err());
        let conflict = result.unwrap_err();
        assert_eq!(conflict.attestor_id, make_id(1));
        assert_eq!(conflict.conflict_type, "ApprovalFlip");
        assert!(collector.has_conflicts());
    }

    #[test]
    fn test_committee_member_vote_accepted() {
        // FIND-017: a vote from a committee member must be accepted
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        let committee: BTreeSet<[u8; 32]> = [make_id(1), make_id(2)].iter().cloned().collect();
        let vote = make_vote(proposal_id, 1, true);
        assert_eq!(collector.add_vote_from_committee(vote, &committee), Ok(true));
        assert_eq!(collector.votes.len(), 1);
    }

    #[test]
    fn test_non_committee_member_vote_rejected() {
        // FIND-017: a vote from a non-committee node must be rejected and recorded as a conflict
        let proposal_id = make_id(1);
        let mut collector = AttestationCollector::new(proposal_id);
        let committee: BTreeSet<[u8; 32]> = [make_id(2)].iter().cloned().collect(); // attestor 1 not in committee
        let vote = make_vote(proposal_id, 1, true);
        let result = collector.add_vote_from_committee(vote, &committee);
        assert!(result.is_err());
        let conflict = result.unwrap_err();
        assert_eq!(conflict.conflict_type, "NonCommitteeMember");
        assert!(collector.has_conflicts(), "non-committee vote must populate conflicts");
        assert_eq!(collector.votes.len(), 0, "vote must not be counted");
    }

    #[test]
    fn test_quorum_hash_determinism() {
        let proposal_id = make_id(42);
        let mut c1 = AttestationCollector::new(proposal_id);
        let mut c2 = AttestationCollector::new(proposal_id);
        for i in 1..=3u8 {
            c1.add_vote(make_vote(proposal_id, i, true)).unwrap();
            c2.add_vote(make_vote(proposal_id, i, true)).unwrap();
        }
        let threshold = AttestationThreshold::two_thirds(4);
        let r1 = c1.check_quorum(&threshold, 4);
        let r2 = c2.check_quorum(&threshold, 4);
        assert_eq!(r1.quorum_hash, r2.quorum_hash);
    }
}
