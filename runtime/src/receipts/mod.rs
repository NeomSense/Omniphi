//! Enhanced Execution Receipts — Section 10 of the Omniphi Intent Execution Architecture.
//!
//! Extends the base ExecutionReceipt with intent-specific metadata, indexing,
//! and state proofs needed for disputes and auditing.

pub mod indexing;

pub use indexing::{ReceiptIndex, IntentExecutionReceipt};
