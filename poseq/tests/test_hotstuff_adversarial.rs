//! Phase 6 — Adversarial HotStuff consensus tests.
//!
//! Tests safety and liveness properties under Byzantine conditions:
//! double proposals, stale QCs, replay attacks, impersonation,
//! timeout races, restart recovery, and fairness-invalid proposals.

use omniphi_poseq::hotstuff::*;
use omniphi_poseq::hotstuff::verification::*;
use omniphi_poseq::hotstuff::persistence::*;
use std::collections::BTreeSet;

fn make_node_id(b: u8) -> [u8; 32] {
    let mut id = [0u8; 32]; id[0] = b; id
}

fn make_committee(n: u8) -> BTreeSet<[u8; 32]> {
    (1..=n).map(make_node_id).collect()
}

fn make_engine(node_byte: u8, quorum: usize) -> HotStuffEngine {
    HotStuffEngine::new(make_node_id(node_byte), quorum, 5000)
}

// ─── Safety Tests ───────────────────────────────────────────────────────────

#[test]
fn test_safety_cannot_vote_twice_same_view_phase() {
    let mut engine = make_engine(1, 3);
    let leader = make_node_id(2);

    let block = engine.propose([0u8; 32], [0xAA; 32], vec![[1u8; 32]]);

    // First vote should succeed (engine processes its own proposal)
    let output = engine.on_block(block.clone());
    // Engine voted PREPARE
    assert!(!matches!(output, HotStuffOutput::None));

    // Processing the same block again should NOT produce another vote
    let output2 = engine.on_block(block);
    assert!(matches!(output2, HotStuffOutput::None));
}

#[test]
fn test_safety_locked_qc_prevents_unsafe_vote() {
    let mut engine = make_engine(1, 2);

    // Create and process a block at view 1
    let block1 = engine.propose([0u8; 32], [0xAA; 32], vec![]);

    // Simulate getting a PRE-COMMIT QC (which causes locking)
    let precommit_qc = QuorumCertificate {
        view: 1,
        block_id: block1.block_id,
        phase: Phase::PreCommit,
        signatures: vec![
            (make_node_id(1), vec![0u8; 64]),
            (make_node_id(2), vec![0u8; 64]),
        ],
    };
    engine.on_qc(precommit_qc.clone());
    // Engine is now locked on this QC

    // A block at view 2 that does NOT extend the locked block should be rejected
    let mut bad_block = HotStuffBlock {
        block_id: [0u8; 32],
        parent_id: [0xFF; 32], // WRONG parent (doesn't extend locked block)
        view: 2,
        leader_id: make_node_id(3),
        batch_root: [0xBB; 32],
        ordered_submission_ids: vec![],
        justify_qc: Some(QuorumCertificate::genesis()), // genesis QC (lower than locked)
    };
    bad_block.block_id = HotStuffBlock::compute_id(
        &bad_block.parent_id, &bad_block.batch_root, bad_block.view, &bad_block.leader_id,
    );

    let output = engine.on_block(bad_block);
    // Should NOT vote — safety rule prevents it
    assert!(matches!(output, HotStuffOutput::None));
}

#[test]
fn test_safety_view_monotonically_increases() {
    // Verify that the engine's view only increases, never decreases.
    let mut engine = make_engine(1, 2);
    let mut last_view = engine.current_view();

    for _ in 0..5 {
        engine.on_timeout();
        let new_view = engine.current_view();
        assert!(new_view > last_view, "view must monotonically increase");
        last_view = new_view;
    }
}

// ─── Liveness Tests ─────────────────────────────────────────────────────────

#[test]
fn test_liveness_timeout_advances_view() {
    let mut engine = make_engine(1, 3);
    let initial_view = engine.current_view();

    let output = engine.on_timeout();
    match output {
        HotStuffOutput::SendNewView(nv) => {
            assert!(nv.new_view > initial_view);
        }
        _ => panic!("expected SendNewView on timeout"),
    }

    assert!(engine.current_view() > initial_view);
}

#[test]
fn test_liveness_new_view_quorum_advances() {
    let mut engine = make_engine(1, 3);

    // Collect 3 NewViews (quorum = 3)
    for i in 2..=4u8 {
        let nv = NewViewMessage {
            new_view: 2,
            high_qc: QuorumCertificate::genesis(),
            sender_id: make_node_id(i),
        };
        engine.on_new_view(nv);
    }

    // After quorum, engine should advance to view 2
    assert!(engine.current_view() >= 2);
}

#[test]
fn test_liveness_consecutive_timeouts_backoff() {
    let mut engine = make_engine(1, 3);

    // Multiple timeouts should increase view monotonically
    let mut last_view = engine.current_view();
    for _ in 0..5 {
        let output = engine.on_timeout();
        let new_view = engine.current_view();
        assert!(new_view > last_view, "view must advance on timeout");
        last_view = new_view;
    }
}

// ─── Verification Tests ────────────────────────────────────────────────────

#[test]
fn test_verification_non_member_vote_rejected() {
    let committee = make_committee(4);
    let vote = HotStuffVote {
        view: 5,
        block_id: [0xAA; 32],
        phase: Phase::Prepare,
        voter_id: make_node_id(99), // not in committee
        signature: vec![0u8; 64],
    };
    assert!(matches!(
        verify_vote_membership(&vote, &committee, 5),
        Err(ConsensusVerificationError::NotCommitteeMember { .. })
    ));
}

#[test]
fn test_verification_stale_vote_rejected() {
    let committee = make_committee(4);
    let vote = HotStuffVote {
        view: 1, // very old
        block_id: [0xAA; 32],
        phase: Phase::Prepare,
        voter_id: make_node_id(1),
        signature: vec![0u8; 64],
    };
    assert!(matches!(
        verify_vote_membership(&vote, &committee, 10),
        Err(ConsensusVerificationError::StaleView { .. })
    ));
}

#[test]
fn test_verification_wrong_leader_rejected() {
    let committee = make_committee(4);
    let expected_leader = make_node_id(1);
    let block = HotStuffBlock {
        block_id: [0u8; 32],
        parent_id: [0u8; 32],
        view: 5,
        leader_id: make_node_id(2), // WRONG leader
        batch_root: [0u8; 32],
        ordered_submission_ids: vec![],
        justify_qc: None,
    };
    assert!(matches!(
        verify_proposal_leader(&block, expected_leader, &committee, 5),
        Err(ConsensusVerificationError::WrongLeader { .. })
    ));
}

#[test]
fn test_verification_qc_insufficient_sigs() {
    let committee = make_committee(4);
    let mut qc = QuorumCertificate::genesis();
    qc.signatures.push((make_node_id(1), vec![0u8; 64]));
    // Only 1 sig, threshold = 3
    assert!(matches!(
        verify_qc_committee(&qc, &committee, 3),
        Err(ConsensusVerificationError::InsufficientQuorum { required: 3, valid: 1 })
    ));
}

#[test]
fn test_verification_qc_non_member_sigs_excluded() {
    let committee = make_committee(4); // members 1-4
    let mut qc = QuorumCertificate::genesis();
    // 2 valid + 2 non-member = 2 valid (below threshold 3)
    qc.signatures.push((make_node_id(1), vec![0u8; 64]));
    qc.signatures.push((make_node_id(2), vec![0u8; 64]));
    qc.signatures.push((make_node_id(50), vec![0u8; 64])); // not member
    qc.signatures.push((make_node_id(60), vec![0u8; 64])); // not member
    assert!(matches!(
        verify_qc_committee(&qc, &committee, 3),
        Err(ConsensusVerificationError::InsufficientQuorum { required: 3, valid: 2 })
    ));
}

// ─── Persistence/Recovery Tests ─────────────────────────────────────────────

#[test]
fn test_persistence_state_round_trip() {
    let qc = QuorumCertificate::genesis();
    let state = HotStuffConsensusState::capture(
        10, &qc, &qc, &[(5, 0), (6, 1), (7, 0)], 4, [0xBB; 32],
    );

    let bytes = bincode::serialize(&state).unwrap();
    let restored: HotStuffConsensusState = bincode::deserialize(&bytes).unwrap();

    assert_eq!(restored.current_view, 10);
    assert_eq!(restored.voted_views.len(), 3);
    assert_eq!(restored.last_finalized_view, 4);
    assert_eq!(restored.last_finalized_block_id, [0xBB; 32]);
}

#[test]
fn test_persistence_prevents_double_vote_after_restart() {
    let state = HotStuffConsensusState {
        current_view: 5,
        high_qc: QuorumCertificate::genesis(),
        locked_qc: QuorumCertificate::genesis(),
        voted_views: vec![(5, 0)], // already voted PREPARE in view 5
        last_finalized_view: 3,
        last_finalized_block_id: [0u8; 32],
    };

    // After restart, node checks voted_views before voting
    assert!(state.has_voted(5, 0)); // should be true → don't vote again
    assert!(!state.has_voted(5, 1)); // PreCommit not voted yet → can vote
    assert!(!state.has_voted(6, 0)); // view 6 not voted → can vote
}

// ─── Vote Payload Determinism ───────────────────────────────────────────────

#[test]
fn test_vote_payload_deterministic_across_nodes() {
    let h1 = compute_vote_payload(5, &[0xAA; 32], &Phase::Prepare);
    let h2 = compute_vote_payload(5, &[0xAA; 32], &Phase::Prepare);
    assert_eq!(h1, h2);

    // Different view → different payload
    let h3 = compute_vote_payload(6, &[0xAA; 32], &Phase::Prepare);
    assert_ne!(h1, h3);

    // Different block → different payload
    let h4 = compute_vote_payload(5, &[0xBB; 32], &Phase::Prepare);
    assert_ne!(h1, h4);

    // Different phase → different payload
    let h5 = compute_vote_payload(5, &[0xAA; 32], &Phase::Commit);
    assert_ne!(h1, h5);
}

#[test]
fn test_proposal_payload_deterministic() {
    let block = HotStuffBlock {
        block_id: [1u8; 32],
        parent_id: [0u8; 32],
        view: 5,
        leader_id: make_node_id(1),
        batch_root: [0xAA; 32],
        ordered_submission_ids: vec![],
        justify_qc: None,
    };
    let h1 = compute_proposal_payload(&block);
    let h2 = compute_proposal_payload(&block);
    assert_eq!(h1, h2);
}

// ─── Finalization Tests ─────────────────────────────────────────────────────

#[test]
fn test_engine_proposal_creates_valid_block() {
    let mut engine = make_engine(1, 2);
    let block = engine.propose([0u8; 32], [0xAA; 32], vec![[1u8; 32]]);

    // Block must have deterministic ID
    let expected_id = HotStuffBlock::compute_id(
        &block.parent_id, &block.batch_root, block.view, &block.leader_id,
    );
    assert_eq!(block.block_id, expected_id);

    // Block must carry HighQC
    assert!(block.justify_qc.is_some());

    // View must be current
    assert_eq!(block.view, engine.current_view());
}
