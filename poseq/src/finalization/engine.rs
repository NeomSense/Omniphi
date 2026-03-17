use std::collections::{BTreeMap, BTreeSet};
use sha2::{Sha256, Digest};
use crate::proposals::batch::ProposedBatch;
use crate::attestations::collector::{AttestationCollector, AttestationThreshold, AttestationQuorumResult};

/// A canonically finalized batch — immutable after creation.
#[derive(Debug, Clone)]
pub struct FinalizedBatch {
    pub batch_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub batch_root: [u8; 32],
    pub parent_batch_id: [u8; 32],
    pub finalized_at_height: u64,
    pub quorum_summary: AttestationQuorumResult,
    pub finalization_hash: [u8; 32],
}

impl FinalizedBatch {
    /// Computes the finalization hash over all deterministic fields.
    pub fn compute_finalization_hash(
        proposal_id: &[u8; 32],
        slot: u64,
        epoch: u64,
        leader_id: &[u8; 32],
        batch_root: &[u8; 32],
        parent_batch_id: &[u8; 32],
        finalized_at_height: u64,
        quorum_hash: &[u8; 32],
    ) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(proposal_id);
        hasher.update(&slot.to_be_bytes());
        hasher.update(&epoch.to_be_bytes());
        hasher.update(leader_id);
        hasher.update(batch_root);
        hasher.update(parent_batch_id);
        hasher.update(&finalized_at_height.to_be_bytes());
        hasher.update(quorum_hash);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }

    /// The batch_id is the finalization_hash itself — unique per finalization event.
    pub fn batch_id(&self) -> [u8; 32] {
        self.batch_id
    }
}

/// Outcome of attempting finalization.
#[derive(Debug, Clone)]
pub enum FinalizationDecision {
    Finalized(FinalizedBatch),
    InsufficientAttestations,
    ConflictDetected(String),
    AlreadyFinalized,
    /// FIND-012: a different proposal for the same (slot, epoch) was already finalized.
    /// This is a critical invariant violation — one canonical batch per slot.
    SlotAlreadyFinalized {
        slot: u64,
        epoch: u64,
        existing_proposal_id: [u8; 32],
    },
}

/// Tracks finalized proposal IDs and (slot, epoch) pairs to prevent double-finalization.
///
/// FIND-012: `finalized_proposals` alone cannot prevent two distinct proposals for the
/// same slot/epoch from both finalizing. `finalized_slots` closes that gap.
pub struct FinalizationEngine {
    /// Prevents re-finalizing the exact same proposal.
    finalized_proposals: BTreeSet<[u8; 32]>,
    /// Prevents any second proposal for the same (slot, epoch) from finalizing.
    /// Value is the winning proposal_id for audit purposes.
    finalized_slots: BTreeMap<(u64, u64), [u8; 32]>,
}

impl FinalizationEngine {
    pub fn new() -> Self {
        FinalizationEngine {
            finalized_proposals: BTreeSet::new(),
            finalized_slots: BTreeMap::new(),
        }
    }

    /// Attempt to finalize a proposed batch given its attestation collector.
    pub fn finalize(
        &mut self,
        proposed: &ProposedBatch,
        collector: &AttestationCollector,
        threshold: &AttestationThreshold,
        committee_size: usize,
        height: u64,
    ) -> FinalizationDecision {
        // Already finalized? (same proposal)
        if self.finalized_proposals.contains(&proposed.proposal_id) {
            return FinalizationDecision::AlreadyFinalized;
        }

        // FIND-012: Check if a *different* proposal already won this (slot, epoch).
        // This enforces the invariant: one canonical finalized batch per slot.
        let slot_key = (proposed.slot, proposed.epoch);
        if let Some(&existing_proposal_id) = self.finalized_slots.get(&slot_key) {
            return FinalizationDecision::SlotAlreadyFinalized {
                slot: proposed.slot,
                epoch: proposed.epoch,
                existing_proposal_id,
            };
        }

        // Conflict check
        if collector.has_conflicts() {
            return FinalizationDecision::ConflictDetected(
                format!("{} conflicting votes detected", collector.conflicts.len())
            );
        }

        // Quorum check
        let quorum = collector.check_quorum(threshold, committee_size);
        if !quorum.reached {
            return FinalizationDecision::InsufficientAttestations;
        }

        // Compute finalization hash
        let finalization_hash = FinalizedBatch::compute_finalization_hash(
            &proposed.proposal_id,
            proposed.slot,
            proposed.epoch,
            &proposed.leader_id,
            &proposed.batch_root,
            &proposed.parent_batch_id,
            height,
            &quorum.quorum_hash,
        );

        let batch = FinalizedBatch {
            batch_id: finalization_hash,
            proposal_id: proposed.proposal_id,
            slot: proposed.slot,
            epoch: proposed.epoch,
            leader_id: proposed.leader_id,
            ordered_submission_ids: proposed.ordered_submission_ids.clone(),
            batch_root: proposed.batch_root,
            parent_batch_id: proposed.parent_batch_id,
            finalized_at_height: height,
            quorum_summary: quorum,
            finalization_hash,
        };

        self.finalized_proposals.insert(proposed.proposal_id);
        self.finalized_slots.insert(slot_key, proposed.proposal_id);
        FinalizationDecision::Finalized(batch)
    }
}

impl Default for FinalizationEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::proposals::batch::ProposedBatch;
    use crate::attestations::collector::{AttestationCollector, AttestationThreshold, BatchAttestationVote};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_batch() -> ProposedBatch {
        ProposedBatch::new(
            1, 1,
            make_id(10),
            vec![make_id(1), make_id(2)],
            [0u8; 32],
            1,
            100,
        )
    }

    fn make_collector_with_votes(proposal_id: [u8; 32], count: usize, approved: bool) -> AttestationCollector {
        let mut c = AttestationCollector::new(proposal_id);
        for i in 1..=(count as u8) {
            let v = BatchAttestationVote::new(proposal_id, make_id(i), approved, 1);
            let _ = c.add_vote(v);
        }
        c
    }

    #[test]
    fn test_finalization_succeeds_with_quorum() {
        let batch = make_batch();
        let collector = make_collector_with_votes(batch.proposal_id, 4, true);
        let threshold = AttestationThreshold::two_thirds(5);
        let mut engine = FinalizationEngine::new();
        match engine.finalize(&batch, &collector, &threshold, 5, 101) {
            FinalizationDecision::Finalized(fb) => {
                assert_eq!(fb.proposal_id, batch.proposal_id);
                assert_eq!(fb.finalized_at_height, 101);
            }
            other => panic!("Expected Finalized, got {:?}", other),
        }
    }

    #[test]
    fn test_finalization_fails_without_quorum() {
        let batch = make_batch();
        let collector = make_collector_with_votes(batch.proposal_id, 1, true);
        let threshold = AttestationThreshold::two_thirds(6);
        let mut engine = FinalizationEngine::new();
        match engine.finalize(&batch, &collector, &threshold, 6, 101) {
            FinalizationDecision::InsufficientAttestations => {}
            other => panic!("Expected InsufficientAttestations, got {:?}", other),
        }
    }

    #[test]
    fn test_already_finalized_on_second_call() {
        let batch = make_batch();
        let collector = make_collector_with_votes(batch.proposal_id, 4, true);
        let threshold = AttestationThreshold::two_thirds(5);
        let mut engine = FinalizationEngine::new();
        // First call succeeds
        assert!(matches!(
            engine.finalize(&batch, &collector, &threshold, 5, 101),
            FinalizationDecision::Finalized(_)
        ));
        // Second call returns AlreadyFinalized
        assert!(matches!(
            engine.finalize(&batch, &collector, &threshold, 5, 101),
            FinalizationDecision::AlreadyFinalized
        ));
    }

    #[test]
    fn test_conflict_detected_blocks_finalization() {
        let batch = make_batch();
        let proposal_id = batch.proposal_id;
        let mut collector = AttestationCollector::new(proposal_id);
        // Add approve then reject from same attestor
        let v1 = BatchAttestationVote::new(proposal_id, make_id(1), true, 1);
        let v2 = BatchAttestationVote::new(proposal_id, make_id(1), false, 1);
        collector.add_vote(v1).unwrap();
        let _ = collector.add_vote(v2); // this introduces conflict
        let threshold = AttestationThreshold::two_thirds(3);
        let mut engine = FinalizationEngine::new();
        match engine.finalize(&batch, &collector, &threshold, 3, 101) {
            FinalizationDecision::ConflictDetected(_) => {}
            other => panic!("Expected ConflictDetected, got {:?}", other),
        }
    }

    #[test]
    fn test_two_different_proposals_for_same_slot_epoch_second_is_rejected() {
        // FIND-012: only one canonical batch per (slot, epoch)
        let batch_a = ProposedBatch::new(1, 1, make_id(10), vec![make_id(1)], [0u8; 32], 1, 100);
        // Different leader → different proposal_id, same slot/epoch
        let batch_b = ProposedBatch::new(1, 1, make_id(11), vec![make_id(2)], [0u8; 32], 1, 101);

        let collector_a = make_collector_with_votes(batch_a.proposal_id, 4, true);
        let collector_b = make_collector_with_votes(batch_b.proposal_id, 4, true);
        let threshold = AttestationThreshold::two_thirds(5);
        let mut engine = FinalizationEngine::new();

        // First proposal finalizes
        assert!(matches!(
            engine.finalize(&batch_a, &collector_a, &threshold, 5, 101),
            FinalizationDecision::Finalized(_)
        ));

        // Second proposal for the same slot/epoch must be rejected
        assert!(matches!(
            engine.finalize(&batch_b, &collector_b, &threshold, 5, 102),
            FinalizationDecision::SlotAlreadyFinalized { slot: 1, epoch: 1, .. }
        ));
    }

    #[test]
    fn test_finalization_hash_determinism() {
        let h1 = FinalizedBatch::compute_finalization_hash(
            &[1u8; 32], 5, 1, &[2u8; 32], &[3u8; 32], &[0u8; 32], 100, &[4u8; 32],
        );
        let h2 = FinalizedBatch::compute_finalization_hash(
            &[1u8; 32], 5, 1, &[2u8; 32], &[3u8; 32], &[0u8; 32], 100, &[4u8; 32],
        );
        assert_eq!(h1, h2);
    }
}
