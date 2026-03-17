//! Determinism verification tests — Phase B of the security hardening.
//!
//! Simulates multiple nodes processing identical inputs and verifies
//! they produce identical outputs (ordering, sequence roots, receipts).

use omniphi_poseq::intent_pool::types::*;
use omniphi_poseq::intent_pool::pool::IntentPool;
use omniphi_poseq::auction::types::*;
use omniphi_poseq::auction::state::*;
use omniphi_poseq::auction::ordering::*;
use std::collections::BTreeMap;

// ─── Helpers ────────────────────────────────────────────────────────────────

fn make_asset(b: u8) -> AssetId {
    let mut id = [0u8; 32]; id[0] = b;
    AssetId::token(id)
}

fn make_swap(user_byte: u8, nonce: u64, tip: u64) -> IntentTransaction {
    let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
    let swap = SwapIntent {
        asset_in: make_asset(1),
        asset_out: make_asset(2),
        amount_in: 1000,
        min_amount_out: 950,
        max_slippage_bps: 50,
        route_hint: None,
    };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32],
        intent_type: IntentKind::Swap(swap),
        version: 1,
        user,
        nonce,
        recipient: None,
        deadline: 1000,
        valid_from: None,
        max_fee: 100,
        tip: Some(tip),
        partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None,
        signature: vec![1u8; 64],
        metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    intent
}

fn make_reveal_for(
    bundle_byte: u8,
    solver_byte: u8,
    intent: &IntentTransaction,
    amount_out: u128,
    fee_bps: u64,
) -> (BundleCommitment, BundleReveal) {
    let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte;
    let mut solver_id = [0u8; 32]; solver_id[0] = solver_byte;

    let steps = vec![ExecutionStep {
        step_index: 0,
        operation: OperationType::Debit,
        object_id: [10u8; 32],
        read_set: vec![[10u8; 32]],
        write_set: vec![[10u8; 32]],
        params: OperationParams {
            asset: Some(make_asset(1)),
            amount: Some(1000),
            recipient: None, pool_id: None, custom_data: None,
        },
    }];

    let outputs = vec![PredictedOutput {
        intent_id: intent.intent_id,
        asset_out: make_asset(2),
        amount_out,
        fee_charged_bps: fee_bps,
    }];

    let fee = FeeBreakdown {
        solver_fee_bps: fee_bps / 2,
        protocol_fee_bps: fee_bps - fee_bps / 2,
        total_fee_bps: fee_bps,
    };

    let reveal = BundleReveal {
        bundle_id, solver_id, batch_window: 1,
        target_intent_ids: vec![intent.intent_id],
        execution_steps: steps,
        liquidity_sources: vec![],
        predicted_outputs: outputs,
        fee_breakdown: fee,
        resource_declarations: vec![],
        nonce: [bundle_byte; 32],
        proof_data: vec![],
        signature: vec![1u8; 64],
    };

    let commitment = BundleCommitment {
        bundle_id, solver_id, batch_window: 1,
        target_intent_count: 1,
        commitment_hash: reveal.compute_commitment_hash(),
        expected_outputs_hash: reveal.compute_expected_outputs_hash(),
        execution_plan_hash: reveal.compute_execution_plan_hash(),
        valid_until: 200, bond_locked: 50_000,
        signature: vec![1u8; 64],
    };

    (commitment, reveal)
}

/// Simulate one "node" processing a batch window and returning the ordering result.
fn simulate_node(
    intents: &[IntentTransaction],
    commits_and_reveals: &[(BundleCommitment, BundleReveal)],
) -> IntentOrderingResult {
    let mut auction = AuctionWindow::new(1, 100);

    for (c, _) in commits_and_reveals {
        auction.record_commitment(c.clone(), 102).unwrap();
    }
    for (_, r) in commits_and_reveals {
        auction.record_reveal(r.clone(), 106).unwrap();
    }

    let reveals = auction.valid_reveals();
    let refs: Vec<&BundleReveal> = reveals.iter().copied().collect();

    // Build intent metadata for spec-compliant ordering
    let mut meta = BTreeMap::new();
    for intent in intents {
        meta.insert(intent.intent_id, IntentOrderingMeta {
            intent_id: intent.intent_id,
            deadline: intent.deadline,
            tip: intent.tip.unwrap_or(0),
        });
    }

    order_bundles_with_meta(&refs, 1, &meta)
}

// ─── Determinism Tests ──────────────────────────────────────────────────────

#[test]
fn test_determinism_identical_inputs_same_output() {
    let intent = make_swap(1, 1, 50);
    let (c, r) = make_reveal_for(1, 10, &intent, 960, 30);

    let result_a = simulate_node(&[intent.clone()], &[(c.clone(), r.clone())]);
    let result_b = simulate_node(&[intent], &[(c, r)]);

    assert_eq!(result_a.sequence_root, result_b.sequence_root);
    assert_eq!(result_a.ordered_bundles.len(), result_b.ordered_bundles.len());
    for (a, b) in result_a.ordered_bundles.iter().zip(result_b.ordered_bundles.iter()) {
        assert_eq!(a.bundle_id, b.bundle_id);
        assert_eq!(a.solver_id, b.solver_id);
        assert_eq!(a.sequence_index, b.sequence_index);
    }
}

#[test]
fn test_determinism_multiple_solvers_competing() {
    let intent = make_swap(1, 1, 50);

    // 5 solvers with different outputs
    let pairs: Vec<(BundleCommitment, BundleReveal)> = (1u8..=5).map(|i| {
        make_reveal_for(i, i + 10, &intent, 950 + i as u128 * 5, 30)
    }).collect();

    // Run 10 times — must always get same result
    let first = simulate_node(&[intent.clone()], &pairs);
    for _ in 0..10 {
        let result = simulate_node(&[intent.clone()], &pairs);
        assert_eq!(result.sequence_root, first.sequence_root);
        assert_eq!(result.ordered_bundles[0].bundle_id, first.ordered_bundles[0].bundle_id);
    }
}

#[test]
fn test_determinism_tiebreak_with_equal_scores() {
    let intent = make_swap(1, 1, 50);

    // Two solvers with IDENTICAL user_outcome_score
    let (c1, r1) = make_reveal_for(1, 10, &intent, 960, 30);
    let (c2, r2) = make_reveal_for(2, 20, &intent, 960, 30);

    // Order A: solver 1 first, solver 2 second
    let result_a = simulate_node(&[intent.clone()], &[(c1.clone(), r1.clone()), (c2.clone(), r2.clone())]);
    // Order B: solver 2 first, solver 1 second
    let result_b = simulate_node(&[intent.clone()], &[(c2, r2), (c1, r1)]);

    // Same winner regardless of insertion order
    assert_eq!(result_a.sequence_root, result_b.sequence_root);
    assert_eq!(result_a.ordered_bundles[0].bundle_id, result_b.ordered_bundles[0].bundle_id);
}

#[test]
fn test_determinism_multiple_intents_ordering() {
    // 3 intents with different deadlines and tips
    let intent_a = make_swap(1, 1, 100); // high tip
    let intent_b = make_swap(2, 1, 10);  // low tip
    let intent_c = make_swap(3, 1, 50);  // medium tip

    let (c_a, r_a) = make_reveal_for(1, 10, &intent_a, 960, 30);
    let (c_b, r_b) = make_reveal_for(2, 20, &intent_b, 960, 30);
    let (c_c, r_c) = make_reveal_for(3, 30, &intent_c, 960, 30);

    let all = &[intent_a.clone(), intent_b.clone(), intent_c.clone()];
    let pairs = vec![(c_a.clone(), r_a.clone()), (c_b.clone(), r_b.clone()), (c_c.clone(), r_c.clone())];

    let result1 = simulate_node(all, &pairs);

    // Shuffle input order
    let pairs_shuffled = vec![(c_c, r_c), (c_a, r_a), (c_b, r_b)];
    let result2 = simulate_node(all, &pairs_shuffled);

    assert_eq!(result1.sequence_root, result2.sequence_root);
    assert_eq!(result1.ordered_bundles.len(), result2.ordered_bundles.len());
    for (a, b) in result1.ordered_bundles.iter().zip(result2.ordered_bundles.iter()) {
        assert_eq!(a.bundle_id, b.bundle_id);
    }
}

#[test]
fn test_determinism_intent_id_computation() {
    // Same parameters → same intent_id across "nodes"
    let a = make_swap(1, 42, 100);
    let b = make_swap(1, 42, 100);
    assert_eq!(a.intent_id, b.intent_id);

    // Different parameters → different intent_id
    let c = make_swap(1, 43, 100);
    assert_ne!(a.intent_id, c.intent_id);
}

#[test]
fn test_determinism_commitment_hash_across_nodes() {
    let intent = make_swap(1, 1, 50);
    let (c1, _) = make_reveal_for(1, 10, &intent, 960, 30);
    let (c2, _) = make_reveal_for(1, 10, &intent, 960, 30);

    // Same solver, same intent → same commitment hash
    assert_eq!(c1.commitment_hash, c2.commitment_hash);
}

#[test]
fn test_determinism_sequence_root_order_sensitive() {
    let intent_a = make_swap(1, 1, 100);
    let intent_b = make_swap(2, 1, 50);

    let (_, r_a) = make_reveal_for(1, 10, &intent_a, 960, 30);
    let (_, r_b) = make_reveal_for(2, 20, &intent_b, 960, 30);

    let bundle_a = SequencedBundle {
        bundle_id: r_a.bundle_id, solver_id: r_a.solver_id,
        target_intent_ids: vec![intent_a.intent_id], execution_steps: vec![],
        predicted_outputs: vec![], resource_declarations: vec![],
        sequence_index: 0,
    };
    let bundle_b = SequencedBundle {
        bundle_id: r_b.bundle_id, solver_id: r_b.solver_id,
        target_intent_ids: vec![intent_b.intent_id], execution_steps: vec![],
        predicted_outputs: vec![], resource_declarations: vec![],
        sequence_index: 1,
    };

    let root_ab = compute_sequence_root(&[bundle_a.clone(), bundle_b.clone()]);
    let root_ba = compute_sequence_root(&[bundle_b, bundle_a]);

    // Different order → different root (proves order matters)
    assert_ne!(root_ab, root_ba);
}

#[test]
fn test_determinism_pool_admission_order_independent() {
    // Two pools admitting the same intents in different order should have same content
    let intents: Vec<IntentTransaction> = (1u8..=5).map(|i| make_swap(i, 1, i as u64 * 10)).collect();

    let mut pool_a = IntentPool::new();
    pool_a.set_block_height(10);
    for intent in &intents {
        pool_a.admit(intent.clone()).unwrap();
    }

    let mut pool_b = IntentPool::new();
    pool_b.set_block_height(10);
    // Reverse order
    for intent in intents.iter().rev() {
        pool_b.admit(intent.clone()).unwrap();
    }

    assert_eq!(pool_a.len(), pool_b.len());

    // Both pools should contain the same intents
    for intent in &intents {
        assert!(pool_a.get(&intent.intent_id).is_some());
        assert!(pool_b.get(&intent.intent_id).is_some());
    }
}

#[test]
fn test_determinism_deadline_ordering_with_meta() {
    // Intents with different deadlines should be ordered by deadline ASC
    let mut intent_urgent = make_swap(1, 1, 50);
    // Override deadline to be closer
    intent_urgent.deadline = 200;
    intent_urgent.intent_id = intent_urgent.compute_intent_id();

    let mut intent_relaxed = make_swap(2, 1, 50);
    intent_relaxed.deadline = 500;
    intent_relaxed.intent_id = intent_relaxed.compute_intent_id();

    let (c1, r1) = make_reveal_for(1, 10, &intent_urgent, 960, 30);
    let (c2, r2) = make_reveal_for(2, 20, &intent_relaxed, 960, 30);

    let result = simulate_node(
        &[intent_urgent.clone(), intent_relaxed.clone()],
        &[(c1, r1), (c2, r2)],
    );

    assert_eq!(result.ordered_bundles.len(), 2);
    // Urgent (deadline 200) should come before relaxed (deadline 500)
    let first_target = result.ordered_bundles[0].target_intent_ids[0];
    assert_eq!(first_target, intent_urgent.intent_id);
}
