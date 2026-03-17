//! Solver failure paths — deterministic handling of solver-related failures.

use serde::{Deserialize, Serialize};

/// Actions to take when a solver fails.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum SolverFailureAction {
    /// Apply penalty to the solver's violation score and bond.
    ApplyPenalty { slash_bps: u64, violation_delta: u64 },
    /// Deactivate the solver.
    Deactivate,
    /// Blacklist the solver permanently.
    PermanentBan,
    /// No action (solver did nothing wrong).
    NoAction,
}

/// Determine actions when a solver commits but does not reveal.
///
/// Rule: commit without reveal → 1% bond slash + violation score increase.
/// No penalty for the intent (it returns to pool via intent_failures).
pub fn handle_commit_without_reveal() -> Vec<SolverFailureAction> {
    vec![SolverFailureAction::ApplyPenalty {
        slash_bps: 100, // 1% of locked bond
        violation_delta: 100,
    }]
}

/// Determine actions when a solver's reveal doesn't match commitment.
///
/// Rule: invalid reveal → 100% bond slash + permanent ban.
pub fn handle_invalid_reveal() -> Vec<SolverFailureAction> {
    vec![
        SolverFailureAction::ApplyPenalty {
            slash_bps: 10_000,
            violation_delta: 10_000,
        },
        SolverFailureAction::PermanentBan,
    ]
}

/// Determine actions when a solver's bundle fails settlement verification.
///
/// Rule: escalating penalty based on consecutive failures.
pub fn handle_settlement_failure(consecutive_failures: u32) -> Vec<SolverFailureAction> {
    let (slash_bps, violation_delta) = match consecutive_failures {
        0..=1 => (0, 100),     // warning only
        2..=3 => (100, 500),   // 1% slash
        4..=5 => (500, 1000),  // 5% slash
        _ => (2500, 2000),     // 25% slash
    };

    let mut actions = vec![SolverFailureAction::ApplyPenalty { slash_bps, violation_delta }];

    if consecutive_failures >= 6 {
        actions.push(SolverFailureAction::Deactivate);
    }

    actions
}

/// Determine actions when a solver attempts a double fill.
///
/// Rule: 100% bond slash + permanent ban.
pub fn handle_double_fill() -> Vec<SolverFailureAction> {
    vec![
        SolverFailureAction::ApplyPenalty {
            slash_bps: 10_000,
            violation_delta: 10_000,
        },
        SolverFailureAction::PermanentBan,
    ]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_commit_without_reveal() {
        let actions = handle_commit_without_reveal();
        assert_eq!(actions.len(), 1);
        match &actions[0] {
            SolverFailureAction::ApplyPenalty { slash_bps, .. } => assert_eq!(*slash_bps, 100),
            _ => panic!("expected ApplyPenalty"),
        }
    }

    #[test]
    fn test_invalid_reveal_permanent_ban() {
        let actions = handle_invalid_reveal();
        assert!(actions.iter().any(|a| matches!(a, SolverFailureAction::PermanentBan)));
        match &actions[0] {
            SolverFailureAction::ApplyPenalty { slash_bps, .. } => assert_eq!(*slash_bps, 10_000),
            _ => panic!("expected ApplyPenalty"),
        }
    }

    #[test]
    fn test_settlement_failure_escalation() {
        // First failure: warning only
        let a1 = handle_settlement_failure(1);
        match &a1[0] {
            SolverFailureAction::ApplyPenalty { slash_bps, .. } => assert_eq!(*slash_bps, 0),
            _ => panic!("expected ApplyPenalty"),
        }

        // 4th failure: 5% slash
        let a4 = handle_settlement_failure(4);
        match &a4[0] {
            SolverFailureAction::ApplyPenalty { slash_bps, .. } => assert_eq!(*slash_bps, 500),
            _ => panic!("expected ApplyPenalty"),
        }

        // 6th failure: 25% slash + deactivation
        let a6 = handle_settlement_failure(6);
        assert!(a6.iter().any(|a| matches!(a, SolverFailureAction::Deactivate)));
    }

    #[test]
    fn test_double_fill_ban() {
        let actions = handle_double_fill();
        assert!(actions.iter().any(|a| matches!(a, SolverFailureAction::PermanentBan)));
    }
}
