use omniphi_runtime::capabilities::checker::CapabilitySet;
use omniphi_runtime::intents::base::{IntentTransaction, IntentType};
use omniphi_runtime::intents::types::SwapIntent;
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::plan_validation::validator::ValidationReasonCode;
use omniphi_runtime::policy::hooks::PermissivePolicy;
use omniphi_runtime::simulation::simulator::PlanSimulator;
use omniphi_runtime::solver_market::market::{CandidatePlan, PlanAction, PlanActionType};
use omniphi_runtime::solver_registry::registry::{
    SolverCapabilities, SolverProfile, SolverRegistry, SolverReputationRecord, SolverStatus,
};
use omniphi_runtime::state::store::ObjectStore;
use std::collections::BTreeMap;

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

fn oid(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}

fn make_active_registry(solver_id: [u8; 32]) -> SolverRegistry {
    let mut registry = SolverRegistry::new();
    let profile = SolverProfile {
        solver_id,
        display_name: "SimSolver".to_string(),
        public_key: addr(100),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: CapabilitySet::all(),
            allowed_intent_classes: vec!["swap".to_string()],
            domain_tags: vec![],
            max_objects_per_plan: 20,
        },
        reputation: SolverReputationRecord::default(),
        stake_amount: 0,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: false,
        metadata: BTreeMap::new(),
    };
    registry.register(profile).unwrap();
    registry
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
    }
}

fn build_valid_plan(solver_id: [u8; 32], balance_id: ObjectId, debit_amount: u128) -> CandidatePlan {
    let mut plan = CandidatePlan {
        plan_id: txid(77),
        intent_id: txid(1),
        solver_id,
        actions: vec![PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: balance_id,
            amount: Some(debit_amount),
            metadata: BTreeMap::new(),
        }],
        required_capabilities: vec![omniphi_runtime::capabilities::checker::Capability::SwapAsset],
        object_reads: vec![balance_id],
        object_writes: vec![balance_id],
        expected_output_amount: debit_amount / 2,
        fee_quote: 50,
        quality_score: 8000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    plan.plan_hash = plan.compute_hash();
    plan
}

#[test]
fn test_simulation_matches_validation_result() {
    let solver_id = addr(30);
    let registry = make_active_registry(solver_id);
    let mut store = ObjectStore::new();
    let balance_id = oid(10);
    store.insert(Box::new(BalanceObject::new(balance_id, addr(0xAA), addr(1), 10_000, 1)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, balance_id, 500);
    let policy = PermissivePolicy;

    let sim_result = PlanSimulator::simulate(&plan, &intent, &store, &registry, &policy);
    assert!(sim_result.would_pass_validation, "simulation should indicate passing plan");
    assert!(sim_result.validation_reason_codes.contains(&ValidationReasonCode::Valid));
}

#[test]
fn test_simulation_preview_balance_changes_correct() {
    let solver_id = addr(31);
    let registry = make_active_registry(solver_id);
    let mut store = ObjectStore::new();
    let balance_id = oid(11);
    store.insert(Box::new(BalanceObject::new(balance_id, addr(0xAA), addr(1), 10_000, 1)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, balance_id, 300);
    let policy = PermissivePolicy;

    let sim_result = PlanSimulator::simulate(&plan, &intent, &store, &registry, &policy);
    assert!(sim_result.would_pass_validation);

    // Should show a debit of -300 for balance_id
    let change = sim_result
        .preview_balance_changes
        .iter()
        .find(|(id, _)| *id == balance_id);
    assert!(change.is_some(), "should have preview change for the debited balance");
    let (_, delta) = change.unwrap();
    assert_eq!(*delta, -300i128);
}

#[test]
fn test_simulation_does_not_mutate_real_store() {
    let solver_id = addr(32);
    let registry = make_active_registry(solver_id);
    let mut store = ObjectStore::new();
    let balance_id = oid(12);
    store.insert(Box::new(BalanceObject::new(balance_id, addr(0xAA), addr(1), 5_000, 1)));

    let intent = make_swap_intent(1000);
    let plan = build_valid_plan(solver_id, balance_id, 1000);
    let policy = PermissivePolicy;

    // Capture balance before simulation
    let balance_before = store.get_balance_by_id(&balance_id).unwrap().amount;

    let _sim_result = PlanSimulator::simulate(&plan, &intent, &store, &registry, &policy);

    // Balance in the real store must be unchanged
    let balance_after = store.get_balance_by_id(&balance_id).unwrap().amount;
    assert_eq!(balance_before, balance_after, "simulation must not mutate real store");
}

#[test]
fn test_simulation_vs_production_consistency() {
    // A plan that would fail validation should also report would_pass_validation=false in simulation
    let solver_id = addr(33);
    let registry = make_active_registry(solver_id);
    let store = ObjectStore::new(); // empty — objects don't exist

    let intent = make_swap_intent(1000);
    let balance_id = oid(99); // not in store
    let plan = build_valid_plan(solver_id, balance_id, 500);
    let policy = PermissivePolicy;

    let sim_result = PlanSimulator::simulate(&plan, &intent, &store, &registry, &policy);
    assert!(!sim_result.would_pass_validation, "missing object should make simulation fail");
    assert!(!sim_result.validation_reason_codes.contains(&ValidationReasonCode::Valid));
}
