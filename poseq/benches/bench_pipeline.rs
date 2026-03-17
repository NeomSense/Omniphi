use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::PoSeqNode;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn make_raw_submission(idx: u32, class: SubmissionClass) -> SequencingSubmission {
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

    SequencingSubmission {
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
    }
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

fn build_node_with_submissions(count: usize) -> PoSeqNode {
    let mut policy = PoSeqPolicy::default_policy();
    policy.batch.max_submissions_per_batch = count.max(1);
    policy.batch.max_pending_queue_size = count + 100;
    let mut node = PoSeqNode::new(policy);
    for i in 0..count as u32 {
        let class = class_for_idx(i);
        let sub = make_raw_submission(i, class);
        node.submit(sub).expect("submit must succeed");
    }
    node
}

fn bench_full_pipeline(c: &mut Criterion) {
    let mut group = c.benchmark_group("pipeline_end_to_end");

    for count in [10, 100, 500] {
        group.bench_with_input(
            BenchmarkId::new("submit_and_produce", count),
            &count,
            |b, &n| {
                b.iter_batched(
                    || {
                        let mut policy = PoSeqPolicy::default_policy();
                        policy.batch.max_submissions_per_batch = n.max(1);
                        policy.batch.max_pending_queue_size = n + 100;
                        (policy, n)
                    },
                    |(policy, n)| {
                        let mut node = PoSeqNode::new(policy);
                        // Submit phase
                        for i in 0..n as u32 {
                            let class = class_for_idx(i);
                            let sub = make_raw_submission(i, class);
                            node.submit(sub).expect("submit must succeed");
                        }
                        // Produce batch phase (ordering + batching)
                        node.produce_batch(1, None).expect("produce_batch must succeed")
                    },
                    criterion::BatchSize::SmallInput,
                );
            },
        );
    }

    group.finish();
}

fn bench_pipeline_phases_separate(c: &mut Criterion) {
    let mut group = c.benchmark_group("pipeline_phases");

    let count = 100usize;

    // Phase: submit only
    group.bench_function("submit_phase_100", |b| {
        b.iter_batched(
            || {
                let mut policy = PoSeqPolicy::default_policy();
                policy.batch.max_submissions_per_batch = count;
                policy.batch.max_pending_queue_size = count + 100;
                let subs: Vec<SequencingSubmission> = (0..count as u32)
                    .map(|i| make_raw_submission(i, class_for_idx(i)))
                    .collect();
                (PoSeqNode::new(policy), subs)
            },
            |(mut node, subs)| {
                for sub in subs {
                    node.submit(sub).expect("submit must succeed");
                }
                node
            },
            criterion::BatchSize::SmallInput,
        );
    });

    // Phase: produce_batch only (ordering + batching after submissions already in queue)
    group.bench_function("produce_batch_phase_100", |b| {
        b.iter_batched(
            || build_node_with_submissions(count),
            |mut node| node.produce_batch(1, None).expect("produce_batch must succeed"),
            criterion::BatchSize::SmallInput,
        );
    });

    // Phase: export to runtime
    group.bench_function("export_to_runtime_phase_100", |b| {
        b.iter_batched(
            || {
                let mut node = build_node_with_submissions(count);
                let (batch, _) = node.produce_batch(1, None).expect("produce_batch must succeed");
                (node, batch)
            },
            |(node, batch)| node.export_to_runtime(&batch).expect("export must succeed"),
            criterion::BatchSize::SmallInput,
        );
    });

    group.finish();
}

fn bench_pipeline_throughput(c: &mut Criterion) {
    // Throughput: how many submissions per second across multiple produce_batch calls
    c.bench_function("pipeline_multi_batch_10x50", |b| {
        b.iter(|| {
            let mut policy = PoSeqPolicy::default_policy();
            policy.batch.max_submissions_per_batch = 50;
            policy.batch.max_pending_queue_size = 600;
            let mut node = PoSeqNode::new(policy);

            let mut global_idx = 0u32;
            for batch_num in 1u64..=10 {
                // Submit 50 fresh submissions for each batch
                for _ in 0..50 {
                    let class = class_for_idx(global_idx);
                    let sub = make_raw_submission(global_idx, class);
                    node.submit(sub).expect("submit must succeed");
                    global_idx += 1;
                }
                node.produce_batch(batch_num, None)
                    .expect("produce_batch must succeed");
            }
        });
    });
}

criterion_group!(
    benches,
    bench_full_pipeline,
    bench_pipeline_phases_separate,
    bench_pipeline_throughput
);
criterion_main!(benches);
