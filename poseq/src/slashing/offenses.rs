use sha2::{Sha256, Digest};
use crate::errors::Phase4Error;

/// Categories of slashable misbehavior.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum SlashableOffense {
    /// Node proposed two different batches for the same slot.
    DoubleProposal,
    /// Node was absent from duty (missed attestation window).
    AbsentFromDuty,
    /// Node violated fair ordering rules (frontrunning etc.).
    FairnessViolation,
    /// Node submitted an attestation with an invalid signature or batch reference.
    InvalidAttestation,
    /// Node re-broadcast a previously seen and rejected message.
    ReplayAttack,
}

impl SlashableOffense {
    /// Human-readable label.
    pub fn label(&self) -> &'static str {
        match self {
            SlashableOffense::DoubleProposal => "double_proposal",
            SlashableOffense::AbsentFromDuty => "absent_from_duty",
            SlashableOffense::FairnessViolation => "fairness_violation",
            SlashableOffense::InvalidAttestation => "invalid_attestation",
            SlashableOffense::ReplayAttack => "replay_attack",
        }
    }
}

/// A record of a slash event.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SlashRecord {
    pub offense: SlashableOffense,
    pub node_id: [u8; 32],
    pub epoch: u64,
    /// Hash of the evidence (e.g., duplicate proposal hashes, missed slot bytes).
    pub evidence_hash: [u8; 32],
    /// Slash amount in basis points of the node's stake.
    pub slash_amount_bps: u64,
}

impl SlashRecord {
    pub fn new(
        offense: SlashableOffense,
        node_id: [u8; 32],
        epoch: u64,
        evidence: &[u8],
        slash_amount_bps: u64,
    ) -> Self {
        let evidence_hash = Self::hash_evidence(evidence);
        SlashRecord { offense, node_id, epoch, evidence_hash, slash_amount_bps }
    }

    pub fn hash_evidence(evidence: &[u8]) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(evidence);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }
}

/// Per-offense slash rates and jail thresholds.
#[derive(Debug, Clone)]
pub struct SlashingConfig {
    /// Slash rate for DoubleProposal (bps).
    pub double_proposal_bps: u64,
    /// Slash rate for AbsentFromDuty (bps).
    pub absent_from_duty_bps: u64,
    /// Slash rate for FairnessViolation (bps).
    pub fairness_violation_bps: u64,
    /// Slash rate for InvalidAttestation (bps).
    pub invalid_attestation_bps: u64,
    /// Slash rate for ReplayAttack (bps).
    pub replay_attack_bps: u64,
    /// Cumulative slash (bps) that triggers jail.
    pub jail_threshold_bps: u64,
    /// Epochs a jailed node must wait before unjailing.
    pub unjail_cooldown_epochs: u64,
}

impl SlashingConfig {
    pub fn default_config() -> Self {
        SlashingConfig {
            double_proposal_bps: 500,     // 5%
            absent_from_duty_bps: 50,     // 0.5%
            fairness_violation_bps: 200,  // 2%
            invalid_attestation_bps: 100, // 1%
            replay_attack_bps: 50,        // 0.5%
            jail_threshold_bps: 1000,    // 10% cumulative → jail
            unjail_cooldown_epochs: 10,
        }
    }

    /// Get the slash rate (bps) for a given offense.
    pub fn slash_rate(&self, offense: &SlashableOffense) -> u64 {
        match offense {
            SlashableOffense::DoubleProposal => self.double_proposal_bps,
            SlashableOffense::AbsentFromDuty => self.absent_from_duty_bps,
            SlashableOffense::FairnessViolation => self.fairness_violation_bps,
            SlashableOffense::InvalidAttestation => self.invalid_attestation_bps,
            SlashableOffense::ReplayAttack => self.replay_attack_bps,
        }
    }
}

/// Processes offenses, accumulates slash records, applies jail.
pub struct SlashingEngine {
    pub config: SlashingConfig,
    /// node_id → cumulative slash bps
    cumulative_slash: std::collections::BTreeMap<[u8; 32], u64>,
}

impl SlashingEngine {
    pub fn new(config: SlashingConfig) -> Self {
        SlashingEngine {
            config,
            cumulative_slash: std::collections::BTreeMap::new(),
        }
    }

    /// Process an offense. Returns the resulting SlashRecord.
    /// Also updates cumulative slash for the node.
    pub fn process_offense(
        &mut self,
        offense: SlashableOffense,
        node_id: [u8; 32],
        epoch: u64,
        evidence: &[u8],
    ) -> Result<SlashRecord, Phase4Error> {
        let bps = self.config.slash_rate(&offense);
        let record = SlashRecord::new(offense, node_id, epoch, evidence, bps);
        let cumulative = self.cumulative_slash.entry(node_id).or_insert(0);
        *cumulative = cumulative.saturating_add(bps);
        Ok(record)
    }

    /// Returns true if the node's cumulative slash meets or exceeds jail threshold.
    pub fn should_jail(&self, node_id: &[u8; 32]) -> bool {
        let cumulative = self.cumulative_slash.get(node_id).cloned().unwrap_or(0);
        cumulative >= self.config.jail_threshold_bps
    }

    /// Get cumulative slash bps for a node.
    pub fn cumulative_slash_bps(&self, node_id: &[u8; 32]) -> u64 {
        self.cumulative_slash.get(node_id).cloned().unwrap_or(0)
    }

    /// Reset cumulative slash for a node (after unjail).
    pub fn reset_slash(&mut self, node_id: &[u8; 32]) {
        self.cumulative_slash.remove(node_id);
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

    #[test]
    fn test_slash_record_evidence_hash_determinism() {
        let r1 = SlashRecord::new(SlashableOffense::DoubleProposal, make_id(1), 5, b"evidence", 500);
        let r2 = SlashRecord::new(SlashableOffense::DoubleProposal, make_id(1), 5, b"evidence", 500);
        assert_eq!(r1.evidence_hash, r2.evidence_hash);
    }

    #[test]
    fn test_slash_record_evidence_hash_differs() {
        let r1 = SlashRecord::new(SlashableOffense::DoubleProposal, make_id(1), 5, b"evidence1", 500);
        let r2 = SlashRecord::new(SlashableOffense::DoubleProposal, make_id(1), 5, b"evidence2", 500);
        assert_ne!(r1.evidence_hash, r2.evidence_hash);
    }

    #[test]
    fn test_slashing_config_rates() {
        let cfg = SlashingConfig::default_config();
        assert_eq!(cfg.slash_rate(&SlashableOffense::DoubleProposal), 500);
        assert_eq!(cfg.slash_rate(&SlashableOffense::AbsentFromDuty), 50);
        assert_eq!(cfg.slash_rate(&SlashableOffense::FairnessViolation), 200);
        assert_eq!(cfg.slash_rate(&SlashableOffense::InvalidAttestation), 100);
        assert_eq!(cfg.slash_rate(&SlashableOffense::ReplayAttack), 50);
    }

    #[test]
    fn test_process_offense_accumulates() {
        let mut engine = SlashingEngine::new(SlashingConfig::default_config());
        let node = make_id(1);
        engine.process_offense(SlashableOffense::AbsentFromDuty, node, 1, b"slot1").unwrap();
        engine.process_offense(SlashableOffense::AbsentFromDuty, node, 2, b"slot2").unwrap();
        assert_eq!(engine.cumulative_slash_bps(&node), 100);
    }

    #[test]
    fn test_should_jail_when_threshold_exceeded() {
        let mut engine = SlashingEngine::new(SlashingConfig::default_config());
        let node = make_id(1);
        // jail_threshold_bps = 1000, DoubleProposal = 500 bps
        engine.process_offense(SlashableOffense::DoubleProposal, node, 1, b"e1").unwrap();
        assert!(!engine.should_jail(&node)); // 500 < 1000
        engine.process_offense(SlashableOffense::DoubleProposal, node, 2, b"e2").unwrap();
        assert!(engine.should_jail(&node)); // 1000 >= 1000
    }

    #[test]
    fn test_reset_slash_clears_accumulation() {
        let mut engine = SlashingEngine::new(SlashingConfig::default_config());
        let node = make_id(1);
        engine.process_offense(SlashableOffense::DoubleProposal, node, 1, b"e").unwrap();
        engine.reset_slash(&node);
        assert_eq!(engine.cumulative_slash_bps(&node), 0);
        assert!(!engine.should_jail(&node));
    }

    #[test]
    fn test_different_nodes_tracked_independently() {
        let mut engine = SlashingEngine::new(SlashingConfig::default_config());
        let n1 = make_id(1);
        let n2 = make_id(2);
        engine.process_offense(SlashableOffense::DoubleProposal, n1, 1, b"e").unwrap();
        assert_eq!(engine.cumulative_slash_bps(&n1), 500);
        assert_eq!(engine.cumulative_slash_bps(&n2), 0);
    }

    #[test]
    fn test_offense_labels() {
        assert_eq!(SlashableOffense::DoubleProposal.label(), "double_proposal");
        assert_eq!(SlashableOffense::AbsentFromDuty.label(), "absent_from_duty");
        assert_eq!(SlashableOffense::FairnessViolation.label(), "fairness_violation");
        assert_eq!(SlashableOffense::InvalidAttestation.label(), "invalid_attestation");
        assert_eq!(SlashableOffense::ReplayAttack.label(), "replay_attack");
    }

    #[test]
    fn test_process_offense_returns_correct_bps() {
        let mut engine = SlashingEngine::new(SlashingConfig::default_config());
        let record = engine.process_offense(
            SlashableOffense::FairnessViolation, make_id(5), 3, b"fv_evidence"
        ).unwrap();
        assert_eq!(record.slash_amount_bps, 200);
        assert_eq!(record.offense, SlashableOffense::FairnessViolation);
    }

    #[test]
    fn test_cumulative_slash_saturates() {
        let cfg = SlashingConfig {
            double_proposal_bps: u64::MAX / 2,
            jail_threshold_bps: u64::MAX,
            ..SlashingConfig::default_config()
        };
        let mut engine = SlashingEngine::new(cfg);
        let node = make_id(1);
        // Should not overflow
        engine.process_offense(SlashableOffense::DoubleProposal, node, 1, b"e").unwrap();
        engine.process_offense(SlashableOffense::DoubleProposal, node, 2, b"e").unwrap();
        // saturating_add should handle overflow
        assert!(engine.cumulative_slash_bps(&node) > 0);
    }

    #[test]
    fn test_zero_cumulative_for_unknown_node() {
        let engine = SlashingEngine::new(SlashingConfig::default_config());
        assert_eq!(engine.cumulative_slash_bps(&make_id(99)), 0);
        assert!(!engine.should_jail(&make_id(99)));
    }
}
