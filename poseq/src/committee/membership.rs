use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::identities::node::{NodeIdentity, NodeRole, NodeStatus};
use crate::identities::registry::{ChainSequencerStatus, SequencerRegistry};
use crate::membership::MembershipStore;
use crate::chain_bridge::snapshot::ChainCommitteeSnapshot;

/// A snapshot of committee membership for a given epoch.
#[derive(Debug, Clone)]
pub struct PoSeqCommittee {
    pub epoch: u64,
    /// Active sequencers (eligible proposers + attestors), keyed by node_id.
    pub sequencers: BTreeMap<[u8; 32], NodeIdentity>,
    /// Active validators (eligible attestors only), keyed by node_id.
    pub attestors: BTreeMap<[u8; 32], NodeIdentity>,
}

impl PoSeqCommittee {
    pub fn new(epoch: u64) -> Self {
        PoSeqCommittee {
            epoch,
            sequencers: BTreeMap::new(),
            attestors: BTreeMap::new(),
        }
    }

    /// Add a node to the appropriate map based on role. Only Active nodes count.
    pub fn add_node(&mut self, node: NodeIdentity) {
        match node.role {
            NodeRole::Sequencer => {
                self.sequencers.insert(node.node_id, node);
            }
            NodeRole::Validator => {
                self.attestors.insert(node.node_id, node);
            }
            NodeRole::ObserverOnly => {
                // observers do not participate in committee
            }
        }
    }

    /// Returns all nodes eligible to propose (active sequencers).
    pub fn eligible_proposers(&self) -> Vec<&NodeIdentity> {
        self.sequencers
            .values()
            .filter(|n| n.is_active() && n.is_eligible_proposer)
            .collect()
    }

    /// Returns all nodes eligible to attest (active sequencers + active validators).
    pub fn eligible_attestors(&self) -> Vec<&NodeIdentity> {
        let mut attestors: Vec<&NodeIdentity> = self
            .sequencers
            .values()
            .filter(|n| n.is_active() && n.is_eligible_attestor)
            .collect();
        let mut validators: Vec<&NodeIdentity> = self
            .attestors
            .values()
            .filter(|n| n.is_active() && n.is_eligible_attestor)
            .collect();
        attestors.append(&mut validators);
        // sort by node_id for determinism
        attestors.sort_by_key(|n| n.node_id);
        attestors
    }

    /// Check if a node_id is a member of this committee (any role).
    pub fn is_member(&self, node_id: &[u8; 32]) -> bool {
        self.sequencers.contains_key(node_id) || self.attestors.contains_key(node_id)
    }

    /// Returns total number of eligible attestors (for quorum calculation).
    pub fn quorum_size(&self) -> usize {
        self.eligible_attestors().len()
    }

    /// Build a committee from a chain-imported snapshot.
    ///
    /// Excludes nodes whose `chain_status` is not `Active` or whose
    /// `MembershipStore` status is `Suspended`, `Banned`, or `Removed`.
    ///
    /// Returns `Err` if:
    /// - The snapshot hash fails verification
    /// - The snapshot epoch does not match `epoch`
    /// - Fewer than `min_members` eligible nodes remain after filtering
    pub fn from_chain_snapshot(
        snapshot: &ChainCommitteeSnapshot,
        registry: &SequencerRegistry,
        membership: &MembershipStore,
        epoch: u64,
        min_members: usize,
    ) -> Result<Self, CommitteeFormationError> {
        if snapshot.epoch != epoch {
            return Err(CommitteeFormationError::EpochMismatch {
                snapshot_epoch: snapshot.epoch,
                requested_epoch: epoch,
            });
        }
        if !snapshot.verify_hash() {
            return Err(CommitteeFormationError::SnapshotVerificationFailed);
        }

        let mut committee = PoSeqCommittee::new(epoch);

        for member in &snapshot.members {
            // Decode node_id
            let Ok(id_bytes) = hex::decode(&member.node_id) else {
                continue;
            };
            if id_bytes.len() != 32 {
                continue;
            }
            let mut node_id = [0u8; 32];
            node_id.copy_from_slice(&id_bytes);

            // Check chain-authoritative status
            if !registry.is_chain_eligible(&node_id) {
                continue;
            }

            // Check internal MembershipStore status
            if !membership.is_eligible_for_committee(&node_id) {
                continue;
            }

            // Resolve public key from registry (or decode from snapshot)
            let public_key = if let Some(rec) = registry.get(&node_id) {
                rec.public_key
            } else {
                let Ok(pk_bytes) = hex::decode(&member.public_key) else { continue };
                if pk_bytes.len() != 32 { continue; }
                let mut pk = [0u8; 32];
                pk.copy_from_slice(&pk_bytes);
                pk
            };

            let role = match member.role.as_str() {
                "Sequencer" => NodeRole::Sequencer,
                "Validator" => NodeRole::Validator,
                _ => NodeRole::Sequencer,
            };

            let mut identity = NodeIdentity::new(node_id, public_key, role, epoch);
            identity.activate();
            committee.add_node(identity);
        }

        let total = committee.sequencers.len() + committee.attestors.len();
        if total < min_members {
            return Err(CommitteeFormationError::InsufficientActiveMembers {
                required: min_members,
                available: total,
            });
        }

        Ok(committee)
    }

    /// Compute the committee root: SHA256 over sorted node_ids of all members.
    pub fn compute_committee_root(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        // BTreeMap iteration is sorted, so this is deterministic
        for id in self.sequencers.keys() {
            hasher.update(id);
        }
        for id in self.attestors.keys() {
            hasher.update(id);
        }
        hasher.update(&self.epoch.to_be_bytes());
        let hash = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&hash);
        root
    }
}

/// Metadata record for a committee's epoch span.
#[derive(Debug, Clone)]
pub struct CommitteeEpoch {
    pub epoch_number: u64,
    pub start_height: u64,
    pub end_height: u64,
    pub committee_root: [u8; 32],
}

impl CommitteeEpoch {
    pub fn new(epoch_number: u64, start_height: u64, end_height: u64, committee: &PoSeqCommittee) -> Self {
        CommitteeEpoch {
            epoch_number,
            start_height,
            end_height,
            committee_root: committee.compute_committee_root(),
        }
    }
}

/// Immutable membership snapshot for audit / state-sync purposes.
#[derive(Debug, Clone)]
pub struct CommitteeMembershipRecord {
    pub epoch: u64,
    pub sequencer_ids: Vec<[u8; 32]>,
    pub attestor_ids: Vec<[u8; 32]>,
    pub committee_root: [u8; 32],
    pub quorum_size: usize,
}

impl CommitteeMembershipRecord {
    pub fn from_committee(committee: &PoSeqCommittee) -> Self {
        let sequencer_ids: Vec<[u8; 32]> = committee.sequencers.keys().cloned().collect();
        let attestor_ids: Vec<[u8; 32]> = committee.attestors.keys().cloned().collect();
        CommitteeMembershipRecord {
            epoch: committee.epoch,
            sequencer_ids,
            attestor_ids,
            committee_root: committee.compute_committee_root(),
            quorum_size: committee.quorum_size(),
        }
    }
}

// ─── CommitteeFormationError ─────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CommitteeFormationError {
    /// The `snapshot_hash` field does not match the recomputed hash.
    SnapshotVerificationFailed,
    /// Fewer than `required` nodes remain after filtering inactive/suspended nodes.
    InsufficientActiveMembers { required: usize, available: usize },
    /// The snapshot's epoch does not match the requested epoch.
    EpochMismatch { snapshot_epoch: u64, requested_epoch: u64 },
}

impl std::fmt::Display for CommitteeFormationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::SnapshotVerificationFailed => write!(f, "snapshot hash verification failed"),
            Self::InsufficientActiveMembers { required, available } => {
                write!(f, "insufficient active members: need {required}, have {available}")
            }
            Self::EpochMismatch { snapshot_epoch, requested_epoch } => {
                write!(f, "epoch mismatch: snapshot={snapshot_epoch}, requested={requested_epoch}")
            }
        }
    }
}

impl std::error::Error for CommitteeFormationError {}

// ─── sync_membership_from_chain ──────────────────────────────────────────────

/// Sync the local `MembershipStore` to reflect chain-authoritative status changes.
///
/// Iterates over all records in `registry` and, for each node whose
/// `chain_status` is not `Active`, transitions the `MembershipStore` entry
/// to `Suspended` (if it isn't already terminal).
///
/// Called after importing a snapshot or ingesting an `ExportBatch` with
/// `status_recommendations`.
pub fn sync_membership_from_chain(
    membership: &mut MembershipStore,
    registry: &SequencerRegistry,
    epoch: u64,
) {
    for node_id in registry.all_node_ids() {
        if let Some(rec) = registry.get(&node_id) {
            if !rec.chain_status.is_committee_eligible() {
                // Drive the local membership store to Suspended so it
                // reflects the chain-authoritative view.
                membership.suspend_node(&node_id, epoch);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::identities::node::{NodeIdentity, NodeRole};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn active_sequencer(b: u8) -> NodeIdentity {
        let mut n = NodeIdentity::new(make_id(b), make_id(b + 100), NodeRole::Sequencer, 0);
        n.activate();
        n
    }

    fn active_validator(b: u8) -> NodeIdentity {
        let mut n = NodeIdentity::new(make_id(b), make_id(b + 100), NodeRole::Validator, 0);
        n.activate();
        n
    }

    #[test]
    fn test_eligible_proposers() {
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(active_sequencer(1));
        committee.add_node(active_sequencer(2));
        committee.add_node(active_validator(3));
        let proposers = committee.eligible_proposers();
        assert_eq!(proposers.len(), 2);
    }

    #[test]
    fn test_eligible_attestors_includes_both() {
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(active_sequencer(1));
        committee.add_node(active_validator(2));
        committee.add_node(active_validator(3));
        let attestors = committee.eligible_attestors();
        assert_eq!(attestors.len(), 3);
    }

    #[test]
    fn test_quorum_size() {
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(active_sequencer(1));
        committee.add_node(active_sequencer(2));
        committee.add_node(active_validator(3));
        assert_eq!(committee.quorum_size(), 3);
    }

    #[test]
    fn test_is_member() {
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(active_sequencer(1));
        assert!(committee.is_member(&make_id(1)));
        assert!(!committee.is_member(&make_id(99)));
    }

    #[test]
    fn test_suspended_node_not_eligible() {
        let mut committee = PoSeqCommittee::new(1);
        let mut node = active_sequencer(1);
        node.suspend();
        committee.add_node(node);
        assert_eq!(committee.eligible_proposers().len(), 0);
        assert_eq!(committee.eligible_attestors().len(), 0);
    }

    #[test]
    fn test_committee_root_determinism() {
        let mut c1 = PoSeqCommittee::new(1);
        c1.add_node(active_sequencer(1));
        c1.add_node(active_validator(2));

        let mut c2 = PoSeqCommittee::new(1);
        c2.add_node(active_sequencer(1));
        c2.add_node(active_validator(2));

        assert_eq!(c1.compute_committee_root(), c2.compute_committee_root());
    }
}
