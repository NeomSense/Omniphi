//! PoSeq fee calculator — deterministic sequencing fee computation.
//!
//! Computes the sequencing fee from a fee envelope, current congestion state,
//! and protocol parameters. All arithmetic is integer-only (no floating point).
//!
//! Formula:
//! ```text
//! effective_tip = min(priority_tip, priority_tip_cap)
//! raw_fee = (base_admission_fee + bytes_fee + effective_tip) * congestion_multiplier / 10000
//! charged_fee = clamp(raw_fee, poseq_fee_floor, max_poseq_fee_cap)
//! charged_fee = min(charged_fee, max_poseq_fee)
//! refund = max_poseq_fee - charged_fee
//! ```

use super::parameters::PoSeqFeeParameters;
use super::types::{FeeEnvelope, FeeEnvelopeError, PoSeqFeeResult};

/// Current congestion state for fee calculation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CongestionState {
    /// Number of intents currently in the pool or pending for this batch.
    pub current_intent_count: u64,
    /// Number of intents in the most recent completed batch (for EMA smoothing).
    pub last_batch_intent_count: u64,
}

impl CongestionState {
    pub fn new(current: u64, last_batch: u64) -> Self {
        CongestionState {
            current_intent_count: current,
            last_batch_intent_count: last_batch,
        }
    }
}

/// The PoSeq fee calculator. Stateless — all inputs are explicit.
pub struct PoSeqFeeCalculator;

impl PoSeqFeeCalculator {
    /// Compute the PoSeq sequencing fee for an intent.
    ///
    /// Arguments:
    /// - `params`: governance-controlled fee parameters
    /// - `envelope`: the user's fee envelope
    /// - `intent_size_bytes`: serialized size of the intent payload
    /// - `congestion`: current pool/batch congestion state
    /// - `current_block`: current block height (for expiry check)
    ///
    /// Returns `PoSeqFeeResult` on success, or `FeeEnvelopeError` if the
    /// envelope is invalid or the user's budget is insufficient.
    pub fn compute(
        params: &PoSeqFeeParameters,
        envelope: &FeeEnvelope,
        intent_size_bytes: u64,
        congestion: &CongestionState,
        current_block: u64,
    ) -> Result<PoSeqFeeResult, FeeEnvelopeError> {
        // 1. Validate envelope
        envelope.validate()?;
        if envelope.is_expired(current_block) {
            return Err(FeeEnvelopeError::Expired {
                expiry: envelope.expiry,
                current_block,
            });
        }

        // 2. Compute base components
        let base_fee = params.base_admission_fee;
        let bytes_fee = (intent_size_bytes as u128).saturating_mul(params.fee_per_byte);

        // 3. Cap the priority tip
        let effective_tip = std::cmp::min(envelope.priority_tip, params.priority_tip_cap);

        // 4. Compute congestion multiplier (in basis points, 10000 = 1.0x)
        let congestion_multiplier_bps = Self::compute_congestion_multiplier(params, congestion);

        // 5. Compute raw fee:
        //    raw = (base + bytes + tip) * multiplier / 10000
        let pre_multiplier = base_fee
            .saturating_add(bytes_fee)
            .saturating_add(effective_tip);

        let computed_fee = pre_multiplier
            .saturating_mul(congestion_multiplier_bps as u128)
            / 10_000;

        // 6. Apply floor and cap
        let mut charged_fee = computed_fee;
        if charged_fee < params.poseq_fee_floor {
            charged_fee = params.poseq_fee_floor;
        }
        if charged_fee > params.max_poseq_fee_cap {
            charged_fee = params.max_poseq_fee_cap;
        }

        // 7. Cannot exceed user's declared max
        if charged_fee > envelope.max_poseq_fee {
            return Err(FeeEnvelopeError::InsufficientPoseqFee {
                required: charged_fee,
                available: envelope.max_poseq_fee,
            });
        }

        // 8. Compute refund
        let refund = envelope.max_poseq_fee.saturating_sub(charged_fee);

        Ok(PoSeqFeeResult {
            base_fee,
            bytes_fee,
            effective_tip,
            congestion_multiplier_bps,
            computed_fee,
            charged_fee,
            refund,
        })
    }

    /// Compute the congestion multiplier in basis points.
    ///
    /// `multiplier_bps = clamp(current / target * 10000, min_bps, max_bps)`
    ///
    /// Uses integer arithmetic: `(current * 10000) / target`.
    pub fn compute_congestion_multiplier(
        params: &PoSeqFeeParameters,
        congestion: &CongestionState,
    ) -> u64 {
        let target = params.target_intents_per_batch;
        if target == 0 {
            return params.max_congestion_multiplier_bps;
        }

        // Use the higher of current pool count and last batch count
        // to avoid oscillation between batches
        let effective_count = std::cmp::max(
            congestion.current_intent_count,
            congestion.last_batch_intent_count,
        );

        // Integer division: (count * 10000) / target
        // When count == target, result = 10000 (1.0x)
        let raw_bps = effective_count
            .saturating_mul(10_000)
            .checked_div(target)
            .unwrap_or(params.max_congestion_multiplier_bps);

        // Clamp to [min, max]
        std::cmp::min(
            std::cmp::max(raw_bps, params.min_congestion_multiplier_bps),
            params.max_congestion_multiplier_bps,
        )
    }

    /// Compute the expiry penalty when an intent expires without execution.
    ///
    /// `penalty = charged_fee * expiry_penalty_bps / 10000`
    pub fn compute_expiry_penalty(params: &PoSeqFeeParameters, charged_fee: u128) -> u128 {
        charged_fee
            .saturating_mul(params.expiry_penalty_bps as u128)
            / 10_000
    }

    /// Quick check: can this envelope afford the minimum possible fee?
    pub fn can_afford_minimum(params: &PoSeqFeeParameters, envelope: &FeeEnvelope) -> bool {
        envelope.max_poseq_fee >= params.poseq_fee_floor
    }
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn default_params() -> PoSeqFeeParameters {
        PoSeqFeeParameters::testnet_defaults()
    }

    fn make_envelope(max_poseq: u128, tip: u128) -> FeeEnvelope {
        FeeEnvelope {
            payer: [1u8; 32],
            max_poseq_fee: max_poseq,
            max_runtime_fee: 50_000,
            priority_tip: tip,
            expiry: 1000,
        }
    }

    fn low_congestion() -> CongestionState {
        CongestionState::new(10, 10) // well below target of 64
    }

    fn at_target_congestion() -> CongestionState {
        CongestionState::new(64, 64) // exactly at target
    }

    fn high_congestion() -> CongestionState {
        CongestionState::new(320, 320) // 5x target
    }

    // ─── Congestion multiplier tests ───────────────────────────────────

    #[test]
    fn test_congestion_at_target() {
        let params = default_params();
        let cong = at_target_congestion();
        let bps = PoSeqFeeCalculator::compute_congestion_multiplier(&params, &cong);
        assert_eq!(bps, 10_000); // 1.0x
    }

    #[test]
    fn test_congestion_low_clamped() {
        let params = default_params();
        let cong = CongestionState::new(1, 1); // 1/64 = 156 bps, below min 8000
        let bps = PoSeqFeeCalculator::compute_congestion_multiplier(&params, &cong);
        assert_eq!(bps, 8_000); // clamped to floor 0.8x
    }

    #[test]
    fn test_congestion_high_clamped() {
        let params = default_params();
        let cong = CongestionState::new(1000, 1000); // 1000/64 = 156250 bps, above max 50000
        let bps = PoSeqFeeCalculator::compute_congestion_multiplier(&params, &cong);
        assert_eq!(bps, 50_000); // clamped to cap 5.0x
    }

    #[test]
    fn test_congestion_double_target() {
        let params = default_params();
        let cong = CongestionState::new(128, 128); // 2x target
        let bps = PoSeqFeeCalculator::compute_congestion_multiplier(&params, &cong);
        assert_eq!(bps, 20_000); // 2.0x
    }

    #[test]
    fn test_congestion_uses_higher_of_current_and_last() {
        let params = default_params();
        // current=10 but last batch was 128
        let cong = CongestionState::new(10, 128);
        let bps = PoSeqFeeCalculator::compute_congestion_multiplier(&params, &cong);
        assert_eq!(bps, 20_000); // uses 128 (last batch), not 10
    }

    // ─── Fee computation tests ─────────────────────────────────────────

    #[test]
    fn test_compute_at_target_no_tip() {
        let params = default_params();
        let env = make_envelope(100_000, 0);
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 200, &cong, 500).unwrap();

        // base=1000, bytes=200*10=2000, tip=0, multiplier=1.0x
        // raw = (1000 + 2000 + 0) * 10000 / 10000 = 3000
        assert_eq!(result.base_fee, 1_000);
        assert_eq!(result.bytes_fee, 2_000);
        assert_eq!(result.effective_tip, 0);
        assert_eq!(result.congestion_multiplier_bps, 10_000);
        assert_eq!(result.computed_fee, 3_000);
        assert_eq!(result.charged_fee, 3_000);
        assert_eq!(result.refund, 97_000);
    }

    #[test]
    fn test_compute_with_tip() {
        let params = default_params();
        let env = make_envelope(100_000, 5_000);
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 200, &cong, 500).unwrap();

        // base=1000, bytes=2000, tip=5000 (within cap of 50000), multiplier=1.0x
        // raw = (1000 + 2000 + 5000) * 10000 / 10000 = 8000
        assert_eq!(result.effective_tip, 5_000);
        assert_eq!(result.charged_fee, 8_000);
    }

    #[test]
    fn test_tip_capped() {
        let params = default_params();
        let env = make_envelope(1_000_000, 100_000); // tip exceeds cap of 50,000
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 100, &cong, 500).unwrap();

        assert_eq!(result.effective_tip, 50_000); // capped
    }

    #[test]
    fn test_fee_floor_enforced() {
        let params = default_params();
        // Very small intent with no tip and low congestion
        let env = make_envelope(100_000, 0);
        let cong = low_congestion(); // multiplier = 0.8x (floor)
        let result = PoSeqFeeCalculator::compute(&params, &env, 1, &cong, 500).unwrap();

        // base=1000, bytes=10, tip=0, multiplier=0.8x
        // raw = (1000 + 10) * 8000 / 10000 = 808
        // floor = 500, raw 808 > 500 so no floor enforcement here
        assert_eq!(result.computed_fee, 808);
        assert_eq!(result.charged_fee, 808);
    }

    #[test]
    fn test_fee_floor_kicks_in() {
        // Create params with a high floor to test enforcement
        let mut params = default_params();
        params.poseq_fee_floor = 5_000;
        let env = make_envelope(100_000, 0);
        let cong = low_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 1, &cong, 500).unwrap();

        // raw = (1000 + 10) * 8000 / 10000 = 808 < 5000 floor
        assert_eq!(result.computed_fee, 808);
        assert_eq!(result.charged_fee, 5_000); // floor enforced
    }

    #[test]
    fn test_fee_cap_enforced() {
        let mut params = default_params();
        params.max_poseq_fee_cap = 2_000;
        let env = make_envelope(100_000, 10_000);
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 100, &cong, 500).unwrap();

        // raw would be high, but capped at 2000
        assert_eq!(result.charged_fee, 2_000);
    }

    #[test]
    fn test_insufficient_poseq_fee() {
        let params = default_params();
        let env = make_envelope(100, 0); // only 100 available, floor is 500
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 200, &cong, 500);

        assert!(matches!(result, Err(FeeEnvelopeError::InsufficientPoseqFee { .. })));
    }

    #[test]
    fn test_expired_envelope_rejected() {
        let params = default_params();
        let env = make_envelope(100_000, 0);
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 200, &cong, 1001); // past expiry

        assert!(matches!(result, Err(FeeEnvelopeError::Expired { .. })));
    }

    #[test]
    fn test_high_congestion_increases_fee() {
        let params = default_params();
        let env = make_envelope(1_000_000, 0);

        let normal_result = PoSeqFeeCalculator::compute(
            &params, &env, 200, &at_target_congestion(), 500,
        ).unwrap();

        let high_result = PoSeqFeeCalculator::compute(
            &params, &env, 200, &high_congestion(), 500,
        ).unwrap();

        assert!(high_result.charged_fee > normal_result.charged_fee);
    }

    #[test]
    fn test_expiry_penalty_computation() {
        let params = default_params();
        let penalty = PoSeqFeeCalculator::compute_expiry_penalty(&params, 10_000);
        // 10000 * 1000 / 10000 = 1000
        assert_eq!(penalty, 1_000);
    }

    #[test]
    fn test_can_afford_minimum() {
        let params = default_params();
        let rich = make_envelope(100_000, 0);
        assert!(PoSeqFeeCalculator::can_afford_minimum(&params, &rich));

        let poor = make_envelope(100, 0); // below floor of 500
        assert!(!PoSeqFeeCalculator::can_afford_minimum(&params, &poor));
    }

    #[test]
    fn test_zero_size_intent() {
        let params = default_params();
        let env = make_envelope(100_000, 0);
        let cong = at_target_congestion();
        let result = PoSeqFeeCalculator::compute(&params, &env, 0, &cong, 500).unwrap();

        // base=1000, bytes=0, tip=0, multiplier=1.0x
        assert_eq!(result.bytes_fee, 0);
        assert_eq!(result.charged_fee, 1_000);
    }

    #[test]
    fn test_determinism() {
        let params = default_params();
        let env = make_envelope(100_000, 3_000);
        let cong = CongestionState::new(80, 80);

        let r1 = PoSeqFeeCalculator::compute(&params, &env, 250, &cong, 500).unwrap();
        let r2 = PoSeqFeeCalculator::compute(&params, &env, 250, &cong, 500).unwrap();

        assert_eq!(r1, r2);
    }

    #[test]
    fn test_large_intent_high_bytes_fee() {
        let params = default_params();
        let env = make_envelope(1_000_000, 0);
        let cong = at_target_congestion();
        // 4096 bytes (MAX_INTENT_SIZE)
        let result = PoSeqFeeCalculator::compute(&params, &env, 4096, &cong, 500).unwrap();

        // bytes = 4096 * 10 = 40960
        assert_eq!(result.bytes_fee, 40_960);
        // total = (1000 + 40960) * 1.0 = 41960
        assert_eq!(result.charged_fee, 41_960);
    }
}
