//! Fee routing and distribution — deterministic split of PoSeq fees.
//!
//! PoSeq fees are split across four buckets:
//! - Sequencer rewards (default 45%)
//! - Shared security / validator pool (default 20%)
//! - Protocol treasury (default 15%)
//! - Burn (default 20%)
//!
//! Rounding rule: any dust from integer division goes to the sequencer bucket
//! (the largest recipient) to ensure no funds are lost.

use serde::{Deserialize, Serialize};
use super::parameters::PoSeqFeeParameters;

/// Result of routing a fee through the distribution split.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PoSeqFeeDistribution {
    /// Total fee being distributed.
    pub total_fee: u128,
    /// Amount allocated to sequencer rewards.
    pub sequencer_rewards: u128,
    /// Amount allocated to shared security (validators).
    pub shared_security: u128,
    /// Amount allocated to protocol treasury.
    pub treasury: u128,
    /// Amount burned (permanently removed from supply).
    pub burn: u128,
}

impl PoSeqFeeDistribution {
    /// Verify that the distribution sums to the total (no lost funds).
    pub fn verify_budget_neutral(&self) -> bool {
        self.sequencer_rewards
            .saturating_add(self.shared_security)
            .saturating_add(self.treasury)
            .saturating_add(self.burn)
            == self.total_fee
    }
}

/// Compute the fee distribution for a given total fee.
///
/// Uses integer basis-point arithmetic with deterministic rounding.
/// The sequencer bucket absorbs any rounding dust.
pub fn distribute_fee(params: &PoSeqFeeParameters, total_fee: u128) -> PoSeqFeeDistribution {
    if total_fee == 0 {
        return PoSeqFeeDistribution {
            total_fee: 0,
            sequencer_rewards: 0,
            shared_security: 0,
            treasury: 0,
            burn: 0,
        };
    }

    // Compute each share using integer division (rounds down)
    let shared_security = total_fee
        .saturating_mul(params.shared_security_bps as u128)
        / 10_000;
    let treasury = total_fee
        .saturating_mul(params.treasury_bps as u128)
        / 10_000;
    let burn = total_fee
        .saturating_mul(params.burn_bps as u128)
        / 10_000;

    // Sequencer gets the remainder — absorbs all rounding dust.
    // This is deterministic: dust always goes to the largest bucket.
    let sequencer_rewards = total_fee
        .saturating_sub(shared_security)
        .saturating_sub(treasury)
        .saturating_sub(burn);

    PoSeqFeeDistribution {
        total_fee,
        sequencer_rewards,
        shared_security,
        treasury,
        burn,
    }
}

/// Aggregate multiple fee distributions into one.
pub fn aggregate_distributions(distributions: &[PoSeqFeeDistribution]) -> PoSeqFeeDistribution {
    let mut agg = PoSeqFeeDistribution {
        total_fee: 0,
        sequencer_rewards: 0,
        shared_security: 0,
        treasury: 0,
        burn: 0,
    };
    for d in distributions {
        agg.total_fee = agg.total_fee.saturating_add(d.total_fee);
        agg.sequencer_rewards = agg.sequencer_rewards.saturating_add(d.sequencer_rewards);
        agg.shared_security = agg.shared_security.saturating_add(d.shared_security);
        agg.treasury = agg.treasury.saturating_add(d.treasury);
        agg.burn = agg.burn.saturating_add(d.burn);
    }
    agg
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn default_params() -> PoSeqFeeParameters {
        PoSeqFeeParameters::testnet_defaults()
    }

    #[test]
    fn test_distribution_budget_neutral() {
        let params = default_params();
        let dist = distribute_fee(&params, 10_000);
        assert!(dist.verify_budget_neutral());
        assert_eq!(dist.total_fee, 10_000);

        // 4500 + 2000 + 1500 + 2000 = 10000
        assert_eq!(dist.sequencer_rewards, 4_500);
        assert_eq!(dist.shared_security, 2_000);
        assert_eq!(dist.treasury, 1_500);
        assert_eq!(dist.burn, 2_000);
    }

    #[test]
    fn test_distribution_with_rounding() {
        let params = default_params();
        // 7 doesn't divide evenly by 10000 in all buckets
        let dist = distribute_fee(&params, 7);
        assert!(dist.verify_budget_neutral());

        // shared_security = 7 * 2000 / 10000 = 1 (truncated)
        // treasury = 7 * 1500 / 10000 = 1 (truncated)
        // burn = 7 * 2000 / 10000 = 1 (truncated)
        // sequencer = 7 - 1 - 1 - 1 = 4 (absorbs dust)
        assert_eq!(dist.shared_security, 1);
        assert_eq!(dist.treasury, 1);
        assert_eq!(dist.burn, 1);
        assert_eq!(dist.sequencer_rewards, 4);
    }

    #[test]
    fn test_distribution_zero_fee() {
        let params = default_params();
        let dist = distribute_fee(&params, 0);
        assert!(dist.verify_budget_neutral());
        assert_eq!(dist.sequencer_rewards, 0);
    }

    #[test]
    fn test_distribution_one_unit() {
        let params = default_params();
        let dist = distribute_fee(&params, 1);
        assert!(dist.verify_budget_neutral());
        // All buckets truncate to 0, sequencer gets the 1
        assert_eq!(dist.sequencer_rewards, 1);
        assert_eq!(dist.shared_security, 0);
        assert_eq!(dist.treasury, 0);
        assert_eq!(dist.burn, 0);
    }

    #[test]
    fn test_distribution_large_fee() {
        let params = default_params();
        let dist = distribute_fee(&params, 1_000_000_000);
        assert!(dist.verify_budget_neutral());

        // Exact for large round numbers
        assert_eq!(dist.sequencer_rewards, 450_000_000);
        assert_eq!(dist.shared_security, 200_000_000);
        assert_eq!(dist.treasury, 150_000_000);
        assert_eq!(dist.burn, 200_000_000);
    }

    #[test]
    fn test_distribution_deterministic() {
        let params = default_params();
        let d1 = distribute_fee(&params, 12_345);
        let d2 = distribute_fee(&params, 12_345);
        assert_eq!(d1, d2);
    }

    #[test]
    fn test_aggregate_distributions() {
        let params = default_params();
        let d1 = distribute_fee(&params, 10_000);
        let d2 = distribute_fee(&params, 20_000);
        let agg = aggregate_distributions(&[d1, d2]);

        assert_eq!(agg.total_fee, 30_000);
        assert_eq!(agg.sequencer_rewards, 4_500 + 9_000);
        assert_eq!(agg.shared_security, 2_000 + 4_000);
        assert_eq!(agg.treasury, 1_500 + 3_000);
        assert_eq!(agg.burn, 2_000 + 4_000);
    }

    #[test]
    fn test_many_small_distributions_budget_neutral() {
        let params = default_params();
        // Run 1000 distributions of various sizes and verify all are budget-neutral
        for fee in 0..1000 {
            let dist = distribute_fee(&params, fee);
            assert!(
                dist.verify_budget_neutral(),
                "budget neutrality failed for fee={}",
                fee
            );
        }
    }

    #[test]
    fn test_custom_split_distribution() {
        let params = PoSeqFeeParameters {
            sequencer_reward_bps: 6_000, // 60%
            shared_security_bps: 2_000,  // 20%
            treasury_bps: 1_000,         // 10%
            burn_bps: 1_000,             // 10%
            ..PoSeqFeeParameters::testnet_defaults()
        };
        let dist = distribute_fee(&params, 100_000);
        assert!(dist.verify_budget_neutral());
        assert_eq!(dist.sequencer_rewards, 60_000);
        assert_eq!(dist.shared_security, 20_000);
        assert_eq!(dist.treasury, 10_000);
        assert_eq!(dist.burn, 10_000);
    }
}
