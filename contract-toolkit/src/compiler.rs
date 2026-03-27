//! Schema compiler — reads YAML contract definitions, validates, and emits
//! compiled JSON schemas ready for on-chain deployment.
//!
//! The compiler performs three phases:
//! 1. **Parse**: Deserialize the YAML source into `ContractSchema`.
//! 2. **Validate**: Run the full validation suite (see `validator.rs`).
//! 3. **Emit**: Transform into `CompiledSchema`, compute the deterministic
//!    `schema_id`, and serialize to JSON.
//!
//! The `schema_id` is `SHA256(canonical_json)` where `canonical_json` is the
//! compiled schema serialized with `schema_id` set to the empty string, keys
//! sorted deterministically. This matches the runtime's ID computation.

use crate::schema::*;
use crate::validator::{validate_schema, ValidationMessage, Severity};
use sha2::{Digest, Sha256};
use std::path::Path;

/// Errors that can occur during compilation.
#[derive(Debug)]
pub enum CompileError {
    /// Failed to read the source file.
    IoError(std::io::Error),
    /// Failed to parse YAML.
    ParseError(String),
    /// Schema validation found errors.
    ValidationErrors(Vec<ValidationMessage>),
}

impl std::fmt::Display for CompileError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CompileError::IoError(e) => write!(f, "I/O error: {}", e),
            CompileError::ParseError(msg) => write!(f, "YAML parse error: {}", msg),
            CompileError::ValidationErrors(msgs) => {
                writeln!(f, "Schema validation failed:")?;
                for msg in msgs {
                    writeln!(f, "  {}", msg)?;
                }
                Ok(())
            }
        }
    }
}

impl std::error::Error for CompileError {}

/// Parse a YAML string into a `ContractSchema`.
pub fn parse_yaml(yaml_str: &str) -> Result<ContractSchema, CompileError> {
    serde_yaml::from_str(yaml_str).map_err(|e| CompileError::ParseError(e.to_string()))
}

/// Parse a YAML file into a `ContractSchema`.
pub fn parse_yaml_file(path: &Path) -> Result<ContractSchema, CompileError> {
    let content = std::fs::read_to_string(path).map_err(CompileError::IoError)?;
    parse_yaml(&content)
}

/// Compile a `ContractSchema` into a `CompiledSchema`.
///
/// This validates the schema, transforms it to the runtime format, and computes
/// the deterministic `schema_id`. Returns validation warnings alongside the
/// compiled output if there are no errors.
pub fn compile(schema: &ContractSchema) -> Result<(CompiledSchema, Vec<ValidationMessage>), CompileError> {
    // Run validation
    let messages = validate_schema(schema);
    let errors: Vec<&ValidationMessage> = messages.iter().filter(|m| m.severity == Severity::Error).collect();
    if !errors.is_empty() {
        return Err(CompileError::ValidationErrors(
            errors.into_iter().cloned().collect(),
        ));
    }

    let warnings: Vec<ValidationMessage> = messages
        .into_iter()
        .filter(|m| m.severity == Severity::Warning)
        .collect();

    // Transform to compiled format
    let domain_tag = format!("contract.{}", schema.name.to_lowercase().replace(' ', "_"));

    let intent_schemas: Vec<CompiledIntentMethod> = schema
        .intents
        .iter()
        .map(|intent| CompiledIntentMethod {
            method: intent.name.clone(),
            description: intent.description.clone(),
            params: intent
                .params
                .iter()
                .map(|p| CompiledParamDef {
                    name: p.name.clone(),
                    type_hint: p.param_type.to_string().to_lowercase(),
                    required: p.required,
                })
                .collect(),
            capabilities: vec!["ContractCall".to_string()],
            preconditions: intent.preconditions.clone(),
            postconditions: intent.postconditions.clone(),
        })
        .collect();

    let state_fields: Vec<CompiledStateField> = schema
        .state_fields
        .iter()
        .map(|sf| CompiledStateField {
            name: sf.name.clone(),
            field_type: sf.field_type.to_string().to_lowercase(),
            default_value: sf.default_value.clone(),
        })
        .collect();

    let constraints: Vec<CompiledConstraint> = schema
        .constraints
        .iter()
        .map(|c| CompiledConstraint {
            name: c.name.clone(),
            constraint_type: c.constraint_type.to_string(),
            params: c.params.clone(),
            applies_to: c.applies_to.clone(),
        })
        .collect();

    // Build the compiled schema with an empty schema_id first (for hashing)
    let mut compiled = CompiledSchema {
        schema_id: String::new(),
        name: schema.name.clone(),
        version: schema.version.clone(),
        description: schema.description.clone(),
        domain_tag,
        intent_schemas,
        state_fields,
        constraints,
        max_gas_per_call: schema.max_gas_per_call,
        max_state_bytes: schema.max_state_bytes,
    };

    // Compute deterministic schema_id
    compiled.schema_id = compute_schema_id(&compiled);

    Ok((compiled, warnings))
}

/// Compute the deterministic schema ID.
///
/// The ID is `hex(SHA256(canonical_json))` where `canonical_json` is the
/// compiled schema serialized with `schema_id` set to `""`. The JSON
/// serialization uses `serde_json` which produces deterministic key ordering
/// for structs (field declaration order), which is consistent across builds.
pub fn compute_schema_id(schema: &CompiledSchema) -> String {
    // Clone and zero out the schema_id for hashing
    let mut hashable = schema.clone();
    hashable.schema_id = String::new();

    let canonical = serde_json::to_string(&hashable).expect("compiled schema serialization is infallible");

    let mut hasher = Sha256::new();
    hasher.update(b"OMNIPHI_CONTRACT_SCHEMA_V1");
    hasher.update(canonical.as_bytes());
    let result = hasher.finalize();

    hex::encode(result)
}

/// Compile a YAML file and write the output JSON.
///
/// Returns the compiled schema and any warnings.
pub fn compile_file(
    input_path: &Path,
    output_path: &Path,
) -> Result<(CompiledSchema, Vec<ValidationMessage>), CompileError> {
    let schema = parse_yaml_file(input_path)?;
    let (compiled, warnings) = compile(&schema)?;

    let json = serde_json::to_string_pretty(&compiled)
        .expect("compiled schema JSON serialization is infallible");

    std::fs::write(output_path, json).map_err(CompileError::IoError)?;

    Ok((compiled, warnings))
}

/// Simple hex encoding (avoids pulling in the `hex` crate for just this).
mod hex {
    pub fn encode(bytes: impl AsRef<[u8]>) -> String {
        bytes
            .as_ref()
            .iter()
            .map(|b| format!("{:02x}", b))
            .collect()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    const COUNTER_YAML: &str = r#"
name: Counter
version: "1.0.0"
description: A simple counter contract

intents:
  - name: increment
    description: Increase the counter
    params:
      - name: amount
        param_type: Uint128
        required: true
    postconditions:
      - "count increases by amount"

  - name: decrement
    description: Decrease the counter
    params:
      - name: amount
        param_type: Uint128
        required: true
    preconditions:
      - "count >= amount"
    postconditions:
      - "count decreases by amount"

state_fields:
  - name: count
    field_type: Uint128
    default_value: "0"
  - name: owner
    field_type: Address

constraints:
  - name: non_negative_count
    constraint_type: StateCheck
    params:
      field: count
      op: gte
      value: "0"
    applies_to:
      - decrement
"#;

    #[test]
    fn test_parse_counter_yaml() {
        let schema = parse_yaml(COUNTER_YAML).unwrap();
        assert_eq!(schema.name, "Counter");
        assert_eq!(schema.version, "1.0.0");
        assert_eq!(schema.intents.len(), 2);
        assert_eq!(schema.state_fields.len(), 2);
        assert_eq!(schema.constraints.len(), 1);
    }

    #[test]
    fn test_compile_counter() {
        let schema = parse_yaml(COUNTER_YAML).unwrap();
        let (compiled, warnings) = compile(&schema).unwrap();

        assert_eq!(compiled.name, "Counter");
        assert_eq!(compiled.domain_tag, "contract.counter");
        assert!(!compiled.schema_id.is_empty());
        assert_eq!(compiled.schema_id.len(), 64); // SHA256 hex
        assert!(warnings.is_empty());
    }

    #[test]
    fn test_schema_id_is_deterministic() {
        let schema = parse_yaml(COUNTER_YAML).unwrap();
        let (compiled1, _) = compile(&schema).unwrap();
        let (compiled2, _) = compile(&schema).unwrap();
        assert_eq!(compiled1.schema_id, compiled2.schema_id);
    }

    #[test]
    fn test_invalid_yaml_rejected() {
        let bad_yaml = "name: [invalid\nversion: ???";
        let result = parse_yaml(bad_yaml);
        assert!(result.is_err());
    }
}
