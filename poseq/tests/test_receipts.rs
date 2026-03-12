use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::receipts::receipt::SequencingReceipt;
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_validated(sender_byte: u8, nonce: u64) -> ValidatedSubmission {
    let body = vec![sender_byte, (nonce & 0xFF) as u8, 0x44];
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
    sub_id[31] = 0x04;

    let sub = SequencingSubmission {
        submission_id: sub_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class: SubmissionClass::Transfer,
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

#[test]
fn test_receipt_id_deterministic() {
    let batch_id = [0xABu8; 32];
    let subs = vec![make_validated(1, 1), make_validated(2, 2)];
    let r1 = SequencingReceipt::build(batch_id, 1, &subs, &[], 1);
    let r2 = SequencingReceipt::build(batch_id, 1, &subs, &[], 1);
    assert_eq!(r1.receipt_id, r2.receipt_id);
    assert_eq!(r1.receipt_hash, r2.receipt_hash);
}

#[test]
fn test_receipt_counts_correct() {
    let batch_id = [0x01u8; 32];
    let subs = vec![make_validated(1, 1), make_validated(2, 2), make_validated(3, 3)];
    let rejected: Vec<([u8; 32], String)> = vec![
        ([0xFFu8; 32], "bad submission".into()),
    ];
    let receipt = SequencingReceipt::build(batch_id, 1, &subs, &rejected, 1);
    assert_eq!(receipt.accepted_count, 3);
    assert_eq!(receipt.rejected_count, 1);
}

#[test]
fn test_receipt_includes_rejected() {
    let batch_id = [0x02u8; 32];
    let subs = vec![make_validated(1, 1)];
    let bad_id = [0xDEu8; 32];
    let rejected = vec![(bad_id, "invalid nonce".into())];
    let receipt = SequencingReceipt::build(batch_id, 1, &subs, &rejected, 1);

    let rejected_decision = receipt.decisions.iter().find(|d| d.submission_id == bad_id);
    assert!(rejected_decision.is_some());
    let rd = rejected_decision.unwrap();
    assert!(rd.reason.as_deref() == Some("invalid nonce"));
}

#[test]
fn test_ordered_ids_match_accepted() {
    let batch_id = [0x03u8; 32];
    let subs = vec![make_validated(1, 1), make_validated(2, 2)];
    let expected_ids: Vec<[u8; 32]> = subs.iter().map(|s| s.envelope.normalized_id).collect();
    let receipt = SequencingReceipt::build(batch_id, 1, &subs, &[], 1);
    assert_eq!(receipt.ordered_ids, expected_ids);
}
