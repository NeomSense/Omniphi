//! Receipt invariants — constraints on execution receipts.

use std::collections::BTreeSet;

/// Receipt invariant: execution receipts must match finalized bundles.
///
/// Every receipt's bundle_id must be in the set of finalized bundle IDs.
pub fn check_receipts_match_finalized(
    receipt_bundle_ids: &[[u8; 32]],
    finalized_bundle_ids: &BTreeSet<[u8; 32]>,
) -> Result<(), ReceiptInvariantViolation> {
    for id in receipt_bundle_ids {
        if !finalized_bundle_ids.contains(id) {
            return Err(ReceiptInvariantViolation::UnfinalizedBundleReceipt { bundle_id: *id });
        }
    }
    Ok(())
}

/// Receipt invariant: double-fill detection must trigger invariant violation.
///
/// If the same intent_id appears in two successful receipts, this is a violation.
pub fn check_no_double_fill(
    successful_receipt_intent_ids: &[[u8; 32]],
) -> Result<(), ReceiptInvariantViolation> {
    let mut seen = BTreeSet::new();
    for id in successful_receipt_intent_ids {
        if !seen.insert(*id) {
            return Err(ReceiptInvariantViolation::DoubleFill { intent_id: *id });
        }
    }
    Ok(())
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReceiptInvariantViolation {
    UnfinalizedBundleReceipt { bundle_id: [u8; 32] },
    DoubleFill { intent_id: [u8; 32] },
}

impl std::fmt::Display for ReceiptInvariantViolation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::UnfinalizedBundleReceipt { bundle_id } => {
                write!(f, "INVARIANT VIOLATION: receipt for unfinalized bundle {}", hex::encode(&bundle_id[..4]))
            }
            Self::DoubleFill { intent_id } => {
                write!(f, "INVARIANT VIOLATION: double fill for intent {}", hex::encode(&intent_id[..4]))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_receipts_match_finalized() {
        let mut finalized = BTreeSet::new();
        finalized.insert([1u8; 32]);
        finalized.insert([2u8; 32]);

        assert!(check_receipts_match_finalized(&[[1u8; 32], [2u8; 32]], &finalized).is_ok());
        assert!(check_receipts_match_finalized(&[[3u8; 32]], &finalized).is_err());
    }

    #[test]
    fn test_no_double_fill() {
        assert!(check_no_double_fill(&[[1u8; 32], [2u8; 32]]).is_ok());
        assert!(check_no_double_fill(&[[1u8; 32], [1u8; 32]]).is_err());
    }
}
