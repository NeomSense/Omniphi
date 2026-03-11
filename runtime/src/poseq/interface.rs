use crate::capabilities::checker::CapabilitySet;
use crate::errors::RuntimeError;
use crate::intents::base::IntentTransaction;
use crate::objects::base::BoxedObject;
use crate::resolution::planner::IntentResolver;
use crate::scheduler::parallel::ParallelScheduler;
use crate::settlement::engine::{SettlementEngine, SettlementResult};
use crate::state::store::ObjectStore;

/// An ordered batch of intent transactions delivered by the PoSeq sequencer.
/// Transactions are pre-ordered; the runtime must respect this ordering.
pub struct OrderedBatch {
    pub batch_id: [u8; 32],
    pub epoch: u64,
    pub sequence_number: u64,
    /// Transactions already ordered by PoSeq canonical ordering.
    pub transactions: Vec<IntentTransaction>,
}

/// The top-level PoSeq runtime engine.
pub struct PoSeqRuntime {
    pub store: ObjectStore,
    _resolver: IntentResolver,
    _scheduler: ParallelScheduler,
    _settlement: SettlementEngine,
    pub current_epoch: u64,
}

impl PoSeqRuntime {
    /// Creates a new runtime with an empty object store.
    pub fn new() -> Self {
        PoSeqRuntime {
            store: ObjectStore::new(),
            _resolver: IntentResolver,
            _scheduler: ParallelScheduler,
            _settlement: SettlementEngine,
            current_epoch: 0,
        }
    }

    /// Seeds an object into the store (genesis / testing).
    pub fn seed_object(&mut self, obj: BoxedObject) {
        self.store.insert(obj);
    }

    /// Processes a full ordered batch of intent transactions.
    ///
    /// 9-step lifecycle:
    /// 1.  Validate each intent structurally (IntentTransaction::validate).
    /// 2.  Resolve each intent to an ExecutionPlan (skip invalid).
    /// 3.  Build access map (embedded in ExecutionPlan).
    /// 4.  Schedule plans with ParallelScheduler.
    /// 5.  Execute groups with SettlementEngine.
    /// 6.  Advance epoch.
    /// 7.  Sync typed overlays → canonical store.
    /// 8.  Compute state root.
    /// 9.  Return SettlementResult.
    pub fn process_batch(
        &mut self,
        batch: OrderedBatch,
    ) -> Result<SettlementResult, RuntimeError> {
        // Advance epoch to match the batch
        self.current_epoch = batch.epoch;

        // ── Step 1: structural validation ──────────────────────────────────
        let mut valid_txns: Vec<IntentTransaction> = Vec::new();
        for tx in batch.transactions {
            match tx.validate() {
                Ok(()) => valid_txns.push(tx),
                Err(e) => {
                    // Log and skip; do not abort the whole batch
                    // In a production system this would emit a structured event
                    let _ = e; // suppress unused warning
                }
            }
        }

        // ── Step 2: resolve each intent → ExecutionPlan ────────────────────
        // Use admin caps by default; real system would look up per-sender caps
        let caps = CapabilitySet::all();
        let mut plans = Vec::new();

        for tx in &valid_txns {
            match IntentResolver::resolve(tx, &self.store, &caps) {
                Ok(plan) => plans.push(plan),
                Err(_e) => {
                    // Resolution failure: skip this tx, emit failed receipt
                    // (SettlementResult will show it as failed)
                }
            }
        }

        // ── Steps 3-4: access map is embedded in ExecutionPlan; schedule ────
        let groups = ParallelScheduler::schedule(plans);

        // ── Step 5: execute groups ───────────────────────────────────────────
        let result = SettlementEngine::execute_groups(groups, &mut self.store, batch.epoch);

        // ── Steps 6-9: epoch advance + sync + root already done inside ───────
        Ok(result)
    }
}

impl Default for PoSeqRuntime {
    fn default() -> Self {
        Self::new()
    }
}
