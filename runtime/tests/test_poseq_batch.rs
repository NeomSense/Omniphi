use omniphi_runtime::intents::base::{IntentTransaction, IntentType, IntentConstraints, ExecutionMode, FeePolicy, SponsorshipLimits};
use omniphi_runtime::intents::types::{SwapIntent, TransferIntent};
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::{BalanceObject, LiquidityPoolObject, WalletObject};
use omniphi_runtime::poseq::interface::{OrderedBatch, PoSeqRuntime};
use std::collections::BTreeMap;

fn id(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}
fn addr(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0] = n;
    b
}
fn txid(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[31] = n;
    b
}
fn batch_id(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0] = 0xBA;
    b[1] = n;
    b
}

// ──────────────────────────────────────────────
// Full end-to-end: 3 intents in one batch
// ──────────────────────────────────────────────

#[test]
fn test_full_e2e_batch_with_three_intents() {
    let alice = addr(0xA1);
    let bob = addr(0xB2);
    let carol = addr(0xC3);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    // Object IDs
    let alice_wallet_id = id(1);
    let alice_a_id = id(2);
    let alice_b_id = id(3);
    let bob_a_id = id(4);
    let carol_a_id = id(5);
    let pool_id = id(10);

    let mut runtime = PoSeqRuntime::new();

    // Seed objects
    runtime.seed_object(Box::new(WalletObject::new(alice_wallet_id, alice, alice, 0)));
    runtime.seed_object(Box::new(BalanceObject::new(alice_a_id, alice, asset_a, 100_000, 0)));
    runtime.seed_object(Box::new(BalanceObject::new(alice_b_id, alice, asset_b, 0, 0)));
    runtime.seed_object(Box::new(BalanceObject::new(bob_a_id, bob, asset_a, 50_000, 0)));
    runtime.seed_object(Box::new(BalanceObject::new(carol_a_id, carol, asset_a, 0, 0)));
    runtime.seed_object(Box::new(LiquidityPoolObject::new(
        pool_id,
        alice,
        asset_a,
        asset_b,
        1_000_000,
        1_000_000,
        30,       // 0.3% fee
        addr(0xEE),
        0,
    )));

    // Intent 1: Alice transfers 10,000 asset_a to Carol
    let tx1 = IntentTransaction {
        tx_id: txid(1),
        sender: alice,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset_a,
            amount: 10_000,
            recipient: carol,
            memo: Some("payment".to_string()),
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
    };

    // Intent 2: Bob transfers 5,000 asset_a to Carol
    let tx2 = IntentTransaction {
        tx_id: txid(2),
        sender: bob,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset_a,
            amount: 5_000,
            recipient: carol,
            memo: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
    };

    // Intent 3: Alice swaps 1,000 asset_a → asset_b
    let tx3 = IntentTransaction {
        tx_id: txid(3),
        sender: alice,
        intent: IntentType::Swap(SwapIntent {
            input_asset: asset_a,
            output_asset: asset_b,
            input_amount: 1_000,
            min_output_amount: 1,
            max_slippage_bps: 500,
            allowed_pool_ids: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 2,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
    };

    let batch = OrderedBatch {
        batch_id: batch_id(1),
        epoch: 1,
        sequence_number: 0,
        transactions: vec![tx1, tx2, tx3],
    };

    let result = runtime.process_batch(batch).expect("batch processing must succeed");

    // ── Verify SettlementResult ──
    assert_eq!(result.epoch, 1);
    assert_eq!(result.total_plans, 3, "all 3 intents should resolve to plans");
    assert_eq!(result.succeeded, 3, "all 3 plans should succeed");
    assert_eq!(result.failed, 0);
    assert_eq!(result.receipts.len(), 3);

    // All receipts should be successful
    for receipt in &result.receipts {
        assert!(
            receipt.success,
            "receipt {:?} should be successful, error: {:?}",
            hex::encode(receipt.tx_id),
            receipt.error
        );
    }

    // ── Verify final balances ──

    // Alice: started with 100_000 asset_a, sent 10_000 to Carol, swapped 1_000
    let alice_a = runtime.store.get_balance_by_id(&alice_a_id).expect("alice asset_a balance");
    assert_eq!(
        alice_a.amount, 89_000,
        "Alice asset_a: 100_000 - 10_000 - 1_000 = 89_000, got {}",
        alice_a.amount
    );

    // Bob: started with 50_000 asset_a, sent 5_000 to Carol
    let bob_a = runtime.store.get_balance_by_id(&bob_a_id).expect("bob asset_a balance");
    assert_eq!(
        bob_a.amount, 45_000,
        "Bob asset_a: 50_000 - 5_000 = 45_000, got {}",
        bob_a.amount
    );

    // Carol: started with 0, received 10_000 + 5_000 = 15_000
    let carol_a = runtime.store.get_balance_by_id(&carol_a_id).expect("carol asset_a balance");
    assert_eq!(
        carol_a.amount, 15_000,
        "Carol asset_a: 0 + 10_000 + 5_000 = 15_000, got {}",
        carol_a.amount
    );

    // Alice should have some asset_b from the swap
    let alice_b = runtime.store.get_balance_by_id(&alice_b_id).expect("alice asset_b balance");
    assert!(alice_b.amount > 0, "Alice should have received some asset_b from the swap");

    // State root must be non-zero
    assert_ne!(result.state_root, [0u8; 32], "state root should be non-zero");
}

// ──────────────────────────────────────────────
// Batch with invalid intent skipped
// ──────────────────────────────────────────────

#[test]
fn test_batch_skips_invalid_intents() {
    let alice = addr(0xA1);
    let bob = addr(0xB2);
    let asset = addr(0x01);

    let mut runtime = PoSeqRuntime::new();
    runtime.seed_object(Box::new(BalanceObject::new(id(1), alice, asset, 1000, 0)));
    runtime.seed_object(Box::new(BalanceObject::new(id(2), bob, asset, 0, 0)));

    // Valid tx
    let valid_tx = IntentTransaction {
        tx_id: txid(1),
        sender: alice,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset,
            amount: 100,
            recipient: bob,
            memo: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
    };

    // Invalid tx: zero amount
    let invalid_tx = IntentTransaction {
        tx_id: txid(2),
        sender: alice,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset,
            amount: 0, // invalid: zero amount
            recipient: bob,
            memo: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 2,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
    };

    let batch = OrderedBatch {
        batch_id: batch_id(2),
        epoch: 1,
        sequence_number: 0,
        transactions: vec![valid_tx, invalid_tx],
    };

    let result = runtime.process_batch(batch).expect("batch must not fail on invalid intents");

    // Only the valid tx should produce a plan
    assert_eq!(result.total_plans, 1, "only 1 plan (invalid tx structurally rejected)");
    assert_eq!(result.succeeded, 1);
}

// ──────────────────────────────────────────────
// Epoch advances with each batch
// ──────────────────────────────────────────────

#[test]
fn test_epoch_advances_with_batch() {
    let mut runtime = PoSeqRuntime::new();
    assert_eq!(runtime.current_epoch, 0);

    let batch = OrderedBatch {
        batch_id: batch_id(1),
        epoch: 42,
        sequence_number: 0,
        transactions: vec![],
    };

    runtime.process_batch(batch).expect("empty batch should succeed");
    assert_eq!(runtime.current_epoch, 42);
}

// ──────────────────────────────────────────────
// State root is deterministic
// ──────────────────────────────────────────────

#[test]
fn test_state_root_deterministic_across_identical_batches() {
    // Two runtimes seeded with identical state and processing identical batches
    // should produce identical state roots.
    let alice = addr(0xA1);
    let bob = addr(0xB2);
    let asset = addr(0x01);

    let setup_runtime = || {
        let mut runtime = PoSeqRuntime::new();
        runtime.seed_object(Box::new(BalanceObject::new(id(1), alice, asset, 1000, 0)));
        runtime.seed_object(Box::new(BalanceObject::new(id(2), bob, asset, 0, 0)));
        runtime
    };

    let make_batch = || OrderedBatch {
        batch_id: batch_id(1),
        epoch: 1,
        sequence_number: 0,
        transactions: vec![IntentTransaction {
            tx_id: txid(1),
            sender: alice,
            intent: IntentType::Transfer(TransferIntent {
                asset_id: asset,
                amount: 100,
                recipient: bob,
                memo: None,
            }),
            max_fee: 1000,
            deadline_epoch: 9999,
            nonce: 1,
            signature: [0u8; 64],
            metadata: BTreeMap::new(),
            target_objects: vec![],
            constraints: IntentConstraints::default(),
            execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
        }],
    };

    let mut r1 = setup_runtime();
    let mut r2 = setup_runtime();

    let result1 = r1.process_batch(make_batch()).unwrap();
    let result2 = r2.process_batch(make_batch()).unwrap();

    assert_eq!(
        result1.state_root, result2.state_root,
        "identical batches must produce identical state roots"
    );
}
