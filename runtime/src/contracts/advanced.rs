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
// 9. TOKEN ALLOWANCES (ERC-20 approve/transferFrom equivalent)
// ─────────────────────────────────────────────────────────────────────────────

/// An allowance granting `spender` the right to debit up to `remaining`
/// units of `asset_id` from `owner`'s balance.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Allowance {
    pub owner: [u8; 32],
    pub spender: [u8; 32],
    pub asset_id: [u8; 32],
    pub remaining: u128,
}

/// Registry of token allowances.
#[derive(Debug, Clone, Default)]
pub struct AllowanceRegistry {
    /// Key: (owner, spender, asset_id) → Allowance
    entries: BTreeMap<([u8; 32], [u8; 32], [u8; 32]), Allowance>,
}

impl AllowanceRegistry {
    pub fn new() -> Self { AllowanceRegistry { entries: BTreeMap::new() } }

    /// Set or replace an allowance. Setting to 0 revokes.
    pub fn approve(&mut self, owner: [u8; 32], spender: [u8; 32], asset_id: [u8; 32], amount: u128) {
        let key = (owner, spender, asset_id);
        if amount == 0 {
            self.entries.remove(&key);
        } else {
            self.entries.insert(key, Allowance { owner, spender, asset_id, remaining: amount });
        }
    }

    /// Check the remaining allowance.
    pub fn allowance(&self, owner: &[u8; 32], spender: &[u8; 32], asset_id: &[u8; 32]) -> u128 {
        self.entries.get(&(*owner, *spender, *asset_id)).map(|a| a.remaining).unwrap_or(0)
    }

    /// Spend from an allowance. Returns Err if insufficient.
    pub fn spend(&mut self, owner: &[u8; 32], spender: &[u8; 32], asset_id: &[u8; 32], amount: u128) -> Result<(), String> {
        let key = (*owner, *spender, *asset_id);
        let entry = self.entries.get_mut(&key)
            .ok_or_else(|| "no allowance set".to_string())?;
        if entry.remaining < amount {
            return Err(format!("allowance {} < requested {}", entry.remaining, amount));
        }
        entry.remaining -= amount;
        if entry.remaining == 0 {
            self.entries.remove(&key);
        }
        Ok(())
    }

    /// Increase an existing allowance.
    pub fn increase(&mut self, owner: [u8; 32], spender: [u8; 32], asset_id: [u8; 32], amount: u128) {
        let key = (owner, spender, asset_id);
        let current = self.entries.get(&key).map(|a| a.remaining).unwrap_or(0);
        self.approve(owner, spender, asset_id, current.saturating_add(amount));
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 10. NON-FUNGIBLE TOKENS (ERC-721 equivalent)
// ─────────────────────────────────────────────────────────────────────────────

/// A non-fungible token. Each instance has a unique `token_id` within its
/// `collection_id`. Owns metadata URI and optional on-chain attributes.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NonFungibleToken {
    /// Unique ID of this specific NFT.
    pub token_id: [u8; 32],
    /// Collection this NFT belongs to.
    pub collection_id: [u8; 32],
    /// Current owner.
    pub owner: [u8; 32],
    /// Metadata URI (e.g., IPFS hash, HTTP URL).
    pub metadata_uri: String,
    /// On-chain attributes (key-value pairs for gaming/identity).
    pub attributes: BTreeMap<String, String>,
    /// Optional approved operator who can transfer this NFT.
    pub approved: Option<[u8; 32]>,
    /// Epoch at which this NFT was minted.
    pub minted_at_epoch: u64,
    /// Whether this NFT is frozen (non-transferable).
    pub frozen: bool,
}

impl NonFungibleToken {
    pub fn new(
        token_id: [u8; 32],
        collection_id: [u8; 32],
        owner: [u8; 32],
        metadata_uri: String,
        epoch: u64,
    ) -> Self {
        NonFungibleToken {
            token_id, collection_id, owner, metadata_uri,
            attributes: BTreeMap::new(),
            approved: None,
            minted_at_epoch: epoch,
            frozen: false,
        }
    }

    /// Compute a deterministic NFT ID from collection + serial number.
    pub fn compute_token_id(collection_id: &[u8; 32], serial: u64) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_NFT_V1");
        h.update(collection_id);
        h.update(&serial.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }

    /// Transfer ownership. Fails if frozen or caller is not owner/approved.
    pub fn transfer(&mut self, from: &[u8; 32], to: [u8; 32]) -> Result<(), String> {
        if self.frozen {
            return Err("NFT is frozen".to_string());
        }
        if &self.owner != from && self.approved.as_ref() != Some(from) {
            return Err("not owner or approved".to_string());
        }
        self.owner = to;
        self.approved = None; // clear approval on transfer
        Ok(())
    }

    /// Approve an operator to transfer this NFT.
    pub fn approve(&mut self, caller: &[u8; 32], operator: [u8; 32]) -> Result<(), String> {
        if &self.owner != caller {
            return Err("only owner can approve".to_string());
        }
        self.approved = Some(operator);
        Ok(())
    }
}

/// Registry of NFT collections and individual tokens.
#[derive(Debug, Clone, Default)]
pub struct NFTRegistry {
    /// token_id → NonFungibleToken
    pub tokens: BTreeMap<[u8; 32], NonFungibleToken>,
    /// collection_id → next serial number (for auto-increment minting)
    pub collection_serials: BTreeMap<[u8; 32], u64>,
    /// owner → set of token_ids owned
    pub owner_index: BTreeMap<[u8; 32], Vec<[u8; 32]>>,
}

impl NFTRegistry {
    pub fn new() -> Self { NFTRegistry::default() }

    /// Mint a new NFT in a collection. Returns the token_id.
    pub fn mint(
        &mut self,
        collection_id: [u8; 32],
        owner: [u8; 32],
        metadata_uri: String,
        epoch: u64,
    ) -> [u8; 32] {
        let serial = self.collection_serials.entry(collection_id).or_insert(0);
        let token_id = NonFungibleToken::compute_token_id(&collection_id, *serial);
        *serial += 1;

        let nft = NonFungibleToken::new(token_id, collection_id, owner, metadata_uri, epoch);
        self.tokens.insert(token_id, nft);
        self.owner_index.entry(owner).or_default().push(token_id);
        token_id
    }

    /// Transfer an NFT.
    pub fn transfer(&mut self, token_id: &[u8; 32], from: &[u8; 32], to: [u8; 32]) -> Result<(), String> {
        let nft = self.tokens.get_mut(token_id).ok_or("NFT not found")?;
        nft.transfer(from, to)?;

        // Update owner index
        if let Some(ids) = self.owner_index.get_mut(from) {
            ids.retain(|id| id != token_id);
        }
        self.owner_index.entry(to).or_default().push(*token_id);
        Ok(())
    }

    /// Get an NFT by token_id.
    pub fn get(&self, token_id: &[u8; 32]) -> Option<&NonFungibleToken> {
        self.tokens.get(token_id)
    }

    /// Get all tokens owned by an address.
    pub fn tokens_of(&self, owner: &[u8; 32]) -> Vec<[u8; 32]> {
        self.owner_index.get(owner).cloned().unwrap_or_default()
    }

    /// Total supply of a collection.
    pub fn collection_supply(&self, collection_id: &[u8; 32]) -> u64 {
        self.collection_serials.get(collection_id).copied().unwrap_or(0)
    }

    /// Burn an NFT. Only owner can burn.
    pub fn burn(&mut self, token_id: &[u8; 32], caller: &[u8; 32]) -> Result<(), String> {
        let nft = self.tokens.get(token_id).ok_or("NFT not found")?;
        if &nft.owner != caller {
            return Err("only owner can burn".to_string());
        }
        let owner = nft.owner;
        self.tokens.remove(token_id);
        if let Some(ids) = self.owner_index.get_mut(&owner) {
            ids.retain(|id| id != token_id);
        }
        Ok(())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 11. STRUCTURED CONTRACT STORAGE
// ─────────────────────────────────────────────────────────────────────────────

/// Typed key-value storage layer over ContractObject's opaque state bytes.
/// Instead of hand-serializing, contracts can use named fields with typed values.
///
/// Stored as the ContractObject.state field via bincode serialization of the
/// inner BTreeMap.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ContractStorage {
    fields: BTreeMap<String, StorageValue>,
}

/// A typed storage value (like an EVM storage slot, but named and typed).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum StorageValue {
    Uint128(u128),
    Int128(i128),
    Bool(bool),
    Bytes32([u8; 32]),
    String(String),
    Bytes(Vec<u8>),
    /// Mapping: key → value (like Solidity `mapping(string => uint)`)
    Map(BTreeMap<String, Box<StorageValue>>),
    /// List of values (like Solidity arrays)
    List(Vec<StorageValue>),
}

impl ContractStorage {
    pub fn new() -> Self { ContractStorage { fields: BTreeMap::new() } }

    pub fn set(&mut self, key: &str, value: StorageValue) {
        self.fields.insert(key.to_string(), value);
    }

    pub fn get(&self, key: &str) -> Option<&StorageValue> {
        self.fields.get(key)
    }

    pub fn get_uint128(&self, key: &str) -> Option<u128> {
        match self.get(key)? { StorageValue::Uint128(v) => Some(*v), _ => None }
    }

    pub fn get_bool(&self, key: &str) -> Option<bool> {
        match self.get(key)? { StorageValue::Bool(v) => Some(*v), _ => None }
    }

    pub fn get_string(&self, key: &str) -> Option<&str> {
        match self.get(key)? { StorageValue::String(v) => Some(v.as_str()), _ => None }
    }

    pub fn get_bytes32(&self, key: &str) -> Option<&[u8; 32]> {
        match self.get(key)? { StorageValue::Bytes32(v) => Some(v), _ => None }
    }

    /// Access a nested mapping: storage["balances"]["alice"] → value
    pub fn get_map_entry(&self, map_key: &str, entry_key: &str) -> Option<&StorageValue> {
        match self.get(map_key)? {
            StorageValue::Map(m) => m.get(entry_key).map(|v| v.as_ref()),
            _ => None,
        }
    }

    /// Set a nested mapping entry.
    pub fn set_map_entry(&mut self, map_key: &str, entry_key: &str, value: StorageValue) {
        let map = self.fields.entry(map_key.to_string())
            .or_insert_with(|| StorageValue::Map(BTreeMap::new()));
        if let StorageValue::Map(m) = map {
            m.insert(entry_key.to_string(), Box::new(value));
        }
    }

    /// Serialize to bytes (for ContractObject.state).
    pub fn to_bytes(&self) -> Vec<u8> {
        bincode::serialize(self).unwrap_or_default()
    }

    /// Deserialize from bytes (from ContractObject.state).
    pub fn from_bytes(data: &[u8]) -> Result<Self, String> {
        bincode::deserialize(data).map_err(|e| format!("storage decode: {}", e))
    }

    /// Remove a key.
    pub fn remove(&mut self, key: &str) -> bool {
        self.fields.remove(key).is_some()
    }

    /// Number of top-level keys.
    pub fn len(&self) -> usize {
        self.fields.len()
    }

    pub fn is_empty(&self) -> bool {
        self.fields.is_empty()
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 12. META-TRANSACTIONS (gasless/relayed)
// ─────────────────────────────────────────────────────────────────────────────

/// A meta-transaction wrapper allowing a relayer to pay gas on behalf of
/// the original sender. The inner intent is signed by the sender; the
/// relayer submits and pays fees.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetaTransaction {
    /// The actual sender (signs the inner intent).
    pub sender: [u8; 32],
    /// The relayer paying gas (signs the outer envelope).
    pub fee_payer: [u8; 32],
    /// The inner intent transaction ID (links to real IntentTransaction).
    pub inner_tx_id: [u8; 32],
    /// Max fee the relayer is willing to pay.
    pub relayer_max_fee: u64,
    /// Optional relayer tip from the sender (incentive for relaying).
    pub relayer_tip: u64,
    /// Deadline epoch for this meta-tx (can differ from inner tx deadline).
    pub deadline_epoch: u64,
    /// Nonce to prevent replay of the same meta-tx.
    pub nonce: u64,
}

impl MetaTransaction {
    /// Compute a deterministic meta-tx ID.
    pub fn compute_id(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_META_TX_V1");
        h.update(&self.sender);
        h.update(&self.fee_payer);
        h.update(&self.inner_tx_id);
        h.update(&self.nonce.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        id
    }

    /// Validate structural correctness.
    pub fn validate(&self) -> Result<(), String> {
        if self.sender == [0u8; 32] { return Err("sender is zero".to_string()); }
        if self.fee_payer == [0u8; 32] { return Err("fee_payer is zero".to_string()); }
        if self.sender == self.fee_payer { return Err("sender and fee_payer must differ".to_string()); }
        if self.relayer_max_fee == 0 { return Err("relayer_max_fee must be > 0".to_string()); }
        Ok(())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 13. VRF RANDOMNESS
// ─────────────────────────────────────────────────────────────────────────────

/// Deterministic, verifiable randomness derived from epoch and contract state.
/// Not a true VRF (no private key involved), but provides deterministic
/// unpredictable-to-users randomness that all validators agree on.
///
/// Security: The seed is unknown until the epoch is finalized, so it cannot
/// be manipulated by a single proposer. A full VRF integration (BLS-based)
/// can replace this with the same interface.
#[derive(Debug, Clone)]
pub struct EpochRandomness {
    /// The finalized epoch whose state_root seeds this randomness.
    pub epoch: u64,
    /// State root of the epoch (committed before randomness is derived).
    pub state_root: [u8; 32],
}

impl EpochRandomness {
    pub fn new(epoch: u64, state_root: [u8; 32]) -> Self {
        EpochRandomness { epoch, state_root }
    }

    /// Derive a deterministic random value for a given domain.
    /// Different domains produce independent random streams.
    pub fn random_bytes32(&self, domain: &str) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_VRF_V1");
        h.update(&self.epoch.to_be_bytes());
        h.update(&self.state_root);
        h.update(domain.as_bytes());
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Derive a random u64 in range [0, max).
    pub fn random_u64(&self, domain: &str, max: u64) -> u64 {
        if max == 0 { return 0; }
        let bytes = self.random_bytes32(domain);
        let raw = u64::from_be_bytes([bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7]]);
        raw % max
    }

    /// Derive a random u128 in range [0, max).
    pub fn random_u128(&self, domain: &str, max: u128) -> u128 {
        if max == 0 { return 0; }
        let bytes = self.random_bytes32(domain);
        let raw = u128::from_be_bytes([
            bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7],
            bytes[8], bytes[9], bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15],
        ]);
        raw % max
    }

    /// Derive a seeded random for a specific contract + nonce.
    /// Useful for per-contract randomness (e.g., lottery draws).
    pub fn contract_random(&self, contract_id: &[u8; 32], nonce: u64) -> [u8; 32] {
        let domain = format!("contract:{}:nonce:{}", hex::encode(&contract_id[..8]), nonce);
        self.random_bytes32(&domain)
    }

    /// Shuffle a list deterministically using Fisher-Yates with epoch randomness.
    pub fn shuffle<T: Clone>(&self, domain: &str, items: &[T]) -> Vec<T> {
        let mut result = items.to_vec();
        let n = result.len();
        for i in (1..n).rev() {
            let sub_domain = format!("{}:shuffle:{}", domain, i);
            let j = self.random_u64(&sub_domain, (i + 1) as u64) as usize;
            result.swap(i, j);
        }
        result
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// 14. COMMIT-REVEAL SCHEME
// ─────────────────────────────────────────────────────────────────────────────

/// A commit-reveal record for sealed-bid auctions, private voting, etc.
///
/// Phase 1 (Commit): User submits `commitment = SHA256(secret || value || salt)`.
/// Phase 2 (Reveal): User submits `(value, salt)`. System verifies hash matches.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommitRevealRecord {
    /// Unique ID for this commit-reveal instance.
    pub id: [u8; 32],
    /// The commitment hash (submitted in commit phase).
    pub commitment: [u8; 32],
    /// The revealed value (None until reveal phase).
    pub revealed_value: Option<Vec<u8>>,
    /// The salt used (None until reveal phase).
    pub revealed_salt: Option<Vec<u8>>,
    /// Who committed.
    pub committer: [u8; 32],
    /// Epoch when commit was submitted.
    pub commit_epoch: u64,
    /// Deadline epoch for reveal (after this, commit expires).
    pub reveal_deadline_epoch: u64,
    /// Whether the reveal has been validated.
    pub revealed: bool,
}

impl CommitRevealRecord {
    /// Compute the expected commitment hash from value + salt.
    pub fn compute_commitment(value: &[u8], salt: &[u8]) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_COMMIT_V1");
        h.update(value);
        h.update(salt);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Create a new commit phase record.
    pub fn commit(
        committer: [u8; 32],
        commitment: [u8; 32],
        commit_epoch: u64,
        reveal_deadline_epoch: u64,
    ) -> Self {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_CR_ID_V1");
        h.update(&committer);
        h.update(&commitment);
        h.update(&commit_epoch.to_be_bytes());
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);

        CommitRevealRecord {
            id,
            commitment,
            revealed_value: None,
            revealed_salt: None,
            committer,
            commit_epoch,
            reveal_deadline_epoch,
            revealed: false,
        }
    }

    /// Attempt to reveal. Returns Ok(()) if hash matches, Err otherwise.
    pub fn reveal(&mut self, value: Vec<u8>, salt: Vec<u8>, current_epoch: u64) -> Result<(), String> {
        if self.revealed {
            return Err("already revealed".to_string());
        }
        if current_epoch > self.reveal_deadline_epoch {
            return Err(format!("reveal deadline passed (deadline={}, now={})", self.reveal_deadline_epoch, current_epoch));
        }
        let expected = Self::compute_commitment(&value, &salt);
        if expected != self.commitment {
            return Err("commitment mismatch: hash(value || salt) != commitment".to_string());
        }
        self.revealed_value = Some(value);
        self.revealed_salt = Some(salt);
        self.revealed = true;
        Ok(())
    }

    /// Check if the reveal deadline has passed without reveal.
    pub fn is_expired(&self, current_epoch: u64) -> bool {
        !self.revealed && current_epoch > self.reveal_deadline_epoch
    }
}

/// Registry of active commit-reveal instances.
#[derive(Debug, Clone, Default)]
pub struct CommitRevealRegistry {
    pub records: BTreeMap<[u8; 32], CommitRevealRecord>,
}

impl CommitRevealRegistry {
    pub fn new() -> Self { CommitRevealRegistry::default() }

    pub fn commit(&mut self, record: CommitRevealRecord) -> Result<[u8; 32], String> {
        let id = record.id;
        if self.records.contains_key(&id) {
            return Err("duplicate commit".to_string());
        }
        self.records.insert(id, record);
        Ok(id)
    }

    pub fn reveal(&mut self, id: &[u8; 32], value: Vec<u8>, salt: Vec<u8>, current_epoch: u64) -> Result<&CommitRevealRecord, String> {
        let record = self.records.get_mut(id).ok_or("commit not found")?;
        record.reveal(value, salt, current_epoch)?;
        Ok(self.records.get(id).unwrap())
    }

    pub fn get(&self, id: &[u8; 32]) -> Option<&CommitRevealRecord> {
        self.records.get(id)
    }

    /// Prune expired unrevealed commits.
    pub fn prune_expired(&mut self, current_epoch: u64) -> usize {
        let before = self.records.len();
        self.records.retain(|_, r| !r.is_expired(current_epoch));
        before - self.records.len()
    }
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

    // ── Feature 9: Token Allowances ──────────────────────────────────────

    #[test]
    fn test_allowance_approve_and_spend() {
        let mut reg = AllowanceRegistry::new();
        let owner = [1u8; 32];
        let spender = [2u8; 32];
        let asset = [0xAA; 32];

        reg.approve(owner, spender, asset, 1000);
        assert_eq!(reg.allowance(&owner, &spender, &asset), 1000);

        reg.spend(&owner, &spender, &asset, 400).unwrap();
        assert_eq!(reg.allowance(&owner, &spender, &asset), 600);

        assert!(reg.spend(&owner, &spender, &asset, 700).is_err());
        assert_eq!(reg.allowance(&owner, &spender, &asset), 600);

        reg.spend(&owner, &spender, &asset, 600).unwrap();
        assert_eq!(reg.allowance(&owner, &spender, &asset), 0);
    }

    #[test]
    fn test_allowance_revoke() {
        let mut reg = AllowanceRegistry::new();
        let owner = [1u8; 32];
        let spender = [2u8; 32];
        let asset = [0xAA; 32];

        reg.approve(owner, spender, asset, 500);
        assert_eq!(reg.allowance(&owner, &spender, &asset), 500);

        reg.approve(owner, spender, asset, 0); // revoke
        assert_eq!(reg.allowance(&owner, &spender, &asset), 0);
        assert!(reg.spend(&owner, &spender, &asset, 1).is_err());
    }

    #[test]
    fn test_allowance_increase() {
        let mut reg = AllowanceRegistry::new();
        let owner = [1u8; 32];
        let spender = [2u8; 32];
        let asset = [0xAA; 32];

        reg.approve(owner, spender, asset, 100);
        reg.increase(owner, spender, asset, 200);
        assert_eq!(reg.allowance(&owner, &spender, &asset), 300);
    }

    // ── Feature 10: NFTs ─────────────────────────────────────────────────

    #[test]
    fn test_nft_mint_and_transfer() {
        let mut reg = NFTRegistry::new();
        let collection = [0xCC; 32];
        let alice = [1u8; 32];
        let bob = [2u8; 32];

        let tid = reg.mint(collection, alice, "ipfs://meta1".to_string(), 1);
        assert_eq!(reg.collection_supply(&collection), 1);
        assert_eq!(reg.tokens_of(&alice).len(), 1);

        reg.transfer(&tid, &alice, bob).unwrap();
        assert_eq!(reg.tokens_of(&alice).len(), 0);
        assert_eq!(reg.tokens_of(&bob).len(), 1);
        assert_eq!(reg.get(&tid).unwrap().owner, bob);
    }

    #[test]
    fn test_nft_approval_and_transfer() {
        let mut reg = NFTRegistry::new();
        let collection = [0xCC; 32];
        let alice = [1u8; 32];
        let operator = [3u8; 32];
        let bob = [2u8; 32];

        let tid = reg.mint(collection, alice, "ipfs://meta2".to_string(), 1);

        // Operator can't transfer without approval
        assert!(reg.transfer(&tid, &operator, bob).is_err());

        // Approve operator
        reg.tokens.get_mut(&tid).unwrap().approve(&alice, operator).unwrap();

        // Now operator can transfer
        reg.transfer(&tid, &operator, bob).unwrap();
        assert_eq!(reg.get(&tid).unwrap().owner, bob);
        assert!(reg.get(&tid).unwrap().approved.is_none()); // cleared on transfer
    }

    #[test]
    fn test_nft_burn() {
        let mut reg = NFTRegistry::new();
        let collection = [0xCC; 32];
        let alice = [1u8; 32];

        let tid = reg.mint(collection, alice, "ipfs://burn".to_string(), 1);
        assert!(reg.get(&tid).is_some());

        reg.burn(&tid, &alice).unwrap();
        assert!(reg.get(&tid).is_none());
        assert!(reg.tokens_of(&alice).is_empty());
    }

    #[test]
    fn test_nft_frozen() {
        let mut reg = NFTRegistry::new();
        let collection = [0xCC; 32];
        let alice = [1u8; 32];
        let bob = [2u8; 32];

        let tid = reg.mint(collection, alice, "ipfs://frozen".to_string(), 1);
        reg.tokens.get_mut(&tid).unwrap().frozen = true;

        assert!(reg.transfer(&tid, &alice, bob).is_err());
    }

    // ── Feature 11: Structured Storage ───────────────────────────────────

    #[test]
    fn test_structured_storage_basic() {
        let mut s = ContractStorage::new();
        s.set("total_supply", StorageValue::Uint128(1_000_000));
        s.set("name", StorageValue::String("MyToken".to_string()));
        s.set("paused", StorageValue::Bool(false));

        assert_eq!(s.get_uint128("total_supply"), Some(1_000_000));
        assert_eq!(s.get_string("name"), Some("MyToken"));
        assert_eq!(s.get_bool("paused"), Some(false));
        assert_eq!(s.len(), 3);
    }

    #[test]
    fn test_structured_storage_mapping() {
        let mut s = ContractStorage::new();
        s.set_map_entry("balances", "alice", StorageValue::Uint128(500));
        s.set_map_entry("balances", "bob", StorageValue::Uint128(300));

        assert_eq!(
            s.get_map_entry("balances", "alice"),
            Some(&StorageValue::Uint128(500))
        );
        assert_eq!(
            s.get_map_entry("balances", "bob"),
            Some(&StorageValue::Uint128(300))
        );
        assert!(s.get_map_entry("balances", "charlie").is_none());
    }

    #[test]
    fn test_structured_storage_roundtrip() {
        let mut s = ContractStorage::new();
        s.set("counter", StorageValue::Uint128(42));
        s.set_map_entry("data", "key1", StorageValue::String("val1".to_string()));

        let bytes = s.to_bytes();
        let restored = ContractStorage::from_bytes(&bytes).unwrap();
        assert_eq!(restored.get_uint128("counter"), Some(42));
        assert_eq!(
            restored.get_map_entry("data", "key1"),
            Some(&StorageValue::String("val1".to_string()))
        );
    }

    // ── Feature 12: Meta-Transactions ────────────────────────────────────

    #[test]
    fn test_meta_transaction_validation() {
        let mt = MetaTransaction {
            sender: [1u8; 32],
            fee_payer: [2u8; 32],
            inner_tx_id: [0xAA; 32],
            relayer_max_fee: 1000,
            relayer_tip: 50,
            deadline_epoch: 100,
            nonce: 1,
        };
        assert!(mt.validate().is_ok());

        let bad = MetaTransaction { sender: [0u8; 32], ..mt.clone() };
        assert!(bad.validate().is_err());

        let same = MetaTransaction { fee_payer: [1u8; 32], ..mt.clone() };
        assert!(same.validate().is_err()); // sender == fee_payer

        let zero_fee = MetaTransaction { relayer_max_fee: 0, ..mt };
        assert!(zero_fee.validate().is_err());
    }

    #[test]
    fn test_meta_transaction_id_deterministic() {
        let mt = MetaTransaction {
            sender: [1u8; 32], fee_payer: [2u8; 32], inner_tx_id: [0xAA; 32],
            relayer_max_fee: 1000, relayer_tip: 0, deadline_epoch: 50, nonce: 1,
        };
        let id1 = mt.compute_id();
        let id2 = mt.compute_id();
        assert_eq!(id1, id2);

        let mt2 = MetaTransaction { nonce: 2, ..mt };
        assert_ne!(id1, mt2.compute_id());
    }

    // ── Feature 13: VRF Randomness ───────────────────────────────────────

    #[test]
    fn test_epoch_randomness_deterministic() {
        let rng = EpochRandomness::new(42, [0xAB; 32]);
        let a = rng.random_bytes32("lottery");
        let b = rng.random_bytes32("lottery");
        assert_eq!(a, b); // same input → same output

        let c = rng.random_bytes32("auction");
        assert_ne!(a, c); // different domain → different output
    }

    #[test]
    fn test_epoch_randomness_range() {
        let rng = EpochRandomness::new(100, [0xCD; 32]);
        for i in 0..100 {
            let v = rng.random_u64(&format!("test_{}", i), 10);
            assert!(v < 10);
        }
    }

    #[test]
    fn test_epoch_randomness_shuffle() {
        let rng = EpochRandomness::new(7, [0xEF; 32]);
        let items = vec![1, 2, 3, 4, 5, 6, 7, 8];
        let shuffled = rng.shuffle("deck", &items);
        assert_eq!(shuffled.len(), items.len());

        // Same epoch + domain → same shuffle
        let shuffled2 = rng.shuffle("deck", &items);
        assert_eq!(shuffled, shuffled2);

        // Different domain → different shuffle
        let shuffled3 = rng.shuffle("other", &items);
        assert_ne!(shuffled, shuffled3);
    }

    #[test]
    fn test_contract_random() {
        let rng = EpochRandomness::new(50, [0x11; 32]);
        let cid = [0xCC; 32];
        let r1 = rng.contract_random(&cid, 0);
        let r2 = rng.contract_random(&cid, 1);
        assert_ne!(r1, r2); // different nonce → different random
    }

    // ── Feature 14: Commit-Reveal ────────────────────────────────────────

    #[test]
    fn test_commit_reveal_happy_path() {
        let mut reg = CommitRevealRegistry::new();
        let committer = [1u8; 32];
        let value = b"my_bid_100".to_vec();
        let salt = b"random_salt_xyz".to_vec();

        let commitment = CommitRevealRecord::compute_commitment(&value, &salt);
        let record = CommitRevealRecord::commit(committer, commitment, 10, 20);
        let id = reg.commit(record).unwrap();

        // Before reveal
        assert!(!reg.get(&id).unwrap().revealed);

        // Reveal at epoch 15 (within deadline)
        let revealed = reg.reveal(&id, value.clone(), salt.clone(), 15).unwrap();
        assert!(revealed.revealed);
        assert_eq!(revealed.revealed_value.as_ref().unwrap(), &value);
    }

    #[test]
    fn test_commit_reveal_wrong_value() {
        let mut reg = CommitRevealRegistry::new();
        let committer = [1u8; 32];
        let value = b"real_value".to_vec();
        let salt = b"salt".to_vec();
        let wrong_value = b"fake_value".to_vec();

        let commitment = CommitRevealRecord::compute_commitment(&value, &salt);
        let record = CommitRevealRecord::commit(committer, commitment, 10, 20);
        let id = reg.commit(record).unwrap();

        assert!(reg.reveal(&id, wrong_value, salt, 15).is_err());
    }

    #[test]
    fn test_commit_reveal_expired() {
        let mut reg = CommitRevealRegistry::new();
        let committer = [1u8; 32];
        let value = b"value".to_vec();
        let salt = b"salt".to_vec();

        let commitment = CommitRevealRecord::compute_commitment(&value, &salt);
        let record = CommitRevealRecord::commit(committer, commitment, 10, 20);
        let id = reg.commit(record).unwrap();

        // Try to reveal after deadline
        assert!(reg.reveal(&id, value, salt, 25).is_err());

        // Prune expired
        assert_eq!(reg.prune_expired(25), 1);
        assert!(reg.get(&id).is_none());
    }

    #[test]
    fn test_commit_reveal_no_double_reveal() {
        let mut reg = CommitRevealRegistry::new();
        let committer = [1u8; 32];
        let value = b"value".to_vec();
        let salt = b"salt".to_vec();

        let commitment = CommitRevealRecord::compute_commitment(&value, &salt);
        let record = CommitRevealRecord::commit(committer, commitment, 10, 20);
        let id = reg.commit(record).unwrap();

        reg.reveal(&id, value.clone(), salt.clone(), 15).unwrap();
        assert!(reg.reveal(&id, value, salt, 16).is_err()); // already revealed
    }
}
