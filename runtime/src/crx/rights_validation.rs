use crate::crx::causal_graph::{CausalGraph, CausalNode, NodeAccessType, NodeExecutionClass, NodeId};
use crate::crx::rights_capsule::{AllowedActionType, RightsCapsule};
use crate::objects::base::ObjectId;
use crate::state::store::ObjectStore;

// Shared constants for metadata keys/values.
const META_DEBIT_DIR: &str = "debit_direction";
const VAL_DEBIT: &str = "debit";
 
// ─────────────────────────────────────────────────────────────────────────────
// Violation types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ScopeBreachReason {
    ObjectNotInScope,
    ActionNotAllowed,
    DomainForbidden,
    SpendLimitExceeded,
    CapsuleExpired,
    SolverMismatch,
    ObjectCountExceeded,
}

#[derive(Debug, Clone)]
pub struct RightsViolation {
    pub node_id: NodeId,
    pub breach_reason: ScopeBreachReason,
    pub detail: String,
}

#[derive(Debug, Clone)]
pub struct NodeRightsCheck {
    pub node_id: NodeId,
    pub passed: bool,
    pub violation: Option<RightsViolation>,
}

#[derive(Debug, Clone)]
pub struct RightsValidationResult {
    pub capsule_id: [u8; 32],
    pub all_passed: bool,
    pub node_checks: Vec<NodeRightsCheck>,
    pub violations: Vec<RightsViolation>,
    pub total_spend: u128,
    pub objects_touched: usize,
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine
// ─────────────────────────────────────────────────────────────────────────────

pub struct RightsValidationEngine;

impl RightsValidationEngine {
    /// Validate every node in the CausalGraph against the RightsCapsule.
    ///
    /// `requesting_solver_id` must match `capsule.authorized_solver_id`; if it
    /// doesn't, every node fails with `SolverMismatch` (FIND-005).
    pub fn validate(
        graph: &CausalGraph,
        capsule: &RightsCapsule,
        store: &ObjectStore,
        epoch: u64,
        requesting_solver_id: &[u8; 32],
    ) -> RightsValidationResult {
        // Check solver identity before anything else
        if capsule.authorized_solver_id != *requesting_solver_id {
            let mut node_checks = Vec::new();
            let mut violations = Vec::new();
            for node_id in &graph.topological_order {
                let v = RightsViolation {
                    node_id: node_id.clone(),
                    breach_reason: ScopeBreachReason::SolverMismatch,
                    detail: format!(
                        "capsule authorized solver {}, got {}",
                        hex::encode(&capsule.authorized_solver_id),
                        hex::encode(requesting_solver_id)
                    ),
                };
                violations.push(v.clone());
                node_checks.push(NodeRightsCheck {
                    node_id: node_id.clone(),
                    passed: false,
                    violation: Some(v),
                });
            }
            return RightsValidationResult {
                capsule_id: capsule.capsule_id,
                all_passed: false,
                node_checks,
                violations,
                total_spend: 0,
                objects_touched: 0,
            };
        }

        // Check capsule epoch validity
        if !capsule.is_valid_at_epoch(epoch) {
            // All nodes fail with CapsuleExpired
            let mut node_checks = Vec::new();
            let mut violations = Vec::new();
            for node_id in &graph.topological_order {
                let v = RightsViolation {
                    node_id: node_id.clone(),
                    breach_reason: ScopeBreachReason::CapsuleExpired,
                    detail: format!(
                        "capsule valid [{}, {}], current epoch {}",
                        capsule.valid_from_epoch, capsule.valid_until_epoch, epoch
                    ),
                };
                violations.push(v.clone());
                node_checks.push(NodeRightsCheck {
                    node_id: node_id.clone(),
                    passed: false,
                    violation: Some(v),
                });
            }
            return RightsValidationResult {
                capsule_id: capsule.capsule_id,
                all_passed: false,
                node_checks,
                violations,
                total_spend: 0,
                objects_touched: 0,
            };
        }

        let mut node_checks: Vec<NodeRightsCheck> = Vec::new();
        let mut violations: Vec<RightsViolation> = Vec::new();
        let mut cumulative_spend: u128 = 0;
        let mut objects_touched: std::collections::BTreeSet<[u8; 32]> =
            std::collections::BTreeSet::new();

        for node_id in &graph.topological_order {
            let node = match graph.nodes.get(node_id) {
                Some(n) => n,
                None => continue,
            };

            let check = Self::check_node(node, capsule, store, cumulative_spend);

            if check.passed {
                // Update cumulative spend for debit nodes
                if node.execution_class == NodeExecutionClass::MutateBalance {
                    let meta = &node.metadata;
                    if meta.get(META_DEBIT_DIR).map(|s| s.as_str()) == Some(VAL_DEBIT) {
                        if let Some(amt) = node.amount {
                            cumulative_spend = cumulative_spend.saturating_add(amt);
                        }
                    }
                }
                // Track objects touched by Write nodes
                if node.access_type == NodeAccessType::Write {
                    if let Some(obj) = &node.target_object {
                        objects_touched.insert(*obj);
                    }
                }
            } else if let Some(ref v) = check.violation {
                violations.push(v.clone());
            }

            // Check object count limit after updating
            let count_exceeded = capsule.scope.max_objects_touched > 0
                && objects_touched.len() > capsule.scope.max_objects_touched;
            if count_exceeded && check.passed {
                let v = RightsViolation {
                    node_id: node_id.clone(),
                    breach_reason: ScopeBreachReason::ObjectCountExceeded,
                    detail: format!(
                        "objects touched {} exceeds max {}",
                        objects_touched.len(),
                        capsule.scope.max_objects_touched
                    ),
                };
                violations.push(v.clone());
                node_checks.push(NodeRightsCheck {
                    node_id: node_id.clone(),
                    passed: false,
                    violation: Some(v),
                });
                continue;
            }

            node_checks.push(check);
        }

        let all_passed = violations.is_empty();

        RightsValidationResult {
            capsule_id: capsule.capsule_id,
            all_passed,
            node_checks,
            violations,
            total_spend: cumulative_spend,
            objects_touched: objects_touched.len(),
        }
    }

    fn check_node(
        node: &CausalNode,
        capsule: &RightsCapsule,
        store: &ObjectStore,
        cumulative_spend: u128,
    ) -> NodeRightsCheck {
        // Determine required action type for this node
        let required_action = node_to_action_type(&node.execution_class, node);

        // Check action is allowed
        if let Some(action) = &required_action {
            if !capsule.can_perform_action(action) {
                return NodeRightsCheck {
                    node_id: node.node_id.clone(),
                    passed: false,
                    violation: Some(RightsViolation {
                        node_id: node.node_id.clone(),
                        breach_reason: ScopeBreachReason::ActionNotAllowed,
                        detail: format!("action {:?} not in capsule scope", action),
                    }),
                };
            }
        }

        // Check object scope (for nodes with a target_object)
        if let Some(obj_bytes) = &node.target_object {
            let obj_id = ObjectId::from(*obj_bytes);

            // Get object type from store if available, otherwise use goal constraint types
            let obj_type = store
                .get(&obj_id)
                .map(|o| o.object_type());

            if let Some(ot) = obj_type {
                if !capsule.can_access_object(obj_bytes, &ot) {
                    return NodeRightsCheck {
                        node_id: node.node_id.clone(),
                        passed: false,
                        violation: Some(RightsViolation {
                            node_id: node.node_id.clone(),
                            breach_reason: ScopeBreachReason::ObjectNotInScope,
                            detail: format!(
                                "object {} (type {:?}) not in capsule scope",
                                hex::encode(obj_bytes),
                                ot
                            ),
                        }),
                    };
                }
            } else if !capsule.scope.allowed_object_ids.is_empty()
                && !capsule.scope.allowed_object_ids.contains(obj_bytes)
            {
                // Object not in store and not in explicit whitelist — reject
                return NodeRightsCheck {
                    node_id: node.node_id.clone(),
                    passed: false,
                    violation: Some(RightsViolation {
                        node_id: node.node_id.clone(),
                        breach_reason: ScopeBreachReason::ObjectNotInScope,
                        detail: format!(
                            "object {} not in capsule allowed_object_ids",
                            hex::encode(obj_bytes)
                        ),
                    }),
                };
            }
        }

        // Check domain
        if let Some(domain) = &node.domain_tag {
            if !capsule.scope.domain_envelope.is_domain_allowed(domain) {
                return NodeRightsCheck {
                    node_id: node.node_id.clone(),
                    passed: false,
                    violation: Some(RightsViolation {
                        node_id: node.node_id.clone(),
                        breach_reason: ScopeBreachReason::DomainForbidden,
                        detail: format!("domain '{}' is forbidden in capsule scope", domain),
                    }),
                };
            }
        }

        // Check spend limit for debit nodes
        if node.execution_class == NodeExecutionClass::MutateBalance {
            let is_debit = node.metadata.get(META_DEBIT_DIR).map(|s| s.as_str()) == Some(VAL_DEBIT);
            if is_debit {
                if let Some(amt) = node.amount {
                    let new_spend = cumulative_spend.saturating_add(amt);
                    if capsule.scope.max_spend > 0 && new_spend > capsule.scope.max_spend {
                        return NodeRightsCheck {
                            node_id: node.node_id.clone(),
                            passed: false,
                            violation: Some(RightsViolation {
                                node_id: node.node_id.clone(),
                                breach_reason: ScopeBreachReason::SpendLimitExceeded,
                                detail: format!(
                                    "cumulative spend {} would exceed max_spend {}",
                                    new_spend, capsule.scope.max_spend
                                ),
                            }),
                        };
                    }
                }
            }
        }

        NodeRightsCheck {
            node_id: node.node_id.clone(),
            passed: true,
            violation: None,
        }
    }
}

fn node_to_action_type(
    class: &NodeExecutionClass,
    node: &CausalNode,
) -> Option<AllowedActionType> {
    match class {
        NodeExecutionClass::ReadObject
        | NodeExecutionClass::CheckBalance
        | NodeExecutionClass::CheckPolicy
        | NodeExecutionClass::ReserveLiquidity => Some(AllowedActionType::Read),
        NodeExecutionClass::MutateBalance => {
            let is_debit = node.metadata.get(META_DEBIT_DIR).map(|s| s.as_str()) == Some(VAL_DEBIT);
            if is_debit {
                Some(AllowedActionType::DebitBalance)
            } else {
                Some(AllowedActionType::CreditBalance)
            }
        }
        NodeExecutionClass::SwapPoolAmounts => Some(AllowedActionType::SwapPoolAmounts),
        NodeExecutionClass::LockObject => Some(AllowedActionType::LockBalance),
        NodeExecutionClass::UnlockObject => Some(AllowedActionType::UnlockBalance),
        NodeExecutionClass::CreateObject => Some(AllowedActionType::CreateObject),
        NodeExecutionClass::FinalizeSettlement => Some(AllowedActionType::UpdateVersion),
        NodeExecutionClass::EmitReceipt => Some(AllowedActionType::EmitReceipt),
        NodeExecutionClass::InvokeSafetyHook => Some(AllowedActionType::InvokeSafetyHook),
        NodeExecutionClass::BranchGate => None,
        // Contract state transitions map to WriteObject in the rights capsule.
        NodeExecutionClass::ContractStateTransition { .. } => Some(AllowedActionType::UpdateVersion),
    }
}
