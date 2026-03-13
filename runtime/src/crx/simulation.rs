use crate::crx::branch_execution::ExecutionSettlementClass;
use crate::crx::causal_graph::{CausalGraph, NodeExecutionClass, NodeId};
use crate::crx::causal_validity::CausalValidityEngine;
use crate::crx::finality::{DomainFinalityPolicy, FinalityClass, FinalityClassifier};
use crate::crx::goal_packet::GoalPacket;
use crate::crx::plan_builder::{CausalPlanBuilder, DomainMapper};
use crate::crx::rights_capsule::{AllowedActionType, RightsCapsule};
use crate::crx::rights_validation::{RightsValidationEngine, RightsViolation};
use crate::crx::causal_validity::CausalViolation;
use crate::crx::branch_execution::BranchAwareExecutor;
use crate::crx::finality::SettlementDisposition;
use crate::gas::meter::GasCosts;
use crate::objects::base::ObjectId;
use crate::solver_market::market::CandidatePlan;
use crate::state::store::ObjectStore;
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Simulation result types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub struct CRXSimulationResult {
    pub goal_packet_id: [u8; 32],
    pub would_pass_rights_validation: bool,
    pub would_pass_causal_validation: bool,
    pub predicted_finality: FinalityClass,
    pub predicted_settlement_class: ExecutionSettlementClass,
    pub preview_graph_hash: [u8; 32],
    pub preview_capsule_hash: [u8; 32],
    pub preview_balance_changes: Vec<([u8; 32], i128)>,
    pub rights_violations: Vec<RightsViolation>,
    pub causal_violations: Vec<CausalViolation>,
    pub estimated_gas: u64,
}

#[derive(Debug, Clone)]
pub struct RightsInspectionView {
    pub capsule_id: [u8; 32],
    pub allowed_objects: Vec<[u8; 32]>,
    pub allowed_actions: Vec<AllowedActionType>,
    pub allowed_domains: Vec<String>,
    pub forbidden_domains: Vec<String>,
    pub max_spend: u128,
    pub valid_epochs: (u64, u64),
}

#[derive(Debug, Clone)]
pub struct GraphExplainabilityRecord {
    pub graph_id: [u8; 32],
    pub node_count: usize,
    pub edge_count: usize,
    pub branch_count: usize,
    pub critical_path: Vec<NodeId>,
    pub domain_map: BTreeMap<String, Vec<NodeId>>,
    pub node_summaries: Vec<(NodeId, String, NodeExecutionClass)>,
}

// ─────────────────────────────────────────────────────────────────────────────
// Simulator
// ─────────────────────────────────────────────────────────────────────────────

pub struct CRXSimulator;

impl CRXSimulator {
    /// Simulate the full CRX pipeline without mutating the store.
    pub fn simulate(
        plan: &CandidatePlan,
        goal: &GoalPacket,
        store: &ObjectStore,
        domain_policies: &[DomainFinalityPolicy],
        epoch: u64,
    ) -> CRXSimulationResult {
        // We need a mutable clone of the store to run the execution
        // Since ObjectStore doesn't implement Clone, we rebuild it from objects
        // by running the execution pipeline on a temporary copy.
        // Strategy: build graph + capsule, run validation, simulate execution on a clone.

        // Build graph + capsule (read-only, no store mutation)
        let build_result = CausalPlanBuilder::build(plan, goal, store, epoch);
        let (graph, capsule) = match build_result {
            Ok(pair) => pair,
            Err(_) => {
                return CRXSimulationResult {
                    goal_packet_id: goal.packet_id,
                    would_pass_rights_validation: false,
                    would_pass_causal_validation: false,
                    predicted_finality: FinalityClass::Rejected,
                    predicted_settlement_class: ExecutionSettlementClass::Rejected,
                    preview_graph_hash: [0u8; 32],
                    preview_capsule_hash: [0u8; 32],
                    preview_balance_changes: vec![],
                    rights_violations: vec![],
                    causal_violations: vec![],
                    estimated_gas: 0,
                };
            }
        };

        let rights_result =
            RightsValidationEngine::validate(&graph, &capsule, store, epoch, &plan.solver_id);
        let causal_result = CausalValidityEngine::validate(&graph, &capsule, goal);

        let rights_valid = rights_result.all_passed;
        let causal_valid = causal_result.is_causally_valid;

        // Simulate balance changes by replaying the graph on a mock store
        let preview_balance_changes = Self::preview_balance_changes(&graph, store);

        // Predict settlement class
        let predicted_settlement_class = if !rights_valid || !causal_valid {
            ExecutionSettlementClass::Rejected
        } else {
            ExecutionSettlementClass::FullSuccess
        };

        // Classify finality
        let finality_disp = FinalityClassifier::classify(
            &predicted_settlement_class,
            goal,
            domain_policies,
        );

        let costs = GasCosts::default_costs();
        let estimated_gas = costs.base_tx
            + (graph.nodes.len() as u64) * (costs.object_write + costs.object_read);

        CRXSimulationResult {
            goal_packet_id: goal.packet_id,
            would_pass_rights_validation: rights_valid,
            would_pass_causal_validation: causal_valid,
            predicted_finality: finality_disp.finality_class,
            predicted_settlement_class,
            preview_graph_hash: graph.graph_hash,
            preview_capsule_hash: capsule.capsule_hash,
            preview_balance_changes,
            rights_violations: rights_result.violations,
            causal_violations: causal_result.violations,
            estimated_gas,
        }
    }

    /// Inspect a rights capsule.
    pub fn inspect_rights(capsule: &RightsCapsule) -> RightsInspectionView {
        RightsInspectionView {
            capsule_id: capsule.capsule_id,
            allowed_objects: capsule
                .scope
                .allowed_object_ids
                .iter()
                .copied()
                .collect(),
            allowed_actions: capsule
                .scope
                .allowed_actions
                .0
                .iter()
                .cloned()
                .collect(),
            allowed_domains: capsule
                .scope
                .domain_envelope
                .allowed_domains
                .iter()
                .cloned()
                .collect(),
            forbidden_domains: capsule
                .scope
                .domain_envelope
                .forbidden_domains
                .iter()
                .cloned()
                .collect(),
            max_spend: capsule.scope.max_spend,
            valid_epochs: (capsule.valid_from_epoch, capsule.valid_until_epoch),
        }
    }

    /// Build an explainability record for a graph.
    pub fn explain_graph(graph: &CausalGraph) -> GraphExplainabilityRecord {
        let domain_map = DomainMapper::map(graph);
        let node_summaries: Vec<(NodeId, String, NodeExecutionClass)> = graph
            .topological_order
            .iter()
            .filter_map(|id| graph.nodes.get(id))
            .map(|n| (n.node_id.clone(), n.label.clone(), n.execution_class.clone()))
            .collect();

        let critical_path = Self::find_critical_path(graph);

        GraphExplainabilityRecord {
            graph_id: graph.graph_id,
            node_count: graph.nodes.len(),
            edge_count: graph.edges.len(),
            branch_count: graph.branches.len(),
            critical_path,
            domain_map,
            node_summaries,
        }
    }

    /// Find the longest path in the topological order (critical path).
    fn find_critical_path(graph: &CausalGraph) -> Vec<NodeId> {
        if graph.topological_order.is_empty() {
            return vec![];
        }

        // Dynamic programming approach: longest path in a DAG
        let mut dist: BTreeMap<NodeId, usize> = graph
            .topological_order
            .iter()
            .map(|id| (id.clone(), 0))
            .collect();

        let mut parent: BTreeMap<NodeId, Option<NodeId>> = graph
            .topological_order
            .iter()
            .map(|id| (id.clone(), None))
            .collect();

        for node_id in &graph.topological_order {
            let current_dist = *dist.get(node_id).unwrap_or(&0);
            // For each outgoing edge from node_id
            for edge in graph.successors_of(node_id) {
                let next_dist = current_dist + 1;
                let entry = dist.entry(edge.to.clone()).or_insert(0);
                if next_dist > *entry {
                    *entry = next_dist;
                    *parent.entry(edge.to.clone()).or_insert(None) = Some(node_id.clone());
                }
            }
        }

        // Find the end node with maximum distance
        let end_node = dist
            .iter()
            .max_by_key(|(_, &d)| d)
            .map(|(id, _)| id.clone());

        // Reconstruct path backwards
        let mut path = Vec::new();
        let mut current = end_node;
        while let Some(node_id) = current {
            path.push(node_id.clone());
            current = parent.get(&node_id).and_then(|p| p.clone());
        }
        path.reverse();
        path
    }

    /// Preview balance changes from a graph (without mutating store).
    fn preview_balance_changes(
        graph: &CausalGraph,
        store: &ObjectStore,
    ) -> Vec<([u8; 32], i128)> {
        let mut changes: BTreeMap<[u8; 32], i128> = BTreeMap::new();

        for node_id in &graph.topological_order {
            let node = match graph.nodes.get(node_id) {
                Some(n) => n,
                None => continue,
            };

            if node.execution_class != NodeExecutionClass::MutateBalance {
                continue;
            }

            let obj_bytes = match &node.target_object {
                Some(b) => b,
                None => continue,
            };

            let amount = node.amount.unwrap_or(0) as i128;
            let is_debit =
                node.metadata.get("debit_direction").map(|s| s.as_str()) == Some("debit");

            let delta = if is_debit { -amount } else { amount };
            *changes.entry(*obj_bytes).or_insert(0) += delta;
        }

        changes.into_iter().collect()
    }
}
