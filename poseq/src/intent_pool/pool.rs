//! Public Intent Pool — Section 3 of the architecture specification.
//!
//! Manages admission, deduplication, expiry pruning, rate limiting,
//! and eviction of intents. Provides solver subscription filtering.

use std::collections::{BTreeMap, BTreeSet, VecDeque};
use super::constants::*;
use super::lifecycle::{IntentLifecycleRecord, IntentState};
use super::types::{IntentAnnouncement, IntentTransaction, IntentValidationError};

/// Entry in the intent pool: the full intent + lifecycle tracking.
#[derive(Debug, Clone)]
pub struct PoolEntry {
    pub intent: IntentTransaction,
    pub lifecycle: IntentLifecycleRecord,
}

/// Per-user rate tracking for the current block.
#[derive(Debug, Default)]
struct UserRateTracker {
    /// (block_height, count) for each user.
    counts: BTreeMap<[u8; 32], (u64, u32)>,
}

impl UserRateTracker {
    fn check_and_increment(&mut self, user: &[u8; 32], current_block: u64) -> Result<(), IntentValidationError> {
        let entry = self.counts.entry(*user).or_insert((current_block, 0));
        // Reset counter if we've moved to a new block.
        if entry.0 != current_block {
            entry.0 = current_block;
            entry.1 = 0;
        }
        if entry.1 >= MAX_INTENTS_PER_BLOCK_PER_USER {
            return Err(IntentValidationError::RateLimitExceeded {
                user: *user,
                count: entry.1,
                max: MAX_INTENTS_PER_BLOCK_PER_USER,
            });
        }
        entry.1 += 1;
        Ok(())
    }
}

/// Per-user nonce tracking.
#[derive(Debug, Default)]
struct NonceTracker {
    /// user → highest seen nonce.
    highest: BTreeMap<[u8; 32], u64>,
}

impl NonceTracker {
    fn check_and_record(&mut self, user: &[u8; 32], nonce: u64) -> Result<(), IntentValidationError> {
        if let Some(&last) = self.highest.get(user) {
            if nonce <= last {
                return Err(IntentValidationError::NonceAlreadyUsed { user: *user, nonce });
            }
            if nonce > last + MAX_NONCE_GAP {
                return Err(IntentValidationError::NonceGapTooLarge {
                    expected: last + 1,
                    got: nonce,
                });
            }
        }
        self.highest.insert(*user, nonce);
        Ok(())
    }
}

/// The public intent pool.
pub struct IntentPool {
    /// All intents indexed by intent_id.
    entries: BTreeMap<[u8; 32], PoolEntry>,
    /// Insertion order for deterministic eviction.
    insertion_order: VecDeque<[u8; 32]>,
    /// Bloom-filter-like set of seen intent_ids (for gossip dedup).
    seen_ids: BTreeSet<[u8; 32]>,
    /// Rate limiter per user.
    rate_tracker: UserRateTracker,
    /// Nonce tracker per user.
    nonce_tracker: NonceTracker,
    /// Current block height (updated externally).
    current_block: u64,

    // Metrics counters.
    pub metrics: PoolMetrics,
}

/// Observable metrics for the intent pool.
#[derive(Debug, Clone, Default)]
pub struct PoolMetrics {
    pub total_admitted: u64,
    pub total_rejected: u64,
    pub total_expired: u64,
    pub total_evicted: u64,
    pub rejected_by_reason: BTreeMap<String, u64>,
}

impl IntentPool {
    pub fn new() -> Self {
        IntentPool {
            entries: BTreeMap::new(),
            insertion_order: VecDeque::new(),
            seen_ids: BTreeSet::new(),
            rate_tracker: UserRateTracker::default(),
            nonce_tracker: NonceTracker::default(),
            current_block: 0,
            metrics: PoolMetrics::default(),
        }
    }

    /// Update the current block height. Call this at the start of each block.
    pub fn set_block_height(&mut self, block: u64) {
        self.current_block = block;
    }

    pub fn current_block(&self) -> u64 {
        self.current_block
    }

    /// Number of intents currently in the pool.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Admit an intent into the pool. Returns the IntentAnnouncement for gossip on success.
    pub fn admit(&mut self, intent: IntentTransaction) -> Result<IntentAnnouncement, IntentValidationError> {
        // 1. Basic validation
        intent.validate_basic()?;

        // 2. Dedup
        if self.seen_ids.contains(&intent.intent_id) || self.entries.contains_key(&intent.intent_id) {
            self.record_rejection("duplicate");
            return Err(IntentValidationError::DuplicateIntent);
        }

        // 3. Size check
        let size = intent.estimated_size();
        if size > MAX_INTENT_SIZE {
            self.record_rejection("too_large");
            return Err(IntentValidationError::IntentTooLarge { size, max: MAX_INTENT_SIZE });
        }

        // 4. Deadline check
        if intent.deadline <= self.current_block + MIN_INTENT_LIFETIME {
            self.record_rejection("deadline_expired");
            return Err(IntentValidationError::DeadlineExpired {
                deadline: intent.deadline,
                current_block: self.current_block,
            });
        }

        // 5. Rate limit
        self.rate_tracker.check_and_increment(&intent.user, self.current_block)?;

        // 6. Nonce check
        self.nonce_tracker.check_and_record(&intent.user, intent.nonce)?;

        // 7. Pool capacity — evict if full
        if self.entries.len() >= MAX_POOL_SIZE {
            if !self.evict_lowest_tip() {
                self.record_rejection("pool_full");
                return Err(IntentValidationError::PoolFull);
            }
        }

        // 8. Admit
        let announcement = IntentAnnouncement::from_intent(&intent);
        let lifecycle = IntentLifecycleRecord::new(intent.intent_id, self.current_block);
        let intent_id = intent.intent_id;
        self.entries.insert(intent_id, PoolEntry { intent, lifecycle });
        self.insertion_order.push_back(intent_id);
        self.seen_ids.insert(intent_id);
        self.metrics.total_admitted += 1;

        Ok(announcement)
    }

    /// Get an intent by ID.
    pub fn get(&self, intent_id: &[u8; 32]) -> Option<&PoolEntry> {
        self.entries.get(intent_id)
    }

    /// Get a mutable reference to an intent's lifecycle.
    pub fn get_mut(&mut self, intent_id: &[u8; 32]) -> Option<&mut PoolEntry> {
        self.entries.get_mut(intent_id)
    }

    /// Remove an intent from the pool (e.g., after sequencing).
    pub fn remove(&mut self, intent_id: &[u8; 32]) -> Option<PoolEntry> {
        if let Some(entry) = self.entries.remove(intent_id) {
            self.insertion_order.retain(|id| id != intent_id);
            Some(entry)
        } else {
            None
        }
    }

    /// Return all Open intents matching a solver subscription filter.
    pub fn query_open_intents(
        &self,
        intent_type_filter: Option<&[u8]>,   // type tags
        min_tip: u64,
    ) -> Vec<&PoolEntry> {
        self.entries.values()
            .filter(|e| e.lifecycle.state == IntentState::Open || e.lifecycle.state == IntentState::Admitted)
            .filter(|e| {
                if let Some(tags) = intent_type_filter {
                    tags.contains(&e.intent.intent_type.type_tag())
                } else {
                    true
                }
            })
            .filter(|e| e.intent.tip.unwrap_or(0) >= min_tip)
            .collect()
    }

    /// Open all admitted intents (called when a new batch window starts).
    pub fn open_admitted_intents(&mut self) {
        for entry in self.entries.values_mut() {
            if entry.lifecycle.state == IntentState::Admitted {
                let _ = entry.lifecycle.transition_to(IntentState::Open, self.current_block);
            }
        }
    }

    /// Mark an intent as matched by a specific bundle.
    pub fn mark_matched(&mut self, intent_id: &[u8; 32], bundle_id: [u8; 32], solver_id: [u8; 32]) -> bool {
        if let Some(entry) = self.entries.get_mut(intent_id) {
            if entry.lifecycle.transition_to(IntentState::Matched, self.current_block).is_ok() {
                entry.lifecycle.matched_bundle_id = Some(bundle_id);
                entry.lifecycle.solver_id = Some(solver_id);
                return true;
            }
        }
        false
    }

    /// Run expiry pruning. Call periodically (every EXPIRY_CHECK_INTERVAL blocks).
    pub fn prune_expired(&mut self) -> Vec<[u8; 32]> {
        let block = self.current_block;
        let mut expired = Vec::new();

        for (id, entry) in self.entries.iter() {
            let deadline_expired = block >= entry.intent.deadline;
            let max_residence_expired = block >= entry.lifecycle.admitted_at_block + MAX_POOL_RESIDENCE;

            if (deadline_expired || max_residence_expired)
                && (entry.lifecycle.state == IntentState::Open
                    || entry.lifecycle.state == IntentState::Admitted
                    || entry.lifecycle.state == IntentState::Matched)
            {
                expired.push(*id);
            }
        }

        for id in &expired {
            if let Some(entry) = self.entries.get_mut(id) {
                let _ = entry.lifecycle.transition_to(IntentState::Expired, block);
            }
            // Remove from pool (keep in seen_ids to prevent re-admission)
            self.entries.remove(id);
            self.metrics.total_expired += 1;
        }

        self.insertion_order.retain(|id| self.entries.contains_key(id));
        expired
    }

    /// Cancel an intent. Returns true if successfully cancelled.
    pub fn cancel(&mut self, intent_id: &[u8; 32]) -> bool {
        if let Some(entry) = self.entries.get_mut(intent_id) {
            if entry.lifecycle.state.is_cancellable() {
                if entry.lifecycle.transition_to(IntentState::Cancelled, self.current_block).is_ok() {
                    self.entries.remove(intent_id);
                    self.insertion_order.retain(|id| id != intent_id);
                    return true;
                }
            }
        }
        false
    }

    /// Drain all Open intents for batch production. Returns intents sorted by (tip DESC, deadline ASC).
    pub fn drain_open_for_batch(&mut self, max_count: usize) -> Vec<IntentTransaction> {
        let mut candidates: Vec<_> = self.entries.values()
            .filter(|e| e.lifecycle.state == IntentState::Open)
            .map(|e| (e.intent.intent_id, e.intent.tip.unwrap_or(0), e.intent.deadline))
            .collect();

        // Sort: tip DESC, deadline ASC (most urgent first), then intent_id for determinism
        candidates.sort_by(|a, b| {
            b.1.cmp(&a.1)
                .then(a.2.cmp(&b.2))
                .then(a.0.cmp(&b.0))
        });

        candidates.truncate(max_count);
        let ids: Vec<[u8; 32]> = candidates.iter().map(|c| c.0).collect();

        let mut result = Vec::with_capacity(ids.len());
        for id in &ids {
            if let Some(entry) = self.entries.get(id) {
                result.push(entry.intent.clone());
            }
        }
        result
    }

    /// Evict the intent with the lowest tip. Returns true if an intent was evicted.
    fn evict_lowest_tip(&mut self) -> bool {
        let mut lowest: Option<([u8; 32], u64)> = None;

        for (id, entry) in &self.entries {
            let tip = entry.intent.tip.unwrap_or(0);
            if lowest.is_none() || tip < lowest.unwrap().1 {
                lowest = Some((*id, tip));
            }
        }

        if let Some((id, _)) = lowest {
            self.entries.remove(&id);
            self.insertion_order.retain(|i| i != &id);
            self.metrics.total_evicted += 1;
            true
        } else {
            false
        }
    }

    fn record_rejection(&mut self, reason: &str) {
        self.metrics.total_rejected += 1;
        *self.metrics.rejected_by_reason.entry(reason.to_string()).or_insert(0) += 1;
    }
}

// ─── Solver Subscription ────────────────────────────────────────────────────

/// Solver subscription filter for intent stream.
#[derive(Debug, Clone)]
pub struct SolverSubscription {
    pub solver_id: [u8; 32],
    pub intent_type_filter: Vec<u8>,  // type tags; empty = all
    pub min_tip: u64,
    pub min_amount: u128,
}

impl SolverSubscription {
    /// Check if an intent matches this subscription filter.
    pub fn matches(&self, intent: &IntentTransaction) -> bool {
        // Type filter
        if !self.intent_type_filter.is_empty()
            && !self.intent_type_filter.contains(&intent.intent_type.type_tag())
        {
            return false;
        }

        // Tip filter
        if intent.tip.unwrap_or(0) < self.min_tip {
            return false;
        }

        // Solver permission filter
        if !intent.solver_permissions.is_permitted(&self.solver_id) {
            return false;
        }

        true
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intent_pool::types::*;
    use std::collections::BTreeMap;

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32];
        id[0] = b;
        AssetId::token(id)
    }

    fn make_swap_intent(user_byte: u8, nonce: u64, tip: u64, deadline: u64) -> IntentTransaction {
        let user = { let mut u = [0u8; 32]; u[0] = user_byte; u };
        let swap = SwapIntent {
            asset_in: make_asset(1),
            asset_out: make_asset(2),
            amount_in: 1000,
            min_amount_out: 950,
            max_slippage_bps: 50,
            route_hint: None,
        };
        let mut intent = IntentTransaction {
            intent_id: [0u8; 32],
            intent_type: IntentKind::Swap(swap),
            version: 1,
            user,
            nonce,
            recipient: None,
            deadline,
            valid_from: None,
            max_fee: 100,
            tip: Some(tip),
            partial_fill_allowed: false,
            solver_permissions: SolverPermissions::default(),
            execution_preferences: ExecutionPrefs::default(),
            witness_hash: None,
            signature: vec![1u8; 64],
            metadata: BTreeMap::new(),
        };
        intent.intent_id = intent.compute_intent_id();
        intent
    }

    #[test]
    fn test_pool_admit_and_query() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent = make_swap_intent(1, 1, 50, 1000);
        let ann = pool.admit(intent.clone()).unwrap();
        assert_eq!(ann.intent_id, intent.intent_id);
        assert_eq!(pool.len(), 1);

        // Query
        let open = pool.query_open_intents(None, 0);
        assert_eq!(open.len(), 1);
    }

    #[test]
    fn test_pool_dedup() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent = make_swap_intent(1, 1, 50, 1000);
        assert!(pool.admit(intent.clone()).is_ok());
        assert_eq!(pool.admit(intent).unwrap_err(), IntentValidationError::DuplicateIntent);
    }

    #[test]
    fn test_pool_nonce_replay() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent1 = make_swap_intent(1, 1, 50, 1000);
        assert!(pool.admit(intent1).is_ok());

        // Same user, same nonce — rejected
        let intent2 = make_swap_intent(1, 1, 60, 2000);
        // Different intent_id due to different tip/deadline, but same nonce
        match pool.admit(intent2) {
            Err(IntentValidationError::NonceAlreadyUsed { .. }) => {}
            other => panic!("expected NonceAlreadyUsed, got {:?}", other),
        }
    }

    #[test]
    fn test_pool_nonce_gap() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent1 = make_swap_intent(1, 1, 50, 1000);
        assert!(pool.admit(intent1).is_ok());

        // Nonce gap > MAX_NONCE_GAP
        let intent2 = make_swap_intent(1, 1 + MAX_NONCE_GAP + 1, 50, 1000);
        match pool.admit(intent2) {
            Err(IntentValidationError::NonceGapTooLarge { .. }) => {}
            other => panic!("expected NonceGapTooLarge, got {:?}", other),
        }
    }

    #[test]
    fn test_pool_rate_limit() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        for i in 1..=(MAX_INTENTS_PER_BLOCK_PER_USER as u64) {
            let intent = make_swap_intent(1, i, 50, 1000);
            assert!(pool.admit(intent).is_ok());
        }

        // One more should fail
        let intent = make_swap_intent(1, MAX_INTENTS_PER_BLOCK_PER_USER as u64 + 1, 50, 1000);
        match pool.admit(intent) {
            Err(IntentValidationError::RateLimitExceeded { .. }) => {}
            other => panic!("expected RateLimitExceeded, got {:?}", other),
        }
    }

    #[test]
    fn test_pool_expiry_pruning() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent = make_swap_intent(1, 1, 50, 100); // deadline = 100
        assert!(pool.admit(intent).is_ok());

        pool.open_admitted_intents();
        assert_eq!(pool.len(), 1);

        // Advance past deadline
        pool.set_block_height(100);
        let expired = pool.prune_expired();
        assert_eq!(expired.len(), 1);
        assert_eq!(pool.len(), 0);
    }

    #[test]
    fn test_pool_cancellation() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        let intent = make_swap_intent(1, 1, 50, 1000);
        let id = intent.intent_id;
        assert!(pool.admit(intent).is_ok());
        pool.open_admitted_intents();

        assert!(pool.cancel(&id));
        assert_eq!(pool.len(), 0);
    }

    #[test]
    fn test_solver_subscription_filter() {
        let solver_id = { let mut s = [0u8; 32]; s[0] = 99; s };
        let sub = SolverSubscription {
            solver_id,
            intent_type_filter: vec![0x01], // swaps only
            min_tip: 20,
            min_amount: 0,
        };

        let swap = make_swap_intent(1, 1, 50, 1000);
        assert!(sub.matches(&swap));

        let low_tip = make_swap_intent(2, 1, 10, 1000);
        assert!(!sub.matches(&low_tip));
    }

    #[test]
    fn test_drain_open_ordering() {
        let mut pool = IntentPool::new();
        pool.set_block_height(10);

        // High tip, far deadline
        let a = make_swap_intent(1, 1, 100, 2000);
        // Low tip, near deadline
        let b = make_swap_intent(2, 1, 10, 500);
        // Medium tip, medium deadline
        let c = make_swap_intent(3, 1, 50, 1000);

        pool.admit(a.clone()).unwrap();
        pool.admit(b.clone()).unwrap();
        pool.admit(c.clone()).unwrap();
        pool.open_admitted_intents();

        let drained = pool.drain_open_for_batch(10);
        assert_eq!(drained.len(), 3);
        // Tip DESC: a(100) > c(50) > b(10)
        assert_eq!(drained[0].intent_id, a.intent_id);
        assert_eq!(drained[1].intent_id, c.intent_id);
        assert_eq!(drained[2].intent_id, b.intent_id);
    }
}
