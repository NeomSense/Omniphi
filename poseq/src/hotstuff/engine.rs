//! HotStuff BFT consensus engine.
//!
//! `HotStuffEngine` integrates `SafetyRule`, `Pacemaker`, and vote aggregation
//! into a state machine that drives the PoSeq node through the 3-phase BFT
//! consensus protocol.
//!
//! # Integration with `NetworkedNode`
//!
//! `HotStuffEngine` is NOT async — it processes messages synchronously and
//! returns `HotStuffOutput` describing what the caller should do next (broadcast
//! a QC, advance a view, finalize a block, etc.).  The async transport calls
//! live in `node_runner.rs`.
//!
//! # Phasing
//!
//! Each slot maps to one HotStuff view.  The sequence for a successful slot:
//!
//! ```text
//! Leader      Validators
//!   |                     |
//!   |-- Block(view=v) --> |   (Prepare proposal)
//!   |<-- Vote(PREPARE) ---|   (validators vote PREPARE if safe)
//!   |   [leader aggregates 2f+1 votes → PREPARE-QC]
//!   |-- QC(PREPARE) ----> |   (broadcast PREPARE-QC, advance pacemaker)
//!   |<-- Vote(PRE-COMMIT) |
//!   |   [leader forms PRE-COMMIT-QC; validators lock]
//!   |-- QC(PRE-COMMIT) -> |
//!   |<-- Vote(COMMIT) ----|
//!   |   [leader forms COMMIT-QC]
//!   |-- QC(COMMIT) -----> |   (all nodes Decide = finalize block)
//! ```
//!
//! In the chained variant, each QC serves double duty so the pipeline runs
//! one view per slot.

use std::collections::BTreeMap;

use crate::hotstuff::types::{
    HotStuffBlock, QuorumCertificate, HotStuffVote, NewViewMessage,
    Phase, SafetyRule, View,
};
use crate::hotstuff::pacemaker::{Pacemaker, ViewState};
use crate::networking::messages::NodeId;

// ─── Engine output ─────────────────────────────────────────────────────────────

/// What the engine wants the node to do after processing a message.
#[derive(Debug, Clone)]
pub enum HotStuffOutput {
    /// Nothing to do.
    None,
    /// Broadcast this vote to all peers (and the leader specifically).
    SendVote(HotStuffVote),
    /// Broadcast this QC (leader only — quorum just formed).
    BroadcastQC(QuorumCertificate),
    /// Broadcast a NewView message (view timed out).
    SendNewView(NewViewMessage),
    /// Finalize this block (Decide phase reached).
    Finalize(HotStuffBlock),
    /// Multiple outputs (send all in order).
    Multi(Vec<HotStuffOutput>),
}

impl HotStuffOutput {
    pub fn is_none(&self) -> bool { matches!(self, HotStuffOutput::None) }
}

// ─── HotStuffEngine ───────────────────────────────────────────────────────────

/// Stateful HotStuff BFT consensus engine.
///
/// Holds all per-view voting state and delegates safety/liveness decisions
/// to `SafetyRule` and `Pacemaker`.
pub struct HotStuffEngine {
    pub self_id: NodeId,
    pub quorum_threshold: usize, // 2f+1
    pub safety: SafetyRule,
    pub pacemaker: Pacemaker,

    // ── Per-view state ──────────────────────────────────────────────────────
    /// Pending block for the current view (if received).
    pending_block: Option<HotStuffBlock>,
    /// Votes collected per (view, phase) from distinct voters.
    votes: BTreeMap<(View, Phase), Vec<HotStuffVote>>,
    /// NewView messages received for the current view.
    new_views: Vec<NewViewMessage>,
    /// Whether we've already voted in the current view+phase.
    voted: BTreeMap<(View, Phase), bool>,
    /// Blocks we have decided (to prevent duplicate Decide outputs).
    decided_blocks: BTreeMap<View, [u8; 32]>,
}

impl HotStuffEngine {
    pub fn new(self_id: NodeId, quorum_threshold: usize, base_timeout_ms: u64) -> Self {
        HotStuffEngine {
            self_id,
            quorum_threshold,
            safety: SafetyRule::new(),
            pacemaker: Pacemaker::new(base_timeout_ms),
            pending_block: None,
            votes: BTreeMap::new(),
            new_views: Vec::new(),
            voted: BTreeMap::new(),
            decided_blocks: BTreeMap::new(),
        }
    }

    // ─── Incoming message handlers ─────────────────────────────────────────

    /// Process a block proposal from the leader.
    ///
    /// If safe to vote, returns `SendVote(PREPARE)`.
    pub fn on_block(&mut self, block: HotStuffBlock) -> HotStuffOutput {
        let view = block.view;

        // Ignore old views
        if view < self.pacemaker.current_view {
            return HotStuffOutput::None;
        }

        // Update HighQC from block's justify_qc
        if let Some(ref qc) = block.justify_qc.clone() {
            self.safety.update_high_qc(qc.clone());
        }

        // Advance pacemaker to this view if needed
        self.pacemaker.reset_view_timer(view);
        self.safety.advance_view(view);

        // Safety check
        if !self.safety.safe_to_vote(&block) {
            return HotStuffOutput::None;
        }

        self.pending_block = Some(block.clone());

        // Cast PREPARE vote if not already voted
        let key = (view, Phase::Prepare);
        if self.voted.contains_key(&key) {
            return HotStuffOutput::None;
        }
        self.voted.insert(key, true);
        self.pacemaker.on_voted();

        // Produce a vote (real signature is a placeholder here — signing key
        // is in the caller; the caller fills in the real signature).
        let vote_hash = QuorumCertificate::vote_hash(view, &block.block_id, Phase::Prepare);
        HotStuffOutput::SendVote(HotStuffVote {
            view,
            block_id: block.block_id,
            phase: Phase::Prepare,
            voter_id: self.self_id,
            signature: placeholder_sig(&vote_hash),
        })
    }

    /// Process a vote received from a validator.
    ///
    /// If 2f+1 votes are collected, returns `BroadcastQC` (leader duty).
    /// If it's a COMMIT-QC being formed → also returns `Finalize`.
    pub fn on_vote(&mut self, vote: HotStuffVote) -> HotStuffOutput {
        let view = vote.view;
        let phase = vote.phase;

        // Only the leader aggregates votes (but we store anyway for simplicity)
        let key = (view, phase);
        let entry = self.votes.entry(key).or_default();

        // Dedup by voter_id
        if entry.iter().any(|v| v.voter_id == vote.voter_id) {
            return HotStuffOutput::None;
        }
        entry.push(vote.clone());

        let approvals = entry.len();
        if approvals < self.quorum_threshold {
            return HotStuffOutput::None;
        }

        // Quorum reached — form QC
        let sigs: Vec<(NodeId, Vec<u8>)> = entry.iter()
            .map(|v| (v.voter_id, v.signature.clone()))
            .collect();
        let qc = QuorumCertificate {
            view,
            block_id: vote.block_id,
            phase,
            signatures: sigs,
        };

        // Update safety state based on phase
        match phase {
            Phase::Prepare => {
                // PREPARE-QC formed → broadcast it; validators will send PRE-COMMIT votes
                self.safety.update_high_qc(qc.clone());
                HotStuffOutput::BroadcastQC(qc)
            }
            Phase::PreCommit => {
                // PRE-COMMIT-QC formed → lock on this block; broadcast for COMMIT votes
                self.safety.lock(qc.clone());
                HotStuffOutput::BroadcastQC(qc)
            }
            Phase::Commit => {
                // COMMIT-QC formed → block is decided (finalized)
                let block = match self.pending_block.clone() {
                    Some(b) if b.block_id == vote.block_id => b,
                    _ => return HotStuffOutput::BroadcastQC(qc), // missing block — just forward QC
                };
                if self.decided_blocks.contains_key(&view) {
                    return HotStuffOutput::None;
                }
                self.decided_blocks.insert(view, block.block_id);
                self.pacemaker.on_decide();
                HotStuffOutput::Multi(vec![
                    HotStuffOutput::BroadcastQC(qc),
                    HotStuffOutput::Finalize(block),
                ])
            }
            Phase::Decide => HotStuffOutput::None,
        }
    }

    /// Process a received `QuorumCertificate` broadcast by the leader.
    ///
    /// Validators respond with the next-phase vote.
    pub fn on_qc(&mut self, qc: QuorumCertificate) -> HotStuffOutput {
        let view = qc.view;
        let next_phase = match qc.phase.next() {
            Some(p) => p,
            None => return HotStuffOutput::None,
        };

        self.safety.update_high_qc(qc.clone());

        // Lock when receiving PRE-COMMIT-QC
        if qc.phase == Phase::PreCommit {
            self.safety.lock(qc.clone());
        }

        // Check if we've already voted in the next phase
        let key = (view, next_phase);
        if self.voted.contains_key(&key) {
            return HotStuffOutput::None;
        }

        // For COMMIT-QC → Decide: finalize directly (non-leader path)
        if qc.phase == Phase::Commit {
            if let Some(block) = self.pending_block.clone() {
                if block.block_id == qc.block_id {
                    if !self.decided_blocks.contains_key(&view) {
                        self.decided_blocks.insert(view, block.block_id);
                        self.pacemaker.on_decide();
                        return HotStuffOutput::Finalize(block);
                    }
                }
            }
            return HotStuffOutput::None;
        }

        self.voted.insert(key, true);

        let vote_hash = QuorumCertificate::vote_hash(view, &qc.block_id, next_phase);
        HotStuffOutput::SendVote(HotStuffVote {
            view,
            block_id: qc.block_id,
            phase: next_phase,
            voter_id: self.self_id,
            signature: placeholder_sig(&vote_hash),
        })
    }

    /// Process a `NewView` message (pacemaker liveness trigger).
    ///
    /// When 2f+1 NewViews are collected, the node advances to the new view.
    /// Returns `None` until threshold is reached; then returns next action.
    pub fn on_new_view(&mut self, nv: NewViewMessage) -> HotStuffOutput {
        let new_view = nv.new_view;

        // Dedup
        if self.new_views.iter().any(|n| n.sender_id == nv.sender_id) {
            return HotStuffOutput::None;
        }
        self.new_views.push(nv);

        // Update HighQC from received NewViews
        if let Some(best) = Pacemaker::select_high_qc(&self.new_views) {
            self.safety.update_high_qc(best.clone());
        }

        if !Pacemaker::new_view_quorum_reached(&self.new_views, self.quorum_threshold) {
            return HotStuffOutput::None;
        }

        // Advance to the new view
        self.pacemaker.reset_view_timer(new_view);
        self.safety.advance_view(new_view);
        self.clear_view_state();

        // If we are the leader for the new view, we would propose a block.
        // The caller is responsible for checking leadership and calling propose().
        HotStuffOutput::None
    }

    /// Called by the node runner when a view timeout is detected.
    ///
    /// Returns the `SendNewView` action to broadcast to peers.
    pub fn on_timeout(&mut self) -> HotStuffOutput {
        let nv = self.pacemaker.advance_view(self.self_id, &self.safety);
        self.clear_view_state();
        HotStuffOutput::SendNewView(nv)
    }

    // ─── Leader duties ─────────────────────────────────────────────────────

    /// Create a new block for the current view (leader only).
    ///
    /// Callers should check `is_leader_for_view(current_view)` before calling.
    pub fn propose(
        &mut self,
        parent_block_id: [u8; 32],
        batch_root: [u8; 32],
        ordered_submission_ids: Vec<[u8; 32]>,
    ) -> HotStuffBlock {
        let view = self.pacemaker.current_view;
        let block_id = HotStuffBlock::compute_id(&parent_block_id, &batch_root, view, &self.self_id);
        let block = HotStuffBlock {
            block_id,
            parent_id: parent_block_id,
            view,
            leader_id: self.self_id,
            batch_root,
            ordered_submission_ids,
            justify_qc: Some(self.safety.high_qc.clone()),
        };
        self.pending_block = Some(block.clone());
        block
    }

    // ─── Helpers ───────────────────────────────────────────────────────────

    /// Clear per-view state (votes, pending block) for a fresh view.
    fn clear_view_state(&mut self) {
        let v = self.pacemaker.current_view;
        // Keep only state for current and future views
        self.votes.retain(|(view, _), _| *view >= v);
        self.voted.retain(|(view, _), _| *view >= v);
        self.new_views.clear();
        if self.pending_block.as_ref().map(|b| b.view) < Some(v) {
            self.pending_block = None;
        }
    }

    pub fn current_view(&self) -> View {
        self.pacemaker.current_view
    }

    pub fn is_timed_out(&self) -> bool {
        self.pacemaker.is_timed_out()
    }
}

// ─── Placeholder signature ────────────────────────────────────────────────────

/// In the engine layer, signatures are computed from the vote_hash as a placeholder
/// (zeros XOR first 8 bytes of hash to distinguish from all-zeros).
/// Real Ed25519 signatures are applied by the node_runner which holds the key pair.
fn placeholder_sig(vote_hash: &[u8; 32]) -> Vec<u8> {
    let mut sig = vec![0u8; 64];
    sig[..32].copy_from_slice(vote_hash);
    sig
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn nid(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_engine(self_id: u8, quorum: usize) -> HotStuffEngine {
        HotStuffEngine::new(nid(self_id), quorum, 1000)
    }

    fn genesis_block(leader: u8, view: View) -> HotStuffBlock {
        let parent = [0u8; 32];
        let root = nid(42);
        let bid = HotStuffBlock::compute_id(&parent, &root, view, &nid(leader));
        HotStuffBlock {
            block_id: bid,
            parent_id: parent,
            view,
            leader_id: nid(leader),
            batch_root: root,
            ordered_submission_ids: vec![],
            justify_qc: None,
        }
    }

    #[test]
    fn test_on_block_returns_prepare_vote() {
        let mut engine = make_engine(2, 2);
        let block = genesis_block(1, 1);
        let block_id = block.block_id;
        let output = engine.on_block(block);
        match output {
            HotStuffOutput::SendVote(v) => {
                assert_eq!(v.phase, Phase::Prepare);
                assert_eq!(v.block_id, block_id);
                assert_eq!(v.view, 1);
            }
            other => panic!("expected SendVote, got {other:?}"),
        }
    }

    #[test]
    fn test_on_block_dedup_no_double_vote() {
        let mut engine = make_engine(2, 2);
        let block = genesis_block(1, 1);
        let _ = engine.on_block(block.clone());
        let output2 = engine.on_block(block);
        assert!(output2.is_none(), "should not vote twice");
    }

    #[test]
    fn test_quorum_prepare_votes_forms_qc() {
        let mut engine = make_engine(1, 2); // leader, quorum=2
        let block = genesis_block(1, 1);
        let block_id = block.block_id;

        // Leader proposes, gets its own vote
        engine.on_block(block.clone());

        // Vote from node 2
        let vote2 = HotStuffVote {
            view: 1, block_id, phase: Phase::Prepare, voter_id: nid(2), signature: vec![0u8; 64],
        };
        // Vote from node 1 (self) to reach quorum=2
        let vote1 = HotStuffVote {
            view: 1, block_id, phase: Phase::Prepare, voter_id: nid(1), signature: vec![0u8; 64],
        };

        let out1 = engine.on_vote(vote1);
        assert!(out1.is_none()); // only 1 vote
        let out2 = engine.on_vote(vote2);
        match out2 {
            HotStuffOutput::BroadcastQC(qc) => {
                assert_eq!(qc.phase, Phase::Prepare);
                assert_eq!(qc.view, 1);
                assert!(qc.has_quorum(2));
            }
            other => panic!("expected BroadcastQC, got {other:?}"),
        }
    }

    #[test]
    fn test_on_qc_prepare_sends_precommit_vote() {
        let mut engine = make_engine(2, 2);
        let block = genesis_block(1, 1);
        let block_id = block.block_id;
        engine.on_block(block);

        let prepare_qc = QuorumCertificate {
            view: 1,
            block_id,
            phase: Phase::Prepare,
            signatures: vec![(nid(1), vec![0u8; 64]), (nid(3), vec![0u8; 64])],
        };
        let out = engine.on_qc(prepare_qc);
        match out {
            HotStuffOutput::SendVote(v) => {
                assert_eq!(v.phase, Phase::PreCommit);
            }
            other => panic!("expected PreCommit vote, got {other:?}"),
        }
    }

    #[test]
    fn test_commit_qc_produces_finalize() {
        let mut engine = make_engine(2, 2);
        let block = genesis_block(1, 1);
        let block_id = block.block_id;
        engine.on_block(block);

        // Simulate receiving COMMIT-QC
        let commit_qc = QuorumCertificate {
            view: 1,
            block_id,
            phase: Phase::Commit,
            signatures: vec![(nid(1), vec![0u8; 64]), (nid(3), vec![0u8; 64])],
        };
        let out = engine.on_qc(commit_qc);
        match out {
            HotStuffOutput::Finalize(b) => {
                assert_eq!(b.block_id, block_id);
            }
            other => panic!("expected Finalize, got {other:?}"),
        }
    }

    #[test]
    fn test_on_timeout_sends_new_view() {
        let mut engine = make_engine(1, 2);
        let out = engine.on_timeout();
        match out {
            HotStuffOutput::SendNewView(nv) => {
                assert_eq!(nv.new_view, 2); // was at view 1, now 2
                assert_eq!(nv.sender_id, nid(1));
            }
            other => panic!("expected SendNewView, got {other:?}"),
        }
    }

    #[test]
    fn test_propose_creates_block_at_current_view() {
        let mut engine = make_engine(1, 2);
        let block = engine.propose([0u8; 32], nid(10), vec![]);
        assert_eq!(block.view, engine.current_view());
        assert_eq!(block.leader_id, nid(1));
    }

    #[test]
    fn test_new_view_quorum_advances_view() {
        let mut engine = make_engine(1, 2);
        let nv1 = NewViewMessage {
            new_view: 2,
            high_qc: QuorumCertificate::genesis(),
            sender_id: nid(1),
        };
        let nv2 = NewViewMessage {
            new_view: 2,
            high_qc: QuorumCertificate::genesis(),
            sender_id: nid(2),
        };
        let out1 = engine.on_new_view(nv1);
        assert!(out1.is_none()); // only 1 new-view
        engine.on_new_view(nv2); // 2nd new-view reaches quorum=2
        assert_eq!(engine.current_view(), 2);
    }
}
