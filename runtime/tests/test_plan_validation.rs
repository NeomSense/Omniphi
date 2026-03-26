use omniphi_runtime::capabilities::checker::CapabilitySet;
use omniphi_runtime::intents::base::{IntentTransaction, IntentType, IntentConstraints, ExecutionMode};
use omniphi_runtime::intents::types::SwapIntent;
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::plan_validation::validator::{PlanValidator, ValidationReasonCode};
use omniphi_runtime::policy::hooks::PermissivePolicy;
use omniphi_runtime::policy::hooks::MaxValuePolicy;
use omniphi_runtime::solver_market::market::{
    CandidatePlan, PlanAction, PlanActionType, PlanConstraintProof,
};
use omniphi_runtime::solver_registry::registry::{
    SolverCapabilities, SolverProfile, SolverRegistry, SolverReputationRecord, SolverStatus,
};
use omniphi_runtime::state::store::ObjectStore;
use std::collections::BTreeMap;

// ─── helpers ────────────────────────────────────────────────────────────────

fn oid(n: u8) -> ObjectId {
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

fn make_registry_with_active_solver(solver_id_byte: u8, max_objects: usize) -> (SolverRegistry, [u8; 32]) {
    let mut registry = SolverRegistry::new();
    let solver_id = addr(solver_id_byte);
    let profile = SolverProfile {
        solver_id,
        display_name: "TestSolver".to_string(),
        public_key: addr(solver_id_byte + 50),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: CapabilitySet::all(),
            allowed_intent_classes: vec!["swap".to_string(), "transfer".to_string()],
            domain_tags: vec!["defi".to_string()],
            max_objects_per_plan: max_objects,
        },
        reputation: SolverReputationRecord::default(),
        stake_amount: 1_000_000,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: false,
        metadata: BTreeMap::new(),
    };
    registry.register(profile).unwrap();
    (registry, solver_id)
}

fn make_swap_intent(max_fee: u64) -> IntentTransaction {
    IntentTransaction {
        tx_id: txid(1),
        sender: addr(0xAA),
        intent: IntentType::Swap(SwapIntent {
            input_asset: addr(1),
            output_asset: addr(2),
            input_amount: 500,
            min_output_amount: 1,
            max_slippage_bps: 500,
            allowed_pool_ids: None,
        }),
        max_fee,
        deadline_epoch: 100,
        nonce: 1,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
    }
}

fn make_balance(id: ObjectId, owner: [u8; 32], asset: [u8; 32], amount: u128) -> BalanceObject {
    BalanceObject::new(id, owner, asset, amount, 1)
}

/// Build a valid plan and compute its hash.
fn build_valid_plan(
    solver_id: [u8; 32],
    intent: &IntentTransaction,
    balance_id: ObjectId,
    output_amount: u128,
    fee_quote: u64,
) -> CandidatePlan {
    let mut plan = CandidatePlan {
        plan_id: txid(99),
        intent_id: intent.tx_id,
        solver_id,
        actions: vec![PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: balance_id,
            amount: Some(500),
            metadata: BTreeMap::new(),
        }],
        required_capabilities: vec![omniphi_runtime::capabilities::checker::Capability::SwapAsset],
        object_reads: vec![balance_id],
        object_writes: vec![balance_id],
        expected_output_amount: output_amount,
        fee_quote,
        quality_score: 8000,
        constraint_proofs: vec![PlanConstraintProof {
            constraint_name: "min_output".to_string(),
            declared_satisfied: true,
            supporting_value: Some(output_amount),
            explanation: None,
        }],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let hash = plan.compute_hash();
    plan.plan_hash = hash;
    plan
}

// ─── tests ──────────────────────────────────────────────────────────────────

#[test]
fn test_valid_plan_passes() {
    let (registry, solver_id) = make_registry_with_active_solver(1, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    let owner = addr(0xAA);
    let asset = addr(1);
    store.insert(Box::new(make_balance(balance_id, owner, asset, 10_000)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(result.passed, "plan should pass; codes: {:?}", result.reason_codes);
    assert!(result.reason_codes.contains(&ValidationReasonCode::Valid));
    assert!(result.normalized_score > 0);
}

#[test]
fn test_plan_hash_mismatch_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(1, 10);
    let store = ObjectStore::new();
    let intent = make_swap_intent(1000);
    let balance_id = oid(10);

    let mut plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    // Corrupt the stored hash
    plan.plan_hash = [0xABu8; 32];

    let policy = PermissivePolicy;
    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::PlanHashMismatch));
}

#[test]
fn test_inactive_solver_rejected() {
    let (mut registry, solver_id) = make_registry_with_active_solver(2, 10);
    registry.set_status(&solver_id, omniphi_runtime::solver_registry::registry::SolverStatus::Paused).unwrap();

    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::SolverNotActive));
}

#[test]
fn test_insufficient_balance_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(3, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    // Only 100 available, plan tries to debit 500
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 100)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::InsufficientBalance));
}

#[test]
fn test_missing_object_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(4, 10);
    let store = ObjectStore::new(); // empty store — no objects

    let intent = make_swap_intent(1000);
    let balance_id = oid(99); // doesn't exist in store
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::ObjectNotFound));
}

#[test]
fn test_capability_insufficient_rejected() {
    let mut registry = SolverRegistry::new();
    let solver_id = addr(5);
    // Solver only has ReadObject
    let mut caps = CapabilitySet::empty();
    caps.add(omniphi_runtime::capabilities::checker::Capability::ReadObject);
    let profile = SolverProfile {
        solver_id,
        display_name: "WeakSolver".to_string(),
        public_key: addr(55),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: caps,
            allowed_intent_classes: vec!["swap".to_string()],
            domain_tags: vec![],
            max_objects_per_plan: 10,
        },
        reputation: SolverReputationRecord::default(),
        stake_amount: 0,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: false,
        metadata: BTreeMap::new(),
    };
    registry.register(profile).unwrap();

    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::SolverCapabilityInsufficient));
}

#[test]
fn test_policy_violation_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(6, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(1000);
    // Plan output is 5000; MaxValuePolicy allows only 100
    let plan = build_valid_plan(solver_id, &intent, balance_id, 5000, 100);
    let policy = MaxValuePolicy { max_value: 100 };

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::PolicyRejected));
}

#[test]
fn test_fee_too_high_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(7, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(50); // max_fee = 50
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 200); // fee_quote = 200
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::FeeTooHigh));
}

#[test]
fn test_zero_output_rejected() {
    let (registry, solver_id) = make_registry_with_active_solver(8, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 0, 100); // expected_output=0

    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(!result.passed);
    assert!(result.reason_codes.contains(&ValidationReasonCode::ZeroOutput));
}

#[test]
fn test_normalized_score_computed() {
    let (registry, solver_id) = make_registry_with_active_solver(9, 10);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(make_balance(balance_id, addr(0xAA), addr(1), 10_000)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, &intent, balance_id, 450, 100);
    let policy = PermissivePolicy;

    let result = PlanValidator::validate(&plan, &intent, &store, &registry, &policy);
    assert!(result.passed);
    // Score should be > 0 and <= 10000
    assert!(result.normalized_score > 0);
    assert!(result.normalized_score <= 10_000);
}
