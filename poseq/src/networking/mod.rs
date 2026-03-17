pub mod interfaces;

// Phase 6: real TCP networking
pub mod messages;
pub mod transport;
pub mod peer_manager;
pub mod node_runner;
pub mod discovery;

// Multi-process devnet scenario tests
#[cfg(test)]
mod devnet_scenarios;

pub use messages::{PoSeqMessage, WireProposal, WireAttestation, WireFinalized, WirePeerStatus, WireEpochAnnounce, NodeRole, NodeId};
pub use transport::NodeTransport;
pub use peer_manager::PeerManager;
pub use node_runner::{NetworkedNode, NodeConfig, NodeControl, NodeState, PeerEntry};
