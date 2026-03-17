use std::collections::BTreeMap;

/// The role a node plays in the PoSeq committee.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum NodeRole {
    Sequencer,
    Validator,
    ObserverOnly,
}

/// Lifecycle status of a node.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum NodeStatus {
    Active,
    Suspended,
    Ejected,
    Pending,
}

/// Full identity record for a node in the PoSeq committee.
#[derive(Debug, Clone)]
pub struct NodeIdentity {
    pub node_id: [u8; 32],
    pub public_key: [u8; 32], // placeholder — real sig verification outside scope
    pub role: NodeRole,
    pub status: NodeStatus,
    pub epoch_joined: u64,
    pub is_eligible_proposer: bool,
    pub is_eligible_attestor: bool,
    pub metadata: BTreeMap<String, String>,
}

impl NodeIdentity {
    pub fn new(
        node_id: [u8; 32],
        public_key: [u8; 32],
        role: NodeRole,
        epoch_joined: u64,
    ) -> Self {
        let is_eligible_proposer = matches!(role, NodeRole::Sequencer);
        let is_eligible_attestor = matches!(role, NodeRole::Sequencer | NodeRole::Validator);
        NodeIdentity {
            node_id,
            public_key,
            role,
            status: NodeStatus::Pending,
            epoch_joined,
            is_eligible_proposer,
            is_eligible_attestor,
            metadata: BTreeMap::new(),
        }
    }

    pub fn activate(&mut self) {
        self.status = NodeStatus::Active;
    }

    pub fn suspend(&mut self) {
        self.status = NodeStatus::Suspended;
        self.is_eligible_proposer = false;
        self.is_eligible_attestor = false;
    }

    pub fn eject(&mut self) {
        self.status = NodeStatus::Ejected;
        self.is_eligible_proposer = false;
        self.is_eligible_attestor = false;
    }

    pub fn is_active(&self) -> bool {
        self.status == NodeStatus::Active
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_node_creation_sequencer() {
        let id = make_id(1);
        let pk = make_id(2);
        let node = NodeIdentity::new(id, pk, NodeRole::Sequencer, 0);
        assert_eq!(node.status, NodeStatus::Pending);
        assert!(node.is_eligible_proposer);
        assert!(node.is_eligible_attestor);
        assert!(!node.is_active());
    }

    #[test]
    fn test_node_creation_validator() {
        let id = make_id(1);
        let pk = make_id(2);
        let node = NodeIdentity::new(id, pk, NodeRole::Validator, 0);
        assert!(!node.is_eligible_proposer);
        assert!(node.is_eligible_attestor);
    }

    #[test]
    fn test_node_creation_observer() {
        let id = make_id(1);
        let pk = make_id(2);
        let node = NodeIdentity::new(id, pk, NodeRole::ObserverOnly, 0);
        assert!(!node.is_eligible_proposer);
        assert!(!node.is_eligible_attestor);
    }

    #[test]
    fn test_status_transition_activate() {
        let id = make_id(1);
        let pk = make_id(2);
        let mut node = NodeIdentity::new(id, pk, NodeRole::Sequencer, 1);
        node.activate();
        assert_eq!(node.status, NodeStatus::Active);
        assert!(node.is_active());
    }

    #[test]
    fn test_status_transition_suspend() {
        let id = make_id(1);
        let pk = make_id(2);
        let mut node = NodeIdentity::new(id, pk, NodeRole::Sequencer, 1);
        node.activate();
        node.suspend();
        assert_eq!(node.status, NodeStatus::Suspended);
        assert!(!node.is_eligible_proposer);
        assert!(!node.is_eligible_attestor);
    }

    #[test]
    fn test_status_transition_eject() {
        let id = make_id(1);
        let pk = make_id(2);
        let mut node = NodeIdentity::new(id, pk, NodeRole::Sequencer, 1);
        node.activate();
        node.eject();
        assert_eq!(node.status, NodeStatus::Ejected);
        assert!(!node.is_eligible_proposer);
    }
}
