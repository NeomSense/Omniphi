//! Auction state machine — manages commit/reveal phases per batch window.
//!
//! Each batch window has three phases:
//! 1. CommitPhase (COMMIT_PHASE_BLOCKS blocks)
//! 2. RevealPhase (REVEAL_PHASE_BLOCKS blocks)
//! 3. SelectionPhase (instant, after reveal closes)

use std::collections::{BTreeMap, BTreeSet};

use super::types::*;
use crate::intent_pool::constants::*;

/// Phase of the current auction.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AuctionPhase {
    /// Accepting commitments.
    Commit,
    /// Accepting reveals.
    Reveal,
    /// Ordering/selection (instant transition after reveal closes).
    Selection,
    /// Window has completed.
    Closed,
}

/// State for one batch window's auction.
pub struct AuctionWindow {
    pub batch_window: u64,
    pub commit_start_block: u64,
    pub reveal_start_block: u64,
    pub selection_block: u64,

    /// All commitments: (bundle_id) → BundleCommitment
    commitments: BTreeMap<[u8; 32], BundleCommitment>,

    /// Commitment count per solver (for rate limiting).
    solver_commitment_counts: BTreeMap<[u8; 32], usize>,

    /// All valid reveals: (bundle_id) → BundleReveal
    reveals: BTreeMap<[u8; 32], BundleReveal>,

    /// Solvers that committed but did not reveal.
    committed_but_not_revealed: BTreeSet<[u8; 32]>,
}

impl AuctionWindow {
    pub fn new(batch_window: u64, start_block: u64) -> Self {
        AuctionWindow {
            batch_window,
            commit_start_block: start_block,
            reveal_start_block: start_block + COMMIT_PHASE_BLOCKS,
            selection_block: start_block + COMMIT_PHASE_BLOCKS + REVEAL_PHASE_BLOCKS,
            commitments: BTreeMap::new(),
            solver_commitment_counts: BTreeMap::new(),
            reveals: BTreeMap::new(),
            committed_but_not_revealed: BTreeSet::new(),
        }
    }

    /// Determine the current phase based on block height.
    pub fn phase_at(&self, block: u64) -> AuctionPhase {
        if block < self.reveal_start_block {
            AuctionPhase::Commit
        } else if block < self.selection_block {
            AuctionPhase::Reveal
        } else if block == self.selection_block {
            AuctionPhase::Selection
        } else {
            AuctionPhase::Closed
        }
    }

    /// Record a commitment. Only valid during CommitPhase.
    pub fn record_commitment(&mut self, commitment: BundleCommitment, current_block: u64) -> Result<(), AuctionError> {
        // Phase check
        if self.phase_at(current_block) != AuctionPhase::Commit {
            return Err(AuctionError::CommitPhaseEnded);
        }

        // Rate limit per solver
        let count = self.solver_commitment_counts.entry(commitment.solver_id).or_insert(0);
        if *count >= MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW {
            return Err(AuctionError::MaxCommitmentsExceeded {
                solver_id: commitment.solver_id,
                max: MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW,
            });
        }

        // Duplicate check
        if self.commitments.contains_key(&commitment.bundle_id) {
            return Err(AuctionError::DuplicateCommitment(commitment.bundle_id));
        }

        self.commitments.insert(commitment.bundle_id, commitment.clone());
        *self.solver_commitment_counts.entry(commitment.solver_id).or_insert(0) += 1;

        Ok(())
    }

    /// Record a reveal. Only valid during RevealPhase.
    /// Verifies that a matching commitment exists and hashes match.
    pub fn record_reveal(&mut self, reveal: BundleReveal, current_block: u64) -> Result<(), AuctionError> {
        // Phase check
        let phase = self.phase_at(current_block);
        if phase == AuctionPhase::Commit {
            return Err(AuctionError::RevealPhaseNotStarted);
        }
        if phase != AuctionPhase::Reveal {
            return Err(AuctionError::RevealPhaseClosed);
        }

        // Duplicate reveal check
        if self.reveals.contains_key(&reveal.bundle_id) {
            return Err(AuctionError::DuplicateReveal(reveal.bundle_id));
        }

        // Find matching commitment
        let commitment = self.commitments.get(&reveal.bundle_id)
            .ok_or(AuctionError::NoMatchingCommitment(reveal.bundle_id))?;

        // Verify hashes
        reveal.verify_against_commitment(commitment)
            .map_err(AuctionError::RevealValidation)?;

        // Step count limit
        if reveal.execution_steps.len() > MAX_BUNDLE_STEPS {
            return Err(AuctionError::TooManySteps {
                count: reveal.execution_steps.len(),
                max: MAX_BUNDLE_STEPS,
            });
        }

        self.reveals.insert(reveal.bundle_id, reveal);

        Ok(())
    }

    /// After reveal phase ends, identify solvers that committed but did not reveal.
    pub fn finalize_no_reveals(&mut self) -> Vec<NoRevealRecord> {
        self.committed_but_not_revealed.clear();
        let mut no_reveals = Vec::new();

        for (bundle_id, commitment) in &self.commitments {
            if !self.reveals.contains_key(bundle_id) {
                self.committed_but_not_revealed.insert(commitment.solver_id);
                no_reveals.push(NoRevealRecord {
                    bundle_id: *bundle_id,
                    solver_id: commitment.solver_id,
                    batch_window: self.batch_window,
                    bond_locked: commitment.bond_locked,
                    penalty_bps: COMMIT_WITHOUT_REVEAL_PENALTY_BPS,
                });
            }
        }

        no_reveals
    }

    /// Get all valid reveals for ordering.
    pub fn valid_reveals(&self) -> Vec<&BundleReveal> {
        self.reveals.values().collect()
    }

    /// Get reveals grouped by target intent.
    pub fn reveals_by_intent(&self) -> BTreeMap<[u8; 32], Vec<&BundleReveal>> {
        let mut grouped: BTreeMap<[u8; 32], Vec<&BundleReveal>> = BTreeMap::new();
        for reveal in self.reveals.values() {
            for intent_id in &reveal.target_intent_ids {
                grouped.entry(*intent_id).or_default().push(reveal);
            }
        }
        grouped
    }

    /// Get a specific commitment.
    pub fn get_commitment(&self, bundle_id: &[u8; 32]) -> Option<&BundleCommitment> {
        self.commitments.get(bundle_id)
    }

    /// Get a specific reveal.
    pub fn get_reveal(&self, bundle_id: &[u8; 32]) -> Option<&BundleReveal> {
        self.reveals.get(bundle_id)
    }

    pub fn commitment_count(&self) -> usize {
        self.commitments.len()
    }

    pub fn reveal_count(&self) -> usize {
        self.reveals.len()
    }
}

/// Record of a solver that committed but did not reveal.
#[derive(Debug, Clone)]
pub struct NoRevealRecord {
    pub bundle_id: [u8; 32],
    pub solver_id: [u8; 32],
    pub batch_window: u64,
    pub bond_locked: u128,
    pub penalty_bps: u64,
}

impl NoRevealRecord {
    /// Compute the penalty amount.
    pub fn penalty_amount(&self) -> u128 {
        self.bond_locked * self.penalty_bps as u128 / 10_000
    }
}

/// Auction-level errors.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AuctionError {
    CommitPhaseEnded,
    RevealPhaseNotStarted,
    RevealPhaseClosed,
    DuplicateCommitment([u8; 32]),
    DuplicateReveal([u8; 32]),
    NoMatchingCommitment([u8; 32]),
    MaxCommitmentsExceeded { solver_id: [u8; 32], max: usize },
    TooManySteps { count: usize, max: usize },
    RevealValidation(RevealValidationError),
    WindowClosed,
}

impl std::fmt::Display for AuctionError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::CommitPhaseEnded => write!(f, "commit phase ended"),
            Self::RevealPhaseNotStarted => write!(f, "reveal phase not started"),
            Self::RevealPhaseClosed => write!(f, "reveal phase closed"),
            Self::DuplicateCommitment(id) => write!(f, "duplicate commitment: {}", hex::encode(&id[..4])),
            Self::DuplicateReveal(id) => write!(f, "duplicate reveal: {}", hex::encode(&id[..4])),
            Self::NoMatchingCommitment(id) => write!(f, "no matching commitment for {}", hex::encode(&id[..4])),
            Self::MaxCommitmentsExceeded { solver_id, max } => {
                write!(f, "solver {} exceeded max {} commitments", hex::encode(&solver_id[..4]), max)
            }
            Self::TooManySteps { count, max } => write!(f, "too many steps: {} > {}", count, max),
            Self::RevealValidation(e) => write!(f, "reveal validation: {}", e),
            Self::WindowClosed => write!(f, "auction window closed"),
        }
    }
}

impl std::error::Error for AuctionError {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intent_pool::types::{AssetId, AssetType};

    fn make_asset(b: u8) -> AssetId {
        let mut id = [0u8; 32]; id[0] = b;
        AssetId { chain_id: 0, asset_type: AssetType::Token, identifier: id }
    }

    fn make_commitment_reveal(bundle_byte: u8, solver_byte: u8) -> (BundleCommitment, BundleReveal) {
        let mut bundle_id = [0u8; 32]; bundle_id[0] = bundle_byte;
        let mut solver_id = [0u8; 32]; solver_id[0] = solver_byte;
        let nonce = [bundle_byte; 32];

        let steps = vec![ExecutionStep {
            step_index: 0,
            operation: OperationType::Debit,
            object_id: [10u8; 32],
            read_set: vec![[10u8; 32]],
            write_set: vec![[10u8; 32]],
            params: OperationParams {
                asset: Some(make_asset(1)),
                amount: Some(1000),
                recipient: None,
                pool_id: None,
                custom_data: None,
            },
        }];

        let outputs = vec![PredictedOutput {
            intent_id: [5u8; 32],
            asset_out: make_asset(2),
            amount_out: 950,
            fee_charged_bps: 30,
        }];

        let fee = FeeBreakdown { solver_fee_bps: 20, protocol_fee_bps: 10, total_fee_bps: 30 };

        let reveal = BundleReveal {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_ids: vec![[5u8; 32]],
            execution_steps: steps,
            liquidity_sources: vec![],
            predicted_outputs: outputs,
            fee_breakdown: fee,
            resource_declarations: vec![],
            nonce,
            proof_data: vec![],
            signature: vec![1u8; 64],
        };

        let commitment = BundleCommitment {
            bundle_id,
            solver_id,
            batch_window: 1,
            target_intent_count: 1,
            commitment_hash: reveal.compute_commitment_hash(),
            expected_outputs_hash: reveal.compute_expected_outputs_hash(),
            execution_plan_hash: reveal.compute_execution_plan_hash(),
            valid_until: 100,
            bond_locked: 50_000,
            signature: vec![1u8; 64],
        };

        (commitment, reveal)
    }

    #[test]
    fn test_auction_phases() {
        let window = AuctionWindow::new(1, 100);
        assert_eq!(window.phase_at(100), AuctionPhase::Commit);
        assert_eq!(window.phase_at(104), AuctionPhase::Commit);
        assert_eq!(window.phase_at(105), AuctionPhase::Reveal); // reveal_start = 105
        assert_eq!(window.phase_at(107), AuctionPhase::Reveal);
        assert_eq!(window.phase_at(108), AuctionPhase::Selection); // selection = 108
        assert_eq!(window.phase_at(109), AuctionPhase::Closed);
    }

    #[test]
    fn test_commit_and_reveal_happy_path() {
        let mut window = AuctionWindow::new(1, 100);
        let (commitment, reveal) = make_commitment_reveal(1, 1);

        // Commit during commit phase
        assert!(window.record_commitment(commitment, 102).is_ok());
        assert_eq!(window.commitment_count(), 1);

        // Reveal during reveal phase
        assert!(window.record_reveal(reveal, 106).is_ok());
        assert_eq!(window.reveal_count(), 1);
    }

    #[test]
    fn test_commit_after_phase_rejected() {
        let mut window = AuctionWindow::new(1, 100);
        let (commitment, _) = make_commitment_reveal(1, 1);

        // Try to commit during reveal phase
        let result = window.record_commitment(commitment, 106);
        assert_eq!(result.unwrap_err(), AuctionError::CommitPhaseEnded);
    }

    #[test]
    fn test_reveal_before_phase_rejected() {
        let mut window = AuctionWindow::new(1, 100);
        let (commitment, reveal) = make_commitment_reveal(1, 1);

        window.record_commitment(commitment, 102).unwrap();

        // Try to reveal during commit phase
        let result = window.record_reveal(reveal, 103);
        assert_eq!(result.unwrap_err(), AuctionError::RevealPhaseNotStarted);
    }

    #[test]
    fn test_reveal_without_commitment() {
        let mut window = AuctionWindow::new(1, 100);
        let (_, reveal) = make_commitment_reveal(1, 1);

        let result = window.record_reveal(reveal, 106);
        match result {
            Err(AuctionError::NoMatchingCommitment(_)) => {}
            other => panic!("expected NoMatchingCommitment, got {:?}", other),
        }
    }

    #[test]
    fn test_tampered_reveal_rejected() {
        let mut window = AuctionWindow::new(1, 100);
        let (commitment, mut reveal) = make_commitment_reveal(1, 1);

        window.record_commitment(commitment, 102).unwrap();

        // Tamper with output amount
        reveal.predicted_outputs[0].amount_out = 999;

        let result = window.record_reveal(reveal, 106);
        match result {
            Err(AuctionError::RevealValidation(_)) => {}
            other => panic!("expected RevealValidation, got {:?}", other),
        }
    }

    #[test]
    fn test_no_reveal_tracking() {
        let mut window = AuctionWindow::new(1, 100);
        let (commitment, _) = make_commitment_reveal(1, 1);

        window.record_commitment(commitment, 102).unwrap();

        // Don't reveal — finalize no-reveals
        let no_reveals = window.finalize_no_reveals();
        assert_eq!(no_reveals.len(), 1);
        assert_eq!(no_reveals[0].solver_id[0], 1);
        assert!(no_reveals[0].penalty_amount() > 0);
    }

    #[test]
    fn test_max_commitments_per_solver() {
        let mut window = AuctionWindow::new(1, 100);

        for i in 0..MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW {
            let (mut commitment, _) = make_commitment_reveal(i as u8 + 1, 1);
            commitment.bundle_id[1] = i as u8; // unique bundle_id
            assert!(window.record_commitment(commitment, 102).is_ok());
        }

        // One more should fail
        let (mut commitment, _) = make_commitment_reveal(99, 1);
        commitment.bundle_id[1] = 99;
        match window.record_commitment(commitment, 102) {
            Err(AuctionError::MaxCommitmentsExceeded { .. }) => {}
            other => panic!("expected MaxCommitmentsExceeded, got {:?}", other),
        }
    }

    #[test]
    fn test_reveals_by_intent() {
        let mut window = AuctionWindow::new(1, 100);
        let (c1, r1) = make_commitment_reveal(1, 1);
        let (c2, r2) = make_commitment_reveal(2, 2);

        window.record_commitment(c1, 102).unwrap();
        window.record_commitment(c2, 102).unwrap();
        window.record_reveal(r1, 106).unwrap();
        window.record_reveal(r2, 106).unwrap();

        let by_intent = window.reveals_by_intent();
        // Both target intent [5u8; 32]
        let intent_5 = [5u8; 32];
        assert_eq!(by_intent.get(&intent_5).unwrap().len(), 2);
    }
}
