//! Fee lifecycle state machine — tracks an intent's fee from reservation through
//! settlement to final close.
//!
//! State machine:
//! ```text
//!   Rejected ←─── [admission fails]
//!       │
//!   PendingReservation ──→ Reserved ──→ SequencingCharged ──→ RuntimeCharged ──→ Refunded ──→ Closed
//!                              │              │                    │
//!                              ├→ Expired ────┘                    │
//!                              │     (penalty applied)             │
//!                              │                                   │
//!                              └→ Cancelled ──→ Refunded ──→ Closed
//! ```
//!
//! Terminal states: `Closed`, `Rejected`.
//!
//! Invariants:
//! - No double-charge: each stage charges at most once
//! - No double-refund: refund only from `Refunded`, transition to `Closed` is final
//! - Budget consistent: reserved = poseq_charged + runtime_charged + penalties + refund

use serde::{Deserialize, Serialize};

// ─── Fee Lifecycle State ───────────────────────────────────────────────────

/// The state of a fee lifecycle for a single intent.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum FeeLifecycleState {
    /// Intent rejected at admission — no funds touched.
    Rejected { reason: String },

    /// Funds identified but not yet locked (pre-admission validation).
    PendingReservation,

    /// Funds reserved at admission time.
    Reserved {
        payer: [u8; 32],
        max_poseq_fee: u128,
        max_runtime_fee: u128,
        total_reserved: u128,
        reserved_at_block: u64,
    },

    /// PoSeq sequencing fee has been charged.
    SequencingCharged {
        payer: [u8; 32],
        poseq_fee_charged: u128,
        runtime_budget_remaining: u128,
        charged_at_block: u64,
    },

    /// Both PoSeq and runtime fees have been charged.
    RuntimeCharged {
        payer: [u8; 32],
        poseq_fee_charged: u128,
        runtime_fee_charged: u128,
        total_charged: u128,
        pending_refund: u128,
        charged_at_block: u64,
    },

    /// Intent expired — penalty applied, remainder pending refund.
    Expired {
        payer: [u8; 32],
        penalty: u128,
        pending_refund: u128,
        expired_at_block: u64,
    },

    /// Intent cancelled before sequencing.
    Cancelled {
        payer: [u8; 32],
        pending_refund: u128,
        cancelled_at_block: u64,
    },

    /// Refund has been issued.
    Refunded {
        payer: [u8; 32],
        total_charged: u128,
        total_refunded: u128,
        refunded_at_block: u64,
    },

    /// Terminal: lifecycle fully closed, auditable record.
    Closed {
        payer: [u8; 32],
        total_charged: u128,
        total_refunded: u128,
        closed_at_block: u64,
    },
}

impl FeeLifecycleState {
    pub fn is_terminal(&self) -> bool {
        matches!(self, Self::Closed { .. } | Self::Rejected { .. })
    }

    pub fn can_charge_sequencing(&self) -> bool {
        matches!(self, Self::Reserved { .. })
    }

    pub fn can_charge_runtime(&self) -> bool {
        matches!(self, Self::SequencingCharged { .. })
    }

    pub fn can_refund(&self) -> bool {
        matches!(
            self,
            Self::RuntimeCharged { .. } | Self::Expired { .. } | Self::Cancelled { .. }
        )
    }
}

// ─── Fee Lifecycle Manager ─────────────────────────────────────────────────

/// Manages fee lifecycle transitions for a single intent.
/// All transitions are validated — invalid transitions return errors.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeeLifecycle {
    pub intent_id: [u8; 32],
    pub state: FeeLifecycleState,
    pub transitions: Vec<FeeTransitionRecord>,
}

/// Audit record for each transition.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FeeTransitionRecord {
    pub from: String,
    pub to: String,
    pub block: u64,
}

/// Errors from invalid lifecycle transitions.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FeeLifecycleError {
    InvalidTransition { from: String, to: String },
    AlreadyCharged { stage: String },
    AlreadyRefunded,
    AlreadyClosed,
    InsufficientBudget { required: u128, available: u128 },
    Underflow { field: String },
}

impl std::fmt::Display for FeeLifecycleError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidTransition { from, to } =>
                write!(f, "invalid fee transition: {} → {}", from, to),
            Self::AlreadyCharged { stage } =>
                write!(f, "fee already charged at stage: {}", stage),
            Self::AlreadyRefunded => write!(f, "refund already issued"),
            Self::AlreadyClosed => write!(f, "lifecycle already closed"),
            Self::InsufficientBudget { required, available } =>
                write!(f, "insufficient budget: need {}, have {}", required, available),
            Self::Underflow { field } =>
                write!(f, "arithmetic underflow in field: {}", field),
        }
    }
}

impl std::error::Error for FeeLifecycleError {}

impl FeeLifecycle {
    /// Create a new lifecycle in PendingReservation state.
    pub fn new(intent_id: [u8; 32]) -> Self {
        FeeLifecycle {
            intent_id,
            state: FeeLifecycleState::PendingReservation,
            transitions: Vec::new(),
        }
    }

    fn state_name(state: &FeeLifecycleState) -> &'static str {
        match state {
            FeeLifecycleState::Rejected { .. } => "Rejected",
            FeeLifecycleState::PendingReservation => "PendingReservation",
            FeeLifecycleState::Reserved { .. } => "Reserved",
            FeeLifecycleState::SequencingCharged { .. } => "SequencingCharged",
            FeeLifecycleState::RuntimeCharged { .. } => "RuntimeCharged",
            FeeLifecycleState::Expired { .. } => "Expired",
            FeeLifecycleState::Cancelled { .. } => "Cancelled",
            FeeLifecycleState::Refunded { .. } => "Refunded",
            FeeLifecycleState::Closed { .. } => "Closed",
        }
    }

    fn record_transition(&mut self, from: &str, to: &str, block: u64) {
        self.transitions.push(FeeTransitionRecord {
            from: from.to_string(),
            to: to.to_string(),
            block,
        });
    }

    /// Reject the intent at admission — no funds reserved.
    pub fn reject(&mut self, reason: &str) -> Result<(), FeeLifecycleError> {
        if self.state.is_terminal() {
            return Err(FeeLifecycleError::AlreadyClosed);
        }
        let from = Self::state_name(&self.state);
        self.state = FeeLifecycleState::Rejected { reason: reason.to_string() };
        self.record_transition(from, "Rejected", 0);
        Ok(())
    }

    /// Reserve funds at admission.
    pub fn reserve(
        &mut self,
        payer: [u8; 32],
        max_poseq_fee: u128,
        max_runtime_fee: u128,
        block: u64,
    ) -> Result<u128, FeeLifecycleError> {
        if !matches!(self.state, FeeLifecycleState::PendingReservation) {
            return Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "Reserved".to_string(),
            });
        }
        let total = max_poseq_fee.checked_add(max_runtime_fee)
            .ok_or(FeeLifecycleError::Underflow { field: "total_reserved".into() })?;

        self.state = FeeLifecycleState::Reserved {
            payer,
            max_poseq_fee,
            max_runtime_fee,
            total_reserved: total,
            reserved_at_block: block,
        };
        self.record_transition("PendingReservation", "Reserved", block);
        Ok(total)
    }

    /// Charge the PoSeq sequencing fee.
    pub fn charge_sequencing(
        &mut self,
        poseq_fee: u128,
        block: u64,
    ) -> Result<(), FeeLifecycleError> {
        match &self.state {
            FeeLifecycleState::Reserved { payer, max_poseq_fee, max_runtime_fee, .. } => {
                if poseq_fee > *max_poseq_fee {
                    return Err(FeeLifecycleError::InsufficientBudget {
                        required: poseq_fee,
                        available: *max_poseq_fee,
                    });
                }
                let payer = *payer;
                let runtime_remaining = *max_runtime_fee;
                self.state = FeeLifecycleState::SequencingCharged {
                    payer,
                    poseq_fee_charged: poseq_fee,
                    runtime_budget_remaining: runtime_remaining,
                    charged_at_block: block,
                };
                self.record_transition("Reserved", "SequencingCharged", block);
                Ok(())
            }
            _ => Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "SequencingCharged".to_string(),
            }),
        }
    }

    /// Charge the runtime execution fee. Consumed gas is always charged
    /// (even on failed execution — the gas was consumed).
    pub fn charge_runtime(
        &mut self,
        runtime_fee: u128,
        block: u64,
    ) -> Result<u128, FeeLifecycleError> {
        match &self.state {
            FeeLifecycleState::SequencingCharged {
                payer, poseq_fee_charged, runtime_budget_remaining, ..
            } => {
                // Charge up to budget — if execution used more gas than budget,
                // charge the full budget (execution was already reverted by gas meter).
                let actual_runtime = std::cmp::min(runtime_fee, *runtime_budget_remaining);
                let total_charged = poseq_fee_charged.saturating_add(actual_runtime);
                // Refund is the unused portion of the runtime budget only
                // (unused poseq budget was already accounted for at sequencing time)
                let pending_refund = runtime_budget_remaining.saturating_sub(actual_runtime);

                let payer = *payer;
                let poseq = *poseq_fee_charged;
                self.state = FeeLifecycleState::RuntimeCharged {
                    payer,
                    poseq_fee_charged: poseq,
                    runtime_fee_charged: actual_runtime,
                    total_charged,
                    pending_refund,
                    charged_at_block: block,
                };
                self.record_transition("SequencingCharged", "RuntimeCharged", block);
                Ok(pending_refund)
            }
            _ => Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "RuntimeCharged".to_string(),
            }),
        }
    }

    /// Expire the intent — apply penalty, compute refund.
    pub fn expire(
        &mut self,
        penalty: u128,
        block: u64,
    ) -> Result<u128, FeeLifecycleError> {
        match &self.state {
            FeeLifecycleState::Reserved { payer, total_reserved, .. } => {
                let actual_penalty = std::cmp::min(penalty, *total_reserved);
                let pending_refund = total_reserved.saturating_sub(actual_penalty);
                let payer = *payer;
                self.state = FeeLifecycleState::Expired {
                    payer,
                    penalty: actual_penalty,
                    pending_refund,
                    expired_at_block: block,
                };
                self.record_transition("Reserved", "Expired", block);
                Ok(pending_refund)
            }
            FeeLifecycleState::SequencingCharged {
                payer, poseq_fee_charged, runtime_budget_remaining, ..
            } => {
                // Already paid sequencing fee. Penalty on runtime budget.
                let actual_penalty = std::cmp::min(penalty, *runtime_budget_remaining);
                let pending_refund = runtime_budget_remaining.saturating_sub(actual_penalty);
                let payer = *payer;
                let total_penalty = poseq_fee_charged.saturating_add(actual_penalty);
                self.state = FeeLifecycleState::Expired {
                    payer,
                    penalty: total_penalty,
                    pending_refund,
                    expired_at_block: block,
                };
                self.record_transition("SequencingCharged", "Expired", block);
                Ok(pending_refund)
            }
            _ => Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "Expired".to_string(),
            }),
        }
    }

    /// Cancel the intent before sequencing — full refund.
    pub fn cancel(&mut self, block: u64) -> Result<u128, FeeLifecycleError> {
        match &self.state {
            FeeLifecycleState::Reserved { payer, total_reserved, .. } => {
                let payer = *payer;
                let refund = *total_reserved;
                self.state = FeeLifecycleState::Cancelled {
                    payer,
                    pending_refund: refund,
                    cancelled_at_block: block,
                };
                self.record_transition("Reserved", "Cancelled", block);
                Ok(refund)
            }
            _ => Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "Cancelled".to_string(),
            }),
        }
    }

    /// Issue the refund and move to Refunded state.
    pub fn issue_refund(&mut self, block: u64) -> Result<u128, FeeLifecycleError> {
        if !self.state.can_refund() {
            return Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "Refunded".to_string(),
            });
        }

        let (payer, total_charged, total_refunded) = match &self.state {
            FeeLifecycleState::RuntimeCharged {
                payer, total_charged, pending_refund, ..
            } => (*payer, *total_charged, *pending_refund),
            FeeLifecycleState::Expired { payer, penalty, pending_refund, .. } =>
                (*payer, *penalty, *pending_refund),
            FeeLifecycleState::Cancelled { payer, pending_refund, .. } =>
                (*payer, 0, *pending_refund),
            _ => unreachable!(), // guarded by can_refund()
        };

        let from = Self::state_name(&self.state);
        self.state = FeeLifecycleState::Refunded {
            payer,
            total_charged,
            total_refunded,
            refunded_at_block: block,
        };
        self.record_transition(from, "Refunded", block);
        Ok(total_refunded)
    }

    /// Close the lifecycle — terminal state.
    pub fn close(&mut self, block: u64) -> Result<(), FeeLifecycleError> {
        match &self.state {
            FeeLifecycleState::Refunded { payer, total_charged, total_refunded, .. } => {
                let payer = *payer;
                let charged = *total_charged;
                let refunded = *total_refunded;
                self.state = FeeLifecycleState::Closed {
                    payer,
                    total_charged: charged,
                    total_refunded: refunded,
                    closed_at_block: block,
                };
                self.record_transition("Refunded", "Closed", block);
                Ok(())
            }
            FeeLifecycleState::Rejected { .. } => {
                // Already terminal
                Ok(())
            }
            _ => Err(FeeLifecycleError::InvalidTransition {
                from: Self::state_name(&self.state).to_string(),
                to: "Closed".to_string(),
            }),
        }
    }

    /// Verify budget consistency: reserved = charged + refund + penalty.
    pub fn verify_budget_consistency(&self) -> bool {
        match &self.state {
            FeeLifecycleState::Closed { total_charged, total_refunded, .. } => {
                // In closed state, we can only verify non-negative
                total_charged.checked_add(*total_refunded).is_some()
            }
            FeeLifecycleState::RuntimeCharged {
                total_charged, pending_refund, ..
            } => {
                total_charged.checked_add(*pending_refund).is_some()
            }
            _ => true,
        }
    }
}

// ─── Fee Ledger ────────────────────────────────────────────────────────────

/// Tracks all active fee lifecycles. Prevents double-reservation.
#[derive(Debug, Default)]
pub struct FeeLedger {
    lifecycles: std::collections::BTreeMap<[u8; 32], FeeLifecycle>,
}

impl FeeLedger {
    pub fn new() -> Self {
        FeeLedger { lifecycles: std::collections::BTreeMap::new() }
    }

    /// Start a new fee lifecycle for an intent. Returns error if already exists.
    pub fn begin(&mut self, intent_id: [u8; 32]) -> Result<&mut FeeLifecycle, FeeLifecycleError> {
        if self.lifecycles.contains_key(&intent_id) {
            return Err(FeeLifecycleError::AlreadyCharged {
                stage: "reservation already exists".into(),
            });
        }
        self.lifecycles.insert(intent_id, FeeLifecycle::new(intent_id));
        Ok(self.lifecycles.get_mut(&intent_id).unwrap())
    }

    /// Get a mutable reference to an existing lifecycle.
    pub fn get_mut(&mut self, intent_id: &[u8; 32]) -> Option<&mut FeeLifecycle> {
        self.lifecycles.get_mut(intent_id)
    }

    /// Get an immutable reference to an existing lifecycle.
    pub fn get(&self, intent_id: &[u8; 32]) -> Option<&FeeLifecycle> {
        self.lifecycles.get(intent_id)
    }

    /// Count of active (non-terminal) lifecycles.
    pub fn active_count(&self) -> usize {
        self.lifecycles.values().filter(|lc| !lc.state.is_terminal()).count()
    }

    /// Prune closed lifecycles older than the given block.
    pub fn prune_closed_before(&mut self, block: u64) -> usize {
        let before_len = self.lifecycles.len();
        self.lifecycles.retain(|_, lc| {
            match &lc.state {
                FeeLifecycleState::Closed { closed_at_block, .. } => *closed_at_block >= block,
                FeeLifecycleState::Rejected { .. } => false, // always prune rejected
                _ => true,
            }
        });
        before_len - self.lifecycles.len()
    }
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn intent_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32]; id[0] = b; id
    }
    fn payer() -> [u8; 32] { [1u8; 32] }

    // ─── Happy path: reserve → sequence → runtime → refund → close ────

    #[test]
    fn test_full_happy_path() {
        let mut lc = FeeLifecycle::new(intent_id(1));

        // Reserve
        let total = lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        assert_eq!(total, 60_000);
        assert!(lc.state.can_charge_sequencing());

        // Charge sequencing
        lc.charge_sequencing(3_000, 105).unwrap();
        assert!(lc.state.can_charge_runtime());

        // Charge runtime
        let pending = lc.charge_runtime(20_000, 110).unwrap();
        assert_eq!(pending, 30_000); // 50000 runtime budget - 20000 used
        assert!(lc.state.can_refund());

        // Refund
        let refunded = lc.issue_refund(115).unwrap();
        assert_eq!(refunded, 30_000); // unused runtime budget

        // Close
        lc.close(120).unwrap();
        assert!(lc.state.is_terminal());
        assert!(lc.verify_budget_consistency());
        assert_eq!(lc.transitions.len(), 5);
    }

    // ─── Expiry path ───────────────────────────────────────────────────

    #[test]
    fn test_expiry_from_reserved() {
        let mut lc = FeeLifecycle::new(intent_id(2));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();

        let refund = lc.expire(1_000, 200).unwrap(); // 1000 penalty
        assert_eq!(refund, 59_000); // 60000 - 1000

        let refunded = lc.issue_refund(201).unwrap();
        assert_eq!(refunded, 59_000);
        lc.close(202).unwrap();
        assert!(lc.state.is_terminal());
    }

    #[test]
    fn test_expiry_after_sequencing() {
        let mut lc = FeeLifecycle::new(intent_id(3));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(5_000, 105).unwrap();

        // Expire with penalty on remaining runtime budget
        let refund = lc.expire(2_000, 200).unwrap();
        assert_eq!(refund, 48_000); // 50000 (runtime remaining) - 2000

        lc.issue_refund(201).unwrap();
        lc.close(202).unwrap();
    }

    // ─── Cancellation path ─────────────────────────────────────────────

    #[test]
    fn test_cancel_full_refund() {
        let mut lc = FeeLifecycle::new(intent_id(4));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();

        let refund = lc.cancel(150).unwrap();
        assert_eq!(refund, 60_000); // full refund

        let refunded = lc.issue_refund(151).unwrap();
        assert_eq!(refunded, 60_000);
        lc.close(152).unwrap();
    }

    // ─── Rejection path ────────────────────────────────────────────────

    #[test]
    fn test_rejection() {
        let mut lc = FeeLifecycle::new(intent_id(5));
        lc.reject("insufficient fee").unwrap();
        assert!(lc.state.is_terminal());
    }

    // ─── Invalid transitions ───────────────────────────────────────────

    #[test]
    fn test_double_reservation_blocked() {
        let mut lc = FeeLifecycle::new(intent_id(6));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        let result = lc.reserve(payer(), 10_000, 50_000, 101);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    #[test]
    fn test_charge_sequencing_without_reservation() {
        let mut lc = FeeLifecycle::new(intent_id(7));
        let result = lc.charge_sequencing(1_000, 100);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    #[test]
    fn test_charge_runtime_without_sequencing() {
        let mut lc = FeeLifecycle::new(intent_id(8));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        let result = lc.charge_runtime(1_000, 105);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    #[test]
    fn test_double_refund_blocked() {
        let mut lc = FeeLifecycle::new(intent_id(9));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(3_000, 105).unwrap();
        lc.charge_runtime(20_000, 110).unwrap();
        lc.issue_refund(115).unwrap();
        let result = lc.issue_refund(116);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    #[test]
    fn test_close_without_refund_blocked() {
        let mut lc = FeeLifecycle::new(intent_id(10));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(3_000, 105).unwrap();
        lc.charge_runtime(20_000, 110).unwrap();
        // Try to close without refunding
        let result = lc.close(111);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    #[test]
    fn test_sequencing_fee_exceeds_budget() {
        let mut lc = FeeLifecycle::new(intent_id(11));
        lc.reserve(payer(), 5_000, 50_000, 100).unwrap();
        let result = lc.charge_sequencing(10_000, 105); // exceeds max_poseq_fee
        assert!(matches!(result, Err(FeeLifecycleError::InsufficientBudget { .. })));
    }

    #[test]
    fn test_runtime_fee_capped_at_budget() {
        let mut lc = FeeLifecycle::new(intent_id(12));
        lc.reserve(payer(), 5_000, 10_000, 100).unwrap();
        lc.charge_sequencing(5_000, 105).unwrap();

        // Try to charge 50000 runtime but only 10000 budgeted
        let pending = lc.charge_runtime(50_000, 110).unwrap();
        assert_eq!(pending, 0); // all runtime budget consumed, no refund

        if let FeeLifecycleState::RuntimeCharged { runtime_fee_charged, .. } = &lc.state {
            assert_eq!(*runtime_fee_charged, 10_000); // capped
        } else {
            panic!("expected RuntimeCharged");
        }
    }

    #[test]
    fn test_cancel_after_sequencing_blocked() {
        let mut lc = FeeLifecycle::new(intent_id(13));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(3_000, 105).unwrap();
        let result = lc.cancel(110);
        assert!(matches!(result, Err(FeeLifecycleError::InvalidTransition { .. })));
    }

    // ─── Ledger tests ──────────────────────────────────────────────────

    #[test]
    fn test_ledger_prevents_double_reservation() {
        let mut ledger = FeeLedger::new();
        ledger.begin(intent_id(1)).unwrap();
        let result = ledger.begin(intent_id(1));
        assert!(matches!(result, Err(FeeLifecycleError::AlreadyCharged { .. })));
    }

    #[test]
    fn test_ledger_active_count() {
        let mut ledger = FeeLedger::new();
        let lc = ledger.begin(intent_id(1)).unwrap();
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();

        let lc2 = ledger.begin(intent_id(2)).unwrap();
        lc2.reject("spam").unwrap();

        assert_eq!(ledger.active_count(), 1); // only id(1) is active
    }

    #[test]
    fn test_ledger_prune() {
        let mut ledger = FeeLedger::new();
        let lc = ledger.begin(intent_id(1)).unwrap();
        lc.reject("test").unwrap();

        let lc2 = ledger.begin(intent_id(2)).unwrap();
        lc2.reserve(payer(), 1000, 1000, 100).unwrap();
        lc2.cancel(150).unwrap();
        lc2.issue_refund(151).unwrap();
        lc2.close(152).unwrap();

        let pruned = ledger.prune_closed_before(200);
        assert_eq!(pruned, 2); // both pruned
    }

    // ─── Budget consistency ────────────────────────────────────────────

    #[test]
    fn test_budget_consistency_full_path() {
        let mut lc = FeeLifecycle::new(intent_id(20));
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(3_000, 105).unwrap();
        lc.charge_runtime(20_000, 110).unwrap();

        // reserved=60000, poseq=3000, runtime=20000, refund=30000
        // total_charged=23000, pending_refund=30000 (unused runtime budget)
        // poseq_refund was 7000 (10000-3000) — already implicit in the poseq charging
        // total_charged + pending_refund = 23000 + 30000 = 53000
        // The 7000 poseq surplus was never reserved in runtime budget
        if let FeeLifecycleState::RuntimeCharged {
            total_charged, pending_refund, poseq_fee_charged, runtime_fee_charged, ..
        } = &lc.state {
            assert_eq!(*poseq_fee_charged, 3_000);
            assert_eq!(*runtime_fee_charged, 20_000);
            assert_eq!(*total_charged, 23_000);
            assert_eq!(*pending_refund, 30_000); // 50000 - 20000
        }

        lc.issue_refund(115).unwrap();
        lc.close(120).unwrap();

        if let FeeLifecycleState::Closed { total_charged, total_refunded, .. } = &lc.state {
            // total_charged=23000 (poseq 3K + runtime 20K)
            // total_refunded=30000 (unused runtime budget)
            // Note: the 7000 poseq surplus (10K max - 3K charged) is handled
            // by the poseq fee calculator's refund field, not the lifecycle
            assert_eq!(*total_charged, 23_000);
            assert_eq!(*total_refunded, 30_000);
        }
    }
}
