# Omniphi Intent Execution — Phase 1 Runbook

## Overview

This document explains how to use, test, and operate the Phase 1 intent-based execution system.

---

## System Components

### Rust Crates

| Crate | Path | Purpose |
|-------|------|---------|
| `omniphi-poseq` | `poseq/` | Sequencing, intent pool, auction, ordering, HotStuff BFT |
| `omniphi-runtime` | `runtime/` | Settlement, verification, disputes, economics, receipts |
| `integration` | `integration/` | E2E integration tests |

### Key Modules

| Module | Location | Purpose |
|--------|----------|---------|
| `intent_pool` | `poseq/src/intent_pool/` | Intent types, lifecycle, public pool, anti-spam |
| `auction` | `poseq/src/auction/` | Commit-reveal, ordering, DA checks |
| `verification` | `runtime/src/verification/` | Per-intent constraint checking |
| `disputes` | `runtime/src/disputes/` | Fraud proofs, auto-verification |
| `economics` | `runtime/src/economics/` | Slashing, rewards, bond management |
| `receipts` | `runtime/src/receipts/` | Enhanced receipts, multi-key indexing |

---

## How a Slot Progresses

```
Block N:   Batch window opens → commit phase starts
Block N+5: Commit phase closes → reveal phase opens
Block N+8: Reveal phase closes → selection phase (instant)
           PoSeq leader produces SequencedBatch
           HotStuff proposes block with batch payload
           Validators verify (sequence + DA + commit-reveal integrity)
           QuorumCertificate formed
           FinalizationEnvelope → Settlement Runtime
           Runtime replays execution plans
           Verification checks intent constraints
           Receipts generated and indexed
Block N+9: Next batch window begins
```

---

## How to Submit Intents

### Construct an IntentTransaction

```rust
use omniphi_poseq::intent_pool::types::*;

let swap = SwapIntent {
    asset_in: AssetId::token([1u8; 32]),
    asset_out: AssetId::token([2u8; 32]),
    amount_in: 1_000_000,
    min_amount_out: 950_000,
    max_slippage_bps: 50,
    route_hint: None,
};

let mut intent = IntentTransaction {
    intent_id: [0u8; 32],
    intent_type: IntentKind::Swap(swap),
    version: 1,
    user: user_pubkey,
    nonce: 1,
    recipient: None,
    deadline: current_block + 100,
    valid_from: None,
    max_fee: 100,  // basis points
    tip: Some(50),
    partial_fill_allowed: false,
    solver_permissions: SolverPermissions::default(),
    execution_preferences: ExecutionPrefs::default(),
    witness_hash: None,
    signature: sign_intent(&intent_bytes, &private_key),
    metadata: BTreeMap::new(),
};
intent.intent_id = intent.compute_intent_id();
```

### Admit to Pool

```rust
use omniphi_poseq::intent_pool::pool::IntentPool;

let mut pool = IntentPool::new();
pool.set_block_height(current_block);
let announcement = pool.admit(intent)?;
// Gossip `announcement` to peers
```

---

## How to Register a Solver

### Runtime Solver Registration

```rust
use omniphi_runtime::solver_registry::*;

let profile = SolverProfile {
    solver_id: solver_pubkey,
    display_name: "MySolver".to_string(),
    public_key: solver_pubkey,
    status: SolverStatus::Active,
    capabilities: SolverCapabilities { /* ... */ },
    reputation: SolverReputationRecord::default(),
    stake_amount: 50_000,
    registered_at_epoch: current_epoch,
    last_active_epoch: current_epoch,
    is_agent: false,
    metadata: BTreeMap::new(),
};

registry.register(profile)?;
```

### Bond Management

```rust
use omniphi_runtime::economics::bonding::BondState;

let mut bond = BondState::new(solver_id, 50_000);
bond.lock(10_000)?;      // Lock for a commitment
bond.unlock(10_000);     // After reveal
bond.slash(500);         // On violation
bond.begin_unbonding(5_000, completion_block)?;
```

---

## Commit-Reveal Lifecycle

### Commit Phase

```rust
use omniphi_poseq::auction::*;

let mut window = AuctionWindow::new(batch_window, start_block);

let commitment = BundleCommitment {
    bundle_id: random_id(),
    solver_id: my_solver_id,
    batch_window,
    target_intent_count: 1,
    commitment_hash: reveal.compute_commitment_hash(),
    expected_outputs_hash: reveal.compute_expected_outputs_hash(),
    execution_plan_hash: reveal.compute_execution_plan_hash(),
    valid_until: start_block + 20,
    bond_locked: 10_000,
    signature: sign(&commitment_bytes),
};

window.record_commitment(commitment, current_block)?;
```

### Reveal Phase

```rust
window.record_reveal(reveal, current_block)?;
```

### Selection

```rust
let reveals: Vec<&BundleReveal> = window.valid_reveals();
let result = order_bundles(&reveals, batch_window);
// result.ordered_bundles = canonical execution order
// result.sequence_root = commitment over ordering
```

---

## How Disputes Are Triggered

### Submit a Fraud Proof

```rust
use omniphi_runtime::disputes::*;

let proof = FraudProof {
    proof_id: FraudProof::compute_proof_id(&challenger, &receipt_id, &proof_type),
    proof_type: FraudProofType::FeeViolation,
    challenger: my_pubkey,
    receipt_id,
    evidence: evidence_bytes,
    bond_amount: 1_000,
    submitted_at_block: current_block,
    signature: sign(&proof_bytes),
};

let result = DisputeVerifier::verify(&proof);
match result {
    DisputeVerificationResult::ProofValid { slash_bps, .. } => {
        // Apply solver slash
    }
    DisputeVerificationResult::ProofInvalid { reason } => {
        // Slash challenger bond
    }
    DisputeVerificationResult::RequiresGovernance => {
        // Queue for governance vote
    }
}
```

---

## How to Run Tests

### Unit Tests (per crate)

```bash
# PoSeq crate (includes intent_pool, auction tests)
cd poseq && cargo test --lib

# Runtime crate (includes verification, disputes, economics tests)
cd runtime && cargo test --lib

# Integration tests
cd integration && cargo test
```

### Specific Test Modules

```bash
# Intent pool tests
cargo test --lib intent_pool

# Auction tests
cargo test --lib auction

# Verification tests
cargo test --lib verification

# Dispute tests
cargo test --lib disputes

# Economics tests
cargo test --lib economics
```

---

## Protocol Constants

See `poseq/src/intent_pool/constants.rs` for all Phase 1 constants including:
- Timing: `COMMIT_PHASE_BLOCKS=5`, `REVEAL_PHASE_BLOCKS=3`, `BATCH_WINDOW_BLOCKS=9`
- Economic: `MIN_SOLVER_BOND=10,000`, `ACTIVE_SOLVER_BOND=50,000`
- Limits: `MAX_POOL_SIZE=50,000`, `MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW=10`
- Reputation: `MAX_VIOLATION_SCORE=9,500` (auto-deactivation threshold)

---

## Key Invariants

1. **Deterministic ordering**: Same inputs → same `sequence_root` on all nodes
2. **Commit-reveal binding**: `commitment_hash = SHA256(reveal_preimage)` — tampered reveals rejected
3. **No trust in solver output**: Runtime independently verifies every constraint
4. **Idempotent delivery**: Duplicate batch deliveries safely ignored
5. **Conservation**: No asset creation/destruction during settlement
6. **Anti-censorship**: Forced inclusion after threshold blocks
