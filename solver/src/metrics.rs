//! Solver performance metrics tracking.

use std::collections::BTreeMap;

/// Tracks solver performance over time.
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
pub struct SolverMetrics {
    /// Total commitments submitted.
    pub commitments_submitted: u64,
    /// Total reveals submitted.
    pub reveals_submitted: u64,
    /// Times our bundle was selected as winner.
    pub auctions_won: u64,
    /// Times our bundle was not selected.
    pub auctions_lost: u64,
    /// Successful settlements.
    pub settlements_succeeded: u64,
    /// Failed settlements.
    pub settlements_failed: u64,
    /// Commit-without-reveal events (penalty).
    pub commit_without_reveal: u64,
    /// Total fees earned.
    pub total_fees_earned: u128,
    /// Total bond slashed.
    pub total_bond_slashed: u128,
    /// Per-intent-class stats.
    pub by_intent_class: BTreeMap<String, IntentClassStats>,
}

#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
pub struct IntentClassStats {
    pub attempted: u64,
    pub won: u64,
    pub settled: u64,
    pub failed: u64,
}

impl SolverMetrics {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn record_commitment(&mut self, intent_class: &str) {
        self.commitments_submitted += 1;
        self.by_intent_class.entry(intent_class.into())
            .or_default().attempted += 1;
    }

    pub fn record_reveal(&mut self) {
        self.reveals_submitted += 1;
    }

    pub fn record_win(&mut self, intent_class: &str, fee: u64) {
        self.auctions_won += 1;
        self.total_fees_earned += fee as u128;
        self.by_intent_class.entry(intent_class.into())
            .or_default().won += 1;
    }

    pub fn record_loss(&mut self) {
        self.auctions_lost += 1;
    }

    pub fn record_settlement_success(&mut self, intent_class: &str) {
        self.settlements_succeeded += 1;
        self.by_intent_class.entry(intent_class.into())
            .or_default().settled += 1;
    }

    pub fn record_settlement_failure(&mut self, intent_class: &str) {
        self.settlements_failed += 1;
        self.by_intent_class.entry(intent_class.into())
            .or_default().failed += 1;
    }

    pub fn record_no_reveal_penalty(&mut self, slashed: u128) {
        self.commit_without_reveal += 1;
        self.total_bond_slashed += slashed;
    }

    /// Win rate as basis points (0-10000).
    pub fn win_rate_bps(&self) -> u64 {
        let total = self.auctions_won + self.auctions_lost;
        if total == 0 { return 0; }
        (self.auctions_won * 10000) / total
    }

    /// Settlement success rate as basis points (0-10000).
    pub fn settlement_rate_bps(&self) -> u64 {
        let total = self.settlements_succeeded + self.settlements_failed;
        if total == 0 { return 10000; }
        (self.settlements_succeeded * 10000) / total
    }

    /// Net profit (fees earned minus bond slashed).
    pub fn net_profit(&self) -> i128 {
        self.total_fees_earned as i128 - self.total_bond_slashed as i128
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_metrics() {
        let m = SolverMetrics::new();
        assert_eq!(m.win_rate_bps(), 0);
        assert_eq!(m.settlement_rate_bps(), 10000); // No settlements = 100%
    }

    #[test]
    fn test_win_rate() {
        let mut m = SolverMetrics::new();
        m.record_win("transfer", 50);
        m.record_win("transfer", 60);
        m.record_loss();
        assert_eq!(m.win_rate_bps(), 6666); // 2/3
    }

    #[test]
    fn test_settlement_tracking() {
        let mut m = SolverMetrics::new();
        m.record_settlement_success("transfer");
        m.record_settlement_success("transfer");
        m.record_settlement_failure("swap");
        assert_eq!(m.settlement_rate_bps(), 6666);
    }

    #[test]
    fn test_net_profit() {
        let mut m = SolverMetrics::new();
        m.record_win("transfer", 100);
        m.record_no_reveal_penalty(30);
        assert_eq!(m.net_profit(), 70);
    }

    #[test]
    fn test_intent_class_breakdown() {
        let mut m = SolverMetrics::new();
        m.record_commitment("transfer");
        m.record_commitment("swap");
        m.record_win("transfer", 50);

        assert_eq!(m.by_intent_class["transfer"].attempted, 1);
        assert_eq!(m.by_intent_class["transfer"].won, 1);
        assert_eq!(m.by_intent_class["swap"].attempted, 1);
        assert_eq!(m.by_intent_class["swap"].won, 0);
    }

    #[test]
    fn test_serialization_roundtrip() {
        let mut m = SolverMetrics::new();
        m.record_commitment("transfer");
        m.record_win("transfer", 50);

        let json = serde_json::to_string(&m).unwrap();
        let parsed: SolverMetrics = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.commitments_submitted, 1);
        assert_eq!(parsed.total_fees_earned, 50);
    }
}
