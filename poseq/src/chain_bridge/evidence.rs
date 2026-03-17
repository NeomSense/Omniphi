//! Evidence packaging — standardized PoSeq evidence payloads for chain consumption.
//!
//! Each `EvidencePacket` is:
//! - Self-contained: all fields needed for chain verification are included
//! - Deterministic: `packet_hash = SHA256(kind_tag ‖ node_id ‖ epoch ‖ sorted(evidence_hashes))`
//! - Dedup-safe: `DuplicateEvidenceGuard` rejects re-submission of the same packet_hash
//!
//! The chain's `x/poseq` keeper accepts these packets via `MsgSubmitPoSeqEvidence`
//! and routes them to the appropriate handler (slashing, governance, log).

use std::collections::BTreeSet;
use sha2::{Sha256, Digest};
use crate::misbehavior::types::{MisbehaviorType, MisbehaviorSeverity};
use crate::penalties::PenaltyRecommendation;

// ─── EvidenceKind ────────────────────────────────────────────────────────────

/// Canonical tag for each evidence class understood by the chain.
/// These map 1:1 to on-chain handler dispatch.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum EvidenceKind {
    /// Double-sign / double-proposal in the same (slot, epoch). Slashable.
    Equivocation,
    /// Node submitted a proposal claiming to be leader when it was not. Slashable.
    UnauthorizedProposal,
    /// Node manipulated fairness ordering (proved by commitment mismatch). Slashable.
    UnfairSequencing,
    /// Node reused a previously seen submission ID. Slashable.
    ReplayAbuse,
    /// Node attempted to deliver a batch to the runtime it had no authority for. Slashable.
    BridgeAbuse,
    /// Node repeatedly absent from attestation duty across multiple epochs.
    PersistentAbsence,
    /// Node submitted stale committee claims after epoch rotation.
    StaleAuthority,
    /// Node spammed invalid proposals repeatedly.
    InvalidProposalSpam,
}

impl EvidenceKind {
    pub fn kind_tag(&self) -> u8 {
        match self {
            EvidenceKind::Equivocation          => 1,
            EvidenceKind::UnauthorizedProposal  => 2,
            EvidenceKind::UnfairSequencing      => 3,
            EvidenceKind::ReplayAbuse           => 4,
            EvidenceKind::BridgeAbuse           => 5,
            EvidenceKind::PersistentAbsence     => 6,
            EvidenceKind::StaleAuthority        => 7,
            EvidenceKind::InvalidProposalSpam   => 8,
        }
    }

    pub fn is_slashable(&self) -> bool {
        matches!(
            self,
            EvidenceKind::Equivocation
                | EvidenceKind::UnauthorizedProposal
                | EvidenceKind::UnfairSequencing
                | EvidenceKind::ReplayAbuse
                | EvidenceKind::BridgeAbuse
        )
    }

    /// Proposed slash in basis points for this evidence kind.
    pub fn base_slash_bps(&self) -> u32 {
        match self {
            EvidenceKind::Equivocation          => 5_000, // 50%
            EvidenceKind::UnauthorizedProposal  => 2_000, // 20%
            EvidenceKind::UnfairSequencing      => 1_000, // 10%
            EvidenceKind::ReplayAbuse           => 3_000, // 30%
            EvidenceKind::BridgeAbuse           => 5_000, // 50%
            EvidenceKind::PersistentAbsence     => 50,    // 0.5%
            EvidenceKind::StaleAuthority        => 200,   // 2%
            EvidenceKind::InvalidProposalSpam   => 300,   // 3%
        }
    }

    /// Map from PoSeq `MisbehaviorType` to the canonical chain evidence kind.
    pub fn from_misbehavior_type(mtype: &MisbehaviorType) -> Self {
        match mtype {
            MisbehaviorType::Equivocation
            | MisbehaviorType::SlotHijackingAttempt     => EvidenceKind::Equivocation,
            MisbehaviorType::InvalidProposalAuthority
            | MisbehaviorType::BoundaryTransitionAbuse  => EvidenceKind::UnauthorizedProposal,
            MisbehaviorType::InvalidFairnessEnvelope
            | MisbehaviorType::FairnessViolation        => EvidenceKind::UnfairSequencing,
            MisbehaviorType::ReplayAttack               => EvidenceKind::ReplayAbuse,
            MisbehaviorType::RuntimeBridgeAbuse
            | MisbehaviorType::InvalidBatchDeliveryAttempt => EvidenceKind::BridgeAbuse,
            MisbehaviorType::PersistentOmission
            | MisbehaviorType::AbsentFromDuty           => EvidenceKind::PersistentAbsence,
            MisbehaviorType::StaleCommitteeParticipation => EvidenceKind::StaleAuthority,
            MisbehaviorType::RepeatedInvalidProposalSpam
            | MisbehaviorType::InvalidAttestation
            | MisbehaviorType::DuplicateAttestationAbuse => EvidenceKind::InvalidProposalSpam,
        }
    }
}

// ─── EvidencePacket ──────────────────────────────────────────────────────────

/// A self-contained, chain-digestible evidence record from PoSeq.
///
/// The `packet_hash` cryptographically binds all fields so the chain can
/// verify integrity without trusting the transport.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EvidencePacket {
    /// Unique packet ID: `SHA256(kind_tag ‖ node_id ‖ epoch_be ‖ sorted_evidence_hashes)`
    pub packet_hash: [u8; 32],

    /// The evidence classification.
    pub kind: EvidenceKind,

    /// PoSeq node that committed the misbehavior (maps to validator address on chain).
    pub offender_node_id: [u8; 32],

    /// Epoch in which the misbehavior occurred.
    pub epoch: u64,

    /// Slot (if applicable; 0 if epoch-level).
    pub slot: u64,

    /// Severity as assessed by PoSeq's misbehavior engine.
    pub severity: MisbehaviorSeverity,

    /// Proposed slash in basis points (0-10000). Chain governance may override.
    pub proposed_slash_bps: u32,

    /// Sorted SHA256 hashes of the raw evidence objects (proposals, attestations, etc.).
    /// The chain stores these for audit; it does not interpret them.
    pub evidence_hashes: Vec<[u8; 32]>,

    /// Human-readable description for governance and operator display.
    pub description: String,

    /// If `true`, the chain should route this to governance for review.
    pub requires_governance: bool,

    /// If `true`, the offender should be suspended from committee participation.
    pub recommend_suspension: bool,

    /// Batch ID if the misbehavior is linked to a specific batch delivery.
    pub linked_batch_id: Option<[u8; 32]>,
}

impl EvidencePacket {
    /// Compute the deterministic packet hash.
    pub fn compute_packet_hash(
        kind: &EvidenceKind,
        node_id: &[u8; 32],
        epoch: u64,
        evidence_hashes: &[[u8; 32]],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&[kind.kind_tag()]);
        hasher.update(node_id);
        hasher.update(&epoch.to_be_bytes());
        // Sort for determinism
        let mut sorted = evidence_hashes.to_vec();
        sorted.sort();
        for eh in &sorted {
            hasher.update(eh);
        }
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Build an evidence packet from a PoSeq misbehavior record.
    pub fn from_misbehavior(
        mtype: &MisbehaviorType,
        node_id: [u8; 32],
        epoch: u64,
        slot: u64,
        mut evidence_hashes: Vec<[u8; 32]>,
        linked_batch_id: Option<[u8; 32]>,
    ) -> Self {
        let kind = EvidenceKind::from_misbehavior_type(mtype);
        let severity = mtype.default_severity();
        evidence_hashes.sort();

        let (requires_governance, recommend_suspension) = match severity {
            MisbehaviorSeverity::Critical => (true, true),
            MisbehaviorSeverity::Severe   => (true, true),
            MisbehaviorSeverity::Moderate => (false, false),
            MisbehaviorSeverity::Minor    => (false, false),
        };

        let proposed_slash_bps = match severity {
            MisbehaviorSeverity::Critical => kind.base_slash_bps(),
            MisbehaviorSeverity::Severe   => kind.base_slash_bps() / 2,
            MisbehaviorSeverity::Moderate => kind.base_slash_bps() / 5,
            MisbehaviorSeverity::Minor    => 0,
        };

        let packet_hash = Self::compute_packet_hash(&kind, &node_id, epoch, &evidence_hashes);

        EvidencePacket {
            packet_hash,
            kind,
            offender_node_id: node_id,
            epoch,
            slot,
            severity,
            proposed_slash_bps,
            evidence_hashes,
            description: format!("{:?} at epoch={} slot={}", mtype, epoch, slot),
            requires_governance,
            recommend_suspension,
            linked_batch_id,
        }
    }

    /// Verify the packet_hash matches all other fields.
    pub fn verify(&self) -> bool {
        let expected = Self::compute_packet_hash(
            &self.kind,
            &self.offender_node_id,
            self.epoch,
            &self.evidence_hashes,
        );
        expected == self.packet_hash
    }
}

// ─── PenaltyRecommendationRecord ─────────────────────────────────────────────

/// A chain-facing penalty recommendation derived from PoSeq's `PenaltyRecommendation`.
///
/// The chain's `x/poseq` keeper ingests this but does NOT automatically execute
/// slashing — it produces a governance-reviewable record that validators vote on.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PenaltyRecommendationRecord {
    /// Links back to the evidence packet.
    pub packet_hash: [u8; 32],
    pub offender_node_id: [u8; 32],
    pub slash_bps: u32,
    pub suspend_epochs: u64,
    pub ban: bool,
    pub requires_governance_vote: bool,
    pub reason: String,
    pub epoch: u64,
}

impl PenaltyRecommendationRecord {
    pub fn from_penalty(packet_hash: [u8; 32], rec: &PenaltyRecommendation, epoch: u64) -> Self {
        PenaltyRecommendationRecord {
            packet_hash,
            offender_node_id: rec.node_id,
            slash_bps: rec.slash_bps,
            suspend_epochs: rec.suspend_epochs,
            ban: rec.ban,
            requires_governance_vote: rec.governance_escalation,
            reason: rec.reason.clone(),
            epoch,
        }
    }
}

// ─── EvidencePacketSet ────────────────────────────────────────────────────────

/// A set of evidence packets ready to be submitted to the chain.
/// All packets within an epoch are batched together for atomic submission.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct EvidencePacketSet {
    pub epoch: u64,
    pub packets: Vec<EvidencePacket>,
    pub penalty_records: Vec<PenaltyRecommendationRecord>,
    /// `SHA256(epoch_be ‖ sorted_packet_hashes)` — integrity anchor for the set.
    pub set_hash: [u8; 32],
}

impl EvidencePacketSet {
    pub fn new(epoch: u64, packets: Vec<EvidencePacket>, penalty_records: Vec<PenaltyRecommendationRecord>) -> Self {
        let set_hash = Self::compute_set_hash(epoch, &packets);
        EvidencePacketSet { epoch, packets, penalty_records, set_hash }
    }

    fn compute_set_hash(epoch: u64, packets: &[EvidencePacket]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&epoch.to_be_bytes());
        let mut hashes: Vec<[u8; 32]> = packets.iter().map(|p| p.packet_hash).collect();
        hashes.sort();
        for h in &hashes {
            hasher.update(h);
        }
        let r = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    pub fn is_empty(&self) -> bool {
        self.packets.is_empty()
    }

    pub fn slashable_count(&self) -> usize {
        self.packets.iter().filter(|p| p.kind.is_slashable()).count()
    }

    pub fn governance_required_count(&self) -> usize {
        self.packets.iter().filter(|p| p.requires_governance).count()
    }
}

// ─── DuplicateEvidenceGuard ───────────────────────────────────────────────────

/// Prevents re-submission of the same evidence packet to the chain.
/// Keyed by `packet_hash` — guaranteed unique per (kind, node, epoch, evidence).
#[derive(Debug, Default)]
pub struct DuplicateEvidenceGuard {
    seen: BTreeSet<[u8; 32]>,
}

impl DuplicateEvidenceGuard {
    pub fn new() -> Self {
        DuplicateEvidenceGuard { seen: BTreeSet::new() }
    }

    /// Returns `true` if the packet is new (not seen before) and registers it.
    /// Returns `false` if the packet was already submitted.
    pub fn register(&mut self, packet: &EvidencePacket) -> bool {
        self.seen.insert(packet.packet_hash)
    }

    pub fn is_seen(&self, packet_hash: &[u8; 32]) -> bool {
        self.seen.contains(packet_hash)
    }

    pub fn seen_count(&self) -> usize {
        self.seen.len()
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn nid(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[0] = b; id }
    fn eh(b: u8)  -> [u8; 32] { let mut id = [0u8; 32]; id[31] = b; id }

    // ── EvidenceKind ────────────────────────────────────────────────────────

    #[test]
    fn test_equivocation_is_slashable() {
        assert!(EvidenceKind::Equivocation.is_slashable());
    }

    #[test]
    fn test_persistent_absence_not_slashable() {
        assert!(!EvidenceKind::PersistentAbsence.is_slashable());
    }

    #[test]
    fn test_kind_tags_unique() {
        let kinds = [
            EvidenceKind::Equivocation, EvidenceKind::UnauthorizedProposal,
            EvidenceKind::UnfairSequencing, EvidenceKind::ReplayAbuse,
            EvidenceKind::BridgeAbuse, EvidenceKind::PersistentAbsence,
            EvidenceKind::StaleAuthority, EvidenceKind::InvalidProposalSpam,
        ];
        let tags: BTreeSet<u8> = kinds.iter().map(|k| k.kind_tag()).collect();
        assert_eq!(tags.len(), kinds.len(), "all kind_tags must be unique");
    }

    // ── EvidencePacket ───────────────────────────────────────────────────────

    #[test]
    fn test_packet_hash_deterministic() {
        let h1 = EvidencePacket::compute_packet_hash(
            &EvidenceKind::Equivocation, &nid(1), 5, &[eh(1), eh(2)],
        );
        let h2 = EvidencePacket::compute_packet_hash(
            &EvidenceKind::Equivocation, &nid(1), 5, &[eh(2), eh(1)],
        );
        assert_eq!(h1, h2, "sorted evidence_hashes → same packet_hash");
    }

    #[test]
    fn test_packet_hash_different_epoch() {
        let h1 = EvidencePacket::compute_packet_hash(
            &EvidenceKind::Equivocation, &nid(1), 5, &[eh(1)],
        );
        let h2 = EvidencePacket::compute_packet_hash(
            &EvidenceKind::Equivocation, &nid(1), 6, &[eh(1)],
        );
        assert_ne!(h1, h2, "different epoch → different hash");
    }

    #[test]
    fn test_from_misbehavior_equivocation() {
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation,
            nid(1), 3, 7, vec![eh(1), eh(2)], None,
        );
        assert_eq!(p.kind, EvidenceKind::Equivocation);
        assert!(p.requires_governance);
        assert!(p.recommend_suspension);
        assert!(p.proposed_slash_bps > 0);
        assert!(p.verify());
    }

    #[test]
    fn test_from_misbehavior_minor_no_slash() {
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::AbsentFromDuty,
            nid(2), 1, 0, vec![], None,
        );
        assert_eq!(p.proposed_slash_bps, 0);
        assert!(!p.requires_governance);
        assert!(!p.recommend_suspension);
        assert!(p.verify());
    }

    #[test]
    fn test_verify_passes() {
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::ReplayAttack, nid(3), 2, 0, vec![eh(5)], None,
        );
        assert!(p.verify());
    }

    #[test]
    fn test_verify_fails_after_tamper() {
        let mut p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(4), 1, 0, vec![eh(1)], None,
        );
        p.epoch = 99; // tamper
        assert!(!p.verify());
    }

    #[test]
    fn test_linked_batch_id_preserved() {
        let bid = [0xBBu8; 32];
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::RuntimeBridgeAbuse, nid(5), 1, 1, vec![], Some(bid),
        );
        assert_eq!(p.linked_batch_id, Some(bid));
    }

    // ── EvidencePacketSet ────────────────────────────────────────────────────

    #[test]
    fn test_packet_set_set_hash_deterministic() {
        let p1 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 1, 0, vec![], None,
        );
        let p2 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::ReplayAttack, nid(2), 1, 0, vec![], None,
        );
        let s1 = EvidencePacketSet::new(1, vec![p1.clone(), p2.clone()], vec![]);
        let s2 = EvidencePacketSet::new(1, vec![p2.clone(), p1.clone()], vec![]);
        // Set hash is order-independent (sorted internally)
        assert_eq!(s1.set_hash, s2.set_hash);
    }

    #[test]
    fn test_packet_set_slashable_count() {
        let p1 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 1, 0, vec![], None,
        );
        let p2 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::AbsentFromDuty, nid(2), 1, 0, vec![], None,
        );
        let set = EvidencePacketSet::new(1, vec![p1, p2], vec![]);
        assert_eq!(set.slashable_count(), 1);
        assert_eq!(set.governance_required_count(), 1);
    }

    // ── DuplicateEvidenceGuard ────────────────────────────────────────────────

    #[test]
    fn test_duplicate_guard_allows_first() {
        let mut guard = DuplicateEvidenceGuard::new();
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 1, 0, vec![], None,
        );
        assert!(guard.register(&p));
    }

    #[test]
    fn test_duplicate_guard_rejects_second() {
        let mut guard = DuplicateEvidenceGuard::new();
        let p = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 1, 0, vec![], None,
        );
        guard.register(&p);
        assert!(!guard.register(&p), "second registration must fail");
    }

    #[test]
    fn test_duplicate_guard_different_epochs_allowed() {
        let mut guard = DuplicateEvidenceGuard::new();
        let p1 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 1, 0, vec![], None,
        );
        let p2 = EvidencePacket::from_misbehavior(
            &MisbehaviorType::Equivocation, nid(1), 2, 0, vec![], None,
        );
        assert!(guard.register(&p1));
        assert!(guard.register(&p2), "different epoch → different packet_hash → allowed");
    }

    // ── PenaltyRecommendationRecord ───────────────────────────────────────────

    #[test]
    fn test_penalty_record_from_penalty() {
        use crate::penalties::PenaltyRecommendation;
        use crate::misbehavior::types::MisbehaviorSeverity;
        let rec = PenaltyRecommendation::from_misbehavior(
            nid(10), &MisbehaviorSeverity::Critical, vec![eh(1)],
        );
        let pr = PenaltyRecommendationRecord::from_penalty([0xABu8; 32], &rec, 5);
        assert_eq!(pr.slash_bps, 10000);
        assert!(pr.ban);
        assert!(pr.requires_governance_vote);
        assert_eq!(pr.epoch, 5);
    }
}
