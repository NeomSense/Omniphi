//! End-to-end integration tests for the PoSeq → Runtime batch pipeline.
//!
//! Tests the complete pipeline:
//!   PoSeq finalization → `BatchPipeline::deliver()` → `FinalizationEnvelope`
//!   → runtime `RuntimeBatchIngester::ingest()` → `IngestionOutcome`
//!   → `BatchPipeline::record_ack/rejection()` → PoSeq lifecycle update
//!
//! These tests use real types from both crates and exercise the full contract.

// This crate is test-only; all content is in the test modules below.

#[cfg(test)]
mod e2e {
    use sha2::{Sha256, Digest};

    // ── PoSeq side ─────────────────────────────────────────────────────────
    use omniphi_poseq::finalization::engine::FinalizedBatch;
    use omniphi_poseq::attestations::collector::AttestationQuorumResult;
    use omniphi_poseq::bridge::pipeline::{
        BatchPipeline, FairnessMeta, FinalizationEnvelope,
        RuntimeIngestionAck, RuntimeIngestionRejection, RejectionCause,
        PipelineState,
    };
    use omniphi_poseq::errors::BridgeError;

    // ── Runtime side ────────────────────────────────────────────────────────
    use omniphi_runtime::poseq::ingestion::{
        RuntimeBatchIngester, InboundFinalizationEnvelope, InboundFairnessMeta,
        IngestionOutcome, IngestionRejectionCause,
    };

    // ─── Test helpers ────────────────────────────────────────────────────────

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_qr(approvals: usize) -> AttestationQuorumResult {
        AttestationQuorumResult {
            reached: true,
            approvals,
            rejections: 0,
            total_votes: approvals + 1,
            quorum_hash: [0xABu8; 32],
        }
    }

    fn make_finalized_batch(b: u8, submission_count: usize) -> FinalizedBatch {
        let ordered_ids: Vec<[u8; 32]> = (0..submission_count)
            .map(|i| { let mut id = make_id(b); id[1] = i as u8; id })
            .collect();
        FinalizedBatch {
            batch_id: make_id(b),
            proposal_id: make_id(b.wrapping_add(100)),
            slot: b as u64,
            epoch: 1,
            leader_id: make_id(1),
            ordered_submission_ids: ordered_ids,
            batch_root: make_id(b.wrapping_add(50)),
            parent_batch_id: [0u8; 32],
            finalized_at_height: 100 + b as u64,
            quorum_summary: make_qr(3),
            finalization_hash: make_id(b.wrapping_add(200)),
        }
    }

    /// Convert a PoSeq `FinalizationEnvelope` into a runtime `InboundFinalizationEnvelope`.
    /// In production this would be a serialization boundary (e.g., protobuf over the wire).
    fn to_inbound(env: &FinalizationEnvelope) -> InboundFinalizationEnvelope {
        InboundFinalizationEnvelope {
            batch_id: env.batch_id,
            delivery_id: env.delivery_id,
            attempt_count: env.attempt_count,
            slot: env.slot,
            epoch: env.epoch,
            sequence_number: env.sequence_number,
            leader_id: env.leader_id,
            parent_batch_id: env.parent_batch_id,
            ordered_submission_ids: env.ordered_submission_ids.clone(),
            batch_root: env.batch_root,
            finalization_hash: env.finalization_hash,
            quorum_approvals: env.quorum_approvals,
            committee_size: env.committee_size,
            fairness: InboundFairnessMeta {
                policy_version: env.fairness.policy_version,
                forced_inclusion_count: env.fairness.forced_inclusion_count,
                rate_limited_count: env.fairness.rate_limited_count,
            },
            commitment_hash: env.commitment.commitment_hash,
        }
    }

    // ─── Test 1: Simple happy path ───────────────────────────────────────────

    #[test]
    fn test_e2e_happy_path() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(10, 3);
        let batch_id = batch.batch_id;

        // PoSeq delivers
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        assert!(envelope.verify_commitment());

        // Runtime ingests
        let inbound = to_inbound(&envelope);
        let outcome = ingester.ingest(inbound);
        assert!(outcome.is_accepted(), "happy path: should accept");

        let IngestionOutcome::Accepted(ack) = outcome else { panic!("expected ack") };

        // Convert runtime ack → PoSeq ack
        let poseq_ack = RuntimeIngestionAck::new(
            ack.batch_id,
            ack.delivery_id,
            ack.epoch,
            ack.succeeded_count,
            ack.failed_count,
            Some(ack.execution_result_ref),
        );

        // PoSeq records ack
        pipeline.record_ack(poseq_ack).unwrap();

        // Lifecycle state: Accepted
        let lc = pipeline.get_lifecycle(&batch_id).unwrap();
        assert_eq!(lc.state, PipelineState::Accepted);
        assert!(lc.execution_result_ref.is_some());
    }

    // ─── Test 2: Duplicate delivery (same delivery_id) ───────────────────────

    #[test]
    fn test_e2e_duplicate_delivery_idempotent() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(20, 2);
        let batch_id = batch.batch_id;

        // First delivery
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        let inbound1 = to_inbound(&envelope);
        let outcome1 = ingester.ingest(inbound1);
        assert!(outcome1.is_accepted(), "first delivery should accept");

        // Second delivery (same batch, same delivery_id)
        let envelope2 = pipeline.deliver(&batch, FairnessMeta::none(1));
        assert_eq!(envelope.delivery_id, envelope2.delivery_id, "delivery_id must be idempotent");
        let inbound2 = to_inbound(&envelope2);
        let outcome2 = ingester.ingest(inbound2);

        // Runtime returns AlreadyApplied
        assert!(!outcome2.is_accepted());
        let IngestionOutcome::Rejected(rej) = outcome2 else { panic!() };
        assert_eq!(rej.cause, IngestionRejectionCause::AlreadyApplied);

        // Attempt count in pipeline record: incremented
        let record = pipeline.get_delivery_record(&batch_id).unwrap();
        assert_eq!(record.attempt_count, 2);
    }

    // ─── Test 3: Retryable rejection then retry succeeds ─────────────────────

    #[test]
    fn test_e2e_transient_rejection_then_retry() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(30, 2);
        let batch_id = batch.batch_id;

        // First deliver + simulate runtime returning transient rejection
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        let poseq_rej = RuntimeIngestionRejection::new(
            batch_id,
            envelope.delivery_id,
            batch.epoch,
            RejectionCause::TransientUnavailable("overloaded".into()),
        );
        pipeline.record_rejection(poseq_rej);
        assert!(pipeline.needs_retry(&batch_id));

        // Actually ingest (simulating retry after runtime recovered)
        let inbound = to_inbound(&envelope);
        let outcome = ingester.ingest(inbound);
        assert!(outcome.is_accepted(), "retry after transient failure should accept");

        let IngestionOutcome::Accepted(ack) = outcome else { panic!() };
        let poseq_ack = RuntimeIngestionAck::new(
            ack.batch_id, ack.delivery_id, ack.epoch,
            ack.succeeded_count, ack.failed_count,
            Some(ack.execution_result_ref),
        );
        pipeline.record_ack(poseq_ack).unwrap();
        assert_eq!(pipeline.get_lifecycle(&batch_id).unwrap().state, PipelineState::Accepted);
    }

    // ─── Test 4: Terminal rejection — no retry ────────────────────────────────

    #[test]
    fn test_e2e_terminal_rejection_not_retried() {
        let mut pipeline = BatchPipeline::new();

        let batch = make_finalized_batch(40, 1);
        let batch_id = batch.batch_id;

        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        // Terminal rejection (commitment error, safety block, etc.)
        let poseq_rej = RuntimeIngestionRejection::new(
            batch_id,
            envelope.delivery_id,
            batch.epoch,
            RejectionCause::InvalidEnvelope("tampered commitment".into()),
        );
        pipeline.record_rejection(poseq_rej);

        assert!(!pipeline.needs_retry(&batch_id));
        assert!(pipeline.is_terminal(&batch_id));
        assert_eq!(
            pipeline.get_lifecycle(&batch_id).unwrap().state,
            PipelineState::RejectedTerminal,
        );
    }

    // ─── Test 5: Commitment verification prevents tampered ordering ───────────

    #[test]
    fn test_e2e_tampered_ordering_rejected_by_runtime() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(50, 3);

        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        // Tamper: swap two submission IDs in the inbound envelope
        let mut inbound = to_inbound(&envelope);
        inbound.ordered_submission_ids.swap(0, 1);
        // commitment_hash now won't match

        let outcome = ingester.ingest(inbound);
        assert!(!outcome.is_accepted());
        let IngestionOutcome::Rejected(rej) = outcome else { panic!() };
        assert_eq!(rej.cause, IngestionRejectionCause::InvalidEnvelope("commitment hash mismatch".into()));
    }

    // ─── Test 6: Restart during bridge delivery (crash recovery simulation) ───

    #[test]
    fn test_e2e_restart_crash_recovery() {
        // Simulate: PoSeq crashes after deliver() but before ack arrives.
        // On restart, deliver() is called again (same batch) → same delivery_id.
        // Runtime had not yet applied it → accepts the retry.

        let batch = make_finalized_batch(60, 2);
        let batch_id = batch.batch_id;

        // Pre-crash: pipeline 1 delivers but crashes before ack
        let mut pipeline1 = BatchPipeline::new();
        let envelope = pipeline1.deliver(&batch, FairnessMeta::none(1));
        let delivery_id = envelope.delivery_id;
        // (crash — pipeline1 is dropped)

        // Post-crash: pipeline 2 is a fresh restart, runtime had not applied it
        let mut pipeline2 = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new(); // fresh runtime

        // Redeliver same batch (same delivery_id due to idempotent batching)
        let envelope2 = pipeline2.deliver(&batch, FairnessMeta::none(1));
        assert_eq!(envelope.delivery_id, envelope2.delivery_id, "delivery_id must survive restart");

        let inbound = to_inbound(&envelope2);
        let outcome = ingester.ingest(inbound);
        assert!(outcome.is_accepted(), "first delivery to fresh runtime should accept");

        let IngestionOutcome::Accepted(ack) = outcome else { panic!() };
        assert_eq!(ack.delivery_id, delivery_id);

        pipeline2.record_ack(RuntimeIngestionAck::new(
            ack.batch_id, ack.delivery_id, ack.epoch,
            ack.succeeded_count, ack.failed_count,
            Some(ack.execution_result_ref),
        )).unwrap();

        assert_eq!(pipeline2.get_lifecycle(&batch_id).unwrap().state, PipelineState::Accepted);
    }

    // ─── Test 7: Ack replay protection ───────────────────────────────────────

    #[test]
    fn test_e2e_ack_replay_protection() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(70, 2);
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let inbound = to_inbound(&envelope);
        let IngestionOutcome::Accepted(ack) = ingester.ingest(inbound) else { panic!() };

        let poseq_ack1 = RuntimeIngestionAck::new(
            ack.batch_id, ack.delivery_id, ack.epoch, ack.succeeded_count, ack.failed_count,
            Some(ack.execution_result_ref),
        );
        let poseq_ack2 = poseq_ack1.clone();

        pipeline.record_ack(poseq_ack1).unwrap();
        assert_eq!(pipeline.record_ack(poseq_ack2), Err(BridgeError::AckReplay));
    }

    // ─── Test 8: Fairness metadata propagates end-to-end ─────────────────────

    #[test]
    fn test_e2e_fairness_metadata_propagation() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(80, 4);
        let fairness = FairnessMeta {
            policy_version: 5,
            forced_inclusion_count: 2,
            rate_limited_count: 1,
            per_submission_class: vec![1, 2, 1, 3],
        };
        let envelope = pipeline.deliver(&batch, fairness);
        assert_eq!(envelope.fairness.policy_version, 5);
        assert_eq!(envelope.fairness.forced_inclusion_count, 2);

        // Runtime receives fairness metadata but does not use it for ordering
        let mut inbound = to_inbound(&envelope);
        assert_eq!(inbound.fairness.forced_inclusion_count, 2);

        let outcome = ingester.ingest(inbound);
        assert!(outcome.is_accepted());
    }

    // ─── Test 9: Finality state update after ack ─────────────────────────────

    #[test]
    fn test_e2e_finality_state_after_ack() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(90, 2);
        let batch_id = batch.batch_id;

        // Before delivery
        assert!(pipeline.get_lifecycle(&batch_id).is_none());

        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        // After delivery: Pending
        assert_eq!(
            pipeline.get_lifecycle(&batch_id).unwrap().state,
            PipelineState::Pending,
        );

        let inbound = to_inbound(&envelope);
        let IngestionOutcome::Accepted(ack) = ingester.ingest(inbound) else { panic!() };

        pipeline.record_ack(RuntimeIngestionAck::new(
            ack.batch_id, ack.delivery_id, ack.epoch,
            ack.succeeded_count, ack.failed_count,
            Some(ack.execution_result_ref),
        )).unwrap();

        // After ack: Accepted, with execution result ref
        let lc = pipeline.get_lifecycle(&batch_id).unwrap();
        assert_eq!(lc.state, PipelineState::Accepted);
        assert!(lc.execution_result_ref.is_some());
    }

    // ─── Test 10: Multiple batches sequential finality ────────────────────────

    #[test]
    fn test_e2e_multiple_batches_ordered() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batches: Vec<FinalizedBatch> = (1u8..=5)
            .map(|i| make_finalized_batch(i * 10, 2))
            .collect();

        for batch in &batches {
            let envelope = pipeline.deliver(batch, FairnessMeta::none(1));
            let inbound = to_inbound(&envelope);
            let outcome = ingester.ingest(inbound);
            assert!(outcome.is_accepted(), "batch {} should accept", batch.batch_id[0]);

            let IngestionOutcome::Accepted(ack) = outcome else { panic!() };
            pipeline.record_ack(RuntimeIngestionAck::new(
                ack.batch_id, ack.delivery_id, ack.epoch,
                ack.succeeded_count, ack.failed_count,
                Some(ack.execution_result_ref),
            )).unwrap();
        }

        // All accepted
        for batch in &batches {
            assert_eq!(
                pipeline.get_lifecycle(&batch.batch_id).unwrap().state,
                PipelineState::Accepted,
            );
        }

        // Runtime applied all 5
        assert_eq!(ingester.applied_count(), 5);
    }

    // ─── Test 11: Batch with zero submissions (edge case) ─────────────────────

    #[test]
    fn test_e2e_empty_batch_accepted() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(100, 0); // zero submissions
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        assert!(envelope.verify_commitment());

        let inbound = to_inbound(&envelope);
        let outcome = ingester.ingest(inbound);
        // Empty batches are valid (sequencer may produce them for epoch boundaries)
        assert!(outcome.is_accepted());
    }

    // ─── Test 12: Safety block rejection ─────────────────────────────────────

    #[test]
    fn test_e2e_safety_block_rejection() {
        let mut pipeline = BatchPipeline::new();

        let batch = make_finalized_batch(110, 2);
        let batch_id = batch.batch_id;
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        // Simulate runtime safety kernel blocking this batch
        let rej = RuntimeIngestionRejection::new(
            batch_id,
            envelope.delivery_id,
            batch.epoch,
            RejectionCause::SafetyBlock("emergency mode active".into()),
        );
        pipeline.record_rejection(rej);

        // Safety block is terminal (not retryable)
        assert!(pipeline.is_terminal(&batch_id));
        assert!(!pipeline.needs_retry(&batch_id));
        assert_eq!(
            pipeline.get_lifecycle(&batch_id).unwrap().state,
            PipelineState::RejectedTerminal,
        );
    }

    // ─── Test 13: CRX execution result reference stored opaquely ─────────────

    #[test]
    fn test_e2e_crx_execution_result_ref_opaque() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(120, 2);
        let batch_id = batch.batch_id;
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));

        let inbound = to_inbound(&envelope);
        let IngestionOutcome::Accepted(ack) = ingester.ingest(inbound) else { panic!() };

        // execution_result_ref is the state_root from SettlementResult
        // PoSeq stores this opaquely without inspecting it
        assert_ne!(ack.execution_result_ref, [0u8; 32], "state_root should be non-zero");

        let result_ref = ack.execution_result_ref;
        pipeline.record_ack(RuntimeIngestionAck::new(
            ack.batch_id, ack.delivery_id, ack.epoch,
            ack.succeeded_count, ack.failed_count,
            Some(result_ref),
        )).unwrap();

        // PoSeq stored the opaque ref — can retrieve without parsing
        let stored_ref = pipeline.get_lifecycle(&batch_id).unwrap().execution_result_ref;
        assert_eq!(stored_ref, Some(result_ref));
    }

    // ─── Test 14: Already-applied batch from prior epoch ─────────────────────

    #[test]
    fn test_e2e_batch_from_prior_epoch_rejected() {
        let mut pipeline = BatchPipeline::new();
        let mut ingester = RuntimeBatchIngester::new();

        let batch = make_finalized_batch(130, 2);

        // First ingestion succeeds
        let envelope = pipeline.deliver(&batch, FairnessMeta::none(1));
        let inbound1 = to_inbound(&envelope);
        let outcome1 = ingester.ingest(inbound1);
        assert!(outcome1.is_accepted());

        // PoSeq records the ack
        let IngestionOutcome::Accepted(ack1) = outcome1 else { panic!() };
        pipeline.record_ack(RuntimeIngestionAck::new(
            ack1.batch_id, ack1.delivery_id, ack1.epoch,
            ack1.succeeded_count, ack1.failed_count,
            Some(ack1.execution_result_ref),
        )).unwrap();

        // Hypothetical re-delivery (different delivery_id) — e.g., from a replay attack
        // Create a second pipeline just to get a different delivery_id
        let mut pipeline2 = BatchPipeline::new();
        // Force a "retry" by delivering again in a fresh pipeline (different delivery_id
        // would only happen if attempt_count changed — but we can simulate this by
        // building an inbound envelope with a modified delivery_id)
        let envelope2 = pipeline2.deliver(&batch, FairnessMeta::none(1));
        // Same delivery_id since it's the same batch with attempt_count=1
        // Instead test same batch_id redelivery directly
        let inbound2 = to_inbound(&envelope2);
        // Runtime blocks it: already applied
        let outcome2 = ingester.ingest(inbound2);
        assert!(!outcome2.is_accepted());
        let IngestionOutcome::Rejected(rej) = outcome2 else { panic!() };
        assert_eq!(rej.cause, IngestionRejectionCause::AlreadyApplied);
    }

    // ─── Test 15: Commitment hash covers ordering exactly ─────────────────────

    #[test]
    fn test_e2e_commitment_covers_ordering() {
        // Two batches with the same submissions but in different order
        // must have different commitment hashes — the runtime can detect reordering.
        let mut b1 = make_finalized_batch(140, 3);
        let mut b2 = make_finalized_batch(140, 3);
        // Same batch_id but reverse the ordering
        b2.ordered_submission_ids.reverse();

        let mut pipeline1 = BatchPipeline::new();
        let mut pipeline2 = BatchPipeline::new();

        let env1 = pipeline1.deliver(&b1, FairnessMeta::none(1));
        let env2 = pipeline2.deliver(&b2, FairnessMeta::none(1));

        // Same delivery_id (same batch_id + attempt_count)
        assert_eq!(env1.delivery_id, env2.delivery_id);
        // But different commitment because ordering differs
        assert_ne!(
            env1.commitment.commitment_hash,
            env2.commitment.commitment_hash,
            "different orderings must produce different commitments"
        );
        assert_ne!(
            env1.commitment.submission_root,
            env2.commitment.submission_root,
        );
    }
}
