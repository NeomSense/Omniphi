use crate::errors::RuntimeError;
use crate::intents::types::{SwapIntent, TransferIntent, TreasuryRebalanceIntent, YieldAllocateIntent, RouteLiquidityIntent};
use std::collections::BTreeMap;

/// The variant of intent contained in this transaction.
#[derive(Debug, Clone)]
pub enum IntentType {
    Transfer(TransferIntent),
    Swap(SwapIntent),
    YieldAllocate(YieldAllocateIntent),
    TreasuryRebalance(TreasuryRebalanceIntent),
    RouteLiquidity(RouteLiquidityIntent),
}

/// A fully described, signed intent transaction.
#[derive(Debug, Clone)]
pub struct IntentTransaction {
    pub tx_id: [u8; 32],
    pub sender: [u8; 32],
    pub intent: IntentType,
    pub max_fee: u64,
    pub deadline_epoch: u64,
    pub nonce: u64,
    /// Placeholder — signature verification is out of scope for the runtime engine.
    pub signature: [u8; 64],
    pub metadata: BTreeMap<String, String>,
}

impl IntentTransaction {
    /// Structural validation only — no state access required.
    pub fn validate(&self) -> Result<(), RuntimeError> {
        // Sender must be non-zero
        if self.sender == [0u8; 32] {
            return Err(RuntimeError::InvalidIntent(
                "sender address must be non-zero".to_string(),
            ));
        }
        // tx_id must be non-zero
        if self.tx_id == [0u8; 32] {
            return Err(RuntimeError::InvalidIntent(
                "tx_id must be non-zero".to_string(),
            ));
        }
        // max_fee must be positive
        if self.max_fee == 0 {
            return Err(RuntimeError::InvalidIntent(
                "max_fee must be greater than zero".to_string(),
            ));
        }

        // Intent-specific structural checks
        match &self.intent {
            IntentType::Transfer(t) => {
                if t.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "transfer amount must be greater than zero".to_string(),
                    ));
                }
                if t.recipient == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent(
                        "recipient address must be non-zero".to_string(),
                    ));
                }
                if t.asset_id == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent(
                        "asset_id must be non-zero".to_string(),
                    ));
                }
            }
            IntentType::Swap(s) => {
                if s.input_amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "swap input_amount must be greater than zero".to_string(),
                    ));
                }
                if s.input_asset == s.output_asset {
                    return Err(RuntimeError::InvalidIntent(
                        "swap input_asset and output_asset must differ".to_string(),
                    ));
                }
                if s.max_slippage_bps > 10000 {
                    return Err(RuntimeError::InvalidIntent(
                        "max_slippage_bps must not exceed 10000 (100%)".to_string(),
                    ));
                }
            }
            IntentType::YieldAllocate(y) => {
                if y.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "yield allocate amount must be greater than zero".to_string(),
                    ));
                }
            }
            IntentType::TreasuryRebalance(r) => {
                if r.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance amount must be greater than zero".to_string(),
                    ));
                }
                if r.authorized_by.is_empty() {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance requires at least one authority".to_string(),
                    ));
                }
                if r.from_asset == r.to_asset {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance from_asset and to_asset must differ".to_string(),
                    ));
                }
            }
            IntentType::RouteLiquidity(rl) => {
                if rl.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity amount must be greater than zero".to_string(),
                    ));
                }
                if rl.min_received == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity min_received must be greater than zero".to_string(),
                    ));
                }
                if rl.source_pool == rl.target_pool {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity source and target pools must differ".to_string(),
                    ));
                }
                if rl.max_hops == 0 || rl.max_hops > 5 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity max_hops must be 1-5".to_string(),
                    ));
                }
                if rl.max_price_impact_bps > 500 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity max_price_impact_bps must not exceed 500".to_string(),
                    ));
                }
            }
        }

        Ok(())
    }
}
