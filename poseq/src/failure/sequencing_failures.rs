//! Sequencing failure paths — deterministic handling of sequencer/leader failures.

use serde::{Deserialize, Serialize};

/// Actions to take on sequencer failure.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum SequencingFailureAction {
    /// New leader re-proposes the sequence.
    LeaderRotation,
    /// Ordering root must remain identical after leader change.
    PreserveOrderingRoot,
    /// Force epoch transition if failures persist.
    ForceEpochTransition,
    /// Slash the failed leader.
    SlashLeader { slash_bps: u64 },
}

/// Determine actions when the sequencer/leader fails to propose.
///
/// Rule: new leader re-proposes sequence; ordering root must remain identical.
pub fn handle_leader_failure(consecutive_failures: u32, threshold: u32) -> Vec<SequencingFailureAction> {
    let mut actions = vec![
        SequencingFailureAction::LeaderRotation,
        SequencingFailureAction::PreserveOrderingRoot,
    ];

    if consecutive_failures >= threshold {
        actions.push(SequencingFailureAction::ForceEpochTransition);
    }

    actions
}

/// Determine actions when a proposed sequence has an invalid ordering root.
///
/// Rule: reject the proposal, slash the proposer, rotate leader.
pub fn handle_invalid_ordering_root() -> Vec<SequencingFailureAction> {
    vec![
        SequencingFailureAction::SlashLeader { slash_bps: 500 }, // 5%
        SequencingFailureAction::LeaderRotation,
    ]
}

/// Verify that two ordering roots match (used after leader rotation).
///
/// The new leader must produce the same ordering root from the same inputs.
pub fn verify_ordering_root_preserved(
    original_root: [u8; 32],
    new_root: [u8; 32],
) -> bool {
    original_root == new_root
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_leader_failure_rotation() {
        let actions = handle_leader_failure(1, 3);
        assert!(actions.iter().any(|a| matches!(a, SequencingFailureAction::LeaderRotation)));
        assert!(actions.iter().any(|a| matches!(a, SequencingFailureAction::PreserveOrderingRoot)));
        assert!(!actions.iter().any(|a| matches!(a, SequencingFailureAction::ForceEpochTransition)));
    }

    #[test]
    fn test_leader_failure_epoch_transition() {
        let actions = handle_leader_failure(3, 3);
        assert!(actions.iter().any(|a| matches!(a, SequencingFailureAction::ForceEpochTransition)));
    }

    #[test]
    fn test_invalid_ordering_root_slash() {
        let actions = handle_invalid_ordering_root();
        assert!(actions.iter().any(|a| matches!(a, SequencingFailureAction::SlashLeader { .. })));
    }

    #[test]
    fn test_ordering_root_preserved() {
        let root = [0xAA; 32];
        assert!(verify_ordering_root_preserved(root, root));
        assert!(!verify_ordering_root_preserved(root, [0xBB; 32]));
    }
}
