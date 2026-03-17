use sha2::{Sha256, Digest};
use ed25519_dalek::{
    Signer as DalekSigner, Verifier as DalekVerifier,
    SigningKey, VerifyingKey, Signature,
};
use crate::errors::Phase4Error;

/// A binding of a public key to a node_id for identity verification.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct SignerIdentity {
    pub node_id: [u8; 32],
    pub public_key_bytes: [u8; 32],
}

impl SignerIdentity {
    pub fn new(node_id: [u8; 32], public_key_bytes: [u8; 32]) -> Self {
        SignerIdentity { node_id, public_key_bytes }
    }

    /// Compute the canonical identity hash: SHA256(node_id ‖ public_key_bytes).
    pub fn identity_hash(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&self.node_id);
        hasher.update(&self.public_key_bytes);
        let result = hasher.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&result);
        out
    }
}

/// Wraps a payload_hash with a 64-byte ed25519 signature and signer identity.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SignatureEnvelope {
    pub payload_hash: [u8; 32],
    /// Raw 64-byte ed25519 signature stored as two [u8; 32] halves concatenated.
    pub signature_bytes: [u8; 64],
    pub signer_id: [u8; 32],
}

impl SignatureEnvelope {
    pub fn new(payload_hash: [u8; 32], signature_bytes: [u8; 64], signer_id: [u8; 32]) -> Self {
        SignatureEnvelope { payload_hash, signature_bytes, signer_id }
    }
}

/// Trait for verifying signatures on payload hashes.
pub trait SignatureVerifier {
    fn verify(&self, payload_hash: &[u8; 32], envelope: &SignatureEnvelope) -> bool;
}

/// Real ed25519 signer backed by ed25519-dalek.
/// The signing key seed is a 32-byte secret; the public key is derived from it.
pub struct Ed25519Signer {
    pub identity: SignerIdentity,
    signing_key: SigningKey,
}

impl Ed25519Signer {
    /// Create a signer from a 32-byte seed (secret scalar).
    pub fn from_seed(node_id: [u8; 32], seed: [u8; 32]) -> Self {
        let signing_key = SigningKey::from_bytes(&seed);
        let verifying_key = signing_key.verifying_key();
        let public_key_bytes = verifying_key.to_bytes();
        Ed25519Signer {
            identity: SignerIdentity::new(node_id, public_key_bytes),
            signing_key,
        }
    }

    /// Sign a payload_hash and return a SignatureEnvelope.
    pub fn sign(&self, payload_hash: &[u8; 32]) -> Result<SignatureEnvelope, Phase4Error> {
        let sig: Signature = self.signing_key.sign(payload_hash.as_ref());
        let sig_bytes: [u8; 64] = sig.to_bytes();
        Ok(SignatureEnvelope::new(
            *payload_hash,
            sig_bytes,
            self.identity.node_id,
        ))
    }

    pub fn verifying_key(&self) -> VerifyingKey {
        self.signing_key.verifying_key()
    }
}

/// Verifies ed25519 signatures against a known public key.
pub struct Ed25519Verifier {
    pub public_key_bytes: [u8; 32],
    verifying_key: VerifyingKey,
}

impl Ed25519Verifier {
    pub fn new(public_key_bytes: [u8; 32]) -> Result<Self, Phase4Error> {
        let verifying_key = VerifyingKey::from_bytes(&public_key_bytes)
            .map_err(|_e| Phase4Error::InvalidSignature(public_key_bytes))?;
        Ok(Ed25519Verifier { public_key_bytes, verifying_key })
    }
}

impl SignatureVerifier for Ed25519Verifier {
    fn verify(&self, payload_hash: &[u8; 32], envelope: &SignatureEnvelope) -> bool {
        // Guard: payload_hash in envelope must match what we're verifying
        if envelope.payload_hash != *payload_hash {
            return false;
        }
        // In ed25519-dalek v2, Signature::from_bytes returns Signature directly (infallible).
        let sig = Signature::from_bytes(&envelope.signature_bytes);
        self.verifying_key
            .verify(payload_hash.as_ref(), &sig)
            .is_ok()
    }
}

/// Registry that maps node_id → Ed25519Verifier for cross-node verification.
pub struct Ed25519SignatureRegistry {
    verifiers: std::collections::BTreeMap<[u8; 32], Ed25519Verifier>,
}

impl Ed25519SignatureRegistry {
    pub fn new() -> Self {
        Ed25519SignatureRegistry { verifiers: std::collections::BTreeMap::new() }
    }

    pub fn register(&mut self, node_id: [u8; 32], public_key_bytes: [u8; 32]) -> Result<(), Phase4Error> {
        let verifier = Ed25519Verifier::new(public_key_bytes)?;
        self.verifiers.insert(node_id, verifier);
        Ok(())
    }

    pub fn verify(&self, node_id: &[u8; 32], payload_hash: &[u8; 32], envelope: &SignatureEnvelope) -> Result<bool, Phase4Error> {
        let verifier = self.verifiers.get(node_id)
            .ok_or(Phase4Error::UnknownSigner(*node_id))?;
        Ok(verifier.verify(payload_hash, envelope))
    }

    pub fn is_registered(&self, node_id: &[u8; 32]) -> bool {
        self.verifiers.contains_key(node_id)
    }
}

impl Default for Ed25519SignatureRegistry {
    fn default() -> Self {
        Self::new()
    }
}

// ─── Compatibility alias (old name used in some tests) ───────────────────────

/// Kept for API compatibility — use Ed25519Signer directly.
pub type Ed25519SignerStub = Ed25519Signer;

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn make_payload() -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"test payload");
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    fn make_signer(node_byte: u8, seed_byte: u8) -> Ed25519Signer {
        Ed25519Signer::from_seed(make_id(node_byte), make_id(seed_byte))
    }

    #[test]
    fn test_signer_identity_creation() {
        let id = SignerIdentity::new(make_id(1), make_id(2));
        assert_eq!(id.node_id, make_id(1));
        assert_eq!(id.public_key_bytes, make_id(2));
    }

    #[test]
    fn test_identity_hash_determinism() {
        let id = SignerIdentity::new(make_id(1), make_id(2));
        let h1 = id.identity_hash();
        let h2 = id.identity_hash();
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_identity_hash_differs_with_different_keys() {
        let id1 = SignerIdentity::new(make_id(1), make_id(2));
        let id2 = SignerIdentity::new(make_id(1), make_id(3));
        assert_ne!(id1.identity_hash(), id2.identity_hash());
    }

    #[test]
    fn test_sign_produces_envelope() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let env = signer.sign(&payload).unwrap();
        assert_eq!(env.payload_hash, payload);
        assert_eq!(env.signer_id, make_id(1));
        assert_ne!(env.signature_bytes, [0u8; 64]);
    }

    #[test]
    fn test_sign_is_deterministic() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let e1 = signer.sign(&payload).unwrap();
        let e2 = signer.sign(&payload).unwrap();
        assert_eq!(e1.signature_bytes, e2.signature_bytes);
    }

    #[test]
    fn test_sign_differs_for_different_payloads() {
        let signer = make_signer(1, 42);
        let p1 = make_payload();
        let mut p2 = make_payload();
        p2[0] ^= 1;
        let e1 = signer.sign(&p1).unwrap();
        let e2 = signer.sign(&p2).unwrap();
        assert_ne!(e1.signature_bytes, e2.signature_bytes);
    }

    #[test]
    fn test_verifier_accepts_valid_signature() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let env = signer.sign(&payload).unwrap();
        let verifier = Ed25519Verifier::new(signer.identity.public_key_bytes).unwrap();
        assert!(verifier.verify(&payload, &env), "valid sig must pass");
    }

    #[test]
    fn test_verifier_rejects_tampered_signature() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let mut env = signer.sign(&payload).unwrap();
        env.signature_bytes[0] ^= 0xFF;
        let verifier = Ed25519Verifier::new(signer.identity.public_key_bytes).unwrap();
        assert!(!verifier.verify(&payload, &env), "tampered sig must fail");
    }

    #[test]
    fn test_verifier_rejects_wrong_payload_hash() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let env = signer.sign(&payload).unwrap();
        let verifier = Ed25519Verifier::new(signer.identity.public_key_bytes).unwrap();
        let mut wrong = payload;
        wrong[0] ^= 0xFF;
        assert!(!verifier.verify(&wrong, &env), "mismatched payload must fail");
    }

    #[test]
    fn test_cross_node_verification_rejects_forged_sig() {
        // FIND-001: attacker forges a signature with non-zero bytes but wrong key
        let signer_a = make_signer(1, 10);
        let signer_b = make_signer(2, 20);
        let payload = make_payload();
        // B signs the payload
        let env_b = signer_b.sign(&payload).unwrap();
        // But we verify against A's public key — must fail
        let verifier_a = Ed25519Verifier::new(signer_a.identity.public_key_bytes).unwrap();
        assert!(!verifier_a.verify(&payload, &env_b), "cross-key verification must fail");
    }

    #[test]
    fn test_registry_verify_known_node() {
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let env = signer.sign(&payload).unwrap();

        let mut registry = Ed25519SignatureRegistry::new();
        registry.register(signer.identity.node_id, signer.identity.public_key_bytes).unwrap();
        assert!(registry.verify(&signer.identity.node_id, &payload, &env).unwrap());
    }

    #[test]
    fn test_registry_rejects_unknown_node() {
        let registry = Ed25519SignatureRegistry::new();
        let env = SignatureEnvelope::new([1u8; 32], [2u8; 64], [3u8; 32]);
        let result = registry.verify(&make_id(99), &[1u8; 32], &env);
        assert!(matches!(result, Err(Phase4Error::UnknownSigner(_))));
    }

    #[test]
    fn test_different_signers_produce_different_sigs() {
        let s1 = make_signer(1, 10);
        let s2 = make_signer(2, 20);
        let payload = make_payload();
        let e1 = s1.sign(&payload).unwrap();
        let e2 = s2.sign(&payload).unwrap();
        assert_ne!(e1.signature_bytes, e2.signature_bytes);
    }

    #[test]
    fn test_forged_all_ones_signature_rejected() {
        // FIND-001: StubVerifier accepted any non-zero sig bytes; real verifier must not
        let signer = make_signer(1, 42);
        let payload = make_payload();
        let env = SignatureEnvelope::new(payload, [1u8; 64], signer.identity.node_id);
        let verifier = Ed25519Verifier::new(signer.identity.public_key_bytes).unwrap();
        assert!(!verifier.verify(&payload, &env), "fabricated non-zero sig must fail");
    }
}
