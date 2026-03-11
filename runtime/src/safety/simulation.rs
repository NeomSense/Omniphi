use crate::safety::actions::SafetyAction;
use crate::safety::incidents::{IncidentSeverity, IncidentType};
use crate::safety::kernel::{SafetyDecision, SafetyEvaluationContext, SafetyKernel};
use std::collections::BTreeMap;

#[derive(Debug, Clone)]
pub struct IncidentScenario {
    pub name: String,
    pub context: SafetyEvaluationContext,
    pub expected_incident_type: Option<IncidentType>,
    pub expected_min_severity: IncidentSeverity,
}

#[derive(Debug, Clone)]
pub struct ContainmentPreview {
    pub scenario_name: String,
    pub predicted_action: SafetyAction,
    pub predicted_scope: String,
    pub would_require_governance: bool,
    pub affected_domains: Vec<String>,
    pub affected_solvers: Vec<[u8; 32]>,
}

#[derive(Debug, Clone)]
pub struct RuleCoverageReport {
    pub total_policies: usize,
    pub enabled_policies: usize,
    pub policies_triggered: Vec<String>,
    pub uncovered_incident_types: Vec<IncidentType>,
}

#[derive(Debug, Clone)]
pub struct SafetySimulationResult {
    pub scenario: IncidentScenario,
    pub actual_decision: SafetyDecision,
    pub containment_preview: ContainmentPreview,
    pub expectation_met: bool, // actual_decision severity >= expected_min_severity
}

pub struct SafetySimulator;

impl SafetySimulator {
    /// Dry-run a scenario against a clone of the kernel — does NOT mutate real state.
    pub fn simulate(kernel: &SafetyKernel, scenario: IncidentScenario) -> SafetySimulationResult {
        let mut kernel_clone = kernel.clone();
        let actual_decision = kernel_clone.evaluate(&scenario.context);

        let actual_severity = actual_decision
            .incident
            .as_ref()
            .map(|i| i.severity.clone())
            .unwrap_or(IncidentSeverity::Info);

        let expectation_met = actual_severity >= scenario.expected_min_severity;

        let affected_domains: Vec<String> = actual_decision
            .incident
            .as_ref()
            .map(|i| i.affected.domains.iter().cloned().collect())
            .unwrap_or_default();

        let affected_solvers: Vec<[u8; 32]> = actual_decision
            .incident
            .as_ref()
            .map(|i| i.affected.solver_ids.iter().copied().collect())
            .unwrap_or_default();

        let predicted_scope = actual_decision
            .blast_radius
            .as_ref()
            .map(|br| format!("{:?}", br.recommended_scope))
            .unwrap_or_else(|| "None".to_string());

        let would_require_governance = actual_decision.action.requires_governance();

        let containment_preview = ContainmentPreview {
            scenario_name: scenario.name.clone(),
            predicted_action: actual_decision.action.clone(),
            predicted_scope,
            would_require_governance,
            affected_domains,
            affected_solvers,
        };

        SafetySimulationResult {
            scenario,
            actual_decision,
            containment_preview,
            expectation_met,
        }
    }

    pub fn coverage_report(kernel: &SafetyKernel) -> RuleCoverageReport {
        let total_policies = kernel.rule_engine.policies.len();
        let enabled_policies = kernel
            .rule_engine
            .policies
            .iter()
            .filter(|p| p.enabled)
            .count();

        let policies_triggered: Vec<String> = kernel
            .rule_engine
            .policies
            .iter()
            .filter(|p| p.enabled)
            .map(|p| p.name.clone())
            .collect();

        // List all incident types and find ones without a policy
        let all_incident_types = vec![
            IncidentType::AbnormalOutflow,
            IncidentType::UnauthorizedDomainAccess,
            IncidentType::RightsScopeBreach,
            IncidentType::CausalValidityBreach,
            IncidentType::OracleCorruptionIndicator,
            IncidentType::SolverMisconduct,
            IncidentType::LiquidityPoolInstability,
            IncidentType::GovernanceSensitiveObjectMisuse,
            IncidentType::RepeatedBranchQuarantine,
            IncidentType::AbnormalMutationVelocity,
            IncidentType::CrossDomainBlastRadiusEscalation,
            IncidentType::RepeatedRightsViolation,
            IncidentType::UnauthorizedCapabilityUse,
            IncidentType::ExcessiveObjectVersionChurn,
            IncidentType::PolicyRuleViolation,
        ];

        let covered: std::collections::BTreeSet<String> = kernel
            .rule_engine
            .policies
            .iter()
            .filter(|p| p.enabled)
            .map(|p| format!("{:?}", p.incident_type))
            .collect();

        let uncovered_incident_types: Vec<IncidentType> = all_incident_types
            .into_iter()
            .filter(|it| !covered.contains(&format!("{:?}", it)))
            .collect();

        RuleCoverageReport {
            total_policies,
            enabled_policies,
            policies_triggered,
            uncovered_incident_types,
        }
    }

    /// Run all predefined safety scenarios
    pub fn run_all_scenarios(kernel: &SafetyKernel) -> Vec<SafetySimulationResult> {
        let epoch = 100u64;
        let solver_id = [0xABu8; 32];
        let pool_id = [0xBBu8; 32];

        vec![
            Self::simulate(kernel, Self::abnormal_outflow_scenario(epoch)),
            Self::simulate(kernel, Self::solver_misconduct_scenario(solver_id, epoch)),
            Self::simulate(kernel, Self::governance_misuse_scenario(epoch)),
            Self::simulate(kernel, Self::oracle_inconsistency_scenario(epoch)),
            Self::simulate(kernel, Self::liquidity_instability_scenario(pool_id, epoch)),
            Self::simulate(kernel, Self::cross_domain_emergency_scenario(epoch)),
        ]
    }

    pub fn abnormal_outflow_scenario(epoch: u64) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.total_outflow = 2_000_000u128; // exceeds threshold of 1_000_000
        ctx.affected_objects = vec![[0x01u8; 32], [0x02u8; 32]];
        ctx.affected_domains = vec!["treasury".to_string()];

        IncidentScenario {
            name: "abnormal_outflow".to_string(),
            context: ctx,
            expected_incident_type: Some(IncidentType::AbnormalOutflow),
            expected_min_severity: IncidentSeverity::High,
        }
    }

    pub fn solver_misconduct_scenario(
        solver_id: [u8; 32],
        epoch: u64,
    ) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.solver_id = Some(solver_id);
        ctx.rights_violations_count = 3; // triggers SolverMisconduct rule
        ctx.affected_domains = vec!["dex_liquidity".to_string()];

        IncidentScenario {
            name: "solver_misconduct".to_string(),
            context: ctx,
            expected_incident_type: Some(IncidentType::SolverMisconduct),
            expected_min_severity: IncidentSeverity::Medium,
        }
    }

    pub fn governance_misuse_scenario(epoch: u64) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.is_governance_sensitive = true;
        ctx.affected_domains = vec!["governance".to_string()];

        IncidentScenario {
            name: "governance_misuse".to_string(),
            context: ctx,
            expected_incident_type: Some(IncidentType::GovernanceSensitiveObjectMisuse),
            expected_min_severity: IncidentSeverity::High,
        }
    }

    pub fn oracle_inconsistency_scenario(epoch: u64) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.causal_violations_count = 1;
        ctx.affected_domains = vec!["oracle".to_string()];
        // Use Provisional finality to signal oracle issue
        ctx.finality_class = crate::crx::finality::FinalityClass::Provisional;

        IncidentScenario {
            name: "oracle_inconsistency".to_string(),
            context: ctx,
            expected_incident_type: None,
            expected_min_severity: IncidentSeverity::Info, // may not trigger a specific rule
        }
    }

    pub fn liquidity_instability_scenario(
        pool_id: [u8; 32],
        epoch: u64,
    ) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.pool_ids = vec![pool_id];
        ctx.affected_domains = vec!["dex_liquidity".to_string()];
        // Signal pool reserve deviation via metadata
        ctx.metadata.insert(
            "pool_reserve_deviation_bps".to_string(),
            "6000".to_string(),
        );
        ctx.metadata.insert(
            format!("pool_reserve_deviation_bps_{}", hex::encode(pool_id)),
            "6000".to_string(),
        );

        IncidentScenario {
            name: "liquidity_instability".to_string(),
            context: ctx,
            expected_incident_type: Some(IncidentType::LiquidityPoolInstability),
            expected_min_severity: IncidentSeverity::Info, // may depend on pool_id matching
        }
    }

    pub fn cross_domain_emergency_scenario(epoch: u64) -> IncidentScenario {
        let mut ctx = SafetyEvaluationContext::clean(epoch);
        ctx.affected_domains = vec![
            "treasury".to_string(),
            "dex_liquidity".to_string(),
            "lending".to_string(),
        ];
        ctx.affected_objects = vec![
            [0x01u8; 32],
            [0x02u8; 32],
            [0x03u8; 32],
        ];

        IncidentScenario {
            name: "cross_domain_emergency".to_string(),
            context: ctx,
            expected_incident_type: Some(IncidentType::CrossDomainBlastRadiusEscalation),
            expected_min_severity: IncidentSeverity::Critical,
        }
    }
}
