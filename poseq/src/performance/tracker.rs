use std::collections::BTreeMap;

use super::record::NodePerformanceRecord;

#[derive(Debug, Default, Clone)]
struct NodeCounters {
    proposals: u64,
    attestations: u64,
    missed_attestations: u64,
    fault_events: u64,
    eligible_slots: u64,
}

/// Accumulates per-node performance counters during an epoch.
///
/// Call `record_*` methods as events occur, then call `finalize_epoch`
/// once at the epoch boundary to drain the counters and produce records.
pub struct PerformanceTracker {
    counters: BTreeMap<[u8; 32], NodeCounters>,
}

impl PerformanceTracker {
    pub fn new() -> Self {
        PerformanceTracker {
            counters: BTreeMap::new(),
        }
    }

    pub fn record_proposal(&mut self, node_id: [u8; 32]) {
        self.counters.entry(node_id).or_default().proposals += 1;
    }

    pub fn record_attestation(&mut self, node_id: [u8; 32]) {
        self.counters.entry(node_id).or_default().attestations += 1;
    }

    pub fn record_missed_attestation(&mut self, node_id: [u8; 32]) {
        self.counters
            .entry(node_id)
            .or_default()
            .missed_attestations += 1;
    }

    pub fn record_fault(&mut self, node_id: [u8; 32]) {
        self.counters.entry(node_id).or_default().fault_events += 1;
    }

    /// Increment the eligible-slot counter for a node.
    ///
    /// Called once per slot in which a node was eligible to attest.
    /// Used as the denominator for `participation_rate_bps`.
    pub fn record_eligible_slot(&mut self, node_id: [u8; 32]) {
        self.counters.entry(node_id).or_default().eligible_slots += 1;
    }

    /// Compute `NodePerformanceRecord` for every tracked node, reset counters,
    /// and return the records for this epoch.
    pub fn finalize_epoch(&mut self, epoch: u64) -> Vec<NodePerformanceRecord> {
        let records = self
            .counters
            .iter()
            .map(|(node_id, c)| NodePerformanceRecord {
                node_id: *node_id,
                epoch,
                proposals_count: c.proposals,
                attestations_count: c.attestations,
                missed_attestations: c.missed_attestations,
                fault_events: c.fault_events,
                participation_rate_bps: NodePerformanceRecord::compute_participation_rate(
                    c.attestations,
                    c.eligible_slots,
                ),
            })
            .collect();

        self.counters.clear();
        records
    }
}

impl Default for PerformanceTracker {
    fn default() -> Self {
        Self::new()
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
    fn test_record_and_finalize() {
        let mut tracker = PerformanceTracker::new();
        tracker.record_proposal(id(1));
        tracker.record_attestation(id(1));
        tracker.record_attestation(id(1));
        tracker.record_missed_attestation(id(1));
        tracker.record_fault(id(1));
        tracker.record_eligible_slot(id(1));
        tracker.record_eligible_slot(id(1));
        tracker.record_eligible_slot(id(1));

        let records = tracker.finalize_epoch(5);
        assert_eq!(records.len(), 1);
        let r = &records[0];
        assert_eq!(r.node_id, id(1));
        assert_eq!(r.epoch, 5);
        assert_eq!(r.proposals_count, 1);
        assert_eq!(r.attestations_count, 2);
        assert_eq!(r.missed_attestations, 1);
        assert_eq!(r.fault_events, 1);
        // 2 attestations / 3 eligible slots = 6666 bps
        assert_eq!(r.participation_rate_bps, 6666);
    }

    #[test]
    fn test_rate_bps_zero_eligible_slots() {
        assert_eq!(
            NodePerformanceRecord::compute_participation_rate(100, 0),
            0
        );
    }

    #[test]
    fn test_rate_bps_full_participation() {
        assert_eq!(
            NodePerformanceRecord::compute_participation_rate(10, 10),
            10_000
        );
    }

    #[test]
    fn test_counters_reset_after_finalize() {
        let mut tracker = PerformanceTracker::new();
        tracker.record_proposal(id(2));
        tracker.finalize_epoch(1);
        // After finalize, counters are cleared
        let records = tracker.finalize_epoch(2);
        assert!(records.is_empty());
    }

    #[test]
    fn test_multiple_nodes() {
        let mut tracker = PerformanceTracker::new();
        tracker.record_proposal(id(1));
        tracker.record_attestation(id(2));
        tracker.record_eligible_slot(id(2));

        let records = tracker.finalize_epoch(3);
        assert_eq!(records.len(), 2);
        let node2 = records.iter().find(|r| r.node_id == id(2)).unwrap();
        assert_eq!(node2.participation_rate_bps, 10_000);
    }
}
