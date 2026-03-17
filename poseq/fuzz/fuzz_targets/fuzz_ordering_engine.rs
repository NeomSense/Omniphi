#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::config::policy::{
    OrderingPolicyConfig, PoSeqPolicy, SubmissionClass,
};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::ordering::engine::OrderingEngine;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::SubmissionValidator;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn class_from_byte(b: u8) -> SubmissionClass {
    match b % 6 {
        0 => SubmissionClass::Transfer,
        1 => SubmissionClass::Swap,
        2 => SubmissionClass::YieldAllocate,
        3 => SubmissionClass::TreasuryRebalance,
        4 => SubmissionClass::GoalPacket,
        _ => SubmissionClass::AgentSubmission,
    }
}

fuzz_target!(|data: &[u8]| {
    if data.is_empty() {
        return;
    }

    // First byte: number of submissions (1–50, clamped)
    let num_submissions = (data[0] as usize % 50).max(1);

    // Each submission: 64 bytes = priority_hint(u16, 2 bytes) + sender(32 bytes) + class_byte(1 byte) + padding(29 bytes)
    const BYTES_PER_SUB: usize = 64;

    let policy = PoSeqPolicy::default_policy();
    let engine = OrderingEngine::new(OrderingPolicyConfig::default());

    // Parse up to num_submissions from the remaining bytes
    let payload = if data.len() > 1 { &data[1..] } else { &[] };

    let mut submissions = Vec::new();
    let mut receiver = SubmissionReceiver::new();

    for i in 0..num_submissions {
        let offset = i * BYTES_PER_SUB;

        let priority_hint: u32 = if offset + 2 <= payload.len() {
            let raw = u16::from_le_bytes([payload[offset], payload[offset + 1]]) as u32;
            raw % 10001 // clamp to valid range
        } else {
            (i as u32 * 1000) % 10001
        };

        let sender: [u8; 32] = if offset + 34 <= payload.len() {
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&payload[offset + 2..offset + 34]);
            // Ensure non-zero sender (set last byte if all zero)
            if arr == [0u8; 32] {
                arr[31] = 0x01;
            }
            arr
        } else {
            let mut arr = [0u8; 32];
            arr[0] = i as u8;
            arr[31] = 0x01;
            arr
        };

        let class_byte: u8 = if offset + 35 <= payload.len() {
            payload[offset + 34]
        } else {
            i as u8
        };
        let class = class_from_byte(class_byte);

        // Build a minimal payload with correct hash
        let body: Vec<u8> = {
            let mut v = vec![0u8; 32];
            v[0] = i as u8;
            v[1] = class.priority_weight() as u8;
            v[2] = (priority_hint & 0xFF) as u8;
            v[3] = 0xF0;
            v
        };
        let payload_hash: [u8; 32] = {
            let h = Sha256::digest(&body);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };

        let mut sub_id = [0u8; 32];
        sub_id[0] = i as u8;
        sub_id[1] = class.priority_weight() as u8;
        sub_id[31] = 0x01;

        let sub = SequencingSubmission {
            submission_id: sub_id,
            sender,
            kind: SubmissionKind::IntentTransaction,
            class,
            payload_hash,
            payload_body: body,
            nonce: i as u64,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: i as u64,
                priority_hint,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };

        let envelope = receiver.receive(sub);
        if let Ok(validated) = SubmissionValidator::validate(envelope, &policy) {
            submissions.push(validated);
        }
    }

    if submissions.is_empty() {
        return;
    }

    let input_count = submissions.len();

    // Run ordering once
    let result1 = engine.order(submissions.clone());
    let result2 = engine.order(submissions);

    match (result1, result2) {
        (Ok(ordered1), Ok(ordered2)) => {
            // Invariant 1: output length equals input length (no submissions dropped)
            assert_eq!(
                ordered1.len(),
                input_count,
                "ordering must not drop submissions: expected {} got {}",
                input_count,
                ordered1.len()
            );
            assert_eq!(
                ordered2.len(),
                input_count,
                "second run must also preserve count"
            );

            // Invariant 2: ordering is deterministic (same input → same output)
            let ids1: Vec<[u8; 32]> = ordered1
                .iter()
                .map(|s| s.envelope.normalized_id)
                .collect();
            let ids2: Vec<[u8; 32]> = ordered2
                .iter()
                .map(|s| s.envelope.normalized_id)
                .collect();
            assert_eq!(
                ids1, ids2,
                "ordering must be deterministic: two runs of same input must produce same output"
            );
        }
        (Err(_), Err(_)) => {
            // Both returned errors — acceptable (e.g. EmptyInput after filtering)
        }
        (Ok(_), Err(_)) | (Err(_), Ok(_)) => {
            // Ordering must be deterministic — if one errors, both must error
            panic!("ordering is non-deterministic: one run succeeded, other failed");
        }
    }
});
