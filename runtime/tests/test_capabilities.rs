use omniphi_runtime::capabilities::checker::{Capability, CapabilityChecker, CapabilitySet};
use omniphi_runtime::errors::RuntimeError;
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::{
    BalanceObject, GovernanceProposalObject, IdentityObject, LiquidityPoolObject, TokenObject,
};
use omniphi_runtime::objects::base::Object;

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

// ──────────────────────────────────────────────
// CapabilitySet::user_default
// ──────────────────────────────────────────────

#[test]
fn test_user_default_contains_read_object() {
    let caps = CapabilitySet::user_default();
    assert!(caps.contains(&Capability::ReadObject));
}

#[test]
fn test_user_default_contains_write_object() {
    let caps = CapabilitySet::user_default();
    assert!(caps.contains(&Capability::WriteObject));
}

#[test]
fn test_user_default_contains_transfer_asset() {
    let caps = CapabilitySet::user_default();
    assert!(caps.contains(&Capability::TransferAsset));
}

#[test]
fn test_user_default_contains_swap_asset() {
    let caps = CapabilitySet::user_default();
    assert!(caps.contains(&Capability::SwapAsset));
}

#[test]
fn test_user_default_does_not_contain_mint_asset() {
    let caps = CapabilitySet::user_default();
    assert!(!caps.contains(&Capability::MintAsset));
}

#[test]
fn test_user_default_does_not_contain_burn_asset() {
    let caps = CapabilitySet::user_default();
    assert!(!caps.contains(&Capability::BurnAsset));
}

#[test]
fn test_user_default_does_not_contain_modify_governance() {
    let caps = CapabilitySet::user_default();
    assert!(!caps.contains(&Capability::ModifyGovernance));
}

#[test]
fn test_user_default_does_not_contain_update_identity() {
    let caps = CapabilitySet::user_default();
    assert!(!caps.contains(&Capability::UpdateIdentity));
}

// ──────────────────────────────────────────────
// CapabilitySet::all
// ──────────────────────────────────────────────

#[test]
fn test_all_caps_contains_every_capability() {
    let caps = CapabilitySet::all();
    assert!(caps.contains(&Capability::ReadObject));
    assert!(caps.contains(&Capability::WriteObject));
    assert!(caps.contains(&Capability::TransferAsset));
    assert!(caps.contains(&Capability::SwapAsset));
    assert!(caps.contains(&Capability::ProvideLiquidity));
    assert!(caps.contains(&Capability::WithdrawLiquidity));
    assert!(caps.contains(&Capability::MintAsset));
    assert!(caps.contains(&Capability::BurnAsset));
    assert!(caps.contains(&Capability::ModifyGovernance));
    assert!(caps.contains(&Capability::UpdateIdentity));
}

// ──────────────────────────────────────────────
// CapabilitySet add / remove
// ──────────────────────────────────────────────

#[test]
fn test_add_capability() {
    let mut caps = CapabilitySet::empty();
    assert!(!caps.contains(&Capability::MintAsset));
    caps.add(Capability::MintAsset);
    assert!(caps.contains(&Capability::MintAsset));
}

#[test]
fn test_remove_capability() {
    let mut caps = CapabilitySet::all();
    caps.remove(Capability::MintAsset);
    assert!(!caps.contains(&Capability::MintAsset));
}

// ──────────────────────────────────────────────
// CapabilityChecker::check
// ──────────────────────────────────────────────

#[test]
fn test_check_succeeds_when_capability_held() {
    let caps = CapabilitySet::user_default();
    let result = CapabilityChecker::check(&caps, &[Capability::TransferAsset]);
    assert!(result.is_ok());
}

#[test]
fn test_check_fails_when_capability_missing() {
    let caps = CapabilitySet::user_default();
    let result = CapabilityChecker::check(&caps, &[Capability::MintAsset]);
    assert!(matches!(result, Err(RuntimeError::UnauthorizedCapability { .. })));
}

#[test]
fn test_check_fails_on_first_missing_capability() {
    let mut caps = CapabilitySet::user_default();
    caps.add(Capability::ProvideLiquidity);
    // MintAsset and BurnAsset are not in user_default
    let result = CapabilityChecker::check(
        &caps,
        &[Capability::ProvideLiquidity, Capability::MintAsset],
    );
    assert!(matches!(result, Err(RuntimeError::UnauthorizedCapability { .. })));
}

// ──────────────────────────────────────────────
// CapabilityChecker::check_object_write
// ──────────────────────────────────────────────

#[test]
fn test_check_object_write_balance_requires_transfer_asset() {
    let obj: Box<dyn Object> = Box::new(BalanceObject::new(id(1), addr(1), addr(2), 100, 0));

    // user_default has WriteObject + TransferAsset → should succeed
    let caps = CapabilitySet::user_default();
    assert!(CapabilityChecker::check_object_write(&caps, obj.as_ref()).is_ok());

    // no WriteObject → fails
    let mut no_write = CapabilitySet::user_default();
    no_write.remove(Capability::WriteObject);
    assert!(CapabilityChecker::check_object_write(&no_write, obj.as_ref()).is_err());

    // WriteObject but no TransferAsset → fails
    let mut no_transfer = CapabilitySet::empty();
    no_transfer.add(Capability::WriteObject);
    assert!(CapabilityChecker::check_object_write(&no_transfer, obj.as_ref()).is_err());
}

#[test]
fn test_check_object_write_token_requires_mint_and_burn() {
    let obj: Box<dyn Object> = Box::new(TokenObject::new(
        id(2), addr(1), addr(2), "TEST".to_string(), 6, 1000, 0,
    ));

    // user_default does NOT have MintAsset → should fail
    let user_caps = CapabilitySet::user_default();
    assert!(CapabilityChecker::check_object_write(&user_caps, obj.as_ref()).is_err());

    // All caps → should succeed
    let all_caps = CapabilitySet::all();
    assert!(CapabilityChecker::check_object_write(&all_caps, obj.as_ref()).is_ok());
}

#[test]
fn test_check_object_write_pool_requires_provide_liquidity() {
    let obj: Box<dyn Object> = Box::new(LiquidityPoolObject::new(
        id(3), addr(1), addr(2), addr(3), 1000, 1000, 30, addr(4), 0,
    ));

    let user_caps = CapabilitySet::user_default();
    // user_default lacks ProvideLiquidity
    assert!(CapabilityChecker::check_object_write(&user_caps, obj.as_ref()).is_err());

    let all_caps = CapabilitySet::all();
    assert!(CapabilityChecker::check_object_write(&all_caps, obj.as_ref()).is_ok());
}

#[test]
fn test_check_object_write_governance_requires_modify_governance() {
    let obj: Box<dyn Object> = Box::new(GovernanceProposalObject::new(
        id(4), addr(1), 1, addr(1), "T".to_string(), "D".to_string(), 100, 0,
    ));

    let user_caps = CapabilitySet::user_default();
    assert!(CapabilityChecker::check_object_write(&user_caps, obj.as_ref()).is_err());

    let all_caps = CapabilitySet::all();
    assert!(CapabilityChecker::check_object_write(&all_caps, obj.as_ref()).is_ok());
}

#[test]
fn test_check_object_write_identity_requires_update_identity() {
    let obj: Box<dyn Object> = Box::new(IdentityObject::new(
        id(5), addr(1), addr(1), "Alice".to_string(), 0,
    ));

    let user_caps = CapabilitySet::user_default();
    assert!(CapabilityChecker::check_object_write(&user_caps, obj.as_ref()).is_err());

    let all_caps = CapabilitySet::all();
    assert!(CapabilityChecker::check_object_write(&all_caps, obj.as_ref()).is_ok());
}
