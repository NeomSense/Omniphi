use crate::errors::RuntimeError;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

// ─────────────────────────────────────────────────────────────────────────────
// Risk Tags
// ─────────────────────────────────────────────────────────────────────────────

#[derive(
    Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash,
    serde::Serialize, serde::Deserialize,
)]
pub enum ObjectRiskTag {
    TreasuryCritical,
    GovernanceSensitive,
    OracleDependent,
    UserRecoverable,
    QuarantineEligible,
    BridgeAdjacent,
    LiquiditySystemic,
}

#[derive(Debug, Clone)]
pub struct ExecutionSensitivityProfile {
    pub object_risk_tags: BTreeMap<[u8; 32], Vec<ObjectRiskTag>>,
    pub domain_risk_tags: BTreeMap<String, Vec<ObjectRiskTag>>,
    pub highest_risk_tier: Option<ObjectRiskTag>,
}

// ─────────────────────────────────────────────────────────────────────────────
// Graph node identifiers and classification
// ─────────────────────────────────────────────────────────────────────────────

#[derive(
    Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash,
    serde::Serialize, serde::Deserialize,
)]
pub struct NodeId(pub u32);

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum NodeExecutionClass {
    ReadObject,
    CheckPolicy,
    CheckBalance,
    ReserveLiquidity,
    MutateBalance,
    SwapPoolAmounts,
    LockObject,
    UnlockObject,
    CreateObject,
    EmitReceipt,
    InvokeSafetyHook,
    FinalizeSettlement,
    /// Conditional node: downstream executes only if condition met.
    BranchGate,
    /// Intent Contract state transition. The constraint validator must approve
    /// the proposed state change before this node can execute.
    ContractStateTransition {
        schema_id: [u8; 32],
        method: String,
    },
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum NodeAccessType {
    Read,
    Write,
    Conditional,
    ValidationOnly,
    SettlementOnly,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CausalNode {
    pub node_id: NodeId,
    pub label: String,
    pub execution_class: NodeExecutionClass,
    pub access_type: NodeAccessType,
    pub target_object: Option<[u8; 32]>,
    pub amount: Option<u128>,
    /// 0 = main branch
    pub branch_id: u32,
    pub domain_tag: Option<String>,
    pub risk_tags: Vec<ObjectRiskTag>,
    pub metadata: BTreeMap<String, String>,
}

// ─────────────────────────────────────────────────────────────────────────────
// Edge types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(
    Debug, Clone, PartialEq, Eq, PartialOrd, Ord,
    serde::Serialize, serde::Deserialize,
)]
pub enum NodeDependencyKind {
    /// B can only run if A's state mutation is committed.
    StateDependent,
    /// B requires A's policy check to pass.
    PolicyDependent,
    /// B requires A's oracle read result.
    OracleDependent,
    /// B requires A to prove authorization.
    RightsDependent,
    /// B and A share a domain boundary.
    DomainDependent,
    /// B must come after A in execution order (no data dep).
    OrderingOnly,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CausalEdge {
    pub from: NodeId,
    pub to: NodeId,
    pub dependency_kind: NodeDependencyKind,
    /// If true, failure of this edge's dependency causes the entire branch to fail.
    pub is_critical: bool,
}

// ─────────────────────────────────────────────────────────────────────────────
// Branches
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ExecutionBranch {
    pub branch_id: u32,
    pub nodes: Vec<NodeId>,
    pub is_main: bool,
    /// Derived from PartialFailurePolicy.
    pub failure_allowed: bool,
}

// ─────────────────────────────────────────────────────────────────────────────
// CausalGraph
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CausalGraph {
    pub graph_id: [u8; 32],
    pub goal_packet_id: [u8; 32],
    pub nodes: BTreeMap<NodeId, CausalNode>,
    pub edges: Vec<CausalEdge>,
    pub branches: Vec<ExecutionBranch>,
    /// Pre-computed, deterministic topological order.
    pub topological_order: Vec<NodeId>,
    pub graph_hash: [u8; 32],
}

impl CausalGraph {
    /// Deterministic topological sort using Kahn's algorithm with BTreeSet.
    /// For stable ordering, uses BTreeSet for the zero-in-degree queue.
    pub fn compute_topological_order(
        nodes: &BTreeMap<NodeId, CausalNode>,
        edges: &[CausalEdge],
    ) -> Vec<NodeId> {
        // Build in-degree map and adjacency list
        let mut in_degree: BTreeMap<NodeId, usize> = nodes.keys().map(|id| (id.clone(), 0)).collect();
        let mut adjacency: BTreeMap<NodeId, Vec<NodeId>> = nodes.keys().map(|id| (id.clone(), vec![])).collect();

        for edge in edges {
            *in_degree.entry(edge.to.clone()).or_insert(0) += 1;
            adjacency.entry(edge.from.clone()).or_default().push(edge.to.clone());
        }

        // Sort adjacency lists for determinism
        for v in adjacency.values_mut() {
            v.sort();
        }

        // BTreeSet ensures sorted (smallest NodeId first) processing
        let mut queue: BTreeSet<NodeId> = in_degree
            .iter()
            .filter(|(_, &deg)| deg == 0)
            .map(|(id, _)| id.clone())
            .collect();

        let mut result = Vec::with_capacity(nodes.len());

        while let Some(node) = queue.iter().next().cloned() {
            queue.remove(&node);
            result.push(node.clone());

            if let Some(neighbors) = adjacency.get(&node) {
                for neighbor in neighbors {
                    let deg = in_degree.get_mut(neighbor).unwrap();
                    *deg -= 1;
                    if *deg == 0 {
                        queue.insert(neighbor.clone());
                    }
                }
            }
        }

        result
    }

    pub fn compute_hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&self.graph_id);
        hasher.update(&self.goal_packet_id);

        // Hash nodes in deterministic order
        for (id, node) in &self.nodes {
            hasher.update(&id.0.to_le_bytes());
            let node_bytes = bincode::serialize(node)
                .expect("CausalNode bincode serialization is infallible");
            let len = (node_bytes.len() as u64).to_le_bytes();
            hasher.update(len);
            hasher.update(&node_bytes);
        }

        // Hash edges in order
        for edge in &self.edges {
            hasher.update(&edge.from.0.to_le_bytes());
            hasher.update(&edge.to.0.to_le_bytes());
        }

        hasher.finalize().into()
    }

    /// Returns Err(CausalViolation) if a cycle is detected.
    pub fn validate_acyclic(&self) -> Result<(), RuntimeError> {
        let order = Self::compute_topological_order(&self.nodes, &self.edges);
        if order.len() != self.nodes.len() {
            return Err(RuntimeError::CausalViolation(
                "cycle detected in causal graph".into(),
            ));
        }
        Ok(())
    }

    pub fn nodes_for_branch(&self, branch_id: u32) -> Vec<&CausalNode> {
        self.nodes
            .values()
            .filter(|n| n.branch_id == branch_id)
            .collect()
    }

    /// Returns edges where edge.to == node_id (incoming dependencies).
    pub fn dependencies_of(&self, node_id: &NodeId) -> Vec<&CausalEdge> {
        self.edges.iter().filter(|e| &e.to == node_id).collect()
    }

    /// Returns edges where edge.from == node_id (outgoing dependencies).
    pub fn successors_of(&self, node_id: &NodeId) -> Vec<&CausalEdge> {
        self.edges.iter().filter(|e| &e.from == node_id).collect()
    }
}
