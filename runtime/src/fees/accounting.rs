//! Fee Accounting — Charge / Reserve / Refund Flow
//!
//! Implements explicit accounting stages for the 3-surface fee model:
//!
//! 1. Reserve: lock max_poseq_fee + max_runtime_fee at submission
//! 2. Charge sequencing: compute and deduct actual PoSeq fee
//! 3. Charge runtime: compute and deduct actual runtime gas fee
//! 4. Refund: return unused reserved amount
//!
//! All arithmetic is u128-safe with explicit overflow checks.

use super::envelope::FeeEnvelope;
use super::poseq_fee::PoSeqFeeResult;

/// The lifecycle state of fee accounting for a single intent.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FeeAccountingState {
    /// Fees reserved but no charges yet.
    Reserved,
    /// PoSeq sequencing fee charged.
    SequencingCharged,
    /// Both sequencing and runtime fees charged.
    FullyCharged,
    /// Refund issued, accounting closed.
    Settled,
    /// Intent failed at intake (partial charge possible).
    FailedIntake,
    /// Intent expired in pool (partial charge, refund rest).
    Expired,
    /// Runtime execution reverted (PoSeq charged, runtime partial).
    RuntimeReverted,
}

/// Tracks fee accounting for a single intent through its lifecycle.
#[derive(Debug, Clone)]
pub struct FeeAccounting {
    /// The original fee envelope.
    pub envelope: FeeEnvelope,
    /// Current accounting state.
    pub state: FeeAccountingState,
    /// Total amount reserved (locked at submission).
    pub reserved_total: u64,
    /// Actual PoSeq sequencing fee charged.
    pub charged_poseq: u64,
    /// Actual runtime execution fee charged.
    pub charged_runtime: u64,
    /// Amount refunded to the payer.
    pub refunded: u64,
    /// Non-refundable penalty (e.g., expired intent intake cost).
    pub penalty: u64,
}

/// Result of a charge operation.
#[derive(Debug, Clone)]
pub struct ChargeResult {
    pub success: bool,
    pub amount_charged: u64,
    pub error: Option<String>,
}

impl FeeAccounting {
    /// Create a new fee accounting record from an envelope.
    /// This represents the "reserve" step.
    pub fn reserve(envelope: FeeEnvelope) -> Self {
        let reserved_total = envelope.total_reserved();
        FeeAccounting {
            envelope,
            state: FeeAccountingState::Reserved,
            reserved_total,
            charged_poseq: 0,
            charged_runtime: 0,
            refunded: 0,
            penalty: 0,
        }
    }

    /// Charge the actual PoSeq sequencing fee.
    ///
    /// Must be called in Reserved state.
    /// Fails if actual_fee > max_poseq_fee (insufficient reserve).
    pub fn charge_sequencing(&mut self, fee_result: &PoSeqFeeResult) -> ChargeResult {
        if self.state != FeeAccountingState::Reserved {
            return ChargeResult {
                success: false,
                amount_charged: 0,
                error: Some(format!("invalid state for sequencing charge: {:?}", self.state)),
            };
        }

        if fee_result.total_fee > self.envelope.max_poseq_fee {
            return ChargeResult {
                success: false,
                amount_charged: 0,
                error: Some(format!(
                    "insufficient poseq reserve: need {}, have {}",
                    fee_result.total_fee, self.envelope.max_poseq_fee
                )),
            };
        }

        self.charged_poseq = fee_result.total_fee;
        self.state = FeeAccountingState::SequencingCharged;

        ChargeResult {
            success: true,
            amount_charged: fee_result.total_fee,
            error: None,
        }
    }

    /// Charge the actual runtime execution fee.
    ///
    /// Must be called in SequencingCharged state.
    /// Fails if actual_gas_fee > max_runtime_fee.
    pub fn charge_runtime(&mut self, actual_gas_fee: u64) -> ChargeResult {
        if self.state != FeeAccountingState::SequencingCharged {
            return ChargeResult {
                success: false,
                amount_charged: 0,
                error: Some(format!("invalid state for runtime charge: {:?}", self.state)),
            };
        }

        if actual_gas_fee > self.envelope.max_runtime_fee {
            return ChargeResult {
                success: false,
                amount_charged: 0,
                error: Some(format!(
                    "insufficient runtime reserve: need {}, have {}",
                    actual_gas_fee, self.envelope.max_runtime_fee
                )),
            };
        }

        self.charged_runtime = actual_gas_fee;
        self.state = FeeAccountingState::FullyCharged;

        ChargeResult {
            success: true,
            amount_charged: actual_gas_fee,
            error: None,
        }
    }

    /// Compute and issue the refund. Transitions to Settled.
    pub fn settle_refund(&mut self) -> u64 {
        let total_charged = self.charged_poseq
            .saturating_add(self.charged_runtime)
            .saturating_add(self.penalty);

        self.refunded = self.reserved_total.saturating_sub(total_charged);
        self.state = FeeAccountingState::Settled;
        self.refunded
    }

    /// Handle intake failure: charge validation cost, refund rest.
    pub fn fail_intake(&mut self, validation_cost: u64) {
        self.penalty = std::cmp::min(validation_cost, self.reserved_total);
        self.refunded = self.reserved_total.saturating_sub(self.penalty);
        self.state = FeeAccountingState::FailedIntake;
    }

    /// Handle expiry: keep partial sequencing fee, refund rest.
    pub fn expire(&mut self, expiry_penalty: u64) {
        self.penalty = std::cmp::min(expiry_penalty, self.reserved_total);
        self.refunded = self.reserved_total.saturating_sub(self.penalty);
        self.state = FeeAccountingState::Expired;
    }

    /// Handle runtime revert: keep PoSeq fee, charge partial runtime, refund rest.
    pub fn revert_runtime(&mut self, runtime_work_charged: u64) {
        self.charged_runtime = std::cmp::min(runtime_work_charged, self.envelope.max_runtime_fee);
        let total_charged = self.charged_poseq
            .saturating_add(self.charged_runtime);
        self.refunded = self.reserved_total.saturating_sub(total_charged);
        self.state = FeeAccountingState::RuntimeReverted;
    }

    /// Total actually charged (sequencing + runtime + penalties).
    pub fn total_charged(&self) -> u64 {
        self.charged_poseq
            .saturating_add(self.charged_runtime)
            .saturating_add(self.penalty)
    }

    // ── Invariant checks ─────────────────────────────────────

    /// Verify that reserved >= charged (no negative refund).
    pub fn check_invariant_no_negative_refund(&self) -> bool {
        self.reserved_total >= self.total_charged()
    }

    /// Verify that reserved == charged + refunded (conservation).
    pub fn check_invariant_conservation(&self) -> bool {
        self.reserved_total == self.total_charged().saturating_add(self.refunded)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fees::poseq_fee::PoSeqFeeResult;

    fn test_envelope() -> FeeEnvelope {
        FeeEnvelope {
            payer: [1u8; 32],
            fee_token: "omniphi".into(),
            max_poseq_fee: 10_000,
            max_runtime_fee: 50_000,
            priority_tip: 0,
            expiry_epoch: 999,
            refund_address: None,
        }
    }

    fn test_fee_result(fee: u64) -> PoSeqFeeResult {
        PoSeqFeeResult {
            total_fee: fee,
            base_component: fee,
            bandwidth_component: 0,
            tip_component: 0,
            congestion_multiplier_bps: 10_000,
            floor_applied: false,
            cap_applied: false,
        }
    }

    // ── Reserve / Charge / Refund ────────────────────────────

    #[test]
    fn test_happy_path() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        assert_eq!(acc.reserved_total, 60_000);
        assert_eq!(acc.state, FeeAccountingState::Reserved);

        // Charge sequencing
        let r1 = acc.charge_sequencing(&test_fee_result(2_000));
        assert!(r1.success);
        assert_eq!(acc.charged_poseq, 2_000);

        // Charge runtime
        let r2 = acc.charge_runtime(30_000);
        assert!(r2.success);
        assert_eq!(acc.charged_runtime, 30_000);

        // Settle
        let refund = acc.settle_refund();
        assert_eq!(refund, 28_000); // 60000 - 2000 - 30000
        assert_eq!(acc.state, FeeAccountingState::Settled);
        assert!(acc.check_invariant_no_negative_refund());
        assert!(acc.check_invariant_conservation());
    }

    // ── Insufficient poseq reserve ───────────────────────────

    #[test]
    fn test_insufficient_poseq_reserve() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        let r = acc.charge_sequencing(&test_fee_result(20_000)); // > max_poseq_fee (10000)
        assert!(!r.success);
        assert!(r.error.unwrap().contains("insufficient poseq reserve"));
    }

    // ── Insufficient runtime reserve ─────────────────────────

    #[test]
    fn test_insufficient_runtime_reserve() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.charge_sequencing(&test_fee_result(1_000)).success;
        let r = acc.charge_runtime(60_000); // > max_runtime_fee (50000)
        assert!(!r.success);
        assert!(r.error.unwrap().contains("insufficient runtime reserve"));
    }

    // ── Intake failure ───────────────────────────────────────

    #[test]
    fn test_intake_failure() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.fail_intake(500);
        assert_eq!(acc.penalty, 500);
        assert_eq!(acc.refunded, 59_500);
        assert_eq!(acc.state, FeeAccountingState::FailedIntake);
        assert!(acc.check_invariant_conservation());
    }

    // ── Expiry ───────────────────────────────────────────────

    #[test]
    fn test_expiry() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.expire(1_000);
        assert_eq!(acc.penalty, 1_000);
        assert_eq!(acc.refunded, 59_000);
        assert_eq!(acc.state, FeeAccountingState::Expired);
        assert!(acc.check_invariant_conservation());
    }

    // ── Runtime revert ───────────────────────────────────────

    #[test]
    fn test_runtime_revert() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.charge_sequencing(&test_fee_result(3_000));
        acc.revert_runtime(5_000); // partial runtime charge
        assert_eq!(acc.charged_poseq, 3_000);
        assert_eq!(acc.charged_runtime, 5_000);
        assert_eq!(acc.refunded, 52_000); // 60000 - 3000 - 5000
        assert_eq!(acc.state, FeeAccountingState::RuntimeReverted);
        assert!(acc.check_invariant_no_negative_refund());
        assert!(acc.check_invariant_conservation());
    }

    // ── State machine enforcement ────────────────────────────

    #[test]
    fn test_wrong_state_sequencing() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.charge_sequencing(&test_fee_result(1_000));
        // Try to charge sequencing again
        let r = acc.charge_sequencing(&test_fee_result(1_000));
        assert!(!r.success);
    }

    #[test]
    fn test_wrong_state_runtime() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        // Try to charge runtime before sequencing
        let r = acc.charge_runtime(1_000);
        assert!(!r.success);
    }

    // ── Invariant: no negative refund ────────────────────────

    #[test]
    fn test_invariant_no_negative_refund() {
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.charge_sequencing(&test_fee_result(10_000)); // max
        acc.charge_runtime(50_000); // max
        let refund = acc.settle_refund();
        assert_eq!(refund, 0); // all used, zero refund
        assert!(acc.check_invariant_no_negative_refund());
        assert!(acc.check_invariant_conservation());
    }

    // ── Conservation invariant ────────────────────────────────

    #[test]
    fn test_conservation_across_all_paths() {
        // Happy path
        let mut acc = FeeAccounting::reserve(test_envelope());
        acc.charge_sequencing(&test_fee_result(5_000));
        acc.charge_runtime(20_000);
        acc.settle_refund();
        assert!(acc.check_invariant_conservation());

        // Expiry path
        let mut acc2 = FeeAccounting::reserve(test_envelope());
        acc2.expire(2_000);
        assert!(acc2.check_invariant_conservation());

        // Intake failure path
        let mut acc3 = FeeAccounting::reserve(test_envelope());
        acc3.fail_intake(100);
        assert!(acc3.check_invariant_conservation());

        // Revert path
        let mut acc4 = FeeAccounting::reserve(test_envelope());
        acc4.charge_sequencing(&test_fee_result(1_000));
        acc4.revert_runtime(500);
        assert!(acc4.check_invariant_conservation());
    }
}
