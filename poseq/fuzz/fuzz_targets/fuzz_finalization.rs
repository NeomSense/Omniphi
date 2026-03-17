#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::attestations::collector::{
    AttestationCollector, AttestationThreshold, BatchAttestationVote,
};
use omniphi_poseq::finalization::engine::{FinalizationDecision, FinalizationEngine};
use omniphi_poseq::proposals::batch::ProposedBatch;

/// Parse a u64 from 8 bytes at the given offset; returns 0 if out of bounds.
fn read_u64(data: &[u8], offset: usize) -> u64 {
    if offset + 8 <= data.len() {
        u64::from_le_bytes(data[offset..offset + 8].try_into().unwrap_or([0u8; 8]))
    } else {
        0
    }
}

/// Parse 32 bytes at offset; returns zero array if out of bounds.
fn read_id(data: &[u8], offset: usize) -> [u8; 32] {
    if offset + 32 <= data.len() {
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&data[offset..offset + 32]);
        arr
    } else {
        [0u8; 32]
    }
}

fuzz_target!(|data: &[u8]| {
    if data.len() < 8 {
        return;
    }

    // We test a series of finalization attempts.
    // Input layout per attempt (72 bytes):
    //   [0..8]   epoch (u64)
    //   [8..16]  slot (u64)
    //   [16..48] leader_id ([u8;32])
    //   [48..56] quorum_size (u64, clamped 1–20)
    //   [56..72] parent_batch_id ([u8;32])  (if available)

    const ATTEMPT_SIZE: usize = 72;

    let num_attempts = (data.len() / ATTEMPT_SIZE).min(8);
    if num_attempts == 0 {
        return;
    }

    let mut engine = FinalizationEngine::new();
    // Track which (slot, epoch) pairs we've successfully finalized
    let mut finalized_slots: std::collections::BTreeSet<(u64, u64)> = std::collections::BTreeSet::new();

    for attempt in 0..num_attempts {
        let offset = attempt * ATTEMPT_SIZE;

        let epoch = read_u64(data, offset) % 1000 + 1; // clamp: 1..=1000
        let slot = read_u64(data, offset + 8) % 1000 + 1; // clamp: 1..=1000
        let leader_id = read_id(data, offset + 16);
        let quorum_size_raw = read_u64(data, offset + 48);
        let quorum_size = ((quorum_size_raw % 20) + 1) as usize; // 1..=20
        let parent_batch_id = read_id(data, offset + 56);

        // Build a proposed batch
        let mut submission_ids: Vec<[u8; 32]> = Vec::new();
        for i in 0..3usize {
            let mut sid = [0u8; 32];
            sid[0] = i as u8;
            sid[1] = attempt as u8;
            sid[2] = (epoch & 0xFF) as u8;
            submission_ids.push(sid);
        }

        let proposed = ProposedBatch::new(
            slot,
            epoch,
            leader_id,
            submission_ids,
            parent_batch_id,
            1,
            100,
        );

        // Build an attestation collector with quorum_size approvals (always reaches quorum)
        let mut collector = AttestationCollector::new(proposed.proposal_id);
        for i in 0..quorum_size {
            let mut attestor_id = [0u8; 32];
            attestor_id[0] = i as u8;
            attestor_id[1] = attempt as u8;
            attestor_id[31] = 0x01;
            let vote = BatchAttestationVote::new(proposed.proposal_id, attestor_id, true, epoch);
            let _ = collector.add_vote(vote);
        }

        let threshold = AttestationThreshold::two_thirds(quorum_size);
        let height = attempt as u64 + 100;

        let decision = engine.finalize(&proposed, &collector, &threshold, quorum_size, height);

        match &decision {
            FinalizationDecision::Finalized(batch) => {
                let slot_key = (batch.slot, batch.epoch);

                // Invariant: same (slot, epoch) pair must never finalize twice
                assert!(
                    !finalized_slots.contains(&slot_key),
                    "double finalization detected for slot={} epoch={}: engine allowed it but our guard caught it",
                    batch.slot,
                    batch.epoch
                );
                finalized_slots.insert(slot_key);
            }
            FinalizationDecision::AlreadyFinalized => {
                // Acceptable: same proposal submitted twice
            }
            FinalizationDecision::SlotAlreadyFinalized { slot, epoch, .. } => {
                // Invariant: the engine must report this for slots we already finalized
                let slot_key = (*slot, *epoch);
                assert!(
                    finalized_slots.contains(&slot_key),
                    "engine reported SlotAlreadyFinalized for slot={} epoch={} but we never finalized it",
                    slot,
                    epoch
                );
            }
            FinalizationDecision::InsufficientAttestations => {
                // Acceptable: quorum not reached
            }
            FinalizationDecision::ConflictDetected(_) => {
                // Acceptable: conflicts in attestations
            }
        }
    }

    // Second pass: try to re-finalize every batch from the first pass.
    // All must return AlreadyFinalized or SlotAlreadyFinalized — never Finalized again.
    for attempt in 0..num_attempts {
        let offset = attempt * ATTEMPT_SIZE;

        let epoch = read_u64(data, offset) % 1000 + 1;
        let slot = read_u64(data, offset + 8) % 1000 + 1;
        let leader_id = read_id(data, offset + 16);
        let quorum_size_raw = read_u64(data, offset + 48);
        let quorum_size = ((quorum_size_raw % 20) + 1) as usize;
        let parent_batch_id = read_id(data, offset + 56);

        let slot_key = (slot, epoch);
        if !finalized_slots.contains(&slot_key) {
            continue; // wasn't finalized in first pass, skip
        }

        let mut submission_ids: Vec<[u8; 32]> = Vec::new();
        for i in 0..3usize {
            let mut sid = [0u8; 32];
            sid[0] = i as u8;
            sid[1] = attempt as u8;
            sid[2] = (epoch & 0xFF) as u8;
            submission_ids.push(sid);
        }

        let proposed = ProposedBatch::new(
            slot,
            epoch,
            leader_id,
            submission_ids,
            parent_batch_id,
            1,
            100,
        );

        let mut collector = AttestationCollector::new(proposed.proposal_id);
        for i in 0..quorum_size {
            let mut attestor_id = [0u8; 32];
            attestor_id[0] = i as u8;
            attestor_id[1] = attempt as u8;
            attestor_id[31] = 0x01;
            let vote = BatchAttestationVote::new(proposed.proposal_id, attestor_id, true, epoch);
            let _ = collector.add_vote(vote);
        }

        let threshold = AttestationThreshold::two_thirds(quorum_size);

        let decision = engine.finalize(&proposed, &collector, &threshold, quorum_size, 200);

        // Must not return Finalized again for an already-finalized (slot, epoch)
        assert!(
            !matches!(decision, FinalizationDecision::Finalized(_)),
            "double finalization: engine returned Finalized for slot={} epoch={} which was already finalized",
            slot,
            epoch
        );
    }
});
