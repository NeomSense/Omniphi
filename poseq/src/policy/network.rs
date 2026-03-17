use crate::attestations::collector::AttestationThreshold;
use crate::leader_selection::selector::LeaderSelectionPolicy;

/// Finalization-specific policy parameters.
#[derive(Debug, Clone)]
pub struct FinalizationPolicy {
    pub min_attestors: usize,
    pub quorum_bps: u32,
    pub max_proposal_age_slots: u64,
}

impl FinalizationPolicy {
    pub fn default_two_thirds() -> Self {
        FinalizationPolicy {
            min_attestors: 1,
            quorum_bps: 6667,
            max_proposal_age_slots: 100,
        }
    }

    pub fn to_threshold(&self, committee_size: usize) -> AttestationThreshold {
        let min_approvals = ((committee_size as u64 * self.quorum_bps as u64 + 9999) / 10000) as usize;
        AttestationThreshold {
            min_approvals,
            min_fraction_bps: self.quorum_bps,
        }
    }
}

/// Bridge delivery policy.
#[derive(Debug, Clone)]
pub struct BridgePolicy {
    pub max_delivery_attempts: u32,
    pub retry_on_rejection: bool,
    pub idempotent_delivery: bool,
}

impl Default for BridgePolicy {
    fn default() -> Self {
        BridgePolicy {
            max_delivery_attempts: 3,
            retry_on_rejection: false,
            idempotent_delivery: true,
        }
    }
}

/// Epoch configuration.
#[derive(Debug, Clone)]
pub struct EpochConfig {
    pub slots_per_epoch: u64,
    pub epoch_transition_overlap_slots: u64,
    pub committee_rotation_enabled: bool,
}

impl Default for EpochConfig {
    fn default() -> Self {
        EpochConfig {
            slots_per_epoch: 100,
            epoch_transition_overlap_slots: 5,
            committee_rotation_enabled: true,
        }
    }
}

/// Complete Phase 2 network and consensus policy.
#[derive(Debug, Clone)]
pub struct PoSeqNetworkPolicy {
    pub attestation_threshold: AttestationThreshold,
    pub leader_selection_policy: LeaderSelectionPolicy,
    pub finalization_policy: FinalizationPolicy,
    pub bridge_policy: BridgePolicy,
    pub epoch_config: EpochConfig,
}

impl PoSeqNetworkPolicy {
    pub fn default_with_committee_size(committee_size: usize) -> Self {
        let finalization_policy = FinalizationPolicy::default_two_thirds();
        let attestation_threshold = finalization_policy.to_threshold(committee_size);
        PoSeqNetworkPolicy {
            attestation_threshold,
            leader_selection_policy: LeaderSelectionPolicy::RoundRobin,
            finalization_policy,
            bridge_policy: BridgePolicy::default(),
            epoch_config: EpochConfig::default(),
        }
    }
}
