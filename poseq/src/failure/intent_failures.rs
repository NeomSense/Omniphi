//! Intent failure paths — deterministic handling of intent-related failures.

use serde::{Deserialize, Serialize};

/// Actions to take when an intent fails during auction.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum IntentFailureAction {
    /// Return intent to pool for future matching.
    ReturnToPool,
    /// Remove intent permanently (expired or cancelled).
    Remove,
    /// Invalidate any bundles targeting this intent.
    InvalidateBundles,
}

/// Determine the correct action when an intent expires during auction.
///
/// Rule: Intent expired during auction → bundle invalidated, intent removed.
pub fn handle_expired_during_auction(
    intent_deadline: u64,
    current_block: u64,
) -> Vec<IntentFailureAction> {
    if current_block > intent_deadline {
        vec![
            IntentFailureAction::InvalidateBundles,
            IntentFailureAction::Remove,
        ]
    } else {
        vec![] // not expired, no action
    }
}

/// Determine the correct action when no solver submits a reveal for an intent.
///
/// Rule: No reveals → intent returns to pool, no reward, no penalty.
pub fn handle_no_reveals_for_intent() -> Vec<IntentFailureAction> {
    vec![IntentFailureAction::ReturnToPool]
}

/// Determine the correct action when the winning bundle fails verification.
///
/// Rule: Invalid winning bundle → reject bundle, select next eligible.
pub fn handle_invalid_winning_bundle() -> InvalidBundleResolution {
    InvalidBundleResolution::SelectNextEligible
}

/// Resolution strategy for an invalid winning bundle.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InvalidBundleResolution {
    /// Select the next highest-scoring eligible bundle for the same intent.
    SelectNextEligible,
    /// No eligible bundle available — return intent to pool.
    ReturnIntentToPool,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_expired_during_auction() {
        let actions = handle_expired_during_auction(100, 101);
        assert_eq!(actions.len(), 2);
        assert_eq!(actions[0], IntentFailureAction::InvalidateBundles);
        assert_eq!(actions[1], IntentFailureAction::Remove);
    }

    #[test]
    fn test_not_expired() {
        let actions = handle_expired_during_auction(100, 50);
        assert!(actions.is_empty());
    }

    #[test]
    fn test_no_reveals() {
        let actions = handle_no_reveals_for_intent();
        assert_eq!(actions, vec![IntentFailureAction::ReturnToPool]);
    }

    #[test]
    fn test_invalid_winning_bundle() {
        assert_eq!(
            handle_invalid_winning_bundle(),
            InvalidBundleResolution::SelectNextEligible,
        );
    }
}
