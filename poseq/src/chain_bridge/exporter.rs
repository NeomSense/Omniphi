//! `ChainBridgeExporter` — orchestrates the packaging of all PoSeq outputs
//! into chain-digestible records for an epoch.
//!
//! At the end of each epoch (or on-demand), the exporter:
//! 1. Collects all misbehavior cases and packages them as `EvidencePacket`s
//! 2. Applies the `DuplicateEvidenceGuard` to prevent re-submission
//! 3. Generates `GovernanceEscalationRecord`s for severe/critical cases
//! 4. Produces `CommitteeSuspensionRecommendation`s for nodes that should be blocked
//! 5. Produces an optional `CheckpointAnchorRecord` if a checkpoint is available
//! 6. Returns an `ExportBatch` ready for chain submission

use std::collections::BTreeMap;
use crate::misbehavior::types::{MisbehaviorType, MisbehaviorSeverity};
use crate::penalties::PenaltyRecommendation;
use crate::chain_bridge::evidence::{
    EvidencePacket, EvidencePacketSet, PenaltyRecommendationRecord,
    DuplicateEvidenceGuard,
};
use crate::chain_bridge::escalation::{
    GovernanceEscalationRecord, EscalationAction, CommitteeSuspensionRecommendation,
};
use crate::chain_bridge::anchor::{
    CheckpointAnchorRecord, BatchFinalityReference, EpochStateReference,
};
use crate::liveness::events::{LivenessEventExport, InactivityEvent};
use crate::performance::record::NodePerformanceRecord;
use crate::enforcement::rules::{evaluate_epoch, EnforcementConfig};
use crate::reward::score::{build_reward_score, EpochRewardScore};
use crate::reward::poc::PoCMultiplierStore;
use crate::bonding::store::BondingStore;

// ─── Raw misbehavior input (from PoSeq internal modules) ─────────────────────

/// A PoSeq misbehavior incident ready for packaging.
/// Callers (e.g., epoch-end processing) collect these from `MisbehaviorHistory`
/// and `FairnessIncidentDetector` and pass them to `ChainBridgeExporter`.
#[derive(Debug, Clone)]
pub struct MisbehaviorIncidentInput {
    pub mtype: MisbehaviorType,
    pub offender_node_id: [u8; 32],
    pub epoch: u64,
    pub slot: u64,
    pub evidence_hashes: Vec<[u8; 32]>,
    pub linked_batch_id: Option<[u8; 32]>,
    pub penalty: Option<PenaltyRecommendation>,
}

// ─── ExportBatch ─────────────────────────────────────────────────────────────

/// A status recommendation from PoSeq to the chain.
/// If `AutoApplySuspensions` is enabled on-chain, the chain will apply these automatically.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct StatusRecommendation {
    pub node_id: [u8; 32],
    /// Recommended status string: "Suspended" or "Jailed".
    pub recommended_status: String,
    pub reason: String,
    pub epoch: u64,
}

/// A complete epoch export — all records ready for chain submission.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ExportBatch {
    pub epoch: u64,

    /// Evidence packets (one per unique misbehavior incident).
    pub evidence_set: EvidencePacketSet,

    /// Governance escalation records (severe + critical only).
    pub escalations: Vec<GovernanceEscalationRecord>,

    /// Committee suspension recommendations (applies to severe + critical nodes).
    pub suspensions: Vec<CommitteeSuspensionRecommendation>,

    /// Optional checkpoint anchor (present if an epoch checkpoint exists).
    pub checkpoint_anchor: Option<CheckpointAnchorRecord>,

    /// Epoch state summary for operator visibility.
    pub epoch_state: EpochStateReference,

    /// Liveness events for this epoch — active observations and inactivity alerts.
    pub liveness_events: Vec<LivenessEventExport>,

    /// Per-node performance records for this epoch.
    pub performance_records: Vec<NodePerformanceRecord>,

    /// Inactivity events extracted from liveness exports for convenience.
    pub inactivity_events: Vec<InactivityEvent>,

    /// Status change recommendations for the chain registry.
    pub status_recommendations: Vec<StatusRecommendation>,

    /// Per-node reward scores for this epoch, ready for distribution hooks.
    pub reward_scores: Vec<EpochRewardScore>,
}

impl ExportBatch {
    pub fn is_empty(&self) -> bool {
        self.evidence_set.is_empty()
            && self.escalations.is_empty()
            && self.suspensions.is_empty()
            && self.checkpoint_anchor.is_none()
            && self.liveness_events.is_empty()
            && self.performance_records.is_empty()
    }
}

// ─── ExportResult ─────────────────────────────────────────────────────────────

#[derive(Debug)]
pub struct ExportResult {
    pub epoch: u64,
    pub evidence_packaged: usize,
    pub evidence_deduplicated: usize,
    pub escalations: usize,
    pub suspensions: usize,
    pub has_checkpoint: bool,
}

// ─── ChainBridgeExporter ──────────────────────────────────────────────────────

/// Orchestrates packaging of all PoSeq accountability outputs for chain submission.
pub struct ChainBridgeExporter {
    dedup_guard: DuplicateEvidenceGuard,
}

impl ChainBridgeExporter {
    pub fn new() -> Self {
        ChainBridgeExporter {
            dedup_guard: DuplicateEvidenceGuard::new(),
        }
    }

    /// Package all misbehavior incidents for an epoch into a chain-ready `ExportBatch`.
    ///
    /// # Arguments
    /// - `epoch`: the epoch being exported
    /// - `incidents`: all misbehavior incidents for this epoch
    /// - `committee_hash`: SHA256 of the committee membership this epoch
    /// - `finalized_batch_count`: total finalized batches this epoch
    /// - `checkpoint_anchor`: optional checkpoint anchor if one was created
    pub fn export(
        &mut self,
        epoch: u64,
        incidents: Vec<MisbehaviorIncidentInput>,
        committee_hash: [u8; 32],
        finalized_batch_count: u64,
        checkpoint_anchor: Option<CheckpointAnchorRecord>,
    ) -> (ExportBatch, ExportResult) {
        let mut packets: Vec<EvidencePacket> = Vec::new();
        let mut penalty_records: Vec<PenaltyRecommendationRecord> = Vec::new();
        let mut escalations: Vec<GovernanceEscalationRecord> = Vec::new();
        let mut suspensions: Vec<CommitteeSuspensionRecommendation> = Vec::new();
        let mut dedup_count = 0usize;

        for incident in &incidents {
            // Build evidence packet
            let packet = EvidencePacket::from_misbehavior(
                &incident.mtype,
                incident.offender_node_id,
                incident.epoch,
                incident.slot,
                incident.evidence_hashes.clone(),
                incident.linked_batch_id,
            );

            // Deduplicate
            if !self.dedup_guard.register(&packet) {
                dedup_count += 1;
                continue;
            }

            let packet_hash = packet.packet_hash;

            // Penalty record
            if let Some(ref penalty) = incident.penalty {
                let pr = PenaltyRecommendationRecord::from_penalty(packet_hash, penalty, epoch);
                penalty_records.push(pr);
            }

            // Governance escalation for severe/critical
            match incident.mtype.default_severity() {
                MisbehaviorSeverity::Critical => {
                    let action = if incident.mtype == MisbehaviorType::Equivocation
                        || incident.mtype == MisbehaviorType::SlotHijackingAttempt
                        || incident.mtype == MisbehaviorType::ReplayAttack
                        || incident.mtype == MisbehaviorType::RuntimeBridgeAbuse
                    {
                        EscalationAction::PermanentBan
                    } else {
                        EscalationAction::SuspendFromCommittee { epochs: 10 }
                    };
                    escalations.push(GovernanceEscalationRecord::from_evidence(
                        &packet, action, true,
                    ));
                    suspensions.push(CommitteeSuspensionRecommendation::new(
                        incident.offender_node_id,
                        epoch, 10,
                        packet_hash,
                        format!("Critical misbehavior: {:?}", incident.mtype),
                    ));
                }
                MisbehaviorSeverity::Severe => {
                    let suspend_epochs = 5u64;
                    escalations.push(GovernanceEscalationRecord::from_evidence(
                        &packet,
                        EscalationAction::SuspendFromCommittee { epochs: suspend_epochs },
                        true,
                    ));
                    suspensions.push(CommitteeSuspensionRecommendation::new(
                        incident.offender_node_id,
                        epoch, suspend_epochs,
                        packet_hash,
                        format!("Severe misbehavior: {:?}", incident.mtype),
                    ));
                }
                _ => {}
            }

            packets.push(packet);
        }

        let evidence_count = packets.len();
        let escalation_count = escalations.len();
        let suspension_count = suspensions.len();

        let evidence_set = EvidencePacketSet::new(epoch, packets, penalty_records);

        let epoch_state = EpochStateReference::new(
            epoch,
            committee_hash,
            finalized_batch_count,
            incidents.len() as u32,
            evidence_count as u32,
            escalation_count as u32,
        );

        let has_checkpoint = checkpoint_anchor.is_some();

        let batch = ExportBatch {
            epoch,
            evidence_set,
            escalations,
            suspensions,
            checkpoint_anchor,
            epoch_state,
            liveness_events: Vec::new(),
            performance_records: Vec::new(),
            inactivity_events: Vec::new(),
            status_recommendations: Vec::new(),
            reward_scores: Vec::new(),
        };

        let result = ExportResult {
            epoch,
            evidence_packaged: evidence_count,
            evidence_deduplicated: dedup_count,
            escalations: escalation_count,
            suspensions: suspension_count,
            has_checkpoint,
        };

        (batch, result)
    }

    /// Extended export that includes liveness, performance, and status recommendation data.
    ///
    /// Calls the base `export()` and appends the accountability fields.
    /// Existing callers using `export()` continue to receive empty slices for those fields.
    pub fn export_with_accountability(
        &mut self,
        epoch: u64,
        incidents: Vec<MisbehaviorIncidentInput>,
        committee_hash: [u8; 32],
        finalized_batch_count: u64,
        checkpoint_anchor: Option<CheckpointAnchorRecord>,
        liveness_events: Vec<LivenessEventExport>,
        performance_records: Vec<NodePerformanceRecord>,
        status_recommendations: Vec<StatusRecommendation>,
    ) -> (ExportBatch, ExportResult) {
        let (mut batch, result) = self.export(
            epoch,
            incidents,
            committee_hash,
            finalized_batch_count,
            checkpoint_anchor,
        );

        // Flatten inactivity events from all liveness exports.
        let inactivity_events: Vec<InactivityEvent> = liveness_events
            .iter()
            .flat_map(|le| le.inactivity_events.iter().cloned())
            .collect();

        batch.liveness_events = liveness_events;
        batch.performance_records = performance_records;
        batch.inactivity_events = inactivity_events;
        batch.status_recommendations = status_recommendations;

        (batch, result)
    }

    /// Fully integrated epoch export with automatic enforcement evaluation and
    /// reward score computation.
    ///
    /// This is the top-level method for production epoch-end processing.
    /// It:
    /// 1. Packages misbehavior evidence via `export_with_accountability`
    /// 2. Runs the enforcement engine to generate `StatusRecommendation`s
    ///    from inactivity events and performance records (merging with any
    ///    explicit recommendations passed in)
    /// 3. Computes per-node `EpochRewardScore`s
    ///
    /// The `enforcement_config` controls inactivity and fault thresholds.
    /// Set thresholds to 0 to disable automatic enforcement.
    #[allow(clippy::too_many_arguments)]
    pub fn export_epoch(
        &mut self,
        epoch: u64,
        incidents: Vec<MisbehaviorIncidentInput>,
        committee_hash: [u8; 32],
        finalized_batch_count: u64,
        checkpoint_anchor: Option<CheckpointAnchorRecord>,
        liveness_events: Vec<LivenessEventExport>,
        performance_records: Vec<NodePerformanceRecord>,
        explicit_status_recommendations: Vec<StatusRecommendation>,
        enforcement_config: &EnforcementConfig,
        poc_store: &PoCMultiplierStore,
        bonding_store: &BondingStore,
    ) -> (ExportBatch, ExportResult) {
        // Collect all inactivity events across liveness exports.
        let inactivity_events: Vec<InactivityEvent> = liveness_events
            .iter()
            .flat_map(|le| le.inactivity_events.iter().cloned())
            .collect();

        // Run enforcement engine — merge with explicit recommendations.
        let mut auto_recs = evaluate_epoch(&inactivity_events, &performance_records, enforcement_config, epoch);
        auto_recs.extend(explicit_status_recommendations);
        // Deduplicate: Jailed wins over Suspended for same node.
        let mut deduped: std::collections::BTreeMap<[u8; 32], StatusRecommendation> =
            std::collections::BTreeMap::new();
        for rec in auto_recs {
            let entry = deduped.entry(rec.node_id).or_insert_with(|| rec.clone());
            if rec.recommended_status == "Jailed" && entry.recommended_status == "Suspended" {
                *entry = rec;
            }
        }
        let status_recommendations: Vec<StatusRecommendation> = deduped.into_values().collect();

        // Compute reward scores for each node with performance data.
        let liveness_by_epoch: Option<&LivenessEventExport> = liveness_events.first();
        let reward_scores: Vec<EpochRewardScore> = performance_records
            .iter()
            .map(|pr| {
                let operator = bonding_store.operator_for_node(&pr.node_id).map(str::to_string);
                let is_bonded = bonding_store.has_active_bond(&pr.node_id);
                let poc_mult = operator
                    .as_deref()
                    .map(|op| poc_store.get(epoch, op))
                    .unwrap_or(10_000);
                build_reward_score(pr, liveness_by_epoch, poc_mult, operator, is_bonded)
            })
            .collect();

        let (mut batch, result) = self.export_with_accountability(
            epoch,
            incidents,
            committee_hash,
            finalized_batch_count,
            checkpoint_anchor,
            liveness_events,
            performance_records,
            status_recommendations,
        );

        batch.reward_scores = reward_scores;
        (batch, result)
    }
}

impl Default for ChainBridgeExporter {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::misbehavior::types::MisbehaviorSeverity;

    fn nid(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[0] = b; id }
    fn eh(b: u8) -> [u8; 32] { let mut id = [0u8; 32]; id[31] = b; id }

    fn incident(mtype: MisbehaviorType, node: u8, epoch: u64, slot: u64) -> MisbehaviorIncidentInput {
        let penalty = Some(PenaltyRecommendation::from_misbehavior(
            nid(node), &mtype.default_severity(), vec![],
        ));
        MisbehaviorIncidentInput {
            mtype,
            offender_node_id: nid(node),
            epoch,
            slot,
            evidence_hashes: vec![eh(node)],
            linked_batch_id: None,
            penalty,
        }
    }

    // ── Happy path ───────────────────────────────────────────────────────────

    #[test]
    fn test_export_empty_incidents() {
        let mut exporter = ChainBridgeExporter::new();
        let (batch, result) = exporter.export(1, vec![], [0u8; 32], 0, None);
        assert!(batch.is_empty());
        assert_eq!(result.evidence_packaged, 0);
        assert_eq!(result.escalations, 0);
    }

    #[test]
    fn test_export_minor_incident_no_escalation() {
        let mut exporter = ChainBridgeExporter::new();
        let (batch, result) = exporter.export(
            1,
            vec![incident(MisbehaviorType::AbsentFromDuty, 1, 1, 0)],
            [0u8; 32], 10, None,
        );
        assert_eq!(result.evidence_packaged, 1);
        assert_eq!(result.escalations, 0);
        assert_eq!(result.suspensions, 0);
        assert_eq!(batch.evidence_set.slashable_count(), 0);
    }

    #[test]
    fn test_export_critical_triggers_escalation_and_suspension() {
        let mut exporter = ChainBridgeExporter::new();
        let (batch, result) = exporter.export(
            2,
            vec![incident(MisbehaviorType::Equivocation, 1, 2, 1)],
            [0u8; 32], 5, None,
        );
        assert_eq!(result.evidence_packaged, 1);
        assert_eq!(result.escalations, 1);
        assert_eq!(result.suspensions, 1);
        assert_eq!(batch.evidence_set.slashable_count(), 1);
        // Critical equivocation → permanent ban
        assert!(matches!(
            batch.escalations[0].recommended_action,
            EscalationAction::PermanentBan,
        ));
    }

    #[test]
    fn test_export_severe_triggers_suspension() {
        let mut exporter = ChainBridgeExporter::new();
        let (batch, result) = exporter.export(
            3,
            vec![incident(MisbehaviorType::InvalidProposalAuthority, 2, 3, 0)],
            [0u8; 32], 8, None,
        );
        assert_eq!(result.escalations, 1);
        assert_eq!(result.suspensions, 1);
        assert!(matches!(
            batch.escalations[0].recommended_action,
            EscalationAction::SuspendFromCommittee { epochs: 5 },
        ));
    }

    // ── Deduplication ────────────────────────────────────────────────────────

    #[test]
    fn test_duplicate_incident_dropped() {
        let mut exporter = ChainBridgeExporter::new();
        let inc = incident(MisbehaviorType::Equivocation, 1, 1, 0);

        let (_, r1) = exporter.export(1, vec![inc.clone()], [0u8; 32], 5, None);
        assert_eq!(r1.evidence_packaged, 1);
        assert_eq!(r1.evidence_deduplicated, 0);

        // Same incident same exporter (same epoch) → deduped
        let (_, r2) = exporter.export(1, vec![inc], [0u8; 32], 5, None);
        assert_eq!(r2.evidence_packaged, 0);
        assert_eq!(r2.evidence_deduplicated, 1);
    }

    #[test]
    fn test_different_epochs_not_deduped() {
        let mut exporter = ChainBridgeExporter::new();
        let inc1 = incident(MisbehaviorType::Equivocation, 1, 1, 0);
        let inc2 = incident(MisbehaviorType::Equivocation, 1, 2, 0); // epoch=2

        let (_, r1) = exporter.export(1, vec![inc1], [0u8; 32], 5, None);
        let (_, r2) = exporter.export(2, vec![inc2], [0u8; 32], 5, None);
        assert_eq!(r1.evidence_packaged, 1);
        assert_eq!(r2.evidence_packaged, 1); // different epoch → different hash
    }

    // ── Checkpoint anchoring ─────────────────────────────────────────────────

    #[test]
    fn test_checkpoint_anchor_included_in_batch() {
        let mut exporter = ChainBridgeExporter::new();
        let finality = crate::chain_bridge::anchor::BatchFinalityReference {
            batch_id: nid(1),
            slot: 5,
            epoch: 3,
            finalization_hash: nid(2),
            submission_count: 3,
            quorum_approvals: 3,
            committee_size: 4,
        };
        let anchor = CheckpointAnchorRecord::build(
            [0xAAu8; 32], 3, 5,
            [0xBBu8; 32], [0xCCu8; 32],
            1, finality,
        );
        let (batch, result) = exporter.export(3, vec![], [0u8; 32], 10, Some(anchor));
        assert!(result.has_checkpoint);
        assert!(batch.checkpoint_anchor.is_some());
        assert!(batch.checkpoint_anchor.unwrap().verify());
    }

    // ── Epoch state ──────────────────────────────────────────────────────────

    #[test]
    fn test_epoch_state_populated() {
        let mut exporter = ChainBridgeExporter::new();
        let incidents = vec![
            incident(MisbehaviorType::Equivocation, 1, 4, 0),
            incident(MisbehaviorType::AbsentFromDuty, 2, 4, 0),
        ];
        let (batch, _) = exporter.export(4, incidents, [0x55u8; 32], 20, None);
        assert_eq!(batch.epoch_state.epoch, 4);
        assert_eq!(batch.epoch_state.finalized_batch_count, 20);
        assert_eq!(batch.epoch_state.misbehavior_count, 2);
        assert_eq!(batch.epoch_state.governance_escalations, 1); // only critical
    }

    // ── Multiple incidents same node ─────────────────────────────────────────

    #[test]
    fn test_multiple_incidents_same_node() {
        let mut exporter = ChainBridgeExporter::new();
        let incidents = vec![
            incident(MisbehaviorType::Equivocation, 1, 5, 1),
            incident(MisbehaviorType::ReplayAttack, 1, 5, 2), // same node, different type
        ];
        let (batch, result) = exporter.export(5, incidents, [0u8; 32], 0, None);
        // Both are different types → different evidence_hashes → different packet_hashes
        assert_eq!(result.evidence_packaged, 2);
        assert_eq!(result.escalations, 2);
        assert_eq!(batch.suspensions.len(), 2);
    }

    // ── Penalty records ──────────────────────────────────────────────────────

    #[test]
    fn test_penalty_record_included_for_incident_with_penalty() {
        let mut exporter = ChainBridgeExporter::new();
        let inc = incident(MisbehaviorType::Equivocation, 1, 1, 0);
        let (batch, _) = exporter.export(1, vec![inc], [0u8; 32], 0, None);
        assert_eq!(batch.evidence_set.penalty_records.len(), 1);
        assert_eq!(batch.evidence_set.penalty_records[0].slash_bps, 10000); // Critical
    }

    // ── Bridge abuse linked batch ────────────────────────────────────────────

    #[test]
    fn test_bridge_abuse_with_linked_batch() {
        let mut exporter = ChainBridgeExporter::new();
        let bid = [0xBBu8; 32];
        let mut inc = incident(MisbehaviorType::RuntimeBridgeAbuse, 1, 1, 1);
        inc.linked_batch_id = Some(bid);

        let (batch, _) = exporter.export(1, vec![inc], [0u8; 32], 0, None);
        let pkt = &batch.evidence_set.packets[0];
        assert_eq!(pkt.linked_batch_id, Some(bid));
        assert_eq!(pkt.kind, crate::chain_bridge::evidence::EvidenceKind::BridgeAbuse);
    }
}
