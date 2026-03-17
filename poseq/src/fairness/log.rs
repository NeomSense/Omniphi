//! Phase 3 — Fairness logging for censorship accountability.
//!
//! Every bundle decision (included/excluded) is logged with a reason code.
//! The fairness log produces a Merkle root that becomes part of FinalityCommitment.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Reason code explaining why a bundle was included or excluded.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ReasonCode {
    /// Bundle was included in the canonical sequence.
    Included,
    /// Target intent has expired.
    ExpiredIntent,
    /// Reveal did not match commitment hash.
    InvalidReveal,
    /// Bundle failed data availability checks.
    FailedDA,
    /// A higher-scoring bundle was selected for the same intent.
    OrderingPriority,
    /// Target intent was already filled by another bundle.
    DuplicateIntent,
    /// Solver is not active or has insufficient bond.
    SolverInactive,
}

/// A single entry in the fairness log.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FairnessLogEntry {
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    /// Whether this bundle was eligible for inclusion.
    pub eligibility: bool,
    /// Whether this bundle was actually included in the sequence.
    pub inclusion: bool,
    /// Reason for inclusion or exclusion.
    pub reason_code: ReasonCode,
}

impl FairnessLogEntry {
    /// Compute a deterministic hash of this entry.
    pub fn hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.bundle_id);
        hasher.update(self.solver_id);
        hasher.update([self.eligibility as u8]);
        hasher.update([self.inclusion as u8]);
        let reason_bytes = bincode::serialize(&self.reason_code).unwrap_or_default();
        hasher.update(&reason_bytes);
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }
}

/// The fairness log for a single batch window.
#[derive(Debug, Clone, Default)]
pub struct FairnessLog {
    pub(crate) entries: Vec<FairnessLogEntry>,
}

impl FairnessLog {
    pub fn new() -> Self {
        FairnessLog { entries: Vec::new() }
    }

    /// Record a bundle inclusion.
    pub fn record_included(&mut self, bundle_id: [u8; 32], solver_id: [u8; 32]) {
        self.entries.push(FairnessLogEntry {
            bundle_id,
            solver_id,
            eligibility: true,
            inclusion: true,
            reason_code: ReasonCode::Included,
        });
    }

    /// Record a bundle exclusion with reason.
    pub fn record_excluded(
        &mut self,
        bundle_id: [u8; 32],
        solver_id: [u8; 32],
        reason: ReasonCode,
    ) {
        self.entries.push(FairnessLogEntry {
            bundle_id,
            solver_id,
            eligibility: reason == ReasonCode::OrderingPriority, // eligible but lost
            inclusion: false,
            reason_code: reason,
        });
    }

    /// Compute the Merkle root over all fairness entries.
    ///
    /// For simplicity, uses a flat hash: SHA256(entry_hash[0] ‖ ... ‖ entry_hash[n]).
    /// A proper binary Merkle tree can be used for proof generation in production.
    pub fn compute_fairness_root(&self) -> [u8; 32] {
        if self.entries.is_empty() {
            return [0u8; 32]; // empty log → zero root
        }

        let mut hasher = Sha256::new();
        for entry in &self.entries {
            hasher.update(entry.hash());
        }
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// Get all entries.
    pub fn entries(&self) -> &[FairnessLogEntry] {
        &self.entries
    }

    /// Number of entries.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Count of included bundles.
    pub fn included_count(&self) -> usize {
        self.entries.iter().filter(|e| e.inclusion).count()
    }

    /// Count of excluded bundles.
    pub fn excluded_count(&self) -> usize {
        self.entries.iter().filter(|e| !e.inclusion).count()
    }

    /// Get all exclusion entries for a specific solver.
    pub fn exclusions_for_solver(&self, solver_id: &[u8; 32]) -> Vec<&FairnessLogEntry> {
        self.entries.iter()
            .filter(|e| &e.solver_id == solver_id && !e.inclusion)
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_fairness_log_basic() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        log.record_excluded([2u8; 32], [20u8; 32], ReasonCode::OrderingPriority);
        log.record_excluded([3u8; 32], [30u8; 32], ReasonCode::ExpiredIntent);

        assert_eq!(log.len(), 3);
        assert_eq!(log.included_count(), 1);
        assert_eq!(log.excluded_count(), 2);
    }

    #[test]
    fn test_fairness_root_deterministic() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        log.record_excluded([2u8; 32], [20u8; 32], ReasonCode::FailedDA);

        let root1 = log.compute_fairness_root();
        let root2 = log.compute_fairness_root();
        assert_eq!(root1, root2);
        assert_ne!(root1, [0u8; 32]);
    }

    #[test]
    fn test_fairness_root_empty() {
        let log = FairnessLog::new();
        assert_eq!(log.compute_fairness_root(), [0u8; 32]);
    }

    #[test]
    fn test_fairness_root_changes_with_entries() {
        let mut log1 = FairnessLog::new();
        log1.record_included([1u8; 32], [10u8; 32]);

        let mut log2 = FairnessLog::new();
        log2.record_included([2u8; 32], [20u8; 32]);

        assert_ne!(log1.compute_fairness_root(), log2.compute_fairness_root());
    }

    #[test]
    fn test_fairness_entry_hash_deterministic() {
        let entry = FairnessLogEntry {
            bundle_id: [1u8; 32],
            solver_id: [10u8; 32],
            eligibility: true,
            inclusion: true,
            reason_code: ReasonCode::Included,
        };
        assert_eq!(entry.hash(), entry.hash());
    }

    #[test]
    fn test_exclusions_for_solver() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        log.record_excluded([2u8; 32], [10u8; 32], ReasonCode::OrderingPriority);
        log.record_excluded([3u8; 32], [20u8; 32], ReasonCode::SolverInactive);

        let excl = log.exclusions_for_solver(&[10u8; 32]);
        assert_eq!(excl.len(), 1);
        assert_eq!(excl[0].bundle_id, [2u8; 32]);
    }
}
