//! Omniphi Solver SDK
//!
//! Provides the framework for building PoSeq solvers:
//! - `SolverStrategy` trait for custom solving logic
//! - `SolverClient` for connecting to PoSeq nodes
//! - `SolverRunner` for the main event loop
//! - Reference implementation: `SimpleTransferSolver`
//!
//! # Quick Start
//!
//! ```rust,no_run
//! use omniphi_solver::strategy::SolverStrategy;
//! use omniphi_solver::strategy::simple_transfer::SimpleTransferSolver;
//! use omniphi_solver::runner::SolverRunner;
//! use omniphi_solver::config::SolverConfig;
//!
//! let strategy = SimpleTransferSolver::new();
//! let config = SolverConfig::default();
//! // let runner = SolverRunner::new(config, Box::new(strategy));
//! // runner.run().await;
//! ```

pub mod strategy;
pub mod client;
pub mod runner;
pub mod config;
pub mod metrics;
