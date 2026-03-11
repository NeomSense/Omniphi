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
        }
    }
}

impl std::error::Error for RuntimeError {}
