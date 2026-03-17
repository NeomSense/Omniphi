//! Crash-recovery reconciliation engine.
//!
//! After a node restart the `ReconciliationEngine` scans durable state and
//! rebuilds consistent in-memory view from persisted records.  It explicitly
//! classifies each in-flight item and decides the safe post-crash action:
//!
//! | Persisted state                | Reconciliation action                   |
//! |--------------------------------|-----------------------------------------|
//! | Finality = Finalized, bridge = Pending | Re-export to runtime              |
//! | Finality = RuntimeDelivered, bridge = Exporting | Mark retry-pending  |
//! | Finality = RuntimeAcknowledged | Terminal — no action                    |
//! | Finality = Proposed/Attested   | Orphaned — mark Superseded              |
//! | Bridge = Rejected, retries left | Schedule retry                         |
//! | Bridge = Failed                | Terminal — audit log only              |
//! | Replay entries missing from in-mem guard | Restore into guard             |
//!
//! Every decision produces a `ReconciliationAction` that callers execute.
//! Reconciliation never auto-heals silently: all decisions are logged.

#![allow(dead_code)]

use crate::finality::{FinalityState, BatchFinalityStatus};
use crate::bridge_recovery::{BridgeDeliveryState, BridgeRecoveryRecord};
use crate::persistence::durable_store::DurableStore;

// ─── Action types ─────────────────────────────────────────────────────────────

/// A concrete action the node should take as a result of reconciliation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReconciliationAction {
    /// Re-export this batch to the runtime bridge (it was finalized but export
    /// may not have completed before the crash).
    ReexportToRuntime { batch_id: [u8; 32] },

    /// The batch was in a mid-export state at crash time.  Schedule a retry.
    RetryBridgeExport { batch_id: [u8; 32], attempt: u32 },

    /// The batch is permanently failed (max retries exceeded or fatal error).
    /// Emit to audit log; no further action.
    AuditFailed { batch_id: [u8; 32], reason: String },

    /// The batch was only partially proposed (not yet finalized) when the
    /// crash occurred.  Mark it superseded so a new leader can propose.
    MarkSuperseded { batch_id: [u8; 32] },

    /// Restore this submission ID into the in-memory replay guard.
    RestoreReplayEntry { submission_id: [u8; 32], seq: u64 },

    /// State is fully consistent and terminal — nothing to do.
    NoAction { batch_id: [u8; 32] },
}

/// Summary of a completed reconciliation pass.
#[derive(Debug, Default)]
pub struct ReconciliationReport {
    pub actions: Vec<ReconciliationAction>,
    pub total_finality_records: usize,
    pub total_bridge_records: usize,
    pub total_replay_entries: usize,
    pub inconsistencies: Vec<String>,
}

impl ReconciliationReport {
    pub fn reexport_count(&self) -> usize {
        self.actions.iter().filter(|a| matches!(a, ReconciliationAction::ReexportToRuntime { .. })).count()
    }
    pub fn retry_count(&self) -> usize {
        self.actions.iter().filter(|a| matches!(a, ReconciliationAction::RetryBridgeExport { .. })).count()
    }
    pub fn superseded_count(&self) -> usize {
        self.actions.iter().filter(|a| matches!(a, ReconciliationAction::MarkSuperseded { .. })).count()
    }
    pub fn replay_restore_count(&self) -> usize {
        self.actions.iter().filter(|a| matches!(a, ReconciliationAction::RestoreReplayEntry { .. })).count()
    }
}

// ─── ReconciliationEngine ─────────────────────────────────────────────────────

/// Stateless reconciliation logic — all decisions are pure functions of
/// persisted finality + bridge records.
pub struct ReconciliationEngine {
    pub max_bridge_attempts: u32,
}

impl ReconciliationEngine {
    pub fn new(max_bridge_attempts: u32) -> Self {
        ReconciliationEngine { max_bridge_attempts }
    }

    pub fn default_policy() -> Self {
        Self::new(3)
    }

    /// Run a full reconciliation pass against `store`.
    ///
    /// Returns a `ReconciliationReport` describing every action that must be
    /// taken.  The caller is responsible for executing the actions.
    pub fn reconcile(&self, store: &DurableStore) -> ReconciliationReport {
        let mut report = ReconciliationReport::default();

        let finality_statuses = store.scan_finality_statuses();
        let bridge_records = store.scan_bridge_records();
        let replay_entries = store.scan_replay_entries();

        report.total_finality_records = finality_statuses.len();
        report.total_bridge_records = bridge_records.len();
        report.total_replay_entries = replay_entries.len();

        // Build a bridge-record lookup by batch_id.
        let bridge_map: std::collections::BTreeMap<[u8; 32], &BridgeRecoveryRecord> =
            bridge_records.iter().map(|r| (r.batch_id, r)).collect();

        // ── Finality-driven reconciliation ───────────────────────────────────
        for status in &finality_statuses {
            let action = self.reconcile_finality_record(status, bridge_map.get(&status.batch_id).copied());
            report.actions.push(action);
        }

        // ── Bridge records without a matching finality entry ─────────────────
        // (could happen if finality was persisted then immediately deleted — defensive)
        let finality_ids: std::collections::BTreeSet<[u8; 32]> =
            finality_statuses.iter().map(|s| s.batch_id).collect();
        for record in &bridge_records {
            if !finality_ids.contains(&record.batch_id) {
                report.inconsistencies.push(format!(
                    "bridge record for batch {:?} has no finality entry",
                    &record.batch_id[..4]
                ));
                // If bridge is in a non-terminal state, flag it.
                if !record.state.is_terminal() {
                    report.actions.push(ReconciliationAction::AuditFailed {
                        batch_id: record.batch_id,
                        reason: "bridge record orphaned — no finality entry".into(),
                    });
                }
            }
        }

        // ── Replay-guard restoration ─────────────────────────────────────────
        for (submission_id, seq) in replay_entries {
            report.actions.push(ReconciliationAction::RestoreReplayEntry { submission_id, seq });
        }

        report
    }

    fn reconcile_finality_record(
        &self,
        status: &BatchFinalityStatus,
        bridge: Option<&BridgeRecoveryRecord>,
    ) -> ReconciliationAction {
        use FinalityState::*;

        match &status.current_state {
            // ── Terminal finality states ──────────────────────────────────────
            RuntimeAcknowledged => ReconciliationAction::NoAction { batch_id: status.batch_id },
            Superseded | Invalidated => ReconciliationAction::NoAction { batch_id: status.batch_id },

            // ── Finalized but delivery status unknown ─────────────────────────
            Finalized => {
                match bridge {
                    None => {
                        // Crash before bridge record was created — re-export.
                        ReconciliationAction::ReexportToRuntime { batch_id: status.batch_id }
                    }
                    Some(rec) => self.reconcile_bridge(status.batch_id, rec),
                }
            }

            // ── Runtime delivery in progress ──────────────────────────────────
            RuntimeDelivered => {
                match bridge {
                    None => ReconciliationAction::ReexportToRuntime { batch_id: status.batch_id },
                    Some(rec) => self.reconcile_bridge(status.batch_id, rec),
                }
            }

            // ── Recovery states ───────────────────────────────────────────────
            Recovered | RuntimeRejected => {
                match bridge {
                    None => ReconciliationAction::ReexportToRuntime { batch_id: status.batch_id },
                    Some(rec) => self.reconcile_bridge(status.batch_id, rec),
                }
            }

            // ── Pre-finalization: incomplete proposals ────────────────────────
            // If these exist at restart they are orphaned — a new slot/epoch
            // will supersede them via normal leader election.
            Proposed | Attested | QuorumReached => {
                ReconciliationAction::MarkSuperseded { batch_id: status.batch_id }
            }

            // ── Disputed — needs human/governance resolution ──────────────────
            DisputedPlaceholder => {
                ReconciliationAction::AuditFailed {
                    batch_id: status.batch_id,
                    reason: "batch in DisputedPlaceholder at restart — requires governance resolution".into(),
                }
            }
        }
    }

    fn reconcile_bridge(
        &self,
        batch_id: [u8; 32],
        rec: &BridgeRecoveryRecord,
    ) -> ReconciliationAction {
        use BridgeDeliveryState::*;
        match &rec.state {
            Acknowledged | RecoveredAck => ReconciliationAction::NoAction { batch_id },
            Failed => ReconciliationAction::AuditFailed {
                batch_id,
                reason: format!("bridge permanently failed after {} attempts", rec.attempts),
            },
            Pending => ReconciliationAction::ReexportToRuntime { batch_id },
            Exported | Exporting => {
                // We don't know if the runtime received it — safe to retry.
                if rec.attempts < self.max_bridge_attempts {
                    ReconciliationAction::RetryBridgeExport { batch_id, attempt: rec.attempts }
                } else {
                    ReconciliationAction::AuditFailed {
                        batch_id,
                        reason: format!("max bridge attempts ({}) reached at crash", self.max_bridge_attempts),
                    }
                }
            }
            Rejected => {
                if rec.attempts < self.max_bridge_attempts {
                    ReconciliationAction::RetryBridgeExport { batch_id, attempt: rec.attempts }
                } else {
                    ReconciliationAction::AuditFailed {
                        batch_id,
                        reason: "max bridge attempts reached; batch rejected".into(),
                    }
                }
            }
            RetryPending { attempt } => {
                if *attempt < self.max_bridge_attempts {
                    ReconciliationAction::RetryBridgeExport { batch_id, attempt: *attempt }
                } else {
                    ReconciliationAction::AuditFailed {
                        batch_id,
                        reason: "max bridge retries exhausted".into(),
                    }
                }
            }
        }
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::persistence::backend::InMemoryBackend;
    use crate::persistence::engine::PersistenceEngine;
    use crate::persistence::durable_store::DurableStore;
    use crate::finality::{FinalityStore, FinalityState};
    use crate::bridge_recovery::{BridgeRecoveryStore, BridgeRetryPolicy};

    fn make_store() -> DurableStore {
        DurableStore::new(PersistenceEngine::new(Box::new(InMemoryBackend::new())))
    }

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn ack_hash(b: u8) -> [u8; 32] {
        let mut h = [0xAAu8; 32];
        h[0] = b;
        h
    }

    fn engine() -> ReconciliationEngine {
        ReconciliationEngine::new(3)
    }

    // ── helpers ──

    /// Write a BatchFinalityStatus at the given state into the durable store.
    fn write_finality(ds: &mut DurableStore, batch_id: [u8; 32], state: FinalityState) {
        let mut fs = FinalityStore::new();
        fs.init_batch(batch_id, 1, 1);
        // Transition through states
        let path: Vec<FinalityState> = match state {
            FinalityState::Proposed => vec![],
            FinalityState::Attested => vec![FinalityState::Attested],
            FinalityState::QuorumReached => vec![FinalityState::Attested, FinalityState::QuorumReached],
            FinalityState::Finalized => vec![
                FinalityState::Attested,
                FinalityState::QuorumReached,
                FinalityState::Finalized,
            ],
            FinalityState::RuntimeDelivered => vec![
                FinalityState::Attested,
                FinalityState::QuorumReached,
                FinalityState::Finalized,
                FinalityState::RuntimeDelivered,
            ],
            FinalityState::RuntimeAcknowledged => vec![
                FinalityState::Attested,
                FinalityState::QuorumReached,
                FinalityState::Finalized,
                FinalityState::RuntimeDelivered,
                FinalityState::RuntimeAcknowledged,
            ],
            FinalityState::Superseded => vec![FinalityState::Superseded],
            _ => vec![], // just leave as Proposed for other states
        };
        for s in path {
            let _ = fs.transition(&batch_id, s, "test");
        }
        let status = fs.get(&batch_id).unwrap().clone();
        ds.put_finality_status(&status);
    }

    fn write_bridge(ds: &mut DurableStore, batch_id: [u8; 32], state: BridgeDeliveryState) {
        let mut brs = BridgeRecoveryStore::new(BridgeRetryPolicy::default());
        brs.register_batch(batch_id);
        let record = match state {
            BridgeDeliveryState::Pending => brs.get_record(&batch_id).unwrap().clone(),
            BridgeDeliveryState::Exporting => {
                brs.export_batch(&batch_id).unwrap();
                brs.get_record(&batch_id).unwrap().clone()
            }
            BridgeDeliveryState::Acknowledged => {
                brs.export_batch(&batch_id).unwrap();
                brs.acknowledge(&batch_id, ack_hash(1)).unwrap();
                brs.get_record(&batch_id).unwrap().clone()
            }
            BridgeDeliveryState::Failed => {
                // exhaust retries
                for _ in 0..3 {
                    brs.export_batch(&batch_id).unwrap_or(());
                    let _ = brs.reject_and_retry(&batch_id);
                }
                brs.get_record(&batch_id).unwrap().clone()
            }
            _ => brs.get_record(&batch_id).unwrap().clone(),
        };
        ds.put_bridge_record(&record);
    }

    // ── tests ──

    #[test]
    fn test_empty_store_produces_no_actions() {
        let ds = make_store();
        let report = engine().reconcile(&ds);
        assert!(report.actions.is_empty());
    }

    #[test]
    fn test_finalized_no_bridge_schedules_reexport() {
        let mut ds = make_store();
        let bid = make_id(1);
        write_finality(&mut ds, bid, FinalityState::Finalized);
        let report = engine().reconcile(&ds);
        assert_eq!(report.reexport_count(), 1);
        assert!(matches!(
            report.actions[0],
            ReconciliationAction::ReexportToRuntime { batch_id } if batch_id == bid
        ));
    }

    #[test]
    fn test_acked_produces_no_action() {
        let mut ds = make_store();
        let bid = make_id(2);
        write_finality(&mut ds, bid, FinalityState::RuntimeAcknowledged);
        write_bridge(&mut ds, bid, BridgeDeliveryState::Acknowledged);
        let report = engine().reconcile(&ds);
        assert!(matches!(report.actions[0], ReconciliationAction::NoAction { .. }));
    }

    #[test]
    fn test_exporting_schedules_retry() {
        let mut ds = make_store();
        let bid = make_id(3);
        write_finality(&mut ds, bid, FinalityState::RuntimeDelivered);
        write_bridge(&mut ds, bid, BridgeDeliveryState::Exporting);
        let report = engine().reconcile(&ds);
        assert_eq!(report.retry_count(), 1);
    }

    #[test]
    fn test_proposed_marks_superseded() {
        let mut ds = make_store();
        let bid = make_id(4);
        write_finality(&mut ds, bid, FinalityState::Proposed);
        let report = engine().reconcile(&ds);
        assert_eq!(report.superseded_count(), 1);
        assert!(matches!(report.actions[0], ReconciliationAction::MarkSuperseded { .. }));
    }

    #[test]
    fn test_attested_marks_superseded() {
        let mut ds = make_store();
        let bid = make_id(5);
        write_finality(&mut ds, bid, FinalityState::Attested);
        let report = engine().reconcile(&ds);
        assert_eq!(report.superseded_count(), 1);
    }

    #[test]
    fn test_bridge_failed_produces_audit() {
        let mut ds = make_store();
        let bid = make_id(6);
        write_finality(&mut ds, bid, FinalityState::RuntimeDelivered);
        // Directly write a failed bridge record
        let mut record = BridgeRecoveryRecord::new(bid);
        record.state = BridgeDeliveryState::Failed;
        record.attempts = 3;
        ds.put_bridge_record(&record);
        let report = engine().reconcile(&ds);
        assert!(report.actions.iter().any(|a| matches!(a, ReconciliationAction::AuditFailed { .. })));
    }

    #[test]
    fn test_replay_entries_restored() {
        let mut ds = make_store();
        let sid1 = make_id(10);
        let sid2 = make_id(11);
        ds.put_replay_entry(&sid1, 1);
        ds.put_replay_entry(&sid2, 2);
        let report = engine().reconcile(&ds);
        assert_eq!(report.replay_restore_count(), 2);
        // Must be in insertion-seq order
        let restores: Vec<_> = report.actions.iter().filter_map(|a| {
            if let ReconciliationAction::RestoreReplayEntry { submission_id, seq } = a {
                Some((*submission_id, *seq))
            } else { None }
        }).collect();
        assert_eq!(restores[0].1, 1);
        assert_eq!(restores[1].1, 2);
    }

    #[test]
    fn test_orphaned_bridge_record_flagged() {
        let mut ds = make_store();
        let bid = make_id(20);
        // Write a bridge record with NO matching finality record
        let mut record = BridgeRecoveryRecord::new(bid);
        record.state = BridgeDeliveryState::Exporting;
        ds.put_bridge_record(&record);
        let report = engine().reconcile(&ds);
        assert!(!report.inconsistencies.is_empty());
    }

    #[test]
    fn test_superseded_produces_no_action() {
        let mut ds = make_store();
        let bid = make_id(21);
        write_finality(&mut ds, bid, FinalityState::Superseded);
        let report = engine().reconcile(&ds);
        assert!(matches!(report.actions[0], ReconciliationAction::NoAction { .. }));
    }

    #[test]
    fn test_multiple_batches_reconciled_independently() {
        let mut ds = make_store();
        let b1 = make_id(30); // acked — no action
        let b2 = make_id(31); // finalized, no bridge — reexport
        let b3 = make_id(32); // proposed — superseded

        write_finality(&mut ds, b1, FinalityState::RuntimeAcknowledged);
        write_bridge(&mut ds, b1, BridgeDeliveryState::Acknowledged);
        write_finality(&mut ds, b2, FinalityState::Finalized);
        write_finality(&mut ds, b3, FinalityState::Proposed);

        let report = engine().reconcile(&ds);
        assert_eq!(report.total_finality_records, 3);
        assert_eq!(report.reexport_count(), 1);
        assert_eq!(report.superseded_count(), 1);

        let no_action_count = report.actions.iter().filter(|a| matches!(a, ReconciliationAction::NoAction { .. })).count();
        assert_eq!(no_action_count, 1);
    }
}
