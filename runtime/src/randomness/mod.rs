//! Deterministic Randomness Engine
//!
//! Provides deterministic, domain-separated entropy for protocol use cases
//! (leader election, tiebreaking, shuffling, lottery, contract randomness).
//!
//! ## What This IS
//!
//! A deterministic pseudo-random derivation engine that:
//! - Produces identical output for identical inputs across all validators
//! - Uses domain separation to prevent cross-context collisions
//! - Aggregates multiple entropy sources (epoch seed, validator commits, intent commits)
//! - Validates commitments before inclusion
//! - Provides a clean upgrade boundary for future VRF/beacon integration
//!
//! ## What This IS NOT
//!
//! This is NOT a cryptographic VRF. Limitations:
//! - The last committing validator can bias output by withholding their commitment
//! - Entropy is only as strong as the weakest input source
//! - Not suitable for high-value gambling or adversarial randomness markets
//!   without additional VRF layer
//!
//! ## Future VRF Path
//!
//! The `EntropySource` trait and `RandomnessRequest` interface are designed so
//! that a future VRF-based source (e.g., threshold BLS, DRAND beacon) can be
//! plugged in without changing the consumer API. The upgrade path:
//! 1. Add `VRFBeaconSource` implementing `EntropySource`
//! 2. Register it in `EntropyEngine` alongside existing sources
//! 3. Consumers see stronger randomness with zero API changes

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Entropy Sources
// ─────────────────────────────────────────────────────────────────────────────

/// A validated entropy contribution from a single source.
#[derive(Debug, Clone)]
pub struct EntropyContribution {
    /// The source identifier (e.g., "epoch_seed", "validator:<hex>", "intent_commits").
    pub source_id: String,
    /// The 32-byte entropy value.
    pub value: [u8; 32],
    /// Epoch this contribution was produced for.
    pub epoch: u64,
}

/// A validator's commitment to a random value.
#[derive(Debug, Clone)]
pub struct ValidatorCommitment {
    /// The validator's identifier.
    pub validator_id: [u8; 32],
    /// The commitment: SHA256(reveal || validator_id || epoch).
    pub commitment: [u8; 32],
    /// The revealed value (set after reveal phase).
    pub reveal: Option<[u8; 32]>,
    /// Epoch this commitment is for.
    pub epoch: u64,
}

impl ValidatorCommitment {
    /// Create a new commitment from a secret reveal value.
    pub fn create(validator_id: [u8; 32], reveal: [u8; 32], epoch: u64) -> Self {
        let commitment = Self::compute_commitment(&reveal, &validator_id, epoch);
        ValidatorCommitment {
            validator_id,
            commitment,
            reveal: None, // Not revealed yet
            epoch,
        }
    }

    /// Compute the expected commitment hash.
    pub fn compute_commitment(reveal: &[u8; 32], validator_id: &[u8; 32], epoch: u64) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_VALIDATOR_COMMIT_V1");
        h.update(reveal);
        h.update(validator_id);
        h.update(&epoch.to_be_bytes());
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Verify a reveal against the stored commitment.
    pub fn verify_reveal(&self, reveal: &[u8; 32]) -> bool {
        let expected = Self::compute_commitment(reveal, &self.validator_id, self.epoch);
        expected == self.commitment
    }

    /// Accept a valid reveal.
    pub fn accept_reveal(&mut self, reveal: [u8; 32]) -> Result<(), String> {
        if self.reveal.is_some() {
            return Err("already revealed".to_string());
        }
        if !self.verify_reveal(&reveal) {
            return Err("reveal does not match commitment".to_string());
        }
        self.reveal = Some(reveal);
        Ok(())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Entropy Aggregation
// ─────────────────────────────────────────────────────────────────────────────

/// Aggregated epoch entropy from all sources.
#[derive(Debug, Clone)]
pub struct EpochEntropy {
    /// The epoch this entropy is for.
    pub epoch: u64,
    /// The final aggregated entropy seed.
    pub seed: [u8; 32],
    /// Number of sources that contributed.
    pub source_count: usize,
    /// Whether this entropy includes validator commitments (stronger).
    pub has_validator_commits: bool,
}

/// The entropy engine that aggregates sources and derives randomness.
#[derive(Debug, Clone, Default)]
pub struct EntropyEngine {
    /// Epoch seed (from chain genesis or previous epoch's block hash).
    epoch_seeds: BTreeMap<u64, [u8; 32]>,
    /// Validator commitments per epoch.
    validator_commits: BTreeMap<u64, Vec<ValidatorCommitment>>,
    /// Intent commitment hashes per epoch (from encrypted intents).
    intent_commits: BTreeMap<u64, Vec<[u8; 32]>>,
    /// Cached aggregated entropy per epoch.
    cached_entropy: BTreeMap<u64, EpochEntropy>,
}

impl EntropyEngine {
    pub fn new() -> Self { EntropyEngine::default() }

    /// Set the epoch seed (typically from the previous epoch's finalized block hash).
    pub fn set_epoch_seed(&mut self, epoch: u64, seed: [u8; 32]) {
        self.epoch_seeds.insert(epoch, seed);
        // Invalidate cache for this epoch
        self.cached_entropy.remove(&epoch);
    }

    /// Add a validator commitment for an epoch.
    pub fn add_validator_commitment(&mut self, commit: ValidatorCommitment) -> Result<(), String> {
        if commit.validator_id == [0u8; 32] {
            return Err("validator_id must be non-zero".to_string());
        }
        if commit.commitment == [0u8; 32] {
            return Err("commitment must be non-zero".to_string());
        }

        let epoch = commit.epoch;
        let commits = self.validator_commits.entry(epoch).or_default();

        // Reject duplicate from same validator
        if commits.iter().any(|c| c.validator_id == commit.validator_id) {
            return Err("duplicate commitment from same validator".to_string());
        }

        commits.push(commit);
        self.cached_entropy.remove(&epoch);
        Ok(())
    }

    /// Reveal a validator's committed value.
    pub fn reveal_validator_commitment(
        &mut self,
        epoch: u64,
        validator_id: &[u8; 32],
        reveal: [u8; 32],
    ) -> Result<(), String> {
        let commits = self.validator_commits.get_mut(&epoch)
            .ok_or_else(|| "no commitments for this epoch".to_string())?;

        let commit = commits.iter_mut()
            .find(|c| &c.validator_id == validator_id)
            .ok_or_else(|| "validator commitment not found".to_string())?;

        commit.accept_reveal(reveal)?;
        self.cached_entropy.remove(&epoch);
        Ok(())
    }

    /// Add intent commitment hashes for an epoch (from encrypted intents).
    pub fn add_intent_commits(&mut self, epoch: u64, commits: &[[u8; 32]]) {
        let entry = self.intent_commits.entry(epoch).or_default();
        entry.extend_from_slice(commits);
        self.cached_entropy.remove(&epoch);
    }

    /// Aggregate all sources into a single epoch entropy seed.
    ///
    /// Aggregation: SHA256("OMNIPHI_ENTROPY_AGG_V1" || epoch
    ///              || epoch_seed || sorted_validator_reveals || sorted_intent_commits)
    ///
    /// Only revealed validator commitments contribute (unrevealed are excluded).
    /// Sources are sorted for determinism.
    pub fn aggregate(&mut self, epoch: u64) -> EpochEntropy {
        // Return cached if available
        if let Some(cached) = self.cached_entropy.get(&epoch) {
            return cached.clone();
        }

        let mut h = Sha256::new();
        h.update(b"OMNIPHI_ENTROPY_AGG_V1");
        h.update(&epoch.to_be_bytes());

        let mut source_count = 0;

        // Source 1: Epoch seed
        if let Some(seed) = self.epoch_seeds.get(&epoch) {
            h.update(seed);
            source_count += 1;
        }

        // Source 2: Validator reveals (sorted by validator_id for determinism)
        let has_validator_commits;
        if let Some(commits) = self.validator_commits.get(&epoch) {
            let mut reveals: Vec<([u8; 32], [u8; 32])> = commits.iter()
                .filter_map(|c| c.reveal.map(|r| (c.validator_id, r)))
                .collect();
            reveals.sort_by(|a, b| a.0.cmp(&b.0)); // sort by validator_id

            has_validator_commits = !reveals.is_empty();
            for (vid, reveal) in &reveals {
                h.update(vid);
                h.update(reveal);
                source_count += 1;
            }
        } else {
            has_validator_commits = false;
        }

        // Source 3: Intent commitments (sorted for determinism)
        if let Some(commits) = self.intent_commits.get(&epoch) {
            let mut sorted = commits.clone();
            sorted.sort();
            for c in &sorted {
                h.update(c);
            }
            if !sorted.is_empty() {
                source_count += 1;
            }
        }

        let r = h.finalize();
        let mut seed = [0u8; 32];
        seed.copy_from_slice(&r);

        let entropy = EpochEntropy {
            epoch,
            seed,
            source_count,
            has_validator_commits,
        };

        self.cached_entropy.insert(epoch, entropy.clone());
        entropy
    }

    /// Prune data for epochs older than `before_epoch`.
    pub fn prune(&mut self, before_epoch: u64) {
        self.epoch_seeds.retain(|&e, _| e >= before_epoch);
        self.validator_commits.retain(|&e, _| e >= before_epoch);
        self.intent_commits.retain(|&e, _| e >= before_epoch);
        self.cached_entropy.retain(|&e, _| e >= before_epoch);
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Randomness Request Interface
// ─────────────────────────────────────────────────────────────────────────────

/// A request for deterministic randomness in a specific domain.
#[derive(Debug, Clone)]
pub struct RandomnessRequest {
    /// Domain tag for separation (e.g., "leader_election", "contract:0xAB..CD", "shuffle").
    pub domain: String,
    /// The epoch to derive randomness for.
    pub epoch: u64,
    /// Optional sub-index for multiple draws in the same domain+epoch.
    pub index: u64,
    /// Optional target object or contract identifier for scoped randomness.
    pub target_id: Option<[u8; 32]>,
}

/// Derive deterministic, domain-separated randomness from an epoch entropy seed.
///
/// output = SHA256("OMNIPHI_RAND_V1" || epoch_seed || domain || index || target_id?)
///
/// This is the consumer-facing API. The derivation is:
/// - Deterministic: same inputs always produce same output
/// - Domain-separated: different domains never collide
/// - Index-separated: multiple draws in same domain are independent
/// - Upgradeable: replacing the epoch_seed source with VRF doesn't change this function
pub fn derive_randomness(epoch_entropy: &EpochEntropy, request: &RandomnessRequest) -> [u8; 32] {
    let mut h = Sha256::new();
    h.update(b"OMNIPHI_RAND_V1");
    h.update(&epoch_entropy.seed);
    h.update(request.domain.as_bytes());
    h.update(&request.epoch.to_be_bytes());
    h.update(&request.index.to_be_bytes());
    if let Some(ref target) = request.target_id {
        h.update(target);
    }
    let r = h.finalize();
    let mut out = [0u8; 32];
    out.copy_from_slice(&r);
    out
}

/// Convenience: derive a u64 from randomness (for selection, shuffling, etc.).
pub fn derive_u64(epoch_entropy: &EpochEntropy, request: &RandomnessRequest) -> u64 {
    let bytes = derive_randomness(epoch_entropy, request);
    u64::from_be_bytes([bytes[0], bytes[1], bytes[2], bytes[3],
                         bytes[4], bytes[5], bytes[6], bytes[7]])
}

/// Convenience: derive a value in [0, max) from randomness.
pub fn derive_bounded(epoch_entropy: &EpochEntropy, request: &RandomnessRequest, max: u64) -> u64 {
    if max == 0 { return 0; }
    derive_u64(epoch_entropy, request) % max
}

// ─────────────────────────────────────────────────────────────────────────────
// Upgrade Boundary Trait
// ─────────────────────────────────────────────────────────────────────────────

/// Trait for pluggable entropy sources.
///
/// The current engine uses SHA256-based aggregation. A future VRF source
/// would implement this trait and be registered in the engine.
pub trait EntropySource: std::fmt::Debug {
    /// Return the entropy contribution for a given epoch.
    /// Returns None if no contribution is available for this epoch.
    fn contribute(&self, epoch: u64) -> Option<EntropyContribution>;

    /// Human-readable name of this source (for logging/audit).
    fn name(&self) -> &str;

    /// Strength level: "deterministic", "commit-reveal", "vrf", "beacon".
    fn strength(&self) -> &str;
}

/// A simple epoch seed source (current default).
#[derive(Debug)]
pub struct EpochSeedSource {
    seeds: BTreeMap<u64, [u8; 32]>,
}

impl EpochSeedSource {
    pub fn new() -> Self { EpochSeedSource { seeds: BTreeMap::new() } }
    pub fn set(&mut self, epoch: u64, seed: [u8; 32]) { self.seeds.insert(epoch, seed); }
}

impl EntropySource for EpochSeedSource {
    fn contribute(&self, epoch: u64) -> Option<EntropyContribution> {
        self.seeds.get(&epoch).map(|s| EntropyContribution {
            source_id: "epoch_seed".to_string(),
            value: *s,
            epoch,
        })
    }
    fn name(&self) -> &str { "EpochSeed" }
    fn strength(&self) -> &str { "deterministic" }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn validator(id: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = id; b }
    fn seed(v: u8) -> [u8; 32] { [v; 32] }

    // ── Test 1: deterministic output stability ───────────────

    #[test]
    fn test_deterministic_output_stability() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));

        let entropy1 = engine.aggregate(10);
        let entropy2 = engine.aggregate(10);
        assert_eq!(entropy1.seed, entropy2.seed, "Same inputs must produce same output");

        let req = RandomnessRequest {
            domain: "test".to_string(),
            epoch: 10,
            index: 0,
            target_id: None,
        };
        let r1 = derive_randomness(&entropy1, &req);
        let r2 = derive_randomness(&entropy2, &req);
        assert_eq!(r1, r2, "Derived randomness must be deterministic");
    }

    // ── Test 2: different domains produce different outputs ──

    #[test]
    fn test_different_domains_different_output() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));
        let entropy = engine.aggregate(10);

        let req_a = RandomnessRequest {
            domain: "leader_election".to_string(),
            epoch: 10, index: 0, target_id: None,
        };
        let req_b = RandomnessRequest {
            domain: "contract_shuffle".to_string(),
            epoch: 10, index: 0, target_id: None,
        };

        let r_a = derive_randomness(&entropy, &req_a);
        let r_b = derive_randomness(&entropy, &req_b);
        assert_ne!(r_a, r_b, "Different domains must produce different output");
    }

    // ── Test 3: same domain same inputs same output ──────────

    #[test]
    fn test_same_domain_same_inputs_same_output() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xBB));
        let entropy = engine.aggregate(10);

        let req = RandomnessRequest {
            domain: "lottery".to_string(),
            epoch: 10, index: 5, target_id: Some([0xCC; 32]),
        };

        let r1 = derive_randomness(&entropy, &req);
        let r2 = derive_randomness(&entropy, &req);
        assert_eq!(r1, r2);
    }

    // ── Test 4: malformed commitment rejection ───────────────

    #[test]
    fn test_zero_validator_id_rejected() {
        let mut engine = EntropyEngine::new();
        let commit = ValidatorCommitment {
            validator_id: [0u8; 32],
            commitment: [0xAA; 32],
            reveal: None,
            epoch: 10,
        };
        let result = engine.add_validator_commitment(commit);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("non-zero"));
    }

    #[test]
    fn test_zero_commitment_rejected() {
        let mut engine = EntropyEngine::new();
        let commit = ValidatorCommitment {
            validator_id: validator(1),
            commitment: [0u8; 32],
            reveal: None,
            epoch: 10,
        };
        let result = engine.add_validator_commitment(commit);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("non-zero"));
    }

    #[test]
    fn test_duplicate_validator_commitment_rejected() {
        let mut engine = EntropyEngine::new();
        let reveal = [0xDD; 32];
        let commit = ValidatorCommitment::create(validator(1), reveal, 10);
        engine.add_validator_commitment(commit.clone()).unwrap();

        let result = engine.add_validator_commitment(commit);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("duplicate"));
    }

    // ── Test 5: invalid reveal rejected ──────────────────────

    #[test]
    fn test_invalid_reveal_rejected() {
        let mut engine = EntropyEngine::new();
        let reveal = [0xDD; 32];
        let commit = ValidatorCommitment::create(validator(1), reveal, 10);
        engine.add_validator_commitment(commit).unwrap();

        let wrong_reveal = [0xEE; 32];
        let result = engine.reveal_validator_commitment(10, &validator(1), wrong_reveal);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("does not match"));
    }

    // ── Test 6: valid reveal accepted + changes entropy ──────

    #[test]
    fn test_valid_reveal_changes_entropy() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));

        let reveal = [0xDD; 32];
        let commit = ValidatorCommitment::create(validator(1), reveal, 10);
        engine.add_validator_commitment(commit).unwrap();

        // Entropy before reveal
        let entropy_before = engine.aggregate(10);

        // Reveal
        engine.reveal_validator_commitment(10, &validator(1), reveal).unwrap();

        // Entropy after reveal (should differ because revealed value is now included)
        let entropy_after = engine.aggregate(10);
        assert_ne!(entropy_before.seed, entropy_after.seed);
        assert!(entropy_after.has_validator_commits);
    }

    // ── Test 7: different indices produce different output ───

    #[test]
    fn test_different_indices() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));
        let entropy = engine.aggregate(10);

        let req0 = RandomnessRequest {
            domain: "shuffle".to_string(),
            epoch: 10, index: 0, target_id: None,
        };
        let req1 = RandomnessRequest {
            domain: "shuffle".to_string(),
            epoch: 10, index: 1, target_id: None,
        };

        assert_ne!(derive_randomness(&entropy, &req0), derive_randomness(&entropy, &req1));
    }

    // ── Test 8: different target_id produces different output ─

    #[test]
    fn test_different_target_id() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));
        let entropy = engine.aggregate(10);

        let req_a = RandomnessRequest {
            domain: "contract".to_string(),
            epoch: 10, index: 0, target_id: Some([0x11; 32]),
        };
        let req_b = RandomnessRequest {
            domain: "contract".to_string(),
            epoch: 10, index: 0, target_id: Some([0x22; 32]),
        };

        assert_ne!(derive_randomness(&entropy, &req_a), derive_randomness(&entropy, &req_b));
    }

    // ── Test 9: derive_bounded produces values in range ──────

    #[test]
    fn test_derive_bounded() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));
        let entropy = engine.aggregate(10);

        for i in 0..100 {
            let req = RandomnessRequest {
                domain: "bounded_test".to_string(),
                epoch: 10, index: i, target_id: None,
            };
            let val = derive_bounded(&entropy, &req, 10);
            assert!(val < 10, "derive_bounded(10) produced {} which is >= 10", val);
        }
    }

    // ── Test 10: intent commits contribute to entropy ────────

    #[test]
    fn test_intent_commits_contribute() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));

        let entropy_before = engine.aggregate(10);

        engine.add_intent_commits(10, &[[0x11; 32], [0x22; 32]]);
        let entropy_after = engine.aggregate(10);

        assert_ne!(entropy_before.seed, entropy_after.seed);
    }

    // ── Test 11: multiple validators aggregate correctly ─────

    #[test]
    fn test_multiple_validators() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));

        let reveal1 = [0x11; 32];
        let reveal2 = [0x22; 32];
        let commit1 = ValidatorCommitment::create(validator(1), reveal1, 10);
        let commit2 = ValidatorCommitment::create(validator(2), reveal2, 10);

        engine.add_validator_commitment(commit1).unwrap();
        engine.add_validator_commitment(commit2).unwrap();
        engine.reveal_validator_commitment(10, &validator(1), reveal1).unwrap();
        engine.reveal_validator_commitment(10, &validator(2), reveal2).unwrap();

        let entropy = engine.aggregate(10);
        assert_eq!(entropy.source_count, 3); // 1 epoch_seed + 2 validator reveals
        assert!(entropy.has_validator_commits);
    }

    // ── Test 12: unrevealed validator doesn't contribute ─────

    #[test]
    fn test_unrevealed_excluded() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));

        let reveal1 = [0x11; 32];
        let commit1 = ValidatorCommitment::create(validator(1), reveal1, 10);
        let commit2 = ValidatorCommitment::create(validator(2), [0x22; 32], 10);

        engine.add_validator_commitment(commit1).unwrap();
        engine.add_validator_commitment(commit2).unwrap();

        // Only reveal validator 1
        engine.reveal_validator_commitment(10, &validator(1), reveal1).unwrap();

        let entropy = engine.aggregate(10);
        assert_eq!(entropy.source_count, 2); // 1 epoch_seed + 1 reveal (not 2)
    }

    // ── Test 13: prune removes old epochs ────────────────────

    #[test]
    fn test_prune() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(5, seed(0x05));
        engine.set_epoch_seed(10, seed(0x0A));
        engine.set_epoch_seed(15, seed(0x0F));

        engine.prune(10);

        // Epoch 5 should be gone, 10 and 15 remain
        let entropy5 = engine.aggregate(5);
        let entropy10 = engine.aggregate(10);
        assert_eq!(entropy5.source_count, 0); // no seed for epoch 5
        assert_eq!(entropy10.source_count, 1); // seed exists
    }

    // ── Test 14: EntropySource trait (upgrade boundary) ──────

    #[test]
    fn test_entropy_source_trait() {
        let mut source = EpochSeedSource::new();
        source.set(10, seed(0xAA));

        assert_eq!(source.name(), "EpochSeed");
        assert_eq!(source.strength(), "deterministic");

        let contribution = source.contribute(10);
        assert!(contribution.is_some());
        assert_eq!(contribution.unwrap().value, seed(0xAA));

        // Different epoch → None
        assert!(source.contribute(11).is_none());
    }

    // ── Test 15: commitment create + verify roundtrip ────────

    #[test]
    fn test_commitment_roundtrip() {
        let reveal = [0xDD; 32];
        let vid = validator(5);
        let commit = ValidatorCommitment::create(vid, reveal, 42);

        assert!(commit.verify_reveal(&reveal));
        assert!(!commit.verify_reveal(&[0xEE; 32])); // wrong reveal
        assert!(commit.reveal.is_none()); // not yet accepted

        let mut commit = commit;
        commit.accept_reveal(reveal).unwrap();
        assert_eq!(commit.reveal, Some(reveal));

        // Double reveal rejected
        assert!(commit.accept_reveal(reveal).is_err());
    }

    // ── Test 16: derive_bounded with max=0 returns 0 ─────────

    #[test]
    fn test_derive_bounded_zero() {
        let mut engine = EntropyEngine::new();
        engine.set_epoch_seed(10, seed(0xAA));
        let entropy = engine.aggregate(10);

        let req = RandomnessRequest {
            domain: "test".to_string(),
            epoch: 10, index: 0, target_id: None,
        };
        assert_eq!(derive_bounded(&entropy, &req, 0), 0);
    }
}
