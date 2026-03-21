use crate::capabilities::checker::Capability;
use crate::objects::base::{AccessMode, ObjectAccess, ObjectId, ObjectMeta, ObjectType, ObjectVersion};
use serde::{Deserialize, Serialize};

// ──────────────────────────────────────────────
// Helper: hex-encode a 32-byte array for serde
// Used only for JSON (human-readable) outputs; bincode encode() bypasses these.
// ──────────────────────────────────────────────
mod hex_bytes32 {
    use serde::{Deserialize, Deserializer, Serializer};

    pub fn serialize<S: Serializer>(bytes: &[u8; 32], s: S) -> Result<S::Ok, S::Error> {
        if s.is_human_readable() {
            s.serialize_str(&hex::encode(bytes))
        } else {
            // bincode path: serialize as raw bytes tuple (32 individual u8s)
            use serde::ser::SerializeTuple;
            let mut tup = s.serialize_tuple(32)?;
            for b in bytes.iter() {
                tup.serialize_element(b)?;
            }
            tup.end()
        }
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<[u8; 32], D::Error> {
        if d.is_human_readable() {
            let st = String::deserialize(d)?;
            let v = hex::decode(&st).map_err(serde::de::Error::custom)?;
            if v.len() != 32 {
                return Err(serde::de::Error::custom("expected 32 bytes"));
            }
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&v);
            Ok(arr)
        } else {
            // bincode path: deserialize as raw bytes tuple
            use serde::de::SeqAccess;
            struct Visitor;
            impl<'de> serde::de::Visitor<'de> for Visitor {
                type Value = [u8; 32];
                fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
                    write!(f, "a 32-byte tuple")
                }
                fn visit_seq<A: SeqAccess<'de>>(self, mut seq: A) -> Result<[u8; 32], A::Error> {
                    let mut arr = [0u8; 32];
                    for i in 0..32 {
                        arr[i] = seq.next_element::<u8>()?.ok_or_else(|| {
                            serde::de::Error::invalid_length(i, &"32 bytes")
                        })?;
                    }
                    Ok(arr)
                }
            }
            d.deserialize_tuple(32, Visitor)
        }
    }
}

mod hex_bytes32_vec {
    use serde::{Deserialize, Deserializer, Serializer};
    use serde::ser::SerializeSeq;

    pub fn serialize<S: Serializer>(v: &Vec<[u8; 32]>, s: S) -> Result<S::Ok, S::Error> {
        if s.is_human_readable() {
            let mut seq = s.serialize_seq(Some(v.len()))?;
            for item in v {
                seq.serialize_element(&hex::encode(item))?;
            }
            seq.end()
        } else {
            // bincode path: serialize as seq of 32-byte tuples
            let mut seq = s.serialize_seq(Some(v.len()))?;
            for item in v {
                // serialize each [u8;32] as a newtype wrapping a fixed-size array
                seq.serialize_element(item)?;
            }
            seq.end()
        }
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<Vec<[u8; 32]>, D::Error> {
        if d.is_human_readable() {
            let strs = Vec::<String>::deserialize(d)?;
            strs.iter()
                .map(|st| {
                    let v = hex::decode(st).map_err(serde::de::Error::custom)?;
                    if v.len() != 32 {
                        return Err(serde::de::Error::custom("expected 32 bytes"));
                    }
                    let mut arr = [0u8; 32];
                    arr.copy_from_slice(&v);
                    Ok(arr)
                })
                .collect()
        } else {
            Vec::<[u8; 32]>::deserialize(d)
        }
    }
}

mod hex_bytes32_opt {
    use serde::{Deserialize, Deserializer, Serializer};

    pub fn serialize<S: Serializer>(v: &Option<[u8; 32]>, s: S) -> Result<S::Ok, S::Error> {
        use serde::Serialize as _;
        if s.is_human_readable() {
            match v {
                None => s.serialize_none(),
                Some(b) => s.serialize_str(&hex::encode(b)),
            }
        } else {
            v.serialize(s)
        }
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<Option<[u8; 32]>, D::Error> {
        if d.is_human_readable() {
            let opt = Option::<String>::deserialize(d)?;
            match opt {
                None => Ok(None),
                Some(st) => {
                    let v = hex::decode(&st).map_err(serde::de::Error::custom)?;
                    if v.len() != 32 {
                        return Err(serde::de::Error::custom("expected 32 bytes"));
                    }
                    let mut arr = [0u8; 32];
                    arr.copy_from_slice(&v);
                    Ok(Some(arr))
                }
            }
        } else {
            Option::<[u8; 32]>::deserialize(d)
        }
    }
}

// ──────────────────────────────────────────────
// Trait import
// ──────────────────────────────────────────────
use crate::objects::base::Object;

// ──────────────────────────────────────────────
// WalletObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalletObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub address: [u8; 32],
    #[serde(with = "hex_bytes32_vec")]
    pub authorized_keys: Vec<[u8; 32]>,
    pub nonce: u64,
}

impl WalletObject {
    pub fn new(id: ObjectId, owner: [u8; 32], address: [u8; 32], now: u64) -> Self {
        WalletObject {
            meta: ObjectMeta::new(id, ObjectType::Wallet, owner, now),
            address,
            authorized_keys: vec![owner],
            nonce: 0,
        }
    }
}

impl Object for WalletObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Wallet }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::WriteObject]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("WalletObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// BalanceObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BalanceObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub owner: [u8; 32],
    #[serde(with = "hex_bytes32")]
    pub asset_id: [u8; 32],
    pub amount: u128,
    pub locked_amount: u128,
}

impl BalanceObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        asset_id: [u8; 32],
        amount: u128,
        now: u64,
    ) -> Self {
        BalanceObject {
            meta: ObjectMeta::new(id, ObjectType::Balance, owner, now),
            owner,
            asset_id,
            amount,
            locked_amount: 0,
        }
    }

    pub fn available(&self) -> u128 {
        self.amount.saturating_sub(self.locked_amount)
    }
}

impl Object for BalanceObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Balance }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::TransferAsset]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("BalanceObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// TokenObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub asset_id: [u8; 32],
    pub symbol: String,
    pub decimals: u8,
    pub total_supply: u128,
    pub max_supply: Option<u128>,
    #[serde(with = "hex_bytes32_opt")]
    pub mint_authority: Option<[u8; 32]>,
}

impl TokenObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        asset_id: [u8; 32],
        symbol: String,
        decimals: u8,
        total_supply: u128,
        now: u64,
    ) -> Self {
        TokenObject {
            meta: ObjectMeta::new(id, ObjectType::Token, owner, now),
            asset_id,
            symbol,
            decimals,
            total_supply,
            max_supply: None,
            mint_authority: Some(owner),
        }
    }
}

impl Object for TokenObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Token }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::MintAsset, Capability::BurnAsset]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("TokenObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// LiquidityPoolObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidityPoolObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub asset_a: [u8; 32],
    #[serde(with = "hex_bytes32")]
    pub asset_b: [u8; 32],
    pub reserve_a: u128,
    pub reserve_b: u128,
    /// Fee in basis points (1 bps = 0.01%)
    pub fee_bps: u32,
    #[serde(with = "hex_bytes32")]
    pub lp_token_id: [u8; 32],
    pub total_lp_supply: u128,
}

impl LiquidityPoolObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        asset_a: [u8; 32],
        asset_b: [u8; 32],
        reserve_a: u128,
        reserve_b: u128,
        fee_bps: u32,
        lp_token_id: [u8; 32],
        now: u64,
    ) -> Self {
        LiquidityPoolObject {
            meta: ObjectMeta::new(id, ObjectType::LiquidityPool, owner, now),
            asset_a,
            asset_b,
            reserve_a,
            reserve_b,
            fee_bps,
            lp_token_id,
            total_lp_supply: 0,
        }
    }

    /// Constant product AMM output computed via U256 to prevent overflow.
    /// output = (input_amount * (10000 - fee_bps) * reserve_out) /
    ///          (reserve_in * 10000 + input_amount * (10000 - fee_bps))
    pub fn compute_output(&self, input_amount: u128, input_is_a: bool) -> u128 {
        use primitive_types::U256;
        let (reserve_in, reserve_out) = if input_is_a {
            (self.reserve_a, self.reserve_b)
        } else {
            (self.reserve_b, self.reserve_a)
        };
        let input = U256::from(input_amount);
        let r_in = U256::from(reserve_in);
        let r_out = U256::from(reserve_out);
        let fee = U256::from(self.fee_bps);
        let scale = U256::from(10_000u32);

        let fee_factor = scale - fee;
        let numerator = input * fee_factor * r_out;
        let denominator = r_in * scale + input * fee_factor;
        if denominator.is_zero() {
            return 0;
        }
        let result = numerator / denominator;
        // Safe: result <= reserve_out which is a u128
        if result > U256::from(u128::MAX) {
            return u128::MAX;
        }
        result.as_u128()
    }
}

impl Object for LiquidityPoolObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::LiquidityPool }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::ProvideLiquidity]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("LiquidityPoolObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// VaultObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VaultObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub asset_id: [u8; 32],
    pub deposited_amount: u128,
    #[serde(with = "hex_bytes32_opt")]
    pub strategy_id: Option<[u8; 32]>,
    pub withdrawal_lock_until: u64,
}

impl VaultObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        asset_id: [u8; 32],
        now: u64,
    ) -> Self {
        VaultObject {
            meta: ObjectMeta::new(id, ObjectType::Vault, owner, now),
            asset_id,
            deposited_amount: 0,
            strategy_id: None,
            withdrawal_lock_until: 0,
        }
    }
}

impl Object for VaultObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Vault }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::WriteObject]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("VaultObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// GovernanceProposalObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ProposalStatus {
    Active,
    Passed,
    Rejected,
    Expired,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GovernanceProposalObject {
    pub meta: ObjectMeta,
    pub proposal_id: u64,
    #[serde(with = "hex_bytes32")]
    pub proposer: [u8; 32],
    pub title: String,
    pub description: String,
    pub vote_yes: u128,
    pub vote_no: u128,
    pub vote_abstain: u128,
    pub deadline_epoch: u64,
    pub status: ProposalStatus,
}

impl GovernanceProposalObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        proposal_id: u64,
        proposer: [u8; 32],
        title: String,
        description: String,
        deadline_epoch: u64,
        now: u64,
    ) -> Self {
        GovernanceProposalObject {
            meta: ObjectMeta::new(id, ObjectType::GovernanceProposal, owner, now),
            proposal_id,
            proposer,
            title,
            description,
            vote_yes: 0,
            vote_no: 0,
            vote_abstain: 0,
            deadline_epoch,
            status: ProposalStatus::Active,
        }
    }
}

impl Object for GovernanceProposalObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::GovernanceProposal }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::ModifyGovernance]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("GovernanceProposalObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// IdentityObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IdentityObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub address: [u8; 32],
    pub display_name: String,
    pub verified: bool,
    pub reputation_score: u64,
    pub kyc_tier: u8,
}

impl IdentityObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        address: [u8; 32],
        display_name: String,
        now: u64,
    ) -> Self {
        IdentityObject {
            meta: ObjectMeta::new(id, ObjectType::Identity, owner, now),
            address,
            display_name,
            verified: false,
            reputation_score: 0,
            kyc_tier: 0,
        }
    }
}

impl Object for IdentityObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Identity }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::UpdateIdentity]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("IdentityObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// ExecutionReceiptObject
// ──────────────────────────────────────────────
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionReceiptObject {
    pub meta: ObjectMeta,
    #[serde(with = "hex_bytes32")]
    pub tx_id: [u8; 32],
    pub success: bool,
    pub affected_objects: Vec<ObjectId>,
    pub old_versions: Vec<(ObjectId, ObjectVersion)>,
    pub new_versions: Vec<(ObjectId, ObjectVersion)>,
    pub error_code: Option<u32>,
    pub gas_used: u64,
    pub executed_at_epoch: u64,
}

impl ExecutionReceiptObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        tx_id: [u8; 32],
        success: bool,
        now: u64,
    ) -> Self {
        ExecutionReceiptObject {
            meta: ObjectMeta::new(id, ObjectType::ExecutionReceipt, owner, now),
            tx_id,
            success,
            affected_objects: vec![],
            old_versions: vec![],
            new_versions: vec![],
            error_code: None,
            gas_used: 0,
            executed_at_epoch: now,
        }
    }
}

impl Object for ExecutionReceiptObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::ExecutionReceipt }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::WriteObject]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("ExecutionReceiptObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// ContractObject — Intent Contract state container
// ──────────────────────────────────────────────

/// An Intent Contract object. Stores opaque contract state that is validated
/// by the contract's Wasm constraint validator and mutated by solver plans.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractObject {
    pub meta: ObjectMeta,
    /// Schema ID linking to the on-chain contract schema and constraint validator.
    #[serde(with = "hex_bytes32")]
    pub schema_id: [u8; 32],
    /// Opaque contract state (bincode-serialized, schema-specific).
    pub state: Vec<u8>,
    /// SHA256 hash of `state` for quick equality checks.
    #[serde(with = "hex_bytes32")]
    pub state_hash: [u8; 32],
    /// Address that instantiated this contract object.
    #[serde(with = "hex_bytes32")]
    pub created_by: [u8; 32],
    /// Optional admin address for contract migration.
    #[serde(with = "hex_bytes32_opt")]
    pub admin: Option<[u8; 32]>,
    /// Maximum allowed state size in bytes (from schema).
    pub max_state_bytes: u64,
}

impl ContractObject {
    pub fn new(
        id: ObjectId,
        owner: [u8; 32],
        schema_id: [u8; 32],
        initial_state: Vec<u8>,
        max_state_bytes: u64,
        now: u64,
    ) -> Self {
        use sha2::{Digest, Sha256};
        let state_hash = {
            let mut hasher = Sha256::new();
            hasher.update(&initial_state);
            let h = hasher.finalize();
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };
        ContractObject {
            meta: ObjectMeta::new(id, ObjectType::Contract(schema_id), owner, now),
            schema_id,
            state: initial_state,
            state_hash,
            created_by: owner,
            admin: Some(owner),
            max_state_bytes,
        }
    }

    /// Update the contract state. Recomputes state_hash.
    pub fn set_state(&mut self, new_state: Vec<u8>) {
        use sha2::{Digest, Sha256};
        let mut hasher = Sha256::new();
        hasher.update(&new_state);
        let h = hasher.finalize();
        self.state_hash.copy_from_slice(&h);
        self.state = new_state;
    }

    /// Returns the current state size in bytes.
    pub fn state_size(&self) -> u64 {
        self.state.len() as u64
    }
}

impl Object for ContractObject {
    fn meta(&self) -> &ObjectMeta { &self.meta }
    fn meta_mut(&mut self) -> &mut ObjectMeta { &mut self.meta }
    fn object_type(&self) -> ObjectType { ObjectType::Contract(self.schema_id) }
    fn required_capabilities_for_write(&self) -> Vec<Capability> {
        vec![Capability::WriteObject, Capability::ContractCall(self.schema_id)]
    }
    fn encode(&self) -> Vec<u8> {
        bincode::serialize(self).expect("ContractObject bincode serialization is infallible")
    }
}

// ──────────────────────────────────────────────
// Re-export ObjectAccess helper constructors
// ──────────────────────────────────────────────
impl ObjectAccess {
    pub fn read(object_id: ObjectId) -> Self {
        ObjectAccess { object_id, mode: AccessMode::ReadOnly }
    }

    pub fn write(object_id: ObjectId) -> Self {
        ObjectAccess { object_id, mode: AccessMode::ReadWrite }
    }
}
