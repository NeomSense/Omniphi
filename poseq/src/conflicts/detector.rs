use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::leader_selection::selector::SelectedLeader;
use crate::committee::membership::PoSeqCommittee;

/// Category of sequencing misbehavior.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum IncidentType {
    DualProposal,
    InvalidProposer,
    DuplicateFinalization,
    AttestationEquivocation,
    ReplayedProposal,
}

/// A detected equivocation or misbehavior event.
#[derive(Debug, Clone)]
pub struct EquivocationIncident {
    pub incident_id: [u8; 32],
    pub incident_type: IncidentType,
    pub detected_at_height: u64,
    pub evidence: BTreeMap<String, String>,
}

impl EquivocationIncident {
    pub fn new(
        incident_type: IncidentType,
        detected_at_height: u64,
        evidence: BTreeMap<String, String>,
    ) -> Self {
        let id = Self::compute_id(&incident_type, detected_at_height, &evidence);
        EquivocationIncident {
            incident_id: id,
            incident_type,
            detected_at_height,
            evidence,
        }
    }

    fn compute_id(
        incident_type: &IncidentType,
        height: u64,
        evidence: &BTreeMap<String, String>,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(format!("{:?}", incident_type).as_bytes());
        hasher.update(&height.to_be_bytes());
        for (k, v) in evidence {
            hasher.update(k.as_bytes());
            hasher.update(v.as_bytes());
        }
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Per-node record of all detected misbehavior incidents.
#[derive(Debug, Clone)]
pub struct SequencingMisbehaviorRecord {
    pub node_id: [u8; 32],
    pub incidents: Vec<EquivocationIncident>,
    pub total_incidents: usize,
}

impl SequencingMisbehaviorRecord {
    pub fn new(node_id: [u8; 32]) -> Self {
        SequencingMisbehaviorRecord {
            node_id,
            incidents: Vec::new(),
            total_incidents: 0,
        }
    }

    pub fn record(&mut self, incident: EquivocationIncident) {
        self.total_incidents += 1;
        self.incidents.push(incident);
    }
}

/// A pair of conflicting proposals for the same (slot, epoch).
#[derive(Debug, Clone)]
pub struct ProposalConflict {
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub first_proposal_id: [u8; 32],
    pub second_proposal_id: [u8; 32],
}

/// Stateful conflict detector — tracks seen proposals and logs incidents.
pub struct ConflictDetector {
    /// Map (slot, epoch) → (leader_id, proposal_id) for the first seen proposal per slot.
    pub seen_proposals: BTreeMap<(u64, u64), ([u8; 32], [u8; 32])>,
    pub incidents: Vec<EquivocationIncident>,
    /// Per-node misbehavior records.
    misbehavior: BTreeMap<[u8; 32], SequencingMisbehaviorRecord>,
}

impl ConflictDetector {
    pub fn new() -> Self {
        ConflictDetector {
            seen_proposals: BTreeMap::new(),
            incidents: Vec::new(),
            misbehavior: BTreeMap::new(),
        }
    }

    /// Check a new proposal. Returns Some(incident) if a dual-proposal equivocation
    /// or an invalid proposer is detected.
    pub fn check_proposal(
        &mut self,
        slot: u64,
        epoch: u64,
        leader_id: [u8; 32],
        proposal_id: [u8; 32],
        committee: &PoSeqCommittee,
        height: u64,
    ) -> Option<EquivocationIncident> {
        // Check proposer is in committee
        if !committee.is_member(&leader_id) {
            let mut evidence = BTreeMap::new();
            evidence.insert("slot".to_string(), slot.to_string());
            evidence.insert("epoch".to_string(), epoch.to_string());
            evidence.insert("leader_id".to_string(), hex::encode(leader_id));
            let incident = EquivocationIncident::new(
                IncidentType::InvalidProposer,
                height,
                evidence,
            );
            self.record_incident(leader_id, incident.clone());
            return Some(incident);
        }

        // Check for dual proposal: same (slot, epoch) but different proposal from same or different leader
        let key = (slot, epoch);
        if let Some((existing_leader, existing_proposal)) = self.seen_proposals.get(&key) {
            if *existing_proposal != proposal_id {
                let mut evidence = BTreeMap::new();
                evidence.insert("slot".to_string(), slot.to_string());
                evidence.insert("epoch".to_string(), epoch.to_string());
                evidence.insert("first_proposal".to_string(), hex::encode(existing_proposal));
                evidence.insert("second_proposal".to_string(), hex::encode(proposal_id));
                evidence.insert("existing_leader".to_string(), hex::encode(existing_leader));
                evidence.insert("new_leader".to_string(), hex::encode(leader_id));
                let incident = EquivocationIncident::new(
                    IncidentType::DualProposal,
                    height,
                    evidence,
                );
                self.record_incident(leader_id, incident.clone());
                return Some(incident);
            }
            // Same proposal seen again — replay
            let mut evidence = BTreeMap::new();
            evidence.insert("proposal_id".to_string(), hex::encode(proposal_id));
            let incident = EquivocationIncident::new(
                IncidentType::ReplayedProposal,
                height,
                evidence,
            );
            // Don't insert to seen_proposals again; just log
            self.incidents.push(incident.clone());
            return Some(incident);
        }

        self.seen_proposals.insert(key, (leader_id, proposal_id));
        None
    }

    /// Returns true if the leader_id matches the expected selected leader for this slot.
    pub fn check_leader_authority(
        &self,
        _slot: u64,
        _epoch: u64,
        leader_id: [u8; 32],
        selected_leader: &SelectedLeader,
    ) -> bool {
        leader_id == selected_leader.node_id
    }

    /// Access all misbehavior records.
    pub fn misbehavior_ledger(&self) -> &BTreeMap<[u8; 32], SequencingMisbehaviorRecord> {
        &self.misbehavior
    }

    fn record_incident(&mut self, node_id: [u8; 32], incident: EquivocationIncident) {
        self.incidents.push(incident.clone());
        self.misbehavior
            .entry(node_id)
            .or_insert_with(|| SequencingMisbehaviorRecord::new(node_id))
            .record(incident);
    }
}

impl Default for ConflictDetector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::committee::membership::PoSeqCommittee;
    use crate::identities::node::{NodeIdentity, NodeRole};
    use crate::leader_selection::selector::SelectedLeader;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_committee_with(ids: &[u8]) -> PoSeqCommittee {
        let mut c = PoSeqCommittee::new(1);
        for &b in ids {
            let mut n = NodeIdentity::new(make_id(b), make_id(b + 100), NodeRole::Sequencer, 0);
            n.activate();
            c.add_node(n);
        }
        c
    }

    fn make_selected_leader(node_id: [u8; 32], slot: u64) -> SelectedLeader {
        SelectedLeader {
            node_id,
            slot,
            epoch: 1,
            selection_method: "RoundRobin".to_string(),
            selection_hash: [0u8; 32],
        }
    }

    #[test]
    fn test_valid_proposal_passes() {
        let committee = make_committee_with(&[1, 2, 3]);
        let mut detector = ConflictDetector::new();
        let result = detector.check_proposal(
            1, 1, make_id(1), make_id(10), &committee, 100
        );
        assert!(result.is_none());
    }

    #[test]
    fn test_dual_proposal_detected() {
        let committee = make_committee_with(&[1, 2, 3]);
        let mut detector = ConflictDetector::new();
        // First proposal for slot 1, epoch 1
        detector.check_proposal(1, 1, make_id(1), make_id(10), &committee, 100);
        // Second different proposal for same slot/epoch
        let incident = detector.check_proposal(1, 1, make_id(1), make_id(20), &committee, 101);
        assert!(incident.is_some());
        let inc = incident.unwrap();
        assert_eq!(inc.incident_type, IncidentType::DualProposal);
    }

    #[test]
    fn test_invalid_proposer_detected() {
        let committee = make_committee_with(&[1, 2, 3]);
        let mut detector = ConflictDetector::new();
        // Node 99 is not in committee
        let incident = detector.check_proposal(
            1, 1, make_id(99), make_id(10), &committee, 100
        );
        assert!(incident.is_some());
        assert_eq!(incident.unwrap().incident_type, IncidentType::InvalidProposer);
    }

    #[test]
    fn test_check_leader_authority_valid() {
        let detector = ConflictDetector::new();
        let selected = make_selected_leader(make_id(5), 1);
        assert!(detector.check_leader_authority(1, 1, make_id(5), &selected));
    }

    #[test]
    fn test_check_leader_authority_invalid() {
        let detector = ConflictDetector::new();
        let selected = make_selected_leader(make_id(5), 1);
        assert!(!detector.check_leader_authority(1, 1, make_id(6), &selected));
    }

    #[test]
    fn test_misbehavior_recorded() {
        let committee = make_committee_with(&[1, 2]);
        let mut detector = ConflictDetector::new();
        // First valid proposal
        detector.check_proposal(1, 1, make_id(1), make_id(10), &committee, 100);
        // Dual proposal from same node
        detector.check_proposal(1, 1, make_id(1), make_id(20), &committee, 101);
        let ledger = detector.misbehavior_ledger();
        assert!(ledger.contains_key(&make_id(1)));
        assert_eq!(ledger[&make_id(1)].total_incidents, 1);
    }
}
