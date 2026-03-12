use std::collections::BTreeMap;
use crate::validation::validator::ValidatedSubmission;

#[derive(Debug, Clone)]
pub enum SubmissionDecision {
    Accepted,
    Rejected(String),
}

#[derive(Debug, Clone)]
pub struct SubmissionDecisionRecord {
    pub submission_id: [u8; 32],
    pub decision: SubmissionDecision,
    pub reason: Option<String>,
}

#[derive(Debug, Clone)]
pub struct SequencingReceipt {
    pub receipt_id: [u8; 32],
    pub batch_id: [u8; 32],
    pub height: u64,
    pub accepted_count: usize,
    pub rejected_count: usize,
    pub decisions: Vec<SubmissionDecisionRecord>,
    pub ordered_ids: Vec<[u8; 32]>,
    pub policy_version: u32,
    pub receipt_hash: [u8; 32],
}

impl SequencingReceipt {
    pub fn build(
        batch_id: [u8; 32],
        height: u64,
        accepted: &[ValidatedSubmission],
        rejected: &[([u8; 32], String)],
        policy_version: u32,
    ) -> Self {
        use sha2::{Sha256, Digest};

        let mut decisions = Vec::new();
        let mut ordered_ids = Vec::new();

        for sub in accepted {
            decisions.push(SubmissionDecisionRecord {
                submission_id: sub.envelope.normalized_id,
                decision: SubmissionDecision::Accepted,
                reason: None,
            });
            ordered_ids.push(sub.envelope.normalized_id);
        }
        for (id, reason) in rejected {
            decisions.push(SubmissionDecisionRecord {
                submission_id: *id,
                decision: SubmissionDecision::Rejected(reason.clone()),
                reason: Some(reason.clone()),
            });
        }

        let accepted_count = accepted.len();
        let rejected_count = rejected.len();

        // receipt_id = SHA256(batch_id || height || accepted_count || rejected_count)
        let mut hasher = Sha256::new();
        hasher.update(&batch_id);
        hasher.update(&height.to_be_bytes());
        hasher.update(&accepted_count.to_be_bytes());
        hasher.update(&rejected_count.to_be_bytes());
        let hash = hasher.finalize();
        let mut receipt_id = [0u8; 32];
        receipt_id.copy_from_slice(&hash);

        let mut hasher2 = Sha256::new();
        hasher2.update(&receipt_id);
        hasher2.update(&policy_version.to_be_bytes());
        let hash2 = hasher2.finalize();
        let mut receipt_hash = [0u8; 32];
        receipt_hash.copy_from_slice(&hash2);

        SequencingReceipt {
            receipt_id,
            batch_id,
            height,
            accepted_count,
            rejected_count,
            decisions,
            ordered_ids,
            policy_version,
            receipt_hash,
        }
    }
}

#[derive(Debug, Clone)]
pub struct BatchAuditRecord {
    pub batch_id: [u8; 32],
    pub height: u64,
    pub payload_root: [u8; 32],
    pub policy_version: u32,
    pub receipt: SequencingReceipt,
    pub metadata: BTreeMap<String, String>,
}
