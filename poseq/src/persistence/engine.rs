use crate::persistence::backend::PersistenceBackend;
use crate::persistence::keys::PersistenceKey;

/// Wraps a boxed PersistenceBackend and provides domain-specific helpers.
pub struct PersistenceEngine {
    backend: Box<dyn PersistenceBackend>,
}

impl PersistenceEngine {
    pub fn new(backend: Box<dyn PersistenceBackend>) -> Self {
        PersistenceEngine { backend }
    }

    /// Immutable reference to the underlying backend (e.g. for HotStuff SafetyRule restore).
    pub fn backend(&self) -> &dyn PersistenceBackend {
        self.backend.as_ref()
    }

    /// Mutable reference to the underlying backend (e.g. for HotStuff SafetyRule persist).
    pub fn backend_mut(&mut self) -> &mut dyn PersistenceBackend {
        self.backend.as_mut()
    }

    // ─── Generic read/write ──────────────────────────────────────────────────

    pub fn get_raw(&self, key: &[u8]) -> Option<Vec<u8>> {
        self.backend.get(key)
    }

    pub fn put_raw(&mut self, key: &[u8], value: Vec<u8>) {
        self.backend.put(key, value);
    }

    pub fn delete_raw(&mut self, key: &[u8]) {
        self.backend.delete(key);
    }

    pub fn prefix_scan_raw(&self, prefix: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(prefix)
    }

    pub fn contains(&self, key: &[u8]) -> bool {
        self.backend.contains(key)
    }

    /// Flush all pending writes to durable storage. Call after persisting
    /// finalized batches and committee snapshots to guarantee crash safety.
    pub fn flush(&self) {
        self.backend.flush();
    }

    // ─── Proposal domain ────────────────────────────────────────────────────

    /// Store raw bytes for a proposal.
    pub fn put_proposal(&mut self, proposal_id: &[u8; 32], data: Vec<u8>) {
        let key = PersistenceKey::proposal(proposal_id);
        self.backend.put(&key, data);
    }

    /// Retrieve raw bytes for a proposal.
    pub fn get_proposal(&self, proposal_id: &[u8; 32]) -> Option<Vec<u8>> {
        let key = PersistenceKey::proposal(proposal_id);
        self.backend.get(&key)
    }

    /// Scan all proposal entries.
    pub fn scan_proposals(&self) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(crate::persistence::keys::prefix::PROPOSALS)
    }

    // ─── Attestation domain ─────────────────────────────────────────────────

    pub fn put_attestation(&mut self, proposal_id: &[u8; 32], data: Vec<u8>) {
        let key = PersistenceKey::attestation(proposal_id);
        self.backend.put(&key, data);
    }

    pub fn get_attestation(&self, proposal_id: &[u8; 32]) -> Option<Vec<u8>> {
        let key = PersistenceKey::attestation(proposal_id);
        self.backend.get(&key)
    }

    // ─── Finalized batch domain ──────────────────────────────────────────────

    pub fn put_finalized(&mut self, batch_id: &[u8; 32], data: Vec<u8>) {
        let key = PersistenceKey::finalized(batch_id);
        self.backend.put(&key, data);
    }

    pub fn get_finalized(&self, batch_id: &[u8; 32]) -> Option<Vec<u8>> {
        let key = PersistenceKey::finalized(batch_id);
        self.backend.get(&key)
    }

    pub fn scan_finalized(&self) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(crate::persistence::keys::prefix::FINALIZED)
    }

    // ─── Slash record domain ─────────────────────────────────────────────────

    pub fn put_slash_record(&mut self, node_id: &[u8; 32], index: u64, data: Vec<u8>) {
        let key = PersistenceKey::slash_record(node_id, index);
        self.backend.put(&key, data);
    }

    pub fn get_slash_record(&self, node_id: &[u8; 32], index: u64) -> Option<Vec<u8>> {
        let key = PersistenceKey::slash_record(node_id, index);
        self.backend.get(&key)
    }

    /// Scan all slash records for a given node.
    pub fn scan_slash_records(&self, node_id: &[u8; 32]) -> Vec<(Vec<u8>, Vec<u8>)> {
        let prefix = PersistenceKey::slash_prefix(node_id);
        self.backend.prefix_scan(&prefix)
    }

    // ─── Jail domain ─────────────────────────────────────────────────────────

    pub fn put_jail_record(&mut self, node_id: &[u8; 32], data: Vec<u8>) {
        let key = PersistenceKey::jail(node_id);
        self.backend.put(&key, data);
    }

    pub fn get_jail_record(&self, node_id: &[u8; 32]) -> Option<Vec<u8>> {
        let key = PersistenceKey::jail(node_id);
        self.backend.get(&key)
    }

    pub fn delete_jail_record(&mut self, node_id: &[u8; 32]) {
        let key = PersistenceKey::jail(node_id);
        self.backend.delete(&key);
    }

    pub fn scan_jail_records(&self) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(crate::persistence::keys::prefix::JAIL)
    }

    // ─── Rotation snapshot domain ─────────────────────────────────────────────

    pub fn put_rotation_snapshot(&mut self, epoch: u64, data: Vec<u8>) {
        let key = PersistenceKey::rotation_snapshot(epoch);
        self.backend.put(&key, data);
    }

    pub fn get_rotation_snapshot(&self, epoch: u64) -> Option<Vec<u8>> {
        let key = PersistenceKey::rotation_snapshot(epoch);
        self.backend.get(&key)
    }

    pub fn scan_rotation_snapshots(&self) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(crate::persistence::keys::prefix::ROTATION)
    }

    // ─── Fairness domain ─────────────────────────────────────────────────────

    pub fn put_fairness_record(&mut self, record_id: &[u8; 32], data: Vec<u8>) {
        let key = PersistenceKey::fairness(record_id);
        self.backend.put(&key, data);
    }

    pub fn get_fairness_record(&self, record_id: &[u8; 32]) -> Option<Vec<u8>> {
        let key = PersistenceKey::fairness(record_id);
        self.backend.get(&key)
    }

    pub fn scan_fairness_records(&self) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.backend.prefix_scan(crate::persistence::keys::prefix::FAIRNESS)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::persistence::backend::InMemoryBackend;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_engine() -> PersistenceEngine {
        PersistenceEngine::new(Box::new(InMemoryBackend::new()))
    }

    #[test]
    fn test_put_get_proposal() {
        let mut engine = make_engine();
        engine.put_proposal(&make_id(1), b"proposal_data".to_vec());
        assert_eq!(engine.get_proposal(&make_id(1)), Some(b"proposal_data".to_vec()));
    }

    #[test]
    fn test_get_missing_proposal_returns_none() {
        let engine = make_engine();
        assert!(engine.get_proposal(&make_id(99)).is_none());
    }

    #[test]
    fn test_put_get_attestation() {
        let mut engine = make_engine();
        engine.put_attestation(&make_id(2), b"attest_data".to_vec());
        assert_eq!(engine.get_attestation(&make_id(2)), Some(b"attest_data".to_vec()));
    }

    #[test]
    fn test_put_get_finalized() {
        let mut engine = make_engine();
        engine.put_finalized(&make_id(3), b"finalized_data".to_vec());
        assert_eq!(engine.get_finalized(&make_id(3)), Some(b"finalized_data".to_vec()));
    }

    #[test]
    fn test_scan_proposals() {
        let mut engine = make_engine();
        engine.put_proposal(&make_id(1), b"p1".to_vec());
        engine.put_proposal(&make_id(2), b"p2".to_vec());
        engine.put_attestation(&make_id(3), b"a1".to_vec()); // different domain
        let proposals = engine.scan_proposals();
        assert_eq!(proposals.len(), 2);
    }

    #[test]
    fn test_slash_record_put_scan() {
        let mut engine = make_engine();
        let node = make_id(5);
        engine.put_slash_record(&node, 0, b"slash0".to_vec());
        engine.put_slash_record(&node, 1, b"slash1".to_vec());
        let records = engine.scan_slash_records(&node);
        assert_eq!(records.len(), 2);
    }

    #[test]
    fn test_jail_record_put_get_delete() {
        let mut engine = make_engine();
        let node = make_id(7);
        engine.put_jail_record(&node, b"jailed".to_vec());
        assert_eq!(engine.get_jail_record(&node), Some(b"jailed".to_vec()));
        engine.delete_jail_record(&node);
        assert!(engine.get_jail_record(&node).is_none());
    }

    #[test]
    fn test_rotation_snapshot_put_get() {
        let mut engine = make_engine();
        engine.put_rotation_snapshot(5, b"snap5".to_vec());
        engine.put_rotation_snapshot(6, b"snap6".to_vec());
        assert_eq!(engine.get_rotation_snapshot(5), Some(b"snap5".to_vec()));
        let snaps = engine.scan_rotation_snapshots();
        assert_eq!(snaps.len(), 2);
    }

    #[test]
    fn test_rotation_snapshots_sorted_by_epoch() {
        let mut engine = make_engine();
        engine.put_rotation_snapshot(3, b"s3".to_vec());
        engine.put_rotation_snapshot(1, b"s1".to_vec());
        engine.put_rotation_snapshot(2, b"s2".to_vec());
        let snaps = engine.scan_rotation_snapshots();
        // Keys are epoch big-endian → sorted numerically
        let epochs: Vec<u64> = snaps
            .iter()
            .map(|(k, _)| {
                let epoch_bytes: [u8; 8] = k[k.len()-8..].try_into().unwrap_or([0u8;8]);
                u64::from_be_bytes(epoch_bytes)
            })
            .collect();
        assert_eq!(epochs, vec![1, 2, 3]);
    }

    #[test]
    fn test_fairness_record_put_scan() {
        let mut engine = make_engine();
        engine.put_fairness_record(&make_id(9), b"fair1".to_vec());
        let records = engine.scan_fairness_records();
        assert_eq!(records.len(), 1);
    }

    #[test]
    fn test_raw_put_get() {
        let mut engine = make_engine();
        engine.put_raw(b"custom_key", b"custom_value".to_vec());
        assert_eq!(engine.get_raw(b"custom_key"), Some(b"custom_value".to_vec()));
    }

    #[test]
    fn test_contains() {
        let mut engine = make_engine();
        assert!(!engine.contains(b"k"));
        engine.put_raw(b"k", b"v".to_vec());
        assert!(engine.contains(b"k"));
    }

    #[test]
    fn test_scan_jail_records() {
        let mut engine = make_engine();
        engine.put_jail_record(&make_id(1), b"j1".to_vec());
        engine.put_jail_record(&make_id(2), b"j2".to_vec());
        let jailed = engine.scan_jail_records();
        assert_eq!(jailed.len(), 2);
    }
}
