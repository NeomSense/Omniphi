#![allow(dead_code)]

pub mod metrics;
pub mod exporter;
pub mod node_log;

pub use node_log::{NodeEventLog, NodeLogEntry, LogLevel};

// ---------------------------------------------------------------------------
// PoSeqEventKind
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum PoSeqEventKind {
    EpochStarted,
    EpochCompleted,
    CommitteeRotated,
    BatchProposed,
    BatchAttested,
    BatchFinalized,
    BatchDelivered,
    BatchAcknowledged,
    BatchRejected,
    MisbehaviorDetected,
    PenaltyRecommended,
    NodeJoined,
    NodeSuspended,
    NodeBanned,
    CheckpointCreated,
    RecoveryStarted,
    RecoveryCompleted,
    BridgeRetry,
    BridgeFailed,
}

// ---------------------------------------------------------------------------
// EventRecord
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EventRecord {
    pub seq: u64,
    pub kind: PoSeqEventKind,
    pub epoch: u64,
    pub slot: Option<u64>,
    pub node_id: Option<[u8; 32]>,
    pub batch_id: Option<[u8; 32]>,
    pub details: String,
}

// ---------------------------------------------------------------------------
// NodeStatusSnapshot
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct NodeStatusSnapshot {
    pub node_id: [u8; 32],
    pub membership_status: String,
    pub current_epoch: u64,
    pub finalized_batches: u64,
    pub misbehavior_count: u32,
    pub bridge_pending: u32,
    pub bridge_acked: u32,
}

// ---------------------------------------------------------------------------
// CommitteeHealthSummary
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CommitteeHealthSummary {
    pub epoch: u64,
    pub total_members: usize,
    pub active_members: usize,
    pub suspended_members: usize,
    pub committee_hash: [u8; 32],
}

// ---------------------------------------------------------------------------
// FinalityMetricsSummary
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FinalityMetricsSummary {
    pub epoch: u64,
    pub proposed: u64,
    pub finalized: u64,
    pub delivered: u64,
    pub failed: u64,
    pub avg_transitions_per_batch: f64,
}

// ---------------------------------------------------------------------------
// BridgeHealthSummary
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BridgeHealthSummary {
    pub total_pending: u32,
    pub total_acked: u32,
    pub total_failed: u32,
    pub total_in_retry: u32,
    pub inconsistencies: Vec<String>,
}

// ---------------------------------------------------------------------------
// ObservabilityStore
// ---------------------------------------------------------------------------

pub struct ObservabilityStore {
    events: Vec<EventRecord>,
    seq: u64,
    max_events: usize,
}

impl ObservabilityStore {
    pub fn new(max_events: usize) -> Self {
        ObservabilityStore {
            events: Vec::new(),
            seq: 0,
            max_events: max_events.max(1),
        }
    }

    pub fn emit(
        &mut self,
        kind: PoSeqEventKind,
        epoch: u64,
        slot: Option<u64>,
        node_id: Option<[u8; 32]>,
        batch_id: Option<[u8; 32]>,
        details: String,
    ) {
        let seq = self.seq;
        self.seq += 1;
        let record = EventRecord {
            seq,
            kind,
            epoch,
            slot,
            node_id,
            batch_id,
            details,
        };
        self.events.push(record);
        // Ring-buffer style pruning
        if self.events.len() > self.max_events {
            let overshoot = self.events.len() - self.max_events;
            self.events.drain(0..overshoot);
        }
    }

    pub fn events_for_epoch(&self, epoch: u64) -> Vec<&EventRecord> {
        self.events.iter().filter(|e| e.epoch == epoch).collect()
    }

    pub fn events_of_kind(&self, kind: &PoSeqEventKind) -> Vec<&EventRecord> {
        self.events.iter().filter(|e| &e.kind == kind).collect()
    }

    pub fn recent(&self, n: usize) -> Vec<&EventRecord> {
        let len = self.events.len();
        if n >= len {
            self.events.iter().collect()
        } else {
            self.events[len - n..].iter().collect()
        }
    }

    pub fn all(&self) -> &[EventRecord] {
        &self.events
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn batch(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[31] = n;
        id
    }

    #[test]
    fn test_emit_and_retrieve() {
        let mut store = ObservabilityStore::new(100);
        store.emit(
            PoSeqEventKind::BatchProposed,
            1,
            Some(5),
            Some(node(1)),
            Some(batch(1)),
            "proposed".into(),
        );
        assert_eq!(store.all().len(), 1);
    }

    #[test]
    fn test_events_for_epoch() {
        let mut store = ObservabilityStore::new(100);
        store.emit(PoSeqEventKind::EpochStarted, 1, None, None, None, "".into());
        store.emit(PoSeqEventKind::EpochStarted, 2, None, None, None, "".into());
        store.emit(PoSeqEventKind::BatchProposed, 1, None, None, None, "".into());
        assert_eq!(store.events_for_epoch(1).len(), 2);
        assert_eq!(store.events_for_epoch(2).len(), 1);
    }

    #[test]
    fn test_events_of_kind() {
        let mut store = ObservabilityStore::new(100);
        store.emit(PoSeqEventKind::BatchFinalized, 1, None, None, None, "".into());
        store.emit(PoSeqEventKind::BatchFinalized, 2, None, None, None, "".into());
        store.emit(PoSeqEventKind::NodeJoined, 1, None, None, None, "".into());
        assert_eq!(store.events_of_kind(&PoSeqEventKind::BatchFinalized).len(), 2);
        assert_eq!(store.events_of_kind(&PoSeqEventKind::NodeJoined).len(), 1);
    }

    #[test]
    fn test_recent() {
        let mut store = ObservabilityStore::new(100);
        for i in 0u64..10 {
            store.emit(PoSeqEventKind::BatchProposed, i, None, None, None, "".into());
        }
        let recent = store.recent(3);
        assert_eq!(recent.len(), 3);
        assert_eq!(recent[2].epoch, 9); // latest
    }

    #[test]
    fn test_ring_buffer_pruning() {
        let mut store = ObservabilityStore::new(5);
        for i in 0u64..10 {
            store.emit(PoSeqEventKind::EpochStarted, i, None, None, None, "".into());
        }
        assert_eq!(store.all().len(), 5);
        // Oldest should be epoch 5
        assert_eq!(store.all()[0].epoch, 5);
    }

    #[test]
    fn test_seq_monotonically_increases() {
        let mut store = ObservabilityStore::new(100);
        for _ in 0..5 {
            store.emit(PoSeqEventKind::NodeJoined, 1, None, None, None, "".into());
        }
        let seqs: Vec<u64> = store.all().iter().map(|e| e.seq).collect();
        for w in seqs.windows(2) {
            assert!(w[1] > w[0]);
        }
    }

    #[test]
    fn test_empty_store() {
        let store = ObservabilityStore::new(100);
        assert_eq!(store.all().len(), 0);
        assert_eq!(store.recent(5).len(), 0);
        assert_eq!(store.events_for_epoch(1).len(), 0);
    }

    #[test]
    fn test_all_event_kinds_compile() {
        // Ensure all kinds are constructable (compile check)
        let kinds = vec![
            PoSeqEventKind::EpochStarted,
            PoSeqEventKind::EpochCompleted,
            PoSeqEventKind::CommitteeRotated,
            PoSeqEventKind::BatchProposed,
            PoSeqEventKind::BatchAttested,
            PoSeqEventKind::BatchFinalized,
            PoSeqEventKind::BatchDelivered,
            PoSeqEventKind::BatchAcknowledged,
            PoSeqEventKind::BatchRejected,
            PoSeqEventKind::MisbehaviorDetected,
            PoSeqEventKind::PenaltyRecommended,
            PoSeqEventKind::NodeJoined,
            PoSeqEventKind::NodeSuspended,
            PoSeqEventKind::NodeBanned,
            PoSeqEventKind::CheckpointCreated,
            PoSeqEventKind::RecoveryStarted,
            PoSeqEventKind::RecoveryCompleted,
            PoSeqEventKind::BridgeRetry,
            PoSeqEventKind::BridgeFailed,
        ];
        let mut store = ObservabilityStore::new(100);
        for kind in kinds {
            store.emit(kind, 1, None, None, None, "test".into());
        }
        assert_eq!(store.all().len(), 19);
    }
}
