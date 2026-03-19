#![allow(dead_code)]

use std::collections::BTreeMap;
use std::fmt;

// ---------------------------------------------------------------------------
// MembershipStatus
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum MembershipStatus {
    Active,
    Suspended { reason: String, since_epoch: u64 },
    Banned { reason: String, since_epoch: u64 },
    Recovering,
    Observer,
    Leaving,
    Removed,
}

impl fmt::Display for MembershipStatus {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            MembershipStatus::Active => write!(f, "Active"),
            MembershipStatus::Suspended { .. } => write!(f, "Suspended"),
            MembershipStatus::Banned { .. } => write!(f, "Banned"),
            MembershipStatus::Recovering => write!(f, "Recovering"),
            MembershipStatus::Observer => write!(f, "Observer"),
            MembershipStatus::Leaving => write!(f, "Leaving"),
            MembershipStatus::Removed => write!(f, "Removed"),
        }
    }
}

impl MembershipStatus {
    pub fn can_vote(&self) -> bool {
        matches!(self, MembershipStatus::Active)
    }

    pub fn can_propose(&self) -> bool {
        matches!(self, MembershipStatus::Active)
    }

    pub fn can_participate(&self) -> bool {
        matches!(
            self,
            MembershipStatus::Active | MembershipStatus::Recovering | MembershipStatus::Observer
        )
    }

    pub fn is_terminal(&self) -> bool {
        matches!(self, MembershipStatus::Banned { .. } | MembershipStatus::Removed)
    }

    /// Returns true if the transition from self to next is legal.
    pub fn can_transition_to(&self, next: &MembershipStatus) -> bool {
        use MembershipStatus::*;
        match (self, next) {
            // Active can go to most states
            (Active, Suspended { .. }) => true,
            (Active, Banned { .. }) => true,
            (Active, Leaving) => true,
            (Active, Observer) => true,
            // Suspended can recover or be banned/removed
            (Suspended { .. }, Active) => true,
            (Suspended { .. }, Recovering) => true,
            (Suspended { .. }, Banned { .. }) => true,
            (Suspended { .. }, Removed) => true,
            // Recovering → Active
            (Recovering, Active) => true,
            (Recovering, Suspended { .. }) => true,
            // Observer → Active or Leaving
            (Observer, Active) => true,
            (Observer, Leaving) => true,
            // Leaving → Removed
            (Leaving, Removed) => true,
            // Terminal → nothing
            _ => false,
        }
    }
}

// ---------------------------------------------------------------------------
// NodeMembership
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct MembershipTransitionEvent {
    pub from: String,
    pub to: String,
    pub epoch: u64,
    pub reason: String,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct NodeMembership {
    pub node_id: [u8; 32],
    pub status: MembershipStatus,
    pub joined_epoch: u64,
    pub history: Vec<MembershipTransitionEvent>,
}

// ---------------------------------------------------------------------------
// RoleTransitionRequest
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RoleTransitionRequest {
    pub node_id: [u8; 32],
    pub requested_status: MembershipStatus,
    pub reason: String,
    pub epoch: u64,
}

// ---------------------------------------------------------------------------
// MembershipError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum MembershipError {
    NodeNotFound([u8; 32]),
    IllegalTransition {
        node_id: [u8; 32],
        from: String,
        to: String,
    },
    NodeAlreadyTerminal([u8; 32]),
    InsufficientCommitteeSize,
}

// ---------------------------------------------------------------------------
// MembershipStore
// ---------------------------------------------------------------------------

pub struct MembershipStore {
    members: BTreeMap<[u8; 32], NodeMembership>,
}

impl MembershipStore {
    pub fn new() -> Self {
        MembershipStore {
            members: BTreeMap::new(),
        }
    }

    pub fn register(&mut self, node_id: [u8; 32], epoch: u64) -> Result<(), MembershipError> {
        if self.members.contains_key(&node_id) {
            // idempotent — already registered
            return Ok(());
        }
        let membership = NodeMembership {
            node_id,
            status: MembershipStatus::Active,
            joined_epoch: epoch,
            history: Vec::new(),
        };
        self.members.insert(node_id, membership);
        Ok(())
    }

    pub fn transition(&mut self, req: RoleTransitionRequest) -> Result<(), MembershipError> {
        let member = self
            .members
            .get_mut(&req.node_id)
            .ok_or(MembershipError::NodeNotFound(req.node_id))?;

        if member.status.is_terminal() {
            return Err(MembershipError::NodeAlreadyTerminal(req.node_id));
        }

        if !member.status.can_transition_to(&req.requested_status) {
            return Err(MembershipError::IllegalTransition {
                node_id: req.node_id,
                from: member.status.to_string(),
                to: req.requested_status.to_string(),
            });
        }

        let event = MembershipTransitionEvent {
            from: member.status.to_string(),
            to: req.requested_status.to_string(),
            epoch: req.epoch,
            reason: req.reason,
        };
        member.history.push(event);
        member.status = req.requested_status;
        Ok(())
    }

    pub fn get(&self, node_id: &[u8; 32]) -> Option<&NodeMembership> {
        self.members.get(node_id)
    }

    pub fn active_members(&self) -> Vec<[u8; 32]> {
        self.members
            .iter()
            .filter(|(_, m)| matches!(m.status, MembershipStatus::Active))
            .map(|(id, _)| *id)
            .collect()
    }

    pub fn can_participate(&self, node_id: &[u8; 32]) -> bool {
        self.members
            .get(node_id)
            .map(|m| m.status.can_participate())
            .unwrap_or(false)
    }

    /// Returns true if the node is eligible for committee participation.
    /// Only `Active` nodes are committee-eligible (not Recovering or Observer).
    pub fn is_eligible_for_committee(&self, node_id: &[u8; 32]) -> bool {
        self.members
            .get(node_id)
            .map(|m| matches!(m.status, MembershipStatus::Active))
            .unwrap_or(true) // unknown nodes are not blocked at this layer
    }

    /// Transition a node to Suspended if it isn't already terminal.
    /// Idempotent: if the node is already Suspended, does nothing.
    /// If the node is not registered, registers it as Suspended.
    pub fn suspend_node(&mut self, node_id: &[u8; 32], epoch: u64) {
        if let Some(member) = self.members.get(node_id) {
            if member.status.is_terminal() {
                return;
            }
            if matches!(member.status, MembershipStatus::Suspended { .. }) {
                return;
            }
        } else {
            // Not registered locally — register directly as Suspended.
            let membership = NodeMembership {
                node_id: *node_id,
                status: MembershipStatus::Suspended {
                    reason: "chain-authoritative suspension".into(),
                    since_epoch: epoch,
                },
                joined_epoch: epoch,
                history: Vec::new(),
            };
            self.members.insert(*node_id, membership);
            return;
        }

        let _ = self.transition(RoleTransitionRequest {
            node_id: *node_id,
            requested_status: MembershipStatus::Suspended {
                reason: "chain-authoritative suspension".into(),
                since_epoch: epoch,
            },
            reason: "chain-authoritative suspension".into(),
            epoch,
        });
    }
}

impl Default for MembershipStore {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    #[test]
    fn test_register_and_get() {
        let mut store = MembershipStore::new();
        store.register(node(1), 1).unwrap();
        let m = store.get(&node(1)).unwrap();
        assert_eq!(m.status, MembershipStatus::Active);
        assert_eq!(m.joined_epoch, 1);
    }

    #[test]
    fn test_active_members_list() {
        let mut store = MembershipStore::new();
        store.register(node(1), 1).unwrap();
        store.register(node(2), 1).unwrap();
        assert_eq!(store.active_members().len(), 2);
    }

    #[test]
    fn test_transition_active_to_suspended() {
        let mut store = MembershipStore::new();
        store.register(node(1), 1).unwrap();
        let req = RoleTransitionRequest {
            node_id: node(1),
            requested_status: MembershipStatus::Suspended {
                reason: "low perf".into(),
                since_epoch: 2,
            },
            reason: "low perf".into(),
            epoch: 2,
        };
        store.transition(req).unwrap();
        assert!(matches!(
            store.get(&node(1)).unwrap().status,
            MembershipStatus::Suspended { .. }
        ));
    }

    #[test]
    fn test_transition_suspended_to_active() {
        let mut store = MembershipStore::new();
        store.register(node(2), 1).unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(2),
                requested_status: MembershipStatus::Suspended {
                    reason: "t".into(),
                    since_epoch: 1,
                },
                reason: "t".into(),
                epoch: 1,
            })
            .unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(2),
                requested_status: MembershipStatus::Active,
                reason: "back".into(),
                epoch: 2,
            })
            .unwrap();
        assert_eq!(store.get(&node(2)).unwrap().status, MembershipStatus::Active);
    }

    #[test]
    fn test_illegal_transition_banned_to_active_fails() {
        let mut store = MembershipStore::new();
        store.register(node(3), 1).unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(3),
                requested_status: MembershipStatus::Banned {
                    reason: "bad".into(),
                    since_epoch: 1,
                },
                reason: "bad".into(),
                epoch: 1,
            })
            .unwrap();
        let result = store.transition(RoleTransitionRequest {
            node_id: node(3),
            requested_status: MembershipStatus::Active,
            reason: "unban".into(),
            epoch: 2,
        });
        assert!(matches!(result, Err(MembershipError::NodeAlreadyTerminal(_))));
    }

    #[test]
    fn test_node_not_found() {
        let mut store = MembershipStore::new();
        let result = store.transition(RoleTransitionRequest {
            node_id: node(99),
            requested_status: MembershipStatus::Active,
            reason: "".into(),
            epoch: 1,
        });
        assert!(matches!(result, Err(MembershipError::NodeNotFound(_))));
    }

    #[test]
    fn test_can_participate_recovering() {
        let mut store = MembershipStore::new();
        store.register(node(4), 1).unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(4),
                requested_status: MembershipStatus::Suspended {
                    reason: "t".into(),
                    since_epoch: 1,
                },
                reason: "t".into(),
                epoch: 1,
            })
            .unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(4),
                requested_status: MembershipStatus::Recovering,
                reason: "recovery".into(),
                epoch: 2,
            })
            .unwrap();
        assert!(store.can_participate(&node(4)));
    }

    #[test]
    fn test_can_vote_only_active() {
        assert!(MembershipStatus::Active.can_vote());
        assert!(!MembershipStatus::Observer.can_vote());
        assert!(!MembershipStatus::Recovering.can_vote());
    }

    #[test]
    fn test_terminal_states() {
        assert!(MembershipStatus::Removed.is_terminal());
        assert!(MembershipStatus::Banned { reason: "x".into(), since_epoch: 1 }.is_terminal());
        assert!(!MembershipStatus::Active.is_terminal());
        assert!(!MembershipStatus::Leaving.is_terminal());
    }

    #[test]
    fn test_history_is_recorded() {
        let mut store = MembershipStore::new();
        store.register(node(5), 1).unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(5),
                requested_status: MembershipStatus::Observer,
                reason: "observe".into(),
                epoch: 2,
            })
            .unwrap();
        let m = store.get(&node(5)).unwrap();
        assert_eq!(m.history.len(), 1);
        assert_eq!(m.history[0].reason, "observe");
    }

    #[test]
    fn test_leaving_to_removed() {
        let mut store = MembershipStore::new();
        store.register(node(6), 1).unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(6),
                requested_status: MembershipStatus::Leaving,
                reason: "exit".into(),
                epoch: 2,
            })
            .unwrap();
        store
            .transition(RoleTransitionRequest {
                node_id: node(6),
                requested_status: MembershipStatus::Removed,
                reason: "done".into(),
                epoch: 3,
            })
            .unwrap();
        assert!(store.get(&node(6)).unwrap().status.is_terminal());
    }
}
