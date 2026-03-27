//! Schema validator — checks contract schemas for correctness, consistency,
//! and potential issues before compilation.
//!
//! The validator produces a list of messages, each with a severity level:
//! - **Error**: The schema is invalid and cannot be compiled.
//! - **Warning**: The schema is technically valid but may indicate a mistake.
//!
//! Validation checks include:
//! - Name format validation (identifiers, version strings)
//! - Parameter type validity
//! - Constraint reference integrity (referenced fields/intents exist)
//! - State field uniqueness
//! - Intent name uniqueness
//! - Default value type compatibility

use crate::schema::*;
use std::collections::HashSet;
use std::fmt;

/// Severity level of a validation message.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Severity {
    Error,
    Warning,
}

impl fmt::Display for Severity {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Severity::Error => write!(f, "ERROR"),
            Severity::Warning => write!(f, "WARNING"),
        }
    }
}

/// A single validation message with location context.
#[derive(Debug, Clone)]
pub struct ValidationMessage {
    pub severity: Severity,
    pub location: String,
    pub message: String,
}

impl fmt::Display for ValidationMessage {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "[{}] {}: {}", self.severity, self.location, self.message)
    }
}

/// Validate a contract schema and return all messages.
pub fn validate_schema(schema: &ContractSchema) -> Vec<ValidationMessage> {
    let mut messages = Vec::new();

    validate_contract_metadata(schema, &mut messages);
    validate_intents(schema, &mut messages);
    validate_state_fields(schema, &mut messages);
    validate_constraints(schema, &mut messages);
    validate_cross_references(schema, &mut messages);

    messages
}

// ---------------------------------------------------------------------------
// Contract-level metadata
// ---------------------------------------------------------------------------

fn validate_contract_metadata(schema: &ContractSchema, messages: &mut Vec<ValidationMessage>) {
    // Name must be non-empty
    if schema.name.is_empty() {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.name".to_string(),
            message: "contract name must not be empty".to_string(),
        });
    } else if !is_valid_contract_name(&schema.name) {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.name".to_string(),
            message: format!(
                "contract name '{}' is invalid: must start with a letter and contain only \
                 alphanumeric characters, hyphens, or underscores",
                schema.name
            ),
        });
    }

    // Version must be non-empty and look like semver
    if schema.version.is_empty() {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.version".to_string(),
            message: "version must not be empty".to_string(),
        });
    } else if !is_valid_version(&schema.version) {
        messages.push(ValidationMessage {
            severity: Severity::Warning,
            location: "contract.version".to_string(),
            message: format!(
                "version '{}' does not follow semver format (MAJOR.MINOR.PATCH)",
                schema.version
            ),
        });
    }

    // Must have at least one intent
    if schema.intents.is_empty() {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.intents".to_string(),
            message: "contract must define at least one intent".to_string(),
        });
    }

    // Gas and state limits
    if schema.max_gas_per_call == 0 {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.max_gas_per_call".to_string(),
            message: "max_gas_per_call must be greater than 0".to_string(),
        });
    }

    if schema.max_state_bytes == 0 {
        messages.push(ValidationMessage {
            severity: Severity::Error,
            location: "contract.max_state_bytes".to_string(),
            message: "max_state_bytes must be greater than 0".to_string(),
        });
    }
}

// ---------------------------------------------------------------------------
// Intent validation
// ---------------------------------------------------------------------------

fn validate_intents(schema: &ContractSchema, messages: &mut Vec<ValidationMessage>) {
    let mut seen_names = HashSet::new();

    for (idx, intent) in schema.intents.iter().enumerate() {
        let loc = format!("intents[{}]", idx);

        // Name must be non-empty and valid
        if intent.name.is_empty() {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: "intent name must not be empty".to_string(),
            });
        } else if !is_valid_identifier(&intent.name) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!(
                    "intent name '{}' is invalid: must be a valid identifier \
                     (alphanumeric + underscore, starting with a letter)",
                    intent.name
                ),
            });
        }

        // Check for duplicate intent names
        if !intent.name.is_empty() && !seen_names.insert(intent.name.to_lowercase()) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!("duplicate intent name '{}'", intent.name),
            });
        }

        // Validate parameters
        validate_params(&intent.params, &format!("{}.params", loc), messages);
    }
}

fn validate_params(params: &[ParamDef], parent_loc: &str, messages: &mut Vec<ValidationMessage>) {
    let mut seen_names = HashSet::new();

    for (idx, param) in params.iter().enumerate() {
        let loc = format!("{}[{}]", parent_loc, idx);

        // Name must be valid
        if param.name.is_empty() {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: "parameter name must not be empty".to_string(),
            });
        } else if !is_valid_identifier(&param.name) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!(
                    "parameter name '{}' is invalid: must be a valid identifier",
                    param.name
                ),
            });
        }

        // Check for duplicates
        if !param.name.is_empty() && !seen_names.insert(param.name.to_lowercase()) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!("duplicate parameter name '{}'", param.name),
            });
        }
    }
}

// ---------------------------------------------------------------------------
// State field validation
// ---------------------------------------------------------------------------

fn validate_state_fields(schema: &ContractSchema, messages: &mut Vec<ValidationMessage>) {
    let mut seen_names = HashSet::new();

    for (idx, field) in schema.state_fields.iter().enumerate() {
        let loc = format!("state_fields[{}]", idx);

        // Name must be valid
        if field.name.is_empty() {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: "state field name must not be empty".to_string(),
            });
        } else if !is_valid_identifier(&field.name) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!(
                    "state field name '{}' is invalid: must be a valid identifier",
                    field.name
                ),
            });
        }

        // Check for duplicates
        if !field.name.is_empty() && !seen_names.insert(field.name.to_lowercase()) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!("duplicate state field name '{}'", field.name),
            });
        }

        // Validate default value compatibility
        if let Some(ref default) = field.default_value {
            validate_default_value(&field.field_type, default, &loc, messages);
        }
    }
}

fn validate_default_value(
    field_type: &ParamType,
    default: &str,
    loc: &str,
    messages: &mut Vec<ValidationMessage>,
) {
    match field_type {
        ParamType::Uint128 => {
            if default.parse::<u128>().is_err() {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.default_value", loc),
                    message: format!(
                        "default value '{}' is not a valid Uint128 (must be a non-negative integer)",
                        default
                    ),
                });
            }
        }
        ParamType::Address => {
            if default != "zero" && default != "sender" {
                // Must be valid hex, 64 characters (32 bytes)
                if default.len() != 64 || !default.chars().all(|c| c.is_ascii_hexdigit()) {
                    messages.push(ValidationMessage {
                        severity: Severity::Error,
                        location: format!("{}.default_value", loc),
                        message: format!(
                            "default value '{}' is not a valid Address \
                             (must be 64 hex characters, 'zero', or 'sender')",
                            default
                        ),
                    });
                }
            }
        }
        ParamType::Bytes => {
            if !default.is_empty() && !default.chars().all(|c| c.is_ascii_hexdigit()) {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.default_value", loc),
                    message: format!(
                        "default value '{}' is not valid Bytes (must be hex-encoded)",
                        default
                    ),
                });
            }
            if default.len() % 2 != 0 {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.default_value", loc),
                    message: "Bytes default value must have an even number of hex characters".to_string(),
                });
            }
        }
        ParamType::Bool => {
            if default != "true" && default != "false" {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.default_value", loc),
                    message: format!(
                        "default value '{}' is not a valid Bool (must be 'true' or 'false')",
                        default
                    ),
                });
            }
        }
        ParamType::String => {
            // Any string is valid for String type
        }
    }
}

// ---------------------------------------------------------------------------
// Constraint validation
// ---------------------------------------------------------------------------

fn validate_constraints(schema: &ContractSchema, messages: &mut Vec<ValidationMessage>) {
    let mut seen_names = HashSet::new();

    for (idx, constraint) in schema.constraints.iter().enumerate() {
        let loc = format!("constraints[{}]", idx);

        // Name must be non-empty
        if constraint.name.is_empty() {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: "constraint name must not be empty".to_string(),
            });
        }

        // Check for duplicates
        if !constraint.name.is_empty() && !seen_names.insert(constraint.name.to_lowercase()) {
            messages.push(ValidationMessage {
                severity: Severity::Error,
                location: format!("{}.name", loc),
                message: format!("duplicate constraint name '{}'", constraint.name),
            });
        }

        // Validate constraint-type-specific params
        validate_constraint_params(constraint, &loc, schema, messages);
    }
}

fn validate_constraint_params(
    constraint: &Constraint,
    loc: &str,
    schema: &ContractSchema,
    messages: &mut Vec<ValidationMessage>,
) {
    let state_field_names: HashSet<String> = schema
        .state_fields
        .iter()
        .map(|f| f.name.to_lowercase())
        .collect();

    match constraint.constraint_type {
        ConstraintType::BalanceCheck => {
            if !constraint.params.contains_key("asset") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "BalanceCheck constraint requires an 'asset' parameter".to_string(),
                });
            }
            if !constraint.params.contains_key("min_amount") {
                messages.push(ValidationMessage {
                    severity: Severity::Warning,
                    location: format!("{}.params", loc),
                    message: "BalanceCheck constraint without 'min_amount' will only check existence"
                        .to_string(),
                });
            }
        }
        ConstraintType::OwnershipCheck => {
            if !constraint.params.contains_key("object_field") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "OwnershipCheck constraint requires an 'object_field' parameter"
                        .to_string(),
                });
            }
        }
        ConstraintType::TimeCheck => {
            if !constraint.params.contains_key("op") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "TimeCheck constraint requires an 'op' parameter (gte, lte, gt, lt)"
                        .to_string(),
                });
            } else if let Some(serde_json::Value::String(op)) = constraint.params.get("op") {
                if !["gte", "lte", "gt", "lt"].contains(&op.as_str()) {
                    messages.push(ValidationMessage {
                        severity: Severity::Error,
                        location: format!("{}.params.op", loc),
                        message: format!(
                            "TimeCheck op '{}' is invalid: must be one of gte, lte, gt, lt",
                            op
                        ),
                    });
                }
            }
            if !constraint.params.contains_key("value") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "TimeCheck constraint requires a 'value' parameter".to_string(),
                });
            }
        }
        ConstraintType::StateCheck => {
            if let Some(serde_json::Value::String(field)) = constraint.params.get("field") {
                if !state_field_names.contains(&field.to_lowercase()) {
                    messages.push(ValidationMessage {
                        severity: Severity::Error,
                        location: format!("{}.params.field", loc),
                        message: format!(
                            "StateCheck references field '{}' which is not defined in state_fields",
                            field
                        ),
                    });
                }
            } else {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "StateCheck constraint requires a 'field' parameter".to_string(),
                });
            }

            if !constraint.params.contains_key("op") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "StateCheck constraint requires an 'op' parameter (gte, lte, gt, lt, eq, neq)"
                        .to_string(),
                });
            } else if let Some(serde_json::Value::String(op)) = constraint.params.get("op") {
                if !["gte", "lte", "gt", "lt", "eq", "neq"].contains(&op.as_str()) {
                    messages.push(ValidationMessage {
                        severity: Severity::Error,
                        location: format!("{}.params.op", loc),
                        message: format!(
                            "StateCheck op '{}' is invalid: must be one of gte, lte, gt, lt, eq, neq",
                            op
                        ),
                    });
                }
            }

            if !constraint.params.contains_key("value") {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("{}.params", loc),
                    message: "StateCheck constraint requires a 'value' parameter".to_string(),
                });
            }
        }
        ConstraintType::Custom => {
            if !constraint.params.contains_key("expression") {
                messages.push(ValidationMessage {
                    severity: Severity::Warning,
                    location: format!("{}.params", loc),
                    message: "Custom constraint without 'expression' will need Wasm validator logic"
                        .to_string(),
                });
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Cross-reference validation
// ---------------------------------------------------------------------------

fn validate_cross_references(schema: &ContractSchema, messages: &mut Vec<ValidationMessage>) {
    let intent_names: HashSet<String> = schema
        .intents
        .iter()
        .map(|i| i.name.to_lowercase())
        .collect();

    // Check constraint applies_to references
    for (idx, constraint) in schema.constraints.iter().enumerate() {
        for target in &constraint.applies_to {
            if !intent_names.contains(&target.to_lowercase()) {
                messages.push(ValidationMessage {
                    severity: Severity::Error,
                    location: format!("constraints[{}].applies_to", idx),
                    message: format!(
                        "constraint '{}' references intent '{}' which is not defined",
                        constraint.name, target
                    ),
                });
            }
        }
    }

    // Warn if no state fields but intents reference state
    if schema.state_fields.is_empty() && !schema.intents.is_empty() {
        messages.push(ValidationMessage {
            severity: Severity::Warning,
            location: "contract".to_string(),
            message: "contract has intents but no state_fields defined".to_string(),
        });
    }

    // Check for intents without any params (informational warning)
    for intent in &schema.intents {
        if intent.params.is_empty() && intent.preconditions.is_empty() && intent.postconditions.is_empty() {
            messages.push(ValidationMessage {
                severity: Severity::Warning,
                location: format!("intents.{}", intent.name),
                message: "intent has no params, preconditions, or postconditions".to_string(),
            });
        }
    }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Check if a string is a valid contract name: starts with a letter, contains
/// only alphanumeric, hyphens, underscores.
fn is_valid_contract_name(name: &str) -> bool {
    if name.is_empty() {
        return false;
    }
    let mut chars = name.chars();
    let first = chars.next().unwrap();
    if !first.is_ascii_alphabetic() {
        return false;
    }
    chars.all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
}

/// Check if a string is a valid identifier: starts with a letter or underscore,
/// contains only alphanumeric and underscores.
fn is_valid_identifier(name: &str) -> bool {
    if name.is_empty() {
        return false;
    }
    let mut chars = name.chars();
    let first = chars.next().unwrap();
    if !first.is_ascii_alphabetic() && first != '_' {
        return false;
    }
    chars.all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// Check if a string looks like a semver version.
fn is_valid_version(version: &str) -> bool {
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() != 3 {
        return false;
    }
    parts.iter().all(|p| p.parse::<u32>().is_ok())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn minimal_valid_schema() -> ContractSchema {
        ContractSchema {
            name: "TestContract".to_string(),
            version: "1.0.0".to_string(),
            description: "A test contract".to_string(),
            intents: vec![IntentSchema {
                name: "do_something".to_string(),
                description: "Does something".to_string(),
                params: vec![ParamDef {
                    name: "amount".to_string(),
                    param_type: ParamType::Uint128,
                    required: true,
                    description: String::new(),
                }],
                preconditions: vec![],
                postconditions: vec!["state changes".to_string()],
            }],
            state_fields: vec![StateField {
                name: "balance".to_string(),
                field_type: ParamType::Uint128,
                default_value: Some("0".to_string()),
                description: String::new(),
            }],
            constraints: vec![],
            max_gas_per_call: 1_000_000,
            max_state_bytes: 65_536,
        }
    }

    #[test]
    fn test_valid_schema_passes() {
        let schema = minimal_valid_schema();
        let messages = validate_schema(&schema);
        let errors: Vec<_> = messages.iter().filter(|m| m.severity == Severity::Error).collect();
        assert!(errors.is_empty(), "expected no errors, got: {:?}", errors);
    }

    #[test]
    fn test_empty_name_rejected() {
        let mut schema = minimal_valid_schema();
        schema.name = String::new();
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("name must not be empty")));
    }

    #[test]
    fn test_invalid_name_rejected() {
        let mut schema = minimal_valid_schema();
        schema.name = "123bad".to_string();
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("invalid")));
    }

    #[test]
    fn test_no_intents_rejected() {
        let mut schema = minimal_valid_schema();
        schema.intents.clear();
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("at least one intent")));
    }

    #[test]
    fn test_duplicate_intent_names_rejected() {
        let mut schema = minimal_valid_schema();
        let dup = schema.intents[0].clone();
        schema.intents.push(dup);
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("duplicate intent")));
    }

    #[test]
    fn test_duplicate_state_field_rejected() {
        let mut schema = minimal_valid_schema();
        let dup = schema.state_fields[0].clone();
        schema.state_fields.push(dup);
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("duplicate state field")));
    }

    #[test]
    fn test_bad_default_value_rejected() {
        let mut schema = minimal_valid_schema();
        schema.state_fields[0].default_value = Some("not_a_number".to_string());
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("not a valid Uint128")));
    }

    #[test]
    fn test_constraint_references_nonexistent_intent() {
        let mut schema = minimal_valid_schema();
        schema.constraints.push(Constraint {
            name: "test_constraint".to_string(),
            constraint_type: ConstraintType::Custom,
            params: serde_json::Map::new(),
            applies_to: vec!["nonexistent_intent".to_string()],
        });
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("not defined")));
    }

    #[test]
    fn test_state_check_references_nonexistent_field() {
        let mut schema = minimal_valid_schema();
        let mut params = serde_json::Map::new();
        params.insert("field".to_string(), serde_json::Value::String("nonexistent".to_string()));
        params.insert("op".to_string(), serde_json::Value::String("gte".to_string()));
        params.insert("value".to_string(), serde_json::Value::String("0".to_string()));
        schema.constraints.push(Constraint {
            name: "bad_check".to_string(),
            constraint_type: ConstraintType::StateCheck,
            params,
            applies_to: vec![],
        });
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("not defined in state_fields")));
    }

    #[test]
    fn test_semver_warning() {
        let mut schema = minimal_valid_schema();
        schema.version = "v1".to_string();
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Warning && m.message.contains("semver")));
    }

    #[test]
    fn test_bool_default_validation() {
        let mut schema = minimal_valid_schema();
        schema.state_fields.push(StateField {
            name: "flag".to_string(),
            field_type: ParamType::Bool,
            default_value: Some("maybe".to_string()),
            description: String::new(),
        });
        let messages = validate_schema(&schema);
        assert!(messages.iter().any(|m| m.severity == Severity::Error && m.message.contains("not a valid Bool")));
    }
}
