//! Runtime-side batch ingestion from PoSeq.
//!
//! This module implements the runtime's half of the PoSeq→Runtime bridge contract.
//!
//! # Ingestion contract
//!
//! The runtime accepts a `FinalizationEnvelope` (canonical bridge type from PoSeq),
//! validates it, executes it idempotently, and returns an explicit `IngestionOutcome`
//! (ack or typed rejection).  The runtime NEVER communicates execution internals
//! back to PoSeq — only: "applied with N successes / M failures" or "rejected with cause".
//!
//! # Idempotency
//!
//! The runtime tracks applied `delivery_id` values.  Re-delivering the same batch
//! (same `delivery_id`) returns `AlreadyApplied` immediately without re-executing.
//! A new delivery_id (retry with a different attempt_count, theoretically) would
//! be treated as a fresh delivery and re-validated from scratch.
//!
//! # Separation of concerns
//!
//! - This module knows about `PoSeqRuntime` (execution engine) and `FinalizationEnvelope`.
//! - It does NOT know about PoSeq internals (proposals, attestations, committees, etc.).
//! - Execution results (state root, receipts) are summarised into opaque hashes that
//!   PoSeq stores without inspecting.

use std::collections::BTreeMap;
use std::collections::BTreeSet;
use sha2::{Sha256, Digest};

use crate::capabilities::registry::CapabilityRegistry;
use crate::errors::RuntimeError;
use crate::intents::base::{IntentTransaction, IntentType};
use crate::intents::types::TransferIntent;
use crate::poseq::interface::{OrderedBatch, PoSeqRuntime};
use crate::poseq::mempool::IntentMempool;
use crate::settlement::engine::SettlementResult;

// ─── Bridge contract types ────────────────────────────────────────────────────
//
// These mirror the PoSeq-side types exactly.  In a real production system these
// would come from a shared crate.  Here they are re-declared on the runtime side
// to maintain strict crate separation.

/// Minimal fairness metadata the runtime receives but does not act on for ordering.
#[derive(Debug, Clone)]
pub struct InboundFairnessMeta {
    pub policy_version: u32,
    pub forced_inclusion_count: u32,
    pub rate_limited_count: u32,
}

/// Inbound batch commitment — verified before ingestion.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct InboundCommitment {
    pub commitment_hash: [u8; 32],
    pub finalization_hash: [u8; 32],
    pub delivery_id: [u8; 32],
    pub submission_root: [u8; 32],
}

impl InboundCommitment {
    /// Recompute and verify the commitment hash over the delivered content.
    pub fn verify(
        finalization_hash: &[u8; 32],
        delivery_id: &[u8; 32],
        ordered_ids: &[[u8; 32]],
        claimed_hash: &[u8; 32],
    ) -> bool {
        // Rebuild submission root
        let submission_root = {
            let mut h = Sha256::new();
            for id in ordered_ids {
                h.update(id);
            }
            let r = h.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };
        // Rebuild commitment hash
        let computed = {
            let mut h = Sha256::new();
            h.update(finalization_hash);
            h.update(delivery_id);
            h.update(&submission_root);
            let r = h.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };
        &computed == claimed_hash
    }
}

/// The canonical inbound envelope the runtime accepts from PoSeq.
///
/// Mirrors `poseq::bridge::pipeline::FinalizationEnvelope`.
#[derive(Debug, Clone)]
pub struct InboundFinalizationEnvelope {
    // Identity
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub attempt_count: u32,
    // Sequencing metadata
    pub slot: u64,
    pub epoch: u64,
    pub sequence_number: u64,
    pub leader_id: [u8; 32],
    pub parent_batch_id: [u8; 32],
    // Payload
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub batch_root: [u8; 32],
    pub finalization_hash: [u8; 32],
    // Provenance
    pub quorum_approvals: usize,
    pub committee_size: usize,
    // Fairness
    pub fairness: InboundFairnessMeta,
    // Commitment
    pub commitment_hash: [u8; 32],
}

impl InboundFinalizationEnvelope {
    /// Structural validation: reject any obviously malformed envelope before execution.
    pub fn validate(&self) -> Result<(), IngestionRejectionCause> {
        if self.batch_id == [0u8; 32] {
            return Err(IngestionRejectionCause::InvalidEnvelope("zero batch_id".into()));
        }
        if self.delivery_id == [0u8; 32] {
            return Err(IngestionRejectionCause::InvalidEnvelope("zero delivery_id".into()));
        }
        if self.finalization_hash == [0u8; 32] {
            return Err(IngestionRejectionCause::InvalidEnvelope("zero finalization_hash".into()));
        }
        if self.committee_size == 0 {
            return Err(IngestionRejectionCause::InvalidEnvelope("zero committee_size".into()));
        }
        if self.quorum_approvals == 0 {
            return Err(IngestionRejectionCause::InvalidEnvelope("zero quorum_approvals".into()));
        }
        // Verify commitment hash
        if !InboundCommitment::verify(
            &self.finalization_hash,
            &self.delivery_id,
            &self.ordered_submission_ids,
            &self.commitment_hash,
        ) {
            return Err(IngestionRejectionCause::InvalidEnvelope("commitment hash mismatch".into()));
        }
        Ok(())
    }
}

/// Typed rejection cause from the runtime.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum IngestionRejectionCause {
    /// Envelope was structurally invalid (commitment mismatch, zero fields).
    InvalidEnvelope(String),
    /// This delivery_id was already applied — do not retry with same delivery_id.
    AlreadyApplied,
    /// Transient unavailability — may be retried.
    TransientUnavailable(String),
    /// Execution failed; partial state may have been applied.
    ExecutionFailure(String),
    /// Safety kernel blocked the batch.
    SafetyBlock(String),
}

impl IngestionRejectionCause {
    pub fn is_retryable(&self) -> bool {
        matches!(self, IngestionRejectionCause::TransientUnavailable(_))
    }

    pub fn tag(&self) -> &'static str {
        match self {
            IngestionRejectionCause::InvalidEnvelope(_)      => "invalid_envelope",
            IngestionRejectionCause::AlreadyApplied          => "already_applied",
            IngestionRejectionCause::TransientUnavailable(_) => "transient_unavailable",
            IngestionRejectionCause::ExecutionFailure(_)     => "execution_failure",
            IngestionRejectionCause::SafetyBlock(_)          => "safety_block",
        }
    }
}

/// Explicit ingestion outcome returned to the PoSeq bridge.
#[derive(Debug, Clone)]
pub enum IngestionOutcome {
    /// Batch was accepted and applied.
    Accepted(IngestionAck),
    /// Batch was rejected for a typed reason.
    Rejected(IngestionRejection),
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

/// Acknowledgment returned to PoSeq after successful ingestion.
#[derive(Debug, Clone)]
pub struct IngestionAck {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub epoch: u64,
    pub succeeded_count: usize,
    pub failed_count: usize,
    /// Opaque hash of the execution result (state_root of SettlementResult).
    /// PoSeq stores this as a reference without inspecting it.
    pub execution_result_ref: [u8; 32],
    /// SHA256(batch_id ‖ delivery_id ‖ "ack" ‖ epoch)
    pub ack_hash: [u8; 32],
}

impl IngestionAck {
    pub fn build(
        envelope: &InboundFinalizationEnvelope,
        settlement: &SettlementResult,
    ) -> Self {
        let ack_hash = {
            let mut h = Sha256::new();
            h.update(&envelope.batch_id);
            h.update(&envelope.delivery_id);
            h.update(b"ack");
            h.update(&envelope.epoch.to_be_bytes());
            let r = h.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };
        IngestionAck {
            batch_id: envelope.batch_id,
            delivery_id: envelope.delivery_id,
            epoch: envelope.epoch,
            succeeded_count: settlement.succeeded,
            failed_count: settlement.failed,
            execution_result_ref: settlement.state_root,
            ack_hash,
        }
    }
}

/// Rejection returned to PoSeq.
#[derive(Debug, Clone)]
pub struct IngestionRejection {
    pub batch_id: [u8; 32],
    pub delivery_id: [u8; 32],
    pub epoch: u64,
    pub cause: IngestionRejectionCause,
    pub rejection_hash: [u8; 32],
}

impl IngestionRejection {
    pub fn new(
        envelope: &InboundFinalizationEnvelope,
        cause: IngestionRejectionCause,
    ) -> Self {
        let rejection_hash = {
            let mut h = Sha256::new();
            h.update(&envelope.batch_id);
            h.update(&envelope.delivery_id);
            h.update(b"reject");
            h.update(cause.tag().as_bytes());
            let r = h.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };
        IngestionRejection {
            batch_id: envelope.batch_id,
            delivery_id: envelope.delivery_id,
            epoch: envelope.epoch,
            cause,
            rejection_hash,
        }
    }
}

// ─── RuntimeBatchIngester ─────────────────────────────────────────────────────

/// Runtime-side ingestion handler.
///
/// Owns the `PoSeqRuntime` execution engine and tracks applied delivery_ids
/// for idempotent duplicate handling.
///
/// When `mempool` is set, submission IDs are resolved to real `IntentTransaction`
/// payloads from the mempool. When `None`, falls back to scaffold mode (devnet only).
pub struct RuntimeBatchIngester {
    pub runtime: PoSeqRuntime,
    /// Set of delivery_ids already applied.  Prevents double-execution.
    applied_deliveries: BTreeSet<[u8; 32]>,
    /// Set of batch_ids already applied (for cross-delivery-id dedup).
    applied_batches: BTreeSet<[u8; 32]>,
    /// Intent mempool for looking up real transaction payloads by submission ID.
    /// When set, the ingester resolves transactions from here instead of
    /// synthesizing scaffolds.
    pub mempool: Option<IntentMempool>,
    /// Capability registry for per-sender capability resolution.
    /// When set, used by process_batch (via interface.rs) for capability gating.
    /// When None, falls back to CapabilitySet::all() (scaffold mode).
    pub capability_registry: Option<CapabilityRegistry>,
    /// When true and mempool is set, use the mempool path for transaction resolution.
    /// When false, always use the scaffold path (devnet compatibility).
    pub use_mempool: bool,
}

impl RuntimeBatchIngester {
    pub fn new() -> Self {
        RuntimeBatchIngester {
            runtime: PoSeqRuntime::new(),
            applied_deliveries: BTreeSet::new(),
            applied_batches: BTreeSet::new(),
            mempool: None,
            capability_registry: None,
            use_mempool: false,
        }
    }

    /// Create a new ingester with mempool and capability registry enabled.
    pub fn with_mempool(mempool: IntentMempool, registry: CapabilityRegistry) -> Self {
        RuntimeBatchIngester {
            runtime: PoSeqRuntime::new(),
            applied_deliveries: BTreeSet::new(),
            applied_batches: BTreeSet::new(),
            mempool: Some(mempool),
            capability_registry: Some(registry),
            use_mempool: true,
        }
    }

    /// Ingest a finalized batch envelope from PoSeq.
    ///
    /// # Lifecycle
    ///
    /// 1. **Structural validation** — commitment hash, non-zero fields
    /// 2. **Idempotency check** — same delivery_id or batch_id already applied
    /// 3. **Execution** — convert submission IDs to intent transactions, run through PoSeqRuntime
    /// 4. **Emit outcome** — ack with execution_result_ref, or typed rejection
    pub fn ingest(&mut self, envelope: InboundFinalizationEnvelope) -> IngestionOutcome {
        // ── Step 1: structural validation ────────────────────────────────────
        if let Err(cause) = envelope.validate() {
            return IngestionOutcome::Rejected(IngestionRejection::new(&envelope, cause));
        }

        // ── Step 2: idempotency — same delivery_id already applied ────────────
        if self.applied_deliveries.contains(&envelope.delivery_id) {
            return IngestionOutcome::Rejected(IngestionRejection::new(
                &envelope,
                IngestionRejectionCause::AlreadyApplied,
            ));
        }

        // ── Step 2b: idempotency — same batch_id applied under a prior delivery ─
        if self.applied_batches.contains(&envelope.batch_id) {
            return IngestionOutcome::Rejected(IngestionRejection::new(
                &envelope,
                IngestionRejectionCause::AlreadyApplied,
            ));
        }

        // ── Step 3: build OrderedBatch for runtime ────────────────────────────
        //
        // Two paths:
        //   (a) Mempool path (use_mempool=true, mempool is Some): look up real
        //       IntentTransaction payloads by submission_id, verify signatures,
        //       check deadlines, and resolve per-sender capabilities.
        //   (b) Scaffold path (use_mempool=false or mempool is None): synthesize
        //       placeholder transactions for devnet/testing only.
        let transactions: Vec<IntentTransaction> = if self.use_mempool {
            if let Some(ref mempool) = self.mempool {
                // ── Mempool path: resolve real transactions ──────────────────
                let mut valid_txns = Vec::new();
                for sid in &envelope.ordered_submission_ids {
                    // (a) Look up the real IntentTransaction from the mempool
                    let tx = match mempool.get(sid) {
                        Some(tx) => tx.clone(),
                        None => {
                            // Transaction not in mempool — skip it.
                            // In production this is expected if the tx was evicted
                            // or never gossipped to this node.
                            continue;
                        }
                    };

                    // (b) Verify signature (placeholder SHA256 scheme)
                    // MAINNET_BLOCKER(ed25519): Replace with real Ed25519
                    // verification once ed25519-dalek is added to Cargo.toml.
                    if !tx.verify_signature() {
                        // Invalid signature — skip this transaction
                        continue;
                    }

                    // (c) Check deadline: reject transactions whose deadline
                    //     has already passed
                    if tx.deadline_epoch < envelope.epoch {
                        // Expired deadline — skip
                        continue;
                    }

                    // (d) Capability check is deferred to process_batch via
                    //     the capability_registry (see interface.rs Step 5).
                    //     We only collect structurally valid, authenticated
                    //     transactions here.

                    valid_txns.push(tx);
                }
                valid_txns
            } else {
                // use_mempool is true but mempool is None — configuration error.
                // Fall through to scaffold path with a warning.
                Self::synthesize_scaffold_transactions(&envelope)
            }
        } else {
            // ── Scaffold path (devnet/testing) ──────────────────────────────
            // SCAFFOLD(devnet-only): This path synthesizes placeholder
            // IntentTransactions from submission IDs. It does NOT constitute
            // a production-safe intent execution path. Retained for devnet
            // backward compatibility.
            let txns = Self::synthesize_scaffold_transactions(&envelope);

            // Collision guard: the scaffold path uses submission_id as asset_id.
            // If any submission_id collides with a real asset in the store,
            // the synthetic transaction could mutate real balances.
            for sid in &envelope.ordered_submission_ids {
                if self.runtime.store.find_balance(&envelope.leader_id, sid).is_some() {
                    return IngestionOutcome::Rejected(IngestionRejection::new(
                        &envelope,
                        IngestionRejectionCause::ExecutionFailure(
                            format!(
                                "scaffold collision: submission_id {:02x}{:02x}{:02x}{:02x} matches \
                                 a real asset for sender {:02x}{:02x}{:02x}{:02x}",
                                sid[0], sid[1], sid[2], sid[3],
                                envelope.leader_id[0], envelope.leader_id[1],
                                envelope.leader_id[2], envelope.leader_id[3],
                            ),
                        ),
                    ));
                }
            }
            txns
        };

        let ordered_batch = OrderedBatch {
            batch_id: envelope.batch_id,
            epoch: envelope.epoch,
            sequence_number: envelope.sequence_number,
            transactions,
        };

        // ── Step 4: execute ───────────────────────────────────────────────────
        match self.runtime.process_batch(ordered_batch) {
            Ok(settlement) => {
                self.applied_deliveries.insert(envelope.delivery_id);
                self.applied_batches.insert(envelope.batch_id);
                let ack = IngestionAck::build(&envelope, &settlement);
                IngestionOutcome::Accepted(ack)
            }
            Err(RuntimeError::ObjectQuarantined(_)) | Err(RuntimeError::DomainPaused(_)) => {
                IngestionOutcome::Rejected(IngestionRejection::new(
                    &envelope,
                    IngestionRejectionCause::SafetyBlock(
                        "object quarantined or domain paused".into(),
                    ),
                ))
            }
            Err(e) => {
                IngestionOutcome::Rejected(IngestionRejection::new(
                    &envelope,
                    IngestionRejectionCause::ExecutionFailure(e.to_string()),
                ))
            }
        }
    }

    /// Check if a delivery_id was already applied.
    pub fn is_applied(&self, delivery_id: &[u8; 32]) -> bool {
        self.applied_deliveries.contains(delivery_id)
    }

    /// Check if a batch_id was already applied (under any delivery_id).
    pub fn is_batch_applied(&self, batch_id: &[u8; 32]) -> bool {
        self.applied_batches.contains(batch_id)
    }

    /// Count of successfully applied batches.
    pub fn applied_count(&self) -> usize {
        self.applied_batches.len()
    }

    /// Synthesize scaffold transactions from submission IDs (devnet/testing only).
    ///
    /// SCAFFOLD(devnet-only): These synthetic transactions have:
    ///   - sender = leader_id (not the actual submitter)
    ///   - signature = [0u8;64] (no verification)
    ///   - amount = 1 (no real economic effect)
    /// Retained for backward compatibility with devnet test infrastructure.
    fn synthesize_scaffold_transactions(
        envelope: &InboundFinalizationEnvelope,
    ) -> Vec<IntentTransaction> {
        envelope
            .ordered_submission_ids
            .iter()
            .enumerate()
            .map(|(i, sid)| {
                // Derive a non-zero recipient from the submission_id
                let mut recipient = *sid;
                recipient[31] ^= 0x01; // ensure recipient != sid
                if recipient == [0u8; 32] {
                    recipient[0] = 0x01;
                }
                IntentTransaction {
                    tx_id: *sid,
                    sender: envelope.leader_id,
                    nonce: envelope.slot * 1000 + i as u64,
                    intent: IntentType::Transfer(TransferIntent {
                        asset_id: *sid,
                        amount: 1, // minimal valid amount
                        recipient,
                        memo: None,
                    }),
                    max_fee: 1_000,
                    deadline_epoch: envelope.epoch + 10,
                    signature: [0u8; 64],
                    metadata: BTreeMap::new(),
                }
            })
            .collect()
    }
}

impl Default for RuntimeBatchIngester {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn compute_submission_root(ids: &[[u8; 32]]) -> [u8; 32] {
        let mut h = Sha256::new();
        for id in ids {
            h.update(id);
        }
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    fn compute_commitment_hash(fin_hash: &[u8; 32], delivery_id: &[u8; 32], ids: &[[u8; 32]]) -> [u8; 32] {
        let sub_root = compute_submission_root(ids);
        let mut h = Sha256::new();
        h.update(fin_hash);
        h.update(delivery_id);
        h.update(&sub_root);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    fn make_envelope(batch_byte: u8, delivery_byte: u8) -> InboundFinalizationEnvelope {
        let batch_id = make_id(batch_byte);
        let delivery_id = make_id(delivery_byte);
        let fin_hash = make_id(batch_byte + 1);
        let ids = vec![make_id(batch_byte + 2), make_id(batch_byte + 3)];
        let commitment_hash = compute_commitment_hash(&fin_hash, &delivery_id, &ids);

        InboundFinalizationEnvelope {
            batch_id,
            delivery_id,
            attempt_count: 1,
            slot: batch_byte as u64,
            epoch: 1,
            sequence_number: batch_byte as u64,
            leader_id: make_id(1),
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: ids,
            batch_root: make_id(batch_byte + 4),
            finalization_hash: fin_hash,
            quorum_approvals: 3,
            committee_size: 3,
            fairness: InboundFairnessMeta {
                policy_version: 1,
                forced_inclusion_count: 0,
                rate_limited_count: 0,
            },
            commitment_hash,
        }
    }

    // ── Structural validation ────────────────────────────────────────────────

    #[test]
    fn test_valid_envelope_passes_validation() {
        let env = make_envelope(10, 20);
        assert!(env.validate().is_ok());
    }

    #[test]
    fn test_zero_batch_id_rejected() {
        let mut env = make_envelope(10, 20);
        env.batch_id = [0u8; 32];
        assert!(matches!(
            env.validate(),
            Err(IngestionRejectionCause::InvalidEnvelope(_))
        ));
    }

    #[test]
    fn test_commitment_mismatch_rejected() {
        let mut env = make_envelope(10, 20);
        env.commitment_hash[0] ^= 0xFF; // corrupt
        assert!(matches!(
            env.validate(),
            Err(IngestionRejectionCause::InvalidEnvelope(_))
        ));
    }

    // ── Happy path ───────────────────────────────────────────────────────────

    #[test]
    fn test_happy_path_ingest() {
        let mut ingester = RuntimeBatchIngester::new();
        let env = make_envelope(20, 30);
        let batch_id = env.batch_id;
        let delivery_id = env.delivery_id;

        let outcome = ingester.ingest(env);
        assert!(outcome.is_accepted(), "should accept valid envelope");

        let IngestionOutcome::Accepted(ack) = outcome else { panic!() };
        assert_eq!(ack.batch_id, batch_id);
        assert_eq!(ack.delivery_id, delivery_id);
        assert_ne!(ack.execution_result_ref, [0u8; 32]);
    }

    // ── Idempotency ──────────────────────────────────────────────────────────

    #[test]
    fn test_duplicate_delivery_id_returns_already_applied() {
        let mut ingester = RuntimeBatchIngester::new();
        let env1 = make_envelope(30, 40);
        let env2 = make_envelope(30, 40); // same delivery_id

        ingester.ingest(env1);
        let outcome = ingester.ingest(env2);

        assert!(!outcome.is_accepted());
        let IngestionOutcome::Rejected(rej) = outcome else { panic!() };
        assert_eq!(rej.cause, IngestionRejectionCause::AlreadyApplied);
    }

    #[test]
    fn test_duplicate_batch_id_different_delivery_returns_already_applied() {
        let mut ingester = RuntimeBatchIngester::new();
        let env1 = make_envelope(31, 41);
        // Same batch_id, different delivery_id (retry scenario)
        let env2 = {
            let mut e = make_envelope(31, 42); // delivery_id[0]=42
            // Must recompute commitment_hash for new delivery_id
            e.batch_id = env1.batch_id;
            e.finalization_hash = env1.finalization_hash;
            e.commitment_hash = compute_commitment_hash(
                &e.finalization_hash,
                &e.delivery_id,
                &e.ordered_submission_ids,
            );
            e
        };

        ingester.ingest(env1);
        let outcome = ingester.ingest(env2);

        assert!(!outcome.is_accepted());
        let IngestionOutcome::Rejected(rej) = outcome else { panic!() };
        assert_eq!(rej.cause, IngestionRejectionCause::AlreadyApplied);
    }

    // ── Ordering preserved ───────────────────────────────────────────────────

    #[test]
    fn test_applied_count_increments() {
        let mut ingester = RuntimeBatchIngester::new();
        assert_eq!(ingester.applied_count(), 0);
        ingester.ingest(make_envelope(50, 60));
        assert_eq!(ingester.applied_count(), 1);
        ingester.ingest(make_envelope(51, 61));
        assert_eq!(ingester.applied_count(), 2);
    }

    // ── Ack hash determinism ─────────────────────────────────────────────────

    #[test]
    fn test_ack_hash_deterministic() {
        let mut ingester = RuntimeBatchIngester::new();
        let env = make_envelope(60, 70);
        let IngestionOutcome::Accepted(ack) = ingester.ingest(env) else { panic!() };

        // Recompute expected ack_hash
        let expected = {
            let mut h = Sha256::new();
            h.update(&ack.batch_id);
            h.update(&ack.delivery_id);
            h.update(b"ack");
            h.update(&ack.epoch.to_be_bytes());
            let r = h.finalize();
            let mut out = [0u8; 32];
            out.copy_from_slice(&r);
            out
        };
        assert_eq!(ack.ack_hash, expected);
    }

    // ── Fairness meta preserved (does not affect ordering) ───────────────────

    #[test]
    fn test_fairness_meta_accepted_with_batch() {
        let mut ingester = RuntimeBatchIngester::new();
        let mut env = make_envelope(70, 80);
        env.fairness.forced_inclusion_count = 2;
        env.fairness.rate_limited_count = 1;

        let outcome = ingester.ingest(env);
        assert!(outcome.is_accepted());
    }
}
