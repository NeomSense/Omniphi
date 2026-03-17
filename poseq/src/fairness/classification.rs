use std::collections::{BTreeMap, BTreeSet};
use crate::fairness::policy::FairnessClass;

/// A rule that maps metadata key/value pairs to a FairnessClass.
#[derive(Debug, Clone)]
pub struct ClassAssignmentRule {
    pub name: String,
    pub matches_metadata_key: Option<String>,
    pub matches_metadata_value: Option<String>,
    pub assigned_class: FairnessClass,
    /// Higher priority rules are applied first.
    pub priority: u32,
}

/// A bucket grouping submission IDs that share the same FairnessClass.
/// submission_ids is ordered by receipt sequence (via BTreeSet on the raw [u8;32] id,
/// which is insertion-sequence-stamped by the caller's sequence numbering).
#[derive(Debug, Clone)]
pub struct FairnessBucket {
    pub class: FairnessClass,
    /// Ordered by receipt sequence: callers must use sequence-stamped IDs or
    /// sort externally; the BTreeSet here preserves lexicographic order of the IDs.
    pub submission_ids: BTreeSet<[u8; 32]>,
}

/// Classifies submissions into FairnessClasses using a prioritized rule set.
#[derive(Debug, Clone)]
pub struct FairnessClassifier {
    /// Rules sorted descending by priority (highest priority first).
    pub rules: Vec<ClassAssignmentRule>,
}

impl FairnessClassifier {
    /// Create a new classifier. Rules are sorted by priority descending on construction.
    pub fn new(mut rules: Vec<ClassAssignmentRule>) -> Self {
        rules.sort_by(|a, b| b.priority.cmp(&a.priority));
        FairnessClassifier { rules }
    }

    /// Classify a single submission using its metadata.
    /// Returns the first matching rule's assigned class, or Normal if no rule matches.
    pub fn classify(
        &self,
        _submission_id: &[u8; 32],
        metadata: &BTreeMap<String, String>,
    ) -> FairnessClass {
        for rule in &self.rules {
            let key_match = match &rule.matches_metadata_key {
                None => true,
                Some(k) => metadata.contains_key(k),
            };
            let value_match = match (&rule.matches_metadata_key, &rule.matches_metadata_value) {
                (Some(k), Some(v)) => metadata.get(k).map(|mv| mv == v).unwrap_or(false),
                (Some(_), None) => key_match, // key present, any value
                (None, _) => true,            // no key constraint
            };
            if key_match && value_match {
                return rule.assigned_class.clone();
            }
        }
        FairnessClass::Normal
    }

    /// Classify all submissions in the map. Returns a map from submission_id to FairnessClass.
    pub fn classify_all(
        &self,
        submissions: &BTreeMap<[u8; 32], BTreeMap<String, String>>,
    ) -> BTreeMap<[u8; 32], FairnessClass> {
        submissions
            .iter()
            .map(|(id, metadata)| (*id, self.classify(id, metadata)))
            .collect()
    }

    /// Group a classification map into FairnessBuckets keyed by FairnessClass.
    pub fn build_buckets(
        classifications: &BTreeMap<[u8; 32], FairnessClass>,
    ) -> BTreeMap<FairnessClass, FairnessBucket> {
        let mut buckets: BTreeMap<FairnessClass, FairnessBucket> = BTreeMap::new();
        for (id, class) in classifications {
            buckets
                .entry(class.clone())
                .or_insert_with(|| FairnessBucket {
                    class: class.clone(),
                    submission_ids: BTreeSet::new(),
                })
                .submission_ids
                .insert(*id);
        }
        buckets
    }
}

/// Per-submission fairness profiling record.
#[derive(Debug, Clone)]
pub struct SubmissionFairnessProfile {
    pub submission_id: [u8; 32],
    pub assigned_class: FairnessClass,
    pub received_at_slot: u64,
    pub received_at_sequence: u64,
    /// current_slot - received_at_slot
    pub age_slots: u64,
    pub is_forced_inclusion: bool,
    /// Which rule matched (rule name, or "default:Normal" if none matched).
    pub classification_rule: String,
}

impl SubmissionFairnessProfile {
    pub fn new(
        submission_id: [u8; 32],
        assigned_class: FairnessClass,
        received_at_slot: u64,
        received_at_sequence: u64,
        current_slot: u64,
        is_forced_inclusion: bool,
        classification_rule: String,
    ) -> Self {
        let age_slots = current_slot.saturating_sub(received_at_slot);
        SubmissionFairnessProfile {
            submission_id,
            assigned_class,
            received_at_slot,
            received_at_sequence,
            age_slots,
            is_forced_inclusion,
            classification_rule,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_rule(name: &str, key: &str, value: &str, class: FairnessClass, priority: u32) -> ClassAssignmentRule {
        ClassAssignmentRule {
            name: name.to_string(),
            matches_metadata_key: Some(key.to_string()),
            matches_metadata_value: Some(value.to_string()),
            assigned_class: class,
            priority,
        }
    }

    #[test]
    fn test_classify_with_matching_rule() {
        let rules = vec![
            make_rule("governance", "type", "governance", FairnessClass::GovernanceSensitive, 100),
            make_rule("solver", "type", "solver", FairnessClass::SolverRelated, 90),
        ];
        let classifier = FairnessClassifier::new(rules);
        let mut metadata = BTreeMap::new();
        metadata.insert("type".to_string(), "governance".to_string());
        let class = classifier.classify(&make_id(1), &metadata);
        assert_eq!(class, FairnessClass::GovernanceSensitive);
    }

    #[test]
    fn test_classify_no_match_defaults_to_normal() {
        let rules = vec![make_rule("solver", "type", "solver", FairnessClass::SolverRelated, 10)];
        let classifier = FairnessClassifier::new(rules);
        let metadata = BTreeMap::new();
        let class = classifier.classify(&make_id(1), &metadata);
        assert_eq!(class, FairnessClass::Normal);
    }

    #[test]
    fn test_classify_higher_priority_rule_wins() {
        let rules = vec![
            make_rule("low", "type", "tx", FairnessClass::LatencySensitive, 10),
            make_rule("high", "type", "tx", FairnessClass::SafetyCritical, 100),
        ];
        let classifier = FairnessClassifier::new(rules);
        let mut metadata = BTreeMap::new();
        metadata.insert("type".to_string(), "tx".to_string());
        let class = classifier.classify(&make_id(1), &metadata);
        // High-priority rule should win
        assert_eq!(class, FairnessClass::SafetyCritical);
    }

    #[test]
    fn test_classify_all_handles_multiple_submissions() {
        let rules = vec![
            make_rule("bridge", "domain", "bridge", FairnessClass::BridgeAdjacent, 50),
            make_rule("solver", "type", "solver", FairnessClass::SolverRelated, 40),
        ];
        let classifier = FairnessClassifier::new(rules);

        let mut submissions: BTreeMap<[u8; 32], BTreeMap<String, String>> = BTreeMap::new();
        let id1 = make_id(1);
        let id2 = make_id(2);
        let id3 = make_id(3);

        let mut m1 = BTreeMap::new();
        m1.insert("domain".to_string(), "bridge".to_string());
        let mut m2 = BTreeMap::new();
        m2.insert("type".to_string(), "solver".to_string());
        let m3 = BTreeMap::new(); // no match → Normal

        submissions.insert(id1, m1);
        submissions.insert(id2, m2);
        submissions.insert(id3, m3);

        let results = classifier.classify_all(&submissions);
        assert_eq!(results[&id1], FairnessClass::BridgeAdjacent);
        assert_eq!(results[&id2], FairnessClass::SolverRelated);
        assert_eq!(results[&id3], FairnessClass::Normal);
    }

    #[test]
    fn test_build_buckets_groups_by_class() {
        let mut classifications: BTreeMap<[u8; 32], FairnessClass> = BTreeMap::new();
        classifications.insert(make_id(1), FairnessClass::Normal);
        classifications.insert(make_id(2), FairnessClass::Normal);
        classifications.insert(make_id(3), FairnessClass::SafetyCritical);

        let buckets = FairnessClassifier::build_buckets(&classifications);
        assert_eq!(buckets.len(), 2);
        assert_eq!(buckets[&FairnessClass::Normal].submission_ids.len(), 2);
        assert_eq!(buckets[&FairnessClass::SafetyCritical].submission_ids.len(), 1);
        assert!(buckets[&FairnessClass::Normal].submission_ids.contains(&make_id(1)));
        assert!(buckets[&FairnessClass::Normal].submission_ids.contains(&make_id(2)));
    }

    #[test]
    fn test_build_buckets_empty_input() {
        let classifications: BTreeMap<[u8; 32], FairnessClass> = BTreeMap::new();
        let buckets = FairnessClassifier::build_buckets(&classifications);
        assert!(buckets.is_empty());
    }

    #[test]
    fn test_submission_fairness_profile_age_computation() {
        let profile = SubmissionFairnessProfile::new(
            make_id(1),
            FairnessClass::Normal,
            10, // received_at_slot
            5,
            25, // current_slot
            false,
            "default:Normal".to_string(),
        );
        assert_eq!(profile.age_slots, 15);
    }

    #[test]
    fn test_rules_sorted_by_priority_descending() {
        let rules = vec![
            ClassAssignmentRule {
                name: "low".to_string(),
                matches_metadata_key: None,
                matches_metadata_value: None,
                assigned_class: FairnessClass::Normal,
                priority: 1,
            },
            ClassAssignmentRule {
                name: "high".to_string(),
                matches_metadata_key: None,
                matches_metadata_value: None,
                assigned_class: FairnessClass::SafetyCritical,
                priority: 100,
            },
        ];
        let classifier = FairnessClassifier::new(rules);
        // First rule after sorting should be highest priority
        assert_eq!(classifier.rules[0].priority, 100);
        assert_eq!(classifier.rules[1].priority, 1);
    }
}
