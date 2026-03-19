/// Domain prefixes for persistence keys.
pub mod prefix {
    pub const PROPOSALS: &[u8] = b"proposals:";
    pub const ATTESTATIONS: &[u8] = b"attestations:";
    pub const FAIRNESS: &[u8] = b"fairness:";
    pub const SLASHING: &[u8] = b"slashing:";
    pub const ROTATION: &[u8] = b"rotation:";
    pub const JAIL: &[u8] = b"jail:";
    pub const FINALIZED: &[u8] = b"finalized:";
    pub const NODE: &[u8] = b"node:";
    pub const EPOCH: &[u8] = b"epoch:";
    pub const SIMULATION: &[u8] = b"simulation:";
    // Phase 6A: cross-layer persistence
    pub const RUNTIME_INGESTION: &[u8] = b"runtime:last_ingested:";
    pub const RUNTIME_DELIVERY: &[u8] = b"runtime:delivery_log:";
    pub const SETTLEMENT_LAST: &[u8] = b"settlement:last_submitted:";
    pub const SNAPSHOT: &[u8] = b"snapshot:";
    pub const SNAPSHOT_META: &[u8] = b"snapshot_meta:";
    pub const DISCOVERED_PEERS: &[u8] = b"peers:discovered:";
    // Dual-lane operator alignment
    pub const CHAIN_SNAPSHOT: &[u8] = b"chain_snapshot:";
    pub const LIVENESS_TRACKER: &[u8] = b"liveness:";
    pub const PERF_TRACKER: &[u8] = b"perf:";
}

/// Typed key builder with prefixes for each data domain.
pub struct PersistenceKey;

impl PersistenceKey {
    /// Build a proposal key: `proposals:<proposal_id_hex>`.
    pub fn proposal(proposal_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::PROPOSALS, proposal_id)
    }

    /// Build an attestation key: `attestations:<proposal_id_hex>`.
    pub fn attestation(proposal_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::ATTESTATIONS, proposal_id)
    }

    /// Build a finalized batch key: `finalized:<batch_id_hex>`.
    pub fn finalized(batch_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::FINALIZED, batch_id)
    }

    /// Build a fairness record key: `fairness:<record_id_hex>`.
    pub fn fairness(record_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::FAIRNESS, record_id)
    }

    /// Build a slash record key: `slashing:<node_id_hex>:<index_bytes>`.
    pub fn slash_record(node_id: &[u8; 32], index: u64) -> Vec<u8> {
        let mut key = prefix::SLASHING.to_vec();
        key.extend_from_slice(node_id);
        key.push(b':');
        key.extend_from_slice(&index.to_be_bytes());
        key
    }

    /// Build a jail key: `jail:<node_id_hex>`.
    pub fn jail(node_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::JAIL, node_id)
    }

    /// Build a rotation snapshot key: `rotation:<epoch_bytes>`.
    pub fn rotation_snapshot(epoch: u64) -> Vec<u8> {
        let mut key = prefix::ROTATION.to_vec();
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    /// Build a node key: `node:<node_id_hex>`.
    pub fn node(node_id: &[u8; 32]) -> Vec<u8> {
        Self::join(prefix::NODE, node_id)
    }

    /// Build an epoch key: `epoch:<epoch_bytes>`.
    pub fn epoch(epoch: u64) -> Vec<u8> {
        let mut key = prefix::EPOCH.to_vec();
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    /// Build a simulation state key: `simulation:<label>`.
    pub fn simulation(label: &[u8]) -> Vec<u8> {
        let mut key = prefix::SIMULATION.to_vec();
        key.extend_from_slice(label);
        key
    }

    /// Construct prefix for a node's slash records: `slashing:<node_id_hex>:`.
    pub fn slash_prefix(node_id: &[u8; 32]) -> Vec<u8> {
        let mut key = prefix::SLASHING.to_vec();
        key.extend_from_slice(node_id);
        key.push(b':');
        key
    }

    /// Build a chain snapshot key: `chain_snapshot:<epoch_be(8)>`.
    pub fn chain_snapshot(epoch: u64) -> Vec<u8> {
        let mut key = prefix::CHAIN_SNAPSHOT.to_vec();
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    /// Build a liveness tracker key: `liveness:<node_id><epoch_be(8)>`.
    pub fn liveness(node_id: &[u8; 32], epoch: u64) -> Vec<u8> {
        let mut key = prefix::LIVENESS_TRACKER.to_vec();
        key.extend_from_slice(node_id);
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    /// Build a performance tracker key: `perf:<node_id><epoch_be(8)>`.
    pub fn performance(node_id: &[u8; 32], epoch: u64) -> Vec<u8> {
        let mut key = prefix::PERF_TRACKER.to_vec();
        key.extend_from_slice(node_id);
        key.extend_from_slice(&epoch.to_be_bytes());
        key
    }

    fn join(prefix: &[u8], id: &[u8; 32]) -> Vec<u8> {
        let mut key = prefix.to_vec();
        key.extend_from_slice(id);
        key
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
    fn test_proposal_key_has_prefix() {
        let key = PersistenceKey::proposal(&make_id(1));
        assert!(key.starts_with(prefix::PROPOSALS));
    }

    #[test]
    fn test_different_ids_different_keys() {
        let k1 = PersistenceKey::proposal(&make_id(1));
        let k2 = PersistenceKey::proposal(&make_id(2));
        assert_ne!(k1, k2);
    }

    #[test]
    fn test_different_domain_different_keys() {
        let id = make_id(1);
        let k1 = PersistenceKey::proposal(&id);
        let k2 = PersistenceKey::attestation(&id);
        assert_ne!(k1, k2);
    }

    #[test]
    fn test_slash_record_key_distinguishes_index() {
        let id = make_id(1);
        let k1 = PersistenceKey::slash_record(&id, 0);
        let k2 = PersistenceKey::slash_record(&id, 1);
        assert_ne!(k1, k2);
    }

    #[test]
    fn test_slash_prefix_is_prefix_of_record_key() {
        let id = make_id(1);
        let prefix = PersistenceKey::slash_prefix(&id);
        let key = PersistenceKey::slash_record(&id, 42);
        assert!(key.starts_with(&prefix));
    }

    #[test]
    fn test_rotation_snapshot_key_sorted_by_epoch() {
        let k1 = PersistenceKey::rotation_snapshot(1);
        let k2 = PersistenceKey::rotation_snapshot(2);
        // big-endian encoding → lexicographic order = numeric order
        assert!(k1 < k2);
    }

    #[test]
    fn test_epoch_key_sorted() {
        let k1 = PersistenceKey::epoch(5);
        let k2 = PersistenceKey::epoch(6);
        assert!(k1 < k2);
    }

    #[test]
    fn test_jail_key_has_prefix() {
        let key = PersistenceKey::jail(&make_id(5));
        assert!(key.starts_with(prefix::JAIL));
    }

    #[test]
    fn test_simulation_key_has_prefix() {
        let key = PersistenceKey::simulation(b"state");
        assert!(key.starts_with(prefix::SIMULATION));
    }

    #[test]
    fn test_finalized_key_has_prefix() {
        let key = PersistenceKey::finalized(&make_id(7));
        assert!(key.starts_with(prefix::FINALIZED));
    }

    #[test]
    fn test_node_key_has_prefix() {
        let key = PersistenceKey::node(&make_id(3));
        assert!(key.starts_with(prefix::NODE));
    }
}
