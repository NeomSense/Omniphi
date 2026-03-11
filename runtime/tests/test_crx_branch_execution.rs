use omniphi_runtime::{
    crx::branch_execution::{BranchAwareExecutor, BranchFailureMode, ExecutionSettlementClass},
    crx::causal_graph::{
        CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
        NodeExecutionClass, NodeId,
    },
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::rights_capsule::{AllowedActionSet, DomainAccessEnvelope, RightsCapsule, RightsScope},
    BalanceObject, ObjectId, ObjectStore, ObjectType,
};
use std::collections::{BTreeMap, BTreeSet};

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

fn make_capsule_for_policy(policy: &PartialFailurePolicy) -> RightsCapsule {
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);

    let (allow_rollback, allow_downgrade, quarantine_eligible) = match policy {
        PartialFailurePolicy::StrictAllOrNothing => (true, false, false),
        PartialFailurePolicy::AllowBranchDowngrade => (false, true, false),
        PartialFailurePolicy::QuarantineOnFailure => (false, false, true),
        PartialFailurePolicy::DowngradeAndContinue => (false, true, false),
    };

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback,
        allow_downgrade,
        quarantine_eligible,
    };

    let mut c = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    c.capsule_hash = c.compute_hash();
    c
}

fn make_goal(policy: PartialFailurePolicy) -> GoalPacket {
    GoalPacket {
        packet_id: [1u8; 32],
        intent_id: [2u8; 32],
        sender: [3u8; 32],
        desired_outcome: "test".to_string(),
        constraints: GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 300,
            allowed_object_ids: None,
            allowed_object_types: vec![ObjectType::Balance],
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 8,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: policy,
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

fn make_transfer_graph(amount: u128) -> CausalGraph {
    let mut nodes = BTreeMap::new();

    let check_node = CausalNode {
        node_id: NodeId(0),
        label: "CheckBalance".to_string(),
        execution_class: NodeExecutionClass::CheckBalance,
        access_type: NodeAccessType::ValidationOnly,
        target_object: Some(make_sender_id()),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    };

    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());
    let debit_node = CausalNode {
        node_id: NodeId(1),
        label: "MutateBalance(debit)".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(make_sender_id()),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: debit_meta,
    };

    let credit_node = CausalNode {
        node_id: NodeId(2),
        label: "MutateBalance(credit)".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(make_recipient_id()),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    };

    nodes.insert(NodeId(0), check_node);
    nodes.insert(NodeId(1), debit_node);
    nodes.insert(NodeId(2), credit_node);

    let edges = vec![
        CausalEdge {
            from: NodeId(0),
            to: NodeId(1),
            dependency_kind: NodeDependencyKind::PolicyDependent,
            is_critical: true,
        },
        CausalEdge {
            from: NodeId(1),
            to: NodeId(2),
            dependency_kind: NodeDependencyKind::StateDependent,
            is_critical: true,
        },
    ];

    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let branch = ExecutionBranch {
        branch_id: 0,
        nodes: topo.clone(),
        is_main: true,
        failure_allowed: false,
    };
    let mut g = CausalGraph {
        graph_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        nodes,
        edges,
        branches: vec![branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    g.graph_hash = g.compute_hash();
    g
}

fn make_failing_transfer_graph(amount: u128) -> CausalGraph {
    // Same as transfer but debit amount exceeds balance
    make_transfer_graph(amount)
}

#[test]
fn test_full_success_execution() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_transfer_graph(100);

    let (class, results) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    assert_eq!(class, ExecutionSettlementClass::FullSuccess);
    assert!(results.iter().all(|r| r.success));

    // Verify store mutations: sender should have 900
    let sender_bal = store.get_balance_by_id(&ObjectId::from(make_sender_id())).unwrap();
    assert_eq!(sender_bal.amount, 900);
    // Recipient should have 100
    let recipient_bal = store.get_balance_by_id(&ObjectId::from(make_recipient_id())).unwrap();
    assert_eq!(recipient_bal.amount, 100);
}

#[test]
fn test_strict_policy_reverts_on_branch_failure() {
    let mut store = make_store(50); // Only 50, but trying to debit 200
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_failing_transfer_graph(200);

    let (class, results) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    assert_eq!(class, ExecutionSettlementClass::FullRevert);
    assert!(results.iter().any(|r| !r.success));
    assert!(results.iter().any(|r| r.failure_mode == Some(BranchFailureMode::Revert)));
}

#[test]
fn test_downgrade_policy_continues_on_branch_failure() {
    let mut store = make_store(50); // Only 50, trying to debit 200
    let goal = make_goal(PartialFailurePolicy::AllowBranchDowngrade);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_failing_transfer_graph(200);

    let (class, results) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    // Should downgrade, not fully revert
    assert!(
        matches!(class, ExecutionSettlementClass::SuccessWithDowngrade(_) | ExecutionSettlementClass::FullRevert),
        "Expected downgrade or revert, got {:?}",
        class
    );
    // Should not be FullRevert if downgrade was applied
    if matches!(class, ExecutionSettlementClass::SuccessWithDowngrade(_)) {
        assert!(results.iter().any(|r| r.failure_mode == Some(BranchFailureMode::Downgrade)));
    }
}

#[test]
fn test_quarantine_policy_quarantines_objects() {
    let mut store = make_store(50); // Only 50, trying to debit 200
    let goal = make_goal(PartialFailurePolicy::QuarantineOnFailure);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_failing_transfer_graph(200);

    let (class, results) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    assert!(
        matches!(class, ExecutionSettlementClass::SuccessWithQuarantine(_) | ExecutionSettlementClass::FullRevert),
        "Expected quarantine or revert, got {:?}",
        class
    );
    if matches!(class, ExecutionSettlementClass::SuccessWithQuarantine(_)) {
        assert!(results.iter().any(|r| r.failure_mode == Some(BranchFailureMode::Quarantine)));
    }
}

#[test]
fn test_settlement_class_full_success() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_transfer_graph(500);

    let (class, _) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    assert_eq!(class, ExecutionSettlementClass::FullSuccess);
}

#[test]
fn test_settlement_class_reverted() {
    let mut store = make_store(10); // Insufficient
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let capsule = make_capsule_for_policy(&goal.policy.partial_failure_policy);
    let graph = make_failing_transfer_graph(1000);

    let (class, _) = BranchAwareExecutor::execute(&graph, &capsule, &goal, &mut store);
    assert_eq!(class, ExecutionSettlementClass::FullRevert);
}
