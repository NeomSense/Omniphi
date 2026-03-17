# PoSeq → Runtime Bridge: Batch Lifecycle Specification

## Overview

The PoSeq→Runtime bridge is the boundary between the sequencing layer (PoSeq) and the execution layer (runtime). PoSeq provides total ordering and finality guarantees; the runtime provides execution, settlement, and state transitions. Neither layer leaks internals into the other.

```
PoSeq (sequencing only)          Runtime (execution only)
─────────────────────────        ─────────────────────────
FinalizedBatch
    │
    ▼ BatchPipeline::deliver()
FinalizationEnvelope ──────────► RuntimeBatchIngester::ingest()
                                     │
                                     ├── structural validation
                                     ├── idempotency check
                                     ├── execution (PoSeqRuntime)
                                     └── IngestionOutcome
                                             │
                                     ◄───────┘ (ack or reject)
    │
    ▼ BatchPipeline::record_ack/rejection()
PipelineState (Accepted | RejectedRetryable | RejectedTerminal)
```

---

## Canonical Bridge Types

### `FinalizationEnvelope` (PoSeq → Runtime)

The only structure the runtime accepts from PoSeq.

| Field | Type | Description |
|-------|------|-------------|
| `batch_id` | `[u8;32]` | Unique ID of the finalized batch |
| `delivery_id` | `[u8;32]` | `SHA256(batch_id ‖ attempt_count)` — stable per attempt |
| `attempt_count` | `u32` | Delivery attempt number (1 on first delivery) |
| `slot` | `u64` | PoSeq slot number |
| `epoch` | `u64` | PoSeq epoch number |
| `sequence_number` | `u64` | Pipeline-assigned sequence (monotonic within pipeline) |
| `leader_id` | `[u8;32]` | ID of the leader that proposed this batch |
| `parent_batch_id` | `[u8;32]` | Parent batch for chain linkage |
| `ordered_submission_ids` | `Vec<[u8;32]>` | **Canonical execution order — runtime must preserve** |
| `batch_root` | `[u8;32]` | Merkle root of ordered submissions |
| `finalization_hash` | `[u8;32]` | Cryptographic anchor of the PoSeq finalization event |
| `quorum_approvals` | `usize` | Number of committee attestations |
| `committee_size` | `usize` | Total committee members |
| `fairness` | `FairnessMeta` | Informational fairness metadata (does not affect ordering) |
| `commitment` | `BatchCommitment` | Cryptographic commitment over ordering + provenance |

### `BatchCommitment`

```
submission_root    = SHA256(ordered_submission_ids[0] ‖ ... ‖ ordered_submission_ids[n])
commitment_hash    = SHA256(finalization_hash ‖ delivery_id ‖ submission_root)
```

The runtime **must** recompute and verify `commitment_hash` before accepting any batch. This prevents:
- Reordering attacks (swapping submission IDs changes `submission_root`)
- Replay attacks with different `delivery_id`
- Forgery (all fields are committed)

### `FairnessMeta`

Informational metadata the runtime may log but **must not** use to alter `ordered_submission_ids`:

| Field | Description |
|-------|-------------|
| `policy_version` | Version of the ordering policy |
| `forced_inclusion_count` | Submissions promoted by forced-inclusion rules |
| `rate_limited_count` | Submissions excluded by rate limits |
| `per_submission_class` | Per-submission fairness class tag (audit use only) |

### `RuntimeIngestionAck` (Runtime → PoSeq)

| Field | Description |
|-------|-------------|
| `batch_id` | Batch that was accepted |
| `delivery_id` | Delivery that was accepted |
| `epoch` | Epoch at time of execution |
| `succeeded_count` | Number of intents that executed successfully |
| `failed_count` | Number of intents that failed (but batch still accepted) |
| `execution_result_ref` | Opaque `state_root` hash from `SettlementResult` |
| `ack_hash` | `SHA256(batch_id ‖ delivery_id ‖ "ack" ‖ epoch)` |

**PoSeq stores `execution_result_ref` as an opaque reference without inspecting it.** This is the only way CRX/safety outputs flow back to PoSeq — as an opaque 32-byte hash.

### `RuntimeIngestionRejection` (Runtime → PoSeq)

| Field | Description |
|-------|-------------|
| `batch_id` | Batch that was rejected |
| `delivery_id` | Delivery that was rejected |
| `epoch` | Epoch at time of rejection |
| `cause` | Typed `RejectionCause` (see below) |
| `rejection_hash` | `SHA256(batch_id ‖ delivery_id ‖ "reject" ‖ cause_tag)` |

---

## Rejection Causes and Retry Policy

| Cause | Tag | Retryable | Description |
|-------|-----|-----------|-------------|
| `InvalidEnvelope(msg)` | `invalid_envelope` | No | Commitment mismatch, zero fields, malformed structure |
| `AlreadyApplied` | `already_applied` | No | Batch was previously applied (idempotent duplicate) |
| `TransientUnavailable(msg)` | `transient_unavailable` | **Yes** | Runtime temporarily overloaded or restarting |
| `ExecutionFailure(msg)` | `execution_failure` | No | Execution failed; partial state may exist |
| `SafetyBlock(msg)` | `safety_block` | No | Safety kernel blocked the batch |

**PoSeq retry logic:**
- `TransientUnavailable` → redeliver same envelope (same `delivery_id`)
- All other rejections → terminal; mark batch as `RejectedTerminal`

---

## Batch Pipeline State Machine

```
                   deliver()
                      │
                      ▼
              ┌───────────────┐
              │    Pending    │
              └───────────────┘
                    │   │
         record_ack()   record_rejection()
                    │   │
          ┌─────────┘   └──────────────────────────┐
          │                                        │
          ▼                              ┌─────────┴──────────┐
    ┌──────────┐            retryable?  NO │ RejectedTerminal │
    │ Accepted │                           └──────────────────┘
    └──────────┘              YES
                              ▼
                    ┌─────────────────────┐
                    │ RejectedRetryable   │◄── re-deliver same envelope
                    └─────────────────────┘
                              │
                     record_ack() after retry
                              ▼
                         Accepted
```

States:
- **Pending**: `deliver()` called, awaiting outcome from runtime
- **Accepted**: runtime accepted and applied; `execution_result_ref` stored
- **RejectedRetryable**: transient; pipeline will redeliver
- **RejectedTerminal**: permanent; no further action

---

## Idempotency Guarantees

### PoSeq side (`BatchPipeline`)

- `deliver(batch)` always returns the same `delivery_id = SHA256(batch_id ‖ 1)` for the first attempt
- Re-delivering increments `attempt_count` in the tracking record (for abuse detection) but does not change the envelope's commitment
- `record_ack(ack)` is guarded by `delivery_id`-keyed replay protection; returns `Err(AckReplay)` if called twice

### Runtime side (`RuntimeBatchIngester`)

- Tracks `delivery_id` set and `batch_id` set independently
- Re-delivery with same `delivery_id` → `AlreadyApplied` (no re-execution)
- Re-delivery with same `batch_id` but different `delivery_id` → `AlreadyApplied` (guards against retry via fresh pipeline)

---

## CRX/Safety Result Flow

The runtime executes batches through the full Phase 3 (CRX) and Phase 4 (Safety Kernel) pipeline internally. PoSeq only sees the outcome as:

1. **`execution_result_ref`** — the `state_root` from `SettlementResult` (32-byte hash)
2. **Success/failure counts** in the ack

PoSeq stores the `execution_result_ref` as an opaque reference. It does not inspect causal graphs, rights capsules, safety incidents, or constrained state updates. This preserves strict layer separation.

If the safety kernel blocks a batch (e.g., emergency mode, quarantined objects), the runtime returns `SafetyBlock` rejection — PoSeq marks it as terminal and does not retry.

---

## Integration Code Layout

```
poseq/src/bridge/
├── mod.rs          — bridge module exports
├── runtime.rs      — Phase 1 bridge (OrderedBatch → RuntimeBatchEnvelope)
├── hardened.rs     — HardenedRuntimeBridge (idempotent delivery + ack replay)
└── pipeline.rs     — BatchPipeline (canonical contract: FinalizationEnvelope + ack/reject)

runtime/src/poseq/
├── mod.rs          — poseq module exports
├── interface.rs    — PoSeqRuntime, SolverMarketRuntime (Phase 1+2)
└── ingestion.rs    — RuntimeBatchIngester (accepts FinalizationEnvelope → IngestionOutcome)

integration/src/
└── lib.rs          — 15 end-to-end integration tests
```

---

## End-to-End Test Coverage

| Test | Validates |
|------|-----------|
| `test_e2e_happy_path` | Full pipeline: deliver → ingest → ack → Accepted state |
| `test_e2e_duplicate_delivery_idempotent` | Same delivery_id → AlreadyApplied; attempt_count increments |
| `test_e2e_transient_rejection_then_retry` | Retry after TransientUnavailable succeeds |
| `test_e2e_terminal_rejection_not_retried` | InvalidEnvelope → RejectedTerminal; needs_retry=false |
| `test_e2e_tampered_ordering_rejected_by_runtime` | Swapped IDs → commitment mismatch → rejection |
| `test_e2e_restart_crash_recovery` | Same delivery_id survives pipeline restart; fresh runtime accepts |
| `test_e2e_ack_replay_protection` | Second ack with same delivery_id → AckReplay error |
| `test_e2e_fairness_metadata_propagation` | FairnessMeta flows end-to-end; ordering unchanged |
| `test_e2e_finality_state_after_ack` | Pipeline state transitions: None → Pending → Accepted |
| `test_e2e_multiple_batches_ordered` | 5 sequential batches all accepted; runtime applied_count=5 |
| `test_e2e_empty_batch_accepted` | Zero-submission batch is valid |
| `test_e2e_safety_block_rejection` | SafetyBlock → RejectedTerminal |
| `test_e2e_crx_execution_result_ref_opaque` | execution_result_ref stored opaquely in PoSeq |
| `test_e2e_batch_from_prior_epoch_rejected` | Re-delivery same batch_id → AlreadyApplied |
| `test_e2e_commitment_covers_ordering` | Reordered IDs → different commitment_hash |

Run all:
```bash
cd integration
cargo test
```

---

## Running PoSeq + Runtime Locally

### Prerequisites

```bash
# Rust 1.86+
rustup toolchain install 1.86.0
```

### Run unit + integration tests

```bash
# PoSeq (604+ tests including networking)
cd poseq
cargo test

# Runtime (130+ tests)
cd runtime
cargo test

# Integration (15 end-to-end pipeline tests)
cd integration
cargo test
```

### Run the PoSeq devnet

The `poseq-devnet` binary runs N in-process nodes over real loopback TCP:

```bash
cd poseq
cargo run --bin poseq-devnet -- --nodes 3 --quorum 2 --slots 5
```

### Multi-process mode

```bash
# Terminal 1
cargo run --bin poseq-node -- \
  --id 0101010101010101010101010101010101010101010101010101010101010101 \
  --addr 127.0.0.1:7001 --quorum 2

# Terminal 2
cargo run --bin poseq-node -- \
  --id 0202020202020202020202020202020202020202020202020202020202020202 \
  --addr 127.0.0.1:7002 --peer 127.0.0.1:7001 --quorum 2

# Terminal 3
cargo run --bin poseq-node -- \
  --id 0303030303030303030303030303030303030303030303030303030303030303 \
  --addr 127.0.0.1:7003 --peer 127.0.0.1:7001 --peer 127.0.0.1:7002 --quorum 2
```

---

## Operational Notes

### Layer boundary enforcement

- PoSeq **must not** pass runtime state into `ordered_submission_ids` — these are opaque IDs
- Runtime **must not** reorder `ordered_submission_ids` — PoSeq ordering is canonical
- The `commitment_hash` is the enforcement mechanism: runtime recomputes it and rejects if it doesn't match

### Delivery attempt limits

The hardened bridge tracks `attempt_count` per batch. Operators should configure a maximum (e.g., 3 attempts) before escalating to governance review. The pipeline itself has no hard limit — this is a policy decision.

### Schema versioning

PoSeq persists bridge state to sled (schema version 1). If the schema changes, the node will refuse to start with `StorageError::SchemaMismatch`. Run the migration tool (future work) or wipe and resync from a checkpoint.

### Crash recovery

After a crash, `ReconciliationEngine::reconcile()` scans all `finality:` and `bridge_rec:` entries and emits:
- `ReexportToRuntime` for batches finalized but not yet exported
- `RetryBridgeExport` for batches stuck in `Exporting` state
- `AuditFailed` for disputed or orphaned records

These actions must be replayed by the operator or an automated recovery process before the node resumes normal operation.
