//! Solver event loop runner.
//!
//! Orchestrates the solver lifecycle:
//! 1. Connect to PoSeq node
//! 2. Subscribe to batch windows
//! 3. For each window: filter intents → build plan → commit → reveal
//! 4. Monitor outcomes → update metrics

use std::sync::Arc;
use tokio::sync::Mutex;

use crate::config::SolverConfig;
use crate::metrics::SolverMetrics;
use crate::strategy::SolverStrategy;
use crate::client::SolverClient;

/// The main solver runner.
pub struct SolverRunner {
    pub config: SolverConfig,
    pub strategy: Box<dyn SolverStrategy>,
    pub client: SolverClient,
    pub metrics: Arc<Mutex<SolverMetrics>>,
}

/// Current solver state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SolverState {
    /// Waiting for next batch window.
    Idle,
    /// Building plans for current window.
    Planning { batch_window: u64 },
    /// Commitment submitted, waiting for reveal phase.
    Committed { batch_window: u64, bundle_id: [u8; 32] },
    /// Reveal submitted, waiting for selection.
    Revealed { batch_window: u64, bundle_id: [u8; 32] },
    /// Watching settlement outcome.
    AwaitingSettlement { batch_window: u64, bundle_id: [u8; 32] },
    /// Shutting down.
    Stopping,
}

impl SolverRunner {
    pub fn new(config: SolverConfig, strategy: Box<dyn SolverStrategy>) -> Self {
        let solver_id = {
            let bytes = hex::decode(&config.solver_id).unwrap_or_else(|_| vec![0u8; 32]);
            let mut id = [0u8; 32];
            let len = bytes.len().min(32);
            id[..len].copy_from_slice(&bytes[..len]);
            id
        };

        let client = SolverClient::new(config.poseq_endpoint.clone(), solver_id);

        SolverRunner {
            config,
            strategy,
            client,
            metrics: Arc::new(Mutex::new(SolverMetrics::new())),
        }
    }

    /// Get current metrics snapshot.
    pub async fn get_metrics(&self) -> SolverMetrics {
        self.metrics.lock().await.clone()
    }

    /// Log current stats.
    pub async fn print_stats(&self) {
        let m = self.metrics.lock().await;
        println!(
            "[solver] Stats: commits={} reveals={} wins={} losses={} settled={} failed={} win_rate={}bps profit={}",
            m.commitments_submitted,
            m.reveals_submitted,
            m.auctions_won,
            m.auctions_lost,
            m.settlements_succeeded,
            m.settlements_failed,
            m.win_rate_bps(),
            m.net_profit(),
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::strategy::simple_transfer::SimpleTransferSolver;

    #[test]
    fn test_runner_creation() {
        let config = SolverConfig::default();
        let strategy = Box::new(SimpleTransferSolver::new());
        let runner = SolverRunner::new(config, strategy);
        assert_eq!(runner.client.solver_id, [0u8; 32]);
    }

    #[test]
    fn test_solver_state_transitions() {
        let state = SolverState::Idle;
        assert_eq!(state, SolverState::Idle);

        let state = SolverState::Planning { batch_window: 1 };
        assert_eq!(state, SolverState::Planning { batch_window: 1 });
    }

    #[tokio::test]
    async fn test_metrics_tracking() {
        let config = SolverConfig::default();
        let strategy = Box::new(SimpleTransferSolver::new());
        let runner = SolverRunner::new(config, strategy);

        {
            let mut m = runner.metrics.lock().await;
            m.record_commitment("transfer");
            m.record_reveal();
            m.record_win("transfer", 50);
        }

        let metrics = runner.get_metrics().await;
        assert_eq!(metrics.commitments_submitted, 1);
        assert_eq!(metrics.reveals_submitted, 1);
        assert_eq!(metrics.auctions_won, 1);
    }
}
