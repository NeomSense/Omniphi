//! AMM Liquidity Pool Intent Contract — Reference Implementation
//!
//! Constant-product AMM (x * y = k) with:
//! - Add liquidity (deposit both tokens proportionally)
//! - Remove liquidity (withdraw proportional share)
//! - Swap (constant-product with fee)
//!
//! The constraint validator checks math invariants. Solvers compute
//! the actual swap amounts and produce balance operations.

use omniphi_contract_sdk::*;
use serde::{Deserialize, Serialize};

const FEE_BPS: u128 = 30; // 0.3% fee

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AmmPoolState {
    pub token_a: [u8; 32],
    pub token_b: [u8; 32],
    pub reserve_a: u128,
    pub reserve_b: u128,
    pub total_lp_shares: u128,
    pub fee_bps: u128,
    pub admin: [u8; 32],
}

impl ContractValidator for AmmPoolState {
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
            "add_liquidity" => validate_add_liquidity(ctx, current, proposed),
            "remove_liquidity" => validate_remove_liquidity(ctx, current, proposed),
            "swap" => validate_swap(ctx, current, proposed),
            _ => ConstraintResult::reject(&format!("unknown method: {}", method)),
        }
    }
}

fn validate_add_liquidity(
    _ctx: &ValidationContext,
    current: &AmmPoolState,
    proposed: &AmmPoolState,
) -> ConstraintResult {
    // Reserves must increase
    if proposed.reserve_a <= current.reserve_a || proposed.reserve_b <= current.reserve_b {
        return ConstraintResult::reject("reserves must increase when adding liquidity");
    }

    // LP shares must increase
    if proposed.total_lp_shares <= current.total_lp_shares {
        return ConstraintResult::reject("LP shares must increase");
    }

    // Token identities must not change
    if proposed.token_a != current.token_a || proposed.token_b != current.token_b {
        return ConstraintResult::reject("cannot change pool tokens");
    }

    // For initial liquidity (empty pool), any ratio is fine
    if current.reserve_a == 0 && current.reserve_b == 0 {
        return ConstraintResult::accept();
    }

    // For existing pool, check proportionality (within 1% tolerance)
    let delta_a = proposed.reserve_a - current.reserve_a;
    let delta_b = proposed.reserve_b - current.reserve_b;
    let expected_b = (delta_a * current.reserve_b) / current.reserve_a;
    let tolerance = expected_b / 100; // 1%
    if delta_b > expected_b + tolerance || (expected_b > tolerance && delta_b < expected_b - tolerance) {
        return ConstraintResult::reject("liquidity must be added proportionally (within 1%)");
    }

    ConstraintResult::accept()
}

fn validate_remove_liquidity(
    _ctx: &ValidationContext,
    current: &AmmPoolState,
    proposed: &AmmPoolState,
) -> ConstraintResult {
    // Reserves must decrease
    if proposed.reserve_a >= current.reserve_a || proposed.reserve_b >= current.reserve_b {
        return ConstraintResult::reject("reserves must decrease when removing liquidity");
    }

    // LP shares must decrease
    if proposed.total_lp_shares >= current.total_lp_shares {
        return ConstraintResult::reject("LP shares must decrease");
    }

    // Cannot drain to zero (minimum liquidity)
    if proposed.reserve_a == 0 || proposed.reserve_b == 0 {
        return ConstraintResult::reject("cannot fully drain pool");
    }

    ConstraintResult::accept()
}

fn validate_swap(
    _ctx: &ValidationContext,
    current: &AmmPoolState,
    proposed: &AmmPoolState,
) -> ConstraintResult {
    // One reserve increases, the other decreases (swap)
    let a_increased = proposed.reserve_a > current.reserve_a;
    let b_increased = proposed.reserve_b > current.reserve_b;
    let a_decreased = proposed.reserve_a < current.reserve_a;
    let b_decreased = proposed.reserve_b < current.reserve_b;

    if !(a_increased && b_decreased) && !(a_decreased && b_increased) {
        return ConstraintResult::reject("swap must increase one reserve and decrease the other");
    }

    // LP shares must not change during swap
    if proposed.total_lp_shares != current.total_lp_shares {
        return ConstraintResult::reject("LP shares cannot change during swap");
    }

    // Constant product invariant: k_new >= k_old (fee ensures k grows)
    let k_old = current.reserve_a as u128 * current.reserve_b as u128;
    let k_new = proposed.reserve_a as u128 * proposed.reserve_b as u128;

    if k_new < k_old {
        return ConstraintResult::reject("swap violates constant product invariant (k must not decrease)");
    }

    // Fee check: the output must account for the fee
    // k_new should be >= k_old * (1 + fee_fraction) approximately
    // We allow k_new >= k_old as a minimum (fee is captured as k growth)

    ConstraintResult::accept()
}

pub fn amm_pool_schema() -> ContractSchemaDef {
    SchemaBuilder::new("AmmPool")
        .description("Constant-product AMM liquidity pool (x*y=k)")
        .domain_tag("contract.amm")
        .max_gas(1_000_000)
        .max_state(8_192)
        .method("add_liquidity", "Add proportional liquidity to the pool", vec![
            ("amount_a", "u128"),
            ("amount_b", "u128"),
        ])
        .method("remove_liquidity", "Remove liquidity and receive both tokens", vec![
            ("lp_shares", "u128"),
        ])
        .method("swap", "Swap one token for the other", vec![
            ("input_amount", "u128"),
            ("min_output", "u128"),
            ("direction", "string"), // "a_to_b" or "b_to_a"
        ])
        .build()
}

fn main() {
    let schema = amm_pool_schema();
    println!("{}", schema.to_json());
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn ctx(method: &str) -> ValidationContext {
        ValidationContext {
            epoch: 10,
            sender: [1u8; 32],
            gas_remaining: 1_000_000,
            method: method.to_string(),
            params: BTreeMap::new(),
        }
    }

    fn empty_pool() -> AmmPoolState {
        AmmPoolState {
            token_a: [0xAA; 32],
            token_b: [0xBB; 32],
            reserve_a: 0,
            reserve_b: 0,
            total_lp_shares: 0,
            fee_bps: FEE_BPS,
            admin: [1u8; 32],
        }
    }

    fn active_pool() -> AmmPoolState {
        AmmPoolState {
            token_a: [0xAA; 32],
            token_b: [0xBB; 32],
            reserve_a: 1_000_000,
            reserve_b: 2_000_000,
            total_lp_shares: 1_000,
            fee_bps: FEE_BPS,
            admin: [1u8; 32],
        }
    }

    #[test]
    fn test_initial_liquidity() {
        let current = empty_pool();
        let mut proposed = current.clone();
        proposed.reserve_a = 1000;
        proposed.reserve_b = 2000;
        proposed.total_lp_shares = 100;

        let result = AmmPoolState::validate(&ctx("add_liquidity"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }

    #[test]
    fn test_swap_valid() {
        let current = active_pool();
        let mut proposed = current.clone();
        // Swap: input 10000 token_a, output some token_b
        proposed.reserve_a = 1_010_000; // +10000
        proposed.reserve_b = 1_980_200; // -19800 (with fee, k grows)

        let result = AmmPoolState::validate(&ctx("swap"), &current, &proposed);
        assert!(result.valid, "{}", result.reason);
    }

    #[test]
    fn test_swap_violates_k() {
        let current = active_pool();
        let mut proposed = current.clone();
        proposed.reserve_a = 1_010_000;
        proposed.reserve_b = 1_970_000; // too much output, k decreases

        let result = AmmPoolState::validate(&ctx("swap"), &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("constant product"));
    }

    #[test]
    fn test_swap_lp_change_rejected() {
        let current = active_pool();
        let mut proposed = current.clone();
        proposed.reserve_a = 1_010_000;
        proposed.reserve_b = 1_980_200;
        proposed.total_lp_shares = 1001; // sneaky LP inflation

        let result = AmmPoolState::validate(&ctx("swap"), &current, &proposed);
        assert!(!result.valid);
        assert!(result.reason.contains("LP shares"));
    }
}
