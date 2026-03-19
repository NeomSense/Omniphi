use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::chain_bridge::exporter::StatusRecommendation;

/// How an evidence event is routed for processing.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum AdjudicationPath {
    /// Deterministic penalty applied automatically from the PoSeq node.
    Automatic,
    /// Requires governance vote before any penalty is applied.
    GovernanceReview,
}

/// The outcome of adjudication for one evidence event.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum AdjudicationDecision {
    /// Not yet decided.
    Pending,
    /// Automatic penalty applied.
    Penalized,
    /// Escalated to governance for manual review.
    Escalated,
    /// Evidence insufficient or not penalizable; no action.
    Dismissed,
}

/// The adjudication record for one evidence packet.
///
/// Keyed by `packet_hash` in the persistence store.
/// Write-once for finalized decisions (Penalized/Dismissed/Escalated).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AdjudicationRecord {
    /// 32-byte evidence packet hash.
    pub packet_hash: [u8; 32],
    /// 32-byte node identity of the accused.
    pub node_id: [u8; 32],
    /// String misbehavior type (e.g. "Equivocation").
    pub misbehavior_type: String,
    /// Epoch when the misbehavior occurred.
    pub epoch: u64,
    /// Routing path.
    pub path: AdjudicationPath,
    /// Current decision state.
    pub decision: AdjudicationDecision,
    /// Slash applied in basis points. 0 if dismissed or pending.
    pub slash_bps: u32,
    /// Epoch when the final decision was recorded. 0 = still pending.
    pub decided_at_epoch: u64,
    /// Human-readable reason.
    pub reason: String,
    /// True if the penalty was applied automatically without governance.
    pub auto_applied: bool,
}

/// Errors from the adjudication engine.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AdjudicationError {
    /// A finalized record already exists for this packet hash.
    AlreadyFinalized { packet_hash: [u8; 32] },
    /// The packet hash is invalid (wrong length).
    InvalidPacketHash,
}

impl std::fmt::Display for AdjudicationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::AlreadyFinalized { .. } => write!(f, "adjudication already finalized for this evidence"),
            Self::InvalidPacketHash => write!(f, "invalid evidence packet hash"),
        }
    }
}

impl std::error::Error for AdjudicationError {}

/// Returns the default slash_bps for a severity string.
///
/// - Minor    → 0 bps (Informational — no slash)
/// - Moderate → 300 bps (3%)
/// - Severe   → 1000 bps (10%)
/// - Critical → 2000 bps (20%)
pub fn slash_bps_for_severity(severity: &str) -> u32 {
    match severity {
        "Minor" => 0,
        "Moderate" => 300,
        "Severe" => 1000,
        "Critical" => 2000,
        _ => 0,
    }
}

/// Returns true if the severity qualifies for automatic adjudication (auto-penalty).
/// Only Moderate is automatic; Minor is Informational (dismissed), Severe and Critical require governance.
pub fn is_auto_adjudicable(severity: &str) -> bool {
    matches!(severity, "Moderate")
}

/// The adjudication engine.
///
/// Holds in-memory adjudication records keyed by packet hash.
/// Acts as the PoSeq side of the evidence adjudication pipeline.
pub struct AdjudicationEngine {
    /// Records indexed by packet_hash.
    records: BTreeMap<[u8; 32], AdjudicationRecord>,
}

impl AdjudicationEngine {
    pub fn new() -> Self {
        AdjudicationEngine {
            records: BTreeMap::new(),
        }
    }

    /// Adjudicate an evidence event. Returns the resulting record.
    ///
    /// If a finalized record already exists for this packet_hash, returns
    /// `AdjudicationError::AlreadyFinalized` (idempotent protection).
    ///
    /// Routing:
    /// - Minor    → Automatic path → Dismissed (Informational, no slash)
    /// - Moderate → Automatic path → Penalized + slash_bps applied
    /// - Severe / Critical → GovernanceReview path → Escalated (no immediate slash)
    pub fn adjudicate(
        &mut self,
        packet_hash: [u8; 32],
        node_id: [u8; 32],
        misbehavior_type: &str,
        severity: &str,
        epoch: u64,
    ) -> Result<&AdjudicationRecord, AdjudicationError> {
        // Idempotency: return existing if already finalized
        if let Some(existing) = self.records.get(&packet_hash) {
            if existing.decision != AdjudicationDecision::Pending {
                return Err(AdjudicationError::AlreadyFinalized { packet_hash });
            }
        }

        let slash_bps = slash_bps_for_severity(severity);
        let (path, decision) = if severity == "Minor" {
            // Minor is Informational — no enforcement action, no slash.
            (AdjudicationPath::Automatic, AdjudicationDecision::Dismissed)
        } else if is_auto_adjudicable(severity) {
            (AdjudicationPath::Automatic, AdjudicationDecision::Penalized)
        } else {
            (AdjudicationPath::GovernanceReview, AdjudicationDecision::Escalated)
        };

        let reason = format!("auto-adjudicated: {} severity {}", misbehavior_type, severity);
        let auto_applied = path == AdjudicationPath::Automatic;

        let rec = AdjudicationRecord {
            packet_hash,
            node_id,
            misbehavior_type: misbehavior_type.to_string(),
            epoch,
            path,
            decision,
            slash_bps,
            decided_at_epoch: epoch,
            reason,
            auto_applied,
        };

        self.records.insert(packet_hash, rec);
        Ok(self.records.get(&packet_hash).unwrap())
    }

    /// Return the record for a packet hash, if any.
    pub fn get(&self, packet_hash: &[u8; 32]) -> Option<&AdjudicationRecord> {
        self.records.get(packet_hash)
    }

    /// Build `StatusRecommendation`s from all escalated (governance-review) records
    /// for a given epoch. These are included in the ExportBatch.
    pub fn escalated_recommendations(&self, epoch: u64) -> Vec<StatusRecommendation> {
        self.records
            .values()
            .filter(|r| r.epoch == epoch && r.decision == AdjudicationDecision::Escalated)
            .map(|r| StatusRecommendation {
                node_id: r.node_id,
                recommended_status: "Jailed".to_string(),
                reason: r.reason.clone(),
                epoch,
            })
            .collect()
    }

    /// Total number of records.
    pub fn len(&self) -> usize {
        self.records.len()
    }

    pub fn is_empty(&self) -> bool {
        self.records.is_empty()
    }
}

impl Default for AdjudicationEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pkt(b: u8) -> [u8; 32] {
        let mut h = [0u8; 32];
        h[0] = b;
        h
    }
    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_minor_dismissed_informational() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(1), nid(1), "AbsentFromDuty", "Minor", 5).unwrap();
        // Minor is Informational — dismissed with no slash.
        assert_eq!(rec.decision, AdjudicationDecision::Dismissed);
        assert_eq!(rec.path, AdjudicationPath::Automatic);
        assert_eq!(rec.slash_bps, 0);
        assert!(rec.auto_applied); // auto_applied = path is Automatic
    }

    #[test]
    fn test_moderate_adjudicated_automatically() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(2), nid(1), "UnfairSequencing", "Moderate", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Penalized);
        assert_eq!(rec.slash_bps, 300);
    }

    #[test]
    fn test_severe_escalated_to_governance() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(3), nid(1), "DoubleProposal", "Severe", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Escalated);
        assert_eq!(rec.path, AdjudicationPath::GovernanceReview);
        assert_eq!(rec.slash_bps, 1000);
        assert!(!rec.auto_applied);
    }

    #[test]
    fn test_critical_escalated_to_governance() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(4), nid(1), "BridgeAbuse", "Critical", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Escalated);
        assert_eq!(rec.slash_bps, 2000);
    }

    #[test]
    fn test_double_adjudication_rejected() {
        let mut engine = AdjudicationEngine::new();
        engine.adjudicate(pkt(5), nid(1), "Equivocation", "Minor", 5).unwrap();
        let err = engine.adjudicate(pkt(5), nid(1), "Equivocation", "Minor", 5);
        assert!(matches!(err, Err(AdjudicationError::AlreadyFinalized { .. })));
    }

    #[test]
    fn test_escalated_recommendations_for_epoch() {
        let mut engine = AdjudicationEngine::new();
        engine.adjudicate(pkt(6), nid(1), "BridgeAbuse", "Critical", 10).unwrap();
        engine.adjudicate(pkt(7), nid(2), "Equivocation", "Minor", 10).unwrap(); // not escalated
        let recs = engine.escalated_recommendations(10);
        assert_eq!(recs.len(), 1);
        assert_eq!(recs[0].recommended_status, "Jailed");
    }

    #[test]
    fn test_unknown_severity_zero_slash() {
        assert_eq!(slash_bps_for_severity("Unknown"), 0);
        assert!(!is_auto_adjudicable("Unknown"));
    }
}
