//! Phase 5 — Governance parameter management for Omniphi.
//!
//! All adjustable protocol parameters are stored in governance-controlled state.
//! Parameter updates require governance proposals and voting.

pub mod parameters;

pub use parameters::{
    ProtocolParameters, AuctionParameters, FairnessParameters,
    SlashingParameters, DAParameters, IntentPoolParameters,
    GovernanceAction, ParameterUpdateResult,
};
