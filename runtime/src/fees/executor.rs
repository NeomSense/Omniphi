//! Fee Executor — Connects Fee Math to State Mutations
//!
//! This is the "last mile" that actually moves money:
//! - Debits the payer's balance for reserved fees
//! - Credits destinations after routing (burn, sequencer, validators, treasury)
//! - Issues refunds for unused reserves
//!
//! Without this module, fees are computed but no balances change.

use crate::fees::accounting::{FeeAccounting, ChargeResult};
use crate::fees::envelope::FeeEnvelope;
use crate::fees::poseq_fee::{PoSeqFeeCalculator, PoSeqFeeParams, PoSeqFeeResult, CongestionState};
use crate::fees::routing::{FeeRouting, FeeRoutingParams, RoutedFees};
use crate::objects::base::ObjectId;
use crate::state::store::ObjectStore;

/// Addresses for fee destinations.
#[derive(Debug, Clone)]
pub struct FeeDestinations {
    /// Sequencer's address (receives sequencer_reward portion).
    pub sequencer: [u8; 32],
    /// Shared security / validator pool address.
    pub shared_security: [u8; 32],
    /// Treasury address.
    pub treasury: [u8; 32],
}

/// Result of executing fee operations against the store.
#[derive(Debug, Clone)]
pub struct FeeExecutionResult {
    /// Whether the fee execution succeeded.
    pub success: bool,
    /// The accounting record.
    pub accounting: FeeAccounting,
    /// The routed fees (if sequencing was charged).
    pub routed: Option<RoutedFees>,
    /// Amount burned (removed from supply).
    pub burned: u64,
    /// Error message if failed.
    pub error: Option<String>,
}

/// The fee executor that operates on real balances.
pub struct FeeExecutor;

impl FeeExecutor {
    /// Reserve fees by locking the payer's balance.
    ///
    /// This is called at intent submission time. It verifies the payer
    /// has sufficient balance and locks the reserved amount.
    pub fn reserve(
        store: &mut ObjectStore,
        envelope: &FeeEnvelope,
        fee_token: &[u8; 32],
    ) -> Result<FeeAccounting, String> {
        let total = envelope.total_reserved();
        if total == 0 {
            return Err("nothing to reserve".into());
        }

        // Check payer has sufficient balance
        let balance = store.find_balance(&envelope.payer, fee_token)
            .ok_or_else(|| "payer balance not found".to_string())?;

        if balance.amount < total as u128 {
            return Err(format!(
                "insufficient balance: need {}, have {}", total, balance.amount
            ));
        }

        // Lock the reserved amount
        let bal = store.find_balance_mut(&envelope.payer, fee_token)
            .ok_or_else(|| "payer balance not found (mut)".to_string())?;
        bal.locked_amount = bal.locked_amount.saturating_add(total as u128);

        Ok(FeeAccounting::reserve(envelope.clone()))
    }

    /// Charge the PoSeq sequencing fee and route it to destinations.
    ///
    /// Called after PoSeq has ordered the intent.
    pub fn charge_sequencing(
        store: &mut ObjectStore,
        accounting: &mut FeeAccounting,
        fee_result: &PoSeqFeeResult,
        routing_params: &FeeRoutingParams,
        destinations: &FeeDestinations,
        fee_token: &[u8; 32],
    ) -> Result<RoutedFees, String> {
        // Charge in accounting
        let charge = accounting.charge_sequencing(fee_result);
        if !charge.success {
            return Err(charge.error.unwrap_or_default());
        }

        // Route the fee
        let routed = FeeRouting::route(fee_result.total_fee, routing_params);

        // Debit payer (move from locked to spent)
        let payer_bal = store.find_balance_mut(&accounting.envelope.payer, fee_token)
            .ok_or_else(|| "payer balance not found".to_string())?;
        payer_bal.locked_amount = payer_bal.locked_amount
            .saturating_sub(fee_result.total_fee as u128);
        payer_bal.amount = payer_bal.amount
            .saturating_sub(fee_result.total_fee as u128);

        // Credit destinations
        Self::credit_if_exists(store, &destinations.sequencer, fee_token, routed.sequencer_reward);
        Self::credit_if_exists(store, &destinations.shared_security, fee_token, routed.shared_security);
        Self::credit_if_exists(store, &destinations.treasury, fee_token, routed.treasury);
        // Burned amount: simply not credited anywhere (removed from supply)

        Ok(routed)
    }

    /// Charge runtime execution fees.
    ///
    /// Called after settlement engine completes execution.
    pub fn charge_runtime(
        store: &mut ObjectStore,
        accounting: &mut FeeAccounting,
        actual_gas_fee: u64,
        fee_token: &[u8; 32],
    ) -> Result<(), String> {
        let charge = accounting.charge_runtime(actual_gas_fee);
        if !charge.success {
            return Err(charge.error.unwrap_or_default());
        }

        // Debit payer
        let payer_bal = store.find_balance_mut(&accounting.envelope.payer, fee_token)
            .ok_or_else(|| "payer balance not found".to_string())?;
        payer_bal.locked_amount = payer_bal.locked_amount
            .saturating_sub(actual_gas_fee as u128);
        payer_bal.amount = payer_bal.amount
            .saturating_sub(actual_gas_fee as u128);

        Ok(())
    }

    /// Issue refund for unused reserved fees.
    ///
    /// Called after both sequencing and runtime charges are complete.
    pub fn settle_refund(
        store: &mut ObjectStore,
        accounting: &mut FeeAccounting,
        fee_token: &[u8; 32],
    ) -> u64 {
        let refund = accounting.settle_refund();

        if refund > 0 {
            // Unlock the remaining reserved amount
            let refund_addr = accounting.envelope.effective_refund_address();
            if let Some(bal) = store.find_balance_mut(&refund_addr, fee_token) {
                bal.locked_amount = bal.locked_amount.saturating_sub(refund as u128);
            }
        }

        refund
    }

    /// Credit a balance if the account exists in the store.
    fn credit_if_exists(
        store: &mut ObjectStore,
        addr: &[u8; 32],
        fee_token: &[u8; 32],
        amount: u64,
    ) {
        if amount == 0 { return; }
        if let Some(bal) = store.find_balance_mut(addr, fee_token) {
            bal.amount = bal.amount.saturating_add(amount as u128);
        }
        // If destination doesn't exist, the amount is effectively burned
        // (same as sending to a nonexistent address)
    }
}

/// EMA-based congestion tracker that carries state across batches.
///
/// Solves the "stateless multiplier" problem where congestion resets
/// to 1.0x after every batch clear.
#[derive(Debug, Clone)]
pub struct CongestionTracker {
    /// EMA of the congestion multiplier in basis points.
    /// Carries forward across batches with smoothing.
    pub ema_multiplier_bps: u64,
    /// Smoothing factor in basis points (e.g., 3000 = 0.3 = 30% weight to new).
    pub alpha_bps: u64,
    /// Minimum EMA value (prevents decay to zero in quiet periods).
    pub ema_floor_bps: u64,
}

impl CongestionTracker {
    pub fn new() -> Self {
        CongestionTracker {
            ema_multiplier_bps: 10_000, // start at 1.0x
            alpha_bps: 3_000,           // 30% weight to new observation
            ema_floor_bps: 10_000,      // never below 1.0x
        }
    }

    /// Update the EMA with a new batch's congestion observation.
    ///
    /// EMA = alpha * new_value + (1 - alpha) * old_ema
    /// All in basis points to avoid floating point.
    pub fn update(&mut self, observed_multiplier_bps: u64) {
        // new_component = alpha * observed / 10_000
        let new_component = (self.alpha_bps as u128)
            .saturating_mul(observed_multiplier_bps as u128)
            / 10_000;

        // old_component = (10_000 - alpha) * ema / 10_000
        let old_component = ((10_000 - self.alpha_bps) as u128)
            .saturating_mul(self.ema_multiplier_bps as u128)
            / 10_000;

        let new_ema = (new_component + old_component) as u64;
        self.ema_multiplier_bps = std::cmp::max(new_ema, self.ema_floor_bps);
    }

    /// Get the current EMA congestion multiplier.
    pub fn current_multiplier_bps(&self) -> u64 {
        self.ema_multiplier_bps
    }

    /// Compute the effective congestion multiplier for a fee calculation.
    /// Uses max(instantaneous, EMA) to prevent gaming via timing.
    pub fn effective_multiplier(
        &self,
        params: &PoSeqFeeParams,
        congestion: &CongestionState,
    ) -> u64 {
        let instant = PoSeqFeeCalculator::compute_congestion_multiplier(params, congestion);
        std::cmp::max(instant, self.ema_multiplier_bps)
    }
}

impl Default for CongestionTracker {
    fn default() -> Self { Self::new() }
}

/// Fee estimation for wallets and dApps.
///
/// Answers "what should I set my fees to?" before submitting.
pub struct FeeEstimator;

impl FeeEstimator {
    /// Estimate the PoSeq sequencing fee for an intent of given size.
    pub fn estimate_poseq_fee(
        params: &PoSeqFeeParams,
        intent_bytes: u64,
        congestion: &CongestionState,
        congestion_tracker: Option<&CongestionTracker>,
    ) -> PoSeqFeeResult {
        // Use EMA-adjusted congestion if tracker available
        let effective_congestion = if let Some(tracker) = congestion_tracker {
            CongestionState {
                pool_intent_count: std::cmp::max(
                    congestion.pool_intent_count,
                    // Inflate count to match EMA pressure
                    (params.target_intents_per_batch as u128
                        * tracker.ema_multiplier_bps as u128
                        / 10_000) as u64,
                ),
                pool_total_bytes: congestion.pool_total_bytes,
            }
        } else {
            congestion.clone()
        };

        PoSeqFeeCalculator::calculate(params, intent_bytes, 0, &effective_congestion)
    }

    /// Suggest a fee envelope for an intent.
    ///
    /// Returns recommended max_poseq_fee and max_runtime_fee with
    /// headroom for price movement.
    pub fn suggest_fee_envelope(
        params: &PoSeqFeeParams,
        intent_bytes: u64,
        estimated_runtime_gas: u64,
        congestion: &CongestionState,
        congestion_tracker: Option<&CongestionTracker>,
        payer: [u8; 32],
        deadline_epoch: u64,
    ) -> FeeEnvelope {
        let estimated_poseq = Self::estimate_poseq_fee(
            params, intent_bytes, congestion, congestion_tracker,
        );

        // Add 50% headroom on PoSeq fee for congestion movement
        let max_poseq = estimated_poseq.total_fee
            .saturating_mul(3) / 2; // 1.5x

        // Add 20% headroom on runtime for gas estimation uncertainty
        let max_runtime = estimated_runtime_gas
            .saturating_mul(6) / 5; // 1.2x

        FeeEnvelope {
            payer,
            fee_token: "omniphi".into(),
            max_poseq_fee: max_poseq,
            max_runtime_fee: max_runtime,
            priority_tip: 0,
            expiry_epoch: deadline_epoch,
            refund_address: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // ── CongestionTracker ────────────────────────────────────

    #[test]
    fn test_ema_starts_at_1x() {
        let tracker = CongestionTracker::new();
        assert_eq!(tracker.current_multiplier_bps(), 10_000);
    }

    #[test]
    fn test_ema_increases_with_high_demand() {
        let mut tracker = CongestionTracker::new();
        // Observe 3.0x congestion
        tracker.update(30_000);
        // EMA = 0.3 * 30000 + 0.7 * 10000 = 9000 + 7000 = 16000
        assert_eq!(tracker.ema_multiplier_bps, 16_000);

        // Another high observation
        tracker.update(30_000);
        // EMA = 0.3 * 30000 + 0.7 * 16000 = 9000 + 11200 = 20200
        assert_eq!(tracker.ema_multiplier_bps, 20_200);
    }

    #[test]
    fn test_ema_decays_after_quiet() {
        let mut tracker = CongestionTracker::new();
        tracker.update(30_000); // spike to 16000
        tracker.update(10_000); // quiet: 0.3*10000 + 0.7*16000 = 3000+11200 = 14200
        assert_eq!(tracker.ema_multiplier_bps, 14_200);
        // Continues decaying
        tracker.update(10_000); // 0.3*10000 + 0.7*14200 = 3000+9940 = 12940
        assert_eq!(tracker.ema_multiplier_bps, 12_940);
    }

    #[test]
    fn test_ema_never_below_floor() {
        let mut tracker = CongestionTracker::new();
        tracker.update(5_000); // below floor
        assert!(tracker.ema_multiplier_bps >= 10_000); // clamped
    }

    #[test]
    fn test_effective_multiplier_uses_max() {
        let mut tracker = CongestionTracker::new();
        tracker.ema_multiplier_bps = 20_000; // EMA at 2.0x

        let params = PoSeqFeeParams::default_params();
        let quiet = CongestionState { pool_intent_count: 10, pool_total_bytes: 50_000 };

        // Instantaneous is 1.0x (quiet), but EMA is 2.0x → uses 2.0x
        let effective = tracker.effective_multiplier(&params, &quiet);
        assert_eq!(effective, 20_000);
    }

    // ── FeeEstimator ─────────────────────────────────────────

    #[test]
    fn test_estimate_poseq_fee() {
        let params = PoSeqFeeParams::default_params();
        let congestion = CongestionState { pool_intent_count: 50, pool_total_bytes: 250_000 };

        let est = FeeEstimator::estimate_poseq_fee(&params, 500, &congestion, None);
        assert!(est.total_fee > 0);
        assert!(est.total_fee >= params.poseq_fee_floor);
    }

    #[test]
    fn test_suggest_fee_envelope() {
        let params = PoSeqFeeParams::default_params();
        let congestion = CongestionState { pool_intent_count: 50, pool_total_bytes: 250_000 };

        let envelope = FeeEstimator::suggest_fee_envelope(
            &params, 500, 10_000, &congestion, None,
            [1u8; 32], 999,
        );

        // Should have headroom above estimated
        let est = FeeEstimator::estimate_poseq_fee(&params, 500, &congestion, None);
        assert!(envelope.max_poseq_fee >= est.total_fee);
        assert!(envelope.max_runtime_fee >= 10_000); // at least the estimate
        assert_eq!(envelope.fee_token, "omniphi");
    }

    #[test]
    fn test_suggest_with_ema_tracker() {
        let params = PoSeqFeeParams::default_params();
        let congestion = CongestionState { pool_intent_count: 50, pool_total_bytes: 250_000 };
        let mut tracker = CongestionTracker::new();
        tracker.ema_multiplier_bps = 20_000; // EMA at 2.0x

        let with_tracker = FeeEstimator::suggest_fee_envelope(
            &params, 500, 10_000, &congestion, Some(&tracker),
            [1u8; 32], 999,
        );
        let without_tracker = FeeEstimator::suggest_fee_envelope(
            &params, 500, 10_000, &congestion, None,
            [1u8; 32], 999,
        );

        // With tracker should suggest higher fees (EMA pressure)
        assert!(with_tracker.max_poseq_fee >= without_tracker.max_poseq_fee);
    }
}
