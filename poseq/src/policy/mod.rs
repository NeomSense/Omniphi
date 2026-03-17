#![allow(dead_code)]

pub mod network;

// ---------------------------------------------------------------------------
// Phase 5 Policy Extensions
// ---------------------------------------------------------------------------

use crate::bridge_recovery::BridgeRetryPolicy;
use crate::checkpoints::CheckpointPolicy;
use crate::epochs::rotation::CommitteeRotationConfig;

// ---------------------------------------------------------------------------
// Phase5Error
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum Phase5Error {
    InvalidConfig(String),
    ZeroEpochLength,
    InvalidQuorumBps(u32),
    InvalidSlashBps(u32),
    CommitteeSizeMismatch { min: usize, max: usize },
}

// ---------------------------------------------------------------------------
// EpochConfig
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EpochConfig {
    pub epoch_length_slots: u64,
    pub finality_quorum_bps: u32,
    pub max_unfinalized_per_epoch: u32,
    pub carry_forward_enabled: bool,
}

impl EpochConfig {
    pub fn default_config() -> Self {
        EpochConfig {
            epoch_length_slots: 100,
            finality_quorum_bps: 6667, // 2/3
            max_unfinalized_per_epoch: 100,
            carry_forward_enabled: false,
        }
    }

    pub fn validate(&self) -> Result<(), Phase5Error> {
        if self.epoch_length_slots == 0 {
            return Err(Phase5Error::ZeroEpochLength);
        }
        if self.finality_quorum_bps > 10000 {
            return Err(Phase5Error::InvalidQuorumBps(self.finality_quorum_bps));
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// PenaltyPolicyConfig
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PenaltyPolicyConfig {
    pub equivocation_slash_bps: u32,
    pub replay_slash_bps: u32,
    pub omission_slash_bps: u32,
    pub suspension_duration_epochs: u64,
    pub auto_ban_after_critical_count: u32,
}

impl PenaltyPolicyConfig {
    pub fn default_config() -> Self {
        PenaltyPolicyConfig {
            equivocation_slash_bps: 10000, // 100%
            replay_slash_bps: 5000,        // 50%
            omission_slash_bps: 100,       // 1%
            suspension_duration_epochs: 5,
            auto_ban_after_critical_count: 2,
        }
    }
}

// ---------------------------------------------------------------------------
// RecoveryPolicyConfig
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RecoveryPolicyConfig {
    pub max_replay_epochs: u64,
    pub checkpoint_verification_required: bool,
    pub peer_sync_enabled: bool,
    pub max_inconsistencies_tolerated: usize,
}

impl RecoveryPolicyConfig {
    pub fn default_config() -> Self {
        RecoveryPolicyConfig {
            max_replay_epochs: 10,
            checkpoint_verification_required: true,
            peer_sync_enabled: false,
            max_inconsistencies_tolerated: 0,
        }
    }
}

// ---------------------------------------------------------------------------
// DevnetConfig (policy-level, separate from devnet module)
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct DevnetConfig {
    pub num_nodes: usize,
    pub simulate_partitions: bool,
    pub inject_misbehavior: bool,
}

impl DevnetConfig {
    pub fn default_config() -> Self {
        DevnetConfig {
            num_nodes: 5,
            simulate_partitions: false,
            inject_misbehavior: false,
        }
    }
}

// ---------------------------------------------------------------------------
// FullPoSeqConfig
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FullPoSeqConfig {
    pub epoch: EpochConfig,
    pub rotation: CommitteeRotationConfig,
    pub penalty: PenaltyPolicyConfig,
    pub recovery: RecoveryPolicyConfig,
    pub checkpoint: CheckpointPolicy,
    pub bridge_retry: BridgeRetryPolicy,
    pub devnet: Option<DevnetConfig>,
}

impl FullPoSeqConfig {
    pub fn default_config() -> Self {
        FullPoSeqConfig {
            epoch: EpochConfig::default_config(),
            rotation: CommitteeRotationConfig::default_config(),
            penalty: PenaltyPolicyConfig::default_config(),
            recovery: RecoveryPolicyConfig::default_config(),
            checkpoint: CheckpointPolicy::default_policy(),
            bridge_retry: BridgeRetryPolicy::default_policy(),
            devnet: None,
        }
    }

    pub fn validate(&self) -> Result<(), Phase5Error> {
        self.epoch.validate()?;

        // Check committee size mismatch explicitly before calling rotation.validate()
        if self.rotation.min_committee_size > self.rotation.max_committee_size {
            return Err(Phase5Error::CommitteeSizeMismatch {
                min: self.rotation.min_committee_size,
                max: self.rotation.max_committee_size,
            });
        }

        self.rotation
            .validate()
            .map_err(|e| Phase5Error::InvalidConfig(format!("{:?}", e)))?;

        if self.penalty.equivocation_slash_bps > 10000 {
            return Err(Phase5Error::InvalidSlashBps(
                self.penalty.equivocation_slash_bps,
            ));
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config_validates() {
        let config = FullPoSeqConfig::default_config();
        assert!(config.validate().is_ok());
    }

    #[test]
    fn test_zero_epoch_length_fails() {
        let mut config = FullPoSeqConfig::default_config();
        config.epoch.epoch_length_slots = 0;
        assert!(matches!(config.validate(), Err(Phase5Error::ZeroEpochLength)));
    }

    #[test]
    fn test_invalid_quorum_bps_fails() {
        let mut config = FullPoSeqConfig::default_config();
        config.epoch.finality_quorum_bps = 10001;
        assert!(matches!(
            config.validate(),
            Err(Phase5Error::InvalidQuorumBps(_))
        ));
    }

    #[test]
    fn test_invalid_slash_bps_fails() {
        let mut config = FullPoSeqConfig::default_config();
        config.penalty.equivocation_slash_bps = 20000;
        assert!(matches!(
            config.validate(),
            Err(Phase5Error::InvalidSlashBps(_))
        ));
    }

    #[test]
    fn test_committee_size_mismatch_fails() {
        let mut config = FullPoSeqConfig::default_config();
        config.rotation.min_committee_size = 10;
        config.rotation.max_committee_size = 5;
        assert!(matches!(
            config.validate(),
            Err(Phase5Error::CommitteeSizeMismatch { .. })
        ));
    }

    #[test]
    fn test_epoch_config_defaults() {
        let cfg = EpochConfig::default_config();
        assert!(cfg.epoch_length_slots > 0);
        assert!(cfg.finality_quorum_bps <= 10000);
    }

    #[test]
    fn test_penalty_policy_defaults() {
        let cfg = PenaltyPolicyConfig::default_config();
        assert!(cfg.equivocation_slash_bps <= 10000);
        assert!(cfg.auto_ban_after_critical_count > 0);
    }

    #[test]
    fn test_recovery_policy_defaults() {
        let cfg = RecoveryPolicyConfig::default_config();
        assert!(cfg.max_replay_epochs > 0);
        assert!(cfg.checkpoint_verification_required);
    }
}
