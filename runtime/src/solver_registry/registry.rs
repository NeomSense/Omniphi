use crate::capabilities::checker::CapabilitySet;
use crate::errors::RuntimeError;
use std::collections::BTreeMap;

/// Lifecycle status of a registered solver.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum SolverStatus {
    Pending,
    Active,
    Paused,
    Banned,
}

/// The capability scope and intent class restrictions for a solver.
#[derive(Debug, Clone)]
pub struct SolverCapabilities {
    pub capability_set: CapabilitySet,
    /// Which intent classes this solver is allowed to handle.
    /// Valid values: "transfer", "swap", "yield_allocate", "treasury_rebalance"
    pub allowed_intent_classes: Vec<String>,
    /// Domain tags for filtering. E.g., "defi", "treasury", "identity", "governance"
    pub domain_tags: Vec<String>,
    pub max_objects_per_plan: usize,
}

/// Running statistics for a solver's submission history.
#[derive(Debug, Clone)]
pub struct SolverReputationRecord {
    pub total_plans_submitted: u64,
    pub plans_accepted: u64,
    pub plans_rejected: u64,
    pub plans_won: u64,
    pub consecutive_valid_plans: u64,
    pub consecutive_invalid_plans: u64,
    /// 0–10000 basis points. Default: 5000 (neutral).
    pub reputation_score: u64,
    pub is_flagged: bool,
}

impl Default for SolverReputationRecord {
    fn default() -> Self {
        SolverReputationRecord {
            total_plans_submitted: 0,
            plans_accepted: 0,
            plans_rejected: 0,
            plans_won: 0,
            consecutive_valid_plans: 0,
            consecutive_invalid_plans: 0,
            reputation_score: 5000,
            is_flagged: false,
        }
    }
}

/// Full description of a registered solver.
#[derive(Debug, Clone)]
pub struct SolverProfile {
    pub solver_id: [u8; 32],
    pub display_name: String,
    pub public_key: [u8; 32],
    pub status: SolverStatus,
    pub capabilities: SolverCapabilities,
    pub reputation: SolverReputationRecord,
    /// Placeholder for bonding/staking; not enforced in Phase 2.
    pub stake_amount: u128,
    pub registered_at_epoch: u64,
    pub last_active_epoch: u64,
    /// True when this profile represents an AI/agent solver.
    pub is_agent: bool,
    pub metadata: BTreeMap<String, String>,
}

/// Registry of all known solvers, keyed by solver_id.
pub struct SolverRegistry {
    solvers: BTreeMap<[u8; 32], SolverProfile>,
}

impl SolverRegistry {
    pub fn new() -> Self {
        SolverRegistry {
            solvers: BTreeMap::new(),
        }
    }

    /// Register a new solver profile.
    ///
    /// Errors:
    /// - `SolverNotRegistered` variant is not applicable here; this returns
    ///   `InvalidIntent` if solver_id is all-zeros.
    /// - `SolverNotEligible` if already registered.
    pub fn register(&mut self, profile: SolverProfile) -> Result<(), RuntimeError> {
        if profile.solver_id == [0u8; 32] {
            return Err(RuntimeError::SolverNotEligible {
                solver_id: hex::encode(profile.solver_id),
                reason: "solver_id must be non-zero".to_string(),
            });
        }
        if self.solvers.contains_key(&profile.solver_id) {
            return Err(RuntimeError::SolverNotEligible {
                solver_id: hex::encode(profile.solver_id),
                reason: "solver already registered".to_string(),
            });
        }
        self.solvers.insert(profile.solver_id, profile);
        Ok(())
    }

    /// Immutable reference to a solver profile by id.
    pub fn get(&self, solver_id: &[u8; 32]) -> Option<&SolverProfile> {
        self.solvers.get(solver_id)
    }

    /// Mutable reference to a solver profile by id.
    pub fn get_mut(&mut self, solver_id: &[u8; 32]) -> Option<&mut SolverProfile> {
        self.solvers.get_mut(solver_id)
    }

    /// Update the status of a registered solver.
    pub fn set_status(
        &mut self,
        solver_id: &[u8; 32],
        status: SolverStatus,
    ) -> Result<(), RuntimeError> {
        match self.solvers.get_mut(solver_id) {
            Some(profile) => {
                profile.status = status;
                Ok(())
            }
            None => Err(RuntimeError::SolverNotRegistered(hex::encode(solver_id))),
        }
    }

    /// Returns all solvers currently in Active status.
    pub fn active_solvers(&self) -> Vec<&SolverProfile> {
        self.solvers
            .values()
            .filter(|p| p.status == SolverStatus::Active)
            .collect()
    }

    /// Record a plan submission outcome and update the solver's reputation.
    ///
    /// - `accepted`: the plan passed validation
    /// - `won`: the plan was selected as the winner
    ///
    /// If a solver accumulates 3 or more consecutive invalid plans, they are
    /// automatically flagged for review.
    pub fn record_submission(
        &mut self,
        solver_id: &[u8; 32],
        accepted: bool,
        won: bool,
    ) {
        if let Some(profile) = self.solvers.get_mut(solver_id) {
            let rep = &mut profile.reputation;
            rep.total_plans_submitted += 1;
            if accepted {
                rep.plans_accepted += 1;
                rep.consecutive_valid_plans += 1;
                rep.consecutive_invalid_plans = 0;
            } else {
                rep.plans_rejected += 1;
                rep.consecutive_invalid_plans += 1;
                rep.consecutive_valid_plans = 0;
                // Auto-flag after 3+ consecutive invalid plans
                if rep.consecutive_invalid_plans >= 3 {
                    rep.is_flagged = true;
                }
            }
            if won {
                rep.plans_won += 1;
            }
        }
    }

    /// Directly set the reputation score for a solver (governance action).
    pub fn update_reputation_score(&mut self, solver_id: &[u8; 32], score: u64) {
        if let Some(profile) = self.solvers.get_mut(solver_id) {
            profile.reputation.reputation_score = score.min(10_000);
        }
    }

    /// Returns all registered solvers (sorted by solver_id due to BTreeMap).
    pub fn all_solvers(&self) -> Vec<&SolverProfile> {
        self.solvers.values().collect()
    }
}

impl Default for SolverRegistry {
    fn default() -> Self {
        Self::new()
    }
}
