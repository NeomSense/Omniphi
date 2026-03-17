//! Validator registry — registration, staking, status management.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ValidatorStatus {
    Pending,
    Active,
    Inactive,
    Jailed,
    Tombstoned,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ValidatorProfile {
    pub validator_id: [u8; 32],
    pub moniker: String,
    pub operator_address: [u8; 32],
    pub signing_key: [u8; 32],
    pub network_address: String,
    pub stake: u128,
    pub status: ValidatorStatus,
    pub registered_at_epoch: u64,
    pub last_active_epoch: u64,
    pub signed_blocks: u64,
    pub missed_blocks: u64,
    pub slashing_events: u64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValidatorError {
    AlreadyRegistered([u8; 32]),
    NotFound([u8; 32]),
    InsufficientStake { required: u128, provided: u128 },
    InvalidMoniker(String),
    ZeroId,
    AlreadyActive,
    AlreadyJailed,
    NotJailed,
    Tombstoned,
}

impl std::fmt::Display for ValidatorError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::AlreadyRegistered(id) => write!(f, "validator {} already registered", hex::encode(&id[..4])),
            Self::NotFound(id) => write!(f, "validator {} not found", hex::encode(&id[..4])),
            Self::InsufficientStake { required, provided } => write!(f, "stake {} < required {}", provided, required),
            Self::InvalidMoniker(m) => write!(f, "invalid moniker: {}", m),
            Self::ZeroId => write!(f, "zero validator ID"),
            Self::AlreadyActive => write!(f, "already active"),
            Self::AlreadyJailed => write!(f, "already jailed"),
            Self::NotJailed => write!(f, "not jailed"),
            Self::Tombstoned => write!(f, "validator tombstoned"),
        }
    }
}

pub struct ValidatorRegistry {
    validators: BTreeMap<[u8; 32], ValidatorProfile>,
    min_stake: u128,
}

impl ValidatorRegistry {
    pub fn new(min_stake: u128) -> Self {
        ValidatorRegistry { validators: BTreeMap::new(), min_stake }
    }

    pub fn register(&mut self, profile: ValidatorProfile) -> Result<(), ValidatorError> {
        if profile.validator_id == [0u8; 32] {
            return Err(ValidatorError::ZeroId);
        }
        if self.validators.contains_key(&profile.validator_id) {
            return Err(ValidatorError::AlreadyRegistered(profile.validator_id));
        }
        if profile.moniker.is_empty() || profile.moniker.len() > 64 {
            return Err(ValidatorError::InvalidMoniker(profile.moniker.clone()));
        }
        if profile.stake < self.min_stake {
            return Err(ValidatorError::InsufficientStake {
                required: self.min_stake,
                provided: profile.stake,
            });
        }
        self.validators.insert(profile.validator_id, profile);
        Ok(())
    }

    pub fn activate(&mut self, id: &[u8; 32]) -> Result<(), ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        if v.status == ValidatorStatus::Tombstoned { return Err(ValidatorError::Tombstoned); }
        if v.status == ValidatorStatus::Active { return Err(ValidatorError::AlreadyActive); }
        v.status = ValidatorStatus::Active;
        Ok(())
    }

    pub fn deactivate(&mut self, id: &[u8; 32]) -> Result<(), ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        v.status = ValidatorStatus::Inactive;
        Ok(())
    }

    pub fn jail(&mut self, id: &[u8; 32]) -> Result<(), ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        if v.status == ValidatorStatus::Jailed { return Err(ValidatorError::AlreadyJailed); }
        v.status = ValidatorStatus::Jailed;
        v.slashing_events += 1;
        Ok(())
    }

    pub fn unjail(&mut self, id: &[u8; 32]) -> Result<(), ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        if v.status != ValidatorStatus::Jailed { return Err(ValidatorError::NotJailed); }
        v.status = ValidatorStatus::Inactive;
        Ok(())
    }

    pub fn tombstone(&mut self, id: &[u8; 32]) -> Result<(), ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        v.status = ValidatorStatus::Tombstoned;
        Ok(())
    }

    pub fn slash_stake(&mut self, id: &[u8; 32], bps: u64) -> Result<u128, ValidatorError> {
        let v = self.validators.get_mut(id).ok_or(ValidatorError::NotFound(*id))?;
        let slash_amount = v.stake * bps as u128 / 10_000;
        v.stake = v.stake.saturating_sub(slash_amount);
        v.slashing_events += 1;
        Ok(slash_amount)
    }

    pub fn record_signed_block(&mut self, id: &[u8; 32]) {
        if let Some(v) = self.validators.get_mut(id) {
            v.signed_blocks += 1;
        }
    }

    pub fn record_missed_block(&mut self, id: &[u8; 32]) {
        if let Some(v) = self.validators.get_mut(id) {
            v.missed_blocks += 1;
        }
    }

    pub fn get(&self, id: &[u8; 32]) -> Option<&ValidatorProfile> {
        self.validators.get(id)
    }

    pub fn active_validators(&self) -> Vec<&ValidatorProfile> {
        self.validators.values()
            .filter(|v| v.status == ValidatorStatus::Active)
            .collect()
    }

    pub fn active_validator_ids(&self) -> std::collections::BTreeSet<[u8; 32]> {
        self.active_validators().iter().map(|v| v.validator_id).collect()
    }

    pub fn count(&self) -> usize {
        self.validators.len()
    }

    pub fn active_count(&self) -> usize {
        self.active_validators().len()
    }

    pub fn update_min_stake(&mut self, new_min: u128) {
        self.min_stake = new_min;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_validator(b: u8, stake: u128) -> ValidatorProfile {
        let mut id = [0u8; 32]; id[0] = b;
        ValidatorProfile {
            validator_id: id,
            moniker: format!("validator-{}", b),
            operator_address: id,
            signing_key: id,
            network_address: format!("127.0.0.1:{}", 26600 + b as u16),
            stake,
            status: ValidatorStatus::Pending,
            registered_at_epoch: 1,
            last_active_epoch: 1,
            signed_blocks: 0,
            missed_blocks: 0,
            slashing_events: 0,
        }
    }

    #[test]
    fn test_register_and_activate() {
        let mut reg = ValidatorRegistry::new(1_000);
        let v = make_validator(1, 10_000);
        reg.register(v).unwrap();
        assert_eq!(reg.count(), 1);
        assert_eq!(reg.active_count(), 0);

        let id = { let mut i = [0u8; 32]; i[0] = 1; i };
        reg.activate(&id).unwrap();
        assert_eq!(reg.active_count(), 1);
    }

    #[test]
    fn test_insufficient_stake() {
        let mut reg = ValidatorRegistry::new(100_000);
        let v = make_validator(1, 1_000);
        match reg.register(v) {
            Err(ValidatorError::InsufficientStake { .. }) => {}
            other => panic!("expected InsufficientStake, got {:?}", other),
        }
    }

    #[test]
    fn test_duplicate_registration() {
        let mut reg = ValidatorRegistry::new(1_000);
        reg.register(make_validator(1, 10_000)).unwrap();
        match reg.register(make_validator(1, 10_000)) {
            Err(ValidatorError::AlreadyRegistered(_)) => {}
            other => panic!("expected AlreadyRegistered, got {:?}", other),
        }
    }

    #[test]
    fn test_jail_unjail_cycle() {
        let mut reg = ValidatorRegistry::new(1_000);
        reg.register(make_validator(1, 10_000)).unwrap();
        let id = { let mut i = [0u8; 32]; i[0] = 1; i };
        reg.activate(&id).unwrap();
        reg.jail(&id).unwrap();
        assert_eq!(reg.get(&id).unwrap().status, ValidatorStatus::Jailed);
        reg.unjail(&id).unwrap();
        assert_eq!(reg.get(&id).unwrap().status, ValidatorStatus::Inactive);
    }

    #[test]
    fn test_slash_stake() {
        let mut reg = ValidatorRegistry::new(1_000);
        reg.register(make_validator(1, 10_000)).unwrap();
        let id = { let mut i = [0u8; 32]; i[0] = 1; i };
        let slashed = reg.slash_stake(&id, 1_000).unwrap(); // 10%
        assert_eq!(slashed, 1_000);
        assert_eq!(reg.get(&id).unwrap().stake, 9_000);
    }

    #[test]
    fn test_tombstone_prevents_activation() {
        let mut reg = ValidatorRegistry::new(1_000);
        reg.register(make_validator(1, 10_000)).unwrap();
        let id = { let mut i = [0u8; 32]; i[0] = 1; i };
        reg.tombstone(&id).unwrap();
        assert!(reg.activate(&id).is_err());
    }

    #[test]
    fn test_block_tracking() {
        let mut reg = ValidatorRegistry::new(1_000);
        reg.register(make_validator(1, 10_000)).unwrap();
        let id = { let mut i = [0u8; 32]; i[0] = 1; i };
        reg.record_signed_block(&id);
        reg.record_signed_block(&id);
        reg.record_missed_block(&id);
        let v = reg.get(&id).unwrap();
        assert_eq!(v.signed_blocks, 2);
        assert_eq!(v.missed_blocks, 1);
    }
}
