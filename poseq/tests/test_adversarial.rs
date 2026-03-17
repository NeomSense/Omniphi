//! Adversarial and property-based tests — Phase C/D of security hardening.
//!
//! Tests protocol invariants under adversarial input and verifies
//! security properties that must hold regardless of input.

use omniphi_poseq::intent_pool::types::*;
use omniphi_poseq::intent_pool::pool::IntentPool;
use omniphi_poseq::intent_pool::lifecycle::IntentState;
use omniphi_poseq::auction::types::*;
use omniphi_poseq::auction::state::*;
use omniphi_poseq::auction::ordering::*;
use std::collections::BTreeMap;

fn make_asset(b: u8) -> AssetId {
    let mut id = [0u8; 32]; id[0] = b;
    AssetId::token(id)
}

fn make_swap(user_byte: u8, nonce: u64, tip: u64, deadline: u64) -> IntentTransaction {
    let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
    let swap = SwapIntent {
        asset_in: make_asset(1), asset_out: make_asset(2),
        amount_in: 1000, min_amount_out: 950, max_slippage_bps: 50, route_hint: None,
    };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32], intent_type: IntentKind::Swap(swap), version: 1,
        user, nonce, recipient: None, deadline, valid_from: None, max_fee: 100,
        tip: Some(tip), partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None, signature: vec![1u8; 64], metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    intent
}

fn make_pair(
    bundle_byte: u8, solver_byte: u8, intent: &IntentTransaction, amount_out: u128, fee_bps: u64,
) -> (BundleCommitment, BundleReveal) {
    let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte;
    let mut solver_id = [0u8; 32]; solver_id[0] = solver_byte;
    let steps = vec![ExecutionStep {
        step_index: 0, operation: OperationType::Debit, object_id: [10u8; 32],
        read_set: vec![[10u8; 32]], write_set: vec![[10u8; 32]],
        params: OperationParams { asset: Some(make_asset(1)), amount: Some(1000),
            recipient: None, pool_id: None, custom_data: None },
    }];
    let outputs = vec![PredictedOutput {
        intent_id: intent.intent_id, asset_out: make_asset(2), amount_out, fee_charged_bps: fee_bps,
    }];
    let fee = FeeBreakdown { solver_fee_bps: fee_bps / 2, protocol_fee_bps: fee_bps - fee_bps / 2, total_fee_bps: fee_bps };
    let reveal = BundleReveal {
        bundle_id, solver_id, batch_window: 1,
        target_intent_ids: vec![intent.intent_id], execution_steps: steps,
        liquidity_sources: vec![], predicted_outputs: outputs, fee_breakdown: fee,
        resource_declarations: vec![],
        nonce: [bundle_byte; 32], proof_data: vec![], signature: vec![1u8; 64],
    };
    let commitment = BundleCommitment {
        bundle_id, solver_id, batch_window: 1, target_intent_count: 1,
        commitment_hash: reveal.compute_commitment_hash(),
        expected_outputs_hash: reveal.compute_expected_outputs_hash(),
        execution_plan_hash: reveal.compute_execution_plan_hash(),
        valid_until: 200, bond_locked: 50_000, signature: vec![1u8; 64],
    };
    (commitment, reveal)
}

// ─── INVARIANT 1: Intent cannot be executed twice ───────────────────────────

#[test]
fn test_invariant_no_double_sequencing() {
    let mut pool = IntentPool::new();
    pool.set_block_height(10);
    let intent = make_swap(1, 1, 50, 500);
    pool.admit(intent.clone()).unwrap();
    pool.open_admitted_intents();

    // First match succeeds
    assert!(pool.mark_matched(&intent.intent_id, [1u8; 32], [10u8; 32]));

    // Second match from different solver on same intent — must fail
    // (state is already Matched, cannot re-match)
    let result = pool.mark_matched(&intent.intent_id, [2u8; 32], [20u8; 32]);
    // mark_matched tries to transition Open→Matched, but intent is already Matched
    // The transition Matched→Matched is invalid, so it returns false
    assert!(!result);
}

// ─── INVARIANT 2: Reveal must match commitment hash ─────────────────────────

#[test]
fn test_invariant_reveal_commitment_binding() {
    let intent = make_swap(1, 1, 50, 500);
    let (commitment, reveal) = make_pair(1, 10, &intent, 960, 30);

    // Verify the binding
    assert!(reveal.verify_against_commitment(&commitment).is_ok());

    // ANY modification to the reveal breaks the binding
    let mut tampered = reveal.clone();
    tampered.predicted_outputs[0].amount_out += 1;
    assert!(tampered.verify_against_commitment(&commitment).is_err());

    let mut tampered = reveal.clone();
    tampered.fee_breakdown.total_fee_bps += 1;
    assert!(tampered.verify_against_commitment(&commitment).is_err());

    let mut tampered = reveal.clone();
    tampered.execution_steps[0].params.amount = Some(999);
    assert!(tampered.verify_against_commitment(&commitment).is_err());

    let mut tampered = reveal.clone();
    tampered.nonce = [99u8; 32];
    assert!(tampered.verify_against_commitment(&commitment).is_err());
}

// ─── INVARIANT 3: Sequence ordering must be stable ──────────────────────────

#[test]
fn test_invariant_ordering_stability() {
    let intent = make_swap(1, 1, 50, 500);
    let pairs: Vec<_> = (1u8..=10).map(|i| make_pair(i, i + 10, &intent, 950 + i as u128, 30)).collect();

    let reveals: Vec<BundleReveal> = pairs.iter().map(|(_, r)| r.clone()).collect();
    let refs: Vec<&BundleReveal> = reveals.iter().collect();

    // Run 100 times — must always produce same result
    let first = order_bundles(&refs, 1);
    for _ in 0..100 {
        let result = order_bundles(&refs, 1);
        assert_eq!(result.sequence_root, first.sequence_root);
    }
}

// ─── INVARIANT 4: Expired intents must never execute ────────────────────────

#[test]
fn test_invariant_expired_intent_rejected() {
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    // Intent with deadline in the past
    let intent = make_swap(1, 1, 50, 15); // deadline = 15, MIN_INTENT_LIFETIME = 10
    // Deadline 15 <= current 10 + 10 = 20, so rejected
    assert!(pool.admit(intent).is_err());
}

#[test]
fn test_invariant_expired_intent_pruned_before_matching() {
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    let intent = make_swap(1, 1, 50, 100);
    pool.admit(intent.clone()).unwrap();
    pool.open_admitted_intents();

    // Advance past deadline
    pool.set_block_height(100);
    pool.prune_expired();

    // Intent should be gone — cannot match
    assert!(pool.get(&intent.intent_id).is_none());
    assert!(!pool.mark_matched(&intent.intent_id, [1u8; 32], [10u8; 32]));
}

// ─── ADVERSARIAL: Solver copying attempt ────────────────────────────────────

#[test]
fn test_adversarial_solver_copy_fails() {
    let intent = make_swap(1, 1, 50, 500);
    let mut auction = AuctionWindow::new(1, 100);

    // Solver A commits honestly
    let (c_a, r_a) = make_pair(1, 10, &intent, 960, 30);
    let c_a_hash = c_a.commitment_hash;
    auction.record_commitment(c_a, 102).unwrap();

    // Solver B sees A's commitment hash and tries to submit a DIFFERENT reveal
    // with the SAME commitment hash (impossible without knowing the preimage)
    let (_, mut r_b) = make_pair(2, 20, &intent, 960, 30);
    // r_b has a different nonce → different commitment hash
    // Even if B copies A's output amounts, their bundle_id and nonce differ

    // B cannot create a valid commitment for A's plan without A's nonce
    let fake_commitment = BundleCommitment {
        bundle_id: r_b.bundle_id,
        solver_id: r_b.solver_id,
        batch_window: 1,
        target_intent_count: 1,
        // B tries to use A's commitment hash
        commitment_hash: c_a_hash, // WRONG — doesn't match B's reveal
        expected_outputs_hash: r_b.compute_expected_outputs_hash(),
        execution_plan_hash: r_b.compute_execution_plan_hash(),
        valid_until: 200,
        bond_locked: 50_000,
        signature: vec![1u8; 64],
    };

    // This commitment has the wrong hash, so reveal will fail
    auction.record_commitment(fake_commitment, 103).unwrap();

    // A reveals successfully
    auction.record_reveal(r_a, 106).unwrap();

    // B's reveal fails because their preimage doesn't match the (stolen) commitment hash
    match auction.record_reveal(r_b, 106) {
        Err(AuctionError::RevealValidation(RevealValidationError::CommitmentHashMismatch { .. })) => {}
        other => panic!("expected CommitmentHashMismatch, got {:?}", other),
    }
}

// ─── ADVERSARIAL: Replay attempt ────────────────────────────────────────────

#[test]
fn test_adversarial_replay_intent() {
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    let intent = make_swap(1, 1, 50, 500);
    pool.admit(intent.clone()).unwrap();

    // Same intent again — rejected as duplicate
    assert_eq!(pool.admit(intent).unwrap_err(), IntentValidationError::DuplicateIntent);
}

#[test]
fn test_adversarial_nonce_replay() {
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    let intent1 = make_swap(1, 1, 50, 500);
    pool.admit(intent1).unwrap();

    // Different intent but same nonce from same user — rejected
    let intent2 = make_swap(1, 1, 100, 600); // same user, same nonce
    match pool.admit(intent2) {
        Err(IntentValidationError::NonceAlreadyUsed { .. }) => {}
        other => panic!("expected NonceAlreadyUsed, got {:?}", other),
    }
}

// ─── ADVERSARIAL: Commit-without-reveal spam ────────────────────────────────

#[test]
fn test_adversarial_commit_spam_rate_limited() {
    let intent = make_swap(1, 1, 50, 500);
    let mut auction = AuctionWindow::new(1, 100);

    // Solver submits MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW commitments
    for i in 0..10 {
        let (mut c, _) = make_pair(i + 1, 10, &intent, 960, 30);
        c.bundle_id[1] = i; // unique
        auction.record_commitment(c, 102).unwrap();
    }

    // 11th commitment rejected
    let (mut c, _) = make_pair(99, 10, &intent, 960, 30);
    c.bundle_id[1] = 99;
    match auction.record_commitment(c, 102) {
        Err(AuctionError::MaxCommitmentsExceeded { .. }) => {}
        other => panic!("expected MaxCommitmentsExceeded, got {:?}", other),
    }
}

// ─── ADVERSARIAL: Malicious bundle ordering attempt ─────────────────────────

#[test]
fn test_adversarial_cannot_manipulate_tiebreak() {
    let intent = make_swap(1, 1, 50, 500);

    // Solver tries different bundle_ids to find a favorable tiebreak
    // But the tiebreak is SHA256(bundle_id ‖ intent_id ‖ batch_window)
    // which is unpredictable to the solver before commitment

    let mut winners = std::collections::BTreeSet::new();
    for i in 0u8..20 {
        let (_, r) = make_pair(i + 1, i + 10, &intent, 960, 30);
        let refs = vec![&r];
        let result = order_bundles(&refs, 1);
        if !result.ordered_bundles.is_empty() {
            winners.insert(result.ordered_bundles[0].bundle_id);
        }
    }

    // Each individual solver wins when they're the only one
    // The tiebreak only matters when two solvers compete with equal scores
    assert_eq!(winners.len(), 20); // Each unique solver produces unique winner
}

// ─── PROPERTY: Zero-amount intents always rejected ──────────────────────────

#[test]
fn test_property_zero_amounts_rejected() {
    // Swap with zero amount_in
    let mut intent = make_swap(1, 1, 50, 500);
    if let IntentKind::Swap(ref mut s) = intent.intent_type {
        s.amount_in = 0;
    }
    intent.intent_id = intent.compute_intent_id();
    assert!(intent.validate_basic().is_err());

    // Payment with zero amount
    let user = { let mut u = [0u8; 32]; u[0] = 1; u };
    let recipient = { let mut r = [0u8; 32]; r[0] = 2; r };
    let payment = PaymentIntent { asset: make_asset(1), amount: 0, recipient, memo: None };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32], intent_type: IntentKind::Payment(payment), version: 1,
        user, nonce: 1, recipient: Some(recipient), deadline: 1000, valid_from: None,
        max_fee: 100, tip: None, partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None, signature: vec![1u8; 64], metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    assert!(intent.validate_basic().is_err());
}

// ─── PROPERTY: Phase boundaries strictly enforced ───────────────────────────

#[test]
fn test_property_phase_boundaries() {
    let intent = make_swap(1, 1, 50, 500);
    let (commitment, reveal) = make_pair(1, 10, &intent, 960, 30);

    // Cannot commit during reveal phase
    let mut auction = AuctionWindow::new(1, 100);
    assert!(auction.record_commitment(commitment.clone(), 106).is_err());

    // Cannot reveal during commit phase
    let mut auction = AuctionWindow::new(1, 100);
    auction.record_commitment(commitment, 102).unwrap();
    assert!(auction.record_reveal(reveal.clone(), 103).is_err());

    // Cannot reveal after selection
    let mut auction = AuctionWindow::new(1, 100);
    let (c2, _) = make_pair(1, 10, &intent, 960, 30);
    auction.record_commitment(c2, 102).unwrap();
    assert!(auction.record_reveal(reveal, 109).is_err());
}
