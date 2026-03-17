//! Peer discovery service for the PoSeq network.
//!
//! # Protocol
//! 1. On startup, the node connects to all configured `seed_peers` and sends
//!    a `WireDiscoveryRequest` (which is the node's own `WirePeerStatus`).
//! 2. The seed node responds with a `WirePeerList` containing its known peers.
//! 3. The requesting node merges new peers into its `PeerManager` and
//!    connects to them (repeating the discovery exchange if desired).
//!
//! # Anti-gossip amplification
//! A node only requests its peer list once per seed per startup.  It does NOT
//! automatically re-broadcast the received list to further nodes; each node
//! does its own seed query.
//!
//! # `WirePeerList` message
//! Transmitted as `PoSeqMessage::PeerList(WirePeerList)`.
//! Contains a bounded list (max 64) of `WirePeerInfo` entries.

use std::sync::Arc;

use tokio::sync::Mutex;
use tokio::time::{sleep, Duration};

use crate::networking::messages::{PoSeqMessage, WirePeerStatus, NodeId};
use crate::networking::peer_manager::PeerManager;
use crate::networking::transport::NodeTransport;
use crate::networking::node_runner::PeerEntry;

/// Maximum number of peers returned in a single `WirePeerList`.
pub const MAX_PEERS_IN_LIST: usize = 64;

// ─── Wire types ────────────────────────────────────────────────────────────────

/// A single peer entry in a `WirePeerList`.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct WirePeerInfo {
    pub node_id: NodeId,
    pub listen_addr: String,
}

/// Sent in response to a discovery request.  Contains up to
/// `MAX_PEERS_IN_LIST` known healthy peers.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct WirePeerList {
    /// Node ID of the responder.
    pub sender_id: NodeId,
    /// Up to 64 known healthy peers.
    pub peers: Vec<WirePeerInfo>,
}

// ─── DiscoveryService ──────────────────────────────────────────────────────────

/// Manages peer discovery on node startup.
pub struct DiscoveryService {
    self_id: NodeId,
    self_addr: String,
    seed_peers: Vec<String>,
    transport: Arc<NodeTransport>,
    peer_manager: Arc<Mutex<PeerManager>>,
}

impl DiscoveryService {
    pub fn new(
        self_id: NodeId,
        self_addr: String,
        seed_peers: Vec<String>,
        transport: Arc<NodeTransport>,
        peer_manager: Arc<Mutex<PeerManager>>,
    ) -> Self {
        DiscoveryService { self_id, self_addr, seed_peers, transport, peer_manager }
    }

    /// Bootstrap: query all seed peers for their peer lists.
    /// New peers are registered in `PeerManager` and can be used immediately.
    ///
    /// Runs once at startup; errors are logged but do not abort the node.
    pub async fn bootstrap(&self, self_status: &WirePeerStatus) {
        if self.seed_peers.is_empty() {
            return;
        }
        println!("[discovery] Bootstrapping from {} seed peer(s)", self.seed_peers.len());

        // Send our status to each seed; they will respond with their peer list.
        // The response is handled by the normal `handle_message` path via the
        // `PeerList` variant — here we just initiate the exchange.
        let status_msg = PoSeqMessage::PeerStatus(self_status.clone());
        for seed_addr in &self.seed_peers {
            if let Err(e) = self.transport.send_to(seed_addr, &status_msg).await {
                println!("[discovery] Failed to contact seed {seed_addr}: {e}");
            } else {
                println!("[discovery] Contacted seed {seed_addr}");
            }
        }

        // Give seeds a moment to respond (their PeerList comes through the inbox)
        sleep(Duration::from_millis(500)).await;
    }

    /// Build a `WirePeerList` from our current `PeerManager` state.
    /// Called when we receive a `PeerStatus` from a new node that wants peers.
    pub async fn build_peer_list(&self) -> WirePeerList {
        let pm = self.peer_manager.lock().await;
        let peers: Vec<WirePeerInfo> = pm.all_peer_addrs()
            .into_iter()
            .zip(pm.all_peer_ids().into_iter())
            .take(MAX_PEERS_IN_LIST)
            .map(|(addr, node_id)| WirePeerInfo { node_id, listen_addr: addr })
            .collect();
        WirePeerList { sender_id: self.self_id, peers }
    }

    /// Merge a received `WirePeerList` into our `PeerManager`.
    /// New peers are registered and available for future connections.
    pub async fn merge_peer_list(&self, list: &WirePeerList) {
        let mut pm = self.peer_manager.lock().await;
        let mut added = 0usize;
        for peer in &list.peers {
            if peer.node_id == self.self_id {
                continue; // skip ourselves
            }
            if !pm.has_peer(&peer.node_id) {
                pm.register_peer(peer.node_id, peer.listen_addr.clone());
                added += 1;
            }
        }
        if added > 0 {
            println!("[discovery] Merged {added} new peer(s) from {}", hex::encode(&list.sender_id[..4]));
        }
    }

    /// Respond to a `PeerStatus` from a new node by sending them our peer list.
    pub async fn respond_with_peer_list(&self, requester_addr: &str) {
        let list = self.build_peer_list().await;
        let msg = PoSeqMessage::PeerList(list);
        if let Err(e) = self.transport.send_to(requester_addr, &msg).await {
            println!("[discovery] Failed to send PeerList to {requester_addr}: {e}");
        }
    }

    /// Convert a `WirePeerList` into `PeerEntry` items (used to bootstrap `NodeConfig.peers`).
    pub fn peer_list_to_entries(list: &WirePeerList) -> Vec<PeerEntry> {
        list.peers.iter().map(|p| PeerEntry {
            node_id: p.node_id,
            addr: p.listen_addr.clone(),
        }).collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_wire_peer_list_roundtrip() {
        let list = WirePeerList {
            sender_id: [1u8; 32],
            peers: vec![
                WirePeerInfo { node_id: [2u8; 32], listen_addr: "127.0.0.1:7002".into() },
                WirePeerInfo { node_id: [3u8; 32], listen_addr: "127.0.0.1:7003".into() },
            ],
        };
        let encoded = bincode::serialize(&list).unwrap();
        let decoded: WirePeerList = bincode::deserialize(&encoded).unwrap();
        assert_eq!(decoded.sender_id, list.sender_id);
        assert_eq!(decoded.peers.len(), 2);
        assert_eq!(decoded.peers[0].listen_addr, "127.0.0.1:7002");
    }

    #[test]
    fn test_wire_peer_list_empty() {
        let list = WirePeerList { sender_id: [0u8; 32], peers: vec![] };
        let encoded = bincode::serialize(&list).unwrap();
        let decoded: WirePeerList = bincode::deserialize(&encoded).unwrap();
        assert!(decoded.peers.is_empty());
    }

    #[test]
    fn test_wire_peer_list_truncated_to_max() {
        // Verify MAX_PEERS_IN_LIST is 64
        assert_eq!(MAX_PEERS_IN_LIST, 64);
        let peers: Vec<WirePeerInfo> = (0..100u8).map(|i| WirePeerInfo {
            node_id: { let mut id = [0u8; 32]; id[0] = i; id },
            listen_addr: format!("127.0.0.1:{}", 7000 + i as u16),
        }).collect();
        // Build peer list manually truncated
        let list = WirePeerList {
            sender_id: [0u8; 32],
            peers: peers.into_iter().take(MAX_PEERS_IN_LIST).collect(),
        };
        assert_eq!(list.peers.len(), MAX_PEERS_IN_LIST);
    }
}
