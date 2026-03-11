use omniphi_runtime::{
    crx::branch_execution::ExecutionSettlementClass,
    crx::finality::FinalityClass,
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    crx::settlement::CRXSettlementEngine,
    solver_market::market::{CandidatePlan, PlanAction, PlanActionType},
    BalanceObject, ObjectId, ObjectStore, ObjectType,
};
use std::collections::BTreeMap;

fn make_sender_id() -> [u8; 32] { [10u8; 32] }
fn make_recipient_id() -> [u8; 32] { [11u8; 32] }

fn make_store(sender_amount: u128) -> ObjectStore {
    let mut store = ObjectStore::new();
    let sender = BalanceObject::new(
        ObjectId::from(make_sender_id()),
        [1u8; 32],
        [5u8; 32],
        sender_amount,
        1,
    );
    let recipient = BalanceObject::new(
        ObjectId::from(make_recipient_id()),
        [2u8; 32],
        [5u8; 32],
        0,
        1,
    );
    store.insert(Box::new(sender));
    store.insert(Box::new(recipient));
    store
}

fn make_goal(policy: PartialFailurePolicy) -> GoalPacket {
    GoalPacket {
        packet_id: [1u8; 32],
        intent_id: [2u8; 32],
        sender: [3u8; 32],
        desired_outcome: "test transfer".to_string(),
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

fn make_transfer_plan(amount: u128) -> CandidatePlan {
    let mut debit_meta_action: BTreeMap<String, String> = BTreeMap::new();
    debit_meta_action.insert("debit_direction".to_string(), "debit".to_string());

    let actions = vec![
        PlanAction {
            action_type: PlanActionType::DebitBalance,
            target_object: ObjectId::from(make_sender_id()),
            amount: Some(amount),
            metadata: debit_meta_action,
        },
        PlanAction {
            action_type: PlanActionType::CreditBalance,
            target_object: ObjectId::from(make_recipient_id()),
            amount: Some(amount),
            metadata: BTreeMap::new(),
        },
    ];

    let mut plan = CandidatePlan {
        plan_id: [1u8; 32],
        intent_id: [2u8; 32],
        solver_id: [3u8; 32],
        actions,
        required_capabilities: vec![omniphi_runtime::Capability::TransferAsset],
        object_reads: vec![],
        object_writes: vec![ObjectId::from(make_sender_id()), ObjectId::from(make_recipient_id())],
        expected_output_amount: amount,
        fee_quote: 100,
        quality_score: 8000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 0,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    let hash = plan.compute_hash();
    plan.plan_hash = hash;
    plan
}

#[test]
fn test_settle_transfer_success() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let plan = make_transfer_plan(100);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50);
    assert!(result.is_ok(), "Settlement should succeed: {:?}", result.unwrap_err());

    let record = result.unwrap();
    assert_eq!(record.finality.finality_class, FinalityClass::Finalized);
    assert_eq!(record.settlement_class, ExecutionSettlementClass::FullSuccess);
    assert!(record.gas_used > 0, "gas_used should be > 0, got {}", record.gas_used);
}

#[test]
fn test_settle_produces_version_transitions() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let plan = make_transfer_plan(100);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50).unwrap();

    // After successful settlement, hashes must be valid and non-zero
    assert_ne!(result.capsule_receipt.capsule_hash, [0u8; 32]);
    assert_ne!(result.graph_receipt.graph_hash, [0u8; 32]);
    assert_ne!(result.causal_summary.audit_hash, [0u8; 32]);
}

#[test]
fn test_settle_reverted_does_not_mutate_store() {
    let mut store = make_store(50); // Only 50, but plan tries to debit 200
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let plan = make_transfer_plan(200);

    let initial_sender_balance = {
        store.get_balance_by_id(&ObjectId::from(make_sender_id()))
            .map(|b| b.amount)
            .unwrap_or(0)
    };

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50).unwrap();

    assert_eq!(result.finality.finality_class, FinalityClass::Reverted);

    // Store should be unchanged since we reverted
    let sender_balance_after = store
        .get_balance_by_id(&ObjectId::from(make_sender_id()))
        .map(|b| b.amount)
        .unwrap_or(0);

    assert_eq!(
        sender_balance_after, initial_sender_balance,
        "Store should not be mutated after revert"
    );
}

#[test]
fn test_settle_record_contains_correct_hashes() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let plan = make_transfer_plan(100);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50).unwrap();

    assert_ne!(result.capsule_receipt.capsule_hash, [0u8; 32], "capsule_hash must be non-zero");
    assert_ne!(result.graph_receipt.graph_hash, [0u8; 32], "graph_hash must be non-zero");
    assert_ne!(result.causal_summary.audit_hash, [0u8; 32], "audit_hash must be non-zero");
    assert_eq!(result.goal_packet_id, goal.packet_id);
    assert_eq!(result.solver_id, plan.solver_id);
    assert_eq!(result.plan_id, plan.plan_id);
    assert_eq!(result.epoch, 50);
}

#[test]
fn test_settle_gas_used_nonzero() {
    let mut store = make_store(1000);
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    let plan = make_transfer_plan(100);

    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50).unwrap();
    assert!(result.gas_used > 0, "gas_used must be > 0, got {}", result.gas_used);
}

#[test]
fn test_settle_rejected_when_rights_expire() {
    let mut store = make_store(1000);
    let mut goal = make_goal(PartialFailurePolicy::StrictAllOrNothing);
    // Make deadline_epoch in the past (epoch 50, but deadline is 10)
    goal.deadline_epoch = 10;

    let plan = make_transfer_plan(100);
    let result = CRXSettlementEngine::settle(&plan, &goal, &mut store, &[], 50).unwrap();

    // Capsule valid_until = goal.deadline_epoch = 10, but we're at epoch 50
    // So rights validation fails → Rejected finality
    assert_eq!(result.settlement_class, ExecutionSettlementClass::Rejected);
    assert_eq!(result.finality.finality_class, FinalityClass::Rejected);
}
