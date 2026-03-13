use crate::crx::branch_execution::{BranchAwareExecutor, BranchExecutionResult, ExecutionSettlementClass};
use crate::crx::causal_validity::CausalValidityEngine;
use crate::crx::finality::{DomainFinalityPolicy, FinalityClass, FinalityClassifier, SettlementDisposition};
use crate::crx::goal_packet::{GoalPacket, PartialFailurePolicy};
use crate::crx::plan_builder::CausalPlanBuilder;
use crate::crx::rights_capsule::RightsCapsule;
use crate::crx::rights_validation::RightsValidationEngine;
use crate::crx::causal_graph::CausalGraph;
use crate::errors::RuntimeError;
use crate::gas::meter::GasCosts;
use crate::objects::base::ObjectId;
use crate::solver_market::market::CandidatePlan;
use crate::state::store::ObjectStore;
use crate::objects::types::{BalanceObject, LiquidityPoolObject, VaultObject};
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Receipt types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub struct RightsCapsuleReceipt {
    pub capsule_id: [u8; 32],
    pub capsule_hash: [u8; 32],
    /// Human-readable summary of granted scope.
    pub scope_summary: String,
    pub was_respected: bool,
}

#[derive(Debug, Clone)]
pub struct GraphExecutionReceipt {
    pub graph_id: [u8; 32],
    pub graph_hash: [u8; 32],
    pub total_nodes: usize,
    pub executed_nodes: usize,
    pub skipped_nodes: usize,
    pub failed_nodes: usize,
    pub branch_results: Vec<BranchExecutionResult>,
}

#[derive(Debug, Clone)]
pub struct CausalSettlementSummary {
    pub causal_validity: bool,
    pub rights_validity: bool,
    pub violations_count: usize,
    pub audit_hash: [u8; 32],
}

#[derive(Debug, Clone)]
pub struct CRXSettlementRecord {
    pub goal_packet_id: [u8; 32],
    pub intent_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub plan_id: [u8; 32],
    pub finality: SettlementDisposition,
    pub settlement_class: ExecutionSettlementClass,
    pub capsule_receipt: RightsCapsuleReceipt,
    pub graph_receipt: GraphExecutionReceipt,
    pub causal_summary: CausalSettlementSummary,
    pub affected_objects: Vec<[u8; 32]>,
    /// (object_id, old_version, new_version)
    pub version_transitions: Vec<([u8; 32], u64, u64)>,
    pub gas_used: u64,
    pub epoch: u64,
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot for revert support
// ─────────────────────────────────────────────────────────────────────────────

struct StoreSnapshot {
    balances: BTreeMap<ObjectId, BalanceObject>,
    pools: BTreeMap<ObjectId, LiquidityPoolObject>,
    vaults: BTreeMap<ObjectId, VaultObject>,
}

impl StoreSnapshot {
    /// Snapshot all typed objects referenced by the given object IDs.
    fn take(store: &ObjectStore, object_ids: &[ObjectId]) -> Self {
        let mut balances = BTreeMap::new();
        let mut pools = BTreeMap::new();
        let mut vaults = BTreeMap::new();
        for id in object_ids {
            if let Some(b) = store.get_balance_by_id(id) {
                balances.insert(*id, b.clone());
            }
            if let Some(p) = store.get_pool_by_id(id) {
                pools.insert(*id, p.clone());
            }
            if let Some(v) = store.get_vault(id) {
                vaults.insert(*id, v.clone());
            }
        }
        StoreSnapshot { balances, pools, vaults }
    }

    /// Restore all snapshotted typed objects back into the store.
    fn restore(self, store: &mut ObjectStore) {
        for (id, bal) in self.balances {
            if let Some(existing) = store.get_balance_by_id_mut(&id) {
                *existing = bal;
            }
        }
        for (id, pool) in self.pools {
            if let Some(existing) = store.get_pool_by_id_mut(&id) {
                *existing = pool;
            }
        }
        for (id, vault) in self.vaults {
            if let Some(existing) = store.get_vault_mut(&id) {
                *existing = vault;
            }
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Settlement Engine
// ─────────────────────────────────────────────────────────────────────────────

pub struct CRXSettlementEngine;

impl CRXSettlementEngine {
    /// Full CRX settlement pipeline for a single goal.
    pub fn settle(
        plan: &CandidatePlan,
        goal: &GoalPacket,
        store: &mut ObjectStore,
        domain_policies: &[DomainFinalityPolicy],
        epoch: u64,
    ) -> Result<CRXSettlementRecord, RuntimeError> {
        // Step 1: Build CausalGraph + RightsCapsule
        let (graph, capsule) = CausalPlanBuilder::build(plan, goal, store, epoch)?;

        // Step 2: Validate rights (includes solver identity check — FIND-005)
        let rights_result =
            RightsValidationEngine::validate(&graph, &capsule, store, epoch, &plan.solver_id);

        // Step 3: Validate causal legitimacy
        let causal_result = CausalValidityEngine::validate(&graph, &capsule, goal);

        let rights_valid = rights_result.all_passed;
        let causal_valid = causal_result.is_causally_valid;
        let violations_count = rights_result.violations.len() + causal_result.violations.len();

        let causal_summary = CausalSettlementSummary {
            causal_validity: causal_valid,
            rights_validity: rights_valid,
            violations_count,
            audit_hash: causal_result.audit.audit_hash,
        };

        let capsule_receipt = RightsCapsuleReceipt {
            capsule_id: capsule.capsule_id,
            capsule_hash: capsule.capsule_hash,
            scope_summary: format!(
                "actions={}, obj_types={}, max_spend={}",
                capsule.scope.allowed_actions.0.len(),
                capsule.scope.allowed_object_types.len(),
                capsule.scope.max_spend
            ),
            was_respected: rights_valid,
        };

        // If either validation fails → Rejected
        if !rights_valid || !causal_valid {
            let finality = FinalityClassifier::classify(
                &ExecutionSettlementClass::Rejected,
                goal,
                domain_policies,
            );

            let graph_receipt = GraphExecutionReceipt {
                graph_id: graph.graph_id,
                graph_hash: graph.graph_hash,
                total_nodes: graph.nodes.len(),
                executed_nodes: 0,
                skipped_nodes: graph.nodes.len(),
                failed_nodes: violations_count,
                branch_results: vec![],
            };

            return Ok(CRXSettlementRecord {
                goal_packet_id: goal.packet_id,
                intent_id: plan.intent_id,
                solver_id: plan.solver_id,
                plan_id: plan.plan_id,
                finality,
                settlement_class: ExecutionSettlementClass::Rejected,
                capsule_receipt,
                graph_receipt,
                causal_summary,
                affected_objects: vec![],
                version_transitions: vec![],
                gas_used: GasCosts::default_costs().base_tx,
                epoch,
            });
        }

        // Step 4: Snapshot pre-execution state for possible revert
        let all_obj_ids: Vec<ObjectId> = graph
            .nodes
            .values()
            .filter_map(|n| n.target_object.map(ObjectId::from))
            .collect();

        // Snapshot balance objects involved in this plan
        let snapshot = StoreSnapshot::take(store, &all_obj_ids);

        // Record pre-execution versions
        let pre_versions: BTreeMap<ObjectId, u64> = all_obj_ids
            .iter()
            .filter_map(|id| {
                store.get(id).map(|obj| (*id, obj.meta().version))
            })
            .collect();

        // Step 5: Execute
        let (settlement_class, branch_results) =
            BranchAwareExecutor::execute(&graph, &capsule, goal, store);

        // Sync typed overlays back to canonical store after execution
        store.sync_typed_to_canonical();

        // Step 6: Classify finality
        let finality = FinalityClassifier::classify(&settlement_class, goal, domain_policies);

        // Step 7: If Reverted → restore pre-execution state
        let is_reverted = finality.finality_class == FinalityClass::Reverted
            || settlement_class == ExecutionSettlementClass::FullRevert;

        if is_reverted {
            snapshot.restore(store);
            store.sync_typed_to_canonical();
        }

        // Collect affected objects and version transitions
        let affected_objects: Vec<[u8; 32]> = branch_results
            .iter()
            .flat_map(|br| br.mutated_objects.iter().copied())
            .collect::<std::collections::BTreeSet<_>>()
            .into_iter()
            .collect();

        let version_transitions: Vec<([u8; 32], u64, u64)> = if !is_reverted {
            affected_objects
                .iter()
                .map(|obj_bytes| {
                    let id = ObjectId::from(*obj_bytes);
                    let old_v = pre_versions.get(&id).copied().unwrap_or(0);
                    let new_v = store
                        .get(&id)
                        .map(|obj| obj.meta().version)
                        .unwrap_or(old_v);
                    (*obj_bytes, old_v, new_v)
                })
                .collect()
        } else {
            vec![]
        };

        // Count executed/skipped/failed nodes
        let executed_count: usize = branch_results.iter().filter(|br| br.success).count()
            * (graph.nodes.len() / graph.branches.len().max(1));
        let failed_count: usize = branch_results.iter().filter(|br| !br.success).count();

        let graph_receipt = GraphExecutionReceipt {
            graph_id: graph.graph_id,
            graph_hash: graph.graph_hash,
            total_nodes: graph.nodes.len(),
            executed_nodes: if is_reverted { 0 } else { graph.topological_order.len() },
            skipped_nodes: if is_reverted { graph.nodes.len() } else { 0 },
            failed_nodes: failed_count,
            branch_results: branch_results.clone(),
        };

        // Estimate gas: base + per-node cost
        let costs = GasCosts::default_costs();
        let gas_used = costs.base_tx
            + (graph.nodes.len() as u64) * (costs.object_write + costs.object_read);

        Ok(CRXSettlementRecord {
            goal_packet_id: goal.packet_id,
            intent_id: plan.intent_id,
            solver_id: plan.solver_id,
            plan_id: plan.plan_id,
            finality,
            settlement_class,
            capsule_receipt,
            graph_receipt,
            causal_summary,
            affected_objects,
            version_transitions,
            gas_used,
            epoch,
        })
    }
}
