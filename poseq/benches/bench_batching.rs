use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::batching::builder::BatchBuilder;
use omniphi_poseq::config::policy::{OrderingPolicyConfig, PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::ordering::engine::OrderingEngine;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn make_validated_submission(idx: u32, class: SubmissionClass) -> ValidatedSubmission {
    let payload_body = {
        let mut v = vec![0u8; 64];
        v[0] = (idx & 0xFF) as u8;
        v[1] = ((idx >> 8) & 0xFF) as u8;
        v[2] = ((idx >> 16) & 0xFF) as u8;
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
    sender[2] = 0x01;

    let mut submission_id = [0u8; 32];
    submission_id[0] = (idx & 0xFF) as u8;
    submission_id[1] = ((idx >> 8) & 0xFF) as u8;
    submission_id[2] = class.priority_weight() as u8;
    submission_id[31] = 0x01;

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
            priority_hint: 0,
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

fn make_ordered_submissions(count: usize) -> Vec<ValidatedSubmission> {
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());
    let subs: Vec<ValidatedSubmission> = (0..count as u32)
        .map(|i| make_validated_submission(i, class_for_idx(i)))
        .collect();
    engine.order(subs).expect("ordering must succeed")
}

fn bench_batching(c: &mut Criterion) {
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let mut group = c.benchmark_group("batch_builder");

    for size in [10, 100, 500] {
        let ordered = make_ordered_submissions(size);
        group.bench_with_input(
            BenchmarkId::new("build", size),
            &ordered,
            |b, subs| {
                b.iter(|| {
                    let cloned = subs.clone();
                    builder
                        .build(cloned, 1, None, 1, None)
                        .expect("build must succeed")
                });
            },
        );
    }

    group.finish();
}

fn bench_batch_id_computation(c: &mut Criterion) {
    use omniphi_poseq::batching::builder::OrderedBatch;

    let ordered = make_ordered_submissions(100);
    let policy = PoSeqPolicy::default_policy();
    let builder = BatchBuilder::new(policy);
    let batch = builder
        .build(ordered.clone(), 1, None, 1, None)
        .expect("build must succeed");

    c.bench_function("compute_payload_root_100", |b| {
        b.iter(|| OrderedBatch::compute_payload_root(&ordered));
    });

    c.bench_function("compute_batch_id_from_header", |b| {
        b.iter(|| OrderedBatch::compute_batch_id(&batch.header));
    });
}

fn bench_batching_varying_sizes(c: &mut Criterion) {
    let mut group = c.benchmark_group("batch_build_varying_batch_size");

    // Vary batch policy max size
    for max_sub in [50, 100, 256] {
        let mut policy = PoSeqPolicy::default_policy();
        policy.batch.max_submissions_per_batch = max_sub;
        let builder = BatchBuilder::new(policy);
        let count = max_sub.min(500);
        let ordered = make_ordered_submissions(count);

        group.bench_with_input(
            BenchmarkId::new("build_max", max_sub),
            &ordered,
            |b, subs| {
                b.iter(|| {
                    let cloned = subs.clone();
                    builder.build(cloned, 1, None, 1, None).expect("build must succeed")
                });
            },
        );
    }

    group.finish();
}

criterion_group!(
    benches,
    bench_batching,
    bench_batch_id_computation,
    bench_batching_varying_sizes
);
criterion_main!(benches);
