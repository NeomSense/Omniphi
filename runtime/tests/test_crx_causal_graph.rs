use omniphi_runtime::crx::causal_graph::{
    CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
    NodeExecutionClass, NodeId,
};
use std::collections::BTreeMap;

fn make_node(id: u32, class: NodeExecutionClass, access: NodeAccessType) -> CausalNode {
    CausalNode {
        node_id: NodeId(id),
        label: format!("node_{}", id),
        execution_class: class,
        access_type: access,
        target_object: None,
        amount: None,
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    }
}

fn make_simple_graph() -> CausalGraph {
    let mut nodes = BTreeMap::new();
    nodes.insert(
        NodeId(0),
        make_node(0, NodeExecutionClass::CheckBalance, NodeAccessType::ValidationOnly),
    );
    nodes.insert(
        NodeId(1),
        make_node(1, NodeExecutionClass::MutateBalance, NodeAccessType::Write),
    );
    nodes.insert(
        NodeId(2),
        make_node(2, NodeExecutionClass::FinalizeSettlement, NodeAccessType::SettlementOnly),
    );

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

    let main_branch = ExecutionBranch {
        branch_id: 0,
        nodes: topo.clone(),
        is_main: true,
        failure_allowed: false,
    };

    let mut graph = CausalGraph {
        graph_id: [1u8; 32],
        goal_packet_id: [2u8; 32],
        nodes,
        edges,
        branches: vec![main_branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    graph.graph_hash = graph.compute_hash();
    graph
}

#[test]
fn test_simple_graph_topology() {
    let graph = make_simple_graph();
    assert_eq!(graph.nodes.len(), 3);
    assert_eq!(graph.edges.len(), 2);
    assert_eq!(graph.topological_order.len(), 3);
}

#[test]
fn test_topological_order_deterministic() {
    let graph = make_simple_graph();
    let order1 = CausalGraph::compute_topological_order(&graph.nodes, &graph.edges);
    let order2 = CausalGraph::compute_topological_order(&graph.nodes, &graph.edges);
    assert_eq!(order1, order2, "Topological order must be deterministic");
}

#[test]
fn test_topological_order_correct() {
    let graph = make_simple_graph();
    let order = &graph.topological_order;
    // Node 0 must come before Node 1 (PolicyDependent edge)
    let pos_0 = order.iter().position(|n| n == &NodeId(0)).unwrap();
    let pos_1 = order.iter().position(|n| n == &NodeId(1)).unwrap();
    let pos_2 = order.iter().position(|n| n == &NodeId(2)).unwrap();
    assert!(pos_0 < pos_1, "CheckBalance must precede MutateBalance");
    assert!(pos_1 < pos_2, "MutateBalance must precede FinalizeSettlement");
}

#[test]
fn test_cycle_detection() {
    let mut nodes = BTreeMap::new();
    nodes.insert(NodeId(0), make_node(0, NodeExecutionClass::CheckBalance, NodeAccessType::ValidationOnly));
    nodes.insert(NodeId(1), make_node(1, NodeExecutionClass::MutateBalance, NodeAccessType::Write));

    // Introduce a cycle: 0 → 1 → 0
    let edges = vec![
        CausalEdge {
            from: NodeId(0),
            to: NodeId(1),
            dependency_kind: NodeDependencyKind::StateDependent,
            is_critical: true,
        },
        CausalEdge {
            from: NodeId(1),
            to: NodeId(0),
            dependency_kind: NodeDependencyKind::StateDependent,
            is_critical: true,
        },
    ];

    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let branch = ExecutionBranch {
        branch_id: 0,
        nodes: vec![],
        is_main: true,
        failure_allowed: false,
    };
    let mut graph = CausalGraph {
        graph_id: [3u8; 32],
        goal_packet_id: [4u8; 32],
        nodes,
        edges,
        branches: vec![branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    graph.graph_hash = graph.compute_hash();

    let result = graph.validate_acyclic();
    assert!(result.is_err(), "Cyclic graph should fail acyclicity check");
    assert!(result.unwrap_err().to_string().contains("cycle"));
}

#[test]
fn test_nodes_for_branch() {
    let mut nodes = BTreeMap::new();
    let mut n0 = make_node(0, NodeExecutionClass::CheckBalance, NodeAccessType::ValidationOnly);
    n0.branch_id = 0;
    let mut n1 = make_node(1, NodeExecutionClass::MutateBalance, NodeAccessType::Write);
    n1.branch_id = 1; // different branch
    let mut n2 = make_node(2, NodeExecutionClass::FinalizeSettlement, NodeAccessType::SettlementOnly);
    n2.branch_id = 0;

    nodes.insert(NodeId(0), n0);
    nodes.insert(NodeId(1), n1);
    nodes.insert(NodeId(2), n2);

    let edges: Vec<CausalEdge> = vec![];
    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let mut graph = CausalGraph {
        graph_id: [5u8; 32],
        goal_packet_id: [6u8; 32],
        nodes,
        edges,
        branches: vec![],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    graph.graph_hash = graph.compute_hash();

    let branch_0_nodes = graph.nodes_for_branch(0);
    assert_eq!(branch_0_nodes.len(), 2);
    let branch_1_nodes = graph.nodes_for_branch(1);
    assert_eq!(branch_1_nodes.len(), 1);
}

#[test]
fn test_graph_hash_deterministic() {
    let g = make_simple_graph();
    let h1 = g.compute_hash();
    let h2 = g.compute_hash();
    assert_eq!(h1, h2);
}

#[test]
fn test_graph_hash_changes_with_nodes() {
    let g1 = make_simple_graph();
    let mut g2 = make_simple_graph();
    // Add an extra node to g2
    g2.nodes.insert(
        NodeId(99),
        make_node(99, NodeExecutionClass::EmitReceipt, NodeAccessType::SettlementOnly),
    );
    g2.graph_hash = g2.compute_hash();
    assert_ne!(g1.compute_hash(), g2.compute_hash());
}

#[test]
fn test_dependencies_of_node() {
    let graph = make_simple_graph();
    // Node 1 has one incoming edge (from Node 0)
    let deps = graph.dependencies_of(&NodeId(1));
    assert_eq!(deps.len(), 1);
    assert_eq!(deps[0].from, NodeId(0));
    assert_eq!(deps[0].to, NodeId(1));

    // Node 0 has no incoming edges
    let deps_0 = graph.dependencies_of(&NodeId(0));
    assert_eq!(deps_0.len(), 0);
}

#[test]
fn test_acyclic_graph_passes() {
    let graph = make_simple_graph();
    assert!(graph.validate_acyclic().is_ok());
}
