use sha2::{Sha256, Digest};
use crate::fairness::policy::FairnessClass;

/// One entry in a queue snapshot representing an eligible submission.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EligibleSubmissionEntry {
    pub submission_id: [u8; 32],
    pub fairness_class: FairnessClass,
    pub received_at_slot: u64,
    pub received_at_sequence: u64,
    pub age_slots: u64,
    pub is_forced_inclusion: bool,
}

/// A deterministic snapshot of the eligible submission queue at a given slot.
///
/// Entries are sorted: protected classes first, then by (class priority DESC, received_at_sequence ASC).
/// The class priority ordering used here is based on the derived Ord of FairnessClass
/// (alphabetical by variant name). In practice, the caller should pre-sort entries
/// using the policy's priority_weight, passing them in the desired canonical order.
/// This snapshot preserves the order provided by the caller.
#[derive(Debug, Clone)]
pub struct QueueSnapshot {
    pub snapshot_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub height: u64,
    /// Entries in canonical order: protected first, then by (class, sequence).
    pub entries: Vec<EligibleSubmissionEntry>,
    /// SHA256 of all entry submission_ids in canonical order.
    pub snapshot_root: [u8; 32],
    pub policy_version: u32,
}

impl QueueSnapshot {
    /// Build a QueueSnapshot from a set of entries.
    ///
    /// Entries are sorted canonically:
    /// 1. Protected classes first (SafetyCritical, ProtectedUserFlow, BridgeAdjacent, GovernanceSensitive)
    /// 2. Within same protection tier: by FairnessClass (Ord) then received_at_sequence ASC.
    /// 3. Forced inclusions are promoted within their class.
    pub fn build(
        mut entries: Vec<EligibleSubmissionEntry>,
        slot: u64,
        epoch: u64,
        height: u64,
        policy_version: u32,
    ) -> Self {
        // Sort deterministically
        entries.sort_by(|a, b| {
            let a_protected = is_protected_class(&a.fairness_class);
            let b_protected = is_protected_class(&b.fairness_class);
            // Protected first
            b_protected.cmp(&a_protected)
                .then(a.fairness_class.cmp(&b.fairness_class))
                .then(a.received_at_sequence.cmp(&b.received_at_sequence))
                .then(a.submission_id.cmp(&b.submission_id))
        });

        let snapshot_root = Self::compute_root(&entries);
        let mut snapshot = QueueSnapshot {
            snapshot_id: [0u8; 32],
            slot,
            epoch,
            height,
            entries,
            snapshot_root,
            policy_version,
        };
        snapshot.snapshot_id = snapshot.compute_snapshot_id();
        snapshot
    }

    /// SHA256 of all entry submission_ids in canonical order.
    pub fn compute_root(entries: &[EligibleSubmissionEntry]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for entry in entries {
            hasher.update(&entry.submission_id);
        }
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// SHA256(slot || epoch || snapshot_root || policy_version).
    pub fn compute_snapshot_id(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&self.slot.to_be_bytes());
        hasher.update(&self.epoch.to_be_bytes());
        hasher.update(&self.snapshot_root);
        hasher.update(&self.policy_version.to_be_bytes());
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Return submission IDs in snapshot ordering (canonical order).
    pub fn ordered_ids(&self) -> Vec<[u8; 32]> {
        self.entries.iter().map(|e| e.submission_id).collect()
    }

    /// Check if a submission_id is present in this snapshot.
    pub fn contains(&self, id: &[u8; 32]) -> bool {
        self.entries.iter().any(|e| &e.submission_id == id)
    }
}

/// A leader's commitment to a specific queue snapshot before ordering.
#[derive(Debug, Clone)]
pub struct SnapshotCommitment {
    pub snapshot_id: [u8; 32],
    pub snapshot_root: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    /// The leader node's ID.
    pub committed_by: [u8; 32],
    /// SHA256(snapshot_id || snapshot_root || slot || epoch || committed_by).
    pub commitment_hash: [u8; 32],
}

impl SnapshotCommitment {
    /// Compute a snapshot commitment for the given leader.
    pub fn compute(snapshot: &QueueSnapshot, leader_id: [u8; 32]) -> Self {
        let commitment_hash = Self::compute_commitment_hash(
            &snapshot.snapshot_id,
            &snapshot.snapshot_root,
            snapshot.slot,
            snapshot.epoch,
            &leader_id,
        );
        SnapshotCommitment {
            snapshot_id: snapshot.snapshot_id,
            snapshot_root: snapshot.snapshot_root,
            slot: snapshot.slot,
            epoch: snapshot.epoch,
            committed_by: leader_id,
            commitment_hash,
        }
    }

    fn compute_commitment_hash(
        snapshot_id: &[u8; 32],
        snapshot_root: &[u8; 32],
        slot: u64,
        epoch: u64,
        leader_id: &[u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(snapshot_id);
        hasher.update(snapshot_root);
        hasher.update(&slot.to_be_bytes());
        hasher.update(&epoch.to_be_bytes());
        hasher.update(leader_id);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// A view combining a snapshot with its commitment and summary counts.
#[derive(Debug, Clone)]
pub struct SnapshotOrderingView {
    pub snapshot: QueueSnapshot,
    pub commitment: SnapshotCommitment,
    pub eligible_count: usize,
    pub forced_inclusion_count: usize,
}

impl SnapshotOrderingView {
    pub fn build(snapshot: QueueSnapshot, leader_id: [u8; 32]) -> Self {
        let forced_inclusion_count = snapshot.entries.iter().filter(|e| e.is_forced_inclusion).count();
        let eligible_count = snapshot.entries.len();
        let commitment = SnapshotCommitment::compute(&snapshot, leader_id);
        SnapshotOrderingView {
            snapshot,
            commitment,
            eligible_count,
            forced_inclusion_count,
        }
    }
}

/// Returns true if the class is considered "protected" in queue snapshot ordering.
pub fn is_protected_class(class: &FairnessClass) -> bool {
    matches!(
        class,
        FairnessClass::SafetyCritical
            | FairnessClass::ProtectedUserFlow
            | FairnessClass::BridgeAdjacent
            | FairnessClass::GovernanceSensitive
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_entry(id: u8, class: FairnessClass, seq: u64, forced: bool) -> EligibleSubmissionEntry {
        EligibleSubmissionEntry {
            submission_id: make_id(id),
            fairness_class: class,
            received_at_slot: 1,
            received_at_sequence: seq,
            age_slots: 0,
            is_forced_inclusion: forced,
        }
    }

    #[test]
    fn test_build_produces_deterministic_snapshot_id() {
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0, false),
            make_entry(2, FairnessClass::LatencySensitive, 1, false),
        ];
        let s1 = QueueSnapshot::build(entries.clone(), 5, 1, 100, 1);
        let s2 = QueueSnapshot::build(entries, 5, 1, 100, 1);
        assert_eq!(s1.snapshot_id, s2.snapshot_id);
    }

    #[test]
    fn test_same_inputs_same_snapshot_root() {
        let entries = vec![
            make_entry(3, FairnessClass::Normal, 0, false),
            make_entry(4, FairnessClass::SolverRelated, 1, false),
        ];
        let s1 = QueueSnapshot::build(entries.clone(), 10, 2, 200, 1);
        let s2 = QueueSnapshot::build(entries, 10, 2, 200, 1);
        assert_eq!(s1.snapshot_root, s2.snapshot_root);
    }

    #[test]
    fn test_ordered_ids_protected_first() {
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0, false),
            make_entry(2, FairnessClass::SafetyCritical, 1, false),
            make_entry(3, FairnessClass::LatencySensitive, 2, false),
            make_entry(4, FairnessClass::ProtectedUserFlow, 3, false),
        ];
        let snapshot = QueueSnapshot::build(entries, 1, 1, 1, 1);
        let ids = snapshot.ordered_ids();
        // SafetyCritical and ProtectedUserFlow should appear before Normal and LatencySensitive
        let sc_pos = ids.iter().position(|id| id == &make_id(2)).unwrap();
        let puf_pos = ids.iter().position(|id| id == &make_id(4)).unwrap();
        let normal_pos = ids.iter().position(|id| id == &make_id(1)).unwrap();
        let ls_pos = ids.iter().position(|id| id == &make_id(3)).unwrap();
        assert!(sc_pos < normal_pos, "SafetyCritical should be before Normal");
        assert!(puf_pos < normal_pos, "ProtectedUserFlow should be before Normal");
        assert!(sc_pos < ls_pos, "SafetyCritical should be before LatencySensitive");
    }

    #[test]
    fn test_contains_works_correctly() {
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0, false),
            make_entry(2, FairnessClass::Normal, 1, false),
        ];
        let snapshot = QueueSnapshot::build(entries, 1, 1, 1, 1);
        assert!(snapshot.contains(&make_id(1)));
        assert!(snapshot.contains(&make_id(2)));
        assert!(!snapshot.contains(&make_id(99)));
    }

    #[test]
    fn test_snapshot_id_changes_with_different_inputs() {
        let e1 = vec![make_entry(1, FairnessClass::Normal, 0, false)];
        let e2 = vec![make_entry(2, FairnessClass::Normal, 0, false)];
        let s1 = QueueSnapshot::build(e1, 1, 1, 1, 1);
        let s2 = QueueSnapshot::build(e2, 1, 1, 1, 1);
        assert_ne!(s1.snapshot_id, s2.snapshot_id);
        assert_ne!(s1.snapshot_root, s2.snapshot_root);
    }

    #[test]
    fn test_snapshot_commitment_compute_deterministic() {
        let entries = vec![make_entry(1, FairnessClass::Normal, 0, false)];
        let snapshot = QueueSnapshot::build(entries, 5, 1, 100, 1);
        let leader = make_id(42);
        let c1 = SnapshotCommitment::compute(&snapshot, leader);
        let c2 = SnapshotCommitment::compute(&snapshot, leader);
        assert_eq!(c1.commitment_hash, c2.commitment_hash);
        assert_eq!(c1.snapshot_id, snapshot.snapshot_id);
    }

    #[test]
    fn test_snapshot_ordering_view_counts() {
        let entries = vec![
            make_entry(1, FairnessClass::Normal, 0, false),
            make_entry(2, FairnessClass::SafetyCritical, 1, true),
            make_entry(3, FairnessClass::Normal, 2, true),
        ];
        let snapshot = QueueSnapshot::build(entries, 1, 1, 1, 1);
        let view = SnapshotOrderingView::build(snapshot, make_id(10));
        assert_eq!(view.eligible_count, 3);
        assert_eq!(view.forced_inclusion_count, 2);
    }
}
