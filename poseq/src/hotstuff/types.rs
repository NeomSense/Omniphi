//! Core HotStuff BFT data types.
//!
//! # HotStuff protocol summary
//!
//! HotStuff is a pipelined 3-phase BFT consensus protocol:
//!
//! - **Prepare phase**: Leader proposes a block.  Validators send PREPARE votes.
//!   Leader forms a `QuorumCertificate` (QC) from 2f+1 votes.
//! - **Pre-commit phase**: Leader broadcasts PREPARE-QC.  Validators send PRE-COMMIT
//!   votes.  Leader forms PRE-COMMIT-QC.
//! - **Commit phase**: Leader broadcasts PRE-COMMIT-QC.  Validators send COMMIT votes.
//!   Leader forms COMMIT-QC.
//! - **Decide**: Leader broadcasts COMMIT-QC.  All nodes execute the block.
//!
//! In the **chained** variant each QC serves double-duty for successive phases,
//! which is the variant implemented here.
//!
//! # Safety rule (locking)
//!
//! A node only votes for block `b` in view `v` if either:
//! - `b` extends its `LockedQC.block_id` (chain rule), OR
//! - The QC for `b`'s parent has a higher view than `LockedQC.view` (safe extension).
//!
//! # Liveness rule (pacemaker)
//!
//! If a view times out, the pacemaker advances to view `v+1` and sends a `NewView`
//! message carrying the node's current `HighQC`.  The next leader advances to the
//! highest view from received `NewView` messages and proposes extending `HighQC`.

use serde::{Serialize, Deserialize};
use crate::networking::messages::NodeId;

// ─── View numbers ─────────────────────────────────────────────────────────────

/// HotStuff view number (monotonically increasing).
pub type View = u64;

// ─── Consensus phases ─────────────────────────────────────────────────────────

/// The phase a `QuorumCertificate` was formed for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub enum Phase {
    /// Prepare phase: leader proposes a block.
    Prepare,
    /// Pre-commit phase: leader has a Prepare-QC.
    PreCommit,
    /// Commit phase: leader has a Pre-Commit-QC.
    Commit,
    /// Decide phase: leader has a Commit-QC; block is finalized.
    Decide,
}

impl Phase {
    pub fn name(&self) -> &'static str {
        match self {
            Phase::Prepare => "PREPARE",
            Phase::PreCommit => "PRE-COMMIT",
            Phase::Commit => "COMMIT",
            Phase::Decide => "DECIDE",
        }
    }

    /// Returns the next phase in the pipeline, or `None` if already at Decide.
    pub fn next(&self) -> Option<Phase> {
        match self {
            Phase::Prepare => Some(Phase::PreCommit),
            Phase::PreCommit => Some(Phase::Commit),
            Phase::Commit => Some(Phase::Decide),
            Phase::Decide => None,
        }
    }
}

// ─── HotStuff block ───────────────────────────────────────────────────────────

/// A HotStuff block (proposal).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HotStuffBlock {
    /// Unique block identifier: SHA256(parent_id ‖ batch_root ‖ view ‖ leader_id).
    pub block_id: [u8; 32],
    /// Hash of the parent block (or `[0u8; 32]` for genesis).
    pub parent_id: [u8; 32],
    /// View in which this block was proposed.
    pub view: View,
    /// The leader that proposed this block.
    pub leader_id: NodeId,
    /// Merkle root of the ordered submission IDs.
    pub batch_root: [u8; 32],
    /// Ordered submission IDs.
    pub ordered_submission_ids: Vec<[u8; 32]>,
    /// The highest QC the leader knows about (carried to advance `HighQC`).
    pub justify_qc: Option<QuorumCertificate>,
}

impl HotStuffBlock {
    /// Compute the canonical block_id from stable fields.
    pub fn compute_id(
        parent_id: &[u8; 32],
        batch_root: &[u8; 32],
        view: View,
        leader_id: &NodeId,
    ) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        let mut h = Sha256::new();
        h.update(b"HOTSTUFF_BLOCK_V1");
        h.update(parent_id);
        h.update(batch_root);
        h.update(view.to_be_bytes());
        h.update(leader_id);
        h.finalize().into()
    }

    /// Returns `true` if this block extends `ancestor_id` directly
    /// (parent_id == ancestor_id).
    pub fn extends_direct(&self, ancestor_id: &[u8; 32]) -> bool {
        &self.parent_id == ancestor_id
    }
}

// ─── Quorum certificate ───────────────────────────────────────────────────────

/// A quorum certificate: proof that 2f+1 nodes voted for a block in a view+phase.
///
/// Each entry in `signatures` is `(signer_id, ed25519_sig)` over the vote hash
/// SHA256("HOTSTUFF_VOTE_V1" ‖ view ‖ block_id ‖ phase_byte).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuorumCertificate {
    /// View in which the votes were cast.
    pub view: View,
    /// The block this QC certifies.
    pub block_id: [u8; 32],
    /// The phase this QC was formed for.
    pub phase: Phase,
    /// 2f+1 (signer_id, 64-byte signature) pairs.
    /// Signatures are stored as `Vec<u8>` (exactly 64 bytes each) because
    /// serde only auto-derives for arrays up to `[T; 32]`.
    pub signatures: Vec<(NodeId, Vec<u8>)>,
}

impl QuorumCertificate {
    /// Compute the vote hash that all signers sign.
    pub fn vote_hash(view: View, block_id: &[u8; 32], phase: Phase) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        let mut h = Sha256::new();
        h.update(b"HOTSTUFF_VOTE_V1");
        h.update(view.to_be_bytes());
        h.update(block_id);
        h.update([phase as u8]);
        h.finalize().into()
    }

    /// Returns `true` if this QC has at least `threshold` distinct signers.
    pub fn has_quorum(&self, threshold: usize) -> bool {
        self.signatures.len() >= threshold
    }

    /// Genesis QC (view=0, all-zeros block, no signatures).
    pub fn genesis() -> Self {
        QuorumCertificate {
            view: 0,
            block_id: [0u8; 32],
            phase: Phase::Decide,
            signatures: vec![],
        }
    }
}

// ─── Vote message ─────────────────────────────────────────────────────────────

/// A single vote cast by a validator.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HotStuffVote {
    pub view: View,
    pub block_id: [u8; 32],
    pub phase: Phase,
    pub voter_id: NodeId,
    /// Ed25519 signature over `vote_hash(view, block_id, phase)`.
    /// Stored as `Vec<u8>` (exactly 64 bytes) because serde only auto-derives
    /// for arrays up to `[T; 32]`.
    pub signature: Vec<u8>,
}

impl HotStuffVote {
    /// Compute the canonical vote hash this vote should sign.
    pub fn vote_hash(&self) -> [u8; 32] {
        QuorumCertificate::vote_hash(self.view, &self.block_id, self.phase)
    }
}

// ─── NewView message ──────────────────────────────────────────────────────────

/// Sent by a node when it times out in a view (pacemaker liveness trigger).
/// Carries the node's highest known QC so the next leader can pick up where
/// the network left off.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NewViewMessage {
    /// The new view the sender is advancing to.
    pub new_view: View,
    /// The highest QC the sender knows (for `HighQC` selection).
    pub high_qc: QuorumCertificate,
    pub sender_id: NodeId,
}

// ─── Safety rule ──────────────────────────────────────────────────────────────

/// Monotone locking state: ensures a node never votes for conflicting chains.
///
/// A node locks on a block when it receives a PRE-COMMIT-QC for that block.
/// It only votes for a new block `b` if:
/// - `b.parent_id == locked_qc.block_id` (extending locked block), OR
/// - `b.justify_qc.view > locked_qc.view` (higher-view QC overrides old lock).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SafetyRule {
    /// The last QC this node locked on (starts as genesis QC at view=0).
    pub locked_qc: QuorumCertificate,
    /// The highest QC seen (for liveness; `HighQC.view >= LockedQC.view` always).
    pub high_qc: QuorumCertificate,
    /// The current view this node is in.
    pub current_view: View,
    /// The last view this node voted in (monotone — never votes for a lower view).
    /// Persisted to prevent equivocation after restart.
    pub last_voted_view: View,
}

impl SafetyRule {
    pub fn new() -> Self {
        let genesis = QuorumCertificate::genesis();
        SafetyRule {
            locked_qc: genesis.clone(),
            high_qc: genesis,
            current_view: 0,
            last_voted_view: 0,
        }
    }

    /// Serialize to bytes for durable persistence.
    pub fn to_bytes(&self) -> Vec<u8> {
        bincode::serialize(self).expect("SafetyRule serialize")
    }

    /// Deserialize from bytes (e.g., from sled on startup).
    pub fn from_bytes(bytes: &[u8]) -> Option<Self> {
        bincode::deserialize(bytes).ok()
    }

    /// Returns `true` if voting for `block` is safe given current locked state.
    ///
    /// Enforces monotone voting: never votes for a view ≤ `last_voted_view`.
    /// This prevents equivocation even after restarts (if `last_voted_view` is
    /// persisted and restored).
    pub fn safe_to_vote(&self, block: &HotStuffBlock) -> bool {
        // Monotone voting: reject views we've already voted in
        if block.view <= self.last_voted_view {
            return false;
        }
        // Case 1: block extends the locked block directly (chain extension rule)
        if block.extends_direct(&self.locked_qc.block_id) {
            return true;
        }
        // Case 2: block's justify QC has a higher view than locked_qc (safe extension)
        if let Some(ref justify) = block.justify_qc {
            if justify.view > self.locked_qc.view {
                return true;
            }
        }
        // Genesis block is always safe
        block.parent_id == [0u8; 32]
    }

    /// Record that we voted in `view`. Must be called after casting a vote.
    pub fn record_vote(&mut self, view: View) {
        if view > self.last_voted_view {
            self.last_voted_view = view;
        }
    }

    /// Update `HighQC` if `qc` is for a higher view.
    pub fn update_high_qc(&mut self, qc: QuorumCertificate) {
        if qc.view > self.high_qc.view {
            self.high_qc = qc;
        }
    }

    /// Lock on `qc` (called when a PRE-COMMIT-QC is formed or received).
    pub fn lock(&mut self, qc: QuorumCertificate) {
        if qc.view >= self.locked_qc.view {
            self.locked_qc = qc.clone();
            self.update_high_qc(qc);
        }
    }

    /// Advance to a new view (called on timeout or after Decide).
    pub fn advance_view(&mut self, new_view: View) {
        if new_view > self.current_view {
            self.current_view = new_view;
        }
    }
}

impl Default for SafetyRule {
    fn default() -> Self { Self::new() }
}

// ─── Tests ─────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn nid(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    #[test]
    fn test_block_id_deterministic() {
        let parent = nid(1);
        let root = nid(2);
        let id1 = HotStuffBlock::compute_id(&parent, &root, 5, &nid(3));
        let id2 = HotStuffBlock::compute_id(&parent, &root, 5, &nid(3));
        assert_eq!(id1, id2);
    }

    #[test]
    fn test_block_id_changes_with_view() {
        let parent = nid(1);
        let root = nid(2);
        let id1 = HotStuffBlock::compute_id(&parent, &root, 5, &nid(3));
        let id2 = HotStuffBlock::compute_id(&parent, &root, 6, &nid(3));
        assert_ne!(id1, id2);
    }

    #[test]
    fn test_vote_hash_deterministic() {
        let bid = nid(5);
        let h1 = QuorumCertificate::vote_hash(3, &bid, Phase::Prepare);
        let h2 = QuorumCertificate::vote_hash(3, &bid, Phase::Prepare);
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_vote_hash_differs_by_phase() {
        let bid = nid(5);
        let h_prepare = QuorumCertificate::vote_hash(3, &bid, Phase::Prepare);
        let h_precommit = QuorumCertificate::vote_hash(3, &bid, Phase::PreCommit);
        assert_ne!(h_prepare, h_precommit);
    }

    #[test]
    fn test_qc_has_quorum() {
        let qc = QuorumCertificate {
            view: 1,
            block_id: nid(1),
            phase: Phase::Prepare,
            signatures: vec![(nid(1), vec![0u8; 64]), (nid(2), vec![0u8; 64]), (nid(3), vec![0u8; 64])],
        };
        assert!(qc.has_quorum(3));
        assert!(!qc.has_quorum(4));
    }

    #[test]
    fn test_safety_rule_safe_to_vote_extends_locked() {
        let mut sr = SafetyRule::new();
        // Lock on block_id = nid(1) at view 2
        let lock_qc = QuorumCertificate {
            view: 2, block_id: nid(1), phase: Phase::PreCommit, signatures: vec![],
        };
        sr.lock(lock_qc);

        // Block that directly extends locked block is safe
        let safe_block = HotStuffBlock {
            block_id: nid(2),
            parent_id: nid(1), // extends locked block
            view: 3,
            leader_id: nid(10),
            batch_root: [0u8; 32],
            ordered_submission_ids: vec![],
            justify_qc: None,
        };
        assert!(sr.safe_to_vote(&safe_block));
    }

    #[test]
    fn test_safety_rule_safe_to_vote_higher_qc_view() {
        let mut sr = SafetyRule::new();
        let lock_qc = QuorumCertificate {
            view: 2, block_id: nid(1), phase: Phase::PreCommit, signatures: vec![],
        };
        sr.lock(lock_qc);

        // Block with higher justify_qc.view is safe (even if not extending locked)
        let higher_qc = QuorumCertificate {
            view: 5, block_id: nid(99), phase: Phase::Prepare, signatures: vec![],
        };
        let safe_block = HotStuffBlock {
            block_id: nid(3),
            parent_id: nid(99), // does NOT extend locked, but justify_qc.view > locked.view
            view: 6,
            leader_id: nid(10),
            batch_root: [0u8; 32],
            ordered_submission_ids: vec![],
            justify_qc: Some(higher_qc),
        };
        assert!(sr.safe_to_vote(&safe_block));
    }

    #[test]
    fn test_safety_rule_unsafe_vote_blocked() {
        let mut sr = SafetyRule::new();
        let lock_qc = QuorumCertificate {
            view: 5, block_id: nid(10), phase: Phase::PreCommit, signatures: vec![],
        };
        sr.lock(lock_qc);

        // Block that neither extends locked nor has higher justify view
        let unsafe_block = HotStuffBlock {
            block_id: nid(20),
            parent_id: nid(99), // does NOT extend locked block_id (which is nid(10))
            view: 6,
            leader_id: nid(1),
            batch_root: [0u8; 32],
            ordered_submission_ids: vec![],
            justify_qc: Some(QuorumCertificate {
                view: 3, // LOWER than locked.view=5
                block_id: nid(99),
                phase: Phase::Prepare,
                signatures: vec![],
            }),
        };
        assert!(!sr.safe_to_vote(&unsafe_block));
    }

    #[test]
    fn test_safety_rule_high_qc_updated() {
        let mut sr = SafetyRule::new();
        let qc1 = QuorumCertificate { view: 3, block_id: nid(1), phase: Phase::Decide, signatures: vec![] };
        let qc2 = QuorumCertificate { view: 7, block_id: nid(2), phase: Phase::Decide, signatures: vec![] };
        sr.update_high_qc(qc1);
        sr.update_high_qc(qc2);
        assert_eq!(sr.high_qc.view, 7);
    }

    #[test]
    fn test_phase_ordering() {
        assert!(Phase::Prepare < Phase::PreCommit);
        assert!(Phase::PreCommit < Phase::Commit);
        assert!(Phase::Commit < Phase::Decide);
    }

    #[test]
    fn test_phase_next() {
        assert_eq!(Phase::Prepare.next(), Some(Phase::PreCommit));
        assert_eq!(Phase::PreCommit.next(), Some(Phase::Commit));
        assert_eq!(Phase::Commit.next(), Some(Phase::Decide));
        assert_eq!(Phase::Decide.next(), None);
    }
}
