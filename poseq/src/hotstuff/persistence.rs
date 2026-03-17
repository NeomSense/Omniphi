//! Phase 6 — HotStuff consensus state persistence for crash-safe recovery.
//!
//! Persists the critical consensus state (locked QC, high QC, current view)
//! so that a restarted node can safely rejoin without double-voting or forking.

use serde::{Deserialize, Serialize};
use crate::hotstuff::types::{QuorumCertificate, View};

/// Snapshot of HotStuff consensus state for persistence.
///
/// This is the minimum state needed to safely resume consensus after a crash.
/// A node that restarts must load this state before processing any messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HotStuffConsensusState {
    /// Current view number.
    pub current_view: View,
    /// Highest QC seen (for liveness — determines which proposal to extend).
    pub high_qc: QuorumCertificate,
    /// Locked QC (safety — node must not vote for proposals that conflict with this).
    pub locked_qc: QuorumCertificate,
    /// Views that this node has already voted in (prevents double-voting after restart).
    /// Stores (view, phase_byte) pairs.
    pub voted_views: Vec<(View, u8)>,
    /// Last finalized view number.
    pub last_finalized_view: View,
    /// Last finalized block ID.
    pub last_finalized_block_id: [u8; 32],
}

impl HotStuffConsensusState {
    /// Create initial state from genesis.
    pub fn genesis() -> Self {
        let genesis_qc = QuorumCertificate::genesis();
        HotStuffConsensusState {
            current_view: 0,
            high_qc: genesis_qc.clone(),
            locked_qc: genesis_qc,
            voted_views: Vec::new(),
            last_finalized_view: 0,
            last_finalized_block_id: [0u8; 32],
        }
    }

    /// Capture current state from a running engine.
    pub fn capture(
        current_view: View,
        high_qc: &QuorumCertificate,
        locked_qc: &QuorumCertificate,
        voted_views: &[(View, u8)],
        last_finalized_view: View,
        last_finalized_block_id: [u8; 32],
    ) -> Self {
        HotStuffConsensusState {
            current_view,
            high_qc: high_qc.clone(),
            locked_qc: locked_qc.clone(),
            voted_views: voted_views.to_vec(),
            last_finalized_view,
            last_finalized_block_id,
        }
    }

    /// Check if we already voted in a given view and phase.
    pub fn has_voted(&self, view: View, phase_byte: u8) -> bool {
        self.voted_views.iter().any(|(v, p)| *v == view && *p == phase_byte)
    }
}

/// Key prefix for HotStuff consensus state in the durable store.
pub const HOTSTUFF_STATE_KEY: &[u8] = b"hotstuff:consensus_state";

/// Key prefix for persisted finalized blocks.
pub const HOTSTUFF_FINALIZED_PREFIX: &[u8] = b"hotstuff:finalized:";

/// Build key for a finalized block by view.
pub fn finalized_block_key(view: View) -> Vec<u8> {
    let mut k = HOTSTUFF_FINALIZED_PREFIX.to_vec();
    k.extend_from_slice(&view.to_be_bytes());
    k
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_genesis_state() {
        let state = HotStuffConsensusState::genesis();
        assert_eq!(state.current_view, 0);
        assert_eq!(state.high_qc.view, 0);
        assert_eq!(state.locked_qc.view, 0);
        assert!(state.voted_views.is_empty());
    }

    #[test]
    fn test_has_voted() {
        let mut state = HotStuffConsensusState::genesis();
        assert!(!state.has_voted(1, 0));
        state.voted_views.push((1, 0));
        assert!(state.has_voted(1, 0));
        assert!(!state.has_voted(1, 1));
        assert!(!state.has_voted(2, 0));
    }

    #[test]
    fn test_capture_and_serialize() {
        let qc = QuorumCertificate::genesis();
        let state = HotStuffConsensusState::capture(
            5, &qc, &qc, &[(3, 0), (4, 1)], 2, [0xAA; 32],
        );
        assert_eq!(state.current_view, 5);
        assert_eq!(state.voted_views.len(), 2);
        assert_eq!(state.last_finalized_view, 2);

        // Round-trip serialization
        let bytes = bincode::serialize(&state).unwrap();
        let restored: HotStuffConsensusState = bincode::deserialize(&bytes).unwrap();
        assert_eq!(restored.current_view, 5);
        assert_eq!(restored.voted_views.len(), 2);
    }
}
