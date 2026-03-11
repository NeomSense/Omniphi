use std::collections::{BTreeMap, BTreeSet};

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum IncidentSeverity {
    Info,       // Logged only, no action
    Low,        // Rate-limit or monitoring flag
    Medium,     // Scoped restriction (object/solver)
    High,       // Domain-level pause or suspension
    Critical,   // Multi-domain containment
    Emergency,  // Full-chain emergency mode placeholder
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum IncidentType {
    AbnormalOutflow,
    UnauthorizedDomainAccess,
    RightsScopeBreach,
    CausalValidityBreach,
    OracleCorruptionIndicator,
    SolverMisconduct,
    LiquidityPoolInstability,
    GovernanceSensitiveObjectMisuse,
    RepeatedBranchQuarantine,
    AbnormalMutationVelocity,
    CrossDomainBlastRadiusEscalation,
    RepeatedRightsViolation,
    UnauthorizedCapabilityUse,
    ExcessiveObjectVersionChurn,
    PolicyRuleViolation,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub enum IncidentScope {
    SingleObject([u8; 32]),
    ObjectClass(String),
    Solver([u8; 32]),
    Domain(String),
    MultiDomain(Vec<String>),
    ExecutionClass(String),
    LiquidityVenue([u8; 32]),
    FullChain,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct AffectedDomainSet {
    pub domains: BTreeSet<String>,
    pub object_ids: BTreeSet<[u8; 32]>,
    pub solver_ids: BTreeSet<[u8; 32]>,
}

impl AffectedDomainSet {
    pub fn new() -> Self {
        AffectedDomainSet {
            domains: BTreeSet::new(),
            object_ids: BTreeSet::new(),
            solver_ids: BTreeSet::new(),
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SafetyIncident {
    pub incident_id: [u8; 32],
    pub incident_type: IncidentType,
    pub severity: IncidentSeverity,
    pub scope: IncidentScope,
    pub affected: AffectedDomainSet,
    pub triggering_rule: String,
    pub detail: String,
    pub goal_packet_id: Option<[u8; 32]>,
    pub plan_id: Option<[u8; 32]>,
    pub solver_id: Option<[u8; 32]>,
    pub capsule_hash: Option<[u8; 32]>,
    pub epoch: u64,
    pub reversible: bool,
    pub requires_governance: bool,
    pub metadata: BTreeMap<String, String>,
}

#[derive(Debug, Clone)]
pub struct SafetyClassificationResult {
    pub incident: SafetyIncident,
    pub recommended_action: super::actions::SafetyAction,
    pub escalation_required: bool,
}

impl SafetyIncident {
    pub fn compute_id(
        incident_type: &IncidentType,
        epoch: u64,
        solver_id: Option<[u8; 32]>,
    ) -> [u8; 32] {
        use sha2::{Digest, Sha256};
        let input = format!(
            "{:?}:{}:{}",
            incident_type,
            epoch,
            solver_id
                .map(|s| hex::encode(s))
                .unwrap_or_default()
        );
        let hash = Sha256::digest(input.as_bytes());
        let mut id = [0u8; 32];
        id.copy_from_slice(&hash);
        id
    }
}
