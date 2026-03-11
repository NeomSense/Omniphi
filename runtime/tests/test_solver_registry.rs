use omniphi_runtime::capabilities::checker::CapabilitySet;
use omniphi_runtime::solver_registry::registry::{
    SolverCapabilities, SolverProfile, SolverRegistry, SolverReputationRecord, SolverStatus,
};
use std::collections::BTreeMap;

fn make_id(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[0] = n;
    b
}

fn make_profile(id_byte: u8) -> SolverProfile {
    SolverProfile {
        solver_id: make_id(id_byte),
        display_name: format!("Solver-{}", id_byte),
        public_key: make_id(id_byte + 100),
        status: SolverStatus::Active,
        capabilities: SolverCapabilities {
            capability_set: CapabilitySet::all(),
            allowed_intent_classes: vec!["swap".to_string(), "transfer".to_string()],
            domain_tags: vec!["defi".to_string()],
            max_objects_per_plan: 10,
        },
        reputation: SolverReputationRecord::default(),
        stake_amount: 1_000_000,
        registered_at_epoch: 1,
        last_active_epoch: 1,
        is_agent: false,
        metadata: BTreeMap::new(),
    }
}

#[test]
fn test_register_solver_success() {
    let mut registry = SolverRegistry::new();
    let profile = make_profile(1);
    let result = registry.register(profile);
    assert!(result.is_ok());
    assert_eq!(registry.all_solvers().len(), 1);
}

#[test]
fn test_register_duplicate_solver_fails() {
    let mut registry = SolverRegistry::new();
    let profile1 = make_profile(1);
    let profile2 = make_profile(1); // same id
    registry.register(profile1).unwrap();
    let result = registry.register(profile2);
    assert!(result.is_err());
}

#[test]
fn test_zero_id_solver_rejected() {
    let mut registry = SolverRegistry::new();
    let mut profile = make_profile(1);
    profile.solver_id = [0u8; 32];
    let result = registry.register(profile);
    assert!(result.is_err());
}

#[test]
fn test_set_status_updates_correctly() {
    let mut registry = SolverRegistry::new();
    let profile = make_profile(5);
    let solver_id = profile.solver_id;
    registry.register(profile).unwrap();

    registry.set_status(&solver_id, SolverStatus::Paused).unwrap();
    let p = registry.get(&solver_id).unwrap();
    assert_eq!(p.status, SolverStatus::Paused);

    registry.set_status(&solver_id, SolverStatus::Banned).unwrap();
    let p = registry.get(&solver_id).unwrap();
    assert_eq!(p.status, SolverStatus::Banned);
}

#[test]
fn test_active_solvers_filter() {
    let mut registry = SolverRegistry::new();
    let p1 = make_profile(1); // Active
    let mut p2 = make_profile(2);
    p2.status = SolverStatus::Paused;
    let mut p3 = make_profile(3);
    p3.status = SolverStatus::Pending;

    registry.register(p1).unwrap();
    registry.register(p2).unwrap();
    registry.register(p3).unwrap();

    let active = registry.active_solvers();
    assert_eq!(active.len(), 1);
    assert_eq!(active[0].solver_id, make_id(1));
}

#[test]
fn test_reputation_auto_flag_after_3_consecutive_invalid() {
    let mut registry = SolverRegistry::new();
    let profile = make_profile(7);
    let solver_id = profile.solver_id;
    registry.register(profile).unwrap();

    // 2 invalid submissions — should not flag yet
    registry.record_submission(&solver_id, false, false);
    registry.record_submission(&solver_id, false, false);
    assert!(!registry.get(&solver_id).unwrap().reputation.is_flagged);

    // 3rd consecutive invalid — should auto-flag
    registry.record_submission(&solver_id, false, false);
    assert!(registry.get(&solver_id).unwrap().reputation.is_flagged);
}

#[test]
fn test_record_submission_updates_counts() {
    let mut registry = SolverRegistry::new();
    let profile = make_profile(8);
    let solver_id = profile.solver_id;
    registry.register(profile).unwrap();

    // 2 accepted, 1 rejected, 1 won
    registry.record_submission(&solver_id, true, false);
    registry.record_submission(&solver_id, true, true);
    registry.record_submission(&solver_id, false, false);

    let rep = &registry.get(&solver_id).unwrap().reputation;
    assert_eq!(rep.total_plans_submitted, 3);
    assert_eq!(rep.plans_accepted, 2);
    assert_eq!(rep.plans_rejected, 1);
    assert_eq!(rep.plans_won, 1);
    // After a valid submission, consecutive_invalid resets
    assert_eq!(rep.consecutive_invalid_plans, 1);
    assert_eq!(rep.consecutive_valid_plans, 0);
}

#[test]
fn test_set_status_on_unknown_solver_returns_error() {
    let mut registry = SolverRegistry::new();
    let unknown_id = make_id(99);
    let result = registry.set_status(&unknown_id, SolverStatus::Active);
    assert!(result.is_err());
}

#[test]
fn test_update_reputation_score_clamped_to_10000() {
    let mut registry = SolverRegistry::new();
    let profile = make_profile(3);
    let solver_id = profile.solver_id;
    registry.register(profile).unwrap();

    registry.update_reputation_score(&solver_id, 99999);
    let score = registry.get(&solver_id).unwrap().reputation.reputation_score;
    assert_eq!(score, 10_000);

    registry.update_reputation_score(&solver_id, 7500);
    let score = registry.get(&solver_id).unwrap().reputation.reputation_score;
    assert_eq!(score, 7500);
}
