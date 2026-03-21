//! Chain-imported committee snapshot types.
//!
//! The Cosmos chain (`x/poseq`) produces a deterministic `CommitteeSnapshot`
//! from the set of Active sequencers at each epoch boundary. PoSeq nodes
//! import this snapshot via a relayer and use it as the authoritative
//! committee composition, enforcing operator alignment without merging
//! consensus domains.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// One member entry in a chain-produced committee snapshot.
/// Mirrors Go `types.CommitteeSnapshotMember`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ChainCommitteeMember {
    /// 64-char hex-encoded 32-byte node identity.
    pub node_id: String,
    /// 64-char hex-encoded Ed25519 public key (32 bytes).
    pub public_key: String,
    /// Human-readable operator label.
    pub moniker: String,
    /// `"Sequencer"` (propose + attest) or `"Validator"` (attest only).
    pub role: String,
}

/// A deterministic committee snapshot produced by the Cosmos chain.
/// Mirrors Go `types.CommitteeSnapshot`.
///
/// `snapshot_hash = SHA256("committee_snapshot" | epoch_be(8) | member_count_be(4) | sorted_node_id_bytes...)`
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainCommitteeSnapshot {
    /// The PoSeq epoch this snapshot covers.
    pub epoch: u64,
    /// Committee members, sorted by `node_id` (lexicographic on raw 32-byte value).
    pub members: Vec<ChainCommitteeMember>,
    /// 32-byte canonical integrity hash.
    pub snapshot_hash: Vec<u8>,
    /// Cosmos block height at which the snapshot was produced.
    pub produced_at_block: i64,
}

impl ChainCommitteeSnapshot {
    /// Compute the canonical snapshot hash for the given epoch and sorted node IDs.
    ///
    /// `hash = SHA256("committee_snapshot" | epoch_be(8) | member_count_be(4) | sorted_node_id_bytes...)`
    pub fn compute_hash(epoch: u64, sorted_node_ids: &[[u8; 32]]) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"committee_snapshot");
        h.update(epoch.to_be_bytes());
        h.update((sorted_node_ids.len() as u32).to_be_bytes());
        for id in sorted_node_ids {
            h.update(id);
        }
        h.finalize().into()
    }

    /// Decode member node IDs into sorted raw `[u8; 32]` arrays.
    pub fn sorted_member_ids(&self) -> Result<Vec<[u8; 32]>, SnapshotImportError> {
        let mut ids = Vec::with_capacity(self.members.len());
        for m in &self.members {
            let b = hex::decode(&m.node_id).map_err(|e| SnapshotImportError::InvalidNodeId {
                node_id: m.node_id.clone(),
                error: e.to_string(),
            })?;
            if b.len() != 32 {
                return Err(SnapshotImportError::InvalidNodeId {
                    node_id: m.node_id.clone(),
                    error: format!("expected 32 bytes, got {}", b.len()),
                });
            }
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&b);
            ids.push(arr);
        }
        // Ensure sorted (chain should already sort, but verify)
        ids.sort();
        Ok(ids)
    }

    /// Verify that `snapshot_hash` matches the recomputed hash.
    pub fn verify_hash(&self) -> bool {
        let Ok(ids) = self.sorted_member_ids() else {
            return false;
        };
        let computed = Self::compute_hash(self.epoch, &ids);
        if self.snapshot_hash.len() != 32 {
            return false;
        }
        let mut expected = [0u8; 32];
        expected.copy_from_slice(&self.snapshot_hash);
        computed == expected
    }
}

// ─── SnapshotImporter ────────────────────────────────────────────────────────

/// Manages import and in-memory caching of chain-produced committee snapshots.
/// One instance per PoSeq node; populated at startup and at each epoch boundary.
pub struct SnapshotImporter {
    /// Verified snapshots keyed by epoch.
    snapshots: BTreeMap<u64, ChainCommitteeSnapshot>,
    /// Highest produced_at_block seen across all imported snapshots.
    /// Used to reject stale/replayed snapshots from a compromised relayer.
    max_produced_at_block: i64,
}

impl SnapshotImporter {
    pub fn new() -> Self {
        Self {
            snapshots: BTreeMap::new(),
            max_produced_at_block: 0,
        }
    }

    /// Import a snapshot received from the chain (via relayer or wire message).
    ///
    /// Verifies the hash before caching. Returns `Err` if:
    /// - Hash verification fails
    /// - A snapshot for this epoch is already cached
    /// - Any member node_id is not valid 32-byte hex
    /// - The snapshot's produced_at_block is older than a previously seen snapshot
    ///   (prevents relayer from rolling back committee composition)
    pub fn import(&mut self, snap: ChainCommitteeSnapshot) -> Result<(), SnapshotImportError> {
        if self.snapshots.contains_key(&snap.epoch) {
            return Err(SnapshotImportError::DuplicateEpoch(snap.epoch));
        }
        if !snap.verify_hash() {
            return Err(SnapshotImportError::HashMismatch { epoch: snap.epoch });
        }
        // Reject snapshots produced at a block height older than what we've
        // already seen. This prevents a compromised relayer from delivering
        // old, valid snapshots to roll back committee composition.
        if snap.produced_at_block > 0 && snap.produced_at_block < self.max_produced_at_block {
            return Err(SnapshotImportError::StaleSnapshot {
                epoch: snap.epoch,
                produced_at: snap.produced_at_block,
                max_seen: self.max_produced_at_block,
            });
        }
        if snap.produced_at_block > self.max_produced_at_block {
            self.max_produced_at_block = snap.produced_at_block;
        }
        self.snapshots.insert(snap.epoch, snap);
        Ok(())
    }

    /// Get the verified snapshot for an epoch. Returns `None` if not yet imported.
    pub fn get(&self, epoch: u64) -> Option<&ChainCommitteeSnapshot> {
        self.snapshots.get(&epoch)
    }

    /// Returns `true` if a verified snapshot exists for the given epoch.
    pub fn has_epoch(&self, epoch: u64) -> bool {
        self.snapshots.contains_key(&epoch)
    }

    /// Returns the latest (highest epoch) snapshot available, if any.
    pub fn latest(&self) -> Option<&ChainCommitteeSnapshot> {
        self.snapshots.values().next_back()
    }
}

impl Default for SnapshotImporter {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Errors ──────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SnapshotImportError {
    /// The `snapshot_hash` in the payload does not match the recomputed hash.
    HashMismatch { epoch: u64 },
    /// A snapshot for this epoch was already imported.
    DuplicateEpoch(u64),
    /// A member `node_id` is not valid 32-byte hex.
    InvalidNodeId { node_id: String, error: String },
    /// The snapshot was produced at a block height older than a previously imported snapshot.
    StaleSnapshot { epoch: u64, produced_at: i64, max_seen: i64 },
}

impl std::fmt::Display for SnapshotImportError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::HashMismatch { epoch } => write!(f, "snapshot hash mismatch for epoch {epoch}"),
            Self::DuplicateEpoch(epoch) => write!(f, "snapshot already imported for epoch {epoch}"),
            Self::StaleSnapshot { epoch, produced_at, max_seen } => {
                write!(f, "stale snapshot for epoch {epoch}: produced_at_block={produced_at} < max_seen={max_seen}")
            }
            Self::InvalidNodeId { node_id, error } => {
                write!(f, "invalid node_id {node_id}: {error}")
            }
        }
    }
}

impl std::error::Error for SnapshotImportError {}

// ─── Tests ───────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn make_member(node_id_byte: u8) -> ChainCommitteeMember {
        let mut id = [0u8; 32];
        id[0] = node_id_byte;
        ChainCommitteeMember {
            node_id: hex::encode(id),
            public_key: hex::encode([node_id_byte; 32]),
            moniker: format!("node-{node_id_byte}"),
            role: "Sequencer".to_string(),
        }
    }

    fn make_snapshot(epoch: u64, members: Vec<ChainCommitteeMember>) -> ChainCommitteeSnapshot {
        // Build sorted IDs and compute hash
        let mut ids: Vec<[u8; 32]> = members
            .iter()
            .map(|m| {
                let b = hex::decode(&m.node_id).unwrap();
                let mut arr = [0u8; 32];
                arr.copy_from_slice(&b);
                arr
            })
            .collect();
        ids.sort();
        let hash = ChainCommitteeSnapshot::compute_hash(epoch, &ids);
        ChainCommitteeSnapshot {
            epoch,
            members,
            snapshot_hash: hash.to_vec(),
            produced_at_block: 100,
        }
    }

    #[test]
    fn test_snapshot_hash_roundtrip() {
        let snap = make_snapshot(5, vec![make_member(1), make_member(2), make_member(3)]);
        assert!(snap.verify_hash(), "hash should verify");
    }

    #[test]
    fn test_snapshot_hash_tamper_rejected() {
        let mut snap = make_snapshot(5, vec![make_member(1), make_member(2)]);
        snap.snapshot_hash[0] ^= 0xFF; // tamper
        assert!(!snap.verify_hash(), "tampered hash should fail");
    }

    #[test]
    fn test_import_success() {
        let mut importer = SnapshotImporter::new();
        let snap = make_snapshot(1, vec![make_member(1)]);
        assert!(importer.import(snap).is_ok());
        assert!(importer.has_epoch(1));
    }

    #[test]
    fn test_import_duplicate_epoch_rejected() {
        let mut importer = SnapshotImporter::new();
        let snap1 = make_snapshot(2, vec![make_member(1)]);
        let snap2 = make_snapshot(2, vec![make_member(2)]);
        importer.import(snap1).unwrap();
        let err = importer.import(snap2).unwrap_err();
        assert_eq!(err, SnapshotImportError::DuplicateEpoch(2));
    }

    #[test]
    fn test_import_bad_hash_rejected() {
        let mut snap = make_snapshot(3, vec![make_member(1)]);
        snap.snapshot_hash[0] ^= 0xFF;
        let mut importer = SnapshotImporter::new();
        let err = importer.import(snap).unwrap_err();
        assert!(matches!(err, SnapshotImportError::HashMismatch { epoch: 3 }));
    }

    #[test]
    fn test_snapshot_determinism() {
        // Same inputs → same hash regardless of insertion order
        let members_a = vec![make_member(3), make_member(1), make_member(2)];
        let members_b = vec![make_member(1), make_member(2), make_member(3)];
        let snap_a = make_snapshot(10, members_a);
        let snap_b = make_snapshot(10, members_b);
        assert_eq!(snap_a.snapshot_hash, snap_b.snapshot_hash);
    }
}
