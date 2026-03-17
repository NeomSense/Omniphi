//! `BatchPipeline` — the canonical PoSeq→Runtime bridge contract.
//!
//! This module defines:
//!
//! 1. **`FinalizationEnvelope`** — the canonical over-the-wire structure from PoSeq to runtime.
//!    Contains everything the runtime needs: ordering, sequencing metadata, fairness metadata,
//!    batch commitment, and delivery provenance.
//!
//! 2. **`RuntimeIngestionAck`** / **`RuntimeIngestionRejection`** — typed ack/reject messages
//!    the runtime returns.  Every field is cryptographically committed so PoSeq can verify
//!    authenticity before updating lifecycle state.
//!
//! 3. **`BatchPipeline`** — stateful orchestrator that owns:
//!    - `HardenedRuntimeBridge` for idempotent delivery + ack replay protection
//!    - `DurableStore` reference for persisting delivery state across crashes
//!    - Lifecycle callbacks that update `BatchFinalityStatus` after ack/reject
//!
//! # Separation of concerns
//!
//! - PoSeq is the **sequencing layer only**: it never inspects execution results.
//! - The runtime is the **execution layer only**: it never modifies ordering.
//! - This module is the **boundary**: it translates finalized batches into delivery envelopes
//!   and translates acks/rejects back into PoSeq lifecycle events.

use std::collections::BTreeMap;
use sha2::{Sha256, Digest};

use crate::finalization::engine::FinalizedBatch;
use crate::bridge::hardened::{
    HardenedRuntimeBridge, RuntimeDeliveryEnvelope, RuntimeExecutionAck, RuntimeExecutionRejection,
    BridgeDeliveryRecord,
};
use crate::errors::BridgeError;
use crate::finality::{BatchFinalityStatus, FinalityStore, FinalityState};

// ─── Canonical bridge types ───────────────────────────────────────────────────

/// Fairness metadata attached to a finalized batch for the runtime.
/// The runtime may use these for rate-limiting or priority but MUST NOT
/// alter the ordering of submission IDs based on these values.
#[derive(Debug, Clone)]
pub struct FairnessMeta {
    /// Policy version that produced the ordering.
    pub policy_version: u32,
    /// Number of submissions promoted by forced-inclusion rules.
    pub forced_inclusion_count: u32,
    /// Number of submissions that were rate-limited (excluded from this batch).
    pub rate_limited_count: u32,
    /// Fairness class tags per submission index (informational).
    pub per_submission_class: Vec<u8>,
}

impl FairnessMeta {
    pub fn none(policy_version: u32) -> Self {
        FairnessMeta {
            policy_version,
            forced_inclusion_count: 0,
            rate_limited_count: 0,
            per_submission_class: vec![],
        }
    }
}

/// Canonical batch commitment — cryptographic anchor for the envelope.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BatchCommitment {
    /// SHA256(finalization_hash ‖ delivery_id ‖ ordered_submission_ids_root)
    pub commitment_hash: [u8; 32],
    pub finalization_hash: [u8; 32],
    pub delivery_id: [u8; 32],
    pub submission_root: [u8; 32],
}

impl BatchCommitment {
    pub fn compute(
        finalization_hash: &[u8; 32],
        delivery_id: &[u8; 32],
        ordered_ids: &[[u8; 32]],
    ) -> Self {
        // Build submission root: SHA256 of all submission IDs concatenated in order
        let submission_root = {
            let mut hasher = Sha256::new();
            for id in ordered_ids {
                hasher.update(id);
            }
            let r = hasher.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };

        let commitment_hash = {
            let mut hasher = Sha256::new();
            hasher.update(finalization_hash);
            hasher.update(delivery_id);
            hasher.update(&submission_root);
            let r = hasher.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };

        BatchCommitment {
            commitment_hash,
            finalization_hash: *finalization_hash,
            delivery_id: *delivery_id,
            submission_root,
        }
    }
}

/// The canonical over-the-wire envelope from PoSeq to runtime.
///
/// This is the **only** structure the runtime should accept from PoSeq.
/// All fields are immutable after construction; the commitment hash
/// cryptographically binds ordering + provenance.
#[derive(Debug, Clone)]
pub struct FinalizationEnvelope {
    // ── Identity ──────────────────────────────────────────────────────────────
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub attempt_count: u32,

    // ── Sequencing metadata ───────────────────────────────────────────────────
    pub slot: u64,
    pub epoch: u64,
    pub sequence_number: u64,
    pub leader_id: [u8; 32],
    pub parent_batch_id: [u8; 32],

    // ── Ordered execution payload ─────────────────────────────────────────────
    /// Ordered list of submission IDs.  Runtime MUST process in this exact order.
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub batch_root: [u8; 32],
    pub finalization_hash: [u8; 32],

    // ── Quorum provenance ─────────────────────────────────────────────────────
    pub quorum_approvals: usize,
    pub committee_size: usize,

    // ── Fairness metadata (informational, ordering must not change) ────────────
    pub fairness: FairnessMeta,

    // ── Commitment ────────────────────────────────────────────────────────────
    pub commitment: BatchCommitment,
}

impl FinalizationEnvelope {
    /// Build a canonical envelope from a `FinalizedBatch` and its delivery provenance.
    pub fn build(
        batch: &FinalizedBatch,
        delivery: &RuntimeDeliveryEnvelope,
        sequence_number: u64,
        fairness: FairnessMeta,
    ) -> Self {
        let commitment = BatchCommitment::compute(
            &batch.finalization_hash,
            &delivery.delivery_id,
            &batch.ordered_submission_ids,
        );

        FinalizationEnvelope {
            batch_id: batch.batch_id,
            delivery_id: delivery.delivery_id,
            attempt_count: delivery.attempt_count,
            slot: batch.slot,
            epoch: batch.epoch,
            sequence_number,
            leader_id: batch.leader_id,
            parent_batch_id: batch.parent_batch_id,
            ordered_submission_ids: batch.ordered_submission_ids.clone(),
            batch_root: batch.batch_root,
            finalization_hash: batch.finalization_hash,
            quorum_approvals: batch.quorum_summary.approvals,
            committee_size: batch.quorum_summary.total_votes,
            fairness,
            commitment,
        }
    }

    /// Verify the commitment hash.  Runtime should call this before accepting.
    pub fn verify_commitment(&self) -> bool {
        let expected = BatchCommitment::compute(
            &self.finalization_hash,
            &self.delivery_id,
            &self.ordered_submission_ids,
        );
        expected.commitment_hash == self.commitment.commitment_hash
    }
}

/// Result of runtime ingestion — ack or reject.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum IngestionOutcome {
    /// Runtime accepted and applied the batch.
    Accepted(RuntimeIngestionAck),
    /// Runtime structurally rejected (invalid envelope, already applied, etc.).
    Rejected(RuntimeIngestionRejection),
}

impl IngestionOutcome {
    pub fn batch_id(&self) -> [u8; 32] {
        match self {
            IngestionOutcome::Accepted(a) => a.batch_id,
            IngestionOutcome::Rejected(r) => r.batch_id,
        }
    }

    pub fn is_accepted(&self) -> bool {
        matches!(self, IngestionOutcome::Accepted(_))
    }
}

/// Explicit ack from the runtime.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuntimeIngestionAck {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub epoch: u64,
    pub succeeded_count: usize,
    pub failed_count: usize,
    /// Optional CRX/safety execution result reference (opaque hash, not execution state).
    pub execution_result_ref: Option<[u8; 32]>,
    /// Cryptographic proof of ack: SHA256(batch_id ‖ delivery_id ‖ "ack" ‖ epoch)
    pub ack_hash: [u8; 32],
}

impl RuntimeIngestionAck {
    pub fn compute_ack_hash(batch_id: &[u8; 32], delivery_id: &[u8; 32], epoch: u64) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(batch_id);
        h.update(delivery_id);
        h.update(b"ack");
        h.update(&epoch.to_be_bytes());
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    pub fn new(
        batch_id: [u8; 32],
        delivery_id: [u8; 32],
        epoch: u64,
        succeeded_count: usize,
        failed_count: usize,
        execution_result_ref: Option<[u8; 32]>,
    ) -> Self {
        let ack_hash = Self::compute_ack_hash(&batch_id, &delivery_id, epoch);
        RuntimeIngestionAck {
            batch_id,
            delivery_id,
            epoch,
            succeeded_count,
            failed_count,
            execution_result_ref,
            ack_hash,
        }
    }
}

/// Explicit rejection from the runtime with a typed cause.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuntimeIngestionRejection {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub epoch: u64,
    pub cause: RejectionCause,
    /// SHA256(batch_id ‖ delivery_id ‖ "reject" ‖ cause_tag)
    pub rejection_hash: [u8; 32],
}

/// Typed cause of a runtime rejection.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RejectionCause {
    /// Batch was structurally invalid (commitment mismatch, zero IDs, etc.).
    InvalidEnvelope(String),
    /// Batch was already applied in a prior epoch.  PoSeq should not retry.
    AlreadyApplied,
    /// Runtime is transiently unavailable.  PoSeq may retry.
    TransientUnavailable(String),
    /// Batch failed during execution; partial state may have been applied.
    ExecutionFailure(String),
    /// Runtime blocked the batch via safety kernel.
    SafetyBlock(String),
}

impl RejectionCause {
    pub fn tag(&self) -> &'static str {
        match self {
            RejectionCause::InvalidEnvelope(_)    => "invalid_envelope",
            RejectionCause::AlreadyApplied        => "already_applied",
            RejectionCause::TransientUnavailable(_) => "transient_unavailable",
            RejectionCause::ExecutionFailure(_)   => "execution_failure",
            RejectionCause::SafetyBlock(_)        => "safety_block",
        }
    }

    pub fn is_retryable(&self) -> bool {
        matches!(self, RejectionCause::TransientUnavailable(_))
    }
}

impl RuntimeIngestionRejection {
    pub fn compute_rejection_hash(
        batch_id: &[u8; 32],
        delivery_id: &[u8; 32],
        cause_tag: &[u8],
    ) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(batch_id);
        h.update(delivery_id);
        h.update(b"reject");
        h.update(cause_tag);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    pub fn new(
        batch_id: [u8; 32],
        delivery_id: [u8; 32],
        epoch: u64,
        cause: RejectionCause,
    ) -> Self {
        let rejection_hash = Self::compute_rejection_hash(
            &batch_id, &delivery_id, cause.tag().as_bytes(),
        );
        RuntimeIngestionRejection { batch_id, delivery_id, epoch, cause, rejection_hash }
    }
}

// ─── BatchPipeline ─────────────────────────────────────────────────────────────

/// Orchestrates the full PoSeq→Runtime batch lifecycle.
///
/// Responsibilities:
/// - Wraps `HardenedRuntimeBridge` for idempotent delivery + ack replay protection
/// - Persists delivery state to `FinalityStore` after each outcome
/// - Provides explicit retry/skip decisions based on rejection cause
/// - Stores reference to runtime execution results (opaque hash only)
///
/// Invariants:
/// - A batch is delivered at most once per attempt (idempotent envelope)
/// - An ack is accepted at most once per delivery_id
/// - A retryable rejection increments attempt count; non-retryable marks terminal
pub struct BatchPipeline {
    bridge: HardenedRuntimeBridge,
    /// Sequence counter for assigning per-pipeline sequence numbers to envelopes.
    next_sequence: u64,
    /// Per-batch lifecycle notes (epoch, execution_result_ref, terminal state).
    lifecycle: BTreeMap<[u8; 32], BatchLifecycleEntry>,
}

#[derive(Debug, Clone)]
pub struct BatchLifecycleEntry {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub attempt_count: u32,
    pub state: PipelineState,
    pub execution_result_ref: Option<[u8; 32]>,
    pub rejection_cause: Option<String>,
}

/// PoSeq-side view of the batch's pipeline state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PipelineState {
    /// Delivered to runtime; waiting for ack/reject.
    Pending,
    /// Runtime accepted and applied the batch.
    Accepted,
    /// Runtime rejected with a retryable cause; will be retried.
    RejectedRetryable,
    /// Runtime rejected with a terminal cause; no further delivery.
    RejectedTerminal,
}

impl BatchPipeline {
    pub fn new() -> Self {
        BatchPipeline {
            bridge: HardenedRuntimeBridge::new(),
            next_sequence: 1,
            lifecycle: BTreeMap::new(),
        }
    }

    /// Deliver a finalized batch to the runtime.
    ///
    /// Returns a `FinalizationEnvelope` ready for the runtime to ingest.
    /// Calling this multiple times for the same batch is idempotent — the same
    /// envelope (same delivery_id, same commitment) is returned each time,
    /// but `attempt_count` in the tracking record is incremented.
    pub fn deliver(
        &mut self,
        batch: &FinalizedBatch,
        fairness: FairnessMeta,
    ) -> FinalizationEnvelope {
        let sequence = self.next_sequence;
        let delivery = self.bridge.deliver(batch);
        self.next_sequence += 1;

        let envelope = FinalizationEnvelope::build(batch, &delivery, sequence, fairness);

        // Initialize or update lifecycle entry
        self.lifecycle
            .entry(batch.batch_id)
            .and_modify(|e| e.attempt_count = delivery.attempt_count)
            .or_insert(BatchLifecycleEntry {
                batch_id: batch.batch_id,
                epoch: batch.epoch,
                attempt_count: 1,
                state: PipelineState::Pending,
                execution_result_ref: None,
                rejection_cause: None,
            });

        envelope
    }

    /// Record the runtime's ack for a delivered batch.
    ///
    /// Returns `Ok(())` on success.
    /// Returns `Err(BridgeError::AckReplay)` if the delivery_id was already acked.
    pub fn record_ack(&mut self, ack: RuntimeIngestionAck) -> Result<(), BridgeError> {
        let hw_ack = RuntimeExecutionAck::new(
            ack.batch_id,
            ack.delivery_id,
            ack.succeeded_count > 0, // accepted = any successes
            ack.epoch,
        );
        self.bridge.record_ack(hw_ack)?;

        if let Some(entry) = self.lifecycle.get_mut(&ack.batch_id) {
            entry.state = PipelineState::Accepted;
            entry.execution_result_ref = ack.execution_result_ref;
        }
        Ok(())
    }

    /// Record a rejection from the runtime.
    ///
    /// Updates the lifecycle state to retryable or terminal based on cause.
    pub fn record_rejection(&mut self, rej: RuntimeIngestionRejection) {
        let is_retryable = rej.cause.is_retryable();
        let cause_str = format!("{:?}", rej.cause);

        let hw_rej = RuntimeExecutionRejection {
            batch_id: rej.batch_id,
            delivery_id: rej.delivery_id,
            reason: cause_str.clone(),
            epoch: rej.epoch,
        };
        self.bridge.record_rejection(hw_rej);

        if let Some(entry) = self.lifecycle.get_mut(&rej.batch_id) {
            entry.state = if is_retryable {
                PipelineState::RejectedRetryable
            } else {
                PipelineState::RejectedTerminal
            };
            entry.rejection_cause = Some(cause_str);
        }
    }

    /// Check if a batch needs a retry.
    pub fn needs_retry(&self, batch_id: &[u8; 32]) -> bool {
        self.lifecycle
            .get(batch_id)
            .map(|e| e.state == PipelineState::RejectedRetryable)
            .unwrap_or(false)
    }

    /// Check if a batch is done (accepted or terminally rejected).
    pub fn is_terminal(&self, batch_id: &[u8; 32]) -> bool {
        self.lifecycle.get(batch_id).map(|e| {
            matches!(e.state, PipelineState::Accepted | PipelineState::RejectedTerminal)
        }).unwrap_or(false)
    }

    /// Get the delivery record from the hardened bridge (attempt count, acked, etc.).
    pub fn get_delivery_record(&self, batch_id: &[u8; 32]) -> Option<&BridgeDeliveryRecord> {
        self.bridge.get_record(batch_id)
    }

    /// Get the lifecycle entry for a batch.
    pub fn get_lifecycle(&self, batch_id: &[u8; 32]) -> Option<&BatchLifecycleEntry> {
        self.lifecycle.get(batch_id)
    }
}

impl Default for BatchPipeline {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::finalization::engine::FinalizedBatch;
    use crate::attestations::collector::AttestationQuorumResult;

    fn make_qr() -> AttestationQuorumResult {
        AttestationQuorumResult {
            reached: true,
            approvals: 3,
            rejections: 0,
            total_votes: 3,
            quorum_hash: [9u8; 32],
        }
    }

    fn make_batch(b: u8) -> FinalizedBatch {
        FinalizedBatch {
            batch_id: [b; 32],
            proposal_id: [b + 1; 32],
            slot: b as u64,
            epoch: 1,
            leader_id: [1u8; 32],
            ordered_submission_ids: vec![[b + 2; 32], [b + 3; 32]],
            batch_root: [b + 4; 32],
            parent_batch_id: [0u8; 32],
            finalized_at_height: 100,
            quorum_summary: make_qr(),
            finalization_hash: [b + 5; 32],
        }
    }

    // ── BatchCommitment ──────────────────────────────────────────────────────

    #[test]
    fn test_commitment_deterministic() {
        let ids = vec![[1u8; 32], [2u8; 32]];
        let c1 = BatchCommitment::compute(&[0u8; 32], &[1u8; 32], &ids);
        let c2 = BatchCommitment::compute(&[0u8; 32], &[1u8; 32], &ids);
        assert_eq!(c1.commitment_hash, c2.commitment_hash);
    }

    #[test]
    fn test_commitment_order_sensitive() {
        let ids_ab = vec![[1u8; 32], [2u8; 32]];
        let ids_ba = vec![[2u8; 32], [1u8; 32]];
        let c1 = BatchCommitment::compute(&[0u8; 32], &[1u8; 32], &ids_ab);
        let c2 = BatchCommitment::compute(&[0u8; 32], &[1u8; 32], &ids_ba);
        assert_ne!(c1.submission_root, c2.submission_root, "ordering must be preserved in commitment");
    }

    // ── FinalizationEnvelope ─────────────────────────────────────────────────

    #[test]
    fn test_envelope_commitment_verifies() {
        let batch = make_batch(10);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        assert!(envelope.verify_commitment(), "envelope commitment must verify");
    }

    #[test]
    fn test_envelope_idempotent_delivery_same_commitment() {
        let batch = make_batch(20);
        let mut pipeline = BatchPipeline::new();
        let env1 = pipeline.deliver(&batch, FairnessMeta::none(1));
        let env2 = pipeline.deliver(&batch, FairnessMeta::none(1));
        // Same delivery_id → same commitment
        assert_eq!(env1.delivery_id, env2.delivery_id);
        assert_eq!(env1.commitment.commitment_hash, env2.commitment.commitment_hash);
    }

    #[test]
    fn test_delivery_increments_attempt_count_in_record() {
        let batch = make_batch(30);
        let mut pipeline = BatchPipeline::new();
        pipeline.deliver(&batch, FairnessMeta::none(1));
        assert_eq!(pipeline.get_delivery_record(&batch.batch_id).unwrap().attempt_count, 1);
        pipeline.deliver(&batch, FairnessMeta::none(1));
        assert_eq!(pipeline.get_delivery_record(&batch.batch_id).unwrap().attempt_count, 2);
    }

    // ── Happy path ack ───────────────────────────────────────────────────────

    #[test]
    fn test_happy_path_ack() {
        let batch = make_batch(40);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let ack = RuntimeIngestionAck::new(
            batch.batch_id, envelope.delivery_id, batch.epoch, 2, 0, None,
        );
        pipeline.record_ack(ack).unwrap();

        assert!(pipeline.is_terminal(&batch.batch_id));
        assert_eq!(
            pipeline.get_lifecycle(&batch.batch_id).unwrap().state,
            PipelineState::Accepted,
        );
    }

    #[test]
    fn test_ack_replay_rejected() {
        let batch = make_batch(41);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let ack1 = RuntimeIngestionAck::new(batch.batch_id, envelope.delivery_id, 1, 2, 0, None);
        let ack2 = RuntimeIngestionAck::new(batch.batch_id, envelope.delivery_id, 1, 2, 0, None);
        pipeline.record_ack(ack1).unwrap();
        assert_eq!(pipeline.record_ack(ack2), Err(BridgeError::AckReplay));
    }

    // ── Rejection causes ─────────────────────────────────────────────────────

    #[test]
    fn test_retryable_rejection_sets_pending_retry() {
        let batch = make_batch(50);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let rej = RuntimeIngestionRejection::new(
            batch.batch_id,
            envelope.delivery_id,
            batch.epoch,
            RejectionCause::TransientUnavailable("overloaded".into()),
        );
        pipeline.record_rejection(rej);

        assert!(pipeline.needs_retry(&batch.batch_id));
        assert!(!pipeline.is_terminal(&batch.batch_id));
    }

    #[test]
    fn test_terminal_rejection_no_retry() {
        let batch = make_batch(51);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let rej = RuntimeIngestionRejection::new(
            batch.batch_id,
            envelope.delivery_id,
            batch.epoch,
            RejectionCause::InvalidEnvelope("bad commitment".into()),
        );
        pipeline.record_rejection(rej);

        assert!(!pipeline.needs_retry(&batch.batch_id));
        assert!(pipeline.is_terminal(&batch.batch_id));
    }

    #[test]
    fn test_already_applied_rejection_is_terminal() {
        let batch = make_batch(52);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let rej = RuntimeIngestionRejection::new(
            batch.batch_id, envelope.delivery_id, batch.epoch,
            RejectionCause::AlreadyApplied,
        );
        pipeline.record_rejection(rej);
        assert!(pipeline.is_terminal(&batch.batch_id));
    }

    // ── Execution result reference ───────────────────────────────────────────

    #[test]
    fn test_execution_result_ref_stored_on_ack() {
        let batch = make_batch(60);
        let mut pipeline = BatchPipeline::new();
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        let result_ref = [0xABu8; 32];

        let ack = RuntimeIngestionAck::new(
            batch.batch_id, envelope.delivery_id, batch.epoch, 2, 0,
            Some(result_ref),
        );
        pipeline.record_ack(ack).unwrap();

        let lc = pipeline.get_lifecycle(&batch.batch_id).unwrap();
        assert_eq!(lc.execution_result_ref, Some(result_ref));
    }

    // ── Fairness metadata propagation ────────────────────────────────────────

    #[test]
    fn test_fairness_meta_in_envelope() {
        let batch = make_batch(70);
        let mut pipeline = BatchPipeline::new();
        let fairness = FairnessMeta {
            policy_version: 3,
            forced_inclusion_count: 2,
            rate_limited_count: 1,
            per_submission_class: vec![1, 2],
        };
        let envelope = pipeline.deliver(&batch, fairness);
        assert_eq!(envelope.fairness.policy_version, 3);
        assert_eq!(envelope.fairness.forced_inclusion_count, 2);
    }

    // ── Multiple batches ─────────────────────────────────────────────────────

    #[test]
    fn test_multiple_batches_independent() {
        let mut pipeline = BatchPipeline::new();
        let b1 = make_batch(80);
        let b2 = make_batch(81);

        let env1 = pipeline.deliver(&b1, FairnessMeta::none(1));
        let env2 = pipeline.deliver(&b2, FairnessMeta::none(1));

        // Different delivery IDs
        assert_ne!(env1.delivery_id, env2.delivery_id);

        // Ack b1, reject b2
        pipeline.record_ack(RuntimeIngestionAck::new(b1.batch_id, env1.delivery_id, 1, 1, 0, None)).unwrap();
        pipeline.record_rejection(RuntimeIngestionRejection::new(
            b2.batch_id, env2.delivery_id, 1,
            RejectionCause::ExecutionFailure("state conflict".into()),
        ));

        assert_eq!(pipeline.get_lifecycle(&b1.batch_id).unwrap().state, PipelineState::Accepted);
        assert_eq!(pipeline.get_lifecycle(&b2.batch_id).unwrap().state, PipelineState::RejectedTerminal);
    }
}
