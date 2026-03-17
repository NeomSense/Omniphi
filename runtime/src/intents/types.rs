use crate::objects::base::ObjectId;

/// Transfer an asset from sender to recipient.
#[derive(Debug, Clone)]
pub struct TransferIntent {
    pub asset_id: [u8; 32],
    pub amount: u128,
    pub recipient: [u8; 32],
    pub memo: Option<String>,
}

/// Swap one asset for another through a liquidity pool.
#[derive(Debug, Clone)]
pub struct SwapIntent {
    pub input_asset: [u8; 32],
    pub output_asset: [u8; 32],
    pub input_amount: u128,
    pub min_output_amount: u128,
    pub max_slippage_bps: u32,
    /// `None` = use any eligible pool; `Some(ids)` = restrict to listed pools.
    pub allowed_pool_ids: Option<Vec<ObjectId>>,
}

/// Deposit assets into a yield-bearing vault.
#[derive(Debug, Clone)]
pub struct YieldAllocateIntent {
    pub asset_id: [u8; 32],
    pub amount: u128,
    pub target_vault_id: ObjectId,
    pub min_apy_bps: u32,
}

/// Move assets between treasury asset classes (requires multisig authorities).
#[derive(Debug, Clone)]
pub struct TreasuryRebalanceIntent {
    pub from_asset: [u8; 32],
    pub to_asset: [u8; 32],
    pub amount: u128,
    /// Addresses of multisig authorities that must have authorised this transaction.
    pub authorized_by: Vec<[u8; 32]>,
}

/// Route liquidity between pools through multi-hop paths.
#[derive(Debug, Clone)]
pub struct RouteLiquidityIntent {
    pub source_pool: ObjectId,
    pub target_pool: ObjectId,
    pub asset_id: [u8; 32],
    pub amount: u128,
    pub min_received: u128,
    pub max_hops: u8,
    pub max_price_impact_bps: u16,
}
