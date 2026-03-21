//! Contract Schema Bridge
//!
//! Imports contract schemas from the Go chain so PoSeq nodes can validate
//! contract-related plans. Schemas are imported via the same file-based bridge
//! used for committee snapshots.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// A contract schema as exported from the Go chain's x/contracts module.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainContractSchema {
    pub schema_id: String,        // hex-encoded 32 bytes
    pub deployer: String,         // bech32
    pub version: u64,
    pub name: String,
    pub description: String,
    pub domain_tag: String,
    pub intent_schemas: Vec<ChainIntentSchema>,
    pub max_gas_per_call: u64,
    pub max_state_bytes: u64,
    pub validator_hash: String,   // hex SHA256 of Wasm bytecode
    pub wasm_size: u64,
    pub status: String,
    pub deployed_at: i64,
}

/// An intent method schema from the chain.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainIntentSchema {
    pub method: String,
    pub params: Vec<ChainIntentSchemaField>,
    pub capabilities: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainIntentSchemaField {
    pub name: String,
    pub type_hint: String,
}

/// Local cache of imported contract schemas.
#[derive(Debug, Default)]
pub struct ContractSchemaCache {
    schemas: BTreeMap<[u8; 32], ChainContractSchema>,
}

impl ContractSchemaCache {
    pub fn new() -> Self {
        Self::default()
    }

    /// Import a schema from the chain. Returns Ok(true) if new, Ok(false) if
    /// already cached at the same or newer version, Err on invalid data.
    pub fn import_schema(&mut self, schema: ChainContractSchema) -> Result<bool, String> {
        let id = hex_to_32bytes(&schema.schema_id)
            .map_err(|e| format!("invalid schema_id: {}", e))?;

        if let Some(existing) = self.schemas.get(&id) {
            if existing.version >= schema.version {
                return Ok(false); // already have same or newer
            }
        }

        self.schemas.insert(id, schema);
        Ok(true)
    }

    /// Look up a schema by its 32-byte ID.
    pub fn get(&self, schema_id: &[u8; 32]) -> Option<&ChainContractSchema> {
        self.schemas.get(schema_id)
    }

    /// Check if a schema exists and is active.
    pub fn is_active(&self, schema_id: &[u8; 32]) -> bool {
        self.schemas
            .get(schema_id)
            .map(|s| s.status == "ACTIVE")
            .unwrap_or(false)
    }

    /// Number of cached schemas.
    pub fn count(&self) -> usize {
        self.schemas.len()
    }

    /// Verify that a validator hash matches the expected value for a schema.
    pub fn verify_validator_hash(&self, schema_id: &[u8; 32], wasm_bytes: &[u8]) -> bool {
        if let Some(schema) = self.schemas.get(schema_id) {
            let hash = Sha256::digest(wasm_bytes);
            let expected = hex::encode(hash);
            expected == schema.validator_hash
        } else {
            false
        }
    }
}

fn hex_to_32bytes(s: &str) -> Result<[u8; 32], String> {
    let bytes = hex::decode(s).map_err(|e| e.to_string())?;
    if bytes.len() != 32 {
        return Err(format!("expected 32 bytes, got {}", bytes.len()));
    }
    let mut arr = [0u8; 32];
    arr.copy_from_slice(&bytes);
    Ok(arr)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_test_schema(name: &str, version: u64) -> ChainContractSchema {
        let id_bytes = Sha256::digest(format!("test_deployer{}v{}", name, version).as_bytes());
        ChainContractSchema {
            schema_id: hex::encode(id_bytes),
            deployer: "omni1test".to_string(),
            version,
            name: name.to_string(),
            description: "test contract".to_string(),
            domain_tag: format!("contract.{}", name),
            intent_schemas: vec![ChainIntentSchema {
                method: "execute".to_string(),
                params: vec![],
                capabilities: vec!["ContractCall".to_string()],
            }],
            max_gas_per_call: 1_000_000,
            max_state_bytes: 65536,
            validator_hash: hex::encode([0u8; 32]),
            wasm_size: 1024,
            status: "ACTIVE".to_string(),
            deployed_at: 100,
        }
    }

    #[test]
    fn test_import_and_lookup() {
        let mut cache = ContractSchemaCache::new();
        let schema = make_test_schema("escrow", 1);
        let id = hex_to_32bytes(&schema.schema_id).unwrap();

        assert!(cache.import_schema(schema.clone()).unwrap());
        assert!(cache.get(&id).is_some());
        assert!(cache.is_active(&id));
        assert_eq!(cache.count(), 1);
    }

    #[test]
    fn test_duplicate_import_rejected() {
        let mut cache = ContractSchemaCache::new();
        let schema = make_test_schema("escrow", 1);
        assert!(cache.import_schema(schema.clone()).unwrap());
        assert!(!cache.import_schema(schema).unwrap()); // same version = no-op
    }

    #[test]
    fn test_version_upgrade() {
        let mut cache = ContractSchemaCache::new();
        let v1 = make_test_schema("escrow", 1);
        let mut v2 = make_test_schema("escrow", 2);
        v2.schema_id = v1.schema_id.clone(); // same ID, higher version

        assert!(cache.import_schema(v1).unwrap());
        assert!(cache.import_schema(v2).unwrap()); // upgrade accepted
    }
}
