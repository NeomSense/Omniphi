use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::reward::score::EpochRewardScore;

/// The net reward settlement for one node in one epoch.
///
/// All fields are integer basis points (0–20000). No floating-point.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EpochSettlement {
    /// 32-byte node identity.
    pub node_id: [u8; 32],
    /// Cosmos bech32 operator address. Empty if unbonded.
    pub operator_address: String,
    /// Epoch this settlement covers.
    pub epoch: u64,
    /// Gross reward score before deductions (final_score_bps from EpochRewardScore).
    pub gross_reward_score_bps: u32,
    /// PoC multiplier applied this epoch (basis points, 5000–15000).
    pub poc_multiplier_bps: u32,
    /// Fault penalty deducted (basis points).
    pub fault_penalty_bps: u32,
    /// Slash penalty deducted this epoch (sum of slash_bps for all slashes executed).
    pub slash_penalty_bps: u32,
    /// Net reward score after all deductions. Clamped to [0, 20000].
    pub net_reward_score_bps: u32,
    /// Whether the operator had an active bond this epoch.
    pub is_bonded: bool,
    /// Number of slashes executed against this node this epoch.
    pub slash_count: u32,
}

impl EpochSettlement {
    /// Compute the net reward score from components.
    ///
    /// Formula (all integer):
    /// ```text
    /// slash_penalty = min(slash_bps_sum, gross)
    /// net = clamp(gross - slash_penalty, 0, 20000)
    /// ```
    pub fn compute(
        node_id: [u8; 32],
        operator_address: String,
        epoch: u64,
        reward_score: &EpochRewardScore,
        slash_bps_sum: u32,
        slash_count: u32,
        operator_address_from_bond: Option<String>,
    ) -> Self {
        let gross = reward_score.final_score_bps;
        let fault_penalty = reward_score.fault_penalty_bps;
        let poc_mult = if reward_score.poc_multiplier_bps == 0 {
            10_000
        } else {
            reward_score.poc_multiplier_bps
        };

        let slash_penalty = slash_bps_sum.min(gross);
        let net = gross.saturating_sub(slash_penalty).min(20_000);

        let op_addr = operator_address_from_bond.unwrap_or(operator_address);

        EpochSettlement {
            node_id,
            operator_address: op_addr,
            epoch,
            gross_reward_score_bps: gross,
            poc_multiplier_bps: poc_mult,
            fault_penalty_bps: fault_penalty,
            slash_penalty_bps: slash_penalty,
            net_reward_score_bps: net,
            is_bonded: reward_score.is_bonded,
            slash_count,
        }
    }
}

/// Computes and stores epoch settlement records for a set of nodes.
pub struct SettlementEngine {
    settlements: BTreeMap<([u8; 32], u64), EpochSettlement>, // (node_id, epoch)
}

impl SettlementEngine {
    pub fn new() -> Self {
        SettlementEngine {
            settlements: BTreeMap::new(),
        }
    }

    /// Store a settlement record. Overwrites any prior record for (node_id, epoch).
    pub fn store(&mut self, settlement: EpochSettlement) {
        self.settlements
            .insert((settlement.node_id, settlement.epoch), settlement);
    }

    /// Retrieve a settlement for (node_id, epoch).
    pub fn get(&self, node_id: &[u8; 32], epoch: u64) -> Option<&EpochSettlement> {
        self.settlements.get(&(*node_id, epoch))
    }

    /// All settlements for a given epoch.
    pub fn for_epoch(&self, epoch: u64) -> Vec<&EpochSettlement> {
        self.settlements
            .values()
            .filter(|s| s.epoch == epoch)
            .collect()
    }

    /// Total number of records.
    pub fn len(&self) -> usize {
        self.settlements.len()
    }

    pub fn is_empty(&self) -> bool {
        self.settlements.is_empty()
    }
}

impl Default for SettlementEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::reward::score::EpochRewardScore;

    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn reward_score(node: u8, final_bps: u32, fault_bps: u32) -> EpochRewardScore {
        EpochRewardScore {
            node_id: nid(node),
            operator_address: None,
            epoch: 5,
            base_score_bps: 8000,
            uptime_score_bps: 10000,
            poc_multiplier_bps: 10000,
            fault_penalty_bps: fault_bps,
            final_score_bps: final_bps,
            is_bonded: true,
        }
    }

    #[test]
    fn test_settlement_no_slash() {
        let rs = reward_score(1, 9000, 0);
        let s = EpochSettlement::compute(nid(1), "".into(), 5, &rs, 0, 0, None);
        assert_eq!(s.gross_reward_score_bps, 9000);
        assert_eq!(s.slash_penalty_bps, 0);
        assert_eq!(s.net_reward_score_bps, 9000);
    }

    #[test]
    fn test_settlement_slash_reduces_net() {
        let rs = reward_score(2, 9000, 0);
        // slash 500 bps
        let s = EpochSettlement::compute(nid(2), "".into(), 5, &rs, 500, 1, None);
        assert_eq!(s.slash_penalty_bps, 500);
        assert_eq!(s.net_reward_score_bps, 8500);
        assert_eq!(s.slash_count, 1);
    }

    #[test]
    fn test_settlement_slash_capped_at_gross() {
        let rs = reward_score(3, 1000, 0);
        // slash exceeds gross → net = 0
        let s = EpochSettlement::compute(nid(3), "".into(), 5, &rs, 5000, 1, None);
        assert_eq!(s.slash_penalty_bps, 1000); // capped at gross=1000
        assert_eq!(s.net_reward_score_bps, 0);
    }

    #[test]
    fn test_settlement_engine_store_and_retrieve() {
        let mut engine = SettlementEngine::new();
        let rs = reward_score(4, 8000, 500);
        let s = EpochSettlement::compute(nid(4), "omni1op".into(), 7, &rs, 0, 0, None);
        engine.store(s);

        let got = engine.get(&nid(4), 7).unwrap();
        assert_eq!(got.epoch, 7);
        assert_eq!(got.net_reward_score_bps, 8000);
    }

    #[test]
    fn test_settlement_engine_for_epoch() {
        let mut engine = SettlementEngine::new();
        for i in 1u8..=3 {
            let rs = reward_score(i, 7000, 0);
            engine.store(EpochSettlement::compute(nid(i), "".into(), 9, &rs, 0, 0, None));
        }
        // Add one for a different epoch
        let rs = reward_score(4, 5000, 0);
        engine.store(EpochSettlement::compute(nid(4), "".into(), 10, &rs, 0, 0, None));

        let epoch9 = engine.for_epoch(9);
        assert_eq!(epoch9.len(), 3);
    }
}
