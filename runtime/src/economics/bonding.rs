//! Bond management — lock, unlock, unbonding queue.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

/// State of a solver's bond.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BondState {
    pub solver_id: [u8; 32],
    pub total_bond: u128,
    pub locked_bond: u128,        // locked for active commitments
    pub unbonding: Vec<UnbondingEntry>,
}

impl BondState {
    pub fn new(solver_id: [u8; 32], initial_bond: u128) -> Self {
        BondState {
            solver_id,
            total_bond: initial_bond,
            locked_bond: 0,
            unbonding: Vec::new(),
        }
    }

    /// Available bond (total - locked - unbonding).
    pub fn available(&self) -> u128 {
        let unbonding_total: u128 = self.unbonding.iter().map(|u| u.amount).sum();
        self.total_bond.saturating_sub(self.locked_bond).saturating_sub(unbonding_total)
    }

    /// Lock bond for a commitment.
    pub fn lock(&mut self, amount: u128) -> Result<(), BondError> {
        if self.available() < amount {
            return Err(BondError::InsufficientAvailable {
                required: amount,
                available: self.available(),
            });
        }
        self.locked_bond += amount;
        Ok(())
    }

    /// Unlock bond after reveal or window close.
    pub fn unlock(&mut self, amount: u128) {
        self.locked_bond = self.locked_bond.saturating_sub(amount);
    }

    /// Slash the bond. Reduces total_bond.
    pub fn slash(&mut self, amount: u128) -> u128 {
        let actual = amount.min(self.total_bond);
        self.total_bond -= actual;
        // Ensure locked doesn't exceed total
        self.locked_bond = self.locked_bond.min(self.total_bond);
        actual
    }

    /// Start unbonding process.
    pub fn begin_unbonding(&mut self, amount: u128, completion_block: u64) -> Result<(), BondError> {
        if self.available() < amount {
            return Err(BondError::InsufficientAvailable {
                required: amount,
                available: self.available(),
            });
        }
        self.unbonding.push(UnbondingEntry { amount, completion_block });
        Ok(())
    }

    /// Complete unbonding entries that have matured.
    pub fn complete_unbonding(&mut self, current_block: u64) -> u128 {
        let mut completed = 0u128;
        self.unbonding.retain(|entry| {
            if current_block >= entry.completion_block {
                completed += entry.amount;
                self.total_bond -= entry.amount;
                false // remove
            } else {
                true // keep
            }
        });
        completed
    }

    /// Check if bond meets minimum threshold.
    pub fn meets_minimum(&self, min_bond: u128) -> bool {
        self.total_bond >= min_bond
    }
}

/// An entry in the unbonding queue.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnbondingEntry {
    pub amount: u128,
    pub completion_block: u64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BondError {
    InsufficientAvailable { required: u128, available: u128 },
    MinimumBondViolation { minimum: u128, current: u128 },
}

impl std::fmt::Display for BondError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InsufficientAvailable { required, available } => {
                write!(f, "insufficient bond: required {}, available {}", required, available)
            }
            Self::MinimumBondViolation { minimum, current } => {
                write!(f, "bond {} below minimum {}", current, minimum)
            }
        }
    }
}

impl std::error::Error for BondError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bond_lock_unlock() {
        let mut bond = BondState::new([1u8; 32], 50_000);
        assert_eq!(bond.available(), 50_000);

        bond.lock(10_000).unwrap();
        assert_eq!(bond.available(), 40_000);
        assert_eq!(bond.locked_bond, 10_000);

        bond.unlock(10_000);
        assert_eq!(bond.available(), 50_000);
    }

    #[test]
    fn test_bond_insufficient() {
        let mut bond = BondState::new([1u8; 32], 5_000);
        assert!(bond.lock(10_000).is_err());
    }

    #[test]
    fn test_bond_slash() {
        let mut bond = BondState::new([1u8; 32], 50_000);
        bond.lock(20_000).unwrap();

        let slashed = bond.slash(30_000);
        assert_eq!(slashed, 30_000);
        assert_eq!(bond.total_bond, 20_000);
        assert_eq!(bond.locked_bond, 20_000); // clamped to total
    }

    #[test]
    fn test_unbonding() {
        let mut bond = BondState::new([1u8; 32], 50_000);
        bond.begin_unbonding(10_000, 100).unwrap();

        assert_eq!(bond.available(), 40_000);
        assert_eq!(bond.total_bond, 50_000); // not yet deducted

        // Before completion
        let completed = bond.complete_unbonding(50);
        assert_eq!(completed, 0);

        // After completion
        let completed = bond.complete_unbonding(100);
        assert_eq!(completed, 10_000);
        assert_eq!(bond.total_bond, 40_000);
    }
}
