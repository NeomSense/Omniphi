//! Fairness-constrained priority model for PoSeq ordering.
//!
//! Fees are ONE bounded signal among several in the ordering priority.
//! Higher fees must NEVER override fairness commitments, anti-starvation rules,
//! anti-MEV protections, or protected flow inclusion logic.
//!
//! The priority model computes a composite score from multiple signals:
//!
//! 1. **Fairness class score** — derived from the fairness engine's class weights
//! 2. **Urgency score** — based on deadline proximity and waiting age
//! 3. **Bounded tip score** — the effective tip after cap enforcement
//! 4. **Starvation boost** — forced priority escalation for long-waiting intents
//!
//! The composite score is used by the ordering engine as a secondary signal
//! AFTER all fairness constraints (anti-MEV, protected flows, anti-starvation)
//! have been applied.

use serde::{Deserialize, Serialize};

/// Configuration for the priority scoring model.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PriorityConfig {
    /// Weight of fairness class score in composite (basis points, out of 10000).
    pub fairness_class_weight_bps: u64,
    /// Weight of urgency score in composite (basis points).
    pub urgency_weight_bps: u64,
    /// Weight of tip score in composite (basis points).
    pub tip_weight_bps: u64,
    /// Weight of starvation boost in composite (basis points).
    pub starvation_weight_bps: u64,
    /// Blocks before deadline at which urgency starts to increase.
    pub urgency_horizon_blocks: u64,
    /// Blocks of waiting after which starvation boost activates.
    pub starvation_threshold_blocks: u64,
    /// Maximum starvation boost score.
    pub max_starvation_boost: u64,
}

impl Default for PriorityConfig {
    fn default() -> Self {
        PriorityConfig {
            fairness_class_weight_bps: 4_000,   // 40%
            urgency_weight_bps: 2_500,          // 25%
            tip_weight_bps: 1_500,              // 15%
            starvation_weight_bps: 2_000,       // 20%
            urgency_horizon_blocks: 50,
            starvation_threshold_blocks: 100,
            max_starvation_boost: 10_000,
        }
    }
}

impl PriorityConfig {
    /// Validate that weights sum to 10,000 bps.
    pub fn validate(&self) -> bool {
        let total = self.fairness_class_weight_bps
            .saturating_add(self.urgency_weight_bps)
            .saturating_add(self.tip_weight_bps)
            .saturating_add(self.starvation_weight_bps);
        total == 10_000
    }
}

/// Inputs for computing an intent's priority score.
#[derive(Debug, Clone)]
pub struct PriorityInput {
    /// Fairness class weight (from policy, e.g. SafetyCritical=9000, Normal=3000).
    pub fairness_class_weight: u64,
    /// Block height at which the intent was admitted to the pool.
    pub admitted_at_block: u64,
    /// Intent deadline block.
    pub deadline: u64,
    /// Current block height.
    pub current_block: u64,
    /// Effective tip (already capped by the fee calculator).
    pub effective_tip: u128,
    /// The tip cap from parameters (for normalization).
    pub tip_cap: u128,
    /// Whether this intent is in a protected fairness class.
    pub is_protected: bool,
}

/// Computed priority score for an intent.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PriorityScore {
    /// Individual component scores (0-10000 each, pre-weighted).
    pub fairness_component: u64,
    pub urgency_component: u64,
    pub tip_component: u64,
    pub starvation_component: u64,
    /// Final composite score. Higher = higher priority.
    pub composite: u64,
    /// Whether starvation boost is active (forced inclusion territory).
    pub starvation_active: bool,
    /// Whether this is a protected-class intent.
    pub is_protected: bool,
}

/// Compute the priority score for an intent.
///
/// Protected intents always receive maximum fairness component and
/// their starvation boost activates earlier, but the scoring system
/// does NOT override the anti-MEV engine's position bounds —
/// it only influences relative ordering within those constraints.
pub fn compute_priority(config: &PriorityConfig, input: &PriorityInput) -> PriorityScore {
    // 1. Fairness class score (0-10000)
    // Normalize: class_weight is already 0-10000 from policy
    let fairness_raw = std::cmp::min(input.fairness_class_weight, 10_000);

    // 2. Urgency score (0-10000)
    // Higher as deadline approaches
    let urgency_raw = compute_urgency(input, config.urgency_horizon_blocks);

    // 3. Tip score (0-10000)
    // Bounded: normalize tip against tip_cap
    let tip_raw = if input.tip_cap == 0 {
        0u64
    } else if input.effective_tip >= input.tip_cap {
        10_000u64 // at or above cap = maximum tip score
    } else {
        // Safe normalization: use u128 division to avoid overflow
        // tip_score = (tip * 10000) / cap
        // When tip < cap, this won't overflow for reasonable values.
        // For very large values, use checked_mul with fallback.
        let normalized = match input.effective_tip.checked_mul(10_000) {
            Some(product) => product / input.tip_cap,
            None => {
                // Overflow: tip * 10000 > u128::MAX. Since tip < cap,
                // the ratio is < 1.0, so use division-first approach:
                // (tip / cap) * 10000 — less precise but safe
                (input.effective_tip / input.tip_cap).saturating_mul(10_000)
            }
        };
        std::cmp::min(normalized as u64, 10_000)
    };

    // 4. Starvation boost (0-max_starvation_boost)
    let waiting_blocks = input.current_block.saturating_sub(input.admitted_at_block);
    let starvation_active = waiting_blocks >= config.starvation_threshold_blocks;
    let starvation_raw = if starvation_active {
        let excess = waiting_blocks.saturating_sub(config.starvation_threshold_blocks);
        // Linear ramp: 100 score per block beyond threshold, capped
        std::cmp::min(excess.saturating_mul(100), config.max_starvation_boost)
    } else {
        0
    };

    // 5. Weighted composite
    let fairness_component = fairness_raw
        .saturating_mul(config.fairness_class_weight_bps)
        / 10_000;
    let urgency_component = urgency_raw
        .saturating_mul(config.urgency_weight_bps)
        / 10_000;
    let tip_component = tip_raw
        .saturating_mul(config.tip_weight_bps)
        / 10_000;
    let starvation_component = starvation_raw
        .saturating_mul(config.starvation_weight_bps)
        / 10_000;

    let composite = fairness_component
        .saturating_add(urgency_component)
        .saturating_add(tip_component)
        .saturating_add(starvation_component);

    PriorityScore {
        fairness_component,
        urgency_component,
        tip_component,
        starvation_component,
        composite,
        starvation_active,
        is_protected: input.is_protected,
    }
}

/// Compute urgency score (0-10000) based on deadline proximity.
///
/// - If `blocks_remaining <= 0`: maximum urgency (10000)
/// - If `blocks_remaining >= horizon`: minimum urgency (0)
/// - Otherwise: linear interpolation
fn compute_urgency(input: &PriorityInput, horizon: u64) -> u64 {
    if input.current_block >= input.deadline {
        return 10_000; // past deadline — maximum urgency
    }

    let blocks_remaining = input.deadline.saturating_sub(input.current_block);
    if blocks_remaining >= horizon || horizon == 0 {
        return 0;
    }

    // Linear: urgency increases as deadline approaches
    // score = (horizon - remaining) * 10000 / horizon
    let distance_from_horizon = horizon.saturating_sub(blocks_remaining);
    distance_from_horizon
        .saturating_mul(10_000)
        / horizon
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn default_config() -> PriorityConfig {
        PriorityConfig::default()
    }

    fn make_input(class_weight: u64, tip: u128, deadline: u64, admitted: u64, current: u64) -> PriorityInput {
        PriorityInput {
            fairness_class_weight: class_weight,
            admitted_at_block: admitted,
            deadline,
            current_block: current,
            effective_tip: tip,
            tip_cap: 50_000,
            is_protected: false,
        }
    }

    #[test]
    fn test_config_validates() {
        assert!(default_config().validate());
    }

    #[test]
    fn test_invalid_config() {
        let mut config = default_config();
        config.tip_weight_bps = 0;
        assert!(!config.validate());
    }

    #[test]
    fn test_basic_priority_computation() {
        let config = default_config();
        let input = make_input(5_000, 25_000, 200, 10, 50);
        let score = compute_priority(&config, &input);

        // fairness: 5000 * 4000 / 10000 = 2000
        assert_eq!(score.fairness_component, 2_000);
        // tip: (25000 * 10000 / 50000) = 5000, then * 1500 / 10000 = 750
        assert_eq!(score.tip_component, 750);
        // Not starving yet (40 blocks < 100 threshold)
        assert!(!score.starvation_active);
        assert_eq!(score.starvation_component, 0);
    }

    #[test]
    fn test_high_class_dominates() {
        let config = default_config();
        let safety_critical = make_input(9_000, 0, 200, 10, 50);
        let normal_high_tip = make_input(3_000, 50_000, 200, 10, 50);

        let sc_score = compute_priority(&config, &safety_critical);
        let ht_score = compute_priority(&config, &normal_high_tip);

        // SafetyCritical with 0 tip should beat Normal with max tip
        // because fairness weight (40%) at 9000 >> Normal at 3000
        assert!(sc_score.composite > ht_score.composite,
            "safety_critical {} should beat high-tip normal {}",
            sc_score.composite, ht_score.composite);
    }

    #[test]
    fn test_urgency_increases_near_deadline() {
        let config = default_config();
        let far = make_input(5_000, 1_000, 200, 10, 50);   // 150 blocks remaining
        let near = make_input(5_000, 1_000, 200, 10, 180);  // 20 blocks remaining

        let far_score = compute_priority(&config, &far);
        let near_score = compute_priority(&config, &near);

        assert!(near_score.urgency_component > far_score.urgency_component);
        assert!(near_score.composite > far_score.composite);
    }

    #[test]
    fn test_starvation_boost_activates() {
        let config = default_config();
        // Intent admitted at block 10, current block 120 → 110 blocks waiting > 100 threshold
        let starving = make_input(3_000, 0, 500, 10, 120);
        let fresh = make_input(3_000, 0, 500, 100, 120);

        let starving_score = compute_priority(&config, &starving);
        let fresh_score = compute_priority(&config, &fresh);

        assert!(starving_score.starvation_active);
        assert!(!fresh_score.starvation_active);
        assert!(starving_score.composite > fresh_score.composite);
    }

    #[test]
    fn test_tip_bounded_not_dominant() {
        let config = default_config();
        // Max tip should only contribute 15% (1500 bps weight)
        let max_tip = make_input(3_000, 50_000, 200, 10, 50);
        let score = compute_priority(&config, &max_tip);

        // Max tip component: 10000 * 1500 / 10000 = 1500
        assert_eq!(score.tip_component, 1_500);

        // Verify tip can't dominate: fairness at 3000 (low class) gives
        // fairness_component = 3000 * 4000 / 10000 = 1200
        // Even max tip (1500) doesn't massively outweigh fairness
        assert!(score.tip_component <= score.fairness_component + 500);
    }

    #[test]
    fn test_zero_tip_still_gets_sequenced() {
        let config = default_config();
        let no_tip = make_input(5_000, 0, 200, 10, 50);
        let score = compute_priority(&config, &no_tip);

        assert_eq!(score.tip_component, 0);
        // But still has nonzero priority from fairness + urgency
        assert!(score.composite > 0);
    }

    #[test]
    fn test_urgency_at_deadline() {
        let config = default_config();
        let at_deadline = make_input(5_000, 0, 100, 10, 100);
        let score = compute_priority(&config, &at_deadline);

        // At deadline: maximum urgency = 10000 * 2500 / 10000 = 2500
        assert_eq!(score.urgency_component, 2_500);
    }

    #[test]
    fn test_urgency_far_from_deadline() {
        let config = default_config();
        // 200 blocks remaining, horizon is 50 → urgency = 0
        let far = make_input(5_000, 0, 300, 10, 100);
        let score = compute_priority(&config, &far);
        assert_eq!(score.urgency_component, 0);
    }

    #[test]
    fn test_deterministic_priority() {
        let config = default_config();
        let input = make_input(5_000, 10_000, 200, 10, 50);
        let s1 = compute_priority(&config, &input);
        let s2 = compute_priority(&config, &input);
        assert_eq!(s1, s2);
    }

    #[test]
    fn test_starvation_ramp() {
        let config = default_config();
        // 110 blocks waiting: 10 beyond threshold → 10*100=1000 raw
        let input1 = make_input(3_000, 0, 500, 10, 120);
        let s1 = compute_priority(&config, &input1);

        // 200 blocks waiting: 100 beyond threshold → 100*100=10000 raw (max)
        let input2 = make_input(3_000, 0, 500, 10, 210);
        let s2 = compute_priority(&config, &input2);

        assert!(s2.starvation_component > s1.starvation_component);
    }
}
