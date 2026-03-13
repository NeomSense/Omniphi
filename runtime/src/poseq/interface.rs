use crate::attribution::record::{SelectionAuditRecord, SolverAttributionRecord};
use crate::capabilities::checker::CapabilitySet;
use crate::errors::RuntimeError;
use crate::GasCosts;
use crate::intents::base::{IntentTransaction, IntentType};
use crate::matching::matcher::IntentSolverMatcher;
use crate::objects::base::{AccessMode, BoxedObject, ObjectAccess, ObjectId};
use crate::plan_validation::validator::PlanValidator;
use crate::policy::hooks::PlanPolicyEvaluator;
use crate::resolution::planner::{ExecutionPlan, IntentResolver, ObjectOperation};
use crate::scheduler::parallel::ParallelScheduler;
use crate::selection::ranker::{SelectionPolicy, WinningPlanSelector};
use crate::settlement::engine::{SettlementEngine, SettlementResult};
use crate::solver_market::market::{CandidatePlan, PlanEvaluationResult};
use crate::solver_registry::registry::{SolverProfile, SolverRegistry};
use crate::state::store::ObjectStore;
use std::collections::BTreeMap;

/// Special solver ID reserved for the Phase 1 internal fallback resolver.
const INTERNAL_SOLVER_ID: [u8; 32] = [0xFF; 32];

/// An ordered batch of intent transactions delivered by the PoSeq sequencer.
/// Transactions are pre-ordered; the runtime must respect this ordering.
pub struct OrderedBatch {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub sequence_number: u64,
    /// Transactions already ordered by PoSeq canonical ordering.
    pub transactions: Vec<IntentTransaction>,
}

/// The top-level PoSeq runtime engine.
pub struct PoSeqRuntime {
    pub store: ObjectStore,
    _resolver: IntentResolver,
    _scheduler: ParallelScheduler,
    _settlement: SettlementEngine,
    pub current_epoch: u64,
}

impl PoSeqRuntime {
    /// Creates a new runtime with an empty object store.
    pub fn new() -> Self {
        PoSeqRuntime {
            store: ObjectStore::new(),
            _resolver: IntentResolver,
            _scheduler: ParallelScheduler,
            _settlement: SettlementEngine,
            current_epoch: 0,
        }
    }

    /// Seeds an object into the store (genesis / testing).
    pub fn seed_object(&mut self, obj: BoxedObject) {
        self.store.insert(obj);
    }

    /// Processes a full ordered batch of intent transactions.
    ///
    /// 9-step lifecycle:
    /// 1.  Validate each intent structurally (IntentTransaction::validate).
    /// 2.  Resolve each intent to an ExecutionPlan (skip invalid).
    /// 3.  Build access map (embedded in ExecutionPlan).
    /// 4.  Schedule plans with ParallelScheduler.
    /// 5.  Execute groups with SettlementEngine.
    /// 6.  Advance epoch.
    /// 7.  Sync typed overlays → canonical store.
    /// 8.  Compute state root.
    /// 9.  Return SettlementResult.
    pub fn process_batch(
        &mut self,
        batch: OrderedBatch,
    ) -> Result<SettlementResult, RuntimeError> {
        // Advance epoch to match the batch
        self.current_epoch = batch.epoch;

        // ── Step 1: structural validation ──────────────────────────────────
        let mut valid_txns: Vec<IntentTransaction> = Vec::new();
        for tx in batch.transactions {
            match tx.validate() {
                Ok(()) => valid_txns.push(tx),
                Err(e) => {
                    // Log and skip; do not abort the whole batch
                    // In a production system this would emit a structured event
                    let _ = e; // suppress unused warning
                }
            }
        }

        // ── Step 2: resolve each intent → ExecutionPlan ────────────────────
        // Use admin caps by default; real system would look up per-sender caps
        let caps = CapabilitySet::all();
        let mut plans = Vec::new();

        for tx in &valid_txns {
            match IntentResolver::resolve(tx, &self.store, &caps) {
                Ok(plan) => plans.push(plan),
                Err(_e) => {
                    // Resolution failure: skip this tx, emit failed receipt
                    // (SettlementResult will show it as failed)
                }
            }
        }

        // ── Steps 3-4: access map is embedded in ExecutionPlan; schedule ────
        let groups = ParallelScheduler::schedule(plans);

        // ── Step 5: execute groups ───────────────────────────────────────────
        let result = SettlementEngine::execute_groups(groups, &mut self.store, batch.epoch);

        // ── Steps 6-9: epoch advance + sync + root already done inside ───────
        Ok(result)
    }
}

impl Default for PoSeqRuntime {
    fn default() -> Self {
        Self::new()
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 2: Solver Market types
// ─────────────────────────────────────────────────────────────────────────────

/// A batch that includes both intents and pre-submitted solver plans.
/// Phase 2 upgrade over simple `OrderedBatch`.
pub struct SolverMarketBatch {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub sequence_number: u64,
    pub intents: Vec<IntentTransaction>,
    /// intent_id → candidate plans submitted by solvers for that intent.
    pub candidate_plans: BTreeMap<[u8; 32], Vec<CandidatePlan>>,
}

/// The winning plan and all audit metadata for a single intent.
pub struct SelectedPlanResult {
    pub intent_id: [u8; 32],
    pub winning_plan: CandidatePlan,
    pub evaluation: PlanEvaluationResult,
    pub attribution: SolverAttributionRecord,
    pub audit: SelectionAuditRecord,
}

/// The settled outcome of an entire `SolverMarketBatch`.
pub struct FinalSettlement {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub selected_plans: Vec<SelectedPlanResult>,
    pub settlement_result: SettlementResult,
}

/// Phase 2 runtime that extends `PoSeqRuntime` with solver market support.
pub struct SolverMarketRuntime {
    pub base: PoSeqRuntime,
    pub registry: SolverRegistry,
    pub selection_policy: SelectionPolicy,
    pub policy: Box<dyn PlanPolicyEvaluator>,
}

impl SolverMarketRuntime {
    pub fn new(
        selection_policy: SelectionPolicy,
        policy: Box<dyn PlanPolicyEvaluator>,
    ) -> Self {
        SolverMarketRuntime {
            base: PoSeqRuntime::new(),
            registry: SolverRegistry::new(),
            selection_policy,
            policy,
        }
    }

    /// Register a solver with the market.
    pub fn register_solver(&mut self, profile: SolverProfile) -> Result<(), RuntimeError> {
        self.registry.register(profile)
    }

    /// Full Phase 2 lifecycle: validate all candidate plans, rank, select winners, execute.
    pub fn process_solver_batch(
        &mut self,
        batch: SolverMarketBatch,
    ) -> Result<FinalSettlement, RuntimeError> {
        let epoch = batch.epoch;
        self.base.current_epoch = epoch;

        let mut selected_plans: Vec<SelectedPlanResult> = Vec::new();
        let mut execution_plans: Vec<ExecutionPlan> = Vec::new();

        // Process each intent in canonical order
        for intent in &batch.intents {
            // Validate intent structure; skip malformed intents
            if intent.validate().is_err() {
                continue;
            }

            // Collect candidate plans for this intent
            let intent_id = intent.tx_id;
            let candidates = batch
                .candidate_plans
                .get(&intent_id)
                .cloned()
                .unwrap_or_default();

            if candidates.is_empty() {
                // No candidates: fall back to Phase 1 internal resolver
                match self.fallback_to_internal_resolver(intent, epoch) {
                    Ok(result) => {
                        let exec_plan = candidate_plan_to_execution_plan(&result.winning_plan);
                        execution_plans.push(exec_plan);
                        selected_plans.push(result);
                    }
                    Err(_) => {
                        // Resolution failure: skip this intent
                    }
                }
                continue;
            }

            // Validate each candidate plan
            let mut eval_results: Vec<(CandidatePlan, PlanEvaluationResult)> = Vec::new();
            let mut all_plan_ids: Vec<[u8; 32]> = Vec::new();
            let mut all_evals: Vec<PlanEvaluationResult> = Vec::new();

            for plan in &candidates {
                all_plan_ids.push(plan.plan_id);
                let eval = PlanValidator::validate(
                    plan,
                    intent,
                    &self.base.store,
                    &self.registry,
                    self.policy.as_ref(),
                );

                // Update registry reputation for non-accepted plans now;
                // accepted plans are recorded after winner selection (is_winner determined there).
                let accepted = eval.passed;
                if !accepted {
                    self.registry.record_submission(&plan.solver_id, false, false);
                }
                // accepted-but-not-winner plans are recorded after selection below

                all_evals.push(eval.clone());
                if eval.passed {
                    eval_results.push((plan.clone(), eval));
                }
            }

            // If no valid plans, fall back to Phase 1
            if eval_results.is_empty() {
                match self.fallback_to_internal_resolver(intent, epoch) {
                    Ok(result) => {
                        let exec_plan = candidate_plan_to_execution_plan(&result.winning_plan);
                        execution_plans.push(exec_plan);
                        selected_plans.push(result);
                    }
                    Err(_) => {}
                }
                continue;
            }

            // Select winner
            let winner_idx = WinningPlanSelector::select(
                &eval_results,
                self.selection_policy.clone(),
            )?;

            // Record reputation for all valid (accepted) plans exactly once.
            // Non-accepted plans were already recorded above. Winner gets is_winner=true;
            // other valid plans get is_winner=false. This prevents double-counting.
            for (i, (plan, _eval)) in eval_results.iter().enumerate() {
                self.registry
                    .record_submission(&plan.solver_id, true, i == winner_idx);
            }

            let (winning_plan, winning_eval) = eval_results[winner_idx].clone();
            let all_evaluations_count = all_evals.len();
            let valid_count = eval_results.len();
            let rejected_count = all_evaluations_count - valid_count;

            // Build ranking (indices into all_evals for the valid subset, mapped back)
            let ranking = (0..eval_results.len()).collect::<Vec<usize>>();

            let selection_policy_str = format!("{:?}", self.selection_policy);
            let selection_basis = format!(
                "score:{} fee:{} solver:{}",
                winning_eval.normalized_score,
                winning_plan.fee_quote,
                hex::encode(&winning_plan.solver_id[..4])
            );

            let attribution = SolverAttributionRecord {
                intent_id,
                winning_solver_id: winning_plan.solver_id,
                winning_plan_id: winning_plan.plan_id,
                winning_plan_hash: winning_plan.plan_hash,
                total_candidates: all_evaluations_count,
                valid_candidates: valid_count,
                rejected_candidates: rejected_count,
                selection_policy: selection_policy_str.clone(),
                selection_basis,
                epoch,
            };

            let audit = SelectionAuditRecord {
                intent_id,
                all_plan_ids,
                evaluation_results: all_evals,
                ranking,
                winner_index: winner_idx,
                policy_used: selection_policy_str,
            };

            // Convert winning CandidatePlan → ExecutionPlan
            let exec_plan = candidate_plan_to_execution_plan(&winning_plan);
            execution_plans.push(exec_plan);

            selected_plans.push(SelectedPlanResult {
                intent_id,
                winning_plan,
                evaluation: winning_eval,
                attribution,
                audit,
            });
        }

        // Schedule and settle all winning execution plans
        let groups = ParallelScheduler::schedule(execution_plans);
        let settlement_result =
            SettlementEngine::execute_groups(groups, &mut self.base.store, epoch);

        Ok(FinalSettlement {
            batch_id: batch.batch_id,
            epoch,
            selected_plans,
            settlement_result,
        })
    }

    /// Fall back to Phase 1 internal resolver for an intent with no solver candidates.
    fn fallback_to_internal_resolver(
        &mut self,
        intent: &IntentTransaction,
        epoch: u64,
    ) -> Result<SelectedPlanResult, RuntimeError> {
        let caps = CapabilitySet::all();
        let exec_plan = IntentResolver::resolve(intent, &self.base.store, &caps)?;

        // Construct a synthetic CandidatePlan representing the internally resolved plan
        let synthetic_solver_id = INTERNAL_SOLVER_ID;
        let synthetic_plan_id = intent.tx_id; // reuse tx_id as plan_id

        let actions = exec_plan
            .operations
            .iter()
            .map(|op| operation_to_plan_action(op))
            .collect::<Vec<_>>();

        let object_reads: Vec<ObjectId> = exec_plan
            .object_access
            .iter()
            .filter(|a| a.mode == AccessMode::ReadOnly)
            .map(|a| a.object_id)
            .collect();
        let object_writes: Vec<ObjectId> = exec_plan
            .object_access
            .iter()
            .filter(|a| a.mode == AccessMode::ReadWrite)
            .map(|a| a.object_id)
            .collect();

        let mut synthetic_plan = CandidatePlan {
            plan_id: synthetic_plan_id,
            intent_id: intent.tx_id,
            solver_id: synthetic_solver_id,
            actions,
            required_capabilities: exec_plan.required_capabilities.clone(),
            object_reads: object_reads.clone(),
            object_writes: object_writes.clone(),
            expected_output_amount: 1, // non-zero; actual output is in operations
            fee_quote: intent.max_fee,
            quality_score: 5000,
            constraint_proofs: vec![],
            plan_hash: [0u8; 32],
            submitted_at_sequence: 0,
            explanation: Some("Phase 1 internal resolver fallback".to_string()),
            metadata: BTreeMap::new(),
        };
        // Compute and set hash
        let hash = synthetic_plan.compute_hash();
        synthetic_plan.plan_hash = hash;

        let eval = PlanEvaluationResult {
            plan_id: synthetic_plan_id,
            solver_id: synthetic_solver_id,
            passed: true,
            reason_codes: vec![crate::plan_validation::validator::ValidationReasonCode::Valid],
            normalized_score: 5000,
            validated_object_reads: object_reads,
            validated_object_writes: object_writes,
            settlement_footprint: exec_plan.gas_estimate,
            validated_output_amount: 1,
        };

        let intent_id = intent.tx_id;
        let policy_str = "FallbackPhase1".to_string();

        let attribution = SolverAttributionRecord {
            intent_id,
            winning_solver_id: synthetic_solver_id,
            winning_plan_id: synthetic_plan_id,
            winning_plan_hash: synthetic_plan.plan_hash,
            total_candidates: 0,
            valid_candidates: 1,
            rejected_candidates: 0,
            selection_policy: policy_str.clone(),
            selection_basis: "fallback:internal_resolver".to_string(),
            epoch,
        };

        let audit = SelectionAuditRecord {
            intent_id,
            all_plan_ids: vec![synthetic_plan_id],
            evaluation_results: vec![eval.clone()],
            ranking: vec![0],
            winner_index: 0,
            policy_used: policy_str,
        };

        Ok(SelectedPlanResult {
            intent_id,
            winning_plan: synthetic_plan,
            evaluation: eval,
            attribution,
            audit,
        })
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Conversion helpers
// ─────────────────────────────────────────────────────────────────────────────

/// Convert a `CandidatePlan` to an `ExecutionPlan` for the scheduler.
fn candidate_plan_to_execution_plan(plan: &CandidatePlan) -> ExecutionPlan {
    use crate::objects::base::AccessMode;
    use crate::resolution::planner::ObjectOperation;

    let operations: Vec<ObjectOperation> = plan
        .actions
        .iter()
        .map(|a| plan_action_to_operation(a))
        .collect();

    // Build object access list from reads + writes
    let mut object_access: Vec<ObjectAccess> = Vec::new();
    for id in &plan.object_reads {
        object_access.push(ObjectAccess {
            object_id: *id,
            mode: AccessMode::ReadOnly,
        });
    }
    for id in &plan.object_writes {
        object_access.push(ObjectAccess {
            object_id: *id,
            mode: AccessMode::ReadWrite,
        });
    }

    let costs = GasCosts::default_costs();
    let gas_limit = (plan.fee_quote as u64).saturating_mul(1_000);
    // Use base_tx cost from shared config instead of magic number 1000
    let gas_estimate = operations.len() as u64 * 300 + costs.base_tx;

    ExecutionPlan {
        tx_id: plan.plan_id,
        operations,
        required_capabilities: plan.required_capabilities.clone(),
        object_access,
        gas_estimate,
        gas_limit,
    }
}

/// Convert a `CandidatePlan` `PlanAction` to an `ObjectOperation`.
fn plan_action_to_operation(
    action: &crate::solver_market::market::PlanAction,
) -> ObjectOperation {
    use crate::solver_market::market::PlanActionType;
    match &action.action_type {
        PlanActionType::DebitBalance => ObjectOperation::DebitBalance {
            balance_id: action.target_object,
            amount: action.amount.unwrap_or(0),
        },
        PlanActionType::CreditBalance => ObjectOperation::CreditBalance {
            balance_id: action.target_object,
            amount: action.amount.unwrap_or(0),
        },
        PlanActionType::SwapPoolAmounts => ObjectOperation::SwapPoolAmounts {
            pool_id: action.target_object,
            delta_a: action.amount.unwrap_or(0) as i128,
            delta_b: 0,
        },
        PlanActionType::LockBalance => ObjectOperation::LockBalance {
            balance_id: action.target_object,
            amount: action.amount.unwrap_or(0),
        },
        PlanActionType::UnlockBalance => ObjectOperation::UnlockBalance {
            balance_id: action.target_object,
            amount: action.amount.unwrap_or(0),
        },
        PlanActionType::Custom(_) => ObjectOperation::UpdateVersion {
            object_id: action.target_object,
        },
    }
}

/// Convert an `ObjectOperation` to a `PlanAction`.
fn operation_to_plan_action(
    op: &ObjectOperation,
) -> crate::solver_market::market::PlanAction {
    use crate::solver_market::market::{PlanAction, PlanActionType};
    use std::collections::BTreeMap;

    match op {
        ObjectOperation::DebitBalance { balance_id, amount } => PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: *balance_id,
            amount: Some(*amount),
            metadata: BTreeMap::new(),
        },
        ObjectOperation::CreditBalance { balance_id, amount } => PlanAction {
            action_type: PlanActionType::CreditBalance,
            target_object: *balance_id,
            amount: Some(*amount),
            metadata: BTreeMap::new(),
        },
        ObjectOperation::SwapPoolAmounts { pool_id, delta_a, delta_b: _ } => PlanAction {
            action_type: PlanActionType::SwapPoolAmounts,
            target_object: *pool_id,
            amount: Some(*delta_a as u128),
            metadata: BTreeMap::new(),
        },
        ObjectOperation::LockBalance { balance_id, amount } => PlanAction {
            action_type: PlanActionType::LockBalance,
            target_object: *balance_id,
            amount: Some(*amount),
            metadata: BTreeMap::new(),
        },
        ObjectOperation::UnlockBalance { balance_id, amount } => PlanAction {
            action_type: PlanActionType::UnlockBalance,
            target_object: *balance_id,
            amount: Some(*amount),
            metadata: BTreeMap::new(),
        },
        ObjectOperation::UpdateVersion { object_id } => PlanAction {
            action_type: PlanActionType::Custom("update_version".to_string()),
            target_object: *object_id,
            amount: None,
            metadata: BTreeMap::new(),
        },
    }
}
