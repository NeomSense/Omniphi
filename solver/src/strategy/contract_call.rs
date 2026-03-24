//! Contract call solver — handles `contract_call` intents.
//!
//! Builds execution plans for Intent Contract interactions. The solver reads
//! the contract's current state, computes the proposed state transition based
//! on the intent method and parameters, and builds a plan with a
//! `ContractStateTransition` step.

use sha2::{Sha256, Digest};
use rand::RngCore;

use omniphi_poseq::auction::types::{
    ExecutionStep, OperationType, OperationParams,
};
use super::{IntentDescriptor, SolverPlan, SolverStrategy};

/// A solver that handles contract_call intents by computing state transitions.
pub struct ContractCallSolver {
    /// Base fee this solver charges per contract call.
    pub base_fee: u64,
}

impl ContractCallSolver {
    pub fn new() -> Self {
        ContractCallSolver { base_fee: 50 }
    }

    pub fn with_fee(base_fee: u64) -> Self {
        ContractCallSolver { base_fee }
    }

    fn build_contract_plan(
        &self,
        intent: &IntentDescriptor,
        batch_window: u64,
        solver_id: [u8; 32],
    ) -> Option<SolverPlan> {
        // Parse contract call parameters
        let schema_id_hex = intent.params.get("schema_id")?.as_str()?;
        let contract_id_hex = intent.params.get("contract_id")?.as_str()?;
        let method = intent.params.get("method")?.as_str()?;
        let proposed_state_hex = intent.params.get("proposed_state")?.as_str()?;

        let schema_bytes = hex::decode(schema_id_hex).ok()?;
        let contract_bytes = hex::decode(contract_id_hex).ok()?;
        let proposed_state = hex::decode(proposed_state_hex).ok()?;

        if schema_bytes.len() != 32 || contract_bytes.len() != 32 {
            return None;
        }

        // Check fee budget
        if self.base_fee > intent.max_fee {
            return None;
        }

        let mut contract_id = [0u8; 32];
        contract_id.copy_from_slice(&contract_bytes);

        // Generate bundle_id deterministically
        let mut bundle_hasher = Sha256::new();
        bundle_hasher.update(&solver_id);
        bundle_hasher.update(&intent.intent_id);
        bundle_hasher.update(&batch_window.to_be_bytes());
        bundle_hasher.update(b"contract_call");
        let bundle_id: [u8; 32] = bundle_hasher.finalize().into();

        // Random nonce
        let mut nonce = [0u8; 32];
        rand::thread_rng().fill_bytes(&mut nonce);

        // Build the execution step: a ContractStateTransition
        // The custom_data carries the schema_id, method, and proposed_state
        // which the runtime settlement engine will parse.
        let mut custom = std::collections::BTreeMap::new();
        custom.insert("schema_id".to_string(), schema_id_hex.to_string());
        custom.insert("method".to_string(), method.to_string());
        custom.insert("proposed_state".to_string(), proposed_state_hex.to_string());

        let custom_json = serde_json::to_vec(&custom).ok()?;

        let steps = vec![
            ExecutionStep {
                step_index: 0,
                operation: OperationType::ContractCall,
                object_id: contract_id,
                read_set: vec![contract_id],
                write_set: vec![contract_id],
                params: OperationParams {
                    asset: None,
                    amount: None,
                    recipient: None,
                    pool_id: None,
                    custom_data: Some(custom_json),
                },
            },
        ];

        Some(SolverPlan {
            bundle_id,
            target_intent_ids: vec![intent.intent_id],
            execution_steps: steps,
            predicted_output_amount: 0, // state transition, not a value transfer
            fee_quote: self.base_fee,
            nonce,
        })
    }
}

impl SolverStrategy for ContractCallSolver {
    fn name(&self) -> &str {
        "ContractCallSolver"
    }

    fn supported_classes(&self) -> Vec<String> {
        vec!["contract_call".into()]
    }

    fn build_plan(
        &self,
        intents: &[IntentDescriptor],
        batch_window: u64,
        solver_id: [u8; 32],
    ) -> Option<SolverPlan> {
        let supported = self.filter_intents(intents);
        let intent = supported.first()?;
        self.build_contract_plan(intent, batch_window, solver_id)
    }

    fn on_win(&self, bundle_id: [u8; 32], _intents: &[IntentDescriptor]) {
        println!(
            "[ContractCallSolver] Won auction for bundle {}",
            hex::encode(&bundle_id[..4])
        );
    }

    fn on_loss(&self, bundle_id: [u8; 32]) {
        println!(
            "[ContractCallSolver] Lost auction for bundle {}",
            hex::encode(&bundle_id[..4])
        );
    }

    fn on_settlement(&self, bundle_id: [u8; 32], success: bool) {
        println!(
            "[ContractCallSolver] Settlement {}: bundle {}",
            if success { "SUCCEEDED" } else { "FAILED" },
            hex::encode(&bundle_id[..4])
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_contract_intent(
        schema_id: [u8; 32],
        contract_id: [u8; 32],
        method: &str,
        proposed_state: &[u8],
        max_fee: u64,
    ) -> IntentDescriptor {
        IntentDescriptor {
            intent_id: {
                let mut id = [0u8; 32];
                id[0] = 0xCC;
                id
            },
            intent_class: "contract_call".into(),
            sender: [1u8; 32],
            max_fee,
            deadline_epoch: 100,
            params: serde_json::json!({
                "schema_id": hex::encode(schema_id),
                "contract_id": hex::encode(contract_id),
                "method": method,
                "proposed_state": hex::encode(proposed_state),
            }),
        }
    }

    #[test]
    fn test_contract_call_plan() {
        let solver = ContractCallSolver::new();
        let schema_id = [0xAA; 32];
        let contract_id = [0xBB; 32];
        let state = b"funded:1000";

        let intents = vec![make_contract_intent(schema_id, contract_id, "fund", state, 100)];
        let plan = solver.build_plan(&intents, 1, [0x01; 32]);
        assert!(plan.is_some());

        let plan = plan.unwrap();
        assert_eq!(plan.target_intent_ids.len(), 1);
        assert_eq!(plan.execution_steps.len(), 1);
        assert_eq!(plan.execution_steps[0].object_id, contract_id);
        assert_eq!(plan.fee_quote, 50);
    }

    #[test]
    fn test_fee_too_high() {
        let solver = ContractCallSolver::with_fee(200);
        let intents = vec![make_contract_intent([0xAA; 32], [0xBB; 32], "fund", b"state", 100)];
        let plan = solver.build_plan(&intents, 1, [0x01; 32]);
        assert!(plan.is_none());
    }

    #[test]
    fn test_wrong_class_filtered() {
        let solver = ContractCallSolver::new();
        let mut intent = make_contract_intent([0xAA; 32], [0xBB; 32], "fund", b"state", 100);
        intent.intent_class = "transfer".into();
        let plan = solver.build_plan(&[intent], 1, [0x01; 32]);
        assert!(plan.is_none());
    }

    #[test]
    fn test_deterministic_bundle() {
        let solver = ContractCallSolver::new();
        let intents = vec![make_contract_intent([0xAA; 32], [0xBB; 32], "fund", b"state", 100)];
        let p1 = solver.build_plan(&intents, 1, [0x01; 32]).unwrap();
        let p2 = solver.build_plan(&intents, 1, [0x01; 32]).unwrap();
        assert_eq!(p1.bundle_id, p2.bundle_id);
    }

    #[test]
    fn test_supported_classes() {
        let solver = ContractCallSolver::new();
        assert_eq!(solver.supported_classes(), vec!["contract_call".to_string()]);
    }
}
