//! Performance benchmarks for the intent execution pipeline — Phase G.

use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::intent_pool::types::*;
use omniphi_poseq::intent_pool::pool::IntentPool;
use omniphi_poseq::auction::types::*;
use omniphi_poseq::auction::state::AuctionWindow;
use omniphi_poseq::auction::ordering::order_bundles;
use std::collections::BTreeMap;

fn make_asset(b: u8) -> AssetId {
    let mut id = [0u8; 32]; id[0] = b;
    AssetId::token(id)
}

fn make_swap(user_byte: u8, nonce: u64) -> IntentTransaction {
    let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
    let swap = SwapIntent {
        asset_in: make_asset(1), asset_out: make_asset(2),
        amount_in: 1000, min_amount_out: 950, max_slippage_bps: 50, route_hint: None,
    };
    let mut intent = IntentTransaction {
        intent_id: [0u8; 32], intent_type: IntentKind::Swap(swap), version: 1,
        user, nonce, recipient: None, deadline: 100_000, valid_from: None,
        max_fee: 100, tip: Some(50), partial_fill_allowed: false,
        solver_permissions: SolverPermissions::default(),
        execution_preferences: ExecutionPrefs::default(),
        witness_hash: None, signature: vec![1u8; 64], metadata: BTreeMap::new(),
    };
    intent.intent_id = intent.compute_intent_id();
    intent
}

fn make_reveal(bundle_byte: u8, intent_id: [u8; 32], amount_out: u128) -> BundleReveal {
    let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte;
    let mut solver_id = [0u8; 32]; solver_id[0] = bundle_byte + 100;
    BundleReveal {
        bundle_id, solver_id, batch_window: 1,
        target_intent_ids: vec![intent_id],
        execution_steps: vec![ExecutionStep {
            step_index: 0, operation: OperationType::Debit, object_id: [10u8; 32],
            read_set: vec![[10u8; 32]], write_set: vec![[10u8; 32]],
            params: OperationParams { asset: Some(make_asset(1)), amount: Some(1000),
                recipient: None, pool_id: None, custom_data: None },
        }],
        liquidity_sources: vec![],
        predicted_outputs: vec![PredictedOutput {
            intent_id, asset_out: make_asset(2), amount_out, fee_charged_bps: 30,
        }],
        fee_breakdown: FeeBreakdown { solver_fee_bps: 15, protocol_fee_bps: 15, total_fee_bps: 30 },
        resource_declarations: vec![],
        nonce: [bundle_byte; 32], proof_data: vec![], signature: vec![1u8; 64],
    }
}

fn bench_intent_pool_admission(c: &mut Criterion) {
    let mut group = c.benchmark_group("intent_pool_admission");

    for count in [100, 1000, 5000] {
        group.bench_with_input(
            BenchmarkId::new("admit", count),
            &count,
            |b, &n| {
                b.iter_batched(
                    || {
                        let intents: Vec<IntentTransaction> = (0..n as u8)
                            .flat_map(|user| (1..=(n / 256 + 1) as u64).map(move |nonce| make_swap(user, nonce)))
                            .take(n)
                            .collect();
                        (IntentPool::new(), intents)
                    },
                    |(mut pool, intents)| {
                        pool.set_block_height(10);
                        for intent in intents {
                            let _ = pool.admit(intent);
                        }
                        pool
                    },
                    criterion::BatchSize::SmallInput,
                );
            },
        );
    }
    group.finish();
}

fn bench_ordering(c: &mut Criterion) {
    let mut group = c.benchmark_group("intent_ordering");

    for solver_count in [10, 50, 100] {
        group.bench_with_input(
            BenchmarkId::new("order_bundles", solver_count),
            &solver_count,
            |b, &n| {
                let intent = make_swap(1, 1);
                let reveals: Vec<BundleReveal> = (0..n as u8)
                    .map(|i| make_reveal(i + 1, intent.intent_id, 950 + i as u128))
                    .collect();
                let refs: Vec<&BundleReveal> = reveals.iter().collect();

                b.iter(|| order_bundles(&refs, 1));
            },
        );
    }
    group.finish();
}

fn bench_commit_reveal(c: &mut Criterion) {
    let mut group = c.benchmark_group("commit_reveal");

    for count in [10, 50, 100] {
        group.bench_with_input(
            BenchmarkId::new("full_window", count),
            &count,
            |b, &n| {
                let intent = make_swap(1, 1);
                let pairs: Vec<(BundleCommitment, BundleReveal)> = (0..n as u8)
                    .map(|i| {
                        let reveal = make_reveal(i + 1, intent.intent_id, 950 + i as u128);
                        let commitment = BundleCommitment {
                            bundle_id: reveal.bundle_id, solver_id: reveal.solver_id,
                            batch_window: 1, target_intent_count: 1,
                            commitment_hash: reveal.compute_commitment_hash(),
                            expected_outputs_hash: reveal.compute_expected_outputs_hash(),
                            execution_plan_hash: reveal.compute_execution_plan_hash(),
                            valid_until: 200, bond_locked: 50_000, signature: vec![1u8; 64],
                        };
                        (commitment, reveal)
                    })
                    .collect();

                b.iter(|| {
                    let mut auction = AuctionWindow::new(1, 100);
                    for (c, _) in &pairs {
                        let _ = auction.record_commitment(c.clone(), 102);
                    }
                    for (_, r) in &pairs {
                        let _ = auction.record_reveal(r.clone(), 106);
                    }
                    auction
                });
            },
        );
    }
    group.finish();
}

fn bench_hash_verification(c: &mut Criterion) {
    let intent = make_swap(1, 1);
    let reveal = make_reveal(1, intent.intent_id, 960);
    let commitment = BundleCommitment {
        bundle_id: reveal.bundle_id, solver_id: reveal.solver_id,
        batch_window: 1, target_intent_count: 1,
        commitment_hash: reveal.compute_commitment_hash(),
        expected_outputs_hash: reveal.compute_expected_outputs_hash(),
        execution_plan_hash: reveal.compute_execution_plan_hash(),
        valid_until: 200, bond_locked: 50_000, signature: vec![1u8; 64],
    };

    c.bench_function("commitment_hash_verify", |b| {
        b.iter(|| reveal.verify_against_commitment(&commitment))
    });
}

criterion_group!(
    benches,
    bench_intent_pool_admission,
    bench_ordering,
    bench_commit_reveal,
    bench_hash_verification,
);
criterion_main!(benches);
