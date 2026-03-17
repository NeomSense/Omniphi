#![allow(dead_code)]

// Phase 3: Finality Commitment and Validator Checks
pub mod commitment;
pub mod validator_checks;

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::fmt;

// ---------------------------------------------------------------------------
// FinalityState
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum FinalityState {
    Proposed,
    Attested,
    QuorumReached,
    Finalized,
    RuntimeDelivered,
    RuntimeAcknowledged,
    RuntimeRejected,
    Superseded,
    Invalidated,
    DisputedPlaceholder,
    Recovered,
}

impl fmt::Display for FinalityState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

impl FinalityState {
    /// Returns true if this transition is legal.
    pub fn can_transition_to(&self, next: &FinalityState) -> bool {
        use FinalityState::*;
        matches!(
            (self, next),
            (Proposed, Attested)
                | (Proposed, Superseded)
                | (Proposed, Invalidated)
                | (Attested, QuorumReached)
                | (Attested, Superseded)
                | (Attested, Invalidated)
                | (QuorumReached, Finalized)
                | (QuorumReached, Invalidated)
                | (Finalized, RuntimeDelivered)
                | (Finalized, DisputedPlaceholder)
                | (RuntimeDelivered, RuntimeAcknowledged)
                | (RuntimeDelivered, RuntimeRejected)
                | (RuntimeDelivered, Recovered)
                | (RuntimeRejected, Recovered)
                | (RuntimeRejected, DisputedPlaceholder)
                | (DisputedPlaceholder, Recovered)
                | (DisputedPlaceholder, Invalidated)
                | (Recovered, RuntimeDelivered)
        )
    }

    pub fn is_terminal(&self) -> bool {
        use FinalityState::*;
        matches!(self, RuntimeAcknowledged | Superseded | Invalidated)
    }
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum FinalityStateError {
    IllegalTransition {
        from: FinalityState,
        to: FinalityState,
    },
    AlreadyTerminal(FinalityState),
    BatchNotFound([u8; 32]),
}

impl fmt::Display for FinalityStateError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

// ---------------------------------------------------------------------------
// FinalityTransitionRecord
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FinalityTransitionRecord {
    pub from: FinalityState,
    pub to: FinalityState,
    pub reason: String,
    pub timestamp_seq: u64,
}

// ---------------------------------------------------------------------------
// BatchFinalityStatus
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchFinalityStatus {
    pub batch_id: [u8; 32],
    pub current_state: FinalityState,
    pub epoch: u64,
    pub slot: u64,
    pub transitions: Vec<FinalityTransitionRecord>,
}

impl BatchFinalityStatus {
    pub fn new(batch_id: [u8; 32], epoch: u64, slot: u64) -> Self {
        BatchFinalityStatus {
            batch_id,
            current_state: FinalityState::Proposed,
            epoch,
            slot,
            transitions: Vec::new(),
        }
    }

    pub fn transition(
        &mut self,
        next: FinalityState,
        reason: &str,
    ) -> Result<(), FinalityStateError> {
        if self.current_state.is_terminal() {
            return Err(FinalityStateError::AlreadyTerminal(
                self.current_state.clone(),
            ));
        }
        if !self.current_state.can_transition_to(&next) {
            return Err(FinalityStateError::IllegalTransition {
                from: self.current_state.clone(),
                to: next,
            });
        }
        let record = FinalityTransitionRecord {
            from: self.current_state.clone(),
            to: next.clone(),
            reason: reason.to_string(),
            timestamp_seq: self.transitions.len() as u64,
        };
        self.transitions.push(record);
        self.current_state = next;
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// FinalityGuaranteeLevel
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum FinalityGuaranteeLevel {
    Tentative,
    WeakFinality,
    StrongFinality,
    Delivered,
    /// FIND-019: a batch in the Recovered state is pending re-delivery — it is NOT yet
    /// re-delivered to the runtime and must not be treated as equivalent to Delivered.
    RecoveryPending,
    Acknowledged,
}

// ---------------------------------------------------------------------------
// FinalityCheckpoint
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FinalityCheckpoint {
    pub epoch: u64,
    pub last_finalized_batch_id: [u8; 32],
    pub last_finalized_slot: u64,
    pub checkpoint_hash: [u8; 32],
}

impl FinalityCheckpoint {
    pub fn compute(epoch: u64, batch_id: [u8; 32], slot: u64) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(epoch.to_le_bytes());
        hasher.update(batch_id);
        hasher.update(slot.to_le_bytes());
        let checkpoint_hash: [u8; 32] = hasher.finalize().into();
        FinalityCheckpoint {
            epoch,
            last_finalized_batch_id: batch_id,
            last_finalized_slot: slot,
            checkpoint_hash,
        }
    }
}

// ---------------------------------------------------------------------------
// FinalityStore
// ---------------------------------------------------------------------------

pub struct FinalityStore {
    statuses: BTreeMap<[u8; 32], BatchFinalityStatus>,
    seq: u64,
}

impl FinalityStore {
    pub fn new() -> Self {
        FinalityStore {
            statuses: BTreeMap::new(),
            seq: 0,
        }
    }

    pub fn init_batch(&mut self, batch_id: [u8; 32], epoch: u64, slot: u64) {
        let status = BatchFinalityStatus::new(batch_id, epoch, slot);
        self.statuses.insert(batch_id, status);
    }

    pub fn transition(
        &mut self,
        batch_id: &[u8; 32],
        next: FinalityState,
        reason: &str,
    ) -> Result<(), FinalityStateError> {
        let seq = self.seq;
        self.seq += 1;
        let status = self
            .statuses
            .get_mut(batch_id)
            .ok_or(FinalityStateError::BatchNotFound(*batch_id))?;

        if status.current_state.is_terminal() {
            return Err(FinalityStateError::AlreadyTerminal(
                status.current_state.clone(),
            ));
        }
        if !status.current_state.can_transition_to(&next) {
            return Err(FinalityStateError::IllegalTransition {
                from: status.current_state.clone(),
                to: next,
            });
        }
        let record = FinalityTransitionRecord {
            from: status.current_state.clone(),
            to: next.clone(),
            reason: reason.to_string(),
            timestamp_seq: seq,
        };
        status.transitions.push(record);
        status.current_state = next;
        Ok(())
    }

    pub fn get(&self, batch_id: &[u8; 32]) -> Option<&BatchFinalityStatus> {
        self.statuses.get(batch_id)
    }

    pub fn guarantee_level(&self, batch_id: &[u8; 32]) -> Option<FinalityGuaranteeLevel> {
        let status = self.statuses.get(batch_id)?;
        let level = match &status.current_state {
            FinalityState::Proposed => FinalityGuaranteeLevel::Tentative,
            FinalityState::Attested => FinalityGuaranteeLevel::WeakFinality,
            FinalityState::QuorumReached | FinalityState::Finalized => {
                FinalityGuaranteeLevel::StrongFinality
            }
            FinalityState::RuntimeDelivered | FinalityState::RuntimeRejected => {
                FinalityGuaranteeLevel::Delivered
            }
            // FIND-019: Recovered means re-delivery is pending — not yet re-delivered.
            FinalityState::Recovered => FinalityGuaranteeLevel::RecoveryPending,
            FinalityState::RuntimeAcknowledged => FinalityGuaranteeLevel::Acknowledged,
            _ => FinalityGuaranteeLevel::Tentative,
        };
        Some(level)
    }

    pub fn all_in_state(&self, state: &FinalityState) -> Vec<[u8; 32]> {
        self.statuses
            .iter()
            .filter(|(_, s)| &s.current_state == state)
            .map(|(id, _)| *id)
            .collect()
    }

    /// Returns a checkpoint for the given epoch: the latest finalized batch in that epoch.
    pub fn checkpoint(&self, epoch: u64) -> Option<FinalityCheckpoint> {
        let finalized: Vec<&BatchFinalityStatus> = self
            .statuses
            .values()
            .filter(|s| {
                s.epoch == epoch
                    && matches!(
                        s.current_state,
                        FinalityState::Finalized
                            | FinalityState::RuntimeDelivered
                            | FinalityState::RuntimeAcknowledged
                    )
            })
            .collect();

        if finalized.is_empty() {
            return None;
        }

        // Pick the one with highest slot
        let best = finalized.iter().max_by_key(|s| s.slot)?;
        Some(FinalityCheckpoint::compute(
            epoch,
            best.batch_id,
            best.slot,
        ))
    }
}

impl Default for FinalityStore {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn batch_id(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    #[test]
    fn test_legal_transition_proposed_to_attested() {
        let mut status = BatchFinalityStatus::new(batch_id(1), 1, 10);
        assert!(status.transition(FinalityState::Attested, "got votes").is_ok());
        assert_eq!(status.current_state, FinalityState::Attested);
    }

    #[test]
    fn test_illegal_transition_proposed_to_finalized() {
        let mut status = BatchFinalityStatus::new(batch_id(2), 1, 10);
        let result = status.transition(FinalityState::Finalized, "skip");
        assert!(result.is_err());
        assert_eq!(status.current_state, FinalityState::Proposed);
    }

    #[test]
    fn test_terminal_state_blocks_further_transitions() {
        let mut status = BatchFinalityStatus::new(batch_id(3), 1, 10);
        status.transition(FinalityState::Attested, "r1").unwrap();
        status.transition(FinalityState::QuorumReached, "r2").unwrap();
        status.transition(FinalityState::Finalized, "r3").unwrap();
        status.transition(FinalityState::RuntimeDelivered, "r4").unwrap();
        status.transition(FinalityState::RuntimeAcknowledged, "r5").unwrap();
        assert!(status.current_state.is_terminal());
        let result = status.transition(FinalityState::Recovered, "r6");
        assert!(result.is_err());
    }

    #[test]
    fn test_transition_to_superseded_is_terminal() {
        let mut status = BatchFinalityStatus::new(batch_id(4), 1, 5);
        status.transition(FinalityState::Superseded, "fork").unwrap();
        assert!(status.current_state.is_terminal());
    }

    #[test]
    fn test_transition_to_invalidated_is_terminal() {
        let mut status = BatchFinalityStatus::new(batch_id(5), 1, 5);
        status.transition(FinalityState::Invalidated, "bad").unwrap();
        assert!(status.current_state.is_terminal());
    }

    #[test]
    fn test_store_init_and_get() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(10), 2, 20);
        let s = store.get(&batch_id(10)).unwrap();
        assert_eq!(s.current_state, FinalityState::Proposed);
    }

    #[test]
    fn test_store_transition_happy_path() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(11), 2, 21);
        store.transition(&batch_id(11), FinalityState::Attested, "ok").unwrap();
        store.transition(&batch_id(11), FinalityState::QuorumReached, "ok").unwrap();
        store.transition(&batch_id(11), FinalityState::Finalized, "ok").unwrap();
        assert_eq!(
            store.get(&batch_id(11)).unwrap().current_state,
            FinalityState::Finalized
        );
    }

    #[test]
    fn test_store_batch_not_found() {
        let mut store = FinalityStore::new();
        let result = store.transition(&batch_id(99), FinalityState::Attested, "na");
        assert!(matches!(result, Err(FinalityStateError::BatchNotFound(_))));
    }

    #[test]
    fn test_guarantee_level_tentative() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(20), 1, 1);
        assert_eq!(
            store.guarantee_level(&batch_id(20)),
            Some(FinalityGuaranteeLevel::Tentative)
        );
    }

    #[test]
    fn test_guarantee_level_strong_finality() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(21), 1, 2);
        store.transition(&batch_id(21), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(21), FinalityState::QuorumReached, "b").unwrap();
        assert_eq!(
            store.guarantee_level(&batch_id(21)),
            Some(FinalityGuaranteeLevel::StrongFinality)
        );
    }

    #[test]
    fn test_guarantee_level_acknowledged() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(22), 1, 3);
        store.transition(&batch_id(22), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(22), FinalityState::QuorumReached, "b").unwrap();
        store.transition(&batch_id(22), FinalityState::Finalized, "c").unwrap();
        store.transition(&batch_id(22), FinalityState::RuntimeDelivered, "d").unwrap();
        store.transition(&batch_id(22), FinalityState::RuntimeAcknowledged, "e").unwrap();
        assert_eq!(
            store.guarantee_level(&batch_id(22)),
            Some(FinalityGuaranteeLevel::Acknowledged)
        );
    }

    #[test]
    fn test_all_in_state() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(30), 1, 1);
        store.init_batch(batch_id(31), 1, 2);
        store.transition(&batch_id(30), FinalityState::Attested, "a").unwrap();
        let proposed = store.all_in_state(&FinalityState::Proposed);
        assert_eq!(proposed.len(), 1);
        assert!(proposed.contains(&batch_id(31)));
    }

    #[test]
    fn test_checkpoint_returns_highest_slot() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(40), 3, 10);
        store.init_batch(batch_id(41), 3, 20);
        // finalize batch_id(40)
        store.transition(&batch_id(40), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(40), FinalityState::QuorumReached, "b").unwrap();
        store.transition(&batch_id(40), FinalityState::Finalized, "c").unwrap();
        // finalize batch_id(41) — higher slot
        store.transition(&batch_id(41), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(41), FinalityState::QuorumReached, "b").unwrap();
        store.transition(&batch_id(41), FinalityState::Finalized, "c").unwrap();
        let cp = store.checkpoint(3).unwrap();
        assert_eq!(cp.epoch, 3);
        assert_eq!(cp.last_finalized_slot, 20);
        assert_eq!(cp.last_finalized_batch_id, batch_id(41));
    }

    #[test]
    fn test_guarantee_level_recovered_is_recovery_pending_not_delivered() {
        // FIND-019: Recovered must NOT report Delivered — it is pending re-delivery.
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(45), 1, 5);
        store.transition(&batch_id(45), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(45), FinalityState::QuorumReached, "b").unwrap();
        store.transition(&batch_id(45), FinalityState::Finalized, "c").unwrap();
        store.transition(&batch_id(45), FinalityState::RuntimeDelivered, "d").unwrap();
        store.transition(&batch_id(45), FinalityState::RuntimeRejected, "e").unwrap();
        store.transition(&batch_id(45), FinalityState::Recovered, "f").unwrap();
        assert_eq!(
            store.guarantee_level(&batch_id(45)),
            Some(FinalityGuaranteeLevel::RecoveryPending),
            "Recovered state must map to RecoveryPending, not Delivered"
        );
    }

    #[test]
    fn test_recovered_can_reenter_runtime_delivered() {
        let mut store = FinalityStore::new();
        store.init_batch(batch_id(50), 1, 1);
        store.transition(&batch_id(50), FinalityState::Attested, "a").unwrap();
        store.transition(&batch_id(50), FinalityState::QuorumReached, "b").unwrap();
        store.transition(&batch_id(50), FinalityState::Finalized, "c").unwrap();
        store.transition(&batch_id(50), FinalityState::RuntimeDelivered, "d").unwrap();
        store.transition(&batch_id(50), FinalityState::RuntimeRejected, "e").unwrap();
        store.transition(&batch_id(50), FinalityState::Recovered, "f").unwrap();
        store.transition(&batch_id(50), FinalityState::RuntimeDelivered, "g").unwrap();
        assert_eq!(
            store.get(&batch_id(50)).unwrap().current_state,
            FinalityState::RuntimeDelivered
        );
    }

    #[test]
    fn test_transition_record_is_stored() {
        let mut status = BatchFinalityStatus::new(batch_id(60), 1, 1);
        status.transition(FinalityState::Attested, "first vote").unwrap();
        assert_eq!(status.transitions.len(), 1);
        assert_eq!(status.transitions[0].reason, "first vote");
        assert_eq!(status.transitions[0].from, FinalityState::Proposed);
        assert_eq!(status.transitions[0].to, FinalityState::Attested);
    }

    #[test]
    fn test_can_transition_to_disputed_from_finalized() {
        assert!(FinalityState::Finalized
            .can_transition_to(&FinalityState::DisputedPlaceholder));
    }

    #[test]
    fn test_finality_checkpoint_hash_deterministic() {
        let cp1 = FinalityCheckpoint::compute(5, batch_id(1), 42);
        let cp2 = FinalityCheckpoint::compute(5, batch_id(1), 42);
        assert_eq!(cp1.checkpoint_hash, cp2.checkpoint_hash);
    }
}
