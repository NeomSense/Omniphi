#![allow(dead_code)]

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ---------------------------------------------------------------------------
// RecoveryCheckpoint
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RecoveryCheckpoint {
    pub checkpoint_id: [u8; 32],
    pub epoch: u64,
    pub last_slot: u64,
    pub last_finalized_batch_id: [u8; 32],
    pub finality_store_hash: [u8; 32],
    pub committee_hash: [u8; 32],
    pub bridge_state_hash: [u8; 32],
    pub checkpoint_hash: [u8; 32],
}

impl RecoveryCheckpoint {
    pub fn compute(
        epoch: u64,
        last_slot: u64,
        last_batch: [u8; 32],
        finality_hash: [u8; 32],
        committee_hash: [u8; 32],
        bridge_hash: [u8; 32],
    ) -> Self {
        let checkpoint_hash = Self::compute_hash(
            epoch,
            last_slot,
            last_batch,
            finality_hash,
            committee_hash,
            bridge_hash,
        );
        // Use checkpoint_hash as checkpoint_id too (could be different in practice)
        RecoveryCheckpoint {
            checkpoint_id: checkpoint_hash,
            epoch,
            last_slot,
            last_finalized_batch_id: last_batch,
            finality_store_hash: finality_hash,
            committee_hash,
            bridge_state_hash: bridge_hash,
            checkpoint_hash,
        }
    }

    fn compute_hash(
        epoch: u64,
        last_slot: u64,
        last_batch: [u8; 32],
        finality_hash: [u8; 32],
        committee_hash: [u8; 32],
        bridge_hash: [u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(epoch.to_le_bytes());
        hasher.update(last_slot.to_le_bytes());
        hasher.update(last_batch);
        hasher.update(finality_hash);
        hasher.update(committee_hash);
        hasher.update(bridge_hash);
        hasher.finalize().into()
    }

    pub fn verify(&self) -> bool {
        let expected = Self::compute_hash(
            self.epoch,
            self.last_slot,
            self.last_finalized_batch_id,
            self.finality_store_hash,
            self.committee_hash,
            self.bridge_state_hash,
        );
        expected == self.checkpoint_hash
    }
}

// ---------------------------------------------------------------------------
// RecoveredStateSummary
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RecoveredStateSummary {
    pub checkpoint_used: RecoveryCheckpoint,
    pub batches_replayed: u64,
    pub finality_states_restored: usize,
    pub bridge_records_restored: usize,
    pub recovery_success: bool,
    pub inconsistencies: Vec<String>,
}

// ---------------------------------------------------------------------------
// StateSyncRequest / Response
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct StateSyncRequest {
    pub requesting_node: [u8; 32],
    pub from_epoch: u64,
    pub from_slot: u64,
    pub known_checkpoint_id: Option<[u8; 32]>,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct StateSyncResponse {
    pub responding_node: [u8; 32],
    pub checkpoints: Vec<RecoveryCheckpoint>,
    pub available: bool,
}

// ---------------------------------------------------------------------------
// ConsistencyVerificationResult
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ConsistencyVerificationResult {
    pub is_consistent: bool,
    pub mismatches: Vec<String>,
    pub verified_epoch: u64,
}

// ---------------------------------------------------------------------------
// RecoveryError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum RecoveryError {
    CheckpointNotFound(u64),
    CheckpointHashMismatch { epoch: u64 },
    InvalidCheckpoint(String),
    RecoveryFailed(String),
}

// ---------------------------------------------------------------------------
// NodeRecoveryManager
// ---------------------------------------------------------------------------

pub struct NodeRecoveryManager {
    checkpoints: BTreeMap<u64, RecoveryCheckpoint>,
}

impl NodeRecoveryManager {
    pub fn new() -> Self {
        NodeRecoveryManager {
            checkpoints: BTreeMap::new(),
        }
    }

    pub fn store_checkpoint(&mut self, cp: RecoveryCheckpoint) -> Result<(), RecoveryError> {
        if !cp.verify() {
            return Err(RecoveryError::CheckpointHashMismatch { epoch: cp.epoch });
        }
        self.checkpoints.insert(cp.epoch, cp);
        Ok(())
    }

    pub fn latest_checkpoint(&self) -> Option<&RecoveryCheckpoint> {
        self.checkpoints.values().next_back()
    }

    pub fn checkpoint_for_epoch(&self, epoch: u64) -> Option<&RecoveryCheckpoint> {
        self.checkpoints.get(&epoch)
    }

    pub fn verify_consistency(&self, epoch: u64) -> ConsistencyVerificationResult {
        match self.checkpoints.get(&epoch) {
            None => ConsistencyVerificationResult {
                is_consistent: false,
                mismatches: vec![format!("No checkpoint for epoch {}", epoch)],
                verified_epoch: epoch,
            },
            Some(cp) => {
                let mut mismatches = Vec::new();
                if !cp.verify() {
                    mismatches.push(format!("Checkpoint hash mismatch at epoch {}", epoch));
                }
                ConsistencyVerificationResult {
                    is_consistent: mismatches.is_empty(),
                    mismatches,
                    verified_epoch: epoch,
                }
            }
        }
    }

    pub fn simulate_recovery(
        &self,
        checkpoint: &RecoveryCheckpoint,
        batches_since: u64,
    ) -> RecoveredStateSummary {
        let valid = checkpoint.verify();
        RecoveredStateSummary {
            checkpoint_used: checkpoint.clone(),
            batches_replayed: batches_since,
            finality_states_restored: batches_since as usize,
            bridge_records_restored: (batches_since / 2) as usize,
            recovery_success: valid,
            inconsistencies: if valid {
                Vec::new()
            } else {
                vec!["checkpoint hash invalid".into()]
            },
        }
    }
}

impl Default for NodeRecoveryManager {
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

    fn make_checkpoint(epoch: u64) -> RecoveryCheckpoint {
        RecoveryCheckpoint::compute(
            epoch,
            epoch * 100,
            [epoch as u8; 32],
            [0xAAu8; 32],
            [0xBBu8; 32],
            [0xCCu8; 32],
        )
    }

    #[test]
    fn test_checkpoint_verify_passes() {
        let cp = make_checkpoint(1);
        assert!(cp.verify());
    }

    #[test]
    fn test_checkpoint_tampered_fails_verify() {
        let mut cp = make_checkpoint(2);
        cp.epoch = 99; // tamper
        assert!(!cp.verify());
    }

    #[test]
    fn test_store_valid_checkpoint() {
        let mut mgr = NodeRecoveryManager::new();
        let cp = make_checkpoint(1);
        mgr.store_checkpoint(cp).unwrap();
        assert!(mgr.checkpoint_for_epoch(1).is_some());
    }

    #[test]
    fn test_store_invalid_checkpoint_fails() {
        let mut mgr = NodeRecoveryManager::new();
        let mut cp = make_checkpoint(3);
        cp.epoch = 99; // tamper
        let result = mgr.store_checkpoint(cp);
        assert!(matches!(result, Err(RecoveryError::CheckpointHashMismatch { .. })));
    }

    #[test]
    fn test_latest_checkpoint() {
        let mut mgr = NodeRecoveryManager::new();
        mgr.store_checkpoint(make_checkpoint(1)).unwrap();
        mgr.store_checkpoint(make_checkpoint(2)).unwrap();
        mgr.store_checkpoint(make_checkpoint(3)).unwrap();
        assert_eq!(mgr.latest_checkpoint().unwrap().epoch, 3);
    }

    #[test]
    fn test_verify_consistency_no_checkpoint() {
        let mgr = NodeRecoveryManager::new();
        let result = mgr.verify_consistency(5);
        assert!(!result.is_consistent);
        assert!(!result.mismatches.is_empty());
    }

    #[test]
    fn test_verify_consistency_valid() {
        let mut mgr = NodeRecoveryManager::new();
        mgr.store_checkpoint(make_checkpoint(4)).unwrap();
        let result = mgr.verify_consistency(4);
        assert!(result.is_consistent);
        assert!(result.mismatches.is_empty());
    }

    #[test]
    fn test_simulate_recovery_success() {
        let mgr = NodeRecoveryManager::new();
        let cp = make_checkpoint(5);
        let summary = mgr.simulate_recovery(&cp, 50);
        assert!(summary.recovery_success);
        assert_eq!(summary.batches_replayed, 50);
    }

    #[test]
    fn test_checkpoint_deterministic() {
        let cp1 = make_checkpoint(10);
        let cp2 = make_checkpoint(10);
        assert_eq!(cp1.checkpoint_hash, cp2.checkpoint_hash);
    }

    #[test]
    fn test_checkpoint_id_equals_hash() {
        let cp = make_checkpoint(7);
        assert_eq!(cp.checkpoint_id, cp.checkpoint_hash);
    }
}
