//! Governance-controlled protocol parameters.
//!
//! Every adjustable parameter lives here. Updates go through governance proposals.
//! Validation ensures parameters stay within safe ranges.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

// ─── Parameter Groups ───────────────────────────────────────────────────────

/// Auction timing and limits.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuctionParameters {
    pub commit_phase_blocks: u64,
    pub reveal_phase_blocks: u64,
    pub max_commitments_per_solver_per_window: u32,
    pub max_bundle_steps: u32,
}

impl Default for AuctionParameters {
    fn default() -> Self {
        AuctionParameters {
            commit_phase_blocks: 5,
            reveal_phase_blocks: 3,
            max_commitments_per_solver_per_window: 10,
            max_bundle_steps: 64,
        }
    }
}

/// Fairness and censorship detection.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FairnessParameters {
    pub censorship_exclusion_threshold: u32,
    pub forced_inclusion_after_blocks: u64,
    pub max_leader_discretion_bps: u32,
}

impl Default for FairnessParameters {
    fn default() -> Self {
        FairnessParameters {
            censorship_exclusion_threshold: 3,
            forced_inclusion_after_blocks: 20,
            max_leader_discretion_bps: 500,
        }
    }
}

/// Slashing percentages and violation thresholds.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SlashingParameters {
    pub commit_without_reveal_bps: u64,
    pub invalid_settlement_bps: u64,
    pub double_fill_bps: u64,
    pub fraudulent_route_bps: u64,
    pub max_violation_score: u64,
    pub auto_deactivation_threshold: u64,
    pub validator_equivocation_bps: u64,
    pub validator_liveness_bps: u64,
}

impl Default for SlashingParameters {
    fn default() -> Self {
        SlashingParameters {
            commit_without_reveal_bps: 100,
            invalid_settlement_bps: 2_500,
            double_fill_bps: 10_000,
            fraudulent_route_bps: 5_000,
            max_violation_score: 10_000,
            auto_deactivation_threshold: 9_500,
            validator_equivocation_bps: 10_000,
            validator_liveness_bps: 100,
        }
    }
}

/// Data availability policy.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DAParameters {
    pub failure_threshold: u32,
    pub timeout_ms: u64,
    pub archive_retention_epochs: u64,
}

impl Default for DAParameters {
    fn default() -> Self {
        DAParameters {
            failure_threshold: 3,
            timeout_ms: 5_000,
            archive_retention_epochs: 100,
        }
    }
}

/// Intent pool capacity and anti-spam.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct IntentPoolParameters {
    pub max_pool_size: u32,
    pub max_intents_per_block_per_user: u32,
    pub max_nonce_gap: u64,
    pub max_intent_size_bytes: u32,
    pub min_intent_fee_bps: u64,
    pub min_intent_lifetime_blocks: u64,
    pub max_pool_residence_blocks: u64,
    pub expiry_check_interval_blocks: u64,
}

impl Default for IntentPoolParameters {
    fn default() -> Self {
        IntentPoolParameters {
            max_pool_size: 50_000,
            max_intents_per_block_per_user: 10,
            max_nonce_gap: 3,
            max_intent_size_bytes: 4_096,
            min_intent_fee_bps: 10,
            min_intent_lifetime_blocks: 10,
            max_pool_residence_blocks: 1_000,
            expiry_check_interval_blocks: 5,
        }
    }
}

/// Top-level protocol parameters container.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProtocolParameters {
    pub version: u32,
    pub auction: AuctionParameters,
    pub fairness: FairnessParameters,
    pub slashing: SlashingParameters,
    pub da: DAParameters,
    pub intent_pool: IntentPoolParameters,
    /// Minimum validator stake (token units).
    pub min_validator_stake: u128,
    /// Minimum solver bond (token units).
    pub min_solver_bond: u128,
    /// Active solver bond threshold (token units).
    pub active_solver_bond: u128,
    /// Fast dispute window (blocks).
    pub fast_dispute_window: u64,
    /// Extended dispute window (blocks).
    pub extended_dispute_window: u64,
    /// Unbonding period (blocks).
    pub unbonding_period_blocks: u64,
}

impl Default for ProtocolParameters {
    fn default() -> Self {
        ProtocolParameters {
            version: 1,
            auction: AuctionParameters::default(),
            fairness: FairnessParameters::default(),
            slashing: SlashingParameters::default(),
            da: DAParameters::default(),
            intent_pool: IntentPoolParameters::default(),
            min_validator_stake: 100_000,
            min_solver_bond: 10_000,
            active_solver_bond: 50_000,
            fast_dispute_window: 100,
            extended_dispute_window: 50_400,
            unbonding_period_blocks: 50_400,
        }
    }
}

impl ProtocolParameters {
    /// Conservative devnet/testnet parameters.
    pub fn testnet() -> Self {
        ProtocolParameters {
            version: 1,
            auction: AuctionParameters {
                commit_phase_blocks: 2,
                reveal_phase_blocks: 2,
                max_commitments_per_solver_per_window: 10,
                max_bundle_steps: 64,
            },
            fairness: FairnessParameters::default(),
            slashing: SlashingParameters::default(),
            da: DAParameters { failure_threshold: 3, timeout_ms: 10_000, archive_retention_epochs: 10 },
            intent_pool: IntentPoolParameters {
                max_pool_size: 5_000,
                max_intents_per_block_per_user: 20,
                max_nonce_gap: 5,
                max_intent_size_bytes: 4_096,
                min_intent_fee_bps: 1,
                min_intent_lifetime_blocks: 3,
                max_pool_residence_blocks: 200,
                expiry_check_interval_blocks: 3,
            },
            min_validator_stake: 1_000,
            min_solver_bond: 100,
            active_solver_bond: 500,
            fast_dispute_window: 10,
            extended_dispute_window: 100,
            unbonding_period_blocks: 50,
        }
    }

    /// Validate all parameter ranges.
    pub fn validate(&self) -> Result<(), Vec<String>> {
        let mut errors = Vec::new();

        // Auction
        if self.auction.commit_phase_blocks == 0 { errors.push("auction.commit_phase_blocks must be > 0".into()); }
        if self.auction.reveal_phase_blocks == 0 { errors.push("auction.reveal_phase_blocks must be > 0".into()); }
        if self.auction.max_commitments_per_solver_per_window == 0 { errors.push("auction.max_commitments must be > 0".into()); }
        if self.auction.max_bundle_steps == 0 { errors.push("auction.max_bundle_steps must be > 0".into()); }

        // Slashing
        if self.slashing.commit_without_reveal_bps > 10_000 { errors.push("slashing.commit_without_reveal_bps must be <= 10000".into()); }
        if self.slashing.max_violation_score > 10_000 { errors.push("slashing.max_violation_score must be <= 10000".into()); }
        if self.slashing.auto_deactivation_threshold > self.slashing.max_violation_score {
            errors.push("slashing.auto_deactivation_threshold must be <= max_violation_score".into());
        }

        // Economics
        if self.active_solver_bond < self.min_solver_bond {
            errors.push("active_solver_bond must be >= min_solver_bond".into());
        }
        if self.extended_dispute_window <= self.fast_dispute_window {
            errors.push("extended_dispute_window must be > fast_dispute_window".into());
        }

        // Intent pool
        if self.intent_pool.max_pool_size == 0 { errors.push("intent_pool.max_pool_size must be > 0".into()); }
        if self.intent_pool.max_pool_residence_blocks < self.intent_pool.min_intent_lifetime_blocks {
            errors.push("intent_pool.max_pool_residence must be >= min_intent_lifetime".into());
        }

        // DA
        if self.da.failure_threshold == 0 { errors.push("da.failure_threshold must be > 0".into()); }

        if errors.is_empty() { Ok(()) } else { Err(errors) }
    }
}

// ─── Governance Actions ─────────────────────────────────────────────────────

/// A governance action that modifies protocol parameters.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum GovernanceAction {
    UpdateAuction(AuctionParameters),
    UpdateFairness(FairnessParameters),
    UpdateSlashing(SlashingParameters),
    UpdateDA(DAParameters),
    UpdateIntentPool(IntentPoolParameters),
    UpdateValidatorStake(u128),
    UpdateSolverBond { min_bond: u128, active_bond: u128 },
    UpdateDisputeWindows { fast: u64, extended: u64 },
    UpdateUnbondingPeriod(u64),
    EmergencyParameterOverride(ProtocolParameters),
}

/// Result of applying a governance action.
#[derive(Debug, Clone)]
pub struct ParameterUpdateResult {
    pub success: bool,
    pub old_version: u32,
    pub new_version: u32,
    pub action: String,
    pub errors: Vec<String>,
}

/// Apply a governance action to protocol parameters.
///
/// Validates the result before accepting. Returns the old and new parameters.
pub fn apply_governance_action(
    params: &ProtocolParameters,
    action: &GovernanceAction,
) -> Result<(ProtocolParameters, ParameterUpdateResult), Vec<String>> {
    let mut new = params.clone();
    let action_desc;

    match action {
        GovernanceAction::UpdateAuction(a) => {
            new.auction = a.clone();
            action_desc = "update_auction";
        }
        GovernanceAction::UpdateFairness(f) => {
            new.fairness = f.clone();
            action_desc = "update_fairness";
        }
        GovernanceAction::UpdateSlashing(s) => {
            new.slashing = s.clone();
            action_desc = "update_slashing";
        }
        GovernanceAction::UpdateDA(d) => {
            new.da = d.clone();
            action_desc = "update_da";
        }
        GovernanceAction::UpdateIntentPool(ip) => {
            new.intent_pool = ip.clone();
            action_desc = "update_intent_pool";
        }
        GovernanceAction::UpdateValidatorStake(stake) => {
            new.min_validator_stake = *stake;
            action_desc = "update_validator_stake";
        }
        GovernanceAction::UpdateSolverBond { min_bond, active_bond } => {
            new.min_solver_bond = *min_bond;
            new.active_solver_bond = *active_bond;
            action_desc = "update_solver_bond";
        }
        GovernanceAction::UpdateDisputeWindows { fast, extended } => {
            new.fast_dispute_window = *fast;
            new.extended_dispute_window = *extended;
            action_desc = "update_dispute_windows";
        }
        GovernanceAction::UpdateUnbondingPeriod(period) => {
            new.unbonding_period_blocks = *period;
            action_desc = "update_unbonding_period";
        }
        GovernanceAction::EmergencyParameterOverride(p) => {
            new = p.clone();
            action_desc = "emergency_override";
        }
    }

    // Validate new parameters
    new.validate()?;

    new.version = params.version + 1;

    let result = ParameterUpdateResult {
        success: true,
        old_version: params.version,
        new_version: new.version,
        action: action_desc.to_string(),
        errors: vec![],
    };

    Ok((new, result))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_params_valid() {
        assert!(ProtocolParameters::default().validate().is_ok());
    }

    #[test]
    fn test_testnet_params_valid() {
        assert!(ProtocolParameters::testnet().validate().is_ok());
    }

    #[test]
    fn test_invalid_zero_commit_phase() {
        let mut p = ProtocolParameters::default();
        p.auction.commit_phase_blocks = 0;
        assert!(p.validate().is_err());
    }

    #[test]
    fn test_invalid_bond_ordering() {
        let mut p = ProtocolParameters::default();
        p.min_solver_bond = 100_000;
        p.active_solver_bond = 50_000;
        assert!(p.validate().is_err());
    }

    #[test]
    fn test_governance_action_update_auction() {
        let params = ProtocolParameters::default();
        let new_auction = AuctionParameters { commit_phase_blocks: 10, ..params.auction.clone() };
        let action = GovernanceAction::UpdateAuction(new_auction.clone());
        let (new_params, result) = apply_governance_action(&params, &action).unwrap();
        assert_eq!(new_params.auction.commit_phase_blocks, 10);
        assert_eq!(new_params.version, 2);
        assert!(result.success);
    }

    #[test]
    fn test_governance_action_rejected_invalid() {
        let params = ProtocolParameters::default();
        let bad_auction = AuctionParameters { commit_phase_blocks: 0, ..params.auction.clone() };
        let action = GovernanceAction::UpdateAuction(bad_auction);
        assert!(apply_governance_action(&params, &action).is_err());
    }

    #[test]
    fn test_governance_action_update_solver_bond() {
        let params = ProtocolParameters::default();
        let action = GovernanceAction::UpdateSolverBond { min_bond: 20_000, active_bond: 100_000 };
        let (new_params, _) = apply_governance_action(&params, &action).unwrap();
        assert_eq!(new_params.min_solver_bond, 20_000);
        assert_eq!(new_params.active_solver_bond, 100_000);
    }

    #[test]
    fn test_version_increments() {
        let params = ProtocolParameters::default();
        assert_eq!(params.version, 1);
        let action = GovernanceAction::UpdateUnbondingPeriod(100);
        let (p2, _) = apply_governance_action(&params, &action).unwrap();
        assert_eq!(p2.version, 2);
        let action2 = GovernanceAction::UpdateUnbondingPeriod(200);
        let (p3, _) = apply_governance_action(&p2, &action2).unwrap();
        assert_eq!(p3.version, 3);
    }
}
