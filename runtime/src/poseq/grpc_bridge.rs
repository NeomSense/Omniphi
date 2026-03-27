//! gRPC Bridge Protocol — Replaces File-Based Export/Import
//!
//! Defines the message types and service interface for the PoSeq ↔ Runtime
//! ↔ Go Chain bridge. This replaces the file-based JSON export with a
//! proper RPC protocol suitable for production deployment.
//!
//! ## Architecture
//!
//! ```text
//!  PoSeq Node                    Runtime                        Go Chain
//!  ──────────                    ───────                        ────────
//!  Finalize batch ──(gRPC)──→ IngestBatch()
//!                              Validate + Execute
//!                              ←─(gRPC)──── AckBatch()   ←── Consensus commit
//!
//!  ←──(gRPC)──── CommitteeUpdate()                       ──→ Epoch transition
//! ```
//!
//! ## Why gRPC Over Files
//!
//! - **Latency**: File polling adds 100-500ms. gRPC: <5ms.
//! - **Reliability**: File writes can be partial. gRPC has request/response semantics.
//! - **Backpressure**: gRPC supports flow control. Files do not.
//! - **Observability**: gRPC has built-in metrics, tracing, deadlines.
//! - **Multiplexing**: Single connection for all message types.

use serde::{Deserialize, Serialize};

/// A finalized batch from PoSeq, ready for runtime ingestion.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchDelivery {
    /// Unique batch identifier.
    pub batch_id: [u8; 32],
    /// Monotonically increasing sequence number.
    pub sequence: u64,
    /// Epoch this batch belongs to.
    pub epoch: u64,
    /// Ordered submission IDs (canonical PoSeq order).
    pub submission_ids: Vec<[u8; 32]>,
    /// SHA256 of the canonical batch content (for ACK verification).
    pub content_hash: [u8; 32],
    /// Timestamp (unix millis) when PoSeq finalized this batch.
    pub finalized_at_ms: u64,
}

/// Acknowledgment from the Go chain after committing a batch.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchAck {
    /// The batch_id being acknowledged.
    pub batch_id: [u8; 32],
    /// The sequence number being acknowledged.
    pub sequence: u64,
    /// Whether the batch was accepted (true) or rejected (false).
    pub accepted: bool,
    /// The state root after applying this batch (if accepted).
    pub state_root: Option<[u8; 32]>,
    /// Block height where this batch was committed.
    pub block_height: Option<u64>,
    /// Rejection reason (if not accepted).
    pub rejection_reason: Option<String>,
}

/// Committee update from the Go chain to PoSeq.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommitteeUpdate {
    /// New epoch number.
    pub epoch: u64,
    /// Validator set: (validator_id, voting_power).
    pub validators: Vec<([u8; 32], u64)>,
    /// Current leader (sequencer) for this epoch.
    pub leader: [u8; 32],
    /// Total voting power.
    pub total_power: u64,
}

/// Health status of the bridge connection.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BridgeHealth {
    pub connected: bool,
    pub last_delivered_sequence: u64,
    pub last_acked_sequence: u64,
    pub pending_batches: usize,
    pub latency_ms: u64,
}

/// The bridge service trait — implemented by both the gRPC server and client.
///
/// In production, this is a tonic gRPC service. The trait allows testing
/// with in-memory implementations.
pub trait BridgeService {
    /// Deliver a finalized batch from PoSeq to the runtime.
    fn deliver_batch(&mut self, batch: BatchDelivery) -> Result<(), String>;

    /// Acknowledge a batch from the Go chain.
    fn ack_batch(&mut self, ack: BatchAck) -> Result<(), String>;

    /// Update the committee composition (epoch transition).
    fn update_committee(&mut self, update: CommitteeUpdate) -> Result<(), String>;

    /// Get bridge health status.
    fn health(&self) -> BridgeHealth;
}

/// In-memory bridge implementation for testing and single-process deployment.
#[derive(Debug, Default)]
pub struct InMemoryBridge {
    delivered: Vec<BatchDelivery>,
    acks: Vec<BatchAck>,
    latest_committee: Option<CommitteeUpdate>,
    last_sequence: u64,
    last_acked: u64,
}

impl InMemoryBridge {
    pub fn new() -> Self { InMemoryBridge::default() }

    pub fn delivered_count(&self) -> usize { self.delivered.len() }
    pub fn ack_count(&self) -> usize { self.acks.len() }
}

impl BridgeService for InMemoryBridge {
    fn deliver_batch(&mut self, batch: BatchDelivery) -> Result<(), String> {
        if batch.sequence <= self.last_sequence && self.last_sequence > 0 {
            return Err(format!(
                "duplicate or out-of-order batch: got seq {}, last was {}",
                batch.sequence, self.last_sequence
            ));
        }
        self.last_sequence = batch.sequence;
        self.delivered.push(batch);
        Ok(())
    }

    fn ack_batch(&mut self, ack: BatchAck) -> Result<(), String> {
        self.last_acked = ack.sequence;
        self.acks.push(ack);
        Ok(())
    }

    fn update_committee(&mut self, update: CommitteeUpdate) -> Result<(), String> {
        self.latest_committee = Some(update);
        Ok(())
    }

    fn health(&self) -> BridgeHealth {
        BridgeHealth {
            connected: true,
            last_delivered_sequence: self.last_sequence,
            last_acked_sequence: self.last_acked,
            pending_batches: (self.last_sequence - self.last_acked) as usize,
            latency_ms: 0,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_batch(seq: u64) -> BatchDelivery {
        BatchDelivery {
            batch_id: { let mut b = [0u8; 32]; b[0..8].copy_from_slice(&seq.to_be_bytes()); b },
            sequence: seq,
            epoch: 1,
            submission_ids: vec![[0xAA; 32]],
            content_hash: [0xBB; 32],
            finalized_at_ms: 1000,
        }
    }

    #[test]
    fn test_deliver_and_ack() {
        let mut bridge = InMemoryBridge::new();
        bridge.deliver_batch(make_batch(1)).unwrap();
        bridge.deliver_batch(make_batch(2)).unwrap();
        assert_eq!(bridge.delivered_count(), 2);

        bridge.ack_batch(BatchAck {
            batch_id: [0u8; 32], sequence: 1, accepted: true,
            state_root: Some([0xCC; 32]), block_height: Some(100),
            rejection_reason: None,
        }).unwrap();

        let health = bridge.health();
        assert!(health.connected);
        assert_eq!(health.last_delivered_sequence, 2);
        assert_eq!(health.last_acked_sequence, 1);
        assert_eq!(health.pending_batches, 1);
    }

    #[test]
    fn test_duplicate_batch_rejected() {
        let mut bridge = InMemoryBridge::new();
        bridge.deliver_batch(make_batch(1)).unwrap();
        let err = bridge.deliver_batch(make_batch(1)).unwrap_err();
        assert!(err.contains("duplicate"));
    }

    #[test]
    fn test_out_of_order_rejected() {
        let mut bridge = InMemoryBridge::new();
        bridge.deliver_batch(make_batch(5)).unwrap();
        let err = bridge.deliver_batch(make_batch(3)).unwrap_err();
        assert!(err.contains("out-of-order"));
    }

    #[test]
    fn test_committee_update() {
        let mut bridge = InMemoryBridge::new();
        bridge.update_committee(CommitteeUpdate {
            epoch: 10,
            validators: vec![([1u8; 32], 100), ([2u8; 32], 200)],
            leader: [1u8; 32],
            total_power: 300,
        }).unwrap();
        assert_eq!(bridge.latest_committee.unwrap().epoch, 10);
    }
}
