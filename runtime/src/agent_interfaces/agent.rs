use crate::errors::RuntimeError;
use crate::objects::base::ObjectType;
use crate::solver_market::market::CandidatePlan;

/// The functional role of an agent solver.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AgentType {
    TreasuryAgent,
    RoutingAgent,
    GovernanceAgent,
    SafetyAgent,
    Custom(String),
}

/// Bounded permission envelope for an agent solver.
///
/// Agents can only act within the scope declared here.
#[derive(Debug, Clone)]
pub struct AgentPolicyEnvelope {
    /// Which object types the agent is permitted to include in plans.
    pub allowed_object_types: Vec<ObjectType>,
    /// Which intent classes the agent is permitted to resolve.
    pub allowed_intent_classes: Vec<String>,
    /// Hard cap on `expected_output_amount` per intent.
    pub max_value_per_intent: u128,
    /// Hard cap on total (reads + writes) objects per plan.
    pub max_objects_per_plan: usize,
    /// Future: require a human countersignature before execution.
    pub require_human_countersign: bool,
    /// Epoch after which this envelope is no longer valid.
    pub valid_until_epoch: u64,
}

/// Human-readable or machine-parseable explanation attached to an agent plan.
#[derive(Debug, Clone)]
pub struct PlanExplanationMetadata {
    pub reasoning_summary: String,
    /// 0–10000 bps, agent self-reported confidence.
    pub confidence_bps: u64,
    pub inputs_considered: Vec<String>,
    pub alternatives_rejected: Vec<String>,
    pub safety_checks_passed: Vec<String>,
}

/// Output format for `PlanExplanationMetadata`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExplanationFormat {
    PlainText,
    StructuredJson,
    None,
}

/// Extended metadata profile for an AI/agent solver.
///
/// Agents are registered as normal solvers (`SolverProfile.is_agent = true`)
/// plus this supplemental metadata stored separately.
#[derive(Debug, Clone)]
pub struct AgentSolverProfile {
    pub solver_id: [u8; 32],
    pub agent_type: AgentType,
    pub policy_envelope: AgentPolicyEnvelope,
    pub explanation_format: ExplanationFormat,
    pub max_plan_complexity: usize,
}

/// Wraps a `CandidatePlan` submission from an agent solver with agent-specific
/// metadata for the runtime to verify.
#[derive(Debug, Clone)]
pub struct DeterministicAgentSubmission {
    pub plan: CandidatePlan,
    pub agent_profile: AgentSolverProfile,
    /// Agent asserts it checked its own policy; runtime re-validates.
    pub policy_check_passed: bool,
    pub explanation_metadata: Option<PlanExplanationMetadata>,
    pub submission_epoch: u64,
}

impl DeterministicAgentSubmission {
    /// Validate the agent submission against its policy envelope.
    ///
    /// Returns `Err` if:
    /// - The plan's `expected_output_amount` exceeds the envelope's `max_value_per_intent`
    /// - `current_epoch` > `valid_until_epoch`
    /// - Total objects (reads + writes) exceed `max_objects_per_plan`
    pub fn validate_against_policy(&self, current_epoch: u64) -> Result<(), RuntimeError> {
        let envelope = &self.agent_profile.policy_envelope;

        // Check epoch validity
        if current_epoch > envelope.valid_until_epoch {
            return Err(RuntimeError::PolicyViolation(format!(
                "agent policy envelope expired at epoch {}, current epoch {}",
                envelope.valid_until_epoch, current_epoch
            )));
        }

        // Check max value per intent
        if self.plan.expected_output_amount > envelope.max_value_per_intent {
            return Err(RuntimeError::PolicyViolation(format!(
                "plan output {} exceeds agent max_value_per_intent {}",
                self.plan.expected_output_amount, envelope.max_value_per_intent
            )));
        }

        // Check max objects per plan
        let total_objects = self.plan.object_reads.len() + self.plan.object_writes.len();
        if total_objects > envelope.max_objects_per_plan {
            return Err(RuntimeError::PolicyViolation(format!(
                "plan uses {} objects, agent max_objects_per_plan is {}",
                total_objects, envelope.max_objects_per_plan
            )));
        }

        Ok(())
    }
}
