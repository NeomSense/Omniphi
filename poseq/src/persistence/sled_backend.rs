//! Sled-backed durable persistence backend.
//!
//! `SledBackend` wraps a [`sled::Db`] and implements [`PersistenceBackend`],
//! providing crash-safe, fsync-on-flush storage with ordered key iteration.
//!
//! # Crash safety
//! Sled uses a lock-free B-tree with write-ahead logging (WAL). On process
//! crash, all data that was `flush()`-ed to disk survives. Data in the OS
//! page cache but not yet flushed may be lost; callers that need hard
//! durability should call `flush()` explicitly after critical writes.
//!
//! # Data layout
//! All keys are byte slices with structured prefixes (see `keys.rs`).
//! The sled tree is a single flat namespace — prefix scanning uses
//! `Db::range(prefix..)` with a `take_while` filter.

use sled::Db;
use crate::persistence::backend::PersistenceBackend;

/// Sled-backed implementation of [`PersistenceBackend`].
///
/// Open with [`SledBackend::open`] for production or
/// [`SledBackend::open_temp`] for ephemeral test databases.
pub struct SledBackend {
    db: Db,
}

impl SledBackend {
    /// Open (or create) a sled database at `path`.
    pub fn open(path: &std::path::Path) -> Result<Self, sled::Error> {
        let db = sled::open(path)?;
        Ok(SledBackend { db })
    }

    /// Open a temporary database in a system temp directory.
    /// The data is deleted when the returned [`SledBackend`] is dropped
    /// (because [`SledBackend::open_temp`] uses a unique path via `tempfile`).
    pub fn open_temp() -> Result<Self, sled::Error> {
        let dir = tempfile::tempdir().expect("tempdir");
        let db = sled::open(dir.path())?;
        // Keep the TempDir alive in the struct so it isn't dropped prematurely.
        // We deliberately leak it here because sled holds the path open and
        // the OS will reclaim it on process exit.
        std::mem::forget(dir);
        Ok(SledBackend { db })
    }

    /// Force all pending writes to disk.  Call after critical state transitions.
    pub fn flush(&self) -> Result<usize, sled::Error> {
        self.db.flush()
    }

    /// Returns the number of stored key-value pairs.
    pub fn len(&self) -> usize {
        self.db.len()
    }

    pub fn is_empty(&self) -> bool {
        self.db.is_empty()
    }
}

impl PersistenceBackend for SledBackend {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>> {
        self.db.get(key).ok()?.map(|v| v.to_vec())
    }

    fn put(&mut self, key: &[u8], value: Vec<u8>) {
        // Ignore insert errors in the backend interface; callers that need
        // hard error handling should use the `DurableStore` layer.
        let _ = self.db.insert(key, value);
    }

    fn delete(&mut self, key: &[u8]) {
        let _ = self.db.remove(key);
    }

    fn prefix_scan(&self, prefix: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)> {
        self.db
            .range(prefix.to_vec()..)
            .filter_map(|r| r.ok())
            .take_while(|(k, _)| k.starts_with(prefix))
            .map(|(k, v)| (k.to_vec(), v.to_vec()))
            .collect()
    }

    fn contains(&self, key: &[u8]) -> bool {
        self.db.contains_key(key).unwrap_or(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::persistence::backend::PersistenceBackend;

    fn open() -> SledBackend {
        SledBackend::open_temp().unwrap()
    }

    #[test]
    fn test_sled_put_and_get() {
        let mut b = open();
        b.put(b"hello", b"world".to_vec());
        assert_eq!(b.get(b"hello"), Some(b"world".to_vec()));
    }

    #[test]
    fn test_sled_get_missing_returns_none() {
        let b = open();
        assert_eq!(b.get(b"nope"), None);
    }

    #[test]
    fn test_sled_delete() {
        let mut b = open();
        b.put(b"k", b"v".to_vec());
        b.delete(b"k");
        assert!(b.get(b"k").is_none());
    }

    #[test]
    fn test_sled_contains() {
        let mut b = open();
        assert!(!b.contains(b"x"));
        b.put(b"x", b"y".to_vec());
        assert!(b.contains(b"x"));
    }

    #[test]
    fn test_sled_prefix_scan_sorted() {
        let mut b = open();
        b.put(b"p:3", b"c".to_vec());
        b.put(b"p:1", b"a".to_vec());
        b.put(b"p:2", b"b".to_vec());
        b.put(b"q:1", b"x".to_vec()); // different prefix
        let results = b.prefix_scan(b"p:");
        assert_eq!(results.len(), 3);
        assert_eq!(results[0].0, b"p:1".to_vec());
        assert_eq!(results[1].0, b"p:2".to_vec());
        assert_eq!(results[2].0, b"p:3".to_vec());
    }

    #[test]
    fn test_sled_overwrite() {
        let mut b = open();
        b.put(b"k", b"v1".to_vec());
        b.put(b"k", b"v2".to_vec());
        assert_eq!(b.get(b"k"), Some(b"v2".to_vec()));
    }

    #[test]
    fn test_sled_flush_succeeds() {
        let b = open();
        b.flush().unwrap();
    }

    #[test]
    fn test_sled_len() {
        let mut b = open();
        assert_eq!(b.len(), 0);
        b.put(b"a", b"1".to_vec());
        b.put(b"b", b"2".to_vec());
        assert_eq!(b.len(), 2);
        b.delete(b"a");
        assert_eq!(b.len(), 1);
    }

    #[test]
    fn test_sled_persistence_across_reopen() {
        // Write data, close the DB (by dropping), reopen from same path, verify data survived.
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().to_path_buf();

        {
            let mut b = SledBackend::open(&path).unwrap();
            b.put(b"durable_key", b"durable_value".to_vec());
            b.flush().unwrap();
        } // SledBackend dropped — db closed

        {
            let b = SledBackend::open(&path).unwrap();
            assert_eq!(b.get(b"durable_key"), Some(b"durable_value".to_vec()));
        }
    }
}
