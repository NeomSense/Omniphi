//! Phase 3 — Deterministic failure path semantics.
//!
//! Defines how the system behaves under each failure condition.
//! All failure paths must be deterministic across nodes.

pub mod intent_failures;
pub mod solver_failures;
pub mod sequencing_failures;
