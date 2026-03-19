use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

/// Sequencer market tier classification.
///
/// Mirrors `SequencerTier` in `chain/x/poseq/types/adjudication.go`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum SequencerTier {
    /// Bonded, poc_mult > 11000, participation > 9000, zero faults.
    Elite,
    /// Bonded, poc_mult ≥ 9000, participation ≥ 7000, faults ≤ 2.
    Established,
    /// Bonded, meets minimum thresholds, no critical faults.
    Standard,
    /// Recently activated or recovering; limited history.
    Probationary,
    /// Participation below threshold or high fault history; or unbonded.
    Underperforming,
}

impl SequencerTier {
    /// Classify a sequencer into a tier.
    ///
    /// All thresholds are integers — no floating-point.
    ///
    /// Parameters:
    /// - `is_bonded`: whether operator has an active bond
    /// - `poc_mult_bps`: PoC multiplier in basis points (5000–15000)
    /// - `participation_bps`: participation rate in basis points (0–10000)
    /// - `fault_events_recent`: fault events in trailing N epochs
    /// - `epochs_since_activation`: epochs since node became Active
    pub fn classify(
        is_bonded: bool,
        poc_mult_bps: u32,
        participation_bps: u32,
        fault_events_recent: u64,
        epochs_since_activation: u64,
    ) -> Self {
        if !is_bonded {
            return SequencerTier::Underperforming;
        }
        if epochs_since_activation < 3 {
            return SequencerTier::Probationary;
        }
        if participation_bps < 5000 || fault_events_recent > 5 {
            return SequencerTier::Underperforming;
        }
        if poc_mult_bps > 11_000 && participation_bps > 9_000 && fault_events_recent == 0 {
            return SequencerTier::Elite;
        }
        if poc_mult_bps >= 9_000 && participation_bps >= 7_000 && fault_events_recent <= 2 {
            return SequencerTier::Established;
        }
        SequencerTier::Standard
    }
}

/// The computed ranking profile for a sequencer at a given epoch.
///
/// RankScore = participation_rate_bps × poc_multiplier_bps / 10000.
/// Bond acts as a gate (admission filter), not a continuous rank factor.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SequencerRankingProfile {
    pub node_id: [u8; 32],
    pub operator_address: String,
    pub epoch: u64,
    pub available_bond: u64,
    pub poc_multiplier_bps: u32,
    pub participation_rate_bps: u32,
    pub fault_events_recent: u64,
    pub rank_score: u32,
    pub tier: SequencerTier,
}

impl SequencerRankingProfile {
    /// Build a ranking profile from components.
    pub fn build(
        node_id: [u8; 32],
        operator_address: String,
        epoch: u64,
        available_bond: u64,
        poc_multiplier_bps: u32,
        participation_rate_bps: u32,
        fault_events_recent: u64,
        epochs_since_activation: u64,
        is_bonded: bool,
    ) -> Self {
        let poc_mult = if poc_multiplier_bps == 0 { 10_000 } else { poc_multiplier_bps };
        let rank_score = (participation_rate_bps as u64 * poc_mult as u64 / 10_000) as u32;
        let tier = SequencerTier::classify(
            is_bonded,
            poc_mult,
            participation_rate_bps,
            fault_events_recent,
            epochs_since_activation,
        );

        SequencerRankingProfile {
            node_id,
            operator_address,
            epoch,
            available_bond,
            poc_multiplier_bps: poc_mult,
            participation_rate_bps,
            fault_events_recent,
            rank_score,
            tier,
        }
    }
}

/// Stores and computes ranking profiles for sequencers.
pub struct RankingEngine {
    profiles: BTreeMap<[u8; 32], SequencerRankingProfile>,
}

impl RankingEngine {
    pub fn new() -> Self {
        RankingEngine {
            profiles: BTreeMap::new(),
        }
    }

    pub fn store(&mut self, profile: SequencerRankingProfile) {
        self.profiles.insert(profile.node_id, profile);
    }

    pub fn get(&self, node_id: &[u8; 32]) -> Option<&SequencerRankingProfile> {
        self.profiles.get(node_id)
    }

    /// Returns profiles sorted by rank_score descending (best first).
    pub fn ranked(&self) -> Vec<&SequencerRankingProfile> {
        let mut v: Vec<_> = self.profiles.values().collect();
        v.sort_by(|a, b| b.rank_score.cmp(&a.rank_score));
        v
    }

    pub fn len(&self) -> usize {
        self.profiles.len()
    }

    pub fn is_empty(&self) -> bool {
        self.profiles.is_empty()
    }
}

impl Default for RankingEngine {
    fn default() -> Self {
        Self::new()
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

    #[test]
    fn test_tier_elite() {
        let t = SequencerTier::classify(true, 12000, 9500, 0, 10);
        assert_eq!(t, SequencerTier::Elite);
    }

    #[test]
    fn test_tier_established() {
        let t = SequencerTier::classify(true, 9500, 7500, 1, 10);
        assert_eq!(t, SequencerTier::Established);
    }

    #[test]
    fn test_tier_standard() {
        let t = SequencerTier::classify(true, 8000, 6000, 0, 10);
        assert_eq!(t, SequencerTier::Standard);
    }

    #[test]
    fn test_tier_probationary_new_node() {
        let t = SequencerTier::classify(true, 12000, 9500, 0, 2);
        assert_eq!(t, SequencerTier::Probationary);
    }

    #[test]
    fn test_tier_underperforming_unbonded() {
        let t = SequencerTier::classify(false, 12000, 9500, 0, 10);
        assert_eq!(t, SequencerTier::Underperforming);
    }

    #[test]
    fn test_tier_underperforming_high_faults() {
        let t = SequencerTier::classify(true, 12000, 9500, 6, 10);
        assert_eq!(t, SequencerTier::Underperforming);
    }

    #[test]
    fn test_ranking_engine_sorted() {
        let mut engine = RankingEngine::new();
        for (i, participation) in [(1u8, 6000u32), (2, 9000), (3, 8000)] {
            let p = SequencerRankingProfile::build(
                nid(i), "op".into(), 5, 1_000_000, 10_000,
                participation, 0, 10, true,
            );
            engine.store(p);
        }
        let ranked = engine.ranked();
        // Should be sorted 9000 > 8000 > 6000
        assert_eq!(ranked[0].participation_rate_bps, 9000);
        assert_eq!(ranked[1].participation_rate_bps, 8000);
        assert_eq!(ranked[2].participation_rate_bps, 6000);
    }

    #[test]
    fn test_rank_score_formula() {
        // participation=8000, poc_mult=12000 → 8000 * 12000 / 10000 = 9600
        let p = SequencerRankingProfile::build(
            nid(1), "op".into(), 5, 1_000_000, 12_000, 8_000, 0, 10, true,
        );
        assert_eq!(p.rank_score, 9600);
    }
}
