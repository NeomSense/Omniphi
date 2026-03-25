use omniphi_runtime::objects::base::{ObjectId, ObjectType, AccessMode, ObjectAccess};
use omniphi_runtime::objects::types::{
    BalanceObject, ExecutionReceiptObject, GovernanceProposalObject, IdentityObject,
    LiquidityPoolObject, ProposalStatus, TokenObject, VaultObject, WalletObject,
};
use omniphi_runtime::objects::base::Object;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::scheduler::parallel::ParallelScheduler;

fn id(n: u8) -> ObjectId {
    let mut bytes = [0u8; 32];
    bytes[0] = n;
    ObjectId::new(bytes)
}

fn addr(n: u8) -> [u8; 32] {
    let mut bytes = [0u8; 32];
    bytes[0] = n;
    bytes
}

// ──────────────────────────────────────────────
// WalletObject
// ──────────────────────────────────────────────

#[test]
fn test_wallet_object_version_starts_at_zero() {
    let obj = WalletObject::new(id(1), addr(0xAA), addr(0xAA), 1000);
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_wallet_object_encode_deterministic() {
    let obj = WalletObject::new(id(1), addr(0xAA), addr(0xAA), 1000);
    assert_eq!(obj.encode(), obj.encode(), "WalletObject::encode must be deterministic");
}

#[test]
fn test_wallet_object_type() {
    let obj = WalletObject::new(id(1), addr(0xAA), addr(0xAA), 1000);
    assert_eq!(obj.object_type(), ObjectType::Wallet);
}

// ──────────────────────────────────────────────
// BalanceObject
// ──────────────────────────────────────────────

#[test]
fn test_balance_object_version_starts_at_zero() {
    let obj = BalanceObject::new(id(2), addr(0x01), addr(0x02), 1_000_000, 0);
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_balance_object_encode_deterministic() {
    let obj = BalanceObject::new(id(2), addr(0x01), addr(0x02), 500, 0);
    assert_eq!(obj.encode(), obj.encode());
}

#[test]
fn test_balance_object_available() {
    let mut obj = BalanceObject::new(id(2), addr(0x01), addr(0x02), 1000, 0);
    obj.locked_amount = 300;
    assert_eq!(obj.available(), 700);
}

#[test]
fn test_balance_object_type() {
    let obj = BalanceObject::new(id(2), addr(0x01), addr(0x02), 1000, 0);
    assert_eq!(obj.object_type(), ObjectType::Balance);
}

// ──────────────────────────────────────────────
// TokenObject
// ──────────────────────────────────────────────

#[test]
fn test_token_object_version_starts_at_zero() {
    let obj = TokenObject::new(
        id(3), addr(0x10), addr(0x20), "OMNI".to_string(), 6, 1_000_000_000, 0,
    );
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_token_object_encode_deterministic() {
    let obj = TokenObject::new(
        id(3), addr(0x10), addr(0x20), "OMNI".to_string(), 6, 1_000_000_000, 0,
    );
    assert_eq!(obj.encode(), obj.encode());
}

#[test]
fn test_token_object_type() {
    let obj = TokenObject::new(
        id(3), addr(0x10), addr(0x20), "OMNI".to_string(), 6, 1_000_000_000, 0,
    );
    assert_eq!(obj.object_type(), ObjectType::Token);
}

// ──────────────────────────────────────────────
// LiquidityPoolObject
// ──────────────────────────────────────────────

#[test]
fn test_pool_object_version_starts_at_zero() {
    let obj = LiquidityPoolObject::new(
        id(4), addr(0x01), addr(0x02), addr(0x03), 1000, 2000, 30, addr(0x05), 0,
    );
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_pool_object_encode_deterministic() {
    let obj = LiquidityPoolObject::new(
        id(4), addr(0x01), addr(0x02), addr(0x03), 1000, 2000, 30, addr(0x05), 0,
    );
    assert_eq!(obj.encode(), obj.encode());
}

#[test]
fn test_pool_object_type() {
    let obj = LiquidityPoolObject::new(
        id(4), addr(0x01), addr(0x02), addr(0x03), 1000, 2000, 30, addr(0x05), 0,
    );
    assert_eq!(obj.object_type(), ObjectType::LiquidityPool);
}

// ──────────────────────────────────────────────
// VaultObject
// ──────────────────────────────────────────────

#[test]
fn test_vault_object_version_starts_at_zero() {
    let obj = VaultObject::new(id(5), addr(0x01), addr(0x02), 0);
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_vault_object_encode_deterministic() {
    let obj = VaultObject::new(id(5), addr(0x01), addr(0x02), 0);
    assert_eq!(obj.encode(), obj.encode());
}

// ──────────────────────────────────────────────
// GovernanceProposalObject
// ──────────────────────────────────────────────

#[test]
fn test_governance_object_version_starts_at_zero() {
    let obj = GovernanceProposalObject::new(
        id(6), addr(0x01), 1, addr(0x01),
        "Proposal 1".to_string(), "Do something".to_string(), 100, 0,
    );
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_governance_object_encode_deterministic() {
    let obj = GovernanceProposalObject::new(
        id(6), addr(0x01), 1, addr(0x01),
        "Proposal 1".to_string(), "Do something".to_string(), 100, 0,
    );
    assert_eq!(obj.encode(), obj.encode());
}

#[test]
fn test_governance_object_initial_status() {
    let obj = GovernanceProposalObject::new(
        id(6), addr(0x01), 1, addr(0x01),
        "Title".to_string(), "Desc".to_string(), 100, 0,
    );
    assert_eq!(obj.status, ProposalStatus::Active);
}

// ──────────────────────────────────────────────
// IdentityObject
// ──────────────────────────────────────────────

#[test]
fn test_identity_object_version_starts_at_zero() {
    let obj = IdentityObject::new(id(7), addr(0x01), addr(0x01), "Alice".to_string(), 0);
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_identity_object_encode_deterministic() {
    let obj = IdentityObject::new(id(7), addr(0x01), addr(0x01), "Alice".to_string(), 0);
    assert_eq!(obj.encode(), obj.encode());
}

// ──────────────────────────────────────────────
// ExecutionReceiptObject
// ──────────────────────────────────────────────

#[test]
fn test_receipt_object_version_starts_at_zero() {
    let obj = ExecutionReceiptObject::new(id(8), addr(0x01), [1u8; 32], true, 0);
    assert_eq!(obj.meta().version, 0);
}

#[test]
fn test_receipt_object_encode_deterministic() {
    let obj = ExecutionReceiptObject::new(id(8), addr(0x01), [1u8; 32], true, 0);
    assert_eq!(obj.encode(), obj.encode());
}

// ──────────────────────────────────────────────
// ObjectId display
// ──────────────────────────────────────────────

#[test]
fn test_object_id_display_is_hex() {
    let mut bytes = [0u8; 32];
    bytes[0] = 0xDE;
    bytes[1] = 0xAD;
    let id = ObjectId::new(bytes);
    let s = id.to_string();
    assert!(s.starts_with("dead"), "Expected hex display starting with 'dead', got: {}", s);
    assert_eq!(s.len(), 64);
}

// ──────────────────────────────────────────────
// Section 1 gap: Invalid object creation
// ──────────────────────────────────────────────

#[test]
fn test_zero_id_object_has_zero_id() {
    let zero_id = ObjectId::zero();
    assert_eq!(zero_id.0, [0u8; 32]);
}

#[test]
fn test_object_with_zero_owner_still_has_zero_owner() {
    let obj = WalletObject::new(id(1), [0u8; 32], addr(0xAA), 1000);
    assert_eq!(obj.meta().owner, [0u8; 32]);
    // Zero owner is structurally valid but should be rejected at intent validation
}

#[test]
fn test_balance_negative_available_saturates() {
    let mut obj = BalanceObject::new(id(2), addr(1), addr(0xAA), 100, 0);
    obj.locked_amount = 200; // locked > amount
    assert_eq!(obj.available(), 0); // saturating_sub returns 0, not underflow
}

// ──────────────────────────────────────────────
// Section 1 gap: Ownership transfer
// ──────────────────────────────────────────────

#[test]
fn test_transfer_ownership_changes_owner() {
    let mut store = ObjectStore::new();
    let alice = addr(0xAA);
    let bob = addr(0xBB);
    let asset = addr(0xCC);

    let bal = BalanceObject::new(id(1), alice, asset, 1000, 0);
    store.insert(Box::new(bal));

    let plan = ExecutionPlan {
        tx_id: [0x01; 32],
        operations: vec![
            ObjectOperation::TransferOwnership {
                object_id: id(1),
                new_owner: bob,
            },
        ],
        required_capabilities: vec![],
        object_access: vec![ObjectAccess {
            object_id: id(1),
            mode: AccessMode::ReadWrite,
        }],
        gas_estimate: 1000,
        gas_limit: 100_000,
    };

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.succeeded, 1);
    let obj = store.get(&id(1)).unwrap();
    assert_eq!(obj.meta().owner, bob);
}

#[test]
fn test_transfer_ownership_to_zero_rejected() {
    let mut store = ObjectStore::new();
    let alice = addr(0xAA);
    let asset = addr(0xCC);

    let bal = BalanceObject::new(id(1), alice, asset, 1000, 0);
    store.insert(Box::new(bal));

    let plan = ExecutionPlan {
        tx_id: [0x02; 32],
        operations: vec![
            ObjectOperation::TransferOwnership {
                object_id: id(1),
                new_owner: [0u8; 32], // zero address
            },
        ],
        required_capabilities: vec![],
        object_access: vec![ObjectAccess {
            object_id: id(1),
            mode: AccessMode::ReadWrite,
        }],
        gas_estimate: 1000,
        gas_limit: 100_000,
    };

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
    // Owner unchanged
    let obj = store.get(&id(1)).unwrap();
    assert_eq!(obj.meta().owner, alice);
}

#[test]
fn test_transfer_ownership_to_self_rejected() {
    let mut store = ObjectStore::new();
    let alice = addr(0xAA);
    let asset = addr(0xCC);

    let bal = BalanceObject::new(id(1), alice, asset, 500, 0);
    store.insert(Box::new(bal));

    let plan = ExecutionPlan {
        tx_id: [0x03; 32],
        operations: vec![
            ObjectOperation::TransferOwnership {
                object_id: id(1),
                new_owner: alice, // same as current owner
            },
        ],
        required_capabilities: vec![],
        object_access: vec![ObjectAccess {
            object_id: id(1),
            mode: AccessMode::ReadWrite,
        }],
        gas_estimate: 1000,
        gas_limit: 100_000,
    };

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
}

#[test]
fn test_transfer_ownership_version_increments() {
    let mut store = ObjectStore::new();
    let alice = addr(0xAA);
    let bob = addr(0xBB);

    let wallet = WalletObject::new(id(5), alice, alice, 0);
    store.insert(Box::new(wallet));

    let old_version = store.get(&id(5)).unwrap().meta().version;

    let plan = ExecutionPlan {
        tx_id: [0x04; 32],
        operations: vec![
            ObjectOperation::TransferOwnership {
                object_id: id(5),
                new_owner: bob,
            },
        ],
        required_capabilities: vec![],
        object_access: vec![ObjectAccess {
            object_id: id(5),
            mode: AccessMode::ReadWrite,
        }],
        gas_estimate: 1000,
        gas_limit: 100_000,
    };

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.succeeded, 1);
    let new_version = store.get(&id(5)).unwrap().meta().version;
    assert!(new_version > old_version, "Version should increment after transfer");
}
