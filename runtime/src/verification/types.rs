//! Verification result and error types.

use crate::objects::base::ObjectId;

/// Result of verifying a single intent's settlement.
#[derive(Debug, Clone)]
pub struct VerificationResult {
    pub intent_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub passed: bool,
    pub errors: Vec<VerificationError>,
}

impl VerificationResult {
    pub fn success(intent_id: [u8; 32], solver_id: [u8; 32]) -> Self {
        VerificationResult { intent_id, solver_id, passed: true, errors: vec![] }
    }

    pub fn failure(intent_id: [u8; 32], solver_id: [u8; 32], errors: Vec<VerificationError>) -> Self {
        VerificationResult { intent_id, solver_id, passed: false, errors }
    }
}

/// Specific verification failures.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum VerificationError {
    // Swap
    WrongAssetIn { expected: [u8; 32], got: [u8; 32] },
    WrongAssetOut { expected: [u8; 32], got: [u8; 32] },
    InputAmountMismatch { expected: u128, actual: u128 },
    OutputBelowMinimum { min_required: u128, actual: u128 },
    FeeExceedsMax { max_fee_bps: u64, actual_fee_bps: u64 },
    WrongRecipient { expected: [u8; 32], got: [u8; 32] },
    DeadlineExceeded { deadline: u64, execution_block: u64 },

    // Payment
    TransferAmountMismatch { expected: u128, actual: u128 },
    SenderBalanceInsufficient { required: u128, available: u128 },

    // Route Liquidity
    InvalidPool(ObjectId),
    TooManyHops { max: u8, actual: u8 },
    PriceImpactExceeded { max_bps: u16, actual_bps: u16 },
    ReceivedBelowMinimum { min_received: u128, actual: u128 },

    // General
    NegativeBalance(ObjectId),
    ConservationViolation { asset: [u8; 32], discrepancy: i128 },
    DoubleFill { intent_id: [u8; 32] },
    StateCorruption(String),
    NonceReplay { user: [u8; 32], nonce: u64 },
}

impl std::fmt::Display for VerificationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::OutputBelowMinimum { min_required, actual } => {
                write!(f, "output {} below minimum {}", actual, min_required)
            }
            Self::FeeExceedsMax { max_fee_bps, actual_fee_bps } => {
                write!(f, "fee {} bps exceeds max {} bps", actual_fee_bps, max_fee_bps)
            }
            Self::ConservationViolation { asset, discrepancy } => {
                write!(f, "conservation violation: asset {} discrepancy {}", hex::encode(&asset[..4]), discrepancy)
            }
            Self::DoubleFill { intent_id } => {
                write!(f, "double fill: intent {}", hex::encode(&intent_id[..4]))
            }
            other => write!(f, "{:?}", other),
        }
    }
}

impl std::error::Error for VerificationError {}
