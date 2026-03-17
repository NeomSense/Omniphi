//! Settlement Verification — Section 9 of the Omniphi Intent Execution Architecture.
//!
//! Post-execution constraint verification for each intent type.
//! The runtime replays solver execution plans and independently verifies
//! that the result satisfies all user-declared constraints.

pub mod types;
pub mod swap;
pub mod payment;
pub mod route;

pub use types::{VerificationError, VerificationResult};
