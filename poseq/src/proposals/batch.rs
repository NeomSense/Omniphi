use std::collections::BTreeMap;
use sha2::{Sha256, Digest};
use crate::crypto::signer::SignatureEnvelope;

/// Lifecycle state of a batch proposal.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProposalState {
    Pending,
    Collecting,
    Attested,
    Finalized,
    Expired,
    Conflicted,
}

/// Compact header used for ID computation and broadcast.
#[derive(Debug, Clone)]
pub struct ProposalHeader {
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub batch_root: [u8; 32],
    pub parent_batch_id: [u8; 32],
    pub policy_version: u32,
}

/// A proposed batch waiting for attestations before finalization.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProposedBatch {
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: [u8; 32],
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub batch_root: [u8; 32],
    pub parent_batch_id: [u8; 32],
    pub policy_version: u32,
    pub created_at_height: u64,
    pub state: ProposalState,
    pub metadata: BTreeMap<String, String>,
}

impl ProposedBatch {
    /// SHA256 of submission IDs in their canonical proposed order.
    /// This is an ordering commitment: the same set in a different order
    /// produces a different root, making the batch root tamper-evident.
    pub fn compute_root(ids: &[[u8; 32]]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for id in ids {
            hasher.update(id);
        }
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// SHA256 of sorted submission IDs — order-independent set commitment.
    /// Use this when you need to check set equality regardless of order.
    pub fn compute_set_root(ids: &[[u8; 32]]) -> [u8; 32] {
        let mut sorted = ids.to_vec();
        sorted.sort();
        let mut hasher = Sha256::new();
        for id in &sorted {
            hasher.update(id);
        }
        let result = hasher.finalize();
        let mut root = [0u8; 32];
        root.copy_from_slice(&result);
        root
    }

    /// SHA256 of all ProposalHeader fields in a canonical byte order.
    pub fn compute_id(header: &ProposalHeader) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&header.slot.to_be_bytes());
        hasher.update(&header.epoch.to_be_bytes());
        hasher.update(&header.leader_id);
        hasher.update(&header.batch_root);
        hasher.update(&header.parent_batch_id);
        hasher.update(&header.policy_version.to_be_bytes());
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Build a new ProposedBatch, computing root and ID deterministically.
    pub fn new(
        slot: u64,
        epoch: u64,
        leader_id: [u8; 32],
        ordered_submission_ids: Vec<[u8; 32]>,
        parent_batch_id: [u8; 32],
        policy_version: u32,
        created_at_height: u64,
    ) -> Self {
        let batch_root = Self::compute_root(&ordered_submission_ids);
        let header = ProposalHeader {
            proposal_id: [0u8; 32], // placeholder for ID computation
            slot,
            epoch,
            leader_id,
            batch_root,
            parent_batch_id,
            policy_version,
        };
        let proposal_id = Self::compute_id(&header);
        ProposedBatch {
            proposal_id,
            slot,
            epoch,
            leader_id,
            ordered_submission_ids,
            batch_root,
            parent_batch_id,
            policy_version,
            created_at_height,
            state: ProposalState::Pending,
            metadata: BTreeMap::new(),
        }
    }

    /// Advance to Collecting state.
    pub fn start_collecting(&mut self) -> bool {
        if self.state == ProposalState::Pending {
            self.state = ProposalState::Collecting;
            true
        } else {
            false
        }
    }

    /// Mark as Attested (quorum of approval votes received).
    pub fn mark_attested(&mut self) -> bool {
        if self.state == ProposalState::Collecting {
            self.state = ProposalState::Attested;
            true
        } else {
            false
        }
    }

    /// Mark as Finalized.
    pub fn mark_finalized(&mut self) -> bool {
        if matches!(self.state, ProposalState::Attested | ProposalState::Collecting) {
            self.state = ProposalState::Finalized;
            true
        } else {
            false
        }
    }

    /// Mark as Expired.
    pub fn mark_expired(&mut self) -> bool {
        if matches!(self.state, ProposalState::Pending | ProposalState::Collecting) {
            self.state = ProposalState::Expired;
            true
        } else {
            false
        }
    }

    /// Mark as Conflicted.
    ///
    /// FIND-014: only allowed from pre-finalization states (Pending, Collecting, Attested).
    /// A conflict claim against a Finalized batch is evidence of equivocation and must be
    /// routed to the misbehavior module — it must NOT mutate the finalized batch's state.
    /// Returns `true` if the transition was applied, `false` if rejected.
    pub fn mark_conflicted(&mut self) -> bool {
        match self.state {
            ProposalState::Pending | ProposalState::Collecting | ProposalState::Attested => {
                self.state = ProposalState::Conflicted;
                true
            }
            // Finalized, Expired, and already-Conflicted are all guarded.
            _ => false,
        }
    }

    pub fn header(&self) -> ProposalHeader {
        ProposalHeader {
            proposal_id: self.proposal_id,
            slot: self.slot,
            epoch: self.epoch,
            leader_id: self.leader_id,
            batch_root: self.batch_root,
            parent_batch_id: self.parent_batch_id,
            policy_version: self.policy_version,
        }
    }
}

/// Full batch proposal wrapper (for broadcast / network serialization).
#[derive(Debug, Clone)]
pub struct BatchProposal {
    pub header: ProposalHeader,
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub created_at_height: u64,
    /// Optional ed25519 leader signature over the proposal payload.
    pub leader_signature: Option<SignatureEnvelope>,
}

impl BatchProposal {
    pub fn from_proposed(batch: &ProposedBatch) -> Self {
        BatchProposal {
            header: batch.header(),
            ordered_submission_ids: batch.ordered_submission_ids.clone(),
            created_at_height: batch.created_at_height,
            leader_signature: None,
        }
    }

    /// Sign this proposal as leader. Computes the ProposalPayload from `batch_root` and
    /// sets `leader_signature`.
    pub fn sign(
        &mut self,
        key: &crate::crypto::node_keys::NodeKeyPair,
        batch_root: [u8; 32],
    ) -> Result<(), crate::errors::Phase4Error> {
        let payload = crate::crypto::payloads::ProposalPayload {
            leader_id: key.node_id,
            slot: self.header.slot,
            epoch: self.header.epoch,
            batch_root,
            submission_count: self.ordered_submission_ids.len() as u32,
        };
        let envelope = key.sign_proposal(&payload)?;
        self.leader_signature = Some(envelope);
        Ok(())
    }

    /// Verify the leader signature, if present.
    /// Returns `Ok(())` when the proposal is unsigned (backward-compatible) or when the
    /// signature is valid.
    pub fn verify_signature(
        &self,
        verifier: &crate::crypto::verifier::PoSeqVerifier,
        batch_root: [u8; 32],
    ) -> Result<(), crate::crypto::verifier::VerificationError> {
        let env = match &self.leader_signature {
            Some(e) => e,
            None => return Ok(()), // unsigned — allowed for backward compat
        };
        let payload = crate::crypto::payloads::ProposalPayload {
            leader_id: env.signer_id,
            slot: self.header.slot,
            epoch: self.header.epoch,
            batch_root,
            submission_count: self.ordered_submission_ids.len() as u32,
        };
        verifier.verify_proposal(&payload, env)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_batch(slot: u64) -> ProposedBatch {
        ProposedBatch::new(
            slot,
            1,
            make_id(99),
            vec![make_id(1), make_id(2), make_id(3)],
            [0u8; 32],
            1,
            100,
        )
    }

    #[test]
    fn test_compute_root_determinism() {
        let ids = vec![make_id(3), make_id(1), make_id(2)];
        let r1 = ProposedBatch::compute_root(&ids);
        let r2 = ProposedBatch::compute_root(&ids);
        assert_eq!(r1, r2, "same order must produce same root");
    }

    #[test]
    fn test_compute_root_differs_for_different_orderings_of_same_ids() {
        // FIND-005: ordering commitment — different order must differ
        let order_a = vec![make_id(1), make_id(2), make_id(3)];
        let order_b = vec![make_id(3), make_id(1), make_id(2)];
        assert_ne!(
            ProposedBatch::compute_root(&order_a),
            ProposedBatch::compute_root(&order_b),
            "batch_root must commit to ordering, not just set membership"
        );
    }

    #[test]
    fn test_compute_set_root_is_order_independent() {
        let ids = vec![make_id(3), make_id(1), make_id(2)];
        let r1 = ProposedBatch::compute_set_root(&ids);
        let ids2 = vec![make_id(1), make_id(2), make_id(3)];
        let r2 = ProposedBatch::compute_set_root(&ids2);
        assert_eq!(r1, r2, "set root must be order-independent");
    }

    #[test]
    fn test_compute_root_changes_with_different_ids() {
        let r1 = ProposedBatch::compute_root(&[make_id(1), make_id(2)]);
        let r2 = ProposedBatch::compute_root(&[make_id(1), make_id(3)]);
        assert_ne!(r1, r2);
    }

    #[test]
    fn test_compute_id_determinism() {
        let header = ProposalHeader {
            proposal_id: [0u8; 32],
            slot: 5,
            epoch: 1,
            leader_id: make_id(10),
            batch_root: [1u8; 32],
            parent_batch_id: [0u8; 32],
            policy_version: 1,
        };
        let id1 = ProposedBatch::compute_id(&header);
        let id2 = ProposedBatch::compute_id(&header);
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_compute_id_changes_with_different_fields() {
        let mut header = ProposalHeader {
            proposal_id: [0u8; 32],
            slot: 5,
            epoch: 1,
            leader_id: make_id(10),
            batch_root: [1u8; 32],
            parent_batch_id: [0u8; 32],
            policy_version: 1,
        };
        let id1 = ProposedBatch::compute_id(&header);
        header.slot = 6;
        let id2 = ProposedBatch::compute_id(&header);
        assert_ne!(id1, id2);
    }

    #[test]
    fn test_proposal_state_transitions() {
        let mut batch = make_batch(1);
        assert_eq!(batch.state, ProposalState::Pending);

        assert!(batch.start_collecting());
        assert_eq!(batch.state, ProposalState::Collecting);

        assert!(batch.mark_attested());
        assert_eq!(batch.state, ProposalState::Attested);

        assert!(batch.mark_finalized());
        assert_eq!(batch.state, ProposalState::Finalized);
    }

    #[test]
    fn test_invalid_state_transition_rejected() {
        let mut batch = make_batch(1);
        // Cannot attest from Pending
        assert!(!batch.mark_attested());
        assert_eq!(batch.state, ProposalState::Pending);
    }

    #[test]
    fn test_expired_transition() {
        let mut batch = make_batch(1);
        assert!(batch.mark_expired());
        assert_eq!(batch.state, ProposalState::Expired);
    }

    #[test]
    fn test_conflicted_transition() {
        let mut batch = make_batch(1);
        assert!(batch.mark_conflicted());
        assert_eq!(batch.state, ProposalState::Conflicted);
    }

    #[test]
    fn test_mark_conflicted_on_finalized_batch_is_rejected() {
        let mut batch = make_batch(1);
        batch.start_collecting();
        batch.mark_attested();
        batch.mark_finalized();
        assert!(!batch.mark_conflicted(), "Finalized batch must not be conflicted");
        assert_eq!(batch.state, ProposalState::Finalized);
    }

    #[test]
    fn test_mark_conflicted_on_expired_batch_is_rejected() {
        let mut batch = make_batch(1);
        batch.mark_expired();
        assert!(!batch.mark_conflicted());
        assert_eq!(batch.state, ProposalState::Expired);
    }
}
