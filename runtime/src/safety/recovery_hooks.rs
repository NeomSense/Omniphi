use crate::safety::incidents::{IncidentSeverity, SafetyIncident};

#[derive(Debug, Clone)]
pub struct GovernanceEscalationMarker {
    pub incident_id: [u8; 32],
    pub reason: String,
    pub required_action_type: String,
    pub urgency: EscalationUrgency,
    pub epoch: u64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EscalationUrgency {
    Routine,
    Expedited,
    Emergency,
}

#[derive(Debug, Clone)]
pub struct RecoveryProposalReference {
    pub incident_id: [u8; 32],
    pub proposal_type: String, // "unquarantine_object", "lift_domain_pause", "reinstate_solver", etc.
    pub target_id: Option<[u8; 32]>, // object_id, solver_id, or None for domain
    pub target_domain: Option<String>,
    pub proposed_at_epoch: u64,
    pub status: ProposalReferenceStatus,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProposalReferenceStatus {
    Pending,
    Approved,
    Rejected,
    Executed,
}

#[derive(Debug, Clone)]
pub struct PostIncidentReviewRecord {
    pub incident_id: [u8; 32],
    pub review_epoch: u64,
    pub findings: String,
    pub policy_changes_recommended: Vec<String>,
    pub resolved: bool,
}

/// Trait for recovery hook implementations.
/// All implementations must be deterministic.
pub trait RecoveryHook: Send + Sync {
    fn hook_name(&self) -> &str;
    /// Called when an incident requires governance escalation.
    /// Returns a GovernanceEscalationMarker if escalation is needed.
    fn on_incident(&self, incident: &SafetyIncident) -> Option<GovernanceEscalationMarker>;
}

/// Default no-op hook (used in tests and as placeholder).
pub struct NoOpRecoveryHook;

impl RecoveryHook for NoOpRecoveryHook {
    fn hook_name(&self) -> &str {
        "noop"
    }
    fn on_incident(&self, _incident: &SafetyIncident) -> Option<GovernanceEscalationMarker> {
        None
    }
}

/// Hook that emits escalation for Critical+ incidents.
pub struct CriticalEscalationHook;

impl RecoveryHook for CriticalEscalationHook {
    fn hook_name(&self) -> &str {
        "critical_escalation"
    }
    fn on_incident(&self, incident: &SafetyIncident) -> Option<GovernanceEscalationMarker> {
        if incident.severity >= IncidentSeverity::Critical {
            Some(GovernanceEscalationMarker {
                incident_id: incident.incident_id,
                reason: incident.detail.clone(),
                required_action_type: "governance_review".into(),
                urgency: EscalationUrgency::Emergency,
                epoch: incident.epoch,
            })
        } else {
            None
        }
    }
}
