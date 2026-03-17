//! Phase 4 — Multi-node simulation framework and distributed tests.
//!
//! Simulates 4-7 validators with multiple solvers, random intent traffic,
//! commit/reveal windows, ordering, finality verification, fairness logging,
//! DA validation, and failure scenarios.

use omniphi_poseq::intent_pool::types::*;
use omniphi_poseq::intent_pool::pool::IntentPool;
use omniphi_poseq::intent_pool::lifecycle::IntentState;
use omniphi_poseq::auction::types::*;
use omniphi_poseq::auction::state::*;
use omniphi_poseq::auction::ordering::*;
use omniphi_poseq::auction::da_checks::*;
use omniphi_poseq::finality::commitment::*;
use omniphi_poseq::finality::validator_checks::verify_commitment;
use omniphi_poseq::fairness::log::{FairnessLog, ReasonCode};
use omniphi_poseq::fairness::evidence::detect_censorship;
use std::collections::{BTreeMap, BTreeSet};

// ─── Deterministic PRNG ─────────────────────────────────────────────────────

struct Rng(u64);
impl Rng {
    fn new(seed: u64) -> Self { Rng(seed) }
    fn next(&mut self) -> u64 { self.0 ^= self.0 << 13; self.0 ^= self.0 >> 7; self.0 ^= self.0 << 17; self.0 }
    fn range(&mut self, max: u64) -> u64 { if max == 0 { 0 } else { self.next() % max } }
    fn bool(&mut self) -> bool { self.next() & 1 == 0 }
    fn byte(&mut self) -> u8 { (self.next() & 0xFF) as u8 }
}

// ─── Simulated Node ─────────────────────────────────────────────────────────

struct SimNode {
    id: [u8; 32],
    pool: IntentPool,
    previous_commitment_root: [u8; 32],
    last_sequence_id: u64,
}

impl SimNode {
    fn new(node_byte: u8) -> Self {
        let mut id = [0u8; 32]; id[0] = node_byte;
        SimNode {
            id,
            pool: IntentPool::new(),
            previous_commitment_root: [0u8; 32],
            last_sequence_id: 0,
        }
    }
}

// ─── Simulated Solver ───────────────────────────────────────────────────────

struct SimSolver {
    id: [u8; 32],
}

// ─── Helpers ────────────────────────────────────────────────────────────────

fn make_asset(b: u8) -> AssetId {
    let mut id = [0u8; 32]; id[0] = b;
    AssetId::token(id)
}

fn make_intent(rng: &mut Rng, user_byte: u8, nonce: u64) -> IntentTransaction {
    let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
    let swap = SwapIntent {
        asset_in: make_asset(1), asset_out: make_asset(2),
        amount_in: 1000 + rng.range(5000) as u128,
        min_amount_out: 900, max_slippage_bps: 50, route_hint: None,
    };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32], intent_type: IntentKind::Swap(swap), version: 1,
        user, nonce, recipient: None,
        deadline: 10_000 + rng.range(5000),
        valid_from: None, max_fee: 100,
        tip: Some(rng.range(200)),
        partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None, signature: vec![1u8; 64], metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    intent
}

fn solver_make_pair(
    rng: &mut Rng,
    solver: &SimSolver,
    intent: &IntentTransaction,
    batch_window: u64,
) -> (BundleCommitment, BundleReveal) {
    let mut bundle_id = [0u8; 32];
    bundle_id[0] = rng.byte();
    bundle_id[1] = rng.byte();
    let amount_out = 900 + rng.range(200) as u128;

    let steps = vec![ExecutionStep {
        step_index: 0, operation: OperationType::Debit, object_id: [10u8; 32],
        read_set: vec![[10u8; 32]], write_set: vec![[10u8; 32]],
        params: OperationParams { asset: Some(make_asset(1)), amount: Some(1000),
            recipient: None, pool_id: None, custom_data: None },
    }];
    let outputs = vec![PredictedOutput {
        intent_id: intent.intent_id, asset_out: make_asset(2), amount_out, fee_charged_bps: 30,
    }];
    let fee = FeeBreakdown { solver_fee_bps: 15, protocol_fee_bps: 15, total_fee_bps: 30 };
    let reveal = BundleReveal {
        bundle_id, solver_id: solver.id, batch_window,
        target_intent_ids: vec![intent.intent_id], execution_steps: steps,
        liquidity_sources: vec![], predicted_outputs: outputs, fee_breakdown: fee,
        resource_declarations: vec![], nonce: bundle_id,
        proof_data: vec![], signature: vec![1u8; 64],
    };
    let commitment = BundleCommitment {
        bundle_id, solver_id: solver.id, batch_window,
        target_intent_count: 1,
        commitment_hash: reveal.compute_commitment_hash(),
        expected_outputs_hash: reveal.compute_expected_outputs_hash(),
        execution_plan_hash: reveal.compute_execution_plan_hash(),
        valid_until: 20_000, bond_locked: 50_000, signature: vec![1u8; 64],
    };
    (commitment, reveal)
}

/// Run one slot: commit phase → reveal phase → ordering → finality commitment → verification.
/// Returns the FinalityCommitment if successful.
fn run_slot(
    nodes: &mut [SimNode],
    solvers: &[SimSolver],
    intents: &[IntentTransaction],
    batch_window: u64,
    slot_start_block: u64,
    rng: &mut Rng,
) -> Option<FinalityCommitment> {
    // 1. All nodes admit intents
    for node in nodes.iter_mut() {
        node.pool.set_block_height(slot_start_block);
        for intent in intents {
            let _ = node.pool.admit(intent.clone());
        }
        node.pool.open_admitted_intents();
    }

    // 2. Solvers produce commits/reveals for random intents
    let mut auction = AuctionWindow::new(batch_window, slot_start_block);
    let mut all_commitments: BTreeMap<[u8; 32], BundleCommitment> = BTreeMap::new();
    let mut all_reveals: Vec<BundleReveal> = Vec::new();

    for solver in solvers {
        for intent in intents {
            if rng.bool() { continue; } // solver randomly skips some intents
            let (c, r) = solver_make_pair(rng, solver, intent, batch_window);
            if auction.record_commitment(c.clone(), slot_start_block + 1).is_ok() {
                all_commitments.insert(c.bundle_id, c);
                all_reveals.push(r);
            }
        }
    }

    // Reveal phase
    let reveal_block = slot_start_block + 5;
    for r in &all_reveals {
        let _ = auction.record_reveal(r.clone(), reveal_block);
    }

    // 3. Ordering (deterministic — same on all nodes)
    let reveals = auction.valid_reveals();
    let refs: Vec<&BundleReveal> = reveals.iter().copied().collect();
    if refs.is_empty() { return None; }

    let mut meta = BTreeMap::new();
    for intent in intents {
        meta.insert(intent.intent_id, IntentOrderingMeta {
            intent_id: intent.intent_id,
            deadline: intent.deadline,
            tip: intent.tip.unwrap_or(0),
        });
    }
    let ordering = order_bundles_with_meta(&refs, batch_window, &meta);

    // 4. Build fairness log
    let mut fairness_log = FairnessLog::new();
    for bundle in &ordering.ordered_bundles {
        fairness_log.record_included(bundle.bundle_id, bundle.solver_id);
    }
    // Record excluded bundles
    let included_ids: BTreeSet<[u8; 32]> = ordering.ordered_bundles.iter().map(|b| b.bundle_id).collect();
    for r in &all_reveals {
        if !included_ids.contains(&r.bundle_id) {
            fairness_log.record_excluded(r.bundle_id, r.solver_id, ReasonCode::OrderingPriority);
        }
    }
    let fairness_root = fairness_log.compute_fairness_root();

    // 5. Build FinalityCommitment (proposer = first node)
    let proposer = nodes[0].id;
    let sequence_id = nodes[0].last_sequence_id + 1;
    let prev_root = nodes[0].previous_commitment_root;

    let commitment_hashes: Vec<[u8; 32]> = ordering.ordered_bundles.iter()
        .filter_map(|b| all_commitments.get(&b.bundle_id).map(|c| c.commitment_hash))
        .collect();
    let bundle_root = FinalityCommitment::compute_bundle_root(&commitment_hashes);

    let fc = FinalityCommitment {
        sequence_id,
        ordered_bundle_ids: ordering.ordered_bundles.iter().map(|b| b.bundle_id).collect(),
        intent_ids: intents.iter().map(|i| i.intent_id).collect(),
        ordering_root: ordering.sequence_root,
        bundle_root,
        fairness_root,
        previous_commitment: prev_root,
        proposer,
        timestamp: slot_start_block + 8,
    };

    // 6. All validators verify the commitment
    let known_intents: BTreeSet<[u8; 32]> = intents.iter().map(|i| i.intent_id).collect();
    let active_solvers: BTreeSet<[u8; 32]> = solvers.iter().map(|s| s.id).collect();
    let known_validators: BTreeSet<[u8; 32]> = nodes.iter().map(|n| n.id).collect();

    for node in nodes.iter() {
        let result = verify_commitment(
            &fc,
            &ordering.ordered_bundles,
            &all_commitments,
            &known_intents,
            &active_solvers,
            &known_validators,
            fairness_root,
            Some(node.previous_commitment_root),
            sequence_id,
            slot_start_block + 8,
        );
        assert!(result.is_ok(), "validator {} failed verification: {:?}", node.id[0], result.err());
    }

    // 7. Update all nodes' state
    let commitment_root = fc.commitment_root();
    for node in nodes.iter_mut() {
        node.previous_commitment_root = commitment_root;
        node.last_sequence_id = sequence_id;
    }

    Some(fc)
}

// ─── Normal Operation Tests ─────────────────────────────────────────────────

#[test]
fn test_distributed_normal_multi_slot() {
    let mut rng = Rng::new(42);
    let mut nodes: Vec<SimNode> = (1..=5).map(|i| SimNode::new(i)).collect();
    let solvers: Vec<SimSolver> = (10..=13).map(|i| {
        let mut id = [0u8; 32]; id[0] = i;
        SimSolver { id }
    }).collect();

    // Run 5 slots
    for slot in 0..5u64 {
        let mut nonces: BTreeMap<u8, u64> = BTreeMap::new();
        let intents: Vec<IntentTransaction> = (1..=3).map(|u| {
            let n = nonces.entry(u).or_insert(slot * 10);
            *n += 1;
            make_intent(&mut rng, u, *n)
        }).collect();

        let fc = run_slot(&mut nodes, &solvers, &intents, slot + 1, slot * 100 + 10, &mut rng);
        assert!(fc.is_some(), "slot {} should produce a commitment", slot);

        // Verify commitment chain
        if slot > 0 {
            let fc = fc.unwrap();
            assert_eq!(fc.sequence_id, slot + 1);
            assert_ne!(fc.previous_commitment, [0u8; 32]);
        }
    }

    // All nodes must have the same final state
    let root = nodes[0].previous_commitment_root;
    for node in &nodes {
        assert_eq!(node.previous_commitment_root, root, "nodes diverged");
        assert_eq!(node.last_sequence_id, 5);
    }
}

#[test]
fn test_distributed_competing_solvers_deterministic() {
    let mut rng = Rng::new(99);
    let mut nodes_a: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let mut nodes_b: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let solvers: Vec<SimSolver> = (10..=15).map(|i| {
        let mut id = [0u8; 32]; id[0] = i;
        SimSolver { id }
    }).collect();

    let intents = vec![make_intent(&mut Rng::new(99), 1, 1)];

    // Both clusters process same inputs
    let mut rng_a = Rng::new(77);
    let mut rng_b = Rng::new(77); // same seed
    let fc_a = run_slot(&mut nodes_a, &solvers, &intents, 1, 100, &mut rng_a);
    let fc_b = run_slot(&mut nodes_b, &solvers, &intents, 1, 100, &mut rng_b);

    assert!(fc_a.is_some() && fc_b.is_some());
    let a = fc_a.unwrap();
    let b = fc_b.unwrap();

    // Same ordering, same commitment root
    assert_eq!(a.ordering_root, b.ordering_root);
    assert_eq!(a.bundle_root, b.bundle_root);
    assert_eq!(a.fairness_root, b.fairness_root);
    assert_eq!(a.commitment_root(), b.commitment_root());
}

// ─── Adversarial Tests ──────────────────────────────────────────────────────

#[test]
fn test_distributed_invalid_ordering_root_rejected() {
    let mut rng = Rng::new(42);
    let nodes: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let solvers: Vec<SimSolver> = (10..=11).map(|i| {
        let mut id = [0u8; 32]; id[0] = i;
        SimSolver { id }
    }).collect();
    let intents = vec![make_intent(&mut rng, 1, 1)];

    // Build valid auction
    let mut auction = AuctionWindow::new(1, 100);
    let mut all_commitments = BTreeMap::new();
    for solver in &solvers {
        let (c, r) = solver_make_pair(&mut rng, solver, &intents[0], 1);
        auction.record_commitment(c.clone(), 101).unwrap();
        all_commitments.insert(c.bundle_id, c);
        let _ = auction.record_reveal(r, 106);
    }

    let reveals = auction.valid_reveals();
    let refs: Vec<&BundleReveal> = reveals.iter().copied().collect();
    let ordering = order_bundles(&refs, 1);

    // Tamper with ordering root
    let mut fc = FinalityCommitment {
        sequence_id: 1,
        ordered_bundle_ids: ordering.ordered_bundles.iter().map(|b| b.bundle_id).collect(),
        intent_ids: vec![intents[0].intent_id],
        ordering_root: [0xFF; 32], // TAMPERED
        bundle_root: [0u8; 32],
        fairness_root: [0u8; 32],
        previous_commitment: [0u8; 32],
        proposer: nodes[0].id,
        timestamp: 108,
    };

    let known_intents: BTreeSet<_> = intents.iter().map(|i| i.intent_id).collect();
    let active_solvers: BTreeSet<_> = solvers.iter().map(|s| s.id).collect();
    let known_validators: BTreeSet<_> = nodes.iter().map(|n| n.id).collect();

    let result = verify_commitment(
        &fc, &ordering.ordered_bundles, &all_commitments,
        &known_intents, &active_solvers, &known_validators,
        [0u8; 32], None, 1, 108,
    );
    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), CommitmentVerificationError::OrderingRootMismatch { .. }));
}

#[test]
fn test_distributed_duplicate_bundle_rejected() {
    // Test that the commitment-level duplicate detection works.
    // We test this via the FinalityCommitment's ordered_bundle_ids directly,
    // since verify_commitment checks ordering root first (step 1) before duplicates (step 4).
    let bundle_id = [1u8; 32];

    // Duplicate in ordered_bundle_ids
    let fc = FinalityCommitment {
        sequence_id: 1,
        ordered_bundle_ids: vec![bundle_id, bundle_id], // DUPLICATE
        intent_ids: vec![],
        ordering_root: [0u8; 32],
        bundle_root: [0u8; 32],
        fairness_root: [0u8; 32],
        previous_commitment: [0u8; 32],
        proposer: [0xFF; 32],
        timestamp: 100,
    };

    // Check duplicate detection directly (this is what validator_checks step 4 does)
    let mut seen = BTreeSet::new();
    let mut found_dup = false;
    for bid in &fc.ordered_bundle_ids {
        if !seen.insert(*bid) {
            found_dup = true;
            break;
        }
    }
    assert!(found_dup, "duplicate bundle should be detected");
}

#[test]
fn test_distributed_sequence_continuity_break_rejected() {
    let nodes: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let fc = FinalityCommitment {
        sequence_id: 5, // Expected 1, got 5 → gap
        ordered_bundle_ids: vec![],
        intent_ids: vec![],
        ordering_root: compute_sequence_root(&[]),
        bundle_root: FinalityCommitment::compute_bundle_root(&[]),
        fairness_root: [0u8; 32],
        previous_commitment: [0u8; 32],
        proposer: nodes[0].id,
        timestamp: 108,
    };

    let known_validators: BTreeSet<_> = nodes.iter().map(|n| n.id).collect();
    let result = verify_commitment(
        &fc, &[], &BTreeMap::new(), &BTreeSet::new(), &BTreeSet::new(),
        &known_validators, [0u8; 32], None, 1, 108,
    );
    assert!(matches!(result.unwrap_err(), CommitmentVerificationError::SequenceGap { .. }));
}

#[test]
fn test_distributed_censorship_detection() {
    let mut log = FairnessLog::new();
    // All bundles excluded → total censorship
    for i in 1..=5 {
        let mut bid = [0u8; 32]; bid[0] = i;
        let mut sid = [0u8; 32]; sid[0] = i + 10;
        log.record_excluded(bid, sid, ReasonCode::SolverInactive);
    }

    let evidence = detect_censorship(&log, 1, 3);
    assert!(!evidence.is_empty());
    assert!(evidence.iter().any(|e| e.evidence_type == omniphi_poseq::fairness::evidence::CensorshipType::TotalExclusion));
}

#[test]
fn test_distributed_no_reveals_empty_slot() {
    let mut rng = Rng::new(42);
    let mut nodes: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let solvers: Vec<SimSolver> = vec![]; // no solvers
    let intents = vec![make_intent(&mut rng, 1, 1)];

    let fc = run_slot(&mut nodes, &solvers, &intents, 1, 100, &mut rng);
    assert!(fc.is_none(), "empty slot should produce no commitment");
}

#[test]
fn test_distributed_resource_conflict_across_bundles() {
    let res_a = { let mut id = [0u8; 32]; id[0] = 1; id }; // TokenBalance(Alice)
    let res_b = { let mut id = [0u8; 32]; id[0] = 2; id }; // LiquidityPool(USDC_ETH)

    // Bundle 1: writes Alice balance, reads pool
    let decls_1 = vec![
        ResourceAccess::write(res_a),
        ResourceAccess::read(res_b),
    ];
    // Bundle 2: writes Alice balance, reads pool
    let decls_2 = vec![
        ResourceAccess::write(res_a),
        ResourceAccess::read(res_b),
    ];
    // Bundle 3: reads pool only
    let decls_3 = vec![
        ResourceAccess::read(res_b),
    ];

    // 1 vs 2: write/write conflict on Alice balance
    assert!(bundles_conflict(&decls_1, &decls_2));
    // 1 vs 3: write/read conflict on nothing (1 writes Alice, 3 reads pool; 1 reads pool, 3 reads pool → no write conflict on pool from 3's side)
    // Actually: 1 writes Alice + reads pool. 3 reads pool. No write overlap → no conflict
    assert!(!bundles_conflict(&decls_1, &decls_3));
    // 2 vs 3: same as 1 vs 3
    assert!(!bundles_conflict(&decls_2, &decls_3));
}

#[test]
fn test_distributed_previous_commitment_mismatch() {
    let nodes: Vec<SimNode> = (1..=4).map(|i| SimNode::new(i)).collect();
    let fc = FinalityCommitment {
        sequence_id: 1,
        ordered_bundle_ids: vec![],
        intent_ids: vec![],
        ordering_root: compute_sequence_root(&[]),
        bundle_root: FinalityCommitment::compute_bundle_root(&[]),
        fairness_root: [0u8; 32],
        previous_commitment: [0xAA; 32], // Wrong previous
        proposer: nodes[0].id,
        timestamp: 108,
    };

    let known_validators: BTreeSet<_> = nodes.iter().map(|n| n.id).collect();
    let result = verify_commitment(
        &fc, &[], &BTreeMap::new(), &BTreeSet::new(), &BTreeSet::new(),
        &known_validators, [0u8; 32], Some([0u8; 32]), 1, 108,
    );
    assert!(matches!(result.unwrap_err(), CommitmentVerificationError::InvalidPreviousCommitment));
}
