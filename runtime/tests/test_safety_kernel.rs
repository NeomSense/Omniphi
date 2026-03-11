use omniphi_runtime::safety::actions::{PauseTarget, SafetyAction};
use omniphi_runtime::safety::incidents::IncidentSeverity;
use omniphi_runtime::safety::kernel::{ConstrainedStateUpdate, SafetyEvaluationContext, SafetyKernel};
use omniphi_runtime::safety::recovery_hooks::CriticalEscalationHook;

fn clean_kernel() -> SafetyKernel {
    SafetyKernel::new()
}

fn clean_ctx(epoch: u64) -> SafetyEvaluationContext {
    SafetyEvaluationContext::clean(epoch)
}

#[test]
fn test_kernel_no_incident_on_clean_context() {
    let mut kernel = clean_kernel();
    let ctx = clean_ctx(1);
    let decision = kernel.evaluate(&ctx);

    assert!(decision.incident.is_none(), "clean context → no incident");
    assert!(matches!(decision.action, SafetyAction::NoAction));
}

#[test]
fn test_kernel_triggers_on_abnormal_outflow() {
    let mut kernel = clean_kernel();
    let mut ctx = clean_ctx(1);
    ctx.total_outflow = 2_000_000u128;
    ctx.affected_objects = vec![[0x01u8; 32]];

    let decision = kernel.evaluate(&ctx);
    assert!(decision.incident.is_some(), "should produce an incident");

    let incident = decision.incident.unwrap();
    assert!(incident.severity >= IncidentSeverity::High);
}

#[test]
fn test_kernel_quarantines_objects_on_high_severity() {
    let mut kernel = clean_kernel();
    let obj_id = [0x11u8; 32];
    let mut ctx = clean_ctx(1);
    ctx.total_outflow = 5_000_000u128; // triggers AbnormalOutflow → Quarantine
    ctx.affected_objects = vec![obj_id];

    let decision = kernel.evaluate(&ctx);

    // Should have quarantined the object
    let quarantined = kernel.is_object_quarantined(&obj_id);
    assert!(quarantined, "affected object should be quarantined after outflow incident");
}

#[test]
fn test_kernel_suspends_solver_on_misconduct() {
    let mut kernel = clean_kernel();
    let solver_id = [0xABu8; 32];
    let mut ctx = clean_ctx(1);
    ctx.solver_id = Some(solver_id);
    ctx.rights_violations_count = 3;

    let decision = kernel.evaluate(&ctx);
    assert!(decision.incident.is_some());

    // Solver should be suspended
    assert!(!kernel.is_solver_allowed(&solver_id), "solver should be suspended after misconduct");
}

#[test]
fn test_kernel_pauses_domain_on_governance_misuse() {
    let mut kernel = clean_kernel();
    let mut ctx = clean_ctx(1);
    ctx.is_governance_sensitive = true;
    ctx.affected_domains = vec!["governance".to_string()];

    let decision = kernel.evaluate(&ctx);
    assert!(decision.incident.is_some());

    // Domain should be paused
    assert!(
        kernel.is_domain_paused("governance"),
        "governance domain should be paused after misuse"
    );
}

#[test]
fn test_kernel_emergency_mode_on_cross_domain() {
    let mut kernel = clean_kernel();
    let mut ctx = clean_ctx(1);
    ctx.affected_domains = vec![
        "treasury".to_string(),
        "dex_liquidity".to_string(),
        "lending".to_string(),
    ];

    let decision = kernel.evaluate(&ctx);
    assert!(decision.incident.is_some());

    // Emergency mode or multi-domain action should be triggered
    let incident = decision.incident.unwrap();
    assert!(
        incident.severity >= IncidentSeverity::Critical,
        "cross-domain with 3+ domains should be Critical+"
    );
}

#[test]
fn test_object_quarantine_state_persists() {
    let mut kernel = clean_kernel();
    let obj_id = [0x22u8; 32];

    // First evaluation triggers quarantine
    let mut ctx = clean_ctx(1);
    ctx.total_outflow = 3_000_000u128;
    ctx.affected_objects = vec![obj_id];
    kernel.evaluate(&ctx);

    // Check persistence across a second evaluation
    let ctx2 = clean_ctx(2);
    kernel.evaluate(&ctx2);

    assert!(
        kernel.is_object_quarantined(&obj_id),
        "quarantine should persist across evaluations"
    );
}

#[test]
fn test_domain_pause_state_persists() {
    let mut kernel = clean_kernel();

    let mut ctx = clean_ctx(1);
    ctx.is_governance_sensitive = true;
    ctx.affected_domains = vec!["governance".to_string()];
    kernel.evaluate(&ctx);

    // After pause, domain stays paused
    let ctx2 = clean_ctx(2);
    kernel.evaluate(&ctx2);

    assert!(
        kernel.is_domain_paused("governance"),
        "domain pause should persist"
    );
}

#[test]
fn test_solver_suspension_state_persists() {
    let mut kernel = clean_kernel();
    let solver_id = [0xCCu8; 32];

    let mut ctx = clean_ctx(1);
    ctx.solver_id = Some(solver_id);
    ctx.rights_violations_count = 3;
    kernel.evaluate(&ctx);

    // After suspension, solver stays suspended
    let ctx2 = clean_ctx(2);
    kernel.evaluate(&ctx2);

    assert!(
        !kernel.is_solver_allowed(&solver_id),
        "solver suspension should persist"
    );
}
