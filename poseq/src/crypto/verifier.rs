#![allow(dead_code)]

use std::collections::BTreeMap;

use crate::crypto::signer::{Ed25519Verifier, SignatureEnvelope, SignatureVerifier};
use crate::crypto::payloads::{ProposalPayload, AttestationPayload, EvidencePayload, FinalizedBatchPayload};
use crate::errors::Phase4Error;

/// Errors that can arise when verifying a PoSeq message signature.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum VerificationError {
    /// The signer_id in the envelope is not registered.
    UnknownSigner([u8; 32]),
    /// The cryptographic signature is invalid.
    SignatureMismatch,
    /// The payload hash in the envelope does not match the recomputed hash.
    PayloadMismatch,
    /// The epoch is too old.
    StaleEpoch { expected_min: u64, got: u64 },
    /// The envelope's signer_id does not match the expected field (e.g. leader_id).
    WrongSigner { expected: [u8; 32], got: [u8; 32] },
}

/// Verifies PoSeq message signatures using a registry of known node public keys.
pub struct PoSeqVerifier {
    verifiers: BTreeMap<[u8; 32], Ed25519Verifier>,
}

impl PoSeqVerifier {
    pub fn new() -> Self {
        PoSeqVerifier { verifiers: BTreeMap::new() }
    }

    /// Register a node's public key. Returns an error if the public key bytes are invalid.
    pub fn register(&mut self, node_id: [u8; 32], public_key_bytes: [u8; 32]) -> Result<(), Phase4Error> {
        let v = Ed25519Verifier::new(public_key_bytes)?;
        self.verifiers.insert(node_id, v);
        Ok(())
    }

    // ── Internal helper ──────────────────────────────────────────────────────

    fn get_verifier(&self, node_id: &[u8; 32]) -> Result<&Ed25519Verifier, VerificationError> {
        self.verifiers.get(node_id).ok_or(VerificationError::UnknownSigner(*node_id))
    }

    fn check_sig(
        &self,
        signer_id: &[u8; 32],
        payload_hash: &[u8; 32],
        envelope: &SignatureEnvelope,
    ) -> Result<(), VerificationError> {
        let v = self.get_verifier(signer_id)?;
        if envelope.payload_hash != *payload_hash {
            return Err(VerificationError::PayloadMismatch);
        }
        if !v.verify(payload_hash, envelope) {
            return Err(VerificationError::SignatureMismatch);
        }
        Ok(())
    }

    // ── Public verify methods ────────────────────────────────────────────────

    /// Verify a proposal envelope. The envelope's `signer_id` must equal `payload.leader_id`.
    pub fn verify_proposal(
        &self,
        payload: &ProposalPayload,
        envelope: &SignatureEnvelope,
    ) -> Result<(), VerificationError> {
        // Check that the signer claims to be the leader.
        if envelope.signer_id != payload.leader_id {
            return Err(VerificationError::WrongSigner {
                expected: payload.leader_id,
                got: envelope.signer_id,
            });
        }
        let hash = payload.to_payload_hash();
        self.check_sig(&envelope.signer_id, &hash, envelope)
    }

    /// Verify an attestation envelope. The envelope's `signer_id` must equal `payload.attestor_id`.
    pub fn verify_attestation(
        &self,
        payload: &AttestationPayload,
        envelope: &SignatureEnvelope,
    ) -> Result<(), VerificationError> {
        if envelope.signer_id != payload.attestor_id {
            return Err(VerificationError::WrongSigner {
                expected: payload.attestor_id,
                got: envelope.signer_id,
            });
        }
        let hash = payload.to_payload_hash();
        self.check_sig(&envelope.signer_id, &hash, envelope)
    }

    /// Verify an evidence envelope. The envelope's `signer_id` must equal `payload.reporter_id`.
    pub fn verify_evidence(
        &self,
        payload: &EvidencePayload,
        envelope: &SignatureEnvelope,
    ) -> Result<(), VerificationError> {
        if envelope.signer_id != payload.reporter_id {
            return Err(VerificationError::WrongSigner {
                expected: payload.reporter_id,
                got: envelope.signer_id,
            });
        }
        let hash = payload.to_payload_hash();
        self.check_sig(&envelope.signer_id, &hash, envelope)
    }

    /// Verify a finalized-batch envelope. The envelope's `signer_id` must equal `payload.finalizer_id`.
    pub fn verify_finalized(
        &self,
        payload: &FinalizedBatchPayload,
        envelope: &SignatureEnvelope,
    ) -> Result<(), VerificationError> {
        if envelope.signer_id != payload.finalizer_id {
            return Err(VerificationError::WrongSigner {
                expected: payload.finalizer_id,
                got: envelope.signer_id,
            });
        }
        let hash = payload.to_payload_hash();
        self.check_sig(&envelope.signer_id, &hash, envelope)
    }
}

impl Default for PoSeqVerifier {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::node_keys::NodeKeyPair;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn node(b: u8) -> NodeKeyPair {
        NodeKeyPair::for_testing(make_id(b))
    }

    fn register(v: &mut PoSeqVerifier, kp: &NodeKeyPair) {
        v.register(kp.node_id, kp.public_key_bytes).unwrap();
    }

    // ── Proposal ────────────────────────────────────────────────────────────

    #[test]
    fn test_verify_proposal_valid() {
        let kp = node(1);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let env = kp.sign_proposal(&payload).unwrap();
        assert_eq!(v.verify_proposal(&payload, &env), Ok(()));
    }

    #[test]
    fn test_verify_proposal_tampered_signature() {
        let kp = node(1);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let mut env = kp.sign_proposal(&payload).unwrap();
        env.signature_bytes[0] ^= 0xFF;
        assert_eq!(v.verify_proposal(&payload, &env), Err(VerificationError::SignatureMismatch));
    }

    #[test]
    fn test_verify_proposal_unknown_signer() {
        let kp = node(1);
        let v = PoSeqVerifier::new(); // kp not registered

        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let env = kp.sign_proposal(&payload).unwrap();
        assert_eq!(v.verify_proposal(&payload, &env), Err(VerificationError::UnknownSigner(kp.node_id)));
    }

    #[test]
    fn test_verify_proposal_wrong_signer() {
        // Node B signs a payload that claims leader_id = A.
        let kp_a = node(1);
        let kp_b = node(2);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp_a);
        register(&mut v, &kp_b);

        let payload = ProposalPayload {
            leader_id: kp_a.node_id, // claims to be A
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        // B signs it (envelope.signer_id = B)
        let env = kp_b.sign_proposal(&payload).unwrap();
        assert_eq!(
            v.verify_proposal(&payload, &env),
            Err(VerificationError::WrongSigner { expected: kp_a.node_id, got: kp_b.node_id })
        );
    }

    #[test]
    fn test_verify_proposal_payload_mismatch() {
        let kp = node(1);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 5,
            epoch: 1,
            batch_root: make_id(10),
            submission_count: 3,
        };
        let env = kp.sign_proposal(&payload).unwrap();
        // Verify against a different payload (different slot)
        let mut payload2 = payload.clone();
        payload2.slot = 99;
        assert_eq!(v.verify_proposal(&payload2, &env), Err(VerificationError::PayloadMismatch));
    }

    // ── Attestation ─────────────────────────────────────────────────────────

    #[test]
    fn test_verify_attestation_valid() {
        let kp = node(2);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = AttestationPayload {
            attestor_id: kp.node_id,
            slot: 7,
            epoch: 1,
            batch_id: make_id(20),
            vote_accept: true,
        };
        let env = kp.sign_attestation(&payload).unwrap();
        assert_eq!(v.verify_attestation(&payload, &env), Ok(()));
    }

    #[test]
    fn test_verify_attestation_tampered() {
        let kp = node(2);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = AttestationPayload {
            attestor_id: kp.node_id,
            slot: 7,
            epoch: 1,
            batch_id: make_id(20),
            vote_accept: true,
        };
        let mut env = kp.sign_attestation(&payload).unwrap();
        env.signature_bytes[5] ^= 0xAB;
        assert_eq!(v.verify_attestation(&payload, &env), Err(VerificationError::SignatureMismatch));
    }

    // ── Evidence ────────────────────────────────────────────────────────────

    #[test]
    fn test_verify_evidence_valid() {
        let kp = node(3);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = EvidencePayload {
            reporter_id: kp.node_id,
            accused_id: make_id(99),
            slot_a: 5, epoch_a: 1, batch_id_a: make_id(30),
            slot_b: 6, epoch_b: 1, batch_id_b: make_id(31),
        };
        let env = kp.sign_evidence(&payload).unwrap();
        assert_eq!(v.verify_evidence(&payload, &env), Ok(()));
    }

    #[test]
    fn test_verify_evidence_unknown_signer() {
        let kp = node(3);
        let v = PoSeqVerifier::new();

        let payload = EvidencePayload {
            reporter_id: kp.node_id,
            accused_id: make_id(99),
            slot_a: 5, epoch_a: 1, batch_id_a: make_id(30),
            slot_b: 6, epoch_b: 1, batch_id_b: make_id(31),
        };
        let env = kp.sign_evidence(&payload).unwrap();
        assert_eq!(v.verify_evidence(&payload, &env), Err(VerificationError::UnknownSigner(kp.node_id)));
    }

    // ── FinalizedBatch ──────────────────────────────────────────────────────

    #[test]
    fn test_verify_finalized_valid() {
        let kp = node(4);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let payload = FinalizedBatchPayload {
            finalizer_id: kp.node_id,
            slot: 9,
            epoch: 2,
            batch_id: make_id(50),
            finalization_hash: make_id(77),
        };
        let env = kp.sign_finalized(&payload).unwrap();
        assert_eq!(v.verify_finalized(&payload, &env), Ok(()));
    }

    #[test]
    fn test_verify_finalized_wrong_signer() {
        let kp_a = node(4);
        let kp_b = node(5);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp_a);
        register(&mut v, &kp_b);

        let payload = FinalizedBatchPayload {
            finalizer_id: kp_a.node_id,
            slot: 9,
            epoch: 2,
            batch_id: make_id(50),
            finalization_hash: make_id(77),
        };
        let env = kp_b.sign_finalized(&payload).unwrap();
        assert_eq!(
            v.verify_finalized(&payload, &env),
            Err(VerificationError::WrongSigner { expected: kp_a.node_id, got: kp_b.node_id })
        );
    }

    // ── Cross-type rejection ─────────────────────────────────────────────────

    #[test]
    fn test_proposal_sig_rejected_as_attestation() {
        // A valid proposal signature must not validate as an attestation signature
        // (payload hashes differ due to domain tags).
        let kp = node(6);
        let mut v = PoSeqVerifier::new();
        register(&mut v, &kp);

        let prop_payload = ProposalPayload {
            leader_id: kp.node_id,
            slot: 1,
            epoch: 1,
            batch_root: make_id(1),
            submission_count: 1,
        };
        let prop_env = kp.sign_proposal(&prop_payload).unwrap();

        // Build an attestation payload that has the same signer_id.
        let att_payload = AttestationPayload {
            attestor_id: kp.node_id,
            slot: 1,
            epoch: 1,
            batch_id: make_id(1),
            vote_accept: true,
        };
        // Use the proposal envelope to verify against an attestation payload — must fail.
        let result = v.verify_attestation(&att_payload, &prop_env);
        assert!(result.is_err(), "proposal sig must not pass as attestation");
    }
}
