use omniphi_runtime::safety::incidents::{IncidentSeverity, IncidentType};
use omniphi_runtime::safety::kernel::SafetyEvaluationContext;
use omniphi_runtime::safety::policies::{SafetyRuleEngine, default_policies};
use std::collections::BTreeMap;

fn clean_ctx(epoch: u64) -> SafetyEvaluationContext {
    SafetyEvaluationContext::clean(epoch)
}

#[test]
fn test_default_policies_load() {
    let engine = SafetyRuleEngine::with_defaults();
    assert!(!engine.policies.is_empty(), "default policies should be non-empty");
    assert!(engine.policies.iter().any(|p| p.enabled), "at least one policy should be enabled");
    let names: Vec<&str> = engine.policies.iter().map(|p| p.name.as_str()).collect();
    assert!(names.contains(&"AbnormalOutflow"));
    assert!(names.contains(&"SolverMisconduct"));
    assert!(names.contains(&"CrossDomainBlastRadius"));
}

#[test]
fn test_outflow_rule_triggers_above_threshold() {
    let mut engine = SafetyRuleEngine::with_defaults();
    let mut ctx = clean_ctx(1);
    ctx.total_outflow = 2_000_000u128; // above 1_000_000 threshold

    let results = engine.evaluate(&ctx);
    let outflow_triggered = results.iter().any(|r| r.policy_name == "AbnormalOutflow" && r.triggered);
    assert!(outflow_triggered, "AbnormalOutflow should trigger at 2_000_000");
}

#[test]
fn test_outflow_rule_does_not_trigger_below_threshold() {
    let mut engine = SafetyRuleEngine::with_defaults();
    let mut ctx = clean_ctx(1);
    ctx.total_outflow = 500_000u128; // below 1_000_000 threshold

    let results = engine.evaluate(&ctx);
    let outflow_triggered = results.iter().any(|r| r.policy_name == "AbnormalOutflow" && r.triggered);
    assert!(!outflow_triggered, "AbnormalOutflow should NOT trigger at 500_000");
}

#[test]
fn test_mutation_velocity_rule_triggers() {
    let mut engine = SafetyRuleEngine::with_defaults();
    let mut ctx = clean_ctx(1);
    ctx.domain_mutation_counts.insert("global".to_string(), 150); // above 100

    let results = engine.evaluate(&ctx);
    let triggered = results.iter().any(|r| r.policy_name == "AbnormalMutationVelocity" && r.triggered);
    assert!(triggered, "AbnormalMutationVelocity should trigger at 150 mutations");
}

#[test]
fn test_solver_misconduct_escalation() {
    let mut engine = SafetyRuleEngine::with_defaults();
    let solver_id = [0xABu8; 32];
    let mut ctx = clean_ctx(1);
    ctx.solver_id = Some(solver_id);
    ctx.rights_violations_count = 3; // triggers threshold

    let results = engine.evaluate(&ctx);
    // SolverMisconduct should trigger
    let triggered = results.iter().any(|r| r.policy_name == "SolverMisconduct" && r.triggered);
    assert!(triggered, "SolverMisconduct should trigger with 3 violations");
}

#[test]
fn test_governance_sensitive_rule_triggers() {
    let mut engine = SafetyRuleEngine::with_defaults();
    let mut ctx = clean_ctx(1);
    ctx.is_governance_sensitive = true;

    let results = engine.evaluate(&ctx);
    let triggered = results.iter().any(|r| r.policy_name == "GovernanceSensitiveObjectMisuse" && r.triggered);
    assert!(triggered, "GovernanceSensitiveObjectMisuse should trigger when is_governance_sensitive");
}
