use omniphi_runtime::safety::incidents::{
    AffectedDomainSet, IncidentScope, IncidentSeverity, IncidentType, SafetyIncident,
};
use omniphi_runtime::safety::actions::SafetyAction;
use omniphi_runtime::safety::kernel::SafetyEvaluationContext;
use std::collections::BTreeMap;

fn make_incident(severity: IncidentSeverity, incident_type: IncidentType) -> SafetyIncident {
    let incident_id = SafetyIncident::compute_id(&incident_type, 1, None);
    SafetyIncident {
        incident_id,
        incident_type,
        severity,
        scope: IncidentScope::FullChain,
        affected: AffectedDomainSet::new(),
        triggering_rule: "test_rule".to_string(),
        detail: "test detail".to_string(),
        goal_packet_id: None,
        plan_id: None,
        solver_id: None,
        capsule_hash: None,
        epoch: 1,
        reversible: true,
        requires_governance: false,
        metadata: BTreeMap::new(),
    }
}

#[test]
fn test_incident_id_deterministic() {
    let id1 = SafetyIncident::compute_id(&IncidentType::AbnormalOutflow, 100, None);
    let id2 = SafetyIncident::compute_id(&IncidentType::AbnormalOutflow, 100, None);
    assert_eq!(id1, id2, "same inputs → same id");

    let id3 = SafetyIncident::compute_id(&IncidentType::AbnormalOutflow, 101, None);
    assert_ne!(id1, id3, "different epoch → different id");

    let solver = [0xABu8; 32];
    let id4 = SafetyIncident::compute_id(&IncidentType::SolverMisconduct, 100, Some(solver));
    let id5 = SafetyIncident::compute_id(&IncidentType::SolverMisconduct, 100, Some(solver));
    assert_eq!(id4, id5, "same solver → same id");

    let id6 = SafetyIncident::compute_id(&IncidentType::SolverMisconduct, 100, None);
    assert_ne!(id4, id6, "solver vs no solver → different id");
}

#[test]
fn test_incident_severity_ordering() {
    assert!(IncidentSeverity::Info < IncidentSeverity::Low);
    assert!(IncidentSeverity::Low < IncidentSeverity::Medium);
    assert!(IncidentSeverity::Medium < IncidentSeverity::High);
    assert!(IncidentSeverity::High < IncidentSeverity::Critical);
    assert!(IncidentSeverity::Critical < IncidentSeverity::Emergency);
}

#[test]
fn test_affected_domain_set() {
    let mut set = AffectedDomainSet::new();
    set.domains.insert("treasury".to_string());
    set.domains.insert("dex".to_string());
    set.object_ids.insert([0x01u8; 32]);
    set.solver_ids.insert([0xFFu8; 32]);

    assert_eq!(set.domains.len(), 2);
    assert!(set.domains.contains("treasury"));
    assert_eq!(set.object_ids.len(), 1);
    assert_eq!(set.solver_ids.len(), 1);
}

#[test]
fn test_classification_result_has_action() {
    use omniphi_runtime::safety::incidents::SafetyClassificationResult;

    let incident = make_incident(IncidentSeverity::High, IncidentType::AbnormalOutflow);
    let result = SafetyClassificationResult {
        incident: incident.clone(),
        recommended_action: SafetyAction::NoAction,
        escalation_required: false,
    };

    assert_eq!(result.incident.severity, IncidentSeverity::High);
    assert!(!result.escalation_required);
    assert_eq!(result.recommended_action, SafetyAction::NoAction);
}
