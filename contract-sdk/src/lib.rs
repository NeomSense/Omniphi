//! Omniphi Intent Contract SDK
//!
//! This SDK provides the types, traits, and helpers needed to build Intent
//! Contracts for the Omniphi blockchain. Contracts are constraint schemas —
//! they declare valid state transitions rather than executing them directly.
//!
//! # Architecture
//!
//! An Omniphi Intent Contract consists of:
//! 1. **State struct** — the contract's data model
//! 2. **Intent methods** — what users can request
//! 3. **Constraint validators** — pure functions that approve/reject state transitions
//!
//! Solvers compete to fulfill intents. The constraint validator only checks
//! whether a proposed state transition is valid — it never executes mutations.
//!
//! # Example
//!
//! ```rust
//! use omniphi_contract_sdk::*;
//!
//! #[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
//! struct EscrowState {
//!     depositor: [u8; 32],
//!     beneficiary: [u8; 32],
//!     arbiter: [u8; 32],
//!     amount: u128,
//!     status: String, // "created", "funded", "released", "refunded"
//! }
//!
//! fn validate_release(ctx: &ValidationContext, current: &EscrowState, proposed: &EscrowState) -> ConstraintResult {
//!     if current.status != "funded" {
//!         return ConstraintResult::reject("escrow not funded");
//!     }
//!     if proposed.status != "released" {
//!         return ConstraintResult::reject("proposed status must be 'released'");
//!     }
//!     if ctx.sender != current.arbiter && ctx.sender != current.depositor {
//!         return ConstraintResult::reject("only arbiter or depositor can release");
//!     }
//!     ConstraintResult::accept()
//! }
//! ```

pub mod types;
pub mod validator;
pub mod schema;
pub mod wasm_entry;

pub use types::*;
pub use validator::*;
pub use schema::*;
