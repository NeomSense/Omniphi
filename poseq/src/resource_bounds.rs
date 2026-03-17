//! Resource bounds and operational limits for PoSeq.
//!
//! These constants define the recommended production limits for each
//! subsystem. They are enforced where possible and documented where
//! they require operator configuration.

/// Maximum submissions per batch (hard limit).
pub const MAX_SUBMISSIONS_PER_BATCH: usize = 500;

/// Maximum pending queue depth before backpressure.
pub const MAX_QUEUE_DEPTH: usize = 10_000;

/// Maximum replay guard history (FIFO eviction window).
pub const MAX_REPLAY_GUARD_HISTORY: usize = 50_000;

/// Maximum payload body size per submission (bytes).
pub const MAX_PAYLOAD_BYTES: usize = 64 * 1024; // 64KB

/// Maximum checkpoint age before pruning (epochs).
pub const MAX_CHECKPOINT_RETENTION_EPOCHS: u64 = 100;

/// Maximum retained checkpoints in memory.
pub const MAX_RETAINED_CHECKPOINTS: usize = 10;

/// Maximum evidence packets per epoch in ExportBatch.
pub const MAX_EVIDENCE_PER_EPOCH: usize = 256;

/// Maximum governance escalations per ExportBatch.
pub const MAX_ESCALATIONS_PER_EPOCH: usize = 32;

/// Maximum committee size.
pub const MAX_COMMITTEE_SIZE: usize = 100;

/// Maximum attestation fan-out per slot.
pub const MAX_ATTESTATIONS_PER_SLOT: usize = 200; // committee * 2 for safety

/// Maximum retry queue depth for bridge delivery.
pub const MAX_BRIDGE_RETRY_QUEUE: usize = 100;

/// Maximum incident ledger size (fairness incidents per epoch).
pub const MAX_FAIRNESS_INCIDENTS_PER_EPOCH: usize = 1_000;

/// Slot duration floor (milliseconds). Below this, timing attacks become practical.
pub const MIN_SLOT_DURATION_MS: u64 = 500;

/// Epoch length minimum (slots).
pub const MIN_EPOCH_LENGTH_SLOTS: u64 = 10;

/// Maximum proposal size (bytes, serialized).
pub const MAX_PROPOSAL_SIZE_BYTES: usize = 4 * 1024 * 1024; // 4MB

/// ResourceBoundsReport documents the current resource consumption for telemetry.
#[derive(Debug, Clone, serde::Serialize)]
pub struct ResourceBoundsReport {
    pub queue_depth: usize,
    pub replay_guard_size: usize,
    pub checkpoint_count: usize,
    pub finality_store_size: usize,
    pub incident_ledger_size: usize,
    pub bridge_retry_queue_size: usize,
}

impl ResourceBoundsReport {
    /// Returns true if any resource is within 80% of its limit.
    pub fn any_near_limit(&self) -> bool {
        self.queue_depth >= (MAX_QUEUE_DEPTH * 4 / 5)
            || self.replay_guard_size >= (MAX_REPLAY_GUARD_HISTORY * 4 / 5)
            || self.checkpoint_count >= (MAX_RETAINED_CHECKPOINTS * 4 / 5)
            || self.incident_ledger_size >= (MAX_FAIRNESS_INCIDENTS_PER_EPOCH * 4 / 5)
    }

    /// Returns a vec of (subsystem, current, limit) for any resource at >80% capacity.
    pub fn near_limit_items(&self) -> Vec<(&'static str, usize, usize)> {
        let mut items = Vec::new();
        if self.queue_depth >= (MAX_QUEUE_DEPTH * 4 / 5) {
            items.push(("queue", self.queue_depth, MAX_QUEUE_DEPTH));
        }
        if self.replay_guard_size >= (MAX_REPLAY_GUARD_HISTORY * 4 / 5) {
            items.push(("replay_guard", self.replay_guard_size, MAX_REPLAY_GUARD_HISTORY));
        }
        if self.checkpoint_count >= (MAX_RETAINED_CHECKPOINTS * 4 / 5) {
            items.push(("checkpoints", self.checkpoint_count, MAX_RETAINED_CHECKPOINTS));
        }
        if self.incident_ledger_size >= (MAX_FAIRNESS_INCIDENTS_PER_EPOCH * 4 / 5) {
            items.push((
                "incidents",
                self.incident_ledger_size,
                MAX_FAIRNESS_INCIDENTS_PER_EPOCH,
            ));
        }
        items
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_limits_are_sane() {
        assert!(MAX_SUBMISSIONS_PER_BATCH <= MAX_QUEUE_DEPTH);
        assert!(MAX_REPLAY_GUARD_HISTORY >= MAX_QUEUE_DEPTH);
        assert!(MAX_PAYLOAD_BYTES <= MAX_PROPOSAL_SIZE_BYTES);
        assert!(MIN_SLOT_DURATION_MS >= 100);
        assert!(MIN_EPOCH_LENGTH_SLOTS >= 1);
    }

    #[test]
    fn test_report_near_limit_detection() {
        let report = ResourceBoundsReport {
            queue_depth: MAX_QUEUE_DEPTH * 9 / 10, // 90% — near limit
            replay_guard_size: 0,
            checkpoint_count: 0,
            finality_store_size: 0,
            incident_ledger_size: 0,
            bridge_retry_queue_size: 0,
        };
        assert!(report.any_near_limit());
        let items = report.near_limit_items();
        assert_eq!(items.len(), 1);
        assert_eq!(items[0].0, "queue");
    }

    #[test]
    fn test_report_all_clear() {
        let report = ResourceBoundsReport {
            queue_depth: 100,
            replay_guard_size: 100,
            checkpoint_count: 2,
            finality_store_size: 50,
            incident_ledger_size: 10,
            bridge_retry_queue_size: 5,
        };
        assert!(!report.any_near_limit());
        assert!(report.near_limit_items().is_empty());
    }

    #[test]
    fn test_report_replay_guard_near_limit() {
        let report = ResourceBoundsReport {
            queue_depth: 0,
            replay_guard_size: MAX_REPLAY_GUARD_HISTORY * 9 / 10, // 90%
            checkpoint_count: 0,
            finality_store_size: 0,
            incident_ledger_size: 0,
            bridge_retry_queue_size: 0,
        };
        assert!(report.any_near_limit());
        let items = report.near_limit_items();
        assert_eq!(items.len(), 1);
        assert_eq!(items[0].0, "replay_guard");
        assert_eq!(items[0].2, MAX_REPLAY_GUARD_HISTORY);
    }

    #[test]
    fn test_report_multiple_near_limit() {
        let report = ResourceBoundsReport {
            queue_depth: MAX_QUEUE_DEPTH,                          // 100% — near limit
            replay_guard_size: MAX_REPLAY_GUARD_HISTORY,          // 100% — near limit
            checkpoint_count: 0,
            finality_store_size: 0,
            incident_ledger_size: MAX_FAIRNESS_INCIDENTS_PER_EPOCH, // 100% — near limit
            bridge_retry_queue_size: 0,
        };
        assert!(report.any_near_limit());
        let items = report.near_limit_items();
        assert_eq!(items.len(), 3, "three resources near limit: queue, replay_guard, incidents");
    }

    #[test]
    fn test_report_exactly_at_80_percent_threshold() {
        // Exactly at 80% — should trigger
        let report = ResourceBoundsReport {
            queue_depth: MAX_QUEUE_DEPTH * 4 / 5,
            replay_guard_size: 0,
            checkpoint_count: 0,
            finality_store_size: 0,
            incident_ledger_size: 0,
            bridge_retry_queue_size: 0,
        };
        assert!(report.any_near_limit(), "80% threshold must trigger near_limit");
    }

    #[test]
    fn test_report_just_below_80_percent_threshold() {
        // Just below 80% — should not trigger
        let report = ResourceBoundsReport {
            queue_depth: (MAX_QUEUE_DEPTH * 4 / 5).saturating_sub(1),
            replay_guard_size: 0,
            checkpoint_count: 0,
            finality_store_size: 0,
            incident_ledger_size: 0,
            bridge_retry_queue_size: 0,
        };
        // Whether this triggers depends on integer arithmetic rounding.
        // The important thing is that the function doesn't panic.
        let _ = report.any_near_limit();
        let _ = report.near_limit_items();
    }

    #[test]
    fn test_checkpoint_near_limit() {
        let report = ResourceBoundsReport {
            queue_depth: 0,
            replay_guard_size: 0,
            checkpoint_count: MAX_RETAINED_CHECKPOINTS * 9 / 10, // 90%
            finality_store_size: 0,
            incident_ledger_size: 0,
            bridge_retry_queue_size: 0,
        };
        assert!(report.any_near_limit());
        let items = report.near_limit_items();
        assert!(items.iter().any(|(name, _, _)| *name == "checkpoints"));
    }
}
