//! Solver market economics — bid model, auction ranking, QoS tracking,
//! user protections, and fee lifecycle integration for solver-based execution.
//!
//! This module extends the PoSeq gas model to cover the solver fee surface:
//! ```text
//! Total Reserved = max_poseq_fee + max_runtime_fee + max_solver_fee
//! ```
//!
//! Solver fees are bounded by user-approved limits, ranked deterministically,
//! and integrated into the fee lifecycle state machine.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ═══════════════════════════════════════════════════════════════════════════
// SECTION 1 — Solver Fee Model
// ═══════════════════════════════════════════════════════════════════════════

/// Solver fee terms declared by the user as part of intent submission.
/// Sets the maximum extraction boundary for solver-based execution.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SolverFeeTerms {
    /// Maximum solver fee the user will pay (OMNI base units).
    pub max_solver_fee: u128,
    /// Pricing mode preference.
    pub pricing_mode: SolverPricingMode,
    /// If true, solver must settle within max_completion_blocks or face penalty.
    pub require_timely_settlement: bool,
    /// Maximum blocks after sequencing before solver must settle.
    pub max_completion_blocks: u64,
}

/// How the solver fee is determined.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SolverPricingMode {
    /// Solver quotes a fixed fee in their bid.
    FixedQuote,
    /// Solver quotes a fee capped at user's max; actual fee = min(quote, max).
    CappedQuote,
    /// Protocol uses market-determined fee (default for most intents).
    MarketRate,
}

impl Default for SolverFeeTerms {
    fn default() -> Self {
        SolverFeeTerms {
            max_solver_fee: 0,
            pricing_mode: SolverPricingMode::CappedQuote,
            require_timely_settlement: true,
            max_completion_blocks: 20,
        }
    }
}

impl SolverFeeTerms {
    pub fn validate(&self) -> Result<(), SolverMarketError> {
        if self.max_completion_blocks == 0 && self.require_timely_settlement {
            return Err(SolverMarketError::InvalidFeeTerms(
                "max_completion_blocks cannot be 0 with timely settlement required".into(),
            ));
        }
        Ok(())
    }
}

/// A solver's bid for an intent or batch.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SolverBid {
    /// Unique bid identifier.
    pub bid_id: [u8; 32],
    /// Solver submitting this bid.
    pub solver_id: [u8; 32],
    /// Intent or batch this bid targets.
    pub target_id: [u8; 32],
    /// Batch window this bid applies to.
    pub batch_window: u64,
    /// Solver's quoted fee (OMNI base units).
    pub quoted_solver_fee: u128,
    /// Solver's estimated execution gas cost.
    pub expected_execution_cost: u128,
    /// Solver's estimated completion time in blocks.
    pub expected_completion_blocks: u64,
    /// Block height after which this bid expires.
    pub expiry: u64,
    /// Solver's signature over the bid.
    pub signature: Vec<u8>,
}

impl SolverBid {
    /// Compute deterministic bid_id from canonical fields.
    pub fn compute_bid_id(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.solver_id);
        hasher.update(self.target_id);
        hasher.update(self.batch_window.to_be_bytes());
        hasher.update(self.quoted_solver_fee.to_be_bytes());
        hasher.update(self.expiry.to_be_bytes());
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Validate structural correctness of the bid.
    pub fn validate(&self, current_block: u64) -> Result<(), SolverMarketError> {
        if self.solver_id == [0u8; 32] {
            return Err(SolverMarketError::InvalidBid("zero solver_id".into()));
        }
        if self.target_id == [0u8; 32] {
            return Err(SolverMarketError::InvalidBid("zero target_id".into()));
        }
        if self.signature.is_empty() {
            return Err(SolverMarketError::InvalidBid("missing signature".into()));
        }
        if current_block > self.expiry {
            return Err(SolverMarketError::BidExpired {
                bid_id: self.bid_id,
                expiry: self.expiry,
                current_block,
            });
        }
        Ok(())
    }

    /// Check if this bid respects the user's fee terms.
    pub fn respects_terms(&self, terms: &SolverFeeTerms) -> bool {
        self.quoted_solver_fee <= terms.max_solver_fee
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// SECTION 2 — Deterministic Auction Ranking
// ═══════════════════════════════════════════════════════════════════════════

/// Parameters for solver auction ranking.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuctionRankingParams {
    /// Weight of price competitiveness (basis points, out of 10000).
    pub price_weight_bps: u64,
    /// Weight of reliability score (basis points).
    pub reliability_weight_bps: u64,
    /// Weight of latency estimate (basis points).
    pub latency_weight_bps: u64,
    /// Weight of settlement success history (basis points).
    pub history_weight_bps: u64,
    /// Minimum reliability score to be eligible (0-10000).
    pub min_reliability_score: u64,
    /// Maximum bids per solver per window.
    pub max_bids_per_solver: u64,
}

impl Default for AuctionRankingParams {
    fn default() -> Self {
        AuctionRankingParams {
            price_weight_bps: 4_000,      // 40%
            reliability_weight_bps: 3_000, // 30%
            latency_weight_bps: 1_000,     // 10%
            history_weight_bps: 2_000,     // 20%
            min_reliability_score: 1_000,  // 10% minimum
            max_bids_per_solver: 5,
        }
    }
}

impl AuctionRankingParams {
    pub fn validate(&self) -> Result<(), SolverMarketError> {
        let total = self.price_weight_bps
            .saturating_add(self.reliability_weight_bps)
            .saturating_add(self.latency_weight_bps)
            .saturating_add(self.history_weight_bps);
        if total != 10_000 {
            return Err(SolverMarketError::InvalidRankingParams(
                format!("weights sum to {} instead of 10000", total),
            ));
        }
        Ok(())
    }
}

/// Solver performance profile used for auction ranking.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SolverPerformanceProfile {
    pub solver_id: [u8; 32],
    pub bids_submitted: u64,
    pub bids_won: u64,
    pub settlements_completed: u64,
    pub settlements_failed: u64,
    pub invalid_plan_count: u64,
    pub timeout_count: u64,
    /// Average completion latency in blocks (0 if no history).
    pub avg_latency_blocks: u64,
    /// Reliability score (0-10000 bps). Deterministically computed.
    pub reliability_score: u64,
}

impl SolverPerformanceProfile {
    pub fn new(solver_id: [u8; 32]) -> Self {
        SolverPerformanceProfile {
            solver_id,
            bids_submitted: 0,
            bids_won: 0,
            settlements_completed: 0,
            settlements_failed: 0,
            invalid_plan_count: 0,
            timeout_count: 0,
            avg_latency_blocks: 0,
            reliability_score: 5_000, // default 50%
        }
    }

    /// Deterministically recompute reliability score from history.
    pub fn recompute_reliability(&mut self) {
        let total_attempts = self.settlements_completed + self.settlements_failed;
        if total_attempts == 0 {
            self.reliability_score = 5_000; // default
            return;
        }

        // Base: success rate (0-10000)
        let success_rate = (self.settlements_completed as u128)
            .saturating_mul(10_000)
            / (total_attempts as u128);

        // Penalty for invalid plans: -500 per invalid, capped at -5000
        let invalid_penalty = std::cmp::min(
            self.invalid_plan_count.saturating_mul(500),
            5_000,
        );

        // Penalty for timeouts: -300 per timeout, capped at -3000
        let timeout_penalty = std::cmp::min(
            self.timeout_count.saturating_mul(300),
            3_000,
        );

        let raw = (success_rate as u64)
            .saturating_sub(invalid_penalty)
            .saturating_sub(timeout_penalty);

        self.reliability_score = std::cmp::min(raw, 10_000);
    }

    /// Record a bid submission.
    pub fn record_bid(&mut self) {
        self.bids_submitted += 1;
    }

    /// Record winning a bid.
    pub fn record_win(&mut self) {
        self.bids_won += 1;
    }

    /// Record a successful settlement.
    pub fn record_settlement_success(&mut self, latency_blocks: u64) {
        self.settlements_completed += 1;
        // EMA for latency: new = (old * (n-1) + new) / n
        let n = self.settlements_completed;
        if n == 1 {
            self.avg_latency_blocks = latency_blocks;
        } else {
            self.avg_latency_blocks = self.avg_latency_blocks
                .saturating_mul(n - 1)
                .saturating_add(latency_blocks)
                / n;
        }
        self.recompute_reliability();
    }

    /// Record a failed settlement.
    pub fn record_settlement_failure(&mut self) {
        self.settlements_failed += 1;
        self.recompute_reliability();
    }

    /// Record an invalid plan submission.
    pub fn record_invalid_plan(&mut self) {
        self.invalid_plan_count += 1;
        self.recompute_reliability();
    }

    /// Record a timeout.
    pub fn record_timeout(&mut self) {
        self.timeout_count += 1;
        self.recompute_reliability();
    }
}

/// Ranked bid with composite score for auction selection.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RankedBid {
    pub bid: SolverBid,
    pub price_score: u64,
    pub reliability_score: u64,
    pub latency_score: u64,
    pub history_score: u64,
    pub composite_score: u64,
    /// Deterministic tiebreak key: SHA256(bid_id ‖ target_id ‖ batch_window).
    pub tiebreak_key: [u8; 32],
}

/// Run deterministic solver auction ranking.
///
/// Returns bids sorted by composite score (descending), with deterministic
/// tiebreak on equal scores.
pub fn rank_bids(
    bids: &[SolverBid],
    profiles: &BTreeMap<[u8; 32], SolverPerformanceProfile>,
    terms: &SolverFeeTerms,
    params: &AuctionRankingParams,
    current_block: u64,
) -> Vec<RankedBid> {
    let mut ranked = Vec::new();

    // Find the maximum quoted fee for normalization (price inversion)
    let max_fee = bids.iter()
        .filter(|b| b.validate(current_block).is_ok() && b.respects_terms(terms))
        .map(|b| b.quoted_solver_fee)
        .max()
        .unwrap_or(1);
    let max_fee = std::cmp::max(max_fee, 1); // avoid division by zero

    for bid in bids {
        // Filter: validate and check terms
        if bid.validate(current_block).is_err() || !bid.respects_terms(terms) {
            continue;
        }

        let profile = profiles.get(&bid.solver_id);
        let reliability = profile.map(|p| p.reliability_score).unwrap_or(5_000);

        // Filter: minimum reliability
        if reliability < params.min_reliability_score {
            continue;
        }

        // Price score: lower fee = higher score (inverted)
        // price_score = (1 - fee/max_fee) * 10000
        let price_score = if max_fee == 0 { 10_000u64 } else {
            let ratio = (bid.quoted_solver_fee as u128)
                .saturating_mul(10_000)
                / (max_fee as u128);
            10_000u64.saturating_sub(ratio as u64)
        };

        // Reliability score: directly from profile
        let reliability_score = reliability;

        // Latency score: lower latency = higher score
        let avg_latency = profile.map(|p| p.avg_latency_blocks).unwrap_or(10);
        let latency_score = if bid.expected_completion_blocks == 0 {
            10_000u64
        } else {
            // Invert: score = max(0, 10000 - completion * 500)
            10_000u64.saturating_sub(bid.expected_completion_blocks.saturating_mul(500))
        };

        // History score: win rate + total experience
        let history_score = if let Some(p) = profile {
            if p.bids_submitted == 0 { 5_000 } else {
                let win_rate = (p.bids_won as u128)
                    .saturating_mul(10_000)
                    / (p.bids_submitted as u128);
                std::cmp::min(win_rate as u64, 10_000)
            }
        } else {
            5_000 // new solver default
        };

        // Weighted composite
        let composite = price_score.saturating_mul(params.price_weight_bps) / 10_000
            + reliability_score.saturating_mul(params.reliability_weight_bps) / 10_000
            + latency_score.saturating_mul(params.latency_weight_bps) / 10_000
            + history_score.saturating_mul(params.history_weight_bps) / 10_000;

        // Deterministic tiebreak
        let mut hasher = Sha256::new();
        hasher.update(bid.bid_id);
        hasher.update(bid.target_id);
        hasher.update(bid.batch_window.to_be_bytes());
        let result = hasher.finalize();
        let mut tiebreak = [0u8; 32];
        tiebreak.copy_from_slice(&result);

        ranked.push(RankedBid {
            bid: bid.clone(),
            price_score,
            reliability_score,
            latency_score,
            history_score,
            composite_score: composite,
            tiebreak_key: tiebreak,
        });
    }

    // Sort: composite DESC, tiebreak ASC (deterministic)
    ranked.sort_by(|a, b| {
        b.composite_score.cmp(&a.composite_score)
            .then(a.tiebreak_key.cmp(&b.tiebreak_key))
    });

    ranked
}

// ═══════════════════════════════════════════════════════════════════════════
// SECTION 5 — User Protection Rules
// ═══════════════════════════════════════════════════════════════════════════

/// Result of applying user protection checks to a winning solver bid.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum UserProtectionResult {
    /// Bid passes all user protection checks.
    Approved,
    /// Bid violates user fee cap.
    FeeCapViolation { quoted: u128, max: u128 },
    /// Bid has expired.
    BidExpired { expiry: u64, current: u64 },
    /// Solver does not meet minimum reliability.
    ReliabilityTooLow { score: u64, min: u64 },
    /// Solver's completion estimate exceeds user's tolerance.
    CompletionTooSlow { estimated: u64, max: u64 },
}

/// Check user protection constraints against a winning bid.
pub fn check_user_protections(
    bid: &SolverBid,
    terms: &SolverFeeTerms,
    profile: Option<&SolverPerformanceProfile>,
    min_reliability: u64,
    current_block: u64,
) -> UserProtectionResult {
    if current_block > bid.expiry {
        return UserProtectionResult::BidExpired {
            expiry: bid.expiry,
            current: current_block,
        };
    }
    if bid.quoted_solver_fee > terms.max_solver_fee {
        return UserProtectionResult::FeeCapViolation {
            quoted: bid.quoted_solver_fee,
            max: terms.max_solver_fee,
        };
    }
    if let Some(p) = profile {
        if p.reliability_score < min_reliability {
            return UserProtectionResult::ReliabilityTooLow {
                score: p.reliability_score,
                min: min_reliability,
            };
        }
    }
    if terms.require_timely_settlement
        && bid.expected_completion_blocks > terms.max_completion_blocks
    {
        return UserProtectionResult::CompletionTooSlow {
            estimated: bid.expected_completion_blocks,
            max: terms.max_completion_blocks,
        };
    }
    UserProtectionResult::Approved
}

// ═══════════════════════════════════════════════════════════════════════════
// SECTION 7 — Auction Failure and Fallback
// ═══════════════════════════════════════════════════════════════════════════

/// Outcome of solver auction selection.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AuctionOutcome {
    /// Winner selected successfully.
    WinnerSelected {
        bid: SolverBid,
        composite_score: u64,
        fallback_count: usize,
    },
    /// Winner selected after primary failed (fallback path).
    FallbackSelected {
        bid: SolverBid,
        original_winner_id: [u8; 32],
        fallback_reason: String,
        composite_score: u64,
    },
    /// No valid bids — intent queued for re-auction.
    NoBids,
    /// All bids expired.
    AllExpired,
    /// All bids failed user protection checks.
    AllRejected { reasons: Vec<String> },
}

/// Select auction winner with fallback logic.
pub fn select_winner(
    ranked_bids: &[RankedBid],
    terms: &SolverFeeTerms,
    profiles: &BTreeMap<[u8; 32], SolverPerformanceProfile>,
    min_reliability: u64,
    current_block: u64,
    max_fallback_attempts: usize,
) -> AuctionOutcome {
    if ranked_bids.is_empty() {
        return AuctionOutcome::NoBids;
    }

    let mut rejections = Vec::new();
    let mut first_winner_id = None;
    let mut attempts = 0;

    for ranked in ranked_bids {
        if attempts >= max_fallback_attempts {
            break;
        }
        attempts += 1;

        let profile = profiles.get(&ranked.bid.solver_id);
        let result = check_user_protections(
            &ranked.bid, terms, profile, min_reliability, current_block,
        );

        match result {
            UserProtectionResult::Approved => {
                if first_winner_id.is_none() {
                    return AuctionOutcome::WinnerSelected {
                        bid: ranked.bid.clone(),
                        composite_score: ranked.composite_score,
                        fallback_count: 0,
                    };
                } else {
                    return AuctionOutcome::FallbackSelected {
                        bid: ranked.bid.clone(),
                        original_winner_id: first_winner_id.unwrap(),
                        fallback_reason: rejections.last().cloned().unwrap_or_default(),
                        composite_score: ranked.composite_score,
                    };
                }
            }
            other => {
                if first_winner_id.is_none() {
                    first_winner_id = Some(ranked.bid.solver_id);
                }
                rejections.push(format!("{:?}", other));
            }
        }
    }

    if rejections.is_empty() {
        AuctionOutcome::AllExpired
    } else {
        AuctionOutcome::AllRejected { reasons: rejections }
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// SECTION 10 — Solver Market Accounting
// ═══════════════════════════════════════════════════════════════════════════

/// Cumulative solver market accounting.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SolverMarketAccounting {
    pub total_solver_fees_reserved: u128,
    pub total_solver_fees_charged: u128,
    pub total_solver_refunds: u128,
    pub total_solver_penalties: u128,
    pub total_auctions_held: u64,
    pub total_bids_received: u64,
    pub total_winners_selected: u64,
    pub total_fallback_selections: u64,
    pub total_no_bid_auctions: u64,
    pub total_settlements_completed: u64,
    pub total_settlements_failed: u64,
    /// Per-solver win counts for concentration tracking.
    pub solver_wins: BTreeMap<[u8; 32], u64>,
}

impl SolverMarketAccounting {
    pub fn new() -> Self { Self::default() }

    pub fn record_auction(&mut self, bids: u64) {
        self.total_auctions_held += 1;
        self.total_bids_received += bids;
    }

    pub fn record_winner(&mut self, solver_id: [u8; 32], is_fallback: bool) {
        self.total_winners_selected += 1;
        if is_fallback { self.total_fallback_selections += 1; }
        *self.solver_wins.entry(solver_id).or_insert(0) += 1;
    }

    pub fn record_no_bids(&mut self) {
        self.total_no_bid_auctions += 1;
    }

    pub fn record_solver_fee(&mut self, reserved: u128, charged: u128, refund: u128) {
        self.total_solver_fees_reserved = self.total_solver_fees_reserved.saturating_add(reserved);
        self.total_solver_fees_charged = self.total_solver_fees_charged.saturating_add(charged);
        self.total_solver_refunds = self.total_solver_refunds.saturating_add(refund);
    }

    pub fn record_penalty(&mut self, amount: u128) {
        self.total_solver_penalties = self.total_solver_penalties.saturating_add(amount);
    }

    pub fn record_settlement(&mut self, success: bool) {
        if success {
            self.total_settlements_completed += 1;
        } else {
            self.total_settlements_failed += 1;
        }
    }

    /// Compute the Herfindahl–Hirschman Index (HHI) for market concentration.
    /// HHI range: 0 (perfect competition) to 10000 (monopoly).
    pub fn concentration_hhi(&self) -> u64 {
        if self.total_winners_selected == 0 { return 0; }
        let total = self.total_winners_selected as u128;
        let mut hhi: u128 = 0;
        for &wins in self.solver_wins.values() {
            let share_bps = (wins as u128).saturating_mul(100) / total; // percent
            hhi = hhi.saturating_add(share_bps.saturating_mul(share_bps));
        }
        std::cmp::min(hhi as u64, 10_000)
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// ERROR TYPES
// ═══════════════════════════════════════════════════════════════════════════

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SolverMarketError {
    InvalidBid(String),
    InvalidFeeTerms(String),
    InvalidRankingParams(String),
    BidExpired { bid_id: [u8; 32], expiry: u64, current_block: u64 },
    SolverNotEligible { solver_id: [u8; 32], reason: String },
    DuplicateBid { bid_id: [u8; 32] },
    BidRateLimitExceeded { solver_id: [u8; 32], count: u64, max: u64 },
}

impl std::fmt::Display for SolverMarketError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidBid(r) => write!(f, "invalid bid: {}", r),
            Self::InvalidFeeTerms(r) => write!(f, "invalid fee terms: {}", r),
            Self::InvalidRankingParams(r) => write!(f, "invalid ranking params: {}", r),
            Self::BidExpired { expiry, current_block, .. } =>
                write!(f, "bid expired at {} (current {})", expiry, current_block),
            Self::SolverNotEligible { reason, .. } =>
                write!(f, "solver not eligible: {}", reason),
            Self::DuplicateBid { .. } => write!(f, "duplicate bid"),
            Self::BidRateLimitExceeded { count, max, .. } =>
                write!(f, "bid rate limit: {} >= {}", count, max),
        }
    }
}

impl std::error::Error for SolverMarketError {}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS
// ═══════════════════════════════════════════════════════════════════════════

#[cfg(test)]
mod tests {
    use super::*;

    fn solver(b: u8) -> [u8; 32] { let mut s = [0u8; 32]; s[0] = b; s }
    fn target() -> [u8; 32] { [0xAA; 32] }

    fn make_bid(solver_byte: u8, fee: u128, latency: u64) -> SolverBid {
        let mut bid = SolverBid {
            bid_id: [0u8; 32],
            solver_id: solver(solver_byte),
            target_id: target(),
            batch_window: 1,
            quoted_solver_fee: fee,
            expected_execution_cost: fee / 2,
            expected_completion_blocks: latency,
            expiry: 1000,
            signature: vec![1u8; 64],
        };
        bid.bid_id = bid.compute_bid_id();
        bid
    }

    fn default_terms() -> SolverFeeTerms {
        SolverFeeTerms {
            max_solver_fee: 50_000,
            pricing_mode: SolverPricingMode::CappedQuote,
            require_timely_settlement: true,
            max_completion_blocks: 20,
        }
    }

    fn make_profile(solver_byte: u8, completed: u64, failed: u64) -> SolverPerformanceProfile {
        let mut p = SolverPerformanceProfile::new(solver(solver_byte));
        p.bids_submitted = completed + failed + 2;
        p.bids_won = completed + failed;
        for _ in 0..completed { p.record_settlement_success(5); }
        for _ in 0..failed { p.record_settlement_failure(); }
        p
    }

    // ─── Bid validation ────────────────────────────────────────────────

    #[test]
    fn test_bid_valid() {
        let bid = make_bid(1, 10_000, 5);
        assert!(bid.validate(500).is_ok());
    }

    #[test]
    fn test_bid_expired() {
        let bid = make_bid(1, 10_000, 5);
        assert!(matches!(bid.validate(1001), Err(SolverMarketError::BidExpired { .. })));
    }

    #[test]
    fn test_bid_zero_solver() {
        let mut bid = make_bid(0, 10_000, 5);
        bid.solver_id = [0u8; 32];
        assert!(matches!(bid.validate(500), Err(SolverMarketError::InvalidBid(_))));
    }

    #[test]
    fn test_bid_respects_terms() {
        let terms = default_terms();
        let cheap = make_bid(1, 30_000, 5);
        assert!(cheap.respects_terms(&terms));

        let expensive = make_bid(2, 100_000, 5);
        assert!(!expensive.respects_terms(&terms));
    }

    // ─── Ranking ───────────────────────────────────────────────────────

    #[test]
    fn test_ranking_cheaper_wins() {
        let terms = default_terms();
        let params = AuctionRankingParams::default();

        let cheap = make_bid(1, 10_000, 5);
        let expensive = make_bid(2, 40_000, 5);
        let bids = vec![expensive, cheap];

        let profiles = BTreeMap::new();
        let ranked = rank_bids(&bids, &profiles, &terms, &params, 500);

        assert_eq!(ranked.len(), 2);
        assert_eq!(ranked[0].bid.solver_id, solver(1)); // cheaper wins
    }

    #[test]
    fn test_ranking_reliability_matters() {
        let terms = default_terms();
        let params = AuctionRankingParams::default();

        let bid_a = make_bid(1, 20_000, 5);
        let bid_b = make_bid(2, 20_000, 5); // same price

        let mut profiles = BTreeMap::new();
        profiles.insert(solver(1), make_profile(1, 100, 0));  // 100% reliable
        profiles.insert(solver(2), make_profile(2, 50, 50));   // 50% reliable

        let ranked = rank_bids(&[bid_a, bid_b], &profiles, &terms, &params, 500);

        assert_eq!(ranked.len(), 2);
        assert_eq!(ranked[0].bid.solver_id, solver(1)); // more reliable wins
    }

    #[test]
    fn test_ranking_deterministic() {
        let terms = default_terms();
        let params = AuctionRankingParams::default();
        let bids: Vec<SolverBid> = (1..=5).map(|i| make_bid(i, 20_000, 5)).collect();
        let profiles = BTreeMap::new();

        let r1 = rank_bids(&bids, &profiles, &terms, &params, 500);
        let r2 = rank_bids(&bids, &profiles, &terms, &params, 500);

        for i in 0..r1.len() {
            assert_eq!(r1[i].bid.bid_id, r2[i].bid.bid_id);
            assert_eq!(r1[i].composite_score, r2[i].composite_score);
        }
    }

    #[test]
    fn test_ranking_filters_expired() {
        let terms = default_terms();
        let params = AuctionRankingParams::default();
        let mut bid = make_bid(1, 10_000, 5);
        bid.expiry = 100;
        let ranked = rank_bids(&[bid], &BTreeMap::new(), &terms, &params, 500);
        assert!(ranked.is_empty());
    }

    #[test]
    fn test_ranking_filters_over_budget() {
        let terms = default_terms(); // max 50,000
        let params = AuctionRankingParams::default();
        let bid = make_bid(1, 100_000, 5); // over budget
        let ranked = rank_bids(&[bid], &BTreeMap::new(), &terms, &params, 500);
        assert!(ranked.is_empty());
    }

    #[test]
    fn test_ranking_filters_low_reliability() {
        let terms = default_terms();
        let params = AuctionRankingParams::default(); // min_reliability = 1000

        let bid = make_bid(1, 10_000, 5);
        let mut profiles = BTreeMap::new();
        let mut p = SolverPerformanceProfile::new(solver(1));
        p.reliability_score = 500; // below 1000 minimum
        profiles.insert(solver(1), p);

        let ranked = rank_bids(&[bid], &profiles, &terms, &params, 500);
        assert!(ranked.is_empty());
    }

    // ─── Winner selection ──────────────────────────────────────────────

    #[test]
    fn test_select_winner_happy_path() {
        let terms = default_terms();
        let params = AuctionRankingParams::default();
        let bids = vec![make_bid(1, 10_000, 5), make_bid(2, 20_000, 5)];
        let profiles = BTreeMap::new();
        let ranked = rank_bids(&bids, &profiles, &terms, &params, 500);

        let outcome = select_winner(&ranked, &terms, &profiles, 1_000, 500, 3);
        assert!(matches!(outcome, AuctionOutcome::WinnerSelected { .. }));
    }

    #[test]
    fn test_select_winner_no_bids() {
        let terms = default_terms();
        let profiles = BTreeMap::new();
        let outcome = select_winner(&[], &terms, &profiles, 1_000, 500, 3);
        assert_eq!(outcome, AuctionOutcome::NoBids);
    }

    #[test]
    fn test_select_winner_fallback() {
        let terms = SolverFeeTerms {
            max_completion_blocks: 3, // very tight
            ..default_terms()
        };
        let params = AuctionRankingParams::default();

        let slow = make_bid(1, 5_000, 10);  // too slow for terms
        let fast = make_bid(2, 10_000, 2);  // within terms

        let profiles = BTreeMap::new();
        let ranked = rank_bids(&[slow, fast], &profiles, &terms, &params, 500);

        let outcome = select_winner(&ranked, &terms, &profiles, 0, 500, 3);
        // fast bid should win since slow fails completion check
        match &outcome {
            AuctionOutcome::WinnerSelected { bid, .. } |
            AuctionOutcome::FallbackSelected { bid, .. } => {
                assert_eq!(bid.solver_id, solver(2));
            }
            _ => panic!("expected winner, got {:?}", outcome),
        }
    }

    // ─── Performance tracking ──────────────────────────────────────────

    #[test]
    fn test_performance_reliability_computation() {
        let mut p = SolverPerformanceProfile::new(solver(1));
        for _ in 0..9 { p.record_settlement_success(5); }
        p.record_settlement_failure();
        // 9/10 = 90% success = 9000 bps
        assert_eq!(p.reliability_score, 9_000);
    }

    #[test]
    fn test_performance_invalid_plans_reduce_reliability() {
        let mut p = SolverPerformanceProfile::new(solver(1));
        for _ in 0..10 { p.record_settlement_success(5); }
        // reliability = 10000
        assert_eq!(p.reliability_score, 10_000);

        p.record_invalid_plan();
        p.record_invalid_plan();
        // reliability = 10000 - 2*500 = 9000
        assert_eq!(p.reliability_score, 9_000);
    }

    #[test]
    fn test_performance_timeouts_reduce_reliability() {
        let mut p = SolverPerformanceProfile::new(solver(1));
        for _ in 0..10 { p.record_settlement_success(5); }
        p.record_timeout();
        // reliability = 10000 - 300 = 9700
        assert_eq!(p.reliability_score, 9_700);
    }

    #[test]
    fn test_performance_latency_tracking() {
        let mut p = SolverPerformanceProfile::new(solver(1));
        p.record_settlement_success(10);
        assert_eq!(p.avg_latency_blocks, 10);
        p.record_settlement_success(20);
        // EMA: (10 * 1 + 20) / 2 = 15
        assert_eq!(p.avg_latency_blocks, 15);
    }

    // ─── User protections ──────────────────────────────────────────────

    #[test]
    fn test_user_protection_approved() {
        let bid = make_bid(1, 10_000, 5);
        let terms = default_terms();
        let result = check_user_protections(&bid, &terms, None, 0, 500);
        assert_eq!(result, UserProtectionResult::Approved);
    }

    #[test]
    fn test_user_protection_fee_cap() {
        let bid = make_bid(1, 100_000, 5);
        let terms = default_terms(); // max 50,000
        let result = check_user_protections(&bid, &terms, None, 0, 500);
        assert!(matches!(result, UserProtectionResult::FeeCapViolation { .. }));
    }

    #[test]
    fn test_user_protection_too_slow() {
        let bid = make_bid(1, 10_000, 50); // 50 blocks
        let terms = default_terms(); // max 20 blocks
        let result = check_user_protections(&bid, &terms, None, 0, 500);
        assert!(matches!(result, UserProtectionResult::CompletionTooSlow { .. }));
    }

    // ─── Accounting ────────────────────────────────────────────────────

    #[test]
    fn test_accounting_concentration() {
        let mut acc = SolverMarketAccounting::new();

        // Monopoly: one solver wins everything
        for _ in 0..100 {
            acc.record_winner(solver(1), false);
        }
        let hhi = acc.concentration_hhi();
        assert_eq!(hhi, 10_000); // perfect monopoly

        // Reset and test competition
        let mut acc2 = SolverMarketAccounting::new();
        for i in 1..=10 {
            for _ in 0..10 {
                acc2.record_winner(solver(i), false);
            }
        }
        let hhi2 = acc2.concentration_hhi();
        // 10 solvers * (10%)^2 = 10 * 100 = 1000
        assert_eq!(hhi2, 1_000); // competitive market
    }

    #[test]
    fn test_accounting_basic() {
        let mut acc = SolverMarketAccounting::new();
        acc.record_auction(5);
        acc.record_winner(solver(1), false);
        acc.record_solver_fee(50_000, 30_000, 20_000);
        acc.record_settlement(true);

        assert_eq!(acc.total_auctions_held, 1);
        assert_eq!(acc.total_bids_received, 5);
        assert_eq!(acc.total_solver_fees_charged, 30_000);
        assert_eq!(acc.total_solver_refunds, 20_000);
        assert_eq!(acc.total_settlements_completed, 1);
    }

    // ─── Ranking params validation ─────────────────────────────────────

    #[test]
    fn test_ranking_params_valid() {
        assert!(AuctionRankingParams::default().validate().is_ok());
    }

    #[test]
    fn test_ranking_params_invalid() {
        let mut params = AuctionRankingParams::default();
        params.price_weight_bps = 0;
        assert!(matches!(params.validate(), Err(SolverMarketError::InvalidRankingParams(_))));
    }

    // ─── Adversarial scenarios ─────────────────────────────────────────

    #[test]
    fn test_whale_underbid_filtered_by_reliability() {
        let terms = default_terms();
        let params = AuctionRankingParams {
            min_reliability_score: 3_000,
            ..AuctionRankingParams::default()
        };

        // Whale: ultra-cheap but terrible reliability
        let whale = make_bid(1, 100, 1);
        // Honest: normal price, great reliability
        let honest = make_bid(2, 30_000, 5);

        let mut profiles = BTreeMap::new();
        let mut whale_p = SolverPerformanceProfile::new(solver(1));
        whale_p.reliability_score = 1_000; // below threshold
        profiles.insert(solver(1), whale_p);
        profiles.insert(solver(2), make_profile(2, 100, 5));

        let ranked = rank_bids(&[whale, honest], &profiles, &terms, &params, 500);

        // Whale should be filtered out
        assert_eq!(ranked.len(), 1);
        assert_eq!(ranked[0].bid.solver_id, solver(2));
    }

    #[test]
    fn test_fake_low_bid_griefing() {
        // Solver bids very low but will fail to settle → tracked in QoS
        let mut p = SolverPerformanceProfile::new(solver(1));
        for _ in 0..5 { p.record_settlement_failure(); }
        p.record_timeout();
        p.record_timeout();

        // Reliability should be very low after repeated failures
        assert!(p.reliability_score < 2_000);
    }

    #[test]
    fn test_bid_id_deterministic() {
        let bid = make_bid(1, 10_000, 5);
        let id1 = bid.compute_bid_id();
        let id2 = bid.compute_bid_id();
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_no_duplicate_winners_in_concentration() {
        let mut acc = SolverMarketAccounting::new();
        acc.record_winner(solver(1), false);
        acc.record_winner(solver(1), false);
        assert_eq!(*acc.solver_wins.get(&solver(1)).unwrap(), 2);
    }
}
