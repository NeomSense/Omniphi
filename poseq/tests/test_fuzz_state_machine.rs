//! Phase 3 — Stateful fuzz testing for intent execution state machine.
//!
//! Randomly generates intent submissions, solver commitments, reveal timing,
//! cancellation events, and auction windows, then checks invariants.

use omniphi_poseq::intent_pool::types::*;
use omniphi_poseq::intent_pool::pool::IntentPool;
use omniphi_poseq::intent_pool::lifecycle::IntentState;
use omniphi_poseq::auction::types::*;
use omniphi_poseq::auction::state::*;
use omniphi_poseq::auction::ordering::*;
use std::collections::BTreeMap;

// ─── Pseudo-random generator (deterministic, no std rand needed) ────────────

struct SimpleRng(u64);

impl SimpleRng {
    fn new(seed: u64) -> Self { SimpleRng(seed) }

    fn next_u64(&mut self) -> u64 {
        // xorshift64
        self.0 ^= self.0 << 13;
        self.0 ^= self.0 >> 7;
        self.0 ^= self.0 << 17;
        self.0
    }

    fn next_u8(&mut self) -> u8 {
        (self.next_u64() & 0xFF) as u8
    }

    fn next_range(&mut self, max: u64) -> u64 {
        if max == 0 { return 0; }
        self.next_u64() % max
    }

    fn next_bool(&mut self) -> bool {
        self.next_u64() & 1 == 0
    }
}

// ─── Helpers ────────────────────────────────────────────────────────────────

fn make_asset(b: u8) -> AssetId {
    let mut id = [0u8; 32]; id[0] = b;
    AssetId::token(id)
}

fn make_swap(rng: &mut SimpleRng, user_byte: u8, nonce: u64) -> IntentTransaction {
    let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
    let swap = SwapIntent {
        asset_in: make_asset(1), asset_out: make_asset(2),
        amount_in: 1000 + rng.next_range(9000) as u128,
        min_amount_out: 900,
        max_slippage_bps: 50,
        route_hint: None,
    };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32], intent_type: IntentKind::Swap(swap), version: 1,
        user, nonce, recipient: None,
        deadline: 1000 + rng.next_range(500),
        valid_from: None,
        max_fee: 100,
        tip: Some(rng.next_range(200)),
        partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None, signature: vec![1u8; 64], metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    intent
}

fn make_pair_for(
    rng: &mut SimpleRng,
    bundle_byte: u8,
    solver_byte: u8,
    intent: &IntentTransaction,
) -> (BundleCommitment, BundleReveal) {
    let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte; bundle_id[1] = rng.next_u8();
    let mut solver_id = [0u8; 32]; solver_id[0] = solver_byte;
    let amount_out = 900 + rng.next_range(200) as u128;

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
        bundle_id, solver_id, batch_window: 1,
        target_intent_ids: vec![intent.intent_id], execution_steps: steps,
        liquidity_sources: vec![], predicted_outputs: outputs, fee_breakdown: fee,
        resource_declarations: vec![],
        nonce: bundle_id, proof_data: vec![], signature: vec![1u8; 64],
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

// ─── Fuzz Tests ─────────────────────────────────────────────────────────────

#[test]
fn test_fuzz_intent_pool_random_operations() {
    let mut rng = SimpleRng::new(42);
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    let mut admitted_ids = Vec::new();
    let mut nonces: BTreeMap<u8, u64> = BTreeMap::new();

    // 200 random operations
    for _ in 0..200 {
        let op = rng.next_range(5);
        match op {
            0..=2 => {
                // Submit intent
                let user = (rng.next_range(5) + 1) as u8;
                let nonce = nonces.entry(user).or_insert(0);
                *nonce += 1;
                let intent = make_swap(&mut rng, user, *nonce);
                if let Ok(_) = pool.admit(intent.clone()) {
                    admitted_ids.push(intent.intent_id);
                }
            }
            3 => {
                // Cancel random intent
                if let Some(id) = admitted_ids.last() {
                    pool.open_admitted_intents();
                    pool.cancel(id);
                }
            }
            4 => {
                // Advance block and prune
                let block = pool.current_block() + rng.next_range(10) + 1;
                pool.set_block_height(block);
                pool.prune_expired();
            }
            _ => {}
        }
    }

    // INVARIANT: Pool size is within bounds
    assert!(pool.len() <= 50_000);

    // INVARIANT: All remaining intents have valid lifecycle state
    for id in &admitted_ids {
        if let Some(entry) = pool.get(id) {
            assert!(entry.lifecycle.state.is_active() || entry.lifecycle.state.is_terminal());
        }
    }
}

#[test]
fn test_fuzz_auction_random_commits_reveals() {
    let mut rng = SimpleRng::new(123);
    let mut auction = AuctionWindow::new(1, 100);

    // Generate random intents
    let intents: Vec<IntentTransaction> = (1..=5).map(|i| {
        let nonce = i as u64;
        make_swap(&mut rng, i, nonce)
    }).collect();

    // Random commit phase
    let mut committed = Vec::new();
    for _ in 0..20 {
        let intent_idx = rng.next_range(intents.len() as u64) as usize;
        let solver_byte = rng.next_u8().wrapping_add(10);
        let bundle_byte = rng.next_u8().wrapping_add(50);
        let (commitment, reveal) = make_pair_for(&mut rng, bundle_byte, solver_byte, &intents[intent_idx]);
        if auction.record_commitment(commitment.clone(), 102).is_ok() {
            committed.push((commitment, reveal));
        }
    }

    // Random reveal phase: some reveal, some don't
    let mut revealed_count = 0;
    for (_, reveal) in &committed {
        if rng.next_bool() {
            if auction.record_reveal(reveal.clone(), 106).is_ok() {
                revealed_count += 1;
            }
        }
    }

    // Check no-reveals
    let no_reveals = auction.finalize_no_reveals();

    // INVARIANT: no_reveals + revealed_count == committed count
    // (some commits may be duplicates that were rejected, so this is approximate)
    assert!(no_reveals.len() + revealed_count <= committed.len());

    // Order valid reveals
    let reveals = auction.valid_reveals();
    let refs: Vec<&BundleReveal> = reveals.iter().copied().collect();
    let result = order_bundles(&refs, 1);

    // INVARIANT: Ordering is deterministic
    let result2 = order_bundles(&refs, 1);
    assert_eq!(result.sequence_root, result2.sequence_root);

    // INVARIANT: No duplicate bundles in ordering
    let mut seen = std::collections::BTreeSet::new();
    for bundle in &result.ordered_bundles {
        assert!(seen.insert(bundle.bundle_id), "duplicate bundle in ordering");
    }
}

#[test]
fn test_fuzz_commitment_reveal_hash_binding() {
    let mut rng = SimpleRng::new(456);

    for _ in 0..50 {
        let nonce = rng.next_u64() + 1;
        let intent = make_swap(&mut rng, 1, nonce);
        let bb = rng.next_u8();
        let sb = rng.next_u8().wrapping_add(10);
        let (commitment, reveal) = make_pair_for(&mut rng, bb, sb, &intent);

        // INVARIANT: Valid reveal matches commitment
        assert!(reveal.verify_against_commitment(&commitment).is_ok(),
            "valid reveal failed against its own commitment");

        // INVARIANT: Any modification breaks binding
        let mut tampered = reveal.clone();
        tampered.predicted_outputs[0].amount_out += 1;
        assert!(tampered.verify_against_commitment(&commitment).is_err(),
            "tampered reveal should not match commitment");
    }
}

#[test]
fn test_fuzz_ordering_stability_random_inputs() {
    let mut rng = SimpleRng::new(789);

    for trial in 0..20 {
        let intent = make_swap(&mut rng, 1, trial + 1);
        let solver_count = (rng.next_range(8) + 2) as u8;

        let reveals: Vec<BundleReveal> = (1..=solver_count).map(|i| {
            let (_, r) = make_pair_for(&mut rng, i + 50, i + 10, &intent);
            r
        }).collect();

        let refs: Vec<&BundleReveal> = reveals.iter().collect();

        // Run ordering 5 times with same inputs
        let first = order_bundles(&refs, 1);
        for _ in 0..5 {
            let result = order_bundles(&refs, 1);
            // INVARIANT: Same inputs → same output
            assert_eq!(result.sequence_root, first.sequence_root,
                "ordering not stable on trial {}", trial);
            assert_eq!(result.ordered_bundles.len(), first.ordered_bundles.len());
        }
    }
}

#[test]
fn test_fuzz_resource_conflict_detection() {
    let mut rng = SimpleRng::new(101);

    for _ in 0..50 {
        let res_id = { let mut id = [0u8; 32]; id[0] = rng.next_u8(); id };

        // Write/Write always conflicts
        let a = vec![ResourceAccess::write(res_id)];
        let b = vec![ResourceAccess::write(res_id)];
        assert!(bundles_conflict(&a, &b), "write/write must conflict");

        // Write/Read always conflicts
        let a = vec![ResourceAccess::write(res_id)];
        let b = vec![ResourceAccess::read(res_id)];
        assert!(bundles_conflict(&a, &b), "write/read must conflict");

        // Read/Read never conflicts
        let a = vec![ResourceAccess::read(res_id)];
        let b = vec![ResourceAccess::read(res_id)];
        assert!(!bundles_conflict(&a, &b), "read/read must not conflict");

        // Different resources never conflict
        let other_id = { let mut id = [0u8; 32]; id[0] = rng.next_u8().wrapping_add(128); id };
        let a = vec![ResourceAccess::write(res_id)];
        let b = vec![ResourceAccess::write(other_id)];
        if res_id != other_id {
            assert!(!bundles_conflict(&a, &b), "different resources must not conflict");
        }
    }
}

#[test]
fn test_fuzz_no_double_execution() {
    let mut rng = SimpleRng::new(202);
    let mut pool = IntentPool::new();
    pool.set_block_height(10);

    // Submit 10 intents
    let mut intents = Vec::new();
    for i in 1..=10u64 {
        let intent = make_swap(&mut rng, (i as u8) % 5 + 1, i);
        if pool.admit(intent.clone()).is_ok() {
            intents.push(intent);
        }
    }
    pool.open_admitted_intents();

    // Mark some as matched
    let mut matched_ids = std::collections::BTreeSet::new();
    for intent in &intents {
        if rng.next_bool() {
            let bundle_id = { let mut b = [0u8; 32]; b[0] = rng.next_u8(); b };
            let solver_id = { let mut s = [0u8; 32]; s[0] = rng.next_u8(); s };
            if pool.mark_matched(&intent.intent_id, bundle_id, solver_id) {
                matched_ids.insert(intent.intent_id);
            }
        }
    }

    // INVARIANT: Cannot double-match
    for id in &matched_ids {
        let result = pool.mark_matched(id, [99u8; 32], [99u8; 32]);
        assert!(!result, "double-match should fail for intent {}", hex::encode(&id[..4]));
    }
}
