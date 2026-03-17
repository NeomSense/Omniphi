#![allow(dead_code)]

//! FIND-002: Misbehavior enforcement pipeline.
//!
//! Wires `NodeMisbehaviorHistory` → `SlashingStore` → `JailStore`, then exposes
//! `jailed_set()` for use as the `excluded` parameter in `CommitteeRotationEngine::rotate()`.
//!
//! Design principles:
//! - Only slashable `MisbehaviorType`s trigger slash processing.
//! - Slashable types are mapped to the closest `SlashableOffense` equivalent.
//! - If a node is already jailed, additional offenses are recorded in history but
//!   skip the jail step (the node is already excluded from the committee).
//! - Non-slashable types are recorded in history for audit but do not trigger slashing.

use std::collections::BTreeSet;

use crate::errors::Phase4Error;
use crate::misbehavior::history::NodeMisbehaviorHistory;
use crate::misbehavior::types::{MisbehaviorCase, MisbehaviorEvidence, MisbehaviorType};
use crate::slashing::offenses::SlashableOffense;
use crate::slashing::store::{ProcessResult, SlashingStore};

/// Result of processing one misbehavior report through the enforcement pipeline.
#[derive(Debug)]
pub struct EnforcementResult {
    /// The case recorded in misbehavior history.
    pub case: MisbehaviorCase,
    /// Set only for slashable offenses.
    pub slash_result: Option<ProcessResult>,
}

/// Maps a slashable `MisbehaviorType` to the nearest `SlashableOffense`.
///
/// Returns `None` for non-slashable types (they are recorded but not slashed).
fn to_slashable_offense(mtype: &MisbehaviorType) -> Option<SlashableOffense> {
    match mtype {
        MisbehaviorType::Equivocation | MisbehaviorType::SlotHijackingAttempt => {
            Some(SlashableOffense::DoubleProposal)
        }
        MisbehaviorType::ReplayAttack => Some(SlashableOffense::ReplayAttack),
        MisbehaviorType::RuntimeBridgeAbuse => Some(SlashableOffense::InvalidAttestation),
        MisbehaviorType::FairnessViolation => Some(SlashableOffense::FairnessViolation),
        MisbehaviorType::AbsentFromDuty => Some(SlashableOffense::AbsentFromDuty),
        MisbehaviorType::InvalidAttestation | MisbehaviorType::DuplicateAttestationAbuse => {
            Some(SlashableOffense::InvalidAttestation)
        }
        // Non-slashable: record in history, no slash
        _ => None,
    }
}

/// Unified enforcement coordinator for the misbehavior → slashing → jail pipeline.
pub struct MisbehaviorEnforcer {
    pub history: NodeMisbehaviorHistory,
    pub slashing: SlashingStore,
}

impl MisbehaviorEnforcer {
    pub fn new(slashing: SlashingStore) -> Self {
        MisbehaviorEnforcer {
            history: NodeMisbehaviorHistory::new(),
            slashing,
        }
    }

    /// Report a misbehavior event.
    ///
    /// 1. Records the case in `NodeMisbehaviorHistory`.
    /// 2. If the `MisbehaviorType` is slashable, forwards to `SlashingStore::process()`.
    /// 3. `SlashingStore::process()` jails the node automatically when the slash threshold
    ///    is exceeded.
    ///
    /// Returns `Err` only for internal processing failures (e.g., JailStore errors).
    pub fn report(
        &mut self,
        evidence: MisbehaviorEvidence,
    ) -> Result<EnforcementResult, Phase4Error> {
        let case = MisbehaviorCase::new(evidence.clone());
        self.history.record(case.clone());

        let slash_result = if evidence.evidence_type.is_slashable() {
            if let Some(offense) = to_slashable_offense(&evidence.evidence_type) {
                let evidence_bytes = evidence.evidence_hash.as_slice().to_vec();
                let result = self.slashing.process(
                    offense,
                    evidence.node_id,
                    evidence.epoch,
                    &evidence_bytes,
                )?;
                Some(result)
            } else {
                None
            }
        } else {
            None
        };

        Ok(EnforcementResult { case, slash_result })
    }

    /// Returns the set of currently jailed node IDs.
    /// Pass this directly as `excluded` to `CommitteeRotationEngine::rotate()`.
    pub fn jailed_set(&self) -> BTreeSet<[u8; 32]> {
        self.slashing.jailed_set()
    }

    /// Whether a node is currently jailed.
    pub fn is_jailed(&self, node_id: &[u8; 32]) -> bool {
        self.slashing.is_jailed(node_id)
    }

    /// Unjail a node after cooldown (resets cumulative slash).
    pub fn unjail(&mut self, node_id: [u8; 32], current_epoch: u64) -> Result<(), Phase4Error> {
        self.slashing.unjail(node_id, current_epoch)
    }

    /// Whether a node has unresolved slashable misbehavior in history.
    pub fn has_unresolved_slashable(&self, node_id: &[u8; 32]) -> bool {
        self.history.has_unresolved_slashable(node_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::slashing::offenses::SlashingConfig;
    use crate::slashing::store::SlashingStore;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_enforcer() -> MisbehaviorEnforcer {
        // double_proposal_bps=500, jail_threshold=800: needs 2 offenses (500+500=1000>=800)
        let config = SlashingConfig {
            double_proposal_bps: 500,
            absent_from_duty_bps: 100,
            fairness_violation_bps: 200,
            invalid_attestation_bps: 300,
            replay_attack_bps: 400,
            jail_threshold_bps: 800,
            unjail_cooldown_epochs: 2,
        };
        MisbehaviorEnforcer::new(SlashingStore::new(config))
    }

    fn make_evidence(node_id: [u8; 32], mtype: MisbehaviorType, epoch: u64) -> MisbehaviorEvidence {
        MisbehaviorEvidence::new(node_id, mtype, epoch, None, "test".to_string())
    }

    #[test]
    fn test_slashable_offense_recorded_and_slashed() {
        let mut enforcer = make_enforcer();
        let node = make_id(1);
        let ev = make_evidence(node, MisbehaviorType::Equivocation, 1);
        let result = enforcer.report(ev).unwrap();
        assert!(result.slash_result.is_some(), "Equivocation must trigger a slash");
        assert_eq!(enforcer.history.cases_for(&node).len(), 1);
    }

    #[test]
    fn test_non_slashable_offense_recorded_not_slashed() {
        let mut enforcer = make_enforcer();
        let node = make_id(2);
        let ev = make_evidence(node, MisbehaviorType::StaleCommitteeParticipation, 1);
        let result = enforcer.report(ev).unwrap();
        assert!(result.slash_result.is_none(), "StaleCommitteeParticipation is not slashable");
        assert_eq!(enforcer.history.cases_for(&node).len(), 1);
    }

    #[test]
    fn test_repeated_slashable_offense_triggers_jail() {
        let mut enforcer = make_enforcer();
        let node = make_id(3);
        // DoubleProposal = 500 bps per offense. Threshold = 800.
        // Two offenses: 500 + 500 = 1000 bps >= 800 → jailed after second.
        let ev1 = make_evidence(node, MisbehaviorType::Equivocation, 1);
        let ev2 = make_evidence(node, MisbehaviorType::Equivocation, 2);
        enforcer.report(ev1).unwrap();
        let r2 = enforcer.report(ev2).unwrap();
        let slash = r2.slash_result.unwrap();
        assert!(slash.jailed, "node should be jailed after crossing threshold");
        assert!(enforcer.is_jailed(&node));
    }

    #[test]
    fn test_jailed_set_contains_jailed_node() {
        let mut enforcer = make_enforcer();
        let node = make_id(4);
        let ev1 = make_evidence(node, MisbehaviorType::Equivocation, 1);
        let ev2 = make_evidence(node, MisbehaviorType::Equivocation, 2);
        enforcer.report(ev1).unwrap();
        enforcer.report(ev2).unwrap();
        let jailed = enforcer.jailed_set();
        assert!(jailed.contains(&node), "jailed_set must include the jailed node");
    }

    #[test]
    fn test_unjail_after_cooldown() {
        let mut enforcer = make_enforcer();
        let node = make_id(5);
        let ev1 = make_evidence(node, MisbehaviorType::Equivocation, 1);
        let ev2 = make_evidence(node, MisbehaviorType::Equivocation, 2);
        enforcer.report(ev1).unwrap();
        enforcer.report(ev2).unwrap();
        assert!(enforcer.is_jailed(&node));
        // cooldown = 2 epochs; jailed at epoch 2 → eligible at epoch 4
        assert!(enforcer.unjail(node, 4).is_ok());
        assert!(!enforcer.is_jailed(&node));
    }

    #[test]
    fn test_jailed_set_used_as_committee_exclusion() {
        use std::collections::BTreeSet;
        use crate::committee_rotation::engine::{CommitteeRotationConfig, CommitteeRotationEngine};

        let mut enforcer = make_enforcer();
        let jailed_node = make_id(10);
        // Trigger jail
        for epoch in 1..=2u64 {
            let ev = make_evidence(jailed_node, MisbehaviorType::Equivocation, epoch);
            enforcer.report(ev).unwrap();
        }
        assert!(enforcer.is_jailed(&jailed_node));

        let excluded = enforcer.jailed_set();
        let cfg = CommitteeRotationConfig::new(1, 2, 10, [0u8; 32]).unwrap();
        let mut rotation_engine = CommitteeRotationEngine::new(cfg);
        let mut candidates: BTreeSet<[u8; 32]> = (1u8..=6).map(make_id).collect();
        candidates.insert(jailed_node);

        let (snapshot, _) = rotation_engine
            .rotate(1, &candidates, None, &excluded, &[0u8; 32])
            .unwrap();

        assert!(
            !snapshot.contains(&jailed_node),
            "jailed node must be excluded from committee rotation"
        );
    }
}
