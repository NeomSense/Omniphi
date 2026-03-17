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
}

#[derive(Debug)]
pub enum ConfigError {
    Io(String),
    Parse(String),
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConfigError::Io(e) => write!(f, "IO error reading config: {e}"),
            ConfigError::Parse(e) => write!(f, "TOML parse error: {e}"),
        }
    }
}
