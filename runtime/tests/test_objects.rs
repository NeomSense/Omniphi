use omniphi_runtime::objects::base::{ObjectId, ObjectType};
use omniphi_runtime::objects::types::{
    BalanceObject, ExecutionReceiptObject, GovernanceProposalObject, IdentityObject,
    LiquidityPoolObject, ProposalStatus, TokenObject, VaultObject, WalletObject,
};
use omniphi_runtime::objects::base::Object;

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
