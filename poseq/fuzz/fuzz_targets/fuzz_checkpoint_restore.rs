#![no_main]

use libfuzzer_sys::fuzz_target;
use omniphi_poseq::checkpoints::{
    CheckpointMetadata, CheckpointPolicy, CheckpointStore, PoSeqCheckpoint,
};
use omniphi_poseq::finality::FinalityCheckpoint;

fn read_u64(data: &[u8], offset: usize) -> u64 {
    if offset + 8 <= data.len() {
        u64::from_le_bytes(data[offset..offset + 8].try_into().unwrap_or([0u8; 8]))
    } else {
        0
    }
}

fn read_u32(data: &[u8], offset: usize) -> u32 {
    if offset + 4 <= data.len() {
        u32::from_le_bytes(data[offset..offset + 4].try_into().unwrap_or([0u8; 4]))
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

/// Build a valid PoSeqCheckpoint from fuzz-derived fields.
fn make_checkpoint(epoch: u64, slot: u64, seq: u64, node_id: [u8; 32]) -> PoSeqCheckpoint {
    let meta = CheckpointMetadata {
        version: 1,
        epoch,
        slot,
        created_seq: seq,
        node_id,
    };
    let finality_cp = FinalityCheckpoint::compute(epoch, [0xABu8; 32], slot);
    let checkpoint_id = PoSeqCheckpoint::compute_id(&meta, &finality_cp.checkpoint_hash);
    PoSeqCheckpoint {
        metadata: meta,
        finality_checkpoint: finality_cp,
        epoch_state_hash: [0x11u8; 32],
        bridge_state_hash: [0x22u8; 32],
        misbehavior_count: 0,
        checkpoint_id,
    }
}

fuzz_target!(|data: &[u8]| {
    // Parse up to 3 checkpoint descriptions.
    // Each requires: epoch(u64) + slot(u64) + seq(u64) + node_id(32) = 56 bytes
    const CP_SIZE: usize = 56;

    if data.len() < CP_SIZE {
        return;
    }

    let policy = CheckpointPolicy {
        checkpoint_interval_epochs: 1,
        max_checkpoints_retained: 50,
    };
    let mut store = CheckpointStore::new(policy);

    let num_checkpoints = (data.len() / CP_SIZE).min(3);

    let mut stored_ids: Vec<[u8; 32]> = Vec::new();

    for i in 0..num_checkpoints {
        let offset = i * CP_SIZE;

        // Use epoch values > 0 (epoch 0 is genesis and rejected by store)
        let epoch_raw = read_u64(data, offset);
        let epoch = (epoch_raw % 999) + 1; // 1..=999 to avoid collisions at scale

        let slot = read_u64(data, offset + 8);
        let seq = read_u64(data, offset + 16);
        let node_id = read_id(data, offset + 24);

        let cp = make_checkpoint(epoch, slot, seq, node_id);
        let cp_id = cp.checkpoint_id;

        let store_result = store.store(cp);
        if store_result.is_ok() {
            stored_ids.push(cp_id);
        }
        // EpochAlreadyCheckpointed or IdMismatch are fine — just don't panic
    }

    // Invariant: restore is idempotent — calling it twice must produce the same result
    for &id in &stored_ids {
        let result1 = store.restore(&id);
        let result2 = store.restore(&id);

        assert_eq!(
            result1.success, result2.success,
            "restore must be idempotent: first={} second={}",
            result1.success, result2.success
        );
        assert_eq!(
            result1.epoch_restored, result2.epoch_restored,
            "restore must return same epoch both times"
        );
        assert_eq!(
            result1.checkpoint_id, result2.checkpoint_id,
            "restore must return same checkpoint_id both times"
        );

        if result1.success {
            assert_eq!(
                result1.errors.len(),
                0,
                "successful restore must have no errors"
            );
        }
    }

    // Invariant: restoring a non-existent ID must fail gracefully (not panic)
    let unknown_id = [0xFFu8; 32];
    let not_found = store.restore(&unknown_id);
    assert!(
        !not_found.success,
        "restoring unknown checkpoint_id must return success=false"
    );
    assert!(
        !not_found.errors.is_empty(),
        "restoring unknown checkpoint_id must produce error messages"
    );

    // Invariant: epoch 0 checkpoint must never be stored (store rejects epoch 0)
    // (The epoch=0 case is guarded by our epoch_raw % 999 + 1 clamping above,
    //  but test it directly to be certain.)
    let epoch0_cp = make_checkpoint(0, 0, 0, [0u8; 32]);
    // store.store() for epoch 0: the checkpoint has epoch=0 in metadata.
    // EpochAlreadyCheckpointed(0) is returned if we already stored epoch=0 (we haven't).
    // The checkpoint itself is valid, but epoch=0 is genesis — store allows it structurally
    // (the guard is in should_checkpoint, not store()). What we assert here is simply
    // that store() never panics regardless of epoch value.
    let _ = store.store(epoch0_cp); // must not panic

    // Invariant: tampered checkpoint must fail verify_id
    let tamper_cp = {
        let mut cp = make_checkpoint(500, 1000, 0, [0x01u8; 32]);
        cp.metadata.epoch = 999; // tamper without recomputing checkpoint_id
        cp
    };
    assert!(
        !tamper_cp.verify_id(),
        "tampered checkpoint must fail verify_id"
    );
    // Storing a tampered checkpoint must return IdMismatch, not panic
    let tamper_result = store.store(tamper_cp);
    assert!(
        tamper_result.is_err(),
        "storing tampered checkpoint must return Err"
    );
});
