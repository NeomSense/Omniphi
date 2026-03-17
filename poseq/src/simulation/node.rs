use std::collections::{BTreeMap, BTreeSet};
use crate::slashing::offenses::SlashRecord;

/// State of a simulated node in the network.
#[derive(Debug, Clone)]
pub struct SimulatedNode {
    pub node_id: [u8; 32],
    pub is_leader: bool,
    /// Whether this node is in the current committee.
    pub in_committee: bool,
    /// Proposals this node has seen (proposal_id → slot).
    pub proposals_seen: BTreeMap<[u8; 32], u64>,
    /// Attestation hashes this node has sent (proposal_id → approved).
    pub attestations_sent: BTreeMap<[u8; 32], bool>,
    /// Finalized batch IDs this node has received.
    pub finalized_batches: BTreeSet<[u8; 32]>,
    /// Slash records affecting this node.
    pub slash_records: Vec<SlashRecord>,
    /// Is this node currently jailed?
    pub is_jailed: bool,
    /// Cumulative slash bps.
    pub cumulative_slash_bps: u64,
    /// Whether this node has crashed (simulating leader crash scenario).
    pub crashed: bool,
    /// Stake weight for this node (used in threshold calculations).
    pub stake_weight: u64,
}

impl SimulatedNode {
    pub fn new(node_id: [u8; 32], stake_weight: u64) -> Self {
        SimulatedNode {
            node_id,
            is_leader: false,
            in_committee: false,
            proposals_seen: BTreeMap::new(),
            attestations_sent: BTreeMap::new(),
            finalized_batches: BTreeSet::new(),
            slash_records: Vec::new(),
            is_jailed: false,
            cumulative_slash_bps: 0,
            crashed: false,
            stake_weight,
        }
    }

    /// Record that this node has seen a proposal.
    pub fn observe_proposal(&mut self, proposal_id: [u8; 32], slot: u64) {
        self.proposals_seen.insert(proposal_id, slot);
    }

    /// Record that this node sent an attestation (approve/reject).
    pub fn record_attestation(&mut self, proposal_id: [u8; 32], approved: bool) {
        self.attestations_sent.insert(proposal_id, approved);
    }

    /// Record that a batch was finalized.
    pub fn receive_finalized(&mut self, batch_id: [u8; 32]) {
        self.finalized_batches.insert(batch_id);
    }

    /// Apply a slash record to this node.
    pub fn apply_slash(&mut self, record: SlashRecord) {
        self.cumulative_slash_bps = self.cumulative_slash_bps.saturating_add(record.slash_amount_bps);
        self.slash_records.push(record);
    }

    /// Jail this node.
    pub fn jail(&mut self) {
        self.is_jailed = true;
    }

    /// Unjail this node and reset slash.
    pub fn unjail(&mut self) {
        self.is_jailed = false;
        self.cumulative_slash_bps = 0;
    }

    /// Number of proposals seen.
    pub fn proposals_seen_count(&self) -> usize {
        self.proposals_seen.len()
    }

    /// Number of attestations sent.
    pub fn attestations_sent_count(&self) -> usize {
        self.attestations_sent.len()
    }

    /// Number of finalized batches received.
    pub fn finalized_count(&self) -> usize {
        self.finalized_batches.len()
    }

    /// Whether this node can participate (not crashed, not jailed, in committee).
    pub fn can_participate(&self) -> bool {
        !self.crashed && !self.is_jailed && self.in_committee
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

    fn make_slash_record(node_id: [u8; 32], bps: u64) -> SlashRecord {
        SlashRecord::new(SlashableOffense::AbsentFromDuty, node_id, 1, b"ev", bps)
    }

    #[test]
    fn test_node_creation() {
        let node = SimulatedNode::new(make_id(1), 1000);
        assert_eq!(node.node_id, make_id(1));
        assert_eq!(node.stake_weight, 1000);
        assert!(!node.is_leader);
        assert!(!node.in_committee);
        assert!(!node.is_jailed);
        assert!(!node.crashed);
    }

    #[test]
    fn test_observe_proposal() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.observe_proposal(make_id(10), 5);
        assert_eq!(node.proposals_seen_count(), 1);
        assert_eq!(*node.proposals_seen.get(&make_id(10)).unwrap(), 5);
    }

    #[test]
    fn test_record_attestation() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.record_attestation(make_id(10), true);
        assert_eq!(node.attestations_sent_count(), 1);
        assert!(*node.attestations_sent.get(&make_id(10)).unwrap());
    }

    #[test]
    fn test_receive_finalized() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.receive_finalized(make_id(20));
        assert_eq!(node.finalized_count(), 1);
        assert!(node.finalized_batches.contains(&make_id(20)));
    }

    #[test]
    fn test_apply_slash_accumulates() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.apply_slash(make_slash_record(make_id(1), 100));
        node.apply_slash(make_slash_record(make_id(1), 200));
        assert_eq!(node.cumulative_slash_bps, 300);
        assert_eq!(node.slash_records.len(), 2);
    }

    #[test]
    fn test_jail_and_unjail() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.in_committee = true;
        node.apply_slash(make_slash_record(make_id(1), 500));
        node.jail();
        assert!(node.is_jailed);
        assert!(!node.can_participate());
        node.unjail();
        assert!(!node.is_jailed);
        assert_eq!(node.cumulative_slash_bps, 0);
        assert!(node.can_participate());
    }

    #[test]
    fn test_can_participate_requires_all_conditions() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        // not in committee
        assert!(!node.can_participate());
        node.in_committee = true;
        assert!(node.can_participate());
        node.crashed = true;
        assert!(!node.can_participate());
        node.crashed = false;
        node.jail();
        assert!(!node.can_participate());
    }

    #[test]
    fn test_slash_saturation() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        node.apply_slash(make_slash_record(make_id(1), u64::MAX));
        node.apply_slash(make_slash_record(make_id(1), 1));
        assert_eq!(node.cumulative_slash_bps, u64::MAX);
    }

    #[test]
    fn test_observe_multiple_proposals() {
        let mut node = SimulatedNode::new(make_id(1), 1000);
        for i in 0..5 {
            node.observe_proposal(make_id(i), i as u64);
        }
        assert_eq!(node.proposals_seen_count(), 5);
    }
}
