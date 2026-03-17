use std::collections::BTreeMap;
use crate::fairness::policy::{FairSequencingPolicy, FairnessClass};
use crate::fairness::classification::SubmissionFairnessProfile;
use crate::anti_mev::policy::AntiMevPolicy;
use crate::anti_mev::engine::{AntiMevEngine, AntiMevValidationResult, AntiMevViolationType};
use crate::queue_snapshot::snapshot::QueueSnapshot;
use crate::fairness_audit::records::{
    FairnessAuditRecord, InclusionAuditEntry, OrderingJustificationRecord,
};
use crate::inclusion::engine::{InclusionDecision, ExclusionReason};

/// Error types for proposal fairness validation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProposalFairnessError {
    SnapshotMissing,
    SnapshotMismatch { expected: [u8; 32], got: [u8; 32] },
    ReorderBoundViolated { submission_id: [u8; 32], delta: i64, max: i64 },
    ProtectedFlowDelayed { submission_id: [u8; 32] },
    EligibleSubmissionOmitted { submission_id: [u8; 32] },
    LeaderDiscretionExceeded,
    ClassOrderViolated,
}

/// Full result of validating a proposal for fairness compliance.
#[derive(Debug, Clone)]
pub struct FairnessValidationResult {
    pub valid: bool,
    pub errors: Vec<ProposalFairnessError>,
    pub warnings: Vec<String>,
    pub anti_mev_result: AntiMevValidationResult,
    pub audit_record: FairnessAuditRecord,
}

/// Validates proposed batch orderings against fairness and anti-MEV policies.
#[derive(Debug, Clone)]
pub struct FairProposalValidator {
    pub fairness_policy: FairSequencingPolicy,
    pub anti_mev_policy: AntiMevPolicy,
}

impl FairProposalValidator {
    pub fn new(fairness_policy: FairSequencingPolicy, anti_mev_policy: AntiMevPolicy) -> Self {
        FairProposalValidator {
            fairness_policy,
            anti_mev_policy,
        }
    }

    /// Validate a proposed ordering against the canonical queue snapshot.
    ///
    /// Logic:
    /// 1. Check proposed_order is a subset of snapshot.ordered_ids().
    /// 2. Compute position deltas via AntiMevEngine.
    /// 3. Validate each delta against anti_mev_policy bounds.
    /// 4. Check protected flows appear within position limits.
    /// 5. Build FairnessAuditRecord.
    pub fn validate(
        &self,
        proposed_order: &[[u8; 32]],
        snapshot: &QueueSnapshot,
        profiles: &BTreeMap<[u8; 32], SubmissionFairnessProfile>,
        leader_id: [u8; 32],
        slot: u64,
        epoch: u64,
        batch_id: [u8; 32],
    ) -> FairnessValidationResult {
        let mut errors = Vec::new();
        let mut warnings = Vec::new();

        // Step 1: check proposed is a subset of snapshot
        let snapshot_ids: std::collections::BTreeSet<[u8; 32]> = snapshot.ordered_ids().into_iter().collect();
        for &id in proposed_order {
            if !snapshot_ids.contains(&id) {
                errors.push(ProposalFairnessError::EligibleSubmissionOmitted { submission_id: id });
                warnings.push(format!(
                    "Submission {:?} in proposed order is not in snapshot",
                    hex::encode(id)
                ));
            }
        }

        // Step 2: build snapshot order with classes for anti-MEV engine
        let snapshot_order_with_classes: Vec<([u8; 32], FairnessClass)> = snapshot
            .entries
            .iter()
            .map(|e| (e.submission_id, e.fairness_class.clone()))
            .collect();

        // Step 3: run anti-MEV engine
        let engine = AntiMevEngine::new(self.anti_mev_policy.clone());
        let anti_mev_result =
            engine.apply_ordering_constraints(&snapshot_order_with_classes, proposed_order, proposed_order.len());

        // Step 4: translate anti-MEV violations to proposal errors
        for violation in &anti_mev_result.violations {
            match violation.violation_type {
                AntiMevViolationType::ExceededForwardReorderBound => {
                    errors.push(ProposalFairnessError::ReorderBoundViolated {
                        submission_id: violation.submission_id,
                        delta: violation.actual_delta,
                        max: violation.allowed_delta,
                    });
                }
                AntiMevViolationType::ExceededBackwardReorderBound => {
                    errors.push(ProposalFairnessError::ReorderBoundViolated {
                        submission_id: violation.submission_id,
                        delta: violation.actual_delta,
                        max: violation.allowed_delta,
                    });
                }
                AntiMevViolationType::ProtectedFlowDelayed => {
                    errors.push(ProposalFairnessError::ProtectedFlowDelayed {
                        submission_id: violation.submission_id,
                    });
                }
                AntiMevViolationType::LeaderDiscretionExceeded => {
                    errors.push(ProposalFairnessError::LeaderDiscretionExceeded);
                }
                AntiMevViolationType::SnapshotCommitmentViolation => {
                    errors.push(ProposalFairnessError::SnapshotMismatch {
                        expected: snapshot.snapshot_id,
                        got: [0u8; 32],
                    });
                }
            }
        }

        // Step 5: check protected flows appear within position limits
        let pf_violations = engine.validate_protected_flows(proposed_order, profiles);
        for pv in &pf_violations {
            errors.push(ProposalFairnessError::ProtectedFlowDelayed {
                submission_id: pv.submission_id,
            });
            warnings.push(format!(
                "Protected flow {:?} at position {} exceeds max {}",
                hex::encode(pv.submission_id),
                pv.actual_position,
                pv.expected_max_position
            ));
        }

        // Build snapshot position map for audit
        let snapshot_positions: BTreeMap<[u8; 32], usize> = snapshot
            .entries
            .iter()
            .enumerate()
            .map(|(i, e)| (e.submission_id, i))
            .collect();

        let proposed_positions: BTreeMap<[u8; 32], usize> = proposed_order
            .iter()
            .enumerate()
            .map(|(i, id)| (*id, i))
            .collect();

        // Build inclusion audit entries
        let mut inclusion_entries = Vec::new();
        let mut class_distribution: BTreeMap<FairnessClass, usize> = BTreeMap::new();
        let mut forced_inclusions = 0usize;

        // For all snapshot entries, determine if they were included in proposed
        for entry in &snapshot.entries {
            let id = entry.submission_id;
            let in_proposed = proposed_positions.contains_key(&id);
            let profile = profiles.get(&id);
            let fairness_class = profile
                .map(|p| p.assigned_class.clone())
                .unwrap_or_else(|| entry.fairness_class.clone());
            let age_slots = profile.map(|p| p.age_slots).unwrap_or(entry.age_slots);
            let is_forced = profile.map(|p| p.is_forced_inclusion).unwrap_or(false);

            let decision = if in_proposed {
                if is_forced {
                    forced_inclusions += 1;
                    InclusionDecision::ForceInclude("forced".to_string())
                } else {
                    InclusionDecision::Include
                }
            } else {
                InclusionDecision::Exclude(ExclusionReason::SnapshotNotIncluded)
            };

            let batch_pos = proposed_positions.get(&id).copied();
            let snap_pos = snapshot_positions.get(&id).copied();

            inclusion_entries.push(InclusionAuditEntry::from_decision(
                id,
                fairness_class.clone(),
                age_slots,
                &decision,
                batch_pos,
                snap_pos,
            ));

            if in_proposed {
                *class_distribution.entry(fairness_class).or_insert(0) += 1;
            }
        }

        // Build ordering justifications for submitted IDs
        let mut ordering_justifications = Vec::new();
        for delta in &anti_mev_result.position_deltas {
            let policy_compliant = self.anti_mev_policy
                .validate_ordering_delta(&delta.fairness_class, delta.delta);
            ordering_justifications.push(OrderingJustificationRecord {
                submission_id: delta.submission_id,
                snapshot_position: delta.snapshot_position,
                final_position: delta.proposed_position,
                delta: delta.delta,
                justified_by: if policy_compliant {
                    "within_reorder_bound".to_string()
                } else {
                    "VIOLATION".to_string()
                },
                policy_compliant,
            });
        }

        let total_eligible = snapshot.entries.len();
        let total_included = proposed_order.len();
        let total_excluded = total_eligible.saturating_sub(total_included);

        // Check for leader discretion exceeded at batch level
        if total_eligible > 0 {
            let moved_count = anti_mev_result.position_deltas.iter().filter(|d| d.delta != 0).count();
            let moved_bps = (moved_count as u64 * 10000)
                / total_eligible.max(1) as u64;
            let limit = self.fairness_policy.max_leader_discretion_bps as u64;
            if moved_bps > limit {
                warnings.push(format!(
                    "Leader discretion {}bps exceeds policy limit {}bps",
                    moved_bps, limit
                ));
            }
        }

        // Compute snapshot commitment for audit
        let snapshot_commitment = crate::queue_snapshot::snapshot::SnapshotCommitment::compute(snapshot, leader_id);

        let audit_record = FairnessAuditRecord::build(
            batch_id,
            slot,
            epoch,
            leader_id,
            snapshot_commitment,
            self.fairness_policy.version,
            total_eligible,
            total_included,
            total_excluded,
            forced_inclusions,
            inclusion_entries,
            ordering_justifications,
            class_distribution,
            anti_mev_result.clone(),
            warnings.clone(),
        );

        let valid = errors.is_empty();
        FairnessValidationResult {
            valid,
            errors,
            warnings,
            anti_mev_result,
            audit_record,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fairness::policy::{FairSequencingPolicy, FairnessClass};
    use crate::anti_mev::policy::AntiMevPolicy;
    use crate::queue_snapshot::snapshot::{QueueSnapshot, EligibleSubmissionEntry};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_entry(id: u8, class: FairnessClass, seq: u64) -> EligibleSubmissionEntry {
        EligibleSubmissionEntry {
            submission_id: make_id(id),
            fairness_class: class,
            received_at_slot: 1,
            received_at_sequence: seq,
            age_slots: 0,
            is_forced_inclusion: false,
        }
    }

    fn make_validator() -> FairProposalValidator {
        FairProposalValidator::new(
            FairSequencingPolicy::default_policy(),
            AntiMevPolicy::default_policy(),
        )
    }

    #[test]
    fn test_validate_compliant_proposal_passes() {
        let validator = make_validator();
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0),
            make_entry(2, FairnessClass::Normal, 1),
            make_entry(3, FairnessClass::Normal, 2),
        ];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        let ordered = snapshot.ordered_ids();
        let profiles = BTreeMap::new();

        let result = validator.validate(
            &ordered,
            &snapshot,
            &profiles,
            make_id(10),
            5,
            1,
            make_id(20),
        );
        assert!(result.valid, "Expected compliant proposal: {:?}", result.errors);
        assert!(result.errors.is_empty());
    }

    #[test]
    fn test_validate_with_reorder_violation_fails() {
        let validator = make_validator();
        // 8 entries; move last one (snapshot pos 7) to position 0 → delta = -7 > max=5
        let entries: Vec<EligibleSubmissionEntry> = (1u8..=8)
            .map(|i| make_entry(i, FairnessClass::Normal, i as u64))
            .collect();
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        let mut ordered = snapshot.ordered_ids();
        // Move last to front
        let last = ordered.pop().unwrap();
        ordered.insert(0, last);

        let result = validator.validate(
            &ordered,
            &snapshot,
            &BTreeMap::new(),
            make_id(10),
            5,
            1,
            make_id(20),
        );
        assert!(!result.valid, "Expected violation for excessive reorder");
        assert!(result.errors.iter().any(|e| matches!(e, ProposalFairnessError::ReorderBoundViolated { .. })));
    }

    #[test]
    fn test_validate_submission_not_in_snapshot_causes_error() {
        let validator = make_validator();
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0),
            make_entry(2, FairnessClass::Normal, 1),
        ];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        // Propose an ID that's not in the snapshot
        let proposed = vec![make_id(1), make_id(99)]; // id=99 not in snapshot

        let result = validator.validate(
            &proposed,
            &snapshot,
            &BTreeMap::new(),
            make_id(10),
            5,
            1,
            make_id(20),
        );
        assert!(!result.valid);
        assert!(result.errors.iter().any(|e| matches!(
            e,
            ProposalFairnessError::EligibleSubmissionOmitted { submission_id } if *submission_id == make_id(99)
        )));
    }

    #[test]
    fn test_validate_empty_proposed_order_is_valid_subset() {
        let validator = make_validator();
        let entries = vec![make_entry(1, FairnessClass::Normal, 0)];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        // Empty proposed order — valid (just doesn't include anything)
        let result = validator.validate(
            &[],
            &snapshot,
            &BTreeMap::new(),
            make_id(10),
            5,
            1,
            make_id(20),
        );
        assert!(result.valid, "Empty proposed order should be valid");
    }

    #[test]
    fn test_validate_produces_audit_record() {
        let validator = make_validator();
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0),
            make_entry(2, FairnessClass::Normal, 1),
        ];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        let ordered = snapshot.ordered_ids();
        let batch_id = make_id(77);

        let result = validator.validate(
            &ordered,
            &snapshot,
            &BTreeMap::new(),
            make_id(10),
            5,
            1,
            batch_id,
        );
        assert_eq!(result.audit_record.batch_id, batch_id);
        assert_ne!(result.audit_record.audit_hash, [0u8; 32]);
    }

    #[test]
    fn test_validate_safety_critical_reorder_by_one_fails() {
        let validator = make_validator();
        // SafetyCritical has bound 0 — any reorder should fail
        let entries = vec![
            make_entry(1, FairnessClass::SafetyCritical, 0),
            make_entry(2, FairnessClass::Normal, 1),
        ];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        // Swap them
        let proposed = vec![make_id(2), make_id(1)];

        let result = validator.validate(
            &proposed,
            &snapshot,
            &BTreeMap::new(),
            make_id(10),
            5,
            1,
            make_id(20),
        );
        assert!(!result.valid, "SafetyCritical must not be reordered at all");
    }
}
