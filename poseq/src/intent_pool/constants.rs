//! Protocol constants for Omniphi Intent-Based Execution (Phase 1).
//!
//! All values are from the architecture spec Appendix A.
//! These constants are consensus-critical — changing them requires a protocol upgrade.

// ─── Timing (block counts) ──────────────────────────────────────────────────

/// Duration of commit phase in blocks.
pub const COMMIT_PHASE_BLOCKS: u64 = 5;

/// Duration of reveal phase in blocks.
pub const REVEAL_PHASE_BLOCKS: u64 = 3;

/// Total batch window = commit + reveal + 1 selection block.
pub const BATCH_WINDOW_BLOCKS: u64 = COMMIT_PHASE_BLOCKS + REVEAL_PHASE_BLOCKS + 1;

/// Minimum number of blocks an intent must be alive before deadline.
pub const MIN_INTENT_LIFETIME: u64 = 10;

/// Maximum blocks an intent can sit in the pool regardless of deadline.
pub const MAX_POOL_RESIDENCE: u64 = 1000;

/// Fast dispute window in blocks (~10 min at 6s blocks).
pub const FAST_DISPUTE_WINDOW: u64 = 100;

/// Extended dispute window in blocks (~7 days at 12s blocks).
pub const EXTENDED_DISPUTE_WINDOW: u64 = 50_400;

/// Timeout for data availability requests (milliseconds).
pub const DA_TIMEOUT_MS: u64 = 5_000;

/// Consecutive DA failures before forcing epoch transition.
pub const DA_FAILURE_THRESHOLD: u32 = 3;

/// Epochs to retain archive data.
pub const ARCHIVE_RETENTION_EPOCHS: u64 = 100;

/// Unbonding period in blocks (~7 days).
pub const UNBONDING_PERIOD_BLOCKS: u64 = 50_400;

/// Expiry check interval in blocks.
pub const EXPIRY_CHECK_INTERVAL: u64 = 5;

// ─── Economic (basis points unless noted) ───────────────────────────────────

/// Minimum intent fee in basis points.
pub const MIN_INTENT_FEE_BPS: u64 = 10;

/// Minimum solver bond to register (OMNI token units).
pub const MIN_SOLVER_BOND: u128 = 10_000;

/// Minimum solver bond to participate in auctions (OMNI token units).
pub const ACTIVE_SOLVER_BOND: u128 = 50_000;

/// Bond required to submit fast dispute (OMNI token units).
pub const FAST_DISPUTE_BOND: u128 = 1_000;

/// Bond required to submit extended dispute (OMNI token units).
pub const EXTENDED_DISPUTE_BOND: u128 = 10_000;

/// Penalty for commit-without-reveal in basis points of locked bond.
pub const COMMIT_WITHOUT_REVEAL_PENALTY_BPS: u64 = 100;

/// Frivolous dispute penalty in basis points of dispute bond.
pub const FRIVOLOUS_DISPUTE_PENALTY_BPS: u64 = 5_000;

/// Challenger reward as percentage of slashed solver bond.
pub const CHALLENGER_REWARD_PCT: u64 = 30;

/// Protocol cut as percentage of slashed solver bond.
pub const PROTOCOL_CUT_PCT: u64 = 20;

/// User refund as percentage of slashed solver bond.
pub const USER_REFUND_PCT: u64 = 50;

// ─── Limits ─────────────────────────────────────────────────────────────────

/// Maximum intents per block per user.
pub const MAX_INTENTS_PER_BLOCK_PER_USER: u32 = 10;

/// Maximum nonce gap before dropping an intent.
pub const MAX_NONCE_GAP: u64 = 3;

/// Maximum intent size in bytes.
pub const MAX_INTENT_SIZE: usize = 4_096;

/// Maximum intents the pool holds.
pub const MAX_POOL_SIZE: usize = 50_000;

/// Maximum commitments a solver can submit per batch window.
pub const MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW: usize = 10;

/// Violation score threshold for auto-deactivation.
pub const MAX_VIOLATION_SCORE: u64 = 9_500;

/// Maximum execution steps per bundle.
pub const MAX_BUNDLE_STEPS: usize = 64;

/// Maximum objects in a read set per step.
pub const MAX_READ_SET_SIZE: usize = 32;

/// Maximum objects in a write set per step.
pub const MAX_WRITE_SET_SIZE: usize = 16;

// ─── Reputation ─────────────────────────────────────────────────────────────

/// EMA smoothing factor (2 / (N+1), N=15).
pub const EMA_ALPHA: f64 = 0.125;

/// Initial performance score for new solvers (0-10000 bps).
pub const PERFORMANCE_SCORE_INIT: u64 = 5_000;

/// Initial violation score for new solvers.
pub const VIOLATION_SCORE_INIT: u64 = 0;

/// Initial latency score for new solvers (0-10000 bps).
pub const LATENCY_SCORE_INIT: u64 = 5_000;

// ─── Reward Distribution (percentages) ─────────────────────────────────────

/// Percentage of epoch revenue going to validators.
pub const VALIDATOR_REWARD_PCT: u64 = 50;

/// Percentage of epoch revenue going to solvers.
pub const SOLVER_REWARD_PCT: u64 = 30;

/// Percentage of epoch revenue going to treasury.
pub const TREASURY_PCT: u64 = 10;

/// Percentage of epoch revenue going to insurance fund.
pub const INSURANCE_PCT: u64 = 10;

// ─── Violation Score Thresholds ─────────────────────────────────────────────

/// Below this: no action (normal operation).
pub const VIOLATION_THRESHOLD_WARNING: u64 = 2_000;

/// Above warning, below this: reduced priority in auctions.
pub const VIOLATION_THRESHOLD_PROBATION: u64 = 5_000;

/// Above probation, below this: limited commits per window.
pub const VIOLATION_THRESHOLD_SUSPENSION: u64 = 8_000;

/// Above suspension, below max: temporary deactivation.
pub const VIOLATION_THRESHOLD_DEACTIVATION: u64 = 9_500;
