# Intent Execution Implementation Plan

**Date**: 2026-03-15
**Spec**: `docs/INTENT_EXECUTION_ARCHITECTURE.md`
**Status**: Phase 1 Implementation

---

## 1. Codebase Audit Summary

### What Already Exists

| Component | Location | Status |
|-----------|----------|--------|
| PoSeq sequencing pipeline | `poseq/src/` (50+ modules) | **Complete** — intake, validation, queue, ordering, batching, receipts |
| HotStuff BFT | `poseq/src/hotstuff/` | **Complete** — engine, pacemaker, safety, types |
| Networking/gossip | `poseq/src/networking/` | **Complete** — messages, peer_manager, node_runner |
| PoSeq→Runtime bridge | `poseq/src/bridge/` | **Complete** — FinalizationEnvelope, BatchPipeline, HardenedBridge |
| Chain bridge (evidence export) | `poseq/src/chain_bridge/` | **Complete** — evidence, escalation, anchor, exporter |
| Fairness/anti-MEV | `poseq/src/fairness*`, `anti_mev/`, `inclusion/` | **Complete** |
| Runtime intent types | `runtime/src/intents/` | **Partial** — Transfer, Swap, YieldAllocate, TreasuryRebalance exist but no RouteLiquidityIntent |
| Object model | `runtime/src/objects/` | **Complete** — Balance, Pool, Vault, Token, Wallet, Identity, etc. |
| Object store | `runtime/src/state/` | **Complete** — typed overlays, state_root, sync |
| Settlement engine | `runtime/src/settlement/` | **Complete** — ExecutionReceipt, SettlementResult, gas metering |
| Parallel scheduler | `runtime/src/scheduler/` | **Complete** — ConflictGraph, ExecutionGroup, greedy coloring |
| Intent resolver | `runtime/src/resolution/` | **Complete** — Transfer, Swap, YieldAllocate, TreasuryRebalance resolvers |
| Solver market types | `runtime/src/solver_market/` | **Complete** — CandidatePlan, PlanAction, PlanActionType |
| Solver registry | `runtime/src/solver_registry/` | **Complete** — SolverProfile, SolverRegistry, reputation |
| Plan validation | `runtime/src/plan_validation/` | **Complete** — PlanValidator, ValidationReasonCode, scoring |
| Plan selection | `runtime/src/selection/` | **Complete** — PlanRanker, WinningPlanSelector |
| Plan policy | `runtime/src/policy/` | **Complete** — CompositePolicy, DomainRisk, MaxValue, SolverSafety |
| PoSeq runtime integration | `runtime/src/poseq/` | **Complete** — PoSeqRuntime, SolverMarketRuntime, RuntimeBatchIngester |
| Capability system | `runtime/src/capabilities/` | **Complete** — CapabilitySet, CapabilityChecker |
| Gas metering | `runtime/src/gas/` | **Complete** — GasMeter, GasCosts |
| x/poseq chain module | `chain/x/poseq/` | **Complete** — keeper, msg_server, genesis, types |
| Integration tests | `integration/src/lib.rs` | **Complete** — 15 E2E tests |

### What Is Missing (Phase 1 Spec Gaps)

| Component | What's Needed | Where It Goes |
|-----------|---------------|---------------|
| **Intent pool** | Dedicated pool with admission, dedup, expiry, anti-spam, solver subscriptions | `poseq/src/intent_pool/` |
| **RouteLiquidityIntent** | New intent type per spec | `runtime/src/intents/types.rs` |
| **Commit-reveal auction** | BundleCommitment, BundleReveal, auction state machine, hash verification | `poseq/src/auction/` |
| **Intent-aware ordering** | Extend PoSeq ordering to rank by user_outcome_score per intent group | `poseq/src/ordering/` (extend) |
| **Solver bonding** | On-chain bond locking/slashing enforcement (beyond reputation) | `runtime/src/solver_registry/` (extend) |
| **Settlement verification** | Per-intent constraint checking (distinct from resolution) | `runtime/src/verification/` |
| **Dispute system** | FraudProof types, submission, verification, resolution | `runtime/src/disputes/` |
| **Slashing/rewards** | Economic enforcement, reward distribution | `runtime/src/economics/` |
| **DA enforcement** | Pre-vote DA checks for validators | `poseq/src/hotstuff/` (extend) |
| **Protocol constants** | Centralized constants file per spec Appendix A | `poseq/src/intent_pool/constants.rs` + `runtime/src/constants.rs` |
| **Intent lifecycle state machine** | Explicit state tracking (Created→Settled) | `poseq/src/intent_pool/lifecycle.rs` |

### What Needs Extension

| Existing Module | Extension Needed |
|----------------|------------------|
| `PoSeqMessage` enum | Add `IntentAnnouncement`, `BundleCommitment`, `BundleReveal`, `AuctionPhaseChange` variants |
| `PoSeqNode` | Add `intent_pool`, `auction_state` fields; new methods for intent-aware batch production |
| `OrderingEngine` | Add intent-grouping mode that ranks by `user_outcome_score` with SHA256 tiebreak |
| `SubmissionClass` | Add `IntentBundle` variant |
| `SolverRegistry` | Add bond enforcement (min bond checks), unbonding queue |
| `SettlementEngine` | Add post-execution constraint verification hook |
| `RuntimeBatchIngester` | Accept bundles (not just opaque submissions) with full execution plans |
| `HotStuffEngine` | Add pre-vote verification callback for DA + commit-reveal checks |

---

## 2. Module Layout

### New Modules

```
poseq/src/
├── intent_pool/          # NEW — Section 2, 3
│   ├── mod.rs
│   ├── types.rs          # IntentTransaction, SwapIntent, PaymentIntent, RouteLiquidityIntent
│   ├── lifecycle.rs      # IntentState, state machine transitions
│   ├── pool.rs           # IntentPool: admission, dedup, expiry, eviction
│   ├── subscription.rs   # SolverSubscription, intent stream filtering
│   └── constants.rs      # Protocol constants (Appendix A)
│
├── auction/              # NEW — Section 5
│   ├── mod.rs
│   ├── types.rs          # BundleCommitment, BundleReveal, ExecutionStep, etc.
│   ├── commitment.rs     # Commitment validation, hash construction
│   ├── reveal.rs         # Reveal validation, commitment matching
│   ├── state.rs          # AuctionState per batch window (commit/reveal/select phases)
│   └── penalties.rs      # Commit-without-reveal tracking

runtime/src/
├── verification/         # NEW — Section 9
│   ├── mod.rs
│   ├── swap.rs           # SwapIntent constraint verification
│   ├── payment.rs        # PaymentIntent constraint verification
│   ├── route.rs          # RouteLiquidityIntent constraint verification
│   └── types.rs          # VerificationResult, VerificationError
│
├── disputes/             # NEW — Section 11
│   ├── mod.rs
│   ├── types.rs          # FraudProof, FraudProofType, DisputeRecord
│   ├── verifier.rs       # Auto-verifiable fraud proof checking
│   └── resolution.rs     # Dispute state machine, challenger rewards
│
├── economics/            # NEW — Section 12
│   ├── mod.rs
│   ├── slashing.rs       # Solver/validator penalty application
│   ├── rewards.rs        # Epoch reward computation and distribution
│   └── bonding.rs        # Bond locking, unbonding queue, min-bond enforcement
│
├── receipts/             # NEW — Section 10
│   ├── mod.rs
│   └── indexing.rs       # Receipt indexing by intent, solver, batch
```

### Extended Modules

```
poseq/src/
├── networking/messages.rs  # Add new PoSeqMessage variants
├── ordering/engine.rs      # Add intent-aware ordering mode
├── config/policy.rs        # Add IntentBundle submission class
├── lib.rs                  # Add intent_pool, auction modules; extend PoSeqNode

runtime/src/
├── intents/types.rs        # Add RouteLiquidityIntent
├── solver_registry/        # Add bond enforcement
├── settlement/engine.rs    # Add verification hook
├── poseq/interface.rs      # Accept solver bundles with execution plans
├── lib.rs                  # Add verification, disputes, economics, receipts modules
```

---

## 3. Integration Points

```
Intent Submission → IntentPool.admit()
                         │
                         ▼
              IntentPool.gossip() → PoSeqMessage::IntentAnnouncement
                         │
                         ▼
            SolverSubscription.filter() → Solver sees intent
                         │
                         ▼
         AuctionState::CommitPhase
              Solver → BundleCommitment → AuctionState.record_commitment()
                         │
                         ▼
         AuctionState::RevealPhase
              Solver → BundleReveal → AuctionState.validate_reveal()
                         │
                         ▼
         AuctionState::SelectionPhase
              AuctionState.get_valid_reveals()
                         │
                         ▼
         PoSeqNode.produce_intent_batch()
              IntentAwareOrdering.order() → SequencedBatch
                         │
                         ▼
         HotStuffEngine.on_propose()
              Validators verify (sequence + DA + commit-reveal)
              QuorumCertificate formed
                         │
                         ▼
         FinalizationEnvelope → RuntimeBatchIngester
              Resolve bundles → ExecutionPlans
              Schedule → ExecutionGroups
              Execute → SettlementEngine
              Verify → SettlementVerifier (per-intent constraints)
              Receipt → ExecutionReceipt + indexing
                         │
                         ▼
         Post-settlement:
              Dispute window opens
              Receipts queryable
              Rewards accrue
```

---

## 4. Dependencies Between Sections

```
Section 1 (skeleton) ──────────┐
    │                          │
    ▼                          │
Section 2 (intent types) ─────┤
    │                          │
    ▼                          │
Section 3 (intent pool) ──────┤
    │                          │
    ▼                          │
Section 4 (solver bonds) ─────┤    All depend on Section 1
    │                          │    types and constants
    ▼                          │
Section 5 (auction) ──────────┤
    │                          │
    ▼                          │
Section 6 (PoSeq ordering) ───┘
    │
    ▼ (requires 2-6 complete)
Section 7 (HotStuff integration)
    │
    ▼ (requires 7 complete)
Section 8 (settlement extensions)
    │
    ▼ (requires 8)
Section 9 (verification)
    │
    ├──▶ Section 10 (receipts) ──▶ Section 11 (disputes) ──▶ Section 12 (slashing/rewards)
    │
    ▼
Section 13 (DA) — can be done after Section 7
Section 14 (storage) — can be done after Section 10
Section 15 (E2E tests) — requires all above
```

---

## 5. Implementation Order

1. **Section 1**: Core skeleton — constants, shared types, error enums, config scaffolding
2. **Section 2**: Intent types — IntentTransaction extensions, RouteLiquidityIntent, lifecycle states
3. **Section 3**: Intent pool — pool struct, admission, dedup, expiry, subscriptions
4. **Section 4**: Solver bonding — extend SolverRegistry with bond enforcement
5. **Section 5**: Commit-reveal auction — BundleCommitment/Reveal types, state machine, hash verification
6. **Section 6**: PoSeq ordering — intent-aware ordering, winner selection, sequence commitment
7. **Section 7**: HotStuff integration — pre-vote verification, DA checks
8. **Section 8**: Settlement extensions — bundle→ExecutionPlan conversion in ingestion path
9. **Section 9**: Verification — per-intent constraint checking post-execution
10. **Section 10**: Receipts — enhanced receipt with intent metadata, indexing
11. **Section 11**: Disputes — fraud proof types, auto-verification
12. **Section 12**: Slashing/rewards — penalty application, reward distribution
13. **Section 13**: DA enforcement — validator-side DA validation
14. **Section 14**: Storage hardening — pruning, retention, deterministic encoding
15. **Section 15**: E2E tests — full pipeline tests

---

## 6. Risks and Blockers

| Risk | Mitigation |
|------|------------|
| PoSeqNode struct has many fields already; adding intent_pool + auction increases complexity | Keep intent_pool and auction as separate owned structs with clean interfaces |
| OrderingEngine currently sorts by priority/nonce; intent-aware mode needs different ranking | Add `IntentAwareOrderingEngine` as separate struct that wraps `OrderingEngine` for non-intent fallback |
| SolverRegistry in runtime has `stake_amount: u128` but it's a "placeholder, not enforced" | Extend with actual min-bond enforcement and unbonding queue |
| PoSeqMessage enum already has 20+ variants | Group new variants logically; consider sub-enum if needed |
| `SequencingSubmission` uses `[u8; 64]` for signature but `WireSignedEnvelope` uses `Vec<u8>` | Keep `SequencingSubmission.signature` as-is (internal); new intent/auction types use `Vec<u8>` |
| Existing `IntentTransaction` in runtime uses `tx_id: [u8; 32]` but spec uses `intent_id` | Add `intent_id` alias; keep backward compat with `tx_id` |
| Integration test crate depends on both poseq and runtime; adding auction types requires poseq dep update | Ensure `poseq/Cargo.toml` stays in sync |
