//! HotStuff pacemaker: view advancement and leader timeout management.
//!
//! The pacemaker provides liveness in the face of faulty or slow leaders.
//! When a view times out, the pacemaker:
//! 1. Increments the view counter.
//! 2. Sends a `NewView` message carrying `HighQC` to the next leader.
//! 3. Applies exponential backoff to the next view's timeout.
//!
//! # Timeout policy
//! - Base timeout: `base_timeout_ms` (configurable, default 4× slot_duration).
//! - On each consecutive timeout, the timeout doubles up to `max_timeout_ms`.
//! - A successful Decide in the new view resets the backoff to `base_timeout_ms`.

use std::time::{Duration, Instant};

use crate::hotstuff::types::{View, QuorumCertificate, NewViewMessage, SafetyRule};
use crate::networking::messages::NodeId;

// ─── ViewState ────────────────────────────────────────────────────────────────

/// What the node is doing in the current view.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ViewState {
    /// Waiting for a proposal from the leader.
    AwaitingProposal,
    /// Voted on a proposal; waiting for the next phase.
    Voted,
    /// Committed a block; view is complete.
    Decided,
    /// View timed out; NewView was sent.
    TimedOut,
}

// ─── Pacemaker ────────────────────────────────────────────────────────────────

/// Manages view timeouts and triggers view changes.
pub struct Pacemaker {
    pub current_view: View,
    pub state: ViewState,
    /// When the current view started.
    view_started_at: Instant,
    /// Current timeout duration for this view.
    current_timeout: Duration,
    /// Base timeout (reset after successful Decide).
    base_timeout: Duration,
    /// Maximum timeout (backoff cap).
    max_timeout: Duration,
    /// Number of consecutive timeouts without a Decide.
    consecutive_timeouts: u32,
    /// Total view changes since start.
    pub total_view_changes: u64,
}

impl Pacemaker {
    pub fn new(base_timeout_ms: u64) -> Self {
        let base = Duration::from_millis(base_timeout_ms);
        Pacemaker {
            current_view: 1, // views start at 1 (0 = genesis)
            state: ViewState::AwaitingProposal,
            view_started_at: Instant::now(),
            current_timeout: base,
            base_timeout: base,
            max_timeout: base * 32, // max 32× base (5 doublings)
            consecutive_timeouts: 0,
            total_view_changes: 0,
        }
    }

    /// Returns `true` if the current view has timed out.
    pub fn is_timed_out(&self) -> bool {
        self.view_started_at.elapsed() >= self.current_timeout
    }

    /// Time remaining in the current view.
    pub fn time_remaining(&self) -> Duration {
        let elapsed = self.view_started_at.elapsed();
        self.current_timeout.saturating_sub(elapsed)
    }

    /// Advance to the next view (called on timeout or after receiving 2f+1 NewView messages).
    ///
    /// Returns the `NewViewMessage` that should be sent to the next leader.
    pub fn advance_view(&mut self, self_id: NodeId, safety: &SafetyRule) -> NewViewMessage {
        self.current_view += 1;
        self.state = ViewState::AwaitingProposal;
        self.consecutive_timeouts += 1;
        self.total_view_changes += 1;

        // Exponential backoff: double timeout each consecutive timeout, up to max
        let new_timeout = self.current_timeout.saturating_mul(2);
        self.current_timeout = new_timeout.min(self.max_timeout);
        self.view_started_at = Instant::now();

        NewViewMessage {
            new_view: self.current_view,
            high_qc: safety.high_qc.clone(),
            sender_id: self_id,
        }
    }

    /// Reset backoff after a successful Decide.
    pub fn on_decide(&mut self) {
        self.state = ViewState::Decided;
        self.consecutive_timeouts = 0;
        self.current_timeout = self.base_timeout;
    }

    /// Record that we voted in this view.
    pub fn on_voted(&mut self) {
        self.state = ViewState::Voted;
    }

    /// Reset the view timer (called when entering a new view without timeout).
    pub fn reset_view_timer(&mut self, new_view: View) {
        if new_view > self.current_view {
            self.current_view = new_view;
            self.state = ViewState::AwaitingProposal;
            self.view_started_at = Instant::now();
            // Don't reset timeout — keep backoff if we didn't Decide
        }
    }

    /// Select the highest-view QC from a set of NewView messages.
    /// The next leader uses this to form its proposal's `justify_qc`.
    pub fn select_high_qc<'a>(new_views: &'a [NewViewMessage]) -> Option<&'a QuorumCertificate> {
        new_views.iter()
            .map(|nv| &nv.high_qc)
            .max_by_key(|qc| qc.view)
    }

    /// The quorum threshold for NewView messages to trigger a view change:
    /// we need 2f+1 = `threshold` NewViews to safely advance.
    pub fn new_view_quorum_reached(new_views: &[NewViewMessage], threshold: usize) -> bool {
        new_views.len() >= threshold
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::hotstuff::types::Phase;

    fn nid(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_qc(view: u64) -> QuorumCertificate {
        QuorumCertificate { view, block_id: [0u8; 32], phase: Phase::Decide, signatures: vec![] }
    }

    #[test]
    fn test_pacemaker_starts_at_view_1() {
        let pm = Pacemaker::new(1000);
        assert_eq!(pm.current_view, 1);
        assert_eq!(pm.state, ViewState::AwaitingProposal);
    }

    #[test]
    fn test_not_timed_out_immediately() {
        let pm = Pacemaker::new(60_000); // 1-minute timeout
        assert!(!pm.is_timed_out());
    }

    #[test]
    fn test_timed_out_with_zero_timeout() {
        let mut pm = Pacemaker::new(0); // 0ms timeout
        pm.view_started_at = Instant::now() - Duration::from_secs(1); // artificially elapsed
        assert!(pm.is_timed_out());
    }

    #[test]
    fn test_advance_view_increments() {
        let mut pm = Pacemaker::new(500);
        let safety = SafetyRule::new();
        pm.advance_view(nid(1), &safety);
        assert_eq!(pm.current_view, 2);
        assert_eq!(pm.total_view_changes, 1);
    }

    #[test]
    fn test_backoff_doubles_on_timeout() {
        let mut pm = Pacemaker::new(500);
        let base = pm.base_timeout;
        let safety = SafetyRule::new();

        pm.advance_view(nid(1), &safety); // timeout: 1000ms
        assert_eq!(pm.current_timeout, base * 2);

        pm.advance_view(nid(1), &safety); // timeout: 2000ms
        assert_eq!(pm.current_timeout, base * 4);
    }

    #[test]
    fn test_backoff_capped_at_max() {
        let mut pm = Pacemaker::new(100);
        let safety = SafetyRule::new();
        // 5 doublings: 100 → 200 → 400 → 800 → 1600 → 3200 (= 32× base, cap)
        for _ in 0..10 {
            pm.advance_view(nid(1), &safety);
        }
        let max = Duration::from_millis(100) * 32;
        assert_eq!(pm.current_timeout, max);
    }

    #[test]
    fn test_on_decide_resets_backoff() {
        let mut pm = Pacemaker::new(200);
        let safety = SafetyRule::new();
        pm.advance_view(nid(1), &safety);
        pm.advance_view(nid(1), &safety);
        assert!(pm.current_timeout > pm.base_timeout);
        pm.on_decide();
        assert_eq!(pm.current_timeout, pm.base_timeout);
        assert_eq!(pm.consecutive_timeouts, 0);
    }

    #[test]
    fn test_select_high_qc_returns_highest_view() {
        let nv1 = NewViewMessage { new_view: 3, high_qc: make_qc(1), sender_id: nid(1) };
        let nv2 = NewViewMessage { new_view: 3, high_qc: make_qc(5), sender_id: nid(2) };
        let nv3 = NewViewMessage { new_view: 3, high_qc: make_qc(3), sender_id: nid(3) };
        let nvs = [nv1, nv2, nv3];
        let best = Pacemaker::select_high_qc(&nvs).unwrap();
        assert_eq!(best.view, 5);
    }

    #[test]
    fn test_new_view_quorum_reached() {
        let nvs: Vec<NewViewMessage> = (0..3).map(|i| NewViewMessage {
            new_view: 2,
            high_qc: make_qc(1),
            sender_id: nid(i),
        }).collect();
        assert!(Pacemaker::new_view_quorum_reached(&nvs, 3));
        assert!(!Pacemaker::new_view_quorum_reached(&nvs, 4));
    }

    #[test]
    fn test_advance_view_carries_high_qc() {
        let mut pm = Pacemaker::new(500);
        let mut safety = SafetyRule::new();
        safety.update_high_qc(make_qc(7));
        let nv = pm.advance_view(nid(99), &safety);
        assert_eq!(nv.high_qc.view, 7);
        assert_eq!(nv.sender_id, nid(99));
    }
}
