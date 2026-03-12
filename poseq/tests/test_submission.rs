use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_payload() -> (Vec<u8>, [u8; 32]) {
    let body = b"test payload body".to_vec();
    let hash = Sha256::digest(&body);
    let mut h = [0u8; 32];
    h.copy_from_slice(&hash);
    (body, h)
}

fn make_submission(sender_byte: u8, nonce: u64) -> SequencingSubmission {
    let (body, hash) = make_payload();
    let mut sender = [0u8; 32];
    sender[0] = sender_byte;

    // compute a real submission_id (non-zero)
    let mut sub_id = [0u8; 32];
    sub_id[0] = sender_byte;
    sub_id[1] = (nonce & 0xff) as u8;
    sub_id[2] = 0xAB;  // make it non-zero

    SequencingSubmission {
        submission_id: sub_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class: SubmissionClass::Transfer,
        payload_hash: hash,
        payload_body: body,
        nonce,
        max_fee: 100,
        deadline_epoch: 999,
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
fn test_compute_id_deterministic() {
    let sub1 = make_submission(1, 42);
    let sub2 = make_submission(1, 42);
    assert_eq!(sub1.compute_id(), sub2.compute_id());
}

#[test]
fn test_zero_submission_id_invalid() {
    use omniphi_poseq::intake::receiver::SubmissionReceiver;
    use omniphi_poseq::validation::validator::SubmissionValidator;
    use omniphi_poseq::errors::ValidationError;

    let (body, hash) = make_payload();
    let mut sender = [0u8; 32];
    sender[0] = 1;
    let sub = SequencingSubmission {
        submission_id: [0u8; 32],  // zero ID
        sender,
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
    let mut receiver = SubmissionReceiver::new();
    let envelope = receiver.receive(sub);
    let policy = PoSeqPolicy::default_policy();
    let result = SubmissionValidator::validate(envelope, &policy);
    assert!(matches!(result, Err(ValidationError::ZeroSubmissionId)));
}

#[test]
fn test_payload_hash_validation() {
    let mut sub = make_submission(1, 1);
    // corrupt the hash
    sub.payload_hash[0] ^= 0xFF;
    assert!(!sub.validate_payload_hash());

    let sub_ok = make_submission(1, 1);
    assert!(sub_ok.validate_payload_hash());
}

#[test]
fn test_submission_kind_class_mapping() {
    assert_eq!(SubmissionKind::GoalPacket.to_class(), SubmissionClass::GoalPacket);
    assert_eq!(SubmissionKind::AgentSubmission.to_class(), SubmissionClass::AgentSubmission);
    assert_eq!(SubmissionKind::CandidatePlan.to_class(), SubmissionClass::Swap);
    assert_eq!(SubmissionKind::IntentTransaction.to_class(), SubmissionClass::Transfer);
    assert_eq!(SubmissionKind::RawBytes.to_class(), SubmissionClass::Other("raw".into()));
}

#[test]
fn test_intake_sequence_increments() {
    let mut receiver = SubmissionReceiver::new();
    let e1 = receiver.receive(make_submission(1, 1));
    let e2 = receiver.receive(make_submission(2, 2));
    let e3 = receiver.receive(make_submission(3, 3));
    assert_eq!(e1.received_at_sequence, 0);
    assert_eq!(e2.received_at_sequence, 1);
    assert_eq!(e3.received_at_sequence, 2);
}
