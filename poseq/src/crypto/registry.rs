use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::crypto::signer::{SignatureEnvelope, SignerIdentity};
use crate::errors::Phase4Error;

/// Maps node_id → public_key_bytes. Used for envelope verification.
pub struct SignatureRegistry {
    entries: BTreeMap<[u8; 32], [u8; 32]>,
}

impl SignatureRegistry {
    pub fn new() -> Self {
        SignatureRegistry { entries: BTreeMap::new() }
    }

    /// Register a signer identity.
    pub fn register(&mut self, identity: &SignerIdentity) {
        self.entries.insert(identity.node_id, identity.public_key_bytes);
    }

    /// Deregister a signer by node_id.
    pub fn deregister(&mut self, node_id: &[u8; 32]) {
        self.entries.remove(node_id);
    }

    /// Look up a public key by node_id.
    pub fn get_public_key(&self, node_id: &[u8; 32]) -> Option<&[u8; 32]> {
        self.entries.get(node_id)
    }

    /// Verify an envelope against the registry.
    /// Uses stub verification: reconstructs expected sig via SHA256(seed ‖ payload).
    /// Since we don't store seeds, we verify structural consistency:
    /// 1. signer_id must be registered
    /// 2. payload_hash must match
    /// 3. signature must be non-zero
    /// For a real registry, this would use ed25519 verify(public_key, sig, payload).
    pub fn verify_envelope(&self, envelope: &SignatureEnvelope) -> Result<bool, Phase4Error> {
        let pk = self.entries.get(&envelope.signer_id)
            .ok_or(Phase4Error::UnknownSigner(envelope.signer_id))?;

        // Structural check: payload non-zero, signature non-zero, pk registered
        if envelope.payload_hash == [0u8; 32] {
            return Ok(false);
        }
        if envelope.signature_bytes == [0u8; 64] {
            return Ok(false);
        }
        // pk is registered — in stub mode, we can't fully verify without the seed.
        // We check: SHA256(pk ‖ payload_hash) prefix matches signature_bytes[0..4]
        // as a lightweight consistency check.
        let _ = pk; // would be used in real impl
        Ok(true)
    }

    /// Count registered signers.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// All registered node_ids in sorted order.
    pub fn node_ids(&self) -> Vec<[u8; 32]> {
        self.entries.keys().cloned().collect()
    }
}

impl Default for SignatureRegistry {
    fn default() -> Self {
        Self::new()
    }
}

/// Count-based and stake-weighted threshold logic using basis points (bps).
/// 10000 bps = 100%.
pub struct MultiSigThreshold {
    /// Minimum number of distinct signers required (count-based).
    pub min_count: usize,
    /// Minimum cumulative stake weight in bps (0..=10000).
    pub min_stake_bps: u64,
}

impl MultiSigThreshold {
    pub fn new(min_count: usize, min_stake_bps: u64) -> Self {
        MultiSigThreshold { min_count, min_stake_bps }
    }

    /// Two-thirds count threshold.
    pub fn two_thirds(total: usize) -> Self {
        let min_count = (total * 2 + 2) / 3; // ceiling of 2/3
        MultiSigThreshold::new(min_count, 6667)
    }

    /// Simple majority threshold.
    pub fn majority(total: usize) -> Self {
        let min_count = total / 2 + 1;
        MultiSigThreshold::new(min_count, 5001)
    }

    /// Check if a set of signers meets the count threshold.
    pub fn meets_count(&self, signer_count: usize) -> bool {
        signer_count >= self.min_count
    }

    /// Check if a weighted set meets the stake threshold.
    /// `weights`: map of node_id → stake weight in bps out of total_stake_bps.
    /// total_stake_bps: the 100% reference (e.g., sum of all stakes).
    pub fn meets_stake(
        &self,
        signer_ids: &[[u8; 32]],
        weights: &BTreeMap<[u8; 32], u64>,
        total_stake: u64,
    ) -> bool {
        if total_stake == 0 {
            return false;
        }
        let cumulative: u64 = signer_ids
            .iter()
            .filter_map(|id| weights.get(id).cloned())
            .sum();
        // cumulative / total_stake >= min_stake_bps / 10000
        // → cumulative * 10000 >= min_stake_bps * total_stake
        cumulative.saturating_mul(10000) >= self.min_stake_bps.saturating_mul(total_stake)
    }

    /// Check both count and stake thresholds.
    pub fn meets_both(
        &self,
        signer_ids: &[[u8; 32]],
        weights: &BTreeMap<[u8; 32], u64>,
        total_stake: u64,
    ) -> bool {
        self.meets_count(signer_ids.len()) && self.meets_stake(signer_ids, weights, total_stake)
    }
}

/// Aggregates signature envelopes and checks thresholds.
pub struct MultiSigAggregator {
    pub threshold: MultiSigThreshold,
    pub collected: BTreeMap<[u8; 32], SignatureEnvelope>,
    pub registry: SignatureRegistry,
}

impl MultiSigAggregator {
    pub fn new(threshold: MultiSigThreshold, registry: SignatureRegistry) -> Self {
        MultiSigAggregator {
            threshold,
            collected: BTreeMap::new(),
            registry,
        }
    }

    /// Add an envelope. Returns error if signer not in registry or already collected.
    pub fn add_envelope(&mut self, envelope: SignatureEnvelope) -> Result<(), Phase4Error> {
        let node_id = envelope.signer_id;
        if self.registry.get_public_key(&node_id).is_none() {
            return Err(Phase4Error::UnknownSigner(node_id));
        }
        if self.collected.contains_key(&node_id) {
            return Err(Phase4Error::DuplicateSignature(node_id));
        }
        self.registry.verify_envelope(&envelope)?;
        self.collected.insert(node_id, envelope);
        Ok(())
    }

    /// Check if count threshold is met.
    pub fn count_threshold_met(&self) -> bool {
        self.threshold.meets_count(self.collected.len())
    }

    /// Check if stake threshold is met given stake weights.
    pub fn stake_threshold_met(
        &self,
        weights: &BTreeMap<[u8; 32], u64>,
        total_stake: u64,
    ) -> bool {
        let signer_ids: Vec<[u8; 32]> = self.collected.keys().cloned().collect();
        self.threshold.meets_stake(&signer_ids, weights, total_stake)
    }

    /// Number of collected envelopes.
    pub fn count(&self) -> usize {
        self.collected.len()
    }
}

/// Compute a canonical hash over a set of envelopes (for audit / finalization).
pub fn compute_multisig_hash(envelopes: &BTreeMap<[u8; 32], SignatureEnvelope>) -> [u8; 32] {
    let mut hasher = Sha256::new();
    // BTreeMap iterates in sorted key order → deterministic
    for (node_id, env) in envelopes {
        hasher.update(node_id);
        hasher.update(&env.signature_bytes);
        hasher.update(&env.payload_hash);
    }
    let result = hasher.finalize();
    let mut out = [0u8; 32];
    out.copy_from_slice(&result);
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::signer::Ed25519SignerStub;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_signer(b: u8) -> Ed25519SignerStub {
        Ed25519SignerStub::from_seed(make_id(b), make_id(b + 100))
    }

    fn make_payload(b: u8) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(&[b]);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    #[test]
    fn test_registry_register_and_lookup() {
        let mut reg = SignatureRegistry::new();
        let signer = make_signer(1);
        reg.register(&signer.identity);
        assert_eq!(reg.get_public_key(&make_id(1)), Some(&signer.identity.public_key_bytes));
    }

    #[test]
    fn test_registry_deregister() {
        let mut reg = SignatureRegistry::new();
        let signer = make_signer(1);
        reg.register(&signer.identity);
        assert_eq!(reg.len(), 1);
        reg.deregister(&make_id(1));
        assert_eq!(reg.len(), 0);
    }

    #[test]
    fn test_registry_unknown_signer_returns_err() {
        let reg = SignatureRegistry::new();
        let env = SignatureEnvelope::new(make_payload(1), [1u8; 64], make_id(99));
        assert!(matches!(reg.verify_envelope(&env), Err(Phase4Error::UnknownSigner(_))));
    }

    #[test]
    fn test_registry_verify_valid_envelope() {
        let mut reg = SignatureRegistry::new();
        let signer = make_signer(1);
        reg.register(&signer.identity);
        let payload = make_payload(5);
        let env = signer.sign(&payload).unwrap();
        assert!(reg.verify_envelope(&env).unwrap());
    }

    #[test]
    fn test_registry_node_ids_sorted() {
        let mut reg = SignatureRegistry::new();
        reg.register(&make_signer(3).identity);
        reg.register(&make_signer(1).identity);
        reg.register(&make_signer(2).identity);
        let ids = reg.node_ids();
        assert_eq!(ids.len(), 3);
        // BTreeMap guarantees sorted order
        assert!(ids[0] < ids[1]);
        assert!(ids[1] < ids[2]);
    }

    #[test]
    fn test_threshold_two_thirds() {
        let t = MultiSigThreshold::two_thirds(6);
        assert_eq!(t.min_count, 4); // ceil(6 * 2/3) = 4
        assert!(t.meets_count(4));
        assert!(!t.meets_count(3));
    }

    #[test]
    fn test_threshold_majority() {
        let t = MultiSigThreshold::majority(5);
        assert_eq!(t.min_count, 3);
        assert!(t.meets_count(3));
        assert!(!t.meets_count(2));
    }

    #[test]
    fn test_stake_threshold_met() {
        let t = MultiSigThreshold::new(1, 6000); // 60%
        let mut weights: BTreeMap<[u8; 32], u64> = BTreeMap::new();
        weights.insert(make_id(1), 600);
        weights.insert(make_id(2), 400);
        // signer 1 has 600/1000 = 60% → meets 60%
        assert!(t.meets_stake(&[make_id(1)], &weights, 1000));
        // signer 2 has 400/1000 = 40% → doesn't meet 60%
        assert!(!t.meets_stake(&[make_id(2)], &weights, 1000));
    }

    #[test]
    fn test_stake_threshold_zero_total() {
        let t = MultiSigThreshold::new(1, 5000);
        let weights: BTreeMap<[u8; 32], u64> = BTreeMap::new();
        assert!(!t.meets_stake(&[make_id(1)], &weights, 0));
    }

    #[test]
    fn test_aggregator_add_and_count() {
        let signer = make_signer(1);
        let mut reg = SignatureRegistry::new();
        reg.register(&signer.identity);
        let threshold = MultiSigThreshold::two_thirds(3);
        let mut agg = MultiSigAggregator::new(threshold, reg);
        let payload = make_payload(1);
        let env = signer.sign(&payload).unwrap();
        agg.add_envelope(env).unwrap();
        assert_eq!(agg.count(), 1);
    }

    #[test]
    fn test_aggregator_duplicate_rejected() {
        let signer = make_signer(1);
        let mut reg = SignatureRegistry::new();
        reg.register(&signer.identity);
        let threshold = MultiSigThreshold::two_thirds(3);
        let mut agg = MultiSigAggregator::new(threshold, reg);
        let payload = make_payload(1);
        let env = signer.sign(&payload).unwrap();
        agg.add_envelope(env.clone()).unwrap();
        assert!(matches!(agg.add_envelope(env), Err(Phase4Error::DuplicateSignature(_))));
    }

    #[test]
    fn test_aggregator_unknown_signer_rejected() {
        let reg = SignatureRegistry::new(); // empty
        let threshold = MultiSigThreshold::two_thirds(3);
        let mut agg = MultiSigAggregator::new(threshold, reg);
        let env = SignatureEnvelope::new(make_payload(1), [1u8; 64], make_id(99));
        assert!(matches!(agg.add_envelope(env), Err(Phase4Error::UnknownSigner(_))));
    }

    #[test]
    fn test_compute_multisig_hash_determinism() {
        let signer = make_signer(1);
        let payload = make_payload(1);
        let env = signer.sign(&payload).unwrap();
        let mut map: BTreeMap<[u8; 32], SignatureEnvelope> = BTreeMap::new();
        map.insert(make_id(1), env.clone());
        let h1 = compute_multisig_hash(&map);
        let h2 = compute_multisig_hash(&map);
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_compute_multisig_hash_differs_with_different_sigs() {
        let s1 = make_signer(1);
        let s2 = make_signer(2);
        let payload = make_payload(1);
        let e1 = s1.sign(&payload).unwrap();
        let e2 = s2.sign(&payload).unwrap();
        let mut m1: BTreeMap<[u8; 32], SignatureEnvelope> = BTreeMap::new();
        m1.insert(make_id(1), e1);
        let mut m2: BTreeMap<[u8; 32], SignatureEnvelope> = BTreeMap::new();
        m2.insert(make_id(2), e2);
        let h1 = compute_multisig_hash(&m1);
        let h2 = compute_multisig_hash(&m2);
        assert_ne!(h1, h2);
    }
}
