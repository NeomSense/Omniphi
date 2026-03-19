//! HotStuff BFT consensus for PoSeq.
//!
//! This module provides a complete implementation of the HotStuff pipelined
//! BFT consensus protocol for use as the sequencing consensus layer.
//!
//! # Module layout
//!
//! - `types` — Core data types: `HotStuffBlock`, `QuorumCertificate`, `HotStuffVote`,
//!              `NewViewMessage`, `SafetyRule`, `Phase`, `View`.
//! - `pacemaker` — View timer, exponential backoff, `NewView` message handling.
//! - `engine` — `HotStuffEngine`: the main state machine that processes messages
//!              and produces `HotStuffOutput` actions.
//!
//! # Usage in `NetworkedNode`
//!
//! The `HotStuffEngine` replaces the existing 2-phase quorum voting in
//! `node_runner.rs`.  The engine is polled on each inbound message and returns
//! an `HotStuffOutput` that the node runner dispatches:
//!
//! ```rust,ignore
//! // In node_runner::handle_message():
//! let output = self.hotstuff.on_block(block);
//! match output {
//!     HotStuffOutput::SendVote(vote) => { /* broadcast vote */ }
//!     HotStuffOutput::BroadcastQC(qc) => { /* broadcast QC */ }
//!     HotStuffOutput::Finalize(block) => { /* commit block to store */ }
//!     HotStuffOutput::SendNewView(nv) => { /* send new-view */ }
//!     _ => {}
//! }
//! ```

pub mod types;
pub mod pacemaker;
pub mod engine;
pub mod persistence;
pub mod verification;

pub use types::{
    HotStuffBlock, QuorumCertificate, HotStuffVote, NewViewMessage,
    Phase, View, SafetyRule,
};
pub use pacemaker::Pacemaker;
pub use engine::{HotStuffEngine, HotStuffOutput};
pub use persistence::HotStuffConsensusState;
pub use verification::{
    ConsensusVerificationError, verify_vote_membership, verify_proposal_leader,
    verify_qc_committee, verify_new_view_membership, compute_vote_payload,
    compute_proposal_payload,
};
