//! Bundle invariants — constraints that must always hold for bundles in a sequence.

use std::collections::BTreeSet;

/// Bundle invariant: a bundle can appear only once per sequence.
pub fn check_no_duplicate_bundles(
    bundle_ids: &[[u8; 32]],
) -> Result<(), BundleInvariantViolation> {
    let mut seen = BTreeSet::new();
    for id in bundle_ids {
        if !seen.insert(*id) {
            return Err(BundleInvariantViolation::DuplicateBundle { bundle_id: *id });
        }
    }
    Ok(())
}

/// Bundle invariant: bundle cannot reference a fulfilled intent.
pub fn check_no_fulfilled_intent_reference(
    bundle_intent_ids: &[[u8; 32]],
    fulfilled_intents: &BTreeSet<[u8; 32]>,
) -> Result<(), BundleInvariantViolation> {
    for id in bundle_intent_ids {
        if fulfilled_intents.contains(id) {
            return Err(BundleInvariantViolation::FulfilledIntentReference { intent_id: *id });
        }
    }
    Ok(())
}

/// Bundle invariant: bundle must have passed verification before settlement.
/// This is a bookkeeping check — the verification flag must be set.
pub fn check_bundle_verified(
    bundle_id: &[u8; 32],
    verified_bundles: &BTreeSet<[u8; 32]>,
) -> Result<(), BundleInvariantViolation> {
    if !verified_bundles.contains(bundle_id) {
        Err(BundleInvariantViolation::UnverifiedBundle { bundle_id: *bundle_id })
    } else {
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BundleInvariantViolation {
    DuplicateBundle { bundle_id: [u8; 32] },
    FulfilledIntentReference { intent_id: [u8; 32] },
    UnverifiedBundle { bundle_id: [u8; 32] },
}

impl std::fmt::Display for BundleInvariantViolation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::DuplicateBundle { bundle_id } => {
                write!(f, "INVARIANT VIOLATION: duplicate bundle {}", hex::encode(&bundle_id[..4]))
            }
            Self::FulfilledIntentReference { intent_id } => {
                write!(f, "INVARIANT VIOLATION: bundle references fulfilled intent {}", hex::encode(&intent_id[..4]))
            }
            Self::UnverifiedBundle { bundle_id } => {
                write!(f, "INVARIANT VIOLATION: unverified bundle {} in settlement", hex::encode(&bundle_id[..4]))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_duplicate_bundles() {
        assert!(check_no_duplicate_bundles(&[[1u8; 32], [2u8; 32]]).is_ok());
        assert!(check_no_duplicate_bundles(&[[1u8; 32], [1u8; 32]]).is_err());
    }

    #[test]
    fn test_no_fulfilled_intent_reference() {
        let mut fulfilled = BTreeSet::new();
        fulfilled.insert([10u8; 32]);

        assert!(check_no_fulfilled_intent_reference(&[[20u8; 32]], &fulfilled).is_ok());
        assert!(check_no_fulfilled_intent_reference(&[[10u8; 32]], &fulfilled).is_err());
    }

    #[test]
    fn test_bundle_verified() {
        let mut verified = BTreeSet::new();
        verified.insert([1u8; 32]);

        assert!(check_bundle_verified(&[1u8; 32], &verified).is_ok());
        assert!(check_bundle_verified(&[2u8; 32], &verified).is_err());
    }
}
