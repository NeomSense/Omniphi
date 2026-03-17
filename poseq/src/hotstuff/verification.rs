//! Phase 6 — Consensus message signature verification.
//!
//! All consensus messages (votes, proposals, QCs, NewViews) must be
//! authenticated before being processed by the HotStuff engine.
//! This module provides the verification layer.

use std::collections::BTreeSet;
use sha2::{Digest, Sha256};

use crate::hotstuff::types::{
    HotStuffBlock, HotStuffVote, QuorumCertificate, NewViewMessage, Phase, View,
};
use crate::networking::messages::NodeId;

/// Errors from consensus message verification.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ConsensusVerificationError {
    /// Signature is invalid (failed Ed25519 verification).
    InvalidSignature { signer: NodeId },
    /// Signer is not a committee member.
    NotCommitteeMember { signer: NodeId },
    /// Message is from a stale view (already progressed past it).
    StaleView { message_view: View, current_view: View },
    /// Duplicate message from same sender in same (view, phase).
    DuplicateMessage { signer: NodeId, view: View },
    /// QC does not have enough valid signatures.
    InsufficientQuorum { required: usize, valid: usize },
    /// Block proposer is not the expected leader for this view.
    WrongLeader { expected: NodeId, actual: NodeId },
    /// Message timestamp is too old (potential replay).
    ReplayDetected { signer: NodeId },
}

impl std::fmt::Display for ConsensusVerificationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidSignature { signer } => write!(f, "invalid signature from {}", hex::encode(&signer[..4])),
            Self::NotCommitteeMember { signer } => write!(f, "not committee member: {}", hex::encode(&signer[..4])),
            Self::StaleView { message_view, current_view } => write!(f, "stale view {} (current {})", message_view, current_view),
            Self::DuplicateMessage { signer, view } => write!(f, "duplicate from {} in view {}", hex::encode(&signer[..4]), view),
            Self::InsufficientQuorum { required, valid } => write!(f, "insufficient quorum: {}/{}", valid, required),
            Self::WrongLeader { expected, actual } => write!(f, "wrong leader: expected {}, got {}", hex::encode(&expected[..4]), hex::encode(&actual[..4])),
            Self::ReplayDetected { signer } => write!(f, "replay from {}", hex::encode(&signer[..4])),
        }
    }
}

impl std::error::Error for ConsensusVerificationError {}

/// Verify a vote is from a valid committee member.
///
/// Does NOT verify the Ed25519 signature (caller must do that with crypto module).
/// This checks committee membership and view freshness.
pub fn verify_vote_membership(
    vote: &HotStuffVote,
    committee: &BTreeSet<NodeId>,
    current_view: View,
) -> Result<(), ConsensusVerificationError> {
    // Committee membership
    if !committee.contains(&vote.voter_id) {
        return Err(ConsensusVerificationError::NotCommitteeMember { signer: vote.voter_id });
    }

    // View freshness (allow current view and one behind for propagation delay)
    if vote.view + 2 < current_view {
        return Err(ConsensusVerificationError::StaleView {
            message_view: vote.view,
            current_view,
        });
    }

    Ok(())
}

/// Verify a block proposal is from the expected leader.
pub fn verify_proposal_leader(
    block: &HotStuffBlock,
    expected_leader: NodeId,
    committee: &BTreeSet<NodeId>,
    current_view: View,
) -> Result<(), ConsensusVerificationError> {
    // Leader must be a committee member
    if !committee.contains(&block.leader_id) {
        return Err(ConsensusVerificationError::NotCommitteeMember { signer: block.leader_id });
    }

    // Leader must be the expected leader for this view
    if block.leader_id != expected_leader {
        return Err(ConsensusVerificationError::WrongLeader {
            expected: expected_leader,
            actual: block.leader_id,
        });
    }

    // View freshness
    if block.view + 2 < current_view {
        return Err(ConsensusVerificationError::StaleView {
            message_view: block.view,
            current_view,
        });
    }

    Ok(())
}

/// Verify a QC has sufficient valid signatures from committee members.
///
/// Returns the number of valid committee-member signatures.
/// Does NOT verify Ed25519 signatures (caller must do that).
pub fn verify_qc_committee(
    qc: &QuorumCertificate,
    committee: &BTreeSet<NodeId>,
    quorum_threshold: usize,
) -> Result<usize, ConsensusVerificationError> {
    let valid_count = qc.signatures.iter()
        .filter(|(signer, _)| committee.contains(signer))
        .count();

    if valid_count < quorum_threshold {
        return Err(ConsensusVerificationError::InsufficientQuorum {
            required: quorum_threshold,
            valid: valid_count,
        });
    }

    Ok(valid_count)
}

/// Verify a NewView message is from a committee member.
pub fn verify_new_view_membership(
    nv: &NewViewMessage,
    committee: &BTreeSet<NodeId>,
) -> Result<(), ConsensusVerificationError> {
    if !committee.contains(&nv.sender_id) {
        return Err(ConsensusVerificationError::NotCommitteeMember { signer: nv.sender_id });
    }
    Ok(())
}

/// Compute the vote hash for a (view, block_id, phase) triple.
/// Used for Ed25519 signing/verification of consensus votes.
pub fn compute_vote_payload(view: View, block_id: &[u8; 32], phase: &Phase) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(b"HOTSTUFF_VOTE_V1");
    hasher.update(view.to_be_bytes());
    hasher.update(block_id);
    let phase_byte = match phase {
        Phase::Prepare => 0u8,
        Phase::PreCommit => 1u8,
        Phase::Commit => 2u8,
        Phase::Decide => 3u8,
    };
    hasher.update([phase_byte]);
    let result = hasher.finalize();
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&result);
    hash
}

/// Compute the proposal hash for signing a block proposal.
pub fn compute_proposal_payload(block: &HotStuffBlock) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(b"HOTSTUFF_PROPOSAL_V1");
    hasher.update(block.block_id);
    hasher.update(block.view.to_be_bytes());
    hasher.update(block.leader_id);
    hasher.update(block.batch_root);
    let result = hasher.finalize();
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&result);
    hash
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_committee() -> BTreeSet<NodeId> {
        let mut c = BTreeSet::new();
        for i in 1..=4u8 {
            let mut id = [0u8; 32]; id[0] = i;
            c.insert(id);
        }
        c
    }

    fn make_vote(voter_byte: u8, view: View) -> HotStuffVote {
        let mut voter_id = [0u8; 32]; voter_id[0] = voter_byte;
        HotStuffVote {
            view,
            block_id: [0xAA; 32],
            phase: Phase::Prepare,
            voter_id,
            signature: vec![0u8; 64],
        }
    }

    #[test]
    fn test_verify_vote_valid_member() {
        let committee = make_committee();
        let vote = make_vote(1, 5);
        assert!(verify_vote_membership(&vote, &committee, 5).is_ok());
    }

    #[test]
    fn test_verify_vote_non_member() {
        let committee = make_committee();
        let vote = make_vote(99, 5); // not in committee
        assert!(matches!(
            verify_vote_membership(&vote, &committee, 5),
            Err(ConsensusVerificationError::NotCommitteeMember { .. })
        ));
    }

    #[test]
    fn test_verify_vote_stale_view() {
        let committee = make_committee();
        let vote = make_vote(1, 1); // view 1, current view 10
        assert!(matches!(
            verify_vote_membership(&vote, &committee, 10),
            Err(ConsensusVerificationError::StaleView { .. })
        ));
    }

    #[test]
    fn test_verify_qc_sufficient() {
        let committee = make_committee();
        let mut qc = QuorumCertificate::genesis();
        // Add 3 committee member sigs (threshold = 3)
        for i in 1..=3u8 {
            let mut id = [0u8; 32]; id[0] = i;
            qc.signatures.push((id, vec![0u8; 64]));
        }
        assert!(verify_qc_committee(&qc, &committee, 3).is_ok());
    }

    #[test]
    fn test_verify_qc_insufficient() {
        let committee = make_committee();
        let mut qc = QuorumCertificate::genesis();
        // Only 1 sig (threshold = 3)
        let mut id = [0u8; 32]; id[0] = 1;
        qc.signatures.push((id, vec![0u8; 64]));
        assert!(matches!(
            verify_qc_committee(&qc, &committee, 3),
            Err(ConsensusVerificationError::InsufficientQuorum { .. })
        ));
    }

    #[test]
    fn test_verify_qc_non_member_sigs_ignored() {
        let committee = make_committee(); // members 1-4
        let mut qc = QuorumCertificate::genesis();
        // 2 valid + 1 non-member = 2 valid (below threshold 3)
        for i in [1u8, 2u8, 99u8] {
            let mut id = [0u8; 32]; id[0] = i;
            qc.signatures.push((id, vec![0u8; 64]));
        }
        assert!(matches!(
            verify_qc_committee(&qc, &committee, 3),
            Err(ConsensusVerificationError::InsufficientQuorum { required: 3, valid: 2 })
        ));
    }

    #[test]
    fn test_verify_proposal_leader() {
        let committee = make_committee();
        let expected = { let mut id = [0u8; 32]; id[0] = 1; id };
        let block = HotStuffBlock {
            block_id: [0u8; 32],
            parent_id: [0u8; 32],
            view: 5,
            leader_id: expected,
            batch_root: [0u8; 32],
            ordered_submission_ids: vec![],
            justify_qc: None,
        };
        assert!(verify_proposal_leader(&block, expected, &committee, 5).is_ok());
    }

    #[test]
    fn test_verify_proposal_wrong_leader() {
        let committee = make_committee();
        let expected = { let mut id = [0u8; 32]; id[0] = 1; id };
        let actual = { let mut id = [0u8; 32]; id[0] = 2; id };
        let block = HotStuffBlock {
            block_id: [0u8; 32], parent_id: [0u8; 32], view: 5,
            leader_id: actual, batch_root: [0u8; 32],
            ordered_submission_ids: vec![], justify_qc: None,
        };
        assert!(matches!(
            verify_proposal_leader(&block, expected, &committee, 5),
            Err(ConsensusVerificationError::WrongLeader { .. })
        ));
    }

    #[test]
    fn test_vote_payload_deterministic() {
        let h1 = compute_vote_payload(5, &[0xAA; 32], &Phase::Prepare);
        let h2 = compute_vote_payload(5, &[0xAA; 32], &Phase::Prepare);
        assert_eq!(h1, h2);

        // Different phase → different hash
        let h3 = compute_vote_payload(5, &[0xAA; 32], &Phase::Commit);
        assert_ne!(h1, h3);
    }

    #[test]
    fn test_new_view_membership() {
        let committee = make_committee();
        let sender = { let mut id = [0u8; 32]; id[0] = 1; id };
        let nv = NewViewMessage {
            new_view: 5,
            high_qc: QuorumCertificate::genesis(),
            sender_id: sender,
        };
        assert!(verify_new_view_membership(&nv, &committee).is_ok());

        let bad_sender = { let mut id = [0u8; 32]; id[0] = 99; id };
        let nv_bad = NewViewMessage {
            new_view: 5,
            high_qc: QuorumCertificate::genesis(),
            sender_id: bad_sender,
        };
        assert!(verify_new_view_membership(&nv_bad, &committee).is_err());
    }
}
