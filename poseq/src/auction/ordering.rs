//! Intent-aware ordering engine — Section 6 of the architecture specification.
//!
//! Extends the existing PoSeq ordering with intent-grouped bundle ranking.
//! Produces a canonical, deterministic ordered bundle list from valid reveals.

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use super::types::{BundleReveal, PredictedOutput, SequencedBundle};

/// Metadata about an intent needed for canonical ordering.
/// Passed alongside reveals so the ordering engine can sort by deadline/tip per spec.
#[derive(Debug, Clone, Copy)]
pub struct IntentOrderingMeta {
    pub intent_id: [u8; 32],
    pub deadline: u64,
    pub tip: u64,
}

/// Outcome of intent-aware ordering for one batch window.
#[derive(Debug, Clone)]
pub struct IntentOrderingResult {
    /// Winning bundles in canonical execution order.
    pub ordered_bundles: Vec<SequencedBundle>,
    /// Sequence root = SHA256(bundle_id[0] ‖ ... ‖ bundle_id[n]).
    pub sequence_root: [u8; 32],
    /// Total intents filled.
    pub total_intents_filled: u32,
    /// Total bundles received (including losers).
    pub total_bundles_received: u32,
    /// Total valid bundles.
    pub total_bundles_valid: u32,
}

/// Run the 7-step intent-aware ordering algorithm from Section 6.3.
///
/// Input: valid reveals and intent metadata for canonical sorting.
/// Output: canonical ordered bundle list with sequence root.
///
/// Sorting: deadline ASC (most urgent first), tip DESC (highest priority),
/// then SHA256(intent_id ‖ batch_window) ASC for deterministic tiebreak.
pub fn order_bundles(
    reveals: &[&BundleReveal],
    batch_window: u64,
) -> IntentOrderingResult {
    // No intent metadata available — delegate to full version with empty metadata
    order_bundles_with_meta(reveals, batch_window, &BTreeMap::new())
}

/// Full ordering with intent metadata for spec-compliant sorting.
pub fn order_bundles_with_meta(
    reveals: &[&BundleReveal],
    batch_window: u64,
    intent_meta: &BTreeMap<[u8; 32], IntentOrderingMeta>,
) -> IntentOrderingResult {
    let total_bundles_received = reveals.len() as u32;

    // STEP 2: Group by intent
    let mut bundles_by_intent: BTreeMap<[u8; 32], Vec<&BundleReveal>> = BTreeMap::new();
    for reveal in reveals {
        for intent_id in &reveal.target_intent_ids {
            bundles_by_intent.entry(*intent_id).or_default().push(reveal);
        }
    }

    // STEP 3: Rank within each intent group by user_outcome_score DESC
    // STEP 4: Break ties deterministically
    // STEP 5: Select winner per intent
    let mut winners: BTreeMap<[u8; 32], &BundleReveal> = BTreeMap::new();

    for (intent_id, mut candidates) in bundles_by_intent {
        // Sort by user_outcome_score for this intent (descending)
        candidates.sort_by(|a, b| {
            let score_a = outcome_score_for_intent(a, &intent_id);
            let score_b = outcome_score_for_intent(b, &intent_id);

            // Higher score wins
            score_b.cmp(&score_a).then_with(|| {
                // STEP 4: Deterministic tiebreak
                let key_a = tiebreak_key(&a.bundle_id, &intent_id, batch_window);
                let key_b = tiebreak_key(&b.bundle_id, &intent_id, batch_window);
                key_a.cmp(&key_b)
            })
        });

        if let Some(winner) = candidates.first() {
            winners.insert(intent_id, winner);
        }
    }

    // Deduplicate: a single bundle can win multiple intents, but should appear only once.
    let mut seen_bundles = std::collections::BTreeSet::new();
    let mut unique_winners: Vec<([u8; 32], &BundleReveal)> = Vec::new();
    for (intent_id, reveal) in &winners {
        if seen_bundles.insert(reveal.bundle_id) {
            unique_winners.push((*intent_id, *reveal));
        }
    }

    // STEP 6: Produce canonical ordered bundle list
    // Sort by: (intent deadline ASC, intent tip DESC, SHA256(intent_id ‖ batch_window) ASC)
    unique_winners.sort_by(|a, b| {
        let meta_a = intent_meta.get(&a.0);
        let meta_b = intent_meta.get(&b.0);

        // Primary: deadline ASC (most urgent first)
        let deadline_a = meta_a.map(|m| m.deadline).unwrap_or(u64::MAX);
        let deadline_b = meta_b.map(|m| m.deadline).unwrap_or(u64::MAX);
        deadline_a.cmp(&deadline_b)
            // Secondary: tip DESC (highest priority)
            .then_with(|| {
                let tip_a = meta_a.map(|m| m.tip).unwrap_or(0);
                let tip_b = meta_b.map(|m| m.tip).unwrap_or(0);
                tip_b.cmp(&tip_a)
            })
            // Tertiary: deterministic hash tiebreak
            .then_with(|| {
                let key_a = canonical_sort_key(&a.0, batch_window);
                let key_b = canonical_sort_key(&b.0, batch_window);
                key_a.cmp(&key_b)
            })
    });

    // Build SequencedBundles
    let ordered_bundles: Vec<SequencedBundle> = unique_winners.iter()
        .enumerate()
        .map(|(i, (_, reveal))| {
            SequencedBundle {
                bundle_id: reveal.bundle_id,
                solver_id: reveal.solver_id,
                target_intent_ids: reveal.target_intent_ids.clone(),
                execution_steps: reveal.execution_steps.clone(),
                predicted_outputs: reveal.predicted_outputs.clone(),
                resource_declarations: reveal.resource_declarations.clone(),
                sequence_index: i as u32,
            }
        })
        .collect();

    // STEP 7: Compute sequence root
    let sequence_root = compute_sequence_root(&ordered_bundles);

    IntentOrderingResult {
        total_intents_filled: winners.len() as u32,
        total_bundles_received,
        total_bundles_valid: reveals.len() as u32,
        ordered_bundles,
        sequence_root,
    }
}

/// Compute user_outcome_score for a specific intent from a reveal's predicted outputs.
fn outcome_score_for_intent(reveal: &BundleReveal, intent_id: &[u8; 32]) -> u128 {
    reveal.predicted_outputs.iter()
        .filter(|o| o.intent_id == *intent_id)
        .map(|o| o.user_outcome_score())
        .max()
        .unwrap_or(0)
}

/// Deterministic tiebreak key: SHA256(bundle_id ‖ intent_id ‖ batch_window).
fn tiebreak_key(bundle_id: &[u8; 32], intent_id: &[u8; 32], batch_window: u64) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(bundle_id);
    hasher.update(intent_id);
    hasher.update(batch_window.to_be_bytes());
    let result = hasher.finalize();
    let mut key = [0u8; 32];
    key.copy_from_slice(&result);
    key
}

/// Canonical sort key: SHA256(intent_id ‖ batch_window).
fn canonical_sort_key(intent_id: &[u8; 32], batch_window: u64) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(intent_id);
    hasher.update(batch_window.to_be_bytes());
    let result = hasher.finalize();
    let mut key = [0u8; 32];
    key.copy_from_slice(&result);
    key
}

/// Compute sequence root = SHA256(bundle_id[0] ‖ ... ‖ bundle_id[n]).
pub fn compute_sequence_root(bundles: &[SequencedBundle]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    for bundle in bundles {
        hasher.update(bundle.bundle_id);
    }
    let result = hasher.finalize();
    let mut root = [0u8; 32];
    root.copy_from_slice(&result);
    root
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::auction::types::*;
    use crate::intent_pool::types::{AssetId, AssetType};

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32]; id[0] = b;
        AssetId { chain_id: 0, asset_type: AssetType::Token, identifier: id }
    }

    fn make_reveal(bundle_byte: u8, solver_byte: u8, intent_id: [u8; 32], amount_out: u128, fee_bps: u64) -> BundleReveal {
        let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte;
        let mut solver_id = [0u8; 32]; solver_id[0] = solver_byte;
        BundleReveal {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_ids: vec![intent_id],
            execution_steps: vec![],
            liquidity_sources: vec![],
            predicted_outputs: vec![PredictedOutput {
                intent_id,
                asset_out: make_asset(2),
                amount_out,
                fee_charged_bps: fee_bps,
            }],
            fee_breakdown: FeeBreakdown { solver_fee_bps: fee_bps / 2, protocol_fee_bps: fee_bps / 2, total_fee_bps: fee_bps },
            resource_declarations: vec![],
            nonce: [bundle_byte; 32],
            proof_data: vec![],
            signature: vec![1u8; 64],
        }
    }

    #[test]
    fn test_ordering_single_intent_best_output_wins() {
        let intent_id = [5u8; 32];
        let r1 = make_reveal(1, 1, intent_id, 950, 30);  // score: 950 * 9970 / 10000 = 947
        let r2 = make_reveal(2, 2, intent_id, 980, 30);  // score: 980 * 9970 / 10000 = 977
        let r3 = make_reveal(3, 3, intent_id, 960, 30);  // score: 960 * 9970 / 10000 = 957

        let reveals: Vec<&BundleReveal> = vec![&r1, &r2, &r3];
        let result = order_bundles(&reveals, 1);

        assert_eq!(result.total_intents_filled, 1);
        assert_eq!(result.ordered_bundles.len(), 1);
        // r2 (amount 980) should win
        assert_eq!(result.ordered_bundles[0].bundle_id[0], 2);
    }

    #[test]
    fn test_ordering_multiple_intents() {
        let intent_a = { let mut id = [0u8; 32]; id[0] = 0xAA; id };
        let intent_b = { let mut id = [0u8; 32]; id[0] = 0xBB; id };

        let r1 = make_reveal(1, 1, intent_a, 950, 30);
        let r2 = make_reveal(2, 2, intent_b, 980, 30);

        let reveals: Vec<&BundleReveal> = vec![&r1, &r2];
        let result = order_bundles(&reveals, 1);

        assert_eq!(result.total_intents_filled, 2);
        assert_eq!(result.ordered_bundles.len(), 2);
    }

    #[test]
    fn test_ordering_deterministic() {
        let intent_id = [5u8; 32];
        let r1 = make_reveal(1, 1, intent_id, 950, 30);
        let r2 = make_reveal(2, 2, intent_id, 950, 30); // same score → tiebreak

        let reveals: Vec<&BundleReveal> = vec![&r1, &r2];
        let result1 = order_bundles(&reveals, 1);

        // Reverse input order
        let reveals2: Vec<&BundleReveal> = vec![&r2, &r1];
        let result2 = order_bundles(&reveals2, 1);

        // Same winner regardless of input order
        assert_eq!(result1.ordered_bundles[0].bundle_id, result2.ordered_bundles[0].bundle_id);
        assert_eq!(result1.sequence_root, result2.sequence_root);
    }

    #[test]
    fn test_sequence_root_changes_with_different_order() {
        let b1 = SequencedBundle {
            bundle_id: [1u8; 32], solver_id: [1u8; 32],
            target_intent_ids: vec![], execution_steps: vec![],
            predicted_outputs: vec![], resource_declarations: vec![],
            sequence_index: 0,
        };
        let b2 = SequencedBundle {
            bundle_id: [2u8; 32], solver_id: [2u8; 32],
            target_intent_ids: vec![], execution_steps: vec![],
            predicted_outputs: vec![], resource_declarations: vec![],
            sequence_index: 1,
        };

        let root_12 = compute_sequence_root(&[b1.clone(), b2.clone()]);
        let root_21 = compute_sequence_root(&[b2, b1]);

        assert_ne!(root_12, root_21); // Different ordering → different root
    }

    #[test]
    fn test_empty_reveals() {
        let result = order_bundles(&[], 1);
        assert_eq!(result.total_intents_filled, 0);
        assert_eq!(result.ordered_bundles.len(), 0);
    }

    #[test]
    fn test_lower_fee_wins_when_same_amount() {
        let intent_id = [5u8; 32];
        let r1 = make_reveal(1, 1, intent_id, 1000, 100);  // score: 1000 * 9900 / 10000 = 990
        let r2 = make_reveal(2, 2, intent_id, 1000, 30);   // score: 1000 * 9970 / 10000 = 997

        let reveals: Vec<&BundleReveal> = vec![&r1, &r2];
        let result = order_bundles(&reveals, 1);

        // r2 (lower fee) should win because user_outcome_score is higher
        assert_eq!(result.ordered_bundles[0].bundle_id[0], 2);
    }
}
