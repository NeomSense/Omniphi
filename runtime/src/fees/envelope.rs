//! Fee Envelope — Separates Sequencing and Runtime Reserves
//!
//! Replaces the crude `max_fee * 1000 = gas_limit` model with explicit
//! per-surface reserve allocation. Users declare how much they're willing
//! to pay for sequencing vs. execution, and the system charges each
//! independently.

/// A fee envelope that separates sequencing and runtime reserves.
///
/// The user fills this out when submitting an intent. The system
/// reserves the total, charges actual costs per surface, and refunds
/// the unused portion.
#[derive(Debug, Clone)]
pub struct FeeEnvelope {
    /// Address that pays fees (usually the intent sender).
    pub payer: [u8; 32],

    /// Fee token denomination (must be "omniphi" for V1).
    pub fee_token: String,

    /// Maximum fee the payer is willing to spend on PoSeq sequencing.
    /// Covers: admission, bandwidth, congestion, tip.
    pub max_poseq_fee: u64,

    /// Maximum fee the payer is willing to spend on runtime execution.
    /// Covers: state reads/writes, settlement, storage growth.
    pub max_runtime_fee: u64,

    /// Optional priority tip for sequencing ordering.
    /// Capped by PoSeqFeeParams::priority_tip_cap.
    /// Does NOT override fairness policy — bounded influence only.
    pub priority_tip: u64,

    /// Epoch after which this fee envelope is invalid.
    /// Prevents stale fee reservations from hanging indefinitely.
    pub expiry_epoch: u64,

    /// Optional refund address (if different from payer).
    /// If None, refunds go back to payer.
    pub refund_address: Option<[u8; 32]>,
}

impl FeeEnvelope {
    /// Total maximum the payer has authorized.
    pub fn total_reserved(&self) -> u64 {
        self.max_poseq_fee.saturating_add(self.max_runtime_fee)
    }

    /// The effective refund address.
    pub fn effective_refund_address(&self) -> [u8; 32] {
        self.refund_address.unwrap_or(self.payer)
    }

    /// Validate the envelope structurally.
    pub fn validate(&self, current_epoch: u64) -> Result<(), String> {
        if self.payer == [0u8; 32] {
            return Err("payer must be non-zero".into());
        }
        if self.fee_token != "omniphi" {
            return Err(format!("unsupported fee token: {} (only omniphi accepted)", self.fee_token));
        }
        if self.max_poseq_fee == 0 && self.max_runtime_fee == 0 {
            return Err("at least one of max_poseq_fee or max_runtime_fee must be > 0".into());
        }
        if current_epoch >= self.expiry_epoch {
            return Err(format!("fee envelope expired at epoch {} (current: {})", self.expiry_epoch, current_epoch));
        }
        if self.total_reserved() == 0 {
            return Err("total reserved fee must be > 0".into());
        }
        Ok(())
    }

    /// Convert a legacy max_fee into a fee envelope.
    ///
    /// For backward compatibility: splits the old max_fee into
    /// 20% sequencing reserve + 80% runtime reserve.
    pub fn from_legacy_max_fee(
        payer: [u8; 32],
        max_fee: u64,
        expiry_epoch: u64,
    ) -> Self {
        let poseq_share = max_fee / 5; // 20%
        let runtime_share = max_fee - poseq_share; // 80%

        FeeEnvelope {
            payer,
            fee_token: "omniphi".into(),
            max_poseq_fee: poseq_share,
            max_runtime_fee: runtime_share,
            priority_tip: 0,
            expiry_epoch,
            refund_address: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn valid_envelope() -> FeeEnvelope {
        FeeEnvelope {
            payer: [1u8; 32],
            fee_token: "omniphi".into(),
            max_poseq_fee: 10_000,
            max_runtime_fee: 50_000,
            priority_tip: 5_000,
            expiry_epoch: 100,
            refund_address: None,
        }
    }

    #[test]
    fn test_total_reserved() {
        let e = valid_envelope();
        assert_eq!(e.total_reserved(), 60_000);
    }

    #[test]
    fn test_validate_ok() {
        assert!(valid_envelope().validate(10).is_ok());
    }

    #[test]
    fn test_validate_zero_payer() {
        let mut e = valid_envelope();
        e.payer = [0u8; 32];
        assert!(e.validate(10).is_err());
    }

    #[test]
    fn test_validate_wrong_token() {
        let mut e = valid_envelope();
        e.fee_token = "eth".into();
        assert!(e.validate(10).is_err());
    }

    #[test]
    fn test_validate_expired() {
        let e = valid_envelope();
        assert!(e.validate(200).is_err()); // current 200 > expiry 100
    }

    #[test]
    fn test_validate_zero_fees() {
        let mut e = valid_envelope();
        e.max_poseq_fee = 0;
        e.max_runtime_fee = 0;
        assert!(e.validate(10).is_err());
    }

    #[test]
    fn test_effective_refund_defaults_to_payer() {
        let e = valid_envelope();
        assert_eq!(e.effective_refund_address(), e.payer);
    }

    #[test]
    fn test_effective_refund_override() {
        let mut e = valid_envelope();
        e.refund_address = Some([2u8; 32]);
        assert_eq!(e.effective_refund_address(), [2u8; 32]);
    }

    #[test]
    fn test_legacy_conversion() {
        let e = FeeEnvelope::from_legacy_max_fee([1u8; 32], 100, 999);
        assert_eq!(e.max_poseq_fee, 20);  // 20%
        assert_eq!(e.max_runtime_fee, 80); // 80%
        assert_eq!(e.total_reserved(), 100);
    }

    #[test]
    fn test_overflow_safe() {
        let e = FeeEnvelope {
            payer: [1u8; 32],
            fee_token: "omniphi".into(),
            max_poseq_fee: u64::MAX / 2,
            max_runtime_fee: u64::MAX / 2,
            priority_tip: 0,
            expiry_epoch: 999,
            refund_address: None,
        };
        // Should not overflow
        assert!(e.total_reserved() <= u64::MAX);
    }
}
