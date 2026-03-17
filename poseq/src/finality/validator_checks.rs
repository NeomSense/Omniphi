//! Phase 3 — Validator pre-vote checks for FinalityCommitment.
//!
//! Before casting a HotStuff vote, validators must verify the commitment.
//! This module implements the full verification checklist from the spec.

use std::collections::{BTreeMap, BTreeSet};

use super::commitment::{CommitmentVerificationError, FinalityCommitment};
use crate::auction::da_checks::{validate_da, DAValidationResult};
use crate::auction::ordering::compute_sequence_root;
use crate::auction::types::{BundleCommitment, SequencedBundle};

/// Full pre-vote verification of a FinalityCommitment.
///
/// Checks:
/// 1. Ordering root matches recomputed sequence_root from ordered bundles
/// 2. Bundle root matches recomputed root from commitment hashes
/// 3. Fairness root matches provided fairness_root
/// 4. No duplicate bundle inclusion
/// 5. All referenced intents exist
/// 6. All bundles passed DA checks
/// 7. Sequence ID is previous + 1
/// 8. Proposer is a known validator
pub fn verify_commitment(
    commitment: &FinalityCommitment,
    ordered_bundles: &[SequencedBundle],
    available_commitments: &BTreeMap<[u8; 32], BundleCommitment>,
    known_intent_ids: &BTreeSet<[u8; 32]>,
    active_solver_ids: &BTreeSet<[u8; 32]>,
    known_validators: &BTreeSet<[u8; 32]>,
    expected_fairness_root: [u8; 32],
    previous_commitment_root: Option<[u8; 32]>,
    expected_sequence_id: u64,
    current_block: u64,
) -> Result<(), CommitmentVerificationError> {
    // 1. No duplicate bundles (cheap fast-fail before expensive root recomputation)
    let mut seen_bundles = BTreeSet::new();
    for bid in &commitment.ordered_bundle_ids {
        if !seen_bundles.insert(*bid) {
            return Err(CommitmentVerificationError::DuplicateBundle(*bid));
        }
    }

    // 2. Ordering root
    let recomputed_ordering_root = compute_sequence_root(ordered_bundles);
    if commitment.ordering_root != recomputed_ordering_root {
        return Err(CommitmentVerificationError::OrderingRootMismatch {
            expected: recomputed_ordering_root,
            actual: commitment.ordering_root,
        });
    }

    // 3. Bundle root from commitment hashes
    let commitment_hashes: Vec<[u8; 32]> = commitment.ordered_bundle_ids.iter()
        .filter_map(|bid| available_commitments.get(bid).map(|c| c.commitment_hash))
        .collect();
    let recomputed_bundle_root = FinalityCommitment::compute_bundle_root(&commitment_hashes);
    if commitment.bundle_root != recomputed_bundle_root {
        return Err(CommitmentVerificationError::BundleRootMismatch {
            expected: recomputed_bundle_root,
            actual: commitment.bundle_root,
        });
    }

    // 4. Fairness root
    if commitment.fairness_root != expected_fairness_root {
        return Err(CommitmentVerificationError::FairnessRootMismatch {
            expected: expected_fairness_root,
            actual: commitment.fairness_root,
        });
    }

    // 5. All referenced intents exist
    for iid in &commitment.intent_ids {
        if !known_intent_ids.contains(iid) {
            return Err(CommitmentVerificationError::MissingIntent(*iid));
        }
    }

    // 6. DA checks pass
    let da_result = validate_da(
        ordered_bundles,
        available_commitments,
        known_intent_ids,
        active_solver_ids,
        current_block,
        true, // payload is complete if we have the bundles
    );
    if !da_result.passed {
        let msg = da_result.failures.iter()
            .map(|f| format!("{:?}", f))
            .collect::<Vec<_>>()
            .join("; ");
        return Err(CommitmentVerificationError::DACheckFailed(msg));
    }

    // 7. Sequence continuity
    if commitment.sequence_id != expected_sequence_id {
        return Err(CommitmentVerificationError::SequenceGap {
            expected: expected_sequence_id,
            actual: commitment.sequence_id,
        });
    }

    // 8. Previous commitment
    if let Some(prev_root) = previous_commitment_root {
        if commitment.previous_commitment != prev_root {
            return Err(CommitmentVerificationError::InvalidPreviousCommitment);
        }
    }

    // 9. Proposer is a known validator
    if !known_validators.contains(&commitment.proposer) {
        return Err(CommitmentVerificationError::InvalidProposer(commitment.proposer));
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::auction::ordering::compute_sequence_root;
    use crate::auction::types::*;
    use crate::intent_pool::types::{AssetId, AssetType};

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32]; id[0] = b;
        AssetId { chain_id: 0, asset_type: AssetType::Token, identifier: id }
    }

    fn setup() -> (
        FinalityCommitment,
        Vec<SequencedBundle>,
        BTreeMap<[u8; 32], BundleCommitment>,
        BTreeSet<[u8; 32]>,
        BTreeSet<[u8; 32]>,
        BTreeSet<[u8; 32]>,
    ) {
        let bundle_id = [1u8; 32];
        let solver_id = [10u8; 32];
        let intent_id = [100u8; 32];
        let proposer = [0xFF; 32];

        let bundle = SequencedBundle {
            bundle_id,
            solver_id,
            target_intent_ids: vec![intent_id],
            execution_steps: vec![],
            predicted_outputs: vec![],
            resource_declarations: vec![],
            sequence_index: 0,
        };

        let ordering_root = compute_sequence_root(&[bundle.clone()]);

        let commitment_obj = BundleCommitment {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_count: 1,
            commitment_hash: [0xAA; 32],
            expected_outputs_hash: [0xBB; 32],
            execution_plan_hash: [0xCC; 32],
            valid_until: 200,
            bond_locked: 50_000,
            signature: vec![1u8; 64],
        };

        let bundle_root = FinalityCommitment::compute_bundle_root(&[commitment_obj.commitment_hash]);

        let fc = FinalityCommitment {
            sequence_id: 1,
            ordered_bundle_ids: vec![bundle_id],
            intent_ids: vec![intent_id],
            ordering_root,
            bundle_root,
            fairness_root: [0xDD; 32],
            previous_commitment: [0u8; 32],
            proposer,
            timestamp: 100,
        };

        let mut commitments = BTreeMap::new();
        commitments.insert(bundle_id, commitment_obj);

        let mut intents = BTreeSet::new();
        intents.insert(intent_id);

        let mut solvers = BTreeSet::new();
        solvers.insert(solver_id);

        let mut validators = BTreeSet::new();
        validators.insert(proposer);

        (fc, vec![bundle], commitments, intents, solvers, validators)
    }

    #[test]
    fn test_verify_commitment_happy_path() {
        let (fc, bundles, commitments, intents, solvers, validators) = setup();
        let result = verify_commitment(
            &fc, &bundles, &commitments, &intents, &solvers, &validators,
            [0xDD; 32], None, 1, 100,
        );
        assert!(result.is_ok());
    }

    #[test]
    fn test_verify_ordering_root_mismatch() {
        let (mut fc, bundles, commitments, intents, solvers, validators) = setup();
        fc.ordering_root = [0xFF; 32]; // tampered
        match verify_commitment(&fc, &bundles, &commitments, &intents, &solvers, &validators, [0xDD; 32], None, 1, 100) {
            Err(CommitmentVerificationError::OrderingRootMismatch { .. }) => {}
            other => panic!("expected OrderingRootMismatch, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_missing_intent() {
        let (fc, bundles, commitments, _intents, solvers, validators) = setup();
        let empty_intents = BTreeSet::new(); // no intents known
        match verify_commitment(&fc, &bundles, &commitments, &empty_intents, &solvers, &validators, [0xDD; 32], None, 1, 100) {
            Err(CommitmentVerificationError::MissingIntent(_)) => {}
            other => panic!("expected MissingIntent, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_sequence_gap() {
        let (fc, bundles, commitments, intents, solvers, validators) = setup();
        // Expect sequence 5 but commitment says 1
        match verify_commitment(&fc, &bundles, &commitments, &intents, &solvers, &validators, [0xDD; 32], None, 5, 100) {
            Err(CommitmentVerificationError::SequenceGap { expected: 5, actual: 1 }) => {}
            other => panic!("expected SequenceGap, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_invalid_proposer() {
        let (fc, bundles, commitments, intents, solvers, _validators) = setup();
        let empty_validators = BTreeSet::new();
        match verify_commitment(&fc, &bundles, &commitments, &intents, &solvers, &empty_validators, [0xDD; 32], None, 1, 100) {
            Err(CommitmentVerificationError::InvalidProposer(_)) => {}
            other => panic!("expected InvalidProposer, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_duplicate_bundle() {
        let (mut fc, bundles, commitments, intents, solvers, validators) = setup();
        fc.ordered_bundle_ids.push([1u8; 32]); // duplicate
        match verify_commitment(&fc, &bundles, &commitments, &intents, &solvers, &validators, [0xDD; 32], None, 1, 100) {
            Err(CommitmentVerificationError::DuplicateBundle(_)) => {}
            other => panic!("expected DuplicateBundle, got {:?}", other),
        }
    }

    #[test]
    fn test_verify_previous_commitment_mismatch() {
        let (fc, bundles, commitments, intents, solvers, validators) = setup();
        match verify_commitment(&fc, &bundles, &commitments, &intents, &solvers, &validators, [0xDD; 32], Some([0xEE; 32]), 1, 100) {
            Err(CommitmentVerificationError::InvalidPreviousCommitment) => {}
            other => panic!("expected InvalidPreviousCommitment, got {:?}", other),
        }
    }
}
