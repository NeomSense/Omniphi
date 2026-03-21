//! Contract Schema Registry
//!
//! Runtime-side mirror of on-chain contract schemas. Schemas are imported from
//! the Go chain via the bridge and cached locally for plan validation.

use crate::capabilities::checker::Capability;
use crate::objects::base::SchemaId;
use std::collections::BTreeMap;

/// Describes a single intent method that a contract supports.
#[derive(Debug, Clone)]
pub struct IntentSchema {
    /// The method name (e.g., "fund", "release", "swap").
    pub method: String,
    /// Parameter names and their type tags (for documentation / solver hints).
    pub param_names: Vec<String>,
    /// Required capabilities to invoke this method.
    pub required_capabilities: Vec<Capability>,
}

/// Describes the object schema for a contract's state.
#[derive(Debug, Clone)]
pub struct ObjectSchema {
    /// Human-readable name for the contract type (e.g., "Escrow").
    pub name: String,
    /// Field names in the contract state (informational, state is opaque bytes).
    pub field_names: Vec<String>,
}

/// A registered contract schema.
#[derive(Debug, Clone)]
pub struct ContractSchema {
    pub schema_id: SchemaId,
    pub deployer: [u8; 32],
    pub version: u64,
    pub object_schema: ObjectSchema,
    pub intent_schemas: Vec<IntentSchema>,
    pub required_capabilities: Vec<Capability>,
    /// Domain tag for safety kernel isolation.
    pub domain_tag: String,
    /// Maximum gas per constraint validation invocation.
    pub max_gas_per_call: u64,
    /// Maximum state size in bytes per contract object.
    pub max_state_bytes: u64,
    /// SHA256 of the Wasm constraint validator bytecode.
    pub validator_hash: [u8; 32],
}

impl ContractSchema {
    /// Check if this schema supports a given method.
    pub fn has_method(&self, method: &str) -> bool {
        self.intent_schemas.iter().any(|s| s.method == method)
    }

    /// Get the intent schema for a method, if it exists.
    pub fn get_intent_schema(&self, method: &str) -> Option<&IntentSchema> {
        self.intent_schemas.iter().find(|s| s.method == method)
    }
}

/// Registry of all known contract schemas.
///
/// Populated from chain bridge imports. Used during plan validation to look up
/// schemas for `PlanActionType::Custom` actions targeting contract objects.
#[derive(Debug, Default)]
pub struct ContractSchemaRegistry {
    schemas: BTreeMap<SchemaId, ContractSchema>,
    /// Index: deployer → list of schema IDs.
    by_deployer: BTreeMap<[u8; 32], Vec<SchemaId>>,
}

impl ContractSchemaRegistry {
    pub fn new() -> Self {
        Self::default()
    }

    /// Register a new schema. Returns error if schema_id already exists
    /// with a different version (use `upgrade` for version bumps).
    pub fn register(&mut self, schema: ContractSchema) -> Result<(), String> {
        if let Some(existing) = self.schemas.get(&schema.schema_id) {
            if existing.version >= schema.version {
                return Err(format!(
                    "schema {} already registered at version {}, cannot register version {}",
                    hex::encode(schema.schema_id),
                    existing.version,
                    schema.version,
                ));
            }
        }
        let deployer = schema.deployer;
        let id = schema.schema_id;
        self.schemas.insert(id, schema);
        self.by_deployer.entry(deployer).or_default().push(id);
        Ok(())
    }

    /// Look up a schema by ID.
    pub fn get(&self, schema_id: &SchemaId) -> Option<&ContractSchema> {
        self.schemas.get(schema_id)
    }

    /// List all schemas deployed by a given address.
    pub fn by_deployer(&self, deployer: &[u8; 32]) -> Vec<&ContractSchema> {
        self.by_deployer
            .get(deployer)
            .map(|ids| ids.iter().filter_map(|id| self.schemas.get(id)).collect())
            .unwrap_or_default()
    }

    /// Total number of registered schemas.
    pub fn count(&self) -> usize {
        self.schemas.len()
    }
}
