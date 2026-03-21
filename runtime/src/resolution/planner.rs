use crate::capabilities::checker::{Capability, CapabilityChecker, CapabilitySet};
use crate::errors::RuntimeError;
use crate::gas::meter::GasCosts;
use crate::intents::base::{IntentTransaction, IntentType};
use crate::objects::base::{AccessMode, ObjectAccess, ObjectId};
use crate::state::store::ObjectStore;
use primitive_types::U256;

// ──────────────────────────────────────────────
// ExecutionPlan
// ──────────────────────────────────────────────

/// A single atomic operation to be applied against the store.
#[derive(Debug, Clone)]
pub enum ObjectOperation {
    DebitBalance { balance_id: ObjectId, amount: u128 },
    CreditBalance { balance_id: ObjectId, amount: u128 },
    SwapPoolAmounts { pool_id: ObjectId, delta_a: i128, delta_b: i128 },
    LockBalance { balance_id: ObjectId, amount: u128 },
    UnlockBalance { balance_id: ObjectId, amount: u128 },
    UpdateVersion { object_id: ObjectId },
    /// Apply a proposed state transition to a contract object.
    /// The constraint validator has already approved this transition.
    ContractStateTransition {
        contract_id: ObjectId,
        schema_id: [u8; 32],
        proposed_state: Vec<u8>,
    },
}

/// The resolved execution plan for a single intent transaction.
#[derive(Debug, Clone)]
pub struct ExecutionPlan {
    pub tx_id: [u8; 32],
    pub operations: Vec<ObjectOperation>,
    pub required_capabilities: Vec<Capability>,
    pub object_access: Vec<ObjectAccess>,
    /// Estimated gas units for this plan (computed at resolution time).
    pub gas_estimate: u64,
    /// Maximum gas the sender is willing to pay (from `intent.max_fee`).
    /// `u64::MAX` means unlimited (should not happen after intent validation).
    pub gas_limit: u64,
}

// ──────────────────────────────────────────────
// AMM helper (overflow-safe via U256)
// ──────────────────────────────────────────────

/// Computes the constant-product AMM output amount using 256-bit integers
/// to prevent overflow for large reserve values.
///
/// Formula: output = (input * (10000 - fee_bps) * reserve_out)
///                   / (reserve_in * 10000 + input * (10000 - fee_bps))
fn amm_output(
    input_amount: u128,
    reserve_in: u128,
    reserve_out: u128,
    fee_bps: u32,
) -> Result<u128, RuntimeError> {
    let input = U256::from(input_amount);
    let r_in = U256::from(reserve_in);
    let r_out = U256::from(reserve_out);
    let fee = U256::from(fee_bps);
    let scale = U256::from(10_000u32);

    let input_with_fee = input * (scale - fee);
    let numerator = input_with_fee * r_out;
    let denominator = r_in * scale + input_with_fee;

    if denominator.is_zero() {
        return Err(RuntimeError::ConstraintViolation(
            "zero denominator in AMM".into(),
        ));
    }

    let output = numerator / denominator;

    // output <= reserve_out < u128::MAX, so this conversion is safe in practice
    if output > U256::from(u128::MAX) {
        return Err(RuntimeError::ConstraintViolation(
            "AMM output overflow".into(),
        ));
    }
    Ok(output.as_u128())
}

// ──────────────────────────────────────────────
// Gas estimation
// ──────────────────────────────────────────────

/// Sums up the gas cost for a list of operations.
fn estimate_gas(ops: &[ObjectOperation], costs: &GasCosts) -> u64 {
    let mut total = costs.base_tx;
    for op in ops {
        let cost = match op {
            ObjectOperation::DebitBalance { .. } => costs.debit_balance,
            ObjectOperation::CreditBalance { .. } => costs.credit_balance,
            ObjectOperation::SwapPoolAmounts { .. } => costs.swap_pool,
            ObjectOperation::LockBalance { .. } => costs.lock_balance,
            ObjectOperation::UnlockBalance { .. } => costs.unlock_balance,
            ObjectOperation::UpdateVersion { .. } => costs.update_version,
            ObjectOperation::ContractStateTransition { proposed_state, .. } => {
                costs.contract_state_write
                    + costs.constraint_validation_base
                    + costs.constraint_validation_per_byte * proposed_state.len() as u64
            }
        };
        total = total.saturating_add(cost);
    }
    total
}

// ──────────────────────────────────────────────
// IntentResolver
// ──────────────────────────────────────────────

pub struct IntentResolver;

impl IntentResolver {
    /// Resolves an intent transaction into an ExecutionPlan.
    /// Performs structural checks, capability pre-checks, and state lookups.
    pub fn resolve(
        intent: &IntentTransaction,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        match &intent.intent {
            IntentType::Transfer(t) => {
                Self::resolve_transfer(intent.tx_id, intent.max_fee, &intent.sender, t, store, caps)
            }
            IntentType::Swap(s) => {
                Self::resolve_swap(intent.tx_id, intent.max_fee, &intent.sender, s, store, caps)
            }
            IntentType::YieldAllocate(y) => {
                Self::resolve_yield_allocate(intent.tx_id, intent.max_fee, &intent.sender, y, store, caps)
            }
            IntentType::TreasuryRebalance(r) => {
                Self::resolve_treasury_rebalance(intent.tx_id, intent.max_fee, &intent.sender, r, store, caps)
            }
            IntentType::RouteLiquidity(rl) => {
                Self::resolve_route_liquidity(intent.tx_id, intent.max_fee, &intent.sender, rl, store, caps)
            }
            IntentType::ContractCall(_) => {
                // Contract calls are resolved by the solver market, not the
                // internal resolver. This path is only reached as a fallback
                // when no solver candidates exist. Return an empty plan that
                // will be settled as a no-op.
                Err(RuntimeError::ResolutionFailure(
                    "contract calls require solver market resolution".to_string(),
                ))
            }
        }
    }

    // ──────────────────────────────────────────
    // Transfer
    // ──────────────────────────────────────────

    fn resolve_transfer(
        tx_id: [u8; 32],
        max_fee: u64,
        sender: &[u8; 32],
        t: &crate::intents::types::TransferIntent,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        // Capability check
        CapabilityChecker::check(caps, &[Capability::TransferAsset])?;

        // Find sender balance object
        let sender_balance = store
            .find_balance(sender, &t.asset_id)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;

        // Ensure sufficient available balance
        if sender_balance.available() < t.amount {
            return Err(RuntimeError::InsufficientBalance {
                required: t.amount,
                available: sender_balance.available(),
            });
        }

        let sender_balance_id = sender_balance.meta.id;

        // Find or note recipient balance object
        let recipient_balance = store.find_balance(&t.recipient, &t.asset_id);
        let recipient_balance_id = match recipient_balance {
            Some(rb) => rb.meta.id,
            None => {
                // Recipient has no balance object for this asset yet.
                // Resolution fails — the balance object must be pre-created.
                return Err(RuntimeError::ObjectNotFound(ObjectId::zero()));
            }
        };

        let operations = vec![
            ObjectOperation::DebitBalance {
                balance_id: sender_balance_id,
                amount: t.amount,
            },
            ObjectOperation::CreditBalance {
                balance_id: recipient_balance_id,
                amount: t.amount,
            },
        ];

        let object_access = vec![
            ObjectAccess { object_id: sender_balance_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recipient_balance_id, mode: AccessMode::ReadWrite },
        ];

        let costs = GasCosts::default_costs();
        let gas_estimate = estimate_gas(&operations, &costs);

        Ok(ExecutionPlan {
            tx_id,
            operations,
            required_capabilities: vec![Capability::TransferAsset],
            object_access,
            gas_estimate,
            gas_limit: max_fee.saturating_mul(1_000),
        })
    }

    // ──────────────────────────────────────────
    // Swap
    // ──────────────────────────────────────────

    fn resolve_swap(
        tx_id: [u8; 32],
        max_fee: u64,
        sender: &[u8; 32],
        s: &crate::intents::types::SwapIntent,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        CapabilityChecker::check(caps, &[Capability::SwapAsset])?;

        // Find the liquidity pool
        let pool = match &s.allowed_pool_ids {
            None => store.find_pool(&s.input_asset, &s.output_asset),
            Some(ids) => {
                // Find the first listed pool that matches the asset pair
                ids.iter().find_map(|pool_id| {
                    // Use the typed pool overlay directly (no serde_json decode needed)
                    let p = store.get_pool_by_id(pool_id)?;
                    if (&p.asset_a == &s.input_asset && &p.asset_b == &s.output_asset)
                        || (&p.asset_a == &s.output_asset && &p.asset_b == &s.input_asset)
                    {
                        store.find_pool(&s.input_asset, &s.output_asset)
                    } else {
                        None
                    }
                })
            }
        };

        let pool = pool.ok_or_else(|| {
            RuntimeError::ResolutionFailure("no eligible liquidity pool found for swap".to_string())
        })?;

        // Determine swap direction: is input == asset_a?
        let input_is_a = pool.asset_a == s.input_asset;
        let (reserve_in, reserve_out) = if input_is_a {
            (pool.reserve_a, pool.reserve_b)
        } else {
            (pool.reserve_b, pool.reserve_a)
        };

        // Compute output via overflow-safe U256 AMM
        let output_amount = amm_output(s.input_amount, reserve_in, reserve_out, pool.fee_bps)?;

        // Enforce minimum output constraint
        if output_amount < s.min_output_amount {
            return Err(RuntimeError::ConstraintViolation(format!(
                "swap output {} < min_output_amount {}",
                output_amount, s.min_output_amount
            )));
        }

        // Enforce slippage using U256 to avoid overflow in the slippage calculation.
        // ideal_output (no fee) = input * reserve_out / (reserve_in + input)
        // We use U256 throughout.
        {
            let input_u = U256::from(s.input_amount);
            let r_in_u = U256::from(reserve_in);
            let r_out_u = U256::from(reserve_out);
            let denominator = r_in_u + input_u;
            if !denominator.is_zero() {
                let ideal = input_u * r_out_u / denominator;
                let actual = U256::from(output_amount);
                if ideal > actual {
                    let slippage_bps =
                        (ideal - actual) * U256::from(10_000u32) / ideal;
                    if slippage_bps > U256::from(s.max_slippage_bps) {
                        return Err(RuntimeError::ConstraintViolation(format!(
                            "swap slippage {} bps exceeds max {} bps",
                            slippage_bps, s.max_slippage_bps
                        )));
                    }
                }
            }
        }

        let pool_id = pool.meta.id;

        // Compute deltas (pool perspective)
        let (delta_a, delta_b): (i128, i128) = if input_is_a {
            (s.input_amount as i128, -(output_amount as i128))
        } else {
            (-(output_amount as i128), s.input_amount as i128)
        };

        // Find sender input-asset balance
        let sender_in_balance = store
            .find_balance(sender, &s.input_asset)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        if sender_in_balance.available() < s.input_amount {
            return Err(RuntimeError::InsufficientBalance {
                required: s.input_amount,
                available: sender_in_balance.available(),
            });
        }
        let sender_in_id = sender_in_balance.meta.id;

        // Find sender output-asset balance
        let sender_out_balance = store
            .find_balance(sender, &s.output_asset)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        let sender_out_id = sender_out_balance.meta.id;

        let operations = vec![
            ObjectOperation::DebitBalance {
                balance_id: sender_in_id,
                amount: s.input_amount,
            },
            ObjectOperation::CreditBalance {
                balance_id: sender_out_id,
                amount: output_amount,
            },
            ObjectOperation::SwapPoolAmounts {
                pool_id,
                delta_a,
                delta_b,
            },
        ];

        let object_access = vec![
            ObjectAccess { object_id: sender_in_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: sender_out_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: pool_id, mode: AccessMode::ReadWrite },
        ];

        let costs = GasCosts::default_costs();
        let gas_estimate = estimate_gas(&operations, &costs);

        Ok(ExecutionPlan {
            tx_id,
            operations,
            required_capabilities: vec![Capability::SwapAsset],
            object_access,
            gas_estimate,
            gas_limit: max_fee.saturating_mul(1_000),
        })
    }

    // ──────────────────────────────────────────
    // YieldAllocate
    // ──────────────────────────────────────────

    fn resolve_yield_allocate(
        tx_id: [u8; 32],
        max_fee: u64,
        sender: &[u8; 32],
        y: &crate::intents::types::YieldAllocateIntent,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        CapabilityChecker::check(caps, &[Capability::WriteObject])?;

        // Find sender balance
        let sender_balance = store
            .find_balance(sender, &y.asset_id)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        if sender_balance.available() < y.amount {
            return Err(RuntimeError::InsufficientBalance {
                required: y.amount,
                available: sender_balance.available(),
            });
        }
        let balance_id = sender_balance.meta.id;

        // Verify vault exists
        let _vault = store.get(&y.target_vault_id).ok_or_else(|| {
            RuntimeError::ObjectNotFound(y.target_vault_id)
        })?;

        // Lock the balance (deducted when user withdraws)
        let operations = vec![
            ObjectOperation::LockBalance {
                balance_id,
                amount: y.amount,
            },
            ObjectOperation::UpdateVersion {
                object_id: y.target_vault_id,
            },
        ];

        let object_access = vec![
            ObjectAccess { object_id: balance_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: y.target_vault_id, mode: AccessMode::ReadWrite },
        ];

        let costs = GasCosts::default_costs();
        let gas_estimate = estimate_gas(&operations, &costs);

        Ok(ExecutionPlan {
            tx_id,
            operations,
            required_capabilities: vec![Capability::WriteObject],
            object_access,
            gas_estimate,
            gas_limit: max_fee.saturating_mul(1_000),
        })
    }

    // ──────────────────────────────────────────
    // TreasuryRebalance
    // ──────────────────────────────────────────

    fn resolve_treasury_rebalance(
        tx_id: [u8; 32],
        max_fee: u64,
        sender: &[u8; 32],
        r: &crate::intents::types::TreasuryRebalanceIntent,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        // Governance authority check
        CapabilityChecker::check(caps, &[Capability::ModifyGovernance])?;

        // Verify that sender is in the authorized_by list
        if !r.authorized_by.contains(sender) {
            return Err(RuntimeError::UnauthorizedCapability {
                required: Capability::ModifyGovernance,
                held: caps.clone(),
            });
        }

        // Find sender from-asset balance
        let from_balance = store
            .find_balance(sender, &r.from_asset)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        if from_balance.available() < r.amount {
            return Err(RuntimeError::InsufficientBalance {
                required: r.amount,
                available: from_balance.available(),
            });
        }
        let from_id = from_balance.meta.id;

        // Find sender to-asset balance
        let to_balance = store
            .find_balance(sender, &r.to_asset)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        let to_id = to_balance.meta.id;

        let operations = vec![
            ObjectOperation::DebitBalance {
                balance_id: from_id,
                amount: r.amount,
            },
            ObjectOperation::CreditBalance {
                balance_id: to_id,
                amount: r.amount,
            },
        ];

        let object_access = vec![
            ObjectAccess { object_id: from_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: to_id, mode: AccessMode::ReadWrite },
        ];

        let costs = GasCosts::default_costs();
        let gas_estimate = estimate_gas(&operations, &costs);

        Ok(ExecutionPlan {
            tx_id,
            operations,
            required_capabilities: vec![Capability::ModifyGovernance],
            object_access,
            gas_estimate,
            gas_limit: max_fee.saturating_mul(1_000),
        })
    }

    // ──────────────────────────────────────────
    // RouteLiquidity
    // ──────────────────────────────────────────

    fn resolve_route_liquidity(
        tx_id: [u8; 32],
        max_fee: u64,
        sender: &[u8; 32],
        rl: &crate::intents::types::RouteLiquidityIntent,
        store: &ObjectStore,
        caps: &CapabilitySet,
    ) -> Result<ExecutionPlan, RuntimeError> {
        CapabilityChecker::check(caps, &[Capability::ProvideLiquidity, Capability::WithdrawLiquidity])?;

        // Verify source pool exists
        let _source_pool = store.get(&rl.source_pool).ok_or_else(|| {
            RuntimeError::ObjectNotFound(rl.source_pool)
        })?;

        // Verify target pool exists
        let _target_pool = store.get(&rl.target_pool).ok_or_else(|| {
            RuntimeError::ObjectNotFound(rl.target_pool)
        })?;

        // Find sender balance for the asset being routed
        let sender_balance = store
            .find_balance(sender, &rl.asset_id)
            .ok_or_else(|| RuntimeError::ObjectNotFound(ObjectId::zero()))?;
        if sender_balance.available() < rl.amount {
            return Err(RuntimeError::InsufficientBalance {
                required: rl.amount,
                available: sender_balance.available(),
            });
        }
        let balance_id = sender_balance.meta.id;

        // For Phase 1, route liquidity is modeled as:
        // 1. Debit from sender balance
        // 2. Update source pool version (withdraw)
        // 3. Update target pool version (deposit)
        // 4. Credit to sender balance (with min_received check at verification)
        let operations = vec![
            ObjectOperation::DebitBalance {
                balance_id,
                amount: rl.amount,
            },
            ObjectOperation::UpdateVersion {
                object_id: rl.source_pool,
            },
            ObjectOperation::UpdateVersion {
                object_id: rl.target_pool,
            },
            ObjectOperation::CreditBalance {
                balance_id,
                amount: rl.min_received, // Optimistic: actual amount determined by solver
            },
        ];

        let object_access = vec![
            ObjectAccess { object_id: balance_id, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: rl.source_pool, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: rl.target_pool, mode: AccessMode::ReadWrite },
        ];

        let costs = GasCosts::default_costs();
        let gas_estimate = estimate_gas(&operations, &costs);

        Ok(ExecutionPlan {
            tx_id,
            operations,
            required_capabilities: vec![Capability::ProvideLiquidity, Capability::WithdrawLiquidity],
            object_access,
            gas_estimate,
            gas_limit: max_fee.saturating_mul(1_000),
        })
    }
}
