use std::collections::BTreeSet;
use crate::fairness::policy::FairnessClass;

/// Bounds on how far a submission can be reordered within a batch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReorderBound {
    /// Maximum positions a submission can be promoted (moved earlier).
    pub max_positions_forward: u32,
    /// Maximum positions a submission can be delayed (moved later).
    pub max_positions_backward: u32,
    /// None = applies to all classes; Some(class) = applies only to that class.
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

    pub fn strict_bound() -> Self {
        ReorderBound {
            max_positions_forward: 0,
            max_positions_backward: 0,
            applies_to_class: None,
        }
    }

    pub fn for_class(class: FairnessClass, forward: u32, backward: u32) -> Self {
        ReorderBound {
            max_positions_forward: forward,
            max_positions_backward: backward,
            applies_to_class: Some(class),
        }
    }
}

/// Policy for protected flow handling inside anti-MEV controls.
#[derive(Debug, Clone)]
pub struct ProtectedFlowPolicy {
    pub enabled: bool,
    pub protected_classes: BTreeSet<FairnessClass>,
    /// Protected flows must be included within this many slots.
    pub max_delay_slots: u64,
}

impl ProtectedFlowPolicy {
    pub fn default_policy() -> Self {
        let mut protected_classes = BTreeSet::new();
        protected_classes.insert(FairnessClass::SafetyCritical);
        protected_classes.insert(FairnessClass::ProtectedUserFlow);
        protected_classes.insert(FairnessClass::BridgeAdjacent);
        protected_classes.insert(FairnessClass::GovernanceSensitive);
        ProtectedFlowPolicy {
            enabled: true,
            protected_classes,
            max_delay_slots: 5,
        }
    }
}

/// Limits on how much of the batch ordering the leader can freely control.
#[derive(Debug, Clone)]
pub struct LeaderDiscretionLimit {
    /// 0-10000 bps: max fraction of batch positions leader can freely reorder.
    pub max_discretion_bps: u32,
    /// If true, protected flows are counted against discretion limit even more strictly.
    pub apply_to_protected: bool,
}

impl LeaderDiscretionLimit {
    pub fn default_limit() -> Self {
        LeaderDiscretionLimit {
            max_discretion_bps: 1000, // 10%
            apply_to_protected: true,
        }
    }
}

/// Commitment rules for snapshot-based ordering.
#[derive(Debug, Clone)]
pub struct OrderingCommitmentRule {
    /// Leader must commit to a snapshot hash before ordering.
    pub require_snapshot_precommit: bool,
    /// How many slots a snapshot commitment is valid.
    pub snapshot_validity_slots: u64,
}

impl OrderingCommitmentRule {
    pub fn default_rule() -> Self {
        OrderingCommitmentRule {
            require_snapshot_precommit: true,
            snapshot_validity_slots: 3,
        }
    }
}

/// Top-level anti-MEV policy governing ordering constraints.
#[derive(Debug, Clone)]
pub struct AntiMevPolicy {
    pub version: u32,
    pub reorder_bounds: Vec<ReorderBound>,
    /// Leader must commit to snapshot before ordering.
    pub ordering_commitment_required: bool,
    pub protected_flow_policy: ProtectedFlowPolicy,
    pub leader_discretion_limit: LeaderDiscretionLimit,
    /// Placeholder for future sandwich detection (not implemented).
    pub anti_sandwich_hook_enabled: bool,
}

impl AntiMevPolicy {
    pub fn default_policy() -> Self {
        AntiMevPolicy {
            version: 1,
            reorder_bounds: vec![
                ReorderBound::default_bound(),
                ReorderBound::for_class(FairnessClass::SafetyCritical, 0, 0),
                ReorderBound::for_class(FairnessClass::ProtectedUserFlow, 1, 1),
                ReorderBound::for_class(FairnessClass::BridgeAdjacent, 2, 2),
            ],
            ordering_commitment_required: true,
            protected_flow_policy: ProtectedFlowPolicy::default_policy(),
            leader_discretion_limit: LeaderDiscretionLimit::default_limit(),
            anti_sandwich_hook_enabled: false,
        }
    }

    /// Get the most specific reorder bound for a given class.
    /// Class-specific bounds take precedence over the global bound (applies_to_class == None).
    pub fn get_reorder_bound_for(&self, class: &FairnessClass) -> Option<&ReorderBound> {
        // First look for a class-specific bound
        let specific = self.reorder_bounds
            .iter()
            .find(|b| b.applies_to_class.as_ref() == Some(class));
        if specific.is_some() {
            return specific;
        }
        // Fall back to global bound
        self.reorder_bounds
            .iter()
            .find(|b| b.applies_to_class.is_none())
    }

    /// Validate that the given position delta is within allowed bounds for the class.
    ///
    /// `positions_moved` is the signed delta: negative = promoted (moved earlier in batch),
    /// positive = demoted (moved later in batch).
    ///
    /// Returns true if the delta is within bounds, false if it violates policy.
    pub fn validate_ordering_delta(&self, class: &FairnessClass, positions_moved: i64) -> bool {
        match self.get_reorder_bound_for(class) {
            None => true, // no bound = unrestricted
            Some(bound) => {
                let forward_ok = if positions_moved < 0 {
                    // Promoted: moved earlier, absolute value is forward movement
                    (-positions_moved) as u64 <= bound.max_positions_forward as u64
                } else {
                    true
                };
                let backward_ok = if positions_moved > 0 {
                    positions_moved as u64 <= bound.max_positions_backward as u64
                } else {
                    true
                };
                forward_ok && backward_ok
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_policy_sane_values() {
        let policy = AntiMevPolicy::default_policy();
        assert_eq!(policy.version, 1);
        assert!(policy.ordering_commitment_required);
        assert!(!policy.anti_sandwich_hook_enabled);
        assert!(!policy.reorder_bounds.is_empty());
        // Should have a global bound
        assert!(policy.get_reorder_bound_for(&FairnessClass::Normal).is_some());
    }

    #[test]
    fn test_validate_ordering_delta_within_bound_passes() {
        let policy = AntiMevPolicy::default_policy();
        // Default global bound: max 5 forward, 5 backward
        assert!(policy.validate_ordering_delta(&FairnessClass::Normal, 0));
        assert!(policy.validate_ordering_delta(&FairnessClass::Normal, 5));
        assert!(policy.validate_ordering_delta(&FairnessClass::Normal, -5));
        assert!(policy.validate_ordering_delta(&FairnessClass::Normal, 3));
        assert!(policy.validate_ordering_delta(&FairnessClass::Normal, -3));
    }

    #[test]
    fn test_validate_ordering_delta_exceeding_forward_bound_fails() {
        let policy = AntiMevPolicy::default_policy();
        // Moved 6 positions earlier (forward) — exceeds max_positions_forward=5
        assert!(!policy.validate_ordering_delta(&FairnessClass::Normal, -6));
        assert!(!policy.validate_ordering_delta(&FairnessClass::Normal, -10));
    }

    #[test]
    fn test_validate_ordering_delta_exceeding_backward_bound_fails() {
        let policy = AntiMevPolicy::default_policy();
        // Moved 6 positions later (backward) — exceeds max_positions_backward=5
        assert!(!policy.validate_ordering_delta(&FairnessClass::Normal, 6));
        assert!(!policy.validate_ordering_delta(&FairnessClass::Normal, 100));
    }

    #[test]
    fn test_safety_critical_strict_bound() {
        let policy = AntiMevPolicy::default_policy();
        // SafetyCritical bound = 0 forward, 0 backward
        assert!(policy.validate_ordering_delta(&FairnessClass::SafetyCritical, 0));
        assert!(!policy.validate_ordering_delta(&FairnessClass::SafetyCritical, 1));
        assert!(!policy.validate_ordering_delta(&FairnessClass::SafetyCritical, -1));
    }

    #[test]
    fn test_get_reorder_bound_class_specific_takes_precedence() {
        let policy = AntiMevPolicy::default_policy();
        let sc_bound = policy.get_reorder_bound_for(&FairnessClass::SafetyCritical).unwrap();
        assert_eq!(sc_bound.max_positions_forward, 0);
        assert_eq!(sc_bound.max_positions_backward, 0);

        let normal_bound = policy.get_reorder_bound_for(&FairnessClass::Normal).unwrap();
        assert_eq!(normal_bound.max_positions_forward, 5);
    }

    #[test]
    fn test_protected_flow_policy_default() {
        let pfp = ProtectedFlowPolicy::default_policy();
        assert!(pfp.enabled);
        assert!(pfp.protected_classes.contains(&FairnessClass::SafetyCritical));
        assert!(pfp.protected_classes.contains(&FairnessClass::ProtectedUserFlow));
        assert!(pfp.max_delay_slots > 0);
    }
}
