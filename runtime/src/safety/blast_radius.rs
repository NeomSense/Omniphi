use crate::safety::incidents::{IncidentSeverity, SafetyIncident};
use std::collections::{BTreeMap, BTreeSet};

#[derive(Debug, Clone)]
pub struct BlastRadiusAssessment {
    pub incident_id: [u8; 32],
    pub minimum_scope: ContainmentScope,
    pub recommended_scope: ContainmentScope,
    pub affected_domains: BTreeSet<String>,
    pub affected_objects: BTreeSet<[u8; 32]>,
    pub affected_solvers: BTreeSet<[u8; 32]>,
    pub cross_domain: bool,
    pub systemic_risk: bool,
    pub escalation_triggers: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum ContainmentScope {
    Minimal,        // Single object or single solver
    Scoped,         // Object class or domain subsection
    Domain,         // Full domain
    MultiDomain,    // Multiple domains
    ExecutionClass, // All intents of a class
    Global,         // Emergency full-chain
}

#[derive(Debug, Clone)]
pub struct ScopeResolutionPolicy {
    pub prefer_minimal_scope: bool,
    pub max_scope_without_governance: ContainmentScope,
    pub cross_domain_threshold: usize, // # domains before escalating to MultiDomain
    pub emergency_threshold: usize,    // # domains before Global
}

impl Default for ScopeResolutionPolicy {
    fn default() -> Self {
        ScopeResolutionPolicy {
            prefer_minimal_scope: true,
            max_scope_without_governance: ContainmentScope::Domain,
            cross_domain_threshold: 2,
            emergency_threshold: 4,
        }
    }
}

#[derive(Debug, Clone)]
pub struct DomainContainmentMap {
    pub domain_to_objects: BTreeMap<String, BTreeSet<[u8; 32]>>,
    pub domain_to_solvers: BTreeMap<String, BTreeSet<[u8; 32]>>,
    pub domain_dependencies: BTreeMap<String, BTreeSet<String>>,
}

pub struct BlastRadiusEngine;

impl BlastRadiusEngine {
    pub fn assess(
        incident: &SafetyIncident,
        domain_map: &DomainContainmentMap,
        policy: &ScopeResolutionPolicy,
    ) -> BlastRadiusAssessment {
        let mut affected_domains = incident.affected.domains.clone();
        let mut affected_objects = incident.affected.object_ids.clone();
        let mut affected_solvers = incident.affected.solver_ids.clone();
        let mut escalation_triggers: Vec<String> = vec![];

        // Expand from domain map
        for domain in &incident.affected.domains {
            if let Some(objs) = domain_map.domain_to_objects.get(domain) {
                affected_objects.extend(objs.iter().copied());
            }
            if let Some(solvers) = domain_map.domain_to_solvers.get(domain) {
                affected_solvers.extend(solvers.iter().copied());
            }
            // Follow domain dependencies
            if let Some(deps) = domain_map.domain_dependencies.get(domain) {
                for dep in deps {
                    affected_domains.insert(dep.clone());
                    if let Some(objs) = domain_map.domain_to_objects.get(dep) {
                        affected_objects.extend(objs.iter().copied());
                    }
                    if let Some(solvers) = domain_map.domain_to_solvers.get(dep) {
                        affected_solvers.extend(solvers.iter().copied());
                    }
                }
            }
        }

        let affected_domain_count = affected_domains.len();
        let cross_domain = affected_domain_count >= policy.cross_domain_threshold;
        let systemic_risk = affected_domain_count >= policy.emergency_threshold;

        if cross_domain {
            escalation_triggers.push(format!(
                "cross_domain_threshold={} reached ({})",
                policy.cross_domain_threshold, affected_domain_count
            ));
        }
        if systemic_risk {
            escalation_triggers.push(format!(
                "emergency_threshold={} reached ({})",
                policy.emergency_threshold, affected_domain_count
            ));
        }

        let minimum_scope =
            Self::scope_from_incident(incident, affected_domain_count, policy);
        let recommended_scope = if policy.prefer_minimal_scope {
            minimum_scope.clone()
        } else {
            minimum_scope.clone()
        };

        BlastRadiusAssessment {
            incident_id: incident.incident_id,
            minimum_scope,
            recommended_scope,
            affected_domains,
            affected_objects,
            affected_solvers,
            cross_domain,
            systemic_risk,
            escalation_triggers,
        }
    }

    fn scope_from_incident(
        incident: &SafetyIncident,
        affected_domain_count: usize,
        policy: &ScopeResolutionPolicy,
    ) -> ContainmentScope {
        // Emergency threshold → Global
        if affected_domain_count >= policy.emergency_threshold {
            return ContainmentScope::Global;
        }

        // Cross-domain threshold → MultiDomain
        if affected_domain_count >= policy.cross_domain_threshold {
            return ContainmentScope::MultiDomain;
        }

        // Based on severity
        match &incident.severity {
            IncidentSeverity::Info | IncidentSeverity::Low => ContainmentScope::Minimal,
            IncidentSeverity::Medium => {
                if affected_domain_count > 0 {
                    ContainmentScope::Scoped
                } else {
                    ContainmentScope::Minimal
                }
            }
            IncidentSeverity::High => {
                if affected_domain_count > 0 {
                    ContainmentScope::Domain
                } else {
                    ContainmentScope::Scoped
                }
            }
            IncidentSeverity::Critical => ContainmentScope::MultiDomain,
            IncidentSeverity::Emergency => ContainmentScope::Global,
        }
    }
}
