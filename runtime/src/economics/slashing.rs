//! Solver and validator penalty application — Section 12.1-12.2.

use serde::{Deserialize, Serialize};

/// Severity classification for violations.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PenaltySeverity {
    Minor,
    Major,
}

/// A concrete penalty to apply to a solver.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SolverPenalty {
    pub solver_id: [u8; 32],
    pub severity: PenaltySeverity,
    pub slash_bps: u64,
    pub violation_score_delta: u64,
    pub reason: String,
    pub epoch: u64,
}

impl SolverPenalty {
    /// Compute the amount to slash from the solver's bond.
    pub fn slash_amount(&self, bond: u128) -> u128 {
        bond * self.slash_bps as u128 / 10_000
    }

    /// Minor violation: commit without reveal.
    pub fn commit_without_reveal(solver_id: [u8; 32], bond_locked: u128, epoch: u64) -> Self {
        SolverPenalty {
            solver_id,
            severity: PenaltySeverity::Minor,
            slash_bps: 100, // 1%
            violation_score_delta: 100,
            reason: "commit without reveal".to_string(),
            epoch,
        }
    }

    /// Major violation: invalid settlement.
    pub fn invalid_settlement(solver_id: [u8; 32], epoch: u64) -> Self {
        SolverPenalty {
            solver_id,
            severity: PenaltySeverity::Major,
            slash_bps: 2_500, // 25%
            violation_score_delta: 2_000,
            reason: "invalid settlement".to_string(),
            epoch,
        }
    }

    /// Major violation: double fill.
    pub fn double_fill(solver_id: [u8; 32], epoch: u64) -> Self {
        SolverPenalty {
            solver_id,
            severity: PenaltySeverity::Major,
            slash_bps: 10_000, // 100%
            violation_score_delta: 10_000,
            reason: "double fill".to_string(),
            epoch,
        }
    }

    /// Major violation: fraudulent route.
    pub fn fraudulent_route(solver_id: [u8; 32], epoch: u64) -> Self {
        SolverPenalty {
            solver_id,
            severity: PenaltySeverity::Major,
            slash_bps: 5_000, // 50%
            violation_score_delta: 5_000,
            reason: "fraudulent route".to_string(),
            epoch,
        }
    }
}

/// Record of a violation for tracking escalation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ViolationRecord {
    pub solver_id: [u8; 32],
    pub penalty: SolverPenalty,
    pub new_violation_score: u64,
    pub auto_deactivated: bool,
    pub slashed_amount: u128,
    pub block: u64,
}

/// Apply a penalty to a solver's violation score and determine if auto-deactivation triggers.
pub fn apply_penalty(
    current_violation_score: u64,
    penalty: &SolverPenalty,
    current_bond: u128,
    block: u64,
) -> ViolationRecord {
    let new_score = current_violation_score.saturating_add(penalty.violation_score_delta).min(10_000);
    let auto_deactivated = new_score >= 9_500; // MAX_VIOLATION_SCORE
    let slashed = penalty.slash_amount(current_bond);

    ViolationRecord {
        solver_id: penalty.solver_id,
        penalty: penalty.clone(),
        new_violation_score: new_score,
        auto_deactivated,
        slashed_amount: slashed,
        block,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_commit_without_reveal_penalty() {
        let penalty = SolverPenalty::commit_without_reveal([1u8; 32], 50_000, 1);
        assert_eq!(penalty.slash_bps, 100);
        assert_eq!(penalty.slash_amount(50_000), 500); // 1% of 50k
    }

    #[test]
    fn test_double_fill_penalty() {
        let penalty = SolverPenalty::double_fill([1u8; 32], 1);
        assert_eq!(penalty.slash_bps, 10_000);
        assert_eq!(penalty.slash_amount(50_000), 50_000); // 100%
    }

    #[test]
    fn test_auto_deactivation_on_high_score() {
        let penalty = SolverPenalty::invalid_settlement([1u8; 32], 1);
        let record = apply_penalty(8_000, &penalty, 50_000, 100);
        // 8000 + 2000 = 10000, clamped to 10000 >= 9500 → auto-deactivated
        assert!(record.auto_deactivated);
        assert_eq!(record.new_violation_score, 10_000);
    }

    #[test]
    fn test_no_deactivation_on_low_score() {
        let penalty = SolverPenalty::commit_without_reveal([1u8; 32], 50_000, 1);
        let record = apply_penalty(0, &penalty, 50_000, 100);
        assert!(!record.auto_deactivated);
        assert_eq!(record.new_violation_score, 100);
    }
}
