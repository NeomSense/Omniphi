use omniphi_runtime::safety::actions::SafetyAction;
use omniphi_runtime::safety::incidents::{
    AffectedDomainSet, IncidentScope, IncidentSeverity, IncidentType, SafetyIncident,
};
use omniphi_runtime::safety::receipts::{
    ContainmentActionRecord, IncidentLedger, RecoveryStatus, SafetyReceipt,
    SafetyRecoveryPlaceholder,
};
use omniphi_runtime::errors::RuntimeError;
use std::collections::BTreeMap;

fn make_receipt(epoch: u64, severity: IncidentSeverity) -> SafetyReceipt {
    let incident_type = IncidentType::AbnormalOutflow;
    let incident_id = SafetyIncident::compute_id(&incident_type, epoch, None);
    let incident = SafetyIncident {
        incident_id,
        incident_type,
        severity,
        scope: IncidentScope::FullChain,
        affected: AffectedDomainSet::new(),
        triggering_rule: "test".to_string(),
        detail: "test".to_string(),
        goal_packet_id: None,
        plan_id: None,
        solver_id: None,
        capsule_hash: None,
        epoch,
        reversible: true,
        requires_governance: false,
        metadata: BTreeMap::new(),
    };

    SafetyReceipt {
        receipt_id: incident_id,
        incident,
        containment_actions: vec![ContainmentActionRecord {
            action: SafetyAction::NoAction,
            applied_at_epoch: epoch,
            applied_to: "test".to_string(),
            success: true,
            error: None,
        }],
        recovery: SafetyRecoveryPlaceholder {
            incident_id,
            recovery_status: RecoveryStatus::Pending,
            governance_proposal_id: None,
            recovery_epoch: None,
            notes: String::new(),
        },
        epoch,
        receipt_hash: [0u8; 32],
    }
}

fn make_receipt_with_solver(epoch: u64, solver_id: [u8; 32]) -> SafetyReceipt {
    let incident_type = IncidentType::SolverMisconduct;
    let incident_id = SafetyIncident::compute_id(&incident_type, epoch, Some(solver_id));
    let mut incident = SafetyIncident {
        incident_id,
        incident_type,
        severity: IncidentSeverity::Medium,
        scope: IncidentScope::Solver(solver_id),
        affected: AffectedDomainSet::new(),
        triggering_rule: "test".to_string(),
        detail: "test".to_string(),
        goal_packet_id: None,
        plan_id: None,
        solver_id: Some(solver_id),
        capsule_hash: None,
        epoch,
        reversible: true,
        requires_governance: false,
        metadata: BTreeMap::new(),
    };

    SafetyReceipt {
        receipt_id: incident_id,
        incident,
        containment_actions: vec![],
        recovery: SafetyRecoveryPlaceholder {
            incident_id,
            recovery_status: RecoveryStatus::Pending,
            governance_proposal_id: None,
            recovery_epoch: None,
            notes: String::new(),
        },
        epoch,
        receipt_hash: [0u8; 32],
    }
}

#[test]
fn test_receipt_hash_deterministic() {
    let r1 = make_receipt(10, IncidentSeverity::High);
    let r2 = make_receipt(10, IncidentSeverity::High);

    let h1 = r1.compute_hash();
    let h2 = r2.compute_hash();
    assert_eq!(h1, h2, "same receipt produces same hash");

    let r3 = make_receipt(11, IncidentSeverity::High);
    let h3 = r3.compute_hash();
    assert_ne!(h1, h3, "different epoch → different hash");
}

#[test]
fn test_ledger_append_increments_sequence() {
    let mut ledger = IncidentLedger::new(0);

    let seq1 = ledger.append(make_receipt(1, IncidentSeverity::Low), None).unwrap();
    let seq2 = ledger.append(make_receipt(2, IncidentSeverity::Medium), None).unwrap();
    let seq3 = ledger.append(make_receipt(3, IncidentSeverity::High), None).unwrap();

    assert_eq!(seq1, 0);
    assert_eq!(seq2, 1);
    assert_eq!(seq3, 2);
    assert_eq!(ledger.len(), 3);
}

#[test]
fn test_ledger_max_entries_enforced() {
    let mut ledger = IncidentLedger::new(2);

    ledger.append(make_receipt(1, IncidentSeverity::Low), None).unwrap();
    ledger.append(make_receipt(2, IncidentSeverity::Medium), None).unwrap();

    let result = ledger.append(make_receipt(3, IncidentSeverity::High), None);
    assert!(
        matches!(result, Err(RuntimeError::IncidentLedgerFull)),
        "should return IncidentLedgerFull when at capacity"
    );
}

#[test]
fn test_get_by_severity() {
    let mut ledger = IncidentLedger::new(0);
    ledger.append(make_receipt(1, IncidentSeverity::Low), None).unwrap();
    ledger.append(make_receipt(2, IncidentSeverity::High), None).unwrap();
    ledger.append(make_receipt(3, IncidentSeverity::Critical), None).unwrap();

    let high_plus = ledger.get_by_severity(&IncidentSeverity::High);
    assert_eq!(high_plus.len(), 2, "High and Critical should be included");

    let critical_plus = ledger.get_by_severity(&IncidentSeverity::Critical);
    assert_eq!(critical_plus.len(), 1, "only Critical");

    let all = ledger.get_by_severity(&IncidentSeverity::Info);
    assert_eq!(all.len(), 3, "all entries");
}

#[test]
fn test_get_by_solver() {
    let mut ledger = IncidentLedger::new(0);
    let solver_a = [0xAAu8; 32];
    let solver_b = [0xBBu8; 32];

    ledger.append(make_receipt_with_solver(1, solver_a), None).unwrap();
    ledger.append(make_receipt_with_solver(2, solver_b), None).unwrap();
    ledger.append(make_receipt_with_solver(3, solver_a), None).unwrap();

    let a_entries = ledger.get_by_solver(&solver_a);
    assert_eq!(a_entries.len(), 2);

    let b_entries = ledger.get_by_solver(&solver_b);
    assert_eq!(b_entries.len(), 1);
}
