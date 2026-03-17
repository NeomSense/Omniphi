# Validator & Solver Operations Guide

## Validator Operations

### Starting a Validator

```bash
poseq-node --config validator.toml --metrics-addr 0.0.0.0:9090
```

Configuration file (`validator.toml`):
```toml
node_id = "0x01..."  # 32-byte hex node ID
listen_addr = "0.0.0.0:26600"
peers = ["peer1:26600", "peer2:26600"]
quorum = 3
slot_duration_ms = 6000
key_seed = "0x..."  # Ed25519 key seed
```

### Monitoring Slot Progression

- Prometheus metrics at `/metrics` on the metrics port
- Key metrics:
  - `poseq_batches_finalized_total` — finalized batches
  - `poseq_current_epoch` / `poseq_current_slot` — progress
  - `poseq_peer_count` — connected peers
  - `poseq_fairness_violations_total` — fairness incidents

### Health Check

```bash
curl http://localhost:9090/healthz
```

### Interpreting Fairness Logs

The fairness log tracks every bundle decision per slot:
- `Included` — bundle in canonical sequence
- `OrderingPriority` — bundle lost to higher-scoring competitor (normal)
- `ExpiredIntent` / `InvalidReveal` / `FailedDA` / `SolverInactive` — bundle excluded for cause

Censorship evidence is generated when:
- All bundles excluded in a slot (TotalExclusion)
- One solver's bundles repeatedly excluded (RepeatedExclusion, threshold: 3+)
- Eligible bundle excluded without valid reason (UnjustifiedExclusion)

### Handling DA Failures

When DA checks fail:
1. Validator does NOT vote on the proposed block
2. After 3 consecutive DA failures → epoch transition forced
3. Check: are commitments available? Are intents in pool? Is payload complete?

### Recovering After Restart

1. Node loads checkpoint from Sled store
2. Reconciliation engine classifies finality + bridge state
3. Missing batches replayed from persisted records
4. Finality chain continuity verified via `previous_commitment`

### Validating Finality Continuity

Each `FinalityCommitment` includes `previous_commitment` — the commitment root of the prior sequence. Verify:
```
commitment_root = SHA256(sequence_id ‖ ordering_root ‖ bundle_root ‖ fairness_root ‖ previous_commitment)
```

## Solver Operations

### Registering a Solver

```rust
// On-chain via x/poseq module
MsgRegisterSolver {
    solver_id: "0x...",
    moniker: "MySolver",
    operator_address: "omni1...",
    bond_amount: 50_000,
    supported_intent_types: vec![0x01, 0x02, 0x03], // Swap, Payment, Route
}
```

### Solver Workflow Per Slot

1. **Subscribe** to intent pool via streaming RPC
2. **Evaluate** intents matching capabilities
3. **Commit phase** (5 blocks): submit `BundleCommitment` with sealed plan hash
4. **Reveal phase** (3 blocks): submit `BundleReveal` with actual plan
5. **Wait**: PoSeq ordering → HotStuff finality → settlement
6. **Check receipts** for execution results

### Bond Management

- Lock bond for commitments: `bond.lock(amount)`
- Unlock after reveal/window: `bond.unlock(amount)`
- Slash on violation: automatic (1% no-reveal, 25% invalid settlement, 100% double fill)
- Unbonding: 50,400 blocks (~7 days)

### Avoiding Penalties

| Violation | Penalty | How to Avoid |
|-----------|---------|--------------|
| Commit without reveal | 1% bond + 100 violation score | Always reveal if committed |
| Invalid reveal | 100% bond + permanent ban | Never tamper with reveals |
| Settlement failure | Escalating (0% → 25%) | Validate plans locally before commit |
| Double fill | 100% bond + permanent ban | Track which intents you've already targeted |

### Inspecting Receipts

Query receipt index by solver_id to check execution outcomes:
- `ExecutionStatus::Succeeded` — plan verified, rewards accrue
- `ExecutionStatus::Failed` — constraint violation, reputation penalty
- `ExecutionStatus::PartialFill` — partial execution if allowed

## Troubleshooting

### Solver Deactivated
- Check violation score: auto-deactivation at 9500+
- Resolution: wait for score decay or re-register with new identity

### DA Check Failures
- Verify your commitments are stored and accessible
- Ensure intents you reference still exist in the pool
- Check that your bond is locked (not zero)

### Commitment Hash Mismatch
- Your reveal must produce EXACTLY the same hash as your commitment
- Any change to execution steps, outputs, fees, or nonce after commit → mismatch
- Use `reveal.verify_against_commitment(&commitment)` locally before submitting
