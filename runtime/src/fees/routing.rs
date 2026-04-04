//! Fee Routing — Distributes PoSeq Fees to Destinations
//!
//! Routes sequencing fees to:
//! - Burn (removed from supply)
//! - Sequencer reward pool
//! - Shared security / validator pool
//! - Treasury
//!
//! All splits use basis points. Totals are validated to sum exactly.
//! No underflow, no overflow, deterministic accounting.

use super::poseq_fee::PoSeqFeeParams;

/// Parameters for fee routing (extracted from PoSeqFeeParams for clarity).
#[derive(Debug, Clone)]
pub struct FeeRoutingParams {
    pub burn_bps: u64,
    pub sequencer_reward_bps: u64,
    pub shared_security_bps: u64,
    pub treasury_bps: u64,
}

impl FeeRoutingParams {
    pub fn from_poseq_params(params: &PoSeqFeeParams) -> Self {
        FeeRoutingParams {
            burn_bps: params.burn_bps,
            sequencer_reward_bps: params.sequencer_reward_bps,
            shared_security_bps: params.shared_security_bps,
            treasury_bps: params.treasury_bps,
        }
    }

    /// Validate that splits sum to 10_000.
    pub fn validate(&self) -> Result<(), String> {
        let total = self.burn_bps + self.sequencer_reward_bps
            + self.shared_security_bps + self.treasury_bps;
        if total != 10_000 {
            return Err(format!("routing splits must sum to 10000, got {}", total));
        }
        Ok(())
    }
}

/// The result of routing a fee amount.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RoutedFees {
    /// Total fee being routed.
    pub total: u64,
    /// Amount burned (removed from supply).
    pub burned: u64,
    /// Amount to the sequencer who ordered this batch.
    pub sequencer_reward: u64,
    /// Amount to the shared security / validator pool.
    pub shared_security: u64,
    /// Amount to the protocol treasury.
    pub treasury: u64,
    /// Dust (rounding remainder) — added to burn to ensure conservation.
    pub dust: u64,
}

impl RoutedFees {
    /// Verify that all routed amounts sum to the total.
    pub fn check_conservation(&self) -> bool {
        let sum = self.burned
            .saturating_add(self.sequencer_reward)
            .saturating_add(self.shared_security)
            .saturating_add(self.treasury);
        sum == self.total
    }
}

/// Routes a fee amount according to the configured basis-point splits.
pub struct FeeRouting;

impl FeeRouting {
    /// Route a fee amount to its destinations.
    ///
    /// Uses integer division with dust correction:
    /// 1. Compute each split as `amount * bps / 10_000`
    /// 2. Sum all splits
    /// 3. Any rounding remainder (dust) is added to burn
    /// 4. Final sum == total (guaranteed)
    pub fn route(amount: u64, params: &FeeRoutingParams) -> RoutedFees {
        if amount == 0 {
            return RoutedFees {
                total: 0, burned: 0, sequencer_reward: 0,
                shared_security: 0, treasury: 0, dust: 0,
            };
        }

        let amount_128 = amount as u128;

        // Compute each share via integer division
        let sequencer_reward = ((amount_128 * params.sequencer_reward_bps as u128) / 10_000) as u64;
        let shared_security = ((amount_128 * params.shared_security_bps as u128) / 10_000) as u64;
        let treasury = ((amount_128 * params.treasury_bps as u128) / 10_000) as u64;
        let burn_base = ((amount_128 * params.burn_bps as u128) / 10_000) as u64;

        // Compute dust (rounding remainder)
        let accounted = sequencer_reward
            .saturating_add(shared_security)
            .saturating_add(treasury)
            .saturating_add(burn_base);
        let dust = amount.saturating_sub(accounted);

        // Add dust to burn (ensures exact conservation)
        let burned = burn_base.saturating_add(dust);

        RoutedFees {
            total: amount,
            burned,
            sequencer_reward,
            shared_security,
            treasury,
            dust,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn default_routing() -> FeeRoutingParams {
        FeeRoutingParams {
            burn_bps: 2_000,          // 20%
            sequencer_reward_bps: 5_000, // 50%
            shared_security_bps: 2_000,  // 20%
            treasury_bps: 1_000,      // 10%
        }
    }

    // ── Validation ───────────────────────────────────────────

    #[test]
    fn test_valid_routing() {
        assert!(default_routing().validate().is_ok());
    }

    #[test]
    fn test_invalid_routing_sum() {
        let mut r = default_routing();
        r.burn_bps = 5_000; // total now 13_000
        assert!(r.validate().is_err());
    }

    // ── Routing correctness ──────────────────────────────────

    #[test]
    fn test_route_10000() {
        let routed = FeeRouting::route(10_000, &default_routing());
        assert_eq!(routed.sequencer_reward, 5_000); // 50%
        assert_eq!(routed.shared_security, 2_000);  // 20%
        assert_eq!(routed.treasury, 1_000);          // 10%
        assert_eq!(routed.burned, 2_000);            // 20%
        assert!(routed.check_conservation());
    }

    #[test]
    fn test_route_odd_amount() {
        // 33333 doesn't divide evenly by 10000
        let routed = FeeRouting::route(33_333, &default_routing());
        assert!(routed.check_conservation(), "Conservation must hold for odd amounts");
        // Dust should be small
        assert!(routed.dust <= 4, "Dust should be at most 3 for 4 splits");
    }

    #[test]
    fn test_route_zero() {
        let routed = FeeRouting::route(0, &default_routing());
        assert_eq!(routed.total, 0);
        assert!(routed.check_conservation());
    }

    #[test]
    fn test_route_one() {
        let routed = FeeRouting::route(1, &default_routing());
        assert_eq!(routed.total, 1);
        assert!(routed.check_conservation());
        // At 1 unit, most splits round to 0, dust goes to burn
        assert_eq!(routed.burned, 1);
    }

    #[test]
    fn test_route_large_amount() {
        let routed = FeeRouting::route(1_000_000_000, &default_routing());
        assert!(routed.check_conservation());
        assert_eq!(routed.sequencer_reward, 500_000_000); // exact 50%
        assert_eq!(routed.shared_security, 200_000_000);  // exact 20%
        assert_eq!(routed.treasury, 100_000_000);          // exact 10%
    }

    // ── Conservation property ────────────────────────────────

    #[test]
    fn test_conservation_many_amounts() {
        let r = default_routing();
        for amount in [1, 2, 3, 7, 13, 99, 100, 999, 10_000, 12_345, 99_999, 1_000_000] {
            let routed = FeeRouting::route(amount, &r);
            assert!(routed.check_conservation(),
                "Conservation failed for amount {}: total={} sum={}",
                amount, routed.total,
                routed.burned + routed.sequencer_reward + routed.shared_security + routed.treasury
            );
        }
    }

    // ── No overflow ──────────────────────────────────────────

    #[test]
    fn test_no_overflow_max() {
        let routed = FeeRouting::route(u64::MAX, &default_routing());
        assert!(routed.check_conservation());
    }

    // ── Custom routing split ─────────────────────────────────

    #[test]
    fn test_all_to_burn() {
        let r = FeeRoutingParams {
            burn_bps: 10_000,
            sequencer_reward_bps: 0,
            shared_security_bps: 0,
            treasury_bps: 0,
        };
        assert!(r.validate().is_ok());
        let routed = FeeRouting::route(50_000, &r);
        assert_eq!(routed.burned, 50_000);
        assert_eq!(routed.sequencer_reward, 0);
        assert!(routed.check_conservation());
    }

    #[test]
    fn test_all_to_sequencer() {
        let r = FeeRoutingParams {
            burn_bps: 0,
            sequencer_reward_bps: 10_000,
            shared_security_bps: 0,
            treasury_bps: 0,
        };
        let routed = FeeRouting::route(50_000, &r);
        assert_eq!(routed.sequencer_reward, 50_000);
        assert!(routed.check_conservation());
    }
}
