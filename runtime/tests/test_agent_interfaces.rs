use omniphi_runtime::agent_interfaces::agent::{
    AgentPolicyEnvelope, AgentSolverProfile, AgentType, DeterministicAgentSubmission,
    ExplanationFormat, PlanExplanationMetadata,
};
use omniphi_runtime::capabilities::checker::CapabilitySet;
use omniphi_runtime::errors::RuntimeError;
use omniphi_runtime::objects::base::{ObjectId, ObjectType};
use omniphi_runtime::solver_market::market::{CandidatePlan, PlanConstraintProof};
use omniphi_runtime::solver_registry::registry::{
    SolverCapabilities, SolverProfile, SolverRegistry, SolverReputationRecord, SolverStatus,
};
use std::collections::BTreeMap;

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

fn oid(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}

fn make_agent_profile(solver_id: [u8; 32], max_value: u128, max_objects: usize, valid_until: u64) -> AgentSolverProfile {
    AgentSolverProfile {
        solver_id,
        agent_type: AgentType::TreasuryAgent,
        policy_envelope: AgentPolicyEnvelope {
            allowed_object_types: vec![ObjectType::Balance, ObjectType::Vault],
            allowed_intent_classes: vec!["swap".to_string(), "treasury_rebalance".to_string()],
            max_value_per_intent: max_value,
            max_objects_per_plan: max_objects,
            require_human_countersign: false,
            valid_until_epoch: valid_until,
        },
        explanation_format: ExplanationFormat::PlainText,
        max_plan_complexity: 10,
    }
}

fn make_plan(solver_id: [u8; 32], output: u128, reads: Vec<ObjectId>, writes: Vec<ObjectId>) -> CandidatePlan {
    let mut plan = CandidatePlan {
        plan_id: txid(50),
        intent_id: txid(1),
        solver_id,
        actions: vec![],
        required_capabilities: vec![],
        object_reads: reads,
        object_writes: writes,
        expected_output_amount: output,
        fee_quote: 100,
        quality_score: 7000,
        constraint_proofs: vec![],
        plan_hash: [0u8; 32],
        submitted_at_sequence: 1,
        explanation: None,
        metadata: BTreeMap::new(),
    };
    plan.plan_hash = plan.compute_hash();
    plan
}

#[test]
fn test_agent_submission_validates_within_policy() {
    let solver_id = addr(20);
    let agent = make_agent_profile(solver_id, 1_000_000, 10, 100);
    let plan = make_plan(solver_id, 500_000, vec![oid(1)], vec![oid(2)]);

    let submission = DeterministicAgentSubmission {
        plan,
        agent_profile: agent,
        policy_check_passed: true,
        explanation_metadata: None,
        submission_epoch: 50,
    };

    let result = submission.validate_against_policy(50);
    assert!(result.is_ok(), "submission within policy should pass: {:?}", result);
}

#[test]
fn test_agent_submission_exceeds_max_value_rejected() {
    let solver_id = addr(21);
    let agent = make_agent_profile(solver_id, 100, 10, 100); // max value = 100
    let plan = make_plan(solver_id, 999_999, vec![], vec![]); // output way above limit

    let submission = DeterministicAgentSubmission {
        plan,
        agent_profile: agent,
        policy_check_passed: false,
        explanation_metadata: None,
        submission_epoch: 50,
    };

    let result = submission.validate_against_policy(50);
    assert!(matches!(result, Err(RuntimeError::PolicyViolation(_))));
}

#[test]
fn test_agent_submission_expired_epoch_rejected() {
    let solver_id = addr(22);
    let agent = make_agent_profile(solver_id, 1_000_000, 10, 10); // valid only until epoch 10
    let plan = make_plan(solver_id, 500, vec![], vec![]);

    let submission = DeterministicAgentSubmission {
        plan,
        agent_profile: agent,
        policy_check_passed: true,
        explanation_metadata: None,
        submission_epoch: 20,
    };

    // current_epoch = 20, valid_until_epoch = 10 → expired
    let result = submission.validate_against_policy(20);
    assert!(matches!(result, Err(RuntimeError::PolicyViolation(_))));
}

#[test]
fn test_explanation_metadata_stored() {
    let solver_id = addr(23);
    let agent = make_agent_profile(solver_id, 1_000_000, 10, 200);
    let plan = make_plan(solver_id, 500, vec![], vec![]);

    let explanation = PlanExplanationMetadata {
        reasoning_summary: "Selected path with lowest gas and highest output".to_string(),
        confidence_bps: 9200,
        inputs_considered: vec!["pool_a".to_string(), "pool_b".to_string()],
        alternatives_rejected: vec!["route_c: insufficient liquidity".to_string()],
        safety_checks_passed: vec!["slippage_check".to_string(), "balance_check".to_string()],
    };

    let submission = DeterministicAgentSubmission {
        plan,
        agent_profile: agent,
        policy_check_passed: true,
        explanation_metadata: Some(explanation.clone()),
        submission_epoch: 100,
    };

    assert!(submission.explanation_metadata.is_some());
    let meta = submission.explanation_metadata.as_ref().unwrap();
    assert_eq!(meta.confidence_bps, 9200);
    assert_eq!(meta.inputs_considered.len(), 2);
    assert_eq!(meta.alternatives_rejected.len(), 1);
    assert_eq!(meta.safety_checks_passed.len(), 2);
}

#[test]
fn test_agent_flagged_as_agent_in_registry() {
    let mut registry = SolverRegistry::new();
    let solver_id = addr(24);

    let profile = SolverProfile {
        solver_id,
        display_name: "AI Treasury Agent".to_string(),
        public_key: addr(124),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: CapabilitySet::all(),
            allowed_intent_classes: vec!["treasury_rebalance".to_string()],
            domain_tags: vec!["treasury".to_string()],
            max_objects_per_plan: 5,
        },
        reputation: SolverReputationRecord::default(),
        stake_amount: 0,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: true,
        metadata: BTreeMap::new(),
    };

    registry.register(profile).unwrap();
    let stored = registry.get(&solver_id).unwrap();
    assert!(stored.is_agent, "solver should be flagged as agent");
}

#[test]
fn test_agent_max_objects_exceeded_rejected() {
    let solver_id = addr(25);
    let agent = make_agent_profile(solver_id, 1_000_000, 2, 100); // max 2 objects
    // Plan has 3 total objects (2 reads + 1 write)
    let plan = make_plan(solver_id, 500, vec![oid(1), oid(2)], vec![oid(3)]);

    let submission = DeterministicAgentSubmission {
        plan,
        agent_profile: agent,
        policy_check_passed: false,
        explanation_metadata: None,
        submission_epoch: 50,
    };

    let result = submission.validate_against_policy(50);
    assert!(matches!(result, Err(RuntimeError::PolicyViolation(_))));
}
