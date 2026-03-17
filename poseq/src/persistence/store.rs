use std::collections::BTreeMap;
use crate::proposals::batch::ProposedBatch;
use crate::finalization::engine::FinalizedBatch;
use crate::attestations::collector::AttestationCollector;
use crate::conflicts::detector::EquivocationIncident;

/// Trait defining all storage operations for the batch lifecycle.
pub trait BatchLifecycleStore {
    fn store_proposed_batch(&mut self, batch: ProposedBatch);
    fn get_proposed_batch(&self, proposal_id: &[u8; 32]) -> Option<&ProposedBatch>;
    fn store_finalized_batch(&mut self, batch: FinalizedBatch);
    fn get_finalized_batch(&self, batch_id: &[u8; 32]) -> Option<&FinalizedBatch>;
    fn store_attestation_collector(&mut self, proposal_id: [u8; 32], collector: AttestationCollector);
    fn get_attestation_collector(&self, proposal_id: &[u8; 32]) -> Option<&AttestationCollector>;
    fn get_attestation_collector_mut(&mut self, proposal_id: &[u8; 32]) -> Option<&mut AttestationCollector>;
    fn mark_exported(&mut self, batch_id: [u8; 32]);
    fn mark_acked(&mut self, batch_id: [u8; 32], acked: bool);
    fn is_exported(&self, batch_id: &[u8; 32]) -> bool;
    fn is_acked(&self, batch_id: &[u8; 32]) -> bool;
    fn get_misbehavior_ledger(&self) -> &Vec<EquivocationIncident>;
    fn record_incident(&mut self, incident: EquivocationIncident);
}

/// In-memory BTreeMap-backed implementation of BatchLifecycleStore.
pub struct InMemoryBatchLifecycleStore {
    proposed_batches: BTreeMap<[u8; 32], ProposedBatch>,
    finalized_batches: BTreeMap<[u8; 32], FinalizedBatch>,
    attestation_collectors: BTreeMap<[u8; 32], AttestationCollector>,
    exported: BTreeMap<[u8; 32], bool>,
    acked: BTreeMap<[u8; 32], bool>,
    incidents: Vec<EquivocationIncident>,
}

impl InMemoryBatchLifecycleStore {
    pub fn new() -> Self {
        InMemoryBatchLifecycleStore {
            proposed_batches: BTreeMap::new(),
            finalized_batches: BTreeMap::new(),
            attestation_collectors: BTreeMap::new(),
            exported: BTreeMap::new(),
            acked: BTreeMap::new(),
            incidents: Vec::new(),
        }
    }
}

impl Default for InMemoryBatchLifecycleStore {
    fn default() -> Self {
        Self::new()
    }
}

impl BatchLifecycleStore for InMemoryBatchLifecycleStore {
    fn store_proposed_batch(&mut self, batch: ProposedBatch) {
        self.proposed_batches.insert(batch.proposal_id, batch);
    }

    fn get_proposed_batch(&self, proposal_id: &[u8; 32]) -> Option<&ProposedBatch> {
        self.proposed_batches.get(proposal_id)
    }

    fn store_finalized_batch(&mut self, batch: FinalizedBatch) {
        self.finalized_batches.insert(batch.batch_id, batch);
    }

    fn get_finalized_batch(&self, batch_id: &[u8; 32]) -> Option<&FinalizedBatch> {
        self.finalized_batches.get(batch_id)
    }

    fn store_attestation_collector(&mut self, proposal_id: [u8; 32], collector: AttestationCollector) {
        self.attestation_collectors.insert(proposal_id, collector);
    }

    fn get_attestation_collector(&self, proposal_id: &[u8; 32]) -> Option<&AttestationCollector> {
        self.attestation_collectors.get(proposal_id)
    }

    fn get_attestation_collector_mut(&mut self, proposal_id: &[u8; 32]) -> Option<&mut AttestationCollector> {
        self.attestation_collectors.get_mut(proposal_id)
    }

    fn mark_exported(&mut self, batch_id: [u8; 32]) {
        self.exported.insert(batch_id, true);
    }

    fn mark_acked(&mut self, batch_id: [u8; 32], acked: bool) {
        self.acked.insert(batch_id, acked);
    }

    fn is_exported(&self, batch_id: &[u8; 32]) -> bool {
        self.exported.get(batch_id).copied().unwrap_or(false)
    }

    fn is_acked(&self, batch_id: &[u8; 32]) -> bool {
        self.acked.get(batch_id).copied().unwrap_or(false)
    }

    fn get_misbehavior_ledger(&self) -> &Vec<EquivocationIncident> {
        &self.incidents
    }

    fn record_incident(&mut self, incident: EquivocationIncident) {
        self.incidents.push(incident);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::proposals::batch::ProposedBatch;
    use crate::finalization::engine::FinalizedBatch;
    use crate::attestations::collector::{AttestationCollector, AttestationQuorumResult};
    use crate::conflicts::detector::{EquivocationIncident, IncidentType};
    use std::collections::BTreeMap;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_proposed_batch() -> ProposedBatch {
        ProposedBatch::new(1, 1, make_id(10), vec![make_id(1)], [0u8; 32], 1, 100)
    }

    fn make_finalized_batch(proposal_id: [u8; 32]) -> FinalizedBatch {
        let qr = AttestationQuorumResult {
            reached: true,
            approvals: 3,
            rejections: 0,
            total_votes: 3,
            quorum_hash: [5u8; 32],
        };
        let fh = FinalizedBatch::compute_finalization_hash(
            &proposal_id, 1, 1, &make_id(10), &[1u8; 32], &[0u8; 32], 101, &[5u8; 32]
        );
        FinalizedBatch {
            batch_id: fh,
            proposal_id,
            slot: 1,
            epoch: 1,
            leader_id: make_id(10),
            ordered_submission_ids: vec![make_id(1)],
            batch_root: [1u8; 32],
            parent_batch_id: [0u8; 32],
            finalized_at_height: 101,
            quorum_summary: qr,
            finalization_hash: fh,
        }
    }

    #[test]
    fn test_store_and_retrieve_proposed_batch() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let batch = make_proposed_batch();
        let id = batch.proposal_id;
        store.store_proposed_batch(batch);
        assert!(store.get_proposed_batch(&id).is_some());
    }

    #[test]
    fn test_store_and_retrieve_finalized_batch() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let proposed = make_proposed_batch();
        let finalized = make_finalized_batch(proposed.proposal_id);
        let bid = finalized.batch_id;
        store.store_finalized_batch(finalized);
        assert!(store.get_finalized_batch(&bid).is_some());
    }

    #[test]
    fn test_mark_exported() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let bid = make_id(99);
        assert!(!store.is_exported(&bid));
        store.mark_exported(bid);
        assert!(store.is_exported(&bid));
    }

    #[test]
    fn test_mark_acked() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let bid = make_id(99);
        assert!(!store.is_acked(&bid));
        store.mark_acked(bid, true);
        assert!(store.is_acked(&bid));
    }

    #[test]
    fn test_missing_batch_returns_none() {
        let store = InMemoryBatchLifecycleStore::new();
        assert!(store.get_proposed_batch(&make_id(1)).is_none());
        assert!(store.get_finalized_batch(&make_id(1)).is_none());
    }

    #[test]
    fn test_record_incident() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let incident = EquivocationIncident::new(
            IncidentType::DualProposal,
            100,
            BTreeMap::new(),
        );
        store.record_incident(incident);
        assert_eq!(store.get_misbehavior_ledger().len(), 1);
    }

    #[test]
    fn test_store_and_retrieve_attestation_collector() {
        let mut store = InMemoryBatchLifecycleStore::new();
        let pid = make_id(5);
        let collector = AttestationCollector::new(pid);
        store.store_attestation_collector(pid, collector);
        assert!(store.get_attestation_collector(&pid).is_some());
    }
}
