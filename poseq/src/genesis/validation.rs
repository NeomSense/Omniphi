//! Genesis state validation.
//!
//! Ensures genesis is well-formed before node startup. Rejects invalid,
//! inconsistent, or incompatible genesis states.

use crate::genesis::PoSeqGenesisState;
use crate::versioning::compat::{check_genesis_compat, CompatResult};

/// Validation errors for genesis state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum GenesisValidationError {
    /// Protocol version is incompatible with this binary.
    IncompatibleVersion(String),
    /// No active validators in genesis.
    NoActiveValidators,
    /// Active validators below minimum committee size.
    TooFewValidators { active: usize, min: usize },
    /// Duplicate node IDs in validator set.
    DuplicateNodeId(String),
    /// Duplicate public keys in validator set.
    DuplicatePublicKey(String),
    /// Invalid node ID (not 64 hex chars).
    InvalidNodeId(String),
    /// Invalid public key (not 64 hex chars).
    InvalidPublicKey(String),
    /// Empty chain ID.
    EmptyChainId,
    /// Invalid quorum threshold.
    InvalidQuorum { numerator: usize, denominator: usize },
    /// Max committee size smaller than min.
    InvalidCommitteeRange { min: usize, max: usize },
    /// Slot duration is zero.
    ZeroSlotDuration,
}

impl std::fmt::Display for GenesisValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::IncompatibleVersion(msg) => write!(f, "incompatible genesis version: {msg}"),
            Self::NoActiveValidators => write!(f, "no active validators in genesis"),
            Self::TooFewValidators { active, min } => {
                write!(f, "only {active} active validators, need at least {min}")
            }
            Self::DuplicateNodeId(id) => write!(f, "duplicate node_id: {id}"),
            Self::DuplicatePublicKey(pk) => write!(f, "duplicate public_key: {pk}"),
            Self::InvalidNodeId(id) => write!(f, "invalid node_id (must be 64 hex): {id}"),
            Self::InvalidPublicKey(pk) => write!(f, "invalid public_key (must be 64 hex): {pk}"),
            Self::EmptyChainId => write!(f, "chain_id cannot be empty"),
            Self::InvalidQuorum { numerator, denominator } => {
                write!(f, "invalid quorum {numerator}/{denominator}")
            }
            Self::InvalidCommitteeRange { min, max } => {
                write!(f, "invalid committee range: min={min} > max={max}")
            }
            Self::ZeroSlotDuration => write!(f, "slot_duration_ms cannot be zero"),
        }
    }
}

/// Validate a genesis state. Returns all errors found.
pub fn validate_genesis(genesis: &PoSeqGenesisState) -> Vec<GenesisValidationError> {
    let mut errors = Vec::new();

    // Version compatibility
    match check_genesis_compat(
        &genesis.version.protocol_version,
        genesis.version.genesis_format,
    ) {
        CompatResult::Incompatible(msg) => {
            errors.push(GenesisValidationError::IncompatibleVersion(msg));
        }
        _ => {}
    }

    // Chain ID
    if genesis.chain_id.is_empty() {
        errors.push(GenesisValidationError::EmptyChainId);
    }

    // Validator set
    let mut seen_ids = std::collections::BTreeSet::new();
    let mut seen_pks = std::collections::BTreeSet::new();

    for v in &genesis.validators {
        // Validate hex format
        if v.node_id.len() != 64 || hex::decode(&v.node_id).is_err() {
            errors.push(GenesisValidationError::InvalidNodeId(v.node_id.clone()));
        }
        if v.public_key.len() != 64 || hex::decode(&v.public_key).is_err() {
            errors.push(GenesisValidationError::InvalidPublicKey(v.public_key.clone()));
        }

        // Duplicates
        if !seen_ids.insert(v.node_id.clone()) {
            errors.push(GenesisValidationError::DuplicateNodeId(v.node_id.clone()));
        }
        if !seen_pks.insert(v.public_key.clone()) {
            errors.push(GenesisValidationError::DuplicatePublicKey(v.public_key.clone()));
        }
    }

    let active = genesis.active_validator_count();
    if active == 0 {
        errors.push(GenesisValidationError::NoActiveValidators);
    } else if active < genesis.min_committee_size {
        errors.push(GenesisValidationError::TooFewValidators {
            active,
            min: genesis.min_committee_size,
        });
    }

    // Committee bounds
    if genesis.min_committee_size > genesis.max_committee_size {
        errors.push(GenesisValidationError::InvalidCommitteeRange {
            min: genesis.min_committee_size,
            max: genesis.max_committee_size,
        });
    }

    // Quorum
    if genesis.quorum_denominator == 0
        || genesis.quorum_numerator == 0
        || genesis.quorum_numerator > genesis.quorum_denominator
    {
        errors.push(GenesisValidationError::InvalidQuorum {
            numerator: genesis.quorum_numerator,
            denominator: genesis.quorum_denominator,
        });
    }

    // Slot duration
    if genesis.slot_duration_ms == 0 {
        errors.push(GenesisValidationError::ZeroSlotDuration);
    }

    errors
}

/// Validate and return Ok(()) or Err with first error.
pub fn validate_genesis_strict(genesis: &PoSeqGenesisState) -> Result<(), GenesisValidationError> {
    let errors = validate_genesis(genesis);
    if errors.is_empty() {
        Ok(())
    } else {
        Err(errors.into_iter().next().unwrap())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::genesis::{GenesisValidator, PoSeqGenesisState};

    fn make_validator(i: u8) -> GenesisValidator {
        GenesisValidator {
            node_id: hex::encode([i; 32]),
            public_key: hex::encode([i + 100; 32]),
            moniker: format!("val-{i}"),
            initial_stake: 1000,
            active: true,
        }
    }

    fn valid_genesis() -> PoSeqGenesisState {
        PoSeqGenesisState::testnet(
            "test-chain".into(),
            vec![make_validator(1), make_validator(2), make_validator(3)],
        )
    }

    #[test]
    fn test_valid_genesis_passes() {
        let errors = validate_genesis(&valid_genesis());
        assert!(errors.is_empty(), "expected no errors, got: {:?}", errors);
    }

    #[test]
    fn test_empty_chain_id() {
        let mut g = valid_genesis();
        g.chain_id = "".into();
        g.version.chain_id = "".into();
        let errors = validate_genesis(&g);
        assert!(errors.contains(&GenesisValidationError::EmptyChainId));
    }

    #[test]
    fn test_no_active_validators() {
        let mut g = valid_genesis();
        for v in &mut g.validators {
            v.active = false;
        }
        let errors = validate_genesis(&g);
        assert!(errors.contains(&GenesisValidationError::NoActiveValidators));
    }

    #[test]
    fn test_too_few_validators() {
        let mut g = valid_genesis();
        g.validators = vec![make_validator(1)]; // Only 1, min is 3
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::TooFewValidators { .. })));
    }

    #[test]
    fn test_duplicate_node_id() {
        let mut g = valid_genesis();
        g.validators.push(make_validator(1)); // duplicate
        // Fix public key to avoid duplicate PK error
        g.validators.last_mut().unwrap().public_key = hex::encode([200u8; 32]);
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::DuplicateNodeId(_))));
    }

    #[test]
    fn test_duplicate_public_key() {
        let mut g = valid_genesis();
        g.validators[1].public_key = g.validators[0].public_key.clone();
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::DuplicatePublicKey(_))));
    }

    #[test]
    fn test_invalid_node_id_format() {
        let mut g = valid_genesis();
        g.validators[0].node_id = "not-hex".into();
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::InvalidNodeId(_))));
    }

    #[test]
    fn test_invalid_public_key_format() {
        let mut g = valid_genesis();
        g.validators[0].public_key = "short".into();
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::InvalidPublicKey(_))));
    }

    #[test]
    fn test_invalid_quorum_zero_denominator() {
        let mut g = valid_genesis();
        g.quorum_denominator = 0;
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::InvalidQuorum { .. })));
    }

    #[test]
    fn test_invalid_quorum_numerator_exceeds() {
        let mut g = valid_genesis();
        g.quorum_numerator = 4;
        g.quorum_denominator = 3;
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::InvalidQuorum { .. })));
    }

    #[test]
    fn test_invalid_committee_range() {
        let mut g = valid_genesis();
        g.min_committee_size = 10;
        g.max_committee_size = 5;
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::InvalidCommitteeRange { .. })));
    }

    #[test]
    fn test_zero_slot_duration() {
        let mut g = valid_genesis();
        g.slot_duration_ms = 0;
        let errors = validate_genesis(&g);
        assert!(errors.contains(&GenesisValidationError::ZeroSlotDuration));
    }

    #[test]
    fn test_incompatible_version() {
        let mut g = valid_genesis();
        g.version.protocol_version = crate::versioning::ProtocolVersion::new(99, 0, 0);
        let errors = validate_genesis(&g);
        assert!(errors.iter().any(|e| matches!(e, GenesisValidationError::IncompatibleVersion(_))));
    }

    #[test]
    fn test_strict_validation_returns_first_error() {
        let mut g = valid_genesis();
        g.chain_id = "".into();
        g.version.chain_id = "".into();
        let result = validate_genesis_strict(&g);
        assert!(result.is_err());
    }

    #[test]
    fn test_strict_validation_ok_on_valid() {
        let result = validate_genesis_strict(&valid_genesis());
        assert!(result.is_ok());
    }

    #[test]
    fn test_deterministic_genesis_reproduction() {
        let g1 = valid_genesis();
        let json1 = serde_json::to_string(&g1).unwrap();
        let g2: PoSeqGenesisState = serde_json::from_str(&json1).unwrap();
        assert_eq!(g1.genesis_hash(), g2.genesis_hash());
    }
}
