//! Pre-validation cache for contract constraint results.
//!
//! The Go chain's wazero runtime validates constraints. Results are cached
//! here so the Rust settlement engine can check them without a cross-language
//! call during the hot path.
//!
//! Flow:
//! 1. Solver submits plan with `proposed_state` in metadata
//! 2. PlanValidator invokes the Go-side wazero validator via bridge
//! 3. Result is cached by (schema_id, state_hash, proposed_hash)
//! 4. Settlement engine checks cache before applying state transition
//! 5. Cache entries expire after N epochs

use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// Cache key: deterministic hash of (schema_id, current_state_hash, proposed_state_hash, method)
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct ValidationCacheKey([u8; 32]);

impl ValidationCacheKey {
    pub fn compute(
        schema_id: &[u8; 32],
        current_state_hash: &[u8; 32],
        proposed_state_hash: &[u8; 32],
        method: &str,
    ) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(b"OMNIPHI_VALIDATION_CACHE_V1");
        hasher.update(schema_id);
        hasher.update(current_state_hash);
        hasher.update(proposed_state_hash);
        hasher.update(method.as_bytes());
        let hash = hasher.finalize();
        let mut key = [0u8; 32];
        key.copy_from_slice(&hash);
        ValidationCacheKey(key)
    }
}

/// Cached validation result.
#[derive(Debug, Clone)]
pub struct CachedValidation {
    pub valid: bool,
    pub reason: String,
    pub cached_at_epoch: u64,
    pub gas_used: u64,
}

/// Validation cache with TTL-based expiration.
pub struct ValidationCache {
    entries: BTreeMap<ValidationCacheKey, CachedValidation>,
    /// Cache entries older than this many epochs are evicted on lookup.
    ttl_epochs: u64,
    /// Maximum cache size. LRU eviction when exceeded.
    max_entries: usize,
}

impl ValidationCache {
    pub fn new(ttl_epochs: u64, max_entries: usize) -> Self {
        ValidationCache {
            entries: BTreeMap::new(),
            ttl_epochs,
            max_entries,
        }
    }

    /// Insert a validation result into the cache.
    pub fn insert(&mut self, key: ValidationCacheKey, result: CachedValidation) {
        // Evict oldest if at capacity
        if self.entries.len() >= self.max_entries {
            if let Some(oldest_key) = self.entries.keys().next().cloned() {
                self.entries.remove(&oldest_key);
            }
        }
        self.entries.insert(key, result);
    }

    /// Look up a cached validation result. Returns None if not found or expired.
    pub fn get(&self, key: &ValidationCacheKey, current_epoch: u64) -> Option<&CachedValidation> {
        let entry = self.entries.get(key)?;
        if current_epoch > entry.cached_at_epoch + self.ttl_epochs {
            return None; // expired
        }
        Some(entry)
    }

    /// Check if a specific transition has been pre-validated as accepted.
    pub fn is_pre_validated(
        &self,
        schema_id: &[u8; 32],
        current_state_hash: &[u8; 32],
        proposed_state_hash: &[u8; 32],
        method: &str,
        current_epoch: u64,
    ) -> Option<bool> {
        let key = ValidationCacheKey::compute(schema_id, current_state_hash, proposed_state_hash, method);
        self.get(&key, current_epoch).map(|v| v.valid)
    }

    /// Prune all expired entries.
    pub fn prune(&mut self, current_epoch: u64) {
        self.entries.retain(|_, v| current_epoch <= v.cached_at_epoch + self.ttl_epochs);
    }

    /// Number of cached entries.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// Compute the SHA256 hash of a state blob.
pub fn state_hash(state: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(state);
    let h = hasher.finalize();
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&h);
    hash
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cache_insert_and_lookup() {
        let mut cache = ValidationCache::new(10, 100);
        let key = ValidationCacheKey::compute(&[1u8; 32], &[2u8; 32], &[3u8; 32], "fund");

        cache.insert(key, CachedValidation {
            valid: true,
            reason: String::new(),
            cached_at_epoch: 5,
            gas_used: 100,
        });

        assert!(cache.get(&key, 5).is_some());
        assert!(cache.get(&key, 14).is_some()); // within TTL
        assert!(cache.get(&key, 16).is_none()); // expired
    }

    #[test]
    fn test_is_pre_validated() {
        let mut cache = ValidationCache::new(10, 100);
        let schema = [1u8; 32];
        let current = [2u8; 32];
        let proposed = [3u8; 32];
        let key = ValidationCacheKey::compute(&schema, &current, &proposed, "release");

        cache.insert(key, CachedValidation {
            valid: true,
            reason: String::new(),
            cached_at_epoch: 5,
            gas_used: 50,
        });

        assert_eq!(cache.is_pre_validated(&schema, &current, &proposed, "release", 5), Some(true));
        assert_eq!(cache.is_pre_validated(&schema, &current, &proposed, "fund", 5), None); // different method
    }

    #[test]
    fn test_prune() {
        let mut cache = ValidationCache::new(5, 100);
        let key1 = ValidationCacheKey::compute(&[1u8; 32], &[2u8; 32], &[3u8; 32], "a");
        let key2 = ValidationCacheKey::compute(&[4u8; 32], &[5u8; 32], &[6u8; 32], "b");

        cache.insert(key1, CachedValidation { valid: true, reason: String::new(), cached_at_epoch: 1, gas_used: 10 });
        cache.insert(key2, CachedValidation { valid: true, reason: String::new(), cached_at_epoch: 8, gas_used: 10 });

        assert_eq!(cache.len(), 2);
        cache.prune(10);
        assert_eq!(cache.len(), 1); // key1 expired (epoch 1 + ttl 5 = 6 < 10)
    }

    #[test]
    fn test_max_entries_eviction() {
        let mut cache = ValidationCache::new(100, 2);

        for i in 0u8..5 {
            let key = ValidationCacheKey::compute(&[i; 32], &[0u8; 32], &[0u8; 32], "x");
            cache.insert(key, CachedValidation { valid: true, reason: String::new(), cached_at_epoch: i as u64, gas_used: 10 });
        }

        assert_eq!(cache.len(), 2); // max entries enforced
    }
}
