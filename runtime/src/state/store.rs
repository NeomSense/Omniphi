use crate::objects::base::{BoxedObject, Object, ObjectId, ObjectType};
use crate::objects::types::{BalanceObject, LiquidityPoolObject, VaultObject};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// An in-memory, BTreeMap-backed object store.
///
/// We maintain a canonical type-erased `objects` map for deterministic state root
/// computation, plus separate typed maps for the three concrete types that the
/// resolution and settlement layers need direct mutable references to.
///
/// Invariant: every id that appears in `balances`, `pools`, or `vaults` also
/// appears in `objects`, and vice-versa for those types.
pub struct ObjectStore {
    /// Canonical store — all objects (type-erased, sorted by ObjectId).
    objects: BTreeMap<ObjectId, BoxedObject>,
    /// Typed overlay for BalanceObjects.
    balances: BTreeMap<ObjectId, BalanceObject>,
    /// Typed overlay for LiquidityPoolObjects.
    pools: BTreeMap<ObjectId, LiquidityPoolObject>,
    /// Typed overlay for VaultObjects.
    vaults: BTreeMap<ObjectId, VaultObject>,
}

impl ObjectStore {
    pub fn new() -> Self {
        ObjectStore {
            objects: BTreeMap::new(),
            balances: BTreeMap::new(),
            pools: BTreeMap::new(),
            vaults: BTreeMap::new(),
        }
    }

    // ─────────────────────────────────────────────────
    // Core insert / remove
    // ─────────────────────────────────────────────────

    /// Inserts or replaces an object in both the canonical and typed stores.
    pub fn insert(&mut self, obj: BoxedObject) {
        let id = obj.meta().id;
        match obj.object_type() {
            ObjectType::Balance => {
                let b: BalanceObject = bincode::deserialize(&obj.encode())
                    .expect("BalanceObject bincode encode/decode infallible");
                self.balances.insert(id, b);
            }
            ObjectType::LiquidityPool => {
                let p: LiquidityPoolObject = bincode::deserialize(&obj.encode())
                    .expect("LiquidityPoolObject bincode encode/decode infallible");
                self.pools.insert(id, p);
            }
            ObjectType::Vault => {
                let v: VaultObject = bincode::deserialize(&obj.encode())
                    .expect("VaultObject bincode encode/decode infallible");
                self.vaults.insert(id, v);
            }
            _ => {}
        }
        self.objects.insert(id, obj);
    }

    /// Removes and returns the boxed object (both stores).
    pub fn remove(&mut self, id: &ObjectId) -> Option<BoxedObject> {
        self.balances.remove(id);
        self.pools.remove(id);
        self.vaults.remove(id);
        self.objects.remove(id)
    }

    // ─────────────────────────────────────────────────
    // Type-erased access
    // ─────────────────────────────────────────────────

    /// Immutable type-erased reference.
    pub fn get(&self, id: &ObjectId) -> Option<&dyn Object> {
        self.objects.get(id).map(|b| b.as_ref())
    }

    /// Mutable type-erased boxed reference.
    pub fn get_mut(&mut self, id: &ObjectId) -> Option<&mut BoxedObject> {
        self.objects.get_mut(id)
    }

    /// All objects of the given type.
    pub fn find_by_type(&self, t: ObjectType) -> Vec<&dyn Object> {
        self.objects
            .values()
            .filter(|o| o.object_type() == t)
            .map(|o| o.as_ref())
            .collect()
    }

    // ─────────────────────────────────────────────────
    // BalanceObject typed access
    // ─────────────────────────────────────────────────

    /// Finds a BalanceObject matching both owner and asset_id.
    pub fn find_balance(&self, owner: &[u8; 32], asset_id: &[u8; 32]) -> Option<&BalanceObject> {
        self.balances
            .values()
            .find(|b| &b.owner == owner && &b.asset_id == asset_id)
    }

    /// Finds a mutable BalanceObject matching both owner and asset_id.
    pub fn find_balance_mut(
        &mut self,
        owner: &[u8; 32],
        asset_id: &[u8; 32],
    ) -> Option<&mut BalanceObject> {
        self.balances
            .values_mut()
            .find(|b| &b.owner == owner && &b.asset_id == asset_id)
    }

    /// Gets a BalanceObject by its ObjectId.
    pub fn get_balance_by_id(&self, id: &ObjectId) -> Option<&BalanceObject> {
        self.balances.get(id)
    }

    /// Gets a mutable BalanceObject by its ObjectId.
    pub fn get_balance_by_id_mut(&mut self, id: &ObjectId) -> Option<&mut BalanceObject> {
        self.balances.get_mut(id)
    }

    /// Gets a BalanceObject by id for version tracking (same as get_balance_by_id).
    pub fn find_balance_by_id_mut(&mut self, id: &ObjectId) -> Option<&mut BalanceObject> {
        self.balances.get_mut(id)
    }

    // ─────────────────────────────────────────────────
    // LiquidityPoolObject typed access
    // ─────────────────────────────────────────────────

    /// Finds a LiquidityPoolObject (order-insensitive asset pair).
    pub fn find_pool(
        &self,
        asset_a: &[u8; 32],
        asset_b: &[u8; 32],
    ) -> Option<&LiquidityPoolObject> {
        self.pools.values().find(|p| {
            (&p.asset_a == asset_a && &p.asset_b == asset_b)
                || (&p.asset_a == asset_b && &p.asset_b == asset_a)
        })
    }

    /// Finds a mutable LiquidityPoolObject (order-insensitive).
    pub fn find_pool_mut(
        &mut self,
        asset_a: &[u8; 32],
        asset_b: &[u8; 32],
    ) -> Option<&mut LiquidityPoolObject> {
        self.pools.values_mut().find(|p| {
            (&p.asset_a == asset_a && &p.asset_b == asset_b)
                || (&p.asset_a == asset_b && &p.asset_b == asset_a)
        })
    }

    /// Gets a LiquidityPoolObject by its ObjectId.
    pub fn get_pool_by_id(&self, id: &ObjectId) -> Option<&LiquidityPoolObject> {
        self.pools.get(id)
    }

    /// Gets a mutable LiquidityPoolObject by its ObjectId.
    pub fn get_pool_by_id_mut(&mut self, id: &ObjectId) -> Option<&mut LiquidityPoolObject> {
        self.pools.get_mut(id)
    }

    /// Gets a mutable LiquidityPoolObject by id for version tracking.
    pub fn find_pool_by_id_mut(&mut self, id: &ObjectId) -> Option<&mut LiquidityPoolObject> {
        self.pools.get_mut(id)
    }

    // ─────────────────────────────────────────────────
    // VaultObject typed access
    // ─────────────────────────────────────────────────

    /// Gets a VaultObject by id.
    pub fn get_vault(&self, id: &ObjectId) -> Option<&VaultObject> {
        self.vaults.get(id)
    }

    /// Gets a mutable VaultObject by id.
    pub fn get_vault_mut(&mut self, id: &ObjectId) -> Option<&mut VaultObject> {
        self.vaults.get_mut(id)
    }

    // ─────────────────────────────────────────────────
    // Sync: flush typed overlays back to canonical store
    // ─────────────────────────────────────────────────

    /// Re-inserts mutated typed objects back into the canonical `objects` map.
    /// Must be called by SettlementEngine after mutations to typed overlays.
    pub fn sync_typed_to_canonical(&mut self) {
        // We collect ids first to avoid borrow conflicts
        let balance_ids: Vec<ObjectId> = self.balances.keys().copied().collect();
        for id in balance_ids {
            if let Some(b) = self.balances.get(&id) {
                if self.objects.contains_key(&id) {
                    let new_box: BoxedObject = Box::new(b.clone());
                    self.objects.insert(id, new_box);
                }
            }
        }
        let pool_ids: Vec<ObjectId> = self.pools.keys().copied().collect();
        for id in pool_ids {
            if let Some(p) = self.pools.get(&id) {
                if self.objects.contains_key(&id) {
                    let new_box: BoxedObject = Box::new(p.clone());
                    self.objects.insert(id, new_box);
                }
            }
        }
        let vault_ids: Vec<ObjectId> = self.vaults.keys().copied().collect();
        for id in vault_ids {
            if let Some(v) = self.vaults.get(&id) {
                if self.objects.contains_key(&id) {
                    let new_box: BoxedObject = Box::new(v.clone());
                    self.objects.insert(id, new_box);
                }
            }
        }
    }

    // ─────────────────────────────────────────────────
    // State root
    // ─────────────────────────────────────────────────

    /// Deterministic SHA256 state root over all objects sorted by ObjectId.
    ///
    /// Uses each object's `encode()` method (bincode-based) for canonical binary
    /// encoding. The BTreeMap ensures lexicographic ordering by ObjectId.
    ///
    /// Callers must call `sync_typed_to_canonical()` before this if mutations
    /// were made through typed overlays.
    pub fn state_root(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        for (id, obj) in &self.objects {
            // Feed the object id bytes (32 bytes, fixed-size — no length prefix needed)
            hasher.update(id.as_bytes());
            // Encode using the canonical bincode path
            let encoded = obj.encode();
            // Feed length-prefixed payload so variable-length encodings are unambiguous
            let len = (encoded.len() as u64).to_le_bytes();
            hasher.update(len);
            hasher.update(&encoded);
        }
        hasher.finalize().into()
    }

    // ─────────────────────────────────────────────────
    // Utility
    // ─────────────────────────────────────────────────

    pub fn len(&self) -> usize {
        self.objects.len()
    }

    pub fn is_empty(&self) -> bool {
        self.objects.is_empty()
    }
}

impl Default for ObjectStore {
    fn default() -> Self {
        Self::new()
    }
}
