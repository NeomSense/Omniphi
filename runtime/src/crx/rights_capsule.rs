use crate::objects::base::ObjectType;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

#[derive(
    Debug,
    Clone,
    PartialEq,
    Eq,
    PartialOrd,
    Ord,
    Hash,
    serde::Serialize,
    serde::Deserialize,
)]
pub enum AllowedActionType {
    Read,
    DebitBalance,
    CreditBalance,
    SwapPoolAmounts,
    LockBalance,
    UnlockBalance,
    CreateObject,
    UpdateVersion,
    EmitReceipt,
    InvokeSafetyHook,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct AllowedActionSet(pub BTreeSet<AllowedActionType>);

impl AllowedActionSet {
    /// All possible actions.
    pub fn full() -> Self {
        let mut s = BTreeSet::new();
        s.insert(AllowedActionType::Read);
        s.insert(AllowedActionType::DebitBalance);
        s.insert(AllowedActionType::CreditBalance);
        s.insert(AllowedActionType::SwapPoolAmounts);
        s.insert(AllowedActionType::LockBalance);
        s.insert(AllowedActionType::UnlockBalance);
        s.insert(AllowedActionType::CreateObject);
        s.insert(AllowedActionType::UpdateVersion);
        s.insert(AllowedActionType::EmitReceipt);
        s.insert(AllowedActionType::InvokeSafetyHook);
        AllowedActionSet(s)
    }

    pub fn read_only() -> Self {
        let mut s = BTreeSet::new();
        s.insert(AllowedActionType::Read);
        AllowedActionSet(s)
    }

    /// DebitBalance + CreditBalance + UpdateVersion + EmitReceipt + Read
    pub fn transfer_only() -> Self {
        let mut s = BTreeSet::new();
        s.insert(AllowedActionType::Read);
        s.insert(AllowedActionType::DebitBalance);
        s.insert(AllowedActionType::CreditBalance);
        s.insert(AllowedActionType::UpdateVersion);
        s.insert(AllowedActionType::EmitReceipt);
        AllowedActionSet(s)
    }

    /// DebitBalance + CreditBalance + SwapPoolAmounts + UpdateVersion + EmitReceipt + Read
    pub fn swap_only() -> Self {
        let mut s = BTreeSet::new();
        s.insert(AllowedActionType::Read);
        s.insert(AllowedActionType::DebitBalance);
        s.insert(AllowedActionType::CreditBalance);
        s.insert(AllowedActionType::SwapPoolAmounts);
        s.insert(AllowedActionType::UpdateVersion);
        s.insert(AllowedActionType::EmitReceipt);
        AllowedActionSet(s)
    }

    pub fn contains(&self, action: &AllowedActionType) -> bool {
        self.0.contains(action)
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct DomainAccessEnvelope {
    pub allowed_domains: BTreeSet<String>,
    pub forbidden_domains: BTreeSet<String>,
}

impl DomainAccessEnvelope {
    pub fn is_domain_allowed(&self, domain: &str) -> bool {
        // forbidden takes precedence over allowed
        !self.forbidden_domains.contains(domain)
            && (self.allowed_domains.is_empty() || self.allowed_domains.contains(domain))
    }

    pub fn unrestricted() -> Self {
        DomainAccessEnvelope {
            allowed_domains: BTreeSet::new(),
            forbidden_domains: BTreeSet::new(),
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RightsScope {
    /// Explicit object whitelist; empty = any allowed by type
    pub allowed_object_ids: BTreeSet<[u8; 32]>,
    pub allowed_object_types: BTreeSet<ObjectType>,
    pub allowed_actions: AllowedActionSet,
    pub domain_envelope: DomainAccessEnvelope,
    /// Cumulative debit cap; 0 = unlimited
    pub max_spend: u128,
    /// 0 = unlimited
    pub max_objects_touched: usize,
    pub allow_rollback: bool,
    pub allow_downgrade: bool,
    pub quarantine_eligible: bool,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct RightsCapsule {
    pub capsule_id: [u8; 32],
    pub goal_packet_id: [u8; 32],
    pub authorized_solver_id: [u8; 32],
    pub scope: RightsScope,
    pub valid_from_epoch: u64,
    pub valid_until_epoch: u64,
    /// SHA256 of bincode of all fields except capsule_hash itself.
    pub capsule_hash: [u8; 32],
    pub metadata: BTreeMap<String, String>,
}

impl RightsCapsule {
    /// Compute hash over all fields EXCEPT capsule_hash.
    pub fn compute_hash(&self) -> [u8; 32] {
        // We serialize a tuple of all fields except capsule_hash
        let mut hasher = Sha256::new();
        hasher.update(&self.capsule_id);
        hasher.update(&self.goal_packet_id);
        hasher.update(&self.authorized_solver_id);
        // serialize scope
        let scope_bytes = bincode::serialize(&self.scope)
            .expect("RightsScope bincode serialization is infallible");
        let scope_len = (scope_bytes.len() as u64).to_le_bytes();
        hasher.update(scope_len);
        hasher.update(&scope_bytes);
        hasher.update(&self.valid_from_epoch.to_le_bytes());
        hasher.update(&self.valid_until_epoch.to_le_bytes());
        // metadata (sorted by BTreeMap)
        for (k, v) in &self.metadata {
            hasher.update(k.as_bytes());
            hasher.update(v.as_bytes());
        }
        hasher.finalize().into()
    }

    pub fn validate_hash(&self) -> bool {
        self.compute_hash() == self.capsule_hash
    }

    pub fn is_valid_at_epoch(&self, epoch: u64) -> bool {
        epoch >= self.valid_from_epoch && epoch <= self.valid_until_epoch
    }

    pub fn can_access_object(&self, object_id: &[u8; 32], object_type: &ObjectType) -> bool {
        (self.scope.allowed_object_ids.is_empty()
            || self.scope.allowed_object_ids.contains(object_id))
            && self.scope.allowed_object_types.contains(object_type)
    }

    pub fn can_perform_action(&self, action: &AllowedActionType) -> bool {
        self.scope.allowed_actions.contains(action)
    }
}
