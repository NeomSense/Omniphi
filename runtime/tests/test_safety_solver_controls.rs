use omniphi_runtime::safety::incidents::IncidentType;
use omniphi_runtime::safety::solver_controls::{
    SolverIncidentRecord, SolverSafetyAction, SolverSafetyController, SolverSafetyStatus,
};

fn make_controller() -> SolverSafetyController {
    SolverSafetyController::new()
}

#[test]
fn test_solver_starts_normal() {
    let controller = make_controller();
    let solver_id = [0x01u8; 32];
    assert_eq!(controller.get_status(&solver_id), SolverSafetyStatus::Normal);
    assert!(controller.is_solver_allowed(&solver_id));
}

#[test]
fn test_suspend_solver() {
    let mut controller = make_controller();
    let solver_id = [0x01u8; 32];

    controller.apply_action(SolverSafetyAction::Suspend {
        solver_id,
        until_epoch: 1000,
    });

    assert_eq!(controller.get_status(&solver_id), SolverSafetyStatus::Suspended);
    assert!(!controller.is_solver_allowed(&solver_id));
}

#[test]
fn test_banned_solver_not_allowed() {
    let mut controller = make_controller();
    let solver_id = [0x02u8; 32];

    controller.apply_action(SolverSafetyAction::Ban {
        solver_id,
        reason: "permanent ban".to_string(),
    });

    assert_eq!(controller.get_status(&solver_id), SolverSafetyStatus::Banned);
    assert!(!controller.is_solver_allowed(&solver_id));

    let profile = controller.get_restriction(&solver_id).unwrap();
    assert_eq!(profile.ban_reason.as_deref(), Some("permanent ban"));
}

#[test]
fn test_incident_count_in_window() {
    let mut controller = make_controller();
    let solver_id = [0x03u8; 32];

    // Record incidents at epochs 100, 102, 110
    for epoch in [100u64, 102, 110] {
        controller.record_incident(SolverIncidentRecord {
            solver_id,
            incident_type: IncidentType::SolverMisconduct,
            detail: format!("violation at epoch {}", epoch),
            epoch,
            escalated: false,
        });
    }

    // Window: last 15 epochs from current epoch 115
    let count = controller.incident_count(&solver_id, 15, 115);
    assert_eq!(count, 3, "all 3 incidents within window");

    // Window: last 5 epochs from current epoch 115 → only epoch 110 qualifies
    let count2 = controller.incident_count(&solver_id, 5, 115);
    assert_eq!(count2, 1, "only epoch 110 within last 5 epochs");
}

#[test]
fn test_rate_restrict_action() {
    let mut controller = make_controller();
    let solver_id = [0x04u8; 32];

    controller.apply_action(SolverSafetyAction::RateRestrict {
        solver_id,
        max_per_epoch: 10,
    });

    assert_eq!(controller.get_status(&solver_id), SolverSafetyStatus::RateRestricted);
    assert!(controller.is_solver_allowed(&solver_id), "rate-restricted solvers are still allowed");

    let profile = controller.get_restriction(&solver_id).unwrap();
    assert_eq!(profile.max_submissions_per_epoch, 10);
}

#[test]
fn test_record_incident_history() {
    let mut controller = make_controller();
    let solver_id = [0x05u8; 32];

    controller.record_incident(SolverIncidentRecord {
        solver_id,
        incident_type: IncidentType::AbnormalOutflow,
        detail: "outflow at epoch 50".to_string(),
        epoch: 50,
        escalated: false,
    });

    let history = controller.incident_history.get(&solver_id).unwrap();
    assert_eq!(history.len(), 1);
    assert_eq!(history[0].epoch, 50);
    assert!(matches!(history[0].incident_type, IncidentType::AbnormalOutflow));
}
