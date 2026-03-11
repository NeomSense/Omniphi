use std::collections::BTreeSet;

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct QuarantineAction {
    pub object_ids: Vec<[u8; 32]>,
    pub reason: String,
    pub reversible: bool,
    pub governance_required_to_lift: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct PauseAction {
    pub target: PauseTarget,
    pub reason: String,
    pub temporary_until_epoch: Option<u64>,
    pub governance_required_to_lift: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum PauseTarget {
    Domain(String),
    ObjectClass(String),
    ExecutionClass(String),
    LiquidityVenue([u8; 32]),
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct RateLimitAction {
    pub target_domain: Option<String>,
    pub target_solver: Option<[u8; 32]>,
    pub max_mutations_per_epoch: u64,
    pub max_value_per_epoch: u128,
    pub reason: String,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct ExecutionRestrictionAction {
    pub restrict_to_downgrade_only: bool,
    pub restrict_to_provisional_only: bool,
    pub blocked_intent_classes: Vec<String>,
    pub blocked_solver_ids: BTreeSet<[u8; 32]>,
    pub reason: String,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct EmergencyModeAction {
    pub activated: bool,
    pub reason: String,
    pub requires_governance_to_deactivate: bool,
    // Placeholder — full-chain emergency is a future governance workflow
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum SafetyAction {
    NoAction,
    LogOnly(String),
    Quarantine(QuarantineAction),
    Pause(PauseAction),
    RateLimit(RateLimitAction),
    SuspendSolver([u8; 32], String),
    ExecutionRestriction(ExecutionRestrictionAction),
    EmergencyMode(EmergencyModeAction),
    MultiAction(Vec<SafetyAction>), // Stacked actions
}

impl SafetyAction {
    pub fn is_scoped(&self) -> bool {
        !matches!(
            self,
            SafetyAction::EmergencyMode(_) | SafetyAction::MultiAction(_)
        )
    }

    pub fn requires_governance(&self) -> bool {
        match self {
            SafetyAction::Quarantine(q) => q.governance_required_to_lift,
            SafetyAction::Pause(p) => p.governance_required_to_lift,
            SafetyAction::EmergencyMode(_) => true,
            SafetyAction::MultiAction(actions) => actions.iter().any(|a| a.requires_governance()),
            _ => false,
        }
    }
}
