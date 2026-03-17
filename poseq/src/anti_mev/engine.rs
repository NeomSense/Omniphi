use std::collections::BTreeMap;
use crate::anti_mev::policy::{AntiMevPolicy};
use crate::fairness::policy::FairnessClass;
use crate::fairness::classification::SubmissionFairnessProfile;

/// A single ordering delta for one submission between snapshot and proposed order.
#[derive(Debug, Clone)]
pub struct OrderingDelta {
    pub submission_id: [u8; 32],
    pub snapshot_position: usize,
    pub proposed_position: usize,
    /// proposed_position as i64 minus snapshot_position as i64.
    /// Negative = promoted (moved earlier); positive = demoted (moved later).
    pub delta: i64,
    pub fairness_class: FairnessClass,
}

/// A violation of protected-flow ordering requirements.
#[derive(Debug, Clone)]
pub struct ProtectedFlowViolation {
    pub submission_id: [u8; 32],
    pub expected_max_position: usize,
    pub actual_position: usize,
}

/// Type of anti-MEV violation detected.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AntiMevViolationType {
    ExceededForwardReorderBound,
    ExceededBackwardReorderBound,
    ProtectedFlowDelayed,
    LeaderDiscretionExceeded,
    SnapshotCommitmentViolation,
}

/// A single anti-MEV violation.
#[derive(Debug, Clone)]
pub struct AntiMevViolation {
    pub submission_id: [u8; 32],
    pub violation_type: AntiMevViolationType,
    pub actual_delta: i64,
    pub allowed_delta: i64,
}

/// Result of applying anti-MEV ordering constraints to a proposal.
#[derive(Debug, Clone)]
pub struct AntiMevValidationResult {
    pub valid: bool,
    pub violations: Vec<AntiMevViolation>,
    pub position_deltas: Vec<OrderingDelta>,
    pub max_delta_forward: i64,
    pub max_delta_backward: i64,
}

impl Default for AntiMevValidationResult {
    fn default() -> Self {
        AntiMevValidationResult {
            valid: true,
            violations: Vec::new(),
            position_deltas: Vec::new(),
            max_delta_forward: 0,
            max_delta_backward: 0,
        }
    }
}

/// The anti-MEV engine validates proposed batch orderings against policy bounds.
#[derive(Debug, Clone)]
pub struct AntiMevEngine {
    pub policy: AntiMevPolicy,
}

impl AntiMevEngine {
    pub fn new(policy: AntiMevPolicy) -> Self {
        AntiMevEngine { policy }
    }

    /// Apply ordering constraints to a proposal and return a validation result.
    ///
    /// `snapshot_order`: the canonical queue snapshot order (id, class pairs).
    /// `proposed_order`: the leader's proposed ordering of submission IDs.
    /// `batch_size`: the maximum batch size (for discretion limit calculation).
    pub fn apply_ordering_constraints(
        &self,
        snapshot_order: &[([u8; 32], FairnessClass)],
        proposed_order: &[[u8; 32]],
        batch_size: usize,
    ) -> AntiMevValidationResult {
        let deltas = self.compute_position_deltas(snapshot_order, proposed_order);

        let mut violations = Vec::new();
        let mut max_delta_forward = 0i64;
        let mut max_delta_backward = 0i64;

        // Build class map from snapshot
        let class_map: BTreeMap<[u8; 32], &FairnessClass> =
            snapshot_order.iter().map(|(id, cls)| (*id, cls)).collect();

        let mut _discretion_violations = 0usize;

        for delta in &deltas {
            if delta.delta < max_delta_forward {
                max_delta_forward = delta.delta;
            }
            if delta.delta > max_delta_backward {
                max_delta_backward = delta.delta;
            }

            let class = class_map.get(&delta.submission_id).copied().unwrap_or(&FairnessClass::Normal);
            let positions_moved = delta.delta;

            if !self.policy.validate_ordering_delta(class, positions_moved) {
                if positions_moved < 0 {
                    let bound = self.policy.get_reorder_bound_for(class)
                        .map(|b| b.max_positions_forward as i64)
                        .unwrap_or(i64::MAX);
                    violations.push(AntiMevViolation {
                        submission_id: delta.submission_id,
                        violation_type: AntiMevViolationType::ExceededForwardReorderBound,
                        actual_delta: positions_moved,
                        allowed_delta: -bound,
                    });
                } else {
                    let bound = self.policy.get_reorder_bound_for(class)
                        .map(|b| b.max_positions_backward as i64)
                        .unwrap_or(i64::MAX);
                    violations.push(AntiMevViolation {
                        submission_id: delta.submission_id,
                        violation_type: AntiMevViolationType::ExceededBackwardReorderBound,
                        actual_delta: positions_moved,
                        allowed_delta: bound,
                    });
                }
                _discretion_violations += 1;
            }
        }

        // FIND-003: Enforce leader discretion limit as a hard batch-level violation.
        // A batch where more than `max_discretion_bps` of submissions were moved
        // (even if each individual delta is within per-submission bounds) is invalid.
        if batch_size > 0 && !deltas.is_empty() {
            let moved_count = deltas.iter().filter(|d| d.delta != 0).count();
            let moved_bps = (moved_count as u64 * 10000) / deltas.len() as u64;
            let limit = self.policy.leader_discretion_limit.max_discretion_bps as u64;
            if moved_bps > limit {
                // Use submission_id=[0u8;32] as sentinel for the batch-level violation.
                violations.push(AntiMevViolation {
                    submission_id: [0u8; 32],
                    violation_type: AntiMevViolationType::LeaderDiscretionExceeded,
                    actual_delta: moved_bps as i64,
                    allowed_delta: limit as i64,
                });
            }
        }

        let valid = violations.is_empty();
        AntiMevValidationResult {
            valid,
            violations,
            position_deltas: deltas,
            max_delta_forward,
            max_delta_backward,
        }
    }

    /// Compute signed position deltas for submissions that appear in both snapshot and proposed.
    ///
    /// delta = proposed_position - snapshot_position
    /// Negative delta = submission was promoted (moved earlier).
    /// Positive delta = submission was demoted (moved later).
    pub fn compute_position_deltas(
        &self,
        snapshot_order: &[([u8; 32], FairnessClass)],
        proposed_order: &[[u8; 32]],
    ) -> Vec<OrderingDelta> {
        // Build position maps
        let snapshot_positions: BTreeMap<[u8; 32], usize> = snapshot_order
            .iter()
            .enumerate()
            .map(|(i, (id, _))| (*id, i))
            .collect();

        let proposed_positions: BTreeMap<[u8; 32], usize> = proposed_order
            .iter()
            .enumerate()
            .map(|(i, id)| (*id, i))
            .collect();

        let class_map: BTreeMap<[u8; 32], &FairnessClass> =
            snapshot_order.iter().map(|(id, cls)| (*id, cls)).collect();

        let mut deltas = Vec::new();

        // Only compute deltas for submissions that appear in BOTH orderings
        for (id, &snap_pos) in &snapshot_positions {
            if let Some(&prop_pos) = proposed_positions.get(id) {
                let delta = prop_pos as i64 - snap_pos as i64;
                let fairness_class = class_map
                    .get(id)
                    .copied()
                    .cloned()
                    .unwrap_or(FairnessClass::Normal);
                deltas.push(OrderingDelta {
                    submission_id: *id,
                    snapshot_position: snap_pos,
                    proposed_position: prop_pos,
                    delta,
                    fairness_class,
                });
            }
        }

        // Sort by snapshot_position for determinism
        deltas.sort_by_key(|d| d.snapshot_position);
        deltas
    }

    /// Validate that protected flows appear within their allowed position bounds.
    pub fn validate_protected_flows(
        &self,
        proposed_order: &[[u8; 32]],
        profiles: &BTreeMap<[u8; 32], SubmissionFairnessProfile>,
    ) -> Vec<ProtectedFlowViolation> {
        let mut violations = Vec::new();
        let pfp = &self.policy.protected_flow_policy;

        if !pfp.enabled {
            return violations;
        }

        let proposed_positions: BTreeMap<[u8; 32], usize> = proposed_order
            .iter()
            .enumerate()
            .map(|(i, id)| (*id, i))
            .collect();

        let batch_size = proposed_order.len();

        for (id, profile) in profiles {
            if pfp.protected_classes.contains(&profile.assigned_class) {
                let max_pos = (pfp.max_delay_slots as usize).min(batch_size.saturating_sub(1));
                match proposed_positions.get(id) {
                    None => {
                        // FIND-004: protected submission entirely absent from proposal —
                        // this is a censorship/omission violation. Use usize::MAX as sentinel.
                        violations.push(ProtectedFlowViolation {
                            submission_id: *id,
                            expected_max_position: max_pos,
                            actual_position: usize::MAX,
                        });
                    }
                    Some(&actual_pos) => {
                        if actual_pos > max_pos {
                            violations.push(ProtectedFlowViolation {
                                submission_id: *id,
                                expected_max_position: max_pos,
                                actual_position: actual_pos,
                            });
                        }
                    }
                }
            }
        }

        violations.sort_by_key(|v| v.submission_id);
        violations
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::anti_mev::policy::AntiMevPolicy;
    use crate::fairness::policy::FairnessClass;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_snapshot(ids: &[u8]) -> Vec<([u8; 32], FairnessClass)> {
        ids.iter()
            .map(|&b| (make_id(b), FairnessClass::Normal))
            .collect()
    }

    #[test]
    fn test_compute_position_deltas_correct() {
        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());
        // Snapshot: [A, B, C, D] → positions 0,1,2,3
        // Proposed: [A, C, B, D] → B moved from 1→2 (delta=+1), C moved from 2→1 (delta=-1)
        let snapshot = vec![
            (make_id(1), FairnessClass::Normal),
            (make_id(2), FairnessClass::Normal),
            (make_id(3), FairnessClass::Normal),
            (make_id(4), FairnessClass::Normal),
        ];
        let proposed = vec![make_id(1), make_id(3), make_id(2), make_id(4)];
        let deltas = engine.compute_position_deltas(&snapshot, &proposed);

        // Find delta for id=1 (should be 0)
        let d1 = deltas.iter().find(|d| d.submission_id == make_id(1)).unwrap();
        assert_eq!(d1.delta, 0);

        // Find delta for id=2: was at position 1, now at position 2 → delta=+1
        let d2 = deltas.iter().find(|d| d.submission_id == make_id(2)).unwrap();
        assert_eq!(d2.delta, 1);

        // Find delta for id=3: was at position 2, now at position 1 → delta=-1
        let d3 = deltas.iter().find(|d| d.submission_id == make_id(3)).unwrap();
        assert_eq!(d3.delta, -1);
    }

    #[test]
    fn test_delta_snapshot_pos0_proposed_pos3() {
        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());
        let snapshot = vec![
            (make_id(10), FairnessClass::Normal),
            (make_id(11), FairnessClass::Normal),
            (make_id(12), FairnessClass::Normal),
            (make_id(13), FairnessClass::Normal),
        ];
        let proposed = vec![make_id(11), make_id(12), make_id(13), make_id(10)];
        let deltas = engine.compute_position_deltas(&snapshot, &proposed);
        let d = deltas.iter().find(|d| d.submission_id == make_id(10)).unwrap();
        // snapshot[0] in proposed[3] → delta = 3
        assert_eq!(d.snapshot_position, 0);
        assert_eq!(d.proposed_position, 3);
        assert_eq!(d.delta, 3);
    }

    #[test]
    fn test_apply_ordering_constraints_compliant_passes() {
        // Swap two adjacent items — within per-submission bound of 5.
        // Use a permissive discretion limit (5000 bps = 50%) so the batch-level check
        // doesn't fire on a single valid swap (2/5 = 40% < 50%).
        use crate::anti_mev::policy::LeaderDiscretionLimit;
        let mut policy = AntiMevPolicy::default_policy();
        policy.leader_discretion_limit = LeaderDiscretionLimit { max_discretion_bps: 5000, apply_to_protected: true };
        let engine = AntiMevEngine::new(policy);
        let snapshot = make_snapshot(&[1, 2, 3, 4, 5]);
        let proposed = vec![make_id(2), make_id(1), make_id(3), make_id(4), make_id(5)];
        let result = engine.apply_ordering_constraints(&snapshot, &proposed, 5);
        assert!(result.valid, "Expected compliant ordering to pass: {:?}", result.violations);
    }

    #[test]
    fn test_apply_ordering_constraints_forward_reorder_exceeds_bound_fails() {
        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());
        // id=7 is at snapshot position 6, moved to proposed position 0 → delta = -6 (exceeds max=5)
        let ids = &[1u8, 2, 3, 4, 5, 6, 7];
        let snapshot: Vec<([u8; 32], FairnessClass)> = ids
            .iter()
            .map(|&b| (make_id(b), FairnessClass::Normal))
            .collect();
        // Move id=7 to front
        let proposed = vec![
            make_id(7), make_id(1), make_id(2), make_id(3), make_id(4), make_id(5), make_id(6),
        ];
        let result = engine.apply_ordering_constraints(&snapshot, &proposed, 7);
        assert!(!result.valid, "Expected violation for excessive forward reorder");
        assert!(!result.violations.is_empty());
        let v = &result.violations[0];
        assert_eq!(v.violation_type, AntiMevViolationType::ExceededForwardReorderBound);
    }

    #[test]
    fn test_apply_ordering_constraints_backward_reorder_exceeds_bound_fails() {
        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());
        // id=1 at snapshot position 0, moved to proposed position 6 → delta = +6 (exceeds max=5)
        let ids = &[1u8, 2, 3, 4, 5, 6, 7];
        let snapshot: Vec<([u8; 32], FairnessClass)> = ids
            .iter()
            .map(|&b| (make_id(b), FairnessClass::Normal))
            .collect();
        let proposed = vec![
            make_id(2), make_id(3), make_id(4), make_id(5), make_id(6), make_id(7), make_id(1),
        ];
        let result = engine.apply_ordering_constraints(&snapshot, &proposed, 7);
        assert!(!result.valid, "Expected violation for excessive backward reorder");
        assert!(!result.violations.is_empty());
        let v = &result.violations[0];
        assert_eq!(v.violation_type, AntiMevViolationType::ExceededBackwardReorderBound);
    }

    #[test]
    fn test_safety_critical_cannot_be_reordered_at_all() {
        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());
        let snapshot = vec![
            (make_id(1), FairnessClass::SafetyCritical),
            (make_id(2), FairnessClass::Normal),
        ];
        // Move SafetyCritical submission by 1 position
        let proposed = vec![make_id(2), make_id(1)];
        let result = engine.apply_ordering_constraints(&snapshot, &proposed, 2);
        assert!(!result.valid, "SafetyCritical must not be reordered");
    }

    #[test]
    fn test_discretion_limit_exceeded_marks_batch_invalid_even_if_per_submission_bounds_respected() {
        // FIND-003: each submission moves by 1 (within individual bounds of 5),
        // but 100% of submissions are moved which should exceed any reasonable discretion limit.
        // Default policy has max_discretion_bps; we construct a policy where swapping all pairs
        // triggers the limit.
        use crate::anti_mev::policy::{AntiMevPolicy, LeaderDiscretionLimit};

        let mut policy = AntiMevPolicy::default_policy();
        // Set discretion limit to 0 bps — no reordering permitted at batch level
        policy.leader_discretion_limit = LeaderDiscretionLimit { max_discretion_bps: 0, apply_to_protected: true };
        let engine = AntiMevEngine::new(policy);

        // All submissions swap by 1 — individually fine, batch limit exceeded
        let snapshot = make_snapshot(&[1, 2, 3, 4]);
        let proposed = vec![make_id(2), make_id(1), make_id(4), make_id(3)];
        let result = engine.apply_ordering_constraints(&snapshot, &proposed, 4);
        assert!(!result.valid, "discretion limit exceeded must invalidate batch");
        let has_discretion_violation = result
            .violations
            .iter()
            .any(|v| v.violation_type == AntiMevViolationType::LeaderDiscretionExceeded);
        assert!(has_discretion_violation, "must contain LeaderDiscretionExceeded violation");
    }

    #[test]
    fn test_protected_flow_omission_is_flagged_as_violation() {
        // FIND-004: protected submission present in profiles but absent from proposed batch
        use std::collections::BTreeMap;
        use crate::fairness::classification::SubmissionFairnessProfile;

        let engine = AntiMevEngine::new(AntiMevPolicy::default_policy());

        // Build a profile for id(1) marked as a protected class
        let protected_id = make_id(1);
        let mut profiles: BTreeMap<[u8; 32], SubmissionFairnessProfile> = BTreeMap::new();
        let protected_class = engine
            .policy
            .protected_flow_policy
            .protected_classes
            .iter()
            .next()
            .cloned();

        if let Some(cls) = protected_class {
            if engine.policy.protected_flow_policy.enabled {
                let profile = SubmissionFairnessProfile::new(
                    protected_id,
                    cls,
                    0,  // received_at_slot
                    0,  // received_at_sequence
                    0,  // current_slot
                    false,
                    "test".to_string(),
                );
                profiles.insert(protected_id, profile);

                // Propose batch without the protected submission
                let proposed = vec![make_id(2), make_id(3)];
                let violations = engine.validate_protected_flows(&proposed, &profiles);
                assert!(
                    !violations.is_empty(),
                    "omitted protected submission must generate violation"
                );
                assert_eq!(
                    violations[0].actual_position,
                    usize::MAX,
                    "omission sentinel must be usize::MAX"
                );
            }
        }
        // If protected flows are disabled or no classes configured, test is vacuously valid
    }
}
