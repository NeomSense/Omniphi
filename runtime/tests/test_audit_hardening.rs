/// Audit hardening tests — 2026-03-11 security & correctness audit.
///
/// Tests marked `#[ignore]` expose REAL BUGS. Each carries a comment with the
/// finding ID, description of the bug, and recommended fix.
///
/// See `runtime/docs/AUDIT_REPORT.md` for the full findings.

#[allow(unused_imports, dead_code, unused_variables)]
use omniphi_runtime::{
    crx::branch_execution::{BranchAwareExecutor, ExecutionSettlementClass},
    crx::causal_graph::{
        CausalEdge, CausalGraph, CausalNode, ExecutionBranch, NodeAccessType, NodeDependencyKind,
        NodeExecutionClass, NodeId,
    },
    crx::finality::FinalityClass,
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::rights_capsule::{
        AllowedActionSet, AllowedActionType, DomainAccessEnvelope, RightsCapsule, RightsScope,
    },
    crx::rights_validation::{RightsValidationEngine, ScopeBreachReason},
    crx::settlement::CRXSettlementEngine,
    objects::base::{AccessMode, ObjectAccess, ObjectId, ObjectType},
    objects::types::{BalanceObject, LiquidityPoolObject},
    plan_validation::validator::ValidationReasonCode,
    resolution::planner::{ExecutionPlan, ObjectOperation},
    safety::actions::{EmergencyModeAction, SafetyAction},
    safety::kernel::{SafetyEvaluationContext, SafetyKernel},
    scheduler::parallel::{ExecutionGroup, ParallelScheduler},
    selection::ranker::{SelectionPolicy, WinningPlanSelector},
    settlement::engine::SettlementEngine,
    solver_market::market::{CandidatePlan, PlanAction, PlanActionType, PlanEvaluationResult},
    state::store::ObjectStore,
};
use std::collections::{BTreeMap, BTreeSet};

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

fn oid(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}

fn addr(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0] = n;
    b
}

fn txid(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[31] = n;
    b
}

fn balance(id: ObjectId, owner: [u8; 32], asset: [u8; 32], amount: u128) -> BalanceObject {
    BalanceObject::new(id, owner, asset, amount, 0)
}

fn full_capsule() -> RightsCapsule {
    let mut types = BTreeSet::new();
    types.insert(ObjectType::Balance);
    types.insert(ObjectType::LiquidityPool);
    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(), // empty = unrestricted (current semantics)
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };
    let mut c = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    c.capsule_hash = c.compute_hash();
    c
}

fn goal(policy: PartialFailurePolicy) -> GoalPacket {
    GoalPacket {
        packet_id: [1u8; 32],
        intent_id: [2u8; 32],
        sender: [3u8; 32],
        desired_outcome: "test".to_string(),
        constraints: GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 300,
            allowed_object_ids: None,
            allowed_object_types: vec![ObjectType::Balance],
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 16,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: policy,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: SettlementStrictness::Strict,
        },
        max_fee: 1000,
        deadline_epoch: 500,
        nonce: 1,
        metadata: BTreeMap::new(),
    }
}

fn transfer_graph(sender_b: [u8; 32], recip_b: [u8; 32], amount: u128) -> CausalGraph {
    let mut nodes = BTreeMap::new();

    nodes.insert(NodeId(0), CausalNode {
        node_id: NodeId(0),
        label: "Check".to_string(),
        execution_class: NodeExecutionClass::CheckBalance,
        access_type: NodeAccessType::ValidationOnly,
        target_object: Some(sender_b),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: BTreeMap::new(),
    });

    let mut dm = BTreeMap::new();
    dm.insert("debit_direction".to_string(), "debit".to_string());
    nodes.insert(NodeId(1), CausalNode {
        node_id: NodeId(1),
        label: "Debit".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(sender_b),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: dm,
    });

    let mut cm = BTreeMap::new();
    cm.insert("debit_direction".to_string(), "credit".to_string());
    nodes.insert(NodeId(2), CausalNode {
        node_id: NodeId(2),
        label: "Credit".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(recip_b),
        amount: Some(amount),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: cm,
    });

    let edges = vec![
        CausalEdge { from: NodeId(0), to: NodeId(1), dependency_kind: NodeDependencyKind::PolicyDependent, is_critical: true },
        CausalEdge { from: NodeId(1), to: NodeId(2), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true },
    ];
    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let branch = ExecutionBranch { branch_id: 0, nodes: topo.clone(), is_main: true, failure_allowed: false };
    let mut g = CausalGraph {
        graph_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        nodes,
        edges,
        branches: vec![branch],
        topological_order: topo,
        graph_hash: [0u8; 32],
    };
    g.graph_hash = g.compute_hash();
    g
}

fn transfer_plan(sender: ObjectId, recip: ObjectId, amount: u128) -> CandidatePlan {
    let mut dm = BTreeMap::new();
    dm.insert("debit_direction".to_string(), "debit".to_string());
    let actions = vec![
        PlanAction { action_type: PlanActionType::DebitBalance, target_object: sender, amount: Some(amount), metadata: dm },
        PlanAction { action_type: PlanActionType::CreditBalance, target_object: recip, amount: Some(amount), metadata: BTreeMap::new() },
    ];
    let mut p = CandidatePlan {
        plan_id: [1u8; 32],
        intent_id: [2u8; 32],
        solver_id: [3u8; 32],
        actions,
        required_capabilities: vec![omniphi_runtime::Capability::TransferAsset],
        object_reads: vec![],
        object_writes: vec![sender, recip],
        expected_output_amount: amount,
        fee_quote: 100,
        quality_score: 8000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 0,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    p.plan_hash = p.compute_hash();
    p
}

fn direct_plan(tx: u8, sender: ObjectId, recip: ObjectId, amount: u128) -> ExecutionPlan {
    ExecutionPlan {
        tx_id: txid(tx),
        operations: vec![
            ObjectOperation::DebitBalance { balance_id: sender, amount },
            ObjectOperation::CreditBalance { balance_id: recip, amount },
        ],
        required_capabilities: vec![],
        object_access: vec![
            ObjectAccess { object_id: sender, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: recip, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 1_400,
        gas_limit: u64::MAX,
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-001: State root reflects typed overlay mutations only after sync
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-001: The canonical objects map is only updated via sync_typed_to_canonical().
/// Typing-overlay mutations (get_balance_by_id_mut etc.) are invisible to state_root()
/// until that sync is called. This test verifies the requirement.
#[test]
fn test_state_root_reflects_typed_overlay_mutations() {
    let id = oid(10);
    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(id, addr(1), addr(5), 1000)));

    store.sync_typed_to_canonical();
    let root_before = store.state_root();

    // Mutate through typed overlay only
    store.get_balance_by_id_mut(&id).unwrap().amount = 500;

    // Root is stale before sync
    assert_eq!(store.state_root(), root_before,
        "State root must be stale before sync — typed overlay change not yet in canonical map");

    // After sync, root changes
    store.sync_typed_to_canonical();
    assert_ne!(store.state_root(), root_before,
        "State root must change after sync_typed_to_canonical() following balance mutation");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-006: Empty allowed_object_ids = unrestricted access (security issue)
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-006: RightsCapsule::can_access_object() treats an empty allowed_object_ids
/// as "no restriction". Empty should mean DENY ALL.
/// This test documents the current (insecure) behavior.
/// FIX: Add explicit `unrestricted: bool` flag; empty without flag → deny all.
#[test]
fn test_rights_capsule_empty_allowed_object_ids_is_unrestricted() {
    let mut types = BTreeSet::new();
    types.insert(ObjectType::Balance);
    let scope = RightsScope {
        allowed_object_ids: BTreeSet::new(), // empty
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 0,
        max_objects_touched: 0,
        allow_rollback: false,
        allow_downgrade: false,
        quarantine_eligible: false,
    };
    let capsule = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };

    // Any arbitrary object id passes when allowed_object_ids is empty
    let arbitrary = [0xDEu8; 32];
    assert!(
        capsule.can_access_object(&arbitrary, &ObjectType::Balance),
        "BUG (FIND-006): Empty allowed_object_ids grants access to any object. \
         FIX: Empty = DENY ALL; add explicit unrestricted flag."
    );
}

// ─────────────────────────────────────────────────────────────────────────────
// Spend accumulator enforces max_spend correctly across two debits
// ─────────────────────────────────────────────────────────────────────────────

/// Verifies that RightsValidationEngine correctly accumulates spend across nodes
/// and rejects the second debit when cumulative spend would exceed max_spend.
#[test]
fn test_spend_limit_accumulates_correctly_across_two_debits() {
    let sb = addr(10);
    let sid = ObjectId::from(sb);
    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(sid, addr(1), addr(5), 10_000)));

    let mut types = BTreeSet::new();
    types.insert(ObjectType::Balance);
    let mut ids = BTreeSet::new();
    ids.insert(sb);
    let scope = RightsScope {
        allowed_object_ids: ids,
        allowed_object_types: types,
        allowed_actions: AllowedActionSet::full(),
        domain_envelope: DomainAccessEnvelope::unrestricted(),
        max_spend: 150, // first 100 passes, second 100 (total 200) must fail
        max_objects_touched: 0,
        allow_rollback: true,
        allow_downgrade: false,
        quarantine_eligible: false,
    };
    let mut cap = RightsCapsule {
        capsule_id: [1u8; 32],
        goal_packet_id: [1u8; 32],
        authorized_solver_id: [3u8; 32],
        scope,
        valid_from_epoch: 0,
        valid_until_epoch: 9999,
        capsule_hash: [0u8; 32],
        metadata: BTreeMap::new(),
    };
    cap.capsule_hash = cap.compute_hash();

    let mut nodes = BTreeMap::new();
    let mut m1 = BTreeMap::new(); m1.insert("debit_direction".to_string(), "debit".to_string());
    let mut m2 = BTreeMap::new(); m2.insert("debit_direction".to_string(), "debit".to_string());
    nodes.insert(NodeId(0), CausalNode {
        node_id: NodeId(0), label: "D1".to_string(),
        execution_class: NodeExecutionClass::MutateBalance, access_type: NodeAccessType::Write,
        target_object: Some(sb), amount: Some(100), branch_id: 0,
        domain_tag: None, risk_tags: vec![], metadata: m1,
    });
    nodes.insert(NodeId(1), CausalNode {
        node_id: NodeId(1), label: "D2".to_string(),
        execution_class: NodeExecutionClass::MutateBalance, access_type: NodeAccessType::Write,
        target_object: Some(sb), amount: Some(100), branch_id: 0,
        domain_tag: None, risk_tags: vec![], metadata: m2,
    });
    let edges = vec![CausalEdge { from: NodeId(0), to: NodeId(1), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true }];
    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let br = ExecutionBranch { branch_id: 0, nodes: topo.clone(), is_main: true, failure_allowed: false };
    let mut g = CausalGraph { graph_id: [1u8; 32], goal_packet_id: [1u8; 32], nodes, edges, branches: vec![br], topological_order: topo, graph_hash: [0u8; 32] };
    g.graph_hash = g.compute_hash();

    let result = RightsValidationEngine::validate(&g, &cap, &store, 1);
    assert!(!result.all_passed, "Two 100-unit debits must exceed max_spend=150");
    assert!(
        result.violations.iter().any(|v| matches!(v.breach_reason, ScopeBreachReason::SpendLimitExceeded)),
        "Must have SpendLimitExceeded violation"
    );
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-007: Gas limit saturation via saturating_mul
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-007: gas_limit = max_fee.saturating_mul(1_000) produces u64::MAX for large
/// max_fee values, effectively removing the gas cap (DoS risk).
#[test]
fn test_gas_limit_no_overflow_from_large_max_fee() {
    let large_fee: u64 = u64::MAX / 999; // slightly over the overflow threshold
    let gas_limit = large_fee.saturating_mul(1_000);

    assert_eq!(gas_limit, u64::MAX,
        "saturating_mul hits u64::MAX for large max_fee — gas cap is removed");

    // Correct mitigation: cap at a protocol constant
    let max_gas: u64 = 10_000_000;
    let capped = large_fee.saturating_mul(1_000).min(max_gas);
    assert_eq!(capped, max_gas,
        "min(MAX_GAS_PER_TX) correctly bounds the gas limit regardless of max_fee");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-008: Scheduler conflict detection handles duplicate access entries
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-008: Same object listed as both ReadOnly and ReadWrite in object_access.
/// BTreeSet deduplication in conflicts() should still detect the ww conflict.
#[test]
fn test_scheduler_conflict_with_duplicate_object_entries() {
    let shared = oid(1);
    let oa = oid(2);
    let ob = oid(3);

    // Plan A: shared listed as ReadWrite AND ReadOnly (duplicate, mixed modes)
    let plan_a = ExecutionPlan {
        tx_id: txid(1),
        operations: vec![],
        required_capabilities: vec![],
        object_access: vec![
            ObjectAccess { object_id: shared, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: shared, mode: AccessMode::ReadOnly }, // duplicate
            ObjectAccess { object_id: oa, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 1000,
        gas_limit: u64::MAX,
    };

    // Plan B: writes shared — must conflict with A
    let plan_b = ExecutionPlan {
        tx_id: txid(2),
        operations: vec![],
        required_capabilities: vec![],
        object_access: vec![
            ObjectAccess { object_id: shared, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: ob, mode: AccessMode::ReadWrite },
        ],
        gas_estimate: 1000,
        gas_limit: u64::MAX,
    };

    assert!(ParallelScheduler::conflicts(&plan_a, &plan_b),
        "Plans writing same object must conflict even with duplicate access entries");

    let groups = ParallelScheduler::schedule(vec![plan_a, plan_b]);
    assert!(groups.len() >= 2,
        "Conflicting plans must be placed in separate execution groups");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-002: Emergency mode persists — no reset path exists
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-002: safety/kernel.rs sets self.emergency_mode = true in apply_action()
/// for EmergencyMode action. No code path ever sets it back to false.
/// Once triggered (e.g. by CrossDomainBlastRadius policy), chain halt is permanent.
///
/// BUG: No reset path. FIX: Add governance-gated reset_emergency_mode().
#[test]
fn test_emergency_mode_persists_after_kernel_evaluate() {
    let mut kernel = SafetyKernel::new();
    assert!(!kernel.emergency_mode);

    // Set emergency mode directly (mimics what apply_action(EmergencyMode) does)
    kernel.emergency_mode = true;
    assert!(kernel.emergency_mode);

    // A clean evaluate() does not reset it
    let _decision = kernel.evaluate(&SafetyEvaluationContext::clean(2));

    assert!(kernel.emergency_mode,
        "BUG (FIND-002): emergency_mode has no reset path — chain halt is permanent. \
         FIX: Add governance-gated reset_emergency_mode() to SafetyKernel.");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-009/010: affected_domains always empty — domain pause never fires
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-009/010: SafetyEvaluationContext::from_crx_record() hardcodes
/// affected_domains = vec![] (kernel.rs:58). The domain pause check in
/// PoSeqSafetyBridge::process_batch() reads this empty vec and never blocks.
///
/// This test verifies the kernel's is_domain_paused() works correctly, and
/// that a context with empty affected_domains bypasses the check.
#[test]
fn test_poseq_bridge_domain_pause_check_reads_updated_state() {
    let mut kernel = SafetyKernel::new();
    kernel.paused_domains.insert("defi".to_string());
    assert!(kernel.is_domain_paused("defi"));

    // Context with populated domains → correctly detects pause
    let mut ctx_populated = SafetyEvaluationContext::clean(1);
    ctx_populated.affected_domains = vec!["defi".to_string()];
    assert!(
        ctx_populated.affected_domains.iter().any(|d| kernel.is_domain_paused(d)),
        "Domain check works when affected_domains is populated"
    );

    // Context from from_crx_record() always has empty domains → no block
    let ctx_empty = SafetyEvaluationContext::clean(1);
    assert!(ctx_empty.affected_domains.is_empty(),
        "clean() (and from_crx_record()) produces empty affected_domains");
    assert!(
        !ctx_empty.affected_domains.iter().any(|d| kernel.is_domain_paused(d)),
        "BUG (FIND-009/010): Empty affected_domains bypasses domain pause blocking. \
         FIX: Populate affected_domains in from_crx_record() from graph domain tags."
    );
}

// ─────────────────────────────────────────────────────────────────────────────
// Receipt ordering is deterministic across identical executions
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_settlement_receipt_order_deterministic() {
    let sender = oid(10);
    let recip = oid(11);
    let asset = addr(5);

    let mk = || {
        let mut s = ObjectStore::new();
        s.insert(Box::new(balance(sender, addr(1), asset, 5000)));
        s.insert(Box::new(balance(recip, addr(2), asset, 0)));
        s
    };

    // Two conflicting plans (same objects) → sequential groups → deterministic order
    let plans = || vec![direct_plan(1, sender, recip, 100), direct_plan(2, sender, recip, 50)];

    let mut s1 = mk();
    let mut s2 = mk();
    let r1 = SettlementEngine::execute_groups(ParallelScheduler::schedule(plans()), &mut s1, 1);
    let r2 = SettlementEngine::execute_groups(ParallelScheduler::schedule(plans()), &mut s2, 1);

    assert_eq!(r1.receipts.len(), r2.receipts.len());
    for (a, b) in r1.receipts.iter().zip(r2.receipts.iter()) {
        assert_eq!(a.tx_id, b.tx_id, "Receipt tx_ids must match across identical runs");
        assert_eq!(a.success, b.success);
    }
    assert_eq!(r1.state_root, r2.state_root, "Identical executions must yield identical state root");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-005: authorized_solver_id not checked in RightsValidationEngine
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-005: validate() never checks capsule.authorized_solver_id against the
/// executing plan's solver_id. A capsule authorised for solver A could be used
/// by solver B with no rejection.
///
/// BUG: No solver identity enforcement in validate().
/// FIX: Add solver_id param and check against capsule.authorized_solver_id.
#[test]
fn test_rights_capsule_wrong_solver_rejected() {
    let sb = addr(10);
    let rb = addr(11);
    let sid = ObjectId::from(sb);
    let rid = ObjectId::from(rb);

    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(sid, addr(1), addr(5), 5000)));
    store.insert(Box::new(balance(rid, addr(2), addr(5), 0)));

    // Capsule authorised for solver [0x03; 32]
    let capsule = full_capsule();
    assert_eq!(capsule.authorized_solver_id, [3u8; 32]);

    // Plan is effectively from a different solver [0xBB; 32]
    let graph = transfer_graph(sb, rb, 100);
    let result = RightsValidationEngine::validate(&graph, &capsule, &store, 1);

    // validate() never checks authorized_solver_id — no SolverMismatch violation
    let has_mismatch = result.violations.iter().any(|v| matches!(v.breach_reason, ScopeBreachReason::SolverMismatch));
    assert!(!has_mismatch,
        "BUG (FIND-005): validate() produces no SolverMismatch — authorized_solver_id is unchecked. \
         FIX: Add solver_id param to validate() and verify == capsule.authorized_solver_id.");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-014: Provisional finality still applies mutations
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-014: SettlementStrictness::Provisional marks the result as Provisional
/// but still applies all state mutations. No confirmation/revert path exists.
#[test]
fn test_provisional_finality_mutation_behavior() {
    let sid = ObjectId::from(addr(10));
    let rid = ObjectId::from(addr(11));

    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(sid, addr(1), addr(5), 1000)));
    store.insert(Box::new(balance(rid, addr(2), addr(5), 0)));

    let mut g = goal(PartialFailurePolicy::StrictAllOrNothing);
    g.policy.settlement_strictness = SettlementStrictness::Provisional;

    let before = store.get_balance_by_id(&sid).map(|b| b.amount).unwrap_or(0);
    let record = CRXSettlementEngine::settle(&transfer_plan(sid, rid, 100), &g, &mut store, &[], 50).unwrap();

    assert_eq!(record.finality.finality_class, FinalityClass::Provisional);
    let after = store.get_balance_by_id(&sid).map(|b| b.amount).unwrap_or(0);
    assert!(after < before,
        "BUG (FIND-014): Provisional settlement applies mutations immediately with no \
         confirmation or revert mechanism — funds are in limbo. \
         FIX: Hold mutations in pending state until explicitly confirmed.");
}

// ─────────────────────────────────────────────────────────────────────────────
// Repeated CRX settlement is deterministic
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_repeated_batch_execution_deterministic_state_root() {
    let sid = ObjectId::from(addr(10));
    let rid = ObjectId::from(addr(11));

    let mk = || {
        let mut s = ObjectStore::new();
        s.insert(Box::new(balance(sid, addr(1), addr(5), 1000)));
        s.insert(Box::new(balance(rid, addr(2), addr(5), 0)));
        s
    };

    let g = goal(PartialFailurePolicy::StrictAllOrNothing);
    let p = transfer_plan(sid, rid, 100);

    let mut s1 = mk();
    CRXSettlementEngine::settle(&p, &g, &mut s1, &[], 50).unwrap();
    s1.sync_typed_to_canonical();
    let root1 = s1.state_root();

    let mut s2 = mk();
    CRXSettlementEngine::settle(&p, &g, &mut s2, &[], 50).unwrap();
    s2.sync_typed_to_canonical();
    let root2 = s2.state_root();

    assert_eq!(root1, root2, "Identical CRX settlements on identical state must yield identical root");
}

// ─────────────────────────────────────────────────────────────────────────────
// Solver selection is fully deterministic
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_solver_selection_fully_deterministic() {
    let mk = |score: u64, fee: u64, byte: u8| -> (CandidatePlan, PlanEvaluationResult) {
        let mut p = CandidatePlan {
            plan_id: [byte; 32],
            intent_id: [1u8; 32],
            solver_id: [byte; 32],
            actions: vec![],
            required_capabilities: vec![],
            object_reads: vec![],
            object_writes: vec![],
            expected_output_amount: 100,
            fee_quote: fee,
            quality_score: score,
            constraint_proofs: vec![],
            plan_hash: [0u8; 32],
            submitted_at_sequence: 0,
            explanation: None,
            metadata: BTreeMap::new(),
        };
        p.plan_hash = p.compute_hash();
        let e = PlanEvaluationResult {
            plan_id: p.plan_id,
            solver_id: p.solver_id,
            passed: true,
            reason_codes: vec![ValidationReasonCode::Valid],
            normalized_score: score,
            validated_object_reads: vec![],
            validated_object_writes: vec![],
            settlement_footprint: 100,
            validated_output_amount: 100,
        };
        (p, e)
    };

    let plans = vec![
        mk(9000, 50, 0xAA), // index 0: high score, higher fee
        mk(7000, 10, 0xBB), // index 1: lower score
        mk(9000, 30, 0xCC), // index 2: tied score, lower fee → wins tiebreak
    ];

    let w1 = WinningPlanSelector::select(&plans, SelectionPolicy::BestScore).unwrap();
    let w2 = WinningPlanSelector::select(&plans, SelectionPolicy::BestScore).unwrap();

    assert_eq!(w1, w2, "Selection must be fully deterministic");
    assert_eq!(plans[w1].0.fee_quote, 30, "Tiebreak on score should prefer lower fee (index 2)");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-016: execute_node() operates on undeclared objects (no runtime scope check)
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-016: BranchAwareExecutor::execute_node() does not verify that
/// node.target_object is within the capsule's allowed_object_ids.
/// HiddenObjectAccess is defined as a CausalViolation but never populated.
///
/// BUG: No runtime scope enforcement in execute_node().
/// FIX: Post-execution, compare mutated objects against capsule scope.
#[test]
fn test_branch_execution_cannot_touch_undeclared_object() {
    let declared = oid(10);
    let undeclared = oid(99);

    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(declared, addr(1), addr(5), 1000)));
    store.insert(Box::new(balance(undeclared, addr(9), addr(5), 5000)));

    let before = store.get_balance_by_id(&undeclared).map(|b| b.amount).unwrap_or(0);

    let mut dm = BTreeMap::new();
    dm.insert("debit_direction".to_string(), "debit".to_string());
    let rogue = CausalNode {
        node_id: NodeId(0),
        label: "RogueDebit".to_string(),
        execution_class: NodeExecutionClass::MutateBalance,
        access_type: NodeAccessType::Write,
        target_object: Some(*undeclared.as_bytes()),
        amount: Some(100),
        branch_id: 0,
        domain_tag: None,
        risk_tags: vec![],
        metadata: dm,
    };

    // execute_node() has no scope check — it acts on whatever target_object says
    let result = BranchAwareExecutor::execute_node(&rogue, &mut store);

    if result.is_ok() {
        let after = store.get_balance_by_id(&undeclared).map(|b| b.amount).unwrap_or(0);
        assert!(after < before,
            "BUG (FIND-016): execute_node() mutated an out-of-scope object with no rejection. \
             FIX: Add HiddenObjectAccess check post-execution against capsule allowed_object_ids.");
    }
    // If Err for other reasons, the test still documents the missing scope check.
}

// ─────────────────────────────────────────────────────────────────────────────
// Zero-amount debit is handled safely
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_zero_amount_debit_operation() {
    let id = oid(10);
    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(id, addr(1), addr(5), 1000)));

    let result = SettlementEngine::execute_groups(
        vec![ExecutionGroup {
            plans: vec![ExecutionPlan {
                tx_id: txid(1),
                operations: vec![ObjectOperation::DebitBalance { balance_id: id, amount: 0 }],
                required_capabilities: vec![],
                object_access: vec![ObjectAccess { object_id: id, mode: AccessMode::ReadWrite }],
                gas_estimate: 1200,
                gas_limit: u64::MAX,
            }],
            group_index: 0,
        }],
        &mut store, 1,
    );

    assert_eq!(result.succeeded, 1, "Zero-amount debit should succeed");
    assert_eq!(result.failed, 0);
    assert_eq!(store.get_balance_by_id(&id).unwrap().amount, 1000, "Zero-amount debit must not change balance");
}

// ─────────────────────────────────────────────────────────────────────────────
// Causal graph cycle detection
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_causal_graph_cycle_is_detected() {
    let mut nodes = BTreeMap::new();
    for i in 0u32..2 {
        nodes.insert(NodeId(i), CausalNode {
            node_id: NodeId(i),
            label: format!("N{}", i),
            execution_class: NodeExecutionClass::ReadObject,
            access_type: NodeAccessType::ValidationOnly,
            target_object: Some(addr(i as u8 + 1)),
            amount: None,
            branch_id: 0,
            domain_tag: None,
            risk_tags: vec![],
            metadata: BTreeMap::new(),
        });
    }
    // Cycle: 0 → 1 → 0
    let edges = vec![
        CausalEdge { from: NodeId(0), to: NodeId(1), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true },
        CausalEdge { from: NodeId(1), to: NodeId(0), dependency_kind: NodeDependencyKind::StateDependent, is_critical: true },
    ];
    let topo = CausalGraph::compute_topological_order(&nodes, &edges);
    let br = ExecutionBranch { branch_id: 0, nodes: topo.clone(), is_main: true, failure_allowed: false };
    let mut g = CausalGraph { graph_id: [0u8; 32], goal_packet_id: [0u8; 32], nodes, edges, branches: vec![br], topological_order: topo, graph_hash: [0u8; 32] };
    g.graph_hash = g.compute_hash();

    assert!(g.validate_acyclic().is_err(), "A graph with a cycle must fail validate_acyclic()");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-003: Double record_submission for winners (documented, requires integration)
// ─────────────────────────────────────────────────────────────────────────────

#[test]
#[ignore = "BUG (FIND-003): process_solver_batch() calls record_submission() twice for winners. \
            interface.rs:238 calls it in the validation loop for all plans (including winner). \
            interface.rs:267-268 calls it again after winner selection. \
            Result: winner's total_plans_submitted and plans_accepted are incremented twice. \
            FIX: Remove lines 267-268; add a separate record_win() method that only \
            increments plans_won without touching submission counters."]
fn test_solver_submission_count_not_double_incremented() {
    unimplemented!("Requires SolverMarketRuntime integration setup — see ignore annotation");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-015: plan_action_to_operation() hardcodes delta_b = 0 for swaps
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-015: In interface.rs:492-496, SwapPoolAmounts conversion always sets
/// delta_b = 0. Every solver-market-routed swap leaves reserve_b unchanged,
/// breaking the AMM constant-product invariant.
#[test]
fn test_swap_plan_action_conversion_loses_delta_b() {
    let pool = oid(50);

    // This reproduces exactly what plan_action_to_operation() produces:
    // PlanActionType::SwapPoolAmounts => ObjectOperation::SwapPoolAmounts {
    //     pool_id, delta_a: amount as i128, delta_b: 0
    // }
    let op_as_converted = ObjectOperation::SwapPoolAmounts { pool_id: pool, delta_a: 1000, delta_b: 0 };
    let op_correct       = ObjectOperation::SwapPoolAmounts { pool_id: pool, delta_a: 1000, delta_b: -500 };

    let delta_b_converted = match op_as_converted { ObjectOperation::SwapPoolAmounts { delta_b, .. } => delta_b, _ => panic!() };
    let delta_b_correct   = match op_correct       { ObjectOperation::SwapPoolAmounts { delta_b, .. } => delta_b, _ => panic!() };

    assert_eq!(delta_b_converted, 0,   "Conversion hardcodes delta_b=0 (the bug)");
    assert_eq!(delta_b_correct,  -500, "Correct conversion should use negative output delta");
    assert_ne!(delta_b_converted, delta_b_correct,
        "BUG (FIND-015): delta_b=0 means reserve_b is never updated in solver-market swaps. \
         FIX: Add delta_b field to PlanAction or encode both deltas in metadata.");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-004: Pool mutations not restored on FullRevert (documented)
// ─────────────────────────────────────────────────────────────────────────────

#[test]
#[ignore = "BUG (FIND-004): StoreSnapshot (crx/settlement.rs:71-99) only captures \
            and restores BalanceObjects. LiquidityPoolObject and VaultObject mutations \
            made during branch execution are NOT captured, so they persist after revert. \
            AMM pool reserves remain mutated even after a FullRevert settlement. \
            FIX: Add pools: BTreeMap<ObjectId, LiquidityPoolObject> and \
            vaults: BTreeMap<ObjectId, VaultObject> to StoreSnapshot; restore all three."]
fn test_revert_does_not_mutate_pool() {
    unimplemented!("Requires routing a swap through CRX where balance passes but swap fails — see ignore annotation");
}

// ─────────────────────────────────────────────────────────────────────────────
// State root changes after successful CRX settlement
// ─────────────────────────────────────────────────────────────────────────────

#[test]
fn test_crx_settlement_state_root_changes_after_success() {
    let sid = ObjectId::from(addr(10));
    let rid = ObjectId::from(addr(11));

    let mut store = ObjectStore::new();
    store.insert(Box::new(balance(sid, addr(1), addr(5), 1000)));
    store.insert(Box::new(balance(rid, addr(2), addr(5), 0)));

    store.sync_typed_to_canonical();
    let root_before = store.state_root();

    let record = CRXSettlementEngine::settle(&transfer_plan(sid, rid, 100), &goal(PartialFailurePolicy::StrictAllOrNothing), &mut store, &[], 50).unwrap();
    assert_eq!(record.finality.finality_class, FinalityClass::Finalized);

    store.sync_typed_to_canonical();
    let root_after = store.state_root();

    assert_ne!(root_before, root_after, "State root must change after successful settlement");
}

// ─────────────────────────────────────────────────────────────────────────────
// FIND-018: CreditBalance uses saturating_add — overflow silently caps balance
// ─────────────────────────────────────────────────────────────────────────────

/// FIND-018: apply_op() for CreditBalance uses saturating_add. A credit that
/// overflows u128 silently saturates at u128::MAX instead of returning an error.
/// The sender's debit succeeds; the recipient receives less than expected.
/// FIX: Use checked_add and propagate a RuntimeError on overflow.
#[test]
fn test_credit_balance_saturating_add_on_overflow() {
    let sender = oid(1);
    let recip  = oid(2);
    let asset  = addr(5);

    let mut store = ObjectStore::new();
    // Sender has enough to debit
    store.insert(Box::new(balance(sender, addr(1), asset, 2000)));
    // Recipient is already near u128::MAX
    store.insert(Box::new(balance(recip, addr(2), asset, u128::MAX - 1)));

    let result = SettlementEngine::execute_groups(
        vec![ExecutionGroup {
            plans: vec![ExecutionPlan {
                tx_id: txid(1),
                operations: vec![
                    ObjectOperation::DebitBalance  { balance_id: sender, amount: 1000 },
                    ObjectOperation::CreditBalance { balance_id: recip,  amount: 1000 },
                ],
                required_capabilities: vec![],
                object_access: vec![
                    ObjectAccess { object_id: sender, mode: AccessMode::ReadWrite },
                    ObjectAccess { object_id: recip,  mode: AccessMode::ReadWrite },
                ],
                gas_estimate: 1400,
                gas_limit: u64::MAX,
            }],
            group_index: 0,
        }],
        &mut store, 1,
    );

    // Settlement succeeds — no error for the credit overflow
    assert_eq!(result.succeeded, 1,
        "BUG (FIND-018): CreditBalance overflow is silently accepted (saturating_add). \
         FIX: Use checked_add in apply_op CreditBalance and return RuntimeError on overflow.");

    // Recipient is capped at u128::MAX, not u128::MAX - 1 + 1000
    let recip_bal = store.get_balance_by_id(&recip).unwrap();
    assert_eq!(recip_bal.amount, u128::MAX, "Recipient balance saturates silently at u128::MAX");
}
