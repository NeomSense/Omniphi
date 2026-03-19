use crate::misbehavior::types::{MisbehaviorSeverity, MisbehaviorType};

/// Coarse classification of accountability events for enforcement routing.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EventClass {
    /// Metrics and minor observations — no enforcement action.
    Informational,
    /// Repeated inactivity or missed duties — triggers suspension.
    Penalizable,
    /// Equivocation, double-proposal, invalid sequencing, censorship —
    /// triggers jailing and slash queue entry.
    Slashable,
}

/// Classify a `MisbehaviorType` into its enforcement class.
///
/// This mapping is deterministic and stateless — the same type always
/// produces the same class.
pub fn classify_misbehavior(mtype: &MisbehaviorType) -> EventClass {
    match mtype {
        // Critical — always Slashable
        MisbehaviorType::Equivocation
        | MisbehaviorType::SlotHijackingAttempt
        | MisbehaviorType::ReplayAttack
        | MisbehaviorType::RuntimeBridgeAbuse => EventClass::Slashable,

        // Severe — Slashable
        MisbehaviorType::InvalidProposalAuthority
        | MisbehaviorType::BoundaryTransitionAbuse
        | MisbehaviorType::StaleCommitteeParticipation => EventClass::Slashable,

        // Moderate — Penalizable (triggers suspension)
        MisbehaviorType::InvalidFairnessEnvelope
        | MisbehaviorType::InvalidAttestation
        | MisbehaviorType::DuplicateAttestationAbuse
        | MisbehaviorType::InvalidBatchDeliveryAttempt
        | MisbehaviorType::RepeatedInvalidProposalSpam
        | MisbehaviorType::FairnessViolation => EventClass::Penalizable,

        // Minor / informational
        MisbehaviorType::PersistentOmission | MisbehaviorType::AbsentFromDuty => {
            EventClass::Informational
        }
    }
}

/// Classify a `MisbehaviorSeverity` into its enforcement class.
pub fn classify_severity(severity: &MisbehaviorSeverity) -> EventClass {
    match severity {
        MisbehaviorSeverity::Critical | MisbehaviorSeverity::Severe => EventClass::Slashable,
        MisbehaviorSeverity::Moderate => EventClass::Penalizable,
        MisbehaviorSeverity::Minor => EventClass::Informational,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_equivocation_is_slashable() {
        assert_eq!(classify_misbehavior(&MisbehaviorType::Equivocation), EventClass::Slashable);
    }

    #[test]
    fn test_absent_is_informational() {
        assert_eq!(classify_misbehavior(&MisbehaviorType::AbsentFromDuty), EventClass::Informational);
    }

    #[test]
    fn test_invalid_attestation_is_penalizable() {
        assert_eq!(
            classify_misbehavior(&MisbehaviorType::InvalidAttestation),
            EventClass::Penalizable
        );
    }

    #[test]
    fn test_severity_critical_slashable() {
        assert_eq!(classify_severity(&MisbehaviorSeverity::Critical), EventClass::Slashable);
    }

    #[test]
    fn test_severity_moderate_penalizable() {
        assert_eq!(classify_severity(&MisbehaviorSeverity::Moderate), EventClass::Penalizable);
    }

    #[test]
    fn test_severity_minor_informational() {
        assert_eq!(classify_severity(&MisbehaviorSeverity::Minor), EventClass::Informational);
    }
}
