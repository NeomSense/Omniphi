use crate::safety::incidents::IncidentType;
use std::collections::{BTreeMap, BTreeSet};

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum SolverSafetyStatus {
    Normal,
    Monitored,        // elevated tracking, no restrictions
    RateRestricted,   // throttled submission rate
    DowngradeOnly,    // can only submit downgrade-class plans
    Suspended,        // barred from submission
    Banned,           // permanent exclusion
}

#[derive(Debug, Clone)]
pub struct SolverRestrictionProfile {
    pub solver_id: [u8; 32],
    pub status: SolverSafetyStatus,
    pub blocked_intent_classes: BTreeSet<String>,
    pub max_submissions_per_epoch: u64, // 0 = unlimited
    pub require_stricter_policy: bool,
    pub suspension_until_epoch: Option<u64>,
    pub ban_reason: Option<String>,
}

#[derive(Debug, Clone)]
pub struct SolverIncidentRecord {
    pub solver_id: [u8; 32],
    pub incident_type: IncidentType,
    pub detail: String,
    pub epoch: u64,
    pub escalated: bool,
}

#[derive(Debug, Clone)]
pub enum SolverSafetyAction {
    Monitor([u8; 32]),
    RateRestrict {
        solver_id: [u8; 32],
        max_per_epoch: u64,
    },
    DowngradeOnly([u8; 32]),
    Suspend {
        solver_id: [u8; 32],
        until_epoch: u64,
    },
    Ban {
        solver_id: [u8; 32],
        reason: String,
    },
}

pub struct SolverSafetyController {
    pub restrictions: BTreeMap<[u8; 32], SolverRestrictionProfile>,
    pub incident_history: BTreeMap<[u8; 32], Vec<SolverIncidentRecord>>,
}

impl SolverSafetyController {
    pub fn new() -> Self {
        SolverSafetyController {
            restrictions: BTreeMap::new(),
            incident_history: BTreeMap::new(),
        }
    }

    pub fn apply_action(&mut self, action: SolverSafetyAction) {
        match action {
            SolverSafetyAction::Monitor(solver_id) => {
                let profile = self
                    .restrictions
                    .entry(solver_id)
                    .or_insert_with(|| SolverRestrictionProfile {
                        solver_id,
                        status: SolverSafetyStatus::Normal,
                        blocked_intent_classes: BTreeSet::new(),
                        max_submissions_per_epoch: 0,
                        require_stricter_policy: false,
                        suspension_until_epoch: None,
                        ban_reason: None,
                    });
                if profile.status == SolverSafetyStatus::Normal {
                    profile.status = SolverSafetyStatus::Monitored;
                }
            }

            SolverSafetyAction::RateRestrict {
                solver_id,
                max_per_epoch,
            } => {
                let profile = self
                    .restrictions
                    .entry(solver_id)
                    .or_insert_with(|| SolverRestrictionProfile {
                        solver_id,
                        status: SolverSafetyStatus::Normal,
                        blocked_intent_classes: BTreeSet::new(),
                        max_submissions_per_epoch: 0,
                        require_stricter_policy: false,
                        suspension_until_epoch: None,
                        ban_reason: None,
                    });
                profile.status = SolverSafetyStatus::RateRestricted;
                profile.max_submissions_per_epoch = max_per_epoch;
            }

            SolverSafetyAction::DowngradeOnly(solver_id) => {
                let profile = self
                    .restrictions
                    .entry(solver_id)
                    .or_insert_with(|| SolverRestrictionProfile {
                        solver_id,
                        status: SolverSafetyStatus::Normal,
                        blocked_intent_classes: BTreeSet::new(),
                        max_submissions_per_epoch: 0,
                        require_stricter_policy: false,
                        suspension_until_epoch: None,
                        ban_reason: None,
                    });
                profile.status = SolverSafetyStatus::DowngradeOnly;
            }

            SolverSafetyAction::Suspend {
                solver_id,
                until_epoch,
            } => {
                let profile = self
                    .restrictions
                    .entry(solver_id)
                    .or_insert_with(|| SolverRestrictionProfile {
                        solver_id,
                        status: SolverSafetyStatus::Normal,
                        blocked_intent_classes: BTreeSet::new(),
                        max_submissions_per_epoch: 0,
                        require_stricter_policy: false,
                        suspension_until_epoch: None,
                        ban_reason: None,
                    });
                profile.status = SolverSafetyStatus::Suspended;
                profile.suspension_until_epoch = Some(until_epoch);
            }

            SolverSafetyAction::Ban { solver_id, reason } => {
                let profile = self
                    .restrictions
                    .entry(solver_id)
                    .or_insert_with(|| SolverRestrictionProfile {
                        solver_id,
                        status: SolverSafetyStatus::Normal,
                        blocked_intent_classes: BTreeSet::new(),
                        max_submissions_per_epoch: 0,
                        require_stricter_policy: false,
                        suspension_until_epoch: None,
                        ban_reason: None,
                    });
                profile.status = SolverSafetyStatus::Banned;
                profile.ban_reason = Some(reason);
            }
        }
    }

    /// Returns false if status >= Suspended
    pub fn is_solver_allowed(&self, solver_id: &[u8; 32]) -> bool {
        match self.restrictions.get(solver_id) {
            Some(profile) => profile.status < SolverSafetyStatus::Suspended,
            None => true, // unknown solvers are allowed by default
        }
    }

    /// Returns Normal if not found
    pub fn get_status(&self, solver_id: &[u8; 32]) -> SolverSafetyStatus {
        self.restrictions
            .get(solver_id)
            .map(|p| p.status.clone())
            .unwrap_or(SolverSafetyStatus::Normal)
    }

    pub fn record_incident(&mut self, record: SolverIncidentRecord) {
        self.incident_history
            .entry(record.solver_id)
            .or_default()
            .push(record);
    }

    pub fn incident_count(
        &self,
        solver_id: &[u8; 32],
        window_epochs: u64,
        current_epoch: u64,
    ) -> u64 {
        let min_epoch = current_epoch.saturating_sub(window_epochs);
        self.incident_history
            .get(solver_id)
            .map(|records| {
                records
                    .iter()
                    .filter(|r| r.epoch >= min_epoch)
                    .count() as u64
            })
            .unwrap_or(0)
    }

    pub fn get_restriction(&self, solver_id: &[u8; 32]) -> Option<&SolverRestrictionProfile> {
        self.restrictions.get(solver_id)
    }
}
