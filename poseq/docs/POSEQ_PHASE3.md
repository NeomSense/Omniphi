# PoSeq Phase 3 — Fair Sequencing + Anti-MEV

## Architecture overview: what changed from Phase 2

Phase 1 built the core submission intake, validation, ordering engine, batching, bridge, and state ledger. Phase 2 added the consensus layer: committee membership, leader selection, batch proposals, attestation collection, finalization, equivocation detection, and hardened bridge delivery with replay protection.

Phase 3 adds an orthogonal layer sitting *above* the raw ordering engine and *inside* the proposal/finalization pipeline. It answers the question: *given a canonical queue view, is the leader's proposed ordering fair?*

Phase 3 does not replace Phase 1/2 components. It adds twelve new modules that validators can run alongside their existing attestation logic to enforce fairness constraints before approving a proposed batch.

---

## Module map

```
src/fairness/           — FairnessClass enum, per-class rules, policy struct
src/inclusion/          — eligible-set computation, age tracking, forced inclusion
src/anti_mev/           — reorder bounds, leader discretion, ordering validation
src/queue_snapshot/     — deterministic queue snapshot + commitment model
src/fairness_audit/     — per-batch audit records and fairness receipts
src/fairness_incidents/ — ongoing unfair-sequencing incident detection
src/fairness_validation/— full proposal validation: combines all of the above
src/protected_flows/    — deadline tracking for protected-class submissions
src/fairness_bridge/    — fairness metadata envelope for downstream consumers
src/fairness_persistence/— ledger: audit records, incidents, leader history
src/fairness_config/    — unified configuration and policy factory
```

---

## Fair sequencing policy model

`FairSequencingPolicy` is the central configuration object. It contains:

- `class_rules: BTreeMap<FairnessClass, SubmissionFairnessClass>` — per-class priority weight, protected flag, and max_wait_slots.
- `reorder_bound: ReorderBound` — global default on how far a submission can be moved within a batch.
- `anti_starvation_slots: u64` — after this many slots without inclusion, a submission is force-included.
- `max_leader_discretion_bps: u32` — fraction of batch positions the leader can freely reorder (0–10000 bps).
- `protected_flow_enabled: bool` — enable the protected-class fast-path.

`FairSequencingPolicy::default_policy()` produces a sane starting point. It can be derived from `FairnessConfig::to_fair_sequencing_policy()` for governance-controlled configuration.

---

## Fairness classification system

Every submission is tagged with a `FairnessClass` before it enters the ordering pipeline.

```
FairnessClass (Ord by variant name — alphabetical, deterministic):
  BridgeAdjacent       — submissions touching cross-chain bridge state
  DomainSensitive      — domain-aware routing (e.g. specific app namespace)
  GovernanceSensitive  — governance transactions
  LatencySensitive     — time-critical but not safety-critical
  Normal               — default class; no special handling
  ProtectedUserFlow    — user-facing transactions that must not be sandwiched
  RestrictedPriority   — explicitly de-prioritized submissions
  SafetyCritical       — must not be reordered at all (bound = 0)
  SolverRelated        — solver/intent-resolution transactions
```

`FairnessClassifier` applies `ClassAssignmentRule` objects (sorted by `priority` descending) against submission metadata key/value pairs. The first matching rule assigns the class; unmatched submissions fall back to `Normal`.

`classify_all()` processes a full map of `(submission_id, metadata)` → `FairnessClass` in one pass, deterministically using `BTreeMap` iteration order.

`build_buckets()` groups the result into `FairnessBucket` objects (one per class), each holding a `BTreeSet<[u8;32]>` of submission IDs.

---

## Inclusion policy and anti-starvation

`InclusionPolicyEngine` answers: *which submissions belong in the next batch?*

### Eligible set computation

`compute_eligible_set()` takes a `BTreeSet<[u8;32]>` of candidate IDs, their `SubmissionFairnessProfile` records, the current slot, and the batch capacity. It returns:

1. `Vec<InclusionEligibilityRecord>` — one record per candidate with an `InclusionDecision` (Include / Exclude / ForceInclude).
2. `Vec<ForcedInclusionCandidate>` — submissions flagged for mandatory inclusion.

Force-included submissions bypass the capacity limit (they are counted separately). This prevents starvation from compounding: a submission that has been waiting too long cannot be lawfully excluded.

### Anti-starvation

`check_anti_starvation()` scans all submission profiles. Any submission older than `policy.anti_starvation_slots` is returned as a `ForcedInclusionCandidate`. Per-class `max_wait_slots` provides a tighter per-class deadline.

`SubmissionAgeTracker` maintains `(received_slot, received_sequence)` for each submission. `get_stale_beyond(current_slot, max_wait)` returns IDs whose age exceeds the threshold — useful for cleanup sweeps.

---

## Anti-MEV controls and their limits

### What is implemented

`AntiMevPolicy` constrains how far submissions can be moved from their canonical snapshot position:

- **`ReorderBound`**: per-class (or global) maximum forward (earlier) and backward (later) position movement.
  - `SafetyCritical`: 0 positions in either direction — zero tolerance.
  - `ProtectedUserFlow`/`BridgeAdjacent`: 1–2 positions.
  - Global default: 5 positions in either direction.

- **`ProtectedFlowPolicy`**: protected-class submissions must appear within the first `max_delay_slots` positions of the batch.

- **`LeaderDiscretionLimit`**: the fraction of batch positions (in bps) the leader can freely reorder. Computed at batch level. Default: 10% (1000 bps).

- **`OrderingCommitmentRule`**: a leader must commit to a `SnapshotCommitment` before ordering. The commitment includes `SHA256(snapshot_id || snapshot_root || slot || epoch || leader_id)`.

### Honest assessment of limits

These controls are *protocol-level constraints*, not cryptographic proofs. They do not prevent:

- **Off-chain coordination**: a leader can collude with users or solvers outside the protocol to arrange favorable ordering within the allowed reorder window.
- **Ordering within the bound**: within 5 positions, the leader can still pick winners and losers. The bound reduces MEV surface but does not eliminate it.
- **Censorship**: a leader who controls which submissions enter the snapshot (upstream) can exclude submissions before they are ever candidates.
- **Sophisticated strategies**: multi-slot manipulation is not detected by single-batch audits.

Phase 3 provides *fairness enforcement*, not *MEV elimination*. Meaningful MEV resistance requires additional layers described under Future Extension Points.

### `AntiMevEngine`

`apply_ordering_constraints()` takes a `snapshot_order` (canonical) and a `proposed_order` (leader's claim), computes signed position deltas for each submission, and validates each delta against the policy bound for the submission's class. Returns `AntiMevValidationResult` with a `valid` flag, per-violation records, and aggregate forward/backward max deltas.

`compute_position_deltas()` is the core computation: `delta = proposed_position - snapshot_position`. Negative = promoted; positive = demoted. Results are sorted by snapshot position for determinism.

---

## Queue snapshot commitment

### `QueueSnapshot`

Built from `Vec<EligibleSubmissionEntry>` at a given `(slot, epoch, height, policy_version)`. Entries are sorted canonically:
1. Protected classes first (SafetyCritical, ProtectedUserFlow, BridgeAdjacent, GovernanceSensitive).
2. Within the same protection tier: by `FairnessClass` (Ord, alphabetical) then `received_at_sequence` ascending.
3. Final tie-break: `submission_id` lexicographic ascending.

`snapshot_root = SHA256(submission_ids in canonical order)` — order-dependent commitment.

`snapshot_id = SHA256(slot || epoch || snapshot_root || policy_version)` — binds the snapshot to its context.

### `SnapshotCommitment`

`commitment_hash = SHA256(snapshot_id || snapshot_root || slot || epoch || leader_id)` — the leader binds themselves to a specific snapshot before ordering. If the leader's proposal uses a different snapshot, the commitment check fails.

### `SnapshotOrderingView`

Combines `QueueSnapshot` + `SnapshotCommitment` with convenience counts (`eligible_count`, `forced_inclusion_count`).

---

## Fairness audit records

### `FairnessAuditRecord`

One record per batch, built by `FairnessAuditRecord::build(...)`. Contains:

- All `InclusionAuditEntry` records (included and excluded submissions with decision, position, and delta).
- `OrderingJustificationRecord` for each ordered submission (snapshot position, final position, delta, compliance flag).
- `class_distribution: BTreeMap<FairnessClass, usize>` — how many slots each class received.
- The full `AntiMevValidationResult`.
- `fairness_warnings: Vec<String>`.
- `audit_hash = SHA256(batch_id || slot || epoch || leader_id || commitment_hash || policy_version || counts || anti_mev_summary || warnings)`.

The `audit_hash` is computed deterministically at build time. Same inputs always produce the same hash.

### `FairnessReceipt`

A compact summary: `(batch_id, audit_hash, policy_version, compliant, warning_count, violation_count)`. Designed for lightweight downstream consumption without carrying the full audit record.

---

## Unfair sequencing detection

`FairnessIncidentDetector` accumulates evidence across batches.

### Incident types

| Type | Trigger |
|---|---|
| `RepeatedOmission(n)` | Same submission skipped ≥ 3 times |
| `ExcessiveReorder` | Delta exceeds reorder bound |
| `ProtectedFlowViolation` | Protected submission not at required position |
| `LeaderDiscretionAbuse` | Batch-level discretion bps exceeded |
| `StarvationDetected` | Age > anti_starvation_slots and still excluded |
| `ClassBucketSkew` | One class consistently under-represented |
| `SnapshotOrderViolation` | Proposal does not match snapshot commitment |
| `UnauthorizedOmission` | Valid eligible submission excluded as PolicyExcluded |

### Incident ID

`SHA256(batch_id || submission_id || violation_type_tag || slot)` — deterministic, unique per (batch, submission, type, slot).

### Severity levels

`Info < Warning < Violation < Critical` (derived Ord).

---

## Protected flows

`ProtectedFlowHandler` tracks submissions from protected classes against slot-based deadlines.

`register(id, class, current_slot)`: creates a `ProtectedFlowRecord` with `must_include_by_slot = current_slot + policy.max_delay_slots`. Only fires if the class is in `policy.protected_classes`.

`get_overdue(current_slot)`: returns records where `included_at_slot.is_none() && current_slot > must_include_by_slot`, sorted by deadline ascending.

`mark_included(id, slot, batch_id)`: clears the overdue status.

---

## Leader fairness obligations

The leader (proposer) is bound to:

1. Commit to a `QueueSnapshot` by publishing a `SnapshotCommitment` before ordering.
2. Propose an ordering that is a subset of the snapshot's `ordered_ids()`.
3. Keep all position deltas within the per-class `ReorderBound`.
4. Include all protected-class submissions within their maximum allowed position.
5. Not exceed the `max_leader_discretion_bps` fraction of positions moved.
6. Not leave any forced-inclusion candidate out of the batch.

Violations are detected by `FairProposalValidator::validate()` and returned as `Vec<ProposalFairnessError>`. Validators are expected to reject attesting to proposals with `!result.valid`.

---

## Runtime bridge fairness metadata

`BatchFairnessMetadata` carries fairness state alongside the batch to runtime consumers:

- `snapshot_commitment` — the leader's pre-order commitment.
- `fairness_receipt` — compact compliance summary.
- `protected_flow_count`, `forced_inclusion_count`, `incident_count`.
- `metadata_hash = SHA256(batch_id || policy_version || snapshot_root || receipt_audit_hash)`.

`RuntimeFairnessEnvelope` wraps a batch with its `BatchFairnessMetadata` and an `audit_reference` (hash of the full `FairnessAuditRecord`). The full audit is stored off-band in `FairnessLedger`; the envelope carries only the reference hash.

`FairnessReferenceId` is the smallest possible pointer: `(batch_id, audit_hash, policy_version)`.

---

## Persistence

`FairnessLedger` is an in-memory BTreeMap-backed store (same pattern as `InMemoryBatchLifecycleStore` from Phase 2). All maps use `BTreeMap` for deterministic iteration.

| Store | Key | Value |
|---|---|---|
| `audit_records` | batch_id | FairnessAuditRecord |
| `snapshot_commitments` | snapshot_id | SnapshotCommitment |
| `leader_history` | leader_id | Vec<LeaderFairnessEntry> |
| `incidents` | — | Vec<UnfairSequencingIncident> |
| `forced_inclusions` | — | Vec<ForcedInclusionRecord> |

`get_incidents_for_leader(leader_id)` filters the flat incidents list — O(n) by design; in production this would be indexed.

---

## Future extension points

### Commit-reveal for ordering

The snapshot commitment model is the foundation. A stronger version requires the leader to commit to their *entire proposed ordering* (not just the snapshot) before attestors can see it. Attestors verify the reveal matches the commitment. This eliminates last-look advantage but requires two rounds of network communication per slot.

### Encrypted mempools

Submissions can be submitted encrypted (e.g. using threshold BLS or TEE-based decryption). The leader sequences ciphertexts without seeing content. After finalization, submissions are decrypted in order. This prevents all content-based MEV but requires a threshold decryption committee and increases latency. The `SubmissionFairnessProfile.classification_rule` and `FairnessClassifier` would need to classify based on metadata tags (not payload content) in this model.

### Stronger censorship resistance

The current anti-starvation mechanism forces inclusion after `anti_starvation_slots`. A stronger design: validators maintain their own queue snapshots and refuse to attest to batches that omit an eligible submission they have seen for more than the threshold. This requires validators to track the pending queue (not just verify the leader's claim).

### Governance-controlled policy

`FairnessConfig` is designed to be serialized and updated via on-chain governance. The `version` field tracks policy epochs. `FairnessAuditRecord.policy_version` and `FairnessReceipt.policy_version` allow historical records to be re-interpreted against the correct policy. The Cosmos SDK governance module in `x/poc` or `x/guard` can hold the canonical `FairnessConfig` and publish updates with time-delayed activation to prevent sudden parameter shifts.

### Sandwich detection

`AntiMevPolicy.anti_sandwich_hook_enabled` is a placeholder. A full sandwich detector would identify `(A, victim, A)` submission patterns from the same sender across a batch and flag them as `ClassBucketSkew` or a new `SandwichPattern` violation type. This requires cross-submission analysis within `AntiMevEngine` and is deferred.

### Statistical fairness scoring

Phase 3 detects per-batch violations. A higher-level layer could compute rolling statistics over `FairnessLedger.leader_history` to score leaders on their fairness track record (e.g. Gini coefficient of class distribution, omission rate per class). This could feed into the `RewardMult` module in the Cosmos chain to adjust leader rewards based on measured fairness.
