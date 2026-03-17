#![allow(dead_code)]

use sha2::{Digest, Sha256};

// ---------------------------------------------------------------------------
// MisbehaviorType
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum MisbehaviorType {
    Equivocation,
    InvalidProposalAuthority,
    InvalidFairnessEnvelope,
    InvalidAttestation,
    DuplicateAttestationAbuse,
    PersistentOmission,
    InvalidBatchDeliveryAttempt,
    RuntimeBridgeAbuse,
    StaleCommitteeParticipation,
    SlotHijackingAttempt,
    BoundaryTransitionAbuse,
    RepeatedInvalidProposalSpam,
    FairnessViolation,
    ReplayAttack,
    AbsentFromDuty,
}

impl MisbehaviorType {
    pub fn type_tag(&self) -> u8 {
        match self {
            MisbehaviorType::Equivocation => 1,
            MisbehaviorType::InvalidProposalAuthority => 2,
            MisbehaviorType::InvalidFairnessEnvelope => 3,
            MisbehaviorType::InvalidAttestation => 4,
            MisbehaviorType::DuplicateAttestationAbuse => 5,
            MisbehaviorType::PersistentOmission => 6,
            MisbehaviorType::InvalidBatchDeliveryAttempt => 7,
            MisbehaviorType::RuntimeBridgeAbuse => 8,
            MisbehaviorType::StaleCommitteeParticipation => 9,
            MisbehaviorType::SlotHijackingAttempt => 10,
            MisbehaviorType::BoundaryTransitionAbuse => 11,
            MisbehaviorType::RepeatedInvalidProposalSpam => 12,
            MisbehaviorType::FairnessViolation => 13,
            MisbehaviorType::ReplayAttack => 14,
            MisbehaviorType::AbsentFromDuty => 15,
        }
    }
}

// ---------------------------------------------------------------------------
// MisbehaviorSeverity
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum MisbehaviorSeverity {
    Minor,
    Moderate,
    Severe,
    Critical,
}

impl MisbehaviorType {
    pub fn default_severity(&self) -> MisbehaviorSeverity {
        match self {
            MisbehaviorType::Equivocation => MisbehaviorSeverity::Critical,
            MisbehaviorType::SlotHijackingAttempt => MisbehaviorSeverity::Critical,
            MisbehaviorType::RuntimeBridgeAbuse => MisbehaviorSeverity::Critical,
            MisbehaviorType::ReplayAttack => MisbehaviorSeverity::Critical,
            MisbehaviorType::InvalidProposalAuthority => MisbehaviorSeverity::Severe,
            MisbehaviorType::InvalidBatchDeliveryAttempt => MisbehaviorSeverity::Severe,
            MisbehaviorType::BoundaryTransitionAbuse => MisbehaviorSeverity::Severe,
            MisbehaviorType::DuplicateAttestationAbuse => MisbehaviorSeverity::Moderate,
            MisbehaviorType::InvalidFairnessEnvelope => MisbehaviorSeverity::Moderate,
            MisbehaviorType::InvalidAttestation => MisbehaviorSeverity::Moderate,
            MisbehaviorType::StaleCommitteeParticipation => MisbehaviorSeverity::Moderate,
            MisbehaviorType::RepeatedInvalidProposalSpam => MisbehaviorSeverity::Moderate,
            MisbehaviorType::FairnessViolation => MisbehaviorSeverity::Minor,
            MisbehaviorType::PersistentOmission => MisbehaviorSeverity::Minor,
            MisbehaviorType::AbsentFromDuty => MisbehaviorSeverity::Minor,
        }
    }

    pub fn is_slashable(&self) -> bool {
        matches!(
            self,
            MisbehaviorType::Equivocation
                | MisbehaviorType::SlotHijackingAttempt
                | MisbehaviorType::RuntimeBridgeAbuse
                | MisbehaviorType::ReplayAttack
        )
    }
}

// ---------------------------------------------------------------------------
// MisbehaviorEvidence
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct MisbehaviorEvidence {
    pub evidence_type: MisbehaviorType,
    pub node_id: [u8; 32],
    pub epoch: u64,
    pub slot: Option<u64>,
    pub evidence_hash: [u8; 32],
    pub details: String,
}

impl MisbehaviorEvidence {
    pub fn compute_hash(
        node_id: &[u8; 32],
        mtype: &MisbehaviorType,
        epoch: u64,
        slot: Option<u64>,
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(node_id);
        hasher.update([mtype.type_tag()]);
        hasher.update(epoch.to_le_bytes());
        hasher.update(slot.unwrap_or(0).to_le_bytes());
        hasher.finalize().into()
    }

    pub fn new(
        node_id: [u8; 32],
        evidence_type: MisbehaviorType,
        epoch: u64,
        slot: Option<u64>,
        details: String,
    ) -> Self {
        let evidence_hash = Self::compute_hash(&node_id, &evidence_type, epoch, slot);
        MisbehaviorEvidence {
            evidence_type,
            node_id,
            epoch,
            slot,
            evidence_hash,
            details,
        }
    }
}

// ---------------------------------------------------------------------------
// MisbehaviorCase
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct MisbehaviorCase {
    pub case_id: [u8; 32],
    pub evidence: MisbehaviorEvidence,
    pub severity: MisbehaviorSeverity,
    pub resolved: bool,
}

impl MisbehaviorCase {
    pub fn new(evidence: MisbehaviorEvidence) -> Self {
        let severity = evidence.evidence_type.default_severity();
        let severity_tag = match &severity {
            MisbehaviorSeverity::Minor => 1u8,
            MisbehaviorSeverity::Moderate => 2,
            MisbehaviorSeverity::Severe => 3,
            MisbehaviorSeverity::Critical => 4,
        };
        let mut hasher = Sha256::new();
        hasher.update(evidence.evidence_hash);
        hasher.update([severity_tag]);
        let case_id = hasher.finalize().into();
        MisbehaviorCase {
            case_id,
            evidence,
            severity,
            resolved: false,
        }
    }
}
