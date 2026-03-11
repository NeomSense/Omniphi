use omniphi_runtime::errors::RuntimeError;
use omniphi_runtime::objects::base::ObjectId;
use omniphi_runtime::plan_validation::validator::ValidationReasonCode;
use omniphi_runtime::selection::ranker::{PlanRanker, SelectionPolicy, WinningPlanSelector};
use omniphi_runtime::solver_market::market::{CandidatePlan, PlanConstraintProof, PlanEvaluationResult};
use std::collections::BTreeMap;

// ─── helpers ────────────────────────────────────────────────────────────────

fn addr(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0] = n;
    b
}

fn txid(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[31] = n;
    b
}

fn oid(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}

/// Build a minimal passing plan+eval pair with specified attributes.
fn make_pair(
    plan_id_byte: u8,
    solver_id_byte: u8,
    score: u64,
    fee: u64,
    output: u128,
) -> (CandidatePlan, PlanEvaluationResult) {
    let plan_id = txid(plan_id_byte);
    let solver_id = addr(solver_id_byte);

    let mut plan = CandidatePlan {
        plan_id,
        intent_id: txid(1),
        solver_id,
        actions: vec![],
        required_capabilities: vec![],
        object_reads: vec![],
        object_writes: vec![],
        expected_output_amount: output,
        fee_quote: fee,
        quality_score: score,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let hash = plan.compute_hash();
    plan.plan_hash = hash;

    let eval = PlanEvaluationResult {
        plan_id,
        solver_id,
        passed: true,
        reason_codes: vec![ValidationReasonCode::Valid],
        normalized_score: score,
        validated_object_reads: vec![],
        validated_object_writes: vec![],
        settlement_footprint: 100,
        validated_output_amount: output,
    };

    (plan, eval)
}

// ─── tests ──────────────────────────────────────────────────────────────────

#[test]
fn test_best_score_selects_highest_score_plan() {
    let pairs = vec![
        make_pair(1, 1, 5000, 100, 500),
        make_pair(2, 2, 9000, 100, 500), // highest score
        make_pair(3, 3, 3000, 100, 500),
    ];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestScore).unwrap();
    assert_eq!(idx, 1, "plan with score 9000 should win");
}

#[test]
fn test_lowest_fee_policy() {
    let pairs = vec![
        make_pair(1, 1, 5000, 500, 500),
        make_pair(2, 2, 5000, 50, 500),  // lowest fee
        make_pair(3, 3, 5000, 200, 500),
    ];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::LowestFee).unwrap();
    assert_eq!(idx, 1, "plan with fee 50 should win");
}

#[test]
fn test_best_output_policy() {
    let pairs = vec![
        make_pair(1, 1, 5000, 100, 500),
        make_pair(2, 2, 5000, 100, 900), // highest output
        make_pair(3, 3, 5000, 100, 700),
    ];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestOutput).unwrap();
    assert_eq!(idx, 1, "plan with output 900 should win");
}

#[test]
fn test_tie_break_by_fee() {
    // Two plans with same score; lower fee wins
    let pairs = vec![
        make_pair(1, 1, 9000, 300, 500),
        make_pair(2, 2, 9000, 100, 500), // same score, lower fee
    ];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestScore).unwrap();
    assert_eq!(idx, 1, "plan with lower fee should win on tie");
}

#[test]
fn test_tie_break_by_solver_id_lexical() {
    // Same score, same fee — lower solver_id wins
    let mut solver_a = [0u8; 32];
    solver_a[0] = 0x01; // "smaller" id
    let mut solver_b = [0u8; 32];
    solver_b[0] = 0x02;

    let plan_id_a = txid(10);
    let plan_id_b = txid(11);

    let mut plan_a = CandidatePlan {
        plan_id: plan_id_a,
        intent_id: txid(1),
        solver_id: solver_a,
        actions: vec![],
        required_capabilities: vec![],
        object_reads: vec![],
        object_writes: vec![],
        expected_output_amount: 500,
        fee_quote: 100,
        quality_score: 9000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    plan_a.plan_hash = plan_a.compute_hash();

    let mut plan_b = CandidatePlan {
        plan_id: plan_id_b,
        intent_id: txid(1),
        solver_id: solver_b,
        ..plan_a.clone()
    };
    plan_b.solver_id = solver_b;
    plan_b.plan_id = plan_id_b;
    plan_b.plan_hash = [0u8; 32];
    plan_b.plan_hash = plan_b.compute_hash();

    let eval_a = PlanEvaluationResult {
        plan_id: plan_id_a,
        solver_id: solver_a,
        passed: true,
        reason_codes: vec![ValidationReasonCode::Valid],
        normalized_score: 9000,
        validated_object_reads: vec![],
        validated_object_writes: vec![],
        settlement_footprint: 100,
        validated_output_amount: 500,
    };
    let eval_b = PlanEvaluationResult {
        plan_id: plan_id_b,
        solver_id: solver_b,
        ..eval_a.clone()
    };

    let pairs = vec![(plan_a, eval_a), (plan_b, eval_b)];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestScore).unwrap();
    assert_eq!(idx, 0, "solver with lexically smaller id should win final tie-break");
}

#[test]
fn test_no_valid_plans_returns_error() {
    let pairs: Vec<(CandidatePlan, PlanEvaluationResult)> = vec![];
    let result = WinningPlanSelector::select(&pairs, SelectionPolicy::BestScore);
    assert!(matches!(result, Err(RuntimeError::NoCandidatePlans)));
}

#[test]
fn test_single_plan_always_wins() {
    let pairs = vec![make_pair(1, 1, 5000, 100, 500)];
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestScore).unwrap();
    assert_eq!(idx, 0);
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::LowestFee).unwrap();
    assert_eq!(idx, 0);
    let idx = WinningPlanSelector::select(&pairs, SelectionPolicy::BestOutput).unwrap();
    assert_eq!(idx, 0);
}

#[test]
fn test_rank_returns_all_plans_sorted() {
    let pairs = vec![
        make_pair(1, 1, 3000, 100, 500),
        make_pair(2, 2, 9000, 100, 500),
        make_pair(3, 3, 6000, 100, 500),
    ];
    let ranking = PlanRanker::rank(&pairs, SelectionPolicy::BestScore);
    assert_eq!(ranking.len(), 3);
    // Best score first: index 1 (9000), then 2 (6000), then 0 (3000)
    assert_eq!(ranking[0], 1);
    assert_eq!(ranking[1], 2);
    assert_eq!(ranking[2], 0);
}
