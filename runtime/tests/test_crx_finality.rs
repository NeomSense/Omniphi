use omniphi_runtime::{
    crx::branch_execution::ExecutionSettlementClass,
    crx::finality::{DomainFinalityPolicy, FinalityClass, FinalityClassifier},
    crx::goal_packet::{
        GoalConstraints, GoalPacket, GoalPolicyEnvelope, PartialFailurePolicy, RiskTier,
        SettlementStrictness,
    },
    ObjectType,
};
use std::collections::BTreeMap;

fn make_goal(policy: PartialFailurePolicy, strictness: SettlementStrictness) -> GoalPacket {
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
            max_objects_touched: 8,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: policy,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: strictness,
        },
        max_fee: 1000,
        deadline_epoch: 500,
        nonce: 1,
        metadata: BTreeMap::new(),
    }
}

#[test]
fn test_full_success_gives_finalized() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::FullSuccess, &goal, &[]);
    assert_eq!(disp.finality_class, FinalityClass::Finalized);
    assert!(!disp.escalation_required);
}

#[test]
fn test_downgrade_gives_finalized_with_downgrade() {
    let goal = make_goal(PartialFailurePolicy::AllowBranchDowngrade, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::SuccessWithDowngrade(0),
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::FinalizedWithDowngrade);
}

#[test]
fn test_downgrade_with_strict_policy_gives_reverted() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::SuccessWithDowngrade(0),
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::Reverted);
}

#[test]
fn test_quarantine_gives_finalized_with_quarantine() {
    let goal = make_goal(PartialFailurePolicy::QuarantineOnFailure, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::SuccessWithQuarantine(vec![[1u8; 32]]),
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::FinalizedWithQuarantine);
}

#[test]
fn test_quarantine_with_strict_policy_gives_reverted() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::SuccessWithQuarantine(vec![[1u8; 32]]),
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::Reverted);
}

#[test]
fn test_revert_gives_reverted() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::FullRevert, &goal, &[]);
    assert_eq!(disp.finality_class, FinalityClass::Reverted);
}

#[test]
fn test_rejected_gives_rejected() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::Rejected, &goal, &[]);
    assert_eq!(disp.finality_class, FinalityClass::Rejected);
}

#[test]
fn test_provisional_strictness_overrides() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Provisional);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::FullSuccess, &goal, &[]);
    // Should override Finalized → Provisional
    assert_eq!(disp.finality_class, FinalityClass::Provisional);
    assert!(disp.provisional_until_epoch.is_some());
}

#[test]
fn test_provisional_does_not_apply_to_rejected() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Provisional);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::Rejected, &goal, &[]);
    // Rejected stays Rejected even with Provisional strictness
    assert_eq!(disp.finality_class, FinalityClass::Rejected);
}

#[test]
fn test_provisional_does_not_apply_to_reverted() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Provisional);
    let disp = FinalityClassifier::classify(&ExecutionSettlementClass::FullRevert, &goal, &[]);
    // Reverted stays Reverted
    assert_eq!(disp.finality_class, FinalityClass::Reverted);
}

#[test]
fn test_domain_policy_escalation() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);

    // Domain policy requires at least FinalizedWithDowngrade; we only get Finalized
    // Wait — Finalized is HIGHER rank than FinalizedWithDowngrade, so should not escalate
    let domain_policies = vec![DomainFinalityPolicy {
        domain: "treasury".to_string(),
        min_finality_class: FinalityClass::Finalized,
        require_audit: true,
    }];

    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::FullSuccess,
        &goal,
        &domain_policies,
    );
    assert_eq!(disp.finality_class, FinalityClass::Finalized);
    assert!(!disp.escalation_required); // Finalized >= Finalized, no escalation
}

#[test]
fn test_domain_policy_escalation_triggered() {
    let goal = make_goal(PartialFailurePolicy::AllowBranchDowngrade, SettlementStrictness::Strict);

    // Domain policy requires Finalized, but we got FinalizedWithDowngrade (lower rank)
    let domain_policies = vec![DomainFinalityPolicy {
        domain: "critical".to_string(),
        min_finality_class: FinalityClass::Finalized,
        require_audit: true,
    }];

    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::SuccessWithDowngrade(0),
        &goal,
        &domain_policies,
    );
    assert_eq!(disp.finality_class, FinalityClass::FinalizedWithDowngrade);
    assert!(disp.escalation_required); // FinalizedWithDowngrade < Finalized
}

#[test]
fn test_partial_success_with_allow_downgrade() {
    let goal = make_goal(PartialFailurePolicy::AllowBranchDowngrade, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::PartialSuccess {
            succeeded_branches: vec![0],
            failed_branches: vec![1],
        },
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::FinalizedWithDowngrade);
}

#[test]
fn test_partial_success_with_strict_reverts() {
    let goal = make_goal(PartialFailurePolicy::StrictAllOrNothing, SettlementStrictness::Strict);
    let disp = FinalityClassifier::classify(
        &ExecutionSettlementClass::PartialSuccess {
            succeeded_branches: vec![0],
            failed_branches: vec![1],
        },
        &goal,
        &[],
    );
    assert_eq!(disp.finality_class, FinalityClass::Reverted);
}
