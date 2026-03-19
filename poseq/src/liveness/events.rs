use serde::{Deserialize, Serialize};

/// Records that a node was observed active in a slot within an epoch.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LivenessEvent {
    pub node_id: [u8; 32],
    pub epoch: u64,
    pub last_seen_slot: u64,
    pub was_proposer: bool,
    pub was_attestor: bool,
}

/// Emitted when a node's consecutive missed epoch count crosses the inactivity threshold.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InactivityEvent {
    pub node_id: [u8; 32],
    pub detected_at_epoch: u64,
    pub last_active_epoch: u64,
    /// Integer count of missed epochs — no floating point.
    pub missed_epochs: u64,
}

/// The full liveness export for one epoch, sent in the ExportBatch.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct LivenessEventExport {
    pub epoch: u64,
    pub active_events: Vec<LivenessEvent>,
    pub inactivity_events: Vec<InactivityEvent>,
}
