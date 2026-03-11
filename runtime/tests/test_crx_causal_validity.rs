use omniphi_runtime::{
    crx::causal_graph::{
        CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
        NodeExecutionClass, NodeId,
    },
    crx::causal_validity::{CausalValidityEngine, CausalViolation},
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::rights_capsule::{
        AllowedActionSet, DomainAccessEnvelope, RightsCapsule, RightsScope,
    },
    ObjectType,
};
use std::collections::{BTreeMap, BTreeSet};

fn make_goal() -> GoalPacket {
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

fn make_capsule() -> RightsCapsule {
    let mut types: BTreeSet<ObjectType> = BTreeSet::new();
    types.insert(ObjectType::Balance);
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

fn make_valid_graph() -> CausalGraph {
    let mut nodes = BTreeMap::new();
    let mut debit_meta = BTreeMap::new();
    debit_meta.insert("debit_direction".to_string(), "debit".to_string());

    nodes.insert(NodeId(0), CausalNode {
        node_id: NodeId(0),
        label: "CheckBalance".to_string(),
        execution_class: NodeExecutionClass::CheckBalance,
        access_type: NodeAccessType::ValidationOnly,
        target_object: Some([10u8; 32]),
        amount: Some(100),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    });
    nodes.insert(NodeId(1), CausalNode {
        node_id: NodeId(1),
        label: "MutateBalance".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some([10u8; 32]),
        amount: Some(100),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: debit_meta,
    });
    nodes.insert(NodeId(2), CausalNode {
        node_id: NodeId(2),
        label: "FinalizeSettlement".to_string(),
        execution_class: NodeExecutionClass::FinalizeSettlement,
        access_type: NodeAccessType::SettlementOnly,
        target_object: None,
        amount: None,
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    });

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
            dependency_kind: NodeDependencyKind::OrderingOnly,
            is_critical: false,
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

#[test]
fn test_valid_graph_passes() {
    let graph = make_valid_graph();
    let capsule = make_capsule();
    let goal = make_goal();

    let result = CausalValidityEngine::validate(&graph, &capsule, &goal);
    assert!(
        result.is_causally_valid,
        "Valid graph should pass causal validation; violations: {:?}",
        result.violations.iter().map(|v| format!("{:?}", v)).collect::<Vec<_>>()
    );
}

#[test]
fn test_missing_critical_dependency_caught() {
    let mut graph = make_valid_graph();
    // Make the PolicyDependent edge reverse direction (violation of ordering)
    graph.edges = vec![
        CausalEdge {
            from: NodeId(1), // mutation before check — inverted
            to: NodeId(0),
            dependency_kind: NodeDependencyKind::PolicyDependent,
            is_critical: true,
        },
        CausalEdge {
            from: NodeId(0),
            to: NodeId(2),
            dependency_kind: NodeDependencyKind::OrderingOnly,
            is_critical: false,
        },
    ];
    // Recompute topological order (this now has 0→2, 1→0)
    graph.topological_order = CausalGraph::compute_topological_order(&graph.nodes, &graph.edges);
    graph.graph_hash = graph.compute_hash();

    let capsule = make_capsule();
    let goal = make_goal();

    let result = CausalValidityEngine::validate(&graph, &capsule, &goal);
    assert!(
        !result.is_causally_valid || result.violations.iter().any(|v| matches!(v, CausalViolation::MissingDependency(_) | CausalViolation::UnauthorizedPath(_))),
        "Inverted critical dependency should produce a violation"
    );
}

#[test]
fn test_forbidden_domain_caught() {
    let mut graph = make_valid_graph();
    // Tag one node with a forbidden domain
    if let Some(node) = graph.nodes.get_mut(&NodeId(1)) {
        node.domain_tag = Some("treasury".to_string());
    }
    graph.graph_hash = graph.compute_hash();

    let mut goal = make_goal();
    goal.constraints.forbidden_domains = vec!["treasury".to_string()];

    let capsule = make_capsule();
    let result = CausalValidityEngine::validate(&graph, &capsule, &goal);
    assert!(
        !result.is_causally_valid,
        "Graph with forbidden domain node should fail causal validation"
    );
    assert!(result.violations.iter().any(|v| matches!(v, CausalViolation::ForbiddenDomain { .. })));
}

#[test]
fn test_cycle_caught() {
    let mut nodes = BTreeMap::new();
    nodes.insert(NodeId(0), CausalNode {
        node_id: NodeId(0),
        label: "A".to_string(),
        execution_class: NodeExecutionClass::CheckBalance,
        access_type: NodeAccessType::ValidationOnly,
        target_object: None,
        amount: None,
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    });
    nodes.insert(NodeId(1), CausalNode {
        node_id: NodeId(1),
        label: "B".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: None,
        amount: None,
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    });

    // Cycle: 0 → 1 → 0
    let edges = vec![
        CausalEdge { from: NodeId(0), to: NodeId(1), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true },
        CausalEdge { from: NodeId(1), to: NodeId(0), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true },
    ];
    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let branch = ExecutionBranch {
        branch_id: 0,
        nodes: vec![],
        is_main: true,
        failure_allowed: false,
    };
    let mut graph = CausalGraph {
        graph_id: [9u8; 32],
        goal_packet_id: [9u8; 32],
        nodes,
        edges,
        branches: vec![branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    graph.graph_hash = graph.compute_hash();

    let capsule = make_capsule();
    let goal = make_goal();

    let result = CausalValidityEngine::validate(&graph, &capsule, &goal);
    assert!(!result.is_causally_valid);
    assert!(result.violations.iter().any(|v| matches!(v, CausalViolation::CycleDetected)));
}

#[test]
fn test_audit_record_generated() {
    let graph = make_valid_graph();
    let capsule = make_capsule();
    let goal = make_goal();

    let result = CausalValidityEngine::validate(&graph, &capsule, &goal);
    assert_eq!(result.audit.graph_id, graph.graph_id);
    assert_eq!(result.audit.total_nodes_validated, graph.nodes.len());
    assert_ne!(result.audit.audit_hash, [0u8; 32], "Audit hash must be non-zero");
}
