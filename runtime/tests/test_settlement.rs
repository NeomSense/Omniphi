use omniphi_runtime::capabilities::checker::Capability;
use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId};
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::scheduler::parallel::{ExecutionGroup, ParallelScheduler};
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;

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

/// Build a transfer plan directly (without going through IntentResolver).
fn direct_transfer_plan(
    tx: u8,
    sender_balance_id: ObjectId,
    recipient_balance_id: ObjectId,
    amount: u128,
) -> ExecutionPlan {
    ExecutionPlan {
        tx_id: txid(tx),
        operations: vec![
            ObjectOperation::DebitBalance { balance_id: sender_balance_id, amount },
            ObjectOperation::CreditBalance { balance_id: recipient_balance_id, amount },
        ],
        required_capabilities: vec![Capability::TransferAsset],
        object_access: vec![
            ObjectAccess { object_id: sender_balance_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recipient_balance_id, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 1_400,   // base(1000) + debit(200) + credit(200)
        gas_limit: u64::MAX,
    }
}

fn single_group(plan: ExecutionPlan) -> Vec<ExecutionGroup> {
    vec![ExecutionGroup { plans: vec![plan], group_index: 0 }]
}

// ──────────────────────────────────────────────
// Successful transfer
// ──────────────────────────────────────────────

#[test]
fn test_transfer_updates_balances() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 1000, 0)));

    let plan = direct_transfer_plan(1, sender_id, recip_id, 2000);
    let groups = single_group(plan);

    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.succeeded, 1);
    assert_eq!(result.failed, 0);

    let sender_balance = store.get_balance_by_id(&sender_id).expect("sender balance must exist");
    assert_eq!(sender_balance.amount, 3000, "sender should have 5000 - 2000 = 3000");

    let recip_balance = store.get_balance_by_id(&recip_id).expect("recipient balance must exist");
    assert_eq!(recip_balance.amount, 3000, "recipient should have 1000 + 2000 = 3000");
}

// ──────────────────────────────────────────────
// Version incremented on mutated objects
// ──────────────────────────────────────────────

#[test]
fn test_version_incremented_after_settlement() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    assert_eq!(store.get_balance_by_id(&sender_id).unwrap().meta.version, 0);
    assert_eq!(store.get_balance_by_id(&recip_id).unwrap().meta.version, 0);

    let plan = direct_transfer_plan(1, sender_id, recip_id, 100);
    let groups = single_group(plan);
    SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(
        store.get_balance_by_id(&sender_id).unwrap().meta.version, 1,
        "sender version should be 1 after mutation"
    );
    assert_eq!(
        store.get_balance_by_id(&recip_id).unwrap().meta.version, 1,
        "recipient version should be 1 after mutation"
    );
}

// ──────────────────────────────────────────────
// Receipt contains correct version_transitions
// ──────────────────────────────────────────────

#[test]
fn test_receipt_contains_version_transitions() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    let plan = direct_transfer_plan(1, sender_id, recip_id, 100);
    let groups = single_group(plan);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.receipts.len(), 1);
    let receipt = &result.receipts[0];
    assert!(receipt.success);

    // Both objects should have version transitions 0 → 1
    for (_obj_id, old_v, new_v) in &receipt.version_transitions {
        assert_eq!(*old_v, 0, "old version should be 0");
        assert_eq!(*new_v, 1, "new version should be 1");
    }
    assert!(
        receipt.version_transitions.len() >= 2,
        "should have version transitions for both objects"
    );
}

// ──────────────────────────────────────────────
// Failed plan does NOT mutate state (atomicity)
// ──────────────────────────────────────────────

#[test]
fn test_failed_plan_does_not_mutate_state() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    // Sender only has 50, plan tries to send 1000
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 50, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    let plan = direct_transfer_plan(1, sender_id, recip_id, 1000);
    let groups = single_group(plan);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
    assert_eq!(result.succeeded, 0);

    // State must be unchanged
    let sender_balance = store.get_balance_by_id(&sender_id).unwrap();
    assert_eq!(sender_balance.amount, 50, "sender balance must not change on failure");
    assert_eq!(sender_balance.meta.version, 0, "version must not increment on failure");

    let recip_balance = store.get_balance_by_id(&recip_id).unwrap();
    assert_eq!(recip_balance.amount, 0, "recipient balance must not change on failure");
    assert_eq!(recip_balance.meta.version, 0, "version must not increment on failure");
}

// ──────────────────────────────────────────────
// State root changes after mutation
// ──────────────────────────────────────────────

#[test]
fn test_state_root_changes_after_transfer() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    let root_before = store.state_root();

    let plan = direct_transfer_plan(1, sender_id, recip_id, 100);
    let groups = single_group(plan);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_ne!(
        root_before, result.state_root,
        "state root must change after a successful transfer"
    );
}

// ──────────────────────────────────────────────
// Failed plan: state root unchanged
// ──────────────────────────────────────────────

#[test]
fn test_state_root_unchanged_after_failed_plan() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 50, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    // Sync canonical store before computing root
    store.sync_typed_to_canonical();
    let root_before = store.state_root();

    let plan = direct_transfer_plan(1, sender_id, recip_id, 1000); // will fail
    let groups = single_group(plan);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
    assert_eq!(
        root_before, result.state_root,
        "state root must not change after a failed plan"
    );
}

// ──────────────────────────────────────────────
// Multiple groups execute in order
// ──────────────────────────────────────────────

#[test]
fn test_multiple_groups_execute_in_order() {
    // Two conflicting transfers that get scheduled into 2 groups.
    // Transfer 1: sender → recipient1, 1000
    // Transfer 2: sender → recipient2, 500
    // Both debit the sender → must be sequential.

    let sender = addr(0xAA);
    let r1 = addr(0xBB);
    let r2 = addr(0xCC);
    let asset = addr(0x11);

    let sender_id = id(10);
    let r1_id = id(11);
    let r2_id = id(12);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 2000, 0)));
    store.insert(Box::new(BalanceObject::new(r1_id, r1, asset, 0, 0)));
    store.insert(Box::new(BalanceObject::new(r2_id, r2, asset, 0, 0)));

    let plan1 = direct_transfer_plan(1, sender_id, r1_id, 1000);
    let plan2 = direct_transfer_plan(2, sender_id, r2_id, 500);

    // Schedule them (they conflict on sender_id)
    let groups = ParallelScheduler::schedule(vec![plan1, plan2]);
    assert_eq!(groups.len(), 2, "conflicting plans should be in 2 groups");

    let result = SettlementEngine::execute_groups(groups, &mut store, 1);
    assert_eq!(result.succeeded, 2, "both transfers should succeed");
    assert_eq!(result.failed, 0);

    let sender_after = store.get_balance_by_id(&sender_id).unwrap().amount;
    assert_eq!(sender_after, 500, "sender: 2000 - 1000 - 500 = 500");

    let r1_after = store.get_balance_by_id(&r1_id).unwrap().amount;
    assert_eq!(r1_after, 1000);

    let r2_after = store.get_balance_by_id(&r2_id).unwrap().amount;
    assert_eq!(r2_after, 500);
}

// ──────────────────────────────────────────────
// Fix C: Gas metering — gas_limit = 1 must fail atomically
// ──────────────────────────────────────────────

/// Creates a plan with gas_limit = 1 (far below any real operation cost) and
/// verifies that:
///   1. The SettlementResult marks the plan as failed.
///   2. The error string contains "gas" (or similar out-of-gas indicator).
///   3. Balance objects are NOT mutated (atomicity guarantee).
#[test]
fn test_gas_limit_enforced() {
    let sender = addr(0xAA);
    let recipient = addr(0xBB);
    let asset = addr(0x11);

    let sender_id = id(10);
    let recip_id = id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(sender_id, sender, asset, 5000, 0)));
    store.insert(Box::new(BalanceObject::new(recip_id, recipient, asset, 0, 0)));

    // Build a plan with gas_limit = 1 — impossibly small, must trigger out-of-gas
    let plan = ExecutionPlan {
        tx_id: txid(99),
        operations: vec![
            ObjectOperation::DebitBalance { balance_id: sender_id, amount: 100 },
            ObjectOperation::CreditBalance { balance_id: recip_id, amount: 100 },
        ],
        required_capabilities: vec![Capability::TransferAsset],
        object_access: vec![
            ObjectAccess { object_id: sender_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recip_id, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 1_400,
        gas_limit: 1,  // intentionally too small
    };

    let groups = single_group(plan);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    // Must fail
    assert_eq!(result.failed, 1, "plan with gas_limit=1 must fail");
    assert_eq!(result.succeeded, 0);

    // Error must reference gas exhaustion
    let receipt = &result.receipts[0];
    assert!(!receipt.success, "receipt must be unsuccessful");
    let err_msg = receipt.error.as_deref().unwrap_or("");
    assert!(
        err_msg.contains("gas") || err_msg.contains("Gas"),
        "error must mention gas, got: {}",
        err_msg
    );

    // Balances must be unchanged (atomicity)
    let sender_balance = store.get_balance_by_id(&sender_id).unwrap();
    assert_eq!(sender_balance.amount, 5000, "sender balance must not change on out-of-gas failure");
    assert_eq!(sender_balance.meta.version, 0, "version must not increment on gas failure");

    let recip_balance = store.get_balance_by_id(&recip_id).unwrap();
    assert_eq!(recip_balance.amount, 0, "recipient balance must not change on out-of-gas failure");
    assert_eq!(recip_balance.meta.version, 0, "version must not increment on gas failure");
}
