use omniphi_poseq::batching::builder::BatchBuilder;
use omniphi_poseq::bridge::runtime::RuntimeBridge;
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::errors::BridgeError;
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_validated_sub(sender_byte: u8, nonce: u64, class: SubmissionClass) -> ValidatedSubmission {
    let body = vec![sender_byte, (nonce & 0xFF) as u8, 0x33];
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
    sub_id[31] = 0x03;

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

fn make_batch() -> omniphi_poseq::batching::builder::OrderedBatch {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let subs = vec![
        make_validated_sub(1, 1, SubmissionClass::Transfer),
        make_validated_sub(2, 2, SubmissionClass::Swap),
        make_validated_sub(3, 3, SubmissionClass::GoalPacket),
    ];
    builder.build(subs, 1, None, 1, None).unwrap()
}

#[test]
fn test_runtime_bridge_export_preserves_order() {
    let batch = make_batch();
    let expected_ids: Vec<[u8; 32]> = batch.ordered_submissions.iter()
        .map(|s| s.envelope.normalized_id)
        .collect();

    let export = RuntimeBridge::export(&batch).unwrap();
    assert_eq!(export.envelope.ordered_submission_ids, expected_ids);
}

#[test]
fn test_runtime_bridge_empty_batch_fails() {
    // Construct a batch and make its submissions empty (simulate post-drain)
    // Instead, construct a manual OrderedBatch with empty submissions
    use omniphi_poseq::batching::builder::{BatchHeader, BatchMetadata, OrderedBatch};
    use omniphi_poseq::attestation::record::BatchAttestation;

    let header = BatchHeader {
        batch_id: [1u8; 32],
        height: 1,
        parent_batch_id: None,
        submission_count: 0,
        payload_root: [0u8; 32],
        policy_version: 1,
        epoch: 1,
    };
    let metadata = BatchMetadata {
        ordering_policy_version: 1,
        class_counts: BTreeMap::new(),
        total_payload_bytes: 0,
        sequencer_id: None,
    };
    let empty_batch = OrderedBatch {
        header,
        ordered_submissions: vec![],
        metadata,
        attestation: BatchAttestation::placeholder([1u8; 32]),
    };

    let result = RuntimeBridge::export(&empty_batch);
    assert!(matches!(result, Err(BridgeError::EmptyBatch)));
}

#[test]
fn test_export_payload_hashes_match_submissions() {
    let batch = make_batch();
    let expected_hashes: Vec<[u8; 32]> = batch.ordered_submissions.iter()
        .map(|s| s.envelope.submission.payload_hash)
        .collect();

    let export = RuntimeBridge::export(&batch).unwrap();
    assert_eq!(export.envelope.ordered_payload_hashes, expected_hashes);
}

#[test]
fn test_mock_ack_success() {
    let batch = make_batch();
    let sub_count = batch.ordered_submissions.len();
    let export = RuntimeBridge::export(&batch).unwrap();
    let ack = RuntimeBridge::mock_ack(&export);

    assert!(ack.success);
    assert_eq!(ack.processed_count, sub_count);
    assert_eq!(ack.failed_count, 0);
    assert!(ack.error.is_none());
    assert_eq!(ack.batch_id, export.envelope.batch_id);
}
