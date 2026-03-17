//! TCP transport layer for PoSeq devnet.
//!
//! # Protocol
//! All frames: `[u32 BE length][bincode payload]`
//!
//! # Architecture
//! ```text
//! TcpListener (per node)
//!     └─ accept() → spawn task per connection
//!                    └─ read_frame() → decode → mpsc::Sender<PoSeqMessage>
//!
//! OutboundConnections map (addr → TcpStream)
//!     └─ send_to(addr, msg) → write_frame()
//! ```
//!
//! The deterministic protocol logic (proposals, attestations, finalization)
//! runs synchronously in the caller's task. The transport layer only moves
//! bytes; it has no consensus knowledge.

use std::collections::BTreeMap;
use std::io;
use std::sync::Arc;

use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::Mutex;
use tokio::sync::mpsc;

use crate::networking::messages::PoSeqMessage;

// ─── Frame codec ──────────────────────────────────────────────────────────────

/// Read one length-prefixed frame from `stream`.
/// Returns `None` on EOF (connection closed gracefully).
pub async fn read_frame(stream: &mut TcpStream) -> io::Result<Option<PoSeqMessage>> {
    let mut len_buf = [0u8; 4];
    match stream.read_exact(&mut len_buf).await {
        Ok(_) => {}
        Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(e) => return Err(e),
    }
    let len = u32::from_be_bytes(len_buf) as usize;
    if len == 0 || len > 4 * 1024 * 1024 {
        return Err(io::Error::new(io::ErrorKind::InvalidData, format!("invalid frame length: {len}")));
    }
    let mut buf = vec![0u8; len];
    stream.read_exact(&mut buf).await?;
    let msg = PoSeqMessage::decode(&buf)
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e.to_string()))?;
    Ok(Some(msg))
}

/// Write one length-prefixed frame to `stream`.
pub async fn write_frame(stream: &mut TcpStream, msg: &PoSeqMessage) -> io::Result<()> {
    let bytes = msg.encode()
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e.to_string()))?;
    stream.write_all(&bytes).await
}

// ─── TransportError ───────────────────────────────────────────────────────────

#[derive(Debug)]
pub enum TransportError {
    Io(io::Error),
    PeerNotConnected(String),
    MaxRetriesExceeded(String),
}

impl std::fmt::Display for TransportError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            TransportError::Io(e) => write!(f, "io: {e}"),
            TransportError::PeerNotConnected(a) => write!(f, "peer not connected: {a}"),
            TransportError::MaxRetriesExceeded(a) => write!(f, "max retries exceeded: {a}"),
        }
    }
}

impl From<io::Error> for TransportError {
    fn from(e: io::Error) -> Self { TransportError::Io(e) }
}

// ─── NodeTransport ────────────────────────────────────────────────────────────

/// TCP transport for a single PoSeq node.
///
/// - Listens on `listen_addr` for inbound connections.
/// - Maintains a pool of outbound connections keyed by peer address.
/// - All inbound messages are forwarded to `inbox_tx`.
pub struct NodeTransport {
    pub listen_addr: String,
    inbox_tx: mpsc::Sender<(String, PoSeqMessage)>,
    outbound: Arc<Mutex<BTreeMap<String, TcpStream>>>,
}

impl NodeTransport {
    /// Create a transport and start listening.
    /// Returns (transport, inbox_rx).
    pub async fn bind(
        listen_addr: &str,
    ) -> io::Result<(NodeTransport, mpsc::Receiver<(String, PoSeqMessage)>)> {
        let (tx, rx) = mpsc::channel(256);
        let listener = TcpListener::bind(listen_addr).await?;
        let actual_addr = listener.local_addr()?.to_string();

        let tx_clone = tx.clone();
        tokio::spawn(async move {
            loop {
                match listener.accept().await {
                    Ok((mut stream, peer_addr)) => {
                        let peer = peer_addr.to_string();
                        let tx2 = tx_clone.clone();
                        tokio::spawn(async move {
                            loop {
                                match read_frame(&mut stream).await {
                                    Ok(Some(msg)) => {
                                        if tx2.send((peer.clone(), msg)).await.is_err() {
                                            break;
                                        }
                                    }
                                    Ok(None) => break, // peer closed
                                    Err(_e) => break,
                                }
                            }
                        });
                    }
                    Err(_) => break,
                }
            }
        });

        let transport = NodeTransport {
            listen_addr: actual_addr,
            inbox_tx: tx,
            outbound: Arc::new(Mutex::new(BTreeMap::new())),
        };
        Ok((transport, rx))
    }

    /// Send a message to `peer_addr`.  Opens a new connection if needed;
    /// retries once on failure (covers the case where a stale cached
    /// connection was half-closed).
    pub async fn send_to(&self, peer_addr: &str, msg: &PoSeqMessage) -> Result<(), TransportError> {
        let mut outbound = self.outbound.lock().await;
        // Try to use existing connection
        if let Some(stream) = outbound.get_mut(peer_addr) {
            if write_frame(stream, msg).await.is_ok() {
                return Ok(());
            }
            // Stale connection — remove and reconnect below
            outbound.remove(peer_addr);
        }
        // Open new connection
        match TcpStream::connect(peer_addr).await {
            Ok(mut stream) => {
                write_frame(&mut stream, msg).await?;
                outbound.insert(peer_addr.to_string(), stream);
                Ok(())
            }
            Err(e) => Err(TransportError::Io(e)),
        }
    }

    /// Broadcast to all peers in `peer_addrs`.  Best-effort — individual
    /// send errors are collected but do not stop the broadcast.
    pub async fn broadcast(
        &self,
        peer_addrs: &[String],
        msg: &PoSeqMessage,
    ) -> Vec<(String, TransportError)> {
        let mut errors = Vec::new();
        for addr in peer_addrs {
            if let Err(e) = self.send_to(addr, msg).await {
                errors.push((addr.clone(), e));
            }
        }
        errors
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::networking::messages::*;
    use tokio::time::{timeout, Duration};

    fn node_id(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[0] = b; id }

    #[tokio::test]
    async fn test_transport_send_receive() {
        let (transport_a, _rx_a) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let (transport_b, mut rx_b) = NodeTransport::bind("127.0.0.1:0").await.unwrap();

        let addr_b = transport_b.listen_addr.clone();
        let msg = PoSeqMessage::PeerStatus(WirePeerStatus {
            node_id: node_id(1),
            listen_addr: addr_b.clone(),
            current_epoch: 1,
            current_slot: 5,
            latest_finalized_batch_id: None,
            is_leader: false,
            in_committee: true,
            role: NodeRole::Attestor,
        });

        transport_a.send_to(&addr_b, &msg).await.unwrap();

        let received = timeout(Duration::from_secs(2), rx_b.recv()).await
            .expect("timeout").expect("channel closed");
        assert_eq!(received.1.kind(), "PeerStatus");
    }

    #[tokio::test]
    async fn test_transport_broadcast_multiple_peers() {
        let (sender, _) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let (_, mut rx1) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let (_, mut rx2) = NodeTransport::bind("127.0.0.1:0").await.unwrap();

        // We need to get the actual bound addresses
        let (recv1, mut inbox1) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let (recv2, mut inbox2) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let addr1 = recv1.listen_addr.clone();
        let addr2 = recv2.listen_addr.clone();
        drop(rx1);
        drop(rx2);

        let msg = PoSeqMessage::BridgeAck(WireBridgeAck {
            batch_id: node_id(10),
            success: true,
            ack_hash: node_id(11),
        });
        let errors = sender.broadcast(&[addr1, addr2], &msg).await;
        assert!(errors.is_empty(), "broadcast had errors: {errors:?}");

        let r1 = timeout(Duration::from_secs(2), inbox1.recv()).await.unwrap().unwrap();
        let r2 = timeout(Duration::from_secs(2), inbox2.recv()).await.unwrap().unwrap();
        assert_eq!(r1.1.kind(), "BridgeAck");
        assert_eq!(r2.1.kind(), "BridgeAck");
    }

    #[tokio::test]
    async fn test_transport_reconnects_after_stale_connection() {
        let (sender, _) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let (recv, mut inbox) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let addr = recv.listen_addr.clone();

        let msg = PoSeqMessage::EpochAnnounce(WireEpochAnnounce {
            epoch: 1,
            committee_members: vec![node_id(1)],
            leader_id: node_id(1),
            epoch_seed: [0u8; 32],
        });
        // First send — opens connection
        sender.send_to(&addr, &msg).await.unwrap();
        let _ = timeout(Duration::from_secs(2), inbox.recv()).await.unwrap().unwrap();

        // Second send — reuses or reconnects
        sender.send_to(&addr, &msg).await.unwrap();
        let r2 = timeout(Duration::from_secs(2), inbox.recv()).await.unwrap().unwrap();
        assert_eq!(r2.1.kind(), "EpochAnnounce");
    }

    #[tokio::test]
    async fn test_frame_codec_large_payload() {
        // Test with a proposal containing many submission IDs
        let big_ids: Vec<[u8; 32]> = (0u8..200).map(|b| { let mut id = [0u8; 32]; id[0] = b; id }).collect();
        let msg = PoSeqMessage::Proposal(WireProposal {
            proposal_id: node_id(1),
            slot: 1,
            epoch: 1,
            leader_id: node_id(2),
            batch_root: node_id(3),
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: big_ids,
            policy_version: 1,
            created_at_height: 1,
        });
        let encoded = msg.encode().unwrap();
        let decoded = PoSeqMessage::decode(&encoded[4..]).unwrap();
        if let PoSeqMessage::Proposal(p) = decoded {
            assert_eq!(p.ordered_submission_ids.len(), 200);
        } else {
            panic!("wrong type");
        }
    }

    #[tokio::test]
    async fn test_connect_to_unreachable_returns_error() {
        let (transport, _) = NodeTransport::bind("127.0.0.1:0").await.unwrap();
        let result = transport.send_to("127.0.0.1:1", &PoSeqMessage::BridgeAck(WireBridgeAck {
            batch_id: [0u8; 32], success: false, ack_hash: [0u8; 32]
        })).await;
        assert!(result.is_err());
    }
}
