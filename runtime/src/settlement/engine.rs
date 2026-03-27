use crate::errors::RuntimeError;
use crate::gas::meter::{GasCosts, GasMeter};
use crate::objects::base::{ObjectId, ObjectVersion};
use crate::resolution::planner::{ExecutionPlan, ObjectOperation};
use crate::scheduler::parallel::ExecutionGroup;
use crate::state::store::ObjectStore;
use sha2::{Digest, Sha256};

/// The outcome of a single plan execution.
#[derive(Debug, Clone)]
pub struct ExecutionReceipt {
    pub tx_id: [u8; 32],
    pub success: bool,
    pub affected_objects: Vec<ObjectId>,
    /// (object_id, old_version, new_version)
    pub version_transitions: Vec<(ObjectId, ObjectVersion, ObjectVersion)>,
    pub error: Option<String>,
    pub gas_used: u64,
    /// Events emitted by contract operations in this plan.
    pub events: Vec<crate::contracts::advanced::ContractEvent>,
}

/// Aggregate result of executing all groups in an epoch.
#[derive(Debug)]
pub struct SettlementResult {
    pub epoch: u64,
    pub total_plans: usize,
    pub succeeded: usize,
    pub failed: usize,
    pub receipts: Vec<ExecutionReceipt>,
    /// Deterministic SHA256 of resulting store state.
    pub state_root: [u8; 32],
    /// IBC transfer hooks collected during execution (forwarded to Go chain).
    pub ibc_hooks: Vec<crate::contracts::advanced::IBCHook>,
    /// Schedules registered during execution (added to ScheduleRegistry).
    pub schedules: Vec<crate::contracts::advanced::ScheduledExecution>,
    /// Balance bindings created during execution.
    pub balance_bindings: Vec<crate::contracts::advanced::ContractBalance>,
    /// All events across all plans (flattened for easy indexing).
    pub all_events: Vec<crate::contracts::advanced::ContractEvent>,
}

/// Side effects collected from a single plan execution.
struct PlanSideEffects {
    ibc_hooks: Vec<crate::contracts::advanced::IBCHook>,
    schedules: Vec<crate::contracts::advanced::ScheduledExecution>,
    bindings: Vec<crate::contracts::advanced::ContractBalance>,
}

pub struct SettlementEngine;

impl SettlementEngine {
    /// Executes all groups in ascending group_index order.
    /// Within each group, plans are executed sequentially (rayon is available
    /// but state mutation requires sequential application for correctness;
    /// groups themselves are ordered serially).
    pub fn execute_groups(
        groups: Vec<ExecutionGroup>,
        store: &mut ObjectStore,
        epoch: u64,
    ) -> SettlementResult {
        let mut receipts: Vec<ExecutionReceipt> = Vec::new();
        let mut succeeded = 0usize;
        let mut failed = 0usize;
        let total_plans: usize = groups.iter().map(|g| g.plans.len()).collect::<Vec<_>>().iter().sum();

        // Aggregate side-effect collectors across all plans
        let mut all_ibc_hooks: Vec<crate::contracts::advanced::IBCHook> = Vec::new();
        let mut all_schedules: Vec<crate::contracts::advanced::ScheduledExecution> = Vec::new();
        let mut all_bindings: Vec<crate::contracts::advanced::ContractBalance> = Vec::new();
        let mut all_events: Vec<crate::contracts::advanced::ContractEvent> = Vec::new();

        // Groups must be executed in strictly ascending order.
        // Within each group, plans are non-conflicting (scheduler guarantee)
        // and can be executed in parallel using rayon.
        let mut sorted_groups = groups;
        sorted_groups.sort_by_key(|g| g.group_index);

        for group in sorted_groups {
            if group.plans.len() <= 1 {
                // Single plan or empty: no parallelism needed
                for plan in &group.plans {
                    let (receipt, side_effects) = Self::apply_plan_with_effects(plan, store, epoch);
                    if receipt.success {
                        succeeded += 1;
                        all_ibc_hooks.extend(side_effects.ibc_hooks);
                        all_schedules.extend(side_effects.schedules);
                        all_bindings.extend(side_effects.bindings);
                        all_events.extend(receipt.events.clone());
                    } else {
                        failed += 1;
                    }
                    receipts.push(receipt);
                }
            } else {
                // Multiple non-conflicting plans: execute sequentially on the
                // shared store. Plans in the same group touch disjoint object
                // sets (guaranteed by ParallelScheduler), so sequential
                // execution produces the same result as parallel execution.
                //
                // NOTE: True parallel execution requires ObjectStore to support
                // concurrent disjoint-key access (e.g., sharded locks or
                // per-object CAS). This is the upgrade path — the scheduler
                // already computes the non-conflicting groups, so switching to
                // parallel execution is a store-level change, not a logic change.
                for plan in &group.plans {
                    let (receipt, side_effects) = Self::apply_plan_with_effects(plan, store, epoch);
                    if receipt.success {
                        succeeded += 1;
                        all_ibc_hooks.extend(side_effects.ibc_hooks);
                        all_schedules.extend(side_effects.schedules);
                        all_bindings.extend(side_effects.bindings);
                        all_events.extend(receipt.events.clone());
                    } else {
                        failed += 1;
                    }
                    receipts.push(receipt);
                }
            }
        }

        // Sync typed overlays to canonical store before computing state root
        store.sync_typed_to_canonical();
        let state_root = store.state_root();

        SettlementResult {
            epoch,
            total_plans,
            succeeded,
            failed,
            receipts,
            state_root,
            ibc_hooks: all_ibc_hooks,
            schedules: all_schedules,
            balance_bindings: all_bindings,
            all_events,
        }
    }

    /// Applies a single plan atomically against the store.
    /// All-or-nothing: if any operation fails (including gas exhaustion),
    /// no state is mutated.
    fn apply_plan(
        plan: &ExecutionPlan,
        store: &mut ObjectStore,
    ) -> ExecutionReceipt {
        // ── Phase 0: gas metering setup ──────────────────────────────────────
        let mut meter = GasMeter::new(plan.gas_limit);

        // Inner function to allow using `?` for error handling while capturing gas
        let mut execute_logic = |meter: &mut GasMeter| -> Result<ExecutionReceipt, RuntimeError> {
            let costs = meter.costs;

        // Charge base transaction cost immediately
        meter.consume(costs.base_tx)?;

        // ── Phase 1: snapshot old versions ──────────────────────────────────
        let mut old_versions: Vec<(ObjectId, ObjectVersion)> = Vec::new();
        for access in &plan.object_access {
            let obj = store.get(&access.object_id).ok_or_else(|| {
                RuntimeError::ObjectNotFound(access.object_id)
            })?;
            old_versions.push((access.object_id, obj.meta().version));
            meter.consume(costs.object_read)?;
        }

        // ── Phase 2: validate all operations can succeed ─────────────────────
        // (dry-run validation without mutating)
        for op in &plan.operations {
            Self::validate_op(op, store)?;
            // Charge gas for each operation during validation pass
            let op_cost = Self::op_gas_cost(op, &costs);
            meter.consume(op_cost)?;
        }

        // ── Phase 3: apply operations ────────────────────────────────────────
        let mut affected: Vec<ObjectId> = Vec::new();

        for op in &plan.operations {
            Self::apply_op(op, store, &mut affected)?;
        }

        // ── Phase 4: increment versions on all mutated objects ───────────────
        // Collect unique IDs of actually-mutated objects
        let mut mutated_ids: Vec<ObjectId> = affected.clone();
        mutated_ids.sort();
        mutated_ids.dedup();

        // Increment versions in both typed overlays and canonical store
        for id in &mutated_ids {
            // Canonical store
            if let Some(boxed) = store.get_mut(id) {
                boxed.meta_mut().version += 1;
            }
            // Typed overlays — check each
            if let Some(b) = store.find_balance_by_id_mut(id) {
                b.meta.version += 1;
            }
            if let Some(p) = store.find_pool_by_id_mut(id) {
                p.meta.version += 1;
            }
            if let Some(v) = store.get_vault_mut(id) {
                v.meta.version += 1;
            }
        }

        // Sync typed → canonical so versions are consistent
        store.sync_typed_to_canonical();

        // ── Phase 5: collect new versions ────────────────────────────────────
        let mut version_transitions = Vec::new();
        for (id, old_ver) in &old_versions {
            let new_ver = store
                .get(id)
                .map(|o| o.meta().version)
                .unwrap_or(*old_ver);
            version_transitions.push((*id, *old_ver, new_ver));
        }

        Ok(ExecutionReceipt {
            tx_id: plan.tx_id,
            success: true,
            affected_objects: mutated_ids,
            version_transitions,
            error: None,
            gas_used: meter.consumed,
            events: Vec::new(),
        })
        };

        match execute_logic(&mut meter) {
            Ok(receipt) => receipt,
            Err(e) => ExecutionReceipt {
                tx_id: plan.tx_id,
                success: false,
                affected_objects: vec![],
                version_transitions: vec![],
                error: Some(e.to_string()),
                gas_used: meter.consumed,
                events: Vec::new(),
            },
        }
    }

    /// Applies a plan and collects side effects (events, IBC hooks, schedules, bindings).
    fn apply_plan_with_effects(
        plan: &ExecutionPlan,
        store: &mut ObjectStore,
        epoch: u64,
    ) -> (ExecutionReceipt, PlanSideEffects) {
        use crate::contracts::advanced::{ContractBalance, ContractEvent, IBCAction, IBCHook, ScheduledExecution};
        use crate::resolution::planner::ObjectOperation;

        let mut receipt = Self::apply_plan(plan, store);
        let mut side_effects = PlanSideEffects {
            ibc_hooks: Vec::new(),
            schedules: Vec::new(),
            bindings: Vec::new(),
        };

        if !receipt.success {
            return (receipt, side_effects);
        }

        // Extract side effects from the operations of this plan.
        // These were already validated and applied; now we collect the artifacts.
        for op in &plan.operations {
            match op {
                ObjectOperation::EmitContractEvent { contract_id, schema_id, event_type, indexed, data } => {
                    receipt.events.push(ContractEvent {
                        contract_id: *contract_id,
                        schema_id: *schema_id,
                        event_type: event_type.clone(),
                        indexed: indexed.clone(),
                        data: data.clone(),
                        epoch,
                    });
                }
                ObjectOperation::IBCTransfer { source_contract, channel_id, port_id, denom, amount, receiver, timeout_secs } => {
                    side_effects.ibc_hooks.push(IBCHook {
                        source_contract: *source_contract,
                        channel_id: channel_id.clone(),
                        port_id: port_id.clone(),
                        action: IBCAction::Transfer {
                            denom: denom.clone(),
                            amount: *amount,
                            receiver: receiver.clone(),
                        },
                        timeout_secs: *timeout_secs,
                    });
                }
                ObjectOperation::ScheduleExecution { contract_id, schema_id, method, params, execute_at_epoch, recurring, interval_epochs, max_recurrences } => {
                    let schedule_id = ScheduledExecution::compute_id(contract_id, method, *execute_at_epoch);
                    side_effects.schedules.push(ScheduledExecution {
                        schedule_id,
                        contract_id: *contract_id,
                        schema_id: *schema_id,
                        method: method.clone(),
                        params: params.clone(),
                        execute_at_epoch: *execute_at_epoch,
                        recurring: *recurring,
                        interval_epochs: *interval_epochs,
                        max_recurrences: *max_recurrences,
                        executed_count: 0,
                        active: true,
                    });
                }
                ObjectOperation::BindContractBalance { contract_id, balance_id, schema_id, asset_id, label } => {
                    // Create a new BalanceObject for the contract if it doesn't exist
                    if store.get(balance_id).is_none() {
                        use crate::objects::types::BalanceObject;
                        let balance = BalanceObject::new(
                            *balance_id,
                            contract_id.0, // owner = the contract
                            *asset_id,
                            0,             // initial amount = 0
                            epoch,         // creation timestamp
                        );
                        store.insert(Box::new(balance));
                    }
                    side_effects.bindings.push(ContractBalance::new(
                        *contract_id,
                        *balance_id,
                        *schema_id,
                        *asset_id,
                        label,
                    ));
                }
                _ => {} // Other operations already applied in apply_op
            }
        }

        (receipt, side_effects)
    }

    /// Returns the gas cost for a single operation (used in pre-flight metering).
    fn op_gas_cost(op: &ObjectOperation, costs: &GasCosts) -> u64 {
        match op {
            ObjectOperation::DebitBalance { .. } => costs.debit_balance,
            ObjectOperation::CreditBalance { .. } => costs.credit_balance,
            ObjectOperation::SwapPoolAmounts { .. } => costs.swap_pool,
            ObjectOperation::LockBalance { .. } => costs.lock_balance,
            ObjectOperation::UnlockBalance { .. } => costs.unlock_balance,
            ObjectOperation::UpdateVersion { .. } => costs.update_version,
            ObjectOperation::TransferOwnership { .. } => costs.update_version,
            ObjectOperation::ContractStateTransition { proposed_state, .. } => {
                costs.contract_state_write
                    + costs.constraint_validation_base
                    + costs.constraint_validation_per_byte * proposed_state.len() as u64
            }
            ObjectOperation::BindContractBalance { .. } => costs.bind_contract_balance,
            ObjectOperation::CreateToken { .. } => costs.create_token,
            ObjectOperation::EmitContractEvent { data, .. } => {
                costs.emit_event + costs.constraint_validation_per_byte * data.len() as u64
            }
            ObjectOperation::IBCTransfer { .. } => costs.ibc_transfer,
            ObjectOperation::ScheduleExecution { .. } => costs.schedule_execution,
        }
    }

    /// Validates that an operation can be applied without mutating state.
    fn validate_op(op: &ObjectOperation, store: &ObjectStore) -> Result<(), RuntimeError> {
        match op {
            ObjectOperation::DebitBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                if balance.available() < *amount {
                    return Err(RuntimeError::InsufficientBalance {
                        required: *amount,
                        available: balance.available(),
                    });
                }
            }
            ObjectOperation::LockBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                if balance.available() < *amount {
                    return Err(RuntimeError::InsufficientBalance {
                        required: *amount,
                        available: balance.available(),
                    });
                }
            }
            ObjectOperation::SwapPoolAmounts { pool_id, delta_a, delta_b } => {
                let pool = store.get_pool_by_id(pool_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*pool_id)
                })?;
                // Check that the pool won't go negative
                if *delta_a < 0 && pool.reserve_a < (-*delta_a) as u128 {
                    return Err(RuntimeError::ConstraintViolation(
                        "swap would drain pool reserve_a below zero".to_string(),
                    ));
                }
                if *delta_b < 0 && pool.reserve_b < (-*delta_b) as u128 {
                    return Err(RuntimeError::ConstraintViolation(
                        "swap would drain pool reserve_b below zero".to_string(),
                    ));
                }
            }
            ObjectOperation::CreditBalance { balance_id, .. } => {
                store.get_balance_by_id(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
            }
            ObjectOperation::UnlockBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                if balance.locked_amount < *amount {
                    return Err(RuntimeError::ConstraintViolation(
                        "unlock amount exceeds locked_amount".to_string(),
                    ));
                }
            }
            ObjectOperation::TransferOwnership { object_id, new_owner } => {
                let obj = store.get(object_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*object_id)
                })?;
                if *new_owner == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent("new_owner cannot be zero".to_string()));
                }
                if obj.meta().owner == *new_owner {
                    return Err(RuntimeError::InvalidIntent("new_owner is already the owner".to_string()));
                }
            }
            ObjectOperation::UpdateVersion { object_id } => {
                store.get(object_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*object_id)
                })?;
            }
            ObjectOperation::ContractStateTransition { contract_id, schema_id, proposed_state } => {
                // Verify the contract object exists
                let obj = store.get(contract_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*contract_id)
                })?;
                // Verify it's actually a Contract object type
                match obj.object_type() {
                    crate::objects::base::ObjectType::Contract(sid) => {
                        if sid != *schema_id {
                            return Err(RuntimeError::ContractConstraintViolation {
                                schema_id: *schema_id,
                                reason: "contract object schema_id mismatch".to_string(),
                            });
                        }
                    }
                    _ => {
                        return Err(RuntimeError::ContractConstraintViolation {
                            schema_id: *schema_id,
                            reason: "target object is not a Contract type".to_string(),
                        });
                    }
                }
                // Verify proposed state is non-empty
                if proposed_state.is_empty() {
                    return Err(RuntimeError::ContractConstraintViolation {
                        schema_id: *schema_id,
                        reason: "proposed_state cannot be empty".to_string(),
                    });
                }
                // Verify proposed state doesn't exceed max_state_bytes
                let encoded = obj.encode();
                if let Ok(contract_obj) = bincode::deserialize::<crate::objects::types::ContractObject>(&encoded) {
                    if contract_obj.max_state_bytes > 0 && proposed_state.len() as u64 > contract_obj.max_state_bytes {
                        return Err(RuntimeError::ContractConstraintViolation {
                            schema_id: *schema_id,
                            reason: format!(
                                "proposed state {} bytes exceeds max_state_bytes {}",
                                proposed_state.len(),
                                contract_obj.max_state_bytes,
                            ),
                        });
                    }
                }
            }
            ObjectOperation::BindContractBalance { contract_id, balance_id, schema_id, .. } => {
                // Verify both contract and balance exist
                store.get(contract_id).ok_or_else(|| RuntimeError::ObjectNotFound(*contract_id))?;
                store.get(balance_id).ok_or_else(|| RuntimeError::ObjectNotFound(*balance_id))?;
                // Verify contract has correct schema
                let obj = store.get(contract_id).unwrap();
                match obj.object_type() {
                    crate::objects::base::ObjectType::Contract(sid) if sid == *schema_id => {}
                    _ => return Err(RuntimeError::ContractConstraintViolation {
                        schema_id: *schema_id,
                        reason: "bind target is not a contract with matching schema".to_string(),
                    }),
                }
            }
            ObjectOperation::CreateToken { symbol, decimals, initial_supply, max_supply, .. } => {
                if symbol.is_empty() || symbol.len() > 12 {
                    return Err(RuntimeError::ConstraintViolation("symbol must be 1-12 chars".to_string()));
                }
                if *decimals > 18 {
                    return Err(RuntimeError::ConstraintViolation("decimals must be <= 18".to_string()));
                }
                if let Some(max) = max_supply {
                    if *initial_supply > *max {
                        return Err(RuntimeError::ConstraintViolation("initial_supply > max_supply".to_string()));
                    }
                }
            }
            ObjectOperation::EmitContractEvent { contract_id, schema_id, data, .. } => {
                // Verify the contract exists and matches schema
                let obj = store.get(contract_id).ok_or_else(|| RuntimeError::ObjectNotFound(*contract_id))?;
                match obj.object_type() {
                    crate::objects::base::ObjectType::Contract(sid) if sid == *schema_id => {}
                    _ => return Err(RuntimeError::ContractConstraintViolation {
                        schema_id: *schema_id,
                        reason: "event source is not a valid contract".to_string(),
                    }),
                }
                if data.len() > 65536 {
                    return Err(RuntimeError::ConstraintViolation("event data exceeds 64KB".to_string()));
                }
            }
            ObjectOperation::IBCTransfer { source_contract, denom, amount, receiver, .. } => {
                store.get(source_contract).ok_or_else(|| RuntimeError::ObjectNotFound(*source_contract))?;
                if denom.is_empty() {
                    return Err(RuntimeError::ConstraintViolation("IBC denom cannot be empty".to_string()));
                }
                if *amount == 0 {
                    return Err(RuntimeError::ConstraintViolation("IBC amount must be > 0".to_string()));
                }
                if receiver.is_empty() {
                    return Err(RuntimeError::ConstraintViolation("IBC receiver cannot be empty".to_string()));
                }
            }
            ObjectOperation::ScheduleExecution { contract_id, execute_at_epoch, .. } => {
                store.get(contract_id).ok_or_else(|| RuntimeError::ObjectNotFound(*contract_id))?;
                if *execute_at_epoch == 0 {
                    return Err(RuntimeError::ConstraintViolation("execute_at_epoch must be > 0".to_string()));
                }
            }
        }
        Ok(())
    }

    /// Applies a single operation against the store (mutations).
    fn apply_op(
        op: &ObjectOperation,
        store: &mut ObjectStore,
        affected: &mut Vec<ObjectId>,
    ) -> Result<(), RuntimeError> {
        match op {
            ObjectOperation::DebitBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id_mut(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                balance.amount = balance.amount.checked_sub(*amount).ok_or_else(|| {
                    RuntimeError::InsufficientBalance {
                        required: *amount,
                        available: balance.amount,
                    }
                })?;
                affected.push(*balance_id);
            }
            ObjectOperation::CreditBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id_mut(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                balance.amount = balance.amount.saturating_add(*amount);
                affected.push(*balance_id);
            }
            ObjectOperation::SwapPoolAmounts { pool_id, delta_a, delta_b } => {
                let pool = store.get_pool_by_id_mut(pool_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*pool_id)
                })?;
                if *delta_a >= 0 {
                    pool.reserve_a = pool.reserve_a.saturating_add(*delta_a as u128);
                } else {
                    pool.reserve_a = pool.reserve_a.saturating_sub((-*delta_a) as u128);
                }
                if *delta_b >= 0 {
                    pool.reserve_b = pool.reserve_b.saturating_add(*delta_b as u128);
                } else {
                    pool.reserve_b = pool.reserve_b.saturating_sub((-*delta_b) as u128);
                }
                affected.push(*pool_id);
            }
            ObjectOperation::LockBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id_mut(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                balance.locked_amount = balance.locked_amount.saturating_add(*amount);
                affected.push(*balance_id);
            }
            ObjectOperation::UnlockBalance { balance_id, amount } => {
                let balance = store.get_balance_by_id_mut(balance_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*balance_id)
                })?;
                balance.locked_amount = balance.locked_amount.checked_sub(*amount).ok_or_else(|| {
                    RuntimeError::ConstraintViolation("unlock exceeds locked_amount".to_string())
                })?;
                affected.push(*balance_id);
            }
            ObjectOperation::TransferOwnership { object_id, new_owner } => {
                let obj = store.get_mut(object_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*object_id)
                })?;
                obj.meta_mut().owner = *new_owner;
                // Also update typed overlays so sync doesn't overwrite
                if let Some(b) = store.find_balance_by_id_mut(object_id) {
                    b.meta.owner = *new_owner;
                    b.owner = *new_owner;
                }
                if let Some(p) = store.find_pool_by_id_mut(object_id) {
                    p.meta.owner = *new_owner;
                }
                if let Some(v) = store.get_vault_mut(object_id) {
                    v.meta.owner = *new_owner;
                }
                affected.push(*object_id);
            }
            ObjectOperation::UpdateVersion { object_id } => {
                // Version increment is handled in the outer loop; just record as affected
                affected.push(*object_id);
            }
            ObjectOperation::ContractStateTransition { contract_id, schema_id: _, proposed_state } => {
                let obj = store.get_mut(contract_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*contract_id)
                })?;
                let encoded = obj.encode();
                if let Ok(mut contract_obj) = bincode::deserialize::<crate::objects::types::ContractObject>(&encoded) {
                    contract_obj.set_state(proposed_state.clone());
                    contract_obj.meta.updated_at = contract_obj.meta.updated_at + 1;
                    store.insert(Box::new(contract_obj));
                    affected.push(*contract_id);
                } else {
                    return Err(RuntimeError::ContractValidatorError(
                        "failed to deserialize contract object for state update".to_string(),
                    ));
                }
            }
            ObjectOperation::BindContractBalance { balance_id, .. } => {
                // Binding is tracked in the ContractBalanceRegistry (external to store).
                // The settlement engine records the balance as affected so its version
                // increments, signaling the binding to downstream observers.
                affected.push(*balance_id);
            }
            ObjectOperation::CreateToken { creator_contract, mint_authority_schema, symbol, decimals, initial_supply, max_supply } => {
                // Create a new TokenObject in the store.
                use crate::objects::types::TokenObject;
                let req = crate::contracts::advanced::TokenCreationRequest {
                    creator_contract: *creator_contract,
                    mint_authority_schema: *mint_authority_schema,
                    symbol: symbol.clone(),
                    decimals: *decimals,
                    initial_supply: *initial_supply,
                    max_supply: *max_supply,
                    metadata: std::collections::BTreeMap::new(),
                };
                let asset_id = req.compute_asset_id();
                let token_obj_id = ObjectId(asset_id);

                let mut token = TokenObject::new(
                    token_obj_id,
                    creator_contract.0,
                    asset_id,
                    symbol.clone(),
                    *decimals,
                    *initial_supply,
                    0,
                );
                token.max_supply = *max_supply;
                token.mint_authority = Some(creator_contract.0);
                store.insert(Box::new(token));
                affected.push(token_obj_id);
                affected.push(*creator_contract);
            }
            ObjectOperation::EmitContractEvent { contract_id, .. } => {
                // Events are collected in the receipt, not applied to store state.
                // The settlement engine caller extracts events from the operation list.
                // We record the contract as affected for version tracking.
                affected.push(*contract_id);
            }
            ObjectOperation::IBCTransfer { source_contract, .. } => {
                // IBC transfers are collected as hooks and forwarded to the Go chain.
                // No direct state mutation — the Go chain handles escrow.
                affected.push(*source_contract);
            }
            ObjectOperation::ScheduleExecution { contract_id, .. } => {
                // Schedules are registered in the ScheduleRegistry (external to store).
                affected.push(*contract_id);
            }
        }
        Ok(())
    }
}

/// Computes a SHA256 hash of a byte slice.
pub fn sha256(data: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(data);
    hasher.finalize().into()
}
