//! Checkpoint and batch anchoring — optional on-chain finality references.
//!
//! These records allow the Cosmos chain to anchor PoSeq outputs for:
//! - Operator visibility (query finalized batches from the chain)
//! - Audit trails (governance can inspect what was sequenced in an epoch)
//! - Accountability (batch references tied to evidence packages)
//!
//! Anchoring is **write-once**: once a `CheckpointAnchorRecord` or
//! `BatchFinalityReference` is stored on-chain it is immutable. The chain
//! does not need to validate the PoSeq-internal details — it just stores
//! the cryptographic commitments for future reference.

use sha2::{Sha256, Digest};

// ─── BatchFinalityReference ───────────────────────────────────────────────────

/// A reference to a single finalized PoSeq batch, suitable for on-chain anchoring.
/// The chain stores the `batch_id` and `finalization_hash` as immutable records.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchFinalityReference {
    /// The canonical batch ID (from `FinalizedBatch.batch_id`).
    pub batch_id: [u8; 32],
    /// Slot in which this batch was finalized.
    pub slot: u64,
    /// Epoch in which this batch was finalized.
    pub epoch: u64,
    /// SHA256 cryptographic anchor of the finalization event.
    pub finalization_hash: [u8; 32],
    /// Number of submissions in this batch.
    pub submission_count: usize,
    /// Quorum approvals (for operator visibility).
    pub quorum_approvals: usize,
    /// Committee size at finalization.
    pub committee_size: usize,
}

impl BatchFinalityReference {
    /// Anchor hash: `SHA256("batch" ‖ batch_id ‖ epoch_be ‖ finalization_hash)`
    pub fn compute_anchor_hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"batch");
        hasher.update(&self.batch_id);
        hasher.update(&self.epoch.to_be_bytes());
        hasher.update(&self.finalization_hash);
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

// ─── EpochStateReference ──────────────────────────────────────────────────────

/// Reference to PoSeq epoch state for governance and accountability queries.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EpochStateReference {
    pub epoch: u64,
    /// Hash of the committee membership for this epoch.
    pub committee_hash: [u8; 32],
    /// Total finalized batches in this epoch.
    pub finalized_batch_count: u64,
    /// Total misbehavior incidents in this epoch.
    pub misbehavior_count: u32,
    /// Total evidence packets submitted for this epoch.
    pub evidence_packet_count: u32,
    /// Number of governance escalations triggered this epoch.
    pub governance_escalations: u32,
    /// `SHA256("epoch" ‖ epoch_be ‖ committee_hash ‖ finalized_batch_count_be)`
    pub epoch_state_hash: [u8; 32],
}

impl EpochStateReference {
    pub fn compute_epoch_state_hash(
        epoch: u64,
        committee_hash: &[u8; 32],
        finalized_batch_count: u64,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"epoch");
        hasher.update(&epoch.to_be_bytes());
        hasher.update(committee_hash);
        hasher.update(&finalized_batch_count.to_be_bytes());
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    pub fn new(
        epoch: u64,
        committee_hash: [u8; 32],
        finalized_batch_count: u64,
        misbehavior_count: u32,
        evidence_packet_count: u32,
        governance_escalations: u32,
    ) -> Self {
        let epoch_state_hash = Self::compute_epoch_state_hash(
            epoch, &committee_hash, finalized_batch_count,
        );
        EpochStateReference {
            epoch,
            committee_hash,
            finalized_batch_count,
            misbehavior_count,
            evidence_packet_count,
            governance_escalations,
            epoch_state_hash,
        }
    }
}

// ─── CheckpointAnchorRecord ───────────────────────────────────────────────────

/// On-chain anchor for a PoSeq checkpoint.
///
/// A checkpoint represents a verified consistent state of the sequencing layer
/// at a specific (epoch, slot). The chain anchors this so operators can verify
/// the checkpoint hash against PoSeq's internal state after any dispute.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CheckpointAnchorRecord {
    /// The PoSeq checkpoint ID (from `PoSeqCheckpoint.checkpoint_id`).
    pub checkpoint_id: [u8; 32],

    /// Epoch of the checkpoint.
    pub epoch: u64,

    /// Slot of the checkpoint.
    pub slot: u64,

    /// Hash of the epoch state at checkpoint time.
    pub epoch_state_hash: [u8; 32],

    /// Hash of the bridge delivery state at checkpoint time.
    pub bridge_state_hash: [u8; 32],

    /// Number of misbehavior incidents at checkpoint time.
    pub misbehavior_count: u32,

    /// Finality state summary at checkpoint time.
    pub finality_summary: BatchFinalityReference,

    /// `SHA256("ckpt" ‖ checkpoint_id ‖ epoch_be ‖ epoch_state_hash ‖ bridge_state_hash)`
    pub anchor_hash: [u8; 32],
}

impl CheckpointAnchorRecord {
    pub fn compute_anchor_hash(
        checkpoint_id: &[u8; 32],
        epoch: u64,
        epoch_state_hash: &[u8; 32],
        bridge_state_hash: &[u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"ckpt");
        hasher.update(checkpoint_id);
        hasher.update(&epoch.to_be_bytes());
        hasher.update(epoch_state_hash);
        hasher.update(bridge_state_hash);
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    pub fn build(
        checkpoint_id: [u8; 32],
        epoch: u64,
        slot: u64,
        epoch_state_hash: [u8; 32],
        bridge_state_hash: [u8; 32],
        misbehavior_count: u32,
        finality_summary: BatchFinalityReference,
    ) -> Self {
        let anchor_hash = Self::compute_anchor_hash(
            &checkpoint_id, epoch, &epoch_state_hash, &bridge_state_hash,
        );
        CheckpointAnchorRecord {
            checkpoint_id,
            epoch,
            slot,
            epoch_state_hash,
            bridge_state_hash,
            misbehavior_count,
            finality_summary,
            anchor_hash,
        }
    }

    pub fn verify(&self) -> bool {
        let expected = Self::compute_anchor_hash(
            &self.checkpoint_id,
            self.epoch,
            &self.epoch_state_hash,
            &self.bridge_state_hash,
        );
        expected == self.anchor_hash
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn bid(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_finality_ref(b: u8) -> BatchFinalityReference {
        BatchFinalityReference {
            batch_id: bid(b),
            slot: b as u64,
            epoch: 1,
            finalization_hash: bid(b + 1),
            submission_count: 3,
            quorum_approvals: 3,
            committee_size: 4,
        }
    }

    // ── BatchFinalityReference ────────────────────────────────────────────────

    #[test]
    fn test_batch_anchor_hash_deterministic() {
        let r = make_finality_ref(10);
        let h1 = r.compute_anchor_hash();
        let h2 = r.compute_anchor_hash();
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_batch_anchor_hash_different_batch() {
        let r1 = make_finality_ref(10);
        let r2 = make_finality_ref(11);
        assert_ne!(r1.compute_anchor_hash(), r2.compute_anchor_hash());
    }

    // ── EpochStateReference ───────────────────────────────────────────────────

    #[test]
    fn test_epoch_state_hash_deterministic() {
        let h1 = EpochStateReference::compute_epoch_state_hash(5, &[0u8; 32], 10);
        let h2 = EpochStateReference::compute_epoch_state_hash(5, &[0u8; 32], 10);
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_epoch_state_hash_epoch_sensitive() {
        let h1 = EpochStateReference::compute_epoch_state_hash(5, &[0u8; 32], 10);
        let h2 = EpochStateReference::compute_epoch_state_hash(6, &[0u8; 32], 10);
        assert_ne!(h1, h2);
    }

    #[test]
    fn test_epoch_state_reference_new() {
        let r = EpochStateReference::new(3, [0xAAu8; 32], 50, 2, 3, 1);
        assert_eq!(r.epoch, 3);
        assert_eq!(r.finalized_batch_count, 50);
        assert_ne!(r.epoch_state_hash, [0u8; 32]);
    }

    // ── CheckpointAnchorRecord ────────────────────────────────────────────────

    #[test]
    fn test_checkpoint_anchor_verifies() {
        let anchor = CheckpointAnchorRecord::build(
            [0xCCu8; 32], 5, 10,
            [0xDDu8; 32], [0xEEu8; 32],
            3, make_finality_ref(20),
        );
        assert!(anchor.verify());
    }

    #[test]
    fn test_checkpoint_anchor_tamper_fails() {
        let mut anchor = CheckpointAnchorRecord::build(
            [0xCCu8; 32], 5, 10,
            [0xDDu8; 32], [0xEEu8; 32],
            3, make_finality_ref(20),
        );
        anchor.epoch = 99; // tamper
        assert!(!anchor.verify());
    }

    #[test]
    fn test_checkpoint_anchor_hash_deterministic() {
        let a1 = CheckpointAnchorRecord::build(
            [0x01u8; 32], 1, 5, [0x02u8; 32], [0x03u8; 32], 0, make_finality_ref(1),
        );
        let a2 = CheckpointAnchorRecord::build(
            [0x01u8; 32], 1, 5, [0x02u8; 32], [0x03u8; 32], 0, make_finality_ref(1),
        );
        assert_eq!(a1.anchor_hash, a2.anchor_hash);
    }
}
