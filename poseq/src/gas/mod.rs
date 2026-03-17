//! PoSeq Gas Model — Fairness-aware sequencing fee architecture.
//!
//! This module implements the PoSeq-native fee model that prices sequencing scarcity
//! while preserving fairness guarantees. It is one of three distinct fee surfaces
//! in Omniphi:
//!
//! - **PoSeq fee**: ordering scarcity (this module)
//! - **Runtime fee**: execution scarcity (runtime::gas)
//! - **Control-chain fee**: governance/accountability actions (Cosmos)
//!
//! All fees are denominated in OMNI. Fees influence sequencing priority only within
//! fairness bounds — PoSeq is never a pure highest-fee-wins auction.

pub mod types;
pub mod calculator;
pub mod parameters;
pub mod routing;
pub mod priority;
pub mod accounting;
pub mod lifecycle;
pub mod estimation;
pub mod simulation;
pub mod stress;
pub mod solver_market;
