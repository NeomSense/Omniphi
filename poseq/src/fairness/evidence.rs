//! Phase 3 — Censorship evidence detection from fairness logs.
//!
//! Analyzes fairness logs to detect potential censorship patterns
//! and produce evidence that can be used in disputes or governance.

use super::log::{FairnessLog, FairnessLogEntry, ReasonCode};
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

/// Evidence of potential censorship detected in a fairness log.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CensorshipEvidence {
    pub evidence_type: CensorshipType,
    pub affected_solver_id: [u8; 32],
    pub affected_bundle_ids: Vec<[u8; 32]>,
    pub sequence_id: u64,
    pub description: String,
}

/// Types of censorship patterns.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum CensorshipType {
    /// Solver's bundles consistently excluded across multiple windows.
    RepeatedExclusion,
    /// All bundles excluded in a window (sequencer censored entire slot).
    TotalExclusion,
    /// Eligible bundles excluded without valid reason.
    UnjustifiedExclusion,
}

/// Analyze a fairness log for censorship patterns.
///
/// Returns evidence for any detected patterns.
pub fn detect_censorship(
    log: &FairnessLog,
    sequence_id: u64,
    max_consecutive_exclusions: usize,
) -> Vec<CensorshipEvidence> {
    let mut evidence = Vec::new();

    // Pattern 1: Total exclusion — all bundles excluded
    if !log.is_empty() && log.included_count() == 0 {
        evidence.push(CensorshipEvidence {
            evidence_type: CensorshipType::TotalExclusion,
            affected_solver_id: [0u8; 32], // system-wide
            affected_bundle_ids: log.entries().iter().map(|e| e.bundle_id).collect(),
            sequence_id,
            description: format!(
                "all {} bundles excluded from sequence {}",
                log.len(),
                sequence_id,
            ),
        });
    }

    // Pattern 2: Per-solver repeated exclusion
    let mut solver_exclusions: BTreeMap<[u8; 32], Vec<[u8; 32]>> = BTreeMap::new();
    for entry in log.entries() {
        if !entry.inclusion {
            solver_exclusions.entry(entry.solver_id)
                .or_default()
                .push(entry.bundle_id);
        }
    }

    for (solver_id, bundle_ids) in &solver_exclusions {
        if bundle_ids.len() >= max_consecutive_exclusions {
            evidence.push(CensorshipEvidence {
                evidence_type: CensorshipType::RepeatedExclusion,
                affected_solver_id: *solver_id,
                affected_bundle_ids: bundle_ids.clone(),
                sequence_id,
                description: format!(
                    "solver {} excluded {} times in sequence {}",
                    hex::encode(&solver_id[..4]),
                    bundle_ids.len(),
                    sequence_id,
                ),
            });
        }
    }

    // Pattern 3: Unjustified exclusion — eligible but excluded without OrderingPriority
    for entry in log.entries() {
        if entry.eligibility && !entry.inclusion && entry.reason_code != ReasonCode::OrderingPriority {
            evidence.push(CensorshipEvidence {
                evidence_type: CensorshipType::UnjustifiedExclusion,
                affected_solver_id: entry.solver_id,
                affected_bundle_ids: vec![entry.bundle_id],
                sequence_id,
                description: format!(
                    "bundle {} eligible but excluded with reason {:?}",
                    hex::encode(&entry.bundle_id[..4]),
                    entry.reason_code,
                ),
            });
        }
    }

    evidence
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fairness::log::FairnessLog;

    #[test]
    fn test_no_censorship_on_normal_log() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        log.record_excluded([2u8; 32], [20u8; 32], ReasonCode::OrderingPriority);

        let evidence = detect_censorship(&log, 1, 3);
        assert!(evidence.is_empty());
    }

    #[test]
    fn test_total_exclusion_detected() {
        let mut log = FairnessLog::new();
        log.record_excluded([1u8; 32], [10u8; 32], ReasonCode::SolverInactive);
        log.record_excluded([2u8; 32], [20u8; 32], ReasonCode::FailedDA);

        let evidence = detect_censorship(&log, 1, 3);
        assert!(evidence.iter().any(|e| e.evidence_type == CensorshipType::TotalExclusion));
    }

    #[test]
    fn test_repeated_exclusion_detected() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        // Solver [20u8; 32] excluded 3 times
        for i in 2..=4 {
            let mut bid = [0u8; 32]; bid[0] = i;
            log.record_excluded(bid, [20u8; 32], ReasonCode::OrderingPriority);
        }

        let evidence = detect_censorship(&log, 1, 3);
        assert!(evidence.iter().any(|e| {
            e.evidence_type == CensorshipType::RepeatedExclusion
                && e.affected_solver_id == [20u8; 32]
        }));
    }

    #[test]
    fn test_unjustified_exclusion_detected() {
        let mut log = FairnessLog::new();
        log.record_included([1u8; 32], [10u8; 32]);
        // Eligible bundle excluded with non-priority reason
        log.entries.push(FairnessLogEntry {
            bundle_id: [2u8; 32],
            solver_id: [20u8; 32],
            eligibility: true,
            inclusion: false,
            reason_code: ReasonCode::FailedDA, // eligible but failed DA — suspicious
        });

        let evidence = detect_censorship(&log, 1, 3);
        assert!(evidence.iter().any(|e| e.evidence_type == CensorshipType::UnjustifiedExclusion));
    }
}
