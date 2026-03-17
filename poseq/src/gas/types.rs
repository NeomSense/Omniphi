//! Core types for the PoSeq fee model.
//!
//! `FeeEnvelope` attaches to every intent, reserving both PoSeq and runtime fee budgets.
//! `PoSeqFeeResult` is the computed fee output from the calculator.

use serde::{Deserialize, Serialize};

// ─── Fee Envelope ──────────────────────────────────────────────────────────

/// Fee envelope attached to an intent submission.
///
/// Users declare maximum fees they're willing to pay for both sequencing
/// and runtime execution. The actual charged amounts will be <= these caps.
///
/// All amounts are in OMNI base units (u128 for precision).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FeeEnvelope {
    /// Address that pays the fee (normally the intent submitter).
    pub payer: [u8; 32],
    /// Maximum PoSeq sequencing fee the user will pay.
    pub max_poseq_fee: u128,
    /// Maximum runtime execution fee the user will pay.
    pub max_runtime_fee: u128,
    /// Optional priority tip — bounded by protocol cap.
    /// Influences ordering within fairness constraints only.
    pub priority_tip: u128,
    /// Block height after which this fee authorization expires.
    pub expiry: u64,
}

impl FeeEnvelope {
    /// Total maximum fee across both surfaces.
    pub fn total_max_fee(&self) -> u128 {
        self.max_poseq_fee.saturating_add(self.max_runtime_fee)
    }

    /// Validate the fee envelope for structural correctness.
    pub fn validate(&self) -> Result<(), FeeEnvelopeError> {
        if self.payer == [0u8; 32] {
            return Err(FeeEnvelopeError::ZeroPayer);
        }
        if self.max_poseq_fee == 0 && self.max_runtime_fee == 0 {
            return Err(FeeEnvelopeError::ZeroTotalFee);
        }
        if self.expiry == 0 {
            return Err(FeeEnvelopeError::ZeroExpiry);
        }
        // Overflow check on total
        if self.max_poseq_fee.checked_add(self.max_runtime_fee).is_none() {
            return Err(FeeEnvelopeError::TotalFeeOverflow);
        }
        Ok(())
    }

    /// Check if this envelope has expired at the given block height.
    pub fn is_expired(&self, current_block: u64) -> bool {
        current_block > self.expiry
    }
}

// ─── Fee Envelope Errors ───────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FeeEnvelopeError {
    ZeroPayer,
    ZeroTotalFee,
    ZeroExpiry,
    TotalFeeOverflow,
    Expired { expiry: u64, current_block: u64 },
    InsufficientPoseqFee { required: u128, available: u128 },
    InsufficientRuntimeFee { required: u128, available: u128 },
    TipExceedsCap { tip: u128, cap: u128 },
}

impl std::fmt::Display for FeeEnvelopeError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::ZeroPayer => write!(f, "fee envelope: zero payer address"),
            Self::ZeroTotalFee => write!(f, "fee envelope: zero total fee"),
            Self::ZeroExpiry => write!(f, "fee envelope: zero expiry"),
            Self::TotalFeeOverflow => write!(f, "fee envelope: total fee overflows u128"),
            Self::Expired { expiry, current_block } =>
                write!(f, "fee envelope expired at block {} (current {})", expiry, current_block),
            Self::InsufficientPoseqFee { required, available } =>
                write!(f, "insufficient poseq fee: need {}, have {}", required, available),
            Self::InsufficientRuntimeFee { required, available } =>
                write!(f, "insufficient runtime fee: need {}, have {}", required, available),
            Self::TipExceedsCap { tip, cap } =>
                write!(f, "priority tip {} exceeds cap {}", tip, cap),
        }
    }
}

impl std::error::Error for FeeEnvelopeError {}

// ─── PoSeq Fee Result ──────────────────────────────────────────────────────

/// Computed PoSeq sequencing fee for an intent.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PoSeqFeeResult {
    /// Base admission fee component.
    pub base_fee: u128,
    /// Byte/bandwidth fee component.
    pub bytes_fee: u128,
    /// Effective priority tip (after cap enforcement).
    pub effective_tip: u128,
    /// Congestion multiplier applied (in basis points, 10000 = 1.0x).
    pub congestion_multiplier_bps: u64,
    /// Final computed sequencing fee (before floor/cap).
    pub computed_fee: u128,
    /// Actual charged fee (after floor and cap enforcement).
    pub charged_fee: u128,
    /// Refund amount (max_poseq_fee - charged_fee).
    pub refund: u128,
}

impl PoSeqFeeResult {
    /// The fee priority score used for ordering within fairness bounds.
    /// This is the effective tip, NOT the total fee — so whale fees don't dominate.
    pub fn priority_score(&self) -> u128 {
        self.effective_tip
    }
}

// ─── Intent Fee Status ─────────────────────────────────────────────────────

/// Tracks the fee lifecycle for a single intent through the pipeline.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum IntentFeeStatus {
    /// Fee envelope received and validated at admission.
    Reserved {
        total_reserved: u128,
    },
    /// PoSeq fee charged at sequencing.
    PoseqCharged {
        poseq_fee: u128,
        runtime_remaining: u128,
    },
    /// Both PoSeq and runtime fees charged after settlement.
    FullyCharged {
        poseq_fee: u128,
        runtime_fee: u128,
        total_charged: u128,
        refund: u128,
    },
    /// Intent expired — partial fee retained as penalty.
    Expired {
        penalty: u128,
        refund: u128,
    },
    /// Intent rejected at admission — no fee charged.
    Rejected,
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn make_envelope() -> FeeEnvelope {
        FeeEnvelope {
            payer: [1u8; 32],
            max_poseq_fee: 10_000,
            max_runtime_fee: 50_000,
            priority_tip: 500,
            expiry: 1000,
        }
    }

    #[test]
    fn test_fee_envelope_valid() {
        let env = make_envelope();
        assert!(env.validate().is_ok());
        assert_eq!(env.total_max_fee(), 60_000);
    }

    #[test]
    fn test_fee_envelope_zero_payer() {
        let mut env = make_envelope();
        env.payer = [0u8; 32];
        assert_eq!(env.validate().unwrap_err(), FeeEnvelopeError::ZeroPayer);
    }

    #[test]
    fn test_fee_envelope_zero_total() {
        let mut env = make_envelope();
        env.max_poseq_fee = 0;
        env.max_runtime_fee = 0;
        assert_eq!(env.validate().unwrap_err(), FeeEnvelopeError::ZeroTotalFee);
    }

    #[test]
    fn test_fee_envelope_zero_expiry() {
        let mut env = make_envelope();
        env.expiry = 0;
        assert_eq!(env.validate().unwrap_err(), FeeEnvelopeError::ZeroExpiry);
    }

    #[test]
    fn test_fee_envelope_overflow() {
        let mut env = make_envelope();
        env.max_poseq_fee = u128::MAX;
        env.max_runtime_fee = 1;
        assert_eq!(env.validate().unwrap_err(), FeeEnvelopeError::TotalFeeOverflow);
    }

    #[test]
    fn test_fee_envelope_expiry() {
        let env = make_envelope();
        assert!(!env.is_expired(999));
        assert!(!env.is_expired(1000));
        assert!(env.is_expired(1001));
    }

    #[test]
    fn test_fee_result_priority_score() {
        let result = PoSeqFeeResult {
            base_fee: 1000,
            bytes_fee: 200,
            effective_tip: 500,
            congestion_multiplier_bps: 10_000,
            computed_fee: 1700,
            charged_fee: 1700,
            refund: 8300,
        };
        assert_eq!(result.priority_score(), 500);
    }
}
