use omniphi_runtime::{
    crx::causal_graph::{
        CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
        NodeExecutionClass, NodeId,
    },
    crx::rights_capsule::{
        AllowedActionSet, AllowedActionType, DomainAccessEnvelope, RightsCapsule, RightsScope,
    },
    crx::rights_validation::{RightsValidationEngine, ScopeBreachReason},
    BalanceObject, ObjectId, ObjectStore, ObjectType,
};
use std::collections::{BTreeMap, BTreeSet};

fn make_balance_id() -> [u8; 32] {
    [10u8; 32]
}

fn make_store_with_balance() -> ObjectStore {
    let mut store = ObjectStore::new();
    let bal = BalanceObject::new(
        ObjectId::from(make_balance_id()),
        [1u8; 32],
        [5u8; 32],
        5000,
        1,
    );
    store.insert(Box::new(bal));
    store
}

fn make_transfer_capsule(epoch_from: u64, epoch_until: u64, max_spend: u128) -> RightsCapsule {
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::transfer_only(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend,
        max_objects_touched: 8,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut c = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: epoch_from,
        valid_until_epoch: epoch_until,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    c.capsule_hash = c.compute_hash();
    c
}

fn make_transfer_graph(obj_id: [u8; 32], amount: u128) -> CausalGraph {
    let mut nodes = BTreeMap::new();

    let mut check_node = CausalNode {
        node_id: NodeId(0),
        label: "CheckBalance(debit)".to_string(),
        execution_class: NodeExecutionClass::CheckBalance,
        access_type: NodeAccessType::ValidationOnly,
        target_object: Some(obj_id),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    };

    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());
    let mut mutate_node = CausalNode {
        node_id: NodeId(1),
        label: "MutateBalance(debit)".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(obj_id),
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
        target_object: Some([11u8; 32]),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    };

    nodes.insert(NodeId(0), check_node);
    nodes.insert(NodeId(1), mutate_node);
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

    let mut graph = CausalGraph {
        graph_id: [7u8; 32],
        goal_packet_id: [8u8; 32],
        nodes,
        edges,
        branches: vec![branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    graph.graph_hash = graph.compute_hash();
    graph
}

#[test]
fn test_all_nodes_pass_rights() {
    let store = make_store_with_balance();
    let capsule = make_transfer_capsule(0, 1000, 0); // 0 = unlimited
    let graph = make_transfer_graph(make_balance_id(), 100);

    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]);
    assert!(result.all_passed, "All nodes should pass rights validation; violations: {:?}", result.violations.iter().map(|v| format!("{:?}", v.breach_reason)).collect::<Vec<_>>());
}

#[test]
fn test_expired_capsule_fails() {
    let store = make_store_with_balance();
    let capsule = make_transfer_capsule(0, 5, 0); // expires at epoch 5
    let graph = make_transfer_graph(make_balance_id(), 100);

    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]); // epoch 50 > 5
    assert!(!result.all_passed);
    assert!(result.violations.iter().any(|v| v.breach_reason == ScopeBreachReason::CapsuleExpired));
}

#[test]
fn test_object_out_of_scope_fails() {
    let store = make_store_with_balance();

    // Capsule only allows Wallet type, but graph touches Balance
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Wallet);

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut capsule = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    capsule.capsule_hash = capsule.compute_hash();

    let graph = make_transfer_graph(make_balance_id(), 100);
    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]);
    assert!(!result.all_passed);
    assert!(result.violations.iter().any(|v| v.breach_reason == ScopeBreachReason::ObjectNotInScope));
}

#[test]
fn test_action_not_allowed_fails() {
    let store = make_store_with_balance();

    // Capsule only allows read; graph tries to debit
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::read_only(), // Only Read allowed
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut capsule = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    capsule.capsule_hash = capsule.compute_hash();

    let graph = make_transfer_graph(make_balance_id(), 100);
    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]);
    assert!(!result.all_passed);
    assert!(result.violations.iter().any(|v| v.breach_reason == ScopeBreachReason::ActionNotAllowed));
}

#[test]
fn test_spend_limit_exceeded_fails() {
    let store = make_store_with_balance();
    let capsule = make_transfer_capsule(0, 1000, 50); // max_spend = 50
    let graph = make_transfer_graph(make_balance_id(), 100); // tries to debit 100

    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]);
    assert!(!result.all_passed);
    assert!(result.violations.iter().any(|v| v.breach_reason == ScopeBreachReason::SpendLimitExceeded));
}

#[test]
fn test_node_count_exceeded_fails() {
    let store = make_store_with_balance();

    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);

    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(),
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 1, // Only 1 object allowed
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };

    let mut capsule = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    capsule.capsule_hash = capsule.compute_hash();

    // Graph touches 2 different Write objects
    let graph = make_transfer_graph(make_balance_id(), 100); // touches [10;32] and [11;32]

    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 50, &[3u8; 32]);
    // With max_objects_touched=1, should fail on second write object
    assert!(!result.all_passed);
    assert!(result.violations.iter().any(|v| v.breach_reason == ScopeBreachReason::ObjectCountExceeded));
}
