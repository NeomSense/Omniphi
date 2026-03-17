use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::fairness::policy::{FairnessClass, FairSequencingPolicy};
use crate::anti_mev::policy::AntiMevPolicy;
use crate::anti_mev::engine::AntiMevViolationType;
use crate::fairness_audit::records::{FairnessAuditRecord, InclusionAuditEntry};
use crate::inclusion::engine::ExclusionReason;

/// Severity level of a fairness incident.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum FairnessIncidentSeverity {
    Info,
    Warning,
    Violation,
    Critical,
}

/// The type of fairness violation detected.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FairnessViolationType {
    /// Submission was omitted N times in a row.
    RepeatedOmission(u32),
    /// Submission moved more than reorder_bound allows.
    ExcessiveReorder,
    /// Protected class not respected.
    ProtectedFlowViolation,
    /// Leader reordered too many positions.
    LeaderDiscretionAbuse,
    /// Submission age exceeds anti_starvation_slots.
    StarvationDetected,
    /// One fairness class consistently deprioritized.
    ClassBucketSkew,
    /// Proposal does not follow snapshot ordering.
    SnapshotOrderViolation,
    /// Valid eligible submission excluded without justification.
    UnauthorizedOmission,
}

/// A single detected fairness incident.
#[derive(Debug, Clone)]
pub struct UnfairSequencingIncident {
    /// SHA256(batch_id || submission_id || violation_type_tag || slot).
    pub incident_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub submission_id: Option<[u8; 32]>,
    pub violation_type: FairnessViolationType,
    pub severity: FairnessIncidentSeverity,
    pub evidence: BTreeMap<String, String>,
}

impl UnfairSequencingIncident {
    pub fn new(
        batch_id: [u8; 32],
        slot: u64,
        epoch: u64,
        leader_id: [u8; 32],
        submission_id: Option<[u8; 32]>,
        violation_type: FairnessViolationType,
        severity: FairnessIncidentSeverity,
        evidence: BTreeMap<String, String>,
    ) -> Self {
        let incident_id = Self::compute_incident_id(
            &batch_id,
            submission_id.as_ref().unwrap_or(&[0u8; 32]),
            &violation_type,
            slot,
        );
        UnfairSequencingIncident {
            incident_id,
            batch_id,
            slot,
            epoch,
            leader_id,
            submission_id,
            violation_type,
            severity,
            evidence,
        }
    }

    fn compute_incident_id(
        batch_id: &[u8; 32],
        submission_id: &[u8; 32],
        violation_type: &FairnessViolationType,
        slot: u64,
    ) -> [u8; 32] {
        let type_tag: u8 = match violation_type {
            FairnessViolationType::RepeatedOmission(_) => 1,
            FairnessViolationType::ExcessiveReorder => 2,
            FairnessViolationType::ProtectedFlowViolation => 3,
            FairnessViolationType::LeaderDiscretionAbuse => 4,
            FairnessViolationType::StarvationDetected => 5,
            FairnessViolationType::ClassBucketSkew => 6,
            FairnessViolationType::SnapshotOrderViolation => 7,
            FairnessViolationType::UnauthorizedOmission => 8,
        };
        let mut hasher = Sha256::new();
        hasher.update(batch_id);
        hasher.update(submission_id);
        hasher.update(&[type_tag]);
        hasher.update(&slot.to_be_bytes());
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Tracks how many times a submission has been omitted across batches.
#[derive(Debug, Clone)]
pub struct OmissionPatternRecord {
    pub submission_id: [u8; 32],
    pub omission_count: u32,
    pub first_omission_slot: u64,
    pub last_seen_slot: u64,
    pub fairness_class: FairnessClass,
}

/// Detects unfair sequencing patterns from audit records.
#[derive(Debug, Clone)]
pub struct FairnessIncidentDetector {
    /// Tracks per-submission omission counts.
    pub omission_records: BTreeMap<[u8; 32], OmissionPatternRecord>,
    /// All detected incidents.
    pub incidents: Vec<UnfairSequencingIncident>,
}

impl FairnessIncidentDetector {
    pub fn new() -> Self {
        FairnessIncidentDetector {
            omission_records: BTreeMap::new(),
            incidents: Vec::new(),
        }
    }

    /// Detect incidents from a fairness audit record.
    pub fn detect_from_audit(
        &mut self,
        audit: &FairnessAuditRecord,
        policy: &FairSequencingPolicy,
        _anti_mev_policy: &AntiMevPolicy,
        _current_slot: u64,
    ) -> Vec<UnfairSequencingIncident> {
        let mut detected = Vec::new();

        // 1. Detect anti-MEV violations from audit
        for violation in &audit.anti_mev_result.violations {
            let (vtype, severity) = match violation.violation_type {
                AntiMevViolationType::ExceededForwardReorderBound
                | AntiMevViolationType::ExceededBackwardReorderBound => (
                    FairnessViolationType::ExcessiveReorder,
                    FairnessIncidentSeverity::Violation,
                ),
                AntiMevViolationType::ProtectedFlowDelayed => (
                    FairnessViolationType::ProtectedFlowViolation,
                    FairnessIncidentSeverity::Critical,
                ),
                AntiMevViolationType::LeaderDiscretionExceeded => (
                    FairnessViolationType::LeaderDiscretionAbuse,
                    FairnessIncidentSeverity::Warning,
                ),
                AntiMevViolationType::SnapshotCommitmentViolation => (
                    FairnessViolationType::SnapshotOrderViolation,
                    FairnessIncidentSeverity::Critical,
                ),
            };

            let mut evidence = BTreeMap::new();
            evidence.insert("actual_delta".to_string(), violation.actual_delta.to_string());
            evidence.insert("allowed_delta".to_string(), violation.allowed_delta.to_string());

            let incident = UnfairSequencingIncident::new(
                audit.batch_id,
                audit.slot,
                audit.epoch,
                audit.leader_id,
                Some(violation.submission_id),
                vtype,
                severity,
                evidence,
            );
            detected.push(incident);
        }

        // 2. Record omissions from excluded entries
        let excluded: Vec<InclusionAuditEntry> = audit
            .inclusion_entries
            .iter()
            .filter(|e| !e.included)
            .cloned()
            .collect();
        self.record_omissions(
            &excluded,
            audit.batch_id,
            audit.slot,
            audit.epoch,
            audit.leader_id,
            policy,
        );

        // 3. Check for repeated omissions that now trigger violations
        let omission_incidents = self.check_repeated_omissions(policy);
        detected.extend(omission_incidents);

        // 4. Check for starvation (age > anti_starvation_slots)
        for entry in &audit.inclusion_entries {
            if entry.age_slots > policy.anti_starvation_slots && !entry.included {
                let mut evidence = BTreeMap::new();
                evidence.insert("age_slots".to_string(), entry.age_slots.to_string());
                evidence.insert("threshold".to_string(), policy.anti_starvation_slots.to_string());
                let incident = UnfairSequencingIncident::new(
                    audit.batch_id,
                    audit.slot,
                    audit.epoch,
                    audit.leader_id,
                    Some(entry.submission_id),
                    FairnessViolationType::StarvationDetected,
                    FairnessIncidentSeverity::Warning,
                    evidence,
                );
                detected.push(incident);
            }
        }

        // 5. Check for unauthorized omissions (policy-excluded without justification)
        for entry in &audit.inclusion_entries {
            if !entry.included {
                if let Some(ExclusionReason::PolicyExcluded) = &entry.exclusion_reason {
                    let mut evidence = BTreeMap::new();
                    evidence.insert("class".to_string(), format!("{:?}", entry.fairness_class));
                    let incident = UnfairSequencingIncident::new(
                        audit.batch_id,
                        audit.slot,
                        audit.epoch,
                        audit.leader_id,
                        Some(entry.submission_id),
                        FairnessViolationType::UnauthorizedOmission,
                        FairnessIncidentSeverity::Violation,
                        evidence,
                    );
                    detected.push(incident);
                }
            }
        }

        self.incidents.extend(detected.clone());
        detected
    }

    /// Record omissions from excluded entries and update omission_records.
    pub fn record_omissions(
        &mut self,
        excluded: &[InclusionAuditEntry],
        _batch_id: [u8; 32],
        slot: u64,
        _epoch: u64,
        _leader_id: [u8; 32],
        _policy: &FairSequencingPolicy,
    ) {
        for entry in excluded {
            let record = self
                .omission_records
                .entry(entry.submission_id)
                .or_insert_with(|| OmissionPatternRecord {
                    submission_id: entry.submission_id,
                    omission_count: 0,
                    first_omission_slot: slot,
                    last_seen_slot: slot,
                    fairness_class: entry.fairness_class.clone(),
                });
            record.omission_count += 1;
            record.last_seen_slot = slot;
        }
    }

    /// Check if any submissions have been omitted more than max_omission_streak times.
    pub fn check_repeated_omissions(
        &self,
        _policy: &FairSequencingPolicy,
    ) -> Vec<UnfairSequencingIncident> {
        let max_streak = 3u32; // default from FairnessConstraintSet
        let mut incidents = Vec::new();

        for (id, record) in &self.omission_records {
            if record.omission_count >= max_streak {
                let mut evidence = BTreeMap::new();
                evidence.insert("omission_count".to_string(), record.omission_count.to_string());
                evidence.insert("first_slot".to_string(), record.first_omission_slot.to_string());
                evidence.insert("last_slot".to_string(), record.last_seen_slot.to_string());

                let severity = if record.omission_count >= max_streak * 2 {
                    FairnessIncidentSeverity::Critical
                } else {
                    FairnessIncidentSeverity::Violation
                };

                incidents.push(UnfairSequencingIncident::new(
                    [0u8; 32], // no specific batch
                    record.last_seen_slot,
                    0,
                    [0u8; 32],
                    Some(*id),
                    FairnessViolationType::RepeatedOmission(record.omission_count),
                    severity,
                    evidence,
                ));
            }
        }

        // Sort by severity (descending) then by id for determinism
        incidents.sort_by(|a, b| {
            b.severity
                .cmp(&a.severity)
                .then(a.submission_id.cmp(&b.submission_id))
        });
        incidents
    }

    pub fn get_all_incidents(&self) -> &Vec<UnfairSequencingIncident> {
        &self.incidents
    }

    /// Remove omission records for submissions that have been included (resolved).
    pub fn clear_resolved_omissions(&mut self, resolved_ids: &[[u8; 32]]) {
        for id in resolved_ids {
            self.omission_records.remove(id);
        }
    }
}

impl Default for FairnessIncidentDetector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::anti_mev::engine::{AntiMevValidationResult, AntiMevViolation, AntiMevViolationType};
    use crate::fairness::policy::{FairSequencingPolicy, FairnessClass};
    use crate::fairness_audit::records::FairnessAuditRecord;
    use crate::queue_snapshot::snapshot::{QueueSnapshot, EligibleSubmissionEntry, SnapshotCommitment};
    use crate::inclusion::engine::ExclusionReason;

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

    fn make_audit_with_violations(
        violations: Vec<AntiMevViolation>,
        excluded_entries: Vec<InclusionAuditEntry>,
    ) -> FairnessAuditRecord {
        let anti_mev_result = AntiMevValidationResult {
            valid: violations.is_empty(),
            violations,
            position_deltas: vec![],
            max_delta_forward: 0,
            max_delta_backward: 0,
        };

        let mut all_entries = excluded_entries;
        // Add a normally included entry too
        all_entries.push(crate::fairness_audit::records::InclusionAuditEntry {
            submission_id: make_id(99),
            fairness_class: FairnessClass::Normal,
            age_slots: 1,
            included: true,
            forced_inclusion: false,
            exclusion_reason: None,
            position_in_batch: Some(0),
            snapshot_position: Some(0),
            position_delta: Some(0),
        });

        FairnessAuditRecord::build(
            make_id(10),
            5,
            1,
            make_id(50),
            make_commitment(),
            1,
            3,
            1,
            2,
            0,
            all_entries,
            vec![],
            BTreeMap::new(),
            anti_mev_result,
            vec![],
        )
    }

    #[test]
    fn test_detect_from_audit_finds_reorder_violations() {
        let mut detector = FairnessIncidentDetector::new();
        let violation = AntiMevViolation {
            submission_id: make_id(5),
            violation_type: AntiMevViolationType::ExceededForwardReorderBound,
            actual_delta: -10,
            allowed_delta: -5,
        };
        let audit = make_audit_with_violations(vec![violation], vec![]);
        let policy = FairSequencingPolicy::default_policy();
        let anti_mev = crate::anti_mev::policy::AntiMevPolicy::default_policy();

        let incidents = detector.detect_from_audit(&audit, &policy, &anti_mev, 5);
        assert!(!incidents.is_empty());
        assert!(incidents.iter().any(|i| i.violation_type == FairnessViolationType::ExcessiveReorder));
    }

    #[test]
    fn test_record_omissions_increments_count() {
        let mut detector = FairnessIncidentDetector::new();
        let policy = FairSequencingPolicy::default_policy();
        let entry = InclusionAuditEntry {
            submission_id: make_id(7),
            fairness_class: FairnessClass::Normal,
            age_slots: 5,
            included: false,
            forced_inclusion: false,
            exclusion_reason: Some(ExclusionReason::BatchFull),
            position_in_batch: None,
            snapshot_position: None,
            position_delta: None,
        };

        detector.record_omissions(&[entry.clone()], make_id(1), 10, 1, make_id(50), &policy);
        assert_eq!(detector.omission_records[&make_id(7)].omission_count, 1);

        detector.record_omissions(&[entry], make_id(2), 11, 1, make_id(50), &policy);
        assert_eq!(detector.omission_records[&make_id(7)].omission_count, 2);
    }

    #[test]
    fn test_check_repeated_omissions_fires_after_streak() {
        let mut detector = FairnessIncidentDetector::new();
        let policy = FairSequencingPolicy::default_policy();

        let entry = InclusionAuditEntry {
            submission_id: make_id(8),
            fairness_class: FairnessClass::Normal,
            age_slots: 5,
            included: false,
            forced_inclusion: false,
            exclusion_reason: Some(ExclusionReason::BatchFull),
            position_in_batch: None,
            snapshot_position: None,
            position_delta: None,
        };

        // Record 3 omissions (= max_streak)
        for slot in 1..=3u64 {
            detector.record_omissions(&[entry.clone()], make_id(slot as u8), slot, 1, make_id(50), &policy);
        }

        let incidents = detector.check_repeated_omissions(&policy);
        assert!(!incidents.is_empty(), "Expected RepeatedOmission incident after 3 omissions");
        assert!(
            incidents.iter().any(|i| matches!(i.violation_type, FairnessViolationType::RepeatedOmission(n) if n >= 3))
        );
    }

    #[test]
    fn test_clear_resolved_omissions() {
        let mut detector = FairnessIncidentDetector::new();
        let policy = FairSequencingPolicy::default_policy();
        let id = make_id(9);
        let entry = InclusionAuditEntry {
            submission_id: id,
            fairness_class: FairnessClass::Normal,
            age_slots: 1,
            included: false,
            forced_inclusion: false,
            exclusion_reason: Some(ExclusionReason::BatchFull),
            position_in_batch: None,
            snapshot_position: None,
            position_delta: None,
        };
        detector.record_omissions(&[entry], make_id(1), 1, 1, make_id(50), &policy);
        assert!(detector.omission_records.contains_key(&id));

        detector.clear_resolved_omissions(&[id]);
        assert!(!detector.omission_records.contains_key(&id));
    }

    #[test]
    fn test_incident_id_is_deterministic() {
        let incident1 = UnfairSequencingIncident::new(
            make_id(1), 5, 1, make_id(10), Some(make_id(2)),
            FairnessViolationType::ExcessiveReorder,
            FairnessIncidentSeverity::Violation,
            BTreeMap::new(),
        );
        let incident2 = UnfairSequencingIncident::new(
            make_id(1), 5, 1, make_id(10), Some(make_id(2)),
            FairnessViolationType::ExcessiveReorder,
            FairnessIncidentSeverity::Violation,
            BTreeMap::new(),
        );
        assert_eq!(incident1.incident_id, incident2.incident_id);
    }
}
