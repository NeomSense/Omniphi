use crate::plan_validation::validator::ValidationReasonCode;
use crate::solver_market::market::PlanEvaluationResult;

/// Canonical record of who won an intent and why.
#[derive(Debug, Clone)]
pub struct SolverAttributionRecord {
    pub intent_id: [u8; 32],
    pub winning_solver_id: [u8; 32],
    pub winning_plan_id: [u8; 32],
    pub winning_plan_hash: [u8; 32],
    pub total_candidates: usize,
    pub valid_candidates: usize,
    pub rejected_candidates: usize,
    pub selection_policy: String,
    /// Human-readable basis, e.g. "highest_score:9500 fee:100"
    pub selection_basis: String,
    pub epoch: u64,
}

/// Full audit record for a single intent's solver selection round.
#[derive(Debug, Clone)]
pub struct SelectionAuditRecord {
    pub intent_id: [u8; 32],
    pub all_plan_ids: Vec<[u8; 32]>,
    pub evaluation_results: Vec<PlanEvaluationResult>,
    /// Indices into `evaluation_results` in ranked order (best first).
    pub ranking: Vec<usize>,
    pub winner_index: usize,
    pub policy_used: String,
}

/// Per-plan outcome record (used for solver reputation updates and accounting).
#[derive(Debug, Clone)]
pub struct PlanOutcomeRecord {
    pub plan_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub won: bool,
    pub validation_passed: bool,
    pub reason_codes: Vec<ValidationReasonCode>,
    /// `None` if not yet executed; `Some(true/false)` after settlement.
    pub execution_success: Option<bool>,
    pub quality_score: u64,
}
