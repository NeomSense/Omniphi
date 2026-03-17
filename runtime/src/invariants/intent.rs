//! Intent invariants — constraints that must always hold for intents.

use std::collections::BTreeSet;

/// Intent invariant: an intent may settle at most once.
///
/// Given a set of settled intent IDs and a candidate, returns true if the
/// candidate has NOT already been settled.
pub fn check_no_double_settlement(
    settled_intents: &BTreeSet<[u8; 32]>,
    intent_id: &[u8; 32],
) -> Result<(), InvariantViolation> {
    if settled_intents.contains(intent_id) {
        Err(InvariantViolation::DoubleSettlement { intent_id: *intent_id })
    } else {
        Ok(())
    }
}

/// Intent invariant: expired intent cannot execute.
///
/// An intent whose deadline <= current_block must not be executed.
pub fn check_not_expired(
    intent_deadline: u64,
    current_block: u64,
) -> Result<(), InvariantViolation> {
    if current_block > intent_deadline {
        Err(InvariantViolation::ExpiredIntentExecution {
            deadline: intent_deadline,
            current_block,
        })
    } else {
        Ok(())
    }
}

/// Intent invariant: cancelled intent cannot execute.
pub fn check_not_cancelled(
    cancelled_intents: &BTreeSet<[u8; 32]>,
    intent_id: &[u8; 32],
) -> Result<(), InvariantViolation> {
    if cancelled_intents.contains(intent_id) {
        Err(InvariantViolation::CancelledIntentExecution { intent_id: *intent_id })
    } else {
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InvariantViolation {
    DoubleSettlement { intent_id: [u8; 32] },
    ExpiredIntentExecution { deadline: u64, current_block: u64 },
    CancelledIntentExecution { intent_id: [u8; 32] },
}

impl std::fmt::Display for InvariantViolation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::DoubleSettlement { intent_id } => {
                write!(f, "INVARIANT VIOLATION: intent {} settled twice", hex::encode(&intent_id[..4]))
            }
            Self::ExpiredIntentExecution { deadline, current_block } => {
                write!(f, "INVARIANT VIOLATION: expired intent executed (deadline={}, block={})", deadline, current_block)
            }
            Self::CancelledIntentExecution { intent_id } => {
                write!(f, "INVARIANT VIOLATION: cancelled intent {} executed", hex::encode(&intent_id[..4]))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_double_settlement() {
        let mut settled = BTreeSet::new();
        let id = [1u8; 32];
        assert!(check_no_double_settlement(&settled, &id).is_ok());

        settled.insert(id);
        assert_eq!(
            check_no_double_settlement(&settled, &id),
            Err(InvariantViolation::DoubleSettlement { intent_id: id })
        );
    }

    #[test]
    fn test_not_expired() {
        assert!(check_not_expired(100, 50).is_ok());
        assert!(check_not_expired(100, 100).is_ok());
        assert!(check_not_expired(100, 101).is_err());
    }

    #[test]
    fn test_not_cancelled() {
        let mut cancelled = BTreeSet::new();
        let id = [1u8; 32];
        assert!(check_not_cancelled(&cancelled, &id).is_ok());

        cancelled.insert(id);
        assert!(check_not_cancelled(&cancelled, &id).is_err());
    }
}
