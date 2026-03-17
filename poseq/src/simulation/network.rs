use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::slashing::offenses::SlashRecord;

/// A proposed batch in the simulation (simplified — no full PoSeqNode dependency).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SimProposal {
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub batch_root: [u8; 32],
    pub submission_ids: Vec<[u8; 32]>,
}

impl SimProposal {
    pub fn new(slot: u64, epoch: u64, leader_id: [u8; 32], submission_ids: Vec<[u8; 32]>) -> Self {
        let batch_root = Self::compute_root(&submission_ids);
        let proposal_id = Self::compute_id(slot, epoch, &leader_id, &batch_root);
        SimProposal { proposal_id, slot, epoch, leader_id, batch_root, submission_ids }
    }

    fn compute_root(ids: &[[u8; 32]]) -> [u8; 32] {
        let mut sorted = ids.to_vec();
        sorted.sort();
        let mut h = Sha256::new();
        for id in &sorted { h.update(id); }
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    fn compute_id(slot: u64, epoch: u64, leader_id: &[u8; 32], batch_root: &[u8; 32]) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&slot.to_be_bytes());
        h.update(&epoch.to_be_bytes());
        h.update(leader_id);
        h.update(batch_root);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

/// A finalized batch in the simulation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SimFinalizedBatch {
    pub batch_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub epoch: u64,
    pub slot: u64,
    pub leader_id: [u8; 32],
    pub approvals: usize,
    pub total_attestors: usize,
}

impl SimFinalizedBatch {
    pub fn new(proposal: &SimProposal, approvals: usize, total_attestors: usize) -> Self {
        let mut h = Sha256::new();
        h.update(&proposal.proposal_id);
        h.update(&approvals.to_be_bytes());
        h.update(&total_attestors.to_be_bytes());
        let r = h.finalize();
        let mut batch_id = [0u8; 32];
        batch_id.copy_from_slice(&r);
        SimFinalizedBatch {
            batch_id,
            proposal_id: proposal.proposal_id,
            epoch: proposal.epoch,
            slot: proposal.slot,
            leader_id: proposal.leader_id,
            approvals,
            total_attestors,
        }
    }
}

/// A simulated attestation vote.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SimAttestation {
    pub proposal_id: [u8; 32],
    pub attestor_id: [u8; 32],
    pub approved: bool,
    pub epoch: u64,
}

impl SimAttestation {
    pub fn new(proposal_id: [u8; 32], attestor_id: [u8; 32], approved: bool, epoch: u64) -> Self {
        SimAttestation { proposal_id, attestor_id, approved, epoch }
    }
}

/// Messages that can be sent between simulated nodes.
#[derive(Debug, Clone)]
pub enum NetworkMessage {
    Proposal(SimProposal),
    Attestation(SimAttestation),
    FinalizedBatch(SimFinalizedBatch),
    SlashReport(SlashRecord),
}

impl NetworkMessage {
    pub fn kind(&self) -> &'static str {
        match self {
            NetworkMessage::Proposal(_) => "proposal",
            NetworkMessage::Attestation(_) => "attestation",
            NetworkMessage::FinalizedBatch(_) => "finalized_batch",
            NetworkMessage::SlashReport(_) => "slash_report",
        }
    }
}

/// A simple message bus for the simulation network.
/// Maps node_id → inbox (Vec<NetworkMessage>).
pub struct SimulationNetwork {
    /// node_id → incoming messages
    pub inboxes: BTreeMap<[u8; 32], Vec<NetworkMessage>>,
    /// All nodes registered in the network.
    pub registered_nodes: std::collections::BTreeSet<[u8; 32]>,
    /// Message count for statistics.
    pub total_sent: usize,
}

impl SimulationNetwork {
    pub fn new() -> Self {
        SimulationNetwork {
            inboxes: BTreeMap::new(),
            registered_nodes: std::collections::BTreeSet::new(),
            total_sent: 0,
        }
    }

    /// Register a node so it can receive messages.
    pub fn register_node(&mut self, node_id: [u8; 32]) {
        self.registered_nodes.insert(node_id);
        self.inboxes.entry(node_id).or_insert_with(Vec::new);
    }

    /// Deregister a node (e.g., after crash).
    pub fn deregister_node(&mut self, node_id: &[u8; 32]) {
        self.registered_nodes.remove(node_id);
    }

    /// Send a message to a specific node.
    pub fn send_to(&mut self, node_id: [u8; 32], msg: NetworkMessage) {
        if let Some(inbox) = self.inboxes.get_mut(&node_id) {
            inbox.push(msg);
            self.total_sent += 1;
        }
    }

    /// Broadcast a message to all registered nodes except the sender.
    pub fn broadcast(&mut self, sender_id: &[u8; 32], msg: NetworkMessage) {
        let targets: Vec<[u8; 32]> = self.registered_nodes
            .iter()
            .filter(|id| *id != sender_id)
            .cloned()
            .collect();
        for target in targets {
            self.send_to(target, msg.clone());
        }
    }

    /// Broadcast to ALL nodes including sender.
    pub fn broadcast_all(&mut self, msg: NetworkMessage) {
        let targets: Vec<[u8; 32]> = self.registered_nodes.iter().cloned().collect();
        for target in targets {
            self.send_to(target, msg.clone());
        }
    }

    /// Drain all messages from a node's inbox.
    pub fn drain_inbox(&mut self, node_id: &[u8; 32]) -> Vec<NetworkMessage> {
        self.inboxes.get_mut(node_id)
            .map(|inbox| std::mem::take(inbox))
            .unwrap_or_default()
    }

    /// Peek at a node's inbox without draining.
    pub fn peek_inbox(&self, node_id: &[u8; 32]) -> &[NetworkMessage] {
        self.inboxes.get(node_id)
            .map(|v| v.as_slice())
            .unwrap_or(&[])
    }

    /// Count pending messages for a node.
    pub fn pending_count(&self, node_id: &[u8; 32]) -> usize {
        self.inboxes.get(node_id).map(|v| v.len()).unwrap_or(0)
    }

    /// Total registered nodes.
    pub fn node_count(&self) -> usize {
        self.registered_nodes.len()
    }
}

impl Default for SimulationNetwork {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::slashing::offenses::{SlashRecord, SlashableOffense};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_proposal(slot: u64, leader: u8) -> SimProposal {
        SimProposal::new(slot, 1, make_id(leader), vec![make_id(10), make_id(11)])
    }

    fn make_slash(node: u8) -> SlashRecord {
        SlashRecord::new(SlashableOffense::DoubleProposal, make_id(node), 1, b"ev", 500)
    }

    #[test]
    fn test_proposal_id_determinism() {
        let p1 = make_proposal(1, 1);
        let p2 = make_proposal(1, 1);
        assert_eq!(p1.proposal_id, p2.proposal_id);
    }

    #[test]
    fn test_proposal_id_differs_by_slot() {
        let p1 = make_proposal(1, 1);
        let p2 = make_proposal(2, 1);
        assert_ne!(p1.proposal_id, p2.proposal_id);
    }

    #[test]
    fn test_network_register_and_send() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        let msg = NetworkMessage::Proposal(make_proposal(1, 99));
        net.send_to(make_id(1), msg);
        assert_eq!(net.pending_count(&make_id(1)), 1);
    }

    #[test]
    fn test_broadcast_excludes_sender() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        net.register_node(make_id(2));
        net.register_node(make_id(3));
        let msg = NetworkMessage::Proposal(make_proposal(1, 1));
        net.broadcast(&make_id(1), msg);
        assert_eq!(net.pending_count(&make_id(1)), 0);
        assert_eq!(net.pending_count(&make_id(2)), 1);
        assert_eq!(net.pending_count(&make_id(3)), 1);
    }

    #[test]
    fn test_broadcast_all_includes_all() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        net.register_node(make_id(2));
        let msg = NetworkMessage::Proposal(make_proposal(1, 1));
        net.broadcast_all(msg);
        assert_eq!(net.pending_count(&make_id(1)), 1);
        assert_eq!(net.pending_count(&make_id(2)), 1);
    }

    #[test]
    fn test_drain_inbox() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        net.send_to(make_id(1), NetworkMessage::Proposal(make_proposal(1, 1)));
        net.send_to(make_id(1), NetworkMessage::Proposal(make_proposal(2, 1)));
        let drained = net.drain_inbox(&make_id(1));
        assert_eq!(drained.len(), 2);
        assert_eq!(net.pending_count(&make_id(1)), 0);
    }

    #[test]
    fn test_send_to_unregistered_is_noop() {
        let mut net = SimulationNetwork::new();
        let msg = NetworkMessage::Proposal(make_proposal(1, 1));
        net.send_to(make_id(99), msg); // no registration → no panic
        assert_eq!(net.total_sent, 0);
    }

    #[test]
    fn test_slash_report_message() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        let sr = make_slash(5);
        net.send_to(make_id(1), NetworkMessage::SlashReport(sr));
        let inbox = net.drain_inbox(&make_id(1));
        assert_eq!(inbox.len(), 1);
        assert_eq!(inbox[0].kind(), "slash_report");
    }

    #[test]
    fn test_message_kind_labels() {
        let proposal = NetworkMessage::Proposal(make_proposal(1, 1));
        let attest = NetworkMessage::Attestation(SimAttestation::new(make_id(1), make_id(2), true, 1));
        let fb = NetworkMessage::FinalizedBatch(SimFinalizedBatch::new(&make_proposal(1, 1), 3, 5));
        let slash = NetworkMessage::SlashReport(make_slash(1));
        assert_eq!(proposal.kind(), "proposal");
        assert_eq!(attest.kind(), "attestation");
        assert_eq!(fb.kind(), "finalized_batch");
        assert_eq!(slash.kind(), "slash_report");
    }

    #[test]
    fn test_node_count() {
        let mut net = SimulationNetwork::new();
        assert_eq!(net.node_count(), 0);
        net.register_node(make_id(1));
        net.register_node(make_id(2));
        assert_eq!(net.node_count(), 2);
        net.deregister_node(&make_id(1));
        assert_eq!(net.node_count(), 1);
    }

    #[test]
    fn test_total_sent_counter() {
        let mut net = SimulationNetwork::new();
        net.register_node(make_id(1));
        net.register_node(make_id(2));
        let msg = NetworkMessage::Proposal(make_proposal(1, 1));
        net.broadcast(&make_id(99), msg); // 99 is sender; sends to 1 and 2
        assert_eq!(net.total_sent, 2);
    }
}
