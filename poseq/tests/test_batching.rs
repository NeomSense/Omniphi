use omniphi_poseq::batching::builder::{BatchBuilder, OrderedBatch};
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::errors::BatchingError;
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_validated_sub(sender_byte: u8, nonce: u64, class: SubmissionClass) -> ValidatedSubmission {
    let body = vec![sender_byte, (nonce & 0xFF) as u8, 0x22];
    let hash = {
        let h = Sha256::digest(&body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    let mut sender = [0u8; 32];
    sender[0] = sender_byte;
    let mut sub_id = [0u8; 32];
    sub_id[0] = sender_byte;
    sub_id[1] = (nonce & 0xFF) as u8;
    sub_id[31] = 0x02;

    let sub = SequencingSubmission {
        submission_id: sub_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class,
        payload_hash: hash,
        payload_body: body,
        nonce,
        max_fee: 50,
        deadline_epoch: 100,
        metadata: SubmissionMetadata {
            sequence_hint: nonce,
            priority_hint: 0,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    };

    let mut receiver = SubmissionReceiver::new();
    let envelope = receiver.receive(sub);
    let policy = PoSeqPolicy::default_policy();
    SubmissionValidator::validate(envelope, &policy).unwrap()
}

fn make_three_subs() -> Vec<ValidatedSubmission> {
    vec![
        make_validated_sub(1, 1, SubmissionClass::Transfer),
        make_validated_sub(2, 2, SubmissionClass::Swap),
        make_validated_sub(3, 3, SubmissionClass::GoalPacket),
    ]
}

#[test]
fn test_batch_id_deterministic() {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let subs = make_three_subs();
    let b1 = builder.build(subs.clone(), 1, None, 10, None).unwrap();
    let b2 = builder.build(subs, 1, None, 10, None).unwrap();
    assert_eq!(b1.header.batch_id, b2.header.batch_id);
}

#[test]
fn test_payload_root_deterministic() {
    let subs = make_three_subs();
    let root1 = OrderedBatch::compute_payload_root(&subs);
    let root2 = OrderedBatch::compute_payload_root(&subs);
    assert_eq!(root1, root2);
    assert_ne!(root1, [0u8; 32]);
}

#[test]
fn test_batch_builder_correct_count() {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let subs = make_three_subs();
    let batch = builder.build(subs, 1, None, 1, None).unwrap();
    assert_eq!(batch.header.submission_count, 3);
    assert_eq!(batch.ordered_submissions.len(), 3);
}

#[test]
fn test_batch_size_exceeded_rejected() {
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_submissions_per_batch = 2;
    let builder = BatchBuilder::new(policy);
    let subs = make_three_subs();
    let result = builder.build(subs, 1, None, 1, None);
    assert!(matches!(result, Err(BatchingError::BatchSizeExceeded { count: 3, max: 2 })));
}

#[test]
fn test_empty_batch_rejected() {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let result = builder.build(vec![], 1, None, 1, None);
    assert!(matches!(result, Err(BatchingError::EmptyOrderedSet)));
}

#[test]
fn test_class_counts_in_metadata() {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let subs = make_three_subs();
    let batch = builder.build(subs, 1, None, 1, None).unwrap();
    // Should have 3 distinct class entries
    assert_eq!(batch.metadata.class_counts.len(), 3);
    // Each class appears once
    for (_, count) in &batch.metadata.class_counts {
        assert_eq!(*count, 1);
    }
}
