//! Deterministic Safety Engine — Scoped Protocol-Level Defense
//!
//! Provides runtime-level anomaly detection and scoped containment that:
//! - Tracks per-epoch outflows and detects abnormal drains
//! - Counts repeated execution failures per path and auto-blocks
//! - Validates balance conservation invariants after settlement
//! - Freezes only affected objects/paths (no global chain halt)
//! - Emits deterministic safety events for audit
//!
//! ## Institutional Safety Benefits
//!
//! Traditional blockchains have two modes: running or halted. Omniphi adds
//! granular containment — an exploit in one AMM pool doesn't freeze the
//! entire chain or even other pools. This is the same principle used in
//! aircraft (isolate failed engine, keep flying) and nuclear reactors
//! (SCRAM individual rods, not the whole plant).
//!
//! Safety decisions are fully deterministic: same execution results +
//! same engine state = same safety response. No admin keys, no manual
//! override in the core logic. Governance override is a separate hook
//! (not implemented here).

use crate::objects::base::ObjectId;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

// ─────────────────────────────────────────────────────────────────────────────
// Safety Events
// ─────────────────────────────────────────────────────────────────────────────

/// A deterministic safety event emitted by the engine.
#[derive(Debug, Clone)]
pub struct SafetyEvent {
    pub event_id: [u8; 32],
    pub epoch: u64,
    pub trigger: SafetyTrigger,
    pub action: SafetyAction,
    pub affected_objects: Vec<ObjectId>,
    pub affected_paths: Vec<String>,
    pub detail: String,
}

/// What triggered the safety response.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SafetyTrigger {
    /// Total outflow in an epoch exceeds threshold.
    AbnormalOutflow { epoch: u64, outflow: u128, threshold: u128 },
    /// A specific object drained faster than allowed.
    ObjectDrain { object_id: ObjectId, drained: u128, threshold: u128 },
    /// Balance invariant violated (inflow != outflow after settlement).
    InvariantViolation { expected_sum: u128, actual_sum: u128 },
    /// Execution path failed more than N times in a window.
    RepeatedFailure { path: String, count: u32, threshold: u32 },
    /// State hash mismatch after settlement (state divergence).
    StateDivergence { expected_hash: [u8; 32], actual_hash: [u8; 32] },
    /// Oracle/dependency input is stale or invalid.
    InvalidDependency { source: String, reason: String },
}

/// What the engine does in response.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SafetyAction {
    /// Freeze specific objects (they cannot be read or written).
    FreezeObjects(Vec<ObjectId>),
    /// Block a specific execution path (intent type + target combination).
    BlockPath(String),
    /// Mark objects with a risk level (advisory, does not block).
    MarkRisk(Vec<ObjectId>, RiskLevel),
    /// Emit an alert event (informational only).
    Alert(String),
}

/// Risk level for marked objects.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum RiskLevel {
    Low,
    Medium,
    High,
    Critical,
}

// ─────────────────────────────────────────────────────────────────────────────
// Safety Configuration
// ─────────────────────────────────────────────────────────────────────────────

/// Configuration for safety thresholds. All values are deterministic and
/// set at genesis or via governance (not modifiable at runtime).
#[derive(Debug, Clone)]
pub struct SafetyConfig {
    /// Maximum total outflow per epoch before triggering AbnormalOutflow.
    pub max_epoch_outflow: u128,
    /// Maximum outflow per individual object per epoch.
    pub max_object_outflow: u128,
    /// Number of failures in a path before auto-blocking.
    pub failure_threshold: u32,
    /// Number of epochs to track failures (sliding window).
    pub failure_window_epochs: u64,
    /// Maximum allowed invariant deviation (0 = exact).
    pub invariant_epsilon: u128,
}

impl Default for SafetyConfig {
    fn default() -> Self {
        SafetyConfig {
            max_epoch_outflow: 10_000_000_000_000, // 10T units
            max_object_outflow: 1_000_000_000_000,  // 1T per object
            failure_threshold: 5,
            failure_window_epochs: 10,
            invariant_epsilon: 0,
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Deterministic Safety Engine
// ─────────────────────────────────────────────────────────────────────────────

/// The core safety engine. All state transitions are deterministic.
#[derive(Debug, Clone)]
pub struct DeterministicSafetyEngine {
    config: SafetyConfig,

    /// Frozen objects — cannot be accessed until unfrozen.
    frozen_objects: BTreeSet<ObjectId>,
    /// Blocked execution paths.
    blocked_paths: BTreeSet<String>,
    /// Risk markings on objects.
    risk_marks: BTreeMap<ObjectId, RiskLevel>,

    /// Per-epoch outflow tracking: epoch → total outflow.
    epoch_outflows: BTreeMap<u64, u128>,
    /// Per-object outflow tracking: (epoch, object_id) → outflow.
    object_outflows: BTreeMap<(u64, ObjectId), u128>,
    /// Failure counts: (path, epoch) → count.
    failure_counts: BTreeMap<(String, u64), u32>,

    /// Emitted safety events (append-only log).
    events: Vec<SafetyEvent>,
    /// Next event sequence number.
    event_seq: u64,
}

impl DeterministicSafetyEngine {
    pub fn new(config: SafetyConfig) -> Self {
        DeterministicSafetyEngine {
            config,
            frozen_objects: BTreeSet::new(),
            blocked_paths: BTreeSet::new(),
            risk_marks: BTreeMap::new(),
            epoch_outflows: BTreeMap::new(),
            object_outflows: BTreeMap::new(),
            failure_counts: BTreeMap::new(),
            events: Vec::new(),
            event_seq: 0,
        }
    }

    // ── Query Methods ────────────────────────────────────────

    /// Check if an object is frozen.
    pub fn is_frozen(&self, object_id: &ObjectId) -> bool {
        self.frozen_objects.contains(object_id)
    }

    /// Check if a path is blocked.
    pub fn is_path_blocked(&self, path: &str) -> bool {
        self.blocked_paths.contains(path)
    }

    /// Get the risk level for an object (None = no risk marking).
    pub fn risk_level(&self, object_id: &ObjectId) -> Option<RiskLevel> {
        self.risk_marks.get(object_id).copied()
    }

    /// Get all frozen objects.
    pub fn frozen_objects(&self) -> &BTreeSet<ObjectId> {
        &self.frozen_objects
    }

    /// Get all blocked paths.
    pub fn blocked_paths(&self) -> &BTreeSet<String> {
        &self.blocked_paths
    }

    /// Get all safety events.
    pub fn events(&self) -> &[SafetyEvent] {
        &self.events
    }

    /// Number of safety events emitted.
    pub fn event_count(&self) -> usize {
        self.events.len()
    }

    // ── Trigger Detection ────────────────────────────────────

    /// Record an outflow and check for abnormal levels.
    pub fn record_outflow(
        &mut self,
        epoch: u64,
        object_id: ObjectId,
        amount: u128,
    ) -> Vec<SafetyEvent> {
        let mut triggered = vec![];

        // Update epoch total
        let epoch_total = self.epoch_outflows.entry(epoch).or_insert(0);
        *epoch_total = epoch_total.saturating_add(amount);
        let epoch_total_val = *epoch_total;

        // Check epoch threshold
        if epoch_total_val > self.config.max_epoch_outflow {
            let event = self.emit(epoch, SafetyTrigger::AbnormalOutflow {
                epoch,
                outflow: epoch_total_val,
                threshold: self.config.max_epoch_outflow,
            }, SafetyAction::Alert(format!(
                "Epoch {} outflow {} exceeds threshold {}", epoch, epoch_total_val, self.config.max_epoch_outflow
            )), vec![], vec![]);
            triggered.push(event);
        }

        // Update per-object total
        let obj_total = self.object_outflows.entry((epoch, object_id)).or_insert(0);
        *obj_total = obj_total.saturating_add(amount);
        let obj_total_val = *obj_total;

        // Check per-object threshold
        if obj_total_val > self.config.max_object_outflow {
            let event = self.emit(epoch, SafetyTrigger::ObjectDrain {
                object_id,
                drained: obj_total_val,
                threshold: self.config.max_object_outflow,
            }, SafetyAction::FreezeObjects(vec![object_id]),
                vec![object_id], vec![]);

            self.frozen_objects.insert(object_id);
            triggered.push(event);
        }

        triggered
    }

    /// Check a balance invariant after settlement.
    /// Returns a safety event if the invariant is violated.
    pub fn check_invariant(
        &mut self,
        epoch: u64,
        expected_sum: u128,
        actual_sum: u128,
        affected_objects: Vec<ObjectId>,
    ) -> Option<SafetyEvent> {
        let diff = if expected_sum > actual_sum {
            expected_sum - actual_sum
        } else {
            actual_sum - expected_sum
        };

        if diff > self.config.invariant_epsilon {
            // Freeze all affected objects
            for obj in &affected_objects {
                self.frozen_objects.insert(*obj);
            }

            let event = self.emit(epoch, SafetyTrigger::InvariantViolation {
                expected_sum,
                actual_sum,
            }, SafetyAction::FreezeObjects(affected_objects.clone()),
                affected_objects, vec![]);

            Some(event)
        } else {
            None
        }
    }

    /// Record an execution failure on a path.
    /// Auto-blocks the path after `failure_threshold` failures in the window.
    pub fn record_failure(
        &mut self,
        epoch: u64,
        path: &str,
    ) -> Option<SafetyEvent> {
        let key = (path.to_string(), epoch);
        let count = self.failure_counts.entry(key).or_insert(0);
        *count += 1;

        // Sum failures across the window
        let window_start = epoch.saturating_sub(self.config.failure_window_epochs);
        let total: u32 = self.failure_counts.iter()
            .filter(|((p, e), _)| p == path && *e >= window_start && *e <= epoch)
            .map(|(_, c)| c)
            .sum();

        if total >= self.config.failure_threshold {
            self.blocked_paths.insert(path.to_string());

            let event = self.emit(epoch, SafetyTrigger::RepeatedFailure {
                path: path.to_string(),
                count: total,
                threshold: self.config.failure_threshold,
            }, SafetyAction::BlockPath(path.to_string()),
                vec![], vec![path.to_string()]);

            Some(event)
        } else {
            None
        }
    }

    /// Report a state divergence (settlement produced unexpected hash).
    pub fn report_state_divergence(
        &mut self,
        epoch: u64,
        expected_hash: [u8; 32],
        actual_hash: [u8; 32],
        affected_objects: Vec<ObjectId>,
    ) -> SafetyEvent {
        for obj in &affected_objects {
            self.frozen_objects.insert(*obj);
            self.risk_marks.insert(*obj, RiskLevel::Critical);
        }

        self.emit(epoch, SafetyTrigger::StateDivergence {
            expected_hash,
            actual_hash,
        }, SafetyAction::FreezeObjects(affected_objects.clone()),
            affected_objects, vec![])
    }

    /// Report invalid dependency/oracle input.
    pub fn report_invalid_dependency(
        &mut self,
        epoch: u64,
        source: &str,
        reason: &str,
        affected_paths: Vec<String>,
    ) -> SafetyEvent {
        for path in &affected_paths {
            self.blocked_paths.insert(path.clone());
        }

        self.emit(epoch, SafetyTrigger::InvalidDependency {
            source: source.to_string(),
            reason: reason.to_string(),
        }, SafetyAction::BlockPath(affected_paths.first().cloned().unwrap_or_default()),
            vec![], affected_paths)
    }

    /// Mark objects with a risk level (advisory).
    pub fn mark_risk(&mut self, objects: &[ObjectId], level: RiskLevel) {
        for obj in objects {
            self.risk_marks.insert(*obj, level);
        }
    }

    // ── Unfreeze (for future governance hook) ────────────────

    /// Unfreeze a specific object. This is the governance hook boundary —
    /// the engine itself never unfreezes; only an external authority can.
    pub fn unfreeze_object(&mut self, object_id: &ObjectId) -> bool {
        self.frozen_objects.remove(object_id)
    }

    /// Unblock a specific path.
    pub fn unblock_path(&mut self, path: &str) -> bool {
        self.blocked_paths.remove(path)
    }

    /// Clear a risk marking.
    pub fn clear_risk(&mut self, object_id: &ObjectId) -> bool {
        self.risk_marks.remove(object_id).is_some()
    }

    // ── Maintenance ──────────────────────────────────────────

    /// Prune tracking data for epochs older than `before_epoch`.
    pub fn prune(&mut self, before_epoch: u64) {
        self.epoch_outflows.retain(|&e, _| e >= before_epoch);
        self.object_outflows.retain(|&(e, _), _| e >= before_epoch);
        self.failure_counts.retain(|&(_, e), _| e >= before_epoch);
    }

    // ── Internal ─────────────────────────────────────────────

    fn emit(
        &mut self,
        epoch: u64,
        trigger: SafetyTrigger,
        action: SafetyAction,
        affected_objects: Vec<ObjectId>,
        affected_paths: Vec<String>,
    ) -> SafetyEvent {
        let event_id = Self::compute_event_id(self.event_seq, epoch);
        self.event_seq += 1;

        let detail = format!("{:?} → {:?}", trigger, action);
        let event = SafetyEvent {
            event_id,
            epoch,
            trigger,
            action,
            affected_objects,
            affected_paths,
            detail,
        };

        self.events.push(event.clone());
        event
    }

    fn compute_event_id(seq: u64, epoch: u64) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_SAFETY_EVENT_V1");
        h.update(&seq.to_be_bytes());
        h.update(&epoch.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn oid(v: u8) -> ObjectId { ObjectId({ let mut b = [0u8; 32]; b[0] = v; b }) }

    fn small_config() -> SafetyConfig {
        SafetyConfig {
            max_epoch_outflow: 10_000,
            max_object_outflow: 5_000,
            failure_threshold: 3,
            failure_window_epochs: 5,
            invariant_epsilon: 0,
        }
    }

    // ── Test 1: abnormal outflow trigger ─────────────────────

    #[test]
    fn test_abnormal_epoch_outflow() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        // Under threshold — no trigger
        let events = engine.record_outflow(1, oid(1), 3000);
        assert!(events.is_empty());

        // Push epoch over threshold (3000 + 8000 = 11000 > 10000)
        // Also triggers object drain (8000 > 5000) — so 2 events
        let events = engine.record_outflow(1, oid(2), 8000);
        assert!(events.len() >= 1);
        assert!(events.iter().any(|e| matches!(e.trigger, SafetyTrigger::AbnormalOutflow { .. })));
    }

    // ── Test 2: object drain trigger + freeze ────────────────

    #[test]
    fn test_object_drain_freeze() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.record_outflow(1, oid(1), 3000);
        assert!(!engine.is_frozen(&oid(1)));

        // Push object over its limit
        let events = engine.record_outflow(1, oid(1), 3000);
        assert_eq!(events.len(), 1);
        assert!(matches!(events[0].trigger, SafetyTrigger::ObjectDrain { .. }));
        assert!(engine.is_frozen(&oid(1)));
    }

    // ── Test 3: invariant violation trigger ───────────────────

    #[test]
    fn test_invariant_violation() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        // No violation (exact match)
        let event = engine.check_invariant(1, 1000, 1000, vec![oid(1)]);
        assert!(event.is_none());

        // Violation detected
        let event = engine.check_invariant(1, 1000, 900, vec![oid(1), oid(2)]);
        assert!(event.is_some());
        assert!(matches!(event.unwrap().trigger, SafetyTrigger::InvariantViolation { .. }));

        // Affected objects are frozen
        assert!(engine.is_frozen(&oid(1)));
        assert!(engine.is_frozen(&oid(2)));
    }

    // ── Test 4: scoped freeze — unaffected objects remain ────

    #[test]
    fn test_scoped_freeze_unaffected_objects_ok() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        // Freeze object 1 via drain
        engine.record_outflow(1, oid(1), 6000);
        assert!(engine.is_frozen(&oid(1)));

        // Object 2 is NOT frozen
        assert!(!engine.is_frozen(&oid(2)));

        // Object 3 is NOT frozen
        assert!(!engine.is_frozen(&oid(3)));
    }

    // ── Test 5: repeated failure → path blocked ──────────────

    #[test]
    fn test_repeated_failure_blocks_path() {
        let mut engine = DeterministicSafetyEngine::new(small_config());
        let path = "swap:pool_0xAA";

        // 2 failures — under threshold
        assert!(engine.record_failure(1, path).is_none());
        assert!(engine.record_failure(1, path).is_none());
        assert!(!engine.is_path_blocked(path));

        // 3rd failure — threshold reached
        let event = engine.record_failure(1, path);
        assert!(event.is_some());
        assert!(engine.is_path_blocked(path));
    }

    // ── Test 6: different paths are independent ──────────────

    #[test]
    fn test_different_paths_independent() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        // Fail path A 3 times → blocked
        engine.record_failure(1, "path_a");
        engine.record_failure(1, "path_a");
        engine.record_failure(1, "path_a");
        assert!(engine.is_path_blocked("path_a"));

        // Path B is NOT blocked
        assert!(!engine.is_path_blocked("path_b"));
    }

    // ── Test 7: deterministic trigger reproducibility ─────────

    #[test]
    fn test_deterministic_reproducibility() {
        let config = small_config();

        let mut engine1 = DeterministicSafetyEngine::new(config.clone());
        let mut engine2 = DeterministicSafetyEngine::new(config);

        // Same operations on both
        engine1.record_outflow(1, oid(1), 6000);
        engine2.record_outflow(1, oid(1), 6000);

        assert_eq!(engine1.is_frozen(&oid(1)), engine2.is_frozen(&oid(1)));
        assert_eq!(engine1.event_count(), engine2.event_count());
        assert_eq!(engine1.events()[0].event_id, engine2.events()[0].event_id);
    }

    // ── Test 8: state divergence → freeze + critical risk ────

    #[test]
    fn test_state_divergence() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        let event = engine.report_state_divergence(
            1, [0xAA; 32], [0xBB; 32], vec![oid(1), oid(2)],
        );

        assert!(matches!(event.trigger, SafetyTrigger::StateDivergence { .. }));
        assert!(engine.is_frozen(&oid(1)));
        assert!(engine.is_frozen(&oid(2)));
        assert_eq!(engine.risk_level(&oid(1)), Some(RiskLevel::Critical));
    }

    // ── Test 9: invalid dependency → block paths ─────────────

    #[test]
    fn test_invalid_dependency() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        let event = engine.report_invalid_dependency(
            1, "oracle_price_feed", "stale data (>5 epochs old)",
            vec!["swap:pool_usdc".into(), "swap:pool_eth".into()],
        );

        assert!(matches!(event.trigger, SafetyTrigger::InvalidDependency { .. }));
        assert!(engine.is_path_blocked("swap:pool_usdc"));
        assert!(engine.is_path_blocked("swap:pool_eth"));
    }

    // ── Test 10: risk marking (advisory) ─────────────────────

    #[test]
    fn test_risk_marking() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.mark_risk(&[oid(1), oid(2)], RiskLevel::Medium);
        assert_eq!(engine.risk_level(&oid(1)), Some(RiskLevel::Medium));
        assert_eq!(engine.risk_level(&oid(2)), Some(RiskLevel::Medium));
        assert_eq!(engine.risk_level(&oid(3)), None);

        // Clear
        engine.clear_risk(&oid(1));
        assert_eq!(engine.risk_level(&oid(1)), None);
    }

    // ── Test 11: unfreeze (governance hook boundary) ─────────

    #[test]
    fn test_unfreeze_governance_hook() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.record_outflow(1, oid(1), 6000);
        assert!(engine.is_frozen(&oid(1)));

        // Governance unfreezes
        assert!(engine.unfreeze_object(&oid(1)));
        assert!(!engine.is_frozen(&oid(1)));

        // Double unfreeze returns false
        assert!(!engine.unfreeze_object(&oid(1)));
    }

    // ── Test 12: unblock path ────────────────────────────────

    #[test]
    fn test_unblock_path() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.record_failure(1, "bad_path");
        engine.record_failure(1, "bad_path");
        engine.record_failure(1, "bad_path");
        assert!(engine.is_path_blocked("bad_path"));

        assert!(engine.unblock_path("bad_path"));
        assert!(!engine.is_path_blocked("bad_path"));
    }

    // ── Test 13: prune old epochs ────────────────────────────

    #[test]
    fn test_prune() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.record_outflow(1, oid(1), 100);
        engine.record_outflow(5, oid(2), 200);
        engine.record_outflow(10, oid(3), 300);

        engine.prune(5);

        // Epoch 1 data should be gone — recording to epoch 1 starts fresh
        // But frozen state persists (it's not epoch-scoped)
    }

    // ── Test 14: failure window sliding ──────────────────────

    #[test]
    fn test_failure_window_sliding() {
        let mut engine = DeterministicSafetyEngine::new(small_config());
        // Window is 5 epochs

        engine.record_failure(1, "path_x"); // epoch 1
        engine.record_failure(2, "path_x"); // epoch 2

        // At epoch 10 (epoch 1 and 2 are outside window [5, 10])
        // Only 1 failure in the window
        let event = engine.record_failure(10, "path_x");
        assert!(event.is_none()); // Only 1 in window, threshold is 3
    }

    // ── Test 15: invariant with epsilon tolerance ─────────────

    #[test]
    fn test_invariant_epsilon_tolerance() {
        let mut config = small_config();
        config.invariant_epsilon = 10; // allow up to 10 units deviation

        let mut engine = DeterministicSafetyEngine::new(config);

        // Within tolerance
        let event = engine.check_invariant(1, 1000, 995, vec![oid(1)]);
        assert!(event.is_none()); // 5 < 10 epsilon

        // Outside tolerance
        let event = engine.check_invariant(1, 1000, 980, vec![oid(1)]);
        assert!(event.is_some()); // 20 > 10 epsilon
    }

    // ── Test 16: event log is append-only ────────────────────

    #[test]
    fn test_event_log_append_only() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        engine.record_outflow(1, oid(1), 6000); // triggers drain
        assert_eq!(engine.event_count(), 1);

        engine.record_outflow(2, oid(2), 6000); // triggers another drain
        assert_eq!(engine.event_count(), 2);

        // Events are in order
        assert_eq!(engine.events()[0].epoch, 1);
        assert_eq!(engine.events()[1].epoch, 2);
    }

    // ── Test 17: combined outflow — epoch + object triggers ──

    #[test]
    fn test_combined_triggers() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        // Single outflow that exceeds BOTH epoch and object thresholds
        let events = engine.record_outflow(1, oid(1), 11_000);

        // Should get 2 events: AbnormalOutflow + ObjectDrain
        assert_eq!(events.len(), 2);
        let has_outflow = events.iter().any(|e| matches!(e.trigger, SafetyTrigger::AbnormalOutflow { .. }));
        let has_drain = events.iter().any(|e| matches!(e.trigger, SafetyTrigger::ObjectDrain { .. }));
        assert!(has_outflow);
        assert!(has_drain);
    }

    // ── Test 18: frozen object count ─────────────────────────

    #[test]
    fn test_frozen_object_count() {
        let mut engine = DeterministicSafetyEngine::new(small_config());

        assert_eq!(engine.frozen_objects().len(), 0);

        engine.record_outflow(1, oid(1), 6000);
        engine.record_outflow(1, oid(2), 6000);
        assert_eq!(engine.frozen_objects().len(), 2);

        engine.unfreeze_object(&oid(1));
        assert_eq!(engine.frozen_objects().len(), 1);
    }
}
