//! Section 3 — Intent Transaction Model tests
//!
//! Tests: valid intent, invalid signature, expired, replayed nonce, invalid objects,
//! constraint violations, deterministic validation, admission pipeline.

use omniphi_runtime::intents::base::{
    AdmissionResult, ContractCallIntent, ContractConstraints, ExecutionMode, FeePolicy,
    IntentAdmissionPipeline, IntentConstraints, IntentTransaction, IntentType, NonceTracker,
    SponsorshipLimits,
};
use omniphi_runtime::intents::types::TransferIntent;
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::state::store::ObjectStore;
use std::collections::BTreeMap;

fn addr(n: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = n; b }
fn oid(n: u8) -> ObjectId { ObjectId::new(addr(n)) }

/// Create a valid signed transfer intent.
fn make_signed_transfer(seed: &[u8; 32], nonce: u64, epoch: u64) -> IntentTransaction {
    use ed25519_dalek::SigningKey;
    let key = SigningKey::from_bytes(seed);
    let sender = key.verifying_key().to_bytes();

    let mut tx = IntentTransaction {
        tx_id: {
            let mut id = [0u8; 32];
            id[0] = nonce as u8;
            id[1] = epoch as u8;
            id
        },
        sender,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: addr(0xAA),
            amount: 1000,
            recipient: addr(0xBB),
            memo: None,
        }),
        max_fee: 100,
        deadline_epoch: epoch + 10,
        nonce,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
    };
    tx.signature = tx.sign(seed);
    tx
}

// ──────────────────────────────────────────────
// Valid intent acceptance
// ──────────────────────────────────────────────

#[test]
fn test_valid_intent_accepted() {
    let seed = [42u8; 32];
    let tx = make_signed_transfer(&seed, 0, 5);
    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Accepted => {}
        AdmissionResult::Rejected(r) => panic!("Expected accepted, got rejected: {}", r),
    }
}

// ──────────────────────────────────────────────
// Invalid signature
// ──────────────────────────────────────────────

#[test]
fn test_invalid_signature_rejected() {
    let seed = [42u8; 32];
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.signature[0] ^= 0xFF; // corrupt signature

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("signature"), "Expected sig error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for bad signature"),
    }
}

// ──────────────────────────────────────────────
// Expired intent
// ──────────────────────────────────────────────

#[test]
fn test_expired_intent_rejected() {
    let seed = [42u8; 32];
    let tx = make_signed_transfer(&seed, 0, 5); // deadline = 15

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(20); // current epoch past deadline

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("expired"), "Expected expired, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for expired intent"),
    }
}

// ──────────────────────────────────────────────
// Replayed nonce
// ──────────────────────────────────────────────

#[test]
fn test_replayed_nonce_rejected() {
    let seed = [42u8; 32];
    let tx1 = make_signed_transfer(&seed, 0, 5);
    let tx2 = make_signed_transfer(&seed, 0, 5); // same nonce

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    // First admission succeeds
    match pipeline.admit(&tx1, &store) {
        AdmissionResult::Accepted => {}
        AdmissionResult::Rejected(r) => panic!("First should succeed: {}", r),
    }

    // Second admission with same nonce is rejected
    match pipeline.admit(&tx2, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("nonce"), "Expected nonce error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for replayed nonce"),
    }
}

#[test]
fn test_sequential_nonces_accepted() {
    let seed = [42u8; 32];
    let tx0 = make_signed_transfer(&seed, 0, 5);
    let tx1 = make_signed_transfer(&seed, 1, 5);
    let tx2 = make_signed_transfer(&seed, 2, 5);

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    assert!(matches!(pipeline.admit(&tx0, &store), AdmissionResult::Accepted));
    assert!(matches!(pipeline.admit(&tx1, &store), AdmissionResult::Accepted));
    assert!(matches!(pipeline.admit(&tx2, &store), AdmissionResult::Accepted));
}

#[test]
fn test_out_of_order_nonce_rejected() {
    let seed = [42u8; 32];
    let tx2 = make_signed_transfer(&seed, 2, 5); // skip nonce 0 and 1

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx2, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("nonce"), "Expected nonce error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for out-of-order nonce"),
    }
}

// ──────────────────────────────────────────────
// Invalid object targets
// ──────────────────────────────────────────────

#[test]
fn test_nonexistent_target_object_rejected() {
    let seed = [42u8; 32];
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.target_objects = vec![oid(0xFF)]; // object not in store

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("not found"), "Expected not found, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for missing target object"),
    }
}

#[test]
fn test_existing_target_object_accepted() {
    let seed = [42u8; 32];
    let mut tx = make_signed_transfer(&seed, 0, 5);
    let bal_id = oid(0x10);
    tx.target_objects = vec![bal_id];

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(bal_id, addr(1), addr(0xAA), 1000, 0)));

    let mut pipeline = IntentAdmissionPipeline::new(5);
    assert!(matches!(pipeline.admit(&tx, &store), AdmissionResult::Accepted));
}

// ──────────────────────────────────────────────
// Constraint violations
// ──────────────────────────────────────────────

#[test]
fn test_max_slippage_exceeds_10000_rejected() {
    let seed = [42u8; 32];
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.constraints.max_slippage_bps = Some(15000); // > 10000

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("slippage"), "Expected slippage error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for excessive slippage"),
    }
}

#[test]
fn test_required_version_mismatch_rejected() {
    let seed = [42u8; 32];
    let bal_id = oid(0x10);
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.constraints.required_versions = vec![(bal_id, 5)]; // require version 5

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(bal_id, addr(1), addr(0xAA), 1000, 0))); // version 0

    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("version"), "Expected version error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for version mismatch"),
    }
}

#[test]
fn test_allowed_objects_restriction() {
    let seed = [42u8; 32];
    let bal_id = oid(0x10);
    let other_id = oid(0x20);
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.target_objects = vec![bal_id];
    tx.constraints.allowed_objects = vec![other_id]; // bal_id not in allowed list

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(bal_id, addr(1), addr(0xAA), 1000, 0)));

    let mut pipeline = IntentAdmissionPipeline::new(5);

    match pipeline.admit(&tx, &store) {
        AdmissionResult::Rejected(r) => assert!(r.contains("allowed_objects"), "Expected allowed error, got: {}", r),
        AdmissionResult::Accepted => panic!("Expected rejected for object not in allowed list"),
    }
}

// ──────────────────────────────────────────────
// Deterministic validation
// ──────────────────────────────────────────────

#[test]
fn test_deterministic_validation_result() {
    let seed = [42u8; 32];

    // Run admission 100 times with same input — must always produce same result
    for _ in 0..100 {
        let tx = make_signed_transfer(&seed, 0, 5);
        let store = ObjectStore::new();
        let mut pipeline = IntentAdmissionPipeline::new(5);
        assert!(matches!(pipeline.admit(&tx, &store), AdmissionResult::Accepted));
    }
}

#[test]
fn test_signing_payload_deterministic() {
    let seed = [42u8; 32];
    let tx = make_signed_transfer(&seed, 0, 5);

    let p1 = tx.signing_payload();
    let p2 = tx.signing_payload();
    assert_eq!(p1, p2, "signing_payload must be deterministic");
}

// ──────────────────────────────────────────────
// Nonce tracker unit tests
// ──────────────────────────────────────────────

#[test]
fn test_nonce_tracker_advance() {
    let mut tracker = NonceTracker::new();
    let sender = addr(1);

    assert_eq!(tracker.expected_nonce(&sender), 0);
    assert!(tracker.is_valid(&sender, 0));
    assert!(!tracker.is_valid(&sender, 1));

    tracker.advance(&sender, 0).unwrap();
    assert_eq!(tracker.expected_nonce(&sender), 1);
    assert!(tracker.is_valid(&sender, 1));
    assert!(!tracker.is_valid(&sender, 0)); // replay blocked
}

#[test]
fn test_nonce_tracker_multiple_senders() {
    let mut tracker = NonceTracker::new();
    let alice = addr(1);
    let bob = addr(2);

    tracker.advance(&alice, 0).unwrap();
    tracker.advance(&alice, 1).unwrap();
    tracker.advance(&bob, 0).unwrap();

    assert_eq!(tracker.expected_nonce(&alice), 2);
    assert_eq!(tracker.expected_nonce(&bob), 1);
}

// ──────────────────────────────────────────────
// Structural validation
// ──────────────────────────────────────────────

#[test]
fn test_zero_sender_rejected() {
    let mut tx = IntentTransaction {
        tx_id: [1u8; 32],
        sender: [0u8; 32], // invalid
        intent: IntentType::Transfer(TransferIntent {
            asset_id: addr(0xAA), amount: 100, recipient: addr(0xBB), memo: None,
        }),
        max_fee: 10,
        deadline_epoch: 100,
        nonce: 0,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
    };

    assert!(tx.validate().is_err());
}

#[test]
fn test_zero_max_fee_rejected() {
    let tx = IntentTransaction {
        tx_id: [1u8; 32],
        sender: addr(1),
        intent: IntentType::Transfer(TransferIntent {
            asset_id: addr(0xAA), amount: 100, recipient: addr(0xBB), memo: None,
        }),
        max_fee: 0, // invalid
        deadline_epoch: 100,
        nonce: 0,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
    };

    assert!(tx.validate().is_err());
}

// ──────────────────────────────────────────────
// Execution mode
// ──────────────────────────────────────────────

#[test]
fn test_execution_mode_default_is_best_effort() {
    assert_eq!(ExecutionMode::default(), ExecutionMode::BestEffort);
}

#[test]
fn test_execution_mode_fill_or_kill() {
    let seed = [42u8; 32];
    let mut tx = make_signed_transfer(&seed, 0, 5);
    tx.execution_mode = ExecutionMode::FillOrKill;

    let store = ObjectStore::new();
    let mut pipeline = IntentAdmissionPipeline::new(5);
    // Execution mode doesn't affect admission — it's solver-facing
    assert!(matches!(pipeline.admit(&tx, &store), AdmissionResult::Accepted));
}
