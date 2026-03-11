use crate::intents::base::{IntentTransaction, IntentType};
use crate::solver_registry::registry::{SolverProfile, SolverRegistry, SolverStatus};

/// Matches intents to eligible solver profiles.
pub struct IntentSolverMatcher;

impl IntentSolverMatcher {
    /// Filter the registry for solvers eligible to handle this intent.
    ///
    /// Criteria:
    /// 1. `status == Active`
    /// 2. `allowed_intent_classes` contains the intent type string
    /// 3. Not flagged in reputation (`is_flagged == false`)
    ///
    /// Returns a `Vec<&SolverProfile>` sorted by `solver_id` (deterministic).
    pub fn find_eligible_solvers<'a>(
        intent: &IntentTransaction,
        registry: &'a SolverRegistry,
    ) -> Vec<&'a SolverProfile> {
        let class = Self::intent_class(intent);

        let mut eligible: Vec<&SolverProfile> = registry
            .all_solvers()
            .into_iter()
            .filter(|solver| {
                solver.status == SolverStatus::Active
                    && !solver.reputation.is_flagged
                    && solver
                        .capabilities
                        .allowed_intent_classes
                        .iter()
                        .any(|c| c.as_str() == class)
            })
            .collect();

        // Sort by solver_id for determinism (BTreeMap already gives sorted order,
        // but be explicit in case the API ever changes).
        eligible.sort_by_key(|s| s.solver_id);
        eligible
    }

    /// Map an `IntentType` to its canonical class string.
    fn intent_class(intent: &IntentTransaction) -> &'static str {
        match &intent.intent {
            IntentType::Transfer(_) => "transfer",
            IntentType::Swap(_) => "swap",
            IntentType::YieldAllocate(_) => "yield_allocate",
            IntentType::TreasuryRebalance(_) => "treasury_rebalance",
        }
    }
}
