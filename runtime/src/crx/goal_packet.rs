use crate::errors::RuntimeError;
use crate::intents::base::{IntentTransaction, IntentType};
use crate::objects::base::ObjectType;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub enum RiskTier {
    Low,
    Standard,
    Elevated,
    Critical,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum PartialFailurePolicy {
    /// Revert entire execution on any branch failure.
    StrictAllOrNothing,
    /// Allow non-critical branches to fail; settle successful branches.
    AllowBranchDowngrade,
    /// On branch failure, quarantine affected objects and continue.
    QuarantineOnFailure,
    /// Downgrade execution scope and retry with reduced rights.
    DowngradeAndContinue,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GoalConstraints {
    pub min_output_amount: u128,
    pub max_slippage_bps: u32,
    pub allowed_object_ids: Option<Vec<[u8; 32]>>, // None = any
    pub allowed_object_types: Vec<ObjectType>,
    pub allowed_domains: Vec<String>,
    pub forbidden_domains: Vec<String>,
    pub max_objects_touched: usize,
}

impl Default for GoalConstraints {
    fn default() -> Self {
        GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 300,
            allowed_object_ids: None,
            allowed_object_types: vec![ObjectType::Balance, ObjectType::Wallet],
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 16,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum SettlementStrictness {
    /// Only settle fully verified causal paths.
    Strict,
    /// Allow finalization with logged warnings.
    Lenient,
    /// Provisional settlement, pending final audit.
    Provisional,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GoalPolicyEnvelope {
    pub risk_tier: RiskTier,
    pub partial_failure_policy: PartialFailurePolicy,
    pub require_rights_capsule: bool,
    pub require_causal_audit: bool,
    pub settlement_strictness: SettlementStrictness,
}

impl Default for GoalPolicyEnvelope {
    fn default() -> Self {
        GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: PartialFailurePolicy::StrictAllOrNothing,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: SettlementStrictness::Strict,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GoalPacket {
    pub packet_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub sender: [u8; 32],
    pub desired_outcome: String, // human-readable description
    pub constraints: GoalConstraints,
    pub policy: GoalPolicyEnvelope,
    pub max_fee: u64,
    pub deadline_epoch: u64,
    pub nonce: u64,
    pub metadata: BTreeMap<String, String>,
}

impl GoalPacket {
    pub fn validate(&self) -> Result<(), RuntimeError> {
        if self.packet_id == [0u8; 32] {
            return Err(RuntimeError::GoalPacketInvalid("zero packet_id".into()));
        }
        if self.max_fee == 0 {
            return Err(RuntimeError::GoalPacketInvalid(
                "max_fee must be > 0".into(),
            ));
        }
        if self.deadline_epoch == 0 {
            return Err(RuntimeError::GoalPacketInvalid(
                "deadline_epoch must be > 0".into(),
            ));
        }
        if self.constraints.allowed_object_types.is_empty()
            && self.constraints.allowed_domains.is_empty()
        {
            return Err(RuntimeError::GoalPacketInvalid(
                "must specify at least one allowed_object_type or allowed_domain".into(),
            ));
        }
        Ok(())
    }

    /// Compute a deterministic SHA256 hash of this packet.
    pub fn compute_hash(&self) -> [u8; 32] {
        let bytes =
            bincode::serialize(self).expect("GoalPacket bincode serialization is infallible");
        let mut hasher = Sha256::new();
        hasher.update(&bytes);
        hasher.finalize().into()
    }
}

/// Helper to build a GoalPacket from an existing IntentTransaction.
pub fn goal_packet_from_intent(intent: &IntentTransaction, epoch: u64) -> GoalPacket {
    let mut metadata = intent.metadata.clone();

    // Determine allowed_object_types and desired_outcome based on intent type
    let (allowed_object_types, desired_outcome) = match &intent.intent {
        IntentType::Transfer(_) => (
            vec![ObjectType::Balance, ObjectType::Wallet],
            "Transfer assets from sender to recipient".to_string(),
        ),
        IntentType::Swap(_) => (
            vec![ObjectType::Balance, ObjectType::LiquidityPool],
            "Swap input asset for output asset via liquidity pool".to_string(),
        ),
        IntentType::YieldAllocate(_) => (
            vec![ObjectType::Balance, ObjectType::Vault],
            "Allocate assets to yield-bearing vault".to_string(),
        ),
        IntentType::TreasuryRebalance(_) => (
            vec![ObjectType::Balance],
            "Rebalance treasury assets between asset classes".to_string(),
        ),
        IntentType::RouteLiquidity(_) => (
            vec![ObjectType::Balance, ObjectType::LiquidityPool],
            "Route liquidity between pools via multi-hop path".to_string(),
        ),
        IntentType::ContractCall(c) => (
            vec![ObjectType::Contract(c.schema_id)],
            format!("Contract call: {}.{}", hex::encode(&c.schema_id[..4]), c.method_selector),
        ),
    };

    let (min_output, max_slippage) = match &intent.intent {
        IntentType::Swap(s) => (s.min_output_amount, s.max_slippage_bps),
        _ => (0u128, 300u32),
    };

    metadata.insert("intent_type".to_string(), intent_type_name(&intent.intent).to_string());

    // Derive packet_id from intent tx_id (same bytes — deterministic mapping)
    let mut packet_id = intent.tx_id;
    packet_id[0] ^= 0x01; // distinguish from tx_id

    GoalPacket {
        packet_id,
        intent_id: intent.tx_id,
        sender: intent.sender,
        desired_outcome,
        constraints: GoalConstraints {
            min_output_amount: min_output,
            max_slippage_bps: max_slippage,
            allowed_object_ids: None,
            allowed_object_types,
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 16,
        },
        policy: GoalPolicyEnvelope::default(),
        max_fee: intent.max_fee,
        deadline_epoch: if intent.deadline_epoch > 0 {
            intent.deadline_epoch
        } else {
            epoch + 100
        },
        nonce: intent.nonce,
        metadata,
    }
}

fn intent_type_name(intent: &IntentType) -> &'static str {
    match intent {
        IntentType::Transfer(_) => "transfer",
        IntentType::Swap(_) => "swap",
        IntentType::YieldAllocate(_) => "yield_allocate",
        IntentType::TreasuryRebalance(_) => "treasury_rebalance",
        IntentType::RouteLiquidity(_) => "route_liquidity",
        IntentType::ContractCall(_) => "contract_call",
    }
}
