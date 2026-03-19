use std::collections::BTreeMap;

use super::events::{InactivityEvent, LivenessEvent, LivenessEventExport};

/// Tracks per-node liveness across epochs.
///
/// For each registered node, it tracks:
/// - `last_seen_epoch`: the last epoch the node was observed active.
/// - `consecutive_missed`: count of consecutive epochs with no activity.
/// - `current_epoch_events`: events recorded for the current epoch (reset at finalize).
///
/// An `InactivityEvent` is emitted when `consecutive_missed > inactivity_threshold`.
pub struct LivenessTracker {
    last_seen_epoch: BTreeMap<[u8; 32], u64>,
    consecutive_missed: BTreeMap<[u8; 32], u64>,
    current_epoch_events: BTreeMap<[u8; 32], LivenessEvent>,
    pub inactivity_threshold: u64,
    pub current_epoch: u64,
}

impl LivenessTracker {
    pub fn new(inactivity_threshold: u64) -> Self {
        LivenessTracker {
            last_seen_epoch: BTreeMap::new(),
            consecutive_missed: BTreeMap::new(),
            current_epoch_events: BTreeMap::new(),
            inactivity_threshold,
            current_epoch: 0,
        }
    }

    /// Record that a node was active in the current epoch at the given slot.
    ///
    /// Multiple calls for the same node in the same epoch are idempotent
    /// (last call wins for slot number and flags).
    pub fn record_active(
        &mut self,
        node_id: [u8; 32],
        slot: u64,
        was_proposer: bool,
        was_attestor: bool,
    ) {
        let event = LivenessEvent {
            node_id,
            epoch: self.current_epoch,
            last_seen_slot: slot,
            was_proposer,
            was_attestor,
        };
        self.current_epoch_events.insert(node_id, event);
    }

    /// Finalize the current epoch: detect inactive nodes from `registered_nodes`,
    /// advance the epoch counter, and return the full `LivenessEventExport`.
    ///
    /// Called once per epoch boundary, after all `record_active` calls are done.
    pub fn finalize_epoch(
        &mut self,
        epoch: u64,
        registered_nodes: &[[u8; 32]],
    ) -> LivenessEventExport {
        let active_events: Vec<LivenessEvent> =
            self.current_epoch_events.values().cloned().collect();

        let mut inactivity_events = Vec::new();
        for node_id in registered_nodes {
            if self.current_epoch_events.contains_key(node_id) {
                // Node was active — update last_seen, reset consecutive_missed.
                self.last_seen_epoch.insert(*node_id, epoch);
                self.consecutive_missed.insert(*node_id, 0);
            } else {
                // Node was absent this epoch — increment consecutive missed.
                let missed = self.consecutive_missed.entry(*node_id).or_insert(0);
                *missed += 1;
                if *missed > self.inactivity_threshold {
                    let last_active = self
                        .last_seen_epoch
                        .get(node_id)
                        .copied()
                        .unwrap_or(0);
                    inactivity_events.push(InactivityEvent {
                        node_id: *node_id,
                        detected_at_epoch: epoch,
                        last_active_epoch: last_active,
                        missed_epochs: *missed,
                    });
                }
            }
        }

        let export = LivenessEventExport {
            epoch,
            active_events,
            inactivity_events,
        };

        // Reset for next epoch.
        self.current_epoch_events.clear();
        self.current_epoch = epoch + 1;

        export
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn id(b: u8) -> [u8; 32] {
        let mut arr = [0u8; 32];
        arr[0] = b;
        arr
    }

    #[test]
    fn test_active_node_recorded() {
        let mut tracker = LivenessTracker::new(3);
        tracker.record_active(id(1), 10, true, true);
        let export = tracker.finalize_epoch(1, &[id(1)]);
        assert_eq!(export.active_events.len(), 1);
        assert_eq!(export.active_events[0].last_seen_slot, 10);
        assert!(export.active_events[0].was_proposer);
        assert_eq!(export.inactivity_events.len(), 0);
    }

    #[test]
    fn test_inactivity_threshold_not_crossed() {
        let mut tracker = LivenessTracker::new(3);
        // 3 consecutive missed epochs = threshold exactly → no event yet (threshold means >)
        tracker.finalize_epoch(1, &[id(1)]);
        tracker.finalize_epoch(2, &[id(1)]);
        let export = tracker.finalize_epoch(3, &[id(1)]);
        assert_eq!(export.inactivity_events.len(), 0);
    }

    #[test]
    fn test_inactivity_threshold_crossed() {
        let mut tracker = LivenessTracker::new(3);
        // 4 consecutive missed epochs > threshold of 3 → emit event
        tracker.finalize_epoch(1, &[id(1)]);
        tracker.finalize_epoch(2, &[id(1)]);
        tracker.finalize_epoch(3, &[id(1)]);
        let export = tracker.finalize_epoch(4, &[id(1)]);
        // After epoch 4, missed = 4 > 3
        assert!(!export.inactivity_events.is_empty());
        assert_eq!(export.inactivity_events[0].node_id, id(1));
        assert_eq!(export.inactivity_events[0].detected_at_epoch, 4);
    }

    #[test]
    fn test_activity_resets_missed_count() {
        let mut tracker = LivenessTracker::new(2);
        // Miss 2 epochs
        tracker.finalize_epoch(1, &[id(1)]);
        tracker.finalize_epoch(2, &[id(1)]);
        // Now active
        tracker.record_active(id(1), 5, false, true);
        tracker.finalize_epoch(3, &[id(1)]);
        // Miss again — only 1 missed, below threshold
        let export = tracker.finalize_epoch(4, &[id(1)]);
        assert_eq!(export.inactivity_events.len(), 0);
    }

    #[test]
    fn test_multiple_nodes() {
        let mut tracker = LivenessTracker::new(2);
        let nodes = [id(1), id(2), id(3)];
        // node 1 and 3 active, node 2 absent
        tracker.record_active(id(1), 1, true, false);
        tracker.record_active(id(3), 2, false, true);
        tracker.finalize_epoch(1, &nodes);
        // node 2 and 3 absent again
        tracker.record_active(id(1), 3, false, false);
        tracker.finalize_epoch(2, &nodes);
        // node 2 absent a third time (missed = 3 > threshold 2 → inactivity event)
        tracker.record_active(id(1), 5, false, false);
        let export = tracker.finalize_epoch(3, &nodes);
        assert_eq!(
            export
                .inactivity_events
                .iter()
                .filter(|e| e.node_id == id(2))
                .count(),
            1
        );
        // node 3 missed 2 epochs = threshold, not yet exceeding → no event
        assert!(export
            .inactivity_events
            .iter()
            .all(|e| e.node_id != id(3)));
    }
}
