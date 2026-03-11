//! Omniphi Object + Intent Runtime
//!
//! A production-grade blockchain execution engine using:
//! - Object-based state model
//! - Intent-based transactions
//! - Capability permission system
//! - Parallel execution compatible with PoSeq consensus

pub mod capabilities;
pub mod errors;
pub mod gas;
pub mod intents;
pub mod objects;
pub mod poseq;
pub mod resolution;
pub mod scheduler;
pub mod settlement;
pub mod state;

// Convenient top-level re-exports
pub use capabilities::{Capability, CapabilityChecker, CapabilitySet};
pub use errors::RuntimeError;
pub use gas::{GasCost, GasCosts, GasMeter};
pub use intents::{IntentTransaction, IntentType, SwapIntent, TransferIntent,
                  TreasuryRebalanceIntent, YieldAllocateIntent};
pub use objects::{AccessMode, BalanceObject, BoxedObject, GovernanceProposalObject,
                  IdentityObject, LiquidityPoolObject, Object, ObjectAccess, ObjectId,
                  ObjectMeta, ObjectType, ObjectVersion, ProposalStatus, TokenObject,
                  VaultObject, WalletObject, ExecutionReceiptObject};
pub use poseq::{OrderedBatch, PoSeqRuntime};
pub use resolution::{ExecutionPlan, IntentResolver, ObjectOperation};
pub use scheduler::{ConflictGraph, ExecutionGroup, ParallelScheduler};
pub use settlement::{ExecutionReceipt, SettlementEngine, SettlementResult};
pub use state::ObjectStore;
