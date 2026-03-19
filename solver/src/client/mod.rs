//! PoSeq node client for solver interactions.
//!
//! Provides methods to:
//! - Query the intent pool
//! - Submit bundle commitments
//! - Submit bundle reveals
//! - Monitor auction outcomes
//! - Track settlement results

use sha2::{Sha256, Digest};
use omniphi_poseq::auction::types::{
    BundleCommitment, BundleReveal, ExecutionStep, PredictedOutput, FeeBreakdown,
    LiquiditySource,
};
use omniphi_poseq::auction::types::ResourceAccess;
use omniphi_poseq::intent_pool::types::{AssetId, AssetType};
use crate::strategy::SolverPlan;

/// Client for interacting with a PoSeq node.
pub struct SolverClient {
    pub endpoint: String,
    pub solver_id: [u8; 32],
}

impl SolverClient {
    pub fn new(endpoint: String, solver_id: [u8; 32]) -> Self {
        SolverClient { endpoint, solver_id }
    }

    /// Build a `BundleCommitment` from a solver plan.
    ///
    /// Internally builds the full reveal first to compute hashes that match
    /// `BundleReveal::compute_commitment_hash()` exactly.
    pub fn build_commitment(
        &self,
        plan: &SolverPlan,
        batch_window: u64,
        bond: u128,
        valid_until: u64,
    ) -> BundleCommitment {
        // Build the corresponding reveal to derive hashes that will verify later
        let reveal = self.build_reveal(plan, batch_window);

        BundleCommitment {
            bundle_id: plan.bundle_id,
            solver_id: self.solver_id,
            batch_window,
            target_intent_count: plan.target_intent_ids.len() as u16,
            commitment_hash: reveal.compute_commitment_hash(),
            expected_outputs_hash: reveal.compute_expected_outputs_hash(),
            execution_plan_hash: reveal.compute_execution_plan_hash(),
            valid_until,
            bond_locked: bond,
            signature: Vec::new(), // Caller signs after building
        }
    }

    /// Build a `BundleReveal` from a solver plan.
    pub fn build_reveal(
        &self,
        plan: &SolverPlan,
        batch_window: u64,
    ) -> BundleReveal {
        let predicted_outputs: Vec<PredictedOutput> = plan.target_intent_ids.iter().map(|intent_id| {
            PredictedOutput {
                intent_id: *intent_id,
                asset_out: AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: [0u8; 32] },
                amount_out: plan.predicted_output_amount,
                fee_charged_bps: ((plan.fee_quote as u128 * 10000) / plan.predicted_output_amount.max(1)) as u64,
            }
        }).collect();

        let mut resource_decls: Vec<ResourceAccess> = Vec::new();
        for step in &plan.execution_steps {
            for r in &step.read_set {
                resource_decls.push(ResourceAccess::read(*r));
            }
            for w in &step.write_set {
                resource_decls.push(ResourceAccess::write(*w));
            }
        }

        BundleReveal {
            bundle_id: plan.bundle_id,
            solver_id: self.solver_id,
            batch_window,
            target_intent_ids: plan.target_intent_ids.clone(),
            execution_steps: plan.execution_steps.clone(),
            liquidity_sources: Vec::new(),
            predicted_outputs,
            fee_breakdown: FeeBreakdown {
                solver_fee_bps: plan.fee_quote,
                protocol_fee_bps: 0,
                total_fee_bps: plan.fee_quote,
            },
            resource_declarations: resource_decls,
            nonce: plan.nonce,
            proof_data: Vec::new(),
            signature: Vec::new(), // Caller signs after building
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use omniphi_poseq::auction::types::{OperationType, OperationParams};
    use omniphi_poseq::intent_pool::types::{AssetId, AssetType};

    fn make_plan() -> SolverPlan {
        let intent_id = [0x01; 32];
        let sender = [0x02; 32];
        let recipient = [0x03; 32];
        let asset = [0xAA; 32];

        SolverPlan {
            bundle_id: [0x10; 32],
            target_intent_ids: vec![intent_id],
            execution_steps: vec![
                ExecutionStep {
                    step_index: 0,
                    operation: OperationType::Debit,
                    object_id: sender,
                    read_set: vec![sender],
                    write_set: vec![sender],
                    params: OperationParams {
                        asset: Some(AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: asset }),
                        amount: Some(1000),
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
                        asset: Some(AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: asset }),
                        amount: Some(1000),
                        recipient: Some(recipient),
                        pool_id: None,
                        custom_data: None,
                    },
                },
            ],
            predicted_output_amount: 1000,
            fee_quote: 10,
            nonce: [0xFF; 32],
        }
    }

    #[test]
    fn test_build_commitment() {
        let client = SolverClient::new("http://localhost".into(), [0x01; 32]);
        let plan = make_plan();
        let commitment = client.build_commitment(&plan, 1, 500, 100);

        assert_eq!(commitment.bundle_id, plan.bundle_id);
        assert_eq!(commitment.solver_id, [0x01; 32]);
        assert_eq!(commitment.batch_window, 1);
        assert_eq!(commitment.target_intent_count, 1);
        assert_eq!(commitment.bond_locked, 500);
        assert_eq!(commitment.valid_until, 100);
        assert_ne!(commitment.commitment_hash, [0u8; 32]);
    }

    #[test]
    fn test_build_reveal() {
        let client = SolverClient::new("http://localhost".into(), [0x01; 32]);
        let plan = make_plan();
        let reveal = client.build_reveal(&plan, 1);

        assert_eq!(reveal.bundle_id, plan.bundle_id);
        assert_eq!(reveal.solver_id, [0x01; 32]);
        assert_eq!(reveal.target_intent_ids.len(), 1);
        assert_eq!(reveal.execution_steps.len(), 2);
        assert_eq!(reveal.fee_breakdown.solver_fee_bps, 10);
        assert_eq!(reveal.nonce, plan.nonce);
    }

    #[test]
    fn test_commitment_reveal_hash_consistency() {
        let client = SolverClient::new("http://localhost".into(), [0x01; 32]);
        let plan = make_plan();
        let commitment = client.build_commitment(&plan, 1, 500, 100);
        let reveal = client.build_reveal(&plan, 1);

        // The reveal should be able to verify against the commitment
        let verify_result = reveal.verify_against_commitment(&commitment);
        assert!(verify_result.is_ok(), "reveal must verify against its commitment: {:?}", verify_result.err());
    }

    #[test]
    fn test_different_plans_different_commitments() {
        let client = SolverClient::new("http://localhost".into(), [0x01; 32]);
        let mut plan1 = make_plan();
        let mut plan2 = make_plan();
        plan2.bundle_id = [0x20; 32];

        let c1 = client.build_commitment(&plan1, 1, 500, 100);
        let c2 = client.build_commitment(&plan2, 1, 500, 100);
        assert_ne!(c1.commitment_hash, c2.commitment_hash);
    }
}
