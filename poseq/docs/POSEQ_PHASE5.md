# PoSeq Phase 5: Production Finality, Epochs, Recovery, and Devnet

## Overview

Phase 5 adds production-grade finality state tracking, epoch/committee lifecycle management,
membership safety, misbehavior classification, penalty/slashing hooks, bridge delivery recovery,
node recovery/checkpointing, structured observability, devnet simulation, validation hardening,
and unified policy configuration.

---

## 1. Finality State Machine (`finality/`)

The finality module models the complete lifecycle of a batch from proposal to runtime delivery.

```
                    ┌──────────────────────────────────────────────────────┐
                    │                 FINALITY STATE MACHINE                │
                    └──────────────────────────────────────────────────────┘

  Proposed ──────────────────────────────────────── Superseded (TERMINAL)
     │                                                      ▲
     │                                                      │
     ▼                                                      │
  Attested ──────────────────────────────────────────────── │ (Superseded)
     │                                                      │
     ▼                                                      │
  QuorumReached ─────────────────────────────── Invalidated (TERMINAL)
     │                                                      ▲
     ▼                                                      │
  Finalized ──────────────────── DisputedPlaceholder ───────┘
     │                                  │         ▲
     ▼                                  ▼         │
  RuntimeDelivered ──────────── Recovered ─────────
     │        │                     │
     │        ▼                     ▼
     │    RuntimeRejected ── DisputedPlaceholder
     │                                │
     ▼                                ▼
  RuntimeAcknowledged (TERMINAL)   Recovered
```

### Guarantee Levels

| State              | Guarantee Level  |
|--------------------|-----------------|
| Proposed           | Tentative        |
| Attested           | WeakFinality     |
| QuorumReached      | StrongFinality   |
| Finalized          | StrongFinality   |
| RuntimeDelivered   | Delivered        |
| RuntimeAcknowledged| Acknowledged     |

### FinalityCheckpoint

`FinalityStore::checkpoint(epoch)` returns the highest-slot finalized batch for that epoch,
with a deterministic SHA256(epoch ‖ batch_id ‖ slot) hash for cross-component verification.

---

## 2. Epoch and Committee Lifecycle (`epochs/`)

### Epoch/Committee Rotation

```
  Epoch N                          Epoch N+1
  ┌─────────────────────────┐      ┌─────────────────────────┐
  │ ActiveCommittee          │      │ ActiveCommittee          │
  │  members: {A,B,C,D,E}   │─────▶│  members: {B,C,D,E,F}   │
  │  leader_for_slot: [...]  │      │  leader_for_slot: [...]  │
  └─────────────────────────┘      └─────────────────────────┘
           │                                    │
           ▼                                    ▼
  MembershipTransitionRecord:           SHA256(seed ‖ epoch_id ‖ node_id)
    joined: {F}                         → ranked candidates
    left:   {A}                         → top N selected
```

### Deterministic Leader Assignment

Per-slot leaders are assigned via: `SHA256(committee_hash ‖ slot) mod committee_size`

### Epoch Boundary Policy

Three policies govern cross-epoch batch handling:

| Policy    | Unfinalized | Undelivered | Pending Attestations |
|-----------|-------------|-------------|---------------------|
| strict()  | Drop        | Drop        | Drop                |
| lenient() | Carry       | Carry       | Carry               |
| custom    | Escalate    | Escalate    | Escalate            |

---

## 3. Misbehavior → Penalty → Governance Escalation Flow

```
  MisbehaviorEvidence
        │
        ▼
  MisbehaviorCase (with auto severity)
        │
        ├── Minor ────────────────── Warning (0 slash, 0 suspend)
        ├── Moderate ───────────────  3% slash, 2-epoch suspend
        ├── Severe ─────────────────  30% slash, 5-epoch suspend, gov escalation
        └── Critical ───────────────  100% slash, ban, gov escalation
                │
                ▼
        PenaltyRecommendation
                │
                ├── slash_bps > 0 ──── SlashableEvidencePlaceholder
                │                           │
                │                           ▼
                │                    GovernanceEscalationFlag
                │                    requires_governance_vote: true
                └── ban: true ──────────────┘

  Slashable types: Equivocation, SlotHijackingAttempt,
                   RuntimeBridgeAbuse, ReplayAttack
```

---

## 4. Bridge Recovery State Machine (`bridge_recovery/`)

```
                    BRIDGE DELIVERY STATE MACHINE

  Pending ──────▶ Exporting ──────▶ Exported
                      │                 │
                      │                 ├──── Acknowledged (TERMINAL)
                      │                 │
                      │                 └──── Rejected ──────▶ RetryPending{n}
                      │                                              │
                      │                             attempt < max ───┤
                      │                                              │
                      │                             attempt >= max ──▼
                      │                                           Failed (TERMINAL)
                      │
                      └──────── (RetryPending → Exporting via try_export)

  RecoveredAck (TERMINAL) — acknowledged after recovery path
```

### Retry Policy

Default: max 3 attempts, sequence-based backoff.
`BridgeRecoveryStore::reject_and_retry()` atomically marks rejected + sets RetryPending.
On max retries exceeded, state transitions to Failed.

---

## 5. Recovery and Checkpointing Flow

```
  NodeRecoveryManager                 CheckpointStore
  ┌──────────────────────┐           ┌──────────────────────────┐
  │ checkpoints:         │           │ checkpoints: BTreeMap     │
  │  BTreeMap<epoch,CP>  │           │ epoch_index: BTreeMap     │
  │                      │           │ policy: interval + retain │
  │ store_checkpoint()   │           │                           │
  │   ─ verifies hash    │           │ store() → SnapshotExport  │
  │   ─ rejects tampered │           │   ─ verifies checkpoint_id│
  │                      │           │   ─ bincode size estimate │
  │ simulate_recovery()  │           │ prune_old() → LRU evict  │
  └──────────────────────┘           └──────────────────────────┘

  RecoveryCheckpoint fields:
    SHA256(epoch ‖ last_slot ‖ last_batch_id ‖ finality_hash ‖
           committee_hash ‖ bridge_hash) = checkpoint_hash

  PoSeqCheckpoint fields:
    SHA256(version ‖ epoch ‖ slot ‖ created_seq ‖ node_id ‖
           finality_checkpoint.checkpoint_hash) = checkpoint_id
```

---

## 6. Observability Model (`observability/`)

`ObservabilityStore` is a ring-buffer of `EventRecord`s (capped at `max_events`).

### Event Kinds (19 total)

- Epoch lifecycle: `EpochStarted`, `EpochCompleted`, `CommitteeRotated`
- Batch lifecycle: `BatchProposed`, `BatchAttested`, `BatchFinalized`,
  `BatchDelivered`, `BatchAcknowledged`, `BatchRejected`
- Governance: `MisbehaviorDetected`, `PenaltyRecommended`
- Node lifecycle: `NodeJoined`, `NodeSuspended`, `NodeBanned`
- System: `CheckpointCreated`, `RecoveryStarted`, `RecoveryCompleted`,
  `BridgeRetry`, `BridgeFailed`

### Query API

```rust
store.events_for_epoch(epoch)          // filter by epoch
store.events_of_kind(&kind)            // filter by event type
store.recent(n)                        // last N events
store.all()                            // all retained events
```

---

## 7. Devnet Simulation Design (`devnet/`)

`NodeLifecycleController` manages `PoSeqDevnetNode` instances over a `MockPeerNetwork`.

### Supported Scenarios

| Scenario               | Tests                                              |
|------------------------|----------------------------------------------------|
| `HappyPath`            | All nodes finalize + ack across N epochs           |
| `LeaderCrash`          | First node crashes; remaining nodes continue       |
| `DoubleProposal`       | Equivocation detected, misbehavior reported        |
| `FairnessViolation`    | Violator misbehavior recorded                      |
| `CommitteeRotation`    | Epoch boundary triggers committee change           |
| `NodeRecoveryAfterCrash`| Node crashes epoch 1, recovers epoch 2            |
| `StaleMemberRejected`  | Out-of-epoch node rejected, misbehavior recorded   |
| `BridgeRetryAndAck`    | Delivery rejected, retried, acked successfully     |

`MockPeerNetwork` routes messages between nodes; `broadcast()` excludes sender.
`DevnetScenarioResult` records: batches finalized, bridge acks, misbehaviors,
nodes crashed/recovered, and an event log.

---

## 8. Validation Hardening (`validation_hardening/`)

Three validator types:

```
  FinalizationValidator
    validate_epoch_authority() ─── checks proposal_epoch == current_epoch
                                   AND proposer ∈ active_committee
    validate_slot_authority()  ─── checks proposer == expected slot leader

  TransitionSafetyValidator
    validate_finality_transition() ─── delegates to FinalityState::can_transition_to()

  BridgeStateValidator
    validate_export_state() ─── rejects Acknowledged/Failed/RecoveredAck states
                                 (already terminal, cannot re-export)
```

All validators return `ValidationReport { batch_id, passed, failures: Vec<ValidationFailureReason> }`.

---

## 9. Policy (`policy/`)

`FullPoSeqConfig` is the single configuration root for all Phase 5 subsystems:

```rust
FullPoSeqConfig {
    epoch:        EpochConfig,                // quorum bps, epoch length, carry-forward
    rotation:     CommitteeRotationConfig,    // seed, min/max committee size, slot count
    penalty:      PenaltyPolicyConfig,        // slash bps per offense type, auto-ban threshold
    recovery:     RecoveryPolicyConfig,       // max replay epochs, checkpoint verification
    checkpoint:   CheckpointPolicy,           // interval epochs, max retained
    bridge_retry: BridgeRetryPolicy,          // max attempts, backoff base seq
    devnet:       Option<DevnetConfig>,       // partition simulation, misbehavior injection
}
```

`FullPoSeqConfig::validate()` enforces:
- Non-zero epoch length
- Quorum BPS ≤ 10000
- Slash BPS ≤ 10000
- min_committee_size ≤ max_committee_size

---

## Future Extension Points

1. **Real clock integration**: Replace `timestamp_seq` (monotonic counter) with actual timestamps once a clock source is available.
2. **P2P state sync**: `StateSyncRequest`/`StateSyncResponse` scaffolding in `recovery/` is ready for peer-to-peer checkpoint exchange.
3. **On-chain slashing**: `SlashableEvidencePlaceholder` and `GovernanceEscalationFlag` are designed to be serialized and submitted as governance proposals to the Cosmos SDK chain in `chain/x/poc/`.
4. **Quorum aggregation**: `FinalityStore::guarantee_level()` can be wired to actual attestation counts from `attestations/` to dynamically compute `QuorumReached`.
5. **Cross-epoch finality**: The `EpochBoundaryHandler` and `BoundaryTransitionResult` are ready to feed into `FinalityStore` for carried-over batches in epoch N+1.
6. **Metrics export**: `FinalityMetricsSummary`, `CommitteeHealthSummary`, `BridgeHealthSummary` in `observability/` can be serialized as Prometheus metrics or JSON API responses.
7. **Deterministic fuzzing**: The devnet `NodeLifecycleController::run_scenario()` interface can be driven by property-based tests (e.g., `proptest`) for randomized epoch/crash/recovery sequences.
