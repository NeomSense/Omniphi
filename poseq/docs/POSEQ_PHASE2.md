# PoSeq Phase 2 Architecture

## Overview

Phase 2 extends the Omniphi PoSeq sequencing layer from a single-node pipeline (Phase 1) into a full
distributed consensus system. Where Phase 1 handled submission intake, ordering, and batching for a
single sequencer, Phase 2 adds the committee model, deterministic leader election, multi-node
attestation, canonical finalization, conflict detection, and a hardened delivery bridge to the
Cosmos SDK runtime.

All logic is deterministic: `BTreeMap`/`BTreeSet` everywhere (no `HashMap`/`HashSet`), SHA256 for
all hashing, and zero wall-clock time in any ordering or finalization path.

---

## Component Roles

### `identities` — Node Identity Model

`NodeIdentity` models a single participant in the PoSeq committee. It carries a `node_id`, a
placeholder `public_key`, a `NodeRole` (Sequencer / Validator / ObserverOnly), and a `NodeStatus`
lifecycle (Pending → Active → Suspended / Ejected). `is_eligible_proposer` and
`is_eligible_attestor` are derived from role and status — suspension or ejection immediately revokes
both flags without requiring committee-level changes.

### `committee` — Committee Membership

`PoSeqCommittee` is the authoritative membership snapshot for an epoch. It stores sequencers and
validators in separate `BTreeMap`s keyed by `node_id`. `eligible_proposers()` returns only active
sequencers; `eligible_attestors()` merges active sequencers and active validators, sorted by
`node_id` for full determinism. `quorum_size()` returns the count of eligible attestors.
`CommitteeEpoch` records the height span of each epoch, and `CommitteeMembershipRecord` is an
immutable audit snapshot.

### `leader_selection` — Deterministic Leader Election

`LeaderSelector::select(slot, epoch, committee, policy)` picks the proposer for a slot without any
external randomness beyond an optional seed. `RoundRobin` cycles through sorted eligible proposers
modulo slot. `SlotHash` computes `SHA256(slot_le || epoch_le || seed) mod n` — purely a function of
on-chain-available data. `WeightedRoundRobin` respects per-node weight metadata while remaining
fully deterministic. All policies return `None` when the committee has no eligible proposers.

### `proposals` — Proposed Batch Lifecycle

`ProposedBatch` is the pre-consensus batch unit. It carries an ordered list of submission IDs, a
`batch_root` (SHA256 of sorted IDs), and a `proposal_id` (SHA256 of all header fields). State
transitions follow a strict machine: `Pending → Collecting → Attested → Finalized`, with
`Expired` and `Conflicted` as terminal abort states. `ProposalHeader` is the compact wire
representation for broadcast and ID derivation.

### `attestations` — Attestation Collection

`AttestationCollector` accumulates `BatchAttestationVote`s keyed by attestor_id. It detects exact
duplicates (idempotent, no error) and conflicting votes (same attestor voting both approve and
reject — returned as `ConflictingAttestationRecord`). `check_quorum` computes approval fraction
in integer basis points to avoid floating point, comparing against `AttestationThreshold` which
requires both a minimum count and a minimum fraction of committee_size.

### `finalization` — Canonical Batch Finalization

`FinalizationEngine` is a one-shot state machine per proposal. On `finalize()`, it checks: (1) not
already finalized; (2) no attestation conflicts; (3) quorum reached. If all pass, it produces a
`FinalizedBatch` whose `finalization_hash` covers all deterministic fields including the
`quorum_hash`. The `batch_id` is the `finalization_hash` itself, making it globally unique per
finalization event. `FinalizationDecision` covers all outcomes including `AlreadyFinalized`.

### `conflicts` — Equivocation and Conflict Detection

`ConflictDetector` maintains a `BTreeMap<(slot, epoch), (leader_id, proposal_id)>` of the first
seen proposal per slot. A new proposal for the same (slot, epoch) with a different ID triggers
`DualProposal`. A proposal from a node not in the committee triggers `InvalidProposal`.
Re-submission of an identical proposal triggers `ReplayedProposal`. All incidents are stored in
`EquivocationIncident` with a deterministic `incident_id` and accumulated in
`SequencingMisbehaviorRecord` per node.

### `persistence` — Batch Lifecycle Store

`BatchLifecycleStore` is a trait that abstracts all storage: proposed batches, finalized batches,
attestation collectors, export/ack state, and the misbehavior ledger. `InMemoryBatchLifecycleStore`
implements it with five `BTreeMap`s. The separation of trait from implementation allows future
IAVL-backed or RocksDB-backed stores to slot in without changing any logic layer.

### `bridge/hardened` — Hardened Runtime Bridge

`HardenedRuntimeBridge` wraps `FinalizedBatch` delivery to the Cosmos SDK runtime. `deliver()` is
idempotent — calling it twice for the same batch returns the same `RuntimeDeliveryEnvelope` without
re-registering. `record_ack()` enforces replay protection via a `BTreeMap` of seen `delivery_id`s:
a duplicate ack returns `BridgeError::AckReplay`. `record_rejection()` marks the record as
not-accepted for audit purposes. All delivery records are keyed by `batch_id`.

### `replay` — Generic Replay Protection

`ReplayGuard<K: Ord>` is a generic FIFO-eviction set. `check_and_record(key)` returns `true` on
first occurrence and `false` on replay. When `capacity > 0`, the oldest entry (tracked via a
sequence-numbered `BTreeMap`) is evicted once the set reaches capacity, preventing unbounded memory
growth. `ProposalReplayGuard` and `AckReplayGuard` are concrete type aliases over `[u8; 32]`.

### `commitment` — Batch Commitment Model

`BatchHashBuilder` is an incremental SHA256 builder that accepts submission IDs and header fields,
sorts both before hashing for determinism, and produces a `CanonicalBatchDigest` (submissions_root,
header_hash, combined digest). `BatchCommitment::compute(proposal, quorum)` derives the full
commitment record including the attestation summary hash (= quorum_hash), suitable for on-chain
anchoring and cross-chain verification.

### `receipts/lifecycle` — Lifecycle Audit Receipts

`FinalizationReceipt` captures the finalization event (batch_id, quorum summary, commitment, export
status). `DeliveryReceipt` records the bridge delivery outcome. `BatchLifecycleAuditRecord`
aggregates both receipts plus any incident IDs, providing a single audit object for tooling and
governance queries.

### `node` — Phase 2 Orchestrator

`Phase2PoSeqNode` wires all Phase 2 components into a single struct. `propose_batch()` verifies
leader authority, runs conflict detection, and initializes attestation collection.
`submit_attestation()` validates eligibility and routes to the collector. `try_finalize()` drives
the `FinalizationEngine`. `export_to_runtime()` invokes the hardened bridge. `record_runtime_ack()`
closes the loop. `get_lifecycle_audit()` assembles the full `BatchLifecycleAuditRecord`.

### `networking` — Network Abstractions

Trait definitions only — no implementations. `ProposalBroadcaster`, `AttestationChannel`, and
`BatchSyncInterface` define the P2P surface without coupling the consensus logic to any specific
transport. Implementations (libp2p, gRPC, in-process) are external to this crate.

### `policy/network` — Phase 2 Policy

`PoSeqNetworkPolicy` aggregates `AttestationThreshold`, `LeaderSelectionPolicy`,
`FinalizationPolicy`, `BridgePolicy`, and `EpochConfig` into a single governance-controlled struct.
`FinalizationPolicy::to_threshold(committee_size)` converts basis-point quorum ratios to concrete
`AttestationThreshold` values. `EpochConfig` controls slot cadence and committee rotation.

---

## Proposal Lifecycle State Machine

```
              propose_batch()
                    │
              ┌─────▼──────┐
              │   Pending  │
              └─────┬──────┘
         start_collecting()
                    │
              ┌─────▼──────────┐
              │   Collecting   │◄──── submit_attestation() (N votes)
              └─────┬──────────┘
          mark_attested() (quorum signal)
                    │
              ┌─────▼──────┐
              │  Attested  │
              └─────┬──────┘
           mark_finalized()
                    │
              ┌─────▼──────┐
              │ Finalized  │  (terminal — success)
              └────────────┘

 From Pending or Collecting:
   mark_expired()   → Expired    (terminal — timeout)
   mark_conflicted()→ Conflicted (terminal — misbehavior)
```

---

## Attestation and Finalization Flow

1. Leader proposes batch: `Phase2PoSeqNode::propose_batch()` verifies the node is the selected
   leader for the slot, checks conflict detector, builds `ProposedBatch`, initializes an
   `AttestationCollector`, stores both in `InMemoryBatchLifecycleStore`.
2. Peers attest: each calls `submit_attestation(BatchAttestationVote)`. The node validates committee
   membership, then delegates to `AttestationCollector::add_vote()`.
3. Finalization attempt: `try_finalize(proposal_id, height)` retrieves the proposal and collector,
   calls `FinalizationEngine::finalize()`. The engine checks conflicts, runs `check_quorum()`, and
   if both pass, produces an immutable `FinalizedBatch`.
4. Export: `export_to_runtime(batch_id)` wraps the `FinalizedBatch` in a `RuntimeDeliveryEnvelope`
   via `HardenedRuntimeBridge::deliver()` (idempotent), marks the batch as exported in the store.
5. Ack: the Cosmos SDK runtime signals acceptance via `RuntimeExecutionAck`. `record_runtime_ack()`
   checks for replay, updates the bridge record, and marks the batch as acked in the store.

---

## Conflict Detection

The `ConflictDetector` runs as part of `propose_batch()` before the batch is stored:

- **DualProposal**: A second different proposal arrives for the same (slot, epoch). The conflict is
  logged and the second proposal is rejected.
- **InvalidProposer**: The leader_id is not present in the committee. Logged and rejected.
- **ReplayedProposal**: The same proposal_id is submitted a second time. Logged; the first proposal
  remains the canonical one.
- **AttestationEquivocation**: Detected by `AttestationCollector::add_vote()` when the same
  attestor votes both approve and reject. Stored as `ConflictingAttestationRecord`; the collector's
  `has_conflicts()` flag blocks finalization.

All incidents are collected into `SequencingMisbehaviorRecord` per node and stored in the
`BatchLifecycleStore` for governance queries.

---

## Runtime Bridge Lifecycle

```
PoSeq                          HardenedRuntimeBridge              Runtime
  │                                     │                            │
  │── deliver(FinalizedBatch) ─────────►│                            │
  │◄─ RuntimeDeliveryEnvelope ──────────│                            │
  │                                     │─── (send envelope) ───────►│
  │                                     │◄── RuntimeExecutionAck ────│
  │── record_ack(ack) ─────────────────►│                            │
  │   (replay check → Ok or AckReplay) │                            │
```

`deliver()` is idempotent: calling it twice returns the same envelope (same delivery_id and
delivery_hash) without creating a duplicate bridge record. This allows safe retry without
double-counting. `record_ack()` is not idempotent — a second call for the same delivery_id returns
`BridgeError::AckReplay`, protecting against malicious or accidental ack duplication.

---

## Replay Protection Model

Two independent replay guards protect against different attack vectors:

- **ProposalReplayGuard**: Guards `proposal_id`s inside `propose_batch()`. A leader cannot
  accidentally or maliciously re-submit the same proposal into the pipeline.
- **AckReplayGuard** (via `HardenedRuntimeBridge`): Guards `delivery_id`s in `record_ack()`.
  Runtime acks cannot be replayed to trigger double-accounting of accepted batches.

Both guards use `ReplayGuard<[u8; 32]>` — a `BTreeSet`-backed structure with optional FIFO
eviction. Capacity=0 means unbounded retention (suitable for consensus nodes with bounded proposal
throughput per epoch). A finite capacity enables sliding-window protection for high-throughput
nodes at the cost of not detecting very old replays.

---

## Recovery Model

The current recovery model is documented here for implementation guidance; the Phase 2 in-memory
store does not yet persist to disk.

- **Crash recovery**: The `BatchLifecycleStore` trait is designed to be backed by an IAVL store
  (same as Cosmos SDK module state) or a WAL-backed KV store. Proposed batches, finalized batches,
  attestation collectors, and export/ack flags should all be persisted atomically per block.
- **Replay on restart**: On restart, the node re-loads all `ProposedBatch`es in `Collecting` state
  from the store and reinitializes their `AttestationCollector`s. Votes received before the restart
  that were stored can be replayed into collectors.
- **Conflict detector state**: `ConflictDetector::seen_proposals` must be persisted per epoch. On
  epoch boundary, only the finalized batch for each slot needs to be retained; the rest can be
  pruned.
- **State recovery errors**: `StateRecoveryError` (CorruptState, MissingData, VersionMismatch)
  covers the three main failure modes. A version mismatch triggers a halt; corrupt or missing data
  triggers a peer-sync of the missing epoch's finalized batches via `BatchSyncInterface`.

---

## Future Extension Points

1. **Cryptographic signatures**: `NodeIdentity.public_key` and `BatchAttestationVote.vote_hash` are
   placeholders. Plug in ed25519 or BLS signatures by replacing the vote hash computation and adding
   a `verify_vote_signature()` call in `AttestationCollector::add_vote()`.

2. **BLS aggregate attestations**: The `AttestationCollector` accumulates individual votes. BLS
   aggregation can replace the per-vote storage with a running aggregate signature, reducing
   on-chain footprint to O(1) per proposal.

3. **IAVL-backed persistence**: Implement `BatchLifecycleStore` on top of Cosmos SDK's
   `KVStoreService` with the same key encoding patterns used in `x/poc` and `x/por`.

4. **Slash-proof generation**: `EquivocationIncident` evidence maps already contain all fields
   needed to generate a slashing proof. Wire `ConflictDetector` output to the `x/por`
   misbehavior handler.

5. **Epoch rotation**: `CommitteeEpoch` and `EpochConfig.committee_rotation_enabled` are the hooks
   for adding stake-weighted committee election at epoch boundaries, driven by `x/poc` validator
   scores.

6. **Network implementation**: Implement `ProposalBroadcaster`, `AttestationChannel`, and
   `BatchSyncInterface` traits on top of Cosmos SDK's P2P reactor pattern or libp2p.

7. **Phase 1 integration**: `Phase2PoSeqNode` currently takes pre-ordered `Vec<[u8;32]>` IDs.
   Wire it to `Phase1PoSeqNode::produce_batch()` output so the full pipeline runs:
   intake → validation → ordering → Phase2 proposal → attestation → finalization → bridge.
