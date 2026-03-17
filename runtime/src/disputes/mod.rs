//! Dispute System — Section 11 of the Omniphi Intent Execution Architecture.
//!
//! Provides fraud proof types, submission, and auto-verification for Phase 1.
//! Fast disputes (100 blocks) with auto-verifiable proofs.

pub mod types;
pub mod verifier;

pub use types::{DisputeRecord, DisputeState, FraudProof, FraudProofType};
pub use verifier::{DisputeVerifier, required_bond_for_proof};
