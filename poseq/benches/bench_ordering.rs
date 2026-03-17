use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::config::policy::{OrderingPolicyConfig, PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::ordering::engine::OrderingEngine;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn make_validated_submission(
    idx: u32,
    class: SubmissionClass,
    priority: u32,
) -> ValidatedSubmission {
    let payload_body = {
        let mut v = vec![0u8; 32];
        v[0] = (idx & 0xFF) as u8;
        v[1] = (idx >> 8) as u8;
        v[2] = (idx >> 16) as u8;
        v[3] = class.priority_weight() as u8;
        v
    };
    let payload_hash = {
        let h = Sha256::digest(&payload_body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };
    let mut sender = [0u8; 32];
    sender[0] = (idx & 0xFF) as u8;
    sender[1] = ((idx >> 8) & 0xFF) as u8;
    sender[2] = ((idx >> 16) & 0xFF) as u8;
    sender[3] = 0x01; // ensure non-zero

    let mut submission_id = [0u8; 32];
    submission_id[0] = (idx & 0xFF) as u8;
    submission_id[1] = ((idx >> 8) & 0xFF) as u8;
    submission_id[2] = ((idx >> 16) & 0xFF) as u8;
    submission_id[3] = class.priority_weight() as u8;
    submission_id[31] = 0x01; // ensure non-zero

    let sub = SequencingSubmission {
        submission_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class,
        payload_hash,
        payload_body,
        nonce: idx as u64,
        max_fee: 100,
        deadline_epoch: 9999,
        metadata: SubmissionMetadata {
            sequence_hint: idx as u64,
            priority_hint: priority,
            solver_id: None,
            domain_tag: None,
            extra: BTreeMap::new(),
        },
        signature: [0u8; 64],
    };

    let policy = PoSeqPolicy::default_policy();
    let mut receiver = SubmissionReceiver::new();
    let envelope = receiver.receive(sub);
    SubmissionValidator::validate(envelope, &policy).expect("valid submission")
}

fn class_for_idx(idx: u32) -> SubmissionClass {
    match idx % 6 {
        0 => SubmissionClass::Transfer,
        1 => SubmissionClass::Swap,
        2 => SubmissionClass::YieldAllocate,
        3 => SubmissionClass::TreasuryRebalance,
        4 => SubmissionClass::GoalPacket,
        _ => SubmissionClass::AgentSubmission,
    }
}

fn make_submission_batch(count: usize) -> Vec<ValidatedSubmission> {
    (0..count as u32)
        .map(|idx| {
            let class = class_for_idx(idx);
            let priority = (idx * 7) % 10001;
            make_validated_submission(idx, class, priority)
        })
        .collect()
}

fn bench_ordering(c: &mut Criterion) {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());
    let mut group = c.benchmark_group("ordering_engine");

    for size in [10, 100, 1000, 5000] {
        let submissions = make_submission_batch(size);
        group.bench_with_input(
            BenchmarkId::new("order", size),
            &submissions,
            |b, subs| {
                b.iter(|| {
                    let cloned = subs.clone();
                    engine.order(cloned).expect("ordering must succeed")
                });
            },
        );
    }

    group.finish();
}

fn bench_ordering_by_class(c: &mut Criterion) {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());
    let mut group = c.benchmark_group("ordering_by_class");

    // All same class
    let all_transfer: Vec<ValidatedSubmission> = (0..100u32)
        .map(|i| make_validated_submission(i, SubmissionClass::Transfer, i * 10))
        .collect();
    group.bench_function("all_transfer_100", |b| {
        b.iter(|| {
            let cloned = all_transfer.clone();
            engine.order(cloned).expect("ordering must succeed")
        });
    });

    // Adversarial: all same priority
    let all_same_priority: Vec<ValidatedSubmission> = (0..100u32)
        .map(|i| make_validated_submission(i, SubmissionClass::Swap, 5000))
        .collect();
    group.bench_function("all_same_priority_100", |b| {
        b.iter(|| {
            let cloned = all_same_priority.clone();
            engine.order(cloned).expect("ordering must succeed")
        });
    });

    // Max priority for all
    let all_max_priority: Vec<ValidatedSubmission> = (0..100u32)
        .map(|i| make_validated_submission(i, SubmissionClass::TreasuryRebalance, 10000))
        .collect();
    group.bench_function("all_max_priority_100", |b| {
        b.iter(|| {
            let cloned = all_max_priority.clone();
            engine.order(cloned).expect("ordering must succeed")
        });
    });

    group.finish();
}

criterion_group!(benches, bench_ordering, bench_ordering_by_class);
criterion_main!(benches);
