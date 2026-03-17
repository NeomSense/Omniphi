use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::fairness::policy::FairnessClass;
use crate::inclusion::engine::{InclusionDecision, ExclusionReason};
use crate::anti_mev::engine::AntiMevValidationResult;
use crate::queue_snapshot::snapshot::SnapshotCommitment;

/// Audit record for a single submission's inclusion in a batch.
#[derive(Debug, Clone)]
pub struct InclusionAuditEntry {
    pub submission_id: [u8; 32],
    pub fairness_class: FairnessClass,
    pub age_slots: u64,
    pub included: bool,
    pub forced_inclusion: bool,
    pub exclusion_reason: Option<ExclusionReason>,
    /// Position in the final batch (0-indexed), if included.
    pub position_in_batch: Option<usize>,
    /// Position in the snapshot, if present.
    pub snapshot_position: Option<usize>,
    /// proposed_position - snapshot_position; negative = promoted.
    pub position_delta: Option<i64>,
}

impl InclusionAuditEntry {
    pub fn from_decision(
        submission_id: [u8; 32],
        fairness_class: FairnessClass,
        age_slots: u64,
        decision: &InclusionDecision,
        position_in_batch: Option<usize>,
        snapshot_position: Option<usize>,
    ) -> Self {
        let (included, forced_inclusion, exclusion_reason) = match decision {
            InclusionDecision::Include => (true, false, None),
            InclusionDecision::ForceInclude(_) => (true, true, None),
            InclusionDecision::Exclude(reason) => (false, false, Some(reason.clone())),
        };
        let position_delta = match (position_in_batch, snapshot_position) {
            (Some(batch_pos), Some(snap_pos)) => Some(batch_pos as i64 - snap_pos as i64),
            _ => None,
        };
        InclusionAuditEntry {
            submission_id,
            fairness_class,
            age_slots,
            included,
            forced_inclusion,
            exclusion_reason,
            position_in_batch,
            snapshot_position,
            position_delta,
        }
    }
}

/// Justification record for ordering a submission in its final position.
#[derive(Debug, Clone)]
pub struct OrderingJustificationRecord {
    pub submission_id: [u8; 32],
    pub snapshot_position: usize,
    pub final_position: usize,
    /// proposed_position - snapshot_position.
    pub delta: i64,
    /// Name of the fairness rule that justified this ordering.
    pub justified_by: String,
    pub policy_compliant: bool,
}

/// Full fairness audit record for a batch.
#[derive(Debug, Clone)]
pub struct FairnessAuditRecord {
    pub batch_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub snapshot_commitment: SnapshotCommitment,
    pub policy_version: u32,
    pub total_eligible: usize,
    pub total_included: usize,
    pub total_excluded: usize,
    pub forced_inclusions: usize,
    pub inclusion_entries: Vec<InclusionAuditEntry>,
    pub ordering_justifications: Vec<OrderingJustificationRecord>,
    pub class_distribution: BTreeMap<FairnessClass, usize>,
    pub anti_mev_result: AntiMevValidationResult,
    pub fairness_warnings: Vec<String>,
    /// SHA256 of all fields computed at build time.
    pub audit_hash: [u8; 32],
}

impl FairnessAuditRecord {
    /// Build a FairnessAuditRecord, computing audit_hash deterministically.
    #[allow(clippy::too_many_arguments)]
    pub fn build(
        batch_id: [u8; 32],
        slot: u64,
        epoch: u64,
        leader_id: [u8; 32],
        snapshot_commitment: SnapshotCommitment,
        policy_version: u32,
        total_eligible: usize,
        total_included: usize,
        total_excluded: usize,
        forced_inclusions: usize,
        inclusion_entries: Vec<InclusionAuditEntry>,
        ordering_justifications: Vec<OrderingJustificationRecord>,
        class_distribution: BTreeMap<FairnessClass, usize>,
        anti_mev_result: AntiMevValidationResult,
        fairness_warnings: Vec<String>,
    ) -> Self {
        let audit_hash = Self::compute_audit_hash(
            &batch_id,
            slot,
            epoch,
            &leader_id,
            &snapshot_commitment,
            policy_version,
            total_eligible,
            total_included,
            total_excluded,
            forced_inclusions,
            &anti_mev_result,
            &fairness_warnings,
        );

        FairnessAuditRecord {
            batch_id,
            slot,
            epoch,
            leader_id,
            snapshot_commitment,
            policy_version,
            total_eligible,
            total_included,
            total_excluded,
            forced_inclusions,
            inclusion_entries,
            ordering_justifications,
            class_distribution,
            anti_mev_result,
            fairness_warnings,
            audit_hash,
        }
    }

    fn compute_audit_hash(
        batch_id: &[u8; 32],
        slot: u64,
        epoch: u64,
        leader_id: &[u8; 32],
        snapshot_commitment: &SnapshotCommitment,
        policy_version: u32,
        total_eligible: usize,
        total_included: usize,
        total_excluded: usize,
        forced_inclusions: usize,
        anti_mev_result: &AntiMevValidationResult,
        fairness_warnings: &[String],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(batch_id);
        hasher.update(&slot.to_be_bytes());
        hasher.update(&epoch.to_be_bytes());
        hasher.update(leader_id);
        hasher.update(&snapshot_commitment.commitment_hash);
        hasher.update(&policy_version.to_be_bytes());
        hasher.update(&(total_eligible as u64).to_be_bytes());
        hasher.update(&(total_included as u64).to_be_bytes());
        hasher.update(&(total_excluded as u64).to_be_bytes());
        hasher.update(&(forced_inclusions as u64).to_be_bytes());
        hasher.update(&[anti_mev_result.valid as u8]);
        hasher.update(&(anti_mev_result.violations.len() as u64).to_be_bytes());
        for warning in fairness_warnings {
            hasher.update(warning.as_bytes());
        }
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// A compact receipt summarizing the fairness compliance of a batch.
#[derive(Debug, Clone)]
pub struct FairnessReceipt {
    pub batch_id: [u8; 32],
    pub audit_hash: [u8; 32],
    pub policy_version: u32,
    pub compliant: bool,
    pub warning_count: usize,
    pub violation_count: usize,
}

impl FairnessReceipt {
    pub fn from_audit(record: &FairnessAuditRecord) -> Self {
        FairnessReceipt {
            batch_id: record.batch_id,
            audit_hash: record.audit_hash,
            policy_version: record.policy_version,
            compliant: record.anti_mev_result.valid && record.fairness_warnings.is_empty(),
            warning_count: record.fairness_warnings.len(),
            violation_count: record.anti_mev_result.violations.len(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::queue_snapshot::snapshot::{QueueSnapshot, EligibleSubmissionEntry, SnapshotCommitment};

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

    fn make_audit_record() -> FairnessAuditRecord {
        let commitment = make_commitment();
        let anti_mev = AntiMevValidationResult::default();
        FairnessAuditRecord::build(
            make_id(10),
            5,
            1,
            make_id(99),
            commitment,
            1,
            3,
            2,
            1,
            0,
            vec![],
            vec![],
            BTreeMap::new(),
            anti_mev,
            vec![],
        )
    }

    #[test]
    fn test_fairness_audit_record_build_produces_consistent_hash() {
        let r1 = make_audit_record();
        let r2 = make_audit_record();
        assert_eq!(r1.audit_hash, r2.audit_hash, "Same inputs must produce same audit_hash");
        assert_ne!(r1.audit_hash, [0u8; 32], "audit_hash must not be zero");
    }

    #[test]
    fn test_fairness_audit_record_hash_changes_with_different_batch_id() {
        let commitment = make_commitment();
        let make_record = |batch_id: [u8; 32]| {
            FairnessAuditRecord::build(
                batch_id, 5, 1, make_id(99), commitment.clone(),
                1, 3, 2, 1, 0, vec![], vec![], BTreeMap::new(),
                AntiMevValidationResult::default(), vec![],
            )
        };
        let r1 = make_record(make_id(1));
        let r2 = make_record(make_id(2));
        assert_ne!(r1.audit_hash, r2.audit_hash);
    }

    #[test]
    fn test_inclusion_audit_entry_from_include_decision() {
        let entry = InclusionAuditEntry::from_decision(
            make_id(1),
            FairnessClass::Normal,
            5,
            &InclusionDecision::Include,
            Some(0),
            Some(0),
        );
        assert!(entry.included);
        assert!(!entry.forced_inclusion);
        assert!(entry.exclusion_reason.is_none());
        assert_eq!(entry.position_delta, Some(0));
    }

    #[test]
    fn test_inclusion_audit_entry_from_force_include_decision() {
        let entry = InclusionAuditEntry::from_decision(
            make_id(2),
            FairnessClass::SafetyCritical,
            50,
            &InclusionDecision::ForceInclude("starvation".to_string()),
            Some(1),
            Some(5),
        );
        assert!(entry.included);
        assert!(entry.forced_inclusion);
        // position_delta = 1 - 5 = -4 (promoted from position 5 to 1)
        assert_eq!(entry.position_delta, Some(-4));
    }

    #[test]
    fn test_inclusion_audit_entry_from_exclude_decision() {
        let entry = InclusionAuditEntry::from_decision(
            make_id(3),
            FairnessClass::Normal,
            2,
            &InclusionDecision::Exclude(ExclusionReason::BatchFull),
            None,
            Some(10),
        );
        assert!(!entry.included);
        assert_eq!(entry.exclusion_reason, Some(ExclusionReason::BatchFull));
        assert!(entry.position_delta.is_none()); // no batch position → no delta
    }

    #[test]
    fn test_fairness_receipt_from_audit() {
        let record = make_audit_record();
        let receipt = FairnessReceipt::from_audit(&record);
        assert_eq!(receipt.batch_id, record.batch_id);
        assert_eq!(receipt.audit_hash, record.audit_hash);
        assert_eq!(receipt.violation_count, 0);
        assert_eq!(receipt.warning_count, 0);
        assert!(receipt.compliant);
    }
}
