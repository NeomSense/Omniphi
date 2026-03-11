use omniphi_runtime::safety::blast_radius::{
    BlastRadiusEngine, ContainmentScope, DomainContainmentMap, ScopeResolutionPolicy,
};
use omniphi_runtime::safety::incidents::{
    AffectedDomainSet, IncidentScope, IncidentSeverity, IncidentType, SafetyIncident,
};
use std::collections::{BTreeMap, BTreeSet};

fn make_incident_with_severity_and_domains(
    severity: IncidentSeverity,
    domains: Vec<String>,
    objects: Vec<[u8; 32]>,
) -> SafetyIncident {
    let incident_type = IncidentType::AbnormalOutflow;
    let incident_id = SafetyIncident::compute_id(&incident_type, 1, None);
    let mut affected = AffectedDomainSet::new();
    for d in &domains {
        affected.domains.insert(d.clone());
    }
    for obj in &objects {
        affected.object_ids.insert(*obj);
    }
    SafetyIncident {
        incident_id,
        incident_type,
        severity,
        scope: if domains.is_empty() {
            IncidentScope::FullChain
        } else if domains.len() == 1 {
            IncidentScope::Domain(domains[0].clone())
        } else {
            IncidentScope::MultiDomain(domains.clone())
        },
        affected,
        triggering_rule: "test".to_string(),
        detail: "test".to_string(),
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

fn empty_domain_map() -> DomainContainmentMap {
    DomainContainmentMap {
        domain_to_objects: BTreeMap::new(),
        domain_to_solvers: BTreeMap::new(),
        domain_dependencies: BTreeMap::new(),
    }
}

#[test]
fn test_low_severity_gives_minimal_scope() {
    let incident = make_incident_with_severity_and_domains(IncidentSeverity::Low, vec![], vec![]);
    let policy = ScopeResolutionPolicy::default();
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    assert_eq!(assessment.minimum_scope, ContainmentScope::Minimal);
}

#[test]
fn test_high_severity_gives_domain_scope() {
    let incident = make_incident_with_severity_and_domains(
        IncidentSeverity::High,
        vec!["treasury".to_string()],
        vec![],
    );
    let policy = ScopeResolutionPolicy::default();
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    // High severity with 1 domain → Domain scope
    assert_eq!(assessment.minimum_scope, ContainmentScope::Domain);
}

#[test]
fn test_critical_gives_multi_domain() {
    let incident = make_incident_with_severity_and_domains(
        IncidentSeverity::Critical,
        vec!["treasury".to_string(), "dex".to_string()],
        vec![],
    );
    let policy = ScopeResolutionPolicy {
        prefer_minimal_scope: true,
        max_scope_without_governance: ContainmentScope::Domain,
        cross_domain_threshold: 2,
        emergency_threshold: 4,
    };
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    // Critical + 2 domains (>= cross_domain_threshold=2) → MultiDomain
    assert_eq!(assessment.minimum_scope, ContainmentScope::MultiDomain);
    assert!(assessment.cross_domain);
}

#[test]
fn test_emergency_gives_global() {
    let incident = make_incident_with_severity_and_domains(
        IncidentSeverity::Emergency,
        vec![
            "treasury".to_string(),
            "dex".to_string(),
            "bridge".to_string(),
            "lending".to_string(),
        ],
        vec![],
    );
    let policy = ScopeResolutionPolicy {
        prefer_minimal_scope: true,
        max_scope_without_governance: ContainmentScope::Domain,
        cross_domain_threshold: 2,
        emergency_threshold: 4,
    };
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    assert_eq!(assessment.minimum_scope, ContainmentScope::Global);
    assert!(assessment.systemic_risk);
}

#[test]
fn test_cross_domain_threshold_escalation() {
    let incident = make_incident_with_severity_and_domains(
        IncidentSeverity::Medium,
        vec!["treasury".to_string(), "dex".to_string()],
        vec![],
    );
    let policy = ScopeResolutionPolicy {
        cross_domain_threshold: 2,
        emergency_threshold: 4,
        ..ScopeResolutionPolicy::default()
    };
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    assert!(assessment.cross_domain);
    assert!(!assessment.systemic_risk);
}

#[test]
fn test_prefer_minimal_scope_policy() {
    let incident = make_incident_with_severity_and_domains(
        IncidentSeverity::Low,
        vec![],
        vec![[0x01u8; 32]],
    );
    let policy = ScopeResolutionPolicy {
        prefer_minimal_scope: true,
        ..ScopeResolutionPolicy::default()
    };
    let map = empty_domain_map();

    let assessment = BlastRadiusEngine::assess(&incident, &map, &policy);
    assert_eq!(assessment.minimum_scope, assessment.recommended_scope);
    assert_eq!(assessment.minimum_scope, ContainmentScope::Minimal);
}
