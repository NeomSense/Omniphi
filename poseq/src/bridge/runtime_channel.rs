//! Async channel from PoSeq node finalization to the Runtime ingester.
//!
//! After a batch reaches quorum and is finalized, the PoSeq node needs to
//! deliver the `FinalizationEnvelope` to the Runtime execution layer.
//! This module provides the channel and conversion plumbing.
//!
//! # Architecture
//!
//! ```text
//!  NetworkedNode                  RuntimeBatchIngester
//!       │                                │
//!       │  finalize batch                │
//!       │──────────────────────────────►│
//!       │  RuntimeDeliverySender.send()  │ ingest_batch()
//!       │                                │
//! ```
//!
//! The `RuntimeDeliverySender` is an `mpsc::Sender<RuntimeDeliveryItem>`.
//! The receiving end is held by the Runtime and polled via `tokio::spawn`.
//!
//! # Construction
//!
//! ```rust,no_run
//! use omniphi_poseq::bridge::runtime_channel::RuntimeDeliveryChannel;
//!
//! let (sender, mut receiver) = RuntimeDeliveryChannel::create(128);
//!
//! // In the runtime process / task:
//! tokio::spawn(async move {
//!     while let Some(item) = receiver.recv().await {
//!         // Convert and ingest
//!     }
//! });
//! ```

use tokio::sync::mpsc;

use crate::bridge::pipeline::FinalizationEnvelope;

/// A single item delivered through the runtime channel.
#[derive(Debug, Clone)]
pub struct RuntimeDeliveryItem {
    pub envelope: FinalizationEnvelope,
    /// Monotonically increasing delivery sequence number (set by the sender).
    pub delivery_seq: u64,
}

/// Sender half of the runtime delivery channel.
/// Clone freely; all clones share the same channel.
#[derive(Clone)]
pub struct RuntimeDeliverySender {
    tx: mpsc::Sender<RuntimeDeliveryItem>,
    seq: std::sync::Arc<std::sync::atomic::AtomicU64>,
}

impl RuntimeDeliverySender {
    /// Send a `FinalizationEnvelope` to the runtime ingester.
    ///
    /// Returns `Err` if the runtime receiver has been dropped (node is shutting down).
    pub async fn deliver(&self, envelope: FinalizationEnvelope) -> Result<(), DeliveryError> {
        let seq = self.seq.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        let item = RuntimeDeliveryItem { envelope, delivery_seq: seq };
        self.tx.send(item).await.map_err(|_| DeliveryError::ReceiverDropped)
    }

    /// Deliver without blocking (returns immediately if channel is full).
    pub fn try_deliver(&self, envelope: FinalizationEnvelope) -> Result<(), DeliveryError> {
        let seq = self.seq.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        let item = RuntimeDeliveryItem { envelope, delivery_seq: seq };
        self.tx.try_send(item).map_err(|e| match e {
            mpsc::error::TrySendError::Full(_) => DeliveryError::ChannelFull,
            mpsc::error::TrySendError::Closed(_) => DeliveryError::ReceiverDropped,
        })
    }

    /// Returns `true` if the runtime receiver is still alive.
    pub fn is_connected(&self) -> bool {
        !self.tx.is_closed()
    }
}

/// Receiver half of the runtime delivery channel.
pub struct RuntimeDeliveryReceiver {
    rx: mpsc::Receiver<RuntimeDeliveryItem>,
}

impl RuntimeDeliveryReceiver {
    /// Receive the next delivery.  Returns `None` when all senders are dropped.
    pub async fn recv(&mut self) -> Option<RuntimeDeliveryItem> {
        self.rx.recv().await
    }

    /// Try to receive without blocking.
    pub fn try_recv(&mut self) -> Option<RuntimeDeliveryItem> {
        self.rx.try_recv().ok()
    }
}

/// Factory for creating connected sender/receiver pairs.
pub struct RuntimeDeliveryChannel;

impl RuntimeDeliveryChannel {
    /// Create a new channel with `buffer` capacity.
    pub fn create(buffer: usize) -> (RuntimeDeliverySender, RuntimeDeliveryReceiver) {
        let (tx, rx) = mpsc::channel(buffer);
        let sender = RuntimeDeliverySender {
            tx,
            seq: std::sync::Arc::new(std::sync::atomic::AtomicU64::new(0)),
        };
        let receiver = RuntimeDeliveryReceiver { rx };
        (sender, receiver)
    }
}

/// Errors from the runtime delivery channel.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DeliveryError {
    /// The runtime receiver has been dropped (node is shutting down).
    ReceiverDropped,
    /// The channel buffer is full (backpressure; use `deliver()` to await space).
    ChannelFull,
}

impl std::fmt::Display for DeliveryError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DeliveryError::ReceiverDropped => write!(f, "runtime receiver dropped"),
            DeliveryError::ChannelFull => write!(f, "runtime delivery channel full"),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::bridge::pipeline::{FinalizationEnvelope, FairnessMeta, BatchCommitment};

    fn make_envelope(batch_id: u8) -> FinalizationEnvelope {
        let id = { let mut b = [0u8; 32]; b[0] = batch_id; b };
        let fin_hash = { let mut b = [0u8; 32]; b[1] = batch_id; b };
        let delivery_id = { let mut b = [0u8; 32]; b[2] = batch_id; b };
        let commitment = BatchCommitment::compute(&fin_hash, &delivery_id, &[]);
        FinalizationEnvelope {
            batch_id: id,
            delivery_id,
            attempt_count: 1,
            slot: 1,
            epoch: 1,
            sequence_number: batch_id as u64,
            leader_id: [0u8; 32],
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![],
            batch_root: [0u8; 32],
            finalization_hash: fin_hash,
            quorum_approvals: 2,
            committee_size: 3,
            fairness: FairnessMeta::none(1),
            commitment,
        }
    }

    #[tokio::test]
    async fn test_channel_send_receive() {
        let (sender, mut receiver) = RuntimeDeliveryChannel::create(8);
        sender.deliver(make_envelope(1)).await.unwrap();
        sender.deliver(make_envelope(2)).await.unwrap();

        let item1 = receiver.recv().await.unwrap();
        assert_eq!(item1.envelope.batch_id[0], 1);
        assert_eq!(item1.delivery_seq, 0);

        let item2 = receiver.recv().await.unwrap();
        assert_eq!(item2.envelope.batch_id[0], 2);
        assert_eq!(item2.delivery_seq, 1);
    }

    #[tokio::test]
    async fn test_receiver_dropped_returns_error() {
        let (sender, receiver) = RuntimeDeliveryChannel::create(4);
        drop(receiver);
        let result = sender.deliver(make_envelope(1)).await;
        assert_eq!(result, Err(DeliveryError::ReceiverDropped));
    }

    #[tokio::test]
    async fn test_try_deliver_full_returns_error() {
        let (sender, _receiver) = RuntimeDeliveryChannel::create(1);
        // Fill the buffer
        sender.try_deliver(make_envelope(1)).unwrap();
        // Now buffer is full
        let result = sender.try_deliver(make_envelope(2));
        assert_eq!(result, Err(DeliveryError::ChannelFull));
    }

    #[tokio::test]
    async fn test_is_connected_true_when_receiver_alive() {
        let (sender, _receiver) = RuntimeDeliveryChannel::create(4);
        assert!(sender.is_connected());
    }

    #[tokio::test]
    async fn test_is_connected_false_after_receiver_drop() {
        let (sender, receiver) = RuntimeDeliveryChannel::create(4);
        drop(receiver);
        assert!(!sender.is_connected());
    }

    #[test]
    fn test_delivery_seq_monotone() {
        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let (sender, mut receiver) = RuntimeDeliveryChannel::create(16);
            for i in 0u8..5 {
                sender.deliver(make_envelope(i)).await.unwrap();
            }
            let mut seqs = Vec::new();
            for _ in 0..5 {
                let item = receiver.recv().await.unwrap();
                seqs.push(item.delivery_seq);
            }
            for w in seqs.windows(2) {
                assert!(w[1] > w[0], "delivery_seq must be monotone");
            }
        });
    }
}
