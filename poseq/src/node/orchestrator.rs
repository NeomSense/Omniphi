//! Unified Omniphi node orchestrator.
//!
//! Wires PoSeq consensus, Runtime batch ingestion, and chain settlement
//! into a single coordinated lifecycle.
//!
//! # Architecture
//!
//! ```text
//! NetworkedNode (PoSeq)
//!       │ finalize batch
//!       ▼
//! RuntimeDeliveryChannel (mpsc)
//!       │ FinalizationEnvelope
//!       ▼
//! RuntimeBatchIngester
//!       │ settlement result
//!       ▼
//! SettlementBridge → posd CLI (chain)
//! ```

use std::sync::Arc;
use tokio::sync::Mutex;

use crate::bridge::runtime_channel::RuntimeDeliveryReceiver;
use crate::bridge::pipeline::FinalizationEnvelope;
use crate::persistence::engine::PersistenceEngine;

// Re-export for callers that need the config
pub use crate::networking::node_runner::NodeConfig;

/// Configuration for the unified Omniphi node.
#[derive(Debug, Clone)]
pub struct OmniphiNodeConfig {
    /// PoSeq node configuration.
    pub poseq: NodeConfig,
    /// Enable runtime integration (if false, runs PoSeq-only mode).
    pub enable_runtime: bool,
    /// Channel buffer size for PoSeq→Runtime delivery.
    pub delivery_buffer: usize,
    /// Enable settlement bridge to Cosmos chain.
    pub enable_settlement: bool,
    /// Path to `posd` binary for chain settlement.
    pub posd_bin: String,
    /// Chain ID for settlement transactions.
    pub chain_id: String,
    /// Cosmos account to sign settlement txs.
    pub settlement_from: String,
}

impl OmniphiNodeConfig {
    /// Create config with defaults and a given PoSeq node config.
    pub fn with_poseq(poseq: NodeConfig) -> Self {
        OmniphiNodeConfig {
            poseq,
            enable_runtime: true,
            delivery_buffer: 128,
            enable_settlement: false,
            posd_bin: "posd".into(),
            chain_id: "omniphi-testnet-1".into(),
            settlement_from: "validator".into(),
        }
    }
}

/// Health status of the unified node.
#[derive(Debug, Clone)]
pub struct NodeHealthStatus {
    pub poseq_running: bool,
    pub runtime_connected: bool,
    pub batches_ingested: u64,
    pub batches_failed: u64,
    pub last_ingested_batch: Option<[u8; 32]>,
}

/// Sled key prefixes for cross-layer state.
pub mod runtime_keys {
    pub const LAST_INGESTED: &[u8] = b"runtime:last_ingested:";
    pub const DELIVERY_LOG: &[u8] = b"runtime:delivery_log:";
    pub const SETTLEMENT_LAST: &[u8] = b"settlement:last_submitted:";
}

/// Tracks runtime ingestion state across restarts.
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
pub struct RuntimeIngestionState {
    pub batches_ingested: u64,
    pub batches_failed: u64,
    pub last_ingested_batch: Option<[u8; 32]>,
    /// Set of batch_ids already processed (for idempotency).
    pub processed_batch_ids: std::collections::BTreeSet<[u8; 32]>,
}

impl RuntimeIngestionState {
    /// Record a successful ingestion.
    pub fn record_success(&mut self, batch_id: [u8; 32]) {
        self.batches_ingested += 1;
        self.last_ingested_batch = Some(batch_id);
        self.processed_batch_ids.insert(batch_id);
    }

    /// Record a failed ingestion.
    pub fn record_failure(&mut self, batch_id: [u8; 32]) {
        self.batches_failed += 1;
        self.processed_batch_ids.insert(batch_id);
    }

    /// Check if a batch was already processed.
    pub fn is_processed(&self, batch_id: &[u8; 32]) -> bool {
        self.processed_batch_ids.contains(batch_id)
    }
}

/// Convert a PoSeq `FinalizationEnvelope` to the Runtime's `InboundFinalizationEnvelope`.
///
/// This is the serialization boundary between the two crates.
pub fn to_inbound(
    env: &FinalizationEnvelope,
) -> omniphi_runtime::poseq::ingestion::InboundFinalizationEnvelope {
    omniphi_runtime::poseq::ingestion::InboundFinalizationEnvelope {
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
        fairness: omniphi_runtime::poseq::ingestion::InboundFairnessMeta {
            policy_version: env.fairness.policy_version,
            forced_inclusion_count: env.fairness.forced_inclusion_count,
            rate_limited_count: env.fairness.rate_limited_count,
        },
        commitment_hash: env.commitment.commitment_hash,
    }
}

/// Run the runtime ingestion loop.
///
/// Receives `FinalizationEnvelope`s from the PoSeq node and feeds them
/// to the `RuntimeBatchIngester`. Persists ingestion state to sled.
pub async fn run_runtime_ingestion(
    mut receiver: RuntimeDeliveryReceiver,
    store: Arc<Mutex<crate::persistence::durable_store::DurableStore>>,
    state: Arc<Mutex<RuntimeIngestionState>>,
) {
    use omniphi_runtime::poseq::ingestion::{RuntimeBatchIngester, IngestionOutcome};

    let mut ingester = RuntimeBatchIngester::new();

    println!("[orchestrator] Runtime ingestion loop started");

    while let Some(item) = receiver.recv().await {
        let batch_id = item.envelope.batch_id;
        let seq = item.delivery_seq;

        // Check idempotency
        {
            let st = state.lock().await;
            if st.is_processed(&batch_id) {
                println!(
                    "[orchestrator] Batch {} already processed, skipping (seq={})",
                    hex::encode(&batch_id[..4]), seq
                );
                continue;
            }
        }

        // Convert and ingest
        let inbound = to_inbound(&item.envelope);
        let outcome = ingester.ingest(inbound);

        match &outcome {
            IngestionOutcome::Accepted(ack) => {
                println!(
                    "[orchestrator] Batch {} ACCEPTED (succeeded={}, failed={}, seq={})",
                    hex::encode(&batch_id[..4]),
                    ack.succeeded_count,
                    ack.failed_count,
                    seq,
                );
                let mut st = state.lock().await;
                st.record_success(batch_id);
            }
            IngestionOutcome::Rejected(rej) => {
                println!(
                    "[orchestrator] Batch {} REJECTED: {:?} (seq={})",
                    hex::encode(&batch_id[..4]),
                    rej.cause,
                    seq,
                );
                let mut st = state.lock().await;
                st.record_failure(batch_id);
            }
        }

        // Persist state
        {
            let st = state.lock().await;
            if let Ok(bytes) = bincode::serialize(&*st) {
                let mut ds = store.lock().await;
                ds.engine.put_raw(runtime_keys::LAST_INGESTED, bytes);
            }
        }
    }

    println!("[orchestrator] Runtime ingestion loop ended (channel closed)");
}

/// Load persisted runtime ingestion state from sled.
pub fn load_ingestion_state(engine: &PersistenceEngine) -> RuntimeIngestionState {
    match engine.get_raw(runtime_keys::LAST_INGESTED) {
        Some(bytes) => bincode::deserialize(&bytes).unwrap_or_default(),
        None => RuntimeIngestionState::default(),
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// Tests
// ═══════════════════════════════════════════════════════════════════════════

#[cfg(test)]
mod tests {
    use super::*;
    use crate::bridge::pipeline::{FinalizationEnvelope, FairnessMeta, BatchCommitment};

    fn make_envelope(batch_byte: u8) -> FinalizationEnvelope {
        let id = { let mut b = [0u8; 32]; b[0] = batch_byte; b };
        let fin = { let mut b = [0u8; 32]; b[1] = batch_byte; b };
        let did = { let mut b = [0u8; 32]; b[2] = batch_byte; b };
        let commitment = BatchCommitment::compute(&fin, &did, &[]);
        FinalizationEnvelope {
            batch_id: id,
            delivery_id: did,
            attempt_count: 1,
            slot: 1,
            epoch: 1,
            sequence_number: batch_byte as u64,
            leader_id: [0u8; 32],
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![],
            batch_root: [0u8; 32],
            finalization_hash: fin,
            quorum_approvals: 2,
            committee_size: 3,
            fairness: FairnessMeta::none(1),
            commitment,
        }
    }

    #[test]
    fn test_ingestion_state_idempotency() {
        let mut state = RuntimeIngestionState::default();
        let batch_id = [1u8; 32];

        assert!(!state.is_processed(&batch_id));
        state.record_success(batch_id);
        assert!(state.is_processed(&batch_id));
        assert_eq!(state.batches_ingested, 1);
    }

    #[test]
    fn test_ingestion_state_failure_tracking() {
        let mut state = RuntimeIngestionState::default();
        let batch_id = [2u8; 32];

        state.record_failure(batch_id);
        assert!(state.is_processed(&batch_id));
        assert_eq!(state.batches_failed, 1);
        assert_eq!(state.batches_ingested, 0);
    }

    #[test]
    fn test_ingestion_state_serialization_roundtrip() {
        let mut state = RuntimeIngestionState::default();
        state.record_success([1u8; 32]);
        state.record_success([2u8; 32]);
        state.record_failure([3u8; 32]);

        let bytes = bincode::serialize(&state).unwrap();
        let restored: RuntimeIngestionState = bincode::deserialize(&bytes).unwrap();

        assert_eq!(restored.batches_ingested, 2);
        assert_eq!(restored.batches_failed, 1);
        assert!(restored.is_processed(&[1u8; 32]));
        assert!(restored.is_processed(&[2u8; 32]));
        assert!(restored.is_processed(&[3u8; 32]));
        assert!(!restored.is_processed(&[4u8; 32]));
    }

    #[test]
    fn test_to_inbound_conversion() {
        let env = make_envelope(42);
        let inbound = to_inbound(&env);

        assert_eq!(inbound.batch_id, env.batch_id);
        assert_eq!(inbound.delivery_id, env.delivery_id);
        assert_eq!(inbound.slot, env.slot);
        assert_eq!(inbound.epoch, env.epoch);
        assert_eq!(inbound.commitment_hash, env.commitment.commitment_hash);
        assert_eq!(inbound.quorum_approvals, env.quorum_approvals);
    }

    #[test]
    fn test_health_status() {
        let status = NodeHealthStatus {
            poseq_running: true,
            runtime_connected: true,
            batches_ingested: 5,
            batches_failed: 1,
            last_ingested_batch: Some([1u8; 32]),
        };
        assert!(status.poseq_running);
        assert_eq!(status.batches_ingested, 5);
    }

    #[tokio::test]
    async fn test_runtime_ingestion_loop_processes_batch() {
        use crate::bridge::runtime_channel::RuntimeDeliveryChannel;

        let (sender, receiver) = RuntimeDeliveryChannel::create(8);

        // Create in-memory store
        let backend = crate::persistence::backend::InMemoryBackend::new();
        let engine = PersistenceEngine::new(Box::new(backend));
        let store = Arc::new(Mutex::new(
            crate::persistence::durable_store::DurableStore::new(engine),
        ));
        let state = Arc::new(Mutex::new(RuntimeIngestionState::default()));

        // Spawn ingestion loop
        let state_clone = state.clone();
        let store_clone = store.clone();
        let handle = tokio::spawn(async move {
            run_runtime_ingestion(receiver, store_clone, state_clone).await;
        });

        // Send a batch
        sender.deliver(make_envelope(1)).await.unwrap();

        // Give ingestion loop time to process
        tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;

        // Drop sender to close channel and end loop
        drop(sender);
        handle.await.unwrap();

        // Verify state
        let st = state.lock().await;
        assert!(st.is_processed(&{ let mut b = [0u8; 32]; b[0] = 1; b }));
        // May be success or failure depending on runtime validation —
        // the important thing is it was processed and persisted
        assert!(st.batches_ingested + st.batches_failed >= 1);
    }

    #[tokio::test]
    async fn test_runtime_ingestion_skips_duplicate() {
        use crate::bridge::runtime_channel::RuntimeDeliveryChannel;

        let (sender, receiver) = RuntimeDeliveryChannel::create(8);

        let backend = crate::persistence::backend::InMemoryBackend::new();
        let engine = PersistenceEngine::new(Box::new(backend));
        let store = Arc::new(Mutex::new(
            crate::persistence::durable_store::DurableStore::new(engine),
        ));

        // Pre-populate state with batch already processed
        let mut initial_state = RuntimeIngestionState::default();
        let batch_id = { let mut b = [0u8; 32]; b[0] = 1; b };
        initial_state.record_success(batch_id);
        let state = Arc::new(Mutex::new(initial_state));

        let state_clone = state.clone();
        let store_clone = store.clone();
        let handle = tokio::spawn(async move {
            run_runtime_ingestion(receiver, store_clone, state_clone).await;
        });

        // Send the same batch again
        sender.deliver(make_envelope(1)).await.unwrap();
        tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;

        drop(sender);
        handle.await.unwrap();

        // Should still be 1 (not re-processed)
        let st = state.lock().await;
        assert_eq!(st.batches_ingested, 1);
    }
}
