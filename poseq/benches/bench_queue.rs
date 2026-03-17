use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::queue::pending::{ReplayGuard, SubmissionQueue};
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::{SubmissionValidator, ValidatedSubmission};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn make_validated_submission(idx: u32) -> ValidatedSubmission {
    let payload_body = {
        let mut v = vec![0u8; 32];
        v[0] = (idx & 0xFF) as u8;
        v[1] = ((idx >> 8) & 0xFF) as u8;
        v[2] = ((idx >> 16) & 0xFF) as u8;
        v[3] = 0x42;
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
    submission_id[2] = 0xBB;
    submission_id[31] = 0x01;

    let sub = SequencingSubmission {
        submission_id,
        sender,
        kind: SubmissionKind::IntentTransaction,
        class: SubmissionClass::Transfer,
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

fn prefill_queue(size: usize) -> SubmissionQueue {
    let mut queue = SubmissionQueue::new(0); // unlimited
    for i in 0..size as u32 {
        let vs = make_validated_submission(i);
        queue.push(vs).expect("push must succeed");
    }
    queue
}

fn bench_queue_drain(c: &mut Criterion) {
    let mut group = c.benchmark_group("queue_drain");

    for queue_size in [100, 1000, 10000] {
        group.bench_with_input(
            BenchmarkId::new("drain_all", queue_size),
            &queue_size,
            |b, &size| {
                b.iter_batched(
                    || prefill_queue(size),
                    |mut queue| queue.drain(size),
                    criterion::BatchSize::SmallInput,
                );
            },
        );

        // Drain half
        group.bench_with_input(
            BenchmarkId::new("drain_half", queue_size),
            &queue_size,
            |b, &size| {
                b.iter_batched(
                    || prefill_queue(size),
                    |mut queue| queue.drain(size / 2),
                    criterion::BatchSize::SmallInput,
                );
            },
        );
    }

    group.finish();
}

fn bench_replay_guard(c: &mut Criterion) {
    let mut group = c.benchmark_group("replay_guard");

    // Insert + lookup pattern bounded to 1000 entries
    group.bench_function("check_and_record_bounded_1000", |b| {
        let mut guard = ReplayGuard::new(1000);
        let mut counter = 0u32;
        b.iter(|| {
            let mut id = [0u8; 32];
            id[0] = (counter & 0xFF) as u8;
            id[1] = ((counter >> 8) & 0xFF) as u8;
            id[2] = ((counter >> 16) & 0xFF) as u8;
            id[3] = 0xAA;
            // Always use a fresh ID so we always succeed (no duplicates)
            id[28] = (counter >> 24) as u8;
            id[29] = (counter >> 16) as u8;
            id[30] = (counter >> 8) as u8;
            id[31] = (counter & 0xFF) as u8;
            counter = counter.wrapping_add(1);
            let _ = guard.check_and_record(id);
        });
    });

    // Unlimited replay guard
    group.bench_function("check_and_record_unlimited", |b| {
        let mut guard = ReplayGuard::new(0);
        let mut counter = 0u32;
        b.iter(|| {
            let mut id = [0u8; 32];
            id[0] = (counter & 0xFF) as u8;
            id[1] = ((counter >> 8) & 0xFF) as u8;
            id[28] = (counter >> 24) as u8;
            id[31] = (counter & 0xFF) as u8;
            counter = counter.wrapping_add(1);
            let _ = guard.check_and_record(id);
        });
    });

    // Duplicate detection (always checking the same ID)
    group.bench_function("check_duplicate_detection", |b| {
        let mut guard = ReplayGuard::new(0);
        let id = [0xABu8; 32];
        let _ = guard.check_and_record(id);
        b.iter(|| {
            // Should always return Err(Duplicate) — measure rejection cost
            let _ = guard.check_and_record(id);
        });
    });

    group.finish();
}

fn bench_queue_churn(c: &mut Criterion) {
    // Realistic producer/consumer: insert 1000, drain 500, insert 500 more
    c.bench_function("queue_churn_1000_500_500", |b| {
        b.iter(|| {
            let mut queue = SubmissionQueue::new(0);

            // Phase 1: insert 1000
            for i in 0..1000u32 {
                let vs = make_validated_submission(i);
                queue.push(vs).expect("push must succeed");
            }

            // Phase 2: drain 500
            let _drained = queue.drain(500);

            // Phase 3: insert 500 more (different IDs to avoid queue key collision)
            for i in 1000..1500u32 {
                let vs = make_validated_submission(i);
                queue.push(vs).expect("push must succeed");
            }

            // Phase 4: drain remaining
            queue.drain(1000)
        });
    });
}

criterion_group!(
    benches,
    bench_queue_drain,
    bench_replay_guard,
    bench_queue_churn
);
criterion_main!(benches);
