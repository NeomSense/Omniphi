use crate::crx::causal_graph::{
    CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
    NodeExecutionClass, NodeId,
};
use crate::crx::goal_packet::{GoalPacket, PartialFailurePolicy};
use crate::crx::rights_capsule::{
    AllowedActionSet, AllowedActionType, DomainAccessEnvelope, RightsCapsule, RightsScope,
};
use crate::errors::RuntimeError;
use crate::objects::base::ObjectType;
use crate::solver_market::market::{CandidatePlan, PlanActionType};
use crate::state::store::ObjectStore;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

// Shared constants for metadata keys/values.
const META_DEBIT_DIR: &str = "debit_direction";
const VAL_DEBIT: &str = "debit";
const VAL_CREDIT: &str = "credit";

// ─────────────────────────────────────────────────────────────────────────────
// DomainMapper
// ─────────────────────────────────────────────────────────────────────────────

pub struct DomainMapper;

impl DomainMapper {
    /// Returns a map of domain_tag → Vec<NodeId> for all tagged nodes.
    pub fn map(graph: &CausalGraph) -> BTreeMap<String, Vec<NodeId>> {
        let mut result: BTreeMap<String, Vec<NodeId>> = BTreeMap::new();
        for (id, node) in &graph.nodes {
            if let Some(domain) = &node.domain_tag {
                result.entry(domain.clone()).or_default().push(id.clone());
            }
        }
        result
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// GraphAssembler
// ─────────────────────────────────────────────────────────────────────────────

pub struct GraphAssembler;

impl GraphAssembler {
    /// Build CausalGraph from CandidatePlan actions.
    pub fn assemble(plan: &CandidatePlan, goal: &GoalPacket) -> CausalGraph {
        let mut nodes: BTreeMap<NodeId, CausalNode> = BTreeMap::new();
        let mut edges: Vec<CausalEdge> = Vec::new();
        let mut next_id: u32 = 0;

        // Track which nodes are "mutation" nodes for the final settlement dependency
        let mut last_mutation_node: Option<NodeId> = None;
        let mut all_mutation_nodes: Vec<NodeId> = Vec::new();

        // For each action, create the appropriate node(s)
        for action in &plan.actions {
            let target_bytes = *action.target_object.as_bytes();

            match &action.action_type {
                PlanActionType::DebitBalance => {
                    // CheckBalance node → MutateBalance node (debit)
                    let check_id = NodeId(next_id);
                    next_id += 1;
                    let mutate_id = NodeId(next_id);
                    next_id += 1;

                    let domain_tag = action.metadata.get("domain_tag").cloned();

                    let check_node = CausalNode {
                        node_id: check_id.clone(),
                        label: format!("CheckBalance(debit:{})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::CheckBalance,
                        access_type: NodeAccessType::ValidationOnly,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: domain_tag.clone(),
                        risk_tags: vec![],
                        metadata: BTreeMap::new(),
                    };

                    let mut mutate_meta = action.metadata.clone();
                    mutate_meta.insert(META_DEBIT_DIR.to_string(), VAL_DEBIT.to_string());
                    let mutate_node = CausalNode {
                        node_id: mutate_id.clone(),
                        label: format!("MutateBalance(debit:{})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::MutateBalance,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag,
                        risk_tags: vec![],
                        metadata: mutate_meta,
                    };

                    nodes.insert(check_id.clone(), check_node);
                    nodes.insert(mutate_id.clone(), mutate_node);

                    // CheckBalance must precede MutateBalance (PolicyDependent)
                    edges.push(CausalEdge {
                        from: check_id.clone(),
                        to: mutate_id.clone(),
                        dependency_kind: NodeDependencyKind::PolicyDependent,
                        is_critical: true,
                    });

                    // If there's a prior mutation node, create StateDependent edge
                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: check_id.clone(),
                            dependency_kind: NodeDependencyKind::OrderingOnly,
                            is_critical: false,
                        });
                    }

                    all_mutation_nodes.push(mutate_id.clone());
                    last_mutation_node = Some(mutate_id);
                }

                PlanActionType::CreditBalance => {
                    // MutateBalance (credit) StateDependent on last mutation (debit)
                    let mutate_id = NodeId(next_id);
                    next_id += 1;

                    let mut mutate_meta = action.metadata.clone();
                    mutate_meta.insert(META_DEBIT_DIR.to_string(), VAL_CREDIT.to_string());
                    let mutate_node = CausalNode {
                        node_id: mutate_id.clone(),
                        label: format!("MutateBalance(credit:{})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::MutateBalance,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: mutate_meta,
                    };

                    nodes.insert(mutate_id.clone(), mutate_node);

                    // CreditBalance is StateDependent on the last mutation (typically debit)
                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: mutate_id.clone(),
                            dependency_kind: NodeDependencyKind::StateDependent,
                            is_critical: true,
                        });
                    }

                    all_mutation_nodes.push(mutate_id.clone());
                    last_mutation_node = Some(mutate_id);
                }

                PlanActionType::SwapPoolAmounts => {
                    // CheckBalance (reserves) + SwapPoolAmounts + MutateBalance for each side
                    let check_id = NodeId(next_id);
                    next_id += 1;
                    let swap_id = NodeId(next_id);
                    next_id += 1;

                    let check_node = CausalNode {
                        node_id: check_id.clone(),
                        label: format!("CheckBalance(pool_reserves:{})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::CheckBalance,
                        access_type: NodeAccessType::ValidationOnly,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: BTreeMap::new(),
                    };

                    let swap_meta = action.metadata.clone();
                    // delta_a/delta_b should be set in metadata by caller
                    let swap_node = CausalNode {
                        node_id: swap_id.clone(),
                        label: format!("SwapPoolAmounts({})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::SwapPoolAmounts,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: swap_meta,
                    };

                    nodes.insert(check_id.clone(), check_node);
                    nodes.insert(swap_id.clone(), swap_node);

                    edges.push(CausalEdge {
                        from: check_id.clone(),
                        to: swap_id.clone(),
                        dependency_kind: NodeDependencyKind::StateDependent,
                        is_critical: true,
                    });

                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: check_id.clone(),
                            dependency_kind: NodeDependencyKind::OrderingOnly,
                            is_critical: false,
                        });
                    }

                    all_mutation_nodes.push(swap_id.clone());
                    last_mutation_node = Some(swap_id);
                }

                PlanActionType::LockBalance => {
                    let lock_id = NodeId(next_id);
                    next_id += 1;

                    let lock_node = CausalNode {
                        node_id: lock_id.clone(),
                        label: format!("LockObject({})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::LockObject,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: action.metadata.clone(),
                    };

                    nodes.insert(lock_id.clone(), lock_node);

                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: lock_id.clone(),
                            dependency_kind: NodeDependencyKind::OrderingOnly,
                            is_critical: false,
                        });
                    }

                    all_mutation_nodes.push(lock_id.clone());
                    last_mutation_node = Some(lock_id);
                }

                PlanActionType::UnlockBalance => {
                    let unlock_id = NodeId(next_id);
                    next_id += 1;

                    let unlock_node = CausalNode {
                        node_id: unlock_id.clone(),
                        label: format!("UnlockObject({})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::UnlockObject,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: action.metadata.clone(),
                    };

                    nodes.insert(unlock_id.clone(), unlock_node);

                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: unlock_id.clone(),
                            dependency_kind: NodeDependencyKind::OrderingOnly,
                            is_critical: false,
                        });
                    }

                    all_mutation_nodes.push(unlock_id.clone());
                    last_mutation_node = Some(unlock_id);
                }

                PlanActionType::Custom(name) if name == "update_version" => {
                    // UpdateVersion → FinalizeSettlement node
                    let finalize_id = NodeId(next_id);
                    next_id += 1;

                    let finalize_node = CausalNode {
                        node_id: finalize_id.clone(),
                        label: format!("FinalizeSettlement({})", hex::encode(&target_bytes[..4])),
                        execution_class: NodeExecutionClass::FinalizeSettlement,
                        access_type: NodeAccessType::SettlementOnly,
                        target_object: Some(target_bytes),
                        amount: None,
                        branch_id: 0,
                        domain_tag: None,
                        risk_tags: vec![],
                        metadata: BTreeMap::new(),
                    };

                    nodes.insert(finalize_id.clone(), finalize_node);

                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: finalize_id.clone(),
                            dependency_kind: NodeDependencyKind::OrderingOnly,
                            is_critical: false,
                        });
                    }

                    all_mutation_nodes.push(finalize_id.clone());
                    last_mutation_node = Some(finalize_id);
                }

                PlanActionType::Custom(method_name) => {
                    // Check if this is a contract action (has schema_id in metadata)
                    let is_contract_action = action.metadata.contains_key("schema_id");
                    let custom_id = NodeId(next_id);
                    next_id += 1;

                    let (exec_class, domain_tag, label) = if is_contract_action {
                        let schema_hex = action.metadata.get("schema_id").cloned().unwrap_or_default();
                        let mut sid = [0u8; 32];
                        if let Ok(bytes) = hex::decode(&schema_hex) {
                            if bytes.len() == 32 {
                                sid.copy_from_slice(&bytes);
                            }
                        }
                        (
                            NodeExecutionClass::ContractStateTransition {
                                schema_id: sid,
                                method: method_name.clone(),
                            },
                            Some(format!("contract.{}", &schema_hex[..8.min(schema_hex.len())])),
                            format!("Contract({}.{})", &schema_hex[..8.min(schema_hex.len())], method_name),
                        )
                    } else {
                        (
                            NodeExecutionClass::EmitReceipt,
                            None,
                            format!("Custom({})", hex::encode(&target_bytes[..4])),
                        )
                    };

                    let custom_node = CausalNode {
                        node_id: custom_id.clone(),
                        label,
                        execution_class: exec_class,
                        access_type: NodeAccessType::Write,
                        target_object: Some(target_bytes),
                        amount: action.amount,
                        branch_id: 0,
                        domain_tag,
                        risk_tags: vec![],
                        metadata: action.metadata.clone(),
                    };

                    nodes.insert(custom_id.clone(), custom_node);

                    if let Some(ref prev) = last_mutation_node {
                        edges.push(CausalEdge {
                            from: prev.clone(),
                            to: custom_id.clone(),
                            dependency_kind: NodeDependencyKind::StateDependent,
                            is_critical: is_contract_action,
                        });
                    }

                    all_mutation_nodes.push(custom_id.clone());
                    last_mutation_node = Some(custom_id);
                }
            }
        }

        // Always add a final FinalizeSettlement node (unless last node already is one)
        let needs_finalize = !nodes.values().any(|n| {
            n.execution_class == NodeExecutionClass::FinalizeSettlement
                && n.branch_id == 0
        });

        if needs_finalize && !all_mutation_nodes.is_empty() {
            let finalize_id = NodeId(next_id);

            let finalize_node = CausalNode {
                node_id: finalize_id.clone(),
                label: "FinalizeSettlement(root)".to_string(),
                execution_class: NodeExecutionClass::FinalizeSettlement,
                access_type: NodeAccessType::SettlementOnly,
                target_object: None,
                amount: None,
                branch_id: 0,
                domain_tag: None,
                risk_tags: vec![],
                metadata: BTreeMap::new(),
            };
            nodes.insert(finalize_id.clone(), finalize_node);

            // OrderingOnly dep on ALL mutation nodes
            for mut_node in &all_mutation_nodes {
                edges.push(CausalEdge {
                    from: mut_node.clone(),
                    to: finalize_id.clone(),
                    dependency_kind: NodeDependencyKind::OrderingOnly,
                    is_critical: false,
                });
            }
        }

        // Compute topological order
        let topological_order = CausalGraph::compute_topological_order(&nodes, &edges);

        // Build branch: single main branch (branch_id = 0) with all nodes
        let main_branch = ExecutionBranch {
            branch_id: 0,
            nodes: topological_order.clone(),
            is_main: true,
            failure_allowed: match &goal.policy.partial_failure_policy {
                PartialFailurePolicy::StrictAllOrNothing => false,
                _ => true,
            },
        };

        // Compute graph_id from plan_id + goal packet_id
        let mut hasher = Sha256::new();
        hasher.update(&plan.plan_id);
        hasher.update(&goal.packet_id);
        let graph_id: [u8; 32] = hasher.finalize().into();

        let mut graph = CausalGraph {
            graph_id,
            goal_packet_id: goal.packet_id,
            nodes,
            edges,
            branches: vec![main_branch],
            topological_order,
            graph_hash: [0u8; 32],
        };

        graph.graph_hash = graph.compute_hash();
        graph
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// RightsSynthesizer
// ─────────────────────────────────────────────────────────────────────────────

pub struct RightsSynthesizer;

impl RightsSynthesizer {
    /// Synthesize a RightsCapsule from a CausalGraph + GoalPacket.
    pub fn synthesize(
        graph: &CausalGraph,
        goal: &GoalPacket,
        solver_id: [u8; 32],
        epoch: u64,
    ) -> RightsCapsule {
        // Collect allowed_object_ids from Write nodes
        let mut allowed_object_ids: BTreeSet<[u8; 32]> = BTreeSet::new();
        let mut allowed_object_types: BTreeSet<ObjectType> = BTreeSet::new();
        let mut action_set: BTreeSet<AllowedActionType> = BTreeSet::new();
        let mut max_spend: u128 = 0;

        for node in graph.nodes.values() {
            // Include all objects touched by any node (read, write, settlement)
            // This ensures the capsule covers the full set of objects accessed
            if let Some(obj_id) = &node.target_object {
                if node.access_type == NodeAccessType::Write
                    || node.access_type == NodeAccessType::SettlementOnly
                    || node.access_type == NodeAccessType::ValidationOnly
                {
                    allowed_object_ids.insert(*obj_id);
                }
            }

            // Map node class to allowed action types
            match &node.execution_class {
                NodeExecutionClass::ReadObject | NodeExecutionClass::CheckBalance | NodeExecutionClass::CheckPolicy => {
                    action_set.insert(AllowedActionType::Read);
                }
                NodeExecutionClass::MutateBalance => {
                    let meta = &node.metadata;
                    if meta.get(META_DEBIT_DIR).map(|s| s.as_str()) == Some(VAL_DEBIT) {
                        action_set.insert(AllowedActionType::DebitBalance);
                        if let Some(amt) = node.amount {
                            max_spend = max_spend.saturating_add(amt);
                        }
                    } else {
                        action_set.insert(AllowedActionType::CreditBalance);
                    }
                }
                NodeExecutionClass::SwapPoolAmounts => {
                    action_set.insert(AllowedActionType::SwapPoolAmounts);
                    action_set.insert(AllowedActionType::DebitBalance);
                    action_set.insert(AllowedActionType::CreditBalance);
                }
                NodeExecutionClass::LockObject => {
                    action_set.insert(AllowedActionType::LockBalance);
                }
                NodeExecutionClass::UnlockObject => {
                    action_set.insert(AllowedActionType::UnlockBalance);
                }
                NodeExecutionClass::FinalizeSettlement => {
                    action_set.insert(AllowedActionType::UpdateVersion);
                    action_set.insert(AllowedActionType::EmitReceipt);
                }
                NodeExecutionClass::EmitReceipt => {
                    action_set.insert(AllowedActionType::EmitReceipt);
                }
                NodeExecutionClass::InvokeSafetyHook => {
                    action_set.insert(AllowedActionType::InvokeSafetyHook);
                }
                NodeExecutionClass::CreateObject => {
                    action_set.insert(AllowedActionType::CreateObject);
                }
                _ => {}
            }

            // Infer object types from allowed types in goal constraints
            for ot in &goal.constraints.allowed_object_types {
                allowed_object_types.insert(*ot);
            }
        }

        // Always include Read
        action_set.insert(AllowedActionType::Read);

        // Build domain envelope from goal constraints
        let mut allowed_domains: BTreeSet<String> = BTreeSet::new();
        let mut forbidden_domains: BTreeSet<String> = BTreeSet::new();
        for d in &goal.constraints.allowed_domains {
            allowed_domains.insert(d.clone());
        }
        for d in &goal.constraints.forbidden_domains {
            forbidden_domains.insert(d.clone());
        }

        let (allow_rollback, allow_downgrade, quarantine_eligible) =
            match &goal.policy.partial_failure_policy {
                PartialFailurePolicy::StrictAllOrNothing => (true, false, false),
                PartialFailurePolicy::AllowBranchDowngrade => (false, true, false),
                PartialFailurePolicy::QuarantineOnFailure => (false, false, true),
                PartialFailurePolicy::DowngradeAndContinue => (false, true, false),
            };

        let scope = RightsScope {
            allowed_object_ids,
            allowed_object_types,
            allowed_actions: AllowedActionSet(action_set),
            domain_envelope: DomainAccessEnvelope {
                allowed_domains,
                forbidden_domains,
            },
            max_spend,
            max_objects_touched: goal.constraints.max_objects_touched,
            allow_rollback,
            allow_downgrade,
            quarantine_eligible,
        };

        // Derive capsule_id from graph_id + solver_id
        let mut hasher = Sha256::new();
        hasher.update(&graph.graph_id);
        hasher.update(&solver_id);
        let capsule_id: [u8; 32] = hasher.finalize().into();

        let mut capsule = RightsCapsule {
            capsule_id,
            goal_packet_id: goal.packet_id,
            authorized_solver_id: solver_id,
            scope,
            valid_from_epoch: epoch,
            valid_until_epoch: goal.deadline_epoch,
            capsule_hash: [0u8; 32],
            metadata: BTreeMap::new(),
        };

        capsule.capsule_hash = capsule.compute_hash();
        capsule
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// CausalPlanBuilder
// ─────────────────────────────────────────────────────────────────────────────

pub struct CausalPlanBuilder;

impl CausalPlanBuilder {
    /// Convert a validated CandidatePlan + GoalPacket into a CausalGraph + RightsCapsule.
    /// This is deterministic: same inputs → same outputs.
    pub fn build(
        plan: &CandidatePlan,
        goal: &GoalPacket,
        _store: &ObjectStore,
        epoch: u64,
    ) -> Result<(CausalGraph, RightsCapsule), RuntimeError> {
        let graph = GraphAssembler::assemble(plan, goal);
        graph.validate_acyclic()?;

        let capsule = RightsSynthesizer::synthesize(&graph, goal, plan.solver_id, epoch);

        Ok((graph, capsule))
    }
}
