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

/// Supported primitive types for schema fields.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FieldType {
    Uint128,
    Int128,
    Bool,
    Bytes32,
    String,
    Bytes,
    Map,
    List,
}

/// A single field definition in an object schema.
#[derive(Debug, Clone)]
pub struct FieldDefinition {
    /// Field name (must be unique within schema).
    pub name: String,
    /// Expected type.
    pub field_type: FieldType,
    /// Whether this field must be present in every valid state.
    pub required: bool,
    /// Default value if field is absent (only for optional fields).
    pub default: Option<crate::contracts::advanced::StorageValue>,
}

/// Describes the object schema for a contract's state.
#[derive(Debug, Clone)]
pub struct ObjectSchema {
    /// Human-readable name for the contract type (e.g., "Escrow").
    pub name: String,
    /// Typed field definitions with required/optional and defaults.
    pub fields: Vec<FieldDefinition>,
    /// Convenience: field names only (derived from fields).
    pub field_names: Vec<String>,
}

impl ObjectSchema {
    /// Create a schema from field definitions.
    pub fn new(name: &str, fields: Vec<FieldDefinition>) -> Self {
        let field_names = fields.iter().map(|f| f.name.clone()).collect();
        ObjectSchema { name: name.to_string(), fields, field_names }
    }

    /// Validate a ContractStorage state against this schema.
    /// Returns Ok(()) if all required fields present and types match.
    pub fn validate_state(&self, storage: &crate::contracts::advanced::ContractStorage) -> Result<(), String> {
        use crate::contracts::advanced::StorageValue;

        for field in &self.fields {
            match storage.get(&field.name) {
                Some(val) => {
                    // Type check
                    let matches = match (&field.field_type, val) {
                        (FieldType::Uint128, StorageValue::Uint128(_)) => true,
                        (FieldType::Int128, StorageValue::Int128(_)) => true,
                        (FieldType::Bool, StorageValue::Bool(_)) => true,
                        (FieldType::Bytes32, StorageValue::Bytes32(_)) => true,
                        (FieldType::String, StorageValue::String(_)) => true,
                        (FieldType::Bytes, StorageValue::Bytes(_)) => true,
                        (FieldType::Map, StorageValue::Map(_)) => true,
                        (FieldType::List, StorageValue::List(_)) => true,
                        _ => false,
                    };
                    if !matches {
                        return Err(format!(
                            "field '{}': expected type {:?}, got {:?}",
                            field.name, field.field_type, val
                        ));
                    }
                }
                None => {
                    if field.required {
                        return Err(format!("missing required field '{}'", field.name));
                    }
                }
            }
        }
        Ok(())
    }

    /// Apply default values for missing optional fields.
    pub fn apply_defaults(&self, storage: &mut crate::contracts::advanced::ContractStorage) {
        for field in &self.fields {
            if !field.required {
                if storage.get(&field.name).is_none() {
                    if let Some(ref default) = field.default {
                        storage.set(&field.name, default.clone());
                    }
                }
            }
        }
    }

    /// Check backward compatibility: new_schema must be a superset of self.
    /// All existing required fields must still exist with the same type.
    /// New fields must be optional (to not break existing state).
    pub fn is_compatible_upgrade(&self, new_schema: &ObjectSchema) -> Result<(), String> {
        // Every field in old schema must exist in new schema with same type
        for old_field in &self.fields {
            let new_field = new_schema.fields.iter().find(|f| f.name == old_field.name);
            match new_field {
                Some(nf) => {
                    if nf.field_type != old_field.field_type {
                        return Err(format!(
                            "field '{}' type changed from {:?} to {:?}",
                            old_field.name, old_field.field_type, nf.field_type
                        ));
                    }
                }
                None => {
                    if old_field.required {
                        return Err(format!(
                            "required field '{}' removed in new schema",
                            old_field.name
                        ));
                    }
                    // Optional fields can be removed
                }
            }
        }

        // New required fields are not allowed (breaks existing state)
        for new_field in &new_schema.fields {
            let existed = self.fields.iter().any(|f| f.name == new_field.name);
            if !existed && new_field.required {
                return Err(format!(
                    "new required field '{}' breaks backward compatibility (must be optional with default)",
                    new_field.name
                ));
            }
        }

        Ok(())
    }
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

    /// Register a new schema. Returns error if schema_id already exists.
    pub fn register(&mut self, schema: ContractSchema) -> Result<(), String> {
        if self.schemas.contains_key(&schema.schema_id) {
            return Err(format!(
                "schema {} already registered (use upgrade for new versions)",
                hex::encode(schema.schema_id),
            ));
        }
        let deployer = schema.deployer;
        let id = schema.schema_id;
        self.schemas.insert(id, schema);
        self.by_deployer.entry(deployer).or_default().push(id);
        Ok(())
    }

    /// Upgrade a schema to a new version. Validates backward compatibility.
    pub fn upgrade(&mut self, schema: ContractSchema) -> Result<(), String> {
        let existing = self.schemas.get(&schema.schema_id).ok_or_else(|| {
            format!("schema {} not found (use register for new schemas)", hex::encode(schema.schema_id))
        })?;

        if schema.version <= existing.version {
            return Err(format!(
                "schema {} version {} <= existing version {}",
                hex::encode(schema.schema_id), schema.version, existing.version,
            ));
        }

        // Check backward compatibility
        existing.object_schema.is_compatible_upgrade(&schema.object_schema)?;

        self.schemas.insert(schema.schema_id, schema);
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
