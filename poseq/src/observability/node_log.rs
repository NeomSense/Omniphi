//! Structured JSON event log for PoSeq node operations.
//!
//! `NodeEventLog` replaces the plain `Vec<String>` event log in `NodeState`
//! with a ring-buffer of typed, JSON-serializable entries. Each entry carries:
//!
//! - `ts_ms` — wall-clock timestamp in milliseconds since Unix epoch
//! - `level` — INFO / WARN / ERROR
//! - `event` — a stable event-name string (e.g. `"batch.finalized"`)
//! - `epoch` / `slot` — protocol coordinates (if known)
//! - `node_id` — hex-encoded node identity (if relevant)
//! - `batch_id` — hex-encoded batch ID (if relevant)
//! - `details` — free-form detail string
//!
//! The log is bounded to `MAX_ENTRIES` to prevent unbounded memory growth.
//! Oldest entries are dropped when the ring is full.
//!
//! JSON serialization is used so entries can be piped to external log systems.

use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

/// Maximum number of entries kept in memory.
const MAX_ENTRIES: usize = 4096;

// ─── Level ───────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum LogLevel {
    Info,
    Warn,
    Error,
}

impl std::fmt::Display for LogLevel {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            LogLevel::Info => write!(f, "INFO"),
            LogLevel::Warn => write!(f, "WARN"),
            LogLevel::Error => write!(f, "ERROR"),
        }
    }
}

// ─── NodeLogEntry ─────────────────────────────────────────────────────────────

/// One structured log entry.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeLogEntry {
    /// Wall-clock milliseconds since Unix epoch.
    pub ts_ms: u64,
    /// Log level.
    pub level: LogLevel,
    /// Stable dot-separated event name, e.g. `"batch.finalized"`.
    pub event: String,
    /// Epoch coordinate (0 if unknown).
    pub epoch: u64,
    /// Slot coordinate (None if not slot-specific).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub slot: Option<u64>,
    /// Hex-encoded node_id of the subject (None if not applicable).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub node_id: Option<String>,
    /// Hex-encoded batch_id (None if not applicable).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub batch_id: Option<String>,
    /// Free-form detail string (empty string if none).
    pub details: String,
}

impl NodeLogEntry {
    /// Format as a single JSON line (for eprintln/file output).
    pub fn to_json_line(&self) -> String {
        serde_json::to_string(self).unwrap_or_else(|_| format!(
            r#"{{"ts_ms":{},"level":"{}","event":"{}","details":"(serialization error)"}}"#,
            self.ts_ms, self.level, self.event
        ))
    }

    /// Human-readable one-liner for terminal output.
    pub fn to_display(&self) -> String {
        let slot_part = self
            .slot
            .map(|s| format!(" slot={s}"))
            .unwrap_or_default();
        let node_part = self
            .node_id
            .as_deref()
            .map(|n| format!(" node={}", &n[..8.min(n.len())]))
            .unwrap_or_default();
        let batch_part = self
            .batch_id
            .as_deref()
            .map(|b| format!(" batch={}", &b[..8.min(b.len())]))
            .unwrap_or_default();
        format!(
            "[{}] {} epoch={}{}{}{} {}",
            self.level, self.event, self.epoch, slot_part, node_part, batch_part, self.details
        )
    }
}

// ─── NodeEventLog ─────────────────────────────────────────────────────────────

/// Bounded ring-buffer structured event log for one PoSeq node.
#[derive(Debug)]
pub struct NodeEventLog {
    entries: Vec<NodeLogEntry>,
    /// Total entries ever appended (for external consumers tracking tail).
    pub total_appended: u64,
    /// Whether to also print each entry as a JSON line to stderr.
    pub print_to_stderr: bool,
}

impl NodeEventLog {
    pub fn new(print_to_stderr: bool) -> Self {
        NodeEventLog {
            entries: Vec::with_capacity(64),
            total_appended: 0,
            print_to_stderr,
        }
    }

    /// Append a new entry. Drops the oldest entry when the ring is full.
    pub fn append(&mut self, entry: NodeLogEntry) {
        if self.print_to_stderr {
            eprintln!("{}", entry.to_json_line());
        }
        if self.entries.len() >= MAX_ENTRIES {
            self.entries.remove(0);
        }
        self.entries.push(entry);
        self.total_appended += 1;
    }

    /// Emit an INFO entry with the given event name and details.
    pub fn info(&mut self, event: &str, epoch: u64, slot: Option<u64>, details: impl Into<String>) {
        self.append(NodeLogEntry {
            ts_ms: now_ms(),
            level: LogLevel::Info,
            event: event.to_string(),
            epoch,
            slot,
            node_id: None,
            batch_id: None,
            details: details.into(),
        });
    }

    /// Emit a WARN entry.
    pub fn warn(&mut self, event: &str, epoch: u64, slot: Option<u64>, details: impl Into<String>) {
        self.append(NodeLogEntry {
            ts_ms: now_ms(),
            level: LogLevel::Warn,
            event: event.to_string(),
            epoch,
            slot,
            node_id: None,
            batch_id: None,
            details: details.into(),
        });
    }

    /// Emit an ERROR entry.
    pub fn error(&mut self, event: &str, epoch: u64, slot: Option<u64>, details: impl Into<String>) {
        self.append(NodeLogEntry {
            ts_ms: now_ms(),
            level: LogLevel::Error,
            event: event.to_string(),
            epoch,
            slot,
            node_id: None,
            batch_id: None,
            details: details.into(),
        });
    }

    /// Emit a batch-related INFO entry with batch_id correlation.
    pub fn batch_event(
        &mut self,
        event: &str,
        epoch: u64,
        slot: Option<u64>,
        batch_id: [u8; 32],
        details: impl Into<String>,
    ) {
        self.append(NodeLogEntry {
            ts_ms: now_ms(),
            level: LogLevel::Info,
            event: event.to_string(),
            epoch,
            slot,
            node_id: None,
            batch_id: Some(hex::encode(batch_id)),
            details: details.into(),
        });
    }

    /// Emit a node-related INFO entry with node_id correlation.
    pub fn node_event(
        &mut self,
        event: &str,
        epoch: u64,
        slot: Option<u64>,
        node_id: [u8; 32],
        details: impl Into<String>,
    ) {
        self.append(NodeLogEntry {
            ts_ms: now_ms(),
            level: LogLevel::Info,
            event: event.to_string(),
            epoch,
            slot,
            node_id: Some(hex::encode(node_id)),
            batch_id: None,
            details: details.into(),
        });
    }

    /// All entries in order (oldest first).
    pub fn entries(&self) -> &[NodeLogEntry] {
        &self.entries
    }

    /// Entries from position `from_seq` onward (based on `total_appended`).
    /// Use for tail-following: call with the last `total_appended` seen.
    pub fn tail_from(&self, from_total: u64) -> &[NodeLogEntry] {
        let already_seen = from_total.min(self.total_appended);
        let new_count = (self.total_appended - already_seen) as usize;
        let start = self.entries.len().saturating_sub(new_count);
        &self.entries[start..]
    }

    /// Number of entries currently in the ring.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Last entry, if any.
    pub fn last(&self) -> Option<&NodeLogEntry> {
        self.entries.last()
    }

    /// All entries as human-readable display strings (for tests / devnet output).
    pub fn display_lines(&self) -> Vec<String> {
        self.entries.iter().map(|e| e.to_display()).collect()
    }
}

fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_info_appended() {
        let mut log = NodeEventLog::new(false);
        log.info("node.started", 0, None, "test node");
        assert_eq!(log.len(), 1);
        assert_eq!(log.entries()[0].event, "node.started");
        assert_eq!(log.entries()[0].level, LogLevel::Info);
        assert_eq!(log.total_appended, 1);
    }

    #[test]
    fn test_batch_event_carries_batch_id() {
        let mut log = NodeEventLog::new(false);
        let bid = [0xABu8; 32];
        log.batch_event("batch.finalized", 5, Some(3), bid, "quorum reached");
        let e = &log.entries()[0];
        assert_eq!(e.batch_id.as_deref().unwrap(), hex::encode(bid));
        assert_eq!(e.epoch, 5);
        assert_eq!(e.slot, Some(3));
    }

    #[test]
    fn test_ring_bounded_at_max() {
        let mut log = NodeEventLog::new(false);
        for i in 0..=(MAX_ENTRIES + 10) {
            log.info("test.event", i as u64, None, "");
        }
        assert_eq!(log.len(), MAX_ENTRIES);
        assert_eq!(log.total_appended as usize, MAX_ENTRIES + 11);
    }

    #[test]
    fn test_tail_from_returns_new_entries() {
        let mut log = NodeEventLog::new(false);
        log.info("e1", 1, None, "");
        log.info("e2", 2, None, "");
        let tail = log.tail_from(1);
        assert_eq!(tail.len(), 1);
        assert_eq!(tail[0].event, "e2");
    }

    #[test]
    fn test_json_line_is_valid_json() {
        let mut log = NodeEventLog::new(false);
        log.warn("snapshot.rejected", 3, None, "hash mismatch");
        let line = log.entries()[0].to_json_line();
        let parsed: serde_json::Value = serde_json::from_str(&line).expect("valid JSON");
        assert_eq!(parsed["event"], "snapshot.rejected");
        assert_eq!(parsed["level"], "WARN");
    }

    #[test]
    fn test_display_line_format() {
        let mut log = NodeEventLog::new(false);
        log.info("peer.connected", 0, None, "peer joined");
        let display = log.display_lines();
        assert!(display[0].contains("peer.connected"));
        assert!(display[0].contains("INFO"));
    }
}
