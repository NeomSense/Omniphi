//! Peer lifecycle manager for the PoSeq devnet.
//!
//! `PeerManager` tracks known peers, their connectivity state, and the
//! last status received from each. It deduplicates incoming messages
//! (same proposal or attestation from multiple forwarding paths) and
//! maintains bounded per-peer queues.
//!
//! # Design
//!
//! - **No implicit consensus**: the manager only tracks connectivity and
//!   deduplication; all finality/quorum decisions happen in the node runner.
//! - **Deterministic ordering**: peer maps use `BTreeMap` so iteration
//!   order is reproducible independent of insertion order.
//! - **No clocks in protocol**: `seen_messages` uses content hashing, not
//!   arrival timestamps, so network reordering cannot affect dedup logic.

#![allow(dead_code)]

use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};

use crate::networking::messages::{NodeId, WirePeerStatus, PoSeqMessage};

// ─── Peer state ────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PeerConnState {
    /// We have received a PeerStatus from this node; connection healthy.
    Connected,
    /// We haven't heard from the peer recently; will retry.
    Degraded,
    /// Peer explicitly disconnected or failed repeatedly.
    Disconnected,
}

#[derive(Debug, Clone)]
pub struct PeerInfo {
    pub node_id: NodeId,
    pub listen_addr: String,
    pub state: PeerConnState,
    pub last_epoch: u64,
    pub last_slot: u64,
    pub in_committee: bool,
    pub is_leader: bool,
    pub latest_finalized_batch_id: Option<[u8; 32]>,
    /// Number of consecutive delivery failures.
    pub failure_count: u32,
    /// When we last received any message from this peer. `None` for statically
    /// registered peers that have never checked in.
    pub last_seen_at: Option<std::time::Instant>,
    /// Exponential backoff for reconnect attempts, in milliseconds.
    pub reconnect_backoff_ms: u64,
}

impl PeerInfo {
    pub fn from_status(status: &WirePeerStatus) -> Self {
        PeerInfo {
            node_id: status.node_id,
            listen_addr: status.listen_addr.clone(),
            state: PeerConnState::Connected,
            last_epoch: status.current_epoch,
            last_slot: status.current_slot,
            in_committee: status.in_committee,
            is_leader: status.is_leader,
            latest_finalized_batch_id: status.latest_finalized_batch_id,
            failure_count: 0,
            last_seen_at: Some(std::time::Instant::now()),
            reconnect_backoff_ms: 1000,
        }
    }
}

// ─── PeerManager ──────────────────────────────────────────────────────────────

/// Manages the known peer set and message deduplication.
pub struct PeerManager {
    /// Our own node ID (excluded from peer set).
    pub self_id: NodeId,
    /// Peers keyed by node_id.
    peers: BTreeMap<NodeId, PeerInfo>,
    /// Deduplication set: SHA256 of (kind, key fields) for recent messages.
    /// Bounded to `max_seen` entries.
    seen_messages: BTreeSet<[u8; 32]>,
    max_seen: usize,
    /// Insertion-order tracker for eviction when `seen_messages` is full.
    seen_order: std::collections::VecDeque<[u8; 32]>,
}

impl PeerManager {
    pub fn new(self_id: NodeId, max_seen: usize) -> Self {
        PeerManager {
            self_id,
            peers: BTreeMap::new(),
            seen_messages: BTreeSet::new(),
            max_seen,
            seen_order: std::collections::VecDeque::new(),
        }
    }

    // ── Peer registry ──────────────────────────────────────────────────────────

    /// Register or update a peer from a received PeerStatus.
    pub fn update_from_status(&mut self, status: &WirePeerStatus) {
        if status.node_id == self.self_id {
            return; // ignore our own reflected messages
        }
        let info = self.peers
            .entry(status.node_id)
            .or_insert_with(|| PeerInfo::from_status(status));
        info.listen_addr = status.listen_addr.clone();
        info.state = PeerConnState::Connected;
        info.last_epoch = status.current_epoch;
        info.last_slot = status.current_slot;
        info.in_committee = status.in_committee;
        info.is_leader = status.is_leader;
        info.latest_finalized_batch_id = status.latest_finalized_batch_id;
        info.failure_count = 0;
        info.last_seen_at = Some(std::time::Instant::now());
        info.reconnect_backoff_ms = 1000;
    }

    /// Manually register a peer with a known address (bootstrap / config).
    pub fn register_peer(&mut self, node_id: NodeId, listen_addr: String) {
        if node_id == self.self_id { return; }
        self.peers.entry(node_id).or_insert_with(|| PeerInfo {
            node_id,
            listen_addr,
            state: PeerConnState::Disconnected,
            last_epoch: 0,
            last_slot: 0,
            in_committee: false,
            is_leader: false,
            latest_finalized_batch_id: None,
            failure_count: 0,
            last_seen_at: None,
            reconnect_backoff_ms: 1000,
        });
    }

    /// Mark a peer as degraded after a send failure.
    pub fn record_send_failure(&mut self, node_id: &NodeId) {
        if let Some(peer) = self.peers.get_mut(node_id) {
            peer.failure_count += 1;
            if peer.failure_count >= 3 {
                peer.state = PeerConnState::Degraded;
                // Double the backoff, capped at 60 seconds
                peer.reconnect_backoff_ms = (peer.reconnect_backoff_ms * 2).min(60_000);
            }
        }
    }

    /// Reset a peer's backoff and mark it Connected after a successful reconnect.
    pub fn record_reconnect_success(&mut self, node_id: &NodeId) {
        if let Some(peer) = self.peers.get_mut(node_id) {
            peer.failure_count = 0;
            peer.reconnect_backoff_ms = 1000;
            peer.state = PeerConnState::Connected;
        }
    }

    /// Return the current reconnect backoff for a peer, or 1000ms if unknown.
    pub fn backoff_ms(&self, node_id: &NodeId) -> u64 {
        self.peers.get(node_id).map(|p| p.reconnect_backoff_ms).unwrap_or(1000)
    }

    /// Iterate Connected peers; if `now - last_seen_at > silence_ms`, set to Degraded.
    /// Peers with `last_seen_at = None` (never seen) are left unchanged.
    pub fn tick_health_check(&mut self, now: std::time::Instant, silence_ms: u64) {
        let threshold = std::time::Duration::from_millis(silence_ms);
        for peer in self.peers.values_mut() {
            if peer.state == PeerConnState::Connected {
                if let Some(last_seen) = peer.last_seen_at {
                    if now.duration_since(last_seen) > threshold {
                        peer.state = PeerConnState::Degraded;
                    }
                }
                // peers with last_seen_at = None are skipped
            }
        }
    }

    /// Iterate Degraded peers; if `now - last_seen_at > max_silence_ms`, set to Disconnected.
    /// Peers with `last_seen_at = None` are left unchanged.
    pub fn evict_dead_peers(&mut self, now: std::time::Instant, max_silence_ms: u64) {
        let threshold = std::time::Duration::from_millis(max_silence_ms);
        for peer in self.peers.values_mut() {
            if peer.state == PeerConnState::Degraded {
                if let Some(last_seen) = peer.last_seen_at {
                    if now.duration_since(last_seen) > threshold {
                        peer.state = PeerConnState::Disconnected;
                    }
                }
            }
        }
    }

    /// Return (node_id, listen_addr) pairs for all Disconnected peers.
    pub fn reconnect_candidates(&self) -> Vec<(NodeId, String)> {
        self.peers.values()
            .filter(|p| p.state == PeerConnState::Disconnected)
            .map(|p| (p.node_id, p.listen_addr.clone()))
            .collect()
    }

    /// Mark a peer as disconnected (e.g., max retries exceeded).
    pub fn mark_disconnected(&mut self, node_id: &NodeId) {
        if let Some(peer) = self.peers.get_mut(node_id) {
            peer.state = PeerConnState::Disconnected;
        }
    }

    // ── Queries ────────────────────────────────────────────────────────────────

    pub fn get_peer(&self, node_id: &NodeId) -> Option<&PeerInfo> {
        self.peers.get(node_id)
    }

    /// All peer addresses (for broadcast).
    pub fn all_peer_addrs(&self) -> Vec<String> {
        self.peers.values().map(|p| p.listen_addr.clone()).collect()
    }

    /// Committee member addresses only.
    pub fn committee_peer_addrs(&self) -> Vec<String> {
        self.peers.values()
            .filter(|p| p.in_committee)
            .map(|p| p.listen_addr.clone())
            .collect()
    }

    /// Address of the current leader, if known.
    pub fn leader_addr(&self) -> Option<String> {
        self.peers.values()
            .find(|p| p.is_leader)
            .map(|p| p.listen_addr.clone())
    }

    /// Nodes that are behind (their latest_finalized differs from `current_batch_id`).
    pub fn nodes_needing_sync(&self, current_batch_id: &[u8; 32]) -> Vec<NodeId> {
        self.peers.values()
            .filter(|p| p.latest_finalized_batch_id.as_ref() != Some(current_batch_id))
            .map(|p| p.node_id)
            .collect()
    }

    pub fn peer_count(&self) -> usize {
        self.peers.len()
    }

    pub fn connected_count(&self) -> usize {
        self.peers.values().filter(|p| p.state == PeerConnState::Connected).count()
    }

    pub fn degraded_count(&self) -> usize {
        self.peers.values().filter(|p| p.state == PeerConnState::Degraded).count()
    }

    pub fn disconnected_count(&self) -> usize {
        self.peers.values().filter(|p| p.state == PeerConnState::Disconnected).count()
    }

    /// All peer node IDs (parallel to `all_peer_addrs()`).
    pub fn all_peer_ids(&self) -> Vec<NodeId> {
        self.peers.keys().copied().collect()
    }

    /// Returns `true` if a peer with the given `node_id` is registered.
    pub fn has_peer(&self, node_id: &NodeId) -> bool {
        self.peers.contains_key(node_id)
    }

    // ── Message deduplication ─────────────────────────────────────────────────

    /// Returns `true` if this message has been seen before (should be dropped).
    /// Returns `false` and records the message if it is new.
    pub fn is_duplicate(&mut self, msg: &PoSeqMessage) -> bool {
        let key = Self::dedup_key(msg);
        if self.seen_messages.contains(&key) {
            return true;
        }
        // Evict oldest if at capacity
        if self.seen_messages.len() >= self.max_seen {
            if let Some(oldest) = self.seen_order.pop_front() {
                self.seen_messages.remove(&oldest);
            }
        }
        self.seen_messages.insert(key);
        self.seen_order.push_back(key);
        false
    }

    /// Compute a deduplication key from stable message fields.
    /// Two messages with the same key are considered duplicates.
    fn dedup_key(msg: &PoSeqMessage) -> [u8; 32] {
        let mut h = Sha256::new();
        // Domain tag ensures cross-type collisions are impossible
        h.update(msg.kind().as_bytes());
        match msg {
            PoSeqMessage::Proposal(p) => {
                h.update(p.proposal_id);
                h.update(p.slot.to_be_bytes());
                h.update(p.epoch.to_be_bytes());
                h.update(p.leader_id);
            }
            PoSeqMessage::Attestation(a) => {
                h.update(a.attestor_id);
                h.update(a.proposal_id);
                h.update([a.approve as u8]);
            }
            PoSeqMessage::Finalized(f) => {
                h.update(f.batch_id);
                h.update(f.epoch.to_be_bytes());
            }
            PoSeqMessage::SyncRequest(r) => {
                h.update(r.requesting_node);
                h.update(r.batch_id);
            }
            PoSeqMessage::SyncResponse(r) => {
                h.update(r.responding_node);
                h.update(r.batch_id);
            }
            PoSeqMessage::PeerStatus(s) => {
                h.update(s.node_id);
                h.update(s.current_epoch.to_be_bytes());
                h.update(s.current_slot.to_be_bytes());
            }
            PoSeqMessage::EpochAnnounce(e) => {
                h.update(e.epoch.to_be_bytes());
                h.update(e.leader_id);
            }
            PoSeqMessage::BridgeAck(a) => {
                h.update(a.batch_id);
                h.update(a.ack_hash);
            }
            PoSeqMessage::MisbehaviorReport(r) => {
                h.update(r.reporter_id);
                h.update(r.accused_id);
                h.update(r.evidence_hash);
            }
            PoSeqMessage::CheckpointAnnounce(c) => {
                h.update(c.node_id);
                h.update(c.checkpoint_id);
            }
            PoSeqMessage::PeerList(pl) => {
                h.update(pl.sender_id);
                h.update((pl.peers.len() as u64).to_be_bytes());
            }
            PoSeqMessage::Signed(env) => {
                // Dedup on the inner bytes hash to avoid signed/unsigned collision
                h.update(&env.inner_bytes);
                h.update(env.signer_id);
            }
            PoSeqMessage::HotStuffBlock(b) => {
                h.update(b.block_id);
                h.update(b.view.to_be_bytes());
                h.update(b.leader_id);
            }
            PoSeqMessage::HotStuffVote(v) => {
                h.update(v.voter_id);
                h.update(v.block_id);
                h.update(v.view.to_be_bytes());
                h.update([v.phase as u8]);
            }
            PoSeqMessage::HotStuffQC(qc) => {
                h.update(qc.block_id);
                h.update(qc.view.to_be_bytes());
                h.update([qc.phase as u8]);
            }
            PoSeqMessage::HotStuffNewView(nv) => {
                h.update(nv.sender_id);
                h.update(nv.new_view.to_be_bytes());
            }
            PoSeqMessage::SequencerRegistered(reg) => {
                h.update(reg.record.node_id);
                h.update(reg.block_height.to_be_bytes());
            }
        }
        h.finalize().into()
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::networking::messages::*;

    fn nid(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_status(node_id: NodeId, addr: &str, epoch: u64, slot: u64) -> WirePeerStatus {
        WirePeerStatus {
            node_id,
            listen_addr: addr.to_string(),
            current_epoch: epoch,
            current_slot: slot,
            latest_finalized_batch_id: None,
            is_leader: false,
            in_committee: true,
            role: NodeRole::Attestor,
            protocol_version: None,
        }
    }

    #[test]
    fn test_update_from_status_registers_peer() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 5));
        assert_eq!(pm.peer_count(), 1);
        assert_eq!(pm.connected_count(), 1);
    }

    #[test]
    fn test_self_id_not_registered() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(0), "127.0.0.1:7000", 1, 1));
        assert_eq!(pm.peer_count(), 0);
    }

    #[test]
    fn test_update_existing_peer() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 5));
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 2, 10));
        let peer = pm.get_peer(&nid(1)).unwrap();
        assert_eq!(peer.last_epoch, 2);
        assert_eq!(peer.last_slot, 10);
    }

    #[test]
    fn test_record_send_failure_degrades_peer() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        for _ in 0..3 {
            pm.record_send_failure(&nid(1));
        }
        let peer = pm.get_peer(&nid(1)).unwrap();
        assert_eq!(peer.state, PeerConnState::Degraded);
    }

    #[test]
    fn test_all_peer_addrs() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        pm.register_peer(nid(2), "127.0.0.1:7002".into());
        let addrs = pm.all_peer_addrs();
        assert_eq!(addrs.len(), 2);
    }

    #[test]
    fn test_committee_peer_addrs_filters() {
        let mut pm = PeerManager::new(nid(0), 100);
        let mut s1 = make_status(nid(1), "127.0.0.1:7001", 1, 1);
        s1.in_committee = true;
        let mut s2 = make_status(nid(2), "127.0.0.1:7002", 1, 1);
        s2.in_committee = false;
        pm.update_from_status(&s1);
        pm.update_from_status(&s2);
        assert_eq!(pm.committee_peer_addrs().len(), 1);
    }

    #[test]
    fn test_leader_addr_returned() {
        let mut pm = PeerManager::new(nid(0), 100);
        let mut s = make_status(nid(1), "127.0.0.1:7001", 1, 1);
        s.is_leader = true;
        pm.update_from_status(&s);
        assert!(pm.leader_addr().is_some());
    }

    #[test]
    fn test_dedup_rejects_same_proposal() {
        let mut pm = PeerManager::new(nid(0), 100);
        let msg = PoSeqMessage::Proposal(WireProposal {
            proposal_id: nid(1),
            slot: 1,
            epoch: 1,
            leader_id: nid(2),
            batch_root: nid(3),
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![],
            policy_version: 1,
            created_at_height: 1,
        });
        assert!(!pm.is_duplicate(&msg));  // first time — new
        assert!(pm.is_duplicate(&msg));   // second time — duplicate
    }

    #[test]
    fn test_dedup_allows_different_proposals() {
        let mut pm = PeerManager::new(nid(0), 100);
        let msg1 = PoSeqMessage::Proposal(WireProposal {
            proposal_id: nid(1), slot: 1, epoch: 1, leader_id: nid(2),
            batch_root: nid(3), parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![], policy_version: 1, created_at_height: 1,
        });
        let msg2 = PoSeqMessage::Proposal(WireProposal {
            proposal_id: nid(99), slot: 2, epoch: 1, leader_id: nid(2),
            batch_root: nid(3), parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![], policy_version: 1, created_at_height: 2,
        });
        assert!(!pm.is_duplicate(&msg1));
        assert!(!pm.is_duplicate(&msg2)); // different proposal
    }

    #[test]
    fn test_dedup_cross_type_no_collision() {
        let mut pm = PeerManager::new(nid(0), 100);
        let proposal = PoSeqMessage::Proposal(WireProposal {
            proposal_id: nid(1), slot: 1, epoch: 1, leader_id: nid(2),
            batch_root: [0u8; 32], parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![], policy_version: 1, created_at_height: 1,
        });
        let attestation = PoSeqMessage::Attestation(WireAttestation {
            attestor_id: nid(1), proposal_id: nid(1), batch_id_attested: nid(2),
            approve: true, epoch: 1, slot: 1,
        });
        assert!(!pm.is_duplicate(&proposal));
        assert!(!pm.is_duplicate(&attestation)); // different type, different key
    }

    #[test]
    fn test_dedup_evicts_at_capacity() {
        let mut pm = PeerManager::new(nid(0), 3); // capacity = 3
        for i in 0u8..4 {
            let msg = PoSeqMessage::BridgeAck(WireBridgeAck {
                batch_id: { let mut id = [0u8; 32]; id[0] = i; id },
                success: true,
                ack_hash: { let mut h = [0u8; 32]; h[0] = i; h },
            });
            assert!(!pm.is_duplicate(&msg));
        }
        // After 4 inserts with capacity 3, the oldest (i=0) is evicted
        // Inserting i=0 again should be treated as new
        let msg0 = PoSeqMessage::BridgeAck(WireBridgeAck {
            batch_id: [0u8; 32],
            success: true,
            ack_hash: [0u8; 32],
        });
        // i=0 was evicted — this should be allowed through
        assert!(!pm.is_duplicate(&msg0));
    }

    #[test]
    fn test_nodes_needing_sync() {
        let mut pm = PeerManager::new(nid(0), 100);
        let mut s = make_status(nid(1), "127.0.0.1:7001", 1, 1);
        s.latest_finalized_batch_id = Some(nid(50));
        pm.update_from_status(&s);
        pm.update_from_status(&make_status(nid(2), "127.0.0.1:7002", 1, 1));

        let current = nid(99);
        let behind = pm.nodes_needing_sync(&current);
        // Both nodes are behind (nid(50) != nid(99), None != nid(99))
        assert_eq!(behind.len(), 2);
    }

    // ── D1: Peer lifecycle hardening tests ───────────────────────────────────

    #[test]
    fn test_tick_health_check_degrades_silent_peer() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 1));
        // peer is Connected after update_from_status

        // Simulate that the peer was last seen 200ms ago by travelling time via
        // a backdated Instant — we can't move Instant::now() directly, so instead
        // we call tick_health_check with a very short silence_ms of 0.
        let now = std::time::Instant::now();
        pm.tick_health_check(now, 0); // any duration > 0 triggers it
        assert_eq!(pm.degraded_count(), 1);
        assert_eq!(pm.connected_count(), 0);
    }

    #[test]
    fn test_tick_health_check_ignores_never_seen_peers() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        // last_seen_at is None, state is Disconnected — tick should not change it to Degraded
        let now = std::time::Instant::now();
        pm.tick_health_check(now, 0);
        // Still Disconnected (not Degraded) because last_seen_at is None
        assert_eq!(pm.degraded_count(), 0);
        assert_eq!(pm.disconnected_count(), 1);
    }

    #[test]
    fn test_evict_dead_peers() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 1));
        let now = std::time::Instant::now();
        // Degrade first with silence_ms=0
        pm.tick_health_check(now, 0);
        assert_eq!(pm.degraded_count(), 1);
        // Then evict with max_silence_ms=0
        pm.evict_dead_peers(now, 0);
        assert_eq!(pm.degraded_count(), 0);
        assert_eq!(pm.disconnected_count(), 1);
    }

    #[test]
    fn test_reconnect_candidates() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        pm.register_peer(nid(2), "127.0.0.1:7002".into());
        pm.update_from_status(&make_status(nid(3), "127.0.0.1:7003", 1, 1)); // Connected

        let candidates = pm.reconnect_candidates();
        assert_eq!(candidates.len(), 2); // only Disconnected peers
        // Connected peer (nid(3)) should not be in candidates
        assert!(!candidates.iter().any(|(id, _)| *id == nid(3)));
    }

    #[test]
    fn test_backoff_doubling_on_failure() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        assert_eq!(pm.backoff_ms(&nid(1)), 1000);

        // 3 failures trigger Degraded and double the backoff
        for _ in 0..3 {
            pm.record_send_failure(&nid(1));
        }
        assert_eq!(pm.get_peer(&nid(1)).unwrap().state, PeerConnState::Degraded);
        // After 3rd failure: 1000 * 2 = 2000
        assert_eq!(pm.backoff_ms(&nid(1)), 2000);
    }

    #[test]
    fn test_backoff_caps_at_60000() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        // Manually set a large backoff to test the cap
        pm.peers.get_mut(&nid(1)).unwrap().reconnect_backoff_ms = 40_000;
        pm.peers.get_mut(&nid(1)).unwrap().failure_count = 2;
        pm.record_send_failure(&nid(1)); // would be 80_000 but capped at 60_000
        assert_eq!(pm.backoff_ms(&nid(1)), 60_000);
    }

    #[test]
    fn test_record_reconnect_success_resets() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.register_peer(nid(1), "127.0.0.1:7001".into());
        for _ in 0..3 {
            pm.record_send_failure(&nid(1));
        }
        assert_eq!(pm.get_peer(&nid(1)).unwrap().state, PeerConnState::Degraded);
        pm.record_reconnect_success(&nid(1));
        let peer = pm.get_peer(&nid(1)).unwrap();
        assert_eq!(peer.state, PeerConnState::Connected);
        assert_eq!(peer.failure_count, 0);
        assert_eq!(peer.reconnect_backoff_ms, 1000);
    }

    #[test]
    fn test_state_counts() {
        let mut pm = PeerManager::new(nid(0), 100);
        pm.update_from_status(&make_status(nid(1), "127.0.0.1:7001", 1, 1)); // Connected
        pm.register_peer(nid(2), "127.0.0.1:7002".into());                   // Disconnected
        pm.register_peer(nid(3), "127.0.0.1:7003".into());                   // Disconnected

        assert_eq!(pm.connected_count(), 1);
        assert_eq!(pm.degraded_count(), 0);
        assert_eq!(pm.disconnected_count(), 2);
    }
}
