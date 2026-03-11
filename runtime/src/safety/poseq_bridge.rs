use crate::crx::poseq_bridge::CRXExecutionResult;
use crate::safety::kernel::{
    ConstrainedStateUpdate, SafetyDecision, SafetyEvaluationContext, SafetyKernel,
};

/// Context passed to the safety kernel for an entire ordered batch.
#[derive(Debug, Clone)]
pub struct OrderedSafetyContext {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub per_goal_contexts: Vec<SafetyEvaluationContext>,
}

/// Annotated settlement result with safety decision attached.
#[derive(Debug)]
pub struct SafetyAnnotatedSettlement {
    pub crx_result: CRXExecutionResult,
    pub safety_decision: SafetyDecision,
    pub final_allowed: bool, // false if safety kernel blocked this settlement
}

/// Runtime state as modified by safety constraints.
#[derive(Debug, Clone)]
pub struct SafetyConstrainedExecutionState {
    pub quarantined_objects: Vec<[u8; 32]>,
    pub paused_domains: Vec<String>,
    pub suspended_solvers: Vec<[u8; 32]>,
    pub emergency_mode: bool,
    pub as_of_epoch: u64,
}

pub struct PoSeqSafetyBridge {
    pub kernel: SafetyKernel,
}

impl PoSeqSafetyBridge {
    pub fn new() -> Self {
        PoSeqSafetyBridge {
            kernel: SafetyKernel::new(),
        }
    }

    /// Evaluate all CRX results in a batch through the safety kernel.
    /// Returns annotated settlements and updated constrained state.
    pub fn process_batch(
        &mut self,
        results: Vec<CRXExecutionResult>,
        batch_id: [u8; 32],
        epoch: u64,
    ) -> (
        Vec<SafetyAnnotatedSettlement>,
        SafetyConstrainedExecutionState,
    ) {
        let mut annotated: Vec<SafetyAnnotatedSettlement> = Vec::new();

        for crx_result in results {
            let ctx = SafetyEvaluationContext::from_crx_record(
                &crx_result.record,
                Some(batch_id),
                epoch,
            );

            let safety_decision = self.kernel.evaluate(&ctx);

            // A settlement is blocked if:
            // - the solver is not allowed
            // - the domain is paused
            // - emergency mode is active
            let solver_id = crx_result.record.solver_id;
            let final_allowed = !self.kernel.emergency_mode
                && self.kernel.is_solver_allowed(&solver_id)
                && {
                    // Check if any affected object is quarantined
                    let any_quarantined = crx_result
                        .record
                        .affected_objects
                        .iter()
                        .any(|obj| self.kernel.is_object_quarantined(obj));
                    !any_quarantined
                };

            annotated.push(SafetyAnnotatedSettlement {
                crx_result,
                safety_decision,
                final_allowed,
            });
        }

        let state = self.constrained_state(epoch);
        (annotated, state)
    }

    pub fn constrained_state(&self, epoch: u64) -> SafetyConstrainedExecutionState {
        let quarantined_objects: Vec<[u8; 32]> =
            self.kernel.quarantined_objects.iter().copied().collect();
        let paused_domains: Vec<String> = self.kernel.paused_domains.iter().cloned().collect();

        // Collect suspended solvers from controller
        let suspended_solvers: Vec<[u8; 32]> = self
            .kernel
            .solver_controller
            .restrictions
            .iter()
            .filter(|(_, p)| {
                p.status >= crate::safety::solver_controls::SolverSafetyStatus::Suspended
            })
            .map(|(id, _)| *id)
            .collect();

        SafetyConstrainedExecutionState {
            quarantined_objects,
            paused_domains,
            suspended_solvers,
            emergency_mode: self.kernel.emergency_mode,
            as_of_epoch: epoch,
        }
    }
}

impl Clone for SafetyAnnotatedSettlement {
    fn clone(&self) -> Self {
        SafetyAnnotatedSettlement {
            crx_result: CRXExecutionResult {
                goal_packet_id: self.crx_result.goal_packet_id,
                record: self.crx_result.record.clone(),
                success: self.crx_result.success,
            },
            safety_decision: self.safety_decision.clone(),
            final_allowed: self.final_allowed,
        }
    }
}
