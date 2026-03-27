//! Contract schema types for the Omniphi Intent Contract system.
//!
//! These types mirror the runtime's internal schema representation and serve as
//! the canonical definition that the compiler, validator, and tester all operate on.
//! Developers author YAML files that deserialize into these types, and the compiler
//! emits JSON matching the runtime's expected `ContractSchemaDef` format.

use serde::{Deserialize, Serialize};
use std::fmt;

// ---------------------------------------------------------------------------
// Parameter types
// ---------------------------------------------------------------------------

/// The set of parameter types supported by the intent contract system.
///
/// Each variant maps to a concrete runtime representation:
/// - `Uint128` — 128-bit unsigned integer (big-endian bytes on-chain)
/// - `Address` — 32-byte Ed25519 public key
/// - `Bytes`   — arbitrary byte blob (hex-encoded in YAML)
/// - `String`  — UTF-8 text
/// - `Bool`    — boolean flag
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ParamType {
    Uint128,
    Address,
    Bytes,
    String,
    Bool,
}

impl fmt::Display for ParamType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ParamType::Uint128 => write!(f, "Uint128"),
            ParamType::Address => write!(f, "Address"),
            ParamType::Bytes => write!(f, "Bytes"),
            ParamType::String => write!(f, "String"),
            ParamType::Bool => write!(f, "Bool"),
        }
    }
}

impl ParamType {
    /// Parse a type string into a `ParamType`, case-insensitive.
    pub fn from_str_loose(s: &str) -> Option<ParamType> {
        match s.to_lowercase().as_str() {
            "uint128" | "u128" => Some(ParamType::Uint128),
            "address" | "addr" => Some(ParamType::Address),
            "bytes" | "binary" => Some(ParamType::Bytes),
            "string" | "str" | "text" => Some(ParamType::String),
            "bool" | "boolean" => Some(ParamType::Bool),
            _ => None,
        }
    }

    /// All recognized type name strings, for error messages.
    pub fn known_type_names() -> &'static [&'static str] {
        &[
            "Uint128", "u128", "Address", "addr", "Bytes", "binary", "String", "str", "text",
            "Bool", "boolean",
        ]
    }
}

// ---------------------------------------------------------------------------
// Parameter definition
// ---------------------------------------------------------------------------

/// A single parameter on an intent method.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParamDef {
    /// Parameter name (must be a valid identifier: alphanumeric + underscore).
    pub name: std::string::String,
    /// The parameter's data type.
    pub param_type: ParamType,
    /// Whether the caller must supply this parameter. Defaults to `true`.
    #[serde(default = "default_true")]
    pub required: bool,
    /// Optional human-readable description.
    #[serde(default)]
    pub description: std::string::String,
}

fn default_true() -> bool {
    true
}

// ---------------------------------------------------------------------------
// Constraints
// ---------------------------------------------------------------------------

/// Built-in constraint types recognized by the runtime's admission pipeline.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ConstraintType {
    /// Check that an account has sufficient balance of a given asset.
    BalanceCheck,
    /// Check that the caller owns a specific object.
    OwnershipCheck,
    /// Check that the current epoch satisfies a time condition.
    TimeCheck,
    /// Check a state field against a condition (e.g., field >= value).
    StateCheck,
    /// Custom constraint evaluated by the contract's Wasm validator.
    Custom,
}

impl fmt::Display for ConstraintType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ConstraintType::BalanceCheck => write!(f, "BalanceCheck"),
            ConstraintType::OwnershipCheck => write!(f, "OwnershipCheck"),
            ConstraintType::TimeCheck => write!(f, "TimeCheck"),
            ConstraintType::StateCheck => write!(f, "StateCheck"),
            ConstraintType::Custom => write!(f, "Custom"),
        }
    }
}

/// A constraint attached to an intent or the contract globally.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Constraint {
    /// Human-readable constraint name (for error reporting).
    pub name: std::string::String,
    /// The type of constraint.
    pub constraint_type: ConstraintType,
    /// Parameters for the constraint, interpreted by type:
    ///
    /// - `BalanceCheck`: `{ "asset": "...", "min_amount": "..." }`
    /// - `OwnershipCheck`: `{ "object_field": "..." }`
    /// - `TimeCheck`: `{ "field": "...", "op": "gte|lte|gt|lt", "value": "..." }`
    /// - `StateCheck`: `{ "field": "...", "op": "gte|lte|gt|lt|eq|neq", "value": "..." }`
    /// - `Custom`: `{ "expression": "..." }`
    #[serde(default)]
    pub params: serde_json::Map<std::string::String, serde_json::Value>,
    /// Optional: only apply this constraint to specific intents (by name).
    /// If empty, the constraint applies to all intents.
    #[serde(default)]
    pub applies_to: Vec<std::string::String>,
}

// ---------------------------------------------------------------------------
// State fields
// ---------------------------------------------------------------------------

/// A field in the contract's persistent state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateField {
    /// Field name (must be a valid identifier).
    pub name: std::string::String,
    /// The field's data type (same type system as params).
    pub field_type: ParamType,
    /// Default value as a string representation.
    /// Parsed according to `field_type` during compilation:
    /// - `Uint128`: decimal integer string
    /// - `Address`: hex-encoded 32 bytes or "zero"
    /// - `Bytes`: hex string
    /// - `String`: literal text
    /// - `Bool`: "true" or "false"
    #[serde(default)]
    pub default_value: Option<std::string::String>,
    /// Optional human-readable description.
    #[serde(default)]
    pub description: std::string::String,
}

// ---------------------------------------------------------------------------
// Intent schema
// ---------------------------------------------------------------------------

/// An intent (method) that the contract exposes to users and solvers.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentSchema {
    /// Intent name — used as the `method_selector` in `ContractCallIntent`.
    pub name: std::string::String,
    /// Human-readable description.
    #[serde(default)]
    pub description: std::string::String,
    /// Parameters that callers supply when submitting this intent.
    #[serde(default)]
    pub params: Vec<ParamDef>,
    /// Preconditions that must hold *before* the solver's proposed state change.
    /// These are evaluated against the current on-chain state.
    #[serde(default)]
    pub preconditions: Vec<std::string::String>,
    /// Postconditions that must hold *after* the solver's proposed state change.
    /// These are evaluated against the proposed new state.
    #[serde(default)]
    pub postconditions: Vec<std::string::String>,
}

// ---------------------------------------------------------------------------
// Top-level contract schema
// ---------------------------------------------------------------------------

/// The complete schema for an Omniphi Intent Contract.
///
/// This is the top-level type that a developer authors in YAML and the compiler
/// transforms into the runtime's JSON format.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractSchema {
    /// Contract name (alphanumeric, hyphens, underscores).
    pub name: std::string::String,
    /// Semantic version string (e.g., "1.0.0").
    pub version: std::string::String,
    /// Human-readable description.
    #[serde(default)]
    pub description: std::string::String,
    /// The intent methods this contract supports.
    pub intents: Vec<IntentSchema>,
    /// The contract's persistent state fields.
    #[serde(default)]
    pub state_fields: Vec<StateField>,
    /// Global constraints that apply across all intents.
    #[serde(default)]
    pub constraints: Vec<Constraint>,
    /// Maximum gas per contract call (default: 1,000,000).
    #[serde(default = "default_max_gas")]
    pub max_gas_per_call: u64,
    /// Maximum state size in bytes (default: 65,536).
    #[serde(default = "default_max_state")]
    pub max_state_bytes: u64,
}

fn default_max_gas() -> u64 {
    1_000_000
}

fn default_max_state() -> u64 {
    65_536
}

// ---------------------------------------------------------------------------
// Compiled output (matches runtime ContractSchemaDef)
// ---------------------------------------------------------------------------

/// The compiled schema format expected by the Omniphi runtime.
///
/// This matches `ContractSchemaDef` from `omniphi-contract-sdk` and is the
/// JSON payload submitted via `posd tx contracts deploy`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledSchema {
    /// Deterministic schema ID: SHA256 of the canonical JSON (without this field).
    pub schema_id: std::string::String,
    /// Contract name.
    pub name: std::string::String,
    /// Semantic version.
    pub version: std::string::String,
    /// Human-readable description.
    pub description: std::string::String,
    /// Domain tag for randomness/signing domain separation.
    pub domain_tag: std::string::String,
    /// Intent method definitions.
    pub intent_schemas: Vec<CompiledIntentMethod>,
    /// State field definitions.
    pub state_fields: Vec<CompiledStateField>,
    /// Constraint definitions.
    pub constraints: Vec<CompiledConstraint>,
    /// Maximum gas per call.
    pub max_gas_per_call: u64,
    /// Maximum state bytes.
    pub max_state_bytes: u64,
}

/// A compiled intent method in the runtime format.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledIntentMethod {
    pub method: std::string::String,
    pub description: std::string::String,
    pub params: Vec<CompiledParamDef>,
    pub capabilities: Vec<std::string::String>,
    pub preconditions: Vec<std::string::String>,
    pub postconditions: Vec<std::string::String>,
}

/// A compiled parameter definition.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledParamDef {
    pub name: std::string::String,
    pub type_hint: std::string::String,
    pub required: bool,
}

/// A compiled state field definition.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledStateField {
    pub name: std::string::String,
    pub field_type: std::string::String,
    pub default_value: Option<std::string::String>,
}

/// A compiled constraint definition.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledConstraint {
    pub name: std::string::String,
    pub constraint_type: std::string::String,
    pub params: serde_json::Map<std::string::String, serde_json::Value>,
    pub applies_to: Vec<std::string::String>,
}

// ---------------------------------------------------------------------------
// Test case types
// ---------------------------------------------------------------------------

/// A test suite for a contract, loaded from YAML.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TestSuite {
    /// Name of the contract under test.
    pub contract: std::string::String,
    /// Path to the contract schema YAML (relative to test file).
    #[serde(default)]
    pub schema_path: std::string::String,
    /// Individual test cases.
    pub tests: Vec<TestCase>,
}

/// A single test case: invoke an intent and check state changes.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TestCase {
    /// Test case name.
    pub name: std::string::String,
    /// The intent method to invoke.
    pub intent: std::string::String,
    /// Input parameters (name → value as string).
    #[serde(default)]
    pub params: serde_json::Map<std::string::String, serde_json::Value>,
    /// The sender address (as a label or hex string).
    #[serde(default)]
    pub sender: std::string::String,
    /// Current epoch for time-based constraints.
    #[serde(default = "default_epoch")]
    pub epoch: u64,
    /// State before the intent is applied.
    #[serde(default)]
    pub state_before: serde_json::Map<std::string::String, serde_json::Value>,
    /// Expected state after the intent is applied by a solver.
    #[serde(default)]
    pub state_after: serde_json::Map<std::string::String, serde_json::Value>,
    /// Whether this test should pass constraint validation (`true`) or fail (`false`).
    #[serde(default = "default_true")]
    pub expect_valid: bool,
    /// If `expect_valid` is false, the expected error substring.
    #[serde(default)]
    pub expect_error: std::string::String,
}

fn default_epoch() -> u64 {
    1
}

// ---------------------------------------------------------------------------
// Project manifest (contract.toml)
// ---------------------------------------------------------------------------

/// The `contract.toml` manifest file at the root of a contract project.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProjectManifest {
    pub contract: ManifestContract,
    #[serde(default)]
    pub build: ManifestBuild,
}

/// `[contract]` section of the manifest.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManifestContract {
    pub name: std::string::String,
    pub version: std::string::String,
    #[serde(default)]
    pub description: std::string::String,
    #[serde(default)]
    pub authors: Vec<std::string::String>,
}

/// `[build]` section of the manifest.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ManifestBuild {
    /// Path to the contract schema YAML, relative to project root.
    #[serde(default = "default_schema_path")]
    pub schema: std::string::String,
    /// Path to the test suite YAML, relative to project root.
    #[serde(default = "default_tests_path")]
    pub tests: std::string::String,
    /// Output directory for compiled artifacts.
    #[serde(default = "default_output_dir")]
    pub output: std::string::String,
}

fn default_schema_path() -> std::string::String {
    "schema.yaml".to_string()
}

fn default_tests_path() -> std::string::String {
    "tests.yaml".to_string()
}

fn default_output_dir() -> std::string::String {
    "build".to_string()
}
