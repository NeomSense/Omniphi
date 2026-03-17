#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::config::policy::{PoSeqPolicy, SubmissionClass};
use omniphi_poseq::intake::receiver::SubmissionReceiver;
use omniphi_poseq::types::submission::{
    SequencingSubmission, SubmissionKind, SubmissionMetadata,
};
use omniphi_poseq::validation::validator::SubmissionValidator;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fuzz_target!(|data: &[u8]| {
    // Need at least 72 bytes to parse the fixed fields; shorter inputs use defaults.
    let submission_id: [u8; 32] = if data.len() >= 32 {
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&data[0..32]);
        arr
    } else {
        [0x01u8; 32]
    };

    let sender: [u8; 32] = if data.len() >= 64 {
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&data[32..64]);
        arr
    } else {
        [0x01u8; 32]
    };

    let nonce: u64 = if data.len() >= 72 {
        u64::from_le_bytes(data[64..72].try_into().unwrap_or([0u8; 8]))
    } else {
        1
    };

    let payload_body: Vec<u8> = if data.len() > 72 {
        data[72..].to_vec()
    } else {
        vec![0u8; 16]
    };

    // Compute correct payload_hash from payload_body
    let correct_payload_hash: [u8; 32] = {
        let h = Sha256::digest(&payload_body);
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&h);
        arr
    };

    let class = SubmissionClass::Transfer;
    let policy = PoSeqPolicy::default_policy();

    // ── Test 1: Valid hash path ────────────────────────────────────────────────
    {
        let sub = SequencingSubmission {
            submission_id,
            sender,
            kind: SubmissionKind::IntentTransaction,
            class: class.clone(),
            payload_hash: correct_payload_hash,
            payload_body: payload_body.clone(),
            nonce,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: nonce,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };

        let mut receiver = SubmissionReceiver::new();
        let envelope = receiver.receive(sub);
        let result = SubmissionValidator::validate(envelope, &policy);

        if let Ok(ref validated) = result {
            // Invariant: if validate() returns Ok, then:
            // 1. sender must be non-zero
            assert_ne!(
                validated.envelope.submission.sender,
                [0u8; 32],
                "validated sender must be non-zero"
            );
            // 2. submission_id must be non-zero
            assert_ne!(
                validated.envelope.submission.submission_id,
                [0u8; 32],
                "validated submission_id must be non-zero"
            );
            // 3. payload_hash must match SHA256(payload_body)
            assert!(
                validated.envelope.submission.validate_payload_hash(),
                "payload_hash must match SHA256(payload_body) after successful validation"
            );
            // 4. payload_hash must not be all zeros
            assert_ne!(
                validated.envelope.submission.payload_hash,
                [0u8; 32],
                "validated payload_hash must be non-zero"
            );
        }
        // Error results are fine — they must not panic.
    }

    // ── Test 2: Corrupted hash path ────────────────────────────────────────────
    // XOR all bytes of the correct hash to ensure mismatch
    {
        let corrupted_hash: [u8; 32] = {
            let mut h = correct_payload_hash;
            // If correct hash is all zeros (edge case), use a known non-zero corrupt value
            for byte in h.iter_mut() {
                *byte ^= 0xFF;
            }
            h
        };

        let sub = SequencingSubmission {
            submission_id,
            sender,
            kind: SubmissionKind::IntentTransaction,
            class: class.clone(),
            payload_hash: corrupted_hash,
            payload_body: payload_body.clone(),
            nonce,
            max_fee: 100,
            deadline_epoch: 9999,
            metadata: SubmissionMetadata {
                sequence_hint: nonce,
                priority_hint: 0,
                solver_id: None,
                domain_tag: None,
                extra: BTreeMap::new(),
            },
            signature: [0u8; 64],
        };

        let mut receiver = SubmissionReceiver::new();
        let envelope = receiver.receive(sub);
        let result = SubmissionValidator::validate(envelope, &policy);

        // The corrupted hash must cause rejection (unless sender or submission_id is zero,
        // which causes an earlier rejection). Either way it must not return Ok.
        // Exception: if submission_id or sender happens to be all-zeros, earlier checks fire.
        // We only enforce the hash-mismatch rejection when submission_id and sender are both
        // non-zero AND the hash was actually changed.
        if submission_id != [0u8; 32]
            && sender != [0u8; 32]
            && corrupted_hash != [0u8; 32]
            && corrupted_hash != correct_payload_hash
        {
            assert!(
                result.is_err(),
                "corrupted payload_hash must cause validation failure"
            );
        }
    }
});
