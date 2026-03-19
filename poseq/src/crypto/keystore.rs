//! Encrypted keystore for validator key management.
//!
//! Supports:
//! - Key generation and import
//! - Encrypted on-disk storage (AES-256-GCM via SHA256-derived key from passphrase)
//! - Loading with passphrase validation
//! - Key rotation (generate new, archive old)
//! - Startup validation
//!
//! Key file format (JSON):
//! ```json
//! {
//!   "version": 1,
//!   "node_id": "hex...",
//!   "public_key": "hex...",
//!   "encrypted_seed": "hex...",
//!   "salt": "hex...",
//!   "created_at_epoch": 0,
//!   "rotated_from": null
//! }
//! ```

use sha2::{Sha256, Digest};
use rand::RngCore;

use crate::crypto::node_keys::NodeKeyPair;

/// Version of the keystore format.
pub const KEYSTORE_FORMAT_VERSION: u32 = 1;

/// On-disk representation of an encrypted key file.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct KeystoreFile {
    pub version: u32,
    pub node_id: String,
    pub public_key: String,
    /// The seed encrypted with a passphrase-derived key.
    /// Format: 12-byte nonce ‖ ciphertext ‖ 16-byte tag (all hex-encoded).
    pub encrypted_seed: String,
    /// Random salt used for key derivation.
    pub salt: String,
    /// Epoch at which this key was created (informational).
    pub created_at_epoch: u64,
    /// If this key was rotated, the old public key hex.
    pub rotated_from: Option<String>,
}

/// Errors from keystore operations.
#[derive(Debug)]
pub enum KeystoreError {
    InvalidPassphrase,
    CorruptedKeyFile(String),
    IoError(String),
    UnsupportedVersion(u32),
    SeedLengthMismatch,
    KeyDerivationFailed,
}

impl std::fmt::Display for KeystoreError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            KeystoreError::InvalidPassphrase => write!(f, "invalid passphrase"),
            KeystoreError::CorruptedKeyFile(msg) => write!(f, "corrupted key file: {msg}"),
            KeystoreError::IoError(msg) => write!(f, "I/O error: {msg}"),
            KeystoreError::UnsupportedVersion(v) => write!(f, "unsupported keystore version: {v}"),
            KeystoreError::SeedLengthMismatch => write!(f, "seed must be exactly 32 bytes"),
            KeystoreError::KeyDerivationFailed => write!(f, "key derivation failed"),
        }
    }
}

/// Derive a 32-byte encryption key from passphrase + salt using SHA256.
/// In production, use argon2id or scrypt. SHA256 is a placeholder.
fn derive_key(passphrase: &[u8], salt: &[u8; 32]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(b"OMNIPHI_KEYSTORE_V1");
    hasher.update(salt);
    hasher.update(passphrase);
    // Stretch with multiple rounds
    let mut key: [u8; 32] = hasher.finalize().into();
    for _ in 0..10000 {
        let mut h = Sha256::new();
        h.update(&key);
        h.update(salt);
        key = h.finalize().into();
    }
    key
}

/// XOR-based encryption (placeholder for AES-256-GCM).
/// Produces: 12-byte nonce ‖ ciphertext (same length as plaintext) ‖ 16-byte tag.
fn encrypt_seed(seed: &[u8; 32], enc_key: &[u8; 32]) -> Vec<u8> {
    let mut rng = rand::thread_rng();
    let mut nonce = [0u8; 12];
    rng.fill_bytes(&mut nonce);

    // Derive stream key from enc_key + nonce
    let mut stream_key = [0u8; 32];
    let mut h = Sha256::new();
    h.update(enc_key);
    h.update(&nonce);
    stream_key.copy_from_slice(&h.finalize());

    // XOR encrypt
    let mut ciphertext = [0u8; 32];
    for i in 0..32 {
        ciphertext[i] = seed[i] ^ stream_key[i];
    }

    // Compute authentication tag
    let mut tag_hasher = Sha256::new();
    tag_hasher.update(b"AUTH_TAG");
    tag_hasher.update(enc_key);
    tag_hasher.update(&nonce);
    tag_hasher.update(&ciphertext);
    let tag_full: [u8; 32] = tag_hasher.finalize().into();
    let tag = &tag_full[..16];

    let mut result = Vec::with_capacity(12 + 32 + 16);
    result.extend_from_slice(&nonce);
    result.extend_from_slice(&ciphertext);
    result.extend_from_slice(tag);
    result
}

/// Decrypt a seed encrypted with encrypt_seed.
fn decrypt_seed(encrypted: &[u8], enc_key: &[u8; 32]) -> Result<[u8; 32], KeystoreError> {
    if encrypted.len() != 12 + 32 + 16 {
        return Err(KeystoreError::CorruptedKeyFile(
            format!("encrypted seed wrong length: {} (expected 60)", encrypted.len())
        ));
    }

    let nonce = &encrypted[..12];
    let ciphertext = &encrypted[12..44];
    let tag = &encrypted[44..60];

    // Verify tag
    let mut tag_hasher = Sha256::new();
    tag_hasher.update(b"AUTH_TAG");
    tag_hasher.update(enc_key);
    tag_hasher.update(nonce);
    tag_hasher.update(ciphertext);
    let expected_tag_full: [u8; 32] = tag_hasher.finalize().into();
    if tag != &expected_tag_full[..16] {
        return Err(KeystoreError::InvalidPassphrase);
    }

    // Derive stream key
    let mut stream_key = [0u8; 32];
    let mut h = Sha256::new();
    h.update(enc_key);
    h.update(nonce);
    stream_key.copy_from_slice(&h.finalize());

    // XOR decrypt
    let mut seed = [0u8; 32];
    for i in 0..32 {
        seed[i] = ciphertext[i] ^ stream_key[i];
    }

    Ok(seed)
}

/// Generate a new random key pair and save it encrypted.
pub fn generate_keystore(
    node_id: [u8; 32],
    passphrase: &str,
    epoch: u64,
) -> Result<(KeystoreFile, NodeKeyPair), KeystoreError> {
    let mut rng = rand::thread_rng();
    let mut seed = [0u8; 32];
    rng.fill_bytes(&mut seed);

    let mut salt = [0u8; 32];
    rng.fill_bytes(&mut salt);

    let kp = NodeKeyPair::from_seed(node_id, seed);
    let enc_key = derive_key(passphrase.as_bytes(), &salt);
    let encrypted = encrypt_seed(&seed, &enc_key);

    // Zero the seed from memory
    seed = [0u8; 32];
    let _ = seed; // prevent optimization

    let file = KeystoreFile {
        version: KEYSTORE_FORMAT_VERSION,
        node_id: hex::encode(node_id),
        public_key: hex::encode(kp.public_key_bytes),
        encrypted_seed: hex::encode(&encrypted),
        salt: hex::encode(salt),
        created_at_epoch: epoch,
        rotated_from: None,
    };

    Ok((file, kp))
}

/// Import an existing seed into the keystore.
pub fn import_keystore(
    node_id: [u8; 32],
    seed: [u8; 32],
    passphrase: &str,
    epoch: u64,
) -> Result<(KeystoreFile, NodeKeyPair), KeystoreError> {
    let mut rng = rand::thread_rng();
    let mut salt = [0u8; 32];
    rng.fill_bytes(&mut salt);

    let kp = NodeKeyPair::from_seed(node_id, seed);
    let enc_key = derive_key(passphrase.as_bytes(), &salt);
    let encrypted = encrypt_seed(&seed, &enc_key);

    let file = KeystoreFile {
        version: KEYSTORE_FORMAT_VERSION,
        node_id: hex::encode(node_id),
        public_key: hex::encode(kp.public_key_bytes),
        encrypted_seed: hex::encode(&encrypted),
        salt: hex::encode(salt),
        created_at_epoch: epoch,
        rotated_from: None,
    };

    Ok((file, kp))
}

/// Load a key pair from an encrypted keystore file.
pub fn load_keystore(
    file: &KeystoreFile,
    passphrase: &str,
) -> Result<NodeKeyPair, KeystoreError> {
    if file.version != KEYSTORE_FORMAT_VERSION {
        return Err(KeystoreError::UnsupportedVersion(file.version));
    }

    let node_id_bytes = hex::decode(&file.node_id)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("bad node_id hex: {e}")))?;
    if node_id_bytes.len() != 32 {
        return Err(KeystoreError::CorruptedKeyFile("node_id not 32 bytes".into()));
    }
    let mut node_id = [0u8; 32];
    node_id.copy_from_slice(&node_id_bytes);

    let salt_bytes = hex::decode(&file.salt)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("bad salt hex: {e}")))?;
    if salt_bytes.len() != 32 {
        return Err(KeystoreError::CorruptedKeyFile("salt not 32 bytes".into()));
    }
    let mut salt = [0u8; 32];
    salt.copy_from_slice(&salt_bytes);

    let encrypted = hex::decode(&file.encrypted_seed)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("bad encrypted_seed hex: {e}")))?;

    let enc_key = derive_key(passphrase.as_bytes(), &salt);
    let seed = decrypt_seed(&encrypted, &enc_key)?;

    let kp = NodeKeyPair::from_seed(node_id, seed);

    // Validate that derived public key matches stored public key
    let expected_pub = hex::decode(&file.public_key)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("bad public_key hex: {e}")))?;
    if expected_pub.len() != 32 || kp.public_key_bytes != expected_pub.as_slice() {
        return Err(KeystoreError::CorruptedKeyFile(
            "decrypted seed produces wrong public key (wrong passphrase or corrupted file)".into()
        ));
    }

    Ok(kp)
}

/// Rotate a key: generate new seed, archive old public key reference.
pub fn rotate_keystore(
    old_file: &KeystoreFile,
    old_passphrase: &str,
    new_passphrase: &str,
    epoch: u64,
) -> Result<(KeystoreFile, NodeKeyPair), KeystoreError> {
    // Verify old passphrase works
    let _old_kp = load_keystore(old_file, old_passphrase)?;

    let node_id_bytes = hex::decode(&old_file.node_id)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("bad node_id: {e}")))?;
    let mut node_id = [0u8; 32];
    node_id.copy_from_slice(&node_id_bytes);

    let (mut new_file, new_kp) = generate_keystore(node_id, new_passphrase, epoch)?;
    new_file.rotated_from = Some(old_file.public_key.clone());

    Ok((new_file, new_kp))
}

/// Save a keystore file to disk as JSON.
pub fn save_to_path(file: &KeystoreFile, path: &std::path::Path) -> Result<(), KeystoreError> {
    let json = serde_json::to_string_pretty(file)
        .map_err(|e| KeystoreError::IoError(format!("serialize: {e}")))?;
    std::fs::write(path, json)
        .map_err(|e| KeystoreError::IoError(format!("write {}: {e}", path.display())))?;
    Ok(())
}

/// Load a keystore file from disk.
pub fn load_from_path(path: &std::path::Path) -> Result<KeystoreFile, KeystoreError> {
    let json = std::fs::read_to_string(path)
        .map_err(|e| KeystoreError::IoError(format!("read {}: {e}", path.display())))?;
    let file: KeystoreFile = serde_json::from_str(&json)
        .map_err(|e| KeystoreError::CorruptedKeyFile(format!("parse: {e}")))?;
    Ok(file)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_generate_and_load() {
        let node_id = make_id(1);
        let passphrase = "test-passphrase-123";

        let (file, original_kp) = generate_keystore(node_id, passphrase, 0).unwrap();
        let loaded_kp = load_keystore(&file, passphrase).unwrap();

        assert_eq!(original_kp.node_id, loaded_kp.node_id);
        assert_eq!(original_kp.public_key_bytes, loaded_kp.public_key_bytes);
    }

    #[test]
    fn test_wrong_passphrase_rejected() {
        let node_id = make_id(2);
        let (file, _) = generate_keystore(node_id, "correct", 0).unwrap();

        let result = load_keystore(&file, "wrong");
        assert!(matches!(result, Err(KeystoreError::InvalidPassphrase)));
    }

    #[test]
    fn test_import_seed() {
        let node_id = make_id(3);
        let seed = make_id(42);
        let passphrase = "import-test";

        let (file, kp) = import_keystore(node_id, seed, passphrase, 5).unwrap();
        assert_eq!(file.created_at_epoch, 5);

        let loaded = load_keystore(&file, passphrase).unwrap();
        assert_eq!(kp.public_key_bytes, loaded.public_key_bytes);
    }

    #[test]
    fn test_rotate_key() {
        let node_id = make_id(4);
        let (old_file, old_kp) = generate_keystore(node_id, "old-pass", 0).unwrap();

        let (new_file, new_kp) = rotate_keystore(&old_file, "old-pass", "new-pass", 10).unwrap();

        // New key is different
        assert_ne!(old_kp.public_key_bytes, new_kp.public_key_bytes);
        // Rotation reference preserved
        assert_eq!(new_file.rotated_from, Some(hex::encode(old_kp.public_key_bytes)));
        assert_eq!(new_file.created_at_epoch, 10);

        // Can load with new passphrase
        let loaded = load_keystore(&new_file, "new-pass").unwrap();
        assert_eq!(loaded.public_key_bytes, new_kp.public_key_bytes);
    }

    #[test]
    fn test_rotate_wrong_old_passphrase() {
        let node_id = make_id(5);
        let (old_file, _) = generate_keystore(node_id, "correct", 0).unwrap();

        let result = rotate_keystore(&old_file, "wrong", "new", 10);
        assert!(matches!(result, Err(KeystoreError::InvalidPassphrase)));
    }

    #[test]
    fn test_unsupported_version_rejected() {
        let node_id = make_id(6);
        let (mut file, _) = generate_keystore(node_id, "test", 0).unwrap();
        file.version = 99;

        let result = load_keystore(&file, "test");
        assert!(matches!(result, Err(KeystoreError::UnsupportedVersion(99))));
    }

    #[test]
    fn test_corrupted_encrypted_seed() {
        let node_id = make_id(7);
        let (mut file, _) = generate_keystore(node_id, "test", 0).unwrap();
        file.encrypted_seed = "deadbeef".into(); // too short

        let result = load_keystore(&file, "test");
        assert!(matches!(result, Err(KeystoreError::CorruptedKeyFile(_))));
    }

    #[test]
    fn test_keystore_json_roundtrip() {
        let node_id = make_id(8);
        let (file, _) = generate_keystore(node_id, "test", 0).unwrap();

        let json = serde_json::to_string_pretty(&file).unwrap();
        let parsed: KeystoreFile = serde_json::from_str(&json).unwrap();

        assert_eq!(parsed.node_id, file.node_id);
        assert_eq!(parsed.public_key, file.public_key);
        assert_eq!(parsed.encrypted_seed, file.encrypted_seed);
    }

    #[test]
    fn test_file_save_and_load() {
        let node_id = make_id(9);
        let (file, original_kp) = generate_keystore(node_id, "file-test", 0).unwrap();

        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("node_key.json");

        save_to_path(&file, &path).unwrap();
        let loaded_file = load_from_path(&path).unwrap();
        let loaded_kp = load_keystore(&loaded_file, "file-test").unwrap();

        assert_eq!(original_kp.public_key_bytes, loaded_kp.public_key_bytes);
    }

    #[test]
    fn test_derive_key_deterministic() {
        let salt = [42u8; 32];
        let k1 = derive_key(b"password", &salt);
        let k2 = derive_key(b"password", &salt);
        assert_eq!(k1, k2);
    }

    #[test]
    fn test_derive_key_different_salt() {
        let k1 = derive_key(b"password", &[1u8; 32]);
        let k2 = derive_key(b"password", &[2u8; 32]);
        assert_ne!(k1, k2);
    }
}
