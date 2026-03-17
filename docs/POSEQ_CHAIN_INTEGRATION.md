# PoSeq ↔ Cosmos Chain Integration

## Overview

PoSeq (Proof of Sequencing) and the Cosmos chain are deliberately loosely coupled:

| Layer | Responsibility |
|-------|---------------|
| **PoSeq** (`poseq/`) | Sequencing, finality, committee management, misbehavior detection |
| **Chain** (`chain/x/poseq/`) | Governance, accountability, operator visibility, slashing recommendations |

PoSeq does **not** know about Cosmos SDK types, staking, or bank modules. The chain does **not** know about PoSeq committees, slots, or proposals. The `chain_bridge` module is the boundary.

---

## Data Flow

```
PoSeq epoch ends
      │
      ▼
ChainBridgeExporter::export()          (Rust: poseq/src/chain_bridge/exporter.rs)
      │
      │  JSON-encoded ExportBatch
      ▼
PoSeq relayer (off-chain process)
      │
      │  MsgSubmitExportBatch tx
      ▼
x/poseq keeper::IngestExportBatch()    (Go: chain/x/poseq/keeper/keeper.go)
      │
      ├── StoreEvidencePackets   → governance slashing proposals
      ├── StoreEscalationRecords → governance proposals for Severe/Critical
      ├── StoreSuspensions       → block nodes from committee (auto or gov-gated)
      ├── StoreCheckpointAnchor  → write-once finality reference
      └── StoreEpochState        → operator visibility queries
```

---

## Record Types

### EvidencePacket
A cryptographically self-contained record of a single misbehavior incident.

```
packet_hash = SHA256(kind_tag | offender_node_id | epoch_be | sorted_evidence_hashes)
```

Stored by `packet_hash`. Idempotent — duplicates are silently skipped.

**Evidence kinds:**
- `Equivocation` — double-signing / conflicting proposals
- `UnauthorizedProposal` — proposal from a non-leader node
- `UnfairSequencing` — front-running, censorship, MEV reordering
- `ReplayAbuse` — replaying already-processed submissions
- `BridgeAbuse` — manipulating cross-chain delivery
- `PersistentAbsence` — sustained validator absence
- `StaleAuthority` — acting as committee member after rotation
- `InvalidProposalSpam` — flooding invalid proposals

### GovernanceEscalationRecord
Produced only for **Severe** and **Critical** misbehavior. Carries a recommended action that governance tooling uses to draft proposals.

```
escalation_id = SHA256("esc" | offender_node_id | epoch_be | evidence_packet_hash)
```

The chain does **not** auto-execute escalation actions. They become governance proposal content.

**Recommended actions:**
- `SuspendFromCommittee { epochs }` — suspend N epochs
- `PermanentBan` — permanently exclude from sequencing
- `CommitteeFreeze { committee_epoch }` — emergency freeze entire committee
- `GovernanceReview` — no automatic action, just governance review

### CommitteeSuspensionRecommendation
A lighter-weight suspension that the keeper can auto-apply (if `auto_apply_suspensions = true`) without a full governance vote. Controlled by `x/poseq` params.

### CheckpointAnchorRecord
A write-once on-chain anchor for a PoSeq checkpoint. Verifiable via:

```
anchor_hash = SHA256("ckpt" | checkpoint_id | epoch_be | epoch_state_hash | bridge_state_hash)
```

The Go keeper re-verifies this hash on ingestion and rejects tampered records.

### EpochStateReference
A human-readable epoch summary for operator dashboards: finalized batch count, misbehavior count, governance escalations.

---

## Go Module: `chain/x/poseq/`

```
chain/x/poseq/
├── types/
│   ├── keys.go         # Store key prefixes and constructors
│   ├── types.go        # All Go-side record types (mirror of Rust chain_bridge)
│   ├── params.go       # Module params + DefaultParams
│   ├── genesis.go      # GenesisState
│   ├── msgs.go         # MsgSubmitExportBatch, MsgSubmitEvidencePacket,
│   │                   # MsgSubmitCheckpointAnchor, MsgUpdateParams
│   └── errors.go       # Sentinel errors
├── keeper/
│   ├── keeper.go       # State management + IngestExportBatch orchestration
│   ├── msg_server.go   # Message handler implementations
│   ├── genesis.go      # InitGenesis / ExportGenesis
│   ├── verify.go       # Hash verification (mirrors Rust verify() methods)
│   └── keeper_test.go  # Unit tests (17 test cases)
└── module/
    └── module.go       # AppModule, DefaultGenesis, InitGenesis, ExportGenesis
```

---

## Rust Module: `poseq/src/chain_bridge/`

```
poseq/src/chain_bridge/
├── mod.rs              # Re-exports all chain bridge types
├── evidence.rs         # EvidencePacket, EvidencePacketSet, DuplicateEvidenceGuard,
│                       # PenaltyRecommendationRecord
├── escalation.rs       # GovernanceEscalationRecord, CommitteeSuspensionRecommendation,
│                       # EscalationAction, EscalationSeverity
├── anchor.rs           # CheckpointAnchorRecord, BatchFinalityReference, EpochStateReference
└── exporter.rs         # ChainBridgeExporter — epoch-end orchestrator
```

### ChainBridgeExporter Usage (Rust)

```rust
use poseq::chain_bridge::ChainBridgeExporter;

let mut exporter = ChainBridgeExporter::new();

// At epoch end, call export() with all incidents
let export_batch = exporter.export(
    epoch,
    incidents,           // Vec<MisbehaviorIncidentInput>
    Some(checkpoint),    // Option<CheckpointAnchorRecord>
    committee_hash,
    finalized_batch_count,
)?;

// Serialize and submit to chain
let json = serde_json::to_string(&export_batch)?;
// → relayer submits MsgSubmitExportBatch with this JSON
```

---

## Governance Integration

### How Escalations Become Proposals

1. PoSeq detects Critical/Severe misbehavior at epoch end
2. `ChainBridgeExporter` produces a `GovernanceEscalationRecord` with a recommended action and rationale text
3. The relayer submits the `ExportBatch` via `MsgSubmitExportBatch`
4. Governance tooling queries `GetEscalationRecord(escalation_id)` and uses the `rationale` field as proposal text
5. Governance votes; if passed, the chain's `x/staking` or `x/slashing` modules execute the penalty

### How Slashing Works

The chain does **not** slash automatically from PoSeq evidence. The process:

1. `EvidencePacket.proposed_slash_bps` provides PoSeq's slash recommendation (basis points of stake)
2. Governance reviews and creates a `x/gov` proposal referencing the `packet_hash`
3. If the proposal passes, governance calls the appropriate chain slashing handlers

This keeps PoSeq's role as **advisory only** — final economic decisions stay with governance.

---

## Params

| Param | Default | Description |
|-------|---------|-------------|
| `authorized_submitter` | `""` (any) | Bech32 address of the authorized relayer. Empty = any address can submit. |
| `auto_apply_suspensions` | `false` | Auto-apply suspension recommendations without governance vote. |
| `max_evidence_per_epoch` | `256` | Cap on evidence packets per ExportBatch to prevent state bloat. |
| `max_escalations_per_epoch` | `32` | Cap on escalation records per ExportBatch. |

Update via governance: `MsgUpdateParams` from the gov authority address.

---

## Store Keys

| Prefix | Key | Value |
|--------|-----|-------|
| `0x01` | `packet_hash[32]` | `EvidencePacket` (JSON) |
| `0x02` | `escalation_id[32]` | `GovernanceEscalationRecord` (JSON) |
| `0x03` | `epoch[8] ‖ slot[8]` | `CheckpointAnchorRecord` (JSON) |
| `0x04` | `epoch[8]` | `EpochStateReference` (JSON) |
| `0x05` | `node_id[N]` | `CommitteeSuspensionRecommendation` (JSON) |
| `0x06` | `epoch[8]` | `ExportBatch` (full, for operator queries) |
| `"params"` | — | `Params` (JSON) |

All keys are big-endian encoded. No overflow risk since epoch and slot are `uint64`.

---

## Security Properties

1. **Idempotency**: Evidence packets and escalation records are write-once by content hash. Submitting the same record twice is a no-op (not an error).

2. **Anchor integrity**: Checkpoint anchors are verified against their `anchor_hash` before storage. Tampered anchors are rejected with `ErrCheckpointAnchorTampered`.

3. **Authorization**: An optional `authorized_submitter` param restricts who can call `MsgSubmitExportBatch`. If unset, any address can submit (suitable for permissionless testnets).

4. **Separation of concerns**: The chain never calls into PoSeq. PoSeq never calls into the chain. The relayer is the only coupling point, and it only submits JSON blobs.

5. **Governance gating**: Slashing and permanent bans require governance votes. PoSeq evidence is advisory, not self-executing.

---

## Adding to app.go

```go
import (
    poseqmodule "pos/x/poseq/module"
    poseqkeeper "pos/x/poseq/keeper"
)

// In app construction:
poseqMod := poseqmodule.NewAppModule(
    appCodec,
    runtime.NewKVStoreService(keys[poseqtypes.StoreKey]),
    logger,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

// Register in module manager:
app.ModuleManager = module.NewManager(
    // ... existing modules ...
    poseqMod,
)

// Add to genesis order:
app.ModuleManager.SetOrderInitGenesis(
    // ... existing modules ...
    poseqtypes.ModuleName,
)

// Register msg server handler (until gRPC stubs are generated):
poseqMsgServer := poseqkeeper.NewMsgServer(poseqMod.Keeper())
// wire poseqMsgServer into your router as needed
```

---

## Testing

```bash
# Unit tests (keeper + hash verification)
cd chain && go test ./x/poseq/... -v -count=1

# Build check
cd chain && go build ./x/poseq/...
```

Test coverage:
- `TestGetSetParams` — params round-trip
- `TestStoreAndGetEvidencePacket` — happy path
- `TestStoreEvidencePacket_Duplicate` — idempotency guard
- `TestStoreEscalationRecord` + duplicate
- `TestStoreCheckpointAnchor` + duplicate + tampered hash rejection
- `TestStoreAndGetEpochState`
- `TestStoreAndGetSuspension` + `TestIsNodeSuspended` (epoch boundary conditions)
- `TestIngestExportBatch_Success` — full ExportBatch ingestion
- `TestIngestExportBatch_Idempotent` — second ingest is safe
- `TestIngestExportBatch_AuthCheck` — unauthorized sender rejected
- `TestIngestExportBatch_EvidenceCap` — cap enforcement
