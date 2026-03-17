use std::collections::BTreeMap;

/// Fairness classification for submissions.
/// Ordering is deterministic (derived Ord uses declaration order).
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum FairnessClass {
    BridgeAdjacent,
    DomainSensitive,
    GovernanceSensitive,
    LatencySensitive,
    Normal,
    ProtectedUserFlow,
    RestrictedPriority,
    SafetyCritical,
    SolverRelated,
}

/// Per-class sequencing rules attached to a FairSequencingPolicy.
#[derive(Debug, Clone)]
pub struct SubmissionFairnessClass {
    pub class: FairnessClass,
    /// 0-10000 basis points priority weight.
    pub priority_weight: u32,
    /// Cannot be reordered past ordering_commit_window when true.
    pub protected: bool,
    /// 0 = no forced inclusion limit. >0 = force-include after this many slots.
    pub max_wait_slots: u64,
}

/// How positions in a batch can be reordered for a given class.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReorderBound {
    pub max_positions_forward: u32,
    pub max_positions_backward: u32,
    pub applies_to_class: Option<FairnessClass>,
}

impl ReorderBound {
    pub fn default_bound() -> Self {
        ReorderBound {
            max_positions_forward: 5,
            max_positions_backward: 5,
            applies_to_class: None,
        }
    }
}

/// Ordering fairness rules for a policy.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum OrderingFairnessRule {
    /// Within a class, strictly by receipt order.
    StrictFIFOWithinClass,
    /// Priority can advance at most N positions.
    BoundedPriority(u32),
    /// Protected classes get ordered before Normal.
    ProtectedClassFirst,
    /// Age-weighted promotion after threshold slots.
    AntiStarvationPromote,
}

/// Full set of fairness constraints applied to a proposal.
#[derive(Debug, Clone)]
pub struct FairnessConstraintSet {
    pub rules: Vec<OrderingFairnessRule>,
    /// Leader must follow queue snapshot order.
    pub enforce_snapshot_ordering: bool,
    /// Max times a valid eligible submission can be skipped.
    pub max_omission_streak: u32,
}

impl FairnessConstraintSet {
    pub fn default_constraints() -> Self {
        FairnessConstraintSet {
            rules: vec![
                OrderingFairnessRule::ProtectedClassFirst,
                OrderingFairnessRule::StrictFIFOWithinClass,
                OrderingFairnessRule::AntiStarvationPromote,
            ],
            enforce_snapshot_ordering: true,
            max_omission_streak: 3,
        }
    }
}

/// The top-level fair sequencing policy governing how batches are ordered.
#[derive(Debug, Clone)]
pub struct FairSequencingPolicy {
    pub version: u32,
    pub class_rules: BTreeMap<FairnessClass, SubmissionFairnessClass>,
    pub reorder_bound: ReorderBound,
    /// Force-include after this many slots of waiting.
    pub anti_starvation_slots: u64,
    /// Max % of batch positions leader can freely order (0-10000 bps).
    pub max_leader_discretion_bps: u32,
    pub protected_flow_enabled: bool,
}

impl FairSequencingPolicy {
    /// Canonical default policy with sensible parameters.
    pub fn default_policy() -> Self {
        let mut class_rules = BTreeMap::new();

        let classes: &[(FairnessClass, u32, bool, u64)] = &[
            (FairnessClass::Normal, 1000, false, 0),
            (FairnessClass::LatencySensitive, 3000, false, 10),
            (FairnessClass::SolverRelated, 2000, false, 20),
            (FairnessClass::GovernanceSensitive, 4000, true, 5),
            (FairnessClass::SafetyCritical, 9000, true, 2),
            (FairnessClass::ProtectedUserFlow, 8000, true, 3),
            (FairnessClass::RestrictedPriority, 500, false, 50),
            (FairnessClass::BridgeAdjacent, 5000, true, 4),
            (FairnessClass::DomainSensitive, 2500, false, 30),
        ];

        for (class, weight, protected, max_wait) in classes {
            class_rules.insert(
                class.clone(),
                SubmissionFairnessClass {
                    class: class.clone(),
                    priority_weight: *weight,
                    protected: *protected,
                    max_wait_slots: *max_wait,
                },
            );
        }

        FairSequencingPolicy {
            version: 1,
            class_rules,
            reorder_bound: ReorderBound::default_bound(),
            anti_starvation_slots: 100,
            max_leader_discretion_bps: 1000, // 10%
            protected_flow_enabled: true,
        }
    }

    /// Get the class rule for a given FairnessClass.
    /// Falls back to a synthetic Normal rule if not found.
    pub fn get_class_rule(&self, class: &FairnessClass) -> &SubmissionFairnessClass {
        self.class_rules.get(class).unwrap_or_else(|| {
            // Safety: Normal is always present in default_policy. If somehow absent,
            // we can't return a reference to a temporary — callers should ensure Normal exists.
            self.class_rules.get(&FairnessClass::Normal)
                .expect("FairSequencingPolicy must always contain a Normal class rule")
        })
    }

    /// Returns true if the given class is marked protected.
    pub fn is_protected(&self, class: &FairnessClass) -> bool {
        self.class_rules.get(class).map(|r| r.protected).unwrap_or(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_policy_has_all_classes() {
        let policy = FairSequencingPolicy::default_policy();
        assert_eq!(policy.version, 1);
        assert!(policy.class_rules.contains_key(&FairnessClass::Normal));
        assert!(policy.class_rules.contains_key(&FairnessClass::SafetyCritical));
        assert!(policy.class_rules.contains_key(&FairnessClass::ProtectedUserFlow));
        assert!(policy.class_rules.contains_key(&FairnessClass::BridgeAdjacent));
        assert_eq!(policy.class_rules.len(), 9);
    }

    #[test]
    fn test_default_policy_sane_values() {
        let policy = FairSequencingPolicy::default_policy();
        assert!(policy.anti_starvation_slots > 0);
        assert!(policy.max_leader_discretion_bps <= 10000);
        assert!(policy.protected_flow_enabled);
        // SafetyCritical should have highest weight
        let sc_rule = policy.get_class_rule(&FairnessClass::SafetyCritical);
        let normal_rule = policy.get_class_rule(&FairnessClass::Normal);
        assert!(sc_rule.priority_weight > normal_rule.priority_weight);
    }

    #[test]
    fn test_is_protected_returns_correct_values() {
        let policy = FairSequencingPolicy::default_policy();
        assert!(policy.is_protected(&FairnessClass::SafetyCritical));
        assert!(policy.is_protected(&FairnessClass::ProtectedUserFlow));
        assert!(policy.is_protected(&FairnessClass::GovernanceSensitive));
        assert!(policy.is_protected(&FairnessClass::BridgeAdjacent));
        assert!(!policy.is_protected(&FairnessClass::Normal));
        assert!(!policy.is_protected(&FairnessClass::LatencySensitive));
        assert!(!policy.is_protected(&FairnessClass::SolverRelated));
        assert!(!policy.is_protected(&FairnessClass::RestrictedPriority));
    }

    #[test]
    fn test_get_class_rule_returns_correct_rule() {
        let policy = FairSequencingPolicy::default_policy();
        let rule = policy.get_class_rule(&FairnessClass::LatencySensitive);
        assert_eq!(rule.class, FairnessClass::LatencySensitive);
        assert_eq!(rule.priority_weight, 3000);
        assert!(!rule.protected);
        assert_eq!(rule.max_wait_slots, 10);
    }

    #[test]
    fn test_get_class_rule_normal_fallback() {
        let policy = FairSequencingPolicy::default_policy();
        // Normal class always exists
        let rule = policy.get_class_rule(&FairnessClass::Normal);
        assert_eq!(rule.class, FairnessClass::Normal);
    }

    #[test]
    fn test_fairness_class_ord_is_deterministic() {
        let mut classes = vec![
            FairnessClass::Normal,
            FairnessClass::SafetyCritical,
            FairnessClass::BridgeAdjacent,
        ];
        classes.sort();
        // BTreeMap/BTreeSet require deterministic ordering — verify sort is stable
        assert_eq!(classes[0], FairnessClass::BridgeAdjacent);
        assert_eq!(classes[1], FairnessClass::Normal);
        assert_eq!(classes[2], FairnessClass::SafetyCritical);
    }

    #[test]
    fn test_reorder_bound_default() {
        let bound = ReorderBound::default_bound();
        assert_eq!(bound.max_positions_forward, 5);
        assert_eq!(bound.max_positions_backward, 5);
        assert!(bound.applies_to_class.is_none());
    }

    #[test]
    fn test_fairness_constraint_set_defaults() {
        let cs = FairnessConstraintSet::default_constraints();
        assert!(cs.enforce_snapshot_ordering);
        assert!(cs.max_omission_streak > 0);
        assert!(cs.rules.contains(&OrderingFairnessRule::ProtectedClassFirst));
        assert!(cs.rules.contains(&OrderingFairnessRule::StrictFIFOWithinClass));
    }
}
