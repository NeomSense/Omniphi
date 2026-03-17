//! Phase 7 — Settlement bridge connecting runtime outputs to the control chain.
//!
//! Monitors settlement results, packages accountability records, and manages
//! submission to the Cosmos control chain with retry and idempotency.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

use super::result::{ChainSettlementResult, SettlementAnchor};

/// Evidence record to submit alongside settlement.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SettlementEvidence {
    pub evidence_type: SettlementEvidenceType,
    pub offender_id: [u8; 32],
    pub batch_hash: [u8; 32],
    pub epoch: u64,
    pub description: String,
    pub evidence_hash: [u8; 32],
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SettlementEvidenceType {
    DoubleProposal,
    DoubleVote,
    InvalidBatch,
    FairnessViolation,
    Equivocation,
}

/// A submission package for the control chain.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainSubmission {
    pub submission_id: [u8; 32],
    pub anchor: SettlementAnchor,
    pub evidence: Vec<SettlementEvidence>,
    pub checkpoint_metadata: Option<CheckpointMetadata>,
    pub attempt: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CheckpointMetadata {
    pub epoch: u64,
    pub batch_hash: [u8; 32],
    pub post_state_root: [u8; 32],
}

/// Submission status tracking.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubmissionStatus {
    Pending,
    Submitted,
    Confirmed,
    Failed,
}

/// Settlement bridge service.
///
/// Manages the lifecycle of settlement submissions to the control chain:
/// - Packages settlement results into chain submissions
/// - Tracks submission status
/// - Supports retry on failure
/// - Deduplicates submissions by batch_hash
pub struct SettlementBridge {
    /// Pending submissions indexed by batch_hash.
    submissions: BTreeMap<[u8; 32], (ChainSubmission, SubmissionStatus)>,
    /// Already-confirmed batch hashes (prevents duplicate submission).
    confirmed_batches: BTreeSet<[u8; 32]>,
    /// Evidence dedup by evidence_hash.
    submitted_evidence: BTreeSet<[u8; 32]>,
    /// Maximum retry attempts.
    max_retries: u32,
}

impl SettlementBridge {
    pub fn new(max_retries: u32) -> Self {
        SettlementBridge {
            submissions: BTreeMap::new(),
            confirmed_batches: BTreeSet::new(),
            submitted_evidence: BTreeSet::new(),
            max_retries,
        }
    }

    /// Package a settlement result for chain submission.
    ///
    /// Returns None if the batch has already been confirmed (idempotent).
    pub fn prepare_submission(
        &mut self,
        result: &ChainSettlementResult,
        evidence: Vec<SettlementEvidence>,
        checkpoint: Option<CheckpointMetadata>,
    ) -> Option<ChainSubmission> {
        // Idempotency: don't re-submit confirmed batches
        if self.confirmed_batches.contains(&result.batch_hash) {
            return None;
        }

        // Dedup evidence
        let new_evidence: Vec<SettlementEvidence> = evidence.into_iter()
            .filter(|e| !self.submitted_evidence.contains(&e.evidence_hash))
            .collect();

        let anchor = SettlementAnchor::from_result(result);
        let submission_id = Self::compute_submission_id(&result.batch_hash, &anchor.settlement_hash);

        let submission = ChainSubmission {
            submission_id,
            anchor,
            evidence: new_evidence,
            checkpoint_metadata: checkpoint,
            attempt: 1,
        };

        self.submissions.insert(result.batch_hash, (submission.clone(), SubmissionStatus::Pending));
        Some(submission)
    }

    /// Mark a submission as successfully confirmed on-chain.
    pub fn confirm(&mut self, batch_hash: &[u8; 32]) -> bool {
        if let Some((sub, status)) = self.submissions.get_mut(batch_hash) {
            *status = SubmissionStatus::Confirmed;
            self.confirmed_batches.insert(*batch_hash);
            // Mark all evidence as submitted
            for e in &sub.evidence {
                self.submitted_evidence.insert(e.evidence_hash);
            }
            true
        } else {
            false
        }
    }

    /// Mark a submission as failed. Returns true if it should be retried.
    pub fn record_failure(&mut self, batch_hash: &[u8; 32]) -> bool {
        if let Some((sub, status)) = self.submissions.get_mut(batch_hash) {
            if sub.attempt < self.max_retries {
                sub.attempt += 1;
                *status = SubmissionStatus::Pending;
                true // retry
            } else {
                *status = SubmissionStatus::Failed;
                false // give up
            }
        } else {
            false
        }
    }

    /// Get all pending submissions that need to be sent.
    pub fn pending_submissions(&self) -> Vec<&ChainSubmission> {
        self.submissions.values()
            .filter(|(_, status)| *status == SubmissionStatus::Pending)
            .map(|(sub, _)| sub)
            .collect()
    }

    /// Check if a batch has already been submitted and confirmed.
    pub fn is_confirmed(&self, batch_hash: &[u8; 32]) -> bool {
        self.confirmed_batches.contains(batch_hash)
    }

    pub fn pending_count(&self) -> usize {
        self.submissions.values().filter(|(_, s)| *s == SubmissionStatus::Pending).count()
    }

    pub fn confirmed_count(&self) -> usize {
        self.confirmed_batches.len()
    }

    fn compute_submission_id(batch_hash: &[u8; 32], settlement_hash: &[u8; 32]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"SETTLEMENT_SUBMISSION_V1");
        hasher.update(batch_hash);
        hasher.update(settlement_hash);
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::settlement::result::*;

    fn make_result(batch_byte: u8) -> ChainSettlementResult {
        let mut batch_hash = [0u8; 32]; batch_hash[0] = batch_byte;
        ChainSettlementResult {
            batch_hash,
            pre_state_root: [0xAA; 32],
            post_state_root: [0xBB; 32],
            execution_receipt_hash: [0xCC; 32],
            transfer_records: vec![],
            object_updates: vec![],
            solver_rewards: vec![],
            validator_fees: vec![],
            gas_summary: GasSummary { total_gas_used: 5000, total_gas_limit: 10000, per_intent_gas: vec![] },
            settlement_status: SettlementStatus::FullySettled,
            epoch: 1,
            sequence_number: 1,
            settled_count: 3,
            failed_count: 0,
        }
    }

    fn make_evidence(batch_byte: u8) -> SettlementEvidence {
        let mut hash = [0u8; 32]; hash[0] = batch_byte;
        SettlementEvidence {
            evidence_type: SettlementEvidenceType::FairnessViolation,
            offender_id: [0x10; 32],
            batch_hash: { let mut h = [0u8; 32]; h[0] = batch_byte; h },
            epoch: 1,
            description: "test evidence".to_string(),
            evidence_hash: hash,
        }
    }

    #[test]
    fn test_bridge_prepare_and_confirm() {
        let mut bridge = SettlementBridge::new(3);
        let result = make_result(1);

        let sub = bridge.prepare_submission(&result, vec![], None);
        assert!(sub.is_some());
        assert_eq!(bridge.pending_count(), 1);

        bridge.confirm(&result.batch_hash);
        assert_eq!(bridge.confirmed_count(), 1);
        assert_eq!(bridge.pending_count(), 0);
        assert!(bridge.is_confirmed(&result.batch_hash));
    }

    #[test]
    fn test_bridge_idempotent_submission() {
        let mut bridge = SettlementBridge::new(3);
        let result = make_result(1);

        bridge.prepare_submission(&result, vec![], None).unwrap();
        bridge.confirm(&result.batch_hash);

        // Second submission for same batch → None (idempotent)
        assert!(bridge.prepare_submission(&result, vec![], None).is_none());
    }

    #[test]
    fn test_bridge_retry_logic() {
        let mut bridge = SettlementBridge::new(3);
        let result = make_result(1);
        bridge.prepare_submission(&result, vec![], None);

        // First two failures → retry
        assert!(bridge.record_failure(&result.batch_hash));
        assert!(bridge.record_failure(&result.batch_hash));
        // Third failure → no more retries (max_retries = 3, attempt started at 1)
        assert!(!bridge.record_failure(&result.batch_hash));
    }

    #[test]
    fn test_bridge_evidence_dedup() {
        let mut bridge = SettlementBridge::new(3);
        let result1 = make_result(1);
        let evidence = vec![make_evidence(1)];

        let sub1 = bridge.prepare_submission(&result1, evidence.clone(), None).unwrap();
        assert_eq!(sub1.evidence.len(), 1);
        bridge.confirm(&result1.batch_hash);

        // Same evidence for different batch → deduped
        let result2 = make_result(2);
        let sub2 = bridge.prepare_submission(&result2, evidence, None).unwrap();
        assert_eq!(sub2.evidence.len(), 0); // already submitted
    }

    #[test]
    fn test_bridge_with_checkpoint() {
        let mut bridge = SettlementBridge::new(3);
        let result = make_result(1);
        let checkpoint = CheckpointMetadata {
            epoch: 1,
            batch_hash: result.batch_hash,
            post_state_root: result.post_state_root,
        };

        let sub = bridge.prepare_submission(&result, vec![], Some(checkpoint)).unwrap();
        assert!(sub.checkpoint_metadata.is_some());
    }

    #[test]
    fn test_pending_submissions() {
        let mut bridge = SettlementBridge::new(3);
        bridge.prepare_submission(&make_result(1), vec![], None);
        bridge.prepare_submission(&make_result(2), vec![], None);

        let pending = bridge.pending_submissions();
        assert_eq!(pending.len(), 2);
    }
}
