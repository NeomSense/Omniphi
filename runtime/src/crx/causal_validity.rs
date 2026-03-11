use crate::crx::causal_graph::{
    CausalGraph, NodeAccessType, NodeDependencyKind, NodeExecutionClass, NodeId,
};
use crate::crx::goal_packet::GoalPacket;
use crate::crx::rights_capsule::RightsCapsule;
use sha2::{Digest, Sha256};
use std::collections::BTreeSet;

// ─────────────────────────────────────────────────────────────────────────────
// Violation types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub struct MissingDependencyViolation {
    pub node_id: NodeId,
    pub missing_dep_kind: NodeDependencyKind,
    pub detail: String,
}

#[derive(Debug, Clone)]
pub struct UnauthorizedPathViolation {
    pub node_id: NodeId,
    pub detail: String,
}

#[derive(Debug, Clone)]
pub enum CausalViolation {
    MissingDependency(MissingDependencyViolation),
    UnauthorizedPath(UnauthorizedPathViolation),
    SkippedValidationNode { validation_node: NodeId },
    BranchBypass { gate_node: NodeId, bypassed_node: NodeId },
    HiddenObjectAccess { object_id: [u8; 32] },
    ForbiddenDomain { domain: String },
    CycleDetected,
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit and result types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub struct CausalAuditRecord {
    pub graph_id: [u8; 32],
    pub total_nodes_validated: usize,
    pub causal_violations: Vec<CausalViolation>,
    pub all_dependencies_satisfied: bool,
    pub no_unauthorized_paths: bool,
    pub no_skipped_validation: bool,
    /// SHA256 of bincode of this record's key fields.
    pub audit_hash: [u8; 32],
}

impl CausalAuditRecord {
    pub fn compute_audit_hash(
        graph_id: &[u8; 32],
        total_nodes: usize,
        violation_count: usize,
        all_dep: bool,
        no_unauth: bool,
        no_skip: bool,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(graph_id);
        hasher.update(&(total_nodes as u64).to_le_bytes());
        hasher.update(&(violation_count as u64).to_le_bytes());
        hasher.update(&[all_dep as u8, no_unauth as u8, no_skip as u8]);
        hasher.finalize().into()
    }
}

#[derive(Debug, Clone)]
pub struct CausalValidityResult {
    pub graph_id: [u8; 32],
    pub is_causally_valid: bool,
    pub violations: Vec<CausalViolation>,
    pub audit: CausalAuditRecord,
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine
// ─────────────────────────────────────────────────────────────────────────────

pub struct CausalValidityEngine;

impl CausalValidityEngine {
    /// Validate causal legitimacy of the graph.
    pub fn validate(
        graph: &CausalGraph,
        capsule: &RightsCapsule,
        goal: &GoalPacket,
    ) -> CausalValidityResult {
        let mut all_violations: Vec<CausalViolation> = Vec::new();

        // 1. Graph is acyclic
        if graph.validate_acyclic().is_err() {
            all_violations.push(CausalViolation::CycleDetected);
        }

        // 2. Check critical dependencies satisfied
        let dep_violations = Self::check_dependencies_satisfied(graph);
        all_violations.extend(dep_violations);

        // 3. Check no branch bypass
        let bypass_violations = Self::check_no_branch_bypass(graph);
        all_violations.extend(bypass_violations);

        // 4. Check no forbidden domain access
        let domain_violations = Self::check_no_forbidden_domain(graph, goal);
        all_violations.extend(domain_violations);

        // 5. Check validation nodes not skipped
        let skip_violations = Self::check_validation_nodes_not_skipped(graph);
        all_violations.extend(skip_violations);

        let all_dep_satisfied = !all_violations.iter().any(|v| {
            matches!(v, CausalViolation::MissingDependency(_))
        });
        let no_unauth = !all_violations.iter().any(|v| {
            matches!(v, CausalViolation::UnauthorizedPath(_) | CausalViolation::HiddenObjectAccess { .. })
        });
        let no_skip = !all_violations.iter().any(|v| {
            matches!(v, CausalViolation::SkippedValidationNode { .. })
        });
        let total_nodes = graph.nodes.len();
        let violation_count = all_violations.len();

        let audit_hash = CausalAuditRecord::compute_audit_hash(
            &graph.graph_id,
            total_nodes,
            violation_count,
            all_dep_satisfied,
            no_unauth,
            no_skip,
        );

        let audit = CausalAuditRecord {
            graph_id: graph.graph_id,
            total_nodes_validated: total_nodes,
            causal_violations: all_violations.clone(),
            all_dependencies_satisfied: all_dep_satisfied,
            no_unauthorized_paths: no_unauth,
            no_skipped_validation: no_skip,
            audit_hash,
        };

        let is_valid = all_violations.is_empty();

        CausalValidityResult {
            graph_id: graph.graph_id,
            is_causally_valid: is_valid,
            violations: all_violations,
            audit,
        }
    }

    /// Check that all critical edges have their dependencies satisfied in topological order.
    fn check_dependencies_satisfied(graph: &CausalGraph) -> Vec<CausalViolation> {
        let mut violations = Vec::new();

        // Build a position map: NodeId → index in topological_order
        let position: std::collections::BTreeMap<NodeId, usize> = graph
            .topological_order
            .iter()
            .enumerate()
            .map(|(i, id)| (id.clone(), i))
            .collect();

        for edge in &graph.edges {
            if !edge.is_critical {
                continue;
            }

            // from must appear BEFORE to in topological order
            let from_pos = position.get(&edge.from);
            let to_pos = position.get(&edge.to);

            match (from_pos, to_pos) {
                (Some(fp), Some(tp)) => {
                    if fp >= tp {
                        violations.push(CausalViolation::MissingDependency(
                            MissingDependencyViolation {
                                node_id: edge.to.clone(),
                                missing_dep_kind: edge.dependency_kind.clone(),
                                detail: format!(
                                    "node {:?} dependency on {:?} violated in topological order (from_pos={}, to_pos={})",
                                    edge.to, edge.from, fp, tp
                                ),
                            },
                        ));
                    }
                }
                _ => {
                    violations.push(CausalViolation::MissingDependency(
                        MissingDependencyViolation {
                            node_id: edge.to.clone(),
                            missing_dep_kind: edge.dependency_kind.clone(),
                            detail: format!(
                                "node {:?} or {:?} missing from topological order",
                                edge.from, edge.to
                            ),
                        },
                    ));
                }
            }

            // Write nodes must have a preceding Check/Policy node for critical deps
            if edge.dependency_kind == NodeDependencyKind::PolicyDependent {
                let from_node = graph.nodes.get(&edge.from);
                let to_node = graph.nodes.get(&edge.to);

                if let (Some(from), Some(to)) = (from_node, to_node) {
                    let from_is_check = matches!(
                        from.execution_class,
                        NodeExecutionClass::CheckBalance
                            | NodeExecutionClass::CheckPolicy
                            | NodeExecutionClass::ReadObject
                    );
                    let to_is_write = to.access_type == NodeAccessType::Write;
                    if to_is_write && !from_is_check {
                        violations.push(CausalViolation::UnauthorizedPath(
                            UnauthorizedPathViolation {
                                node_id: to.node_id.clone(),
                                detail: format!(
                                    "Write node {:?} has PolicyDependent edge from non-check node {:?}",
                                    to.node_id, from.node_id
                                ),
                            },
                        ));
                    }
                }
            }
        }

        violations
    }

    /// Check that no write node in branch B bypasses the BranchGate for B.
    fn check_no_branch_bypass(graph: &CausalGraph) -> Vec<CausalViolation> {
        let mut violations = Vec::new();

        // Find all BranchGate nodes
        let gate_nodes: Vec<&crate::crx::causal_graph::CausalNode> = graph
            .nodes
            .values()
            .filter(|n| n.execution_class == NodeExecutionClass::BranchGate)
            .collect();

        for gate in &gate_nodes {
            let gate_branch = gate.branch_id;

            // Find write nodes in the same branch
            let write_nodes_in_branch: Vec<NodeId> = graph
                .nodes
                .values()
                .filter(|n| {
                    n.branch_id == gate_branch
                        && n.access_type == NodeAccessType::Write
                        && n.node_id != gate.node_id
                })
                .map(|n| n.node_id.clone())
                .collect();

            let position: std::collections::BTreeMap<NodeId, usize> = graph
                .topological_order
                .iter()
                .enumerate()
                .map(|(i, id)| (id.clone(), i))
                .collect();

            let gate_pos = position.get(&gate.node_id).copied().unwrap_or(usize::MAX);

            for write_node_id in write_nodes_in_branch {
                let write_pos = position.get(&write_node_id).copied().unwrap_or(0);
                if write_pos < gate_pos {
                    violations.push(CausalViolation::BranchBypass {
                        gate_node: gate.node_id.clone(),
                        bypassed_node: write_node_id,
                    });
                }
            }
        }

        violations
    }

    /// Check that no node accesses a domain that is forbidden in goal constraints.
    fn check_no_forbidden_domain(graph: &CausalGraph, goal: &GoalPacket) -> Vec<CausalViolation> {
        let mut violations = Vec::new();

        for node in graph.nodes.values() {
            if let Some(domain) = &node.domain_tag {
                if goal.constraints.forbidden_domains.contains(domain) {
                    violations.push(CausalViolation::ForbiddenDomain {
                        domain: domain.clone(),
                    });
                }
            }
        }

        violations
    }

    /// Check that no mutation node appears before its required validation node in execution.
    /// Only DEBIT mutations require a preceding CheckBalance/CheckPolicy node.
    /// Credit mutations (credits, SwapPoolAmounts) inherit validation from the debit path.
    fn check_validation_nodes_not_skipped(graph: &CausalGraph) -> Vec<CausalViolation> {
        let mut violations = Vec::new();

        let position: std::collections::BTreeMap<NodeId, usize> = graph
            .topological_order
            .iter()
            .enumerate()
            .map(|(i, id)| (id.clone(), i))
            .collect();

        for node in graph.nodes.values() {
            if node.access_type != NodeAccessType::Write {
                continue;
            }

            // Only debit MutateBalance and SwapPoolAmounts require prior validation nodes
            let is_debit_mutate = node.execution_class == NodeExecutionClass::MutateBalance
                && node.metadata.get("debit_direction").map(|s| s.as_str()) == Some("debit");
            let is_swap = node.execution_class == NodeExecutionClass::SwapPoolAmounts;

            if !is_debit_mutate && !is_swap {
                // Credit, LockObject, UnlockObject — no validation node required
                continue;
            }

            // Find incoming PolicyDependent edges
            let policy_deps: Vec<&crate::crx::causal_graph::CausalEdge> = graph
                .dependencies_of(&node.node_id)
                .into_iter()
                .filter(|e| e.dependency_kind == NodeDependencyKind::PolicyDependent)
                .collect();

            if policy_deps.is_empty() {
                // Debit/swap node with no policy deps — check for validation node on same target
                let target = node.target_object.as_ref();
                let has_validation = graph.nodes.values().any(|n| {
                    matches!(
                        n.execution_class,
                        NodeExecutionClass::CheckBalance
                            | NodeExecutionClass::CheckPolicy
                            | NodeExecutionClass::ReadObject
                    ) && n.target_object.as_ref() == target
                        && position.get(&n.node_id).copied().unwrap_or(usize::MAX)
                            < position.get(&node.node_id).copied().unwrap_or(0)
                });

                if !has_validation {
                    violations.push(CausalViolation::SkippedValidationNode {
                        validation_node: node.node_id.clone(),
                    });
                }
            }
        }

        violations
    }
}
