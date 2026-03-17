//! Commit-Reveal Solver Auction — Section 5 of the Omniphi Intent Execution Architecture.
//!
//! This module provides:
//! - BundleCommitment and BundleReveal types with hash verification
//! - AuctionWindow state machine (commit → reveal → selection phases)
//! - No-reveal penalty tracking
//! - Intent-grouped reveal access for ordering

pub mod da_checks;
pub mod ordering;
pub mod state;
pub mod types;

pub use da_checks::{validate_da, DAFailureTracker, DAValidationResult, DAFailure};
pub use ordering::{order_bundles, order_bundles_with_meta, compute_sequence_root, IntentOrderingResult, IntentOrderingMeta};
pub use state::{AuctionError, AuctionPhase, AuctionWindow, NoRevealRecord};
pub use types::{
    BundleCommitment, BundleReveal, ExecutionStep, FeeBreakdown, LiquiditySource,
    LiquidityType, OperationParams, OperationType, PredictedOutput, RevealValidationError,
    SequencedBundle,
};
