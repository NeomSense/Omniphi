//! Phase 4 — Protocol parameter validation and configuration.
//!
//! Validates all protocol parameters at startup, rejecting unsafe ranges.
//! Provides documented defaults for devnet/testnet tuning.

use serde::{Deserialize, Serialize};
use super::constants::*;

/// Validated protocol configuration for the intent execution system.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentProtocolConfig {
    // Timing
    pub commit_phase_blocks: u64,
    pub reveal_phase_blocks: u64,
    pub min_intent_lifetime: u64,
    pub max_pool_residence: u64,
    pub fast_dispute_window: u64,
    pub extended_dispute_window: u64,
    pub unbonding_period_blocks: u64,
    pub expiry_check_interval: u64,

    // Economic
    pub min_intent_fee_bps: u64,
    pub min_solver_bond: u128,
    pub active_solver_bond: u128,
    pub fast_dispute_bond: u128,
    pub commit_without_reveal_penalty_bps: u64,

    // Limits
    pub max_intents_per_block_per_user: u32,
    pub max_nonce_gap: u64,
    pub max_intent_size: usize,
    pub max_pool_size: usize,
    pub max_commitments_per_solver_per_window: usize,
    pub max_bundle_steps: usize,

    // Reputation
    pub max_violation_score: u64,
    pub performance_score_init: u64,

    // DA
    pub da_failure_threshold: u32,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConfigValidationError {
    pub field: String,
    pub message: String,
}

impl std::fmt::Display for ConfigValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "config error in '{}': {}", self.field, self.message)
    }
}

impl IntentProtocolConfig {
    /// Create config with all defaults from constants.rs.
    pub fn default_config() -> Self {
        IntentProtocolConfig {
            commit_phase_blocks: COMMIT_PHASE_BLOCKS,
            reveal_phase_blocks: REVEAL_PHASE_BLOCKS,
            min_intent_lifetime: MIN_INTENT_LIFETIME,
            max_pool_residence: MAX_POOL_RESIDENCE,
            fast_dispute_window: FAST_DISPUTE_WINDOW,
            extended_dispute_window: EXTENDED_DISPUTE_WINDOW,
            unbonding_period_blocks: UNBONDING_PERIOD_BLOCKS,
            expiry_check_interval: EXPIRY_CHECK_INTERVAL,
            min_intent_fee_bps: MIN_INTENT_FEE_BPS,
            min_solver_bond: MIN_SOLVER_BOND,
            active_solver_bond: ACTIVE_SOLVER_BOND,
            fast_dispute_bond: FAST_DISPUTE_BOND,
            commit_without_reveal_penalty_bps: COMMIT_WITHOUT_REVEAL_PENALTY_BPS,
            max_intents_per_block_per_user: MAX_INTENTS_PER_BLOCK_PER_USER,
            max_nonce_gap: MAX_NONCE_GAP,
            max_intent_size: MAX_INTENT_SIZE,
            max_pool_size: MAX_POOL_SIZE,
            max_commitments_per_solver_per_window: MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW,
            max_bundle_steps: MAX_BUNDLE_STEPS,
            max_violation_score: MAX_VIOLATION_SCORE,
            performance_score_init: PERFORMANCE_SCORE_INIT,
            da_failure_threshold: DA_FAILURE_THRESHOLD,
        }
    }

    /// Devnet config with relaxed parameters for testing.
    pub fn devnet_config() -> Self {
        let mut c = Self::default_config();
        c.commit_phase_blocks = 2;
        c.reveal_phase_blocks = 2;
        c.min_intent_lifetime = 3;
        c.fast_dispute_window = 10;
        c.extended_dispute_window = 100;
        c.unbonding_period_blocks = 50;
        c.min_solver_bond = 100;
        c.active_solver_bond = 500;
        c.fast_dispute_bond = 10;
        c.max_pool_size = 1_000;
        c
    }

    /// Validate all parameters. Returns errors for any unsafe values.
    pub fn validate(&self) -> Result<(), Vec<ConfigValidationError>> {
        let mut errors = Vec::new();

        // Timing
        if self.commit_phase_blocks == 0 {
            errors.push(err("commit_phase_blocks", "must be > 0"));
        }
        if self.reveal_phase_blocks == 0 {
            errors.push(err("reveal_phase_blocks", "must be > 0"));
        }
        if self.min_intent_lifetime == 0 {
            errors.push(err("min_intent_lifetime", "must be > 0"));
        }
        if self.max_pool_residence < self.min_intent_lifetime {
            errors.push(err("max_pool_residence", "must be >= min_intent_lifetime"));
        }
        if self.fast_dispute_window == 0 {
            errors.push(err("fast_dispute_window", "must be > 0"));
        }
        if self.extended_dispute_window <= self.fast_dispute_window {
            errors.push(err("extended_dispute_window", "must be > fast_dispute_window"));
        }
        if self.expiry_check_interval == 0 {
            errors.push(err("expiry_check_interval", "must be > 0"));
        }

        // Economic
        if self.min_intent_fee_bps > 10_000 {
            errors.push(err("min_intent_fee_bps", "must be <= 10000 bps"));
        }
        if self.active_solver_bond < self.min_solver_bond {
            errors.push(err("active_solver_bond", "must be >= min_solver_bond"));
        }
        if self.commit_without_reveal_penalty_bps > 10_000 {
            errors.push(err("commit_without_reveal_penalty_bps", "must be <= 10000 bps"));
        }

        // Limits
        if self.max_intents_per_block_per_user == 0 {
            errors.push(err("max_intents_per_block_per_user", "must be > 0"));
        }
        if self.max_pool_size == 0 {
            errors.push(err("max_pool_size", "must be > 0"));
        }
        if self.max_commitments_per_solver_per_window == 0 {
            errors.push(err("max_commitments_per_solver_per_window", "must be > 0"));
        }
        if self.max_bundle_steps == 0 {
            errors.push(err("max_bundle_steps", "must be > 0"));
        }

        // Reputation
        if self.max_violation_score > 10_000 {
            errors.push(err("max_violation_score", "must be <= 10000"));
        }
        if self.performance_score_init > 10_000 {
            errors.push(err("performance_score_init", "must be <= 10000"));
        }

        // DA
        if self.da_failure_threshold == 0 {
            errors.push(err("da_failure_threshold", "must be > 0"));
        }

        if errors.is_empty() { Ok(()) } else { Err(errors) }
    }

    /// Total batch window blocks.
    pub fn batch_window_blocks(&self) -> u64 {
        self.commit_phase_blocks + self.reveal_phase_blocks + 1
    }
}

fn err(field: &str, msg: &str) -> ConfigValidationError {
    ConfigValidationError { field: field.to_string(), message: msg.to_string() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config_valid() {
        let c = IntentProtocolConfig::default_config();
        assert!(c.validate().is_ok());
    }

    #[test]
    fn test_devnet_config_valid() {
        let c = IntentProtocolConfig::devnet_config();
        assert!(c.validate().is_ok());
    }

    #[test]
    fn test_invalid_zero_commit_phase() {
        let mut c = IntentProtocolConfig::default_config();
        c.commit_phase_blocks = 0;
        let errs = c.validate().unwrap_err();
        assert!(errs.iter().any(|e| e.field == "commit_phase_blocks"));
    }

    #[test]
    fn test_invalid_bond_ordering() {
        let mut c = IntentProtocolConfig::default_config();
        c.active_solver_bond = 100;
        c.min_solver_bond = 200;
        let errs = c.validate().unwrap_err();
        assert!(errs.iter().any(|e| e.field == "active_solver_bond"));
    }

    #[test]
    fn test_invalid_dispute_window_ordering() {
        let mut c = IntentProtocolConfig::default_config();
        c.extended_dispute_window = 50; // less than fast_dispute_window=100
        let errs = c.validate().unwrap_err();
        assert!(errs.iter().any(|e| e.field == "extended_dispute_window"));
    }

    #[test]
    fn test_batch_window_calculation() {
        let c = IntentProtocolConfig::default_config();
        assert_eq!(c.batch_window_blocks(), 9); // 5 + 3 + 1
    }
}
