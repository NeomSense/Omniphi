use crate::commitment::hash::BatchCommitment;
use crate::attestations::collector::AttestationQuorumResult;

/// Export/delivery status of a finalized batch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExportStatus {
    Pending,
    Delivered,
    Acknowledged,
    Rejected,
    Failed,
}

/// Receipt issued when a batch is finalized.
#[derive(Debug, Clone)]
pub struct FinalizationReceipt {
    pub batch_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub finalized_at_height: u64,
    pub quorum_summary: AttestationQuorumResult,
    pub commitment: BatchCommitment,
    pub export_status: ExportStatus,
}

/// Receipt issued when a batch is delivered to (and acked by) the runtime.
#[derive(Debug, Clone)]
pub struct DeliveryReceipt {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub attempt_count: u32,
    pub acked: bool,
    pub accepted: bool,
    pub rejection_reason: Option<String>,
}

/// Full audit trail for a batch from proposal through delivery.
#[derive(Debug, Clone)]
pub struct BatchLifecycleAuditRecord {
    pub batch_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub finalization_receipt: FinalizationReceipt,
    pub delivery_receipt: Option<DeliveryReceipt>,
    pub incident_ids: Vec<[u8; 32]>,
}
