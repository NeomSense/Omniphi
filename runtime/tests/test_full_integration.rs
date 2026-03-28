//! Full Integration Test — Intent → PoSeq → Runtime → Settlement
//!
//! Validates the complete execution pipeline as a connected system.
//! This is the single most important test for production readiness.

use omniphi_runtime::capabilities::spending::SpendCapabilityRegistry;
use omniphi_runtime::intents::base::*;
use omniphi_runtime::intents::encrypted::{
    EncryptedIntent, EncryptedIntentRegistry, IntentReveal,
};
use omniphi_runtime::intents::sponsorship::{SponsorReplayTracker, SponsorshipValidator};
use omniphi_runtime::intents::types::TransferIntent;
use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId};
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::scheduler::parallel::ParallelScheduler;
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::safety::deterministic::{DeterministicSafetyEngine, SafetyConfig};
use omniphi_runtime::explainability::PreviewGenerator;
use omniphi_runtime::randomness::{EntropyEngine, RandomnessRequest, derive_randomness};
use omniphi_runtime::poseq::grpc_bridge::{BatchDelivery, BatchAck, InMemoryBridge, BridgeService};
use omniphi_runtime::poseq::mempool::IntentMempool;
use ed25519_dalek::SigningKey;
use std::collections::BTreeMap;

fn keypair(seed: u8) -> (SigningKey, [u8; 32]) {
    let mut s = [0u8; 32]; s[0] = seed;
    let sk = SigningKey::from_bytes(&s);
    let pk = sk.verifying_key().to_bytes();
    (sk, pk)
}

fn oid(v: u32) -> ObjectId {
    let mut b = [0u8; 32];
    b[0..4].copy_from_slice(&v.to_be_bytes());
    ObjectId(b)
}

fn make_transfer(sender: [u8; 32], nonce: u64, amount: u128, fee: u64) -> IntentTransaction {
    IntentTransaction {
        tx_id: {
            let mut id = [0u8; 32];
            id[0] = sender[0]; id[1..9].copy_from_slice(&nonce.to_be_bytes()); id
        },
        sender,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: [0xAA; 32], recipient: { let mut r = [0u8;32]; r[0]=99; r },
            amount, memo: None,
        }),
        max_fee: fee,
        deadline_epoch: 999,
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
    }
}

// ═════════════════════════════════════════════════════════════════════════════
// FULL PIPELINE: Mempool → Validate → Preview → Sponsor → Schedule → Settle
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_full_pipeline_transfer() {
    let (sk, pk) = keypair(1);

    // ── Step 1: User creates and signs intent ──
    let mut tx = make_transfer(pk, 1, 1000, 100);
    tx.signature = tx.sign(&sk.to_bytes());
    assert!(tx.validate().is_ok());
    assert!(tx.verify_signature());

    // ── Step 2: Submit to mempool ──
    let mut mempool = IntentMempool::new(1000);
    assert!(mempool.insert(tx.clone()));
    assert_eq!(mempool.len(), 1);

    // ── Step 3: Preview (explainability) ──
    let mut store = ObjectStore::new();
    let sender_bal_id = oid(1);
    let recip_bal_id = oid(99);
    let bal = BalanceObject::new(sender_bal_id, pk, [0xAA; 32], 10_000, 1);
    store.insert(Box::new(bal));

    let preview = PreviewGenerator::preview(&tx, &store, 10);
    assert!(preview.is_exact);
    assert!(preview.failure_conditions.is_empty());

    // ── Step 4: PoSeq orders the batch ──
    let batch = BatchDelivery {
        batch_id: [0x01; 32],
        sequence: 1,
        epoch: 1,
        submission_ids: vec![tx.tx_id],
        content_hash: [0xBB; 32],
        finalized_at_ms: 1000,
    };

    let mut bridge = InMemoryBridge::new();
    bridge.deliver_batch(batch).unwrap();
    assert_eq!(bridge.delivered_count(), 1);

    // ── Step 5: Resolve intent into execution plan ──
    let plan = ExecutionPlan {
        tx_id: tx.tx_id,
        operations: vec![
            ObjectOperation::DebitBalance { balance_id: sender_bal_id, amount: 1000 },
            ObjectOperation::CreditBalance { balance_id: recip_bal_id, amount: 1000 },
        ],
        required_capabilities: vec![],
        object_access: vec![
            ObjectAccess { object_id: sender_bal_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recip_bal_id, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 2000,
        gas_limit: 100_000,
    };

    // ── Step 6: Schedule (parallel grouping) ──
    let groups = ParallelScheduler::schedule(vec![plan]);
    assert_eq!(groups.len(), 1);
    assert_eq!(groups[0].plans.len(), 1);

    // ── Step 7: Settle (execute atomically) ──
    // Create recipient balance object first
    let recip_bal = BalanceObject::new(recip_bal_id, { let mut r = [0u8;32]; r[0]=99; r }, [0xAA; 32], 0, 1);
    store.insert(Box::new(recip_bal));

    let result = SettlementEngine::execute_groups(groups, &mut store, 1);
    assert_eq!(result.succeeded, 1);
    assert_eq!(result.failed, 0);
    assert_eq!(result.total_plans, 1);

    // ── Step 8: Verify state changes ──
    let sender_bal = store.find_balance(&pk, &[0xAA; 32]).unwrap();
    assert_eq!(sender_bal.amount, 9000); // 10000 - 1000

    let recip_bal = store.find_balance(&{ let mut r = [0u8;32]; r[0]=99; r }, &[0xAA; 32]).unwrap();
    assert_eq!(recip_bal.amount, 1000); // 0 + 1000

    // ── Step 9: State root is deterministic ──
    let root1 = result.state_root;
    // Re-execute same pipeline on fresh store → same root
    let mut store2 = ObjectStore::new();
    store2.insert(Box::new(BalanceObject::new(sender_bal_id, pk, [0xAA; 32], 10_000, 1)));
    store2.insert(Box::new(BalanceObject::new(recip_bal_id, { let mut r = [0u8;32]; r[0]=99; r }, [0xAA; 32], 0, 1)));

    let plan2 = ExecutionPlan {
        tx_id: tx.tx_id,
        operations: vec![
            ObjectOperation::DebitBalance { balance_id: sender_bal_id, amount: 1000 },
            ObjectOperation::CreditBalance { balance_id: recip_bal_id, amount: 1000 },
        ],
        required_capabilities: vec![],
        object_access: vec![
            ObjectAccess { object_id: sender_bal_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recip_bal_id, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 2000,
        gas_limit: 100_000,
    };
    let groups2 = ParallelScheduler::schedule(vec![plan2]);
    let result2 = SettlementEngine::execute_groups(groups2, &mut store2, 1);
    assert_eq!(root1, result2.state_root, "Same inputs must produce same state root");

    // ── Step 10: ACK back to PoSeq ──
    bridge.ack_batch(BatchAck {
        batch_id: [0x01; 32],
        sequence: 1,
        accepted: true,
        state_root: Some(root1),
        block_height: Some(100),
        rejection_reason: None,
    }).unwrap();

    let health = bridge.health();
    assert_eq!(health.pending_batches, 0);

    // ── Step 11: Safety check ──
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig::default());
    safety.record_outflow(1, sender_bal_id, 1000);
    assert!(!safety.is_frozen(&sender_bal_id)); // 1000 < default threshold

    // ── Step 12: Remove from mempool ──
    mempool.remove(&tx.tx_id);
    assert!(mempool.is_empty());
}

// ═════════════════════════════════════════════════════════════════════════════
// SPONSORED + ENCRYPTED PIPELINE
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_full_pipeline_sponsored_encrypted() {
    let (user_sk, user_pk) = keypair(1);
    let (sponsor_sk, sponsor_pk) = keypair(2);

    // ── Step 1: User creates intent ──
    let mut tx = make_transfer(user_pk, 1, 500, 100);
    tx.signature = tx.sign(&user_sk.to_bytes());

    // ── Step 2: Encrypt intent ──
    let reveal_nonce = [0xCC; 32];
    let mut ei_reg = EncryptedIntentRegistry::new();
    let ei = EncryptedIntent::create(&tx, &reveal_nonce, vec![0xDE, 0xAD], 100, 5).unwrap();
    let commitment = ei_reg.submit(ei).unwrap();

    // ── Step 3: PoSeq orders the encrypted intent (content hidden) ──
    // At this point, sequencer sees only commitment + ciphertext
    assert_eq!(ei_reg.pending_count(), 1);

    // ── Step 4: Reveal phase ──
    let reveal = IntentReveal {
        commitment,
        plaintext_intent: tx.clone(),
        reveal_nonce,
    };
    let revealed = ei_reg.reveal(&reveal, 50).unwrap();
    assert_eq!(revealed.sender, user_pk);
    assert_eq!(ei_reg.pending_count(), 0);

    // ── Step 5: Add sponsorship ──
    let mut revealed_tx = revealed;
    revealed_tx.sponsor = Some(sponsor_pk);
    revealed_tx.fee_policy = FeePolicy::SponsorPays;
    revealed_tx.sponsorship_limits = SponsorshipLimits {
        max_fee_amount: Some(500),
        ..Default::default()
    };
    let sponsor_sig = revealed_tx.sign_sponsorship(&sponsor_sk.to_bytes());
    revealed_tx.sponsor_signature = Some(sponsor_sig);

    // ── Step 6: Validate sponsorship ──
    let mut replay = SponsorReplayTracker::new();
    let validation = SponsorshipValidator::validate(&revealed_tx, 50, &mut replay);
    assert!(validation.valid, "Sponsor validation failed: {}", validation.reason);
    assert_eq!(validation.sponsor_pays_amount, 100);
    assert_eq!(validation.sender_pays_amount, 0);

    // ── Step 7: Create capability for the spend ──
    let mut cap_reg = SpendCapabilityRegistry::new();
    let scope = {
        use sha2::{Digest, Sha256};
        let mut h = Sha256::new();
        h.update(&revealed_tx.tx_id);
        let r = h.finalize();
        let mut s = [0u8; 32]; s.copy_from_slice(&r); s
    };
    let cap_id = cap_reg.create(
        user_pk, { let mut r = [0u8;32]; r[0]=99; r }, // recipient as spender
        [0xAA; 32], 500, 100, Some(scope), 10,
    ).unwrap();

    // ── Step 8: Consume capability with scope binding ──
    cap_reg.consume(&cap_id, &{ let mut r = [0u8;32]; r[0]=99; r }, 500, 50, Some(&scope)).unwrap();

    // ── Step 9: Full pipeline would continue to schedule → settle ──
    // (Already tested in test_full_pipeline_transfer)
}

// ═════════════════════════════════════════════════════════════════════════════
// PARALLEL BATCH: Multiple non-conflicting transfers
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_full_pipeline_parallel_batch() {
    let mut store = ObjectStore::new();

    // Create 10 sender-recipient pairs (20 balance objects)
    let mut plans = vec![];
    for i in 0..10u32 {
        let sender_id = oid(i * 2);
        let recip_id = oid(i * 2 + 1);
        let sender_addr = { let mut b = [0u8; 32]; b[0..4].copy_from_slice(&(i * 2).to_be_bytes()); b };
        let recip_addr = { let mut b = [0u8; 32]; b[0..4].copy_from_slice(&(i * 2 + 1).to_be_bytes()); b };

        store.insert(Box::new(BalanceObject::new(sender_id, sender_addr, [0xAA; 32], 5000, 1)));
        store.insert(Box::new(BalanceObject::new(recip_id, recip_addr, [0xAA; 32], 0, 1)));

        plans.push(ExecutionPlan {
            tx_id: { let mut t = [0u8; 32]; t[0..4].copy_from_slice(&i.to_be_bytes()); t },
            operations: vec![
                ObjectOperation::DebitBalance { balance_id: sender_id, amount: 1000 },
                ObjectOperation::CreditBalance { balance_id: recip_id, amount: 1000 },
            ],
            required_capabilities: vec![],
            object_access: vec![
                ObjectAccess { object_id: sender_id, mode: AccessMode::ReadWrite },
                ObjectAccess { object_id: recip_id, mode: AccessMode::ReadWrite },
            ],
            gas_estimate: 2000,
            gas_limit: 100_000,
        });
    }

    // All 10 plans are independent → should be in 1 group
    let groups = ParallelScheduler::schedule(plans);
    assert_eq!(groups.len(), 1, "10 independent transfers must parallelize into 1 group");
    assert_eq!(groups[0].plans.len(), 10);

    // Execute
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);
    assert_eq!(result.succeeded, 10);
    assert_eq!(result.failed, 0);

    // Verify all balances
    for i in 0..10u32 {
        let sender_addr = { let mut b = [0u8; 32]; b[0..4].copy_from_slice(&(i * 2).to_be_bytes()); b };
        let recip_addr = { let mut b = [0u8; 32]; b[0..4].copy_from_slice(&(i * 2 + 1).to_be_bytes()); b };

        let sender = store.find_balance(&sender_addr, &[0xAA; 32]).unwrap();
        assert_eq!(sender.amount, 4000, "Sender {} should have 4000", i);

        let recip = store.find_balance(&recip_addr, &[0xAA; 32]).unwrap();
        assert_eq!(recip.amount, 1000, "Recipient {} should have 1000", i);
    }
}

// ═════════════════════════════════════════════════════════════════════════════
// FEE MEMPOOL PRIORITY: Higher fees get processed first
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_mempool_fee_priority_integration() {
    let mut mempool = IntentMempool::new(5);
    let (sk, pk) = keypair(1);

    // Submit 5 intents with different fees
    for (i, fee) in [100u64, 500, 200, 1000, 50].iter().enumerate() {
        let mut tx = make_transfer(pk, i as u64, 100, *fee);
        tx.signature = tx.sign(&sk.to_bytes());
        assert!(mempool.insert(tx));
    }

    // Top 3 should be highest fees
    let top = mempool.top_n(3);
    assert_eq!(top.len(), 3);
    assert_eq!(top[0].max_fee, 1000);
    assert_eq!(top[1].max_fee, 500);
    assert_eq!(top[2].max_fee, 200);

    // Insert a higher-fee tx when full → should evict fee=50
    let mut high = make_transfer(pk, 99, 100, 2000);
    high.signature = high.sign(&sk.to_bytes());
    assert!(mempool.insert(high));
    assert_eq!(mempool.len(), 5);

    let top = mempool.top_n(1);
    assert_eq!(top[0].max_fee, 2000);
}

// ═════════════════════════════════════════════════════════════════════════════
// SAFETY + SETTLEMENT INTERACTION
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_safety_monitors_settlement_outflows() {
    let mut store = ObjectStore::new();
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig {
        max_object_outflow: 5000,
        max_epoch_outflow: 20_000,
        ..SafetyConfig::default()
    });

    let sender_id = oid(1);
    let recip_id = oid(2);
    store.insert(Box::new(BalanceObject::new(sender_id, [1u8; 32], [0xAA; 32], 100_000, 1)));
    store.insert(Box::new(BalanceObject::new(recip_id, [2u8; 32], [0xAA; 32], 0, 1)));

    // Execute 3 transfers of 2000 each = 6000 total from sender
    for i in 0..3u8 {
        let plan = ExecutionPlan {
            tx_id: { let mut t = [0u8; 32]; t[0] = i; t },
            operations: vec![
                ObjectOperation::DebitBalance { balance_id: sender_id, amount: 2000 },
                ObjectOperation::CreditBalance { balance_id: recip_id, amount: 2000 },
            ],
            required_capabilities: vec![],
            object_access: vec![
                ObjectAccess { object_id: sender_id, mode: AccessMode::ReadWrite },
                ObjectAccess { object_id: recip_id, mode: AccessMode::ReadWrite },
            ],
            gas_estimate: 2000,
            gas_limit: 100_000,
        };
        let groups = ParallelScheduler::schedule(vec![plan]);
        let result = SettlementEngine::execute_groups(groups, &mut store, 1);
        assert_eq!(result.succeeded, 1);

        // Report outflow to safety engine
        safety.record_outflow(1, sender_id, 2000);
    }

    // After 3 × 2000 = 6000 outflow, sender should be frozen (threshold 5000)
    assert!(safety.is_frozen(&sender_id));

    // But recipient is NOT frozen
    assert!(!safety.is_frozen(&recip_id));
}

// ═════════════════════════════════════════════════════════════════════════════
// ENTROPY + SETTLEMENT DETERMINISM
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_entropy_determinism_across_nodes() {
    // Two nodes process identical entropy inputs
    let mut e1 = EntropyEngine::new();
    let mut e2 = EntropyEngine::new();

    e1.set_epoch_seed(10, [0xAA; 32]);
    e2.set_epoch_seed(10, [0xAA; 32]);

    let ent1 = e1.aggregate(10);
    let ent2 = e2.aggregate(10);
    assert_eq!(ent1.seed, ent2.seed);

    // Derived randomness for tiebreaking must match
    for i in 0..100u64 {
        let req = RandomnessRequest { domain: "tiebreak".into(), epoch: 10, index: i, target_id: None };
        assert_eq!(
            derive_randomness(&ent1, &req),
            derive_randomness(&ent2, &req),
            "Randomness must be deterministic at index {}", i
        );
    }
}
