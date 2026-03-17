//! Governance-controlled parameters for PoSeq sequencing fees.
//!
//! All parameters are validated and bounded. Basis-point splits must sum to 10,000.
//! Conservative testnet defaults are provided.

use serde::{Deserialize, Serialize};

/// Governance-controlled PoSeq fee parameters.
///
/// All amounts in OMNI base units unless otherwise noted.
/// All percentages in basis points (10,000 = 100%).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PoSeqFeeParameters {
    // ─── Base fee components ───────────────────────────────────────────

    /// Minimum fee to enter the intent pool (anti-spam floor).
    pub base_admission_fee: u128,

    /// Fee per byte of serialized intent payload.
    pub fee_per_byte: u128,

    // ─── Congestion pricing ────────────────────────────────────────────

    /// Target number of intents per batch for congestion pricing.
    /// When actual count equals target, multiplier = 1.0x (10,000 bps).
    pub target_intents_per_batch: u64,

    /// Minimum congestion multiplier in basis points (floor during low load).
    pub min_congestion_multiplier_bps: u64,

    /// Maximum congestion multiplier in basis points (cap during high load).
    pub max_congestion_multiplier_bps: u64,

    // ─── Priority tip ──────────────────────────────────────────────────

    /// Maximum effective priority tip. Tips above this are clamped.
    /// This prevents whale-dominated ordering while allowing preference signals.
    pub priority_tip_cap: u128,

    // ─── Fee bounds ────────────────────────────────────────────────────

    /// Absolute minimum sequencing fee (hard floor).
    pub poseq_fee_floor: u128,

    /// Absolute maximum sequencing fee (hard cap).
    pub max_poseq_fee_cap: u128,

    // ─── Expiry economics ──────────────────────────────────────────────

    /// Penalty retained when an intent expires without execution,
    /// as basis points of the charged sequencing fee.
    pub expiry_penalty_bps: u64,

    // ─── Fee distribution (must sum to 10,000 bps) ─────────────────────

    /// Sequencer reward share in basis points.
    pub sequencer_reward_bps: u64,

    /// Shared security (validator) pool share in basis points.
    pub shared_security_bps: u64,

    /// Protocol treasury share in basis points.
    pub treasury_bps: u64,

    /// Burn share in basis points.
    pub burn_bps: u64,
}

impl PoSeqFeeParameters {
    /// Conservative testnet defaults.
    ///
    /// base_admission = 1,000 OMNI base units (~0.001 OMNI)
    /// fee_per_byte = 10 units/byte
    /// target = 64 intents/batch
    /// congestion range = [0.8x, 5.0x]
    /// tip cap = 50,000 units
    /// floor = 500 units, cap = 10,000,000 units
    /// distribution: 45% sequencer, 20% security, 15% treasury, 20% burn
    pub fn testnet_defaults() -> Self {
        PoSeqFeeParameters {
            base_admission_fee: 1_000,
            fee_per_byte: 10,
            target_intents_per_batch: 64,
            min_congestion_multiplier_bps: 8_000,   // 0.8x
            max_congestion_multiplier_bps: 50_000,  // 5.0x
            priority_tip_cap: 50_000,
            poseq_fee_floor: 500,
            max_poseq_fee_cap: 10_000_000,
            expiry_penalty_bps: 1_000,  // 10% of sequencing fee
            sequencer_reward_bps: 4_500,
            shared_security_bps: 2_000,
            treasury_bps: 1_500,
            burn_bps: 2_000,
        }
    }

    /// Validate all parameter bounds and invariants.
    pub fn validate(&self) -> Result<(), ParameterValidationError> {
        // Base fee sanity
        if self.base_admission_fee == 0 {
            return Err(ParameterValidationError::ZeroBaseAdmissionFee);
        }

        // Congestion multiplier bounds
        if self.min_congestion_multiplier_bps == 0 {
            return Err(ParameterValidationError::ZeroCongestionMultiplier);
        }
        if self.min_congestion_multiplier_bps > self.max_congestion_multiplier_bps {
            return Err(ParameterValidationError::CongestionMultiplierInverted {
                min: self.min_congestion_multiplier_bps,
                max: self.max_congestion_multiplier_bps,
            });
        }
        // Sanity: max multiplier shouldn't exceed 100x (1,000,000 bps)
        if self.max_congestion_multiplier_bps > 1_000_000 {
            return Err(ParameterValidationError::CongestionMultiplierTooHigh(
                self.max_congestion_multiplier_bps,
            ));
        }

        // Target batch size
        if self.target_intents_per_batch == 0 {
            return Err(ParameterValidationError::ZeroTargetIntentsPerBatch);
        }

        // Fee floor <= cap
        if self.poseq_fee_floor > self.max_poseq_fee_cap {
            return Err(ParameterValidationError::FeeFloorExceedsCap {
                floor: self.poseq_fee_floor,
                cap: self.max_poseq_fee_cap,
            });
        }

        // Expiry penalty bounds
        if self.expiry_penalty_bps > 10_000 {
            return Err(ParameterValidationError::ExpiryPenaltyTooHigh(self.expiry_penalty_bps));
        }

        // Distribution splits must sum to 10,000 bps exactly
        let total_bps = self.sequencer_reward_bps
            .checked_add(self.shared_security_bps)
            .and_then(|s| s.checked_add(self.treasury_bps))
            .and_then(|s| s.checked_add(self.burn_bps));

        match total_bps {
            None => return Err(ParameterValidationError::DistributionBpsOverflow),
            Some(total) if total != 10_000 => {
                return Err(ParameterValidationError::DistributionBpsMismatch {
                    total,
                    expected: 10_000,
                });
            }
            _ => {}
        }

        Ok(())
    }
}

impl Default for PoSeqFeeParameters {
    fn default() -> Self {
        Self::testnet_defaults()
    }
}

// ─── Parameter Validation Errors ───────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ParameterValidationError {
    ZeroBaseAdmissionFee,
    ZeroCongestionMultiplier,
    CongestionMultiplierInverted { min: u64, max: u64 },
    CongestionMultiplierTooHigh(u64),
    ZeroTargetIntentsPerBatch,
    FeeFloorExceedsCap { floor: u128, cap: u128 },
    ExpiryPenaltyTooHigh(u64),
    DistributionBpsOverflow,
    DistributionBpsMismatch { total: u64, expected: u64 },
}

impl std::fmt::Display for ParameterValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::ZeroBaseAdmissionFee => write!(f, "base_admission_fee must be > 0"),
            Self::ZeroCongestionMultiplier => write!(f, "min_congestion_multiplier_bps must be > 0"),
            Self::CongestionMultiplierInverted { min, max } =>
                write!(f, "congestion multiplier inverted: min {} > max {}", min, max),
            Self::CongestionMultiplierTooHigh(v) =>
                write!(f, "max congestion multiplier {} exceeds 100x (1,000,000 bps)", v),
            Self::ZeroTargetIntentsPerBatch => write!(f, "target_intents_per_batch must be > 0"),
            Self::FeeFloorExceedsCap { floor, cap } =>
                write!(f, "poseq_fee_floor {} > max_poseq_fee_cap {}", floor, cap),
            Self::ExpiryPenaltyTooHigh(v) =>
                write!(f, "expiry_penalty_bps {} exceeds 100% (10,000)", v),
            Self::DistributionBpsOverflow => write!(f, "distribution bps overflow"),
            Self::DistributionBpsMismatch { total, expected } =>
                write!(f, "distribution bps sum {} != expected {}", total, expected),
        }
    }
}

impl std::error::Error for ParameterValidationError {}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_testnet_defaults_valid() {
        let params = PoSeqFeeParameters::testnet_defaults();
        assert!(params.validate().is_ok());
    }

    #[test]
    fn test_zero_base_fee_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.base_admission_fee = 0;
        assert_eq!(
            params.validate().unwrap_err(),
            ParameterValidationError::ZeroBaseAdmissionFee,
        );
    }

    #[test]
    fn test_inverted_congestion_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.min_congestion_multiplier_bps = 50_000;
        params.max_congestion_multiplier_bps = 8_000;
        assert!(matches!(
            params.validate().unwrap_err(),
            ParameterValidationError::CongestionMultiplierInverted { .. },
        ));
    }

    #[test]
    fn test_congestion_too_high_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.max_congestion_multiplier_bps = 2_000_000; // 200x
        assert!(matches!(
            params.validate().unwrap_err(),
            ParameterValidationError::CongestionMultiplierTooHigh(_),
        ));
    }

    #[test]
    fn test_floor_exceeds_cap_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.poseq_fee_floor = 100_000_000;
        assert!(matches!(
            params.validate().unwrap_err(),
            ParameterValidationError::FeeFloorExceedsCap { .. },
        ));
    }

    #[test]
    fn test_distribution_mismatch_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.burn_bps = 1_000; // total now 9,000
        assert!(matches!(
            params.validate().unwrap_err(),
            ParameterValidationError::DistributionBpsMismatch { .. },
        ));
    }

    #[test]
    fn test_expiry_penalty_too_high() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.expiry_penalty_bps = 15_000; // 150%
        assert_eq!(
            params.validate().unwrap_err(),
            ParameterValidationError::ExpiryPenaltyTooHigh(15_000),
        );
    }

    #[test]
    fn test_custom_valid_params() {
        let params = PoSeqFeeParameters {
            base_admission_fee: 5_000,
            fee_per_byte: 50,
            target_intents_per_batch: 128,
            min_congestion_multiplier_bps: 5_000,
            max_congestion_multiplier_bps: 100_000,
            priority_tip_cap: 100_000,
            poseq_fee_floor: 2_000,
            max_poseq_fee_cap: 50_000_000,
            expiry_penalty_bps: 2_500,
            sequencer_reward_bps: 5_000,
            shared_security_bps: 2_500,
            treasury_bps: 1_000,
            burn_bps: 1_500,
        };
        assert!(params.validate().is_ok());
    }
}
