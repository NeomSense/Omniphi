//! Loads the full PoSeq node configuration from a TOML file.
//!
//! The config file has two top-level sections:
//! - `[node]`   — networking / identity / storage settings
//! - `[policy]` — ordering, batch, and class policies (optional; uses defaults if absent)

use std::path::Path;

use crate::config::node_config_file::NodeConfigFile;
use crate::config::policy::PoSeqPolicy;

/// Root of the `poseq.toml` file.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FullNodeConfig {
    pub node: NodeConfigFile,
    #[serde(default)]
    pub policy: PoSeqPolicy,
}

impl FullNodeConfig {
    /// Load and parse a TOML config file from `path`.
    pub fn load_from_file<P: AsRef<Path>>(path: P) -> Result<Self, ConfigError> {
        let raw = std::fs::read_to_string(path.as_ref())
            .map_err(|e| ConfigError::Io(e.to_string()))?;
        toml::from_str(&raw).map_err(|e| ConfigError::Parse(e.to_string()))
    }

    /// Validate all fields for operational correctness.
    ///
    /// Returns a list of human-readable error strings. An empty Vec means the
    /// config is valid. Call this after `load_from_file` before starting the node.
    pub fn validate(&self) -> Vec<String> {
        let mut errors: Vec<String> = Vec::new();

        // ── node.id ──────────────────────────────────────────────────────────
        match hex::decode(&self.node.id) {
            Ok(b) if b.len() == 32 => {}
            Ok(b) => errors.push(format!(
                "node.id must be 64 hex chars (32 bytes); got {} bytes ({})",
                b.len(), self.node.id.len()
            )),
            Err(e) => errors.push(format!("node.id is not valid hex: {e}")),
        }

        // ── node.listen_addr ──────────────────────────────────────────────────
        if self.node.listen_addr.is_empty() {
            errors.push("node.listen_addr must not be empty".into());
        }

        // ── node.quorum_threshold ─────────────────────────────────────────────
        if self.node.quorum_threshold == 0 {
            errors.push("node.quorum_threshold must be >= 1".into());
        }
        let peer_count = self.node.peers.len();
        // With N peers (+ self = N+1 nodes), quorum should be <= N+1
        if self.node.quorum_threshold > peer_count + 1 {
            errors.push(format!(
                "node.quorum_threshold ({}) exceeds total node count ({} peers + self = {})",
                self.node.quorum_threshold,
                peer_count,
                peer_count + 1,
            ));
        }

        // ── node.slot_duration_ms ────────────────────────────────────────────
        if self.node.slot_duration_ms < 100 {
            errors.push(format!(
                "node.slot_duration_ms ({}) is too low; minimum is 100ms",
                self.node.slot_duration_ms
            ));
        }

        // ── node.data_dir ─────────────────────────────────────────────────────
        if self.node.data_dir.is_empty() {
            errors.push("node.data_dir must not be empty".into());
        }

        // ── node.key_seed ─────────────────────────────────────────────────────
        if let Some(ref seed) = self.node.key_seed {
            match hex::decode(seed) {
                Ok(b) if b.len() == 32 => {}
                Ok(b) => errors.push(format!(
                    "node.key_seed must be 64 hex chars (32 bytes); got {} bytes",
                    b.len()
                )),
                Err(e) => errors.push(format!("node.key_seed is not valid hex: {e}")),
            }
        }

        // ── policy.batch ──────────────────────────────────────────────────────
        if self.policy.batch.max_submissions_per_batch == 0 {
            errors.push("policy.batch.max_submissions_per_batch must be >= 1".into());
        }
        if self.policy.batch.max_payload_bytes_per_submission == 0 {
            errors.push("policy.batch.max_payload_bytes_per_submission must be >= 1".into());
        }
        if self.policy.batch.max_pending_queue_size == 0 {
            errors.push("policy.batch.max_pending_queue_size must be >= 1".into());
        }

        errors
    }
}

#[derive(Debug)]
pub enum ConfigError {
    Io(String),
    Parse(String),
    /// One or more validation errors.
    Validation(Vec<String>),
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConfigError::Io(e) => write!(f, "IO error reading config: {e}"),
            ConfigError::Parse(e) => write!(f, "TOML parse error: {e}"),
            ConfigError::Validation(errs) => {
                write!(f, "Config validation failed ({} error(s)):", errs.len())?;
                for e in errs {
                    write!(f, "\n  • {e}")?;
                }
                Ok(())
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::node_config_file::{NodeConfigFile, NodeRoleConfig};
    use crate::config::policy::PoSeqPolicy;

    fn valid_config() -> FullNodeConfig {
        FullNodeConfig {
            node: NodeConfigFile {
                id: "0101010101010101010101010101010101010101010101010101010101010101".into(),
                listen_addr: "127.0.0.1:7001".into(),
                peers: vec!["127.0.0.1:7002".into(), "127.0.0.1:7003".into()],
                quorum_threshold: 2,
                slot_duration_ms: 2000,
                data_dir: "./poseq_data".into(),
                role: NodeRoleConfig::Attestor,
                key_seed: None,
                metrics_addr: None,
                seed_peers: vec![],
                slots_per_epoch: None,
            },
            policy: PoSeqPolicy::default(),
        }
    }

    #[test]
    fn valid_config_passes() {
        assert!(valid_config().validate().is_empty());
    }

    #[test]
    fn invalid_node_id_rejected() {
        let mut cfg = valid_config();
        cfg.node.id = "nothex".into();
        let errs = cfg.validate();
        assert!(!errs.is_empty());
        assert!(errs[0].contains("node.id"));
    }

    #[test]
    fn zero_quorum_rejected() {
        let mut cfg = valid_config();
        cfg.node.quorum_threshold = 0;
        let errs = cfg.validate();
        assert!(errs.iter().any(|e| e.contains("quorum_threshold")));
    }

    #[test]
    fn quorum_exceeds_cluster_size_rejected() {
        let mut cfg = valid_config();
        cfg.node.quorum_threshold = 10; // 2 peers + self = 3 nodes; 10 > 3
        let errs = cfg.validate();
        assert!(errs.iter().any(|e| e.contains("quorum_threshold")));
    }

    #[test]
    fn slot_ms_too_low_rejected() {
        let mut cfg = valid_config();
        cfg.node.slot_duration_ms = 50;
        let errs = cfg.validate();
        assert!(errs.iter().any(|e| e.contains("slot_duration_ms")));
    }

    #[test]
    fn invalid_key_seed_rejected() {
        let mut cfg = valid_config();
        cfg.node.key_seed = Some("notvalidhex".into());
        let errs = cfg.validate();
        assert!(errs.iter().any(|e| e.contains("key_seed")));
    }
}
