use crate::crx::causal_graph::CausalGraph;
use crate::crx::finality::DomainFinalityPolicy;
use crate::crx::goal_packet::{goal_packet_from_intent, GoalPacket};
use crate::crx::plan_builder::CausalPlanBuilder;
use crate::crx::rights_capsule::RightsCapsule;
use crate::crx::settlement::{CRXSettlementEngine, CRXSettlementRecord};
use crate::errors::RuntimeError;
use crate::intents::base::{IntentTransaction, IntentType};
use crate::intents::types::TransferIntent;
use crate::objects::base::BoxedObject;
use crate::plan_validation::validator::PlanValidator;
use crate::policy::hooks::PlanPolicyEvaluator;
use crate::selection::ranker::{SelectionPolicy, WinningPlanSelector};
use crate::solver_market::market::{CandidatePlan, PlanEvaluationResult};
use crate::solver_registry::registry::{SolverProfile, SolverRegistry};
use crate::state::store::ObjectStore;
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Batch types
// ─────────────────────────────────────────────────────────────────────────────

pub struct OrderedGoalBatch {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub sequence_number: u64,
    pub goals: Vec<GoalPacket>,
    /// goal_packet_id → candidate plans submitted by solvers
    pub candidate_plans: BTreeMap<[u8; 32], Vec<CandidatePlan>>,
}

pub struct SelectedCausalPlan {
    pub goal_packet_id: [u8; 32],
    pub plan: CandidatePlan,
    pub graph: CausalGraph,
    pub capsule: RightsCapsule,
}

#[derive(Debug, Clone)]
pub struct CRXExecutionResult {
    pub goal_packet_id: [u8; 32],
    pub record: CRXSettlementRecord,
    pub success: bool,
}

// ─────────────────────────────────────────────────────────────────────────────
// PoSeqCRXBridge
// ─────────────────────────────────────────────────────────────────────────────

pub struct PoSeqCRXBridge {
    pub store: ObjectStore,
    pub registry: SolverRegistry,
    pub selection_policy: SelectionPolicy,
    pub policy_evaluator: Box<dyn PlanPolicyEvaluator>,
    pub domain_policies: Vec<DomainFinalityPolicy>,
    pub current_epoch: u64,
}

impl PoSeqCRXBridge {
    pub fn new(
        selection_policy: SelectionPolicy,
        policy_evaluator: Box<dyn PlanPolicyEvaluator>,
    ) -> Self {
        PoSeqCRXBridge {
            store: ObjectStore::new(),
            registry: SolverRegistry::new(),
            selection_policy,
            policy_evaluator,
            domain_policies: vec![],
            current_epoch: 0,
        }
    }

    pub fn register_solver(&mut self, profile: SolverProfile) -> Result<(), RuntimeError> {
        self.registry.register(profile)
    }

    pub fn seed_object(&mut self, obj: BoxedObject) {
        self.store.insert(obj);
    }

    /// Full CRX lifecycle for a PoSeq-ordered batch.
    pub fn process_batch(&mut self, batch: OrderedGoalBatch) -> Vec<CRXExecutionResult> {
        self.current_epoch = batch.epoch;
        let mut results: Vec<CRXExecutionResult> = Vec::new();

        for goal in &batch.goals {
            // Step 1: validate goal
            if let Err(_e) = goal.validate() {
                continue;
            }

            // Step 2: collect candidates for this goal
            let candidates = batch
                .candidate_plans
                .get(&goal.packet_id)
                .cloned()
                .unwrap_or_default();

            // Step 3: build a synthetic IntentTransaction from the GoalPacket for matching/validation
            let fake_intent = Self::intent_from_goal(goal);

            // Step 4: validate each candidate plan
            let mut valid_plans: Vec<(CandidatePlan, PlanEvaluationResult)> = Vec::new();

            for plan in &candidates {
                let eval = PlanValidator::validate(
                    plan,
                    &fake_intent,
                    &self.store,
                    &self.registry,
                    self.policy_evaluator.as_ref(),
                );
                if eval.passed {
                    valid_plans.push((plan.clone(), eval));
                }
            }

            // Step 5: select winner (or fall back to first valid candidate)
            let winning_plan = if valid_plans.is_empty() {
                // No valid solver candidates — try to build synthetic fallback plan
                match Self::build_synthetic_plan(goal) {
                    Some(p) => p,
                    None => continue, // skip goal if no plan possible
                }
            } else {
                match WinningPlanSelector::select(&valid_plans, self.selection_policy.clone()) {
                    Ok(idx) => valid_plans[idx].0.clone(),
                    Err(_) => continue,
                }
            };

            // Step 6: run CRX settlement
            match CRXSettlementEngine::settle(
                &winning_plan,
                goal,
                &mut self.store,
                &self.domain_policies,
                self.current_epoch,
            ) {
                Ok(record) => {
                    let success = matches!(
                        record.settlement_class,
                        crate::crx::branch_execution::ExecutionSettlementClass::FullSuccess
                            | crate::crx::branch_execution::ExecutionSettlementClass::SuccessWithDowngrade(_)
                            | crate::crx::branch_execution::ExecutionSettlementClass::SuccessWithQuarantine(_)
                            | crate::crx::branch_execution::ExecutionSettlementClass::PartialSuccess { .. }
                    );
                    results.push(CRXExecutionResult {
                        goal_packet_id: goal.packet_id,
                        record,
                        success,
                    });
                }
                Err(_) => {
                    // Settlement error: skip
                    continue;
                }
            }
        }

        results
    }

    /// Build a fake IntentTransaction from a GoalPacket for use with PlanValidator.
    fn intent_from_goal(goal: &GoalPacket) -> IntentTransaction {
        IntentTransaction {
            tx_id: goal.intent_id,
            sender: goal.sender,
            intent: IntentType::Transfer(TransferIntent {
                asset_id: [1u8; 32],
                amount: goal.constraints.min_output_amount.max(1) as u128,
                recipient: goal.sender,
                memo: Some(goal.desired_outcome.clone()),
            }),
            max_fee: goal.max_fee,
            deadline_epoch: goal.deadline_epoch,
            nonce: goal.nonce,
            signature: [0u8; 64],
            metadata: goal.metadata.clone(),
        }
    }

    /// Build a minimal synthetic CandidatePlan when no solver candidates exist.
    fn build_synthetic_plan(goal: &GoalPacket) -> Option<CandidatePlan> {
        use crate::solver_market::market::{PlanAction, PlanActionType};
        use sha2::{Digest, Sha256};
        use std::collections::BTreeMap;

        // A trivial plan with no actions (will produce an empty graph)
        // This is only for tests/edge cases; in production a real resolver would be called
        let mut plan = CandidatePlan {
            plan_id: goal.packet_id,
            intent_id: goal.intent_id,
            solver_id: [0xFEu8; 32],
            actions: vec![],
            required_capabilities: vec![],
            object_reads: vec![],
            object_writes: vec![],
            expected_output_amount: 0,
            fee_quote: goal.max_fee,
            quality_score: 1000,
            constraint_proofs: vec![],
            plan_hash: [0u8; 32],
            submitted_at_sequence: 0,
            explanation: Some("synthetic fallback plan".to_string()),
            metadata: BTreeMap::new(),
        };
        let hash = plan.compute_hash();
        plan.plan_hash = hash;
        Some(plan)
    }
}
