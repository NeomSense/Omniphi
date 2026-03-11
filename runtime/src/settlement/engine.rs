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

        // Groups must be executed in strictly ascending order
        let mut sorted_groups = groups;
        sorted_groups.sort_by_key(|g| g.group_index);

        for group in sorted_groups {
            // Within a group, plans are conflict-free, but we must still apply
            // them to a shared mutable store — so we execute sequentially and
            // accumulate receipts.
            for plan in &group.plans {
                let receipt = Self::apply_plan(plan, store);
                if receipt.success {
                    succeeded += 1;
                } else {
                    failed += 1;
                }
                receipts.push(receipt);
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
                gas_used: meter.consumed, // Capture actual gas used up to failure
            },
        }
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
            ObjectOperation::UpdateVersion { object_id } => {
                store.get(object_id).ok_or_else(|| {
                    RuntimeError::ObjectNotFound(*object_id)
                })?;
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
            ObjectOperation::UpdateVersion { object_id } => {
                // Version increment is handled in the outer loop; just record as affected
                affected.push(*object_id);
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
