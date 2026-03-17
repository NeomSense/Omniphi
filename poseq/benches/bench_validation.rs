use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion};
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionEnvelope, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::SubmissionValidator;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn make_valid_envelope(idx: u32, payload_size: usize) -> SubmissionEnvelope {
    let payload_body: Vec<u8> = {
        let mut v = vec![0xA5u8; payload_size.max(8)];
        v[0] = (idx & 0xFF) as u8;
        v[1] = ((idx >> 8) & 0xFF) as u8;
        v[2] = ((idx >> 16) & 0xFF) as u8;
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
    sender[1] = 0x01;

    let mut submission_id = [0u8; 32];
    submission_id[0] = (idx & 0xFF) as u8;
    submission_id[1] = ((idx >> 8) & 0xFF) as u8;
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

    let mut receiver = SubmissionReceiver::new();
    receiver.receive(sub)
}

fn make_invalid_hash_envelope(idx: u32) -> SubmissionEnvelope {
    let payload_body: Vec<u8> = {
        let mut v = vec![0xA5u8; 64];
        v[0] = (idx & 0xFF) as u8;
        v
    };
    // Corrupt the hash — XOR all bytes to ensure mismatch
    let payload_hash = {
        let h = Sha256::digest(&payload_body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        for byte in arr.iter_mut() {
            *byte ^= 0xFF;
        }
        arr
    };
    let mut sender = [0u8; 32];
    sender[0] = (idx & 0xFF) as u8;
    sender[1] = 0x01;

    let mut submission_id = [0u8; 32];
    submission_id[0] = (idx & 0xFF) as u8;
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

    let mut receiver = SubmissionReceiver::new();
    receiver.receive(sub)
}

fn bench_validation_valid(c: &mut Criterion) {
    let policy = PoSeqPolicy::default_policy();
    let mut group = c.benchmark_group("validator_valid");

    // Valid submission — fast path
    group.bench_function("valid_32b_payload", |b| {
        b.iter_batched(
            || make_valid_envelope(1, 32),
            |envelope| SubmissionValidator::validate(envelope, &policy),
            criterion::BatchSize::SmallInput,
        );
    });

    group.finish();
}

fn bench_validation_invalid(c: &mut Criterion) {
    let policy = PoSeqPolicy::default_policy();
    let mut group = c.benchmark_group("validator_invalid");

    // Invalid payload_hash — should fail early (hash mismatch)
    group.bench_function("invalid_payload_hash", |b| {
        b.iter_batched(
            || make_invalid_hash_envelope(1),
            |envelope| SubmissionValidator::validate(envelope, &policy),
            criterion::BatchSize::SmallInput,
        );
    });

    // Zero sender — fails at validation step 2 (very fast)
    group.bench_function("zero_sender_rejection", |b| {
        b.iter_batched(
            || {
                let payload_body = vec![0xA5u8; 32];
                let payload_hash = {
                    let h = Sha256::digest(&payload_body);
                    let mut arr = [0u8; 32];
                    arr.copy_from_slice(&h);
                    arr
                };
                let sub = SequencingSubmission {
                    submission_id: [0xABu8; 32],
                    sender: [0u8; 32], // zero — invalid
                    kind: SubmissionKind::IntentTransaction,
                    class: SubmissionClass::Transfer,
                    payload_hash,
                    payload_body,
                    nonce: 1,
                    max_fee: 100,
                    deadline_epoch: 9999,
                    metadata: SubmissionMetadata {
                        sequence_hint: 1,
                        priority_hint: 0,
                        solver_id: None,
                        domain_tag: None,
                        extra: BTreeMap::new(),
                    },
                    signature: [0u8; 64],
                };
                let mut receiver = SubmissionReceiver::new();
                receiver.receive(sub)
            },
            |envelope| SubmissionValidator::validate(envelope, &policy),
            criterion::BatchSize::SmallInput,
        );
    });

    group.finish();
}

fn bench_validation_payload_sizes(c: &mut Criterion) {
    let policy = PoSeqPolicy::default_policy();
    let mut group = c.benchmark_group("validator_payload_size");

    for (label, size) in [("1kb", 1024usize), ("4kb", 4096), ("16kb", 16384)] {
        group.bench_with_input(
            BenchmarkId::new("valid", label),
            &size,
            |b, &sz| {
                b.iter_batched(
                    || make_valid_envelope(1, sz),
                    |envelope| SubmissionValidator::validate(envelope, &policy),
                    criterion::BatchSize::SmallInput,
                );
            },
        );
    }

    // Oversized payload — should be rejected at size check (before hash validation)
    group.bench_function("oversized_rejection", |b| {
        b.iter_batched(
            || make_valid_envelope(1, 128 * 1024), // 128KB > policy max 64KB
            |envelope| SubmissionValidator::validate(envelope, &policy),
            criterion::BatchSize::SmallInput,
        );
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_validation_valid,
    bench_validation_invalid,
    bench_validation_payload_sizes
);
criterion_main!(benches);
