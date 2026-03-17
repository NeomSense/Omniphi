#![allow(dead_code)]

use super::state::{EpochId, EpochState, MembershipTransitionRecord};

// ---------------------------------------------------------------------------
// CarryForwardDecision
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum CarryForwardDecision {
    CarryToNextEpoch,
    DropAndInvalidate,
    EscalateToDispute,
}

// ---------------------------------------------------------------------------
// EpochBoundaryPolicy
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EpochBoundaryPolicy {
    pub unfinalized_batch_policy: CarryForwardDecision,
    pub undelivered_batch_policy: CarryForwardDecision,
    pub pending_attestation_policy: CarryForwardDecision,
}

impl EpochBoundaryPolicy {
    pub fn strict() -> Self {
        EpochBoundaryPolicy {
            unfinalized_batch_policy: CarryForwardDecision::DropAndInvalidate,
            undelivered_batch_policy: CarryForwardDecision::DropAndInvalidate,
            pending_attestation_policy: CarryForwardDecision::DropAndInvalidate,
        }
    }

    pub fn lenient() -> Self {
        EpochBoundaryPolicy {
            unfinalized_batch_policy: CarryForwardDecision::CarryToNextEpoch,
            undelivered_batch_policy: CarryForwardDecision::CarryToNextEpoch,
            pending_attestation_policy: CarryForwardDecision::CarryToNextEpoch,
        }
    }
}

// ---------------------------------------------------------------------------
// BoundaryTransitionResult
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BoundaryTransitionResult {
    pub epoch_id: EpochId,
    pub batches_carried: Vec<[u8; 32]>,
    pub batches_dropped: Vec<[u8; 32]>,
    pub batches_escalated: Vec<[u8; 32]>,
    pub membership_transition: MembershipTransitionRecord,
}

// ---------------------------------------------------------------------------
// EpochBoundaryHandler
// ---------------------------------------------------------------------------

pub struct EpochBoundaryHandler;

impl EpochBoundaryHandler {
    /// pending_batches: (batch_id, is_finalized)
    pub fn handle_boundary(
        outgoing_epoch: &EpochState,
        pending_batches: &[([u8; 32], bool)],
        policy: &EpochBoundaryPolicy,
        transition: MembershipTransitionRecord,
    ) -> BoundaryTransitionResult {
        let mut carried = Vec::new();
        let mut dropped = Vec::new();
        let mut escalated = Vec::new();

        for (batch_id, is_finalized) in pending_batches {
            if *is_finalized {
                // finalized batches are always carried (already done)
                carried.push(*batch_id);
            } else {
                match &policy.unfinalized_batch_policy {
                    CarryForwardDecision::CarryToNextEpoch => carried.push(*batch_id),
                    CarryForwardDecision::DropAndInvalidate => dropped.push(*batch_id),
                    CarryForwardDecision::EscalateToDispute => escalated.push(*batch_id),
                }
            }
        }

        BoundaryTransitionResult {
            epoch_id: outgoing_epoch.epoch_id,
            batches_carried: carried,
            batches_dropped: dropped,
            batches_escalated: escalated,
            membership_transition: transition,
        }
    }
}

// ---------------------------------------------------------------------------
// Tests (epochs boundary + state + rotation combined for count)
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::epochs::state::{
        ActiveCommittee, EpochError, EpochState, EpochStatus, EpochStore, MembershipTransitionRecord,
    };
    use crate::epochs::rotation::{CommitteeRotationConfig, CommitteeRotationEngine};
    use std::collections::{BTreeMap, BTreeSet};

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn make_epoch(id: u64, start: u64, end: u64) -> EpochState {
        let members: BTreeSet<[u8; 32]> = [node(1), node(2), node(3)].into_iter().collect();
        let committee_hash = ActiveCommittee::compute_hash(&members);
        EpochState {
            epoch_id: id,
            start_slot: start,
            end_slot: end,
            active_committee: ActiveCommittee {
                epoch_id: id,
                members,
                leader_for_slot: BTreeMap::new(),
                committee_hash,
            },
            next_committee: None,
            status: EpochStatus::Active,
        }
    }

    fn empty_transition(epoch_id: u64) -> MembershipTransitionRecord {
        MembershipTransitionRecord::compute(epoch_id, BTreeSet::new(), BTreeSet::new())
    }

    #[test]
    fn test_strict_policy_drops_all_unfinalized() {
        let epoch = make_epoch(1, 0, 100);
        let batches = vec![(node(10), false), (node(11), false)];
        let result = EpochBoundaryHandler::handle_boundary(
            &epoch,
            &batches,
            &EpochBoundaryPolicy::strict(),
            empty_transition(1),
        );
        assert_eq!(result.batches_dropped.len(), 2);
        assert_eq!(result.batches_carried.len(), 0);
    }

    #[test]
    fn test_lenient_policy_carries_all() {
        let epoch = make_epoch(1, 0, 100);
        let batches = vec![(node(10), false), (node(11), true)];
        let result = EpochBoundaryHandler::handle_boundary(
            &epoch,
            &batches,
            &EpochBoundaryPolicy::lenient(),
            empty_transition(1),
        );
        assert_eq!(result.batches_carried.len(), 2);
        assert_eq!(result.batches_dropped.len(), 0);
    }

    #[test]
    fn test_escalate_policy() {
        let epoch = make_epoch(2, 100, 200);
        let policy = EpochBoundaryPolicy {
            unfinalized_batch_policy: CarryForwardDecision::EscalateToDispute,
            undelivered_batch_policy: CarryForwardDecision::EscalateToDispute,
            pending_attestation_policy: CarryForwardDecision::EscalateToDispute,
        };
        let batches = vec![(node(20), false)];
        let result = EpochBoundaryHandler::handle_boundary(&epoch, &batches, &policy, empty_transition(2));
        assert_eq!(result.batches_escalated.len(), 1);
    }

    #[test]
    fn test_finalized_always_carried() {
        let epoch = make_epoch(1, 0, 100);
        let batches = vec![(node(30), true)];
        let result = EpochBoundaryHandler::handle_boundary(
            &epoch,
            &batches,
            &EpochBoundaryPolicy::strict(),
            empty_transition(1),
        );
        assert_eq!(result.batches_carried.len(), 1);
    }

    // EpochStore tests
    #[test]
    fn test_epoch_store_advance() {
        let mut store = EpochStore::new(0);
        let e = make_epoch(1, 0, 100);
        store.advance_to_epoch(1, e).unwrap();
        assert!(store.get(1).is_some());
    }

    #[test]
    fn test_epoch_store_non_monotonic_fails() {
        let mut store = EpochStore::new(0);
        store.advance_to_epoch(1, make_epoch(1, 0, 100)).unwrap();
        let result = store.advance_to_epoch(1, make_epoch(1, 100, 200));
        assert!(matches!(result, Err(EpochError::NonMonotonicEpoch { .. })));
    }

    #[test]
    fn test_epoch_store_slot_lookup() {
        let mut store = EpochStore::new(0);
        store.advance_to_epoch(1, make_epoch(1, 0, 100)).unwrap();
        assert!(store.is_slot_in_epoch(1, 50));
        assert!(!store.is_slot_in_epoch(1, 100));
    }

    #[test]
    fn test_epoch_for_slot() {
        let mut store = EpochStore::new(0);
        store.advance_to_epoch(1, make_epoch(1, 0, 100)).unwrap();
        store.advance_to_epoch(2, make_epoch(2, 100, 200)).unwrap();
        assert_eq!(store.epoch_for_slot(50), Some(1));
        assert_eq!(store.epoch_for_slot(150), Some(2));
        assert_eq!(store.epoch_for_slot(250), None);
    }

    #[test]
    fn test_epoch_store_current() {
        let mut store = EpochStore::new(0);
        store.advance_to_epoch(1, make_epoch(1, 0, 100)).unwrap();
        store.advance_to_epoch(2, make_epoch(2, 100, 200)).unwrap();
        assert_eq!(store.current().map(|e| e.epoch_id), Some(2));
    }

    #[test]
    fn test_empty_committee_rejected() {
        let mut store = EpochStore::new(0);
        let bad = EpochState {
            epoch_id: 1,
            start_slot: 0,
            end_slot: 100,
            active_committee: ActiveCommittee {
                epoch_id: 1,
                members: BTreeSet::new(),
                leader_for_slot: BTreeMap::new(),
                committee_hash: [0u8; 32],
            },
            next_committee: None,
            status: EpochStatus::Active,
        };
        assert!(matches!(
            store.advance_to_epoch(1, bad),
            Err(EpochError::CommitteeEmpty)
        ));
    }

    #[test]
    fn test_committee_hash_changes_with_membership() {
        let m1: BTreeSet<[u8; 32]> = [node(1), node(2)].into_iter().collect();
        let m2: BTreeSet<[u8; 32]> = [node(1), node(3)].into_iter().collect();
        assert_ne!(
            ActiveCommittee::compute_hash(&m1),
            ActiveCommittee::compute_hash(&m2)
        );
    }

    #[test]
    fn test_rotation_engine_integrates_with_epoch_store() {
        let config = CommitteeRotationConfig::default_config();
        let candidates: BTreeSet<[u8; 32]> = (1..=6u8).map(|n| { let mut id = [0u8; 32]; id[0]=n; id }).collect();
        let committee = CommitteeRotationEngine::compute_next_committee(&config, &candidates, 3).unwrap();
        let epoch = make_epoch(3, 300, 400);
        let leaders = CommitteeRotationEngine::assign_slot_leaders(&committee, &epoch);
        assert_eq!(leaders.len(), 100);
    }

    #[test]
    fn test_membership_transition_record_hash() {
        let joined: BTreeSet<[u8; 32]> = [node(5)].into_iter().collect();
        let left: BTreeSet<[u8; 32]> = [node(6)].into_iter().collect();
        let r1 = MembershipTransitionRecord::compute(4, joined.clone(), left.clone());
        let r2 = MembershipTransitionRecord::compute(4, joined, left);
        assert_eq!(r1.transition_hash, r2.transition_hash);
    }
}
