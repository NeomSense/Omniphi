use crate::intents::base::IntentTransaction;
use crate::objects::base::ObjectId;
use crate::solver_market::market::CandidatePlan;
use std::collections::BTreeSet;

/// Core policy trait. Implement this to add custom constraints on candidate plans.
pub trait PlanPolicyEvaluator: Send + Sync {
    /// Returns `None` if the plan is allowed, or `Some(reason)` if rejected.
    fn evaluate(&self, plan: &CandidatePlan, intent: &IntentTransaction) -> Option<String>;
    fn name(&self) -> &str;
}

// ─────────────────────────────────────────────────────────────────────────────
// PermissivePolicy — accepts everything; useful in tests
// ─────────────────────────────────────────────────────────────────────────────

/// Default permissive policy. Accepts every plan unconditionally.
pub struct PermissivePolicy;

impl PlanPolicyEvaluator for PermissivePolicy {
    fn evaluate(&self, _plan: &CandidatePlan, _intent: &IntentTransaction) -> Option<String> {
        None
    }

    fn name(&self) -> &str {
        "PermissivePolicy"
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// MaxValuePolicy — rejects plans whose output exceeds a configured threshold
// ─────────────────────────────────────────────────────────────────────────────

/// Rejects plans whose `expected_output_amount` exceeds `max_value`.
pub struct MaxValuePolicy {
    pub max_value: u128,
}

impl PlanPolicyEvaluator for MaxValuePolicy {
    fn evaluate(&self, plan: &CandidatePlan, _intent: &IntentTransaction) -> Option<String> {
        if plan.expected_output_amount > self.max_value {
            Some(format!(
                "output {} exceeds max {}",
                plan.expected_output_amount, self.max_value
            ))
        } else {
            None
        }
    }

    fn name(&self) -> &str {
        "MaxValuePolicy"
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// ObjectBlocklistPolicy — blocks plans that write to specific object IDs
// ─────────────────────────────────────────────────────────────────────────────

/// Domain risk policy: blocks specific object IDs from being written.
pub struct ObjectBlocklistPolicy {
    pub blocked_objects: BTreeSet<ObjectId>,
}

impl PlanPolicyEvaluator for ObjectBlocklistPolicy {
    fn evaluate(&self, plan: &CandidatePlan, _intent: &IntentTransaction) -> Option<String> {
        for obj in &plan.object_writes {
            if self.blocked_objects.contains(obj) {
                return Some(format!("object {} is on the blocklist", obj));
            }
        }
        None
    }

    fn name(&self) -> &str {
        "ObjectBlocklistPolicy"
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// CompositePolicyEvaluator — all contained policies must pass
// ─────────────────────────────────────────────────────────────────────────────

/// Composite policy: all constituent policies must return `None` (pass).
pub struct CompositePolicyEvaluator {
    pub policies: Vec<Box<dyn PlanPolicyEvaluator>>,
}

impl PlanPolicyEvaluator for CompositePolicyEvaluator {
    fn evaluate(&self, plan: &CandidatePlan, intent: &IntentTransaction) -> Option<String> {
        for policy in &self.policies {
            if let Some(reason) = policy.evaluate(plan, intent) {
                return Some(format!("[{}] {}", policy.name(), reason));
            }
        }
        None
    }

    fn name(&self) -> &str {
        "CompositePolicyEvaluator"
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// SolverSafetyPolicy — blocks plans from specific solver IDs
// ─────────────────────────────────────────────────────────────────────────────

/// Solver safety policy: blocks plans submitted by specific solver IDs.
pub struct SolverSafetyPolicy {
    pub blocked_solver_ids: BTreeSet<[u8; 32]>,
}

impl PlanPolicyEvaluator for SolverSafetyPolicy {
    fn evaluate(&self, plan: &CandidatePlan, _intent: &IntentTransaction) -> Option<String> {
        if self.blocked_solver_ids.contains(&plan.solver_id) {
            Some("solver is blocked by safety policy".to_string())
        } else {
            None
        }
    }

    fn name(&self) -> &str {
        "SolverSafetyPolicy"
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// DomainRiskPolicy — quarantines objects and domains
// ─────────────────────────────────────────────────────────────────────────────

/// Domain risk policy: blocks plans that touch quarantined objects.
/// Future extension: pause entire object domains.
pub struct DomainRiskPolicy {
    /// Domain tags that are currently paused (future use).
    pub paused_domains: BTreeSet<String>,
    /// Individual objects that are quarantined and must not be written.
    pub quarantined_objects: BTreeSet<ObjectId>,
}

impl PlanPolicyEvaluator for DomainRiskPolicy {
    fn evaluate(&self, plan: &CandidatePlan, _intent: &IntentTransaction) -> Option<String> {
        for obj in &plan.object_writes {
            if self.quarantined_objects.contains(obj) {
                return Some(format!("object {:?} is quarantined", obj));
            }
        }
        None
    }

    fn name(&self) -> &str {
        "DomainRiskPolicy"
    }
}
