#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::bridge::pipeline::BatchCommitment;

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
    if data.len() < 64 {
        return;
    }

    // Parse: batch_id(32) + delivery_id(32) + variable submission_ids (32 each)
    let finalization_hash = read_id(data, 0);
    let delivery_id = read_id(data, 32);

    // Parse variable-length submission IDs from remaining bytes
    let remaining = if data.len() > 64 { &data[64..] } else { &[] };
    let num_ids = remaining.len() / 32; // however many 32-byte blocks we have
    let num_ids = num_ids.min(50); // cap at 50 to keep fuzz fast

    let mut ordered_ids: Vec<[u8; 32]> = Vec::with_capacity(num_ids);
    for i in 0..num_ids {
        let mut id = [0u8; 32];
        id.copy_from_slice(&remaining[i * 32..(i + 1) * 32]);
        ordered_ids.push(id);
    }

    // Invariant 1: commitment computation must not panic for any input
    let commitment1 = BatchCommitment::compute(&finalization_hash, &delivery_id, &ordered_ids);
    let commitment2 = BatchCommitment::compute(&finalization_hash, &delivery_id, &ordered_ids);

    // Invariant 2: commitment hash is deterministic
    assert_eq!(
        commitment1.commitment_hash,
        commitment2.commitment_hash,
        "commitment hash must be deterministic for same inputs"
    );
    assert_eq!(
        commitment1.submission_root,
        commitment2.submission_root,
        "submission_root must be deterministic"
    );
    assert_eq!(
        commitment1.finalization_hash,
        commitment2.finalization_hash,
        "finalization_hash field must be preserved"
    );
    assert_eq!(
        commitment1.delivery_id,
        commitment2.delivery_id,
        "delivery_id field must be preserved"
    );

    // Invariant 3: submission_root changes if ordering changes
    // (test with reversed list, if we have at least 2 distinct IDs)
    if ordered_ids.len() >= 2 && ordered_ids[0] != ordered_ids[1] {
        let mut reversed = ordered_ids.clone();
        reversed.reverse();
        let commitment_reversed =
            BatchCommitment::compute(&finalization_hash, &delivery_id, &reversed);

        // Different ordering → different submission_root (since it's ordered-sensitive)
        // NOTE: only guaranteed when ids are actually different
        let all_same = ordered_ids.windows(2).all(|w| w[0] == w[1]);
        if !all_same {
            assert_ne!(
                commitment1.submission_root,
                commitment_reversed.submission_root,
                "reversed submission order must produce different submission_root"
            );
        }
    }

    // Invariant 4: tampering with commitment_hash is detectable
    // We simulate what verify_commitment() does by rebuilding and comparing
    {
        let mut tampered_hash = commitment1.commitment_hash;
        // Flip the first byte (or any byte)
        tampered_hash[0] ^= 0xFF;

        // Recompute with same inputs — result must differ from tampered
        let recomputed = BatchCommitment::compute(&finalization_hash, &delivery_id, &ordered_ids);

        assert_ne!(
            recomputed.commitment_hash,
            tampered_hash,
            "tampered commitment_hash must not match recomputed hash"
        );
    }

    // Invariant 5: changing finalization_hash changes commitment
    if finalization_hash != [0xFFu8; 32] {
        let mut alt_finalization = finalization_hash;
        alt_finalization[0] ^= 0x01;
        let alt_commitment =
            BatchCommitment::compute(&alt_finalization, &delivery_id, &ordered_ids);
        assert_ne!(
            commitment1.commitment_hash,
            alt_commitment.commitment_hash,
            "different finalization_hash must produce different commitment"
        );
    }

    // Invariant 6: changing delivery_id changes commitment
    if delivery_id != [0xFFu8; 32] {
        let mut alt_delivery = delivery_id;
        alt_delivery[0] ^= 0x01;
        let alt_commitment =
            BatchCommitment::compute(&finalization_hash, &alt_delivery, &ordered_ids);
        assert_ne!(
            commitment1.commitment_hash,
            alt_commitment.commitment_hash,
            "different delivery_id must produce different commitment"
        );
    }
});
