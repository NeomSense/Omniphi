//! PoSeq Sequencing Fee Model
//!
//! Prices the sequencing surface: ordering scarcity, mempool bandwidth,
//! admission anti-spam, and fairness-bounded priority.
//!
//! This is NOT runtime execution gas. It prices a DIFFERENT scarce resource.
//!
//! ## V1 Formula
//!
//! ```text
//! poseq_fee = max(
//!     fee_floor,
//!     (base_admission_fee + (intent_bytes * fee_per_byte) + min(tip, tip_cap))
//!     * congestion_multiplier_bps / 10_000
//! )
//! ```
//!
//! All arithmetic is integer-safe (u128 with basis points). No floating point.

use std::collections::BTreeMap;

/// Governance-configurable parameters for PoSeq sequencing fees.
///
/// All monetary values are in the smallest denomination (omniphi, 6 decimals).
/// All ratios are in basis points (10_000 = 100%).
#[derive(Debug, Clone)]
pub struct PoSeqFeeParams {
    /// Minimum fee every intent pays regardless of size or demand.
    /// Purpose: anti-spam, minimum economic weight.
    pub base_admission_fee: u64,

    /// Fee per byte of serialized intent.
    /// Purpose: bandwidth cost, propagation overhead.
    pub fee_per_byte: u64,

    /// Target number of intents per batch (for congestion calculation).
    pub target_intents_per_batch: u64,

    /// Target total bytes per batch (for congestion calculation).
    pub target_bytes_per_batch: u64,

    /// Minimum congestion multiplier in basis points (10_000 = 1.0x).
    /// Even in quiet periods, the multiplier never goes below this.
    pub min_congestion_multiplier_bps: u64,

    /// Maximum congestion multiplier in basis points.
    /// Even in extreme congestion, the multiplier never exceeds this.
    pub max_congestion_multiplier_bps: u64,

    /// Maximum priority tip a user can attach.
    /// Tips above this are clamped (not rejected).
    pub priority_tip_cap: u64,

    /// Maximum weight the tip has on ordering (basis points).
    /// If set to 2000 (20%), the tip can only influence 20% of the
    /// priority ranking — fairness policy dominates the other 80%.
    pub max_tip_priority_weight_bps: u64,

    /// Portion of sequencing fee retained when an intent expires
    /// without being sequenced (basis points of the computed fee).
    /// Purpose: compensate for intake/validation resources consumed.
    pub intent_expiry_penalty_bps: u64,

    /// Absolute minimum fee floor (after all computation).
    pub poseq_fee_floor: u64,

    /// Absolute maximum fee cap (prevents runaway congestion).
    pub max_poseq_fee_cap: u64,

    // ── Fee routing (must sum to 10_000) ────────────────────

    /// Portion burned (removed from supply).
    pub burn_bps: u64,

    /// Portion to the sequencer who ordered this batch.
    pub sequencer_reward_bps: u64,

    /// Portion to the shared security / validator pool.
    pub shared_security_bps: u64,

    /// Portion to the protocol treasury.
    pub treasury_bps: u64,
}

impl PoSeqFeeParams {
    /// Conservative defaults suitable for mainnet launch.
    pub fn default_params() -> Self {
        PoSeqFeeParams {
            base_admission_fee: 1_000,       // 0.001 OMNI
            fee_per_byte: 5,                 // 0.000005 OMNI/byte
            target_intents_per_batch: 100,
            target_bytes_per_batch: 500_000,  // 500KB
            min_congestion_multiplier_bps: 10_000,  // 1.0x floor
            max_congestion_multiplier_bps: 50_000,  // 5.0x ceiling
            priority_tip_cap: 100_000,       // 0.1 OMNI max tip
            max_tip_priority_weight_bps: 2_000,  // 20% max ordering influence
            intent_expiry_penalty_bps: 1_000,    // 10% of fee kept on expiry
            poseq_fee_floor: 500,            // 0.0005 OMNI absolute minimum
            max_poseq_fee_cap: 10_000_000,   // 10 OMNI absolute maximum
            burn_bps: 2_000,                 // 20% burned
            sequencer_reward_bps: 5_000,     // 50% to sequencer
            shared_security_bps: 2_000,      // 20% to validators
            treasury_bps: 1_000,             // 10% to treasury
        }
    }

    /// Validate parameters. Returns list of errors (empty = valid).
    pub fn validate(&self) -> Vec<String> {
        let mut errors = vec![];

        if self.target_intents_per_batch == 0 {
            errors.push("target_intents_per_batch must be > 0".into());
        }
        if self.target_bytes_per_batch == 0 {
            errors.push("target_bytes_per_batch must be > 0".into());
        }
        if self.min_congestion_multiplier_bps < 10_000 {
            errors.push("min_congestion_multiplier_bps must be >= 10000 (1.0x)".into());
        }
        if self.max_congestion_multiplier_bps < self.min_congestion_multiplier_bps {
            errors.push("max_congestion_multiplier_bps must be >= min".into());
        }
        if self.poseq_fee_floor > self.max_poseq_fee_cap {
            errors.push("fee_floor must be <= fee_cap".into());
        }
        if self.max_tip_priority_weight_bps > 5_000 {
            errors.push("max_tip_priority_weight_bps must be <= 5000 (50%)".into());
        }

        // Routing splits must sum to 10_000
        let total_routing = self.burn_bps
            + self.sequencer_reward_bps
            + self.shared_security_bps
            + self.treasury_bps;
        if total_routing != 10_000 {
            errors.push(format!(
                "fee routing splits must sum to 10000, got {}", total_routing
            ));
        }

        errors
    }
}

/// Current congestion state of the PoSeq mempool.
#[derive(Debug, Clone, Default)]
pub struct CongestionState {
    /// Current number of intents in the active pool.
    pub pool_intent_count: u64,
    /// Current total bytes of intents in the active pool.
    pub pool_total_bytes: u64,
}

/// Result of a PoSeq fee calculation.
#[derive(Debug, Clone)]
pub struct PoSeqFeeResult {
    /// The computed sequencing fee (in smallest denomination).
    pub total_fee: u64,
    /// Breakdown: base admission component.
    pub base_component: u64,
    /// Breakdown: bandwidth (byte) component.
    pub bandwidth_component: u64,
    /// Breakdown: priority tip component (after capping).
    pub tip_component: u64,
    /// The congestion multiplier applied (basis points).
    pub congestion_multiplier_bps: u64,
    /// Whether the fee was clamped to the floor.
    pub floor_applied: bool,
    /// Whether the fee was clamped to the cap.
    pub cap_applied: bool,
}

/// Computes PoSeq sequencing fees.
pub struct PoSeqFeeCalculator;

impl PoSeqFeeCalculator {
    /// Calculate the sequencing fee for an intent.
    ///
    /// All arithmetic uses u128 intermediates to prevent overflow,
    /// then clamps back to u64.
    pub fn calculate(
        params: &PoSeqFeeParams,
        intent_bytes: u64,
        priority_tip: u64,
        congestion: &CongestionState,
    ) -> PoSeqFeeResult {
        // 1. Base components
        let base_component = params.base_admission_fee;
        let bandwidth_component = (intent_bytes as u128)
            .saturating_mul(params.fee_per_byte as u128);
        let bandwidth_component = u64_clamp(bandwidth_component);

        // 2. Cap the tip
        let tip_component = std::cmp::min(priority_tip, params.priority_tip_cap);

        // 3. Compute congestion multiplier
        let congestion_multiplier_bps = Self::compute_congestion_multiplier(params, congestion);

        // 4. Combine: (base + bandwidth + tip) * multiplier / 10_000
        let pre_multiply = (base_component as u128)
            .saturating_add(bandwidth_component as u128)
            .saturating_add(tip_component as u128);

        let post_multiply = pre_multiply
            .saturating_mul(congestion_multiplier_bps as u128)
            .checked_div(10_000)
            .unwrap_or(u128::MAX);

        let mut total_fee = u64_clamp(post_multiply);

        // 5. Apply floor
        let floor_applied = total_fee < params.poseq_fee_floor;
        if floor_applied {
            total_fee = params.poseq_fee_floor;
        }

        // 6. Apply cap
        let cap_applied = total_fee > params.max_poseq_fee_cap;
        if cap_applied {
            total_fee = params.max_poseq_fee_cap;
        }

        PoSeqFeeResult {
            total_fee,
            base_component,
            bandwidth_component: u64_clamp(bandwidth_component as u128),
            tip_component,
            congestion_multiplier_bps,
            floor_applied,
            cap_applied,
        }
    }

    /// Compute the congestion multiplier in basis points.
    ///
    /// V1: max(intent_ratio, byte_ratio) as demand pressure signal.
    /// Clamped to [min_multiplier, max_multiplier].
    pub fn compute_congestion_multiplier(
        params: &PoSeqFeeParams,
        congestion: &CongestionState,
    ) -> u64 {
        // Intent-based demand ratio (bps)
        let intent_ratio_bps = if params.target_intents_per_batch > 0 {
            (congestion.pool_intent_count as u128)
                .saturating_mul(10_000)
                .checked_div(params.target_intents_per_batch as u128)
                .unwrap_or(0) as u64
        } else {
            10_000 // default 1.0x
        };

        // Byte-based demand ratio (bps)
        let byte_ratio_bps = if params.target_bytes_per_batch > 0 {
            (congestion.pool_total_bytes as u128)
                .saturating_mul(10_000)
                .checked_div(params.target_bytes_per_batch as u128)
                .unwrap_or(0) as u64
        } else {
            10_000
        };

        // Use the higher of the two demand signals
        let demand_bps = std::cmp::max(intent_ratio_bps, byte_ratio_bps);

        // Clamp to configured bounds
        std::cmp::min(
            std::cmp::max(demand_bps, params.min_congestion_multiplier_bps),
            params.max_congestion_multiplier_bps,
        )
    }

    /// Compute the expiry penalty (portion of fee retained on intent expiry).
    pub fn compute_expiry_penalty(params: &PoSeqFeeParams, computed_fee: u64) -> u64 {
        ((computed_fee as u128)
            .saturating_mul(params.intent_expiry_penalty_bps as u128)
            / 10_000) as u64
    }

    /// Compute the tip's priority weight for ordering integration.
    /// Returns a value in [0, max_tip_priority_weight_bps] that the
    /// fairness engine can use as a bounded input to ranking.
    pub fn compute_tip_priority_weight(
        params: &PoSeqFeeParams,
        tip: u64,
    ) -> u64 {
        if params.priority_tip_cap == 0 { return 0; }
        let capped_tip = std::cmp::min(tip, params.priority_tip_cap);
        // Normalize tip to [0, max_weight] proportionally
        ((capped_tip as u128)
            .saturating_mul(params.max_tip_priority_weight_bps as u128)
            / (params.priority_tip_cap as u128)) as u64
    }
}

/// Clamp a u128 to u64::MAX safely.
fn u64_clamp(v: u128) -> u64 {
    if v > u64::MAX as u128 { u64::MAX } else { v as u64 }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn default_params() -> PoSeqFeeParams { PoSeqFeeParams::default_params() }
    fn quiet() -> CongestionState { CongestionState { pool_intent_count: 10, pool_total_bytes: 50_000 } }
    fn normal() -> CongestionState { CongestionState { pool_intent_count: 80, pool_total_bytes: 400_000 } }
    fn congested() -> CongestionState { CongestionState { pool_intent_count: 300, pool_total_bytes: 1_500_000 } }

    // ── Parameter validation ─────────────────────────────────

    #[test]
    fn test_default_params_valid() {
        assert!(default_params().validate().is_empty());
    }

    #[test]
    fn test_invalid_routing_split() {
        let mut p = default_params();
        p.burn_bps = 9_000; // total now > 10_000
        let errors = p.validate();
        assert!(errors.iter().any(|e| e.contains("sum to 10000")));
    }

    #[test]
    fn test_invalid_zero_target() {
        let mut p = default_params();
        p.target_intents_per_batch = 0;
        assert!(!p.validate().is_empty());
    }

    #[test]
    fn test_invalid_tip_weight_too_high() {
        let mut p = default_params();
        p.max_tip_priority_weight_bps = 6_000; // >50%
        assert!(!p.validate().is_empty());
    }

    // ── Base admission fee ───────────────────────────────────

    #[test]
    fn test_base_admission_fee() {
        let p = default_params();
        let result = PoSeqFeeCalculator::calculate(&p, 0, 0, &quiet());
        assert!(result.total_fee >= p.base_admission_fee);
        assert_eq!(result.base_component, p.base_admission_fee);
    }

    // ── Fee per byte ─────────────────────────────────────────

    #[test]
    fn test_fee_per_byte() {
        let p = default_params();
        let r1 = PoSeqFeeCalculator::calculate(&p, 100, 0, &quiet());
        let r2 = PoSeqFeeCalculator::calculate(&p, 1000, 0, &quiet());
        assert!(r2.total_fee > r1.total_fee);
        assert_eq!(r2.bandwidth_component, 1000 * p.fee_per_byte);
    }

    // ── Congestion multiplier clamping ───────────────────────

    #[test]
    fn test_congestion_quiet() {
        let p = default_params();
        let m = PoSeqFeeCalculator::compute_congestion_multiplier(&p, &quiet());
        assert_eq!(m, p.min_congestion_multiplier_bps); // 10% of target = clamped to min
    }

    #[test]
    fn test_congestion_normal() {
        let p = default_params();
        let m = PoSeqFeeCalculator::compute_congestion_multiplier(&p, &normal());
        // 80/100 = 80% = 8000 bps, but min is 10000 → clamped to 10000
        assert!(m >= p.min_congestion_multiplier_bps);
    }

    #[test]
    fn test_congestion_high() {
        let p = default_params();
        let m = PoSeqFeeCalculator::compute_congestion_multiplier(&p, &congested());
        // 300/100 = 300% = 30000 bps → within max (50000)
        assert_eq!(m, 30_000);
    }

    #[test]
    fn test_congestion_extreme_clamped() {
        let p = default_params();
        let extreme = CongestionState { pool_intent_count: 10_000, pool_total_bytes: 50_000_000 };
        let m = PoSeqFeeCalculator::compute_congestion_multiplier(&p, &extreme);
        assert_eq!(m, p.max_congestion_multiplier_bps);
    }

    // ── Tip cap ──────────────────────────────────────────────

    #[test]
    fn test_tip_capped() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, 100, 999_999_999, &quiet());
        assert_eq!(r.tip_component, p.priority_tip_cap); // clamped
    }

    #[test]
    fn test_tip_below_cap() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, 100, 50_000, &quiet());
        assert_eq!(r.tip_component, 50_000);
    }

    // ── Priority weight bounded ──────────────────────────────

    #[test]
    fn test_tip_priority_weight_bounded() {
        let p = default_params();
        let w = PoSeqFeeCalculator::compute_tip_priority_weight(&p, 999_999_999);
        assert!(w <= p.max_tip_priority_weight_bps);
    }

    #[test]
    fn test_tip_priority_weight_proportional() {
        let p = default_params();
        let half = PoSeqFeeCalculator::compute_tip_priority_weight(&p, p.priority_tip_cap / 2);
        let full = PoSeqFeeCalculator::compute_tip_priority_weight(&p, p.priority_tip_cap);
        assert!(full > half);
        assert_eq!(full, p.max_tip_priority_weight_bps);
    }

    // ── Reward routing splits ────────────────────────────────

    #[test]
    fn test_routing_splits_sum() {
        let p = default_params();
        let total = p.burn_bps + p.sequencer_reward_bps + p.shared_security_bps + p.treasury_bps;
        assert_eq!(total, 10_000);
    }

    // ── Fee floor and cap ────────────────────────────────────

    #[test]
    fn test_fee_floor_applied() {
        let mut p = default_params();
        p.base_admission_fee = 0;
        p.fee_per_byte = 0;
        p.poseq_fee_floor = 1_000;
        let r = PoSeqFeeCalculator::calculate(&p, 0, 0, &quiet());
        assert_eq!(r.total_fee, 1_000);
        assert!(r.floor_applied);
    }

    #[test]
    fn test_fee_cap_applied() {
        let mut p = default_params();
        p.base_admission_fee = 10_000_000;
        p.max_poseq_fee_cap = 5_000_000;
        let r = PoSeqFeeCalculator::calculate(&p, 10_000, 100_000, &congested());
        assert_eq!(r.total_fee, 5_000_000);
        assert!(r.cap_applied);
    }

    // ── Expiry penalty ───────────────────────────────────────

    #[test]
    fn test_expiry_penalty() {
        let p = default_params();
        let penalty = PoSeqFeeCalculator::compute_expiry_penalty(&p, 10_000);
        // 10000 * 1000 / 10000 = 1000
        assert_eq!(penalty, 1_000);
    }

    // ── No overflow for large inputs ─────────────────────────

    #[test]
    fn test_no_overflow_large_bytes() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, u64::MAX / 2, 0, &congested());
        assert!(r.total_fee <= p.max_poseq_fee_cap);
    }

    #[test]
    fn test_no_overflow_max_everything() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, u64::MAX, u64::MAX, &CongestionState {
            pool_intent_count: u64::MAX, pool_total_bytes: u64::MAX,
        });
        assert!(r.total_fee <= p.max_poseq_fee_cap);
    }

    // ── Scenario: quiet network ──────────────────────────────

    #[test]
    fn test_scenario_quiet_network() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, 200, 0, &quiet());
        // base=1000, bandwidth=200*5=1000, tip=0, multiplier=1.0x → 2000
        assert_eq!(r.total_fee, 2_000);
        assert!(!r.floor_applied);
        assert!(!r.cap_applied);
    }

    // ── Scenario: congested network ──────────────────────────

    #[test]
    fn test_scenario_congested_network() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, 200, 10_000, &congested());
        // base=1000, bandwidth=1000, tip=10000, subtotal=12000
        // multiplier=30000 bps (3.0x) → 12000*3 = 36000
        assert_eq!(r.total_fee, 36_000);
        assert_eq!(r.congestion_multiplier_bps, 30_000);
    }

    // ── Scenario: huge intent ────────────────────────────────

    #[test]
    fn test_scenario_huge_intent() {
        let p = default_params();
        let r = PoSeqFeeCalculator::calculate(&p, 1_000_000, 0, &quiet());
        // base=1000, bandwidth=1M*5=5M, tip=0, multiplier=1.0x → 5_001_000
        assert_eq!(r.total_fee, 5_001_000);
    }
}
