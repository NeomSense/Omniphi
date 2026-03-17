use sha2::{Sha256, Digest};
use crate::committee::membership::PoSeqCommittee;

/// Policy governing how the leader/proposer is chosen for a slot.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LeaderSelectionPolicy {
    /// Cycle through eligible proposers in sorted order by node_id.
    RoundRobin,
    /// SHA256(slot_le || epoch_le || seed) mod n.
    SlotHash { seed: [u8; 32] },
    /// Weighted round-robin (weight stored in metadata key "weight", default 1).
    WeightedRoundRobin,
}

/// Metadata for a proposal slot.
#[derive(Debug, Clone)]
pub struct ProposalSlot {
    pub slot_number: u64,
    pub epoch: u64,
    pub height: u64,
}

/// The result of a leader selection operation.
#[derive(Debug, Clone)]
pub struct SelectedLeader {
    pub node_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub selection_method: String,
    pub selection_hash: [u8; 32],
}

pub struct LeaderSelector;

impl LeaderSelector {
    /// Deterministically select a leader for the given slot/epoch from the committee.
    ///
    /// Returns `None` if there are no eligible proposers.
    pub fn select(
        slot: u64,
        epoch: u64,
        committee: &PoSeqCommittee,
        policy: &LeaderSelectionPolicy,
    ) -> Option<SelectedLeader> {
        let mut proposers = committee.eligible_proposers();
        if proposers.is_empty() {
            return None;
        }
        // Sort by node_id for full determinism (eligible_proposers() sorts too, but be explicit)
        proposers.sort_by_key(|n| n.node_id);
        let n = proposers.len() as u64;

        let (index, method, selection_hash) = match policy {
            LeaderSelectionPolicy::RoundRobin => {
                let idx = (slot % n) as usize;
                let hash = Self::compute_selection_hash(slot, epoch, idx as u64, &[0u8; 32]);
                (idx, "RoundRobin".to_string(), hash)
            }
            LeaderSelectionPolicy::SlotHash { seed } => {
                let hash = Self::compute_slot_hash(slot, epoch, seed);
                let mut val = 0u64;
                for i in 0..8 {
                    val = (val << 8) | (hash[i] as u64);
                }
                let idx = (val % n) as usize;
                (idx, "SlotHash".to_string(), hash)
            }
            LeaderSelectionPolicy::WeightedRoundRobin => {
                // Build weighted list (sorted by node_id for determinism)
                let weighted: Vec<([u8; 32], u64)> = proposers
                    .iter()
                    .map(|p| {
                        let weight = p
                            .metadata
                            .get("weight")
                            .and_then(|w| w.parse::<u64>().ok())
                            .unwrap_or(1);
                        (p.node_id, weight)
                    })
                    .collect();
                let total_weight: u64 = weighted.iter().map(|(_, w)| w).sum();
                // FIND-018: if all weights are zero, no fair selection is possible — return None.
                if total_weight == 0 {
                    return None;
                }
                let slot_pos = slot % total_weight;
                let mut cumulative = 0u64;
                let mut chosen_idx = 0usize;
                for (i, (_, weight)) in weighted.iter().enumerate() {
                    cumulative += weight;
                    if slot_pos < cumulative {
                        chosen_idx = i;
                        break;
                    }
                }
                let hash = Self::compute_selection_hash(slot, epoch, chosen_idx as u64, &[0u8; 32]);
                (chosen_idx, "WeightedRoundRobin".to_string(), hash)
            }
        };

        let node_id = proposers[index].node_id;
        Some(SelectedLeader {
            node_id,
            slot,
            epoch,
            selection_method: method,
            selection_hash,
        })
    }

    fn compute_slot_hash(slot: u64, epoch: u64, seed: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&slot.to_le_bytes());
        hasher.update(&epoch.to_le_bytes());
        hasher.update(seed);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    fn compute_selection_hash(slot: u64, epoch: u64, index: u64, seed: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&slot.to_be_bytes());
        hasher.update(&epoch.to_be_bytes());
        hasher.update(&index.to_be_bytes());
        hasher.update(seed);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::committee::membership::PoSeqCommittee;
    use crate::identities::node::{NodeIdentity, NodeRole};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_committee_with_sequencers(ids: &[u8]) -> PoSeqCommittee {
        let mut c = PoSeqCommittee::new(1);
        for &b in ids {
            let mut n = NodeIdentity::new(make_id(b), make_id(b + 100), NodeRole::Sequencer, 0);
            n.activate();
            c.add_node(n);
        }
        c
    }

    #[test]
    fn test_round_robin_determinism() {
        let committee = make_committee_with_sequencers(&[1, 2, 3]);
        let r1 = LeaderSelector::select(5, 1, &committee, &LeaderSelectionPolicy::RoundRobin);
        let r2 = LeaderSelector::select(5, 1, &committee, &LeaderSelectionPolicy::RoundRobin);
        assert!(r1.is_some() && r2.is_some());
        assert_eq!(r1.unwrap().node_id, r2.unwrap().node_id);
    }

    #[test]
    fn test_round_robin_cycles() {
        let committee = make_committee_with_sequencers(&[1, 2, 3]);
        let r0 = LeaderSelector::select(0, 1, &committee, &LeaderSelectionPolicy::RoundRobin).unwrap();
        let r1 = LeaderSelector::select(1, 1, &committee, &LeaderSelectionPolicy::RoundRobin).unwrap();
        let r2 = LeaderSelector::select(2, 1, &committee, &LeaderSelectionPolicy::RoundRobin).unwrap();
        let r3 = LeaderSelector::select(3, 1, &committee, &LeaderSelectionPolicy::RoundRobin).unwrap();
        // slot 3 wraps back to slot 0
        assert_eq!(r3.node_id, r0.node_id);
        // all three are distinct
        assert_ne!(r0.node_id, r1.node_id);
        assert_ne!(r1.node_id, r2.node_id);
    }

    #[test]
    fn test_slot_hash_determinism() {
        let committee = make_committee_with_sequencers(&[1, 2, 3]);
        let seed = [42u8; 32];
        let policy = LeaderSelectionPolicy::SlotHash { seed };
        let r1 = LeaderSelector::select(7, 2, &committee, &policy);
        let r2 = LeaderSelector::select(7, 2, &committee, &policy);
        assert!(r1.is_some() && r2.is_some());
        assert_eq!(r1.unwrap().node_id, r2.unwrap().node_id);
    }

    #[test]
    fn test_slot_hash_different_slots_differ() {
        // Not guaranteed to differ (depends on hash), but with 3 nodes, slots 0..9 should have variation
        let committee = make_committee_with_sequencers(&[1, 2, 3]);
        let seed = [1u8; 32];
        let policy = LeaderSelectionPolicy::SlotHash { seed };
        let results: Vec<[u8; 32]> = (0..9)
            .map(|slot| LeaderSelector::select(slot, 1, &committee, &policy).unwrap().node_id)
            .collect();
        // At least two different leaders should appear in 9 slots with 3 nodes
        let unique: std::collections::BTreeSet<[u8; 32]> = results.into_iter().collect();
        assert!(unique.len() >= 2);
    }

    #[test]
    fn test_empty_committee_returns_none() {
        let committee = PoSeqCommittee::new(1);
        let result = LeaderSelector::select(0, 1, &committee, &LeaderSelectionPolicy::RoundRobin);
        assert!(result.is_none());
    }

    #[test]
    fn test_weighted_round_robin_zero_total_weight_returns_none() {
        // FIND-018: all nodes with weight=0 must produce None, not a spurious index-0 result
        let mut committee = make_committee_with_sequencers(&[1, 2]);
        for id in [make_id(1), make_id(2)] {
            if let Some(n) = committee.sequencers.get_mut(&id) {
                n.metadata.insert("weight".to_string(), "0".to_string());
            }
        }
        let result = LeaderSelector::select(5, 1, &committee, &LeaderSelectionPolicy::WeightedRoundRobin);
        assert!(result.is_none(), "zero total weight must return None, not a spurious leader");
    }

    #[test]
    fn test_weighted_round_robin_determinism() {
        let mut committee = make_committee_with_sequencers(&[1, 2]);
        // Give node 2 weight=3
        if let Some(n) = committee.sequencers.get_mut(&make_id(2)) {
            n.metadata.insert("weight".to_string(), "3".to_string());
        }
        let r1 = LeaderSelector::select(0, 1, &committee, &LeaderSelectionPolicy::WeightedRoundRobin);
        let r2 = LeaderSelector::select(0, 1, &committee, &LeaderSelectionPolicy::WeightedRoundRobin);
        assert_eq!(r1.unwrap().node_id, r2.unwrap().node_id);
    }
}
