use omniphi_runtime::safety::incidents::IncidentSeverity;
use omniphi_runtime::safety::kernel::{SafetyEvaluationContext, SafetyKernel};
use omniphi_runtime::safety::simulation::{IncidentScenario, SafetySimulator};

#[test]
fn test_simulate_abnormal_outflow() {
    let kernel = SafetyKernel::new();
    let scenario = SafetySimulator::abnormal_outflow_scenario(100);
    let result = SafetySimulator::simulate(&kernel, scenario);

    assert!(result.expectation_met, "abnormal outflow scenario expectation should be met");
    assert!(result.actual_decision.incident.is_some());
}

#[test]
fn test_simulate_does_not_mutate_kernel() {
    let kernel = SafetyKernel::new();

    // Kernel starts with no quarantined objects
    assert!(kernel.quarantined_objects.is_empty());
    assert!(!kernel.emergency_mode);

    let scenario = SafetySimulator::cross_domain_emergency_scenario(100);
    let _result = SafetySimulator::simulate(&kernel, scenario);

    // Original kernel should be unchanged
    assert!(
        kernel.quarantined_objects.is_empty(),
        "simulation should not mutate the real kernel"
    );
    assert!(!kernel.emergency_mode, "simulation should not activate emergency mode on real kernel");
}

#[test]
fn test_coverage_report_lists_all_policies() {
    let kernel = SafetyKernel::new();
    let report = SafetySimulator::coverage_report(&kernel);

    assert!(report.total_policies > 0);
    assert!(report.enabled_policies > 0);
    assert!(!report.policies_triggered.is_empty(), "enabled policies should appear in report");
    // There will be uncovered types since we only have 7 default policies for 15 incident types
    // Just ensure the report runs without panic
}

#[test]
fn test_run_all_scenarios() {
    let kernel = SafetyKernel::new();
    let results = SafetySimulator::run_all_scenarios(&kernel);

    assert_eq!(results.len(), 6, "should run 6 predefined scenarios");

    // Each scenario should have a name
    for result in &results {
        assert!(!result.scenario.name.is_empty());
    }

    // Original kernel unchanged
    assert!(kernel.quarantined_objects.is_empty());
    assert!(!kernel.emergency_mode);
}
