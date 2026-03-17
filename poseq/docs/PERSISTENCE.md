# PoSeq Persistence & Crash Recovery

## Overview

PoSeq uses a layered persistence architecture:

```
PoSeqNode / ReconciliationEngine          (domain logic)
        │
  DurableStore                            (serialization + flush policy)
        │
  PersistenceEngine                       (domain-typed key helpers)
        │
  SledBackend : PersistenceBackend        (durable on-disk store — WAL)
        │
  sled::Db  (lock-free B-tree + write-ahead log)
```

For tests, `SledBackend` is replaced by `InMemoryBackend` without changing any layer above it.

---

## Backend: SledBackend

**File:** `src/persistence/sled_backend.rs`

[sled](https://github.com/spacejam/sled) is an embedded, crash-safe key-value store with:
- Write-ahead log (WAL) for durability
- Lock-free B-tree for ordered key iteration
- `flush()` for explicit fsync (call after critical writes)

### Open modes

| Method | Use |
|--------|-----|
| `SledBackend::open(path)` | Production — persists to `path` |
| `SledBackend::open_temp()` | Tests — unique temp dir, no guarantees |

### Crash safety contract

Data written and **flushed** survives crashes. Data written but not flushed may be lost. `DurableStore` calls `flush()` implicitly for checkpoints; callers that need hard durability on every write should call `backend.flush()` explicitly.

---

## Data Layout

All keys are byte slices. Prefixes separate domains; numeric fields use **big-endian encoding** so lexicographic key ordering matches numeric ordering.

### Schema version

| Key | Value |
|-----|-------|
| `meta:schema_ver` | `u32` big-endian (current: `1`) |

Checked on every open. Mismatch → `StorageError::SchemaMismatch` → node refuses to start until migration.

### Finality status

| Key | Value |
|-----|-------|
| `finality:<batch_id[32]>` | `bincode(BatchFinalityStatus)` |

Written on every finality state transition. Contains the full transition audit trail (`Vec<FinalityTransitionRecord>`).

### Bridge delivery records

| Key | Value |
|-----|-------|
| `bridge_rec:<batch_id[32]>` | `bincode(BridgeRecoveryRecord)` |

Written when a batch enters the bridge pipeline and on every state change (Pending → Exporting → Exported → Acknowledged/Rejected).

### Checkpoints

| Key | Value |
|-----|-------|
| `checkpoint:<checkpoint_id[32]>` | `bincode(PoSeqCheckpoint)` |
| `ckpt_epoch:<epoch u64 BE>` | `checkpoint_id[32]` (secondary index) |

The secondary index enables `latest_checkpoint()` — scan all `ckpt_epoch:` keys, take the last (highest epoch).

### Replay protection

| Key | Value |
|-----|-------|
| `replay:<submission_id[32]>` | `u64 BE` (insertion sequence number) |

Sequence numbers preserve FIFO insertion order. On restart, `scan_replay_entries()` returns entries sorted by seq so the in-memory `ReplayGuard` can be rebuilt in the same order.

### Sequencing receipts

| Key | Value |
|-----|-------|
| `receipt:<batch_id[32]>` | `bincode(SequencingReceipt)` |

Audit trail of accepted/rejected decisions per batch.

### Legacy/engine-level namespaces

Managed by `PersistenceEngine` (not `DurableStore`):

| Prefix | Contents |
|--------|----------|
| `proposals:<id>` | Raw proposal bytes |
| `attestations:<id>` | Raw attestation bytes |
| `finalized:<id>` | Raw finalized batch bytes |
| `slashing:<node_id>:<index BE>` | Slash records |
| `jail:<node_id>` | Jail records |
| `rotation:<epoch BE>` | Committee rotation snapshots |
| `fairness:<id>` | Fairness records |
| `epoch:<epoch BE>` | Epoch state |
| `node:<id>` | Node metadata |

---

## Schema Versioning

`CURRENT_SCHEMA_VERSION = 1` is stamped on first open. Future migrations follow this pattern:

```rust
match ds.read_schema_version() {
    Some(1) => migrate_v1_to_v2(&mut ds)?,
    Some(v) => return Err(StorageError::SchemaMismatch { stored: v, expected: 2 }),
    None => {} // brand new
}
ds.write_schema_version(); // stamp new version
```

No auto-migration happens silently. Every schema change requires an explicit migration function.

---

## Crash Recovery & Reconciliation

**File:** `src/persistence/reconciler.rs`

On restart, `ReconciliationEngine::reconcile(&store)` scans all persisted state and returns a `ReconciliationReport` listing exactly what actions to take. The node must execute these actions before entering normal operation.

### Decision table

| Finality state | Bridge state | Action |
|----------------|-------------|--------|
| `Proposed` / `Attested` / `QuorumReached` | any | `MarkSuperseded` |
| `Finalized` | none | `ReexportToRuntime` |
| `Finalized` / `RuntimeDelivered` / `Recovered` | `Pending` | `ReexportToRuntime` |
| `Finalized` / `RuntimeDelivered` | `Exporting` / `Exported` / `Rejected` (retries left) | `RetryBridgeExport` |
| `Finalized` / `RuntimeDelivered` | `Exporting` / `Exported` (max retries) | `AuditFailed` |
| `Finalized` / `RuntimeDelivered` | `Failed` | `AuditFailed` |
| `RuntimeAcknowledged` | `Acknowledged` / `RecoveredAck` | `NoAction` |
| `Superseded` / `Invalidated` | any | `NoAction` |
| `DisputedPlaceholder` | any | `AuditFailed` (needs governance) |
| Bridge record with no finality entry | — | `inconsistencies` logged + `AuditFailed` |
| Any replay entry | — | `RestoreReplayEntry` |

### Reconciliation actions

```rust
pub enum ReconciliationAction {
    ReexportToRuntime { batch_id },
    RetryBridgeExport { batch_id, attempt },
    AuditFailed { batch_id, reason },
    MarkSuperseded { batch_id },
    RestoreReplayEntry { submission_id, seq },
    NoAction { batch_id },
}
```

No action is taken silently. Every decision is logged in `ReconciliationReport.actions`.

---

## Crash Test Coverage

**File:** `src/persistence/crash_tests.rs`

| Test | Scenario |
|------|----------|
| `test_crash_after_proposal_restores_superseded` | Crash before quorum → superseded |
| `test_crash_after_attestation_restores_superseded` | Crash pre-finalization → superseded |
| `test_crash_after_finalization_schedules_reexport` | Finalized, no bridge → re-export |
| `test_crash_during_export_schedules_retry` | Bridge mid-export → retry |
| `test_crash_before_ack_write_schedules_retry` | Bridge exported, no ack → retry |
| `test_crash_after_ack_no_action_needed` | Fully terminal → no action |
| `test_checkpoint_survives_crash` | Checkpoint data persists across reopen |
| `test_replay_protection_survives_crash` | Replay entries persist + FIFO order preserved |
| `test_duplicate_replay_after_restart` | Re-opened replay guard correctly rejects duplicates |
| `test_max_retries_at_crash_produces_audit_failed` | Exhausted retries at crash → audit, not retry |
| `test_schema_version_mismatch_detected` | Wrong schema on reopen → error |
| `test_full_pipeline_recovery` | Multi-batch mixed states all reconciled correctly |

All crash tests use real `SledBackend` with actual temp directories — they exercise the full disk I/O path.

---

## Invariants

1. **No silent auto-heal.** Every recovery decision appears in `ReconciliationReport.actions`.
2. **Replay protection is never weakened.** Replay entries restored in FIFO order; the guard rejects the same submissions post-restart as it would have pre-crash.
3. **State transitions are explicit.** `FinalityState` and `BridgeDeliveryState` are state machines; illegal transitions are rejected at the type level.
4. **Ack hash conflicts are fatal.** A duplicate ack with a different hash returns `AckHashConflict` and is never silently accepted.
5. **Determinism is preserved.** Checkpoint IDs are SHA256 hashes of all fields; a restored checkpoint must pass `verify_id()` or it is rejected.
6. **Schema mismatch blocks startup.** A node with a mismatched schema version refuses to enter normal operation.
