use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};

/// Committee membership at a specific epoch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EpochCommitteeSnapshot {
    pub epoch: u64,
    /// node_ids of committee members in sorted order.
    pub members: BTreeSet<[u8; 32]>,
    /// SHA256 over sorted member ids ‖ epoch bytes — commitment to this snapshot.
    pub committee_hash: [u8; 32],
}

impl EpochCommitteeSnapshot {
    pub fn new(epoch: u64, members: BTreeSet<[u8; 32]>) -> Self {
        let committee_hash = Self::compute_hash(epoch, &members);
        EpochCommitteeSnapshot { epoch, members, committee_hash }
    }

    pub fn compute_hash(epoch: u64, members: &BTreeSet<[u8; 32]>) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&epoch.to_be_bytes());
        // BTreeSet iteration is sorted → deterministic
        for m in members {
            hasher.update(m);
        }
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    pub fn size(&self) -> usize {
        self.members.len()
    }

    pub fn contains(&self, node_id: &[u8; 32]) -> bool {
        self.members.contains(node_id)
    }

    /// Returns members as sorted Vec.
    pub fn sorted_members(&self) -> Vec<[u8; 32]> {
        self.members.iter().cloned().collect()
    }
}

/// Stores epoch → EpochCommitteeSnapshot mappings.
pub struct CommitteeRotationStore {
    snapshots: BTreeMap<u64, EpochCommitteeSnapshot>,
}

impl CommitteeRotationStore {
    pub fn new() -> Self {
        CommitteeRotationStore { snapshots: BTreeMap::new() }
    }

    /// Store a snapshot for an epoch. Overwrites if epoch already exists.
    pub fn insert(&mut self, snapshot: EpochCommitteeSnapshot) {
        self.snapshots.insert(snapshot.epoch, snapshot);
    }

    /// Retrieve snapshot for a given epoch.
    pub fn get(&self, epoch: u64) -> Option<&EpochCommitteeSnapshot> {
        self.snapshots.get(&epoch)
    }

    /// Get the latest epoch's snapshot.
    pub fn latest(&self) -> Option<&EpochCommitteeSnapshot> {
        self.snapshots.values().next_back()
    }

    /// Number of stored epochs.
    pub fn len(&self) -> usize {
        self.snapshots.len()
    }

    pub fn is_empty(&self) -> bool {
        self.snapshots.is_empty()
    }

    /// All stored epochs in ascending order.
    pub fn epochs(&self) -> Vec<u64> {
        self.snapshots.keys().cloned().collect()
    }
}

impl Default for CommitteeRotationStore {
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

    fn make_snapshot(epoch: u64, node_bytes: &[u8]) -> EpochCommitteeSnapshot {
        let members: BTreeSet<[u8; 32]> = node_bytes.iter().map(|&b| make_id(b)).collect();
        EpochCommitteeSnapshot::new(epoch, members)
    }

    #[test]
    fn test_snapshot_hash_determinism() {
        let s1 = make_snapshot(1, &[1, 2, 3]);
        let s2 = make_snapshot(1, &[1, 2, 3]);
        assert_eq!(s1.committee_hash, s2.committee_hash);
    }

    #[test]
    fn test_snapshot_hash_differs_with_different_members() {
        let s1 = make_snapshot(1, &[1, 2, 3]);
        let s2 = make_snapshot(1, &[1, 2, 4]);
        assert_ne!(s1.committee_hash, s2.committee_hash);
    }

    #[test]
    fn test_snapshot_hash_differs_with_different_epoch() {
        let s1 = make_snapshot(1, &[1, 2, 3]);
        let s2 = make_snapshot(2, &[1, 2, 3]);
        assert_ne!(s1.committee_hash, s2.committee_hash);
    }

    #[test]
    fn test_snapshot_contains() {
        let s = make_snapshot(1, &[1, 2, 3]);
        assert!(s.contains(&make_id(1)));
        assert!(!s.contains(&make_id(99)));
    }

    #[test]
    fn test_snapshot_sorted_members_order() {
        let s = make_snapshot(1, &[3, 1, 2]);
        let members = s.sorted_members();
        // BTreeSet guarantees sorted order
        assert_eq!(members[0], make_id(1));
        assert_eq!(members[1], make_id(2));
        assert_eq!(members[2], make_id(3));
    }

    #[test]
    fn test_store_insert_and_get() {
        let mut store = CommitteeRotationStore::new();
        let s = make_snapshot(5, &[1, 2, 3]);
        store.insert(s.clone());
        assert_eq!(store.get(5).unwrap().epoch, 5);
    }

    #[test]
    fn test_store_latest() {
        let mut store = CommitteeRotationStore::new();
        store.insert(make_snapshot(1, &[1]));
        store.insert(make_snapshot(3, &[2]));
        store.insert(make_snapshot(2, &[3]));
        assert_eq!(store.latest().unwrap().epoch, 3);
    }

    #[test]
    fn test_store_len() {
        let mut store = CommitteeRotationStore::new();
        assert_eq!(store.len(), 0);
        store.insert(make_snapshot(1, &[1]));
        store.insert(make_snapshot(2, &[2]));
        assert_eq!(store.len(), 2);
    }

    #[test]
    fn test_store_epochs_ascending() {
        let mut store = CommitteeRotationStore::new();
        store.insert(make_snapshot(3, &[3]));
        store.insert(make_snapshot(1, &[1]));
        store.insert(make_snapshot(2, &[2]));
        let epochs = store.epochs();
        assert_eq!(epochs, vec![1, 2, 3]);
    }

    #[test]
    fn test_store_get_missing_returns_none() {
        let store = CommitteeRotationStore::new();
        assert!(store.get(99).is_none());
    }
}
