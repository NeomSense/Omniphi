use omniphi_runtime::{
    crx::branch_execution::ExecutionSettlementClass,
    crx::finality::FinalityClass,
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::poseq_bridge::{OrderedGoalBatch, PoSeqCRXBridge},
    crx::settlement::CRXSettlementEngine,
    crx::simulation::CRXSimulator,
    policy::hooks::PermissivePolicy,
    selection::ranker::SelectionPolicy,
    solver_market::market::{CandidatePlan, PlanAction, PlanActionType},
    solver_registry::registry::{SolverCapabilities, SolverProfile, SolverStatus},
    BalanceObject, CapabilitySet, LiquidityPoolObject, ObjectId, ObjectStore, ObjectType,
};
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

fn mk_id(seed: u8) -> [u8; 32] {
    let mut id = [0u8; 32];
    id[0] = seed;
    id[31] = seed;
    id
}

fn make_goal_with_policy(
    packet_id: [u8; 32],
    intent_id: [u8; 32],
    policy: PartialFailurePolicy,
    forbidden_domains: Vec<String>,
) -> GoalPacket {
    GoalPacket {
        packet_id,
        intent_id,
        sender: mk_id(3),
        desired_outcome: "e2e test".to_string(),
        constraints: GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 500,
            allowed_object_ids: None,
            allowed_object_types: vec![ObjectType::Balance, ObjectType::LiquidityPool],
            allowed_domains: vec![],
            forbidden_domains,
            max_objects_touched: 16,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: policy,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: SettlementStrictness::Strict,
        },
        max_fee: 2000,
        deadline_epoch: 9999,
        nonce: 1,
        metadata: BTreeMap::new(),
    }
}

fn make_transfer_plan(
    plan_id: [u8; 32],
    intent_id: [u8; 32],
    sender_obj: [u8; 32],
    recipient_obj: [u8; 32],
    amount: u128,
) -> CandidatePlan {
    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());

    let actions = vec![
        PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: ObjectId::from(sender_obj),
            amount: Some(amount),
            metadata: debit_meta,
        },
        PlanAction {
            action_type: PlanActionType::CreditBalance,
            target_object: ObjectId::from(recipient_obj),
            amount: Some(amount),
            metadata: BTreeMap::new(),
        },
    ];

    let mut plan = CandidatePlan {
        plan_id,
        intent_id,
        solver_id: mk_id(99),
        actions,
        required_capabilities: vec![omniphi_runtime::Capability::TransferAsset],
        object_reads: vec![],
        object_writes: vec![ObjectId::from(sender_obj), ObjectId::from(recipient_obj)],
        expected_output_amount: amount,
        fee_quote: 200,
        quality_score: 9000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let h = plan.compute_hash();
    plan.plan_hash = h;
    plan
}

fn make_swap_plan(
    plan_id: [u8; 32],
    intent_id: [u8; 32],
    sender_in_obj: [u8; 32],
    sender_out_obj: [u8; 32],
    pool_obj: [u8; 32],
    input_amount: u128,
    output_amount: u128,
) -> CandidatePlan {
    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());
    let mut pool_meta = BTreeMap::new();
    pool_meta.insert("delta_a".to_string(), format!("{}", input_amount as i128));
    pool_meta.insert("delta_b".to_string(), format!("{}", -(output_amount as i128)));

    let actions = vec![
        PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: ObjectId::from(sender_in_obj),
            amount: Some(input_amount),
            metadata: debit_meta,
        },
        PlanAction {
            action_type: PlanActionType::CreditBalance,
            target_object: ObjectId::from(sender_out_obj),
            amount: Some(output_amount),
            metadata: BTreeMap::new(),
        },
        PlanAction {
            action_type: PlanActionType::SwapPoolAmounts,
            target_object: ObjectId::from(pool_obj),
            amount: Some(input_amount),
            metadata: pool_meta,
        },
    ];

    let mut plan = CandidatePlan {
        plan_id,
        intent_id,
        solver_id: mk_id(98),
        actions,
        required_capabilities: vec![omniphi_runtime::Capability::SwapAsset],
        object_reads: vec![],
        object_writes: vec![
            ObjectId::from(sender_in_obj),
            ObjectId::from(sender_out_obj),
            ObjectId::from(pool_obj),
        ],
        expected_output_amount: output_amount,
        fee_quote: 300,
        quality_score: 8500,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let h = plan.compute_hash();
    plan.plan_hash = h;
    plan
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_e2e_simple_transfer_via_crx() {
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        5000,
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    let goal = make_goal_with_policy(mk_id(20), mk_id(21), PartialFailurePolicy::StrictAllOrNothing, vec![]);
    let plan = make_transfer_plan(mk_id(30), mk_id(21), sender_obj, recipient_obj, 1000);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    assert_eq!(result.finality.finality_class, FinalityClass::Finalized);
    assert_eq!(result.settlement_class, ExecutionSettlementClass::FullSuccess);

    // Verify state
    store.sync_typed_to_canonical();
    let sender_bal = store.get_balance_by_id(&ObjectId::from(sender_obj)).unwrap();
    assert_eq!(sender_bal.amount, 4000);
    let recipient_bal = store.get_balance_by_id(&ObjectId::from(recipient_obj)).unwrap();
    assert_eq!(recipient_bal.amount, 1000);
}

#[test]
fn test_e2e_swap_intent_causal_validation() {
    let sender_in_obj = mk_id(10);
    let sender_out_obj = mk_id(11);
    let pool_obj = mk_id(12);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_in_obj),
        mk_id(1),
        mk_id(5),
        2000,
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_out_obj),
        mk_id(1),
        mk_id(6),
        0,
        1,
    )));
    store.insert(Box::new(LiquidityPoolObject::new(
        ObjectId::from(pool_obj),
        mk_id(1),
        mk_id(5),
        mk_id(6),
        100_000,
        100_000,
        30,
        mk_id(50),
        1,
    )));

    let goal = make_goal_with_policy(mk_id(40), mk_id(41), PartialFailurePolicy::StrictAllOrNothing, vec![]);
    let plan = make_swap_plan(mk_id(42), mk_id(41), sender_in_obj, sender_out_obj, pool_obj, 500, 490);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // Should succeed (or at worst be rejected due to rights — we check graph was built)
    assert_ne!(result.graph_receipt.graph_id, [0u8; 32], "Graph should be built");
    // Swap plan has causal graph with CheckBalance → SwapPoolAmounts nodes
    assert!(result.graph_receipt.total_nodes > 0);
}

#[test]
fn test_e2e_treasury_restricted_domain() {
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        5000,
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    // Goal forbids "treasury" domain
    let goal = make_goal_with_policy(
        mk_id(50),
        mk_id(51),
        PartialFailurePolicy::StrictAllOrNothing,
        vec!["treasury".to_string()],
    );

    // Plan with a node tagged as "treasury" domain — simulate via domain-tagged plan metadata
    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());
    debit_meta.insert("domain_tag".to_string(), "treasury".to_string());

    let actions = vec![
        PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: ObjectId::from(sender_obj),
            amount: Some(100),
            metadata: debit_meta,
        },
    ];

    let mut plan = CandidatePlan {
        plan_id: mk_id(52),
        intent_id: mk_id(51),
        solver_id: mk_id(99),
        actions,
        required_capabilities: vec![],
        object_reads: vec![],
        object_writes: vec![ObjectId::from(sender_obj)],
        expected_output_amount: 100,
        fee_quote: 100,
        quality_score: 5000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let h = plan.compute_hash();
    plan.plan_hash = h;

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // The causal validity should fail due to forbidden domain
    // OR the plan may be rejected at rights validation
    // Either way the plan should not succeed with FullSuccess
    assert_ne!(
        result.settlement_class,
        ExecutionSettlementClass::FullSuccess,
        "Plan touching forbidden treasury domain should not fully succeed"
    );
}

#[test]
fn test_e2e_branch_downgrade() {
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        50, // Low balance — will fail debit of 200
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    // AllowBranchDowngrade policy
    let goal = make_goal_with_policy(
        mk_id(60),
        mk_id(61),
        PartialFailurePolicy::AllowBranchDowngrade,
        vec![],
    );
    let plan = make_transfer_plan(mk_id(62), mk_id(61), sender_obj, recipient_obj, 200);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // Should be downgraded or reverted (not FullSuccess since balance insufficient)
    assert!(
        result.finality.finality_class == FinalityClass::FinalizedWithDowngrade
            || result.finality.finality_class == FinalityClass::Reverted
            || result.settlement_class == ExecutionSettlementClass::FullRevert,
        "Expected downgrade or revert, got {:?} / {:?}",
        result.finality.finality_class,
        result.settlement_class
    );
}

#[test]
fn test_e2e_quarantine_scenario() {
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        10, // Insufficient for debit of 500
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    let goal = make_goal_with_policy(
        mk_id(70),
        mk_id(71),
        PartialFailurePolicy::QuarantineOnFailure,
        vec![],
    );
    let plan = make_transfer_plan(mk_id(72), mk_id(71), sender_obj, recipient_obj, 500);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // Quarantine or revert
    assert!(
        result.finality.finality_class == FinalityClass::FinalizedWithQuarantine
            || result.finality.finality_class == FinalityClass::Reverted
            || result.settlement_class == ExecutionSettlementClass::FullRevert,
        "Expected quarantine or revert, got {:?} / {:?}",
        result.finality.finality_class,
        result.settlement_class
    );
}

#[test]
fn test_e2e_causally_invalid_plan_rejected() {
    // A plan where the causal graph would be acyclic (valid)
    // but rights will fail due to expired capsule
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        5000,
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    // Deadline epoch in the past
    let mut goal = make_goal_with_policy(
        mk_id(80),
        mk_id(81),
        PartialFailurePolicy::StrictAllOrNothing,
        vec![],
    );
    goal.deadline_epoch = 5; // Very short deadline

    let plan = make_transfer_plan(mk_id(82), mk_id(81), sender_obj, recipient_obj, 100);

    // Run at epoch 100, which is after deadline (5)
    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // Capsule expires at deadline_epoch (5), so at epoch 100 rights should fail
    assert_eq!(result.settlement_class, ExecutionSettlementClass::Rejected);
    assert_eq!(result.causal_summary.rights_validity, false);
}

#[test]
fn test_e2e_poseq_ordered_batch() {
    let mut bridge = PoSeqCRXBridge::new(
        SelectionPolicy::BestScore,
        Box::new(PermissivePolicy),
    );

    // Register a solver
    let solver = SolverProfile {
        solver_id: mk_id(99),
        display_name: "TestSolver".to_string(),
        public_key: mk_id(99),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: CapabilitySet::all(),
            allowed_intent_classes: vec!["transfer".to_string(), "swap".to_string()],
            domain_tags: vec![],
            max_objects_per_plan: 16,
        },
        reputation: Default::default(),
        stake_amount: 1000,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: false,
        metadata: BTreeMap::new(),
    };
    let _ = bridge.register_solver(solver);

    // Seed store with balance objects
    let sender1 = mk_id(10);
    let recipient1 = mk_id(11);
    let sender2 = mk_id(12);
    let recipient2 = mk_id(13);
    let sender3 = mk_id(14);
    let recipient3 = mk_id(15);

    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(sender1), mk_id(1), mk_id(5), 5000, 1)));
    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(recipient1), mk_id(2), mk_id(5), 0, 1)));
    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(sender2), mk_id(3), mk_id(5), 3000, 1)));
    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(recipient2), mk_id(4), mk_id(5), 0, 1)));
    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(sender3), mk_id(6), mk_id(5), 2000, 1)));
    bridge.seed_object(Box::new(BalanceObject::new(ObjectId::from(recipient3), mk_id(7), mk_id(5), 0, 1)));

    // Build 3 goals
    let goal1 = make_goal_with_policy(mk_id(20), mk_id(21), PartialFailurePolicy::StrictAllOrNothing, vec![]);
    let goal2 = make_goal_with_policy(mk_id(22), mk_id(23), PartialFailurePolicy::StrictAllOrNothing, vec![]);
    let goal3 = make_goal_with_policy(mk_id(24), mk_id(25), PartialFailurePolicy::StrictAllOrNothing, vec![]);

    // Build plans for each goal
    let plan1 = make_transfer_plan(mk_id(30), mk_id(21), sender1, recipient1, 100);
    let plan2 = make_transfer_plan(mk_id(31), mk_id(23), sender2, recipient2, 200);
    let plan3 = make_transfer_plan(mk_id(32), mk_id(25), sender3, recipient3, 300);

    let mut candidate_plans: BTreeMap<[u8; 32], Vec<CandidatePlan>> = BTreeMap::new();
    candidate_plans.insert(goal1.packet_id, vec![plan1]);
    candidate_plans.insert(goal2.packet_id, vec![plan2]);
    candidate_plans.insert(goal3.packet_id, vec![plan3]);

    let batch = OrderedGoalBatch {
        batch_id: mk_id(99),
        epoch: 100,
        sequence_number: 1,
        goals: vec![goal1, goal2, goal3],
        candidate_plans,
    };

    let results = bridge.process_batch(batch);

    // We expect 3 results
    assert_eq!(results.len(), 3, "Should have 3 execution results for 3 goals");

    // All should succeed
    for result in &results {
        assert!(
            result.success,
            "Goal {:?} should succeed; finality={:?}",
            hex::encode(&result.goal_packet_id[..4]),
            result.record.finality.finality_class
        );
    }
}

#[test]
fn test_e2e_simulation_consistency() {
    // Simulate first, then settle, and verify the simulation predicted the outcome
    let sender_obj = mk_id(10);
    let recipient_obj = mk_id(11);

    let mut store = ObjectStore::new();
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(sender_obj),
        mk_id(1),
        mk_id(5),
        1000,
        1,
    )));
    store.insert(Box::new(BalanceObject::new(
        ObjectId::from(recipient_obj),
        mk_id(2),
        mk_id(5),
        0,
        1,
    )));

    let goal = make_goal_with_policy(mk_id(90), mk_id(91), PartialFailurePolicy::StrictAllOrNothing, vec![]);
    let plan = make_transfer_plan(mk_id(92), mk_id(91), sender_obj, recipient_obj, 500);

    // Simulate
    let sim = CRXSimulator::simulate(&plan, &goal, &store, &[], 100);

    // Settle
    let record = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 100).unwrap();

    // Simulation should predict settlement outcome
    assert_eq!(
        sim.predicted_finality,
        record.finality.finality_class,
        "Simulation predicted {:?}, actual was {:?}",
        sim.predicted_finality,
        record.finality.finality_class
    );
}
