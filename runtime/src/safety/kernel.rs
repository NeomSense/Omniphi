use crate::crx::finality::FinalityClass;
use crate::crx::settlement::CRXSettlementRecord;
use crate::safety::actions::{PauseTarget, SafetyAction};
use crate::safety::blast_radius::{BlastRadiusAssessment, BlastRadiusEngine, DomainContainmentMap, ScopeResolutionPolicy};
use crate::safety::domain_profiles::DomainSafetyPolicy;
use crate::safety::incidents::{
    AffectedDomainSet, IncidentScope, IncidentSeverity, IncidentType, SafetyIncident,
};
use crate::safety::policies::SafetyRuleEngine;
use crate::safety::receipts::{
    ContainmentActionRecord, IncidentLedger, RecoveryStatus, SafetyReceipt,
    SafetyRecoveryPlaceholder,
};
use crate::safety::recovery_hooks::{GovernanceEscalationMarker, RecoveryHook};
use crate::safety::solver_controls::{SolverSafetyAction, SolverSafetyController, SolverSafetyStatus};
use std::collections::{BTreeMap, BTreeSet};

/// Governance Multi-Sig Proof used to unhalt the chain.
#[derive(Debug, Clone)]
pub struct GovernanceProof {
    pub is_valid: bool,
    pub proposal_id: Option<u64>,
}

/// Input to the Safety Kernel from CRX settlement.
#[derive(Debug, Clone)]
pub struct SafetyEvaluationContext {
    pub epoch: u64,
    pub batch_id: Option<[u8; 32]>,
    pub goal_packet_id: Option<[u8; 32]>,
    pub plan_id: Option<[u8; 32]>,
    pub solver_id: Option<[u8; 32]>,
    pub capsule_hash: Option<[u8; 32]>,
    pub finality_class: FinalityClass,
    pub affected_objects: Vec<[u8; 32]>,
    pub affected_domains: Vec<String>,
    pub rights_violations_count: usize,
    pub causal_violations_count: usize,
    pub branch_quarantine_count: usize,
    pub branch_downgrade_count: usize,
    pub total_outflow: u128,
    pub mutation_count: u64,
    pub domain_mutation_counts: BTreeMap<String, u64>,
    pub pool_ids: Vec<[u8; 32]>,
    pub is_governance_sensitive: bool,
    pub metadata: BTreeMap<String, String>,
}

impl SafetyEvaluationContext {
    /// Build from a CRXSettlementRecord.
    pub fn from_crx_record(
        record: &CRXSettlementRecord,
        batch_id: Option<[u8; 32]>,
        epoch: u64,
    ) -> Self {
        SafetyEvaluationContext {
            epoch,
            batch_id,
            goal_packet_id: Some(record.goal_packet_id),
            plan_id: Some(record.plan_id),
            solver_id: Some(record.solver_id),
            capsule_hash: Some(record.capsule_receipt.capsule_hash),
            finality_class: record.finality.finality_class.clone(),
            affected_objects: record.affected_objects.clone(),
            affected_domains: vec![], // would be populated from domain map in real integration
            rights_violations_count: record.causal_summary.violations_count,
            causal_violations_count: if !record.causal_summary.causal_validity { 1 } else { 0 },
            branch_quarantine_count: record
                .graph_receipt
                .branch_results
                .iter()
                .filter(|b| b.quarantine.is_some())
                .count(),
            branch_downgrade_count: record
                .graph_receipt
                .branch_results
                .iter()
                .filter(|b| b.downgrade.is_some())
                .count(),
            total_outflow: 0,
            mutation_count: record.graph_receipt.executed_nodes as u64,
            domain_mutation_counts: BTreeMap::new(),
            pool_ids: vec![],
            is_governance_sensitive: false,
            metadata: BTreeMap::new(),
        }
    }

    pub fn clean(epoch: u64) -> Self {
        SafetyEvaluationContext {
            epoch,
            batch_id: None,
            goal_packet_id: None,
            plan_id: None,
            solver_id: None,
            capsule_hash: None,
            finality_class: FinalityClass::Finalized,
            affected_objects: vec![],
            affected_domains: vec![],
            rights_violations_count: 0,
            causal_violations_count: 0,
            branch_quarantine_count: 0,
            branch_downgrade_count: 0,
            total_outflow: 0,
            mutation_count: 0,
            domain_mutation_counts: BTreeMap::new(),
            pool_ids: vec![],
            is_governance_sensitive: false,
            metadata: BTreeMap::new(),
        }
    }
}

/// The output decision from the Safety Kernel.
#[derive(Debug, Clone)]
pub struct SafetyDecision {
    pub incident: Option<SafetyIncident>,
    pub action: SafetyAction,
    pub blast_radius: Option<BlastRadiusAssessment>,
    pub governance_escalation: Option<GovernanceEscalationMarker>,
    pub constrained_state_updates: Vec<ConstrainedStateUpdate>,
}

/// A deterministic state update produced by the safety kernel.
#[derive(Debug, Clone)]
pub enum ConstrainedStateUpdate {
    QuarantineObjects(Vec<[u8; 32]>),
    PauseDomain(String),
    SuspendSolver([u8; 32]),
    ActivateRateLimit { domain: String, max_per_epoch: u64 },
    ActivateEmergencyMode,
}

#[derive(Debug, Clone)]
pub struct SafetyAuditRecord {
    pub context_epoch: u64,
    pub rules_evaluated: usize,
    pub rules_triggered: usize,
    pub decision: SafetyDecision,
    pub audit_hash: [u8; 32],
}

impl SafetyAuditRecord {
    pub fn compute_hash(&self) -> [u8; 32] {
        use sha2::{Digest, Sha256};
        let input = format!(
            "{}:{}:{}",
            self.context_epoch, self.rules_evaluated, self.rules_triggered
        );
        let hash = Sha256::digest(input.as_bytes());
        let mut h = [0u8; 32];
        h.copy_from_slice(&hash);
        h
    }
}

pub struct SafetyKernel {
    pub rule_engine: SafetyRuleEngine,
    pub domain_policy: DomainSafetyPolicy,
    pub solver_controller: SolverSafetyController,
    pub blast_radius_policy: ScopeResolutionPolicy,
    pub domain_map: DomainContainmentMap,
    pub ledger: IncidentLedger,
    pub recovery_hooks: Vec<Box<dyn RecoveryHook>>,
    pub quarantined_objects: BTreeSet<[u8; 32]>,
    pub paused_domains: BTreeSet<String>,
    pub emergency_mode: bool,
}

impl SafetyKernel {
    pub fn new() -> Self {
        SafetyKernel {
            rule_engine: SafetyRuleEngine::with_defaults(),
            domain_policy: DomainSafetyPolicy::with_defaults(),
            solver_controller: SolverSafetyController::new(),
            blast_radius_policy: ScopeResolutionPolicy::default(),
            domain_map: DomainContainmentMap {
                domain_to_objects: BTreeMap::new(),
                domain_to_solvers: BTreeMap::new(),
                domain_dependencies: BTreeMap::new(),
            },
            ledger: IncidentLedger::new(0),
            recovery_hooks: vec![],
            quarantined_objects: BTreeSet::new(),
            paused_domains: BTreeSet::new(),
            emergency_mode: false,
        }
    }

    pub fn with_recovery_hook(mut self, hook: Box<dyn RecoveryHook>) -> Self {
        self.recovery_hooks.push(hook);
        self
    }

    /// Main entry point: evaluate a safety context and produce a SafetyDecision.
    /// This is the 9-step safety lifecycle.
    pub fn evaluate(&mut self, ctx: &SafetyEvaluationContext) -> SafetyDecision {
        // Step 1: Evaluate rules
        let evaluations = self.rule_engine.evaluate(ctx);

        // Step 2: Find highest severity triggered rule
        let highest = evaluations.first(); // sorted by severity descending

        if highest.is_none() {
            return SafetyDecision {
                incident: None,
                action: SafetyAction::NoAction,
                blast_radius: None,
                governance_escalation: None,
                constrained_state_updates: vec![],
            };
        }

        let top_eval = highest.unwrap().clone();

        // Step 3: Classify incident
        let incident_type = match top_eval.policy_name.as_str() {
            "AbnormalOutflow" => IncidentType::AbnormalOutflow,
            "SolverMisconduct" => IncidentType::SolverMisconduct,
            "RepeatedBranchQuarantine" => IncidentType::RepeatedBranchQuarantine,
            "GovernanceSensitiveObjectMisuse" => IncidentType::GovernanceSensitiveObjectMisuse,
            "LiquidityPoolInstability" => IncidentType::LiquidityPoolInstability,
            "AbnormalMutationVelocity" => IncidentType::AbnormalMutationVelocity,
            "CrossDomainBlastRadius" => IncidentType::CrossDomainBlastRadiusEscalation,
            _ => IncidentType::PolicyRuleViolation,
        };

        let incident_id =
            SafetyIncident::compute_id(&incident_type, ctx.epoch, ctx.solver_id);

        let scope = if let Some(solver_id) = ctx.solver_id {
            if matches!(incident_type, IncidentType::SolverMisconduct) {
                IncidentScope::Solver(solver_id)
            } else if ctx.affected_domains.len() > 1 {
                IncidentScope::MultiDomain(ctx.affected_domains.clone())
            } else if let Some(d) = ctx.affected_domains.first() {
                IncidentScope::Domain(d.clone())
            } else if let Some(obj) = ctx.affected_objects.first() {
                IncidentScope::SingleObject(*obj)
            } else {
                IncidentScope::FullChain
            }
        } else if ctx.affected_domains.len() > 1 {
            IncidentScope::MultiDomain(ctx.affected_domains.clone())
        } else if let Some(d) = ctx.affected_domains.first() {
            IncidentScope::Domain(d.clone())
        } else if let Some(obj) = ctx.affected_objects.first() {
            IncidentScope::SingleObject(*obj)
        } else {
            IncidentScope::FullChain
        };

        let mut affected = AffectedDomainSet::new();
        for d in &ctx.affected_domains {
            affected.domains.insert(d.clone());
        }
        for obj in &ctx.affected_objects {
            affected.object_ids.insert(*obj);
        }
        if let Some(sid) = ctx.solver_id {
            affected.solver_ids.insert(sid);
        }

        let requires_governance = top_eval.action.requires_governance()
            || matches!(
                top_eval.computed_severity,
                IncidentSeverity::Critical | IncidentSeverity::Emergency
            );

        let incident = SafetyIncident {
            incident_id,
            incident_type: incident_type.clone(),
            severity: top_eval.computed_severity.clone(),
            scope,
            affected,
            triggering_rule: top_eval.policy_name.clone(),
            detail: top_eval.detail.clone(),
            goal_packet_id: ctx.goal_packet_id,
            plan_id: ctx.plan_id,
            solver_id: ctx.solver_id,
            capsule_hash: ctx.capsule_hash,
            epoch: ctx.epoch,
            reversible: !requires_governance,
            requires_governance,
            metadata: ctx.metadata.clone(),
        };

        // Step 4: Assess blast radius
        let blast_radius =
            BlastRadiusEngine::assess(&incident, &self.domain_map, &self.blast_radius_policy);

        // Step 5: Determine action
        let action = top_eval.action.clone();

        // Step 6: Apply constrained state updates
        let mut constrained_state_updates: Vec<ConstrainedStateUpdate> = vec![];
        self.apply_action(&action, ctx);
        match &action {
            SafetyAction::Quarantine(q) => {
                if !q.object_ids.is_empty() {
                    constrained_state_updates
                        .push(ConstrainedStateUpdate::QuarantineObjects(q.object_ids.clone()));
                } else if !ctx.affected_objects.is_empty() {
                    constrained_state_updates.push(ConstrainedStateUpdate::QuarantineObjects(
                        ctx.affected_objects.clone(),
                    ));
                }
            }
            SafetyAction::Pause(p) => {
                if let PauseTarget::Domain(d) = &p.target {
                    constrained_state_updates
                        .push(ConstrainedStateUpdate::PauseDomain(d.clone()));
                }
            }
            SafetyAction::SuspendSolver(id, _) => {
                constrained_state_updates.push(ConstrainedStateUpdate::SuspendSolver(*id));
            }
            SafetyAction::RateLimit(rl) => {
                if let Some(domain) = &rl.target_domain {
                    constrained_state_updates.push(ConstrainedStateUpdate::ActivateRateLimit {
                        domain: domain.clone(),
                        max_per_epoch: rl.max_mutations_per_epoch,
                    });
                }
            }
            SafetyAction::EmergencyMode(_) => {
                constrained_state_updates.push(ConstrainedStateUpdate::ActivateEmergencyMode);
            }
            SafetyAction::MultiAction(actions) => {
                for a in actions {
                    match a {
                        SafetyAction::Quarantine(q) => {
                            constrained_state_updates.push(
                                ConstrainedStateUpdate::QuarantineObjects(q.object_ids.clone()),
                            );
                        }
                        SafetyAction::Pause(p) => {
                            if let PauseTarget::Domain(d) = &p.target {
                                constrained_state_updates
                                    .push(ConstrainedStateUpdate::PauseDomain(d.clone()));
                            }
                        }
                        SafetyAction::SuspendSolver(id, _) => {
                            constrained_state_updates
                                .push(ConstrainedStateUpdate::SuspendSolver(*id));
                        }
                        SafetyAction::EmergencyMode(_) => {
                            constrained_state_updates
                                .push(ConstrainedStateUpdate::ActivateEmergencyMode);
                        }
                        _ => {}
                    }
                }
            }
            _ => {}
        }

        // Step 7: Run recovery hooks if Critical+
        let mut governance_escalation: Option<GovernanceEscalationMarker> = None;
        if incident.severity >= IncidentSeverity::Critical {
            for hook in &self.recovery_hooks {
                if let Some(marker) = hook.on_incident(&incident) {
                    governance_escalation = Some(marker);
                    break;
                }
            }
        }

        // Step 8: Append to ledger
        let receipt = SafetyReceipt {
            receipt_id: incident_id,
            incident: incident.clone(),
            containment_actions: vec![ContainmentActionRecord {
                action: action.clone(),
                applied_at_epoch: ctx.epoch,
                applied_to: format!("{:?}", incident.scope),
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
            epoch: ctx.epoch,
            receipt_hash: [0u8; 32],
        };
        let _ = self.ledger.append(receipt, ctx.batch_id);

        // Step 9: Return SafetyDecision
        SafetyDecision {
            incident: Some(incident),
            action,
            blast_radius: Some(blast_radius),
            governance_escalation,
            constrained_state_updates,
        }
    }

    /// Check whether an object is quarantined.
    pub fn is_object_quarantined(&self, object_id: &[u8; 32]) -> bool {
        self.quarantined_objects.contains(object_id)
    }

    /// Check whether a domain is paused.
    pub fn is_domain_paused(&self, domain: &str) -> bool {
        self.paused_domains.contains(domain)
    }

    /// Check whether a solver is allowed to submit.
    pub fn is_solver_allowed(&self, solver_id: &[u8; 32]) -> bool {
        self.solver_controller.is_solver_allowed(solver_id)
    }

    /// Resets the emergency mode, unhalting the chain.
    /// Strictly gated by a valid governance multi-sig proof.
    pub fn reset_emergency_mode(&mut self, proof: &GovernanceProof, epoch: u64) -> Result<(), &'static str> {
        if !proof.is_valid {
            return Err("Emergency mode reset denied: invalid governance proof");
        }

        self.emergency_mode = false;

        // Log the recovery into the incident ledger
        let incident_id = SafetyIncident::compute_id(&IncidentType::PolicyRuleViolation, epoch, None);
        let reset_receipt = SafetyReceipt {
            receipt_id: incident_id,
            incident: SafetyIncident {
                incident_id,
                incident_type: IncidentType::PolicyRuleViolation,
                severity: IncidentSeverity::Emergency,
                scope: IncidentScope::FullChain,
                affected: AffectedDomainSet::new(),
                triggering_rule: "GovernanceReset".to_string(),
                detail: "Emergency mode was manually reset by governance multi-sig".to_string(),
                goal_packet_id: None,
                plan_id: None,
                solver_id: None,
                capsule_hash: None,
                epoch,
                reversible: false,
                requires_governance: true,
                metadata: BTreeMap::new(),
            },
            containment_actions: vec![],
            recovery: SafetyRecoveryPlaceholder {
                incident_id,
                recovery_status: RecoveryStatus::Pending,
                governance_proposal_id: None, 
                recovery_epoch: Some(epoch),
                notes: "Emergency mode explicitly reset by governance".to_string(),
            },
            epoch,
            receipt_hash: [0u8; 32],
        };
        let _ = self.ledger.append(reset_receipt, None);

        Ok(())
    }

    pub(crate) fn apply_action(&mut self, action: &SafetyAction, ctx: &SafetyEvaluationContext) {
        match action {
            SafetyAction::Quarantine(q) => {
                for obj_id in &q.object_ids {
                    self.quarantined_objects.insert(*obj_id);
                }
                // Also quarantine affected objects from context
                for obj_id in &ctx.affected_objects {
                    self.quarantined_objects.insert(*obj_id);
                }
            }
            SafetyAction::Pause(p) => {
                if let PauseTarget::Domain(d) = &p.target {
                    self.paused_domains.insert(d.clone());
                }
            }
            SafetyAction::SuspendSolver(id, _reason) => {
                let effective_id = if *id == [0u8; 32] {
                    ctx.solver_id.unwrap_or(*id)
                } else {
                    *id
                };
                self.solver_controller
                    .apply_action(SolverSafetyAction::Suspend {
                        solver_id: effective_id,
                        until_epoch: u64::MAX,
                    });
            }
            SafetyAction::EmergencyMode(_) => {
                self.emergency_mode = true;
            }
            SafetyAction::MultiAction(actions) => {
                for a in actions {
                    self.apply_action(a, ctx);
                }
            }
            SafetyAction::RateLimit(_rl) => {
                // Rate limiting is tracked externally; nothing to do in kernel state
            }
            _ => {}
        }
    }
}

impl Clone for SafetyKernel {
    fn clone(&self) -> Self {
        SafetyKernel {
            rule_engine: SafetyRuleEngine {
                policies: self.rule_engine.policies.clone(),
                violation_counts: self.rule_engine.violation_counts.clone(),
                quarantine_counts_per_window: self.rule_engine.quarantine_counts_per_window.clone(),
            },
            domain_policy: self.domain_policy.clone(),
            solver_controller: SolverSafetyController {
                restrictions: self.solver_controller.restrictions.clone(),
                incident_history: self.solver_controller.incident_history.clone(),
            },
            blast_radius_policy: self.blast_radius_policy.clone(),
            domain_map: self.domain_map.clone(),
            ledger: IncidentLedger {
                entries: self.ledger.entries.clone(),
                max_entries: self.ledger.max_entries,
                next_sequence: self.ledger.next_sequence,
            },
            // Recovery hooks cannot be cloned (trait objects); clone produces empty hooks
            recovery_hooks: vec![],
            quarantined_objects: self.quarantined_objects.clone(),
            paused_domains: self.paused_domains.clone(),
            emergency_mode: self.emergency_mode,
        }
    }
}
