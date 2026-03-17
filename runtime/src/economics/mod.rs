//! Slashing & Rewards — Section 12 of the Omniphi Intent Execution Architecture.
//!
//! Economic enforcement layer: solver/validator penalties, reward distribution,
//! bond management, and violation escalation.

pub mod slashing;
pub mod rewards;
pub mod bonding;

pub use slashing::{SolverPenalty, PenaltySeverity, ViolationRecord};
pub use rewards::{EpochRewardDistribution, RewardRecipient};
pub use bonding::{BondState, UnbondingEntry};
