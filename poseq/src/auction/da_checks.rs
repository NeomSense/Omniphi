//! Data Availability enforcement — Section 13 of the architecture specification.
//!
//! Pre-vote DA validation checks that validators must perform before
//! casting a HotStuff vote on a proposed block.

use super::types::{BundleCommitment, BundleReveal, SequencedBundle};
use std::collections::BTreeMap;

/// Result of DA validation for a proposed block.
#[derive(Debug, Clone)]
pub struct DAValidationResult {
    pub passed: bool,
    pub failures: Vec<DAFailure>,
}

impl DAValidationResult {
    pub fn success() -> Self {
        DAValidationResult { passed: true, failures: vec![] }
    }

    pub fn failure(failures: Vec<DAFailure>) -> Self {
        DAValidationResult { passed: false, failures }
    }
}

/// Specific DA check failures.
#[derive(Debug, Clone)]
pub enum DAFailure {
    /// Block payload is incomplete (only hash, no full data).
    PayloadIncomplete,
    /// A bundle's commitment is not accessible.
    CommitmentUnavailable { bundle_id: [u8; 32] },
    /// A referenced intent is not available in pool or archive.
    IntentUnavailable { intent_id: [u8; 32] },
    /// Insufficient committee members storing the data.
    InsufficientReplication { required: usize, actual: usize },
    /// Commitment has expired (valid_until < current_block).
    CommitmentExpired { bundle_id: [u8; 32], valid_until: u64, current_block: u64 },
    /// Solver is no longer active.
    SolverInactive { bundle_id: [u8; 32], solver_id: [u8; 32] },
    /// Commitment has zero bond locked.
    ZeroBondLocked { bundle_id: [u8; 32] },
}

/// Validate data availability for a proposed batch.
///
/// Per Section 13.2, a validator checks:
/// 1. DA_CHECK_1: Payload complete (full SequencedBatch present, not just hash)
/// 2. DA_CHECK_2: Commitments accessible and valid (bond locked, not expired)
/// 3. DA_CHECK_3: Intent references valid (referenced intents exist)
/// 4. DA_CHECK_4: Historical availability (deferred to post-finalization)
///
/// AUDIT FIX: Added commitment validity checks (bond locked, expiry, solver active).
/// The caller MUST populate the maps from on-chain storage or local commitment pool.
pub fn validate_da(
    ordered_bundles: &[SequencedBundle],
    available_commitments: &BTreeMap<[u8; 32], BundleCommitment>,
    known_intent_ids: &std::collections::BTreeSet<[u8; 32]>,
    active_solver_ids: &std::collections::BTreeSet<[u8; 32]>,
    current_block: u64,
    payload_is_complete: bool,
) -> DAValidationResult {
    let mut failures = Vec::new();

    // DA_CHECK_1: Payload complete
    if !payload_is_complete {
        failures.push(DAFailure::PayloadIncomplete);
    }

    // DA_CHECK_2: Commitments accessible AND valid
    for bundle in ordered_bundles {
        match available_commitments.get(&bundle.bundle_id) {
            None => {
                failures.push(DAFailure::CommitmentUnavailable { bundle_id: bundle.bundle_id });
            }
            Some(commitment) => {
                // Check commitment hasn't expired
                if !commitment.is_valid_at(current_block) {
                    failures.push(DAFailure::CommitmentExpired {
                        bundle_id: bundle.bundle_id,
                        valid_until: commitment.valid_until,
                        current_block,
                    });
                }
                // Check solver is still active
                if !active_solver_ids.contains(&commitment.solver_id) {
                    failures.push(DAFailure::SolverInactive {
                        bundle_id: bundle.bundle_id,
                        solver_id: commitment.solver_id,
                    });
                }
                // Check bond was locked
                if commitment.bond_locked == 0 {
                    failures.push(DAFailure::ZeroBondLocked {
                        bundle_id: bundle.bundle_id,
                    });
                }
            }
        }
    }

    // DA_CHECK_3: Intent references valid
    for bundle in ordered_bundles {
        for intent_id in &bundle.target_intent_ids {
            if !known_intent_ids.contains(intent_id) {
                failures.push(DAFailure::IntentUnavailable { intent_id: *intent_id });
            }
        }
    }

    if failures.is_empty() {
        DAValidationResult::success()
    } else {
        DAValidationResult::failure(failures)
    }
}

/// Track consecutive DA failures for epoch transition forcing.
#[derive(Debug, Default)]
pub struct DAFailureTracker {
    pub consecutive_failures: u32,
    pub threshold: u32,
}

impl DAFailureTracker {
    pub fn new(threshold: u32) -> Self {
        DAFailureTracker { consecutive_failures: 0, threshold }
    }

    /// Record a DA check result. Returns true if threshold is exceeded.
    pub fn record(&mut self, passed: bool) -> bool {
        if passed {
            self.consecutive_failures = 0;
            false
        } else {
            self.consecutive_failures += 1;
            self.consecutive_failures >= self.threshold
        }
    }

    pub fn should_force_epoch_transition(&self) -> bool {
        self.consecutive_failures >= self.threshold
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeSet;

    fn make_bundle(b: u8) -> SequencedBundle {
        let mut bundle_id = [0u8; 32]; bundle_id[0] = b;
        let mut intent_id = [0u8; 32]; intent_id[0] = b + 100;
        SequencedBundle {
            bundle_id,
            solver_id: [0u8; 32],
            target_intent_ids: vec![intent_id],
            execution_steps: vec![],
            predicted_outputs: vec![],
            resource_declarations: vec![],
            sequence_index: 0,
        }
    }

    fn make_solver_set(b: u8) -> BTreeSet<[u8; 32]> {
        let mut set = BTreeSet::new();
        set.insert([b; 32]);
        set
    }

    #[test]
    fn test_da_all_checks_pass() {
        let bundles = vec![make_bundle(1)];
        let mut commitments = BTreeMap::new();
        let mut bundle_id = [0u8; 32]; bundle_id[0] = 1;
        commitments.insert(bundle_id, BundleCommitment {
            bundle_id,
            solver_id: [0u8; 32],
            batch_window: 1,
            target_intent_count: 1,
            commitment_hash: [0u8; 32],
            expected_outputs_hash: [0u8; 32],
            execution_plan_hash: [0u8; 32],
            valid_until: 100,
            bond_locked: 50_000,
            signature: vec![],
        });

        let mut intents = BTreeSet::new();
        let mut intent_id = [0u8; 32]; intent_id[0] = 101;
        intents.insert(intent_id);

        let solvers = make_solver_set(0); // solver_id = [0u8; 32]

        let result = validate_da(&bundles, &commitments, &intents, &solvers, 50, true);
        assert!(result.passed);
    }

    #[test]
    fn test_da_missing_commitment() {
        let bundles = vec![make_bundle(1)];
        let commitments = BTreeMap::new();
        let intents = BTreeSet::new();
        let solvers = BTreeSet::new();

        let result = validate_da(&bundles, &commitments, &intents, &solvers, 50, true);
        assert!(!result.passed);
        assert!(result.failures.iter().any(|f| matches!(f, DAFailure::CommitmentUnavailable { .. })));
    }

    #[test]
    fn test_da_incomplete_payload() {
        let result = validate_da(&[], &BTreeMap::new(), &BTreeSet::new(), &BTreeSet::new(), 50, false);
        assert!(!result.passed);
        assert!(result.failures.iter().any(|f| matches!(f, DAFailure::PayloadIncomplete)));
    }

    #[test]
    fn test_da_failure_tracker() {
        let mut tracker = DAFailureTracker::new(3);
        assert!(!tracker.record(false));
        assert!(!tracker.record(false));
        assert!(tracker.record(false)); // 3rd consecutive failure → threshold
        assert!(tracker.should_force_epoch_transition());

        // Reset on success
        tracker.record(true);
        assert!(!tracker.should_force_epoch_transition());
    }
}
