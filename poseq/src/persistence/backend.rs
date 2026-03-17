use std::collections::BTreeMap;

/// Pluggable storage backend abstraction.
pub trait PersistenceBackend: Send {
    /// Get a value by key. Returns None if not found.
    fn get(&self, key: &[u8]) -> Option<Vec<u8>>;
    /// Insert or overwrite a key-value pair.
    fn put(&mut self, key: &[u8], value: Vec<u8>);
    /// Delete a key. No-op if key doesn't exist.
    fn delete(&mut self, key: &[u8]);
    /// Scan all key-value pairs with the given prefix, in sorted key order.
    fn prefix_scan(&self, prefix: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)>;
    /// Check if a key exists.
    fn contains(&self, key: &[u8]) -> bool {
        self.get(key).is_some()
    }
}

/// BTreeMap-backed in-memory implementation of PersistenceBackend.
pub struct InMemoryBackend {
    store: BTreeMap<Vec<u8>, Vec<u8>>,
}

impl InMemoryBackend {
    pub fn new() -> Self {
        InMemoryBackend { store: BTreeMap::new() }
    }

    /// Total number of entries.
    pub fn len(&self) -> usize {
        self.store.len()
    }

    pub fn is_empty(&self) -> bool {
        self.store.is_empty()
    }

    /// All keys in sorted order.
    pub fn keys(&self) -> Vec<Vec<u8>> {
        self.store.keys().cloned().collect()
    }
}

impl Default for InMemoryBackend {
    fn default() -> Self {
        Self::new()
    }
}

impl PersistenceBackend for InMemoryBackend {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>> {
        self.store.get(key).cloned()
    }

    fn put(&mut self, key: &[u8], value: Vec<u8>) {
        self.store.insert(key.to_vec(), value);
    }

    fn delete(&mut self, key: &[u8]) {
        self.store.remove(key);
    }

    fn prefix_scan(&self, prefix: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.store
            .range(prefix.to_vec()..)
            .take_while(|(k, _)| k.starts_with(prefix))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_put_and_get() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"key1", b"value1".to_vec());
        assert_eq!(backend.get(b"key1"), Some(b"value1".to_vec()));
    }

    #[test]
    fn test_get_missing_returns_none() {
        let backend = InMemoryBackend::new();
        assert_eq!(backend.get(b"missing"), None);
    }

    #[test]
    fn test_delete_removes_key() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"key1", b"value1".to_vec());
        backend.delete(b"key1");
        assert_eq!(backend.get(b"key1"), None);
    }

    #[test]
    fn test_delete_nonexistent_is_noop() {
        let mut backend = InMemoryBackend::new();
        backend.delete(b"no_such_key"); // no panic
    }

    #[test]
    fn test_contains() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"k", b"v".to_vec());
        assert!(backend.contains(b"k"));
        assert!(!backend.contains(b"missing"));
    }

    #[test]
    fn test_prefix_scan_returns_matching() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"proposal:1", b"a".to_vec());
        backend.put(b"proposal:2", b"b".to_vec());
        backend.put(b"attestation:1", b"c".to_vec());
        let results = backend.prefix_scan(b"proposal:");
        assert_eq!(results.len(), 2);
        assert_eq!(results[0].0, b"proposal:1".to_vec());
        assert_eq!(results[1].0, b"proposal:2".to_vec());
    }

    #[test]
    fn test_prefix_scan_empty_prefix_returns_all() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"a", b"1".to_vec());
        backend.put(b"b", b"2".to_vec());
        let results = backend.prefix_scan(b"");
        assert_eq!(results.len(), 2);
    }

    #[test]
    fn test_prefix_scan_no_match() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"proposal:1", b"x".to_vec());
        let results = backend.prefix_scan(b"attestation:");
        assert!(results.is_empty());
    }

    #[test]
    fn test_put_overwrites() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"k", b"v1".to_vec());
        backend.put(b"k", b"v2".to_vec());
        assert_eq!(backend.get(b"k"), Some(b"v2".to_vec()));
    }

    #[test]
    fn test_len_and_is_empty() {
        let mut backend = InMemoryBackend::new();
        assert!(backend.is_empty());
        backend.put(b"k1", b"v1".to_vec());
        backend.put(b"k2", b"v2".to_vec());
        assert_eq!(backend.len(), 2);
        backend.delete(b"k1");
        assert_eq!(backend.len(), 1);
    }

    #[test]
    fn test_prefix_scan_sorted_order() {
        let mut backend = InMemoryBackend::new();
        backend.put(b"p:3", b"c".to_vec());
        backend.put(b"p:1", b"a".to_vec());
        backend.put(b"p:2", b"b".to_vec());
        let results = backend.prefix_scan(b"p:");
        assert_eq!(results[0].0, b"p:1".to_vec());
        assert_eq!(results[1].0, b"p:2".to_vec());
        assert_eq!(results[2].0, b"p:3".to_vec());
    }
}
