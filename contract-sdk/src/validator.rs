//! Constraint validator trait and dispatch.
//!
//! Contract developers implement `ContractValidator` for their state type.
//! The SDK handles serialization, deserialization, and Wasm entry point generation.

use crate::types::{ConstraintResult, ValidationContext};
use serde::{de::DeserializeOwned, Serialize};

/// Trait that contract developers implement.
///
/// Each method corresponds to an intent the contract supports.
/// The validator receives the current state, proposed state, and context,
/// then returns accept/reject.
pub trait ContractValidator: Sized + Serialize + DeserializeOwned {
    /// Validate a proposed state transition.
    ///
    /// # Arguments
    /// * `ctx` — execution context (sender, epoch, method, params)
    /// * `current` — the current on-chain state
    /// * `proposed` — the solver's proposed new state
    ///
    /// # Returns
    /// `ConstraintResult::accept()` or `ConstraintResult::reject(reason)`
    fn validate(
        ctx: &ValidationContext,
        current: &Self,
        proposed: &Self,
    ) -> ConstraintResult;

    /// Optional: validate specifically for a named method.
    /// Default implementation delegates to `validate()`.
    fn validate_method(
        method: &str,
        ctx: &ValidationContext,
        current: &Self,
        proposed: &Self,
    ) -> ConstraintResult {
        let _ = method;
        Self::validate(ctx, current, proposed)
    }
}

/// Dispatch a validation call by deserializing inputs and calling the validator.
///
/// This is used by the Wasm entry point. Contract developers don't call this directly.
pub fn dispatch_validation<T: ContractValidator>(
    current_bytes: &[u8],
    proposed_bytes: &[u8],
    context_bytes: &[u8],
) -> ConstraintResult {
    // Deserialize current state
    let current: T = match serde_json::from_slice(current_bytes) {
        Ok(s) => s,
        Err(e) => return ConstraintResult::reject(&format!("failed to decode current state: {}", e)),
    };

    // Deserialize proposed state
    let proposed: T = match serde_json::from_slice(proposed_bytes) {
        Ok(s) => s,
        Err(e) => return ConstraintResult::reject(&format!("failed to decode proposed state: {}", e)),
    };

    // Deserialize context
    let ctx: ValidationContext = match serde_json::from_slice(context_bytes) {
        Ok(c) => c,
        Err(e) => return ConstraintResult::reject(&format!("failed to decode context: {}", e)),
    };

    // Dispatch to method-specific or generic validator
    T::validate_method(&ctx.method, &ctx, &current, &proposed)
}
