#![allow(dead_code)]

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

use crate::misbehavior::types::MisbehaviorSeverity;

// ---------------------------------------------------------------------------
// PenaltySeverityLevel
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum PenaltySeverityLevel {
    Warning,
    Minor,
    Moderate,
    Severe,
    Full,
}

// ---------------------------------------------------------------------------
// PenaltyRecommendation
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PenaltyRecommendation {
    pub node_id: [u8; 32],
    pub severity: PenaltySeverityLevel,
    pub reason: String,
    pub slash_bps: u32,
    pub suspend_epochs: u64,
    pub ban: bool,
    pub governance_escalation: bool,
    pub evidence_hashes: Vec<[u8; 32]>,
}

impl PenaltyRecommendation {
    pub fn from_misbehavior(
        node_id: [u8; 32],
        severity: &MisbehaviorSeverity,
        evidence_hashes: Vec<[u8; 32]>,
    ) -> Self {
        let (penalty_severity, slash_bps, suspend_epochs, ban, governance_escalation) =
            match severity {
                MisbehaviorSeverity::Minor => {
                    (PenaltySeverityLevel::Warning, 0u32, 0u64, false, false)
                }
                MisbehaviorSeverity::Moderate => {
                    (PenaltySeverityLevel::Minor, 300u32, 2u64, false, false)
                }
                MisbehaviorSeverity::Severe => {
                    (PenaltySeverityLevel::Severe, 3000u32, 5u64, false, true)
                }
                MisbehaviorSeverity::Critical => {
                    (PenaltySeverityLevel::Full, 10000u32, 0u64, true, true)
                }
            };

        PenaltyRecommendation {
            node_id,
            severity: penalty_severity,
            reason: format!("Auto-generated from {:?}", severity),
            slash_bps,
            suspend_epochs,
            ban,
            governance_escalation,
            evidence_hashes,
        }
    }
}

// ---------------------------------------------------------------------------
// SlashableEvidencePlaceholder
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SlashableEvidencePlaceholder {
    pub node_id: [u8; 32],
    pub misbehavior_type: String,
    pub evidence_hashes: Vec<[u8; 32]>,
    pub proposed_slash_bps: u32,
    pub packaged_at_epoch: u64,
    pub payload_hash: [u8; 32],
}

impl SlashableEvidencePlaceholder {
    pub fn new(
        node_id: [u8; 32],
        mtype: &str,
        mut evidence_hashes: Vec<[u8; 32]>,
        slash_bps: u32,
        epoch: u64,
    ) -> Self {
        // Sort evidence hashes for determinism
        evidence_hashes.sort();
        let payload_hash = Self::compute_payload_hash(&node_id, &evidence_hashes, slash_bps);
        SlashableEvidencePlaceholder {
            node_id,
            misbehavior_type: mtype.to_string(),
            evidence_hashes,
            proposed_slash_bps: slash_bps,
            packaged_at_epoch: epoch,
            payload_hash,
        }
    }

    pub fn compute_payload_hash(
        node_id: &[u8; 32],
        evidence_hashes: &[[u8; 32]],
        slash_bps: u32,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(node_id);
        // Sort for determinism
        let mut sorted = evidence_hashes.to_vec();
        sorted.sort();
        for eh in &sorted {
            hasher.update(eh);
        }
        hasher.update(slash_bps.to_le_bytes());
        hasher.finalize().into()
    }
}

// ---------------------------------------------------------------------------
// GovernanceEscalationFlag
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GovernanceEscalationFlag {
    pub node_id: [u8; 32],
    pub escalation_reason: String,
    pub evidence_placeholder: SlashableEvidencePlaceholder,
    pub requires_governance_vote: bool,
    pub epoch: u64,
}

// ---------------------------------------------------------------------------
// PenaltyHookError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum PenaltyHookError {
    NodeNotFound([u8; 32]),
    InvalidSlashBps(u32),
    AlreadyEscalated([u8; 32]),
}

// ---------------------------------------------------------------------------
// PenaltyActionHook
// ---------------------------------------------------------------------------

pub struct PenaltyActionHook {
    recommendations: BTreeMap<[u8; 32], Vec<PenaltyRecommendation>>,
    escalations: Vec<GovernanceEscalationFlag>,
}

impl PenaltyActionHook {
    pub fn new() -> Self {
        PenaltyActionHook {
            recommendations: BTreeMap::new(),
            escalations: Vec::new(),
        }
    }

    pub fn push_recommendation(&mut self, rec: PenaltyRecommendation) {
        self.recommendations
            .entry(rec.node_id)
            .or_default()
            .push(rec);
    }

    pub fn push_escalation(&mut self, flag: GovernanceEscalationFlag) {
        self.escalations.push(flag);
    }

    pub fn recommendations_for(&self, node_id: &[u8; 32]) -> &[PenaltyRecommendation] {
        self.recommendations
            .get(node_id)
            .map(|v| v.as_slice())
            .unwrap_or(&[])
    }

    pub fn all_escalations(&self) -> &[GovernanceEscalationFlag] {
        &self.escalations
    }

    pub fn highest_severity_for(&self, node_id: &[u8; 32]) -> Option<&PenaltySeverityLevel> {
        self.recommendations
            .get(node_id)?
            .iter()
            .map(|r| &r.severity)
            .max()
    }
}

impl Default for PenaltyActionHook {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn eh(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[31] = n;
        id
    }

    #[test]
    fn test_push_and_retrieve_recommendation() {
        let mut hook = PenaltyActionHook::new();
        let rec = PenaltyRecommendation::from_misbehavior(
            node(1),
            &MisbehaviorSeverity::Minor,
            vec![eh(1)],
        );
        hook.push_recommendation(rec);
        assert_eq!(hook.recommendations_for(&node(1)).len(), 1);
    }

    #[test]
    fn test_from_misbehavior_critical_bans() {
        let rec = PenaltyRecommendation::from_misbehavior(
            node(2),
            &MisbehaviorSeverity::Critical,
            vec![eh(2)],
        );
        assert!(rec.ban);
        assert!(rec.governance_escalation);
        assert_eq!(rec.slash_bps, 10000);
    }

    #[test]
    fn test_from_misbehavior_minor_no_slash() {
        let rec = PenaltyRecommendation::from_misbehavior(
            node(3),
            &MisbehaviorSeverity::Minor,
            vec![],
        );
        assert_eq!(rec.slash_bps, 0);
        assert!(!rec.ban);
        assert_eq!(rec.severity, PenaltySeverityLevel::Warning);
    }

    #[test]
    fn test_highest_severity() {
        let mut hook = PenaltyActionHook::new();
        hook.push_recommendation(PenaltyRecommendation::from_misbehavior(
            node(4),
            &MisbehaviorSeverity::Minor,
            vec![],
        ));
        hook.push_recommendation(PenaltyRecommendation::from_misbehavior(
            node(4),
            &MisbehaviorSeverity::Severe,
            vec![],
        ));
        assert_eq!(
            hook.highest_severity_for(&node(4)),
            Some(&PenaltySeverityLevel::Severe)
        );
    }

    #[test]
    fn test_slashable_evidence_placeholder_hash_deterministic() {
        let h1 =
            SlashableEvidencePlaceholder::compute_payload_hash(&node(1), &[eh(1), eh(2)], 500);
        let h2 =
            SlashableEvidencePlaceholder::compute_payload_hash(&node(1), &[eh(2), eh(1)], 500);
        assert_eq!(h1, h2); // sorted before hashing
    }

    #[test]
    fn test_slashable_evidence_placeholder_new() {
        let ph =
            SlashableEvidencePlaceholder::new(node(5), "Equivocation", vec![eh(1)], 1000, 3);
        assert_eq!(ph.proposed_slash_bps, 1000);
        assert_eq!(ph.packaged_at_epoch, 3);
        assert_ne!(ph.payload_hash, [0u8; 32]);
    }

    #[test]
    fn test_governance_escalation_push() {
        let mut hook = PenaltyActionHook::new();
        let ph = SlashableEvidencePlaceholder::new(node(6), "SlotHijacking", vec![eh(3)], 5000, 2);
        let flag = GovernanceEscalationFlag {
            node_id: node(6),
            escalation_reason: "critical offense".into(),
            evidence_placeholder: ph,
            requires_governance_vote: true,
            epoch: 2,
        };
        hook.push_escalation(flag);
        assert_eq!(hook.all_escalations().len(), 1);
    }

    #[test]
    fn test_no_recommendations_returns_empty() {
        let hook = PenaltyActionHook::new();
        assert_eq!(hook.recommendations_for(&node(99)).len(), 0);
        assert!(hook.highest_severity_for(&node(99)).is_none());
    }

    #[test]
    fn test_moderate_slash_bps() {
        let rec = PenaltyRecommendation::from_misbehavior(
            node(7),
            &MisbehaviorSeverity::Moderate,
            vec![],
        );
        assert!(rec.slash_bps > 0 && rec.slash_bps < 10000);
        assert!(!rec.ban);
    }

    #[test]
    fn test_severe_triggers_governance_escalation() {
        let rec = PenaltyRecommendation::from_misbehavior(
            node(8),
            &MisbehaviorSeverity::Severe,
            vec![eh(5)],
        );
        assert!(rec.governance_escalation);
        assert!(!rec.ban);
    }
}
