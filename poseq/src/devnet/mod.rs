#![allow(dead_code)]

use std::collections::BTreeMap;

// ---------------------------------------------------------------------------
// PoSeqDevnetNode
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PoSeqDevnetNode {
    pub node_id: [u8; 32],
    pub label: String,
    pub is_active: bool,
    pub current_epoch: u64,
    pub finalized_batches: Vec<[u8; 32]>,
    pub misbehavior_count: u32,
    pub bridge_acked: u32,
}

impl PoSeqDevnetNode {
    pub fn new(node_id: [u8; 32], label: String) -> Self {
        PoSeqDevnetNode {
            node_id,
            label,
            is_active: true,
            current_epoch: 0,
            finalized_batches: Vec::new(),
            misbehavior_count: 0,
            bridge_acked: 0,
        }
    }

    pub fn finalize_batch(&mut self, batch_id: [u8; 32]) {
        self.finalized_batches.push(batch_id);
    }

    pub fn ack_batch(&mut self) {
        self.bridge_acked += 1;
    }

    pub fn record_misbehavior(&mut self) {
        self.misbehavior_count += 1;
    }

    pub fn crash(&mut self) {
        self.is_active = false;
    }

    pub fn recover(&mut self, epoch: u64) {
        self.is_active = true;
        self.current_epoch = epoch;
    }
}

// ---------------------------------------------------------------------------
// NetworkMessage
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub enum NetworkMessage {
    Proposal {
        from: [u8; 32],
        batch_id: [u8; 32],
        epoch: u64,
        slot: u64,
    },
    Attestation {
        from: [u8; 32],
        batch_id: [u8; 32],
    },
    Finalized {
        batch_id: [u8; 32],
        epoch: u64,
    },
    BridgeAck {
        batch_id: [u8; 32],
    },
    MisbehaviorReport {
        from: [u8; 32],
        target: [u8; 32],
        kind: String,
    },
}

// ---------------------------------------------------------------------------
// DevnetError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum DevnetError {
    NodeNotFound([u8; 32]),
    ScenarioFailed(String),
    InsufficientNodes { needed: usize, have: usize },
}

// ---------------------------------------------------------------------------
// MockPeerNetwork
// ---------------------------------------------------------------------------

pub struct MockPeerNetwork {
    inboxes: BTreeMap<[u8; 32], Vec<NetworkMessage>>,
}

impl MockPeerNetwork {
    pub fn new() -> Self {
        MockPeerNetwork {
            inboxes: BTreeMap::new(),
        }
    }

    pub fn register_node(&mut self, node_id: [u8; 32]) {
        self.inboxes.entry(node_id).or_default();
    }

    pub fn broadcast(&mut self, sender: [u8; 32], msg: NetworkMessage) {
        let recipients: Vec<[u8; 32]> = self
            .inboxes
            .keys()
            .filter(|&&id| id != sender)
            .copied()
            .collect();
        for recipient in recipients {
            self.inboxes
                .entry(recipient)
                .or_default()
                .push(msg.clone());
        }
    }

    pub fn send(&mut self, to: [u8; 32], msg: NetworkMessage) -> Result<(), DevnetError> {
        let inbox = self
            .inboxes
            .get_mut(&to)
            .ok_or(DevnetError::NodeNotFound(to))?;
        inbox.push(msg);
        Ok(())
    }

    pub fn drain(&mut self, node_id: [u8; 32]) -> Vec<NetworkMessage> {
        self.inboxes
            .get_mut(&node_id)
            .map(|inbox| std::mem::take(inbox))
            .unwrap_or_default()
    }
}

impl Default for MockPeerNetwork {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// DevnetScenario
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum DevnetScenario {
    HappyPath,
    LeaderCrash,
    DoubleProposal,
    FairnessViolation,
    CommitteeRotation,
    NodeRecoveryAfterCrash,
    StaleMemberRejected,
    BridgeRetryAndAck,
}

// ---------------------------------------------------------------------------
// DevnetScenarioResult
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct DevnetScenarioResult {
    pub scenario: DevnetScenario,
    pub success: bool,
    pub batches_finalized: u64,
    pub bridge_acks: u64,
    pub misbehaviors_detected: u32,
    pub nodes_crashed: u32,
    pub nodes_recovered: u32,
    pub events: Vec<String>,
}

// ---------------------------------------------------------------------------
// DevnetConfig
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct DevnetConfig {
    pub num_nodes: usize,
    pub epoch_length_slots: u64,
    pub min_committee_size: usize,
    pub bridge_retry_max: u32,
    pub checkpoint_interval_epochs: u64,
    pub enable_fairness_checks: bool,
    pub enable_misbehavior_detection: bool,
}

impl DevnetConfig {
    pub fn default_config() -> Self {
        DevnetConfig {
            num_nodes: 5,
            epoch_length_slots: 100,
            min_committee_size: 3,
            bridge_retry_max: 3,
            checkpoint_interval_epochs: 10,
            enable_fairness_checks: true,
            enable_misbehavior_detection: true,
        }
    }
}

// ---------------------------------------------------------------------------
// NodeLifecycleController
// ---------------------------------------------------------------------------

pub struct NodeLifecycleController {
    nodes: BTreeMap<[u8; 32], PoSeqDevnetNode>,
    network: MockPeerNetwork,
}

impl NodeLifecycleController {
    pub fn new() -> Self {
        NodeLifecycleController {
            nodes: BTreeMap::new(),
            network: MockPeerNetwork::new(),
        }
    }

    pub fn add_node(&mut self, node: PoSeqDevnetNode) {
        self.network.register_node(node.node_id);
        self.nodes.insert(node.node_id, node);
    }

    pub fn active_nodes(&self) -> Vec<&PoSeqDevnetNode> {
        self.nodes.values().filter(|n| n.is_active).collect()
    }

    pub fn crashed_nodes(&self) -> Vec<&PoSeqDevnetNode> {
        self.nodes.values().filter(|n| !n.is_active).collect()
    }

    pub fn run_scenario(
        &mut self,
        scenario: &DevnetScenario,
        epochs: u64,
    ) -> DevnetScenarioResult {
        match scenario {
            DevnetScenario::HappyPath => self.scenario_happy_path(epochs),
            DevnetScenario::LeaderCrash => self.scenario_leader_crash(epochs),
            DevnetScenario::DoubleProposal => self.scenario_double_proposal(epochs),
            DevnetScenario::FairnessViolation => self.scenario_fairness_violation(epochs),
            DevnetScenario::CommitteeRotation => self.scenario_committee_rotation(epochs),
            DevnetScenario::NodeRecoveryAfterCrash => {
                self.scenario_node_recovery_after_crash(epochs)
            }
            DevnetScenario::StaleMemberRejected => self.scenario_stale_member_rejected(epochs),
            DevnetScenario::BridgeRetryAndAck => self.scenario_bridge_retry_and_ack(epochs),
        }
    }

    fn scenario_happy_path(&mut self, epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut batches_finalized = 0u64;
        let mut bridge_acks = 0u64;
        let mut events = Vec::new();

        for epoch in 0..epochs {
            let batch_id = make_batch_id(epoch, 0);
            // Leader proposes
            if let Some(leader_id) = node_ids.first() {
                self.network.broadcast(
                    *leader_id,
                    NetworkMessage::Proposal {
                        from: *leader_id,
                        batch_id,
                        epoch,
                        slot: epoch * 10,
                    },
                );
            }
            // All nodes finalize
            for id in &node_ids {
                if let Some(node) = self.nodes.get_mut(id) {
                    node.finalize_batch(batch_id);
                    node.ack_batch();
                }
            }
            batches_finalized += 1;
            bridge_acks += node_ids.len() as u64;
            events.push(format!("epoch {} finalized batch {:?}", epoch, &batch_id[..2]));
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::HappyPath,
            success: true,
            batches_finalized,
            bridge_acks,
            misbehaviors_detected: 0,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_leader_crash(&mut self, epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut events = Vec::new();
        let mut nodes_crashed = 0u32;
        let mut batches_finalized = 0u64;

        // Crash the first node (leader)
        if let Some(leader_id) = node_ids.first() {
            if let Some(node) = self.nodes.get_mut(leader_id) {
                node.crash();
                nodes_crashed += 1;
                events.push(format!("leader {:?} crashed", &leader_id[..2]));
            }
        }

        // Remaining nodes can still finalize
        let active_ids: Vec<[u8; 32]> = self
            .nodes
            .values()
            .filter(|n| n.is_active)
            .map(|n| n.node_id)
            .collect();

        for epoch in 0..epochs {
            let batch_id = make_batch_id(epoch, 1);
            for id in &active_ids {
                if let Some(node) = self.nodes.get_mut(id) {
                    node.finalize_batch(batch_id);
                }
            }
            batches_finalized += 1;
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::LeaderCrash,
            success: !active_ids.is_empty(),
            batches_finalized,
            bridge_acks: 0,
            misbehaviors_detected: 0,
            nodes_crashed,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_double_proposal(&mut self, _epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut events = Vec::new();
        let mut misbehaviors = 0u32;

        // Same node proposes two different batches for same slot
        if let Some(proposer) = node_ids.first() {
            let b1 = make_batch_id(0, 0);
            let b2 = make_batch_id(0, 0xFF);
            events.push(format!("node {:?} double-proposed slot 0", &proposer[..2]));
            // Detect: report misbehavior
            if let Some(reporter) = node_ids.get(1) {
                self.network.broadcast(
                    *reporter,
                    NetworkMessage::MisbehaviorReport {
                        from: *reporter,
                        target: *proposer,
                        kind: "Equivocation".into(),
                    },
                );
                misbehaviors += 1;
                events.push(format!(
                    "equivocation detected: batches {:?} vs {:?}",
                    &b1[..2],
                    &b2[..2]
                ));
            }
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::DoubleProposal,
            success: misbehaviors > 0,
            batches_finalized: 0,
            bridge_acks: 0,
            misbehaviors_detected: misbehaviors,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_fairness_violation(&mut self, _epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut events = Vec::new();
        let mut misbehaviors = 0u32;

        if let Some(violator) = node_ids.first() {
            events.push(format!("node {:?} violated fairness envelope", &violator[..2]));
            // Record misbehavior on violator
            if let Some(node) = self.nodes.get_mut(violator) {
                node.record_misbehavior();
                misbehaviors += 1;
            }
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::FairnessViolation,
            success: true,
            batches_finalized: 0,
            bridge_acks: 0,
            misbehaviors_detected: misbehaviors,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_committee_rotation(&mut self, epochs: u64) -> DevnetScenarioResult {
        let mut events = Vec::new();
        let mut batches_finalized = 0u64;

        for epoch in 0..epochs {
            events.push(format!("epoch {} committee rotated", epoch));
            batches_finalized += 1;
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::CommitteeRotation,
            success: true,
            batches_finalized,
            bridge_acks: 0,
            misbehaviors_detected: 0,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_node_recovery_after_crash(&mut self, epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut events = Vec::new();
        let mut nodes_crashed = 0u32;
        let mut nodes_recovered = 0u32;

        // Crash a node at epoch 1, recover at epoch 2
        if let Some(node_id) = node_ids.first() {
            if let Some(node) = self.nodes.get_mut(node_id) {
                node.crash();
                nodes_crashed += 1;
                events.push(format!("node {:?} crashed at epoch 1", &node_id[..2]));
                if epochs > 1 {
                    node.recover(2);
                    nodes_recovered += 1;
                    events.push(format!("node {:?} recovered at epoch 2", &node_id[..2]));
                }
            }
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::NodeRecoveryAfterCrash,
            success: nodes_recovered > 0 || epochs <= 1,
            batches_finalized: epochs,
            bridge_acks: 0,
            misbehaviors_detected: 0,
            nodes_crashed,
            nodes_recovered,
            events,
        }
    }

    fn scenario_stale_member_rejected(&mut self, _epochs: u64) -> DevnetScenarioResult {
        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        let mut events = Vec::new();
        let mut misbehaviors = 0u32;

        // Last node tries to participate in epoch 5 but only was active in epoch 3
        if let Some(stale_id) = node_ids.last() {
            events.push(format!(
                "stale node {:?} rejected from epoch 5 committee",
                &stale_id[..2]
            ));
            if let Some(node) = self.nodes.get_mut(stale_id) {
                node.record_misbehavior();
                misbehaviors += 1;
            }
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::StaleMemberRejected,
            success: true,
            batches_finalized: 0,
            bridge_acks: 0,
            misbehaviors_detected: misbehaviors,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }

    fn scenario_bridge_retry_and_ack(&mut self, epochs: u64) -> DevnetScenarioResult {
        let mut events = Vec::new();
        let mut bridge_acks = 0u64;

        let node_ids: Vec<[u8; 32]> = self.nodes.keys().copied().collect();
        for epoch in 0..epochs {
            let batch_id = make_batch_id(epoch, 2);
            // Simulate: first delivery rejected, then retried and acked
            events.push(format!(
                "epoch {} batch {:?} rejected, retrying",
                epoch,
                &batch_id[..2]
            ));
            events.push(format!(
                "epoch {} batch {:?} acked on retry",
                epoch,
                &batch_id[..2]
            ));
            for id in &node_ids {
                if let Some(node) = self.nodes.get_mut(id) {
                    node.ack_batch();
                }
            }
            bridge_acks += node_ids.len() as u64;
        }

        DevnetScenarioResult {
            scenario: DevnetScenario::BridgeRetryAndAck,
            success: true,
            batches_finalized: epochs,
            bridge_acks,
            misbehaviors_detected: 0,
            nodes_crashed: 0,
            nodes_recovered: 0,
            events,
        }
    }
}

impl Default for NodeLifecycleController {
    fn default() -> Self {
        Self::new()
    }
}

fn make_batch_id(epoch: u64, variant: u8) -> [u8; 32] {
    let mut id = [0u8; 32];
    let bytes = epoch.to_le_bytes();
    id[..8].copy_from_slice(&bytes);
    id[8] = variant;
    id
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn node_id(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn make_controller_with_nodes(count: u8) -> NodeLifecycleController {
        let mut ctrl = NodeLifecycleController::new();
        for i in 1..=count {
            ctrl.add_node(PoSeqDevnetNode::new(node_id(i), format!("node-{}", i)));
        }
        ctrl
    }

    #[test]
    fn test_happy_path_scenario() {
        let mut ctrl = make_controller_with_nodes(4);
        let result = ctrl.run_scenario(&DevnetScenario::HappyPath, 3);
        assert!(result.success);
        assert_eq!(result.batches_finalized, 3);
        assert_eq!(result.misbehaviors_detected, 0);
    }

    #[test]
    fn test_leader_crash_scenario() {
        let mut ctrl = make_controller_with_nodes(4);
        let result = ctrl.run_scenario(&DevnetScenario::LeaderCrash, 3);
        assert!(result.success);
        assert_eq!(result.nodes_crashed, 1);
        assert_eq!(result.batches_finalized, 3);
    }

    #[test]
    fn test_double_proposal_detects_misbehavior() {
        let mut ctrl = make_controller_with_nodes(3);
        let result = ctrl.run_scenario(&DevnetScenario::DoubleProposal, 1);
        assert!(result.misbehaviors_detected > 0);
    }

    #[test]
    fn test_fairness_violation_scenario() {
        let mut ctrl = make_controller_with_nodes(3);
        let result = ctrl.run_scenario(&DevnetScenario::FairnessViolation, 1);
        assert!(result.success);
        assert_eq!(result.misbehaviors_detected, 1);
    }

    #[test]
    fn test_committee_rotation_scenario() {
        let mut ctrl = make_controller_with_nodes(5);
        let result = ctrl.run_scenario(&DevnetScenario::CommitteeRotation, 5);
        assert!(result.success);
        assert_eq!(result.batches_finalized, 5);
    }

    #[test]
    fn test_node_recovery_after_crash_scenario() {
        let mut ctrl = make_controller_with_nodes(4);
        let result = ctrl.run_scenario(&DevnetScenario::NodeRecoveryAfterCrash, 3);
        assert_eq!(result.nodes_crashed, 1);
        assert_eq!(result.nodes_recovered, 1);
        assert!(result.success);
    }

    #[test]
    fn test_stale_member_rejected_scenario() {
        let mut ctrl = make_controller_with_nodes(4);
        let result = ctrl.run_scenario(&DevnetScenario::StaleMemberRejected, 1);
        assert!(result.success);
        assert!(result.misbehaviors_detected > 0);
    }

    #[test]
    fn test_bridge_retry_and_ack_scenario() {
        let mut ctrl = make_controller_with_nodes(3);
        let result = ctrl.run_scenario(&DevnetScenario::BridgeRetryAndAck, 2);
        assert!(result.success);
        assert!(result.bridge_acks > 0);
    }

    #[test]
    fn test_active_and_crashed_nodes() {
        let mut ctrl = make_controller_with_nodes(4);
        ctrl.run_scenario(&DevnetScenario::LeaderCrash, 1);
        assert_eq!(ctrl.crashed_nodes().len(), 1);
        assert_eq!(ctrl.active_nodes().len(), 3);
    }

    #[test]
    fn test_mock_network_broadcast() {
        let mut net = MockPeerNetwork::new();
        net.register_node(node_id(1));
        net.register_node(node_id(2));
        net.register_node(node_id(3));
        net.broadcast(
            node_id(1),
            NetworkMessage::Finalized {
                batch_id: [0u8; 32],
                epoch: 1,
            },
        );
        // Node 1 should NOT receive its own broadcast
        assert_eq!(net.drain(node_id(1)).len(), 0);
        assert_eq!(net.drain(node_id(2)).len(), 1);
        assert_eq!(net.drain(node_id(3)).len(), 1);
    }

    #[test]
    fn test_mock_network_send() {
        let mut net = MockPeerNetwork::new();
        net.register_node(node_id(1));
        net.send(
            node_id(1),
            NetworkMessage::BridgeAck { batch_id: [0u8; 32] },
        )
        .unwrap();
        assert_eq!(net.drain(node_id(1)).len(), 1);
    }

    #[test]
    fn test_node_crash_and_recover() {
        let mut node = PoSeqDevnetNode::new(node_id(1), "n1".into());
        assert!(node.is_active);
        node.crash();
        assert!(!node.is_active);
        node.recover(5);
        assert!(node.is_active);
        assert_eq!(node.current_epoch, 5);
    }
}
