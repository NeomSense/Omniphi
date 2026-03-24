//! Solver strategy trait and implementations.
//!
//! To build a custom solver, implement the `SolverStrategy` trait.

pub mod simple_transfer;
pub mod contract_call;

use omniphi_poseq::auction::types::{BundleCommitment, BundleReveal, ExecutionStep};

/// An intent descriptor received from the PoSeq intent pool.
#[derive(Debug, Clone)]
pub struct IntentDescriptor {
    pub intent_id: [u8; 32],
    pub intent_class: String,
    pub sender: [u8; 32],
    pub max_fee: u64,
    pub deadline_epoch: u64,
    /// Serialized intent-specific parameters.
    pub params: serde_json::Value,
}

/// A solver's plan for fulfilling a set of intents.
#[derive(Debug, Clone)]
pub struct SolverPlan {
    pub bundle_id: [u8; 32],
    pub target_intent_ids: Vec<[u8; 32]>,
    pub execution_steps: Vec<ExecutionStep>,
    pub predicted_output_amount: u128,
    pub fee_quote: u64,
    pub nonce: [u8; 32],
}

/// The core trait that every solver strategy must implement.
pub trait SolverStrategy: Send + Sync {
    /// Name of this strategy (for logging/metrics).
    fn name(&self) -> &str;

    /// Which intent classes this strategy handles.
    fn supported_classes(&self) -> Vec<String>;

    /// Filter intents this solver can fill.
    fn filter_intents(&self, intents: &[IntentDescriptor]) -> Vec<IntentDescriptor> {
        let supported = self.supported_classes();
        intents.iter()
            .filter(|i| supported.contains(&i.intent_class))
            .cloned()
            .collect()
    }

    /// Build a plan for the given intents. Returns None if no viable plan.
    fn build_plan(
        &self,
        intents: &[IntentDescriptor],
        batch_window: u64,
        solver_id: [u8; 32],
    ) -> Option<SolverPlan>;

    /// Called when our bundle wins the auction.
    fn on_win(&self, bundle_id: [u8; 32], intents: &[IntentDescriptor]) {
        let _ = (bundle_id, intents);
    }

    /// Called when our bundle loses the auction.
    fn on_loss(&self, bundle_id: [u8; 32]) {
        let _ = bundle_id;
    }

    /// Called when settlement completes (success or failure).
    fn on_settlement(&self, bundle_id: [u8; 32], success: bool) {
        let _ = (bundle_id, success);
    }
}
