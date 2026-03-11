use crate::capabilities::checker::{Capability, CapabilitySet};
use crate::objects::base::ObjectId;
use std::fmt;

#[derive(Debug, Clone)]
pub enum RuntimeError {
    InvalidIntent(String),
    UnauthorizedCapability {
        required: Capability,
        held: CapabilitySet,
    },
    ObjectNotFound(ObjectId),
    InsufficientBalance {
        required: u128,
        available: u128,
    },
    ConstraintViolation(String),
    SchedulerConflict(String),
    ResolutionFailure(String),
    SettlementFailure(String),
    // Safety extension points (not yet implemented):
    ObjectQuarantined(ObjectId),
    DomainPaused(String),
    // Phase 2: Solver Market errors
    SolverNotRegistered(String),
    SolverNotEligible { solver_id: String, reason: String },
    NoCandidatePlans,
    AllPlansInvalid,
    PlanValidationFailed { plan_id: [u8; 32], reason: String },
    PolicyViolation(String),
    SimulationError(String),
    // Phase 3: Causal Rights Execution (CRX) errors
    GoalPacketInvalid(String),
    RightsCapsuleViolation { capsule_id: [u8; 32], reason: String },
    CausalViolation(String),
    BranchExecutionFailed { branch_id: u32, reason: String },
    FinalityEscalationRequired(String),
    CRXSettlementFailed(String),
}

impl fmt::Display for RuntimeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            RuntimeError::InvalidIntent(msg) => write!(f, "InvalidIntent: {}", msg),
            RuntimeError::UnauthorizedCapability { required, held: _ } => {
                write!(f, "UnauthorizedCapability: missing {:?}", required)
            }
            RuntimeError::ObjectNotFound(id) => write!(f, "ObjectNotFound: {}", id),
            RuntimeError::InsufficientBalance { required, available } => {
                write!(
                    f,
                    "InsufficientBalance: required={}, available={}",
                    required, available
                )
            }
            RuntimeError::ConstraintViolation(msg) => write!(f, "ConstraintViolation: {}", msg),
            RuntimeError::SchedulerConflict(msg) => write!(f, "SchedulerConflict: {}", msg),
            RuntimeError::ResolutionFailure(msg) => write!(f, "ResolutionFailure: {}", msg),
            RuntimeError::SettlementFailure(msg) => write!(f, "SettlementFailure: {}", msg),
            RuntimeError::ObjectQuarantined(id) => write!(f, "ObjectQuarantined: {}", id),
            RuntimeError::DomainPaused(domain) => write!(f, "DomainPaused: {}", domain),
            RuntimeError::SolverNotRegistered(id) => write!(f, "SolverNotRegistered: {}", id),
            RuntimeError::SolverNotEligible { solver_id, reason } => {
                write!(f, "SolverNotEligible: solver={} reason={}", solver_id, reason)
            }
            RuntimeError::NoCandidatePlans => write!(f, "NoCandidatePlans"),
            RuntimeError::AllPlansInvalid => write!(f, "AllPlansInvalid"),
            RuntimeError::PlanValidationFailed { plan_id, reason } => {
                write!(f, "PlanValidationFailed: plan={} reason={}", hex::encode(plan_id), reason)
            }
            RuntimeError::PolicyViolation(msg) => write!(f, "PolicyViolation: {}", msg),
            RuntimeError::SimulationError(msg) => write!(f, "SimulationError: {}", msg),
            RuntimeError::GoalPacketInvalid(msg) => write!(f, "GoalPacketInvalid: {}", msg),
            RuntimeError::RightsCapsuleViolation { capsule_id, reason } => {
                write!(f, "RightsCapsuleViolation: capsule={} reason={}", hex::encode(capsule_id), reason)
            }
            RuntimeError::CausalViolation(msg) => write!(f, "CausalViolation: {}", msg),
            RuntimeError::BranchExecutionFailed { branch_id, reason } => {
                write!(f, "BranchExecutionFailed: branch={} reason={}", branch_id, reason)
            }
            RuntimeError::FinalityEscalationRequired(msg) => write!(f, "FinalityEscalationRequired: {}", msg),
            RuntimeError::CRXSettlementFailed(msg) => write!(f, "CRXSettlementFailed: {}", msg),
        }
    }
}

impl std::error::Error for RuntimeError {}
