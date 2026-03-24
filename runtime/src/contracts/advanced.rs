//! Advanced contract features — composability, balances, events, oracles,
//! token factory, cross-reads, IBC hooks, and scheduled execution.
//!
//! These features extend the Intent Contract model to competitive parity
//! with Ethereum/Solana/Cosmos while preserving the constraint-validation
//! architecture (no reentrancy, solver-driven execution, atomic multi-object plans).

use crate::objects::base::{ObjectId, SchemaId};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// 1. CONTRACT EVENTS
// ─────────────────────────────────────────────────────────────────────────────

/// An event emitted by a contract during a state transition.
/// Events are collected by the settlement engine and included in receipts.
/// Indexers, explorers, and notification systems consume these.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractEvent {
    /// Which contract emitted this event.
    pub contract_id: ObjectId,
    /// Schema of the contract (for filtering by contract type).
    pub schema_id: SchemaId,
    /// Event type identifier (e.g., "transfer", "approval", "deposit").
    pub event_type: String,
    /// Indexed attributes (searchable by indexers).
    pub indexed: BTreeMap<String, String>,
    /// Non-indexed data payload.
    pub data: Vec<u8>,
    /// Epoch in which the event was emitted.
    pub epoch: u64,
}

impl ContractEvent {
    /// Compute a deterministic event ID for dedup.
    pub fn event_id(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_EVENT_V1");
        h.update(&self.contract_id.0);
        h.update(self.event_type.as_bytes());
        h.update(&self.epoch.to_be_bytes());
        h.update(&self.data);
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }
}

/// Events collected during a plan execution.
#[derive(Debug, Clone, Default)]
pub struct EventCollector {
    pub events: Vec<ContractEvent>,
}

impl EventCollector {
    pub fn new() -> Self {
        EventCollector { events: Vec::new() }
    }

    pub fn emit(&mut self, event: ContractEvent) {
        self.events.push(event);
    }

    pub fn drain(&mut self) -> Vec<ContractEvent> {
        std::mem::take(&mut self.events)
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. CONTRACT-HELD BALANCES
// ─────────────────────────────────────────────────────────────────────────────

/// Links a balance object to a contract with withdrawal constraints.
/// The balance exists as a normal BalanceObject but can only be debited
/// when the linked contract's constraint validator approves the withdrawal.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractBalance {
    /// The contract that controls this balance.
    pub contract_id: ObjectId,
    /// The balance object ID.
    pub balance_id: ObjectId,
    /// Schema that must validate withdrawals.
    pub withdrawal_schema_id: SchemaId,
    /// Asset identifier for the held tokens.
    pub asset_id: [u8; 32],
    /// Optional label (e.g., "escrow_deposit", "liquidity_reserve").
    pub label: String,
}

impl ContractBalance {
    pub fn new(
        contract_id: ObjectId,
        balance_id: ObjectId,
        withdrawal_schema_id: SchemaId,
        asset_id: [u8; 32],
        label: &str,
    ) -> Self {
        ContractBalance {
            contract_id,
            balance_id,
            withdrawal_schema_id,
            asset_id,
            label: label.to_string(),
        }
    }
}

/// Registry of contract-controlled balances.
#[derive(Debug, Clone, Default)]
pub struct ContractBalanceRegistry {
    /// balance_id → ContractBalance
    pub bindings: BTreeMap<ObjectId, ContractBalance>,
}

impl ContractBalanceRegistry {
    pub fn new() -> Self {
        ContractBalanceRegistry {
            bindings: BTreeMap::new(),
        }
    }

    /// Bind a balance to a contract. Returns error if already bound.
    pub fn bind(&mut self, cb: ContractBalance) -> Result<(), String> {
        if self.bindings.contains_key(&cb.balance_id) {
            return Err(format!("balance {:?} already bound to a contract", cb.balance_id));
        }
        self.bindings.insert(cb.balance_id, cb);
        Ok(())
    }

    /// Check if a balance is contract-controlled.
    pub fn is_contract_controlled(&self, balance_id: &ObjectId) -> bool {
        self.bindings.contains_key(balance_id)
    }

    /// Get the controlling contract for a balance.
    pub fn get_controller(&self, balance_id: &ObjectId) -> Option<&ContractBalance> {
        self.bindings.get(balance_id)
    }

    /// Release a binding (admin only).
    pub fn unbind(&mut self, balance_id: &ObjectId) -> bool {
        self.bindings.remove(balance_id).is_some()
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 3. PLAN CONTEXT (COMPOSABILITY)
// ─────────────────────────────────────────────────────────────────────────────

/// Context passed to constraint validators during multi-contract plans.
/// Enables composability: validator A can see what validator B will do.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlanContext {
    /// All contract state transitions in this plan, indexed by contract_id.
    pub transitions: BTreeMap<[u8; 32], ProposedTransition>,
    /// All balance operations in this plan.
    pub balance_ops: Vec<BalanceOp>,
    /// Epoch of execution.
    pub epoch: u64,
    /// Total plan gas budget.
    pub gas_budget: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProposedTransition {
    pub contract_id: [u8; 32],
    pub schema_id: [u8; 32],
    pub current_state_hash: [u8; 32],
    pub proposed_state_hash: [u8; 32],
    pub method: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BalanceOp {
    pub balance_id: [u8; 32],
    pub op_type: BalanceOpType,
    pub amount: u128,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum BalanceOpType {
    Debit,
    Credit,
    Lock,
    Unlock,
}

impl PlanContext {
    pub fn new(epoch: u64, gas_budget: u64) -> Self {
        PlanContext {
            transitions: BTreeMap::new(),
            balance_ops: Vec::new(),
            epoch,
            gas_budget,
        }
    }

    /// Build context from a list of operations.
    pub fn from_operations(
        operations: &[crate::resolution::planner::ObjectOperation],
        epoch: u64,
        gas_budget: u64,
    ) -> Self {
        use crate::resolution::planner::ObjectOperation;
        let mut ctx = PlanContext::new(epoch, gas_budget);

        for op in operations {
            match op {
                ObjectOperation::ContractStateTransition { contract_id, schema_id, proposed_state } => {
                    let proposed_hash = {
                        let h = Sha256::digest(proposed_state);
                        let mut arr = [0u8; 32];
                        arr.copy_from_slice(&h);
                        arr
                    };
                    ctx.transitions.insert(contract_id.0, ProposedTransition {
                        contract_id: contract_id.0,
                        schema_id: *schema_id,
                        current_state_hash: [0u8; 32], // populated during validation
                        proposed_state_hash: proposed_hash,
                        method: String::new(),
                    });
                }
                ObjectOperation::DebitBalance { balance_id, amount } => {
                    ctx.balance_ops.push(BalanceOp {
                        balance_id: balance_id.0,
                        op_type: BalanceOpType::Debit,
                        amount: *amount,
                    });
                }
                ObjectOperation::CreditBalance { balance_id, amount } => {
                    ctx.balance_ops.push(BalanceOp {
                        balance_id: balance_id.0,
                        op_type: BalanceOpType::Credit,
                        amount: *amount,
                    });
                }
                ObjectOperation::LockBalance { balance_id, amount } => {
                    ctx.balance_ops.push(BalanceOp {
                        balance_id: balance_id.0,
                        op_type: BalanceOpType::Lock,
                        amount: *amount,
                    });
                }
                ObjectOperation::UnlockBalance { balance_id, amount } => {
                    ctx.balance_ops.push(BalanceOp {
                        balance_id: balance_id.0,
                        op_type: BalanceOpType::Unlock,
                        amount: *amount,
                    });
                }
                _ => {}
            }
        }
        ctx
    }

    /// Check if another contract is being transitioned in this plan.
    pub fn has_transition(&self, contract_id: &[u8; 32]) -> bool {
        self.transitions.contains_key(contract_id)
    }

    /// Get the proposed transition for another contract (cross-read).
    pub fn get_transition(&self, contract_id: &[u8; 32]) -> Option<&ProposedTransition> {
        self.transitions.get(contract_id)
    }

    /// Net balance change for a specific balance in this plan.
    pub fn net_balance_change(&self, balance_id: &[u8; 32]) -> i128 {
        let mut net: i128 = 0;
        for op in &self.balance_ops {
            if &op.balance_id == balance_id {
                match op.op_type {
                    BalanceOpType::Credit => net += op.amount as i128,
                    BalanceOpType::Debit => net -= op.amount as i128,
                    _ => {}
                }
            }
        }
        net
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 4. TOKEN FACTORY
// ─────────────────────────────────────────────────────────────────────────────

/// A request to create a new token via a contract.
/// Included in the plan as a CreateToken operation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenCreationRequest {
    /// The contract requesting token creation.
    pub creator_contract: ObjectId,
    /// Schema that controls minting (must be the creator's schema).
    pub mint_authority_schema: SchemaId,
    /// Token symbol (e.g., "USDO", "LP-ETH-OMNI").
    pub symbol: String,
    /// Decimal places.
    pub decimals: u8,
    /// Initial supply to mint.
    pub initial_supply: u128,
    /// Maximum supply (None = unlimited).
    pub max_supply: Option<u128>,
    /// Metadata (name, description, icon_url, etc.).
    pub metadata: BTreeMap<String, String>,
}

impl TokenCreationRequest {
    /// Compute a deterministic asset_id from the creation parameters.
    pub fn compute_asset_id(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_TOKEN_V1");
        h.update(&self.creator_contract.0);
        h.update(&self.mint_authority_schema);
        h.update(self.symbol.as_bytes());
        h.update(&[self.decimals]);
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 5. ORACLE PRECOMPILE
// ─────────────────────────────────────────────────────────────────────────────

/// A price feed entry provided by the solver in the plan context.
/// Validators can check these during constraint validation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OraclePriceFeed {
    /// Asset pair (e.g., "OMNI/USD", "ETH/USDC").
    pub pair: String,
    /// Price in fixed-point (6 decimal places: 1_000_000 = $1.00).
    pub price: u64,
    /// Timestamp of the price observation (unix seconds).
    pub timestamp: u64,
    /// Source identifier (e.g., "chainlink", "pyth", "band").
    pub source: String,
    /// Solver-provided signature over (pair, price, timestamp, source) as hex.
    /// Validators check this to ensure the solver isn't fabricating prices.
    pub attestation: String,
}

/// Oracle data attached to a plan context.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct OracleContext {
    /// Price feeds keyed by pair name.
    pub feeds: BTreeMap<String, OraclePriceFeed>,
    /// Maximum staleness in seconds. Feeds older than this are rejected.
    pub max_staleness_secs: u64,
}

impl OracleContext {
    pub fn new(max_staleness_secs: u64) -> Self {
        OracleContext {
            feeds: BTreeMap::new(),
            max_staleness_secs,
        }
    }

    /// Add a price feed.
    pub fn add_feed(&mut self, feed: OraclePriceFeed) {
        self.feeds.insert(feed.pair.clone(), feed);
    }

    /// Get a price. Returns None if not available or stale.
    pub fn get_price(&self, pair: &str, current_time: u64) -> Option<u64> {
        let feed = self.feeds.get(pair)?;
        if self.max_staleness_secs > 0 && current_time.saturating_sub(feed.timestamp) > self.max_staleness_secs {
            return None; // stale
        }
        Some(feed.price)
    }

    /// Verify all feeds are within staleness window.
    pub fn validate_freshness(&self, current_time: u64) -> Result<(), String> {
        for (pair, feed) in &self.feeds {
            if self.max_staleness_secs > 0 && current_time.saturating_sub(feed.timestamp) > self.max_staleness_secs {
                return Err(format!("oracle feed '{}' is stale: age {}s > max {}s",
                    pair,
                    current_time.saturating_sub(feed.timestamp),
                    self.max_staleness_secs,
                ));
            }
        }
        Ok(())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 6. CROSS-CONTRACT STATE READS
// ─────────────────────────────────────────────────────────────────────────────

/// A state proof provided by the solver for cross-contract reads.
/// The solver reads contract B's state and provides it (with hash proof)
/// so that contract A's validator can verify it.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateProof {
    /// The contract whose state is being proved.
    pub source_contract_id: ObjectId,
    /// Schema of the source contract.
    pub source_schema_id: SchemaId,
    /// The state bytes of the source contract at read time.
    pub state_snapshot: Vec<u8>,
    /// SHA256 hash of the state snapshot.
    pub state_hash: [u8; 32],
    /// Version of the source contract at read time.
    pub version: u64,
    /// Epoch in which the state was observed.
    pub observed_epoch: u64,
}

impl StateProof {
    /// Verify the hash matches the snapshot.
    pub fn verify(&self) -> bool {
        let h = Sha256::digest(&self.state_snapshot);
        let mut expected = [0u8; 32];
        expected.copy_from_slice(&h);
        expected == self.state_hash
    }
}

/// State proofs attached to a plan context for cross-contract reads.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StateProofSet {
    /// contract_id → StateProof
    pub proofs: BTreeMap<[u8; 32], StateProof>,
}

impl StateProofSet {
    pub fn new() -> Self {
        StateProofSet { proofs: BTreeMap::new() }
    }

    pub fn add(&mut self, proof: StateProof) {
        self.proofs.insert(proof.source_contract_id.0, proof);
    }

    /// Get a verified state snapshot for a contract. Returns None if
    /// no proof or hash mismatch.
    pub fn get_verified(&self, contract_id: &[u8; 32]) -> Option<&[u8]> {
        let proof = self.proofs.get(contract_id)?;
        if proof.verify() {
            Some(&proof.state_snapshot)
        } else {
            None // hash mismatch — reject
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 7. IBC SETTLEMENT HOOKS
// ─────────────────────────────────────────────────────────────────────────────

/// An IBC action requested by a contract during settlement.
/// These are collected by the settlement engine and forwarded to the
/// Go chain's IBC module for execution.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IBCHook {
    /// Source contract requesting the IBC action.
    pub source_contract: ObjectId,
    /// IBC channel to use.
    pub channel_id: String,
    /// IBC port (typically "transfer").
    pub port_id: String,
    /// Action type.
    pub action: IBCAction,
    /// Timeout in seconds from current block time.
    pub timeout_secs: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum IBCAction {
    /// Transfer tokens to another chain.
    Transfer {
        denom: String,
        amount: u128,
        receiver: String, // bech32 on destination chain
    },
    /// Send a generic IBC packet (for ICA or custom protocols).
    SendPacket {
        data: Vec<u8>,
    },
}

/// Collected IBC hooks from a plan execution.
#[derive(Debug, Clone, Default)]
pub struct IBCHookCollector {
    pub hooks: Vec<IBCHook>,
}

impl IBCHookCollector {
    pub fn new() -> Self {
        IBCHookCollector { hooks: Vec::new() }
    }

    pub fn add(&mut self, hook: IBCHook) {
        self.hooks.push(hook);
    }

    pub fn drain(&mut self) -> Vec<IBCHook> {
        std::mem::take(&mut self.hooks)
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 8. SCHEDULED EXECUTION
// ─────────────────────────────────────────────────────────────────────────────

/// A scheduled future execution registered by a contract.
/// The chain's EndBlocker checks for due schedules each epoch.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScheduledExecution {
    /// Unique schedule ID.
    pub schedule_id: [u8; 32],
    /// Contract that scheduled this execution.
    pub contract_id: ObjectId,
    /// Schema of the contract (for capability checks).
    pub schema_id: SchemaId,
    /// Method to invoke when the schedule fires.
    pub method: String,
    /// Parameters to pass to the method.
    pub params: Vec<u8>,
    /// Epoch at which to execute (absolute).
    pub execute_at_epoch: u64,
    /// Whether this is recurring.
    pub recurring: bool,
    /// If recurring, interval in epochs between executions.
    pub interval_epochs: u64,
    /// Maximum number of recurrences (0 = unlimited).
    pub max_recurrences: u64,
    /// Number of times this has already executed.
    pub executed_count: u64,
    /// Whether this schedule is active.
    pub active: bool,
}

impl ScheduledExecution {
    pub fn compute_id(contract_id: &ObjectId, method: &str, execute_at: u64) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_SCHEDULE_V1");
        h.update(&contract_id.0);
        h.update(method.as_bytes());
        h.update(&execute_at.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }

    /// Check if this schedule should fire at the given epoch.
    pub fn is_due(&self, current_epoch: u64) -> bool {
        if !self.active {
            return false;
        }
        if self.max_recurrences > 0 && self.executed_count >= self.max_recurrences {
            return false;
        }
        current_epoch >= self.execute_at_epoch
    }

    /// Advance the schedule after execution. Returns false if expired.
    pub fn advance(&mut self) -> bool {
        self.executed_count += 1;
        if !self.recurring {
            self.active = false;
            return false;
        }
        if self.max_recurrences > 0 && self.executed_count >= self.max_recurrences {
            self.active = false;
            return false;
        }
        self.execute_at_epoch += self.interval_epochs;
        true
    }
}

/// Registry of scheduled executions.
#[derive(Debug, Clone, Default)]
pub struct ScheduleRegistry {
    pub schedules: BTreeMap<[u8; 32], ScheduledExecution>,
}

impl ScheduleRegistry {
    pub fn new() -> Self {
        ScheduleRegistry { schedules: BTreeMap::new() }
    }

    /// Register a new schedule.
    pub fn register(&mut self, schedule: ScheduledExecution) -> Result<(), String> {
        if self.schedules.contains_key(&schedule.schedule_id) {
            return Err("schedule already exists".to_string());
        }
        if schedule.execute_at_epoch == 0 {
            return Err("execute_at_epoch must be > 0".to_string());
        }
        self.schedules.insert(schedule.schedule_id, schedule);
        Ok(())
    }

    /// Cancel a schedule.
    pub fn cancel(&mut self, schedule_id: &[u8; 32]) -> bool {
        if let Some(s) = self.schedules.get_mut(schedule_id) {
            s.active = false;
            true
        } else {
            false
        }
    }

    /// Get all due schedules for a given epoch.
    pub fn get_due(&self, current_epoch: u64) -> Vec<&ScheduledExecution> {
        self.schedules.values()
            .filter(|s| s.is_due(current_epoch))
            .collect()
    }

    /// Advance a schedule after execution.
    pub fn advance(&mut self, schedule_id: &[u8; 32]) -> bool {
        if let Some(s) = self.schedules.get_mut(schedule_id) {
            s.advance()
        } else {
            false
        }
    }

    /// Prune inactive schedules.
    pub fn prune_inactive(&mut self) -> usize {
        let before = self.schedules.len();
        self.schedules.retain(|_, s| s.active);
        before - self.schedules.len()
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// EXTENDED VALIDATION CONTEXT
// ─────────────────────────────────────────────────────────────────────────────

/// Extended validation context that includes all advanced features.
/// Passed to constraint validators alongside the basic ValidationContext.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ExtendedValidationContext {
    /// Other contract transitions in this plan (composability).
    pub plan_context: Option<PlanContext>,
    /// Oracle price feeds provided by the solver.
    pub oracle: Option<OracleContext>,
    /// State proofs for cross-contract reads.
    pub state_proofs: Option<StateProofSet>,
    /// Contract balance bindings visible to this validator.
    pub contract_balances: Vec<ContractBalance>,
}

// ─────────────────────────────────────────────────────────────────────────────
// TESTS
// ─────────────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_event_emission() {
        let mut collector = EventCollector::new();
        let event = ContractEvent {
            contract_id: ObjectId([0xAA; 32]),
            schema_id: [0xBB; 32],
            event_type: "deposit".to_string(),
            indexed: {
                let mut m = BTreeMap::new();
                m.insert("sender".to_string(), "omni1abc".to_string());
                m.insert("amount".to_string(), "1000".to_string());
                m
            },
            data: b"extra_data".to_vec(),
            epoch: 5,
        };
        collector.emit(event.clone());
        assert_eq!(collector.events.len(), 1);
        let id = collector.events[0].event_id();
        assert_ne!(id, [0u8; 32]);

        let drained = collector.drain();
        assert_eq!(drained.len(), 1);
        assert!(collector.events.is_empty());
    }

    #[test]
    fn test_contract_balance_registry() {
        let mut registry = ContractBalanceRegistry::new();
        let cb = ContractBalance::new(
            ObjectId([0xAA; 32]),
            ObjectId([0xBB; 32]),
            [0xCC; 32],
            [0xDD; 32],
            "escrow_deposit",
        );

        assert!(!registry.is_contract_controlled(&ObjectId([0xBB; 32])));
        registry.bind(cb).unwrap();
        assert!(registry.is_contract_controlled(&ObjectId([0xBB; 32])));

        let controller = registry.get_controller(&ObjectId([0xBB; 32])).unwrap();
        assert_eq!(controller.contract_id, ObjectId([0xAA; 32]));
        assert_eq!(controller.label, "escrow_deposit");

        // Duplicate bind fails
        let cb2 = ContractBalance::new(
            ObjectId([0xFF; 32]),
            ObjectId([0xBB; 32]),
            [0xCC; 32],
            [0xDD; 32],
            "other",
        );
        assert!(registry.bind(cb2).is_err());

        // Unbind
        assert!(registry.unbind(&ObjectId([0xBB; 32])));
        assert!(!registry.is_contract_controlled(&ObjectId([0xBB; 32])));
    }

    #[test]
    fn test_plan_context_composability() {
        use crate::resolution::planner::ObjectOperation;

        let ops = vec![
            ObjectOperation::ContractStateTransition {
                contract_id: ObjectId([0xAA; 32]),
                schema_id: [0x11; 32],
                proposed_state: b"state_a".to_vec(),
            },
            ObjectOperation::ContractStateTransition {
                contract_id: ObjectId([0xBB; 32]),
                schema_id: [0x22; 32],
                proposed_state: b"state_b".to_vec(),
            },
            ObjectOperation::DebitBalance {
                balance_id: ObjectId([0xCC; 32]),
                amount: 1000,
            },
            ObjectOperation::CreditBalance {
                balance_id: ObjectId([0xDD; 32]),
                amount: 950,
            },
        ];

        let ctx = PlanContext::from_operations(&ops, 5, 100_000);

        // Both transitions visible
        assert!(ctx.has_transition(&[0xAA; 32]));
        assert!(ctx.has_transition(&[0xBB; 32]));
        assert!(!ctx.has_transition(&[0xFF; 32]));

        // Balance ops tracked
        assert_eq!(ctx.balance_ops.len(), 2);
        assert_eq!(ctx.net_balance_change(&[0xCC; 32]), -1000);
        assert_eq!(ctx.net_balance_change(&[0xDD; 32]), 950);
    }

    #[test]
    fn test_token_creation_request() {
        let req = TokenCreationRequest {
            creator_contract: ObjectId([0xAA; 32]),
            mint_authority_schema: [0xBB; 32],
            symbol: "LP-ETH-OMNI".to_string(),
            decimals: 18,
            initial_supply: 1_000_000,
            max_supply: Some(10_000_000),
            metadata: BTreeMap::new(),
        };

        let asset_id = req.compute_asset_id();
        assert_ne!(asset_id, [0u8; 32]);

        // Deterministic
        let asset_id2 = req.compute_asset_id();
        assert_eq!(asset_id, asset_id2);
    }

    #[test]
    fn test_oracle_price_feed() {
        let mut oracle = OracleContext::new(300); // 5 min staleness
        oracle.add_feed(OraclePriceFeed {
            pair: "OMNI/USD".to_string(),
            price: 5_500_000, // $5.50
            timestamp: 1000,
            source: "chainlink".to_string(),
            attestation: String::new(),
        });

        // Fresh price
        assert_eq!(oracle.get_price("OMNI/USD", 1100), Some(5_500_000));

        // Stale price (>300s old)
        assert_eq!(oracle.get_price("OMNI/USD", 1500), None);

        // Unknown pair
        assert_eq!(oracle.get_price("BTC/USD", 1100), None);

        // Freshness validation
        assert!(oracle.validate_freshness(1100).is_ok());
        assert!(oracle.validate_freshness(1500).is_err());
    }

    #[test]
    fn test_cross_contract_state_proof() {
        let state = b"balance:1000,locked:false";
        let state_hash = {
            let h = Sha256::digest(state);
            let mut arr = [0u8; 32];
            arr.copy_from_slice(&h);
            arr
        };

        let proof = StateProof {
            source_contract_id: ObjectId([0xAA; 32]),
            source_schema_id: [0xBB; 32],
            state_snapshot: state.to_vec(),
            state_hash,
            version: 5,
            observed_epoch: 10,
        };

        assert!(proof.verify());

        let mut set = StateProofSet::new();
        set.add(proof);

        let verified = set.get_verified(&[0xAA; 32]);
        assert!(verified.is_some());
        assert_eq!(verified.unwrap(), state);

        // Tampered proof fails
        let mut bad_proof = StateProof {
            source_contract_id: ObjectId([0xCC; 32]),
            source_schema_id: [0xDD; 32],
            state_snapshot: b"tampered".to_vec(),
            state_hash, // wrong hash for "tampered" data
            version: 1,
            observed_epoch: 10,
        };
        let mut bad_set = StateProofSet::new();
        bad_set.add(bad_proof);
        assert!(bad_set.get_verified(&[0xCC; 32]).is_none());
    }

    #[test]
    fn test_ibc_hook_collector() {
        let mut collector = IBCHookCollector::new();
        collector.add(IBCHook {
            source_contract: ObjectId([0xAA; 32]),
            channel_id: "channel-0".to_string(),
            port_id: "transfer".to_string(),
            action: IBCAction::Transfer {
                denom: "uomni".to_string(),
                amount: 1_000_000,
                receiver: "cosmos1xyz".to_string(),
            },
            timeout_secs: 600,
        });

        assert_eq!(collector.hooks.len(), 1);
        let hooks = collector.drain();
        assert_eq!(hooks.len(), 1);
        assert!(collector.hooks.is_empty());
    }

    #[test]
    fn test_scheduled_execution() {
        let mut registry = ScheduleRegistry::new();
        let contract_id = ObjectId([0xAA; 32]);

        // One-shot schedule at epoch 10
        let schedule = ScheduledExecution {
            schedule_id: ScheduledExecution::compute_id(&contract_id, "liquidate", 10),
            contract_id,
            schema_id: [0xBB; 32],
            method: "liquidate".to_string(),
            params: vec![],
            execute_at_epoch: 10,
            recurring: false,
            interval_epochs: 0,
            max_recurrences: 0,
            executed_count: 0,
            active: true,
        };

        registry.register(schedule).unwrap();

        // Not due at epoch 5
        assert!(registry.get_due(5).is_empty());

        // Due at epoch 10
        let due = registry.get_due(10);
        assert_eq!(due.len(), 1);
        assert_eq!(due[0].method, "liquidate");

        // Advance — one-shot deactivates
        let sid = due[0].schedule_id;
        assert!(!registry.advance(&sid)); // returns false = expired

        // Not due anymore
        assert!(registry.get_due(11).is_empty());
    }

    #[test]
    fn test_recurring_schedule() {
        let mut registry = ScheduleRegistry::new();
        let contract_id = ObjectId([0xBB; 32]);

        let schedule = ScheduledExecution {
            schedule_id: ScheduledExecution::compute_id(&contract_id, "accrue_interest", 5),
            contract_id,
            schema_id: [0xCC; 32],
            method: "accrue_interest".to_string(),
            params: vec![],
            execute_at_epoch: 5,
            recurring: true,
            interval_epochs: 3,
            max_recurrences: 4,
            executed_count: 0,
            active: true,
        };

        registry.register(schedule).unwrap();

        // Fire at epoch 5
        let due = registry.get_due(5);
        assert_eq!(due.len(), 1);
        let sid = due[0].schedule_id;
        assert!(registry.advance(&sid)); // still has recurrences

        // Next fire at epoch 8
        assert!(registry.get_due(6).is_empty());
        assert!(registry.get_due(7).is_empty());
        assert_eq!(registry.get_due(8).len(), 1);
        assert!(registry.advance(&sid));

        // Fire at 11, 14
        assert!(registry.advance(&sid)); // #3
        assert!(!registry.advance(&sid)); // #4 — max reached, deactivates
        assert!(registry.get_due(100).is_empty());
    }

    #[test]
    fn test_schedule_cancel() {
        let mut registry = ScheduleRegistry::new();
        let contract_id = ObjectId([0xDD; 32]);

        let schedule = ScheduledExecution {
            schedule_id: ScheduledExecution::compute_id(&contract_id, "payout", 20),
            contract_id,
            schema_id: [0xEE; 32],
            method: "payout".to_string(),
            params: vec![],
            execute_at_epoch: 20,
            recurring: false,
            interval_epochs: 0,
            max_recurrences: 0,
            executed_count: 0,
            active: true,
        };

        let sid = schedule.schedule_id;
        registry.register(schedule).unwrap();

        assert_eq!(registry.get_due(20).len(), 1);
        assert!(registry.cancel(&sid));
        assert!(registry.get_due(20).is_empty());
    }
}
