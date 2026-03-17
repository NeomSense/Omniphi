use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};
use crate::errors::Phase4Error;
use crate::committee_rotation::snapshot::EpochCommitteeSnapshot;

/// Configuration for committee rotation.
#[derive(Debug, Clone)]
pub struct CommitteeRotationConfig {
    /// How many epochs between rotations.
    pub rotation_period_epochs: u64,
    /// Minimum committee size.
    pub min_committee_size: usize,
    /// Maximum committee size.
    pub max_committee_size: usize,
    /// A fixed seed mixed into shuffle (per-epoch determinism).
    pub base_shuffle_seed: [u8; 32],
}

impl CommitteeRotationConfig {
    pub fn new(
        rotation_period_epochs: u64,
        min_committee_size: usize,
        max_committee_size: usize,
        base_shuffle_seed: [u8; 32],
    ) -> Result<Self, Phase4Error> {
        if min_committee_size > max_committee_size {
            return Err(Phase4Error::InvalidRotationConfig(
                format!("min {} > max {}", min_committee_size, max_committee_size)
            ));
        }
        if rotation_period_epochs == 0 {
            return Err(Phase4Error::InvalidRotationConfig(
                "rotation_period_epochs must be > 0".to_string()
            ));
        }
        Ok(CommitteeRotationConfig {
            rotation_period_epochs,
            min_committee_size,
            max_committee_size,
            base_shuffle_seed,
        })
    }

    /// Whether this epoch triggers a rotation.
    pub fn is_rotation_epoch(&self, epoch: u64) -> bool {
        epoch % self.rotation_period_epochs == 0
    }

    /// Compute the shuffle seed for a given epoch.
    ///
    /// FIND-007: the seed mixes in `prev_finalization_hash` — the finalization hash
    /// of the last batch from the previous epoch. This value is only available after
    /// the epoch boundary finalizes, making future committee composition unpredictable
    /// to any attacker who does not control the previous epoch's finalization.
    ///
    /// Formula: SHA256(epoch_be ‖ base_seed ‖ prev_finalization_hash)
    pub fn shuffle_seed(&self, epoch: u64, prev_finalization_hash: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&epoch.to_be_bytes());
        hasher.update(&self.base_shuffle_seed);
        hasher.update(prev_finalization_hash);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Records who joined and left in a rotation event.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RotationRecord {
    pub epoch: u64,
    pub joined: BTreeSet<[u8; 32]>,
    pub left: BTreeSet<[u8; 32]>,
    pub new_committee_hash: [u8; 32],
}

impl RotationRecord {
    pub fn new(
        epoch: u64,
        old_members: &BTreeSet<[u8; 32]>,
        new_members: &BTreeSet<[u8; 32]>,
        new_committee_hash: [u8; 32],
    ) -> Self {
        let joined: BTreeSet<[u8; 32]> = new_members.difference(old_members).cloned().collect();
        let left: BTreeSet<[u8; 32]> = old_members.difference(new_members).cloned().collect();
        RotationRecord { epoch, joined, left, new_committee_hash }
    }

    pub fn is_no_change(&self) -> bool {
        self.joined.is_empty() && self.left.is_empty()
    }
}

/// Drives deterministic committee rotation.
pub struct CommitteeRotationEngine {
    pub config: CommitteeRotationConfig,
    pub rotation_history: BTreeMap<u64, RotationRecord>,
}

impl CommitteeRotationEngine {
    pub fn new(config: CommitteeRotationConfig) -> Self {
        CommitteeRotationEngine {
            config,
            rotation_history: BTreeMap::new(),
        }
    }

    /// Produce a new committee snapshot for the given epoch.
    ///
    /// Algorithm:
    /// 1. Sort candidates by node_id (BTreeSet order).
    /// 2. Compute shuffle_seed = SHA256(epoch ‖ base_seed ‖ prev_finalization_hash).
    /// 3. Use the seed bytes to select up to max_committee_size members.
    /// 4. If selected < min_committee_size → error.
    ///
    /// `prev_finalization_hash` must be the finalization hash of the last batch
    /// committed in the previous epoch. Pass `[0u8; 32]` only at genesis (epoch 0).
    pub fn rotate(
        &mut self,
        epoch: u64,
        candidates: &BTreeSet<[u8; 32]>,
        prev_snapshot: Option<&EpochCommitteeSnapshot>,
        excluded: &BTreeSet<[u8; 32]>,
        prev_finalization_hash: &[u8; 32],
    ) -> Result<(EpochCommitteeSnapshot, RotationRecord), Phase4Error> {
        // Filter excluded (jailed) nodes
        let eligible: Vec<[u8; 32]> = candidates
            .iter()
            .filter(|id| !excluded.contains(*id))
            .cloned()
            .collect();

        let available = eligible.len();
        if available < self.config.min_committee_size {
            return Err(Phase4Error::InsufficientCandidates {
                required: self.config.min_committee_size,
                available,
            });
        }

        let target_size = available.min(self.config.max_committee_size);
        // FIND-007: mix previous epoch finalization hash into seed
        let seed = self.config.shuffle_seed(epoch, prev_finalization_hash);

        // Select `target_size` members deterministically using SHA256(seed ‖ candidate_id) scores.
        // Sort eligible by their scores, take top `target_size`.
        let mut scored: Vec<([u8; 32], [u8; 32])> = eligible
            .iter()
            .map(|id| {
                let mut h = Sha256::new();
                h.update(&seed);
                h.update(id);
                let r = h.finalize();
                let mut score = [0u8; 32];
                score.copy_from_slice(&r);
                (*id, score)
            })
            .collect();

        // Sort by score (deterministic BTreeSet-order input → deterministic scores → deterministic output)
        scored.sort_by(|a, b| a.1.cmp(&b.1));
        scored.truncate(target_size);

        let new_members: BTreeSet<[u8; 32]> = scored.into_iter().map(|(id, _)| id).collect();

        if new_members.len() < self.config.min_committee_size {
            return Err(Phase4Error::InsufficientCandidates {
                required: self.config.min_committee_size,
                available: new_members.len(),
            });
        }

        let snapshot = EpochCommitteeSnapshot::new(epoch, new_members.clone());

        let old_members = prev_snapshot
            .map(|s| &s.members)
            .cloned()
            .unwrap_or_default();

        let record = RotationRecord::new(epoch, &old_members, &new_members, snapshot.committee_hash);
        self.rotation_history.insert(epoch, record.clone());

        Ok((snapshot, record))
    }

    /// Get a rotation record by epoch.
    pub fn get_record(&self, epoch: u64) -> Option<&RotationRecord> {
        self.rotation_history.get(&epoch)
    }

    /// How many rotations have been recorded.
    pub fn rotation_count(&self) -> usize {
        self.rotation_history.len()
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

    fn make_config(min: usize, max: usize) -> CommitteeRotationConfig {
        CommitteeRotationConfig::new(4, min, max, [0u8; 32]).unwrap()
    }

    fn make_candidates(bytes: &[u8]) -> BTreeSet<[u8; 32]> {
        bytes.iter().map(|&b| make_id(b)).collect()
    }

    #[test]
    fn test_config_invalid_min_gt_max() {
        let err = CommitteeRotationConfig::new(4, 5, 3, [0u8; 32]);
        assert!(err.is_err());
    }

    #[test]
    fn test_config_zero_period_rejected() {
        let err = CommitteeRotationConfig::new(0, 3, 5, [0u8; 32]);
        assert!(err.is_err());
    }

    #[test]
    fn test_config_is_rotation_epoch() {
        let cfg = make_config(3, 5);
        assert!(cfg.is_rotation_epoch(0));
        assert!(cfg.is_rotation_epoch(4));
        assert!(!cfg.is_rotation_epoch(1));
        assert!(!cfg.is_rotation_epoch(3));
    }

    #[test]
    fn test_shuffle_seed_determinism() {
        let cfg = make_config(3, 5);
        let prev = [0xABu8; 32];
        let s1 = cfg.shuffle_seed(7, &prev);
        let s2 = cfg.shuffle_seed(7, &prev);
        assert_eq!(s1, s2);
    }

    #[test]
    fn test_shuffle_seed_differs_per_epoch() {
        let cfg = make_config(3, 5);
        let prev = [0xABu8; 32];
        let s1 = cfg.shuffle_seed(1, &prev);
        let s2 = cfg.shuffle_seed(2, &prev);
        assert_ne!(s1, s2);
    }

    #[test]
    fn test_shuffle_seed_differs_with_different_finalization_hash() {
        // FIND-007: different prev finalization hashes produce different seeds
        let cfg = make_config(3, 5);
        let s1 = cfg.shuffle_seed(5, &[0x00u8; 32]);
        let s2 = cfg.shuffle_seed(5, &[0xFFu8; 32]);
        assert_ne!(s1, s2, "seed must depend on prev finalization hash");
    }

    #[test]
    fn test_rotate_basic_selection() {
        let cfg = make_config(3, 5);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3, 4, 5, 6]);
        let excluded = BTreeSet::new();
        let (snapshot, record) = engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        assert!(snapshot.size() >= 3 && snapshot.size() <= 5);
        assert_eq!(record.epoch, 1);
    }

    #[test]
    fn test_rotate_insufficient_candidates() {
        let cfg = make_config(5, 10);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3]); // only 3 < min 5
        let excluded = BTreeSet::new();
        let result = engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]);
        assert!(matches!(result, Err(Phase4Error::InsufficientCandidates { .. })));
    }

    #[test]
    fn test_rotate_is_deterministic() {
        let cfg1 = make_config(3, 5);
        let cfg2 = make_config(3, 5);
        let mut e1 = CommitteeRotationEngine::new(cfg1);
        let mut e2 = CommitteeRotationEngine::new(cfg2);
        let candidates = make_candidates(&[1, 2, 3, 4, 5, 6]);
        let excluded = BTreeSet::new();
        let prev = [0x55u8; 32];
        let (s1, _) = e1.rotate(1, &candidates, None, &excluded, &prev).unwrap();
        let (s2, _) = e2.rotate(1, &candidates, None, &excluded, &prev).unwrap();
        assert_eq!(s1.committee_hash, s2.committee_hash);
        assert_eq!(s1.members, s2.members);
    }

    #[test]
    fn test_rotate_excludes_jailed_nodes() {
        let cfg = make_config(2, 10);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3, 4, 5]);
        let mut excluded = BTreeSet::new();
        excluded.insert(make_id(1));
        excluded.insert(make_id(2));
        let (snapshot, _) = engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        assert!(!snapshot.contains(&make_id(1)));
        assert!(!snapshot.contains(&make_id(2)));
    }

    #[test]
    fn test_rotation_record_joined_left() {
        let old: BTreeSet<[u8; 32]> = [make_id(1), make_id(2)].iter().cloned().collect();
        let new: BTreeSet<[u8; 32]> = [make_id(2), make_id(3)].iter().cloned().collect();
        let record = RotationRecord::new(5, &old, &new, [0u8; 32]);
        assert!(record.joined.contains(&make_id(3)));
        assert!(record.left.contains(&make_id(1)));
        assert!(!record.joined.contains(&make_id(2)));
    }

    #[test]
    fn test_rotation_record_no_change() {
        let members: BTreeSet<[u8; 32]> = [make_id(1), make_id(2)].iter().cloned().collect();
        let record = RotationRecord::new(5, &members, &members, [0u8; 32]);
        assert!(record.is_no_change());
    }

    #[test]
    fn test_rotation_history_stored() {
        let cfg = make_config(3, 5);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3, 4, 5]);
        let excluded = BTreeSet::new();
        engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        engine.rotate(2, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        assert_eq!(engine.rotation_count(), 2);
        assert!(engine.get_record(1).is_some());
        assert!(engine.get_record(2).is_some());
    }

    #[test]
    fn test_rotate_max_size_respected() {
        let cfg = make_config(2, 3);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3, 4, 5, 6, 7, 8]);
        let excluded = BTreeSet::new();
        let (snapshot, _) = engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        assert!(snapshot.size() <= 3);
    }

    #[test]
    fn test_rotate_with_previous_snapshot_computes_diff() {
        let cfg = make_config(2, 10);
        let mut engine = CommitteeRotationEngine::new(cfg);
        let candidates = make_candidates(&[1, 2, 3, 4, 5]);
        let excluded = BTreeSet::new();
        let (snap1, _) = engine.rotate(1, &candidates, None, &excluded, &[0u8; 32]).unwrap();
        let (_snap2, record2) = engine.rotate(2, &candidates, Some(&snap1), &excluded, &[0xAAu8; 32]).unwrap();
        // Record should reflect diff from snap1 → snap2
        assert_eq!(record2.epoch, 2);
    }
}
