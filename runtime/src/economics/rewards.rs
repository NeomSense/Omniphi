//! Epoch reward computation and distribution — Section 12.3.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

/// A recipient of epoch rewards.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RewardRecipient {
    pub id: [u8; 32],
    pub role: RewardRole,
    pub amount: u128,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum RewardRole {
    Validator,
    Solver,
    Treasury,
    Insurance,
    Challenger,
}

/// Distribution of rewards for a single epoch.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EpochRewardDistribution {
    pub epoch: u64,
    pub total_revenue: u128,
    pub validator_pool: u128,
    pub solver_pool: u128,
    pub treasury_amount: u128,
    pub insurance_amount: u128,
    pub recipients: Vec<RewardRecipient>,
}

/// Compute epoch reward distribution.
///
/// Revenue split: 50% validators, 30% solvers, 10% treasury, 10% insurance.
pub fn compute_epoch_rewards(
    epoch: u64,
    total_revenue: u128,
    validator_stakes: &BTreeMap<[u8; 32], (u128, f64)>, // id → (stake, uptime_fraction)
    solver_fills: &BTreeMap<[u8; 32], (u64, u64)>,       // id → (fills, performance_score)
) -> EpochRewardDistribution {
    let validator_pool = total_revenue * 50 / 100;
    let solver_pool = total_revenue * 30 / 100;
    let treasury_amount = total_revenue * 10 / 100;
    let insurance_amount = total_revenue - validator_pool - solver_pool - treasury_amount;

    let mut recipients = Vec::new();

    // Validator rewards: proportional to uptime
    // Residual from rounding goes to treasury to maintain budget neutrality.
    let mut validator_distributed: u128 = 0;
    if !validator_stakes.is_empty() {
        let total_weighted: f64 = validator_stakes.values()
            .map(|(_, uptime)| uptime)
            .sum();

        if total_weighted > 0.0 {
            for (id, (_, uptime)) in validator_stakes {
                let share = (*uptime / total_weighted * validator_pool as f64) as u128;
                if share > 0 {
                    recipients.push(RewardRecipient {
                        id: *id,
                        role: RewardRole::Validator,
                        amount: share,
                    });
                    validator_distributed += share;
                }
            }
        }
    }

    // Solver rewards: proportional to fills weighted by performance
    let mut solver_distributed: u128 = 0;
    if !solver_fills.is_empty() {
        let total_fills: u64 = solver_fills.values().map(|(f, _)| f).sum();
        if total_fills > 0 {
            for (id, (fills, perf_score)) in solver_fills {
                let fill_share = *fills as f64 / total_fills as f64;
                let quality_bonus = 1.0 + (*perf_score as f64 / 10_000.0).min(1.0) * 0.5;
                let share = (fill_share * quality_bonus * solver_pool as f64) as u128;
                // Cap individual share to pool to prevent quality_bonus exceeding pool
                let capped = share.min(solver_pool.saturating_sub(solver_distributed));
                if capped > 0 {
                    recipients.push(RewardRecipient {
                        id: *id,
                        role: RewardRole::Solver,
                        amount: capped,
                    });
                    solver_distributed += capped;
                }
            }
        }
    }

    // Rounding residual goes to treasury for budget neutrality
    let validator_residual = validator_pool.saturating_sub(validator_distributed);
    let solver_residual = solver_pool.saturating_sub(solver_distributed);
    let adjusted_treasury = treasury_amount + validator_residual + solver_residual;

    // Treasury (includes rounding residual for budget neutrality)
    let treasury_id = [0xFE; 32]; // well-known treasury address
    recipients.push(RewardRecipient {
        id: treasury_id,
        role: RewardRole::Treasury,
        amount: adjusted_treasury,
    });

    // Insurance
    let insurance_id = [0xFF; 32]; // well-known insurance address
    recipients.push(RewardRecipient {
        id: insurance_id,
        role: RewardRole::Insurance,
        amount: insurance_amount,
    });

    EpochRewardDistribution {
        epoch,
        total_revenue,
        validator_pool,
        solver_pool,
        treasury_amount,
        insurance_amount,
        recipients,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_reward_distribution_split() {
        let mut validators = BTreeMap::new();
        validators.insert([1u8; 32], (100_000u128, 1.0));

        let mut solvers = BTreeMap::new();
        solvers.insert([2u8; 32], (10u64, 5_000u64));

        let dist = compute_epoch_rewards(1, 100_000, &validators, &solvers);

        assert_eq!(dist.validator_pool, 50_000);
        assert_eq!(dist.solver_pool, 30_000);
        assert_eq!(dist.treasury_amount, 10_000);
        assert_eq!(dist.insurance_amount, 10_000);
        assert!(dist.recipients.len() >= 4); // 1 validator + 1 solver + treasury + insurance
    }

    #[test]
    fn test_zero_revenue() {
        let dist = compute_epoch_rewards(1, 0, &BTreeMap::new(), &BTreeMap::new());
        assert_eq!(dist.total_revenue, 0);
        assert_eq!(dist.recipients.len(), 2); // treasury + insurance only
    }
}
