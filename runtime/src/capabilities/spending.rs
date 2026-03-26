//! Capability-Based Spending — replaces ERC-20 style token approvals.
//!
//! Instead of global `approve(spender, amount)` with no expiry and no scope,
//! Omniphi uses ephemeral SpendCapabilities that are:
//! - Bounded by amount AND time (no infinite approvals)
//! - Optionally scoped to a specific intent (scope_hash)
//! - Revocable by owner at any time
//! - Auto-expired when deadline passes
//! - Deterministically consumed (remaining decreases on use)
//!
//! This is safer than ERC-20 approvals because:
//! 1. No "approve(MAX_UINT)" pattern — amount must be explicit
//! 2. No permanent approvals — every capability has an expiration
//! 3. Intent-scoped spending — a capability can only be used for one specific intent
//! 4. Owner can revoke before expiry — no waiting for spender to return tokens
//! 5. Deterministic consumption — partial use reduces remaining, preventing race conditions

use crate::objects::base::ObjectId;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// Status of a spend capability.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SpendCapabilityStatus {
    /// Active and usable.
    Active,
    /// Fully consumed (remaining_amount == 0).
    Consumed,
    /// Revoked by owner before expiry.
    Revoked,
    /// Expired (deadline_epoch passed).
    Expired,
}

/// An ephemeral, scoped, capability-based asset spend right.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SpendCapability {
    /// Unique capability identifier.
    pub capability_id: [u8; 32],
    /// Owner who granted this capability (their balance will be debited).
    pub owner: [u8; 32],
    /// Address allowed to spend on behalf of owner.
    pub allowed_spender: [u8; 32],
    /// Asset this capability applies to.
    pub asset_id: [u8; 32],
    /// Maximum total amount that can be spent via this capability.
    pub max_amount: u128,
    /// Amount remaining (decreases on each consumption).
    pub remaining_amount: u128,
    /// Epoch after which this capability is no longer valid.
    pub expiration_epoch: u64,
    /// Optional scope hash binding this capability to a specific intent.
    /// When set, the capability can only be consumed by a transaction
    /// whose tx_id hashes to this value.
    pub scope_hash: Option<[u8; 32]>,
    /// Current status.
    pub status: SpendCapabilityStatus,
    /// Epoch when this capability was created.
    pub created_at_epoch: u64,
}

impl SpendCapability {
    /// Compute a deterministic capability ID from creation parameters.
    pub fn compute_id(
        owner: &[u8; 32],
        spender: &[u8; 32],
        asset_id: &[u8; 32],
        max_amount: u128,
        nonce: u64,
    ) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_SPEND_CAP_V1");
        h.update(owner);
        h.update(spender);
        h.update(asset_id);
        h.update(&max_amount.to_be_bytes());
        h.update(&nonce.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }

    /// Create a new spend capability.
    pub fn create(
        owner: [u8; 32],
        allowed_spender: [u8; 32],
        asset_id: [u8; 32],
        max_amount: u128,
        expiration_epoch: u64,
        scope_hash: Option<[u8; 32]>,
        current_epoch: u64,
        nonce: u64,
    ) -> Result<Self, String> {
        if owner == [0u8; 32] { return Err("owner cannot be zero".to_string()); }
        if allowed_spender == [0u8; 32] { return Err("spender cannot be zero".to_string()); }
        if owner == allowed_spender { return Err("owner and spender must differ".to_string()); }
        if max_amount == 0 { return Err("max_amount must be > 0".to_string()); }
        if expiration_epoch <= current_epoch {
            return Err(format!("expiration {} must be > current epoch {}", expiration_epoch, current_epoch));
        }

        let capability_id = Self::compute_id(&owner, &allowed_spender, &asset_id, max_amount, nonce);

        Ok(SpendCapability {
            capability_id,
            owner,
            allowed_spender,
            asset_id,
            max_amount,
            remaining_amount: max_amount,
            expiration_epoch,
            scope_hash,
            status: SpendCapabilityStatus::Active,
            created_at_epoch: current_epoch,
        })
    }

    /// Check if this capability is currently usable.
    pub fn is_usable(&self, current_epoch: u64) -> bool {
        self.status == SpendCapabilityStatus::Active && current_epoch < self.expiration_epoch
    }

    /// Consume part or all of this capability.
    /// Returns the actual amount consumed (may be less than requested if remaining < amount).
    pub fn consume(
        &mut self,
        spender: &[u8; 32],
        amount: u128,
        current_epoch: u64,
        tx_scope_hash: Option<&[u8; 32]>,
    ) -> Result<u128, String> {
        // Status check
        if self.status != SpendCapabilityStatus::Active {
            return Err(format!("capability is {:?}, not Active", self.status));
        }

        // Expiration check
        if current_epoch >= self.expiration_epoch {
            self.status = SpendCapabilityStatus::Expired;
            return Err("capability has expired".to_string());
        }

        // Spender check
        if spender != &self.allowed_spender {
            return Err("unauthorized spender".to_string());
        }

        // Scope check
        if let Some(required_scope) = &self.scope_hash {
            match tx_scope_hash {
                Some(actual) if actual == required_scope => {}
                Some(_) => return Err("scope_hash mismatch".to_string()),
                None => return Err("capability is intent-scoped but no scope_hash provided".to_string()),
            }
        }

        // Amount check
        if amount == 0 {
            return Err("consume amount must be > 0".to_string());
        }
        if amount > self.remaining_amount {
            return Err(format!(
                "insufficient capability: requested {}, remaining {}",
                amount, self.remaining_amount
            ));
        }

        // Consume
        self.remaining_amount -= amount;
        if self.remaining_amount == 0 {
            self.status = SpendCapabilityStatus::Consumed;
        }

        Ok(amount)
    }

    /// Revoke this capability. Only the owner can revoke.
    pub fn revoke(&mut self, caller: &[u8; 32]) -> Result<(), String> {
        if caller != &self.owner {
            return Err("only owner can revoke".to_string());
        }
        if self.status != SpendCapabilityStatus::Active {
            return Err(format!("cannot revoke: status is {:?}", self.status));
        }
        self.status = SpendCapabilityStatus::Revoked;
        Ok(())
    }

    /// Check and transition to Expired if past deadline.
    pub fn check_expiration(&mut self, current_epoch: u64) {
        if self.status == SpendCapabilityStatus::Active && current_epoch >= self.expiration_epoch {
            self.status = SpendCapabilityStatus::Expired;
        }
    }
}

/// Registry of spend capabilities, indexed for fast lookup.
#[derive(Debug, Clone, Default)]
pub struct SpendCapabilityRegistry {
    /// capability_id → SpendCapability
    capabilities: BTreeMap<[u8; 32], SpendCapability>,
    /// (owner, spender, asset_id) → list of capability_ids
    by_grant: BTreeMap<([u8; 32], [u8; 32], [u8; 32]), Vec<[u8; 32]>>,
    /// owner → list of capability_ids (for revocation lookup)
    by_owner: BTreeMap<[u8; 32], Vec<[u8; 32]>>,
    /// Monotonic nonce per owner for deterministic ID generation
    owner_nonces: BTreeMap<[u8; 32], u64>,
}

impl SpendCapabilityRegistry {
    pub fn new() -> Self { SpendCapabilityRegistry::default() }

    /// Create and register a new spend capability.
    pub fn create(
        &mut self,
        owner: [u8; 32],
        spender: [u8; 32],
        asset_id: [u8; 32],
        max_amount: u128,
        expiration_epoch: u64,
        scope_hash: Option<[u8; 32]>,
        current_epoch: u64,
    ) -> Result<[u8; 32], String> {
        let nonce = self.owner_nonces.entry(owner).or_insert(0);
        let cap = SpendCapability::create(
            owner, spender, asset_id, max_amount, expiration_epoch,
            scope_hash, current_epoch, *nonce,
        )?;
        *nonce += 1;

        let id = cap.capability_id;

        // Index
        self.by_grant.entry((owner, spender, asset_id)).or_default().push(id);
        self.by_owner.entry(owner).or_default().push(id);
        self.capabilities.insert(id, cap);

        Ok(id)
    }

    /// Look up a capability by ID.
    pub fn get(&self, capability_id: &[u8; 32]) -> Option<&SpendCapability> {
        self.capabilities.get(capability_id)
    }

    /// Look up a capability by ID (mutable).
    pub fn get_mut(&mut self, capability_id: &[u8; 32]) -> Option<&mut SpendCapability> {
        self.capabilities.get_mut(capability_id)
    }

    /// Consume from a specific capability.
    pub fn consume(
        &mut self,
        capability_id: &[u8; 32],
        spender: &[u8; 32],
        amount: u128,
        current_epoch: u64,
        tx_scope_hash: Option<&[u8; 32]>,
    ) -> Result<u128, String> {
        let cap = self.capabilities.get_mut(capability_id)
            .ok_or_else(|| "capability not found".to_string())?;
        cap.consume(spender, amount, current_epoch, tx_scope_hash)
    }

    /// Find the best active capability for a (owner, spender, asset) tuple.
    /// Returns the one with the most remaining amount.
    pub fn find_active(
        &self,
        owner: &[u8; 32],
        spender: &[u8; 32],
        asset_id: &[u8; 32],
        current_epoch: u64,
    ) -> Option<&SpendCapability> {
        let key = (*owner, *spender, *asset_id);
        let ids = self.by_grant.get(&key)?;
        ids.iter()
            .filter_map(|id| self.capabilities.get(id))
            .filter(|c| c.is_usable(current_epoch))
            .max_by_key(|c| c.remaining_amount)
    }

    /// Revoke a capability.
    pub fn revoke(
        &mut self,
        capability_id: &[u8; 32],
        caller: &[u8; 32],
    ) -> Result<(), String> {
        let cap = self.capabilities.get_mut(capability_id)
            .ok_or_else(|| "capability not found".to_string())?;
        cap.revoke(caller)
    }

    /// Revoke ALL active capabilities granted by an owner.
    pub fn revoke_all_by_owner(&mut self, owner: &[u8; 32]) -> usize {
        let ids: Vec<[u8; 32]> = self.by_owner.get(owner)
            .cloned()
            .unwrap_or_default();
        let mut count = 0;
        for id in &ids {
            if let Some(cap) = self.capabilities.get_mut(id) {
                if cap.status == SpendCapabilityStatus::Active {
                    cap.status = SpendCapabilityStatus::Revoked;
                    count += 1;
                }
            }
        }
        count
    }

    /// Expire all capabilities past their deadline.
    pub fn expire_stale(&mut self, current_epoch: u64) -> usize {
        let mut count = 0;
        for cap in self.capabilities.values_mut() {
            if cap.status == SpendCapabilityStatus::Active && current_epoch >= cap.expiration_epoch {
                cap.status = SpendCapabilityStatus::Expired;
                count += 1;
            }
        }
        count
    }

    /// Total number of registered capabilities (all statuses).
    pub fn count(&self) -> usize {
        self.capabilities.len()
    }

    /// Total active capabilities.
    pub fn active_count(&self, current_epoch: u64) -> usize {
        self.capabilities.values().filter(|c| c.is_usable(current_epoch)).count()
    }

    /// Prune non-active capabilities older than `before_epoch`.
    pub fn prune(&mut self, before_epoch: u64) -> usize {
        let before = self.capabilities.len();
        self.capabilities.retain(|_, c| {
            c.status == SpendCapabilityStatus::Active || c.created_at_epoch >= before_epoch
        });
        before - self.capabilities.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn owner() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 1; b }
    fn spender() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 2; b }
    fn asset() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 0xAA; b }
    fn other() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 3; b }

    // ── Valid creation ───────────────────────────────────────

    #[test]
    fn test_valid_capability_creation() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();
        let cap = reg.get(&id).unwrap();
        assert_eq!(cap.max_amount, 1000);
        assert_eq!(cap.remaining_amount, 1000);
        assert_eq!(cap.status, SpendCapabilityStatus::Active);
        assert_eq!(cap.expiration_epoch, 100);
        assert!(cap.scope_hash.is_none());
    }

    #[test]
    fn test_scoped_capability_creation() {
        let mut reg = SpendCapabilityRegistry::new();
        let scope = [0xBB; 32];
        let id = reg.create(owner(), spender(), asset(), 500, 50, Some(scope), 5).unwrap();
        let cap = reg.get(&id).unwrap();
        assert_eq!(cap.scope_hash, Some(scope));
    }

    // ── Invalid creation ─────────────────────────────────────

    #[test]
    fn test_zero_owner_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        assert!(reg.create([0u8; 32], spender(), asset(), 100, 50, None, 5).is_err());
    }

    #[test]
    fn test_zero_spender_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        assert!(reg.create(owner(), [0u8; 32], asset(), 100, 50, None, 5).is_err());
    }

    #[test]
    fn test_same_owner_spender_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        assert!(reg.create(owner(), owner(), asset(), 100, 50, None, 5).is_err());
    }

    #[test]
    fn test_zero_amount_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        assert!(reg.create(owner(), spender(), asset(), 0, 50, None, 5).is_err());
    }

    #[test]
    fn test_expired_at_creation_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        // expiration_epoch (5) <= current_epoch (10)
        assert!(reg.create(owner(), spender(), asset(), 100, 5, None, 10).is_err());
    }

    // ── Valid consumption ────────────────────────────────────

    #[test]
    fn test_valid_consumption() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        let consumed = reg.consume(&id, &spender(), 400, 20, None).unwrap();
        assert_eq!(consumed, 400);
        assert_eq!(reg.get(&id).unwrap().remaining_amount, 600);
        assert_eq!(reg.get(&id).unwrap().status, SpendCapabilityStatus::Active);
    }

    #[test]
    fn test_full_consumption_marks_consumed() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 500, 100, None, 10).unwrap();

        reg.consume(&id, &spender(), 500, 20, None).unwrap();
        assert_eq!(reg.get(&id).unwrap().remaining_amount, 0);
        assert_eq!(reg.get(&id).unwrap().status, SpendCapabilityStatus::Consumed);
    }

    #[test]
    fn test_partial_then_full_consumption() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        reg.consume(&id, &spender(), 300, 15, None).unwrap();
        reg.consume(&id, &spender(), 700, 20, None).unwrap();
        assert_eq!(reg.get(&id).unwrap().status, SpendCapabilityStatus::Consumed);
    }

    // ── Over-consumption rejection ───────────────────────────

    #[test]
    fn test_over_consumption_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 500, 100, None, 10).unwrap();

        let err = reg.consume(&id, &spender(), 600, 20, None).unwrap_err();
        assert!(err.contains("insufficient"), "Expected insufficient, got: {}", err);
        // Remaining unchanged
        assert_eq!(reg.get(&id).unwrap().remaining_amount, 500);
    }

    // ── Expired capability rejection ─────────────────────────

    #[test]
    fn test_expired_capability_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 50, None, 10).unwrap();

        // Try to consume at epoch 60 (past expiration 50)
        let err = reg.consume(&id, &spender(), 100, 60, None).unwrap_err();
        assert!(err.contains("expired"), "Expected expired, got: {}", err);
        assert_eq!(reg.get(&id).unwrap().status, SpendCapabilityStatus::Expired);
    }

    // ── Revoked capability rejection ─────────────────────────

    #[test]
    fn test_revoked_capability_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        reg.revoke(&id, &owner()).unwrap();
        assert_eq!(reg.get(&id).unwrap().status, SpendCapabilityStatus::Revoked);

        let err = reg.consume(&id, &spender(), 100, 20, None).unwrap_err();
        assert!(err.contains("Revoked"), "Expected revoked, got: {}", err);
    }

    #[test]
    fn test_non_owner_cannot_revoke() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        let err = reg.revoke(&id, &spender()).unwrap_err();
        assert!(err.contains("only owner"), "Expected owner error, got: {}", err);
    }

    // ── Intent-scoped capability ─────────────────────────────

    #[test]
    fn test_scoped_capability_correct_scope() {
        let mut reg = SpendCapabilityRegistry::new();
        let scope = [0xCC; 32];
        let id = reg.create(owner(), spender(), asset(), 1000, 100, Some(scope), 10).unwrap();

        let consumed = reg.consume(&id, &spender(), 500, 20, Some(&scope)).unwrap();
        assert_eq!(consumed, 500);
    }

    #[test]
    fn test_scoped_capability_wrong_scope_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let scope = [0xCC; 32];
        let wrong_scope = [0xDD; 32];
        let id = reg.create(owner(), spender(), asset(), 1000, 100, Some(scope), 10).unwrap();

        let err = reg.consume(&id, &spender(), 500, 20, Some(&wrong_scope)).unwrap_err();
        assert!(err.contains("scope_hash mismatch"), "Expected scope error, got: {}", err);
    }

    #[test]
    fn test_scoped_capability_no_scope_provided_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let scope = [0xCC; 32];
        let id = reg.create(owner(), spender(), asset(), 1000, 100, Some(scope), 10).unwrap();

        let err = reg.consume(&id, &spender(), 500, 20, None).unwrap_err();
        assert!(err.contains("scope_hash provided"), "Expected scope error, got: {}", err);
    }

    // ── Unauthorized spender rejection ───────────────────────

    #[test]
    fn test_unauthorized_spender_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        let err = reg.consume(&id, &other(), 100, 20, None).unwrap_err();
        assert!(err.contains("unauthorized"), "Expected unauthorized, got: {}", err);
    }

    // ── Registry operations ──────────────────────────────────

    #[test]
    fn test_find_active_capability() {
        let mut reg = SpendCapabilityRegistry::new();
        reg.create(owner(), spender(), asset(), 500, 100, None, 10).unwrap();
        reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        let best = reg.find_active(&owner(), &spender(), &asset(), 20).unwrap();
        assert_eq!(best.remaining_amount, 1000); // returns highest remaining
    }

    #[test]
    fn test_revoke_all_by_owner() {
        let mut reg = SpendCapabilityRegistry::new();
        reg.create(owner(), spender(), asset(), 500, 100, None, 10).unwrap();
        reg.create(owner(), other(), asset(), 300, 100, None, 10).unwrap();

        let revoked = reg.revoke_all_by_owner(&owner());
        assert_eq!(revoked, 2);
        assert_eq!(reg.active_count(20), 0);
    }

    #[test]
    fn test_expire_stale() {
        let mut reg = SpendCapabilityRegistry::new();
        reg.create(owner(), spender(), asset(), 500, 30, None, 10).unwrap(); // expires at 30
        reg.create(owner(), other(), asset(), 300, 100, None, 10).unwrap();  // expires at 100

        let expired = reg.expire_stale(50);
        assert_eq!(expired, 1); // only first one expired
        assert_eq!(reg.active_count(50), 1);
    }

    #[test]
    fn test_consumed_capability_cannot_be_reused() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 100, 100, None, 10).unwrap();

        reg.consume(&id, &spender(), 100, 20, None).unwrap(); // fully consumed
        let err = reg.consume(&id, &spender(), 1, 25, None).unwrap_err();
        assert!(err.contains("Consumed"), "Expected consumed, got: {}", err);
    }

    #[test]
    fn test_zero_consume_amount_rejected() {
        let mut reg = SpendCapabilityRegistry::new();
        let id = reg.create(owner(), spender(), asset(), 1000, 100, None, 10).unwrap();

        let err = reg.consume(&id, &spender(), 0, 20, None).unwrap_err();
        assert!(err.contains("must be > 0"), "Expected amount error, got: {}", err);
    }

    #[test]
    fn test_capability_id_deterministic() {
        let id1 = SpendCapability::compute_id(&owner(), &spender(), &asset(), 1000, 0);
        let id2 = SpendCapability::compute_id(&owner(), &spender(), &asset(), 1000, 0);
        assert_eq!(id1, id2);

        let id3 = SpendCapability::compute_id(&owner(), &spender(), &asset(), 1000, 1);
        assert_ne!(id1, id3); // different nonce
    }
}
