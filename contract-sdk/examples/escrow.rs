//! Escrow Intent Contract — Reference Implementation
//!
//! Demonstrates a simple escrow where:
//! - Depositor funds the escrow
//! - Arbiter or depositor can release to beneficiary
//! - Arbiter can refund to depositor
//! - Auto-refund after deadline
//!
//! This is a constraint-only contract: it validates state transitions,
//! not execute them. Solvers produce the actual balance operations.

use omniphi_contract_sdk::*;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EscrowStatus {
    Created,
    Funded,
    Released,
    Refunded,
    Disputed,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EscrowState {
    pub depositor: [u8; 32],
    pub beneficiary: [u8; 32],
    pub arbiter: [u8; 32],
    pub amount: u128,
    pub asset_id: [u8; 32],
    pub status: EscrowStatus,
    pub deadline_epoch: u64,
    pub funded_at_epoch: Option<u64>,
}

impl ContractValidator for EscrowState {
    fn validate(
        ctx: &ValidationContext,
        current: &Self,
        proposed: &Self,
    ) -> ConstraintResult {
        Self::validate_method(&ctx.method, ctx, current, proposed)
    }

    fn validate_method(
        method: &str,
        ctx: &ValidationContext,
        current: &Self,
        proposed: &Self,
    ) -> ConstraintResult {
        match method {
            "fund" => validate_fund(ctx, current, proposed),
            "release" => validate_release(ctx, current, proposed),
            "refund" => validate_refund(ctx, current, proposed),
            _ => ConstraintResult::reject(&format!("unknown method: {}", method)),
        }
    }
}

fn validate_fund(
    ctx: &ValidationContext,
    current: &EscrowState,
    proposed: &EscrowState,
) -> ConstraintResult {
    // Can only fund from Created state
    if current.status != EscrowStatus::Created {
        return ConstraintResult::reject("escrow already funded or completed");
    }

    // Must transition to Funded
    if proposed.status != EscrowStatus::Funded {
        return ConstraintResult::reject("proposed status must be Funded");
    }

    // Sender must be the depositor
    if ctx.sender != current.depositor {
        return ConstraintResult::reject("only depositor can fund");
    }

    // Amount must be positive
    if proposed.amount == 0 {
        return ConstraintResult::reject("amount must be > 0");
    }

    // Deadline must be in the future
    if proposed.deadline_epoch <= ctx.epoch {
        return ConstraintResult::reject("deadline must be in the future");
    }

    // Immutable fields must not change
    if proposed.depositor != current.depositor
        || proposed.beneficiary != current.beneficiary
        || proposed.arbiter != current.arbiter
        || proposed.asset_id != current.asset_id
    {
        return ConstraintResult::reject("cannot change depositor/beneficiary/arbiter/asset");
    }

    ConstraintResult::accept()
}

fn validate_release(
    ctx: &ValidationContext,
    current: &EscrowState,
    proposed: &EscrowState,
) -> ConstraintResult {
    // Can only release from Funded state
    if current.status != EscrowStatus::Funded {
        return ConstraintResult::reject("escrow not funded");
    }

    // Must transition to Released
    if proposed.status != EscrowStatus::Released {
        return ConstraintResult::reject("proposed status must be Released");
    }

    // Only arbiter or depositor can release
    if ctx.sender != current.arbiter && ctx.sender != current.depositor {
        return ConstraintResult::reject("only arbiter or depositor can release");
    }

    // Amount must remain the same (solver handles actual transfer)
    if proposed.amount != current.amount {
        return ConstraintResult::reject("amount cannot change during release");
    }

    ConstraintResult::accept()
}

fn validate_refund(
    ctx: &ValidationContext,
    current: &EscrowState,
    proposed: &EscrowState,
) -> ConstraintResult {
    // Can only refund from Funded state
    if current.status != EscrowStatus::Funded {
        return ConstraintResult::reject("escrow not funded");
    }

    // Must transition to Refunded
    if proposed.status != EscrowStatus::Refunded {
        return ConstraintResult::reject("proposed status must be Refunded");
    }

    // Arbiter can refund anytime; depositor can only after deadline
    if ctx.sender == current.arbiter {
        // Arbiter can always refund
    } else if ctx.sender == current.depositor {
        if ctx.epoch < current.deadline_epoch {
            return ConstraintResult::reject("depositor can only refund after deadline");
        }
    } else {
        return ConstraintResult::reject("only arbiter or depositor can refund");
    }

    ConstraintResult::accept()
}

/// Generate the schema definition for this contract.
pub fn escrow_schema() -> ContractSchemaDef {
    SchemaBuilder::new("Escrow")
        .description("Two-party escrow with arbiter release/refund")
        .domain_tag("contract.escrow")
        .max_gas(500_000)
        .max_state(4_096)
        .method("fund", "Fund the escrow with tokens", vec![
            ("amount", "u128"),
            ("deadline_epoch", "u64"),
        ])
        .method("release", "Release funds to beneficiary", vec![])
        .method("refund", "Refund funds to depositor", vec![])
        .build()
}

// Wasm entry point — uncomment when compiling to wasm32:
// omniphi_contract_entry!(EscrowState);

fn main() {
    // Print the schema JSON for deployment
    let schema = escrow_schema();
    println!("{}", schema.to_json());
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn test_ctx(sender: [u8; 32], method: &str, epoch: u64) -> ValidationContext {
        ValidationContext {
            epoch,
            sender,
            gas_remaining: 1_000_000,
            method: method.to_string(),
            params: BTreeMap::new(),
        }
    }

    fn depositor() -> [u8; 32] { [1u8; 32] }
    fn beneficiary() -> [u8; 32] { [2u8; 32] }
    fn arbiter() -> [u8; 32] { [3u8; 32] }
    fn attacker() -> [u8; 32] { [9u8; 32] }

    fn created_escrow() -> EscrowState {
        EscrowState {
            depositor: depositor(),
            beneficiary: beneficiary(),
            arbiter: arbiter(),
            amount: 0,
            asset_id: [0xAA; 32],
            status: EscrowStatus::Created,
            deadline_epoch: 100,
            funded_at_epoch: None,
        }
    }

    fn funded_escrow() -> EscrowState {
        EscrowState {
            depositor: depositor(),
            beneficiary: beneficiary(),
            arbiter: arbiter(),
            amount: 1000,
            asset_id: [0xAA; 32],
            status: EscrowStatus::Funded,
            deadline_epoch: 100,
            funded_at_epoch: Some(10),
        }
    }

    #[test]
    fn test_fund_happy_path() {
        let current = created_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Funded;
        proposed.amount = 1000;
        proposed.deadline_epoch = 100;
        proposed.funded_at_epoch = Some(10);

        let ctx = test_ctx(depositor(), "fund", 10);
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(result.valid, "fund should succeed: {}", result.reason);
    }

    #[test]
    fn test_fund_wrong_sender() {
        let current = created_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Funded;
        proposed.amount = 1000;

        let ctx = test_ctx(attacker(), "fund", 10);
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("only depositor"));
    }

    #[test]
    fn test_release_by_arbiter() {
        let current = funded_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Released;

        let ctx = test_ctx(arbiter(), "release", 50);
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(result.valid, "arbiter release should succeed: {}", result.reason);
    }

    #[test]
    fn test_release_by_attacker() {
        let current = funded_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Released;

        let ctx = test_ctx(attacker(), "release", 50);
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("only arbiter or depositor"));
    }

    #[test]
    fn test_refund_by_depositor_before_deadline() {
        let current = funded_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Refunded;

        let ctx = test_ctx(depositor(), "refund", 50); // deadline is 100
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("after deadline"));
    }

    #[test]
    fn test_refund_by_depositor_after_deadline() {
        let current = funded_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Refunded;

        let ctx = test_ctx(depositor(), "refund", 200); // after deadline
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(result.valid, "depositor refund after deadline should work: {}", result.reason);
    }

    #[test]
    fn test_refund_by_arbiter_anytime() {
        let current = funded_escrow();
        let mut proposed = current.clone();
        proposed.status = EscrowStatus::Refunded;

        let ctx = test_ctx(arbiter(), "refund", 5); // before deadline
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(result.valid, "arbiter refund should work anytime: {}", result.reason);
    }

    #[test]
    fn test_double_release_rejected() {
        let mut current = funded_escrow();
        current.status = EscrowStatus::Released;
        let proposed = current.clone();

        let ctx = test_ctx(arbiter(), "release", 50);
        let result = EscrowState::validate(&ctx, &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("not funded"));
    }
}
