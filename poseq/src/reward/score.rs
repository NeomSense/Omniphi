use serde::{Deserialize, Serialize};

use crate::liveness::events::LivenessEventExport;
use crate::performance::record::NodePerformanceRecord;

/// Per-node reward attribution for one epoch.
///
/// All values are integer basis points (0–10000 except PoCMultiplierBps and
/// FinalScoreBps which can reach 20000). No floating-point arithmetic.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EpochRewardScore {
    pub node_id: [u8; 32],
    /// Associated operator address, if bonded.
    pub operator_address: Option<String>,
    pub epoch: u64,

    /// Base participation score from proposals + attestations vs eligible slots.
    /// Directly sourced from `NodePerformanceRecord.participation_rate_bps`.
    pub base_score_bps: u32,

    /// Liveness/uptime score: 10000 if node was seen active this epoch, else 0.
    pub uptime_score_bps: u32,

    /// PoC multiplier from the slow lane. Default 10000 (1.0x neutral).
    /// Range: 5000 (0.5x) – 15000 (1.5x).
    pub poc_multiplier_bps: u32,

    /// Penalty for fault events: `min(fault_events * 500, 5000)` bps.
    pub fault_penalty_bps: u32,

    /// Final reward score after all adjustments.
    /// `= clamp((base + uptime) / 2 * poc_mult / 10000 - fault_penalty, 0, 20000)`
    pub final_score_bps: u32,

    /// Whether the operator had an active bond this epoch.
    pub is_bonded: bool,
}

impl EpochRewardScore {
    /// Compute `final_score_bps` from components.
    ///
    /// Formula (all integer):
    /// ```text
    /// combined = (base_score_bps + uptime_score_bps) / 2
    /// scaled   = combined * poc_multiplier_bps / 10000
    /// final    = clamp(scaled - fault_penalty_bps, 0, 20000)
    /// ```
    pub fn compute_final(
        base_score_bps: u32,
        uptime_score_bps: u32,
        poc_multiplier_bps: u32,
        fault_penalty_bps: u32,
    ) -> u32 {
        let combined = (u64::from(base_score_bps) + u64::from(uptime_score_bps)) / 2;
        let scaled = combined * u64::from(poc_multiplier_bps) / 10_000;
        let scaled_u32 = scaled.min(20_000) as u32;
        scaled_u32.saturating_sub(fault_penalty_bps)
    }

    /// Compute fault penalty: 500 bps per fault event, capped at 5000.
    pub fn compute_fault_penalty(fault_events: u64) -> u32 {
        (fault_events.saturating_mul(500)).min(5_000) as u32
    }
}

/// Builds an `EpochRewardScore` from a performance record and optional liveness export.
///
/// - `performance`: required — provides base_score_bps and fault count.
/// - `liveness_export`: optional — if the node appears in active_events, uptime = 10000.
/// - `poc_multiplier_bps`: PoC score from chain. Pass 10000 if unavailable.
/// - `operator_address`: from the bonding store. Pass None if unbonded.
/// - `is_bonded`: whether the operator has an active bond this epoch.
pub fn build_reward_score(
    performance: &NodePerformanceRecord,
    liveness_export: Option<&LivenessEventExport>,
    poc_multiplier_bps: u32,
    operator_address: Option<String>,
    is_bonded: bool,
) -> EpochRewardScore {
    let base_score_bps = performance.participation_rate_bps;

    let uptime_score_bps = liveness_export
        .map(|le| {
            if le.active_events.iter().any(|e| e.node_id == performance.node_id) {
                10_000u32
            } else {
                0u32
            }
        })
        .unwrap_or(0u32);

    let fault_penalty_bps = EpochRewardScore::compute_fault_penalty(performance.fault_events);

    let final_score_bps = EpochRewardScore::compute_final(
        base_score_bps,
        uptime_score_bps,
        poc_multiplier_bps,
        fault_penalty_bps,
    );

    EpochRewardScore {
        node_id: performance.node_id,
        operator_address,
        epoch: performance.epoch,
        base_score_bps,
        uptime_score_bps,
        poc_multiplier_bps,
        fault_penalty_bps,
        final_score_bps,
        is_bonded,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn perf(node: u8, participation_bps: u32, faults: u64) -> NodePerformanceRecord {
        NodePerformanceRecord {
            node_id: nid(node),
            epoch: 1,
            proposals_count: 1,
            attestations_count: 8,
            missed_attestations: 2,
            fault_events: faults,
            participation_rate_bps: participation_bps,
        }
    }

    #[test]
    fn test_compute_final_neutral() {
        // base=10000, uptime=10000, poc=10000 (1.0x), fault=0 → (10000+10000)/2*10000/10000 = 10000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 10_000, 0), 10_000);
    }

    #[test]
    fn test_compute_final_half_participation() {
        // base=5000, uptime=10000 → combined=7500, *1.0 = 7500, -0 = 7500
        assert_eq!(EpochRewardScore::compute_final(5_000, 10_000, 10_000, 0), 7_500);
    }

    #[test]
    fn test_compute_final_poc_boost() {
        // base=10000, uptime=10000, poc=15000 (1.5x) → 10000*1.5=15000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 15_000, 0), 15_000);
    }

    #[test]
    fn test_compute_final_poc_penalty() {
        // poc=5000 (0.5x) → 10000*0.5=5000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 5_000, 0), 5_000);
    }

    #[test]
    fn test_compute_final_fault_penalty() {
        // 2 faults → 1000 bps penalty; 10000 - 1000 = 9000
        let fault_pen = EpochRewardScore::compute_fault_penalty(2);
        assert_eq!(fault_pen, 1_000);
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 10_000, fault_pen), 9_000);
    }

    #[test]
    fn test_fault_penalty_cap() {
        // >10 faults → capped at 5000 bps
        assert_eq!(EpochRewardScore::compute_fault_penalty(20), 5_000);
    }

    #[test]
    fn test_compute_final_zero_participation() {
        // base=0, uptime=0 → final=0
        assert_eq!(EpochRewardScore::compute_final(0, 0, 10_000, 0), 0);
    }

    #[test]
    fn test_compute_final_saturate_at_zero() {
        // fault penalty exceeds score → saturates to 0, no underflow
        assert_eq!(EpochRewardScore::compute_final(100, 100, 10_000, 5_000), 0);
    }

    #[test]
    fn test_compute_final_cap_20000() {
        // poc=20000 but combined=10000 → 10000*2 = 20000 (at cap)
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 20_000, 0), 20_000);
    }

    #[test]
    fn test_build_reward_score_active_node() {
        use crate::liveness::events::{LivenessEvent, LivenessEventExport};
        let p = perf(1, 8_000, 0);
        let le = LivenessEventExport {
            epoch: 1,
            active_events: vec![LivenessEvent {
                node_id: nid(1),
                epoch: 1,
                last_seen_slot: 5,
                was_proposer: false,
                was_attestor: true,
            }],
            inactivity_events: vec![],
        };
        let score = build_reward_score(&p, Some(&le), 10_000, Some("omni1op".into()), true);
        assert_eq!(score.base_score_bps, 8_000);
        assert_eq!(score.uptime_score_bps, 10_000);
        assert_eq!(score.fault_penalty_bps, 0);
        // (8000+10000)/2 = 9000, *1.0 = 9000
        assert_eq!(score.final_score_bps, 9_000);
        assert!(score.is_bonded);
        assert_eq!(score.operator_address.as_deref(), Some("omni1op"));
    }

    #[test]
    fn test_build_reward_score_inactive_node() {
        let p = perf(2, 0, 3);
        let score = build_reward_score(&p, None, 10_000, None, false);
        assert_eq!(score.uptime_score_bps, 0);
        // 3 faults → 1500 bps penalty; (0+0)/2 = 0 - 1500 saturates to 0
        assert_eq!(score.fault_penalty_bps, 1_500);
        assert_eq!(score.final_score_bps, 0);
        assert!(!score.is_bonded);
    }
}
