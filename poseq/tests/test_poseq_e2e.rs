use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::errors::PoSeqError;
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::PoSeqNode;
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_submission(sender_byte: u8, nonce: u64, class: SubmissionClass) -> SequencingSubmission {
    let body = vec![sender_byte, (nonce & 0xFF) as u8, class.priority_weight() as u8, 0xE2];
    let hash = {
        let h = Sha256::digest(&body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    let mut sender = [0u8; 32];
    sender[0] = sender_byte;
    // Ensure non-zero submission_id
    let mut sub_id = [0u8; 32];
    sub_id[0] = sender_byte;
    sub_id[1] = (nonce & 0xFF) as u8;
    sub_id[2] = class.priority_weight() as u8;
    sub_id[31] = 0x05;

    SequencingSubmission {
        submission_id: sub_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class,
        payload_hash: hash,
        payload_body: body,
        nonce,
        max_fee: 100,
        deadline_epoch: 9999,
        metadata: SubmissionMetadata {
            sequence_hint: nonce,
            priority_hint: 0,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    }
}

#[test]
fn test_e2e_full_lifecycle() {
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    // Submit 5 valid submissions
    for i in 1u8..=5 {
        let sub = make_submission(i, i as u64, SubmissionClass::Transfer);
        let id = node.submit(sub).expect("submit should succeed");
        assert_ne!(id, [0u8; 32]);
    }
    assert_eq!(node.queue.len(), 5);

    // Produce batch
    let (batch, receipt) = node.produce_batch(1, None).expect("produce_batch should succeed");
    assert_eq!(batch.ordered_submissions.len(), 5);
    assert_eq!(receipt.accepted_count, 5);
    assert_eq!(receipt.rejected_count, 0);
    assert_eq!(batch.header.height, 1);
    assert_eq!(node.state.current_height, 1);
    assert_eq!(node.queue.len(), 0);

    // Export to runtime
    let export = node.export_to_runtime(&batch).expect("export should succeed");
    assert_eq!(export.envelope.ordered_payload_hashes.len(), 5);
    assert_eq!(export.envelope.batch_id, batch.header.batch_id);

    // Verify ledger
    assert_eq!(node.ledger.len(), 1);
    let audit = node.ledger.get(1).unwrap();
    assert_eq!(audit.batch_id, batch.header.batch_id);
}

#[test]
fn test_e2e_duplicate_rejected() {
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    let sub = make_submission(0x01, 1, SubmissionClass::Transfer);
    // Clone the original before submitting
    let sub_clone = sub.clone();

    let _id = node.submit(sub).expect("first submit should succeed");

    // Submit the same submission again — should be rejected as duplicate
    let result = node.submit(sub_clone);
    assert!(matches!(result, Err(PoSeqError::Duplicate(_))));

    // Queue should only have 1 entry
    assert_eq!(node.queue.len(), 1);
}

#[test]
fn test_e2e_ordering_by_class() {
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    // Submit in this order: Transfer, TreasuryRebalance, Swap
    node.submit(make_submission(0x01, 1, SubmissionClass::Transfer)).unwrap();
    node.submit(make_submission(0x02, 1, SubmissionClass::TreasuryRebalance)).unwrap();
    node.submit(make_submission(0x03, 1, SubmissionClass::Swap)).unwrap();

    let (batch, _) = node.produce_batch(1, None).unwrap();

    // Treasury should be first (priority 1000 > Swap 600 > Transfer 400)
    assert_eq!(batch.ordered_submissions[0].normalized_class, SubmissionClass::TreasuryRebalance);
    assert_eq!(batch.ordered_submissions[1].normalized_class, SubmissionClass::Swap);
    assert_eq!(batch.ordered_submissions[2].normalized_class, SubmissionClass::Transfer);
}

#[test]
fn test_e2e_batch_deterministic_on_replay() {
    // Build two identical PoSeqNodes with the same submissions
    let make_node = || {
        let policy = PoSeqPolicy::default_policy();
        let mut node = PoSeqNode::new(policy);
        for i in 1u8..=3 {
            node.submit(make_submission(i, i as u64, SubmissionClass::Transfer)).unwrap();
        }
        node
    };

    let mut node1 = make_node();
    let mut node2 = make_node();

    let (b1, _) = node1.produce_batch(5, None).unwrap();
    let (b2, _) = node2.produce_batch(5, None).unwrap();

    // Same inputs → same batch_id
    assert_eq!(b1.header.batch_id, b2.header.batch_id);
    assert_eq!(b1.header.payload_root, b2.header.payload_root);
}

#[test]
fn test_e2e_invalid_submission_not_queued() {
    let policy = PoSeqPolicy::default_policy();
    let mut node = PoSeqNode::new(policy);

    // Zero sender
    let body = b"some body".to_vec();
    let hash = {
        let h = Sha256::digest(&body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    let bad_sub = SequencingSubmission {
        submission_id: [0xABu8; 32],
        sender: [0u8; 32],  // zero sender — invalid
        kind: SubmissionKind::IntentTransaction,
        class: SubmissionClass::Transfer,
        payload_hash: hash,
        payload_body: body,
        nonce: 1,
        max_fee: 100,
        deadline_epoch: 999,
        metadata: SubmissionMetadata {
            sequence_hint: 1,
            priority_hint: 0,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    };

    let result = node.submit(bad_sub);
    assert!(result.is_err());
    assert_eq!(node.queue.len(), 0);
}

#[test]
fn test_e2e_queue_overflow() {
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_pending_queue_size = 3;
    let mut node = PoSeqNode::new(policy);

    // Fill the queue to capacity
    for i in 1u8..=3 {
        node.submit(make_submission(i, i as u64, SubmissionClass::Transfer)).unwrap();
    }
    assert_eq!(node.queue.len(), 3);

    // Next submission should fail with QueueFull
    let result = node.submit(make_submission(0xFF, 99, SubmissionClass::Transfer));
    assert!(matches!(result, Err(PoSeqError::QueueFull { capacity: 3 })));
}

#[test]
fn test_e2e_sequencing_height_advances() {
    let policy = PoSeqPolicy::default_policy();
    // Need at least 1 submission per batch
    let policy_clone = policy.clone();
    let mut node = PoSeqNode::new(policy_clone);

    for batch_num in 1u8..=3 {
        // Add a fresh submission per batch
        node.submit(make_submission(batch_num, batch_num as u64, SubmissionClass::Transfer)).unwrap();
        let (batch, _) = node.produce_batch(batch_num as u64, None).unwrap();
        assert_eq!(batch.header.height, batch_num as u64);
    }

    assert_eq!(node.state.current_height, 3);
    assert_eq!(node.ledger.len(), 3);

    // Verify ledger heights
    assert!(node.ledger.get(1).is_some());
    assert!(node.ledger.get(2).is_some());
    assert!(node.ledger.get(3).is_some());
}
