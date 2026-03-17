//! Intent Pool — Sections 2-3 of the Omniphi Intent Execution Architecture.
//!
//! This module provides:
//! - Core intent types (IntentTransaction, SwapIntent, PaymentIntent, RouteLiquidityIntent)
//! - Intent lifecycle state machine (Created → Settled)
//! - Public intent pool with admission, dedup, expiry, anti-spam
//! - Solver subscription and filtering
//! - Protocol constants

pub mod config;
pub mod constants;
pub mod lifecycle;
pub mod pool;
pub mod types;

pub use constants::*;
pub use lifecycle::{IntentLifecycleRecord, IntentState, TransitionError};
pub use pool::{IntentPool, PoolEntry, PoolMetrics, SolverSubscription};
pub use types::{
    AssetId, AssetType, ExecutionPrefs, IntentAnnouncement, IntentKind, IntentTransaction,
    IntentValidationError, PaymentIntent, PermissionMode, Priority, RebalanceParams,
    RouteLiquidityIntent, SolverPermissions, SwapIntent,
};
