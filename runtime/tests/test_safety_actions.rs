use omniphi_runtime::safety::actions::{
    EmergencyModeAction, PauseAction, PauseTarget, QuarantineAction, RateLimitAction,
    SafetyAction,
};

#[test]
fn test_quarantine_action_is_scoped() {
    let action = SafetyAction::Quarantine(QuarantineAction {
        object_ids: vec![[0x01u8; 32]],
        reason: "test".to_string(),
        reversible: true,
        governance_required_to_lift: false,
    });
    assert!(action.is_scoped(), "Quarantine should be scoped");
}

#[test]
fn test_emergency_mode_not_scoped() {
    let action = SafetyAction::EmergencyMode(EmergencyModeAction {
        activated: true,
        reason: "emergency".to_string(),
        requires_governance_to_deactivate: true,
    });
    assert!(!action.is_scoped(), "EmergencyMode should NOT be scoped");
}

#[test]
fn test_multiaction_governance_check() {
    let action = SafetyAction::MultiAction(vec![
        SafetyAction::NoAction,
        SafetyAction::Quarantine(QuarantineAction {
            object_ids: vec![],
            reason: "test".to_string(),
            reversible: false,
            governance_required_to_lift: true, // this requires governance
        }),
    ]);
    assert!(action.requires_governance(), "MultiAction with governance-required quarantine should require governance");

    let action2 = SafetyAction::MultiAction(vec![
        SafetyAction::NoAction,
        SafetyAction::LogOnly("hello".to_string()),
    ]);
    assert!(!action2.requires_governance(), "MultiAction with no-governance actions should not require governance");
}

#[test]
fn test_pause_action_temporary_epoch() {
    let action = SafetyAction::Pause(PauseAction {
        target: PauseTarget::Domain("dex_liquidity".to_string()),
        reason: "reserve deviation".to_string(),
        temporary_until_epoch: Some(200),
        governance_required_to_lift: false,
    });
    if let SafetyAction::Pause(p) = &action {
        assert_eq!(p.temporary_until_epoch, Some(200));
        assert!(!p.governance_required_to_lift);
    } else {
        panic!("expected Pause action");
    }
}

#[test]
fn test_rate_limit_action_values() {
    let action = SafetyAction::RateLimit(RateLimitAction {
        target_domain: Some("dex".to_string()),
        target_solver: None,
        max_mutations_per_epoch: 50,
        max_value_per_epoch: 1_000_000,
        reason: "velocity limit".to_string(),
    });
    if let SafetyAction::RateLimit(r) = &action {
        assert_eq!(r.max_mutations_per_epoch, 50);
        assert_eq!(r.max_value_per_epoch, 1_000_000);
        assert_eq!(r.target_domain.as_deref(), Some("dex"));
        assert!(r.target_solver.is_none());
    } else {
        panic!("expected RateLimit action");
    }
    assert!(!action.requires_governance());
}
