use omniphi_runtime::{
    crx::rights_capsule::{
        AllowedActionSet, AllowedActionType, DomainAccessEnvelope, RightsCapsule, RightsScope,
    },
    ObjectType,
};
use std::collections::{BTreeMap, BTreeSet};

fn make_capsule() -> RightsCapsule {
    let mut allowed_types: BTreeSet<ObjectType> = BTreeSet::new();
    allowed_types.insert(ObjectType::Balance);
    allowed_types.insert(ObjectType::LiquidityPool);

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: allowed_types,
        allowed_actions: AllowedActionSet::transfer_only(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 10_000,
        max_objects_touched: 8,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut capsule = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 10,
        valid_until_epoch: 100,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    capsule.capsule_hash = capsule.compute_hash();
    capsule
}

#[test]
fn test_capsule_hash_deterministic() {
    let c = make_capsule();
    assert_eq!(c.compute_hash(), c.compute_hash());
    assert!(c.validate_hash(), "Hash should be valid after compute_hash");
}

#[test]
fn test_capsule_expired_at_epoch() {
    let c = make_capsule();
    // Before valid_from
    assert!(!c.is_valid_at_epoch(9));
    // After valid_until
    assert!(!c.is_valid_at_epoch(101));
}

#[test]
fn test_capsule_valid_at_epoch() {
    let c = make_capsule();
    assert!(c.is_valid_at_epoch(10));
    assert!(c.is_valid_at_epoch(50));
    assert!(c.is_valid_at_epoch(100));
}

#[test]
fn test_object_not_in_scope() {
    let c = make_capsule();
    // WalletObject is NOT in allowed_object_types
    assert!(!c.can_access_object(&[99u8; 32], &ObjectType::Wallet));
}

#[test]
fn test_object_in_scope() {
    let c = make_capsule();
    // Balance IS in allowed_object_types; allowed_object_ids is empty (any)
    assert!(c.can_access_object(&[99u8; 32], &ObjectType::Balance));
    assert!(c.can_access_object(&[42u8; 32], &ObjectType::LiquidityPool));
}

#[test]
fn test_object_explicit_whitelist() {
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);

    let mut allowed_ids: BTreeSet<[u8; 32]> = BTreeSet::new();
    allowed_ids.insert([7u8; 32]);

    let scope = RightsScope {
        allowed_object_ids: allowed_ids,
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback: false,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut c = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    c.capsule_hash = c.compute_hash();

    // [7u8; 32] is in whitelist and Balance type is allowed
    assert!(c.can_access_object(&[7u8; 32], &ObjectType::Balance));
    // [8u8; 32] is NOT in whitelist
    assert!(!c.can_access_object(&[8u8; 32], &ObjectType::Balance));
}

#[test]
fn test_action_not_allowed() {
    let c = make_capsule();
    // transfer_only has: DebitBalance, CreditBalance, UpdateVersion, EmitReceipt
    assert!(c.can_perform_action(&AllowedActionType::DebitBalance));
    assert!(c.can_perform_action(&AllowedActionType::CreditBalance));
    // SwapPoolAmounts is NOT in transfer_only
    assert!(!c.can_perform_action(&AllowedActionType::SwapPoolAmounts));
    assert!(!c.can_perform_action(&AllowedActionType::LockBalance));
}

#[test]
fn test_domain_forbidden_takes_precedence() {
    let mut forbidden: BTreeSet<String> = BTreeSet::new();
    forbidden.insert("treasury".to_string());
    let mut allowed: BTreeSet<String> = BTreeSet::new();
    allowed.insert("treasury".to_string()); // forbidden takes precedence

    let envelope = DomainAccessEnvelope {
        allowed_domains: allowed,
        forbidden_domains: forbidden,
    };

    // Even though "treasury" is in allowed, forbidden takes precedence
    assert!(!envelope.is_domain_allowed("treasury"));
}

#[test]
fn test_domain_allowed_when_in_allowlist() {
    let mut allowed: BTreeSet<String> = BTreeSet::new();
    allowed.insert("defi".to_string());

    let envelope = DomainAccessEnvelope {
        allowed_domains: allowed,
        forbidden_domains: BTreeSet::new(),
    };

    assert!(envelope.is_domain_allowed("defi"));
    assert!(!envelope.is_domain_allowed("treasury")); // not in allowed list
}

#[test]
fn test_domain_allowed_when_unrestricted() {
    let envelope = DomainAccessEnvelope::unrestricted();
    assert!(envelope.is_domain_allowed("treasury"));
    assert!(envelope.is_domain_allowed("defi"));
    assert!(envelope.is_domain_allowed("any_domain"));
}

#[test]
fn test_transfer_only_action_set() {
    let s = AllowedActionSet::transfer_only();
    assert!(s.contains(&AllowedActionType::DebitBalance));
    assert!(s.contains(&AllowedActionType::CreditBalance));
    assert!(s.contains(&AllowedActionType::UpdateVersion));
    assert!(s.contains(&AllowedActionType::EmitReceipt));
    assert!(!s.contains(&AllowedActionType::SwapPoolAmounts));
    assert!(!s.contains(&AllowedActionType::LockBalance));
    assert!(!s.contains(&AllowedActionType::CreateObject));
}

#[test]
fn test_swap_only_action_set() {
    let s = AllowedActionSet::swap_only();
    assert!(s.contains(&AllowedActionType::DebitBalance));
    assert!(s.contains(&AllowedActionType::CreditBalance));
    assert!(s.contains(&AllowedActionType::SwapPoolAmounts));
    assert!(s.contains(&AllowedActionType::UpdateVersion));
    assert!(s.contains(&AllowedActionType::EmitReceipt));
    assert!(!s.contains(&AllowedActionType::LockBalance));
}

#[test]
fn test_full_action_set() {
    let s = AllowedActionSet::full();
    assert!(s.contains(&AllowedActionType::Read));
    assert!(s.contains(&AllowedActionType::SwapPoolAmounts));
    assert!(s.contains(&AllowedActionType::LockBalance));
    assert!(s.contains(&AllowedActionType::UnlockBalance));
    assert!(s.contains(&AllowedActionType::CreateObject));
    assert!(s.contains(&AllowedActionType::InvokeSafetyHook));
}
