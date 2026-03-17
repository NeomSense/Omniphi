use crate::identities::node::NodeIdentity;
use crate::committee::membership::PoSeqCommittee;
use crate::leader_selection::selector::{LeaderSelector, LeaderSelectionPolicy};
use crate::proposals::batch::ProposedBatch;
use crate::attestations::collector::{AttestationCollector, AttestationThreshold, BatchAttestationVote};
use crate::finalization::engine::{FinalizationDecision, FinalizationEngine};
use crate::conflicts::detector::ConflictDetector;
use crate::persistence::store::{BatchLifecycleStore, InMemoryBatchLifecycleStore};
use crate::bridge::hardened::{HardenedRuntimeBridge, RuntimeDeliveryEnvelope, RuntimeExecutionAck};
use crate::replay::guard::{ProposalReplayGuard, AckReplayGuard};
use crate::commitment::hash::BatchCommitment;
use crate::receipts::lifecycle::{
    BatchLifecycleAuditRecord, DeliveryReceipt, ExportStatus, FinalizationReceipt,
};
use crate::errors::{AttestationError, BridgeError, ProposalError};

/// Phase 2 orchestrator: wires all Phase 2 components together.
pub struct Phase2PoSeqNode {
    pub identity: NodeIdentity,
    pub committee: PoSeqCommittee,
    pub leader_selection_policy: LeaderSelectionPolicy,
    pub attestation_threshold: AttestationThreshold,
    pub store: InMemoryBatchLifecycleStore,
    pub conflict_detector: ConflictDetector,
    pub bridge: HardenedRuntimeBridge,
    pub finalization_engine: FinalizationEngine,
    pub proposal_replay: ProposalReplayGuard,
    pub ack_replay: AckReplayGuard,
}

impl Phase2PoSeqNode {
    pub fn new(
        identity: NodeIdentity,
        committee: PoSeqCommittee,
        leader_selection_policy: LeaderSelectionPolicy,
        attestation_threshold: AttestationThreshold,
    ) -> Self {
        Phase2PoSeqNode {
            identity,
            committee,
            leader_selection_policy,
            attestation_threshold,
            store: InMemoryBatchLifecycleStore::new(),
            conflict_detector: ConflictDetector::new(),
            bridge: HardenedRuntimeBridge::new(),
            finalization_engine: FinalizationEngine::new(),
            proposal_replay: ProposalReplayGuard::new(0),
            ack_replay: AckReplayGuard::new(0),
        }
    }

    /// Propose a new batch for the given slot/epoch.
    /// Validates that this node is the selected leader, and that no replay occurs.
    pub fn propose_batch(
        &mut self,
        slot: u64,
        epoch: u64,
        height: u64,
        ordered_ids: Vec<[u8; 32]>,
        parent_batch_id: [u8; 32],
        policy_version: u32,
    ) -> Result<ProposedBatch, ProposalError> {
        // Check leader selection
        let selected = LeaderSelector::select(
            slot, epoch, &self.committee, &self.leader_selection_policy
        );
        let selected = match selected {
            Some(s) => s,
            None => return Err(ProposalError::InvalidSlot),
        };

        if selected.node_id != self.identity.node_id {
            return Err(ProposalError::NotLeader);
        }

        // Check for conflicts
        let incident = self.conflict_detector.check_proposal(
            slot, epoch, self.identity.node_id,
            // Compute a candidate proposal_id for conflict detection
            ProposedBatch::compute_root(&ordered_ids),
            &self.committee,
            height,
        );

        if let Some(inc) = incident {
            use crate::conflicts::detector::IncidentType;
            return match inc.incident_type {
                IncidentType::DualProposal => Err(ProposalError::AlreadyProposed),
                IncidentType::InvalidProposer => Err(ProposalError::NotLeader),
                IncidentType::ReplayedProposal => Err(ProposalError::AlreadyProposed),
                _ => Err(ProposalError::InvalidSlot),
            };
        }

        let batch = ProposedBatch::new(
            slot, epoch, self.identity.node_id, ordered_ids,
            parent_batch_id, policy_version, height,
        );

        // Replay guard
        if !self.proposal_replay.check_and_record(batch.proposal_id) {
            return Err(ProposalError::AlreadyProposed);
        }

        // Initialize attestation collector
        self.store.store_attestation_collector(
            batch.proposal_id,
            AttestationCollector::new(batch.proposal_id),
        );

        self.store.store_proposed_batch(batch.clone());
        Ok(batch)
    }

    /// Submit an attestation vote for a proposal.
    pub fn submit_attestation(
        &mut self,
        vote: BatchAttestationVote,
    ) -> Result<(), AttestationError> {
        // Check proposal exists
        if self.store.get_proposed_batch(&vote.proposal_id).is_none() {
            return Err(AttestationError::ProposalNotFound);
        }

        // Check attestor eligibility
        if !self.committee.is_member(&vote.attestor_id) {
            return Err(AttestationError::NotEligible);
        }

        let collector = self.store
            .get_attestation_collector_mut(&vote.proposal_id)
            .ok_or(AttestationError::ProposalNotFound)?;

        match collector.add_vote(vote) {
            Ok(_) => Ok(()),
            Err(_) => Err(AttestationError::ConflictingVote),
        }
    }

    /// Attempt to finalize a proposal. Returns the FinalizationDecision.
    pub fn try_finalize(
        &mut self,
        proposal_id: [u8; 32],
        height: u64,
    ) -> FinalizationDecision {
        let proposed = match self.store.get_proposed_batch(&proposal_id) {
            Some(p) => p.clone(),
            None => return FinalizationDecision::InsufficientAttestations,
        };
        let collector = match self.store.get_attestation_collector(&proposal_id) {
            Some(c) => c.clone(),
            None => return FinalizationDecision::InsufficientAttestations,
        };

        let committee_size = self.committee.quorum_size();
        let threshold = self.attestation_threshold.clone();
        let decision = self.finalization_engine.finalize(
            &proposed, &collector, &threshold, committee_size, height,
        );

        if let FinalizationDecision::Finalized(ref fb) = decision {
            self.store.store_finalized_batch(fb.clone());
        }

        decision
    }

    /// Export a finalized batch to the runtime bridge.
    pub fn export_to_runtime(
        &mut self,
        batch_id: [u8; 32],
    ) -> Result<RuntimeDeliveryEnvelope, BridgeError> {
        let fb = self.store.get_finalized_batch(&batch_id)
            .ok_or(BridgeError::DeliveryFailed)?
            .clone();
        let envelope = self.bridge.deliver(&fb);
        self.store.mark_exported(batch_id);
        Ok(envelope)
    }

    /// Record a runtime ack.
    pub fn record_runtime_ack(
        &mut self,
        ack: RuntimeExecutionAck,
    ) -> Result<(), BridgeError> {
        let batch_id = ack.batch_id;
        let accepted = ack.accepted;
        self.bridge.record_ack(ack)?;
        self.store.mark_acked(batch_id, accepted);
        Ok(())
    }

    /// Build a full lifecycle audit record for a batch.
    pub fn get_lifecycle_audit(
        &self,
        batch_id: [u8; 32],
    ) -> Option<BatchLifecycleAuditRecord> {
        let fb = self.store.get_finalized_batch(&batch_id)?;

        let proposed = self.store.get_proposed_batch(&fb.proposal_id)?;
        let commitment = BatchCommitment::compute(proposed, &fb.quorum_summary);

        let export_status = if self.store.is_acked(&batch_id) {
            ExportStatus::Acknowledged
        } else if self.store.is_exported(&batch_id) {
            ExportStatus::Delivered
        } else {
            ExportStatus::Pending
        };

        let finalization_receipt = FinalizationReceipt {
            batch_id: fb.batch_id,
            proposal_id: fb.proposal_id,
            slot: fb.slot,
            epoch: fb.epoch,
            leader_id: fb.leader_id,
            finalized_at_height: fb.finalized_at_height,
            quorum_summary: fb.quorum_summary.clone(),
            commitment,
            export_status,
        };

        let delivery_receipt = if self.bridge.is_delivered(&batch_id) {
            let record = self.bridge.get_record(&batch_id)?;
            Some(DeliveryReceipt {
                batch_id,
                delivery_id: record.delivery_id,
                attempt_count: record.attempt_count,
                acked: record.acked,
                accepted: record.accepted,
                rejection_reason: None,
            })
        } else {
            None
        };

        let incident_ids: Vec<[u8; 32]> = self.store.get_misbehavior_ledger()
            .iter()
            .map(|i| i.incident_id)
            .collect();

        Some(BatchLifecycleAuditRecord {
            batch_id,
            proposal_id: fb.proposal_id,
            finalization_receipt,
            delivery_receipt,
            incident_ids,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::identities::node::{NodeIdentity, NodeRole};
    use crate::committee::membership::PoSeqCommittee;
    use crate::leader_selection::selector::LeaderSelectionPolicy;
    use crate::attestations::collector::{AttestationThreshold, BatchAttestationVote};
    use crate::bridge::hardened::RuntimeExecutionAck;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_active_node(b: u8, role: NodeRole) -> NodeIdentity {
        let mut n = NodeIdentity::new(make_id(b), make_id(b + 100), role, 0);
        n.activate();
        n
    }

    /// Build a node where make_id(1) is the expected RoundRobin leader for slot 0.
    fn build_test_node() -> Phase2PoSeqNode {
        let identity = make_active_node(1, NodeRole::Sequencer);
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(make_active_node(1, NodeRole::Sequencer));
        committee.add_node(make_active_node(2, NodeRole::Sequencer));
        committee.add_node(make_active_node(3, NodeRole::Validator));

        let threshold = AttestationThreshold::two_thirds(3);
        Phase2PoSeqNode::new(
            identity,
            committee,
            LeaderSelectionPolicy::RoundRobin,
            threshold,
        )
    }

    #[test]
    fn test_propose_batch_as_leader() {
        let mut node = build_test_node();
        // RoundRobin slot 0 % 2 = 0 → index 0 → smallest node_id = make_id(1)
        let result = node.propose_batch(
            0, 1, 100,
            vec![make_id(10), make_id(11)],
            [0u8; 32], 1
        );
        assert!(result.is_ok());
        let batch = result.unwrap();
        assert_eq!(batch.leader_id, make_id(1));
    }

    #[test]
    fn test_propose_batch_not_leader() {
        let identity = make_active_node(2, NodeRole::Sequencer); // node 2 is NOT slot-0 leader
        let mut committee = PoSeqCommittee::new(1);
        committee.add_node(make_active_node(1, NodeRole::Sequencer));
        committee.add_node(make_active_node(2, NodeRole::Sequencer));
        let threshold = AttestationThreshold::two_thirds(2);
        let mut node = Phase2PoSeqNode::new(
            identity, committee, LeaderSelectionPolicy::RoundRobin, threshold
        );
        let result = node.propose_batch(0, 1, 100, vec![make_id(10)], [0u8; 32], 1);
        assert_eq!(result, Err(ProposalError::NotLeader));
    }

    #[test]
    fn test_full_lifecycle_propose_attest_finalize_export_ack() {
        let mut node = build_test_node();

        // Propose
        let batch = node.propose_batch(
            0, 1, 100,
            vec![make_id(10), make_id(11)],
            [0u8; 32], 1
        ).unwrap();

        // Attest with all 3 eligible attestors (sequencer 1, sequencer 2, validator 3)
        let attestors = [make_id(1), make_id(2), make_id(3)];
        for &attestor in &attestors {
            let vote = BatchAttestationVote::new(batch.proposal_id, attestor, true, 1);
            node.submit_attestation(vote).unwrap();
        }

        // Try finalize
        let decision = node.try_finalize(batch.proposal_id, 101);
        let finalized_batch = match decision {
            FinalizationDecision::Finalized(fb) => fb,
            other => panic!("Expected Finalized, got {:?}", other),
        };

        // Export
        let envelope = node.export_to_runtime(finalized_batch.batch_id).unwrap();
        assert_eq!(envelope.batch_id, finalized_batch.batch_id);

        // Ack
        let ack = RuntimeExecutionAck::new(
            finalized_batch.batch_id, envelope.delivery_id, true, 1
        );
        assert!(node.record_runtime_ack(ack).is_ok());

        // Audit record
        let audit = node.get_lifecycle_audit(finalized_batch.batch_id);
        assert!(audit.is_some());
        let audit = audit.unwrap();
        assert_eq!(audit.batch_id, finalized_batch.batch_id);
        assert!(audit.delivery_receipt.is_some());
        assert!(audit.delivery_receipt.unwrap().accepted);
    }

    #[test]
    fn test_attestation_not_eligible() {
        let mut node = build_test_node();
        let batch = node.propose_batch(0, 1, 100, vec![make_id(10)], [0u8; 32], 1).unwrap();
        // make_id(99) is not in committee
        let vote = BatchAttestationVote::new(batch.proposal_id, make_id(99), true, 1);
        let result = node.submit_attestation(vote);
        assert_eq!(result, Err(AttestationError::NotEligible));
    }

    #[test]
    fn test_finalize_insufficient_attestations() {
        let mut node = build_test_node();
        let batch = node.propose_batch(0, 1, 100, vec![make_id(10)], [0u8; 32], 1).unwrap();
        // No attestations submitted
        let decision = node.try_finalize(batch.proposal_id, 101);
        assert!(matches!(decision, FinalizationDecision::InsufficientAttestations));
    }
}
