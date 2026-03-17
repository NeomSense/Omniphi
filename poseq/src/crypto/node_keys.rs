#![allow(dead_code)]

use sha2::{Sha256, Digest};

use crate::crypto::signer::{Ed25519Signer, SignatureEnvelope};
use crate::crypto::payloads::{ProposalPayload, AttestationPayload, EvidencePayload, FinalizedBatchPayload};
use crate::errors::Phase4Error;

/// A node's ed25519 key-pair with convenient per-payload-type signing helpers.
pub struct NodeKeyPair {
    pub node_id: [u8; 32],
    pub public_key_bytes: [u8; 32],
    signer: Ed25519Signer,
}

impl NodeKeyPair {
    /// Create from a 32-byte secret seed. The public key is derived deterministically.
    pub fn from_seed(node_id: [u8; 32], seed: [u8; 32]) -> Self {
        let signer = Ed25519Signer::from_seed(node_id, seed);
        let public_key_bytes = signer.identity.public_key_bytes;
        NodeKeyPair { node_id, public_key_bytes, signer }
    }

    /// Generate a deterministic test key-pair from `node_id`.
    /// Seed = SHA256("TEST_SEED" || node_id). **Not for production.**
    pub fn for_testing(node_id: [u8; 32]) -> Self {
        let mut h = Sha256::new();
        h.update(b"TEST_SEED");
        h.update(&node_id);
        let r = h.finalize();
        let mut seed = [0u8; 32];
        seed.copy_from_slice(&r);
        Self::from_seed(node_id, seed)
    }

    pub fn sign_proposal(&self, payload: &ProposalPayload) -> Result<SignatureEnvelope, Phase4Error> {
        let hash = payload.to_payload_hash();
        self.signer.sign(&hash)
    }

    pub fn sign_attestation(&self, payload: &AttestationPayload) -> Result<SignatureEnvelope, Phase4Error> {
        let hash = payload.to_payload_hash();
        self.signer.sign(&hash)
    }

    pub fn sign_evidence(&self, payload: &EvidencePayload) -> Result<SignatureEnvelope, Phase4Error> {
        let hash = payload.to_payload_hash();
        self.signer.sign(&hash)
    }

    pub fn sign_finalized(&self, payload: &FinalizedBatchPayload) -> Result<SignatureEnvelope, Phase4Error> {
        let hash = payload.to_payload_hash();
        self.signer.sign(&hash)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::signer::{Ed25519Verifier, SignatureVerifier};

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn node(b: u8) -> NodeKeyPair {
        NodeKeyPair::for_testing(make_id(b))
    }

    // ── proposal round-trip ─────────────────────────────────────────────────

    #[test]
    fn test_sign_verify_proposal() {
        let kp = node(1);
        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let env = kp.sign_proposal(&payload).unwrap();
        let verifier = Ed25519Verifier::new(kp.public_key_bytes).unwrap();
        let hash = payload.to_payload_hash();
        assert!(verifier.verify(&hash, &env));
    }

    #[test]
    fn test_sign_proposal_is_deterministic() {
        let kp = node(1);
        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let e1 = kp.sign_proposal(&payload).unwrap();
        let e2 = kp.sign_proposal(&payload).unwrap();
        assert_eq!(e1.signature_bytes, e2.signature_bytes);
    }

    // ── attestation round-trip ──────────────────────────────────────────────

    #[test]
    fn test_sign_verify_attestation() {
        let kp = node(2);
        let payload = AttestationPayload {
            attestor_id: kp.node_id,
            slot: 7,
            epoch: 1,
            batch_id: make_id(20),
            vote_accept: true,
        };
        let env = kp.sign_attestation(&payload).unwrap();
        let verifier = Ed25519Verifier::new(kp.public_key_bytes).unwrap();
        let hash = payload.to_payload_hash();
        assert!(verifier.verify(&hash, &env));
    }

    // ── evidence round-trip ─────────────────────────────────────────────────

    #[test]
    fn test_sign_verify_evidence() {
        let kp = node(3);
        let payload = EvidencePayload {
            reporter_id: kp.node_id,
            accused_id: make_id(99),
            slot_a: 5,
            epoch_a: 1,
            batch_id_a: make_id(30),
            slot_b: 6,
            epoch_b: 1,
            batch_id_b: make_id(31),
        };
        let env = kp.sign_evidence(&payload).unwrap();
        let verifier = Ed25519Verifier::new(kp.public_key_bytes).unwrap();
        let hash = payload.to_payload_hash();
        assert!(verifier.verify(&hash, &env));
    }

    // ── finalized round-trip ────────────────────────────────────────────────

    #[test]
    fn test_sign_verify_finalized() {
        let kp = node(4);
        let payload = FinalizedBatchPayload {
            finalizer_id: kp.node_id,
            slot: 9,
            epoch: 2,
            batch_id: make_id(50),
            finalization_hash: make_id(77),
        };
        let env = kp.sign_finalized(&payload).unwrap();
        let verifier = Ed25519Verifier::new(kp.public_key_bytes).unwrap();
        let hash = payload.to_payload_hash();
        assert!(verifier.verify(&hash, &env));
    }

    // ── cross-payload type separation ────────────────────────────────────────

    #[test]
    fn test_proposal_and_attestation_envelopes_differ() {
        let kp = node(5);
        let prop_payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 1,
            epoch: 1,
            batch_root: make_id(1),
            submission_count: 1,
        };
        let att_payload = AttestationPayload {
            attestor_id: kp.node_id,
            slot: 1,
            epoch: 1,
            batch_id: make_id(1),
            vote_accept: true,
        };
        let e1 = kp.sign_proposal(&prop_payload).unwrap();
        let e2 = kp.sign_attestation(&att_payload).unwrap();
        assert_ne!(e1.payload_hash, e2.payload_hash);
        assert_ne!(e1.signature_bytes, e2.signature_bytes);
    }
}
