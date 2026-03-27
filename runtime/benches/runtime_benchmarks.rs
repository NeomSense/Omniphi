//! Omniphi Runtime Performance Benchmarks
//!
//! Measures real throughput for the core execution pipeline.
//! Run with: cargo bench --bench runtime_benchmarks

use criterion::{criterion_group, criterion_main, Criterion, BenchmarkId, Throughput};
use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId};
use omniphi_runtime::objects::types::BalanceObject;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::scheduler::parallel::ParallelScheduler;
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::capabilities::spending::SpendCapabilityRegistry;
use omniphi_runtime::intents::base::*;
use omniphi_runtime::intents::encrypted::{EncryptedIntent, EncryptedIntentRegistry, IntentReveal};
use omniphi_runtime::intents::types::TransferIntent;
use omniphi_runtime::randomness::{EntropyEngine, RandomnessRequest, derive_randomness};
use omniphi_runtime::safety::deterministic::{DeterministicSafetyEngine, SafetyConfig};
use std::collections::BTreeMap;

fn oid(v: u32) -> ObjectId {
    let mut b = [0u8; 32];
    b[0..4].copy_from_slice(&v.to_be_bytes());
    ObjectId(b)
}

fn addr(v: u32) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0..4].copy_from_slice(&v.to_be_bytes());
    b
}

fn make_balance_store(num_accounts: u32) -> ObjectStore {
    let mut store = ObjectStore::new();
    let asset = [0xAA; 32];
    for i in 1..=num_accounts {
        let owner = addr(i);
        let id = oid(i);
        let bal = BalanceObject::new(id, owner, asset, 1_000_000, 1);
        store.insert(Box::new(bal));
    }
    store
}

// ─── Benchmark 1: Scheduler conflict detection ──────────────

fn bench_scheduler(c: &mut Criterion) {
    let mut group = c.benchmark_group("scheduler");

    for &n in &[10, 50, 100, 500, 1000] {
        group.throughput(Throughput::Elements(n as u64));
        group.bench_with_input(BenchmarkId::new("independent_plans", n), &n, |b, &n| {
            let plans: Vec<ExecutionPlan> = (0..n).map(|i| ExecutionPlan {
                tx_id: { let mut t = [0u8; 32]; t[0..4].copy_from_slice(&(i as u32).to_be_bytes()); t },
                operations: vec![],
                required_capabilities: vec![],
                object_access: vec![
                    ObjectAccess { object_id: oid(i as u32 * 2), mode: AccessMode::ReadWrite },
                    ObjectAccess { object_id: oid(i as u32 * 2 + 1), mode: AccessMode::ReadWrite },
                ],
                gas_estimate: 1000,
                gas_limit: u64::MAX,
            }).collect();
            b.iter(|| ParallelScheduler::schedule(plans.clone()));
        });

        group.bench_with_input(BenchmarkId::new("all_conflicting", n), &n, |b, &n| {
            let shared = oid(9999);
            let plans: Vec<ExecutionPlan> = (0..n).map(|i| ExecutionPlan {
                tx_id: { let mut t = [0u8; 32]; t[0..4].copy_from_slice(&(i as u32).to_be_bytes()); t },
                operations: vec![],
                required_capabilities: vec![],
                object_access: vec![
                    ObjectAccess { object_id: shared, mode: AccessMode::ReadWrite },
                ],
                gas_estimate: 1000,
                gas_limit: u64::MAX,
            }).collect();
            b.iter(|| ParallelScheduler::schedule(plans.clone()));
        });
    }
    group.finish();
}

// ─── Benchmark 2: Settlement engine ─────────────────────────

fn bench_settlement(c: &mut Criterion) {
    let mut group = c.benchmark_group("settlement");

    for &n in &[10, 50, 100, 500] {
        group.throughput(Throughput::Elements(n as u64));
        group.bench_with_input(BenchmarkId::new("transfer_plans", n), &n, |b, &n| {
            b.iter_with_setup(|| {
                let mut store = make_balance_store(n as u32 * 2);
                let plans: Vec<ExecutionPlan> = (0..n).map(|i| {
                    let sender_id = oid(i as u32 * 2 + 1);
                    let recip_id = oid(i as u32 * 2 + 2);
                    ExecutionPlan {
                        tx_id: { let mut t = [0u8; 32]; t[0..4].copy_from_slice(&(i as u32).to_be_bytes()); t },
                        operations: vec![
                            ObjectOperation::DebitBalance { object_id: sender_id, amount: 100 },
                            ObjectOperation::CreditBalance { object_id: recip_id, amount: 100 },
                        ],
                        required_capabilities: vec![],
                        object_access: vec![
                            ObjectAccess { object_id: sender_id, mode: AccessMode::ReadWrite },
                            ObjectAccess { object_id: recip_id, mode: AccessMode::ReadWrite },
                        ],
                        gas_estimate: 2000,
                        gas_limit: u64::MAX,
                    }
                }).collect();
                let groups = ParallelScheduler::schedule(plans);
                (store, groups)
            }, |(mut store, groups)| {
                SettlementEngine::execute_groups(groups, &mut store, 1);
            });
        });
    }
    group.finish();
}

// ─── Benchmark 3: Capability operations ─────────────────────

fn bench_capabilities(c: &mut Criterion) {
    let mut group = c.benchmark_group("capabilities");

    for &n in &[100, 1000, 10000] {
        group.bench_with_input(BenchmarkId::new("create_and_consume", n), &n, |b, &n| {
            b.iter_with_setup(|| {
                SpendCapabilityRegistry::new()
            }, |mut reg| {
                for i in 0..n {
                    let id = reg.create(addr(1), addr(2), [0xAA; 32], 1000, 100, None, 10).unwrap();
                    reg.consume(&id, &addr(2), 500, 20, None).unwrap();
                }
            });
        });

        group.bench_with_input(BenchmarkId::new("expire_stale", n), &n, |b, &n| {
            b.iter_with_setup(|| {
                let mut reg = SpendCapabilityRegistry::new();
                for i in 0..n {
                    reg.create(addr(i as u32), addr(2), [0xAA; 32], 1000, 50, None, 10).unwrap();
                }
                reg
            }, |mut reg| {
                reg.expire_stale(60);
            });
        });
    }
    group.finish();
}

// ─── Benchmark 4: Encrypted intent lifecycle ────────────────

fn bench_encrypted_intents(c: &mut Criterion) {
    let mut group = c.benchmark_group("encrypted_intents");

    for &n in &[100, 1000] {
        group.bench_with_input(BenchmarkId::new("submit_and_reveal", n), &n, |b, &n| {
            b.iter_with_setup(|| {
                let intents: Vec<_> = (0..n).map(|i| {
                    let mut tx = IntentTransaction {
                        tx_id: { let mut t = [0u8; 32]; t[0..4].copy_from_slice(&(i as u32).to_be_bytes()); t },
                        sender: addr(1),
                        intent: IntentType::Transfer(TransferIntent {
                            asset_id: [0xAA; 32], recipient: addr(2), amount: 100, memo: None,
                        }),
                        max_fee: 100, deadline_epoch: 999, nonce: i as u64,
                        signature: [0u8; 64], metadata: BTreeMap::new(),
                        target_objects: vec![], constraints: IntentConstraints::default(),
                        execution_mode: ExecutionMode::BestEffort,
                        sponsor: None, sponsor_signature: None,
                        sponsorship_limits: SponsorshipLimits::default(),
                        fee_policy: FeePolicy::SenderPays,
                    };
                    let nonce = { let mut n = [0u8; 32]; n[0..4].copy_from_slice(&(i as u32).to_be_bytes()); n };
                    (tx, nonce)
                }).collect();
                (EncryptedIntentRegistry::new(), intents)
            }, |(mut reg, intents)| {
                for (tx, nonce) in &intents {
                    let ei = EncryptedIntent::create(tx, nonce, vec![0xDE], 100, 10).unwrap();
                    let commitment = reg.submit(ei).unwrap();
                    let reveal = IntentReveal {
                        commitment, plaintext_intent: tx.clone(), reveal_nonce: *nonce,
                    };
                    reg.reveal(&reveal, 50).unwrap();
                }
            });
        });
    }
    group.finish();
}

// ─── Benchmark 5: Randomness derivation ─────────────────────

fn bench_randomness(c: &mut Criterion) {
    let mut group = c.benchmark_group("randomness");

    group.bench_function("derive_1000", |b| {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, [0xAA; 32]);
        let entropy = engine.aggregate(10);

        b.iter(|| {
            for i in 0..1000u64 {
                let req = RandomnessRequest {
                    domain: "bench".to_string(), epoch: 10, index: i, target_id: None,
                };
                derive_randomness(&entropy, &req);
            }
        });
    });

    group.finish();
}

// ─── Benchmark 6: Safety engine throughput ──────────────────

fn bench_safety(c: &mut Criterion) {
    let mut group = c.benchmark_group("safety");

    group.bench_function("record_outflow_10000", |b| {
        b.iter_with_setup(|| {
            DeterministicSafetyEngine::new(SafetyConfig::default())
        }, |mut engine| {
            for i in 0..10000u32 {
                engine.record_outflow(1, oid(i), 100);
            }
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_scheduler,
    bench_settlement,
    bench_capabilities,
    bench_encrypted_intents,
    bench_randomness,
    bench_safety,
);
criterion_main!(benches);
