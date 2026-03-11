use std::collections::{BTreeMap, BTreeSet};

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum DomainCriticality {
    Low,
    Standard,
    High,
    Critical,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RecoveryMode {
    AutomaticAfterEpochs(u64),
    GovernanceVote,
    MultiSigRequired,
    ManualReview,
}

#[derive(Debug, Clone)]
pub struct DomainRiskProfile {
    pub domain: String,
    pub criticality: DomainCriticality,
    pub pause_sensitivity: bool,         // can be paused by safety kernel
    pub quarantine_eligible: bool,
    pub governance_escalation_required: bool,
    pub acceptable_downgrade_modes: Vec<String>,
    pub max_mutations_per_epoch: u64,    // 0 = unlimited
    pub max_outflow_per_epoch: u128,     // 0 = unlimited
    pub oracle_dependent: bool,
    pub recovery_mode: RecoveryMode,
    pub tags: BTreeSet<String>,
}

impl DomainRiskProfile {
    pub fn for_domain(domain: &str) -> Self {
        match domain {
            "treasury" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Critical,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: true,
                acceptable_downgrade_modes: vec![],
                max_mutations_per_epoch: 10,
                max_outflow_per_epoch: 50_000_000,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::GovernanceVote,
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("treasury".to_string());
                    s.insert("critical".to_string());
                    s
                },
            },
            "governance" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Critical,
                pause_sensitivity: false, // can't auto-pause governance
                quarantine_eligible: false,
                governance_escalation_required: true,
                acceptable_downgrade_modes: vec![],
                max_mutations_per_epoch: 5,
                max_outflow_per_epoch: 0,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::GovernanceVote,
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("governance".to_string());
                    s.insert("critical".to_string());
                    s
                },
            },
            "dex_liquidity" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::High,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec!["partial".to_string()],
                max_mutations_per_epoch: 100,
                max_outflow_per_epoch: 10_000_000,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::AutomaticAfterEpochs(5),
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("dex".to_string());
                    s.insert("liquidity".to_string());
                    s
                },
            },
            "lending" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::High,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec!["partial".to_string()],
                max_mutations_per_epoch: 50,
                max_outflow_per_epoch: 5_000_000,
                oracle_dependent: true,
                recovery_mode: RecoveryMode::AutomaticAfterEpochs(10),
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("lending".to_string());
                    s
                },
            },
            "bridge" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Critical,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: true,
                acceptable_downgrade_modes: vec![],
                max_mutations_per_epoch: 20,
                max_outflow_per_epoch: 20_000_000,
                oracle_dependent: true,
                recovery_mode: RecoveryMode::MultiSigRequired,
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("bridge".to_string());
                    s.insert("critical".to_string());
                    s
                },
            },
            "identity" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Standard,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec!["partial".to_string()],
                max_mutations_per_epoch: 200,
                max_outflow_per_epoch: 0,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::AutomaticAfterEpochs(3),
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("identity".to_string());
                    s
                },
            },
            "rewards" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Standard,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec!["partial".to_string()],
                max_mutations_per_epoch: 500,
                max_outflow_per_epoch: 1_000_000,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::AutomaticAfterEpochs(1),
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("rewards".to_string());
                    s
                },
            },
            "oracle" => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::High,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec![],
                max_mutations_per_epoch: 30,
                max_outflow_per_epoch: 0,
                oracle_dependent: true,
                recovery_mode: RecoveryMode::ManualReview,
                tags: {
                    let mut s = BTreeSet::new();
                    s.insert("oracle".to_string());
                    s
                },
            },
            _ => DomainRiskProfile {
                domain: domain.to_string(),
                criticality: DomainCriticality::Standard,
                pause_sensitivity: true,
                quarantine_eligible: true,
                governance_escalation_required: false,
                acceptable_downgrade_modes: vec!["partial".to_string()],
                max_mutations_per_epoch: 0,
                max_outflow_per_epoch: 0,
                oracle_dependent: false,
                recovery_mode: RecoveryMode::AutomaticAfterEpochs(5),
                tags: BTreeSet::new(),
            },
        }
    }
}

#[derive(Debug, Clone)]
pub struct DomainSafetyPolicy {
    pub profiles: BTreeMap<String, DomainRiskProfile>,
}

impl DomainSafetyPolicy {
    pub fn new() -> Self {
        DomainSafetyPolicy {
            profiles: BTreeMap::new(),
        }
    }

    pub fn with_defaults() -> Self {
        let mut policy = Self::new();
        for domain in &[
            "treasury",
            "governance",
            "dex_liquidity",
            "lending",
            "bridge",
            "identity",
            "rewards",
            "oracle",
        ] {
            let profile = DomainRiskProfile::for_domain(domain);
            policy.profiles.insert(domain.to_string(), profile);
        }
        policy
    }

    pub fn get(&self, domain: &str) -> Option<&DomainRiskProfile> {
        self.profiles.get(domain)
    }

    pub fn is_pause_allowed(&self, domain: &str) -> bool {
        self.profiles
            .get(domain)
            .map(|p| p.pause_sensitivity)
            .unwrap_or(true) // default: allow pause for unknown domains
    }

    pub fn requires_governance(&self, domain: &str) -> bool {
        self.profiles
            .get(domain)
            .map(|p| p.governance_escalation_required)
            .unwrap_or(false)
    }

    pub fn max_outflow(&self, domain: &str) -> u128 {
        self.profiles
            .get(domain)
            .map(|p| p.max_outflow_per_epoch)
            .unwrap_or(0)
    }
}
