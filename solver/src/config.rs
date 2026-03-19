//! Solver configuration.

/// Configuration for an Omniphi solver instance.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SolverConfig {
    /// Solver's unique 32-byte ID (hex).
    pub solver_id: String,
    /// Ed25519 public key (hex).
    pub public_key: String,
    /// PoSeq node endpoint to connect to.
    pub poseq_endpoint: String,
    /// Chain RPC endpoint for settlement monitoring.
    pub chain_rpc: String,
    /// Intent classes this solver supports.
    pub supported_intent_classes: Vec<String>,
    /// Maximum fee in base units the solver is willing to charge.
    pub max_fee_quote: u64,
    /// Bond amount to lock per commitment (base units).
    pub bond_per_commitment: u128,
    /// Maximum number of intents to solve per batch window.
    pub max_intents_per_window: usize,
    /// Path to keystore file.
    pub keystore_path: String,
    /// Log level.
    pub log_level: String,
}

impl Default for SolverConfig {
    fn default() -> Self {
        SolverConfig {
            solver_id: "0".repeat(64),
            public_key: "0".repeat(64),
            poseq_endpoint: "http://127.0.0.1:26657".into(),
            chain_rpc: "http://127.0.0.1:26657".into(),
            supported_intent_classes: vec!["transfer".into(), "swap".into()],
            max_fee_quote: 100,
            bond_per_commitment: 1000,
            max_intents_per_window: 16,
            keystore_path: "~/.pos/solver/key.json".into(),
            log_level: "info".into(),
        }
    }
}
