use std::collections::BTreeMap;
use crate::errors::Phase4Error;

/// A jail record for a node.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JailRecord {
    pub node_id: [u8; 32],
    pub jailed_at_epoch: u64,
    pub eligible_unjail_epoch: u64,
}

impl JailRecord {
    pub fn new(node_id: [u8; 32], jailed_at_epoch: u64, cooldown_epochs: u64) -> Self {
        JailRecord {
            node_id,
            jailed_at_epoch,
            eligible_unjail_epoch: jailed_at_epoch.saturating_add(cooldown_epochs),
        }
    }

    /// Check if this node can be unjailed at the given epoch.
    pub fn can_unjail(&self, current_epoch: u64) -> bool {
        current_epoch >= self.eligible_unjail_epoch
    }
}

/// Manages jail records for nodes.
pub struct JailStore {
    records: BTreeMap<[u8; 32], JailRecord>,
}

impl JailStore {
    pub fn new() -> Self {
        JailStore { records: BTreeMap::new() }
    }

    /// Jail a node. Returns error if already jailed.
    pub fn jail(
        &mut self,
        node_id: [u8; 32],
        epoch: u64,
        cooldown_epochs: u64,
    ) -> Result<&JailRecord, Phase4Error> {
        if self.records.contains_key(&node_id) {
            return Err(Phase4Error::AlreadyJailed(node_id));
        }
        let record = JailRecord::new(node_id, epoch, cooldown_epochs);
        self.records.insert(node_id, record);
        Ok(self.records.get(&node_id).unwrap())
    }

    /// Unjail a node. Returns error if not jailed or cooldown not elapsed.
    pub fn unjail(&mut self, node_id: [u8; 32], current_epoch: u64) -> Result<(), Phase4Error> {
        let record = self.records.get(&node_id)
            .ok_or(Phase4Error::NotJailed(node_id))?;

        if !record.can_unjail(current_epoch) {
            return Err(Phase4Error::UnjailCooldownActive {
                eligible_at: record.eligible_unjail_epoch,
                current: current_epoch,
            });
        }

        self.records.remove(&node_id);
        Ok(())
    }

    /// Check if a node is currently jailed.
    pub fn is_jailed(&self, node_id: &[u8; 32]) -> bool {
        self.records.contains_key(node_id)
    }

    /// Get jail record for a node.
    pub fn get_record(&self, node_id: &[u8; 32]) -> Option<&JailRecord> {
        self.records.get(node_id)
    }

    /// All currently jailed node_ids in sorted order.
    pub fn jailed_nodes(&self) -> Vec<[u8; 32]> {
        self.records.keys().cloned().collect()
    }

    /// Count of jailed nodes.
    pub fn jailed_count(&self) -> usize {
        self.records.len()
    }
}

impl Default for JailStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_jail_record_cooldown() {
        let record = JailRecord::new(make_id(1), 10, 5);
        assert_eq!(record.eligible_unjail_epoch, 15);
        assert!(!record.can_unjail(14));
        assert!(record.can_unjail(15));
        assert!(record.can_unjail(100));
    }

    #[test]
    fn test_jail_record_zero_cooldown() {
        let record = JailRecord::new(make_id(1), 10, 0);
        assert_eq!(record.eligible_unjail_epoch, 10);
        assert!(record.can_unjail(10));
    }

    #[test]
    fn test_jail_and_is_jailed() {
        let mut store = JailStore::new();
        let node = make_id(1);
        store.jail(node, 5, 10).unwrap();
        assert!(store.is_jailed(&node));
    }

    #[test]
    fn test_already_jailed_returns_err() {
        let mut store = JailStore::new();
        let node = make_id(1);
        store.jail(node, 5, 10).unwrap();
        let err = store.jail(node, 6, 10);
        assert!(matches!(err, Err(Phase4Error::AlreadyJailed(_))));
    }

    #[test]
    fn test_unjail_not_jailed_returns_err() {
        let mut store = JailStore::new();
        let err = store.unjail(make_id(99), 100);
        assert!(matches!(err, Err(Phase4Error::NotJailed(_))));
    }

    #[test]
    fn test_unjail_cooldown_active_returns_err() {
        let mut store = JailStore::new();
        let node = make_id(1);
        store.jail(node, 5, 10).unwrap(); // eligible at epoch 15
        let err = store.unjail(node, 14);
        assert!(matches!(err, Err(Phase4Error::UnjailCooldownActive { eligible_at: 15, current: 14 })));
    }

    #[test]
    fn test_unjail_succeeds_after_cooldown() {
        let mut store = JailStore::new();
        let node = make_id(1);
        store.jail(node, 5, 10).unwrap();
        store.unjail(node, 15).unwrap();
        assert!(!store.is_jailed(&node));
    }

    #[test]
    fn test_jailed_nodes_sorted() {
        let mut store = JailStore::new();
        store.jail(make_id(3), 1, 10).unwrap();
        store.jail(make_id(1), 1, 10).unwrap();
        store.jail(make_id(2), 1, 10).unwrap();
        let jailed = store.jailed_nodes();
        assert_eq!(jailed[0], make_id(1));
        assert_eq!(jailed[1], make_id(2));
        assert_eq!(jailed[2], make_id(3));
    }

    #[test]
    fn test_jailed_count() {
        let mut store = JailStore::new();
        assert_eq!(store.jailed_count(), 0);
        store.jail(make_id(1), 1, 10).unwrap();
        store.jail(make_id(2), 1, 10).unwrap();
        assert_eq!(store.jailed_count(), 2);
    }

    #[test]
    fn test_get_record() {
        let mut store = JailStore::new();
        let node = make_id(5);
        store.jail(node, 7, 3).unwrap();
        let rec = store.get_record(&node).unwrap();
        assert_eq!(rec.jailed_at_epoch, 7);
        assert_eq!(rec.eligible_unjail_epoch, 10);
    }

    #[test]
    fn test_jail_saturation_overflow_safe() {
        let record = JailRecord::new(make_id(1), u64::MAX, 100);
        // saturating_add: MAX + 100 = MAX
        assert_eq!(record.eligible_unjail_epoch, u64::MAX);
    }
}
