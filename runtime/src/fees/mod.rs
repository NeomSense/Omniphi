//! Omniphi 3-Surface Fee Architecture
//!
//! Three independent resource markets, each pricing a different scarce resource:
//!
//! 1. **PoSeq Sequencing Fee** — prices ordering scarcity
//!    - Admission, bandwidth, congestion, bounded priority
//!    - Anti-spam economics
//!    - Fairness-bounded (fees cannot override ordering policy)
//!
//! 2. **Runtime Execution Fee** — prices execution scarcity
//!    - State reads/writes, deterministic execution, settlement
//!    - Object-based operation costs
//!    - Batch complexity scaling
//!    - (Existing `gas/meter.rs` — reclassified, not replaced)
//!
//! 3. **Control / Anchor Lane Fee** — prices governance/registry scarcity
//!    - Go chain EIP-1559 dynamic base fee
//!    - Validator + treasury split, adaptive burn tiers
//!    - (Existing Go feemarket module — untouched)
//!
//! ## User Cost Model
//!
//! ```text
//! Total Fee = PoSeq Fee + Runtime Fee + (Control Fee when applicable)
//! ```
//!
//! Most user flows: `Total = PoSeq Fee + Runtime Fee`
//! Control fee only on anchoring/governance/registry transactions.

pub mod poseq_fee;
pub mod envelope;
pub mod accounting;
pub mod routing;

pub use poseq_fee::{PoSeqFeeParams, PoSeqFeeCalculator, PoSeqFeeResult, CongestionState};
pub use envelope::FeeEnvelope;
pub use accounting::{FeeAccounting, FeeAccountingState, ChargeResult};
pub use routing::{FeeRouting, FeeRoutingParams, RoutedFees};
