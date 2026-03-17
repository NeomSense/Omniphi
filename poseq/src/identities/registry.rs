//! SequencerRegistry — in-process registry of registered sequencer identities.
//!
//! Sequencers register on-chain via `MsgRegisterSequencer`. At startup the
//! `NetworkedNode` loads the registry snapshot from the chain (or from the
//! durable store) and uses it to:
//!  - validate HotStuff block signer eligibility
//!  - populate the initial committee for `HotStuffEngine`
//!  - reject `WireSignedEnvelope` messages from unknown/unregistered nodes

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::identities::node::{NodeIdentity, NodeRole, NodeStatus};

/// A compact on-chain registration record for a sequencer.
/// Mirrors the Go `SequencerRecord` in chain/x/poseq/types/sequencer.go.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SequencerRecord {
    /// 32-byte node ID (Ed25519 public key hash or operator-chosen ID).
    pub node_id: [u8; 32],
    /// Ed25519 public key bytes — used for wire signature verification.
    pub public_key: [u8; 32],
    /// Human-readable moniker (≤ 64 bytes).
    pub moniker: String,
    /// Cosmos-SDK bech32 operator address.
    pub operator_address: String,
    /// Epoch in which this sequencer registered.
    pub registered_epoch: u64,
    /// Whether the chain has activated this sequencer (bonded + governance approved).
    pub is_active: bool,
}

impl SequencerRecord {
    pub fn node_id_hex(&self) -> String {
        hex::encode(self.node_id)
    }
}

/// SequencerRegistry holds all known registered sequencers.
///
/// This is updated by:
///  1. `load_from_chain_snapshot()` at node startup
///  2. `apply_registration()` when a `WireSequencerRegistered` message is received
///  3. `apply_deactivation()` on slashing/governance ejection
pub struct SequencerRegistry {
    /// node_id → SequencerRecord
    records: BTreeMap<[u8; 32], SequencerRecord>,
}

impl SequencerRegistry {
    pub fn new() -> Self {
        SequencerRegistry {
            records: BTreeMap::new(),
        }
    }

    /// Insert or update a sequencer record.
    pub fn apply_registration(&mut self, record: SequencerRecord) {
        self.records.insert(record.node_id, record);
    }

    /// Deactivate a sequencer (slashing / governance ejection).
    pub fn apply_deactivation(&mut self, node_id: &[u8; 32]) {
        if let Some(rec) = self.records.get_mut(node_id) {
            rec.is_active = false;
        }
    }

    /// Re-activate a sequencer (after appeal / governance reversal).
    pub fn apply_reactivation(&mut self, node_id: &[u8; 32]) {
        if let Some(rec) = self.records.get_mut(node_id) {
            rec.is_active = true;
        }
    }

    /// Look up a sequencer record by node_id.
    pub fn get(&self, node_id: &[u8; 32]) -> Option<&SequencerRecord> {
        self.records.get(node_id)
    }

    /// Returns true if the node_id is registered AND active.
    pub fn is_active(&self, node_id: &[u8; 32]) -> bool {
        self.records.get(node_id).map(|r| r.is_active).unwrap_or(false)
    }

    /// Returns the Ed25519 public key for a node, if registered.
    pub fn public_key(&self, node_id: &[u8; 32]) -> Option<[u8; 32]> {
        self.records.get(node_id).map(|r| r.public_key)
    }

    /// Returns all active sequencer node IDs.
    pub fn active_node_ids(&self) -> Vec<[u8; 32]> {
        self.records
            .values()
            .filter(|r| r.is_active)
            .map(|r| r.node_id)
            .collect()
    }

    /// Returns the total count of registered sequencers (active + inactive).
    pub fn len(&self) -> usize {
        self.records.len()
    }

    pub fn is_empty(&self) -> bool {
        self.records.is_empty()
    }

    /// Convert the active registry entries into `NodeIdentity` structs for
    /// committee formation. Called by `committee::membership` when rebuilding
    /// the epoch committee.
    pub fn to_node_identities(&self) -> Vec<NodeIdentity> {
        self.records
            .values()
            .filter(|r| r.is_active)
            .map(|r| {
                let mut identity = NodeIdentity::new(
                    r.node_id,
                    r.public_key,
                    NodeRole::Sequencer,
                    r.registered_epoch,
                );
                identity.activate();
                identity
            })
            .collect()
    }

    /// Bulk-load from a chain snapshot. Replaces all existing records.
    /// Used at node startup when syncing from chain state.
    pub fn load_from_snapshot(&mut self, records: Vec<SequencerRecord>) {
        self.records.clear();
        for rec in records {
            self.records.insert(rec.node_id, rec);
        }
    }
}

impl Default for SequencerRegistry {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Wire message for p2p propagation ─────────────────────────────────────────

/// Sent by a sequencer to notify peers of its registration event.
/// Triggered after `MsgRegisterSequencer` is included on-chain.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireSequencerRegistered {
    pub record: SequencerRecord,
    /// Chain block height at which the registration tx was finalized.
    pub block_height: u64,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_record(node_byte: u8, pk_byte: u8, epoch: u64, active: bool) -> SequencerRecord {
        SequencerRecord {
            node_id: make_id(node_byte),
            public_key: make_id(pk_byte),
            moniker: format!("seq-{}", node_byte),
            operator_address: format!("omni1test{}", node_byte),
            registered_epoch: epoch,
            is_active: active,
        }
    }

    #[test]
    fn test_register_and_lookup() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(1, 10, 1, true));

        let id = make_id(1);
        assert!(reg.is_active(&id));
        assert_eq!(reg.public_key(&id), Some(make_id(10)));
    }

    #[test]
    fn test_deactivation() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(2, 20, 1, true));
        let id = make_id(2);
        assert!(reg.is_active(&id));
        reg.apply_deactivation(&id);
        assert!(!reg.is_active(&id));
    }

    #[test]
    fn test_reactivation() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(3, 30, 1, false));
        let id = make_id(3);
        assert!(!reg.is_active(&id));
        reg.apply_reactivation(&id);
        assert!(reg.is_active(&id));
    }

    #[test]
    fn test_active_node_ids() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(1, 10, 1, true));
        reg.apply_registration(make_record(2, 20, 1, false));
        reg.apply_registration(make_record(3, 30, 1, true));

        let active = reg.active_node_ids();
        assert_eq!(active.len(), 2);
        assert!(active.contains(&make_id(1)));
        assert!(active.contains(&make_id(3)));
    }

    #[test]
    fn test_to_node_identities() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(1, 10, 2, true));
        reg.apply_registration(make_record(2, 20, 2, false));

        let identities = reg.to_node_identities();
        assert_eq!(identities.len(), 1);
        assert!(identities[0].is_active());
        assert!(identities[0].is_eligible_proposer);
    }

    #[test]
    fn test_load_from_snapshot() {
        let mut reg = SequencerRegistry::new();
        reg.apply_registration(make_record(99, 99, 0, true));
        assert_eq!(reg.len(), 1);

        let snapshot = vec![
            make_record(1, 10, 1, true),
            make_record(2, 20, 1, true),
        ];
        reg.load_from_snapshot(snapshot);
        assert_eq!(reg.len(), 2);
        assert!(!reg.is_active(&make_id(99)));
    }
}
