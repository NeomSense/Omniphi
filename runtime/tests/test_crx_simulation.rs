use omniphi_runtime::{
    crx::branch_execution::ExecutionSettlementClass,
    crx::finality::FinalityClass,
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::plan_builder::CausalPlanBuilder,
    crx::rights_capsule::AllowedActionType,
    crx::settlement::CRXSettlementEngine,
    crx::simulation::CRXSimulator,
    solver_market::market::{CandidatePlan, PlanAction, PlanActionType},
    BalanceObject, ObjectId, ObjectStore, ObjectType,
};
use std::collections::BTreeMap;

fn make_sender_id() -> [u8; 32] { [10u8; 32] }
fn make_recipient_id() -> [u8; 32] { [11u8; 32] }

fn make_store(sender_amount: u128) -> ObjectStore {
    let mut store = ObjectStore::new();
    let sender = BalanceObject::new(
        ObjectId::from(make_sender_id()),
        [1u8; 32],
        [5u8; 32],
        sender_amount,
        1,
    );
    let recipient = BalanceObject::new(
        ObjectId::from(make_recipient_id()),
        [2u8; 32],
        [5u8; 32],
        0,
        1,
    );
    store.insert(Box::new(sender));
    store.insert(Box::new(recipient));
    store
}

fn make_goal() -> GoalPacket {
    GoalPacket {
        packet_id: [1u8; 32],
        intent_id: [2u8; 32],
        sender: [3u8; 32],
        desired_outcome: "simulate transfer".to_string(),
        constraints: GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 300,
            allowed_object_ids: None,
            allowed_object_types: vec![ObjectType::Balance],
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 16,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: PartialFailurePolicy::StrictAllOrNothing,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: SettlementStrictness::Strict,
        },
        max_fee: 1000,
        deadline_epoch: 500,
        nonce: 1,
        metadata: BTreeMap::new(),
    }
}

fn make_transfer_plan(amount: u128) -> CandidatePlan {
    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());

    let actions = vec![
        PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: ObjectId::from(make_sender_id()),
            amount: Some(amount),
            metadata: debit_meta,
        },
        PlanAction {
            action_type: PlanActionType::CreditBalance,
            target_object: ObjectId::from(make_recipient_id()),
            amount: Some(amount),
            metadata: BTreeMap::new(),
        },
    ];

    let mut plan = CandidatePlan {
        plan_id: [1u8; 32],
        intent_id: [2u8; 32],
        solver_id: [3u8; 32],
        actions,
        required_capabilities: vec![omniphi_runtime::Capability::TransferAsset],
        object_reads: vec![],
        object_writes: vec![ObjectId::from(make_sender_id()), ObjectId::from(make_recipient_id())],
        expected_output_amount: amount,
        fee_quote: 100,
        quality_score: 8000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 0,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let hash = plan.compute_hash();
    plan.plan_hash = hash;
    plan
}

#[test]
fn test_simulation_matches_settlement_outcome() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let sim_result = CRXSimulator::simulate(&plan, &goal, &store, &[], 50);

    assert!(sim_result.would_pass_rights_validation, "Should pass rights; violations: {:?}", sim_result.rights_violations.iter().map(|v| format!("{:?}", v.breach_reason)).collect::<Vec<_>>());
    assert!(sim_result.would_pass_causal_validation, "Should pass causal; violations: {:?}", sim_result.causal_violations.iter().map(|v| format!("{:?}", v)).collect::<Vec<_>>());
    assert_eq!(sim_result.predicted_finality, FinalityClass::Finalized);
    assert_eq!(sim_result.predicted_settlement_class, ExecutionSettlementClass::FullSuccess);
}

#[test]
fn test_simulation_does_not_mutate_store() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let sender_before = store
        .get_balance_by_id(&ObjectId::from(make_sender_id()))
        .map(|b| b.amount)
        .unwrap_or(0);

    let _ = CRXSimulator::simulate(&plan, &goal, &store, &[], 50);

    let sender_after = store
        .get_balance_by_id(&ObjectId::from(make_sender_id()))
        .map(|b| b.amount)
        .unwrap_or(0);

    assert_eq!(
        sender_before, sender_after,
        "Simulation must not mutate store"
    );
}

#[test]
fn test_rights_inspection_view_correct() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let (graph, capsule) = CausalPlanBuilder::build(&plan, &goal, &store, 50).unwrap();
    let view = CRXSimulator::inspect_rights(&capsule);

    assert_eq!(view.capsule_id, capsule.capsule_id);
    assert_eq!(view.valid_epochs.0, capsule.valid_from_epoch);
    assert_eq!(view.valid_epochs.1, capsule.valid_until_epoch);
    assert!(view.allowed_actions.contains(&AllowedActionType::DebitBalance));
    assert!(view.allowed_actions.contains(&AllowedActionType::CreditBalance));
}

#[test]
fn test_graph_explainability_record() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let (graph, _) = CausalPlanBuilder::build(&plan, &goal, &store, 50).unwrap();
    let explain = CRXSimulator::explain_graph(&graph);

    assert_eq!(explain.graph_id, graph.graph_id);
    assert_eq!(explain.node_count, graph.nodes.len());
    assert_eq!(explain.edge_count, graph.edges.len());
    assert_eq!(explain.branch_count, graph.branches.len());
    assert!(!explain.node_summaries.is_empty());
    // Critical path should include at least one node
    assert!(!explain.critical_path.is_empty(), "Critical path should not be empty");
}

#[test]
fn test_simulation_previews_balance_changes() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(250);

    let sim = CRXSimulator::simulate(&plan, &goal, &store, &[], 50);

    // Should have balance changes: sender -250, recipient +250
    assert!(!sim.preview_balance_changes.is_empty(), "Should have balance change previews");

    let sender_change = sim.preview_balance_changes.iter()
        .find(|(obj, _)| obj == &make_sender_id())
        .map(|(_, delta)| *delta);
    let recipient_change = sim.preview_balance_changes.iter()
        .find(|(obj, _)| obj == &make_recipient_id())
        .map(|(_, delta)| *delta);

    assert_eq!(sender_change, Some(-250i128), "Sender should show -250 balance change");
    assert_eq!(recipient_change, Some(250i128), "Recipient should show +250 balance change");
}

#[test]
fn test_simulation_gas_estimate_nonzero() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let sim = CRXSimulator::simulate(&plan, &goal, &store, &[], 50);
    assert!(sim.estimated_gas > 0, "Estimated gas must be > 0");
}

#[test]
fn test_simulation_preview_hashes_nonzero() {
    let store = make_store(1000);
    let goal = make_goal();
    let plan = make_transfer_plan(100);

    let sim = CRXSimulator::simulate(&plan, &goal, &store, &[], 50);
    assert_ne!(sim.preview_graph_hash, [0u8; 32]);
    assert_ne!(sim.preview_capsule_hash, [0u8; 32]);
}
