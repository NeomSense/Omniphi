//! Omniphi Intent Contracts (OIC)
//!
//! Contracts in Omniphi are Goal Schemas — declarative descriptions of valid
//! state transitions, constraints, and permissions. Solvers compete to fulfill
//! contract intents; the constraint validator (Wasm) only validates proposed
//! transitions, never executes them directly.

pub mod schema;
pub mod validator;
pub mod validation_cache;
pub mod wasm_engine;
pub mod wazero_bridge;
pub mod advanced;
