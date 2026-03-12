use crate::batching::builder::OrderedBatch;
use crate::errors::BridgeError;

/// The runtime-facing envelope exported by PoSeq.
/// This mirrors the structure expected by runtime's PoSeqRuntime / PoSeqCRXBridge.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RuntimeBatchEnvelope {
    pub batch_id: [u8; 32],
    pub height: u64,
    pub epoch: u64,
    pub sequence_number: u64,
    /// Ordered payload hashes — the runtime uses these to look up actual intent/goal objects
    pub ordered_payload_hashes: Vec<[u8; 32]>,
    /// Ordered sender list (parallel to ordered_payload_hashes)
    pub ordered_senders: Vec<[u8; 32]>,
    /// Ordered submission_ids for traceability
    pub ordered_submission_ids: Vec<[u8; 32]>,
    /// Policy version used for ordering
    pub policy_version: u32,
    /// Sequencing metadata (class distribution, etc.)
    pub metadata_json: String,
    /// Attestation placeholder
    pub attestation_placeholder: Vec<u8>,
}

/// Acknowledgment from runtime after processing an envelope.
#[derive(Debug, Clone)]
pub struct RuntimeBatchAck {
    pub batch_id: [u8; 32],
    pub success: bool,
    pub processed_count: usize,
    pub failed_count: usize,
    pub error: Option<String>,
}

/// PoSeq-to-runtime export object.
#[derive(Debug, Clone)]
pub struct PoSeqRuntimeExport {
    pub envelope: RuntimeBatchEnvelope,
    pub original_batch_id: [u8; 32],
}

pub struct RuntimeBridge;

impl RuntimeBridge {
    /// Convert an OrderedBatch into a RuntimeBatchEnvelope.
    /// Preserves canonical ordering.
    pub fn export(batch: &OrderedBatch) -> Result<PoSeqRuntimeExport, BridgeError> {
        if batch.ordered_submissions.is_empty() {
            return Err(BridgeError::EmptyBatch);
        }

        let ordered_payload_hashes: Vec<[u8; 32]> = batch.ordered_submissions.iter()
            .map(|s| s.envelope.submission.payload_hash)
            .collect();
        let ordered_senders: Vec<[u8; 32]> = batch.ordered_submissions.iter()
            .map(|s| s.envelope.submission.sender)
            .collect();
        let ordered_submission_ids: Vec<[u8; 32]> = batch.ordered_submissions.iter()
            .map(|s| s.envelope.normalized_id)
            .collect();

        // Serialize metadata to JSON string for portability
        let metadata_json = serde_json::to_string(&batch.metadata.class_counts)
            .unwrap_or_else(|_| "{}".into());

        let envelope = RuntimeBatchEnvelope {
            batch_id: batch.header.batch_id,
            height: batch.header.height,
            epoch: batch.header.epoch,
            sequence_number: batch.header.height,
            ordered_payload_hashes,
            ordered_senders,
            ordered_submission_ids,
            policy_version: batch.header.policy_version,
            metadata_json,
            attestation_placeholder: bincode::serialize(&batch.attestation)
                .unwrap_or_default(),
        };

        Ok(PoSeqRuntimeExport {
            envelope,
            original_batch_id: batch.header.batch_id,
        })
    }

    /// Simulate runtime acknowledgment (in production, this would be a real call)
    pub fn mock_ack(export: &PoSeqRuntimeExport) -> RuntimeBatchAck {
        RuntimeBatchAck {
            batch_id: export.envelope.batch_id,
            success: true,
            processed_count: export.envelope.ordered_payload_hashes.len(),
            failed_count: 0,
            error: None,
        }
    }
}
