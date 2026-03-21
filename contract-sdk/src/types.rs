//! Core types for Intent Contracts.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

/// 32-byte identifier used throughout the contract system.
pub type Bytes32 = [u8; 32];

/// The execution context provided to constraint validators.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ValidationContext {
    /// Current epoch number.
    pub epoch: u64,
    /// The sender (user) who submitted the intent.
    pub sender: Bytes32,
    /// Remaining gas for this validation.
    pub gas_remaining: u64,
    /// The method being called (e.g., "fund", "release").
    pub method: String,
    /// Intent parameters as key-value pairs.
    pub params: BTreeMap<String, Vec<u8>>,
}

/// The result of a constraint validation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConstraintResult {
    /// Whether the proposed state transition is valid.
    pub valid: bool,
    /// Human-readable reason (empty on success, populated on rejection).
    pub reason: String,
}

impl ConstraintResult {
    /// Accept the proposed state transition.
    pub fn accept() -> Self {
        ConstraintResult {
            valid: true,
            reason: String::new(),
        }
    }

    /// Reject the proposed state transition with a reason.
    pub fn reject(reason: &str) -> Self {
        ConstraintResult {
            valid: false,
            reason: reason.to_string(),
        }
    }
}

/// Describes a parameter in an intent method.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParamDef {
    pub name: String,
    pub type_hint: String,
}

/// Describes an intent method that the contract supports.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentMethodDef {
    pub method: String,
    pub params: Vec<ParamDef>,
    pub capabilities: Vec<String>,
    pub description: String,
}

/// The complete schema definition for a contract.
/// Used to generate the on-chain schema JSON.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractSchemaDef {
    pub name: String,
    pub description: String,
    pub domain_tag: String,
    pub intent_schemas: Vec<IntentMethodDef>,
    pub max_gas_per_call: u64,
    pub max_state_bytes: u64,
}

impl ContractSchemaDef {
    /// Serialize to JSON for the `posd tx contracts deploy` schema file.
    pub fn to_json(&self) -> String {
        serde_json::to_string_pretty(self).expect("schema serialization is infallible")
    }
}
