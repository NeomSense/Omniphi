//! Fee accounting and observability — tracks cumulative PoSeq fee metrics.
//!
//! Provides structured metrics for dashboards, economic analysis, and
//! protocol health monitoring.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use super::routing::PoSeqFeeDistribution;
use super::types::IntentFeeStatus;

/// Cumulative PoSeq fee accounting for a single epoch or batch window.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PoSeqFeeAccounting {
    /// Total PoSeq fees collected in this period.
    pub total_fees_collected: u128,
    /// Total runtime fees collected (tracked for unified reporting).
    pub total_runtime_fees_collected: u128,
    /// Total refunds issued (unused PoSeq fee allocations).
    pub total_refunds: u128,
    /// Total expiry penalties collected.
    pub total_expiry_penalties: u128,
    /// Total burned (from fee distribution).
    pub total_burned: u128,
    /// Total distributed to sequencer rewards.
    pub total_sequencer_rewards: u128,
    /// Total distributed to shared security.
    pub total_shared_security: u128,
    /// Total distributed to treasury.
    pub total_treasury: u128,

    /// Number of intents charged.
    pub intents_charged: u64,
    /// Number of intents expired with penalty.
    pub intents_expired_with_penalty: u64,
    /// Number of intents rejected (no fee charged).
    pub intents_rejected: u64,

    /// Congestion multiplier history (for monitoring).
    /// Each entry: (block_height, multiplier_bps).
    pub congestion_samples: Vec<(u64, u64)>,

    /// Fee distribution per batch (for auditing).
    pub batch_distributions: Vec<PoSeqFeeDistribution>,

    /// Priority tip usage statistics.
    pub tip_stats: TipStats,

    /// Per-intent-type fee breakdown.
    pub fees_by_intent_type: BTreeMap<String, u128>,
}

/// Statistics about priority tip usage.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TipStats {
    /// Number of intents with nonzero tip.
    pub intents_with_tip: u64,
    /// Number of intents with zero tip.
    pub intents_without_tip: u64,
    /// Sum of all effective tips.
    pub total_effective_tips: u128,
    /// Maximum effective tip seen.
    pub max_effective_tip: u128,
    /// Number of tips that were capped.
    pub tips_capped: u64,
}

impl Default for PoSeqFeeAccounting {
    fn default() -> Self {
        Self::new()
    }
}

impl PoSeqFeeAccounting {
    pub fn new() -> Self {
        PoSeqFeeAccounting {
            total_fees_collected: 0,
            total_runtime_fees_collected: 0,
            total_refunds: 0,
            total_expiry_penalties: 0,
            total_burned: 0,
            total_sequencer_rewards: 0,
            total_shared_security: 0,
            total_treasury: 0,
            intents_charged: 0,
            intents_expired_with_penalty: 0,
            intents_rejected: 0,
            congestion_samples: Vec::new(),
            batch_distributions: Vec::new(),
            tip_stats: TipStats::default(),
            fees_by_intent_type: BTreeMap::new(),
        }
    }

    /// Record a PoSeq fee charge for a sequenced intent.
    pub fn record_poseq_charge(
        &mut self,
        charged_fee: u128,
        refund: u128,
        effective_tip: u128,
        tip_was_capped: bool,
        intent_type: &str,
    ) {
        self.total_fees_collected = self.total_fees_collected.saturating_add(charged_fee);
        self.total_refunds = self.total_refunds.saturating_add(refund);
        self.intents_charged += 1;

        // Tip stats
        if effective_tip > 0 {
            self.tip_stats.intents_with_tip += 1;
            self.tip_stats.total_effective_tips = self.tip_stats
                .total_effective_tips
                .saturating_add(effective_tip);
            if effective_tip > self.tip_stats.max_effective_tip {
                self.tip_stats.max_effective_tip = effective_tip;
            }
        } else {
            self.tip_stats.intents_without_tip += 1;
        }
        if tip_was_capped {
            self.tip_stats.tips_capped += 1;
        }

        // Per-type tracking
        *self.fees_by_intent_type.entry(intent_type.to_string()).or_insert(0) += charged_fee;
    }

    /// Record a runtime fee charge (for unified reporting).
    pub fn record_runtime_charge(&mut self, runtime_fee: u128) {
        self.total_runtime_fees_collected = self.total_runtime_fees_collected
            .saturating_add(runtime_fee);
    }

    /// Record an expiry penalty.
    pub fn record_expiry_penalty(&mut self, penalty: u128) {
        self.total_expiry_penalties = self.total_expiry_penalties.saturating_add(penalty);
        self.intents_expired_with_penalty += 1;
    }

    /// Record an admission rejection (no fee charged).
    pub fn record_rejection(&mut self) {
        self.intents_rejected += 1;
    }

    /// Record a fee distribution result.
    pub fn record_distribution(&mut self, dist: &PoSeqFeeDistribution) {
        self.total_sequencer_rewards = self.total_sequencer_rewards
            .saturating_add(dist.sequencer_rewards);
        self.total_shared_security = self.total_shared_security
            .saturating_add(dist.shared_security);
        self.total_treasury = self.total_treasury
            .saturating_add(dist.treasury);
        self.total_burned = self.total_burned
            .saturating_add(dist.burn);
        self.batch_distributions.push(dist.clone());
    }

    /// Record a congestion multiplier sample.
    pub fn record_congestion_sample(&mut self, block: u64, multiplier_bps: u64) {
        self.congestion_samples.push((block, multiplier_bps));
        // Keep at most 1000 samples to bound memory
        if self.congestion_samples.len() > 1000 {
            self.congestion_samples.remove(0);
        }
    }

    /// Compute the average sequencing fee for the period.
    pub fn average_poseq_fee(&self) -> u128 {
        if self.intents_charged == 0 {
            0
        } else {
            self.total_fees_collected / self.intents_charged as u128
        }
    }

    /// Compute the average congestion multiplier (bps) from samples.
    pub fn average_congestion_bps(&self) -> u64 {
        if self.congestion_samples.is_empty() {
            10_000 // default 1.0x
        } else {
            let sum: u64 = self.congestion_samples.iter().map(|(_, bps)| bps).sum();
            sum / self.congestion_samples.len() as u64
        }
    }

    /// Verify accounting integrity: distribution totals match collected fees.
    pub fn verify_accounting(&self) -> bool {
        let distributed = self.total_sequencer_rewards
            .saturating_add(self.total_shared_security)
            .saturating_add(self.total_treasury)
            .saturating_add(self.total_burned);

        // Distribution should match total fees collected
        // (small discrepancy possible from penalties not yet distributed)
        distributed <= self.total_fees_collected.saturating_add(self.total_expiry_penalties)
    }
}

/// Per-intent fee ledger entry for audit trail.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct IntentFeeLedgerEntry {
    pub intent_id: [u8; 32],
    pub payer: [u8; 32],
    pub status: IntentFeeStatus,
    pub recorded_at_block: u64,
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gas::routing::PoSeqFeeDistribution;

    #[test]
    fn test_new_accounting() {
        let acc = PoSeqFeeAccounting::new();
        assert_eq!(acc.total_fees_collected, 0);
        assert_eq!(acc.intents_charged, 0);
    }

    #[test]
    fn test_record_charges() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_poseq_charge(5_000, 95_000, 1_000, false, "swap");
        acc.record_poseq_charge(3_000, 97_000, 0, false, "payment");

        assert_eq!(acc.total_fees_collected, 8_000);
        assert_eq!(acc.total_refunds, 192_000);
        assert_eq!(acc.intents_charged, 2);
        assert_eq!(acc.tip_stats.intents_with_tip, 1);
        assert_eq!(acc.tip_stats.intents_without_tip, 1);
        assert_eq!(acc.tip_stats.total_effective_tips, 1_000);
        assert_eq!(acc.tip_stats.max_effective_tip, 1_000);
        assert_eq!(*acc.fees_by_intent_type.get("swap").unwrap(), 5_000);
        assert_eq!(*acc.fees_by_intent_type.get("payment").unwrap(), 3_000);
    }

    #[test]
    fn test_record_distribution() {
        let mut acc = PoSeqFeeAccounting::new();
        let dist = PoSeqFeeDistribution {
            total_fee: 10_000,
            sequencer_rewards: 4_500,
            shared_security: 2_000,
            treasury: 1_500,
            burn: 2_000,
        };
        acc.record_distribution(&dist);

        assert_eq!(acc.total_sequencer_rewards, 4_500);
        assert_eq!(acc.total_burned, 2_000);
        assert_eq!(acc.batch_distributions.len(), 1);
    }

    #[test]
    fn test_average_fee() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_poseq_charge(4_000, 0, 0, false, "swap");
        acc.record_poseq_charge(6_000, 0, 0, false, "swap");
        assert_eq!(acc.average_poseq_fee(), 5_000);
    }

    #[test]
    fn test_average_fee_zero_charges() {
        let acc = PoSeqFeeAccounting::new();
        assert_eq!(acc.average_poseq_fee(), 0);
    }

    #[test]
    fn test_congestion_sample_tracking() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_congestion_sample(100, 10_000);
        acc.record_congestion_sample(101, 15_000);
        acc.record_congestion_sample(102, 20_000);
        assert_eq!(acc.average_congestion_bps(), 15_000);
    }

    #[test]
    fn test_congestion_sample_bounded() {
        let mut acc = PoSeqFeeAccounting::new();
        for i in 0..1500 {
            acc.record_congestion_sample(i, 10_000);
        }
        // Should be capped at 1000 samples
        assert!(acc.congestion_samples.len() <= 1000);
    }

    #[test]
    fn test_expiry_penalty_recording() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_expiry_penalty(500);
        acc.record_expiry_penalty(300);
        assert_eq!(acc.total_expiry_penalties, 800);
        assert_eq!(acc.intents_expired_with_penalty, 2);
    }

    #[test]
    fn test_tip_capping_stats() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_poseq_charge(5_000, 0, 50_000, true, "swap");
        acc.record_poseq_charge(3_000, 0, 10_000, false, "swap");
        assert_eq!(acc.tip_stats.tips_capped, 1);
    }

    #[test]
    fn test_accounting_integrity() {
        let mut acc = PoSeqFeeAccounting::new();
        acc.record_poseq_charge(10_000, 0, 0, false, "swap");

        let dist = PoSeqFeeDistribution {
            total_fee: 10_000,
            sequencer_rewards: 4_500,
            shared_security: 2_000,
            treasury: 1_500,
            burn: 2_000,
        };
        acc.record_distribution(&dist);

        assert!(acc.verify_accounting());
    }
}
