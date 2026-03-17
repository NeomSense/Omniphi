#![allow(dead_code)]

use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

pub type EpochId = u64;

// ---------------------------------------------------------------------------
// EpochStatus
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum EpochStatus {
    Active,
    Transitioning,
    Completed,
}

// ---------------------------------------------------------------------------
// ActiveCommittee
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ActiveCommittee {
    pub epoch_id: EpochId,
    pub members: BTreeSet<[u8; 32]>,
    pub leader_for_slot: BTreeMap<u64, [u8; 32]>,
    pub committee_hash: [u8; 32],
}

impl ActiveCommittee {
    pub fn compute_hash(members: &BTreeSet<[u8; 32]>) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for m in members {
            hasher.update(m);
        }
        hasher.finalize().into()
    }

    pub fn leader_at(&self, slot: u64) -> Option<&[u8; 32]> {
        self.leader_for_slot.get(&slot)
    }
}

// ---------------------------------------------------------------------------
// NextCommittee
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct NextCommittee {
    pub epoch_id: EpochId,
    pub members: BTreeSet<[u8; 32]>,
    pub activation_slot: u64,
}

// ---------------------------------------------------------------------------
// MembershipTransitionRecord
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct MembershipTransitionRecord {
    pub epoch_id: EpochId,
    pub joined: BTreeSet<[u8; 32]>,
    pub left: BTreeSet<[u8; 32]>,
    pub transition_hash: [u8; 32],
}

impl MembershipTransitionRecord {
    pub fn compute(epoch_id: EpochId, joined: BTreeSet<[u8; 32]>, left: BTreeSet<[u8; 32]>) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(epoch_id.to_le_bytes());
        for j in &joined {
            hasher.update(b"J");
            hasher.update(j);
        }
        for l in &left {
            hasher.update(b"L");
            hasher.update(l);
        }
        let transition_hash = hasher.finalize().into();
        MembershipTransitionRecord {
            epoch_id,
            joined,
            left,
            transition_hash,
        }
    }
}

// ---------------------------------------------------------------------------
// EpochState
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EpochState {
    pub epoch_id: EpochId,
    pub start_slot: u64,
    pub end_slot: u64,
    pub active_committee: ActiveCommittee,
    pub next_committee: Option<NextCommittee>,
    pub status: EpochStatus,
}

// ---------------------------------------------------------------------------
// EpochError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum EpochError {
    EpochAlreadyExists(EpochId),
    EpochNotFound(EpochId),
    InvalidSlotRange { start: u64, end: u64 },
    CommitteeEmpty,
    NonMonotonicEpoch { current: EpochId, attempted: EpochId },
}

// ---------------------------------------------------------------------------
// EpochStore
// ---------------------------------------------------------------------------

pub struct EpochStore {
    epochs: BTreeMap<EpochId, EpochState>,
    current_epoch: EpochId,
}

impl EpochStore {
    pub fn new(initial_epoch: EpochId) -> Self {
        EpochStore {
            epochs: BTreeMap::new(),
            current_epoch: initial_epoch,
        }
    }

    pub fn current(&self) -> Option<&EpochState> {
        self.epochs.get(&self.current_epoch)
    }

    pub fn get(&self, epoch_id: EpochId) -> Option<&EpochState> {
        self.epochs.get(&epoch_id)
    }

    pub fn advance_to_epoch(
        &mut self,
        new_epoch: EpochId,
        state: EpochState,
    ) -> Result<(), EpochError> {
        if new_epoch <= self.current_epoch && !self.epochs.is_empty() {
            return Err(EpochError::NonMonotonicEpoch {
                current: self.current_epoch,
                attempted: new_epoch,
            });
        }
        if self.epochs.contains_key(&new_epoch) {
            return Err(EpochError::EpochAlreadyExists(new_epoch));
        }
        if state.start_slot >= state.end_slot {
            return Err(EpochError::InvalidSlotRange {
                start: state.start_slot,
                end: state.end_slot,
            });
        }
        if state.active_committee.members.is_empty() {
            return Err(EpochError::CommitteeEmpty);
        }
        self.current_epoch = new_epoch;
        self.epochs.insert(new_epoch, state);
        Ok(())
    }

    pub fn is_slot_in_epoch(&self, epoch_id: EpochId, slot: u64) -> bool {
        match self.epochs.get(&epoch_id) {
            Some(e) => slot >= e.start_slot && slot < e.end_slot,
            None => false,
        }
    }

    pub fn epoch_for_slot(&self, slot: u64) -> Option<EpochId> {
        self.epochs
            .values()
            .find(|e| slot >= e.start_slot && slot < e.end_slot)
            .map(|e| e.epoch_id)
    }
}
