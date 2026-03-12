use std::collections::BTreeMap;
use crate::validation::validator::ValidatedSubmission;
use crate::config::policy::PoSeqPolicy;
use crate::errors::BatchingError;
use crate::attestation::record::BatchAttestation;

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchHeader {
    pub batch_id: [u8; 32],
    pub height: u64,
    pub parent_batch_id: Option<[u8; 32]>,
    pub submission_count: usize,
    pub payload_root: [u8; 32],    // SHA256 of sorted submission_ids
    pub policy_version: u32,
    pub epoch: u64,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchMetadata {
    pub ordering_policy_version: u32,
    pub class_counts: BTreeMap<String, usize>,
    pub total_payload_bytes: usize,
    pub sequencer_id: Option<[u8; 32]>,
}

#[derive(Debug, Clone)]
pub struct OrderedBatch {
    pub header: BatchHeader,
    pub ordered_submissions: Vec<ValidatedSubmission>,
    pub metadata: BatchMetadata,
    pub attestation: BatchAttestation,
}

impl OrderedBatch {
    pub fn compute_payload_root(submissions: &[ValidatedSubmission]) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        let mut hasher = Sha256::new();
        // Sort submission_ids before hashing for determinism
        let mut ids: Vec<[u8; 32]> = submissions.iter().map(|s| s.envelope.normalized_id).collect();
        ids.sort();
        for id in &ids {
            hasher.update(id);
        }
        let hash = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&hash);
        root
    }

    pub fn compute_batch_id(header_without_id: &BatchHeader) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        // Hash: height || parent_or_zeros || payload_root || policy_version || epoch
        let mut hasher = Sha256::new();
        hasher.update(&header_without_id.height.to_be_bytes());
        if let Some(parent) = header_without_id.parent_batch_id {
            hasher.update(&parent);
        } else {
            hasher.update(&[0u8; 32]);
        }
        hasher.update(&header_without_id.payload_root);
        hasher.update(&header_without_id.policy_version.to_be_bytes());
        hasher.update(&header_without_id.epoch.to_be_bytes());
        let hash = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&hash);
        id
    }
}

pub struct BatchBuilder {
    pub policy: PoSeqPolicy,
}

impl BatchBuilder {
    pub fn new(policy: PoSeqPolicy) -> Self {
        BatchBuilder { policy }
    }

    /// Build an OrderedBatch from a canonically-ordered list of ValidatedSubmissions.
    pub fn build(
        &self,
        ordered: Vec<ValidatedSubmission>,
        height: u64,
        parent_batch_id: Option<[u8; 32]>,
        epoch: u64,
        sequencer_id: Option<[u8; 32]>,
    ) -> Result<OrderedBatch, BatchingError> {
        if ordered.is_empty() {
            return Err(BatchingError::EmptyOrderedSet);
        }
        if ordered.len() > self.policy.batch.max_submissions_per_batch {
            return Err(BatchingError::BatchSizeExceeded {
                count: ordered.len(),
                max: self.policy.batch.max_submissions_per_batch,
            });
        }

        let payload_root = OrderedBatch::compute_payload_root(&ordered);

        // Count by class
        let mut class_counts: BTreeMap<String, usize> = BTreeMap::new();
        let mut total_payload_bytes = 0usize;
        for sub in &ordered {
            let class_str = format!("{:?}", sub.normalized_class);
            *class_counts.entry(class_str).or_insert(0) += 1;
            total_payload_bytes += sub.payload_size;
        }

        let mut header = BatchHeader {
            batch_id: [0u8; 32],  // computed below
            height,
            parent_batch_id,
            submission_count: ordered.len(),
            payload_root,
            policy_version: self.policy.version,
            epoch,
        };
        header.batch_id = OrderedBatch::compute_batch_id(&header);

        let metadata = BatchMetadata {
            ordering_policy_version: self.policy.version,
            class_counts,
            total_payload_bytes,
            sequencer_id,
        };

        let attestation = crate::attestation::record::BatchAttestation::placeholder(header.batch_id);

        Ok(OrderedBatch {
            header,
            ordered_submissions: ordered,
            metadata,
            attestation,
        })
    }
}
