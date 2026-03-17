//! Governance escalation records — structured references for chain governance proposals.
//!
//! When PoSeq detects Severe or Critical misbehavior, it produces a
//! `GovernanceEscalationRecord` that the chain's `x/poseq` keeper uses to:
//! 1. Create a governance proposal text referencing the evidence
//! 2. Recommend a specific `EscalationAction` (suspend, ban, committee freeze)
//! 3. Block that node from committee participation until governance resolves it

use sha2::{Sha256, Digest};
use crate::chain_bridge::evidence::EvidencePacket;

// ─── EscalationSeverity ───────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum EscalationSeverity {
    /// PoSeq detected severe misbehavior → governance review recommended.
    Severe,
    /// PoSeq detected critical misbehavior → immediate governance action required.
    Critical,
}

// ─── EscalationAction ─────────────────────────────────────────────────────────

/// The action PoSeq recommends governance to take.
/// The chain does NOT auto-execute these — they become governance proposal content.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum EscalationAction {
    /// Suspend node from committee for N epochs.
    SuspendFromCommittee { epochs: u64 },
    /// Permanently ban node from sequencing participation.
    PermanentBan,
    /// Freeze entire committee pending investigation (emergency governance).
    CommitteeFreeze { committee_epoch: u64 },
    /// Request governance review with no automatic action.
    GovernanceReview,
}

impl EscalationAction {
    pub fn action_tag(&self) -> &'static str {
        match self {
            EscalationAction::SuspendFromCommittee { .. } => "suspend_committee",
            EscalationAction::PermanentBan                => "permanent_ban",
            EscalationAction::CommitteeFreeze { .. }      => "committee_freeze",
            EscalationAction::GovernanceReview            => "governance_review",
        }
    }
}

// ─── GovernanceEscalationRecord ───────────────────────────────────────────────

/// Chain-facing governance escalation reference.
///
/// The `escalation_id` is the primary key for chain storage.
/// The chain's `x/poseq` keeper stores these and exposes them via query
/// so governance tooling can draft the corresponding proposal.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GovernanceEscalationRecord {
    /// `SHA256("esc" ‖ offender_node_id ‖ epoch_be ‖ evidence_packet_hash)`
    pub escalation_id: [u8; 32],

    /// Node that triggered the escalation.
    pub offender_node_id: [u8; 32],

    /// The evidence packet that triggered this escalation.
    pub evidence_packet_hash: [u8; 32],

    /// Epoch in which the misbehavior occurred.
    pub epoch: u64,

    /// Severity level (Severe or Critical only — lower severities don't escalate).
    pub severity: EscalationSeverity,

    /// PoSeq's recommended action for governance to consider.
    pub recommended_action: EscalationAction,

    /// Human-readable rationale for the governance proposal body.
    pub rationale: String,

    /// If `true`, PoSeq requests the chain to block this node from committee
    /// immediately pending governance resolution.
    pub block_pending_governance: bool,
}

impl GovernanceEscalationRecord {
    pub fn compute_escalation_id(
        offender: &[u8; 32],
        epoch: u64,
        evidence_hash: &[u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"esc");
        hasher.update(offender);
        hasher.update(&epoch.to_be_bytes());
        hasher.update(evidence_hash);
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Build an escalation record from a critical/severe evidence packet.
    pub fn from_evidence(
        packet: &EvidencePacket,
        action: EscalationAction,
        block_pending_governance: bool,
    ) -> Self {
        let severity = match packet.severity {
            crate::misbehavior::types::MisbehaviorSeverity::Critical => EscalationSeverity::Critical,
            crate::misbehavior::types::MisbehaviorSeverity::Severe   => EscalationSeverity::Severe,
            // Lower severities should not produce escalation records
            _ => EscalationSeverity::Severe,
        };

        let escalation_id = Self::compute_escalation_id(
            &packet.offender_node_id,
            packet.epoch,
            &packet.packet_hash,
        );

        let rationale = format!(
            "PoSeq misbehavior detected: {:?} at epoch={} slot={}. Evidence hashes: {}. Severity: {:?}. Recommended action: {}.",
            packet.kind,
            packet.epoch,
            packet.slot,
            packet.evidence_hashes.iter()
                .map(|h| hex::encode(&h[..4]))
                .collect::<Vec<_>>()
                .join(", "),
            severity,
            action.action_tag(),
        );

        GovernanceEscalationRecord {
            escalation_id,
            offender_node_id: packet.offender_node_id,
            evidence_packet_hash: packet.packet_hash,
            epoch: packet.epoch,
            severity,
            recommended_action: action,
            rationale,
            block_pending_governance,
        }
    }
}

// ─── CommitteeSuspensionRecommendation ───────────────────────────────────────

/// A recommendation to suspend a node from committee participation.
/// This is less severe than a governance escalation and can be applied by
/// the chain's `x/poseq` keeper automatically if the evidence is sufficient.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CommitteeSuspensionRecommendation {
    pub node_id: [u8; 32],
    pub suspend_from_epoch: u64,
    pub suspend_until_epoch: u64,
    pub evidence_packet_hash: [u8; 32],
    pub reason: String,
}

impl CommitteeSuspensionRecommendation {
    pub fn new(
        node_id: [u8; 32],
        current_epoch: u64,
        suspend_epochs: u64,
        packet_hash: [u8; 32],
        reason: String,
    ) -> Self {
        CommitteeSuspensionRecommendation {
            node_id,
            suspend_from_epoch: current_epoch,
            suspend_until_epoch: current_epoch + suspend_epochs,
            evidence_packet_hash: packet_hash,
            reason,
        }
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::misbehavior::types::MisbehaviorType;
    use crate::chain_bridge::evidence::EvidencePacket;

    fn nid(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_critical_packet() -> EvidencePacket {
        EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation,
            nid(1), 3, 7, vec![[0xABu8; 32]], None,
        )
    }

    #[test]
    fn test_escalation_id_deterministic() {
        let id1 = GovernanceEscalationRecord::compute_escalation_id(&nid(1), 5, &[0u8; 32]);
        let id2 = GovernanceEscalationRecord::compute_escalation_id(&nid(1), 5, &[0u8; 32]);
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_escalation_id_different_epoch() {
        let id1 = GovernanceEscalationRecord::compute_escalation_id(&nid(1), 5, &[0u8; 32]);
        let id2 = GovernanceEscalationRecord::compute_escalation_id(&nid(1), 6, &[0u8; 32]);
        assert_ne!(id1, id2);
    }

    #[test]
    fn test_from_evidence_critical() {
        let packet = make_critical_packet();
        let esc = GovernanceEscalationRecord::from_evidence(
            &packet,
            EscalationAction::SuspendFromCommittee { epochs: 5 },
            true,
        );
        assert_eq!(esc.severity, EscalationSeverity::Critical);
        assert_eq!(esc.offender_node_id, nid(1));
        assert_eq!(esc.evidence_packet_hash, packet.packet_hash);
        assert!(esc.block_pending_governance);
        assert!(!esc.rationale.is_empty());
    }

    #[test]
    fn test_escalation_id_bound_to_packet_hash() {
        let packet1 = make_critical_packet();
        let mut packet2 = make_critical_packet();
        packet2.epoch = 99; // different epoch → different packet_hash
        packet2.packet_hash = EvidencePacket::compute_packet_hash(
            &packet2.kind, &packet2.offender_node_id, packet2.epoch, &packet2.evidence_hashes,
        );

        let esc1 = GovernanceEscalationRecord::from_evidence(
            &packet1, EscalationAction::PermanentBan, true,
        );
        let esc2 = GovernanceEscalationRecord::from_evidence(
            &packet2, EscalationAction::PermanentBan, true,
        );
        assert_ne!(esc1.escalation_id, esc2.escalation_id);
    }

    #[test]
    fn test_committee_suspension_epoch_math() {
        let rec = CommitteeSuspensionRecommendation::new(
            nid(2), 10, 5, [0u8; 32], "test".into(),
        );
        assert_eq!(rec.suspend_from_epoch, 10);
        assert_eq!(rec.suspend_until_epoch, 15);
    }

    #[test]
    fn test_action_tags_unique() {
        use std::collections::BTreeSet;
        let tags: BTreeSet<&str> = [
            EscalationAction::SuspendFromCommittee { epochs: 1 }.action_tag(),
            EscalationAction::PermanentBan.action_tag(),
            EscalationAction::CommitteeFreeze { committee_epoch: 1 }.action_tag(),
            EscalationAction::GovernanceReview.action_tag(),
        ].iter().copied().collect();
        assert_eq!(tags.len(), 4);
    }
}
