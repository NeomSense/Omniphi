# Phase 5: Testnet Launch Checklist

## Pre-Launch

### Code Readiness
- [x] Governance parameter registry with validation
- [x] Validator registry with stake enforcement
- [x] Solver registry with bond management
- [x] Commit-reveal auction system
- [x] Deterministic intent-aware ordering
- [x] FinalityCommitment chaining
- [x] 9-point validator pre-vote verification
- [x] Fairness logging and censorship evidence
- [x] Protocol invariants (11 checks)
- [x] Failure path semantics
- [x] Resource-aware bundles
- [x] Settlement verification (Swap/Payment/Route)
- [x] Dispute system with typed evidence
- [x] Slashing and rewards with budget neutrality
- [x] DA validation with bond/solver/expiry checks

### Test Readiness
- [x] 966+ tests passing, 0 failures
- [x] Distributed multi-node simulation (9 scenarios)
- [x] Adversarial tests (solver copying, replay, spam)
- [x] Determinism verification (9 tests)
- [x] Stateful fuzz testing (6 tests)
- [x] Protocol invariant enforcement

### Config Readiness
- [x] `ProtocolParameters::testnet()` preset
- [x] `testnet_config.toml` template
- [x] `validator_config.toml` template
- [x] `solver_config.toml` template
- [x] Parameter validation rejects unsafe ranges

### Observability Readiness
- [x] Prometheus metrics (9 counters/gauges/histograms)
- [x] HTTP exporter at `/metrics`
- [x] Health check at `/healthz`
- [x] Grafana dashboard template
- [x] Fairness log per slot

### Operator Readiness
- [x] Validator operations guide
- [x] Solver operations guide
- [x] Parameter guide
- [x] Economic calibration report
- [x] Troubleshooting procedures

### Recovery Readiness
- [x] Sled crash-safe persistence
- [x] Reconciliation engine
- [x] Recovery checkpoint with hash verification
- [x] Finality chain continuity

## Launch Steps

1. **Generate keys**: `openssl rand -hex 32` for each validator
2. **Configure network**: Edit `testnet_config.toml` with actual addresses
3. **Generate validator configs**: Run `scripts/testnet_launch.sh`
4. **Start validators**: `poseq-node --config config.toml` on each machine
5. **Verify connectivity**: Check `poseq_peer_count` metric ≥ quorum-1
6. **Start solvers**: Configure with solver bond and supported intent types
7. **Submit test intents**: Use sample workflows
8. **Monitor**: Import `monitoring/grafana_dashboard.json`
9. **Verify finality**: Check `poseq_batches_finalized_total` is increasing
10. **Run scenarios**: Execute structured test experiments

## Initial Testnet Experiments

| Experiment | Description | Success Criteria |
|-----------|-------------|-----------------|
| Basic swap flow | Submit SwapIntent → solver fills → receipt generated | Receipt shows Succeeded status |
| Solver competition | 3+ solvers compete on same intent | Best user_outcome_score wins |
| No-reveal penalty | Solver commits but withholds reveal | 1% bond slashed, violation score increased |
| Expired intent | Submit intent with short deadline | Intent pruned, not executed |
| DA failure | Withhold commitment artifacts | Block proposal rejected by validators |
| Fairness audit | Run 100 slots, check fairness logs | No UnjustifiedExclusion evidence |
| Censorship test | Exclude all bundles from one solver | RepeatedExclusion evidence generated |
| Resource conflict | Two bundles writing same resource | Conflict detected, serialized execution |
| Dispute flow | Submit fee violation fraud proof | Solver slashed, challenger rewarded |
| Stress test | 1000 intents/slot with 10 solvers | All intents processed, ordering deterministic |

## Rollback Plan

If critical issues are found:
1. Stop all validators (`SIGTERM`)
2. Diagnose via metrics and logs
3. Apply fix, rebuild binary
4. Clear data dirs if state is corrupted
5. Restart with fresh genesis if needed
6. If protocol change required: increment `ProtocolParameters.version`

## Known Limitations

- Single-machine devnet (no Kubernetes orchestration)
- Static committee (no dynamic rotation in consensus)
- No view-change timeout (permanent leader per epoch)
- Sled single-writer (no HA without external tooling)
- Prometheus metrics only (no distributed tracing)
