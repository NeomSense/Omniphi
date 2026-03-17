//! Economic invariants — constraints on rewards, slashing, and bonds.

/// Economic invariant: rewards must not exceed the reward pool.
pub fn check_rewards_within_pool(
    total_distributed: u128,
    pool_size: u128,
) -> Result<(), EconomicInvariantViolation> {
    if total_distributed > pool_size {
        Err(EconomicInvariantViolation::RewardsExceedPool {
            distributed: total_distributed,
            pool: pool_size,
        })
    } else {
        Ok(())
    }
}

/// Economic invariant: slashed funds cannot be redistributed twice.
///
/// Given a set of already-redistributed slash IDs, verify the candidate is fresh.
pub fn check_no_double_redistribution(
    redistributed_slashes: &std::collections::BTreeSet<[u8; 32]>,
    slash_id: &[u8; 32],
) -> Result<(), EconomicInvariantViolation> {
    if redistributed_slashes.contains(slash_id) {
        Err(EconomicInvariantViolation::DoubleRedistribution { slash_id: *slash_id })
    } else {
        Ok(())
    }
}

/// Economic invariant: bond cannot be negative.
///
/// After any operation (slash, lock, unlock, unbond), total_bond must be >= 0.
/// Since we use u128, this check verifies that operations didn't wrap.
pub fn check_bond_non_negative(
    total_bond: u128,
    locked_bond: u128,
) -> Result<(), EconomicInvariantViolation> {
    if locked_bond > total_bond {
        Err(EconomicInvariantViolation::NegativeBond {
            total: total_bond,
            locked: locked_bond,
        })
    } else {
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EconomicInvariantViolation {
    RewardsExceedPool { distributed: u128, pool: u128 },
    DoubleRedistribution { slash_id: [u8; 32] },
    NegativeBond { total: u128, locked: u128 },
}

impl std::fmt::Display for EconomicInvariantViolation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::RewardsExceedPool { distributed, pool } => {
                write!(f, "INVARIANT VIOLATION: rewards {} exceed pool {}", distributed, pool)
            }
            Self::DoubleRedistribution { slash_id } => {
                write!(f, "INVARIANT VIOLATION: slash {} redistributed twice", hex::encode(&slash_id[..4]))
            }
            Self::NegativeBond { total, locked } => {
                write!(f, "INVARIANT VIOLATION: locked {} > total bond {}", locked, total)
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rewards_within_pool() {
        assert!(check_rewards_within_pool(90, 100).is_ok());
        assert!(check_rewards_within_pool(100, 100).is_ok());
        assert!(check_rewards_within_pool(101, 100).is_err());
    }

    #[test]
    fn test_no_double_redistribution() {
        let mut set = std::collections::BTreeSet::new();
        let id = [1u8; 32];
        assert!(check_no_double_redistribution(&set, &id).is_ok());
        set.insert(id);
        assert!(check_no_double_redistribution(&set, &id).is_err());
    }

    #[test]
    fn test_bond_non_negative() {
        assert!(check_bond_non_negative(100, 50).is_ok());
        assert!(check_bond_non_negative(100, 100).is_ok());
        assert!(check_bond_non_negative(50, 100).is_err());
    }
}
