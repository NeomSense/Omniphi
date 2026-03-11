use crate::intents::base::IntentTransaction;
use crate::objects::base::ObjectId;
use crate::objects::types::BalanceObject;
use crate::plan_validation::validator::{PlanValidator, ValidationReasonCode};
use crate::policy::hooks::PlanPolicyEvaluator;
use crate::solver_market::market::{CandidatePlan, PlanActionType};
use crate::solver_registry::registry::SolverRegistry;
use crate::state::store::ObjectStore;
use std::collections::BTreeMap;

/// The result of a dry-run simulation for a candidate plan.
#[derive(Debug, Clone)]
pub struct SimulationResult {
    pub plan_id: [u8; 32],
    pub would_pass_validation: bool,
    pub validation_reason_codes: Vec<ValidationReasonCode>,
    /// Predicted balance changes: (ObjectId, delta) where delta < 0 means debit.
    pub preview_balance_changes: Vec<(ObjectId, i128)>,
    /// Predicted version increments: (ObjectId, new_version).
    pub preview_new_versions: Vec<(ObjectId, u64)>,
    pub estimated_gas: u64,
    pub estimated_output: u128,
    pub normalized_score: u64,
}

/// Dry-runs a candidate plan without mutating real state.
pub struct PlanSimulator;

impl PlanSimulator {
    /// Simulate a candidate plan.
    ///
    /// Clones the subset of the store referenced by the plan, runs the same
    /// validation logic as production, applies mutations to the clone, and
    /// returns a `SimulationResult`.
    ///
    /// The real store is never mutated.
    pub fn simulate(
        plan: &CandidatePlan,
        intent: &IntentTransaction,
        store: &ObjectStore,
        registry: &SolverRegistry,
        policy: &dyn PlanPolicyEvaluator,
    ) -> SimulationResult {
        // Run the same validation code path as production
        let eval = PlanValidator::validate(plan, intent, store, registry, policy);

        // Compute preview balance changes and version bumps by simulating actions
        // on a snapshot of referenced balance objects.
        let mut balance_snapshot: BTreeMap<ObjectId, u128> = BTreeMap::new();
        let mut balance_versions: BTreeMap<ObjectId, u64> = BTreeMap::new();

        // Snapshot all referenced balance objects
        for id in plan.object_reads.iter().chain(plan.object_writes.iter()) {
            if let Some(balance) = store.get_balance_by_id(id) {
                balance_snapshot.insert(*id, balance.amount);
                balance_versions.insert(*id, balance.meta.version);
            } else if let Some(obj) = store.get(id) {
                // Non-balance object — just snapshot version
                balance_versions.insert(*id, obj.meta().version);
            }
        }

        // Apply plan actions to the snapshot
        let mut preview_balance_changes: Vec<(ObjectId, i128)> = Vec::new();
        let mut affected_ids: Vec<ObjectId> = Vec::new();

        if eval.passed {
            for action in &plan.actions {
                let id = action.target_object;
                let amount = action.amount.unwrap_or(0);
                match &action.action_type {
                    PlanActionType::DebitBalance => {
                        if let Some(bal) = balance_snapshot.get_mut(&id) {
                            *bal = bal.saturating_sub(amount);
                            preview_balance_changes.push((id, -(amount as i128)));
                            if !affected_ids.contains(&id) {
                                affected_ids.push(id);
                            }
                        }
                    }
                    PlanActionType::CreditBalance => {
                        if let Some(bal) = balance_snapshot.get_mut(&id) {
                            *bal = bal.saturating_add(amount);
                            preview_balance_changes.push((id, amount as i128));
                            if !affected_ids.contains(&id) {
                                affected_ids.push(id);
                            }
                        }
                    }
                    PlanActionType::LockBalance | PlanActionType::UnlockBalance => {
                        // Balance amount unchanged for lock/unlock, but version bumps
                        if !affected_ids.contains(&id) {
                            affected_ids.push(id);
                        }
                    }
                    PlanActionType::SwapPoolAmounts => {
                        if !affected_ids.contains(&id) {
                            affected_ids.push(id);
                        }
                    }
                    PlanActionType::Custom(_) => {
                        if !affected_ids.contains(&id) {
                            affected_ids.push(id);
                        }
                    }
                }
            }
        }

        // Compute preview new versions (increment by 1 for each affected object)
        let mut preview_new_versions: Vec<(ObjectId, u64)> = Vec::new();
        for id in &affected_ids {
            let old_version = balance_versions.get(id).copied().unwrap_or(0);
            preview_new_versions.push((*id, old_version + 1));
        }

        // Estimate gas: base + per-object cost
        let estimated_gas = 1_000u64
            + plan.actions.len() as u64 * 300
            + plan.object_reads.len() as u64 * 100
            + plan.object_writes.len() as u64 * 500;

        SimulationResult {
            plan_id: plan.plan_id,
            would_pass_validation: eval.passed,
            validation_reason_codes: eval.reason_codes,
            preview_balance_changes,
            preview_new_versions,
            estimated_gas,
            estimated_output: plan.expected_output_amount,
            normalized_score: eval.normalized_score,
        }
    }
}
