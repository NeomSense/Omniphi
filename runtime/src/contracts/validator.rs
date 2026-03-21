//! Constraint Validator Bridge
//!
//! Trait for invoking a contract's Wasm constraint validator. The Go chain
//! executes the Wasm via wazero; this trait abstracts the cross-language call.

use crate::objects::base::SchemaId;

/// Result of a constraint validation invocation.
#[derive(Debug, Clone)]
pub struct ConstraintResult {
    /// Whether the proposed state transition is valid.
    pub valid: bool,
    /// Human-readable reason (populated on rejection).
    pub reason: String,
    /// Gas consumed by the validator execution.
    pub gas_used: u64,
}

/// Execution context passed to the constraint validator.
#[derive(Debug, Clone)]
pub struct ValidationContext {
    pub epoch: u64,
    pub sender: [u8; 32],
    pub gas_remaining: u64,
    pub method_selector: String,
}

/// Trait for invoking a contract's constraint validator.
///
/// Implementations:
/// - `WazeroValidatorBridge`: production — calls Go chain's wazero runtime
/// - `MockValidatorBridge`: testing — returns configurable results
pub trait ConstraintValidatorBridge: Send + Sync {
    /// Validate a proposed state transition.
    ///
    /// # Arguments
    /// * `schema_id` — identifies the contract and its Wasm bytecode
    /// * `proposed_state` — the solver's proposed new state bytes
    /// * `current_state` — the current contract object state bytes
    /// * `intent_params` — serialized intent parameters
    /// * `context` — execution context (epoch, sender, gas)
    fn validate(
        &self,
        schema_id: &SchemaId,
        proposed_state: &[u8],
        current_state: &[u8],
        intent_params: &[u8],
        context: &ValidationContext,
    ) -> ConstraintResult;
}

/// Mock validator for testing. Always returns the configured result.
pub struct MockValidatorBridge {
    pub default_valid: bool,
    pub default_reason: String,
    pub default_gas: u64,
}

impl MockValidatorBridge {
    pub fn accepting() -> Self {
        MockValidatorBridge {
            default_valid: true,
            default_reason: String::new(),
            default_gas: 100,
        }
    }

    pub fn rejecting(reason: &str) -> Self {
        MockValidatorBridge {
            default_valid: false,
            default_reason: reason.to_string(),
            default_gas: 50,
        }
    }
}

impl ConstraintValidatorBridge for MockValidatorBridge {
    fn validate(
        &self,
        _schema_id: &SchemaId,
        _proposed_state: &[u8],
        _current_state: &[u8],
        _intent_params: &[u8],
        _context: &ValidationContext,
    ) -> ConstraintResult {
        ConstraintResult {
            valid: self.default_valid,
            reason: self.default_reason.clone(),
            gas_used: self.default_gas,
        }
    }
}
