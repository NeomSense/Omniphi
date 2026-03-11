use omniphi_runtime::capabilities::checker::CapabilitySet;
use omniphi_runtime::errors::RuntimeError;
use omniphi_runtime::intents::base::{IntentTransaction, IntentType};
use omniphi_runtime::intents::types::{SwapIntent, TransferIntent};
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::{BalanceObject, LiquidityPoolObject};
use omniphi_runtime::resolution::planner::{IntentResolver, ObjectOperation};
use omniphi_runtime::state::store::ObjectStore;
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

fn make_transfer_tx(
    sender: [u8; 32],
    recipient: [u8; 32],
    asset: [u8; 32],
    amount: u128,
) -> IntentTransaction {
    IntentTransaction {
        tx_id: txid(1),
        sender,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset,
            amount,
            recipient,
            memo: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
    }
}

fn make_swap_tx(
    sender: [u8; 32],
    input_asset: [u8; 32],
    output_asset: [u8; 32],
    input_amount: u128,
    min_output: u128,
) -> IntentTransaction {
    IntentTransaction {
        tx_id: txid(2),
        sender,
        intent: IntentType::Swap(SwapIntent {
            input_asset,
            output_asset,
            input_amount,
            min_output_amount: min_output,
            max_slippage_bps: 500, // 5%
            allowed_pool_ids: None,
        }),
        max_fee: 1000,
        deadline_epoch: 9999,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
    }
}

// ──────────────────────────────────────────────
// Transfer resolution
// ──────────────────────────────────────────────

#[test]
fn test_transfer_resolves_to_debit_and_credit() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(id(10), sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(id(11), recipient, asset, 0, 0)));

    let tx = make_transfer_tx(sender, recipient, asset, 1000);
    let caps = CapabilitySet::user_default();

    let plan = IntentResolver::resolve(&tx, &store, &caps)
        .expect("transfer resolution should succeed");

    assert_eq!(plan.operations.len(), 2, "expected exactly 2 operations");

    let has_debit = plan.operations.iter().any(|op| matches!(op, ObjectOperation::DebitBalance { amount, .. } if *amount == 1000));
    let has_credit = plan.operations.iter().any(|op| matches!(op, ObjectOperation::CreditBalance { amount, .. } if *amount == 1000));
    assert!(has_debit, "expected a DebitBalance operation for 1000");
    assert!(has_credit, "expected a CreditBalance operation for 1000");
}

#[test]
fn test_transfer_fails_when_sender_balance_missing() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let mut store = ObjectStore::new();
    // Only recipient balance — no sender balance
    store.insert(Box::new(BalanceObject::new(id(11), recipient, asset, 0, 0)));

    let tx = make_transfer_tx(sender, recipient, asset, 1000);
    let caps = CapabilitySet::user_default();

    let result = IntentResolver::resolve(&tx, &store, &caps);
    assert!(
        matches!(result, Err(RuntimeError::ObjectNotFound(_))),
        "expected ObjectNotFound, got: {:?}",
        result
    );
}

#[test]
fn test_transfer_fails_insufficient_balance() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let mut store = ObjectStore::new();
    // Sender only has 100, wants to send 1000
    store.insert(Box::new(BalanceObject::new(id(10), sender, asset, 100, 0)));
    store.insert(Box::new(BalanceObject::new(id(11), recipient, asset, 0, 0)));

    let tx = make_transfer_tx(sender, recipient, asset, 1000);
    let caps = CapabilitySet::user_default();

    let result = IntentResolver::resolve(&tx, &store, &caps);
    assert!(
        matches!(result, Err(RuntimeError::InsufficientBalance { .. })),
        "expected InsufficientBalance, got: {:?}",
        result
    );
}

// ──────────────────────────────────────────────
// Swap resolution
// ──────────────────────────────────────────────

#[test]
fn test_swap_resolves_with_correct_constant_product_output() {
    // Pool: reserve_a=10_000, reserve_b=10_000, fee=30bps (0.3%)
    // Swap 1000 of asset_a for asset_b
    // output = (1000 * (10000 - 30) * 10000) / (10000 * 10000 + 1000 * (10000 - 30))
    //        = (1000 * 9970 * 10000) / (100_000_000 + 1000 * 9970)
    //        = 99_700_000_000 / (100_000_000 + 9_970_000)
    //        = 99_700_000_000 / 109_970_000
    //        ≈ 906
    let sender = addr(0xAA);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    let mut store = ObjectStore::new();
    store.insert(Box::new(LiquidityPoolObject::new(
        id(20), addr(0xFF), asset_a, asset_b, 10_000, 10_000, 30, addr(0xEE), 0,
    )));
    store.insert(Box::new(BalanceObject::new(id(21), sender, asset_a, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(id(22), sender, asset_b, 0, 0)));

    let tx = make_swap_tx(sender, asset_a, asset_b, 1000, 1); // min_output=1 (very low)
    let caps = CapabilitySet::user_default();

    let plan = IntentResolver::resolve(&tx, &store, &caps)
        .expect("swap resolution should succeed");

    // Should have: DebitBalance (input), CreditBalance (output), SwapPoolAmounts
    assert_eq!(plan.operations.len(), 3);

    let credit_amount = plan.operations.iter().find_map(|op| {
        if let ObjectOperation::CreditBalance { amount, .. } = op { Some(*amount) } else { None }
    }).expect("expected CreditBalance");

    // Expected output ≈ 906
    assert_eq!(credit_amount, 906, "constant product output should be 906, got {}", credit_amount);
}

#[test]
fn test_swap_fails_when_output_below_min() {
    let sender = addr(0xAA);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    let mut store = ObjectStore::new();
    store.insert(Box::new(LiquidityPoolObject::new(
        id(20), addr(0xFF), asset_a, asset_b, 10_000, 10_000, 30, addr(0xEE), 0,
    )));
    store.insert(Box::new(BalanceObject::new(id(21), sender, asset_a, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(id(22), sender, asset_b, 0, 0)));

    // Set min_output = 2000 — impossible given 1000 input into 10k/10k pool
    let tx = make_swap_tx(sender, asset_a, asset_b, 1000, 2000);
    let caps = CapabilitySet::user_default();

    let result = IntentResolver::resolve(&tx, &store, &caps);
    assert!(
        matches!(result, Err(RuntimeError::ConstraintViolation(_))),
        "expected ConstraintViolation when output < min_output, got: {:?}",
        result
    );
}

#[test]
fn test_swap_fails_when_no_pool_found() {
    let sender = addr(0xAA);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    let mut store = ObjectStore::new();
    // No pool, just balances
    store.insert(Box::new(BalanceObject::new(id(21), sender, asset_a, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(id(22), sender, asset_b, 0, 0)));

    let tx = make_swap_tx(sender, asset_a, asset_b, 1000, 1);
    let caps = CapabilitySet::user_default();

    let result = IntentResolver::resolve(&tx, &store, &caps);
    assert!(
        matches!(result, Err(RuntimeError::ResolutionFailure(_))),
        "expected ResolutionFailure when no pool, got: {:?}",
        result
    );
}

#[test]
fn test_swap_fails_sender_input_balance_missing() {
    let sender = addr(0xAA);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    let mut store = ObjectStore::new();
    store.insert(Box::new(LiquidityPoolObject::new(
        id(20), addr(0xFF), asset_a, asset_b, 10_000, 10_000, 30, addr(0xEE), 0,
    )));
    // Missing sender input balance
    store.insert(Box::new(BalanceObject::new(id(22), sender, asset_b, 0, 0)));

    let tx = make_swap_tx(sender, asset_a, asset_b, 1000, 1);
    let caps = CapabilitySet::user_default();

    let result = IntentResolver::resolve(&tx, &store, &caps);
    assert!(
        matches!(result, Err(RuntimeError::ObjectNotFound(_))),
        "expected ObjectNotFound when sender input balance missing, got: {:?}",
        result
    );
}

// ──────────────────────────────────────────────
// Fix B: AMM overflow safety test
// ──────────────────────────────────────────────

/// Verifies that the AMM calculation is safe when reserves are near u128::MAX.
/// With the old saturating_mul approach this could produce wrong results
/// (overflow silenced by saturation). With U256 it must either return a
/// valid result or a ConstraintViolation — it must NOT panic.
#[test]
fn test_amm_overflow_safe() {
    let sender = addr(0xAA);
    let asset_a = addr(0x01);
    let asset_b = addr(0x02);

    // Use reserves just below 2^100 — large enough to overflow u128 intermediate
    // multiplication but small enough to fit in u128.
    // 2^100 ≈ 1.267 × 10^30
    let large_reserve: u128 = (1u128 << 100).saturating_sub(1);

    let mut store = ObjectStore::new();
    store.insert(Box::new(LiquidityPoolObject::new(
        id(20),
        addr(0xFF),
        asset_a,
        asset_b,
        large_reserve,
        large_reserve,
        30,          // 0.3% fee
        addr(0xEE),
        0,
    )));
    // Give sender enough input asset (use a small amount to avoid reserve exhaustion)
    store.insert(Box::new(BalanceObject::new(id(21), sender, asset_a, 1_000, 0)));
    // Sender needs an output asset balance object to receive into
    store.insert(Box::new(BalanceObject::new(id(22), sender, asset_b, 0, 0)));

    // Swap a small amount — should succeed deterministically with no panic
    let tx = make_swap_tx(sender, asset_a, asset_b, 1_000, 0);
    let caps = CapabilitySet::user_default();

    // Must NOT panic — the result is either Ok (valid output) or a runtime error
    let result = IntentResolver::resolve(&tx, &store, &caps);
    match result {
        Ok(plan) => {
            // If it resolved, the credit amount must be > 0 and <= reserve_b
            let credit = plan.operations.iter().find_map(|op| {
                if let ObjectOperation::CreditBalance { amount, .. } = op {
                    Some(*amount)
                } else {
                    None
                }
            });
            assert!(credit.is_some(), "expected a CreditBalance operation");
            let out = credit.unwrap();
            assert!(out <= large_reserve, "output must not exceed reserve_b");
        }
        Err(RuntimeError::ConstraintViolation(_)) => {
            // Acceptable: AMM detected an edge case (e.g. slippage exceeded)
        }
        Err(e) => {
            panic!("unexpected error type (not ConstraintViolation): {:?}", e);
        }
    }
}
