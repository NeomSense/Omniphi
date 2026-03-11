use crate::errors::RuntimeError;
use crate::solver_market::market::{CandidatePlan, PlanEvaluationResult};

/// Policy governing how the winning plan is selected from valid candidates.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SelectionPolicy {
    /// Primary: highest normalized_score. Tie-break: lowest fee. Final: lexical solver_id.
    BestScore,
    /// Primary: lowest fee_quote. Tie-break: highest score. Final: lexical solver_id.
    LowestFee,
    /// Primary: highest expected_output_amount. Tie-break: lowest fee. Final: lexical solver_id.
    BestOutput,
}

/// Selects the single winning plan from a list of validated candidates.
pub struct WinningPlanSelector;

impl WinningPlanSelector {
    /// Given a list of (plan, evaluation) pairs where `evaluation.passed == true`,
    /// return the index of the winning plan according to `policy`.
    ///
    /// Returns `Err(RuntimeError::NoCandidatePlans)` if the slice is empty.
    ///
    /// Deterministic: identical inputs always produce the same winner.
    pub fn select(
        valid_plans: &[(CandidatePlan, PlanEvaluationResult)],
        policy: SelectionPolicy,
    ) -> Result<usize, RuntimeError> {
        if valid_plans.is_empty() {
            return Err(RuntimeError::NoCandidatePlans);
        }
        let ranked = PlanRanker::rank(valid_plans, policy);
        Ok(ranked[0])
    }
}

/// Ranks all valid plans according to a selection policy.
pub struct PlanRanker;

impl PlanRanker {
    /// Returns indices into `valid_plans` sorted best-first according to `policy`.
    ///
    /// Stable sort — identical inputs always produce the same ranking.
    pub fn rank(
        valid_plans: &[(CandidatePlan, PlanEvaluationResult)],
        policy: SelectionPolicy,
    ) -> Vec<usize> {
        let mut indices: Vec<usize> = (0..valid_plans.len()).collect();

        indices.sort_by(|&a, &b| {
            let (plan_a, eval_a) = &valid_plans[a];
            let (plan_b, eval_b) = &valid_plans[b];

            match policy {
                SelectionPolicy::BestScore => {
                    // 1. Higher normalized_score is better (descending)
                    let cmp = eval_b.normalized_score.cmp(&eval_a.normalized_score);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 2. Lower fee_quote is better (ascending)
                    let cmp = plan_a.fee_quote.cmp(&plan_b.fee_quote);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 3. Lexical solver_id ascending
                    plan_a.solver_id.cmp(&plan_b.solver_id)
                }
                SelectionPolicy::LowestFee => {
                    // 1. Lower fee_quote is better (ascending)
                    let cmp = plan_a.fee_quote.cmp(&plan_b.fee_quote);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 2. Higher normalized_score is better (descending)
                    let cmp = eval_b.normalized_score.cmp(&eval_a.normalized_score);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 3. Lexical solver_id ascending
                    plan_a.solver_id.cmp(&plan_b.solver_id)
                }
                SelectionPolicy::BestOutput => {
                    // 1. Higher expected_output_amount is better (descending)
                    let cmp = eval_b
                        .validated_output_amount
                        .cmp(&eval_a.validated_output_amount);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 2. Lower fee_quote is better (ascending)
                    let cmp = plan_a.fee_quote.cmp(&plan_b.fee_quote);
                    if cmp != std::cmp::Ordering::Equal {
                        return cmp;
                    }
                    // 3. Lexical solver_id ascending
                    plan_a.solver_id.cmp(&plan_b.solver_id)
                }
            }
        });

        indices
    }
}
