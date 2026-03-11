//! Omniphi Object + Intent Runtime
//!
//! A production-grade blockchain execution engine using:
//! - Object-based state model
//! - Intent-based transactions
//! - Capability permission system
//! - Parallel execution compatible with PoSeq consensus
//!
//! Phase 2 adds:
//! - Solver Market with multi-solver plan submission and selection
//! - AI/Agent-ready intent engine with policy envelopes
//! - Plan validation, ranking, and attribution

// Phase 1 modules
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

// Phase 3: Causal Rights Execution (CRX)
pub mod crx;
pub use crx::*;

// Phase 2 modules
pub mod agent_interfaces;
pub mod attribution;
pub mod matching;
pub mod plan_validation;
pub mod policy;
pub mod selection;
pub mod simulation;
pub mod solver_market;
pub mod solver_registry;

// Phase 1 re-exports
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

// Phase 2 re-exports
pub use agent_interfaces::{
    AgentPolicyEnvelope, AgentSolverProfile, AgentType, DeterministicAgentSubmission,
    ExplanationFormat, PlanExplanationMetadata,
};
pub use attribution::{PlanOutcomeRecord, SelectionAuditRecord, SolverAttributionRecord};
pub use matching::IntentSolverMatcher;
pub use plan_validation::{PlanValidator, ValidationReasonCode};
pub use policy::{
    CompositePolicyEvaluator, DomainRiskPolicy, MaxValuePolicy, ObjectBlocklistPolicy,
    PermissivePolicy, PlanPolicyEvaluator, SolverSafetyPolicy,
};
pub use poseq::{FinalSettlement, SelectedPlanResult, SolverMarketBatch, SolverMarketRuntime};
pub use selection::{PlanRanker, SelectionPolicy, WinningPlanSelector};
pub use simulation::{PlanSimulator, SimulationResult};
pub use solver_market::{
    CandidatePlan, PlanAction, PlanActionType, PlanConstraintProof, PlanEvaluationResult,
};
pub use solver_registry::{
    SolverCapabilities, SolverProfile, SolverRegistry, SolverReputationRecord, SolverStatus,
};
