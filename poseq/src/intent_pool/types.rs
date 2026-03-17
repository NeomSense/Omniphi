//! Core intent types for the Omniphi intent-based execution architecture.
//!
//! These types define the intent schema, Phase 1 intent variants, and supporting
//! structures per Section 2 of the architecture specification.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ─── Asset Identifier ───────────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub enum AssetType {
    Native,
    Token,
    Object,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct AssetId {
    pub chain_id: u32,
    pub asset_type: AssetType,
    pub identifier: [u8; 32],
}

impl AssetId {
    pub fn native(identifier: [u8; 32]) -> Self {
        AssetId { chain_id: 0, asset_type: AssetType::Native, identifier }
    }

    pub fn token(identifier: [u8; 32]) -> Self {
        AssetId { chain_id: 0, asset_type: AssetType::Token, identifier }
    }
}

// ─── Solver Permissions ─────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum PermissionMode {
    AllowAll,
    Whitelist,
    Blacklist,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SolverPermissions {
    pub mode: PermissionMode,
    pub solver_ids: Vec<[u8; 32]>,
}

impl Default for SolverPermissions {
    fn default() -> Self {
        SolverPermissions { mode: PermissionMode::AllowAll, solver_ids: Vec::new() }
    }
}

impl SolverPermissions {
    /// Check if a solver is permitted by this permission set.
    pub fn is_permitted(&self, solver_id: &[u8; 32]) -> bool {
        match self.mode {
            PermissionMode::AllowAll => true,
            PermissionMode::Whitelist => self.solver_ids.contains(solver_id),
            PermissionMode::Blacklist => !self.solver_ids.contains(solver_id),
        }
    }
}

// ─── Execution Preferences ──────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Priority {
    Normal,
    High,
    Urgent,
}

impl Priority {
    pub fn weight(&self) -> u32 {
        match self {
            Priority::Normal => 100,
            Priority::High   => 500,
            Priority::Urgent => 1000,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ExecutionPrefs {
    pub priority: Priority,
    pub max_solver_count: Option<u8>,
    pub require_full_fill: bool,
}

impl Default for ExecutionPrefs {
    fn default() -> Self {
        ExecutionPrefs {
            priority: Priority::Normal,
            max_solver_count: None,
            require_full_fill: false,
        }
    }
}

// ─── Phase 1 Intent Variants ────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SwapIntent {
    pub asset_in: AssetId,
    pub asset_out: AssetId,
    pub amount_in: u128,
    pub min_amount_out: u128,
    pub max_slippage_bps: u16,
    pub route_hint: Option<Vec<AssetId>>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PaymentIntent {
    pub asset: AssetId,
    pub amount: u128,
    pub recipient: [u8; 32],
    pub memo: Option<Vec<u8>>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RebalanceParams {
    pub max_hops: u8,
    pub max_price_impact_bps: u16,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RouteLiquidityIntent {
    pub source_pool: [u8; 32],
    pub target_pool: [u8; 32],
    pub asset: AssetId,
    pub amount: u128,
    pub min_received: u128,
    pub rebalance_params: RebalanceParams,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum IntentKind {
    Swap(SwapIntent),
    Payment(PaymentIntent),
    RouteLiquidity(RouteLiquidityIntent),
}

impl IntentKind {
    /// Returns a single-byte type tag for intent_id derivation.
    pub fn type_tag(&self) -> u8 {
        match self {
            IntentKind::Swap(_)           => 0x01,
            IntentKind::Payment(_)        => 0x02,
            IntentKind::RouteLiquidity(_) => 0x03,
        }
    }

    /// Returns a human-readable kind string.
    pub fn kind_str(&self) -> &'static str {
        match self {
            IntentKind::Swap(_)           => "swap",
            IntentKind::Payment(_)        => "payment",
            IntentKind::RouteLiquidity(_) => "route_liquidity",
        }
    }
}

// ─── Intent Transaction ─────────────────────────────────────────────────────

/// Full intent transaction as submitted by a user.
///
/// The `intent_id` is deterministically derived from the canonical fields.
/// The `signature` covers all fields except `intent_id` itself.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct IntentTransaction {
    // Identity
    pub intent_id: [u8; 32],
    pub intent_type: IntentKind,
    pub version: u16,

    // User
    pub user: [u8; 32],
    pub nonce: u64,

    // Routing
    pub recipient: Option<[u8; 32]>,

    // Time
    pub deadline: u64,
    pub valid_from: Option<u64>,

    // Fees
    pub max_fee: u64,
    pub tip: Option<u64>,

    // Execution preferences
    pub partial_fill_allowed: bool,
    pub solver_permissions: SolverPermissions,
    pub execution_preferences: ExecutionPrefs,

    // Integrity
    pub witness_hash: Option<[u8; 32]>,
    pub signature: Vec<u8>,

    // Metadata
    pub metadata: BTreeMap<String, String>,
}

impl IntentTransaction {
    /// Compute the deterministic intent_id from canonical fields.
    ///
    /// intent_id = SHA256(user ‖ nonce.to_be_bytes() ‖ type_tag ‖ constraints_bytes ‖ deadline.to_be_bytes())
    pub fn compute_intent_id(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(self.user);
        hasher.update(self.nonce.to_be_bytes());
        hasher.update([self.intent_type.type_tag()]);
        // Canonical constraint bytes: deterministic bincode of the intent variant
        let constraints = bincode::serialize(&self.intent_type).unwrap_or_default();
        hasher.update(&constraints);
        hasher.update(self.deadline.to_be_bytes());
        let result = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&result);
        id
    }

    /// Verify that the stored intent_id matches the computed value.
    pub fn verify_intent_id(&self) -> bool {
        self.intent_id == self.compute_intent_id()
    }

    /// Validate structural correctness of the intent.
    pub fn validate_basic(&self) -> Result<(), IntentValidationError> {
        // Version check
        if self.version != 1 {
            return Err(IntentValidationError::UnsupportedVersion(self.version));
        }

        // Zero checks
        if self.user == [0u8; 32] {
            return Err(IntentValidationError::ZeroUser);
        }
        if self.intent_id == [0u8; 32] {
            return Err(IntentValidationError::ZeroIntentId);
        }

        // Intent ID must be correct
        if !self.verify_intent_id() {
            return Err(IntentValidationError::IntentIdMismatch);
        }

        // Fee floor
        if self.max_fee < super::constants::MIN_INTENT_FEE_BPS {
            return Err(IntentValidationError::FeeBelowMinimum {
                fee: self.max_fee,
                min: super::constants::MIN_INTENT_FEE_BPS,
            });
        }

        // Signature present
        if self.signature.is_empty() {
            return Err(IntentValidationError::MissingSignature);
        }

        // Validate intent-specific constraints
        match &self.intent_type {
            IntentKind::Swap(swap) => {
                if swap.amount_in == 0 {
                    return Err(IntentValidationError::ZeroAmount("amount_in"));
                }
                if swap.min_amount_out == 0 {
                    return Err(IntentValidationError::ZeroAmount("min_amount_out"));
                }
                if swap.asset_in == swap.asset_out {
                    return Err(IntentValidationError::SameAsset);
                }
                if swap.max_slippage_bps > 10_000 {
                    return Err(IntentValidationError::InvalidSlippage(swap.max_slippage_bps));
                }
            }
            IntentKind::Payment(payment) => {
                if payment.amount == 0 {
                    return Err(IntentValidationError::ZeroAmount("amount"));
                }
                if payment.recipient == [0u8; 32] {
                    return Err(IntentValidationError::ZeroRecipient);
                }
                if let Some(ref memo) = payment.memo {
                    if memo.len() > 256 {
                        return Err(IntentValidationError::MemoTooLarge(memo.len()));
                    }
                }
            }
            IntentKind::RouteLiquidity(route) => {
                if route.amount == 0 {
                    return Err(IntentValidationError::ZeroAmount("amount"));
                }
                if route.min_received == 0 {
                    return Err(IntentValidationError::ZeroAmount("min_received"));
                }
                if route.source_pool == route.target_pool {
                    return Err(IntentValidationError::SamePool);
                }
                if route.rebalance_params.max_hops == 0 || route.rebalance_params.max_hops > 5 {
                    return Err(IntentValidationError::InvalidMaxHops(route.rebalance_params.max_hops));
                }
                if route.rebalance_params.max_price_impact_bps > 500 {
                    return Err(IntentValidationError::InvalidPriceImpact(
                        route.rebalance_params.max_price_impact_bps,
                    ));
                }
            }
        }

        Ok(())
    }

    /// Serialized size estimate (for pool admission size checks).
    pub fn estimated_size(&self) -> usize {
        bincode::serialize(self).map(|v| v.len()).unwrap_or(0)
    }
}

// ─── Intent Announcement (gossip) ───────────────────────────────────────────

/// Lightweight announcement for gossip. Peers request the full intent if interested.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct IntentAnnouncement {
    pub intent_id: [u8; 32],
    pub intent_type_tag: u8,
    pub user: [u8; 32],
    pub deadline: u64,
    pub tip: u64,
    pub size_bytes: u32,
}

impl IntentAnnouncement {
    pub fn from_intent(intent: &IntentTransaction) -> Self {
        IntentAnnouncement {
            intent_id: intent.intent_id,
            intent_type_tag: intent.intent_type.type_tag(),
            user: intent.user,
            deadline: intent.deadline,
            tip: intent.tip.unwrap_or(0),
            size_bytes: intent.estimated_size() as u32,
        }
    }
}

// ─── Validation Errors ──────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum IntentValidationError {
    UnsupportedVersion(u16),
    ZeroUser,
    ZeroIntentId,
    IntentIdMismatch,
    FeeBelowMinimum { fee: u64, min: u64 },
    MissingSignature,
    ZeroAmount(&'static str),
    ZeroRecipient,
    SameAsset,
    SamePool,
    InvalidSlippage(u16),
    InvalidMaxHops(u8),
    InvalidPriceImpact(u16),
    MemoTooLarge(usize),
    DeadlineExpired { deadline: u64, current_block: u64 },
    IntentTooLarge { size: usize, max: usize },
    NonceAlreadyUsed { user: [u8; 32], nonce: u64 },
    NonceGapTooLarge { expected: u64, got: u64 },
    RateLimitExceeded { user: [u8; 32], count: u32, max: u32 },
    PoolFull,
    DuplicateIntent,
}

impl std::fmt::Display for IntentValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::UnsupportedVersion(v) => write!(f, "unsupported intent version: {}", v),
            Self::ZeroUser => write!(f, "zero user public key"),
            Self::ZeroIntentId => write!(f, "zero intent_id"),
            Self::IntentIdMismatch => write!(f, "intent_id does not match computed value"),
            Self::FeeBelowMinimum { fee, min } => write!(f, "fee {} below minimum {}", fee, min),
            Self::MissingSignature => write!(f, "missing signature"),
            Self::ZeroAmount(field) => write!(f, "zero amount in field: {}", field),
            Self::ZeroRecipient => write!(f, "zero recipient"),
            Self::SameAsset => write!(f, "asset_in == asset_out"),
            Self::SamePool => write!(f, "source_pool == target_pool"),
            Self::InvalidSlippage(v) => write!(f, "invalid slippage: {} bps", v),
            Self::InvalidMaxHops(v) => write!(f, "invalid max_hops: {}", v),
            Self::InvalidPriceImpact(v) => write!(f, "invalid price impact: {} bps", v),
            Self::MemoTooLarge(v) => write!(f, "memo too large: {} bytes", v),
            Self::DeadlineExpired { deadline, current_block } => {
                write!(f, "deadline {} expired at block {}", deadline, current_block)
            }
            Self::IntentTooLarge { size, max } => write!(f, "intent size {} exceeds max {}", size, max),
            Self::NonceAlreadyUsed { user, nonce } => {
                write!(f, "nonce {} already used by {}", nonce, hex::encode(&user[..4]))
            }
            Self::NonceGapTooLarge { expected, got } => {
                write!(f, "nonce gap too large: expected ~{}, got {}", expected, got)
            }
            Self::RateLimitExceeded { user, count, max } => {
                write!(f, "rate limit for {}: {} >= {}", hex::encode(&user[..4]), count, max)
            }
            Self::PoolFull => write!(f, "intent pool full"),
            Self::DuplicateIntent => write!(f, "duplicate intent"),
        }
    }
}

impl std::error::Error for IntentValidationError {}

// ─── Tests ──────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32];
        id[0] = b;
        AssetId::token(id)
    }

    fn make_swap_intent(user_byte: u8, nonce: u64) -> IntentTransaction {
        let user = {
            let mut u = [0u8; 32];
            u[0] = user_byte;
            u
        };
        let swap = SwapIntent {
            asset_in: make_asset(1),
            asset_out: make_asset(2),
            amount_in: 1000,
            min_amount_out: 950,
            max_slippage_bps: 50,
            route_hint: None,
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32], // will be computed
            intent_type: IntentKind::Swap(swap),
            version: 1,
            user,
            nonce,
            recipient: None,
            deadline: 1000,
            valid_from: None,
            max_fee: 100,
            tip: Some(10),
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        intent
    }

    #[test]
    fn test_intent_id_deterministic() {
        let a = make_swap_intent(1, 1);
        let b = make_swap_intent(1, 1);
        assert_eq!(a.intent_id, b.intent_id);
    }

    #[test]
    fn test_intent_id_differs_with_nonce() {
        let a = make_swap_intent(1, 1);
        let b = make_swap_intent(1, 2);
        assert_ne!(a.intent_id, b.intent_id);
    }

    #[test]
    fn test_intent_id_differs_with_user() {
        let a = make_swap_intent(1, 1);
        let b = make_swap_intent(2, 1);
        assert_ne!(a.intent_id, b.intent_id);
    }

    #[test]
    fn test_validate_basic_happy_path() {
        let intent = make_swap_intent(1, 1);
        assert!(intent.validate_basic().is_ok());
    }

    #[test]
    fn test_validate_rejects_zero_user() {
        let mut intent = make_swap_intent(0, 1);
        intent.user = [0u8; 32];
        intent.intent_id = intent.compute_intent_id();
        assert_eq!(intent.validate_basic().unwrap_err(), IntentValidationError::ZeroUser);
    }

    #[test]
    fn test_validate_rejects_wrong_intent_id() {
        let mut intent = make_swap_intent(1, 1);
        intent.intent_id = [0xAA; 32];
        assert_eq!(intent.validate_basic().unwrap_err(), IntentValidationError::IntentIdMismatch);
    }

    #[test]
    fn test_validate_rejects_zero_amount() {
        let user = { let mut u = [0u8; 32]; u[0] = 1; u };
        let swap = SwapIntent {
            asset_in: make_asset(1),
            asset_out: make_asset(2),
            amount_in: 0,
            min_amount_out: 950,
            max_slippage_bps: 50,
            route_hint: None,
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32],
            intent_type: IntentKind::Swap(swap),
            version: 1,
            user,
            nonce: 1,
            recipient: None,
            deadline: 1000,
            valid_from: None,
            max_fee: 100,
            tip: None,
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        assert_eq!(intent.validate_basic().unwrap_err(), IntentValidationError::ZeroAmount("amount_in"));
    }

    #[test]
    fn test_validate_rejects_same_asset() {
        let user = { let mut u = [0u8; 32]; u[0] = 1; u };
        let asset = make_asset(1);
        let swap = SwapIntent {
            asset_in: asset,
            asset_out: asset,
            amount_in: 1000,
            min_amount_out: 950,
            max_slippage_bps: 50,
            route_hint: None,
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32],
            intent_type: IntentKind::Swap(swap),
            version: 1,
            user,
            nonce: 1,
            recipient: None,
            deadline: 1000,
            valid_from: None,
            max_fee: 100,
            tip: None,
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        assert_eq!(intent.validate_basic().unwrap_err(), IntentValidationError::SameAsset);
    }

    #[test]
    fn test_validate_payment_happy_path() {
        let user = { let mut u = [0u8; 32]; u[0] = 1; u };
        let recipient = { let mut r = [0u8; 32]; r[0] = 2; r };
        let payment = PaymentIntent {
            asset: make_asset(1),
            amount: 500,
            recipient,
            memo: Some(b"hello".to_vec()),
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32],
            intent_type: IntentKind::Payment(payment),
            version: 1,
            user,
            nonce: 1,
            recipient: Some(recipient),
            deadline: 1000,
            valid_from: None,
            max_fee: 50,
            tip: None,
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        assert!(intent.validate_basic().is_ok());
    }

    #[test]
    fn test_validate_route_liquidity_happy_path() {
        let user = { let mut u = [0u8; 32]; u[0] = 1; u };
        let mut src = [0u8; 32]; src[0] = 10;
        let mut tgt = [0u8; 32]; tgt[0] = 20;
        let route = RouteLiquidityIntent {
            source_pool: src,
            target_pool: tgt,
            asset: make_asset(1),
            amount: 10_000,
            min_received: 9_500,
            rebalance_params: RebalanceParams { max_hops: 3, max_price_impact_bps: 200 },
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32],
            intent_type: IntentKind::RouteLiquidity(route),
            version: 1,
            user,
            nonce: 1,
            recipient: None,
            deadline: 1000,
            valid_from: None,
            max_fee: 100,
            tip: None,
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        assert!(intent.validate_basic().is_ok());
    }

    #[test]
    fn test_solver_permissions() {
        let solver_a = { let mut s = [0u8; 32]; s[0] = 1; s };
        let solver_b = { let mut s = [0u8; 32]; s[0] = 2; s };

        // AllowAll
        let perms = SolverPermissions::default();
        assert!(perms.is_permitted(&solver_a));
        assert!(perms.is_permitted(&solver_b));

        // Whitelist
        let perms = SolverPermissions {
            mode: PermissionMode::Whitelist,
            solver_ids: vec![solver_a],
        };
        assert!(perms.is_permitted(&solver_a));
        assert!(!perms.is_permitted(&solver_b));

        // Blacklist
        let perms = SolverPermissions {
            mode: PermissionMode::Blacklist,
            solver_ids: vec![solver_a],
        };
        assert!(!perms.is_permitted(&solver_a));
        assert!(perms.is_permitted(&solver_b));
    }

    #[test]
    fn test_intent_announcement() {
        let intent = make_swap_intent(1, 1);
        let ann = IntentAnnouncement::from_intent(&intent);
        assert_eq!(ann.intent_id, intent.intent_id);
        assert_eq!(ann.intent_type_tag, 0x01);
        assert_eq!(ann.tip, 10);
    }
}
