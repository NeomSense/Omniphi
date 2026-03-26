//! Programmable Ownership Objects
//!
//! Replaces static ERC-721 NFTs with dynamic ownership primitives that support:
//! - Time-locked ownership (cannot transfer until epoch N)
//! - Conditional ownership (transfer only if predicate holds)
//! - Fractional ownership (multiple owners with share tracking)
//! - Delegated usage rights (owner keeps title, delegate gets usage)
//! - Linked object relationships (parent-child, bundle, dependency)
//!
//! ## Why This Is Better Than ERC-721
//!
//! ERC-721 is a static pointer: `tokenId → owner`. That's it. Transfer is
//! unconditional, ownership is binary, and there's no on-chain concept of
//! usage rights, time locks, or fractional shares without layering external
//! contracts on top.
//!
//! Omniphi OwnershipObjects embed these rules in the object itself:
//! - A real estate deed can be time-locked during escrow
//! - A subscription can grant usage rights without transferring title
//! - A DAO treasury asset can have fractional ownership with vote-weighted shares
//! - A game item can be conditionally transferable (only if player level ≥ 10)
//! - A bundle of linked objects transfers atomically

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// How ownership is held.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum OwnershipMode {
    /// Single owner, unconditional transfer.
    Direct,
    /// Single owner, transfer blocked until `unlock_epoch`.
    TimeLocked { unlock_epoch: u64 },
    /// Single owner, transfer requires `condition` to evaluate true.
    Conditional { condition: TransferCondition },
    /// Multiple owners with fractional shares (basis points, sum = 10000).
    Fractional { shares: BTreeMap<[u8; 32], u32> },
    /// Owner retains title; delegate has usage rights until `expires_epoch`.
    Delegated { delegate: [u8; 32], expires_epoch: u64 },
}

/// A condition that must be satisfied for a conditional transfer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TransferCondition {
    /// Transfer only if current epoch ≥ threshold.
    MinEpoch(u64),
    /// Transfer only to an address in the allowlist.
    AllowedRecipients(Vec<[u8; 32]>),
    /// Transfer only if a linked object exists and is owned by the same owner.
    RequiresLinkedOwnership([u8; 32]),
    /// Always true (effectively Direct, but kept as Conditional for auditability).
    AlwaysTrue,
}

/// Rights that can be granted on an ownership object.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum OwnershipRight {
    /// Can view the object's state.
    View,
    /// Can use the object (e.g., equip an item, access a service).
    Use,
    /// Can transfer the object to another owner.
    Transfer,
    /// Can modify the object's metadata.
    Modify,
    /// Can link/unlink child objects.
    ManageLinks,
    /// Can change ownership mode or rights.
    Admin,
}

/// Behavior flags controlling object dynamics.
#[derive(Debug, Clone, Default)]
pub struct BehaviorFlags {
    /// Object cannot be transferred (soulbound).
    pub soulbound: bool,
    /// Object is destroyed after a single use.
    pub consumable: bool,
    /// Object automatically expires and is removed after `expires_epoch`.
    pub expires_epoch: Option<u64>,
    /// Object can be split into fractional shares.
    pub splittable: bool,
    /// Object can be merged with other objects of same type.
    pub mergeable: bool,
}

/// Relationship between ownership objects.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LinkType {
    /// Parent-child (child transfers with parent).
    ParentChild,
    /// Bundle (all linked objects transfer together).
    Bundle,
    /// Dependency (object requires linked object to exist).
    Dependency,
    /// Reference (informational link, no transfer coupling).
    Reference,
}

/// A link to another ownership object.
#[derive(Debug, Clone)]
pub struct ObjectLink {
    pub target_id: [u8; 32],
    pub link_type: LinkType,
}

/// A programmable ownership object.
#[derive(Debug, Clone)]
pub struct OwnershipObject {
    /// Unique object identifier.
    pub object_id: [u8; 32],
    /// Current owner (for Direct/TimeLocked/Conditional/Delegated).
    /// For Fractional, this is the largest shareholder.
    pub owner: [u8; 32],
    /// How ownership is held.
    pub ownership_mode: OwnershipMode,
    /// Rights granted to the owner.
    pub rights: Vec<OwnershipRight>,
    /// Arbitrary metadata (name, description, URI, properties).
    pub metadata: BTreeMap<String, String>,
    /// Behavioral flags.
    pub behavior: BehaviorFlags,
    /// Linked objects.
    pub linked_objects: Vec<ObjectLink>,
    /// Version counter (incremented on every mutation).
    pub version: u64,
    /// Epoch when this object was created.
    pub created_at_epoch: u64,
    /// Epoch when this object was last updated.
    pub updated_at_epoch: u64,
    /// Deterministic state hash.
    pub state_hash: [u8; 32],
}

impl OwnershipObject {
    /// Create a new ownership object with direct ownership.
    pub fn create(
        object_id: [u8; 32],
        owner: [u8; 32],
        mode: OwnershipMode,
        metadata: BTreeMap<String, String>,
        behavior: BehaviorFlags,
        current_epoch: u64,
    ) -> Result<Self, String> {
        if object_id == [0u8; 32] { return Err("object_id must be non-zero".into()); }
        if owner == [0u8; 32] { return Err("owner must be non-zero".into()); }

        // Validate fractional shares sum to 10000
        if let OwnershipMode::Fractional { ref shares } = mode {
            let total: u32 = shares.values().sum();
            if total != 10000 {
                return Err(format!("fractional shares must sum to 10000, got {}", total));
            }
            if shares.is_empty() {
                return Err("fractional ownership requires at least one shareholder".into());
            }
        }

        let mut obj = OwnershipObject {
            object_id,
            owner,
            ownership_mode: mode,
            rights: vec![OwnershipRight::View, OwnershipRight::Use,
                         OwnershipRight::Transfer, OwnershipRight::Admin],
            metadata,
            behavior,
            linked_objects: vec![],
            version: 1,
            created_at_epoch: current_epoch,
            updated_at_epoch: current_epoch,
            state_hash: [0u8; 32],
        };
        obj.state_hash = obj.compute_state_hash();
        Ok(obj)
    }

    /// Compute a deterministic hash of the object's state.
    pub fn compute_state_hash(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_OWNERSHIP_V1");
        h.update(&self.object_id);
        h.update(&self.owner);
        h.update(&self.version.to_be_bytes());
        h.update(&self.created_at_epoch.to_be_bytes());
        // Include metadata keys/values in sorted order
        for (k, v) in &self.metadata {
            h.update(k.as_bytes());
            h.update(v.as_bytes());
        }
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Check if the object has expired.
    pub fn is_expired(&self, current_epoch: u64) -> bool {
        self.behavior.expires_epoch.map_or(false, |e| current_epoch >= e)
    }

    /// Check if a transfer from `from` to `to` is allowed at `current_epoch`.
    pub fn can_transfer(&self, from: &[u8; 32], to: &[u8; 32], current_epoch: u64) -> Result<(), String> {
        // Soulbound check
        if self.behavior.soulbound {
            return Err("object is soulbound and cannot be transferred".into());
        }

        // Expiry check
        if self.is_expired(current_epoch) {
            return Err("object has expired".into());
        }

        // Ownership check
        if from != &self.owner {
            return Err("caller is not the owner".into());
        }

        // Zero recipient check
        if to == &[0u8; 32] {
            return Err("cannot transfer to zero address".into());
        }

        // Self-transfer check
        if from == to {
            return Err("cannot transfer to self".into());
        }

        // Mode-specific checks
        match &self.ownership_mode {
            OwnershipMode::Direct => Ok(()),

            OwnershipMode::TimeLocked { unlock_epoch } => {
                if current_epoch < *unlock_epoch {
                    Err(format!("time-locked until epoch {} (current: {})", unlock_epoch, current_epoch))
                } else {
                    Ok(())
                }
            }

            OwnershipMode::Conditional { condition } => {
                match condition {
                    TransferCondition::MinEpoch(min) => {
                        if current_epoch < *min {
                            Err(format!("condition: min epoch {} not reached (current: {})", min, current_epoch))
                        } else {
                            Ok(())
                        }
                    }
                    TransferCondition::AllowedRecipients(list) => {
                        if list.contains(to) {
                            Ok(())
                        } else {
                            Err("condition: recipient not in allowlist".into())
                        }
                    }
                    TransferCondition::RequiresLinkedOwnership(linked_id) => {
                        // This check requires external state — caller must verify
                        // For now, check that the link exists
                        if self.linked_objects.iter().any(|l| &l.target_id == linked_id) {
                            Ok(())
                        } else {
                            Err("condition: required linked object not found".into())
                        }
                    }
                    TransferCondition::AlwaysTrue => Ok(()),
                }
            }

            OwnershipMode::Fractional { .. } => {
                Err("fractional ownership cannot be transferred directly; use transfer_shares()".into())
            }

            OwnershipMode::Delegated { .. } => {
                // Owner can transfer even with active delegation (delegate loses rights)
                Ok(())
            }
        }
    }

    /// Execute a direct transfer. Validates all rules and updates state.
    pub fn transfer(&mut self, from: &[u8; 32], to: [u8; 32], current_epoch: u64) -> Result<(), String> {
        self.can_transfer(from, &to, current_epoch)?;
        self.owner = to;
        self.version += 1;
        self.updated_at_epoch = current_epoch;
        // Clear delegation on transfer
        if let OwnershipMode::Delegated { .. } = self.ownership_mode {
            self.ownership_mode = OwnershipMode::Direct;
        }
        self.state_hash = self.compute_state_hash();
        Ok(())
    }

    /// Transfer fractional shares from one holder to another.
    pub fn transfer_shares(
        &mut self,
        from: &[u8; 32],
        to: [u8; 32],
        amount_bps: u32,
        current_epoch: u64,
    ) -> Result<(), String> {
        if to == [0u8; 32] { return Err("cannot transfer to zero address".into()); }
        if from == &to { return Err("cannot transfer to self".into()); }
        if amount_bps == 0 { return Err("amount must be > 0".into()); }

        let shares = match &mut self.ownership_mode {
            OwnershipMode::Fractional { shares } => shares,
            _ => return Err("not fractional ownership".into()),
        };

        let from_balance = shares.get(from).copied().unwrap_or(0);
        if from_balance < amount_bps {
            return Err(format!("insufficient shares: have {}, need {}", from_balance, amount_bps));
        }

        // Deduct from sender
        let new_from = from_balance - amount_bps;
        if new_from == 0 {
            shares.remove(from);
        } else {
            shares.insert(*from, new_from);
        }

        // Credit to recipient
        let to_balance = shares.get(&to).copied().unwrap_or(0);
        shares.insert(to, to_balance + amount_bps);

        // Update primary owner to largest shareholder
        if let Some((&largest, _)) = shares.iter().max_by_key(|(_, &v)| v) {
            self.owner = largest;
        }

        self.version += 1;
        self.updated_at_epoch = current_epoch;
        self.state_hash = self.compute_state_hash();
        Ok(())
    }

    /// Delegate usage rights to another address.
    pub fn delegate(
        &mut self,
        owner: &[u8; 32],
        delegate: [u8; 32],
        expires_epoch: u64,
        current_epoch: u64,
    ) -> Result<(), String> {
        if owner != &self.owner { return Err("only owner can delegate".into()); }
        if delegate == [0u8; 32] { return Err("delegate must be non-zero".into()); }
        if expires_epoch <= current_epoch {
            return Err(format!("expires_epoch {} must be > current {}", expires_epoch, current_epoch));
        }

        self.ownership_mode = OwnershipMode::Delegated { delegate, expires_epoch };
        self.version += 1;
        self.updated_at_epoch = current_epoch;
        self.state_hash = self.compute_state_hash();
        Ok(())
    }

    /// Revoke delegation (owner only).
    pub fn revoke_delegation(&mut self, owner: &[u8; 32], current_epoch: u64) -> Result<(), String> {
        if owner != &self.owner { return Err("only owner can revoke delegation".into()); }
        match &self.ownership_mode {
            OwnershipMode::Delegated { .. } => {
                self.ownership_mode = OwnershipMode::Direct;
                self.version += 1;
                self.updated_at_epoch = current_epoch;
                self.state_hash = self.compute_state_hash();
                Ok(())
            }
            _ => Err("no active delegation to revoke".into()),
        }
    }

    /// Check if `addr` has usage rights (either owner or active delegate).
    pub fn has_usage_rights(&self, addr: &[u8; 32], current_epoch: u64) -> bool {
        if addr == &self.owner { return true; }
        match &self.ownership_mode {
            OwnershipMode::Delegated { delegate, expires_epoch } => {
                addr == delegate && current_epoch < *expires_epoch
            }
            OwnershipMode::Fractional { shares } => {
                shares.contains_key(addr)
            }
            _ => false,
        }
    }

    /// Add a link to another object.
    pub fn add_link(&mut self, link: ObjectLink, current_epoch: u64) -> Result<(), String> {
        if link.target_id == self.object_id {
            return Err("cannot link to self".into());
        }
        if self.linked_objects.iter().any(|l| l.target_id == link.target_id) {
            return Err("duplicate link".into());
        }
        self.linked_objects.push(link);
        self.version += 1;
        self.updated_at_epoch = current_epoch;
        self.state_hash = self.compute_state_hash();
        Ok(())
    }

    /// Remove a link.
    pub fn remove_link(&mut self, target_id: &[u8; 32], current_epoch: u64) -> Result<(), String> {
        let before = self.linked_objects.len();
        self.linked_objects.retain(|l| &l.target_id != target_id);
        if self.linked_objects.len() == before {
            return Err("link not found".into());
        }
        self.version += 1;
        self.updated_at_epoch = current_epoch;
        self.state_hash = self.compute_state_hash();
        Ok(())
    }
}

/// Registry of ownership objects.
#[derive(Debug, Clone, Default)]
pub struct OwnershipRegistry {
    objects: BTreeMap<[u8; 32], OwnershipObject>,
    by_owner: BTreeMap<[u8; 32], Vec<[u8; 32]>>,
}

impl OwnershipRegistry {
    pub fn new() -> Self { OwnershipRegistry::default() }

    pub fn register(&mut self, obj: OwnershipObject) -> Result<(), String> {
        if self.objects.contains_key(&obj.object_id) {
            return Err("duplicate object_id".into());
        }
        let id = obj.object_id;
        let owner = obj.owner;
        self.objects.insert(id, obj);
        self.by_owner.entry(owner).or_default().push(id);
        Ok(())
    }

    pub fn get(&self, id: &[u8; 32]) -> Option<&OwnershipObject> { self.objects.get(id) }
    pub fn get_mut(&mut self, id: &[u8; 32]) -> Option<&mut OwnershipObject> { self.objects.get_mut(id) }
    pub fn count(&self) -> usize { self.objects.len() }

    pub fn by_owner(&self, owner: &[u8; 32]) -> Vec<&OwnershipObject> {
        self.by_owner.get(owner)
            .map(|ids| ids.iter().filter_map(|id| self.objects.get(id)).collect())
            .unwrap_or_default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn oid(v: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = v; b }
    fn alice() -> [u8; 32] { oid(1) }
    fn bob() -> [u8; 32] { oid(2) }
    fn carol() -> [u8; 32] { oid(3) }

    fn make_direct(id: u8, owner: [u8; 32], epoch: u64) -> OwnershipObject {
        OwnershipObject::create(oid(id), owner, OwnershipMode::Direct,
            BTreeMap::new(), BehaviorFlags::default(), epoch).unwrap()
    }

    // ── Test 1: direct transfer ──────────────────────────────

    #[test]
    fn test_direct_transfer() {
        let mut obj = make_direct(10, alice(), 1);
        assert_eq!(obj.owner, alice());
        obj.transfer(&alice(), bob(), 5).unwrap();
        assert_eq!(obj.owner, bob());
        assert_eq!(obj.version, 2);
    }

    // ── Test 2: rejected invalid transfer (not owner) ────────

    #[test]
    fn test_rejected_not_owner() {
        let mut obj = make_direct(10, alice(), 1);
        let err = obj.transfer(&bob(), carol(), 5).unwrap_err();
        assert!(err.contains("not the owner"));
    }

    // ── Test 3: time-locked transfer rejection ───────────────

    #[test]
    fn test_time_locked_rejection() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::TimeLocked { unlock_epoch: 100 },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        let err = obj.transfer(&alice(), bob(), 50).unwrap_err();
        assert!(err.contains("time-locked"));

        // After unlock epoch, transfer succeeds
        obj.transfer(&alice(), bob(), 100).unwrap();
        assert_eq!(obj.owner, bob());
    }

    // ── Test 4: conditional transfer (min epoch) ─────────────

    #[test]
    fn test_conditional_min_epoch() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(),
            OwnershipMode::Conditional { condition: TransferCondition::MinEpoch(50) },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        let err = obj.transfer(&alice(), bob(), 30).unwrap_err();
        assert!(err.contains("min epoch"));

        obj.transfer(&alice(), bob(), 50).unwrap();
        assert_eq!(obj.owner, bob());
    }

    // ── Test 5: conditional transfer (allowed recipients) ────

    #[test]
    fn test_conditional_allowed_recipients() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(),
            OwnershipMode::Conditional { condition: TransferCondition::AllowedRecipients(vec![bob()]) },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        let err = obj.transfer(&alice(), carol(), 5).unwrap_err();
        assert!(err.contains("not in allowlist"));

        obj.transfer(&alice(), bob(), 5).unwrap();
        assert_eq!(obj.owner, bob());
    }

    // ── Test 6: delegated rights ─────────────────────────────

    #[test]
    fn test_delegated_usage_rights() {
        let mut obj = make_direct(10, alice(), 1);

        // Delegate to bob until epoch 100
        obj.delegate(&alice(), bob(), 100, 5).unwrap();
        assert!(obj.has_usage_rights(&alice(), 10)); // owner always has rights
        assert!(obj.has_usage_rights(&bob(), 10));   // delegate has rights
        assert!(!obj.has_usage_rights(&carol(), 10)); // carol has no rights
        assert!(!obj.has_usage_rights(&bob(), 100));  // expired

        // Owner can still transfer
        obj.transfer(&alice(), carol(), 50).unwrap();
        assert_eq!(obj.owner, carol());
        // Delegation cleared on transfer
        assert!(!obj.has_usage_rights(&bob(), 50));
    }

    // ── Test 7: fractional ownership ─────────────────────────

    #[test]
    fn test_fractional_ownership() {
        let mut shares = BTreeMap::new();
        shares.insert(alice(), 6000); // 60%
        shares.insert(bob(), 4000);   // 40%

        let mut obj = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::Fractional { shares },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        // Both have usage rights
        assert!(obj.has_usage_rights(&alice(), 5));
        assert!(obj.has_usage_rights(&bob(), 5));
        assert!(!obj.has_usage_rights(&carol(), 5));

        // Direct transfer fails for fractional
        let err = obj.transfer(&alice(), carol(), 5).unwrap_err();
        assert!(err.contains("transfer_shares"));

        // Share transfer
        obj.transfer_shares(&alice(), carol(), 2000, 5).unwrap(); // alice: 4000, bob: 4000, carol: 2000

        // Largest shareholder updates
        // alice and bob both at 4000 — BTreeMap max_by_key picks first in iteration order
        assert!(obj.has_usage_rights(&carol(), 5));
    }

    // ── Test 8: fractional shares must sum to 10000 ──────────

    #[test]
    fn test_fractional_invalid_sum() {
        let mut shares = BTreeMap::new();
        shares.insert(alice(), 5000);
        shares.insert(bob(), 3000);

        let err = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::Fractional { shares },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap_err();
        assert!(err.contains("sum to 10000"));
    }

    // ── Test 9: soulbound cannot transfer ────────────────────

    #[test]
    fn test_soulbound() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::Direct,
            BTreeMap::new(),
            BehaviorFlags { soulbound: true, ..Default::default() }, 1,
        ).unwrap();

        let err = obj.transfer(&alice(), bob(), 5).unwrap_err();
        assert!(err.contains("soulbound"));
    }

    // ── Test 10: expired object cannot transfer ──────────────

    #[test]
    fn test_expired_object() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::Direct,
            BTreeMap::new(),
            BehaviorFlags { expires_epoch: Some(50), ..Default::default() }, 1,
        ).unwrap();

        let err = obj.transfer(&alice(), bob(), 60).unwrap_err();
        assert!(err.contains("expired"));
    }

    // ── Test 11: linked objects ──────────────────────────────

    #[test]
    fn test_linked_objects() {
        let mut obj = make_direct(10, alice(), 1);

        obj.add_link(ObjectLink { target_id: oid(20), link_type: LinkType::ParentChild }, 5).unwrap();
        assert_eq!(obj.linked_objects.len(), 1);

        // Duplicate link rejected
        let err = obj.add_link(ObjectLink { target_id: oid(20), link_type: LinkType::Bundle }, 5).unwrap_err();
        assert!(err.contains("duplicate"));

        // Self-link rejected
        let err = obj.add_link(ObjectLink { target_id: oid(10), link_type: LinkType::Reference }, 5).unwrap_err();
        assert!(err.contains("self"));

        // Remove link
        obj.remove_link(&oid(20), 6).unwrap();
        assert_eq!(obj.linked_objects.len(), 0);
    }

    // ── Test 12: conditional with linked ownership ───────────

    #[test]
    fn test_conditional_linked_ownership() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(),
            OwnershipMode::Conditional { condition: TransferCondition::RequiresLinkedOwnership(oid(20)) },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        // Transfer fails without link
        let err = obj.transfer(&alice(), bob(), 5).unwrap_err();
        assert!(err.contains("linked object not found"));

        // Add the link
        obj.add_link(ObjectLink { target_id: oid(20), link_type: LinkType::Dependency }, 3).unwrap();

        // Now transfer succeeds
        obj.transfer(&alice(), bob(), 5).unwrap();
        assert_eq!(obj.owner, bob());
    }

    // ── Test 13: version increments ──────────────────────────

    #[test]
    fn test_version_tracking() {
        let mut obj = make_direct(10, alice(), 1);
        assert_eq!(obj.version, 1);

        obj.transfer(&alice(), bob(), 5).unwrap();
        assert_eq!(obj.version, 2);

        obj.add_link(ObjectLink { target_id: oid(20), link_type: LinkType::Reference }, 6).unwrap();
        assert_eq!(obj.version, 3);
    }

    // ── Test 14: state hash changes on mutation ──────────────

    #[test]
    fn test_state_hash_deterministic() {
        let obj1 = make_direct(10, alice(), 1);
        let obj2 = make_direct(10, alice(), 1);
        assert_eq!(obj1.state_hash, obj2.state_hash);

        let mut obj3 = make_direct(10, alice(), 1);
        let hash_before = obj3.state_hash;
        obj3.transfer(&alice(), bob(), 5).unwrap();
        assert_ne!(hash_before, obj3.state_hash);
    }

    // ── Test 15: revoke delegation ───────────────────────────

    #[test]
    fn test_revoke_delegation() {
        let mut obj = make_direct(10, alice(), 1);
        obj.delegate(&alice(), bob(), 100, 5).unwrap();
        assert!(obj.has_usage_rights(&bob(), 10));

        obj.revoke_delegation(&alice(), 15).unwrap();
        assert!(!obj.has_usage_rights(&bob(), 15));
        assert_eq!(obj.ownership_mode, OwnershipMode::Direct);
    }

    // ── Test 16: transfer to zero rejected ───────────────────

    #[test]
    fn test_transfer_to_zero() {
        let mut obj = make_direct(10, alice(), 1);
        let err = obj.transfer(&alice(), [0u8; 32], 5).unwrap_err();
        assert!(err.contains("zero address"));
    }

    // ── Test 17: fractional insufficient shares ──────────────

    #[test]
    fn test_fractional_insufficient_shares() {
        let mut shares = BTreeMap::new();
        shares.insert(alice(), 6000);
        shares.insert(bob(), 4000);

        let mut obj = OwnershipObject::create(
            oid(10), alice(), OwnershipMode::Fractional { shares },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        let err = obj.transfer_shares(&alice(), carol(), 8000, 5).unwrap_err();
        assert!(err.contains("insufficient"));
    }

    // ── Test 18: registry operations ─────────────────────────

    #[test]
    fn test_registry() {
        let mut reg = OwnershipRegistry::new();
        let obj1 = make_direct(10, alice(), 1);
        let obj2 = make_direct(20, alice(), 1);
        let obj3 = make_direct(30, bob(), 1);

        reg.register(obj1).unwrap();
        reg.register(obj2).unwrap();
        reg.register(obj3).unwrap();

        assert_eq!(reg.count(), 3);
        assert_eq!(reg.by_owner(&alice()).len(), 2);
        assert_eq!(reg.by_owner(&bob()).len(), 1);

        // Duplicate rejected
        let dup = make_direct(10, carol(), 1);
        assert!(reg.register(dup).is_err());
    }

    // ── Test 19: non-owner cannot delegate ───────────────────

    #[test]
    fn test_non_owner_cannot_delegate() {
        let mut obj = make_direct(10, alice(), 1);
        let err = obj.delegate(&bob(), carol(), 100, 5).unwrap_err();
        assert!(err.contains("only owner"));
    }

    // ── Test 20: always-true condition ────────────────────────

    #[test]
    fn test_always_true_condition() {
        let mut obj = OwnershipObject::create(
            oid(10), alice(),
            OwnershipMode::Conditional { condition: TransferCondition::AlwaysTrue },
            BTreeMap::new(), BehaviorFlags::default(), 1,
        ).unwrap();

        obj.transfer(&alice(), bob(), 5).unwrap();
        assert_eq!(obj.owner, bob());
    }
}
