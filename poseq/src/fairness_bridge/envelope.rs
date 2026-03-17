use sha2::{Sha256, Digest};
use crate::queue_snapshot::snapshot::SnapshotCommitment;
use crate::fairness_audit::records::FairnessReceipt;

/// Fairness metadata attached to a batch for downstream consumers.
#[derive(Debug, Clone)]
pub struct BatchFairnessMetadata {
    pub batch_id: [u8; 32],
    pub policy_version: u32,
    pub snapshot_commitment: SnapshotCommitment,
    pub fairness_receipt: FairnessReceipt,
    pub protected_flow_count: usize,
    pub forced_inclusion_count: usize,
    pub incident_count: usize,
    /// SHA256(batch_id || policy_version || snapshot_root || receipt audit_hash).
    pub metadata_hash: [u8; 32],
}

impl BatchFairnessMetadata {
    pub fn build(
        batch_id: [u8; 32],
        policy_version: u32,
        snapshot_commitment: SnapshotCommitment,
        fairness_receipt: FairnessReceipt,
        protected_flow_count: usize,
        forced_inclusion_count: usize,
        incident_count: usize,
    ) -> Self {
        let metadata_hash = Self::compute_metadata_hash(
            &batch_id,
            policy_version,
            &snapshot_commitment.snapshot_root,
            &fairness_receipt.audit_hash,
        );
        BatchFairnessMetadata {
            batch_id,
            policy_version,
            snapshot_commitment,
            fairness_receipt,
            protected_flow_count,
            forced_inclusion_count,
            incident_count,
            metadata_hash,
        }
    }

    fn compute_metadata_hash(
        batch_id: &[u8; 32],
        policy_version: u32,
        snapshot_root: &[u8; 32],
        receipt_audit_hash: &[u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(batch_id);
        hasher.update(&policy_version.to_be_bytes());
        hasher.update(snapshot_root);
        hasher.update(receipt_audit_hash);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// A runtime envelope carrying fairness metadata alongside a batch.
#[derive(Debug, Clone)]
pub struct RuntimeFairnessEnvelope {
    pub batch_id: [u8; 32],
    pub fairness_metadata: BatchFairnessMetadata,
    /// Hash of the associated FairnessAuditRecord.
    pub audit_reference: [u8; 32],
}

impl RuntimeFairnessEnvelope {
    pub fn new(
        batch_id: [u8; 32],
        fairness_metadata: BatchFairnessMetadata,
        audit_hash: [u8; 32],
    ) -> Self {
        RuntimeFairnessEnvelope {
            batch_id,
            fairness_metadata,
            audit_reference: audit_hash,
        }
    }
}

/// A compact reference to a batch's fairness audit.
#[derive(Debug, Clone)]
pub struct FairnessReferenceId {
    pub batch_id: [u8; 32],
    pub audit_hash: [u8; 32],
    pub policy_version: u32,
}

impl FairnessReferenceId {
    pub fn new(batch_id: [u8; 32], audit_hash: [u8; 32], policy_version: u32) -> Self {
        FairnessReferenceId { batch_id, audit_hash, policy_version }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::queue_snapshot::snapshot::{QueueSnapshot, EligibleSubmissionEntry, SnapshotCommitment};
    use crate::fairness::policy::FairnessClass;
    use crate::fairness_audit::records::FairnessReceipt;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_commitment() -> SnapshotCommitment {
        let entries = vec![EligibleSubmissionEntry {
            submission_id: make_id(1),
            fairness_class: FairnessClass::Normal,
            received_at_slot: 1,
            received_at_sequence: 0,
            age_slots: 0,
            is_forced_inclusion: false,
        }];
        let snapshot = QueueSnapshot::build(entries, 1, 1, 100, 1);
        SnapshotCommitment::compute(&snapshot, make_id(99))
    }

    fn make_receipt(batch_id: [u8; 32]) -> FairnessReceipt {
        FairnessReceipt {
            batch_id,
            audit_hash: make_id(55),
            policy_version: 1,
            compliant: true,
            warning_count: 0,
            violation_count: 0,
        }
    }

    #[test]
    fn test_batch_fairness_metadata_build_produces_nontrivial_hash() {
        let batch_id = make_id(10);
        let commitment = make_commitment();
        let receipt = make_receipt(batch_id);
        let metadata = BatchFairnessMetadata::build(batch_id, 1, commitment, receipt, 0, 0, 0);
        assert_ne!(metadata.metadata_hash, [0u8; 32]);
    }

    #[test]
    fn test_batch_fairness_metadata_hash_is_deterministic() {
        let batch_id = make_id(10);
        let m1 = BatchFairnessMetadata::build(batch_id, 1, make_commitment(), make_receipt(batch_id), 0, 0, 0);
        let m2 = BatchFairnessMetadata::build(batch_id, 1, make_commitment(), make_receipt(batch_id), 0, 0, 0);
        assert_eq!(m1.metadata_hash, m2.metadata_hash);
    }

    #[test]
    fn test_batch_fairness_metadata_hash_changes_with_different_batch_id() {
        let m1 = BatchFairnessMetadata::build(make_id(1), 1, make_commitment(), make_receipt(make_id(1)), 0, 0, 0);
        let m2 = BatchFairnessMetadata::build(make_id(2), 1, make_commitment(), make_receipt(make_id(2)), 0, 0, 0);
        assert_ne!(m1.metadata_hash, m2.metadata_hash);
    }

    #[test]
    fn test_runtime_fairness_envelope_construction() {
        let batch_id = make_id(20);
        let metadata = BatchFairnessMetadata::build(
            batch_id, 1, make_commitment(), make_receipt(batch_id), 1, 0, 0,
        );
        let audit_hash = make_id(77);
        let envelope = RuntimeFairnessEnvelope::new(batch_id, metadata, audit_hash);
        assert_eq!(envelope.batch_id, batch_id);
        assert_eq!(envelope.audit_reference, audit_hash);
    }

    #[test]
    fn test_fairness_reference_id() {
        let ref_id = FairnessReferenceId::new(make_id(1), make_id(2), 3);
        assert_eq!(ref_id.batch_id, make_id(1));
        assert_eq!(ref_id.audit_hash, make_id(2));
        assert_eq!(ref_id.policy_version, 3);
    }
}
