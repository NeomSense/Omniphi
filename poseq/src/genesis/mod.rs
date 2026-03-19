//! PoSeq genesis state definition and validation.
//!
//! Defines the complete initial state for a PoSeq network: validators,
//! committee, protocol version, and chain parameters.

pub mod validation;

use crate::versioning::{GenesisVersion, ProtocolVersion, PROTOCOL_VERSION};

/// A single validator entry in genesis.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GenesisValidator {
    /// 32-byte node ID, hex-encoded.
    pub node_id: String,
    /// Ed25519 public key, hex-encoded.
    pub public_key: String,
    /// Human-readable name.
    pub moniker: String,
    /// Initial stake amount (informational).
    pub initial_stake: u64,
    /// Whether this validator is active at genesis.
    pub active: bool,
}

/// PoSeq genesis state — the complete initial configuration.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PoSeqGenesisState {
    /// Version metadata.
    pub version: GenesisVersion,
    /// Initial validator set.
    pub validators: Vec<GenesisValidator>,
    /// Initial committee epoch (typically 0 or 1).
    pub initial_epoch: u64,
    /// Committee rotation period in epochs.
    pub rotation_period_epochs: u64,
    /// Minimum committee size.
    pub min_committee_size: usize,
    /// Maximum committee size.
    pub max_committee_size: usize,
    /// Attestation threshold numerator (e.g., 2 for 2/3).
    pub quorum_numerator: usize,
    /// Attestation threshold denominator (e.g., 3 for 2/3).
    pub quorum_denominator: usize,
    /// Slot duration in milliseconds.
    pub slot_duration_ms: u64,
    /// Batch size target.
    pub target_batch_size: usize,
    /// Chain ID.
    pub chain_id: String,
    /// Genesis timestamp (Unix seconds).
    pub genesis_time: u64,
}

impl PoSeqGenesisState {
    /// Create a default testnet genesis with the given validators and chain ID.
    pub fn testnet(chain_id: String, validators: Vec<GenesisValidator>) -> Self {
        PoSeqGenesisState {
            version: GenesisVersion::current(chain_id.clone()),
            validators,
            initial_epoch: 1,
            rotation_period_epochs: 100,
            min_committee_size: 3,
            max_committee_size: 100,
            quorum_numerator: 2,
            quorum_denominator: 3,
            slot_duration_ms: 2000,
            target_batch_size: 64,
            chain_id,
            genesis_time: 0,
        }
    }

    /// Deterministic genesis hash for reproducibility.
    pub fn genesis_hash(&self) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_GENESIS_V1");
        h.update(&self.chain_id.as_bytes());
        h.update(&self.initial_epoch.to_be_bytes());
        h.update(&self.genesis_time.to_be_bytes());
        for v in &self.validators {
            h.update(v.node_id.as_bytes());
            h.update(v.public_key.as_bytes());
        }
        h.update(&self.version.protocol_version.to_u64().to_be_bytes());
        h.finalize().into()
    }

    /// Number of active validators.
    pub fn active_validator_count(&self) -> usize {
        self.validators.iter().filter(|v| v.active).count()
    }
}

/// Generate a genesis state from a set of validator configs.
pub fn generate_genesis(
    chain_id: String,
    validators: Vec<GenesisValidator>,
    genesis_time: u64,
) -> PoSeqGenesisState {
    let mut genesis = PoSeqGenesisState::testnet(chain_id, validators);
    genesis.genesis_time = genesis_time;
    genesis
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_validator(i: u8) -> GenesisValidator {
        GenesisValidator {
            node_id: hex::encode([i; 32]),
            public_key: hex::encode([i + 100; 32]),
            moniker: format!("validator-{i}"),
            initial_stake: 1000,
            active: true,
        }
    }

    #[test]
    fn test_testnet_genesis() {
        let vals = vec![make_validator(1), make_validator(2), make_validator(3)];
        let genesis = PoSeqGenesisState::testnet("test-chain".into(), vals);

        assert_eq!(genesis.validators.len(), 3);
        assert_eq!(genesis.chain_id, "test-chain");
        assert_eq!(genesis.version.protocol_version, PROTOCOL_VERSION);
        assert_eq!(genesis.active_validator_count(), 3);
    }

    #[test]
    fn test_genesis_hash_deterministic() {
        let vals = vec![make_validator(1), make_validator(2)];
        let g1 = PoSeqGenesisState::testnet("test".into(), vals.clone());
        let g2 = PoSeqGenesisState::testnet("test".into(), vals);
        assert_eq!(g1.genesis_hash(), g2.genesis_hash());
    }

    #[test]
    fn test_genesis_hash_differs_with_different_chain() {
        let vals = vec![make_validator(1)];
        let g1 = PoSeqGenesisState::testnet("chain-a".into(), vals.clone());
        let g2 = PoSeqGenesisState::testnet("chain-b".into(), vals);
        assert_ne!(g1.genesis_hash(), g2.genesis_hash());
    }

    #[test]
    fn test_genesis_hash_differs_with_different_validators() {
        let g1 = PoSeqGenesisState::testnet("test".into(), vec![make_validator(1)]);
        let g2 = PoSeqGenesisState::testnet("test".into(), vec![make_validator(2)]);
        assert_ne!(g1.genesis_hash(), g2.genesis_hash());
    }

    #[test]
    fn test_generate_genesis() {
        let vals = vec![make_validator(1), make_validator(2)];
        let genesis = generate_genesis("my-chain".into(), vals, 1700000000);
        assert_eq!(genesis.genesis_time, 1700000000);
        assert_eq!(genesis.chain_id, "my-chain");
    }

    #[test]
    fn test_serialization_roundtrip() {
        let vals = vec![make_validator(1), make_validator(2)];
        let genesis = PoSeqGenesisState::testnet("test".into(), vals);

        let json = serde_json::to_string_pretty(&genesis).unwrap();
        let parsed: PoSeqGenesisState = serde_json::from_str(&json).unwrap();

        assert_eq!(parsed.chain_id, genesis.chain_id);
        assert_eq!(parsed.validators.len(), genesis.validators.len());
        assert_eq!(parsed.genesis_hash(), genesis.genesis_hash());
    }

    #[test]
    fn test_inactive_validators_not_counted() {
        let mut vals = vec![make_validator(1), make_validator(2)];
        vals[1].active = false;
        let genesis = PoSeqGenesisState::testnet("test".into(), vals);
        assert_eq!(genesis.active_validator_count(), 1);
    }
}
