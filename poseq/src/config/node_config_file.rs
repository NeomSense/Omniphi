//! TOML-deserializable node configuration file types.
//!
//! # Example `poseq.toml`
//! ```toml
//! [node]
//! id = "0101010101010101010101010101010101010101010101010101010101010101"
//! listen_addr = "0.0.0.0:7001"
//! peers = ["127.0.0.1:7002", "127.0.0.1:7003"]
//! quorum_threshold = 2
//! slot_duration_ms = 2000
//! data_dir = "./poseq_data"
//! role = "attestor"
//! key_seed = "deadbeef..."   # optional; 64 hex chars
//! metrics_addr = "0.0.0.0:9090"  # optional
//! seed_peers = ["seed1.omniphi.io:7000"]  # optional bootstrap peers
//! ```

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NodeRoleConfig {
    Leader,
    Attestor,
    Observer,
}

/// The `[node]` section of poseq.toml.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeConfigFile {
    /// 32-byte node identity as 64 hex chars.
    pub id: String,
    /// TCP listen address.
    pub listen_addr: String,
    /// Static peer addresses.
    #[serde(default)]
    pub peers: Vec<String>,
    /// Quorum threshold (number of approvals to finalize).
    pub quorum_threshold: usize,
    /// Slot duration in milliseconds.
    #[serde(default = "default_slot_ms")]
    pub slot_duration_ms: u64,
    /// Directory for durable storage (sled).
    #[serde(default = "default_data_dir")]
    pub data_dir: String,
    /// Node role.
    #[serde(default = "default_role")]
    pub role: NodeRoleConfig,
    /// Optional 32-byte signing key seed as 64 hex chars.
    /// If absent, a random key is generated (not persisted).
    pub key_seed: Option<String>,
    /// Optional Prometheus metrics HTTP address.
    pub metrics_addr: Option<String>,
    /// Optional discovery seed node addresses (for bootstrap).
    #[serde(default)]
    pub seed_peers: Vec<String>,
    /// Slots per epoch.  When current_slot crosses a multiple of this, the node
    /// auto-exports the completed epoch and advances current_epoch.
    /// Defaults to 10 if absent.
    pub slots_per_epoch: Option<u64>,
}

fn default_slot_ms() -> u64 { 2000 }
fn default_data_dir() -> String { "./poseq_data".into() }
fn default_role() -> NodeRoleConfig { NodeRoleConfig::Attestor }

impl NodeConfigFile {
    /// Validate the configuration. Returns `Err(description)` on the first
    /// problem found; returns `Ok(())` if the config is internally consistent.
    pub fn validate(&self) -> Result<(), String> {
        // 1. slots_per_epoch in [1, 10000] if present
        if let Some(spe) = self.slots_per_epoch {
            if spe < 1 || spe > 10_000 {
                return Err(format!(
                    "slots_per_epoch must be in range [1, 10000], got {}",
                    spe
                ));
            }
        }

        // 2. quorum_threshold in 1..=100
        if self.quorum_threshold < 1 || self.quorum_threshold > 100 {
            return Err(format!(
                "quorum_threshold must be in range [1, 100], got {}",
                self.quorum_threshold
            ));
        }

        // 3. All peer addresses parse as valid SocketAddr
        for addr in &self.peers {
            addr.parse::<std::net::SocketAddr>().map_err(|_| {
                format!("invalid peer address: {:?}", addr)
            })?;
        }

        // 4. No duplicate peer addresses
        let mut seen = std::collections::BTreeSet::new();
        for addr in &self.peers {
            if !seen.insert(addr.as_str()) {
                return Err(format!("duplicate peer address: {:?}", addr));
            }
        }

        // 5. slot_duration_ms in [100, 60000]
        if self.slot_duration_ms < 100 || self.slot_duration_ms > 60_000 {
            return Err(format!(
                "slot_duration_ms must be in range [100, 60000], got {}",
                self.slot_duration_ms
            ));
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn valid_config() -> NodeConfigFile {
        NodeConfigFile {
            id: "0".repeat(64),
            listen_addr: "0.0.0.0:7001".into(),
            peers: vec!["127.0.0.1:7002".into(), "127.0.0.1:7003".into()],
            quorum_threshold: 2,
            slot_duration_ms: 2000,
            data_dir: "./data".into(),
            role: NodeRoleConfig::Attestor,
            key_seed: None,
            metrics_addr: None,
            seed_peers: vec![],
            slots_per_epoch: Some(10),
        }
    }

    #[test]
    fn test_validate_ok() {
        assert!(valid_config().validate().is_ok());
    }

    #[test]
    fn test_validate_slots_per_epoch_zero() {
        let mut cfg = valid_config();
        cfg.slots_per_epoch = Some(0);
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("slots_per_epoch"));
    }

    #[test]
    fn test_validate_slots_per_epoch_too_large() {
        let mut cfg = valid_config();
        cfg.slots_per_epoch = Some(10_001);
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("slots_per_epoch"));
    }

    #[test]
    fn test_validate_quorum_zero() {
        let mut cfg = valid_config();
        cfg.quorum_threshold = 0;
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("quorum_threshold"));
    }

    #[test]
    fn test_validate_quorum_too_large() {
        let mut cfg = valid_config();
        cfg.quorum_threshold = 101;
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("quorum_threshold"));
    }

    #[test]
    fn test_validate_invalid_peer_addr() {
        let mut cfg = valid_config();
        cfg.peers = vec!["not-an-address".into()];
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("invalid peer address"));
    }

    #[test]
    fn test_validate_duplicate_peer_addr() {
        let mut cfg = valid_config();
        cfg.peers = vec!["127.0.0.1:7002".into(), "127.0.0.1:7002".into()];
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("duplicate peer address"));
    }

    #[test]
    fn test_validate_slot_ms_too_small() {
        let mut cfg = valid_config();
        cfg.slot_duration_ms = 99;
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("slot_duration_ms"));
    }

    #[test]
    fn test_validate_slot_ms_too_large() {
        let mut cfg = valid_config();
        cfg.slot_duration_ms = 60_001;
        let err = cfg.validate().unwrap_err();
        assert!(err.contains("slot_duration_ms"));
    }

    #[test]
    fn test_validate_slots_per_epoch_none_is_ok() {
        let mut cfg = valid_config();
        cfg.slots_per_epoch = None;
        assert!(cfg.validate().is_ok());
    }

    #[test]
    fn test_validate_empty_peers_is_ok() {
        let mut cfg = valid_config();
        cfg.peers = vec![];
        assert!(cfg.validate().is_ok());
    }
}
