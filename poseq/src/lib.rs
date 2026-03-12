pub mod errors;
pub mod config;
pub mod types;
pub mod intake;
pub mod validation;
pub mod queue;
pub mod ordering;
pub mod batching;
pub mod attestation;
pub mod receipts;
pub mod bridge;
pub mod state;

use config::policy::PoSeqPolicy;
use types::submission::SequencingSubmission;
use intake::receiver::SubmissionReceiver;
use validation::validator::SubmissionValidator;
use queue::pending::{ReplayGuard, SubmissionQueue};
use ordering::engine::OrderingEngine;
use batching::builder::{BatchBuilder, OrderedBatch};
use receipts::receipt::{BatchAuditRecord, SequencingReceipt};
use bridge::runtime::{PoSeqRuntimeExport, RuntimeBridge};
use state::sequencing_state::{BatchLedger, SequencingState};
use errors::PoSeqError;

/// The top-level PoSeq orchestrator that ties all components together.
pub struct PoSeqNode {
    pub policy: PoSeqPolicy,
    pub receiver: SubmissionReceiver,
    pub replay_guard: ReplayGuard,
    pub queue: SubmissionQueue,
    pub ordering_engine: OrderingEngine,
    pub batch_builder: BatchBuilder,
    pub state: SequencingState,
    pub ledger: BatchLedger,
}

impl PoSeqNode {
    pub fn new(policy: PoSeqPolicy) -> Self {
        let max_queue = policy.batch.max_pending_queue_size;
        let ordering_config = policy.ordering.clone();
        let policy_version = policy.version;
        PoSeqNode {
            receiver: SubmissionReceiver::new(),
            replay_guard: ReplayGuard::new(0), // unlimited history
            queue: SubmissionQueue::new(max_queue),
            ordering_engine: OrderingEngine::new(ordering_config),
            batch_builder: BatchBuilder::new(policy.clone()),
            state: SequencingState::new(policy_version),
            ledger: BatchLedger::new(),
            policy,
        }
    }

    /// Submit a raw submission. Returns the normalized_id or error.
    ///
    /// Pipeline:
    /// 1. receiver.receive → envelope
    /// 2. validator.validate → ValidatedSubmission
    /// 3. replay_guard.check_and_record
    /// 4. queue.push
    pub fn submit(&mut self, submission: SequencingSubmission) -> Result<[u8; 32], PoSeqError> {
        // Step 1: normalize into envelope
        let envelope = self.receiver.receive(submission);
        let normalized_id = envelope.normalized_id;

        // Step 2: validate
        let validated = SubmissionValidator::validate(envelope, &self.policy)?;

        // Step 3: replay/duplicate check
        self.replay_guard.check_and_record(normalized_id)?;

        // Step 4: enqueue
        self.queue.push(validated)?;

        Ok(normalized_id)
    }

    /// Produce the next ordered batch from the queue.
    ///
    /// Pipeline:
    /// 1. drain queue up to max_submissions_per_batch
    /// 2. ordering_engine.order
    /// 3. batch_builder.build
    /// 4. Build SequencingReceipt
    /// 5. state.advance
    /// 6. ledger.append
    pub fn produce_batch(
        &mut self,
        epoch: u64,
        sequencer_id: Option<[u8; 32]>,
    ) -> Result<(OrderedBatch, SequencingReceipt), PoSeqError> {
        let max = self.policy.batch.max_submissions_per_batch;
        let drained = self.queue.drain(max);

        // Order submissions
        let ordered = self.ordering_engine.order(drained)?;

        // Build batch
        let height = self.state.current_height + 1;
        let parent_batch_id = self.state.last_batch_id;
        let batch = self.batch_builder.build(ordered, height, parent_batch_id, epoch, sequencer_id)?;

        // Build receipt
        let receipt = SequencingReceipt::build(
            batch.header.batch_id,
            height,
            &batch.ordered_submissions,
            &[],  // no explicitly rejected (rejections happen at submit time)
            self.policy.version,
        );

        // Advance state
        self.state.advance(batch.header.batch_id);

        // Append to ledger
        let audit = BatchAuditRecord {
            batch_id: batch.header.batch_id,
            height,
            payload_root: batch.header.payload_root,
            policy_version: self.policy.version,
            receipt: receipt.clone(),
            metadata: std::collections::BTreeMap::new(),
        };
        self.ledger.append(audit);

        Ok((batch, receipt))
    }

    /// Export batch to runtime.
    pub fn export_to_runtime(&self, batch: &OrderedBatch) -> Result<PoSeqRuntimeExport, PoSeqError> {
        RuntimeBridge::export(batch).map_err(PoSeqError::Bridge)
    }
}
