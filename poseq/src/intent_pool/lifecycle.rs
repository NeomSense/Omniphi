//! Intent lifecycle state machine per Section 2.5 of the architecture spec.
//!
//! Tracks each intent through: Created → Admitted → Open → Matched → Sequenced → Executed → Settled.
//! Terminal states: Expired, Cancelled, Disputed, Slashed.

use serde::{Deserialize, Serialize};

/// Intent lifecycle state.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum IntentState {
    Created,
    Admitted,
    Open,
    Matched,
    Sequenced,
    Executed,
    Settled,
    // Terminal
    Expired,
    Cancelled,
    Disputed,
    Slashed,
}

impl IntentState {
    /// Whether this state is terminal (no further transitions).
    pub fn is_terminal(&self) -> bool {
        matches!(self, Self::Settled | Self::Expired | Self::Cancelled | Self::Slashed)
    }

    /// Whether this state is active (intent still in play).
    pub fn is_active(&self) -> bool {
        matches!(self, Self::Created | Self::Admitted | Self::Open | Self::Matched | Self::Sequenced | Self::Executed | Self::Disputed)
    }

    /// Whether the intent can be cancelled from this state.
    pub fn is_cancellable(&self) -> bool {
        matches!(self, Self::Open | Self::Matched)
    }

    /// Single-byte encoding for storage keys.
    pub fn as_byte(&self) -> u8 {
        match self {
            Self::Created   => 0x00,
            Self::Admitted  => 0x01,
            Self::Open      => 0x02,
            Self::Matched   => 0x03,
            Self::Sequenced => 0x04,
            Self::Executed  => 0x05,
            Self::Settled   => 0x06,
            Self::Expired   => 0x07,
            Self::Cancelled => 0x08,
            Self::Disputed  => 0x09,
            Self::Slashed   => 0x0A,
        }
    }
}

/// Errors that can occur during state transitions.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TransitionError {
    InvalidTransition { from: IntentState, to: IntentState },
}

impl std::fmt::Display for TransitionError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidTransition { from, to } => {
                write!(f, "invalid state transition: {:?} → {:?}", from, to)
            }
        }
    }
}

impl std::error::Error for TransitionError {}

/// Validate and apply a state transition according to the spec's transition table.
pub fn transition(current: IntentState, target: IntentState) -> Result<IntentState, TransitionError> {
    let valid = match (current, target) {
        // Happy path
        (IntentState::Created,   IntentState::Admitted)  => true,
        (IntentState::Admitted,  IntentState::Open)      => true,
        (IntentState::Open,      IntentState::Matched)   => true,
        (IntentState::Matched,   IntentState::Sequenced) => true,
        (IntentState::Sequenced, IntentState::Executed)  => true,
        (IntentState::Executed,  IntentState::Settled)   => true,

        // Expiry (from Open or Matched)
        (IntentState::Open,    IntentState::Expired)    => true,
        (IntentState::Matched, IntentState::Expired)    => true,

        // Cancellation (from Open or Matched)
        (IntentState::Open,    IntentState::Cancelled)  => true,
        (IntentState::Matched, IntentState::Cancelled)  => true,

        // Dispute (from Executed)
        (IntentState::Executed, IntentState::Disputed)  => true,

        // Slash (from Disputed)
        (IntentState::Disputed, IntentState::Slashed)   => true,

        _ => false,
    };

    if valid {
        Ok(target)
    } else {
        Err(TransitionError::InvalidTransition { from: current, to: target })
    }
}

/// Tracked record of an intent's state within the pool/system.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentLifecycleRecord {
    pub intent_id: [u8; 32],
    pub state: IntentState,
    pub admitted_at_block: u64,
    pub last_transition_block: u64,
    pub matched_bundle_id: Option<[u8; 32]>,
    pub solver_id: Option<[u8; 32]>,
    pub receipt_id: Option<[u8; 32]>,
}

impl IntentLifecycleRecord {
    pub fn new(intent_id: [u8; 32], block: u64) -> Self {
        IntentLifecycleRecord {
            intent_id,
            state: IntentState::Admitted,
            admitted_at_block: block,
            last_transition_block: block,
            matched_bundle_id: None,
            solver_id: None,
            receipt_id: None,
        }
    }

    pub fn transition_to(&mut self, target: IntentState, block: u64) -> Result<(), TransitionError> {
        let new_state = transition(self.state, target)?;
        self.state = new_state;
        self.last_transition_block = block;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_happy_path_transitions() {
        let mut record = IntentLifecycleRecord::new([1u8; 32], 100);
        assert_eq!(record.state, IntentState::Admitted);

        assert!(record.transition_to(IntentState::Open, 101).is_ok());
        assert!(record.transition_to(IntentState::Matched, 102).is_ok());
        assert!(record.transition_to(IntentState::Sequenced, 103).is_ok());
        assert!(record.transition_to(IntentState::Executed, 104).is_ok());
        assert!(record.transition_to(IntentState::Settled, 105).is_ok());

        assert!(record.state.is_terminal());
    }

    #[test]
    fn test_invalid_transition() {
        // Cannot go from Created directly to Executed
        assert!(transition(IntentState::Created, IntentState::Executed).is_err());
        // Cannot go backward
        assert!(transition(IntentState::Matched, IntentState::Open).is_err());
        // Cannot settle without executing
        assert!(transition(IntentState::Sequenced, IntentState::Settled).is_err());
    }

    #[test]
    fn test_cancellation() {
        assert!(transition(IntentState::Open, IntentState::Cancelled).is_ok());
        assert!(transition(IntentState::Matched, IntentState::Cancelled).is_ok());
        // Cannot cancel after sequencing
        assert!(transition(IntentState::Sequenced, IntentState::Cancelled).is_err());
    }

    #[test]
    fn test_dispute_and_slash() {
        assert!(transition(IntentState::Executed, IntentState::Disputed).is_ok());
        assert!(transition(IntentState::Disputed, IntentState::Slashed).is_ok());
        // Cannot dispute settled
        assert!(transition(IntentState::Settled, IntentState::Disputed).is_err());
    }

    #[test]
    fn test_expiry() {
        assert!(transition(IntentState::Open, IntentState::Expired).is_ok());
        assert!(transition(IntentState::Matched, IntentState::Expired).is_ok());
        // Cannot expire admitted (must be Open first)
        assert!(transition(IntentState::Admitted, IntentState::Expired).is_err());
    }

    #[test]
    fn test_terminal_states() {
        assert!(IntentState::Settled.is_terminal());
        assert!(IntentState::Expired.is_terminal());
        assert!(IntentState::Cancelled.is_terminal());
        assert!(IntentState::Slashed.is_terminal());
        assert!(!IntentState::Open.is_terminal());
    }
}
