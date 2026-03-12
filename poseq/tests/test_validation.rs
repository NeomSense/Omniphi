use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::validation::validator::SubmissionValidator;
use omniphi_poseq::errors::ValidationError;
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_valid_submission(sender_byte: u8, nonce: u64, class: SubmissionClass) -> SequencingSubmission {
    let body = vec![1u8, 2, 3, 4, 5];
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
    sub_id[31] = 0x01;

    SequencingSubmission {
        submission_id: sub_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class,
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

fn validate(
    sub: SequencingSubmission,
    policy: &PoSeqPolicy,
) -> Result<omniphi_poseq::validation::validator::ValidatedSubmission, ValidationError> {
    let mut receiver = SubmissionReceiver::new();
    let envelope = receiver.receive(sub);
    SubmissionValidator::validate(envelope, policy)
}

#[test]
fn test_valid_submission_passes() {
    let sub = make_valid_submission(1, 1, SubmissionClass::Transfer);
    let policy = PoSeqPolicy::default_policy();
    assert!(validate(sub, &policy).is_ok());
}

#[test]
fn test_zero_sender_rejected() {
    let mut sub = make_valid_submission(1, 1, SubmissionClass::Transfer);
    sub.sender = [0u8; 32];
    let policy = PoSeqPolicy::default_policy();
    assert!(matches!(validate(sub, &policy), Err(ValidationError::ZeroSender)));
}

#[test]
fn test_zero_payload_hash_rejected() {
    let mut sub = make_valid_submission(1, 1, SubmissionClass::Transfer);
    sub.payload_hash = [0u8; 32];
    let policy = PoSeqPolicy::default_policy();
    assert!(matches!(validate(sub, &policy), Err(ValidationError::EmptyPayloadHash)));
}

#[test]
fn test_payload_too_large_rejected() {
    let mut sub = make_valid_submission(1, 1, SubmissionClass::Transfer);
    // Create a policy with small max payload
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_payload_bytes_per_submission = 4;
    // payload body is 10 bytes — recompute hash
    sub.payload_body = vec![0u8; 10];
    let hash = {
        let h = Sha256::digest(&sub.payload_body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    sub.payload_hash = hash;
    assert!(matches!(
        validate(sub, &policy),
        Err(ValidationError::PayloadTooLarge { size: 10, max: 4 })
    ));
}

#[test]
fn test_disallowed_class_rejected() {
    let sub = make_valid_submission(1, 1, SubmissionClass::AgentSubmission);
    // Create policy that disallows AgentSubmission
    let mut policy = PoSeqPolicy::default_policy();
    for p in &mut policy.class_policies {
        if p.class == SubmissionClass::AgentSubmission {
            p.allowed = false;
        }
    }
    assert!(matches!(
        validate(sub, &policy),
        Err(ValidationError::InvalidPayloadKind(_))
    ));
}

#[test]
fn test_payload_hash_mismatch_rejected() {
    let mut sub = make_valid_submission(1, 1, SubmissionClass::Transfer);
    // Corrupt payload_body without updating hash
    sub.payload_body[0] ^= 0xFF;
    let policy = PoSeqPolicy::default_policy();
    assert!(matches!(
        validate(sub, &policy),
        Err(ValidationError::MalformedEnvelope(_))
    ));
}

#[test]
fn test_class_allowed_by_default_policy() {
    let policy = PoSeqPolicy::default_policy();
    assert!(policy.is_class_allowed(&SubmissionClass::Transfer));
    assert!(policy.is_class_allowed(&SubmissionClass::Swap));
    assert!(policy.is_class_allowed(&SubmissionClass::GoalPacket));
    assert!(policy.is_class_allowed(&SubmissionClass::AgentSubmission));
    // Unknown class not in policy should default to allowed=true
    assert!(policy.is_class_allowed(&SubmissionClass::Other("unknown".into())));
}
