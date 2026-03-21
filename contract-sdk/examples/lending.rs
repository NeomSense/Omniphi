//! Lending Protocol Intent Contract — Reference Implementation
//!
//! Collateralized lending with:
//! - Deposit collateral
//! - Borrow against collateral (with LTV ratio)
//! - Repay loan
//! - Liquidate undercollateralized positions
//!
//! This demonstrates cross-contract composition: a solver can atomically
//! swap collateral via the AMM pool contract and deposit into lending
//! in a single CausalGraph — something impossible on EVM without reentrancy risk.

use omniphi_contract_sdk::*;
use serde::{Deserialize, Serialize};

/// Maximum loan-to-value ratio in basis points (7500 = 75%)
const MAX_LTV_BPS: u128 = 7500;
/// Liquidation threshold in basis points (8500 = 85%)
const LIQUIDATION_THRESHOLD_BPS: u128 = 8500;
/// Liquidation penalty in basis points (500 = 5%)
const LIQUIDATION_PENALTY_BPS: u128 = 500;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PositionStatus {
    Active,
    Repaid,
    Liquidated,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LendingPosition {
    pub borrower: [u8; 32],
    pub collateral_asset: [u8; 32],
    pub borrow_asset: [u8; 32],
    pub collateral_amount: u128,
    pub borrowed_amount: u128,
    pub interest_accrued: u128,
    pub collateral_price_bps: u128, // price of collateral in terms of borrow asset (10000 = 1:1)
    pub status: PositionStatus,
    pub opened_at_epoch: u64,
    pub last_interest_epoch: u64,
}

impl LendingPosition {
    /// Current LTV in basis points.
    fn current_ltv_bps(&self) -> u128 {
        if self.collateral_amount == 0 || self.collateral_price_bps == 0 {
            return 10000; // 100% = fully utilized
        }
        let collateral_value = self.collateral_amount * self.collateral_price_bps / 10000;
        if collateral_value == 0 {
            return 10000;
        }
        (self.borrowed_amount + self.interest_accrued) * 10000 / collateral_value
    }
}

impl ContractValidator for LendingPosition {
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
            "deposit_collateral" => validate_deposit(ctx, current, proposed),
            "borrow" => validate_borrow(ctx, current, proposed),
            "repay" => validate_repay(ctx, current, proposed),
            "liquidate" => validate_liquidate(ctx, current, proposed),
            _ => ConstraintResult::reject(&format!("unknown method: {}", method)),
        }
    }
}

fn validate_deposit(
    ctx: &ValidationContext,
    current: &LendingPosition,
    proposed: &LendingPosition,
) -> ConstraintResult {
    if current.status != PositionStatus::Active && current.collateral_amount > 0 {
        return ConstraintResult::reject("position is not active");
    }

    if ctx.sender != current.borrower && current.collateral_amount > 0 {
        return ConstraintResult::reject("only borrower can deposit collateral");
    }

    if proposed.collateral_amount <= current.collateral_amount {
        return ConstraintResult::reject("collateral must increase");
    }

    if proposed.borrowed_amount != current.borrowed_amount {
        return ConstraintResult::reject("cannot change borrowed amount during deposit");
    }

    ConstraintResult::accept()
}

fn validate_borrow(
    ctx: &ValidationContext,
    current: &LendingPosition,
    proposed: &LendingPosition,
) -> ConstraintResult {
    if current.status != PositionStatus::Active {
        return ConstraintResult::reject("position is not active");
    }

    if ctx.sender != current.borrower {
        return ConstraintResult::reject("only borrower can borrow");
    }

    if proposed.borrowed_amount <= current.borrowed_amount {
        return ConstraintResult::reject("borrow amount must increase");
    }

    if proposed.collateral_amount != current.collateral_amount {
        return ConstraintResult::reject("cannot change collateral during borrow");
    }

    // Check LTV after borrow
    let ltv = proposed.current_ltv_bps();
    if ltv > MAX_LTV_BPS {
        return ConstraintResult::reject(&format!(
            "LTV {}bps exceeds max {}bps", ltv, MAX_LTV_BPS
        ));
    }

    ConstraintResult::accept()
}

fn validate_repay(
    ctx: &ValidationContext,
    current: &LendingPosition,
    proposed: &LendingPosition,
) -> ConstraintResult {
    if current.status != PositionStatus::Active {
        return ConstraintResult::reject("position is not active");
    }

    // Anyone can repay (allows third-party repayment)
    let _ = ctx;

    if proposed.borrowed_amount > current.borrowed_amount {
        return ConstraintResult::reject("borrowed amount cannot increase during repay");
    }

    // If fully repaid, status should be Repaid
    if proposed.borrowed_amount == 0 && proposed.interest_accrued == 0 {
        if proposed.status != PositionStatus::Repaid {
            return ConstraintResult::reject("fully repaid position must have Repaid status");
        }
    }

    ConstraintResult::accept()
}

fn validate_liquidate(
    ctx: &ValidationContext,
    current: &LendingPosition,
    proposed: &LendingPosition,
) -> ConstraintResult {
    if current.status != PositionStatus::Active {
        return ConstraintResult::reject("position is not active");
    }

    // Anyone can liquidate (incentivized by penalty)
    let _ = ctx;

    // Position must be undercollateralized
    let ltv = current.current_ltv_bps();
    if ltv < LIQUIDATION_THRESHOLD_BPS {
        return ConstraintResult::reject(&format!(
            "position LTV {}bps below liquidation threshold {}bps",
            ltv, LIQUIDATION_THRESHOLD_BPS
        ));
    }

    // Must transition to Liquidated
    if proposed.status != PositionStatus::Liquidated {
        return ConstraintResult::reject("liquidated position must have Liquidated status");
    }

    // Collateral should decrease (seized by liquidator)
    if proposed.collateral_amount >= current.collateral_amount {
        return ConstraintResult::reject("collateral must decrease during liquidation");
    }

    ConstraintResult::accept()
}

pub fn lending_schema() -> ContractSchemaDef {
    SchemaBuilder::new("Lending")
        .description("Collateralized lending protocol with liquidation")
        .domain_tag("contract.lending")
        .max_gas(2_000_000)
        .max_state(16_384)
        .method("deposit_collateral", "Deposit collateral to open/add to position", vec![
            ("amount", "u128"),
        ])
        .method("borrow", "Borrow against deposited collateral", vec![
            ("amount", "u128"),
        ])
        .method("repay", "Repay borrowed amount (partial or full)", vec![
            ("amount", "u128"),
        ])
        .method("liquidate", "Liquidate an undercollateralized position", vec![
            ("position_id", "bytes32"),
        ])
        .build()
}

fn main() {
    let schema = lending_schema();
    println!("{}", schema.to_json());
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn ctx(sender: [u8; 32], method: &str) -> ValidationContext {
        ValidationContext {
            epoch: 10,
            sender,
            gas_remaining: 1_000_000,
            method: method.to_string(),
            params: BTreeMap::new(),
        }
    }

    fn borrower() -> [u8; 32] { [1u8; 32] }
    fn liquidator() -> [u8; 32] { [5u8; 32] }

    fn empty_position() -> LendingPosition {
        LendingPosition {
            borrower: borrower(),
            collateral_asset: [0xCC; 32],
            borrow_asset: [0xDD; 32],
            collateral_amount: 0,
            borrowed_amount: 0,
            interest_accrued: 0,
            collateral_price_bps: 10000, // 1:1
            status: PositionStatus::Active,
            opened_at_epoch: 10,
            last_interest_epoch: 10,
        }
    }

    fn active_position() -> LendingPosition {
        LendingPosition {
            borrower: borrower(),
            collateral_asset: [0xCC; 32],
            borrow_asset: [0xDD; 32],
            collateral_amount: 10000,
            borrowed_amount: 5000,
            interest_accrued: 0,
            collateral_price_bps: 10000,
            status: PositionStatus::Active,
            opened_at_epoch: 5,
            last_interest_epoch: 10,
        }
    }

    #[test]
    fn test_deposit_collateral() {
        let current = empty_position();
        let mut proposed = current.clone();
        proposed.collateral_amount = 10000;

        let result = LendingPosition::validate(&ctx(borrower(), "deposit_collateral"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }

    #[test]
    fn test_borrow_within_ltv() {
        let current = active_position();
        let mut proposed = current.clone();
        proposed.borrowed_amount = 7000; // 70% LTV, under 75% max

        let result = LendingPosition::validate(&ctx(borrower(), "borrow"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }

    #[test]
    fn test_borrow_exceeds_ltv() {
        let current = active_position();
        let mut proposed = current.clone();
        proposed.borrowed_amount = 8000; // 80% LTV, over 75% max

        let result = LendingPosition::validate(&ctx(borrower(), "borrow"), &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("LTV"));
    }

    #[test]
    fn test_liquidation_undercollateralized() {
        let mut current = active_position();
        current.borrowed_amount = 9000; // 90% LTV, over 85% threshold
        let mut proposed = current.clone();
        proposed.status = PositionStatus::Liquidated;
        proposed.collateral_amount = 5000; // seized partial collateral

        let result = LendingPosition::validate(&ctx(liquidator(), "liquidate"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }

    #[test]
    fn test_liquidation_healthy_position_rejected() {
        let current = active_position(); // 50% LTV, healthy
        let mut proposed = current.clone();
        proposed.status = PositionStatus::Liquidated;
        proposed.collateral_amount = 5000;

        let result = LendingPosition::validate(&ctx(liquidator(), "liquidate"), &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("below liquidation threshold"));
    }

    #[test]
    fn test_full_repay() {
        let current = active_position();
        let mut proposed = current.clone();
        proposed.borrowed_amount = 0;
        proposed.interest_accrued = 0;
        proposed.status = PositionStatus::Repaid;

        let result = LendingPosition::validate(&ctx(borrower(), "repay"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }
}
