use crate::safety::actions::{
    EmergencyModeAction, PauseAction, PauseTarget, QuarantineAction, RateLimitAction, SafetyAction,
};
use crate::safety::incidents::{IncidentSeverity, IncidentType};
use crate::safety::kernel::SafetyEvaluationContext;
use std::collections::BTreeMap;

#[derive(Debug, Clone)]
pub struct RuleThreshold {
    pub name: String,
    pub value: u128,
}

#[derive(Debug, Clone)]
pub enum RuleCondition {
    OutflowExceedsThreshold {
        threshold: u128,
    },
    MutationCountExceedsThreshold {
        domain: String,
        threshold: u64,
    },
    RepeatedRightsViolations {
        solver_id: [u8; 32],
        count: u64,
        window_epochs: u64,
    },
    GovernanceSensitiveObjectTouched {
        execution_class: String,
    },
    RepeatedQuarantinesInWindow {
        window_epochs: u64,
        threshold: u64,
    },
    BranchDowngradeFrequency {
        threshold_bps: u32,
    },
    PoolReserveDeviationExceeds {
        pool_id: [u8; 32],
        threshold_bps: u32,
    },
    ObjectVersionChurnExceeds {
        object_id: [u8; 32],
        threshold: u64,
    },
    SolverInvalidPlanRateExceeds {
        solver_id: [u8; 32],
        threshold_bps: u32,
    },
    DomainCrossBlastRadius {
        domain: String,
        affected_count: usize,
    },
}

#[derive(Debug, Clone)]
pub struct EscalationRule {
    pub from_severity: IncidentSeverity,
    pub condition_count_threshold: usize,
    pub to_severity: IncidentSeverity,
    pub escalated_action: SafetyAction,
}

#[derive(Debug, Clone)]
pub struct SafetyPolicy {
    pub name: String,
    pub incident_type: IncidentType,
    pub condition: RuleCondition,
    pub severity: IncidentSeverity,
    pub default_action: SafetyAction,
    pub escalation: Option<EscalationRule>,
    pub enabled: bool,
}

#[derive(Debug, Clone)]
pub struct SafetyRuleEvaluation {
    pub policy_name: String,
    pub triggered: bool,
    pub computed_severity: IncidentSeverity,
    pub action: SafetyAction,
    pub detail: String,
}

pub struct SafetyRuleEngine {
    pub policies: Vec<SafetyPolicy>,
    /// Track counts per (solver_id, incident_type_name) across epochs
    pub violation_counts: BTreeMap<([u8; 32], String), u64>,
    /// epoch_window -> count
    pub quarantine_counts_per_window: BTreeMap<u64, u64>,
}

impl SafetyRuleEngine {
    pub fn new(policies: Vec<SafetyPolicy>) -> Self {
        SafetyRuleEngine {
            policies,
            violation_counts: BTreeMap::new(),
            quarantine_counts_per_window: BTreeMap::new(),
        }
    }

    /// Add default production-grade policies
    pub fn with_defaults() -> Self {
        Self::new(default_policies())
    }

    /// Evaluate all enabled policies against the context.
    /// Returns triggered evaluations sorted by severity descending.
    pub fn evaluate(&mut self, ctx: &SafetyEvaluationContext) -> Vec<SafetyRuleEvaluation> {
        let policies: Vec<SafetyPolicy> = self.policies.clone();
        let mut results: Vec<SafetyRuleEvaluation> = Vec::new();

        for policy in &policies {
            if !policy.enabled {
                continue;
            }
            let eval = self.eval_policy(policy, ctx);
            if eval.triggered {
                results.push(eval);
            }
        }

        // Sort by severity descending (highest severity first)
        results.sort_by(|a, b| b.computed_severity.cmp(&a.computed_severity));
        results
    }

    fn eval_policy(
        &mut self,
        policy: &SafetyPolicy,
        ctx: &SafetyEvaluationContext,
    ) -> SafetyRuleEvaluation {
        let (triggered, detail) = self.check_condition(&policy.condition, ctx);

        let (computed_severity, action) = if triggered {
            // Check if escalation applies
            if let Some(esc) = &policy.escalation {
                let key = (
                    ctx.solver_id.unwrap_or([0u8; 32]),
                    policy.name.clone(),
                );
                let count = self.violation_counts.get(&key).copied().unwrap_or(0) + 1;
                // Update count
                self.violation_counts.insert(key, count);

                if count >= esc.condition_count_threshold as u64 {
                    (esc.to_severity.clone(), esc.escalated_action.clone())
                } else {
                    (policy.severity.clone(), policy.default_action.clone())
                }
            } else {
                if let Some(solver_id) = ctx.solver_id {
                    let key = (solver_id, policy.name.clone());
                    let count = self.violation_counts.get(&key).copied().unwrap_or(0) + 1;
                    self.violation_counts.insert(key, count);
                }
                (policy.severity.clone(), policy.default_action.clone())
            }
        } else {
            (policy.severity.clone(), SafetyAction::NoAction)
        };

        SafetyRuleEvaluation {
            policy_name: policy.name.clone(),
            triggered,
            computed_severity,
            action,
            detail,
        }
    }

    fn check_condition(&self, condition: &RuleCondition, ctx: &SafetyEvaluationContext) -> (bool, String) {
        match condition {
            RuleCondition::OutflowExceedsThreshold { threshold } => {
                let triggered = ctx.total_outflow > *threshold;
                let detail = format!(
                    "outflow={} threshold={}",
                    ctx.total_outflow, threshold
                );
                (triggered, detail)
            }

            RuleCondition::MutationCountExceedsThreshold { domain, threshold } => {
                let count = ctx
                    .domain_mutation_counts
                    .get(domain)
                    .copied()
                    .unwrap_or(0);
                let triggered = count > *threshold;
                let detail = format!(
                    "domain={} mutations={} threshold={}",
                    domain, count, threshold
                );
                (triggered, detail)
            }

            RuleCondition::RepeatedRightsViolations {
                solver_id,
                count,
                window_epochs: _,
            } => {
                let key = (*solver_id, "SolverMisconduct".to_string());
                let actual = self.violation_counts.get(&key).copied().unwrap_or(0);
                // Also consider the current context
                let effective = actual + ctx.rights_violations_count as u64;
                let triggered = effective >= *count;
                let detail = format!(
                    "solver={} violations={} threshold={}",
                    hex::encode(solver_id),
                    effective,
                    count
                );
                (triggered, detail)
            }

            RuleCondition::GovernanceSensitiveObjectTouched { execution_class: _ } => {
                let triggered = ctx.is_governance_sensitive;
                let detail = "governance-sensitive object touched".to_string();
                (triggered, detail)
            }

            RuleCondition::RepeatedQuarantinesInWindow {
                window_epochs: _,
                threshold,
            } => {
                let triggered = ctx.branch_quarantine_count as u64 > *threshold;
                let detail = format!(
                    "quarantine_count={} threshold={}",
                    ctx.branch_quarantine_count, threshold
                );
                (triggered, detail)
            }

            RuleCondition::BranchDowngradeFrequency { threshold_bps } => {
                let rate = (ctx.branch_downgrade_count as u64 * 10_000)
                    / ctx.mutation_count.max(1);
                let triggered = rate > *threshold_bps as u64;
                let detail = format!(
                    "downgrade_rate_bps={} threshold_bps={}",
                    rate, threshold_bps
                );
                (triggered, detail)
            }

            RuleCondition::PoolReserveDeviationExceeds {
                pool_id,
                threshold_bps,
            } => {
                // Check if any pool_ids in context matches and assume deviation if present
                let triggered = ctx.pool_ids.contains(pool_id)
                    && ctx
                        .metadata
                        .get("pool_reserve_deviation_bps")
                        .and_then(|v| v.parse::<u32>().ok())
                        .map(|v| v > *threshold_bps)
                        .unwrap_or(false);
                let detail = format!(
                    "pool={} threshold_bps={}",
                    hex::encode(pool_id),
                    threshold_bps
                );
                (triggered, detail)
            }

            RuleCondition::ObjectVersionChurnExceeds { object_id, threshold } => {
                let churn = ctx
                    .metadata
                    .get(&format!("version_churn_{}", hex::encode(object_id)))
                    .and_then(|v| v.parse::<u64>().ok())
                    .unwrap_or(0);
                let triggered = churn > *threshold;
                let detail = format!(
                    "object={} churn={} threshold={}",
                    hex::encode(object_id),
                    churn,
                    threshold
                );
                (triggered, detail)
            }

            RuleCondition::SolverInvalidPlanRateExceeds {
                solver_id,
                threshold_bps,
            } => {
                // Look up from violation_counts
                let key = (*solver_id, "invalid_plans".to_string());
                let invalid = self.violation_counts.get(&key).copied().unwrap_or(0);
                let total = invalid + 1;
                let rate = (invalid * 10_000) / total;
                let triggered = rate > *threshold_bps as u64;
                let detail = format!(
                    "solver={} invalid_rate_bps={} threshold_bps={}",
                    hex::encode(solver_id),
                    rate,
                    threshold_bps
                );
                (triggered, detail)
            }

            RuleCondition::DomainCrossBlastRadius {
                domain: _,
                affected_count,
            } => {
                let triggered = ctx.affected_domains.len() >= *affected_count;
                let detail = format!(
                    "affected_domains={} threshold={}",
                    ctx.affected_domains.len(),
                    affected_count
                );
                (triggered, detail)
            }
        }
    }
}

pub fn default_policies() -> Vec<SafetyPolicy> {
    vec![
        // 1. AbnormalOutflow: threshold 1_000_000 → Quarantine affected objects
        SafetyPolicy {
            name: "AbnormalOutflow".to_string(),
            incident_type: IncidentType::AbnormalOutflow,
            condition: RuleCondition::OutflowExceedsThreshold {
                threshold: 1_000_000u128,
            },
            severity: IncidentSeverity::High,
            default_action: SafetyAction::Quarantine(QuarantineAction {
                object_ids: vec![],
                reason: "abnormal outflow detected".to_string(),
                reversible: true,
                governance_required_to_lift: false,
            }),
            escalation: None,
            enabled: true,
        },

        // 2. SolverMisconduct: 3+ violations → SuspendSolver
        SafetyPolicy {
            name: "SolverMisconduct".to_string(),
            incident_type: IncidentType::SolverMisconduct,
            condition: RuleCondition::RepeatedRightsViolations {
                solver_id: [0u8; 32], // wildcard — evaluated per context
                count: 3,
                window_epochs: 10,
            },
            severity: IncidentSeverity::Medium,
            default_action: SafetyAction::SuspendSolver([0u8; 32], "repeated violations".to_string()),
            escalation: Some(EscalationRule {
                from_severity: IncidentSeverity::Medium,
                condition_count_threshold: 5,
                to_severity: IncidentSeverity::High,
                escalated_action: SafetyAction::SuspendSolver(
                    [0u8; 32],
                    "escalated repeated violations".to_string(),
                ),
            }),
            enabled: true,
        },

        // 3. RepeatedBranchQuarantine: 3+ in 5 epochs → RateLimit
        SafetyPolicy {
            name: "RepeatedBranchQuarantine".to_string(),
            incident_type: IncidentType::RepeatedBranchQuarantine,
            condition: RuleCondition::RepeatedQuarantinesInWindow {
                window_epochs: 5,
                threshold: 2,
            },
            severity: IncidentSeverity::Low,
            default_action: SafetyAction::RateLimit(RateLimitAction {
                target_domain: None,
                target_solver: None,
                max_mutations_per_epoch: 50,
                max_value_per_epoch: 500_000,
                reason: "repeated branch quarantine".to_string(),
            }),
            escalation: None,
            enabled: true,
        },

        // 4. GovernanceSensitiveObjectMisuse → Pause(Domain("governance"))
        SafetyPolicy {
            name: "GovernanceSensitiveObjectMisuse".to_string(),
            incident_type: IncidentType::GovernanceSensitiveObjectMisuse,
            condition: RuleCondition::GovernanceSensitiveObjectTouched {
                execution_class: "governance".to_string(),
            },
            severity: IncidentSeverity::High,
            default_action: SafetyAction::Pause(PauseAction {
                target: PauseTarget::Domain("governance".to_string()),
                reason: "governance-sensitive object misuse".to_string(),
                temporary_until_epoch: None,
                governance_required_to_lift: true,
            }),
            escalation: None,
            enabled: true,
        },

        // 5. LiquidityPoolInstability: reserve deviation > 5000 bps → Pause(LiquidityVenue)
        SafetyPolicy {
            name: "LiquidityPoolInstability".to_string(),
            incident_type: IncidentType::LiquidityPoolInstability,
            condition: RuleCondition::PoolReserveDeviationExceeds {
                pool_id: [0u8; 32], // wildcard
                threshold_bps: 5000,
            },
            severity: IncidentSeverity::High,
            default_action: SafetyAction::Pause(PauseAction {
                target: PauseTarget::LiquidityVenue([0u8; 32]),
                reason: "liquidity pool reserve deviation".to_string(),
                temporary_until_epoch: None,
                governance_required_to_lift: false,
            }),
            escalation: None,
            enabled: true,
        },

        // 6. AbnormalMutationVelocity: >100 mutations/epoch → RateLimit
        SafetyPolicy {
            name: "AbnormalMutationVelocity".to_string(),
            incident_type: IncidentType::AbnormalMutationVelocity,
            condition: RuleCondition::MutationCountExceedsThreshold {
                domain: "global".to_string(),
                threshold: 100,
            },
            severity: IncidentSeverity::Medium,
            default_action: SafetyAction::RateLimit(RateLimitAction {
                target_domain: Some("global".to_string()),
                target_solver: None,
                max_mutations_per_epoch: 100,
                max_value_per_epoch: 0,
                reason: "abnormal mutation velocity".to_string(),
            }),
            escalation: None,
            enabled: true,
        },

        // 7. CrossDomainBlastRadius: 3+ domains affected → EmergencyMode placeholder
        SafetyPolicy {
            name: "CrossDomainBlastRadius".to_string(),
            incident_type: IncidentType::CrossDomainBlastRadiusEscalation,
            condition: RuleCondition::DomainCrossBlastRadius {
                domain: "any".to_string(),
                affected_count: 3,
            },
            severity: IncidentSeverity::Critical,
            default_action: SafetyAction::EmergencyMode(EmergencyModeAction {
                activated: true,
                reason: "cross-domain blast radius escalation".to_string(),
                requires_governance_to_deactivate: true,
            }),
            escalation: None,
            enabled: true,
        },
    ]
}
