use std::collections::BTreeMap;
use crate::fairness::policy::FairnessClass;
use crate::anti_mev::policy::ProtectedFlowPolicy;

/// A record tracking a submission's protected-flow status.
#[derive(Debug, Clone)]
pub struct ProtectedFlowRecord {
    pub submission_id: [u8; 32],
    pub fairness_class: FairnessClass,
    pub registered_at_slot: u64,
    /// The latest slot by which this submission must be included.
    pub must_include_by_slot: u64,
    pub included_at_slot: Option<u64>,
    pub included_in_batch: Option<[u8; 32]>,
}

/// Manages protected-flow registrations and deadline tracking.
#[derive(Debug, Clone)]
pub struct ProtectedFlowHandler {
    pub records: BTreeMap<[u8; 32], ProtectedFlowRecord>,
    pub policy: ProtectedFlowPolicy,
}

impl ProtectedFlowHandler {
    pub fn new(policy: ProtectedFlowPolicy) -> Self {
        ProtectedFlowHandler {
            records: BTreeMap::new(),
            policy,
        }
    }

    /// Register a submission as a protected flow.
    /// If already registered, this is a no-op.
    pub fn register(&mut self, id: [u8; 32], class: FairnessClass, current_slot: u64) {
        if !self.policy.enabled || !self.policy.protected_classes.contains(&class) {
            return;
        }
        self.records.entry(id).or_insert_with(|| ProtectedFlowRecord {
            submission_id: id,
            fairness_class: class,
            registered_at_slot: current_slot,
            must_include_by_slot: current_slot + self.policy.max_delay_slots,
            included_at_slot: None,
            included_in_batch: None,
        });
    }

    /// Mark a protected flow as included in a batch.
    pub fn mark_included(&mut self, id: [u8; 32], slot: u64, batch_id: [u8; 32]) {
        if let Some(record) = self.records.get_mut(&id) {
            record.included_at_slot = Some(slot);
            record.included_in_batch = Some(batch_id);
        }
    }

    /// Return all records whose inclusion deadline has passed but haven't been included yet.
    pub fn get_overdue(&self, current_slot: u64) -> Vec<&ProtectedFlowRecord> {
        let mut overdue: Vec<&ProtectedFlowRecord> = self
            .records
            .values()
            .filter(|r| r.included_at_slot.is_none() && current_slot > r.must_include_by_slot)
            .collect();
        // Sort by deadline ascending for determinism
        overdue.sort_by_key(|r| (r.must_include_by_slot, r.submission_id));
        overdue
    }

    /// Check if a submission is registered as a protected flow.
    pub fn is_registered(&self, id: &[u8; 32]) -> bool {
        self.records.contains_key(id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::anti_mev::policy::ProtectedFlowPolicy;
    use crate::fairness::policy::FairnessClass;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_handler() -> ProtectedFlowHandler {
        ProtectedFlowHandler::new(ProtectedFlowPolicy::default_policy())
    }

    #[test]
    fn test_register_creates_record() {
        let mut handler = make_handler();
        let id = make_id(1);
        handler.register(id, FairnessClass::SafetyCritical, 10);
        assert!(handler.is_registered(&id));
        let record = &handler.records[&id];
        assert_eq!(record.registered_at_slot, 10);
        // must_include_by_slot = 10 + max_delay_slots (5 by default)
        assert_eq!(record.must_include_by_slot, 15);
    }

    #[test]
    fn test_register_non_protected_class_is_ignored() {
        let mut handler = make_handler();
        let id = make_id(2);
        // Normal is not in protected_classes
        handler.register(id, FairnessClass::Normal, 10);
        assert!(!handler.is_registered(&id));
    }

    #[test]
    fn test_get_overdue_at_deadline() {
        let mut handler = make_handler();
        let id = make_id(3);
        handler.register(id, FairnessClass::SafetyCritical, 10);
        // must_include_by_slot = 15
        // At slot 15: NOT yet overdue (current_slot > must_include_by_slot)
        let overdue_at_15 = handler.get_overdue(15);
        assert!(overdue_at_15.is_empty(), "At exactly deadline, not yet overdue");
        // At slot 16: overdue
        let overdue_at_16 = handler.get_overdue(16);
        assert_eq!(overdue_at_16.len(), 1);
        assert_eq!(overdue_at_16[0].submission_id, id);
    }

    #[test]
    fn test_mark_included_clears_overdue() {
        let mut handler = make_handler();
        let id = make_id(4);
        let batch_id = make_id(100);
        handler.register(id, FairnessClass::ProtectedUserFlow, 5);
        // Becomes overdue at slot 11
        assert_eq!(handler.get_overdue(11).len(), 1);

        // Mark included
        handler.mark_included(id, 8, batch_id);
        // Now no longer overdue
        let overdue = handler.get_overdue(11);
        assert!(overdue.is_empty(), "Included submissions should not be overdue");
        assert_eq!(handler.records[&id].included_at_slot, Some(8));
        assert_eq!(handler.records[&id].included_in_batch, Some(batch_id));
    }

    #[test]
    fn test_is_registered_returns_false_for_unknown() {
        let handler = make_handler();
        assert!(!handler.is_registered(&make_id(99)));
    }

    #[test]
    fn test_register_is_idempotent() {
        let mut handler = make_handler();
        let id = make_id(5);
        handler.register(id, FairnessClass::BridgeAdjacent, 10);
        handler.register(id, FairnessClass::BridgeAdjacent, 20); // re-register at later slot
        // First registration wins
        assert_eq!(handler.records[&id].registered_at_slot, 10);
    }

    #[test]
    fn test_multiple_overdue_sorted_by_deadline() {
        let mut handler = make_handler();
        let id_a = make_id(1);
        let id_b = make_id(2);
        handler.register(id_a, FairnessClass::SafetyCritical, 1); // deadline = 6
        handler.register(id_b, FairnessClass::GovernanceSensitive, 3); // deadline = 8

        let overdue = handler.get_overdue(10);
        assert_eq!(overdue.len(), 2);
        // id_a has earlier deadline → should appear first
        assert_eq!(overdue[0].submission_id, id_a);
        assert_eq!(overdue[1].submission_id, id_b);
    }
}
