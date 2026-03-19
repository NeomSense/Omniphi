//! Simple transfer solver — reference implementation.
//!
//! Handles `transfer` intents by building a single-step execution plan.
//! This is the simplest possible solver to demonstrate the SDK.

use sha2::{Sha256, Digest};
use rand::RngCore;

use omniphi_poseq::auction::types::{
    ExecutionStep, OperationType, OperationParams,
};
use omniphi_poseq::intent_pool::types::{AssetId, AssetType};
use super::{IntentDescriptor, SolverPlan, SolverStrategy};

/// A reference solver that handles transfer intents.
pub struct SimpleTransferSolver {
    /// Base fee this solver charges (in base units).
    pub base_fee: u64,
}

impl SimpleTransferSolver {
    pub fn new() -> Self {
        SimpleTransferSolver { base_fee: 10 }
    }

    pub fn with_fee(base_fee: u64) -> Self {
        SimpleTransferSolver { base_fee }
    }

    fn build_transfer_plan(
        &self,
        intent: &IntentDescriptor,
        batch_window: u64,
        solver_id: [u8; 32],
    ) -> Option<SolverPlan> {
        // Parse transfer parameters
        let asset_id_hex = intent.params.get("asset_id")?.as_str()?;
        let amount = intent.params.get("amount")?.as_u64()? as u128;
        let recipient_hex = intent.params.get("recipient")?.as_str()?;

        let asset_bytes = hex::decode(asset_id_hex).ok()?;
        let recipient_bytes = hex::decode(recipient_hex).ok()?;

        if asset_bytes.len() != 32 || recipient_bytes.len() != 32 {
            return None;
        }

        // Check fee budget
        if self.base_fee > intent.max_fee {
            return None;
        }

        let mut asset_id = [0u8; 32];
        asset_id.copy_from_slice(&asset_bytes);
        let mut recipient = [0u8; 32];
        recipient.copy_from_slice(&recipient_bytes);

        // Generate bundle_id deterministically
        let mut bundle_hasher = Sha256::new();
        bundle_hasher.update(&solver_id);
        bundle_hasher.update(&intent.intent_id);
        bundle_hasher.update(&batch_window.to_be_bytes());
        let bundle_id: [u8; 32] = bundle_hasher.finalize().into();

        // Random nonce
        let mut nonce = [0u8; 32];
        rand::thread_rng().fill_bytes(&mut nonce);

        // Build execution steps:
        // Step 0: Debit sender
        // Step 1: Credit recipient
        let steps = vec![
            ExecutionStep {
                step_index: 0,
                operation: OperationType::Debit,
                object_id: intent.sender,
                read_set: vec![intent.sender],
                write_set: vec![intent.sender],
                params: OperationParams {
                    asset: Some(AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: asset_id }),
                    amount: Some(amount),
                    recipient: None,
                    pool_id: None,
                    custom_data: None,
                },
            },
            ExecutionStep {
                step_index: 1,
                operation: OperationType::Credit,
                object_id: recipient,
                read_set: vec![recipient],
                write_set: vec![recipient],
                params: OperationParams {
                    asset: Some(AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: asset_id }),
                    amount: Some(amount),
                    recipient: Some(recipient),
                    pool_id: None,
                    custom_data: None,
                },
            },
        ];

        Some(SolverPlan {
            bundle_id,
            target_intent_ids: vec![intent.intent_id],
            execution_steps: steps,
            predicted_output_amount: amount,
            fee_quote: self.base_fee,
            nonce,
        })
    }
}

impl SolverStrategy for SimpleTransferSolver {
    fn name(&self) -> &str {
        "SimpleTransferSolver"
    }

    fn supported_classes(&self) -> Vec<String> {
        vec!["transfer".into()]
    }

    fn build_plan(
        &self,
        intents: &[IntentDescriptor],
        batch_window: u64,
        solver_id: [u8; 32],
    ) -> Option<SolverPlan> {
        // Take the first supported intent and build a plan for it
        let supported = self.filter_intents(intents);
        let intent = supported.first()?;
        self.build_transfer_plan(intent, batch_window, solver_id)
    }

    fn on_win(&self, bundle_id: [u8; 32], _intents: &[IntentDescriptor]) {
        println!(
            "[SimpleTransferSolver] Won auction for bundle {}",
            hex::encode(&bundle_id[..4])
        );
    }

    fn on_loss(&self, bundle_id: [u8; 32]) {
        println!(
            "[SimpleTransferSolver] Lost auction for bundle {}",
            hex::encode(&bundle_id[..4])
        );
    }

    fn on_settlement(&self, bundle_id: [u8; 32], success: bool) {
        println!(
            "[SimpleTransferSolver] Settlement {}: bundle {}",
            if success { "SUCCEEDED" } else { "FAILED" },
            hex::encode(&bundle_id[..4])
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_transfer_intent(sender_byte: u8, amount: u64, max_fee: u64) -> IntentDescriptor {
        let mut sender = [0u8; 32];
        sender[0] = sender_byte;
        let asset = [0xAA; 32];
        let recipient = [0xBB; 32];

        IntentDescriptor {
            intent_id: {
                let mut id = [0u8; 32];
                id[0] = sender_byte;
                id[1] = 0xFF;
                id
            },
            intent_class: "transfer".into(),
            sender,
            max_fee,
            deadline_epoch: 100,
            params: serde_json::json!({
                "asset_id": hex::encode(asset),
                "amount": amount,
                "recipient": hex::encode(recipient),
            }),
        }
    }

    #[test]
    fn test_simple_transfer_plan() {
        let solver = SimpleTransferSolver::new();
        let intents = vec![make_transfer_intent(1, 1000, 50)];
        let solver_id = [0x01; 32];

        let plan = solver.build_plan(&intents, 1, solver_id);
        assert!(plan.is_some());

        let plan = plan.unwrap();
        assert_eq!(plan.target_intent_ids.len(), 1);
        assert_eq!(plan.execution_steps.len(), 2);
        assert_eq!(plan.predicted_output_amount, 1000);
        assert_eq!(plan.fee_quote, 10);
    }

    #[test]
    fn test_fee_too_high_rejected() {
        let solver = SimpleTransferSolver::with_fee(100);
        let intents = vec![make_transfer_intent(1, 1000, 50)]; // max_fee=50 < solver_fee=100
        let solver_id = [0x01; 32];

        let plan = solver.build_plan(&intents, 1, solver_id);
        assert!(plan.is_none());
    }

    #[test]
    fn test_wrong_intent_class_filtered() {
        let solver = SimpleTransferSolver::new();
        let mut intent = make_transfer_intent(1, 1000, 50);
        intent.intent_class = "swap".into();

        let plan = solver.build_plan(&[intent], 1, [0x01; 32]);
        assert!(plan.is_none());
    }

    #[test]
    fn test_supported_classes() {
        let solver = SimpleTransferSolver::new();
        assert_eq!(solver.supported_classes(), vec!["transfer".to_string()]);
    }

    #[test]
    fn test_name() {
        let solver = SimpleTransferSolver::new();
        assert_eq!(solver.name(), "SimpleTransferSolver");
    }

    #[test]
    fn test_deterministic_bundle_id() {
        let solver = SimpleTransferSolver::new();
        let intents = vec![make_transfer_intent(1, 1000, 50)];
        let solver_id = [0x01; 32];

        let p1 = solver.build_plan(&intents, 1, solver_id).unwrap();
        let p2 = solver.build_plan(&intents, 1, solver_id).unwrap();
        assert_eq!(p1.bundle_id, p2.bundle_id);
    }

    #[test]
    fn test_different_batch_window_different_bundle() {
        let solver = SimpleTransferSolver::new();
        let intents = vec![make_transfer_intent(1, 1000, 50)];
        let solver_id = [0x01; 32];

        let p1 = solver.build_plan(&intents, 1, solver_id).unwrap();
        let p2 = solver.build_plan(&intents, 2, solver_id).unwrap();
        assert_ne!(p1.bundle_id, p2.bundle_id);
    }

    #[test]
    fn test_invalid_params_returns_none() {
        let solver = SimpleTransferSolver::new();
        let intent = IntentDescriptor {
            intent_id: [1u8; 32],
            intent_class: "transfer".into(),
            sender: [2u8; 32],
            max_fee: 50,
            deadline_epoch: 100,
            params: serde_json::json!({}), // missing fields
        };

        let plan = solver.build_plan(&[intent], 1, [0x01; 32]);
        assert!(plan.is_none());
    }
}
