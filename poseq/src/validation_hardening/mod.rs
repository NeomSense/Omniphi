#![allow(dead_code)]

use std::collections::BTreeSet;

use crate::bridge_recovery::BridgeDeliveryState;
use crate::finality::FinalityState;

// ---------------------------------------------------------------------------
// EpochAuthorityCheck
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EpochAuthorityCheck {
    pub batch_id: [u8; 32],
    pub proposal_epoch: u64,
    pub current_epoch: u64,
    pub proposer: [u8; 32],
    pub is_stale: bool,
    pub is_future: bool,
}

impl EpochAuthorityCheck {
    pub fn check(
        batch_id: [u8; 32],
        proposal_epoch: u64,
        current_epoch: u64,
        proposer: [u8; 32],
    ) -> Self {
        EpochAuthorityCheck {
            batch_id,
            proposal_epoch,
            current_epoch,
            proposer,
            is_stale: proposal_epoch < current_epoch,
            is_future: proposal_epoch > current_epoch,
        }
    }

    pub fn is_valid(&self) -> bool {
        !self.is_stale && !self.is_future
    }
}

// ---------------------------------------------------------------------------
// ValidationFailureReason
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub enum ValidationFailureReason {
    StaleCommitteeUsed {
        proposal_epoch: u64,
        current_epoch: u64,
    },
    /// FIND-015: distinct from StaleCommitteeUsed — indicates clock manipulation or
    /// slot pre-emption attack rather than a lagged/crashed node.
    FutureEpochProposal {
        proposal_epoch: u64,
        current_epoch: u64,
    },
    WrongSlotAuthority {
        slot: u64,
        expected_leader: [u8; 32],
        actual_proposer: [u8; 32],
    },
    /// FIND-006: leader assignment unavailable (e.g. during recovery window).
    /// Callers must handle this explicitly — it does not auto-pass.
    LeaderAssignmentUnavailable {
        slot: u64,
    },
    IllegalFinalityTransition {
        from: String,
        to: String,
    },
    ConflictingBatchCommitment {
        batch_id: [u8; 32],
    },
    InvalidBridgeExportState {
        batch_id: [u8; 32],
        state: String,
    },
    InvalidRecoveryState {
        reason: String,
    },
    MalformedFairnessEnvelope,
    UnauthorizedCommitteeParticipant([u8; 32]),
}

// ---------------------------------------------------------------------------
// ValidationReport
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ValidationReport {
    pub batch_id: [u8; 32],
    pub passed: bool,
    pub failures: Vec<ValidationFailureReason>,
}

impl ValidationReport {
    pub fn pass(batch_id: [u8; 32]) -> Self {
        ValidationReport {
            batch_id,
            passed: true,
            failures: Vec::new(),
        }
    }

    pub fn fail(batch_id: [u8; 32], reason: ValidationFailureReason) -> Self {
        ValidationReport {
            batch_id,
            passed: false,
            failures: vec![reason],
        }
    }
}

// ---------------------------------------------------------------------------
// ValidationHardeningError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum ValidationHardeningError {
    InvalidEpoch { epoch: u64 },
    CommitteeEmpty,
    ProposerNotInCommittee([u8; 32]),
}

// ---------------------------------------------------------------------------
// FinalizationValidator
// ---------------------------------------------------------------------------

pub struct FinalizationValidator;

impl FinalizationValidator {
    pub fn validate_epoch_authority(
        batch_id: [u8; 32],
        proposal_epoch: u64,
        current_epoch: u64,
        proposer: [u8; 32],
        active_committee: &BTreeSet<[u8; 32]>,
    ) -> ValidationReport {
        let check = EpochAuthorityCheck::check(batch_id, proposal_epoch, current_epoch, proposer);

        if check.is_stale {
            return ValidationReport::fail(
                batch_id,
                ValidationFailureReason::StaleCommitteeUsed {
                    proposal_epoch,
                    current_epoch,
                },
            );
        }

        // FIND-015: future epoch proposals are a distinct attack pattern
        // (clock manipulation / slot pre-emption), not simply a lagged node.
        if check.is_future {
            return ValidationReport::fail(
                batch_id,
                ValidationFailureReason::FutureEpochProposal {
                    proposal_epoch,
                    current_epoch,
                },
            );
        }

        if !active_committee.contains(&proposer) {
            return ValidationReport::fail(
                batch_id,
                ValidationFailureReason::UnauthorizedCommitteeParticipant(proposer),
            );
        }

        ValidationReport::pass(batch_id)
    }

    pub fn validate_slot_authority(
        batch_id: [u8; 32],
        slot: u64,
        proposer: [u8; 32],
        expected_leader: Option<[u8; 32]>,
    ) -> ValidationReport {
        match expected_leader {
            // FIND-006: no leader assignment must NOT auto-pass — callers must handle
            // this explicitly (e.g., block proposal until assignment is available, or
            // queue for deferred re-validation once the leader store is repopulated).
            None => ValidationReport::fail(
                batch_id,
                ValidationFailureReason::LeaderAssignmentUnavailable { slot },
            ),
            Some(leader) if leader == proposer => ValidationReport::pass(batch_id),
            Some(leader) => ValidationReport::fail(
                batch_id,
                ValidationFailureReason::WrongSlotAuthority {
                    slot,
                    expected_leader: leader,
                    actual_proposer: proposer,
                },
            ),
        }
    }
}

// ---------------------------------------------------------------------------
// TransitionSafetyValidator
// ---------------------------------------------------------------------------

pub struct TransitionSafetyValidator;

impl TransitionSafetyValidator {
    pub fn validate_finality_transition(
        batch_id: [u8; 32],
        from: &FinalityState,
        to: &FinalityState,
    ) -> ValidationReport {
        if from.can_transition_to(to) {
            ValidationReport::pass(batch_id)
        } else {
            ValidationReport::fail(
                batch_id,
                ValidationFailureReason::IllegalFinalityTransition {
                    from: format!("{:?}", from),
                    to: format!("{:?}", to),
                },
            )
        }
    }
}

// ---------------------------------------------------------------------------
// BridgeStateValidator
// ---------------------------------------------------------------------------

pub struct BridgeStateValidator;

impl BridgeStateValidator {
    pub fn validate_export_state(
        batch_id: [u8; 32],
        delivery_state: &BridgeDeliveryState,
    ) -> ValidationReport {
        match delivery_state {
            BridgeDeliveryState::Pending | BridgeDeliveryState::RetryPending { .. } => {
                ValidationReport::pass(batch_id)
            }
            BridgeDeliveryState::Acknowledged
            | BridgeDeliveryState::RecoveredAck
            | BridgeDeliveryState::Failed => ValidationReport::fail(
                batch_id,
                ValidationFailureReason::InvalidBridgeExportState {
                    batch_id,
                    state: format!("{:?}", delivery_state),
                },
            ),
            _ => ValidationReport::pass(batch_id),
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn batch(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn committee(nodes: &[u8]) -> BTreeSet<[u8; 32]> {
        nodes.iter().map(|&n| node(n)).collect()
    }

    #[test]
    fn test_epoch_authority_valid() {
        let report = FinalizationValidator::validate_epoch_authority(
            batch(1),
            5,
            5,
            node(1),
            &committee(&[1, 2, 3]),
        );
        assert!(report.passed);
    }

    #[test]
    fn test_epoch_authority_stale() {
        let report = FinalizationValidator::validate_epoch_authority(
            batch(2),
            3,
            5,
            node(1),
            &committee(&[1, 2, 3]),
        );
        assert!(!report.passed);
        assert!(matches!(
            report.failures[0],
            ValidationFailureReason::StaleCommitteeUsed { .. }
        ));
    }

    #[test]
    fn test_epoch_authority_future_classified_as_future_not_stale() {
        // FIND-015: future epoch must use FutureEpochProposal, not StaleCommitteeUsed
        let report = FinalizationValidator::validate_epoch_authority(
            batch(3),
            7,
            5,
            node(1),
            &committee(&[1, 2, 3]),
        );
        assert!(!report.passed);
        assert!(
            matches!(
                report.failures[0],
                ValidationFailureReason::FutureEpochProposal { .. }
            ),
            "future epoch must produce FutureEpochProposal, got {:?}",
            report.failures[0]
        );
    }

    #[test]
    fn test_epoch_authority_unauthorized_proposer() {
        let report = FinalizationValidator::validate_epoch_authority(
            batch(4),
            5,
            5,
            node(99), // not in committee
            &committee(&[1, 2, 3]),
        );
        assert!(!report.passed);
        assert!(matches!(
            report.failures[0],
            ValidationFailureReason::UnauthorizedCommitteeParticipant(_)
        ));
    }

    #[test]
    fn test_slot_authority_correct_leader() {
        let report =
            FinalizationValidator::validate_slot_authority(batch(5), 10, node(1), Some(node(1)));
        assert!(report.passed);
    }

    #[test]
    fn test_slot_authority_wrong_leader() {
        let report =
            FinalizationValidator::validate_slot_authority(batch(6), 10, node(2), Some(node(1)));
        assert!(!report.passed);
        assert!(matches!(
            report.failures[0],
            ValidationFailureReason::WrongSlotAuthority { .. }
        ));
    }

    #[test]
    fn test_slot_authority_no_assignment_does_not_auto_pass() {
        // FIND-006: None leader assignment must fail, not pass
        let report =
            FinalizationValidator::validate_slot_authority(batch(7), 10, node(3), None);
        assert!(!report.passed, "None leader assignment must not auto-pass");
        assert!(
            matches!(
                report.failures[0],
                ValidationFailureReason::LeaderAssignmentUnavailable { slot: 10 }
            ),
            "must produce LeaderAssignmentUnavailable, got {:?}",
            report.failures[0]
        );
    }

    #[test]
    fn test_finality_transition_valid() {
        let report = TransitionSafetyValidator::validate_finality_transition(
            batch(10),
            &FinalityState::Proposed,
            &FinalityState::Attested,
        );
        assert!(report.passed);
    }

    #[test]
    fn test_finality_transition_invalid() {
        let report = TransitionSafetyValidator::validate_finality_transition(
            batch(11),
            &FinalityState::Proposed,
            &FinalityState::Finalized,
        );
        assert!(!report.passed);
        assert!(matches!(
            report.failures[0],
            ValidationFailureReason::IllegalFinalityTransition { .. }
        ));
    }

    #[test]
    fn test_bridge_state_pending_valid() {
        let report =
            BridgeStateValidator::validate_export_state(batch(20), &BridgeDeliveryState::Pending);
        assert!(report.passed);
    }

    #[test]
    fn test_bridge_state_acknowledged_invalid() {
        let report = BridgeStateValidator::validate_export_state(
            batch(21),
            &BridgeDeliveryState::Acknowledged,
        );
        assert!(!report.passed);
    }

    #[test]
    fn test_epoch_authority_check_is_valid() {
        let check = EpochAuthorityCheck::check(batch(30), 5, 5, node(1));
        assert!(check.is_valid());
        let stale = EpochAuthorityCheck::check(batch(31), 3, 5, node(1));
        assert!(!stale.is_valid());
    }
}
