#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::chain_bridge::evidence::{DuplicateEvidenceGuard, EvidencePacket};
use omniphi_poseq::misbehavior::types::MisbehaviorType;

fn read_u64(data: &[u8], offset: usize) -> u64 {
    if offset + 8 <= data.len() {
        u64::from_le_bytes(data[offset..offset + 8].try_into().unwrap_or([0u8; 8]))
    } else {
        0
    }
}

fn read_id(data: &[u8], offset: usize) -> [u8; 32] {
    if offset + 32 <= data.len() {
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&data[offset..offset + 32]);
        arr
    } else {
        [0u8; 32]
    }
}

fn misbehavior_from_byte(b: u8) -> MisbehaviorType {
    match b % 15 {
        0 => MisbehaviorType::Equivocation,
        1 => MisbehaviorType::InvalidProposalAuthority,
        2 => MisbehaviorType::InvalidFairnessEnvelope,
        3 => MisbehaviorType::InvalidAttestation,
        4 => MisbehaviorType::DuplicateAttestationAbuse,
        5 => MisbehaviorType::PersistentOmission,
        6 => MisbehaviorType::InvalidBatchDeliveryAttempt,
        7 => MisbehaviorType::RuntimeBridgeAbuse,
        8 => MisbehaviorType::StaleCommitteeParticipation,
        9 => MisbehaviorType::SlotHijackingAttempt,
        10 => MisbehaviorType::BoundaryTransitionAbuse,
        11 => MisbehaviorType::RepeatedInvalidProposalSpam,
        12 => MisbehaviorType::FairnessViolation,
        13 => MisbehaviorType::ReplayAttack,
        _ => MisbehaviorType::AbsentFromDuty,
    }
}

fuzz_target!(|data: &[u8]| {
    // Input layout:
    //   [0]      misbehavior type byte
    //   [1..33]  offender_node_id
    //   [33..41] epoch (u64)
    //   [41..49] slot (u64)
    //   remaining: evidence_hashes (32 bytes each, up to 10)

    if data.is_empty() {
        return;
    }

    let mtype = misbehavior_from_byte(data[0]);
    let node_id = read_id(data, 1);
    let epoch = read_u64(data, 33);
    let slot = read_u64(data, 41);

    // Parse evidence hashes from the remainder
    let remaining = if data.len() > 49 { &data[49..] } else { &[] };
    let num_hashes = (remaining.len() / 32).min(10);
    let mut evidence_hashes: Vec<[u8; 32]> = Vec::with_capacity(num_hashes);
    for i in 0..num_hashes {
        evidence_hashes.push(read_id(remaining, i * 32));
    }

    // Build packet — must not panic for any input
    let packet = EvidencePacket::from_misbehavior(
        &mtype,
        node_id,
        epoch,
        slot,
        evidence_hashes.clone(),
        None,
    );

    // Invariant 1: packet_hash is deterministic for the same inputs
    let packet2 = EvidencePacket::from_misbehavior(
        &mtype,
        node_id,
        epoch,
        slot,
        evidence_hashes.clone(),
        None,
    );
    assert_eq!(
        packet.packet_hash,
        packet2.packet_hash,
        "packet_hash must be deterministic for identical inputs"
    );

    // Invariant 2: verify() must pass for freshly built packet
    assert!(
        packet.verify(),
        "freshly built EvidencePacket must pass verify()"
    );

    // Invariant 3: evidence_hashes are sorted in the packet (deterministic regardless of input order)
    {
        let mut reversed = evidence_hashes.clone();
        reversed.reverse();
        let packet_reversed = EvidencePacket::from_misbehavior(
            &mtype,
            node_id,
            epoch,
            slot,
            reversed,
            None,
        );
        // Sorting makes from_misbehavior order-insensitive
        assert_eq!(
            packet.packet_hash,
            packet_reversed.packet_hash,
            "packet_hash must be independent of input ordering of evidence_hashes"
        );
    }

    // Invariant 4: different inputs produce different packet_hashes (collision test)
    // Sample a second input by changing epoch
    if epoch != u64::MAX {
        let packet_alt = EvidencePacket::from_misbehavior(
            &mtype,
            node_id,
            epoch + 1,
            slot,
            evidence_hashes.clone(),
            None,
        );
        assert_ne!(
            packet.packet_hash,
            packet_alt.packet_hash,
            "different epoch must produce different packet_hash"
        );
    }

    // Invariant 5: tampered packet fails verify()
    {
        let mut tampered = packet.clone();
        tampered.epoch = tampered.epoch.wrapping_add(1); // change epoch without recomputing hash
        assert!(
            !tampered.verify(),
            "tampered EvidencePacket (epoch changed) must fail verify()"
        );
    }
    {
        let mut tampered = packet.clone();
        tampered.offender_node_id[0] ^= 0xFF; // change node_id without recomputing hash
        assert!(
            !tampered.verify(),
            "tampered EvidencePacket (node_id changed) must fail verify()"
        );
    }

    // Invariant 6: DuplicateEvidenceGuard correctly deduplicates
    {
        let mut guard = DuplicateEvidenceGuard::new();

        // First registration must succeed
        assert!(
            guard.register(&packet),
            "first registration must return true"
        );

        // Second registration of the same packet must fail
        assert!(
            !guard.register(&packet),
            "second registration of same packet must return false"
        );

        // Registering the alt packet (different epoch) must succeed
        if epoch != u64::MAX {
            let packet_alt = EvidencePacket::from_misbehavior(
                &mtype,
                node_id,
                epoch + 1,
                slot,
                evidence_hashes.clone(),
                None,
            );
            assert!(
                guard.register(&packet_alt),
                "different packet (different epoch) must register successfully"
            );

            assert_eq!(
                guard.seen_count(),
                2,
                "guard must contain exactly 2 distinct packets"
            );
        }

        // is_seen() must be consistent with register()
        assert!(
            guard.is_seen(&packet.packet_hash),
            "is_seen must return true after registration"
        );
        assert!(
            !guard.is_seen(&[0xFFu8; 32]),
            "is_seen must return false for unregistered hash"
        );
    }
});
