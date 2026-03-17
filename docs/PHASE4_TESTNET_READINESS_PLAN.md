# Phase 4: Testnet Readiness Plan

**Date**: 2026-03-15
**Status**: Implementation plan for distributed testnet readiness

---

## 1. Current Module Readiness

| Module | Path | Readiness | Notes |
|--------|------|-----------|-------|
| Intent pool | `poseq/src/intent_pool/` | 95% | Production-ready, needs persistence |
| Auction | `poseq/src/auction/` | 95% | Complete with resource declarations |
| Finality | `poseq/src/finality/` | 90% | Commitment chaining + validator checks done |
| Fairness | `poseq/src/fairness/` | 90% | Log, evidence, censorship detection done |
| Failure paths | `poseq/src/failure/` | 95% | All paths deterministic |
| Invariants | `runtime/src/invariants/` | 95% | 11 invariant checks implemented |
| Observability | `poseq/src/observability/` | 80% | Prometheus metrics + event store; needs tracing |
| Persistence | `poseq/src/persistence/` | 85% | Sled backend + reconciliation; needs intent-pool persistence |
| Simulation | `poseq/src/simulation/` | 75% | In-process deterministic; needs network simulation |
| Recovery | `poseq/src/recovery/` | 70% | Checkpoint logic; needs distributed sync |
| Node/Networking | `poseq/src/networking/` | 80% | TCP transport; needs view-change timeout |
| Verification | `runtime/src/verification/` | 90% | Per-intent checks implemented |
| Disputes | `runtime/src/disputes/` | 90% | Typed evidence verification |
| Economics | `runtime/src/economics/` | 90% | Bonding, slashing, rewards with residual handling |
| Receipts | `runtime/src/receipts/` | 90% | Multi-key indexing + double-fill detection |

## 2. Gaps and Implementation Order

### Priority 1: Distributed Simulation + Recovery
1. Multi-node simulation framework with message delay and Byzantine injection
2. Crash recovery persistence tests
3. Config validation layer

### Priority 2: Benchmarking + Observability
4. Full pipeline benchmarks at scale
5. Structured tracing integration
6. Metrics for Phase 3 components (fairness, finality, resource conflicts)

### Priority 3: Operator Tooling + Packaging
7. Parameter guide with validation
8. Operator runbooks
9. Testnet launch checklist
10. E2E distributed validation

## 3. Implementation Order

```
Step 1: Config validation (foundation for all testing)
Step 2: Multi-node simulation harness
Step 3: Byzantine/adversarial tests
Step 4: Persistence/recovery tests
Step 5: Benchmarks
Step 6: Metrics/tracing
Step 7: Parameter guide
Step 8: Operator docs
Step 9: Testnet packaging
Step 10: E2E distributed validation
```
