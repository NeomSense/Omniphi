use crate::capabilities::checker::Capability;
use crate::errors::RuntimeError;
use crate::intents::base::IntentTransaction;
use crate::objects::base::ObjectId;
use crate::policy::hooks::PlanPolicyEvaluator;
use crate::solver_market::market::{CandidatePlan, PlanActionType, PlanEvaluationResult};
use crate::solver_registry::registry::{SolverProfile, SolverRegistry, SolverStatus};
use crate::state::store::ObjectStore;

/// Reason codes produced by plan validation.
/// A passing plan will only carry `Valid`; a failing plan may carry multiple.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValidationReasonCode {
    Valid,
    SolverNotActive,
    SolverCapabilityInsufficient,
    ObjectNotFound,
    InsufficientBalance,
    SlippageExceeded,
    ConstraintViolated,
    CapabilityMissing,
    DisallowedObject,
    PlanHashMismatch,
    MaxObjectsExceeded,
    PolicyRejected,
    FeeTooHigh,
    ZeroOutput,
    /// Contract constraint validator rejected the proposed state transition.
    ContractConstraintRejected,
    /// Referenced contract schema not found in registry.
    ContractSchemaNotFound,
}

pub struct PlanValidator;

impl PlanValidator {
    /// Validate a candidate plan against current state, the original intent,
    /// the solver registry, and a policy evaluator.
    ///
    /// Returns a `PlanEvaluationResult` that may have `passed = true` or
    /// `passed = false` depending on whether all checks succeed.
    pub fn validate(
        plan: &CandidatePlan,
        intent: &IntentTransaction,
        store: &ObjectStore,
        registry: &SolverRegistry,
        policy: &dyn PlanPolicyEvaluator,
    ) -> PlanEvaluationResult {
        let mut reason_codes: Vec<ValidationReasonCode> = Vec::new();

        // Step 1: Verify plan hash matches
        if let Some(code) = Self::check_plan_hash(plan) {
            reason_codes.push(code);
        }

        // Step 2: Lookup solver and check status == Active
        let solver_opt = registry.get(&plan.solver_id);
        if let Some(code) = Self::check_solver_status(solver_opt) {
            reason_codes.push(code);
        }

        // Step 3: Check solver capabilities cover required_capabilities
        if let Some(solver) = solver_opt {
            if let Some(code) =
                Self::check_solver_eligibility(solver, &plan.required_capabilities)
            {
                reason_codes.push(code);
            }

            // Step 4: Check max_objects_per_plan
            if let Some(code) = Self::check_object_count(plan, solver) {
                reason_codes.push(code);
            }
        }

        // Step 5: Check all object_reads exist in store
        if let Some(code) = Self::check_objects_exist(plan, store) {
            reason_codes.push(code);
        }

        // Step 6: Check balance sufficiency for debit actions
        if let Some(code) = Self::check_balance_sufficiency(plan, store) {
            reason_codes.push(code);
        }

        // Step 7: Check policy
        if let Some(rejection) = policy.evaluate(plan, intent) {
            let _ = rejection; // reason embedded in reason code
            reason_codes.push(ValidationReasonCode::PolicyRejected);
        }

        // Step 8: Check fee_quote <= intent.max_fee
        if plan.fee_quote > intent.max_fee {
            reason_codes.push(ValidationReasonCode::FeeTooHigh);
        }

        // Step 9: Check expected_output_amount > 0
        if plan.expected_output_amount == 0 {
            reason_codes.push(ValidationReasonCode::ZeroOutput);
        }

        // Step 10 (OIC): Validate contract actions via constraint validator.
        // For each PlanActionType::Custom that targets a contract object,
        // verify the action metadata contains a valid "schema_id" and
        // "proposed_state". Full constraint validation (Wasm invocation)
        // happens at the CRX execution layer; here we do structural checks.
        for action in &plan.actions {
            if let PlanActionType::Custom(method) = &action.action_type {
                if let Some(schema_hex) = action.metadata.get("schema_id") {
                    // Structural check: schema_id must be valid 64-char hex
                    if schema_hex.len() != 64 || hex::decode(schema_hex).is_err() {
                        reason_codes.push(ValidationReasonCode::ContractSchemaNotFound);
                    }
                    // Structural check: proposed_state must be present
                    if !action.metadata.contains_key("proposed_state") {
                        reason_codes.push(ValidationReasonCode::ContractConstraintRejected);
                    }
                    let _ = method; // method is used in CRX execution
                }
            }
        }

        let passed = reason_codes.is_empty();

        // Step 10: Compute normalized score if all checks passed
        let normalized_score = if passed {
            Self::compute_score(plan)
        } else {
            0
        };

        // Add Valid reason code if everything passed
        if passed {
            reason_codes.push(ValidationReasonCode::Valid);
        }

        let settlement_footprint = plan.actions.len() as u64 * 200
            + plan.object_writes.len() as u64 * 500
            + plan.object_reads.len() as u64 * 100;

        PlanEvaluationResult {
            plan_id: plan.plan_id,
            solver_id: plan.solver_id,
            passed,
            reason_codes,
            normalized_score,
            validated_object_reads: plan.object_reads.clone(),
            validated_object_writes: plan.object_writes.clone(),
            settlement_footprint,
            validated_output_amount: plan.expected_output_amount,
        }
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Private validation helpers
    // ─────────────────────────────────────────────────────────────────────────

    /// Check solver is registered and Active.
    fn check_solver_status(
        solver_opt: Option<&SolverProfile>,
    ) -> Option<ValidationReasonCode> {
        match solver_opt {
            None => Some(ValidationReasonCode::SolverNotActive),
            Some(solver) => {
                if solver.status != SolverStatus::Active {
                    Some(ValidationReasonCode::SolverNotActive)
                } else {
                    None
                }
            }
        }
    }

    /// Check solver is Active and has all required capabilities.
    fn check_solver_eligibility(
        solver: &SolverProfile,
        required_caps: &[Capability],
    ) -> Option<ValidationReasonCode> {
        for cap in required_caps {
            if !solver.capabilities.capability_set.contains(cap) {
                return Some(ValidationReasonCode::SolverCapabilityInsufficient);
            }
        }
        None
    }

    /// Verify all read objects exist in the store.
    fn check_objects_exist(
        plan: &CandidatePlan,
        store: &ObjectStore,
    ) -> Option<ValidationReasonCode> {
        for id in &plan.object_reads {
            if store.get(id).is_none() {
                return Some(ValidationReasonCode::ObjectNotFound);
            }
        }
        // Also check write objects
        for id in &plan.object_writes {
            if store.get(id).is_none() {
                return Some(ValidationReasonCode::ObjectNotFound);
            }
        }
        None
    }

    /// Verify balance objects have sufficient available balance for DebitBalance actions.
    fn check_balance_sufficiency(
        plan: &CandidatePlan,
        store: &ObjectStore,
    ) -> Option<ValidationReasonCode> {
        for action in &plan.actions {
            if action.action_type == PlanActionType::DebitBalance {
                if let Some(amount) = action.amount {
                    if let Some(balance) = store.get_balance_by_id(&action.target_object) {
                        if balance.available() < amount {
                            return Some(ValidationReasonCode::InsufficientBalance);
                        }
                    } else {
                        // Object not found for debit — flag as insufficient
                        return Some(ValidationReasonCode::InsufficientBalance);
                    }
                }
            }
        }
        None
    }

    /// Verify plan hash matches the recomputed hash.
    fn check_plan_hash(plan: &CandidatePlan) -> Option<ValidationReasonCode> {
        if !plan.validate_hash() {
            Some(ValidationReasonCode::PlanHashMismatch)
        } else {
            None
        }
    }

    /// Check max objects per plan limit from solver capabilities.
    fn check_object_count(
        plan: &CandidatePlan,
        solver: &SolverProfile,
    ) -> Option<ValidationReasonCode> {
        let total_objects = plan.object_reads.len() + plan.object_writes.len();
        if total_objects > solver.capabilities.max_objects_per_plan {
            Some(ValidationReasonCode::MaxObjectsExceeded)
        } else {
            None
        }
    }

    /// Compute a composite normalized score (0–10000 bps).
    ///
    /// Scoring weights:
    ///   - quality_score (solver self-reported): 60% weight → 0–6000 pts
    ///   - fee savings (lower fee = higher pts): 30% weight → 0–3000 pts
    ///     (fee savings relative to a 10000-unit reference)
    ///   - constraint satisfaction ratio: 10% weight → 0–1000 pts
    fn compute_score(plan: &CandidatePlan) -> u64 {
        // Quality component: 60% of 10000 = 6000
        let quality_component = (plan.quality_score.min(10_000) * 6000) / 10_000;

        // Fee component: lower fee is better.
        // Map fee in range [0, 10000] inversely to [3000, 0] pts.
        let fee_normalized = plan.fee_quote.min(10_000);
        let fee_component = (10_000u64.saturating_sub(fee_normalized) * 3000) / 10_000;

        // Constraint component: ratio of satisfied constraints
        let constraint_component = if plan.constraint_proofs.is_empty() {
            // No constraints declared: award full constraint points
            1000
        } else {
            let satisfied = plan
                .constraint_proofs
                .iter()
                .filter(|p| p.declared_satisfied)
                .count() as u64;
            let total = plan.constraint_proofs.len() as u64;
            (satisfied * 1000) / total
        };

        (quality_component + fee_component + constraint_component).min(10_000)
    }
}
