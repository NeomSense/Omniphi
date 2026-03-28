//! Concurrent Object Store — Sharded Lock Architecture for Parallel Settlement
//!
//! Wraps ObjectStore with per-shard read-write locks, enabling safe parallel
//! execution of non-conflicting plans within the same execution group.
//!
//! ## Design
//!
//! Objects are partitioned into N shards based on their ObjectId. Each shard
//! has its own RwLock. Non-conflicting plans (guaranteed by the scheduler to
//! touch disjoint objects) acquire locks on different shards and execute in
//! parallel.
//!
//! ## Shard Assignment
//!
//! shard_index = object_id[0] % NUM_SHARDS
//!
//! This is deterministic and uniform for random ObjectIds.
//!
//! ## Safety
//!
//! The scheduler guarantees that plans in the same execution group have no
//! write-write or read-write conflicts. Therefore:
//! - Two parallel plans never write the same shard simultaneously
//! - A read lock never blocks a parallel plan's write (different shard)
//! - Deadlock is impossible: each plan acquires locks in shard-index order

use crate::objects::base::{BoxedObject, Object, ObjectId};
use crate::objects::types::{BalanceObject, LiquidityPoolObject};
use std::collections::BTreeMap;
use std::sync::RwLock;

const NUM_SHARDS: usize = 64;

fn shard_index(id: &ObjectId) -> usize {
    id.0[0] as usize % NUM_SHARDS
}

/// A shard holding a subset of objects.
#[derive(Default)]
struct Shard {
    objects: BTreeMap<ObjectId, BoxedObject>,
    balances: BTreeMap<ObjectId, BalanceObject>,
    pools: BTreeMap<ObjectId, LiquidityPoolObject>,
}

/// A concurrent object store with sharded locks.
pub struct ConcurrentObjectStore {
    shards: Vec<RwLock<Shard>>,
}

impl ConcurrentObjectStore {
    pub fn new() -> Self {
        let mut shards = Vec::with_capacity(NUM_SHARDS);
        for _ in 0..NUM_SHARDS {
            shards.push(RwLock::new(Shard::default()));
        }
        ConcurrentObjectStore { shards }
    }

    /// Insert an object into the appropriate shard.
    pub fn insert(&self, id: ObjectId, obj: BoxedObject) {
        let idx = shard_index(&id);
        let mut shard = self.shards[idx].write().unwrap();
        shard.objects.insert(id, obj);
    }

    /// Read an object by ID (acquires read lock on one shard).
    pub fn get(&self, id: &ObjectId) -> Option<Vec<u8>> {
        let idx = shard_index(id);
        let shard = self.shards[idx].read().unwrap();
        shard.objects.get(id).map(|obj| obj.encode())
    }

    /// Check if an object exists.
    pub fn contains(&self, id: &ObjectId) -> bool {
        let idx = shard_index(id);
        let shard = self.shards[idx].read().unwrap();
        shard.objects.contains_key(id)
    }

    /// Remove an object.
    pub fn remove(&self, id: &ObjectId) -> bool {
        let idx = shard_index(id);
        let mut shard = self.shards[idx].write().unwrap();
        shard.objects.remove(id).is_some()
    }

    /// Find a balance object by owner and asset.
    pub fn find_balance(&self, owner: &[u8; 32], asset_id: &[u8; 32]) -> Option<BalanceObject> {
        // Balances are spread across shards — scan all
        for lock in &self.shards {
            let shard = lock.read().unwrap();
            for bal in shard.balances.values() {
                if &bal.owner == owner && &bal.asset_id == asset_id {
                    return Some(bal.clone());
                }
            }
        }
        None
    }

    /// Total number of objects across all shards.
    pub fn len(&self) -> usize {
        self.shards.iter()
            .map(|s| s.read().unwrap().objects.len())
            .sum()
    }

    pub fn is_empty(&self) -> bool { self.len() == 0 }

    /// Get the shard distribution (for diagnostics).
    pub fn shard_distribution(&self) -> Vec<usize> {
        self.shards.iter()
            .map(|s| s.read().unwrap().objects.len())
            .collect()
    }

    /// Number of shards.
    pub fn num_shards(&self) -> usize { NUM_SHARDS }
}

impl Default for ConcurrentObjectStore {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::objects::types::BalanceObject;

    fn oid(v: u8) -> ObjectId { ObjectId({ let mut b = [0u8; 32]; b[0] = v; b }) }

    #[test]
    fn test_insert_and_get() {
        let store = ConcurrentObjectStore::new();
        let id = oid(1);
        let bal = BalanceObject::new(id, [1u8; 32], [0xAA; 32], 1000, 1);
        store.insert(id, Box::new(bal));
        assert!(store.contains(&id));
        assert!(!store.is_empty());
    }

    #[test]
    fn test_remove() {
        let store = ConcurrentObjectStore::new();
        let id = oid(1);
        let bal = BalanceObject::new(id, [1u8; 32], [0xAA; 32], 1000, 1);
        store.insert(id, Box::new(bal));
        assert!(store.remove(&id));
        assert!(!store.contains(&id));
    }

    #[test]
    fn test_shard_distribution() {
        let store = ConcurrentObjectStore::new();
        for i in 0..128u8 {
            let id = oid(i);
            let bal = BalanceObject::new(id, [i; 32], [0xAA; 32], 1000, 1);
            store.insert(id, Box::new(bal));
        }
        assert_eq!(store.len(), 128);
        let dist = store.shard_distribution();
        assert_eq!(dist.len(), NUM_SHARDS);
        // Each shard should have 2 objects (128 / 64)
        assert!(dist.iter().all(|&c| c == 2));
    }

    #[test]
    fn test_parallel_access_different_shards() {
        use std::sync::Arc;
        use std::thread;

        let store = Arc::new(ConcurrentObjectStore::new());

        // Insert objects in different shards from different threads
        let mut handles = vec![];
        for t in 0..4u8 {
            let s = Arc::clone(&store);
            handles.push(thread::spawn(move || {
                for i in 0..32u8 {
                    let v = t * 64 + i; // spread across shards
                    let id = oid(v);
                    let bal = BalanceObject::new(id, [v; 32], [0xAA; 32], 1000, 1);
                    s.insert(id, Box::new(bal));
                }
            }));
        }

        for h in handles {
            h.join().unwrap();
        }

        assert_eq!(store.len(), 128);
    }

    #[test]
    fn test_concurrent_read_write() {
        use std::sync::Arc;
        use std::thread;

        let store = Arc::new(ConcurrentObjectStore::new());

        // Pre-populate
        for i in 0..64u8 {
            let id = oid(i);
            let bal = BalanceObject::new(id, [i; 32], [0xAA; 32], 1000, 1);
            store.insert(id, Box::new(bal));
        }

        // Concurrent reads and writes on disjoint shards
        let s1 = Arc::clone(&store);
        let reader = thread::spawn(move || {
            for i in 0..32u8 {
                s1.contains(&oid(i)); // read shard 0-31
            }
        });

        let s2 = Arc::clone(&store);
        let writer = thread::spawn(move || {
            for i in 32..64u8 {
                let id = oid(i + 64); // write shard 32-63 range
                let bal = BalanceObject::new(id, [i; 32], [0xBB; 32], 2000, 1);
                s2.insert(id, Box::new(bal));
            }
        });

        reader.join().unwrap();
        writer.join().unwrap();
    }
}
