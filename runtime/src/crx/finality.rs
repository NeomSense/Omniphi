use crate::crx::branch_execution::ExecutionSettlementClass;
use crate::crx::goal_packet::{GoalPacket, PartialFailurePolicy, SettlementStrictness};

// ─────────────────────────────────────────────────────────────────────────────
// Finality types
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum FinalityClass {
    Finalized,
    FinalizedWithDowngrade,
    FinalizedWithQuarantine,
    Provisional,
    Rejected,
    Reverted,
}

#[derive(Debug, Clone)]
pub struct DomainFinalityPolicy {
    pub domain: String,
    /// Anything below this finality class triggers escalation.
    pub min_finality_class: FinalityClass,
    pub require_audit: bool,
}

#[derive(Debug, Clone)]
pub struct SettlementDisposition {
    pub finality_class: FinalityClass,
    pub reason: String,
    pub escalation_required: bool,
    pub provisional_until_epoch: Option<u64>,
}

// ─────────────────────────────────────────────────────────────────────────────
// Finality ordering (higher is better)
// ─────────────────────────────────────────────────────────────────────────────

fn finality_rank(f: &FinalityClass) -> u8 {
    match f {
        FinalityClass::Finalized => 5,
        FinalityClass::FinalizedWithDowngrade => 4,
        FinalityClass::FinalizedWithQuarantine => 3,
        FinalityClass::Provisional => 2,
        FinalityClass::Reverted => 1,
        FinalityClass::Rejected => 0,
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Classifier
// ─────────────────────────────────────────────────────────────────────────────

pub struct FinalityClassifier;

impl FinalityClassifier {
    /// Classify finality based on ExecutionSettlementClass + PartialFailurePolicy + domain policies.
    pub fn classify(
        settlement_class: &ExecutionSettlementClass,
        goal: &GoalPacket,
        domain_policies: &[DomainFinalityPolicy],
    ) -> SettlementDisposition {
        // Base finality class from settlement + policy
        let (base_class, base_reason) = Self::base_classify(settlement_class, goal);

        // Override to Provisional if SettlementStrictness::Provisional
        let (finality_class, reason) = if goal.policy.settlement_strictness == SettlementStrictness::Provisional
            && base_class != FinalityClass::Rejected
            && base_class != FinalityClass::Reverted
        {
            (FinalityClass::Provisional, format!("{}; overridden to Provisional by strictness policy", base_reason))
        } else {
            (base_class, base_reason)
        };

        // Check domain policies: if any domain requires higher finality than achieved → escalation
        let achieved_rank = finality_rank(&finality_class);
        let mut escalation_required = false;

        for dp in domain_policies {
            let min_rank = finality_rank(&dp.min_finality_class);
            if achieved_rank < min_rank {
                escalation_required = true;
                break;
            }
        }

        let provisional_until = if finality_class == FinalityClass::Provisional {
            Some(goal.deadline_epoch)
        } else {
            None
        };

        SettlementDisposition {
            finality_class,
            reason,
            escalation_required,
            provisional_until_epoch: provisional_until,
        }
    }

    fn base_classify(
        settlement_class: &ExecutionSettlementClass,
        goal: &GoalPacket,
    ) -> (FinalityClass, String) {
        match settlement_class {
            ExecutionSettlementClass::FullSuccess => {
                (FinalityClass::Finalized, "full success".to_string())
            }

            ExecutionSettlementClass::SuccessWithDowngrade(branch_id) => {
                if goal.policy.partial_failure_policy == PartialFailurePolicy::AllowBranchDowngrade
                    || goal.policy.partial_failure_policy == PartialFailurePolicy::DowngradeAndContinue
                {
                    (
                        FinalityClass::FinalizedWithDowngrade,
                        format!("branch {} downgraded", branch_id),
                    )
                } else {
                    (
                        FinalityClass::Reverted,
                        format!("branch {} downgraded but policy disallows it", branch_id),
                    )
                }
            }

            ExecutionSettlementClass::SuccessWithQuarantine(objects) => {
                if goal.policy.partial_failure_policy == PartialFailurePolicy::QuarantineOnFailure {
                    (
                        FinalityClass::FinalizedWithQuarantine,
                        format!("{} object(s) quarantined", objects.len()),
                    )
                } else {
                    (
                        FinalityClass::Reverted,
                        format!("{} object(s) quarantined but policy disallows quarantine", objects.len()),
                    )
                }
            }

            ExecutionSettlementClass::PartialSuccess { succeeded_branches, failed_branches } => {
                match &goal.policy.partial_failure_policy {
                    PartialFailurePolicy::AllowBranchDowngrade
                    | PartialFailurePolicy::DowngradeAndContinue => (
                        FinalityClass::FinalizedWithDowngrade,
                        format!(
                            "partial success: {} succeeded, {} failed",
                            succeeded_branches.len(),
                            failed_branches.len()
                        ),
                    ),
                    PartialFailurePolicy::QuarantineOnFailure => (
                        FinalityClass::FinalizedWithQuarantine,
                        format!(
                            "partial success with quarantine: {} succeeded, {} failed",
                            succeeded_branches.len(),
                            failed_branches.len()
                        ),
                    ),
                    PartialFailurePolicy::StrictAllOrNothing => (
                        FinalityClass::Reverted,
                        "strict policy: partial success triggers revert".to_string(),
                    ),
                }
            }

            ExecutionSettlementClass::FullRevert => {
                (FinalityClass::Reverted, "full revert".to_string())
            }

            ExecutionSettlementClass::Rejected => {
                (FinalityClass::Rejected, "rejected before execution".to_string())
            }
        }
    }
}
