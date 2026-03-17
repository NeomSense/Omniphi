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
}

fn default_slot_ms() -> u64 { 2000 }
fn default_data_dir() -> String { "./poseq_data".into() }
fn default_role() -> NodeRoleConfig { NodeRoleConfig::Attestor }
