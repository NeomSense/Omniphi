use omniphi_poseq::config::policy::{
    OrderingPolicyConfig, PoSeqPolicy, SubmissionClass, TieBreakRule,
};
use omniphi_poseq::types::submission::{SequencingSubmission, SubmissionKind, SubmissionMetadata};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use omniphi_poseq::ordering::engine::OrderingEngine;
use omniphi_poseq::errors::OrderingError;
use sha2::{Sha256, Digest};
use std::collections::BTreeMap;

fn make_validated(
    sender_byte: u8,
    nonce: u64,
    class: SubmissionClass,
    priority_hint: u32,
) -> ValidatedSubmission {
    let body = vec![sender_byte, (nonce & 0xFF) as u8, 0x11];
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

    let sub = SequencingSubmission {
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
            priority_hint,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    };

    let mut receiver = SubmissionReceiver::new();
    let envelope = receiver.receive(sub);
    let policy = PoSeqPolicy::default_policy();
    SubmissionValidator::validate(envelope, &policy).expect("valid submission")
}

#[test]
fn test_higher_priority_class_ordered_first() {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());
    let transfer = make_validated(1, 1, SubmissionClass::Transfer, 0);
    let treasury = make_validated(2, 1, SubmissionClass::TreasuryRebalance, 0);
    let swap = make_validated(3, 1, SubmissionClass::Swap, 0);

    let ordered = engine.order(vec![transfer, swap, treasury]).unwrap();
    assert_eq!(ordered[0].normalized_class, SubmissionClass::TreasuryRebalance);
    assert_eq!(ordered[1].normalized_class, SubmissionClass::Swap);
    assert_eq!(ordered[2].normalized_class, SubmissionClass::Transfer);
}

#[test]
fn test_tie_break_lexicographic_ascending() {
    let engine = OrderingEngine::new(OrderingPolicyConfig {
        tie_break: TieBreakRule::LexicographicAscending,
        enforce_sender_nonce_order: false,
    });

    // Two transfers — ordering should be deterministic by submission_id
    let t1 = make_validated(0x10, 1, SubmissionClass::Transfer, 0);
    let t2 = make_validated(0x20, 1, SubmissionClass::Transfer, 0);

    let id1 = t1.envelope.normalized_id;
    let id2 = t2.envelope.normalized_id;

    let ordered = engine.order(vec![t1, t2]).unwrap();
    // Lower id should come first
    if id1 < id2 {
        assert_eq!(ordered[0].envelope.normalized_id, id1);
    } else {
        assert_eq!(ordered[0].envelope.normalized_id, id2);
    }
}

#[test]
fn test_same_sender_nonce_order_enforced() {
    let engine = OrderingEngine::new(OrderingPolicyConfig {
        tie_break: TieBreakRule::LexicographicAscending,
        enforce_sender_nonce_order: true,
    });

    // Same sender, same class — nonce 5 should come after nonce 1
    let s1 = make_validated(0xAA, 1, SubmissionClass::Transfer, 0);
    let s5 = make_validated(0xAA, 5, SubmissionClass::Transfer, 0);

    let ordered = engine.order(vec![s5, s1]).unwrap();
    // Lower nonce (1) should appear first
    assert_eq!(ordered[0].envelope.submission.nonce, 1);
    assert_eq!(ordered[1].envelope.submission.nonce, 5);
}

#[test]
fn test_ordering_deterministic_on_repeated_calls() {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());

    let subs: Vec<ValidatedSubmission> = (1u8..=5).map(|i| {
        make_validated(i, i as u64, SubmissionClass::Transfer, 0)
    }).collect();

    let ordered1 = engine.order(subs.clone()).unwrap();
    let ordered2 = engine.order(subs).unwrap();

    let ids1: Vec<[u8; 32]> = ordered1.iter().map(|s| s.envelope.normalized_id).collect();
    let ids2: Vec<[u8; 32]> = ordered2.iter().map(|s| s.envelope.normalized_id).collect();
    assert_eq!(ids1, ids2);
}

#[test]
fn test_empty_input_returns_error() {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());
    let result = engine.order(vec![]);
    assert!(matches!(result, Err(OrderingError::EmptyInput)));
}

#[test]
fn test_priority_hint_influences_ordering() {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());

    // Both Transfer class, but one has a high priority hint
    let low = make_validated(0x01, 1, SubmissionClass::Transfer, 0);
    let high = make_validated(0x02, 1, SubmissionClass::Transfer, 10000);

    let ordered = engine.order(vec![low, high]).unwrap();
    // High priority_hint submission should come first
    assert_eq!(ordered[0].envelope.submission.metadata.priority_hint, 10000);
}
