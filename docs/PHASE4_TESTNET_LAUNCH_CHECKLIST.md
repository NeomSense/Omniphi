# Phase 4: Testnet Launch Checklist

## Code Readiness

- [x] Intent pool with admission, dedup, expiry, rate limiting
- [x] Commit-reveal solver auction with hash binding
- [x] Deterministic intent-aware ordering with deadline/tip sorting
- [x] FinalityCommitment chaining with commitment root
- [x] Validator pre-vote verification (9-point check)
- [x] Fairness logging and censorship evidence detection
- [x] DA validation with bond/solver/expiry checks
- [x] Settlement verification per intent type (Swap, Payment, Route)
- [x] Typed fraud proof system with evidence deserialization
- [x] Solver bonding, slashing, rewards with budget neutrality
- [x] Protocol invariants (11 checks)
- [x] Failure path semantics (intent, solver, sequencing)
- [x] Resource-aware bundle declarations for parallel scheduling
- [x] Config validation layer with devnet/mainnet presets

## Test Readiness

- [x] Unit tests: all modules covered
- [x] E2E pipeline tests: full intent→receipt flow
- [x] Determinism tests: 9 tests verifying cross-node consistency
- [x] Adversarial tests: 12 tests covering solver copying, replay, spam
- [x] Stateful fuzz tests: 6 tests with random state machine exploration
- [x] Distributed simulation: 9 tests with multi-node slot progression
- [x] Invariant checks: 11 runtime invariants
- [x] Total: 384+ tests, 0 failures

## Config Readiness

- [x] `IntentProtocolConfig` with all protocol parameters
- [x] `default_config()` for mainnet
- [x] `devnet_config()` for testing (relaxed parameters)
- [x] `validate()` rejects unsafe ranges
- [x] Parameter guide documented

## Observability Readiness

- [x] Prometheus metrics (9 counters/gauges/histograms)
- [x] HTTP metrics exporter at `/metrics`
- [x] Health check at `/healthz`
- [x] In-memory event store (19 event types)
- [x] Fairness log per slot with Merkle root
- [x] Censorship evidence detection and reporting

## Operator Readiness

- [x] Validator operations guide
- [x] Solver operations guide
- [x] Parameter guide
- [x] Troubleshooting procedures
- [x] Bond management documentation
- [x] Penalty avoidance guide

## Rollback/Recovery Readiness

- [x] Sled persistence backend (crash-safe)
- [x] Reconciliation engine for post-crash state classification
- [x] Recovery checkpoint with hash verification
- [x] Finality chain continuity via `previous_commitment`
- [x] Idempotent batch delivery (delivery_id dedup)
- [x] Failure path semantics for all scenarios

## Testnet Launch Steps

1. Generate validator keys (Ed25519)
2. Create `validator.toml` configs with peer addresses
3. Set `IntentProtocolConfig::devnet_config()` parameters
4. Start nodes: `poseq-node --config validator.toml`
5. Verify peer connections via `poseq_peer_count` metric
6. Start solvers with intent pool subscription
7. Submit test intents
8. Monitor slot progression via `poseq_current_slot`
9. Verify finality commitments chain via logs
10. Check fairness logs for censorship evidence (should be none)

## Known Limitations

- Single-machine devnet only (no remote node orchestration)
- No dynamic committee rotation in consensus
- No view-change timeout (leader tenure is permanent per epoch)
- Sled is single-writer (no HA without external orchestration)
- No OpenTelemetry distributed tracing (Prometheus metrics only)
