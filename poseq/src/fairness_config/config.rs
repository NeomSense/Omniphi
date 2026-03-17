use std::collections::BTreeMap;
use std::collections::BTreeSet;
use crate::fairness::policy::{
    FairSequencingPolicy, FairnessClass, SubmissionFairnessClass, ReorderBound,
};
use crate::anti_mev::policy::{
    AntiMevPolicy, ProtectedFlowPolicy, LeaderDiscretionLimit,
};

/// Inclusion-related configuration parameters.
#[derive(Debug, Clone)]
pub struct InclusionConfig {
    pub batch_capacity: usize,
    pub anti_starvation_slots: u64,
    pub max_inclusion_age_slots: u64,
    pub force_include_protected_after_slots: u64,
}

impl InclusionConfig {
    pub fn default_config() -> Self {
        InclusionConfig {
            batch_capacity: 256,
            anti_starvation_slots: 100,
            max_inclusion_age_slots: 500,
            force_include_protected_after_slots: 5,
        }
    }
}

/// Thresholds for triggering fairness warnings.
#[derive(Debug, Clone)]
pub struct FairnessThresholds {
    /// Max times a submission can be skipped before an omission incident fires.
    pub max_omission_streak: u32,
    /// Trigger warning if one class receives fewer than X bps of batch slots.
    pub max_class_skew_bps: u32,
    /// Minimum bps of the batch that must be Normal class.
    pub min_normal_class_bps: u32,
}

impl FairnessThresholds {
    pub fn default_thresholds() -> Self {
        FairnessThresholds {
            max_omission_streak: 3,
            max_class_skew_bps: 500,
            min_normal_class_bps: 1000,
        }
    }
}

/// Anti-MEV tuning parameters.
#[derive(Debug, Clone)]
pub struct AntiMevConfig {
    pub max_reorder_positions_forward: u32,
    pub max_reorder_positions_backward: u32,
    pub ordering_commitment_required: bool,
    pub leader_discretion_max_bps: u32,
    pub protected_flow_max_delay_slots: u64,
}

impl AntiMevConfig {
    pub fn default_config() -> Self {
        AntiMevConfig {
            max_reorder_positions_forward: 5,
            max_reorder_positions_backward: 5,
            ordering_commitment_required: true,
            leader_discretion_max_bps: 1000,
            protected_flow_max_delay_slots: 5,
        }
    }
}

/// Configuration for protected flow handling.
#[derive(Debug, Clone)]
pub struct ProtectedFlowConfig {
    pub enabled: bool,
    pub protected_classes: Vec<FairnessClass>,
    pub max_delay_slots: u64,
}

impl ProtectedFlowConfig {
    pub fn default_config() -> Self {
        ProtectedFlowConfig {
            enabled: true,
            protected_classes: vec![
                FairnessClass::SafetyCritical,
                FairnessClass::ProtectedUserFlow,
                FairnessClass::BridgeAdjacent,
                FairnessClass::GovernanceSensitive,
            ],
            max_delay_slots: 5,
        }
    }
}

/// Top-level fairness configuration, combining all sub-configs.
#[derive(Debug, Clone)]
pub struct FairnessConfig {
    pub version: u32,
    pub inclusion: InclusionConfig,
    pub thresholds: FairnessThresholds,
    pub anti_mev: AntiMevConfig,
    pub protected_flows: ProtectedFlowConfig,
    pub audit_enabled: bool,
    pub incident_detection_enabled: bool,
}

impl FairnessConfig {
    pub fn default_config() -> Self {
        FairnessConfig {
            version: 1,
            inclusion: InclusionConfig::default_config(),
            thresholds: FairnessThresholds::default_thresholds(),
            anti_mev: AntiMevConfig::default_config(),
            protected_flows: ProtectedFlowConfig::default_config(),
            audit_enabled: true,
            incident_detection_enabled: true,
        }
    }

    /// Convert this config to a FairSequencingPolicy.
    pub fn to_fair_sequencing_policy(&self) -> FairSequencingPolicy {
        let mut class_rules: BTreeMap<FairnessClass, SubmissionFairnessClass> = BTreeMap::new();

        let protected_set: BTreeSet<FairnessClass> = self
            .protected_flows
            .protected_classes
            .iter()
            .cloned()
            .collect();

        let class_weights: &[(FairnessClass, u32, u64)] = &[
            (FairnessClass::Normal, 1000, 0),
            (FairnessClass::LatencySensitive, 3000, 10),
            (FairnessClass::SolverRelated, 2000, 20),
            (FairnessClass::GovernanceSensitive, 4000, self.protected_flows.max_delay_slots),
            (FairnessClass::SafetyCritical, 9000, self.protected_flows.max_delay_slots),
            (FairnessClass::ProtectedUserFlow, 8000, self.protected_flows.max_delay_slots),
            (FairnessClass::RestrictedPriority, 500, 50),
            (FairnessClass::BridgeAdjacent, 5000, self.protected_flows.max_delay_slots),
            (FairnessClass::DomainSensitive, 2500, 30),
        ];

        for (class, weight, max_wait) in class_weights {
            let is_protected = protected_set.contains(class);
            class_rules.insert(
                class.clone(),
                SubmissionFairnessClass {
                    class: class.clone(),
                    priority_weight: *weight,
                    protected: is_protected,
                    max_wait_slots: *max_wait,
                },
            );
        }

        FairSequencingPolicy {
            version: self.version,
            class_rules,
            reorder_bound: ReorderBound {
                max_positions_forward: self.anti_mev.max_reorder_positions_forward,
                max_positions_backward: self.anti_mev.max_reorder_positions_backward,
                applies_to_class: None,
            },
            anti_starvation_slots: self.inclusion.anti_starvation_slots,
            max_leader_discretion_bps: self.anti_mev.leader_discretion_max_bps,
            protected_flow_enabled: self.protected_flows.enabled,
        }
    }

    /// Convert this config to an AntiMevPolicy.
    pub fn to_anti_mev_policy(&self) -> AntiMevPolicy {
        use crate::anti_mev::policy::ReorderBound as AntiBound;

        let mut protected_classes = BTreeSet::new();
        for class in &self.protected_flows.protected_classes {
            protected_classes.insert(class.clone());
        }

        AntiMevPolicy {
            version: self.version,
            reorder_bounds: vec![
                AntiBound {
                    max_positions_forward: self.anti_mev.max_reorder_positions_forward,
                    max_positions_backward: self.anti_mev.max_reorder_positions_backward,
                    applies_to_class: None,
                },
                // SafetyCritical: strict (0 reorder)
                AntiBound {
                    max_positions_forward: 0,
                    max_positions_backward: 0,
                    applies_to_class: Some(FairnessClass::SafetyCritical),
                },
            ],
            ordering_commitment_required: self.anti_mev.ordering_commitment_required,
            protected_flow_policy: ProtectedFlowPolicy {
                enabled: self.protected_flows.enabled,
                protected_classes,
                max_delay_slots: self.anti_mev.protected_flow_max_delay_slots,
            },
            leader_discretion_limit: LeaderDiscretionLimit {
                max_discretion_bps: self.anti_mev.leader_discretion_max_bps,
                apply_to_protected: true,
            },
            anti_sandwich_hook_enabled: false,
        }
    }

    /// Convert this config to a ProtectedFlowPolicy.
    pub fn to_protected_flow_policy(&self) -> ProtectedFlowPolicy {
        let mut protected_classes = BTreeSet::new();
        for class in &self.protected_flows.protected_classes {
            protected_classes.insert(class.clone());
        }
        ProtectedFlowPolicy {
            enabled: self.protected_flows.enabled,
            protected_classes,
            max_delay_slots: self.anti_mev.protected_flow_max_delay_slots,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config_produces_sensible_values() {
        let config = FairnessConfig::default_config();
        assert_eq!(config.version, 1);
        assert!(config.audit_enabled);
        assert!(config.incident_detection_enabled);
        assert!(config.protected_flows.enabled);
        assert!(config.inclusion.batch_capacity > 0);
        assert!(config.inclusion.anti_starvation_slots > 0);
        assert!(config.anti_mev.max_reorder_positions_forward > 0);
        assert!(config.anti_mev.max_reorder_positions_backward > 0);
        assert!(config.anti_mev.ordering_commitment_required);
        assert!(config.anti_mev.leader_discretion_max_bps <= 10000);
    }

    #[test]
    fn test_to_fair_sequencing_policy_maps_correctly() {
        let config = FairnessConfig::default_config();
        let policy = config.to_fair_sequencing_policy();

        assert_eq!(policy.version, config.version);
        assert_eq!(policy.anti_starvation_slots, config.inclusion.anti_starvation_slots);
        assert_eq!(policy.max_leader_discretion_bps, config.anti_mev.leader_discretion_max_bps);
        assert_eq!(policy.protected_flow_enabled, config.protected_flows.enabled);

        // SafetyCritical should be protected
        assert!(policy.is_protected(&FairnessClass::SafetyCritical));
        assert!(policy.is_protected(&FairnessClass::ProtectedUserFlow));
        // Normal should not be protected
        assert!(!policy.is_protected(&FairnessClass::Normal));

        // All 9 classes should be present
        assert_eq!(policy.class_rules.len(), 9);

        // Reorder bound should match anti_mev config
        assert_eq!(
            policy.reorder_bound.max_positions_forward,
            config.anti_mev.max_reorder_positions_forward
        );
    }

    #[test]
    fn test_to_anti_mev_policy_maps_correctly() {
        let config = FairnessConfig::default_config();
        let anti_mev = config.to_anti_mev_policy();

        assert_eq!(anti_mev.version, config.version);
        assert_eq!(anti_mev.ordering_commitment_required, config.anti_mev.ordering_commitment_required);
        assert_eq!(
            anti_mev.leader_discretion_limit.max_discretion_bps,
            config.anti_mev.leader_discretion_max_bps
        );
        assert_eq!(
            anti_mev.protected_flow_policy.max_delay_slots,
            config.anti_mev.protected_flow_max_delay_slots
        );
        assert_eq!(
            anti_mev.protected_flow_policy.enabled,
            config.protected_flows.enabled
        );

        // SafetyCritical should be in protected classes
        assert!(anti_mev.protected_flow_policy.protected_classes.contains(&FairnessClass::SafetyCritical));

        // Should have global bound + SafetyCritical-specific bound
        assert!(anti_mev.reorder_bounds.len() >= 2);
        let sc_bound = anti_mev.get_reorder_bound_for(&FairnessClass::SafetyCritical).unwrap();
        assert_eq!(sc_bound.max_positions_forward, 0);
    }

    #[test]
    fn test_to_protected_flow_policy_maps_correctly() {
        let config = FairnessConfig::default_config();
        let pfp = config.to_protected_flow_policy();
        assert_eq!(pfp.enabled, config.protected_flows.enabled);
        assert_eq!(pfp.max_delay_slots, config.anti_mev.protected_flow_max_delay_slots);
        assert!(pfp.protected_classes.contains(&FairnessClass::SafetyCritical));
        assert!(pfp.protected_classes.contains(&FairnessClass::ProtectedUserFlow));
    }

    #[test]
    fn test_inclusion_config_defaults() {
        let ic = InclusionConfig::default_config();
        assert!(ic.batch_capacity > 0);
        assert!(ic.anti_starvation_slots > 0);
        assert!(ic.max_inclusion_age_slots > ic.anti_starvation_slots);
    }

    #[test]
    fn test_fairness_thresholds_defaults() {
        let ft = FairnessThresholds::default_thresholds();
        assert!(ft.max_omission_streak > 0);
        assert!(ft.max_class_skew_bps < 10000);
        assert!(ft.min_normal_class_bps > 0);
    }
}
