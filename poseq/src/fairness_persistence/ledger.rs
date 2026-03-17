use std::collections::BTreeMap;
use crate::fairness::policy::FairnessClass;
use crate::fairness_audit::records::FairnessAuditRecord;
use crate::fairness_incidents::detector::UnfairSequencingIncident;
use crate::queue_snapshot::snapshot::SnapshotCommitment;

/// Record of a forced inclusion event for persistence.
#[derive(Debug, Clone)]
pub struct ForcedInclusionRecord {
    pub submission_id: [u8; 32],
    pub forced_in_batch: [u8; 32],
    pub forced_at_slot: u64,
    pub age_at_forcing: u64,
    pub fairness_class: FairnessClass,
    pub reason: String,
}

/// Per-leader compliance entry for a single batch.
#[derive(Debug, Clone)]
pub struct LeaderFairnessEntry {
    pub leader_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub slot: u64,
    pub compliant: bool,
    pub violation_count: u32,
    pub incident_ids: Vec<[u8; 32]>,
}

/// Persistent ledger of all fairness-related records across batches.
#[derive(Debug, Clone)]
pub struct FairnessLedger {
    /// Keyed by batch_id.
    pub audit_records: BTreeMap<[u8; 32], FairnessAuditRecord>,
    pub incidents: Vec<UnfairSequencingIncident>,
    pub forced_inclusions: Vec<ForcedInclusionRecord>,
    /// Keyed by snapshot_id.
    pub snapshot_commitments: BTreeMap<[u8; 32], SnapshotCommitment>,
    /// Keyed by leader_id → list of entries.
    pub leader_history: BTreeMap<[u8; 32], Vec<LeaderFairnessEntry>>,
}

impl FairnessLedger {
    pub fn new() -> Self {
        FairnessLedger {
            audit_records: BTreeMap::new(),
            incidents: Vec::new(),
            forced_inclusions: Vec::new(),
            snapshot_commitments: BTreeMap::new(),
            leader_history: BTreeMap::new(),
        }
    }

    pub fn record_audit(&mut self, record: FairnessAuditRecord) {
        self.audit_records.insert(record.batch_id, record);
    }

    pub fn record_incident(&mut self, incident: UnfairSequencingIncident) {
        self.incidents.push(incident);
    }

    pub fn record_forced_inclusion(&mut self, record: ForcedInclusionRecord) {
        self.forced_inclusions.push(record);
    }

    pub fn record_snapshot_commitment(&mut self, commitment: SnapshotCommitment) {
        self.snapshot_commitments.insert(commitment.snapshot_id, commitment);
    }

    pub fn record_leader_entry(&mut self, entry: LeaderFairnessEntry) {
        self.leader_history
            .entry(entry.leader_id)
            .or_insert_with(Vec::new)
            .push(entry);
    }

    pub fn get_audit(&self, batch_id: &[u8; 32]) -> Option<&FairnessAuditRecord> {
        self.audit_records.get(batch_id)
    }

    pub fn get_leader_history(&self, leader_id: &[u8; 32]) -> Vec<&LeaderFairnessEntry> {
        self.leader_history
            .get(leader_id)
            .map(|entries| entries.iter().collect())
            .unwrap_or_default()
    }

    pub fn get_incidents_for_leader(&self, leader_id: &[u8; 32]) -> Vec<&UnfairSequencingIncident> {
        self.incidents
            .iter()
            .filter(|i| &i.leader_id == leader_id)
            .collect()
    }
}

impl Default for FairnessLedger {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;
    use crate::fairness::policy::FairnessClass;
    use crate::fairness_audit::records::FairnessAuditRecord;
    use crate::fairness_incidents::detector::{UnfairSequencingIncident, FairnessViolationType, FairnessIncidentSeverity};
    use crate::anti_mev::engine::AntiMevValidationResult;
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

    fn make_audit(batch_id: [u8; 32]) -> FairnessAuditRecord {
        FairnessAuditRecord::build(
            batch_id, 1, 1, make_id(99), make_commitment(), 1,
            2, 2, 0, 0, vec![], vec![], BTreeMap::new(),
            AntiMevValidationResult::default(), vec![],
        )
    }

    #[test]
    fn test_record_audit_and_get_audit() {
        let mut ledger = FairnessLedger::new();
        let batch_id = make_id(10);
        let audit = make_audit(batch_id);
        ledger.record_audit(audit);
        let retrieved = ledger.get_audit(&batch_id);
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().batch_id, batch_id);
    }

    #[test]
    fn test_get_audit_returns_none_for_unknown() {
        let ledger = FairnessLedger::new();
        assert!(ledger.get_audit(&make_id(99)).is_none());
    }

    #[test]
    fn test_record_leader_entry_and_get_leader_history() {
        let mut ledger = FairnessLedger::new();
        let leader_id = make_id(5);
        let entry1 = LeaderFairnessEntry {
            leader_id,
            batch_id: make_id(1),
            slot: 10,
            compliant: true,
            violation_count: 0,
            incident_ids: vec![],
        };
        let entry2 = LeaderFairnessEntry {
            leader_id,
            batch_id: make_id(2),
            slot: 11,
            compliant: false,
            violation_count: 2,
            incident_ids: vec![make_id(99)],
        };
        ledger.record_leader_entry(entry1);
        ledger.record_leader_entry(entry2);

        let history = ledger.get_leader_history(&leader_id);
        assert_eq!(history.len(), 2);
        assert_eq!(history[0].slot, 10);
        assert_eq!(history[1].slot, 11);
    }

    #[test]
    fn test_get_leader_history_returns_empty_for_unknown() {
        let ledger = FairnessLedger::new();
        assert!(ledger.get_leader_history(&make_id(99)).is_empty());
    }

    #[test]
    fn test_get_incidents_for_leader_filters_correctly() {
        let mut ledger = FairnessLedger::new();
        let leader_a = make_id(1);
        let leader_b = make_id(2);

        let incident_a = UnfairSequencingIncident::new(
            make_id(10), 5, 1, leader_a, None,
            FairnessViolationType::ExcessiveReorder,
            FairnessIncidentSeverity::Warning,
            BTreeMap::new(),
        );
        let incident_b = UnfairSequencingIncident::new(
            make_id(11), 6, 1, leader_b, None,
            FairnessViolationType::ExcessiveReorder,
            FairnessIncidentSeverity::Warning,
            BTreeMap::new(),
        );
        ledger.record_incident(incident_a);
        ledger.record_incident(incident_b);

        let incidents_a = ledger.get_incidents_for_leader(&leader_a);
        let incidents_b = ledger.get_incidents_for_leader(&leader_b);
        assert_eq!(incidents_a.len(), 1);
        assert_eq!(incidents_b.len(), 1);
        assert_eq!(incidents_a[0].leader_id, leader_a);
    }

    #[test]
    fn test_record_snapshot_commitment() {
        let mut ledger = FairnessLedger::new();
        let commitment = make_commitment();
        let snap_id = commitment.snapshot_id;
        ledger.record_snapshot_commitment(commitment);
        assert!(ledger.snapshot_commitments.contains_key(&snap_id));
    }

    #[test]
    fn test_record_forced_inclusion() {
        let mut ledger = FairnessLedger::new();
        ledger.record_forced_inclusion(ForcedInclusionRecord {
            submission_id: make_id(7),
            forced_in_batch: make_id(8),
            forced_at_slot: 10,
            age_at_forcing: 150,
            fairness_class: FairnessClass::Normal,
            reason: "starvation".to_string(),
        });
        assert_eq!(ledger.forced_inclusions.len(), 1);
        assert_eq!(ledger.forced_inclusions[0].submission_id, make_id(7));
    }
}
