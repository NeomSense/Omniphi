use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};
use crate::errors::Phase4Error;
use crate::simulation::node::SimulatedNode;
use crate::simulation::network::{
    NetworkMessage, SimAttestation, SimFinalizedBatch, SimProposal, SimulationNetwork,
};
use crate::slashing::offenses::{SlashableOffense, SlashingConfig};
use crate::slashing::store::SlashingStore;
use crate::committee_rotation::engine::{CommitteeRotationConfig, CommitteeRotationEngine};
use crate::committee_rotation::snapshot::{CommitteeRotationStore, EpochCommitteeSnapshot};

/// Scenarios that the simulation can exercise.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SimulationScenario {
    /// Normal operation: all nodes online, leader proposes, quorum attests.
    HappyPath,
    /// Leader crashes mid-epoch; no proposal is made.
    LeaderCrash,
    /// Leader sends two conflicting proposals for the same slot.
    DoubleProposal,
    /// Leader reorders transactions in a fairness-violating way.
    FairnessViolation,
    /// Committee membership is rotated after epoch boundary.
    CommitteeRotation,
}

/// Outcome metrics for a simulation run.
#[derive(Debug, Clone)]
pub struct SimulationResult {
    pub scenario: SimulationScenario,
    pub epochs_run: u64,
    pub batches_finalized: usize,
    pub slashes_applied: usize,
    pub jailed_nodes: BTreeSet<[u8; 32]>,
    pub committee_changes: Vec<CommitteeChangeSummary>,
    pub fairness_incidents: usize,
    pub leader_crashes: usize,
    pub double_proposals_detected: usize,
}

/// Summary of a single committee rotation.
#[derive(Debug, Clone)]
pub struct CommitteeChangeSummary {
    pub epoch: u64,
    pub joined: BTreeSet<[u8; 32]>,
    pub left: BTreeSet<[u8; 32]>,
    pub new_size: usize,
}

impl SimulationResult {
    pub fn new(scenario: SimulationScenario) -> Self {
        SimulationResult {
            scenario,
            epochs_run: 0,
            batches_finalized: 0,
            slashes_applied: 0,
            jailed_nodes: BTreeSet::new(),
            committee_changes: Vec::new(),
            fairness_incidents: 0,
            leader_crashes: 0,
            double_proposals_detected: 0,
        }
    }
}

/// Drives the end-to-end multi-node simulation.
pub struct SimulationRunner {
    pub nodes: BTreeMap<[u8; 32], SimulatedNode>,
    pub network: SimulationNetwork,
    pub slashing_store: SlashingStore,
    pub rotation_engine: CommitteeRotationEngine,
    pub rotation_store: CommitteeRotationStore,
    pub current_epoch: u64,
    pub current_slot: u64,
    /// Quorum threshold: number of approvals needed to finalize a batch.
    pub quorum_threshold: usize,
    pub finalized_batches: Vec<SimFinalizedBatch>,
}

impl SimulationRunner {
    pub fn new(
        node_count: usize,
        quorum_threshold: usize,
        rotation_config: CommitteeRotationConfig,
    ) -> Self {
        let mut nodes = BTreeMap::new();
        let mut network = SimulationNetwork::new();

        for i in 1..=(node_count as u8) {
            let node_id = Self::node_id_for(i);
            let node = SimulatedNode::new(node_id, 1000);
            nodes.insert(node_id, node);
            network.register_node(node_id);
        }

        let slashing_store = SlashingStore::new(SlashingConfig::default_config());

        SimulationRunner {
            nodes,
            network,
            slashing_store,
            rotation_engine: CommitteeRotationEngine::new(rotation_config),
            rotation_store: CommitteeRotationStore::new(),
            current_epoch: 1,
            current_slot: 1,
            quorum_threshold,
            finalized_batches: Vec::new(),
        }
    }

    /// Deterministic node_id from a small integer.
    pub fn node_id_for(i: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = i;
        id
    }

    /// Set up the initial committee with all non-jailed nodes.
    pub fn initialize_committee(&mut self) -> Result<EpochCommitteeSnapshot, Phase4Error> {
        let candidates: BTreeSet<[u8; 32]> = self.nodes.keys().cloned().collect();
        let excluded = self.slashing_store.jailed_set();
        let (snapshot, _record) = self.rotation_engine.rotate(
            self.current_epoch,
            &candidates,
            None,
            &excluded,
            &[0u8; 32], // genesis: no previous finalization hash
        )?;

        // Update node committee membership
        for node in self.nodes.values_mut() {
            node.in_committee = snapshot.contains(&node.node_id);
        }

        self.rotation_store.insert(snapshot.clone());
        Ok(snapshot)
    }

    /// Pick the leader for the current epoch: deterministic based on SHA256(epoch ‖ committee).
    pub fn elect_leader(&mut self, snapshot: &EpochCommitteeSnapshot) -> Option<[u8; 32]> {
        let members = snapshot.sorted_members();
        if members.is_empty() {
            return None;
        }
        let mut h = Sha256::new();
        h.update(&self.current_epoch.to_be_bytes());
        h.update(&snapshot.committee_hash);
        let r = h.finalize();
        let idx = r[0] as usize % members.len();
        let leader_id = members[idx];
        Some(leader_id)
    }

    /// Mark a node as leader.
    pub fn set_leader(&mut self, leader_id: [u8; 32]) {
        for node in self.nodes.values_mut() {
            node.is_leader = node.node_id == leader_id;
        }
    }

    /// Run a single HappyPath round: leader proposes, quorum attests, batch finalizes.
    pub fn run_happy_path_round(
        &mut self,
        result: &mut SimulationResult,
    ) -> Result<(), Phase4Error> {
        let snapshot = self.rotation_store.latest()
            .ok_or_else(|| Phase4Error::SimulationError("no committee snapshot".to_string()))?
            .clone();

        let leader_id = self.elect_leader(&snapshot)
            .ok_or_else(|| Phase4Error::SimulationError("no eligible leader".to_string()))?;
        self.set_leader(leader_id);

        // Leader creates a proposal
        let sub_ids: Vec<[u8; 32]> = (1..=3).map(|i| {
            let mut id = [0u8; 32];
            id[0] = i;
            id[1] = (self.current_slot & 0xFF) as u8;
            id
        }).collect();

        let proposal = SimProposal::new(self.current_slot, self.current_epoch, leader_id, sub_ids);
        let proposal_id = proposal.proposal_id;

        // Broadcast proposal
        self.network.broadcast(&leader_id, NetworkMessage::Proposal(proposal.clone()));

        // All committee members observe the proposal
        for node in self.nodes.values_mut() {
            if snapshot.contains(&node.node_id) && node.node_id != leader_id {
                node.observe_proposal(proposal_id, self.current_slot);
            }
        }
        if let Some(leader) = self.nodes.get_mut(&leader_id) {
            leader.observe_proposal(proposal_id, self.current_slot);
        }

        // Collect attestations from committee members
        let attestors: Vec<[u8; 32]> = snapshot.sorted_members()
            .into_iter()
            .filter(|id| {
                self.nodes.get(id).map(|n| n.can_participate()).unwrap_or(false)
            })
            .collect();

        let mut approvals = 0;
        let total = attestors.len();
        for attestor_id in &attestors {
            let attest = SimAttestation::new(proposal_id, *attestor_id, true, self.current_epoch);
            self.network.send_to(leader_id, NetworkMessage::Attestation(attest));
            if let Some(node) = self.nodes.get_mut(attestor_id) {
                node.record_attestation(proposal_id, true);
            }
            approvals += 1;
        }

        // Check quorum
        if approvals >= self.quorum_threshold {
            let finalized = SimFinalizedBatch::new(&proposal, approvals, total);
            let batch_id = finalized.batch_id;

            // Broadcast finalized batch to all
            self.network.broadcast_all(NetworkMessage::FinalizedBatch(finalized.clone()));

            for node in self.nodes.values_mut() {
                node.receive_finalized(batch_id);
            }

            self.finalized_batches.push(finalized);
            result.batches_finalized += 1;
        }

        self.current_slot += 1;
        Ok(())
    }

    /// Simulate a leader crash: leader is marked crashed, no proposal is made.
    pub fn run_leader_crash_round(
        &mut self,
        result: &mut SimulationResult,
    ) -> Result<(), Phase4Error> {
        let snapshot = self.rotation_store.latest()
            .ok_or_else(|| Phase4Error::SimulationError("no committee snapshot".to_string()))?
            .clone();

        let leader_id = self.elect_leader(&snapshot)
            .ok_or_else(|| Phase4Error::SimulationError("no eligible leader".to_string()))?;
        self.set_leader(leader_id);

        // Leader crashes
        if let Some(node) = self.nodes.get_mut(&leader_id) {
            node.crashed = true;
        }

        // Apply absent-from-duty slash to crashed leader
        let slash_result = self.slashing_store.process(
            SlashableOffense::AbsentFromDuty,
            leader_id,
            self.current_epoch,
            &self.current_slot.to_be_bytes(),
        )?;

        if let Some(node) = self.nodes.get_mut(&leader_id) {
            node.apply_slash(slash_result.record.clone());
        }

        self.network.broadcast_all(NetworkMessage::SlashReport(slash_result.record));
        result.slashes_applied += 1;
        result.leader_crashes += 1;

        if slash_result.jailed {
            if let Some(node) = self.nodes.get_mut(&leader_id) {
                node.jail();
            }
            result.jailed_nodes.insert(leader_id);
        }

        self.current_slot += 1;
        Ok(())
    }

    /// Simulate a double proposal: leader sends two proposals for the same slot.
    pub fn run_double_proposal_round(
        &mut self,
        result: &mut SimulationResult,
    ) -> Result<(), Phase4Error> {
        let snapshot = self.rotation_store.latest()
            .ok_or_else(|| Phase4Error::SimulationError("no committee snapshot".to_string()))?
            .clone();

        let leader_id = self.elect_leader(&snapshot)
            .ok_or_else(|| Phase4Error::SimulationError("no eligible leader".to_string()))?;
        self.set_leader(leader_id);

        // Leader sends two different proposals for the same slot
        let p1 = SimProposal::new(self.current_slot, self.current_epoch, leader_id,
            vec![Self::make_sub_id(1), Self::make_sub_id(2)]);
        let p2 = SimProposal::new(self.current_slot, self.current_epoch, leader_id,
            vec![Self::make_sub_id(3), Self::make_sub_id(4)]);

        self.network.broadcast(&leader_id, NetworkMessage::Proposal(p1.clone()));
        self.network.broadcast(&leader_id, NetworkMessage::Proposal(p2.clone()));

        // Detected: apply DoubleProposal slash
        let evidence: Vec<u8> = p1.proposal_id.iter()
            .chain(p2.proposal_id.iter())
            .cloned()
            .collect();

        let slash_result = self.slashing_store.process(
            SlashableOffense::DoubleProposal,
            leader_id,
            self.current_epoch,
            &evidence,
        )?;

        if let Some(node) = self.nodes.get_mut(&leader_id) {
            node.apply_slash(slash_result.record.clone());
        }

        self.network.broadcast_all(NetworkMessage::SlashReport(slash_result.record));
        result.slashes_applied += 1;
        result.double_proposals_detected += 1;

        if slash_result.jailed {
            if let Some(node) = self.nodes.get_mut(&leader_id) {
                node.jail();
            }
            result.jailed_nodes.insert(leader_id);
        }

        self.current_slot += 1;
        Ok(())
    }

    /// Simulate a fairness violation: slash with FairnessViolation.
    pub fn run_fairness_violation_round(
        &mut self,
        result: &mut SimulationResult,
    ) -> Result<(), Phase4Error> {
        let snapshot = self.rotation_store.latest()
            .ok_or_else(|| Phase4Error::SimulationError("no committee snapshot".to_string()))?
            .clone();

        let leader_id = self.elect_leader(&snapshot)
            .ok_or_else(|| Phase4Error::SimulationError("no eligible leader".to_string()))?;
        self.set_leader(leader_id);

        // Leader proposes but with a fairness violation detected
        let slash_result = self.slashing_store.process(
            SlashableOffense::FairnessViolation,
            leader_id,
            self.current_epoch,
            b"frontrun_evidence",
        )?;

        if let Some(node) = self.nodes.get_mut(&leader_id) {
            node.apply_slash(slash_result.record.clone());
        }

        self.network.broadcast_all(NetworkMessage::SlashReport(slash_result.record));
        result.slashes_applied += 1;
        result.fairness_incidents += 1;

        if slash_result.jailed {
            if let Some(node) = self.nodes.get_mut(&leader_id) {
                node.jail();
            }
            result.jailed_nodes.insert(leader_id);
        }

        self.current_slot += 1;
        Ok(())
    }

    /// Advance to next epoch, perform committee rotation.
    pub fn advance_epoch(&mut self, result: &mut SimulationResult) -> Result<(), Phase4Error> {
        self.current_epoch += 1;

        let candidates: BTreeSet<[u8; 32]> = self.nodes.keys().cloned().collect();
        let excluded = self.slashing_store.jailed_set();
        let prev_snapshot = self.rotation_store.latest().cloned();

        let (new_snapshot, rotation_record) = self.rotation_engine.rotate(
            self.current_epoch,
            &candidates,
            prev_snapshot.as_ref(),
            &excluded,
            &[0u8; 32], // TODO: pass actual last finalization hash from epoch state
        )?;

        // Update committee membership on nodes
        for node in self.nodes.values_mut() {
            node.in_committee = new_snapshot.contains(&node.node_id);
            node.is_leader = false; // reset for new epoch
        }

        if !rotation_record.is_no_change() {
            result.committee_changes.push(CommitteeChangeSummary {
                epoch: self.current_epoch,
                joined: rotation_record.joined.clone(),
                left: rotation_record.left.clone(),
                new_size: new_snapshot.size(),
            });
        }

        self.rotation_store.insert(new_snapshot);
        Ok(())
    }

    /// Run a full scenario for `epochs` epochs.
    pub fn run_scenario(
        &mut self,
        scenario: SimulationScenario,
        epochs: u64,
        rounds_per_epoch: u64,
    ) -> Result<SimulationResult, Phase4Error> {
        let mut result = SimulationResult::new(scenario.clone());

        // Initialize committee for first epoch
        self.initialize_committee()?;

        for epoch_i in 0..epochs {
            for _ in 0..rounds_per_epoch {
                match &scenario {
                    SimulationScenario::HappyPath => {
                        self.run_happy_path_round(&mut result)?;
                    }
                    SimulationScenario::LeaderCrash => {
                        if epoch_i == 0 {
                            self.run_leader_crash_round(&mut result)?;
                        } else {
                            self.run_happy_path_round(&mut result)?;
                        }
                    }
                    SimulationScenario::DoubleProposal => {
                        if epoch_i == 0 {
                            self.run_double_proposal_round(&mut result)?;
                        } else {
                            self.run_happy_path_round(&mut result)?;
                        }
                    }
                    SimulationScenario::FairnessViolation => {
                        if epoch_i == 0 {
                            self.run_fairness_violation_round(&mut result)?;
                        } else {
                            self.run_happy_path_round(&mut result)?;
                        }
                    }
                    SimulationScenario::CommitteeRotation => {
                        self.run_happy_path_round(&mut result)?;
                    }
                }
            }

            // Advance epoch and rotate committee
            if epoch_i + 1 < epochs {
                self.advance_epoch(&mut result)?;
            }
        }

        result.epochs_run = epochs;
        Ok(result)
    }

    fn make_sub_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id[31] = 0xFF; // marker
        id
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::committee_rotation::engine::CommitteeRotationConfig;

    fn make_rotation_config(min: usize, max: usize) -> CommitteeRotationConfig {
        CommitteeRotationConfig::new(1, min, max, [0u8; 32]).unwrap()
    }

    fn make_runner(node_count: usize) -> SimulationRunner {
        SimulationRunner::new(node_count, 2, make_rotation_config(2, node_count))
    }

    #[test]
    fn test_runner_initialization() {
        let runner = make_runner(5);
        assert_eq!(runner.nodes.len(), 5);
        assert_eq!(runner.network.node_count(), 5);
    }

    #[test]
    fn test_initialize_committee() {
        let mut runner = make_runner(5);
        let snapshot = runner.initialize_committee().unwrap();
        assert!(snapshot.size() >= 2 && snapshot.size() <= 5);
    }

    #[test]
    fn test_elect_leader_deterministic() {
        let mut runner = make_runner(5);
        let snap = runner.initialize_committee().unwrap();
        let l1 = runner.elect_leader(&snap);
        let l2 = runner.elect_leader(&snap);
        assert_eq!(l1, l2);
    }

    #[test]
    fn test_happy_path_finalizes_batch() {
        let mut runner = make_runner(5);
        runner.initialize_committee().unwrap();
        let mut result = SimulationResult::new(SimulationScenario::HappyPath);
        runner.run_happy_path_round(&mut result).unwrap();
        assert_eq!(result.batches_finalized, 1);
        assert_eq!(runner.finalized_batches.len(), 1);
    }

    #[test]
    fn test_leader_crash_applies_slash() {
        let mut runner = make_runner(5);
        runner.initialize_committee().unwrap();
        let mut result = SimulationResult::new(SimulationScenario::LeaderCrash);
        runner.run_leader_crash_round(&mut result).unwrap();
        assert_eq!(result.slashes_applied, 1);
        assert_eq!(result.leader_crashes, 1);
    }

    #[test]
    fn test_double_proposal_detected() {
        let mut runner = make_runner(5);
        runner.initialize_committee().unwrap();
        let mut result = SimulationResult::new(SimulationScenario::DoubleProposal);
        runner.run_double_proposal_round(&mut result).unwrap();
        assert_eq!(result.double_proposals_detected, 1);
        assert_eq!(result.slashes_applied, 1);
    }

    #[test]
    fn test_fairness_violation_applied() {
        let mut runner = make_runner(5);
        runner.initialize_committee().unwrap();
        let mut result = SimulationResult::new(SimulationScenario::FairnessViolation);
        runner.run_fairness_violation_round(&mut result).unwrap();
        assert_eq!(result.fairness_incidents, 1);
        assert_eq!(result.slashes_applied, 1);
    }

    #[test]
    fn test_advance_epoch_rotates_committee() {
        let mut runner = make_runner(6);
        runner.initialize_committee().unwrap();
        let initial_epoch = runner.current_epoch;
        let mut result = SimulationResult::new(SimulationScenario::CommitteeRotation);
        runner.advance_epoch(&mut result).unwrap();
        assert_eq!(runner.current_epoch, initial_epoch + 1);
        assert_eq!(runner.rotation_store.len(), 2);
    }

    #[test]
    fn test_run_scenario_happy_path() {
        let mut runner = make_runner(5);
        let result = runner.run_scenario(SimulationScenario::HappyPath, 2, 3).unwrap();
        assert_eq!(result.scenario, SimulationScenario::HappyPath);
        assert_eq!(result.epochs_run, 2);
        assert_eq!(result.batches_finalized, 6); // 2 epochs * 3 rounds
    }

    #[test]
    fn test_run_scenario_committee_rotation() {
        let mut runner = make_runner(6);
        let result = runner.run_scenario(SimulationScenario::CommitteeRotation, 3, 1).unwrap();
        assert_eq!(result.epochs_run, 3);
        assert!(result.batches_finalized >= 3);
    }

    #[test]
    fn test_run_scenario_leader_crash() {
        let mut runner = make_runner(5);
        let result = runner.run_scenario(SimulationScenario::LeaderCrash, 2, 1).unwrap();
        assert_eq!(result.leader_crashes, 1);
    }

    #[test]
    fn test_run_scenario_double_proposal() {
        let mut runner = make_runner(5);
        let result = runner.run_scenario(SimulationScenario::DoubleProposal, 2, 1).unwrap();
        assert_eq!(result.double_proposals_detected, 1);
    }

    #[test]
    fn test_run_scenario_fairness_violation() {
        let mut runner = make_runner(5);
        let result = runner.run_scenario(SimulationScenario::FairnessViolation, 2, 1).unwrap();
        assert_eq!(result.fairness_incidents, 1);
    }

    #[test]
    fn test_jailed_node_excluded_from_committee() {
        let config = CommitteeRotationConfig::new(1, 2, 10, [0u8; 32]).unwrap();
        let mut runner = SimulationRunner::new(5, 2, config);
        runner.initialize_committee().unwrap();

        // Get the elected leader and apply enough slashes to jail them
        let snapshot = runner.rotation_store.latest().unwrap().clone();
        let leader = runner.elect_leader(&snapshot).unwrap();

        // Apply 2 DoubleProposal = 1000 bps → jail (threshold = 1000)
        runner.slashing_store.process(
            SlashableOffense::DoubleProposal, leader, 1, b"e1"
        ).unwrap();
        runner.slashing_store.process(
            SlashableOffense::DoubleProposal, leader, 1, b"e2"
        ).unwrap();
        if let Some(node) = runner.nodes.get_mut(&leader) {
            node.jail();
        }

        // Now advance epoch; jailed leader should be excluded
        let mut result = SimulationResult::new(SimulationScenario::HappyPath);
        runner.advance_epoch(&mut result).unwrap();

        let new_snapshot = runner.rotation_store.latest().unwrap();
        assert!(!new_snapshot.contains(&leader));
    }

    #[test]
    fn test_all_nodes_start_not_jailed() {
        let runner = make_runner(4);
        for node in runner.nodes.values() {
            assert!(!node.is_jailed);
        }
    }
}
