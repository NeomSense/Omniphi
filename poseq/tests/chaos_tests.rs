// Chaos / fault-injection integration tests for omniphi-poseq.
//
// These tests use only the public API. They exercise adversarial inputs,
// boundary conditions, and high-load scenarios to verify:
//   - No panics under any valid or invalid input
//   - Resource bounds are respected
//   - Invariants hold under repeated or concurrent stress

use omniphi_poseq::chain_bridge::evidence::{DuplicateEvidenceGuard, EvidencePacket};
use omniphi_poseq::checkpoints::{CheckpointMetadata, CheckpointPolicy, CheckpointStore, PoSeqCheckpoint};
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::errors::PoSeqError;
use omniphi_poseq::finality::FinalityCheckpoint;
use omniphi_poseq::misbehavior::types::MisbehaviorType;
use omniphi_poseq::queue::pending::ReplayGuard;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::PoSeqNode;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ─── Shared Helpers ───────────────────────────────────────────────────────────

fn make_submission(idx: u32, class: SubmissionClass) -> SequencingSubmission {
    let payload_body = {
        let mut v = vec![0u8; 64];
        v[0] = (idx & 0xFF) as u8;
        v[1] = ((idx >> 8) & 0xFF) as u8;
        v[2] = ((idx >> 16) & 0xFF) as u8;
        v[3] = class.priority_weight() as u8;
        v
    };
    let payload_hash = {
        let h = Sha256::digest(&payload_body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    let mut sender = [0u8; 32];
    sender[0] = (idx & 0xFF) as u8;
    sender[1] = ((idx >> 8) & 0xFF) as u8;
    sender[2] = 0x01;

    let mut submission_id = [0u8; 32];
    submission_id[0] = (idx & 0xFF) as u8;
    submission_id[1] = ((idx >> 8) & 0xFF) as u8;
    submission_id[2] = class.priority_weight() as u8;
    submission_id[31] = 0x01;

    SequencingSubmission {
        submission_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class,
        payload_hash,
        payload_body,
        nonce: idx as u64,
        max_fee: 100,
        deadline_epoch: 9999,
        metadata: SubmissionMetadata {
            sequence_hint: idx as u64,
            priority_hint: 0,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    }
}

fn make_valid_checkpoint(epoch: u64, seq: u64) -> PoSeqCheckpoint {
    let node_id = [0x01u8; 32];
    let meta = CheckpointMetadata {
        version: 1,
        epoch,
        slot: epoch * 100,
        created_seq: seq,
        node_id,
    };
    let finality_cp = FinalityCheckpoint::compute(epoch, [0xABu8; 32], epoch * 100);
    let checkpoint_id = PoSeqCheckpoint::compute_id(&meta, &finality_cp.checkpoint_hash);
    PoSeqCheckpoint {
        metadata: meta,
        finality_checkpoint: finality_cp,
        epoch_state_hash: [0x11u8; 32],
        bridge_state_hash: [0x22u8; 32],
        misbehavior_count: 0,
        checkpoint_id,
    }
}

// ─── Test 1: Duplicate Message Storm ─────────────────────────────────────────

#[test]
fn test_chaos_duplicate_message_storm() {
    // Submit 10,000 duplicates of the same submission.
    // Only the first should be accepted; the rest must return Duplicate.
    // The queue must never grow beyond 1 entry.
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    let original = make_submission(0x01, SubmissionClass::Transfer);

    let mut accepted = 0usize;
    let mut rejected = 0usize;

    let first_result = node.submit(original.clone());
    assert!(
        first_result.is_ok(),
        "first submission must be accepted, got {:?}",
        first_result
    );
    accepted += 1;

    for _ in 1..10_000 {
        let dup = original.clone();
        match node.submit(dup) {
            Ok(_) => accepted += 1,
            Err(PoSeqError::Duplicate(_)) => rejected += 1,
            Err(other) => panic!("unexpected error on duplicate: {:?}", other),
        }
    }

    assert_eq!(accepted, 1, "exactly 1 submission must be accepted");
    assert_eq!(rejected, 9_999, "exactly 9,999 duplicates must be rejected");

    // Queue depth must not grow beyond 1
    assert_eq!(node.queue.len(), 1, "queue must contain exactly 1 entry");
}

// ─── Test 2: Queue Overflow Under Load ───────────────────────────────────────

#[test]
fn test_chaos_queue_overflow_under_load() {
    let capacity = 100;
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_pending_queue_size = capacity;
    let mut node = PoSeqNode::new(policy);

    // Fill to capacity
    let mut accepted = 0usize;
    for i in 0..capacity as u32 {
        let sub = make_submission(i, SubmissionClass::Transfer);
        node.submit(sub).expect("submissions up to capacity must succeed");
        accepted += 1;
    }
    assert_eq!(node.queue.len(), capacity);
    assert_eq!(accepted, capacity);

    // Next submission must return QueueFull — not panic
    for i in capacity as u32..(capacity + 50) as u32 {
        let sub = make_submission(i, SubmissionClass::Transfer);
        match node.submit(sub) {
            Err(PoSeqError::QueueFull { capacity: cap }) => {
                assert_eq!(cap, capacity, "QueueFull must report correct capacity");
            }
            Ok(_) => panic!("submission beyond capacity must not succeed"),
            Err(other) => panic!("unexpected error on overflow: {:?}", other),
        }
    }

    // Queue must remain at exactly capacity — not corrupted
    assert_eq!(
        node.queue.len(),
        capacity,
        "queue must remain at capacity after overflow attempts"
    );

    // After draining, should be able to submit again
    let (batch, _) = node.produce_batch(1, None).expect("produce_batch must succeed");
    assert_eq!(batch.ordered_submissions.len(), capacity);
    assert_eq!(node.queue.len(), 0, "queue must be empty after produce_batch");

    // Submit again after drain
    let sub = make_submission(9999, SubmissionClass::Transfer);
    node.submit(sub).expect("should be able to submit after queue drains");
    assert_eq!(node.queue.len(), 1);
}

// ─── Test 3: Finalization Double Slot ────────────────────────────────────────

#[test]
fn test_chaos_finalization_double_slot() {
    use omniphi_poseq::attestations::collector::{
        AttestationCollector, AttestationThreshold, BatchAttestationVote,
    };
    use omniphi_poseq::finalization::engine::{FinalizationDecision, FinalizationEngine};
    use omniphi_poseq::proposals::batch::ProposedBatch;

    let mut engine = FinalizationEngine::new();

    let slot = 5u64;
    let epoch = 1u64;
    let quorum_size = 5;

    // Proposal A: leader_id = [10; 32]
    let leader_a = [10u8; 32];
    let batch_a = ProposedBatch::new(
        slot,
        epoch,
        leader_a,
        vec![[1u8; 32], [2u8; 32]],
        [0u8; 32],
        1,
        100,
    );

    // Proposal B: different leader_id for same (slot, epoch) — simulates competing proposals
    let leader_b = [11u8; 32];
    let batch_b = ProposedBatch::new(
        slot,
        epoch,
        leader_b,
        vec![[3u8; 32], [4u8; 32]],
        [0u8; 32],
        1,
        100,
    );

    fn make_collector(proposal_id: [u8; 32], count: usize, epoch: u64) -> AttestationCollector {
        let mut c = AttestationCollector::new(proposal_id);
        for i in 0..count {
            let mut attestor = [0u8; 32];
            attestor[0] = i as u8;
            attestor[31] = 0x01;
            let vote = BatchAttestationVote::new(proposal_id, attestor, true, epoch);
            let _ = c.add_vote(vote);
        }
        c
    }

    let threshold = AttestationThreshold::two_thirds(quorum_size);

    let collector_a = make_collector(batch_a.proposal_id, quorum_size, epoch);
    let collector_b = make_collector(batch_b.proposal_id, quorum_size, epoch);

    // Finalize proposal A first
    let decision_a = engine.finalize(&batch_a, &collector_a, &threshold, quorum_size, 100);
    assert!(
        matches!(decision_a, FinalizationDecision::Finalized(_)),
        "first finalization for (slot={}, epoch={}) must succeed, got {:?}",
        slot,
        epoch,
        decision_a
    );

    // Attempt to finalize proposal B for the same (slot, epoch) — must be rejected
    let decision_b = engine.finalize(&batch_b, &collector_b, &threshold, quorum_size, 101);
    assert!(
        matches!(
            decision_b,
            FinalizationDecision::SlotAlreadyFinalized { slot: 5, epoch: 1, .. }
        ),
        "second proposal for same (slot, epoch) must return SlotAlreadyFinalized, got {:?}",
        decision_b
    );

    // Attempt to finalize proposal A again — must return AlreadyFinalized
    let decision_a2 = engine.finalize(&batch_a, &collector_a, &threshold, quorum_size, 102);
    assert!(
        matches!(decision_a2, FinalizationDecision::AlreadyFinalized),
        "re-finalizing same proposal must return AlreadyFinalized, got {:?}",
        decision_a2
    );
}

// ─── Test 4: Replay Attack Storm ─────────────────────────────────────────────

#[test]
fn test_chaos_replay_attack_storm() {
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    let original = make_submission(0x42, SubmissionClass::Swap);

    // First submission: accepted
    let first = node.submit(original.clone());
    assert!(first.is_ok(), "first submission must succeed");

    // 10,000 replays: all must be rejected as Duplicate
    let mut replay_count = 0usize;
    for _ in 0..10_000 {
        match node.submit(original.clone()) {
            Err(PoSeqError::Duplicate(_)) => replay_count += 1,
            Ok(_) => panic!("replay must not be accepted"),
            Err(other) => panic!("unexpected error on replay: {:?}", other),
        }
    }

    assert_eq!(replay_count, 10_000, "all 10,000 replays must be rejected");

    // Queue must still have exactly 1 entry (the original)
    assert_eq!(node.queue.len(), 1, "queue must contain exactly 1 entry after replay storm");

    // The replay guard must not grow unboundedly when using bounded mode
    // Test with a bounded ReplayGuard specifically
    let bound = 100usize;
    let mut guard = ReplayGuard::new(bound);
    for i in 0..10_000u32 {
        let mut id = [0u8; 32];
        id[0] = (i & 0xFF) as u8;
        id[1] = ((i >> 8) & 0xFF) as u8;
        id[2] = 0x01;
        let _ = guard.check_and_record(id);
    }
    // Internal size is bounded to `bound` (FIFO eviction)
    // We verify this indirectly: after 10,000 inserts into a capacity-100 guard,
    // the most recently inserted 100 IDs are still present (not evicted),
    // while the earliest ones are gone.
    let mut recent_id = [0u8; 32];
    recent_id[0] = (9999u32 & 0xFF) as u8;
    recent_id[1] = ((9999u32 >> 8) & 0xFF) as u8;
    recent_id[2] = 0x01;
    // The recent ID should be rejected as duplicate (still in guard)
    assert!(
        guard.check_and_record(recent_id).is_err(),
        "most recently inserted ID must still be in bounded guard"
    );

    // An old ID should have been evicted
    let mut old_id = [0u8; 32];
    old_id[0] = 0x00;
    old_id[1] = 0x00;
    old_id[2] = 0x01;
    // After 10,000 inserts into a capacity-100 guard, id[0] was evicted long ago
    assert!(
        guard.check_and_record(old_id).is_ok(),
        "early evicted IDs must be re-insertable into bounded guard"
    );
}

// ─── Test 5: Malformed Batch Submissions ─────────────────────────────────────

#[test]
fn test_chaos_malformed_batch_submission() {
    use omniphi_poseq::errors::ValidationError;

    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    // Zero sender
    {
        let body = b"test payload".to_vec();
        let hash = {
            let h = Sha256::digest(&body);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };
        let sub = SequencingSubmission {
            submission_id: [0xABu8; 32],
            sender: [0u8; 32], // zero — invalid
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: hash,
            payload_body: body,
            nonce: 1,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: 1,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(ValidationError::ZeroSender)) => {}
            other => panic!("zero sender must return ZeroSender, got {:?}", other),
        }
    }

    // Zero submission_id
    {
        let body = b"another test payload".to_vec();
        let hash = {
            let h = Sha256::digest(&body);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };
        let sub = SequencingSubmission {
            submission_id: [0u8; 32], // zero — invalid
            sender: [0x01u8; 32],
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: hash,
            payload_body: body,
            nonce: 2,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: 2,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(ValidationError::ZeroSubmissionId)) => {}
            other => panic!("zero submission_id must return ZeroSubmissionId, got {:?}", other),
        }
    }

    // Mismatched payload_hash
    {
        let body = b"real body".to_vec();
        let wrong_hash = [0xDEu8; 32]; // random non-zero hash that doesn't match body
        let sub = SequencingSubmission {
            submission_id: [0x01u8; 32],
            sender: [0x01u8; 32],
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: wrong_hash,
            payload_body: body,
            nonce: 3,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: 3,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(ValidationError::MalformedEnvelope(_))) => {}
            other => panic!("wrong payload_hash must return MalformedEnvelope, got {:?}", other),
        }
    }

    // Oversized payload (> policy max 64KB)
    {
        let oversized_body = vec![0xBEu8; 70_000]; // 70KB > 65536 policy max
        let hash = {
            let h = Sha256::digest(&oversized_body);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };
        let sub = SequencingSubmission {
            submission_id: [0x02u8; 32],
            sender: [0x02u8; 32],
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: hash,
            payload_body: oversized_body,
            nonce: 4,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: 4,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(ValidationError::PayloadTooLarge { .. })) => {}
            other => panic!("oversized payload must return PayloadTooLarge, got {:?}", other),
        }
    }

    // All zero payload_hash
    {
        let body = vec![0u8; 32];
        let sub = SequencingSubmission {
            submission_id: [0x03u8; 32],
            sender: [0x03u8; 32],
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: [0u8; 32], // zero — invalid regardless of body
            payload_body: body,
            nonce: 5,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: 5,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(ValidationError::EmptyPayloadHash)) => {}
            other => panic!("zero payload_hash must return EmptyPayloadHash, got {:?}", other),
        }
    }

    // Queue must still be empty — all were rejected
    assert_eq!(
        node.queue.len(),
        0,
        "queue must be empty: all malformed submissions must be rejected"
    );
}

// ─── Test 6: Checkpoint Tamper and Restore ────────────────────────────────────

#[test]
fn test_chaos_checkpoint_tamper_and_restore() {
    let policy = CheckpointPolicy {
        checkpoint_interval_epochs: 1,
        max_checkpoints_retained: 50,
    };
    let mut store = CheckpointStore::new(policy);

    // Store valid checkpoint at epoch 10
    let valid_cp = make_valid_checkpoint(10, 0);
    let valid_id = valid_cp.checkpoint_id;
    store.store(valid_cp).expect("valid checkpoint must store successfully");

    // Restore valid checkpoint — must succeed
    let result = store.restore(&valid_id);
    assert!(result.success, "valid checkpoint restore must succeed");
    assert_eq!(result.epoch_restored, 10);
    assert!(result.errors.is_empty());

    // Restore again — must be idempotent
    let result2 = store.restore(&valid_id);
    assert!(result2.success);
    assert_eq!(result2.epoch_restored, result.epoch_restored);

    // Tamper: change epoch without recomputing checkpoint_id
    let mut tampered_cp = make_valid_checkpoint(20, 1);
    tampered_cp.metadata.epoch = 999; // tamper
    // verify_id must fail
    assert!(
        !tampered_cp.verify_id(),
        "tampered checkpoint must fail verify_id"
    );
    // store.store() must reject it (IdMismatch)
    let tamper_result = store.store(tampered_cp);
    assert!(
        tamper_result.is_err(),
        "storing tampered checkpoint must return Err"
    );

    // Tamper: change slot without recomputing id
    let mut tampered_slot = make_valid_checkpoint(30, 2);
    tampered_slot.metadata.slot = u64::MAX; // tamper
    assert!(!tampered_slot.verify_id(), "slot-tampered checkpoint must fail verify_id");
    assert!(store.store(tampered_slot).is_err());

    // Restore unknown id — must fail gracefully
    let unknown = store.restore(&[0xFFu8; 32]);
    assert!(!unknown.success, "restoring unknown id must fail");
    assert!(!unknown.errors.is_empty(), "error list must be non-empty");

    // Valid checkpoint at epoch 20 (without tampering)
    let valid_20 = make_valid_checkpoint(20, 3);
    let id_20 = valid_20.checkpoint_id;
    store.store(valid_20).expect("must store valid checkpoint at epoch 20");
    let r20 = store.restore(&id_20);
    assert!(r20.success, "must restore epoch 20 checkpoint");
    assert_eq!(r20.epoch_restored, 20);
}

// ─── Test 7: Ordering Adversarial Priorities ──────────────────────────────────

#[test]
fn test_chaos_ordering_adversarial_priorities() {
    use omniphi_poseq::config::policy::OrderingPolicyConfig;
    use omniphi_poseq::intake::receiver::SubmissionReceiver;
    use omniphi_poseq::ordering::engine::OrderingEngine;
    use omniphi_poseq::types::submission::SubmissionMetadata;
    use omniphi_poseq::validation::validator::SubmissionValidator;

    let policy = PoSeqPolicy::default_policy();
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());

    fn make_vs_with_priority(
        idx: u32,
        priority: u32,
        class: SubmissionClass,
        policy: &PoSeqPolicy,
    ) -> omniphi_poseq::validation::validator::ValidatedSubmission {
        let payload_body = {
            let mut v = vec![0u8; 32];
            v[0] = (idx & 0xFF) as u8;
            v[1] = ((idx >> 8) & 0xFF) as u8;
            v[2] = priority as u8;
            v
        };
        let payload_hash = {
            let h = Sha256::digest(&payload_body);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };
        let mut sender = [0u8; 32];
        sender[0] = (idx & 0xFF) as u8;
        sender[1] = 0x01;
        let mut sub_id = [0u8; 32];
        sub_id[0] = (idx & 0xFF) as u8;
        sub_id[1] = priority as u8;
        sub_id[31] = 0x01;

        let sub = SequencingSubmission {
            submission_id: sub_id,
            sender,
            kind: SubmissionKind::IntentTransaction,
            class,
            payload_hash,
            payload_body,
            nonce: idx as u64,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: idx as u64,
                priority_hint: priority,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        let mut receiver = SubmissionReceiver::new();
        let envelope = receiver.receive(sub);
        SubmissionValidator::validate(envelope, policy).expect("valid submission")
    }

    // Case 1: Max priority (10000) for all
    {
        let subs: Vec<_> = (0..50u32)
            .map(|i| make_vs_with_priority(i, 10000, SubmissionClass::Transfer, &policy))
            .collect();
        let count = subs.len();
        let ordered = engine.order(subs).expect("ordering must not panic with max priority");
        assert_eq!(ordered.len(), count, "no submissions must be dropped");

        // Run again to verify determinism
        let subs2: Vec<_> = (0..50u32)
            .map(|i| make_vs_with_priority(i, 10000, SubmissionClass::Transfer, &policy))
            .collect();
        let ordered2 = engine.order(subs2).expect("second run must succeed");
        let ids1: Vec<_> = ordered.iter().map(|s| s.envelope.normalized_id).collect();
        let ids2: Vec<_> = ordered2.iter().map(|s| s.envelope.normalized_id).collect();
        assert_eq!(ids1, ids2, "ordering must be deterministic with max priority");
    }

    // Case 2: Min priority (0) for all
    {
        let subs: Vec<_> = (0..50u32)
            .map(|i| make_vs_with_priority(i + 100, 0, SubmissionClass::Transfer, &policy))
            .collect();
        let count = subs.len();
        let ordered = engine.order(subs).expect("ordering must not panic with zero priority");
        assert_eq!(ordered.len(), count);
    }

    // Case 3: All duplicate priorities (5000)
    {
        let subs: Vec<_> = (0..100u32)
            .map(|i| make_vs_with_priority(i + 200, 5000, SubmissionClass::Swap, &policy))
            .collect();
        let count = subs.len();
        let ordered = engine.order(subs.clone()).expect("ordering must not panic with duplicate priorities");
        assert_eq!(ordered.len(), count, "no submissions dropped with duplicate priorities");

        let ordered2 = engine.order(subs).expect("second run must succeed");
        let ids1: Vec<_> = ordered.iter().map(|s| s.envelope.normalized_id).collect();
        let ids2: Vec<_> = ordered2.iter().map(|s| s.envelope.normalized_id).collect();
        assert_eq!(ids1, ids2, "tie-breaking must be deterministic");
    }

    // Case 4: Alternating max/min priority
    {
        let subs: Vec<_> = (0..50u32)
            .map(|i| {
                let p = if i % 2 == 0 { 10000 } else { 0 };
                make_vs_with_priority(i + 300, p, SubmissionClass::GoalPacket, &policy)
            })
            .collect();
        let count = subs.len();
        let ordered = engine.order(subs).expect("ordering must not panic with alternating priorities");
        assert_eq!(ordered.len(), count);
    }
}

// ─── Test 8: Epoch Boundary Stress ───────────────────────────────────────────

#[test]
fn test_chaos_epoch_boundary_stress() {
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_submissions_per_batch = 5;
    policy.batch.max_pending_queue_size = 10;
    let mut node = PoSeqNode::new(policy);

    let mut total_produced = 0usize;
    let mut prev_batch_id: Option<[u8; 32]> = None;

    for epoch in 1u64..=100 {
        // Submit 5 unique submissions per epoch
        for sub_idx in 0..5u32 {
            let global_idx = ((epoch - 1) * 5 + sub_idx as u64) as u32;
            let class = match global_idx % 6 {
                0 => SubmissionClass::Transfer,
                1 => SubmissionClass::Swap,
                2 => SubmissionClass::YieldAllocate,
                3 => SubmissionClass::TreasuryRebalance,
                4 => SubmissionClass::GoalPacket,
                _ => SubmissionClass::AgentSubmission,
            };
            let sub = make_submission(global_idx + 1, class);
            node.submit(sub).expect("submit must succeed in epoch stress test");
        }

        // Produce one batch per epoch
        let (batch, receipt) = node.produce_batch(epoch, None)
            .expect("produce_batch must succeed in epoch stress test");

        // Verify invariants
        assert_eq!(batch.header.epoch, epoch, "epoch must match");
        assert_eq!(receipt.accepted_count, 5, "all 5 submissions must be accepted");
        assert_eq!(receipt.rejected_count, 0);
        assert_eq!(node.queue.len(), 0, "queue must be empty after produce_batch");

        // Verify no batch ID collision across epochs
        let current_batch_id = batch.header.batch_id;
        if let Some(prev) = prev_batch_id {
            assert_ne!(
                current_batch_id,
                prev,
                "batch IDs must be unique across epochs (epoch {})",
                epoch
            );
        }
        prev_batch_id = Some(current_batch_id);

        total_produced += batch.ordered_submissions.len();
    }

    assert_eq!(total_produced, 500, "100 epochs * 5 submissions = 500 total");
    assert_eq!(node.state.current_height, 100, "height must advance once per batch");
    assert_eq!(node.ledger.len(), 100, "ledger must have one entry per batch");

    // Verify no slot collision: all batch IDs in the ledger must be unique
    let mut seen_ids = std::collections::BTreeSet::new();
    for height in 1u64..=100 {
        let record = node.ledger.get(height).expect("ledger must have entry at height");
        assert!(
            seen_ids.insert(record.batch_id),
            "duplicate batch_id detected at height {}",
            height
        );
    }
}

// ─── Test 9: Evidence Dedup Storm ────────────────────────────────────────────

#[test]
fn test_chaos_evidence_dedup_storm() {
    let node_id = [0x01u8; 32];
    let epoch = 5u64;
    let slot = 10u64;
    let evidence_hashes = vec![[0xABu8; 32]];

    // Build the same packet 1000 times and submit to guard
    let packet = EvidencePacket::from_misbehavior(
        &MisbehaviorType::Equivocation,
        node_id,
        epoch,
        slot,
        evidence_hashes.clone(),
        None,
    );

    let mut guard = DuplicateEvidenceGuard::new();

    // First registration must succeed
    assert!(guard.register(&packet), "first registration must succeed");

    // 999 more registrations must all fail
    let mut rejected_count = 0usize;
    for _ in 1..1000 {
        if !guard.register(&packet) {
            rejected_count += 1;
        }
    }
    assert_eq!(rejected_count, 999, "999 duplicate registrations must be rejected");

    // Guard memory stays bounded: the seen set must have exactly 1 entry
    assert_eq!(
        guard.seen_count(),
        1,
        "DuplicateEvidenceGuard must contain exactly 1 entry after storm"
    );

    // is_seen must return true
    assert!(guard.is_seen(&packet.packet_hash));

    // Adding a different packet (different epoch) must succeed
    let packet2 = EvidencePacket::from_misbehavior(
        &MisbehaviorType::Equivocation,
        node_id,
        epoch + 1,
        slot,
        evidence_hashes,
        None,
    );
    assert!(guard.register(&packet2), "distinct packet must register");
    assert_eq!(guard.seen_count(), 2);

    // Test with many distinct packet types
    let misbehavior_types = [
        MisbehaviorType::Equivocation,
        MisbehaviorType::InvalidProposalAuthority,
        MisbehaviorType::ReplayAttack,
        MisbehaviorType::RuntimeBridgeAbuse,
        MisbehaviorType::AbsentFromDuty,
    ];

    let mut multi_guard = DuplicateEvidenceGuard::new();
    let mut expected_count = 0usize;

    for mtype in &misbehavior_types {
        let p = EvidencePacket::from_misbehavior(mtype, node_id, epoch, slot, vec![], None);
        assert!(multi_guard.register(&p), "distinct misbehavior type must register");
        expected_count += 1;
    }
    assert_eq!(multi_guard.seen_count(), expected_count);

    // Re-register all — all must be rejected
    for mtype in &misbehavior_types {
        let p = EvidencePacket::from_misbehavior(mtype, node_id, epoch, slot, vec![], None);
        assert!(!multi_guard.register(&p), "duplicate misbehavior must be rejected");
    }
    // Count unchanged
    assert_eq!(multi_guard.seen_count(), expected_count);
}

// ─── Test 10: Mixed Load Pipeline ────────────────────────────────────────────

#[test]
fn test_chaos_mixed_load_pipeline() {
    // 500 valid submissions + 200 duplicates + 100 invalid (wrong hash) + 50 overflow attempts
    // Expected: exactly 500 accepted, queue drains cleanly

    let capacity = 600usize; // large enough for 500 valid
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_submissions_per_batch = 500;
    policy.batch.max_pending_queue_size = capacity;
    let mut node = PoSeqNode::new(policy);

    // Phase 1: Submit 500 valid unique submissions
    let mut accepted = 0usize;
    for i in 0..500u32 {
        let sub = make_submission(i + 1, SubmissionClass::Transfer);
        node.submit(sub).expect("valid submission must succeed");
        accepted += 1;
    }
    assert_eq!(node.queue.len(), 500);
    assert_eq!(accepted, 500);

    // Phase 2: Submit 200 duplicates (of submissions 0..200)
    let mut dup_rejected = 0usize;
    for i in 0..200u32 {
        let dup = make_submission(i + 1, SubmissionClass::Transfer); // same as phase 1
        match node.submit(dup) {
            Err(PoSeqError::Duplicate(_)) => dup_rejected += 1,
            Ok(_) => panic!("duplicate must not be accepted"),
            Err(other) => panic!("unexpected error: {:?}", other),
        }
    }
    assert_eq!(dup_rejected, 200);

    // Phase 3: Submit 100 invalid submissions (wrong hash)
    let mut invalid_rejected = 0usize;
    for i in 0..100u32 {
        let body = vec![0xBBu8; 64];
        let wrong_hash = [0xFFu8; 32]; // definitely wrong hash
        let mut sender = [0u8; 32];
        sender[0] = ((500 + i) & 0xFF) as u8;
        sender[1] = 0x01;
        let mut sub_id = [0u8; 32];
        sub_id[0] = ((600 + i) & 0xFF) as u8;
        sub_id[31] = 0x01;
        let sub = SequencingSubmission {
            submission_id: sub_id,
            sender,
            kind: SubmissionKind::IntentTransaction,
            class: SubmissionClass::Transfer,
            payload_hash: wrong_hash,
            payload_body: body,
            nonce: (500 + i) as u64,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: (500 + i) as u64,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };
        match node.submit(sub) {
            Err(PoSeqError::Validation(_)) => invalid_rejected += 1,
            Ok(_) => panic!("invalid submission must not be accepted"),
            Err(other) => panic!("unexpected error on invalid: {:?}", other),
        }
    }
    assert_eq!(invalid_rejected, 100);

    // Queue must still have exactly 500 entries
    assert_eq!(
        node.queue.len(),
        500,
        "queue must contain exactly 500 valid entries"
    );

    // Phase 4: Queue overflow attempts (queue is at 500, capacity 600 — so we can add 100 more)
    // But per spec we want 50 overflow attempts — fill to capacity first then overflow
    // Fill to capacity with 100 more valid submissions
    for i in 0..100u32 {
        let sub = make_submission(700 + i, SubmissionClass::Swap);
        node.submit(sub).expect("valid submission must succeed up to capacity");
    }
    assert_eq!(node.queue.len(), 600);

    // Now 50 overflow attempts (capacity is 600, queue is full at 600)
    let mut overflow_count = 0usize;
    for i in 0..50u32 {
        let sub = make_submission(1000 + i, SubmissionClass::GoalPacket);
        match node.submit(sub) {
            Err(PoSeqError::QueueFull { .. }) => overflow_count += 1,
            Ok(_) => panic!("overflow submission must not succeed"),
            Err(other) => panic!("unexpected error on overflow: {:?}", other),
        }
    }
    assert_eq!(overflow_count, 50);

    // Produce the batch — drains all 500 (max_submissions_per_batch = 500)
    let (batch, receipt) = node
        .produce_batch(1, None)
        .expect("produce_batch must succeed");
    assert_eq!(
        batch.ordered_submissions.len(),
        500,
        "batch must contain exactly 500 submissions"
    );
    assert_eq!(receipt.accepted_count, 500);
    assert_eq!(receipt.rejected_count, 0);

    // 100 remain in queue (the ones we added in Phase 4 fill-up)
    assert_eq!(
        node.queue.len(),
        100,
        "100 remaining submissions must be in queue"
    );

    // Produce second batch — drains the remaining 100
    let (batch2, receipt2) = node
        .produce_batch(2, None)
        .expect("second produce_batch must succeed");
    assert_eq!(batch2.ordered_submissions.len(), 100);
    assert_eq!(receipt2.accepted_count, 100);
    assert_eq!(node.queue.len(), 0, "queue must be empty after second batch");
}
