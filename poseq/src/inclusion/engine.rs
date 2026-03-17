use std::collections::{BTreeMap, BTreeSet};
use crate::fairness::policy::FairSequencingPolicy;
use crate::fairness::classification::SubmissionFairnessProfile;

/// The decision made for a submission during inclusion evaluation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InclusionDecision {
    Include,
    Exclude(ExclusionReason),
    ForceInclude(String),
}

/// Why a submission was excluded from a batch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExclusionReason {
    NotYetEligible,
    PolicyExcluded,
    ClassRestricted,
    BatchFull,
    SnapshotNotIncluded,
}

/// Audit record for a single submission's inclusion decision.
#[derive(Debug, Clone)]
pub struct InclusionEligibilityRecord {
    pub submission_id: [u8; 32],
    pub fairness_class: crate::fairness::policy::FairnessClass,
    pub received_at_slot: u64,
    pub age_slots: u64,
    pub decision: InclusionDecision,
    pub forced: bool,
}

/// Tracks when submissions were first seen, for age-based starvation detection.
#[derive(Debug, Clone, Default)]
pub struct SubmissionAgeTracker {
    /// Maps submission_id → (received_slot, received_sequence).
    ages: BTreeMap<[u8; 32], (u64, u64)>,
}

impl SubmissionAgeTracker {
    pub fn new() -> Self {
        SubmissionAgeTracker { ages: BTreeMap::new() }
    }

    /// Register a new submission with its slot and sequence.
    pub fn register(&mut self, id: [u8; 32], slot: u64, sequence: u64) {
        self.ages.entry(id).or_insert((slot, sequence));
    }

    /// Return age in slots: current_slot - received_slot. Returns 0 if unknown.
    pub fn get_age(&self, id: &[u8; 32], current_slot: u64) -> u64 {
        self.ages
            .get(id)
            .map(|(received_slot, _)| current_slot.saturating_sub(*received_slot))
            .unwrap_or(0)
    }

    /// Remove a submission (after it has been included).
    pub fn remove(&mut self, id: &[u8; 32]) {
        self.ages.remove(id);
    }

    /// Return all submission IDs whose age exceeds max_wait slots.
    pub fn get_stale_beyond(&self, current_slot: u64, max_wait: u64) -> Vec<[u8; 32]> {
        self.ages
            .iter()
            .filter(|(_, (received_slot, _))| {
                current_slot.saturating_sub(*received_slot) > max_wait
            })
            .map(|(id, _)| *id)
            .collect()
    }
}

/// A submission that must be force-included in the next batch due to starvation.
#[derive(Debug, Clone)]
pub struct ForcedInclusionCandidate {
    pub submission_id: [u8; 32],
    pub age_slots: u64,
    pub fairness_class: crate::fairness::policy::FairnessClass,
    pub forced_reason: String,
}

/// Engine that evaluates which submissions are eligible for inclusion in a batch.
#[derive(Debug, Clone)]
pub struct InclusionPolicyEngine {
    pub policy: FairSequencingPolicy,
    pub age_tracker: SubmissionAgeTracker,
}

impl InclusionPolicyEngine {
    pub fn new(policy: FairSequencingPolicy) -> Self {
        InclusionPolicyEngine {
            policy,
            age_tracker: SubmissionAgeTracker::new(),
        }
    }

    /// Compute the eligible set for a batch given candidate submission IDs and their profiles.
    ///
    /// Returns:
    /// - A Vec of `InclusionEligibilityRecord` for all candidates (included + excluded).
    /// - A Vec of `ForcedInclusionCandidate` for submissions that must be force-included.
    pub fn compute_eligible_set(
        &mut self,
        candidates: &BTreeSet<[u8; 32]>,
        profiles: &BTreeMap<[u8; 32], SubmissionFairnessProfile>,
        current_slot: u64,
        batch_capacity: usize,
    ) -> (Vec<InclusionEligibilityRecord>, Vec<ForcedInclusionCandidate>) {
        let mut records = Vec::new();
        let mut forced_candidates = Vec::new();
        let mut included_count = 0usize;

        // Check starvation first to build forced set
        let forced_ids = self.check_anti_starvation(profiles, current_slot);
        let forced_set: BTreeSet<[u8; 32]> = forced_ids.iter().map(|f| f.submission_id).collect();
        forced_candidates.extend(forced_ids);

        for id in candidates {
            let profile = profiles.get(id);
            let (received_slot, age_slots, fairness_class, is_forced) = match profile {
                Some(p) => (
                    p.received_at_slot,
                    p.age_slots,
                    p.assigned_class.clone(),
                    p.is_forced_inclusion || forced_set.contains(id),
                ),
                None => {
                    let age = self.age_tracker.get_age(id, current_slot);
                    (
                        current_slot.saturating_sub(age),
                        age,
                        crate::fairness::policy::FairnessClass::Normal,
                        forced_set.contains(id),
                    )
                }
            };

            if is_forced {
                // Force-include regardless of capacity (within reason)
                records.push(InclusionEligibilityRecord {
                    submission_id: *id,
                    fairness_class,
                    received_at_slot: received_slot,
                    age_slots,
                    decision: InclusionDecision::ForceInclude("anti_starvation".to_string()),
                    forced: true,
                });
                included_count += 1;
            } else if included_count >= batch_capacity {
                records.push(InclusionEligibilityRecord {
                    submission_id: *id,
                    fairness_class,
                    received_at_slot: received_slot,
                    age_slots,
                    decision: InclusionDecision::Exclude(ExclusionReason::BatchFull),
                    forced: false,
                });
            } else {
                records.push(InclusionEligibilityRecord {
                    submission_id: *id,
                    fairness_class,
                    received_at_slot: received_slot,
                    age_slots,
                    decision: InclusionDecision::Include,
                    forced: false,
                });
                included_count += 1;
            }
        }

        (records, forced_candidates)
    }

    /// Identify submissions that have been waiting longer than anti_starvation_slots.
    pub fn check_anti_starvation(
        &self,
        profiles: &BTreeMap<[u8; 32], SubmissionFairnessProfile>,
        current_slot: u64,
    ) -> Vec<ForcedInclusionCandidate> {
        let threshold = self.policy.anti_starvation_slots;
        let mut forced = Vec::new();

        for (id, profile) in profiles {
            let age = current_slot.saturating_sub(profile.received_at_slot);
            if age > threshold {
                // Check per-class max_wait_slots
                let class_rule = self.policy.get_class_rule(&profile.assigned_class);
                let class_limit = class_rule.max_wait_slots;
                // Force-include if age exceeds the global anti_starvation_slots or per-class limit
                if age > threshold || (class_limit > 0 && age > class_limit) {
                    forced.push(ForcedInclusionCandidate {
                        submission_id: *id,
                        age_slots: age,
                        fairness_class: profile.assigned_class.clone(),
                        forced_reason: format!(
                            "anti_starvation: age {} > threshold {}",
                            age, threshold
                        ),
                    });
                }
            }
        }

        // Sort for determinism
        forced.sort_by(|a, b| {
            b.age_slots
                .cmp(&a.age_slots)
                .then(a.submission_id.cmp(&b.submission_id))
        });
        forced
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fairness::policy::{FairSequencingPolicy, FairnessClass};
    use crate::fairness::classification::SubmissionFairnessProfile;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_profile(id: [u8; 32], class: FairnessClass, received_at: u64, current: u64) -> SubmissionFairnessProfile {
        SubmissionFairnessProfile::new(id, class, received_at, 0, current, false, "test".to_string())
    }

    #[test]
    fn test_compute_eligible_set_respects_batch_capacity() {
        let policy = FairSequencingPolicy::default_policy();
        let mut engine = InclusionPolicyEngine::new(policy);

        let current_slot = 10u64;
        let mut candidates = BTreeSet::new();
        let mut profiles = BTreeMap::new();

        for i in 1u8..=5 {
            let id = make_id(i);
            candidates.insert(id);
            profiles.insert(
                id,
                make_profile(id, FairnessClass::Normal, current_slot - 1, current_slot),
            );
        }

        let (records, _forced) = engine.compute_eligible_set(&candidates, &profiles, current_slot, 3);

        let included: Vec<_> = records
            .iter()
            .filter(|r| matches!(r.decision, InclusionDecision::Include | InclusionDecision::ForceInclude(_)))
            .collect();
        let excluded: Vec<_> = records
            .iter()
            .filter(|r| matches!(r.decision, InclusionDecision::Exclude(_)))
            .collect();

        // At most batch_capacity non-forced included
        assert!(included.len() <= 3);
        assert_eq!(included.len() + excluded.len(), 5);
    }

    #[test]
    fn test_anti_starvation_triggers_for_old_submissions() {
        let policy = FairSequencingPolicy::default_policy(); // anti_starvation_slots = 100
        let engine = InclusionPolicyEngine::new(policy);

        let current_slot = 200u64;
        let mut profiles = BTreeMap::new();
        let old_id = make_id(1);
        let fresh_id = make_id(2);

        // old_id received at slot 0 → age = 200 > 100
        profiles.insert(old_id, make_profile(old_id, FairnessClass::Normal, 0, current_slot));
        // fresh_id received at slot 195 → age = 5 < 100
        profiles.insert(fresh_id, make_profile(fresh_id, FairnessClass::Normal, 195, current_slot));

        let forced = engine.check_anti_starvation(&profiles, current_slot);
        assert_eq!(forced.len(), 1);
        assert_eq!(forced[0].submission_id, old_id);
        assert!(forced[0].age_slots > 100);
    }

    #[test]
    fn test_forced_inclusion_candidates_correctly_identified() {
        let policy = FairSequencingPolicy::default_policy();
        let mut engine = InclusionPolicyEngine::new(policy);

        let current_slot = 500u64;
        let mut candidates = BTreeSet::new();
        let mut profiles = BTreeMap::new();

        let starved_id = make_id(10);
        let normal_id = make_id(20);

        candidates.insert(starved_id);
        candidates.insert(normal_id);

        // starved_id has been waiting 200 slots (> anti_starvation_slots=100)
        profiles.insert(starved_id, make_profile(starved_id, FairnessClass::Normal, 300, current_slot));
        profiles.insert(normal_id, make_profile(normal_id, FairnessClass::Normal, 499, current_slot));

        let (records, forced) = engine.compute_eligible_set(&candidates, &profiles, current_slot, 10);
        assert!(!forced.is_empty(), "Expected forced inclusion candidates");
        assert!(forced.iter().any(|f| f.submission_id == starved_id));

        let starved_record = records.iter().find(|r| r.submission_id == starved_id).unwrap();
        assert!(matches!(starved_record.decision, InclusionDecision::ForceInclude(_)));
    }

    #[test]
    fn test_submission_age_tracker_basic() {
        let mut tracker = SubmissionAgeTracker::new();
        let id = make_id(1);
        tracker.register(id, 10, 0);
        assert_eq!(tracker.get_age(&id, 20), 10);
        assert_eq!(tracker.get_age(&id, 10), 0);
        tracker.remove(&id);
        assert_eq!(tracker.get_age(&id, 20), 0);
    }

    #[test]
    fn test_get_stale_beyond() {
        let mut tracker = SubmissionAgeTracker::new();
        tracker.register(make_id(1), 0, 0);   // age = 100 at current_slot=100
        tracker.register(make_id(2), 95, 0);  // age = 5 at current_slot=100
        tracker.register(make_id(3), 50, 0);  // age = 50 at current_slot=100

        let stale = tracker.get_stale_beyond(100, 49);
        // IDs with age > 49: id1 (age=100), id3 (age=50)
        assert!(stale.contains(&make_id(1)));
        assert!(stale.contains(&make_id(3)));
        assert!(!stale.contains(&make_id(2)));
    }
}
