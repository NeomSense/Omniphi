use crate::crx::causal_graph::{CausalGraph, CausalNode, NodeExecutionClass, NodeId};
use crate::crx::goal_packet::{GoalPacket, PartialFailurePolicy};
use crate::crx::rights_capsule::RightsCapsule;
use crate::errors::RuntimeError;
use crate::objects::base::ObjectId;
use crate::state::store::ObjectStore;

// ─────────────────────────────────────────────────────────────────────────────
// Branch execution types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BranchFailureMode {
    Revert,
    Downgrade,
    Quarantine,
    Ignore,
}

#[derive(Debug, Clone)]
pub struct DowngradeAction {
    pub branch_id: u32,
    pub reduced_amount: u128,
    pub skipped_nodes: Vec<NodeId>,
    pub reason: String,
}

#[derive(Debug, Clone)]
pub struct QuarantineAction {
    pub branch_id: u32,
    pub quarantined_objects: Vec<[u8; 32]>,
    pub reason: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExecutionSettlementClass {
    FullSuccess,
    /// branch_id that was downgraded
    SuccessWithDowngrade(u32),
    /// quarantined object ids
    SuccessWithQuarantine(Vec<[u8; 32]>),
    PartialSuccess {
        succeeded_branches: Vec<u32>,
        failed_branches: Vec<u32>,
    },
    FullRevert,
    Rejected,
}

#[derive(Debug, Clone)]
pub struct BranchExecutionResult {
    pub branch_id: u32,
    pub success: bool,
    pub failure_mode: Option<BranchFailureMode>,
    pub downgrade: Option<DowngradeAction>,
    pub quarantine: Option<QuarantineAction>,
    pub mutated_objects: Vec<[u8; 32]>,
    pub error: Option<String>,
}

// ─────────────────────────────────────────────────────────────────────────────
// Executor
// ─────────────────────────────────────────────────────────────────────────────

pub struct BranchAwareExecutor;

impl BranchAwareExecutor {
    /// Execute the CausalGraph against the ObjectStore, respecting branch policies.
    pub fn execute(
        graph: &CausalGraph,
        capsule: &RightsCapsule,
        goal: &GoalPacket,
        store: &mut ObjectStore,
    ) -> (ExecutionSettlementClass, Vec<BranchExecutionResult>) {
        let mut branch_results: Vec<BranchExecutionResult> = Vec::new();
        let mut all_succeeded: Vec<u32> = Vec::new();
        let mut all_failed: Vec<u32> = Vec::new();
        let mut all_quarantined: Vec<[u8; 32]> = Vec::new();
        let mut any_downgrade: Option<u32> = None;

        for branch in &graph.branches {
            let branch_id = branch.branch_id;

            // Collect nodes for this branch in topological order
            let branch_node_ids: Vec<NodeId> = graph
                .topological_order
                .iter()
                .filter(|id| {
                    graph
                        .nodes
                        .get(id)
                        .map(|n| n.branch_id == branch_id)
                        .unwrap_or(false)
                })
                .cloned()
                .collect();

            let mut mutated_objects: Vec<[u8; 32]> = Vec::new();
            let mut branch_failed = false;
            let mut failure_error: Option<String> = None;
            let mut failed_node: Option<NodeId> = None;
            let mut skipped_nodes: Vec<NodeId> = Vec::new();

            for node_id in &branch_node_ids {
                let node = match graph.nodes.get(node_id) {
                    Some(n) => n,
                    None => continue,
                };

                match Self::execute_node(node, store) {
                    Ok(()) => {
                        // Track mutated objects
                        if let Some(obj) = &node.target_object {
                            match &node.execution_class {
                                NodeExecutionClass::MutateBalance
                                | NodeExecutionClass::SwapPoolAmounts
                                | NodeExecutionClass::LockObject
                                | NodeExecutionClass::UnlockObject
                                | NodeExecutionClass::FinalizeSettlement => {
                                    if !mutated_objects.contains(obj) {
                                        mutated_objects.push(*obj);
                                    }
                                }
                                _ => {}
                            }
                        }
                    }
                    Err(e) => {
                        branch_failed = true;
                        failure_error = Some(e.to_string());
                        failed_node = Some(node_id.clone());

                        // Collect remaining nodes as skipped
                        let pos = branch_node_ids.iter().position(|id| id == node_id).unwrap_or(0);
                        skipped_nodes = branch_node_ids[pos + 1..].to_vec();
                        break;
                    }
                }
            }

            if !branch_failed {
                all_succeeded.push(branch_id);
                branch_results.push(BranchExecutionResult {
                    branch_id,
                    success: true,
                    failure_mode: None,
                    downgrade: None,
                    quarantine: None,
                    mutated_objects,
                    error: None,
                });
            } else {
                // Apply failure mode based on policy
                let failure_mode = Self::determine_failure_mode(
                    &goal.policy.partial_failure_policy,
                    capsule,
                    branch.failure_allowed,
                );

                match &failure_mode {
                    BranchFailureMode::Revert => {
                        all_failed.push(branch_id);
                        branch_results.push(BranchExecutionResult {
                            branch_id,
                            success: false,
                            failure_mode: Some(BranchFailureMode::Revert),
                            downgrade: None,
                            quarantine: None,
                            mutated_objects,
                            error: failure_error.clone(),
                        });

                        // For StrictAllOrNothing: return immediately with FullRevert
                        if goal.policy.partial_failure_policy == PartialFailurePolicy::StrictAllOrNothing {
                            return (ExecutionSettlementClass::FullRevert, branch_results);
                        }
                    }
                    BranchFailureMode::Downgrade => {
                        any_downgrade = Some(branch_id);
                        all_succeeded.push(branch_id);

                        let downgrade = DowngradeAction {
                            branch_id,
                            reduced_amount: 0, // partial execution
                            skipped_nodes: skipped_nodes.clone(),
                            reason: failure_error.clone().unwrap_or_default(),
                        };

                        branch_results.push(BranchExecutionResult {
                            branch_id,
                            success: true,
                            failure_mode: Some(BranchFailureMode::Downgrade),
                            downgrade: Some(downgrade),
                            quarantine: None,
                            mutated_objects,
                            error: failure_error,
                        });
                    }
                    BranchFailureMode::Quarantine => {
                        all_failed.push(branch_id);

                        // Quarantine all objects that were about to be mutated
                        let quarantined: Vec<[u8; 32]> = failed_node
                            .as_ref()
                            .and_then(|nid| graph.nodes.get(nid))
                            .and_then(|n| n.target_object)
                            .into_iter()
                            .collect();

                        all_quarantined.extend_from_slice(&quarantined);

                        let qa = QuarantineAction {
                            branch_id,
                            quarantined_objects: quarantined,
                            reason: failure_error.clone().unwrap_or_default(),
                        };

                        branch_results.push(BranchExecutionResult {
                            branch_id,
                            success: false,
                            failure_mode: Some(BranchFailureMode::Quarantine),
                            downgrade: None,
                            quarantine: Some(qa),
                            mutated_objects,
                            error: failure_error,
                        });
                    }
                    BranchFailureMode::Ignore => {
                        all_failed.push(branch_id);
                        branch_results.push(BranchExecutionResult {
                            branch_id,
                            success: false,
                            failure_mode: Some(BranchFailureMode::Ignore),
                            downgrade: None,
                            quarantine: None,
                            mutated_objects,
                            error: failure_error,
                        });
                    }
                }
            }
        }

        // Determine overall settlement class
        let settlement_class = if all_failed.is_empty() {
            if let Some(branch_id) = any_downgrade {
                ExecutionSettlementClass::SuccessWithDowngrade(branch_id)
            } else if !all_quarantined.is_empty() {
                ExecutionSettlementClass::SuccessWithQuarantine(all_quarantined)
            } else {
                ExecutionSettlementClass::FullSuccess
            }
        } else if all_succeeded.is_empty() {
            ExecutionSettlementClass::FullRevert
        } else {
            // Some succeeded, some failed
            match &goal.policy.partial_failure_policy {
                PartialFailurePolicy::QuarantineOnFailure => {
                    ExecutionSettlementClass::SuccessWithQuarantine(all_quarantined)
                }
                PartialFailurePolicy::AllowBranchDowngrade
                | PartialFailurePolicy::DowngradeAndContinue => {
                    ExecutionSettlementClass::PartialSuccess {
                        succeeded_branches: all_succeeded,
                        failed_branches: all_failed,
                    }
                }
                PartialFailurePolicy::StrictAllOrNothing => {
                    ExecutionSettlementClass::FullRevert
                }
            }
        };

        (settlement_class, branch_results)
    }

    fn determine_failure_mode(
        policy: &PartialFailurePolicy,
        capsule: &RightsCapsule,
        failure_allowed: bool,
    ) -> BranchFailureMode {
        match policy {
            PartialFailurePolicy::StrictAllOrNothing => BranchFailureMode::Revert,
            PartialFailurePolicy::AllowBranchDowngrade => {
                if capsule.scope.allow_downgrade {
                    BranchFailureMode::Downgrade
                } else {
                    BranchFailureMode::Revert
                }
            }
            PartialFailurePolicy::QuarantineOnFailure => {
                if capsule.scope.quarantine_eligible {
                    BranchFailureMode::Quarantine
                } else {
                    BranchFailureMode::Revert
                }
            }
            PartialFailurePolicy::DowngradeAndContinue => {
                if capsule.scope.allow_downgrade {
                    BranchFailureMode::Downgrade
                } else if failure_allowed {
                    BranchFailureMode::Ignore
                } else {
                    BranchFailureMode::Revert
                }
            }
        }
    }

    /// Execute a single node against the store.
    pub fn execute_node(node: &CausalNode, store: &mut ObjectStore) -> Result<(), RuntimeError> {
        let obj_id = node
            .target_object
            .map(ObjectId::from);

        match &node.execution_class {
            NodeExecutionClass::ReadObject => {
                if let Some(id) = &obj_id {
                    store.get(id).ok_or_else(|| RuntimeError::ObjectNotFound(*id))?;
                }
                Ok(())
            }

            NodeExecutionClass::CheckBalance | NodeExecutionClass::CheckPolicy => {
                if let Some(id) = &obj_id {
                    let min_amount = node.amount.unwrap_or(0);
                    let bal = store
                        .get_balance_by_id(id)
                        .ok_or_else(|| RuntimeError::ObjectNotFound(*id))?;
                    if bal.available() < min_amount {
                        return Err(RuntimeError::InsufficientBalance {
                            required: min_amount,
                            available: bal.available(),
                        });
                    }
                }
                Ok(())
            }

            NodeExecutionClass::MutateBalance => {
                let id = obj_id.ok_or_else(|| {
                    RuntimeError::CausalViolation("MutateBalance node has no target_object".into())
                })?;
                let amount = node.amount.unwrap_or(0);
                let is_debit = node.metadata.get("debit_direction").map(|s| s.as_str()) == Some("debit");

                let bal = store
                    .get_balance_by_id_mut(&id)
                    .ok_or_else(|| RuntimeError::ObjectNotFound(id))?;

                if is_debit {
                    // Check that available balance (amount - locked) covers debit
                    let available = bal.amount.saturating_sub(bal.locked_amount);
                    if available < amount {
                        return Err(RuntimeError::InsufficientBalance {
                            required: amount,
                            available,
                        });
                    }
                    bal.amount = bal.amount.checked_sub(amount).ok_or_else(|| {
                        RuntimeError::InsufficientBalance {
                            required: amount,
                            available: bal.amount,
                        }
                    })?;
                } else {
                    bal.amount = bal.amount.saturating_add(amount);
                }

                Ok(())
            }

            NodeExecutionClass::SwapPoolAmounts => {
                let id = obj_id.ok_or_else(|| {
                    RuntimeError::CausalViolation("SwapPoolAmounts node has no target_object".into())
                })?;

                let delta_a: i128 = node
                    .metadata
                    .get("delta_a")
                    .and_then(|v| v.parse().ok())
                    .unwrap_or(0);
                let delta_b: i128 = node
                    .metadata
                    .get("delta_b")
                    .and_then(|v| v.parse().ok())
                    .unwrap_or(0);

                let pool = store
                    .get_pool_by_id_mut(&id)
                    .ok_or_else(|| RuntimeError::ObjectNotFound(id))?;

                // Apply delta_a
                if delta_a >= 0 {
                    pool.reserve_a = pool.reserve_a.saturating_add(delta_a as u128);
                } else {
                    let sub = (-delta_a) as u128;
                    if pool.reserve_a < sub {
                        return Err(RuntimeError::InsufficientBalance {
                            required: sub,
                            available: pool.reserve_a,
                        });
                    }
                    pool.reserve_a -= sub;
                }

                // Apply delta_b
                if delta_b >= 0 {
                    pool.reserve_b = pool.reserve_b.saturating_add(delta_b as u128);
                } else {
                    let sub = (-delta_b) as u128;
                    if pool.reserve_b < sub {
                        return Err(RuntimeError::InsufficientBalance {
                            required: sub,
                            available: pool.reserve_b,
                        });
                    }
                    pool.reserve_b -= sub;
                }

                Ok(())
            }

            NodeExecutionClass::LockObject => {
                let id = obj_id.ok_or_else(|| {
                    RuntimeError::CausalViolation("LockObject node has no target_object".into())
                })?;
                let amount = node.amount.unwrap_or(0);

                let bal = store
                    .get_balance_by_id_mut(&id)
                    .ok_or_else(|| RuntimeError::ObjectNotFound(id))?;

                // Check sufficient available balance before locking
                let available = bal.amount.saturating_sub(bal.locked_amount);
                if available < amount {
                    return Err(RuntimeError::InsufficientBalance {
                        required: amount,
                        available,
                    });
                }
                bal.locked_amount = bal.locked_amount.saturating_add(amount);
                Ok(())
            }

            NodeExecutionClass::UnlockObject => {
                let id = obj_id.ok_or_else(|| {
                    RuntimeError::CausalViolation("UnlockObject node has no target_object".into())
                })?;
                let amount = node.amount.unwrap_or(0);

                let bal = store
                    .get_balance_by_id_mut(&id)
                    .ok_or_else(|| RuntimeError::ObjectNotFound(id))?;

                bal.locked_amount = bal.locked_amount.saturating_sub(amount);
                Ok(())
            }

            NodeExecutionClass::FinalizeSettlement => {
                // Increment version on the target object (if specified) and sync
                if let Some(id) = &obj_id {
                    if let Some(bal) = store.get_balance_by_id_mut(id) {
                        bal.meta.version += 1;
                    } else if let Some(pool) = store.get_pool_by_id_mut(id) {
                        pool.meta.version += 1;
                    } else if let Some(obj) = store.get_mut(id) {
                        obj.meta_mut().version += 1;
                    }
                }
                Ok(())
            }

            // Contract state transitions are applied as opaque byte updates.
            // The constraint validator has already approved the transition during
            // plan validation. Here we just apply the proposed state.
            NodeExecutionClass::ContractStateTransition { .. } => {
                // Actual state application is handled by the settlement engine
                // after the CRX graph completes. This node marks the dependency.
                Ok(())
            }

            // No-op extension points
            NodeExecutionClass::BranchGate
            | NodeExecutionClass::EmitReceipt
            | NodeExecutionClass::InvokeSafetyHook
            | NodeExecutionClass::CreateObject
            | NodeExecutionClass::ReserveLiquidity => Ok(()),
        }
    }
}
