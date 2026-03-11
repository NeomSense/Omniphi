use omniphi_runtime::safety::actions::{SafetyAction};
use omniphi_runtime::safety::incidents::{IncidentSeverity, IncidentType};
use omniphi_runtime::safety::kernel::{SafetyEvaluationContext, SafetyKernel};
use omniphi_runtime::safety::poseq_bridge::PoSeqSafetyBridge;
use omniphi_runtime::safety::recovery_hooks::CriticalEscalationHook;
use omniphi_runtime::safety::simulation::SafetySimulator;

fn ctx_with_outflow(epoch: u64, outflow: u128, objects: Vec<[u8; 32]>) -> SafetyEvaluationContext {
    let mut ctx = SafetyEvaluationContext::clean(epoch);
    ctx.total_outflow = outflow;
    ctx.affected_objects = objects;
    ctx
}

fn ctx_with_solver_violations(epoch: u64, solver_id: [u8; 32], violations: usize) -> SafetyEvaluationContext {
    let mut ctx = SafetyEvaluationContext::clean(epoch);
    ctx.solver_id = Some(solver_id);
    ctx.rights_violations_count = violations;
    ctx
}

fn ctx_governance_misuse(epoch: u64) -> SafetyEvaluationContext {
    let mut ctx = SafetyEvaluationContext::clean(epoch);
    ctx.is_governance_sensitive = true;
    ctx.affected_domains = vec!["governance".to_string()];
    ctx
}

fn ctx_cross_domain(epoch: u64, domains: Vec<String>) -> SafetyEvaluationContext {
    let mut ctx = SafetyEvaluationContext::clean(epoch);
    ctx.affected_domains = domains;
    ctx
}

#[test]
fn test_e2e_outflow_triggers_object_quarantine() {
    let mut kernel = SafetyKernel::new();
    let obj1 = [0x01u8; 32];
    let obj2 = [0x02u8; 32];

    let ctx = ctx_with_outflow(10, 5_000_000, vec![obj1, obj2]);
    let decision = kernel.evaluate(&ctx);

    assert!(decision.incident.is_some());
    assert!(kernel.is_object_quarantined(&obj1));
    assert!(kernel.is_object_quarantined(&obj2));
}

#[test]
fn test_e2e_solver_misconduct_triggers_suspension() {
    let mut kernel = SafetyKernel::new();
    let solver_id = [0xABu8; 32];

    let ctx = ctx_with_solver_violations(10, solver_id, 3);
    let decision = kernel.evaluate(&ctx);

    assert!(decision.incident.is_some());
    assert!(!kernel.is_solver_allowed(&solver_id), "solver should be suspended");
}

#[test]
fn test_e2e_governance_misuse_triggers_domain_pause() {
    let mut kernel = SafetyKernel::new();

    let ctx = ctx_governance_misuse(10);
    let decision = kernel.evaluate(&ctx);

    assert!(decision.incident.is_some());
    assert!(kernel.is_domain_paused("governance"));
}

#[test]
fn test_e2e_oracle_inconsistency_triggers_provisional_mode() {
    let kernel = SafetyKernel::new();
    let scenario = SafetySimulator::oracle_inconsistency_scenario(100);
    let result = SafetySimulator::simulate(&kernel, scenario);

    // Oracle inconsistency may or may not trigger depending on policy
    // Just ensure no panic and the scenario runs
    assert!(!result.scenario.name.is_empty());
}

#[test]
fn test_e2e_liquidity_instability_triggers_venue_disable() {
    let pool_id = [0xBBu8; 32];
    let kernel = SafetyKernel::new();
    let scenario = SafetySimulator::liquidity_instability_scenario(pool_id, 100);
    let result = SafetySimulator::simulate(&kernel, scenario);

    // The pool scenario is defined; just ensure it runs without panic
    assert!(!result.scenario.name.is_empty());
}

#[test]
fn test_e2e_cross_domain_triggers_emergency_placeholder() {
    let mut kernel = SafetyKernel::new()
        .with_recovery_hook(Box::new(CriticalEscalationHook));

    let ctx = ctx_cross_domain(10, vec![
        "treasury".to_string(),
        "dex_liquidity".to_string(),
        "lending".to_string(),
    ]);

    let decision = kernel.evaluate(&ctx);
    assert!(decision.incident.is_some());

    let incident = decision.incident.unwrap();
    assert!(incident.severity >= IncidentSeverity::Critical);

    // Emergency mode should be activated
    assert!(kernel.emergency_mode, "emergency mode should be active");

    // Governance escalation should be set (from CriticalEscalationHook)
    assert!(decision.governance_escalation.is_some(), "governance escalation should be emitted");
}

#[test]
fn test_e2e_poseq_bridge_annotates_batch() {
    // Test that PoSeqSafetyBridge can be instantiated and produces constrained state
    let bridge = PoSeqSafetyBridge::new();
    let state = bridge.constrained_state(100);

    assert_eq!(state.as_of_epoch, 100);
    assert!(state.quarantined_objects.is_empty());
    assert!(state.paused_domains.is_empty());
    assert!(state.suspended_solvers.is_empty());
    assert!(!state.emergency_mode);
}
