pub mod store;

// Phase 4 persistence abstractions
pub mod backend;
pub mod keys;
pub mod engine;

// Phase 5: durable backend and crash recovery
pub mod sled_backend;
pub mod durable_store;
pub mod reconciler;

// Crash/restart durability tests (compiled only in test mode)
#[cfg(test)]
mod crash_tests;

pub use durable_store::{DurableStore, StorageError, CURRENT_SCHEMA_VERSION};
pub use reconciler::{ReconciliationEngine, ReconciliationAction, ReconciliationReport};
pub use sled_backend::SledBackend;
