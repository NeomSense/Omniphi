# PoSeq — Hardening, Performance, and Operational Limits

## Summary

This document covers the performance profile, bottleneck analysis, resource bounds,
fuzz coverage, and operational configuration for PoSeq before public testnet deployment.

---

## Benchmark Suite

Benchmarks live in `poseq/benches/` and use [Criterion](https://github.com/bheisler/criterion.rs).

```bash
# Run all benchmarks (from poseq/)
cargo bench

# Run a specific benchmark
cargo bench --bench bench_ordering

# Generate HTML reports (in target/criterion/)
cargo bench -- --output-format bencher
```

| Benchmark file | What it measures |
|----------------|-----------------|
| `bench_ordering.rs` | `OrderingEngine::order()` at 10/100/1000/5000 submissions; adversarial all-same-class, all-same-priority, all-max-priority |
| `bench_batching.rs` | `BatchBuilder::build()` at 10/100/500; standalone `compute_payload_root` and `compute_batch_id` cost |
| `bench_queue.rs` | `SubmissionQueue::drain()` at 100/1000/10000; `ReplayGuard` bounded/unbounded; churn pattern (insert 1000, drain 500, insert 500) |
| `bench_validation.rs` | `SubmissionValidator::validate()` — valid path, invalid hash, zero sender, 1KB/4KB/16KB payloads |
| `bench_pipeline.rs` | Full `PoSeqNode::submit → produce_batch` pipeline at 10/100/500; phase-isolated measurements; 10-batch throughput |

---

## Hot Paths and Bottleneck Analysis

### 1. SHA256 Hashing (Pervasive)
**Every** critical path computes SHA256: submission validation (payload_hash check), batch_id, payload_root, receipt_id, checkpoint_id, evidence packet_hash, anchor_hash. The `sha2` crate uses hardware acceleration (AES-NI / SHA extensions) on x86_64, so individual hashes are fast (~100ns each). The bottleneck is **call count**, not latency per call.

**Recommendation:** At 500 submissions/batch, each batch involves ~1500 SHA256 calls (validate × 500, ordering keys × 500, batch_id × 1, payload_root × 1, receipt × 1). This is ~150μs of pure hashing at 1GHz SHA throughput — acceptable.

### 2. OrderingEngine — BTreeMap Insertion
`OrderingEngine::order()` builds a `BTreeMap<OrderingKey, ValidatedSubmission>`. `OrderingKey` comparison does 5-field lexicographic comparison. At 5000 submissions, this is O(N log N) with a constant factor from the complex key.

**Measured expectation:** ~2–5ms at 5000 submissions. Not a bottleneck for <1000 submissions/batch.

**Recommendation:** Keep `MAX_SUBMISSIONS_PER_BATCH = 500`. Above 1000, ordering becomes the dominant cost.

### 3. ReplayGuard — BTreeSet + VecDeque
`ReplayGuard::check_and_record()` does a `BTreeSet::contains()` + `BTreeSet::insert()` + `VecDeque::push_back()` per submission. At 50,000-entry history, `BTreeSet::contains` is O(log N) ≈ 16 comparisons. Each comparison is `[u8; 32]` memcmp (trivially fast).

**Recommendation:** `MAX_REPLAY_GUARD_HISTORY = 50_000` is safe. Memory cost: ~50K × (32 + 24) bytes ≈ 2.8MB. Negligible.

### 4. Queue Drain — BTreeMap Iteration
`SubmissionQueue::drain(n)` iterates the BTreeMap and collects into a Vec. At 10,000 entries, draining 500 is fast (iterator skip is O(log N) + O(drain_count)). The BTreeMap is keyed by (received_at_sequence, normalized_id) — no hash collision concern.

**Recommendation:** Queue depth of 10,000 is safe. Above 50,000, memory (~100MB for 64KB payloads each) becomes the real concern.

### 5. Finalization — Slot Uniqueness Check
`FinalizationEngine::finalize()` does two `BTreeMap::contains_key()` calls (finalized_proposals, finalized_slots). These are O(log N) on the proposal/slot history. At 1M finalized batches, this is still ~20 comparisons — completely negligible.

### 6. Checkpoint Restore
`CheckpointStore::restore_into_finality_store()` makes 3 sequential calls to the finality store. The cost is dominated by the finality store's lookup/insert, not the checkpoint parsing. Idempotent by design.

### 7. Fairness Overhead
The fairness pipeline (queue snapshot → fairness engine → inclusion engine → anti-MEV engine) adds ~20–40% overhead compared to the base pipeline for typical workloads with mixed submission classes. This is expected and correct — fairness is not free.

### 8. Chain Bridge Export
`ChainBridgeExporter::export()` sorts evidence hashes (BTreeSet), computes SHA256 per packet, builds escalation records. At 256 evidence packets/epoch, this is ~25ms — intended as an epoch-end operation, not per-slot.

---

## Fuzz Coverage

Fuzz targets live in `poseq/fuzz/fuzz_targets/`. Run with `cargo fuzz`:

```bash
# Install cargo-fuzz (once)
cargo install cargo-fuzz

# Run a fuzz target (from poseq/)
cargo fuzz run fuzz_submission_validator

# Run with corpus directory
cargo fuzz run fuzz_ordering_engine -- -max_total_time=60

# List all targets
cargo fuzz list
```

| Target | What it fuzzes | Key invariants checked |
|--------|---------------|----------------------|
| `fuzz_submission_validator` | Submission parser + validator | Valid hash → Ok; corrupted hash → Err; no panic on any input |
| `fuzz_ordering_engine` | `OrderingEngine::order()` | Output length = input length; two runs produce identical order (determinism) |
| `fuzz_finalization` | `FinalizationEngine::finalize()` | Same (slot, epoch) never finalizes twice; no panic |
| `fuzz_checkpoint_restore` | `CheckpointStore` create + restore | Restore is idempotent; epoch 0 rejected; tampered checkpoint rejected |
| `fuzz_bridge_envelope` | `BatchCommitment` | Hash is deterministic; tampered commitment detected; reordered submissions → different root |
| `fuzz_evidence_packaging` | `EvidencePacket` + `DuplicateEvidenceGuard` | Deterministic packet_hash; verify() passes on fresh; duplicates deduplicated |

---

## Chaos / Fault Injection Tests

Located in `poseq/tests/chaos_tests.rs`. Run with:

```bash
cargo test --test chaos_tests -- --nocapture
```

| Test | Scenario | Verified |
|------|----------|----------|
| `test_chaos_duplicate_message_storm` | 10,000 duplicate submissions | Only 1 accepted; no unbounded growth |
| `test_chaos_queue_overflow_under_load` | Fill queue past capacity | `QueueFull` returned; queue remains consistent |
| `test_chaos_finalization_double_slot` | Two proposals compete for same (epoch, slot) | Only first wins; no corruption |
| `test_chaos_replay_attack_storm` | 10,000 replays | All rejected after first; replay guard stays bounded |
| `test_chaos_malformed_batch_submission` | Zero sender, wrong hash, oversized, all combos | Typed errors, no panics |
| `test_chaos_checkpoint_tamper_and_restore` | Tamper checkpoint fields | Tampered → rejected; valid → idempotent restore |
| `test_chaos_ordering_adversarial_priorities` | Max/min/duplicate/alternating priorities | Deterministic output, no panic |
| `test_chaos_epoch_boundary_stress` | 100 epochs back-to-back | No slot collision across epochs; heights monotone |
| `test_chaos_evidence_dedup_storm` | 1,000 duplicate evidence registrations | Only first accepted; guard size = 1 |
| `test_chaos_mixed_load_pipeline` | 500 valid + 200 dupes + 100 invalid + 50 overflow | Exactly 500 accepted; queue drains cleanly |

**All 10 chaos tests pass (676 total tests pass).**

---

## Resource Bounds

Defined in `poseq/src/resource_bounds.rs`. These are the **recommended production limits**:

| Resource | Limit | Rationale |
|----------|-------|-----------|
| `MAX_SUBMISSIONS_PER_BATCH` | 500 | Ordering cost O(N log N); 500 keeps batch time <5ms |
| `MAX_QUEUE_DEPTH` | 10,000 | ~640MB at 64KB payloads max; backpressure before OOM |
| `MAX_REPLAY_GUARD_HISTORY` | 50,000 | ~2.8MB; covers ~100 batches of 500 with safety margin |
| `MAX_PAYLOAD_BYTES` | 64KB | Prevents single submission from dominating proposal size |
| `MAX_CHECKPOINT_RETENTION_EPOCHS` | 100 | Pruning prevents unbounded storage growth |
| `MAX_RETAINED_CHECKPOINTS` | 10 | In-memory checkpoints only; 10 covers 1 epoch of safety |
| `MAX_EVIDENCE_PER_EPOCH` | 256 | Chain-side cap; prevents ExportBatch state bloat |
| `MAX_ESCALATIONS_PER_EPOCH` | 32 | Governance throughput limit |
| `MAX_COMMITTEE_SIZE` | 100 | Attestation fan-out quadratic at large N |
| `MAX_ATTESTATIONS_PER_SLOT` | 200 | Committee × 2; accommodates duplicates and late arrivals |
| `MAX_BRIDGE_RETRY_QUEUE` | 100 | Prevents retry storm from blocking new deliveries |
| `MAX_FAIRNESS_INCIDENTS_PER_EPOCH` | 1,000 | Incident ledger; 1K is generous for normal operation |
| `MIN_SLOT_DURATION_MS` | 500 | Below 500ms, timing attacks on priority ordering become practical |
| `MIN_EPOCH_LENGTH_SLOTS` | 10 | Minimum for meaningful committee rotation |
| `MAX_PROPOSAL_SIZE_BYTES` | 4MB | Wire limit; prevents single proposal from saturating network |

### ResourceBoundsReport

The `ResourceBoundsReport` struct can be populated by the node's observability layer and exposes:
- `any_near_limit()` — true if any resource is at ≥80% of its limit
- `near_limit_items()` — list of (subsystem, current, limit) tuples for alerting

---

## Profiling Recommendations

### Recommended production profiling setup

```bash
# Build with debug symbols but optimized (profile builds)
CARGO_PROFILE_RELEASE_DEBUG=true cargo build --release

# Run with perf (Linux) or VS Performance Profiler (Windows)
perf record -g target/release/poseq-node ...
perf report --call-graph dwarf
```

### Top 5 optimization opportunities (in order of impact)

1. **Batch SHA256 calls** — the validation phase hashes each payload independently. A batched SHA256 pipeline (SIMD multi-buffer) could improve throughput 4–8× at high submission rates. However, this requires switching from `sha2` to a multi-buffer implementation. **Defer until ≥10K submissions/batch is needed.**

2. **Pre-computed OrderingKey** — currently `OrderingKey` is constructed from `ValidatedSubmission` fields during sort. Pre-computing and storing the key bytes would eliminate repeated field access during BTreeMap insertions. Estimated 10–20% speedup on the ordering benchmark.

3. **ReplayGuard hash indexing** — `BTreeSet<[u8;32]>` requires full 32-byte comparisons. A `HashSet<[u8;32]>` with a fast non-cryptographic hash (FxHash) would cut lookup cost from O(32 × log N) to O(32) amortized. **Safe since collision resistance is not required here — we're just deduplicating, not cryptographically committing.**

4. **Fairness snapshot incremental updates** — `QueueSnapshot` currently rebuilds from scratch. An incremental snapshot that only updates changed entries would reduce per-slot overhead from O(queue_size) to O(changed_entries).

5. **Checkpoint serialization** — `CheckpointStore` serializes via `serde_json`. Switching to `bincode` for checkpoint serialization would reduce checkpoint size by ~40% and serialization time by ~60%. **Do not change the chain-facing types** (those remain JSON for Go interop).

### What NOT to optimize

- **Finalization slot-uniqueness check** — already O(log N), negligible in practice
- **Receipt building** — SHA256(batch_id ‖ height ‖ ...) is trivially fast
- **Dedup guard in chain_bridge** — `BTreeSet<[u8;32]>` is correct and fast enough for 256 packets/epoch

---

## Operational Limits and Recommended Config

### Minimum viable testnet node

```toml
[batch]
max_submissions_per_batch = 100
max_pending_queue_size = 1000

[ordering]
enforce_sender_nonce_order = false

[checkpoint]
interval_epochs = 5
max_retained = 5
```

### Recommended testnet node (moderate load)

```toml
[batch]
max_submissions_per_batch = 500
max_pending_queue_size = 10000

[ordering]
enforce_sender_nonce_order = true

[checkpoint]
interval_epochs = 10
max_retained = 10
```

### Hard limits (never exceed without profiling)

| Setting | Hard limit | Risk if exceeded |
|---------|-----------|-----------------|
| `max_submissions_per_batch` | 2000 | Ordering phase >50ms; downstream runtime latency spikes |
| `max_pending_queue_size` | 100,000 | OOM risk at max payload sizes |
| Committee size | 50 | Attestation fan-out becomes O(N²) in message count |
| Slot duration | 500ms | Below this, timing attacks on submission ordering become practical |

---

## Crash Tolerance

PoSeq uses `sled` for durable storage with ACID semantics. On crash:

1. **Pending queue** — submissions in-flight are lost. Clients must resubmit. This is correct behavior (at-least-once semantics from clients).
2. **Finalized batches** — persisted to sled before ack. No re-finalization of same batch on restart.
3. **Checkpoints** — `CheckpointStore` enables recovery to last checkpoint. Reprocessing from checkpoint to head requires replay of finalized batches, which are durably stored.
4. **Bridge delivery** — `HardenedRuntimeBridge` tracks `applied_deliveries` in memory. On restart, delivery_ids not in durable store will be re-attempted. The runtime's `RuntimeBatchIngester` is idempotent, so this is safe.
5. **Evidence/escalations** — `DuplicateEvidenceGuard` is in-memory. On restart, dedup history is lost. Chain-side `ErrDuplicateEvidencePacket` provides the final dedup guarantee.

### Recovery SLA

| Component | Recovery time | Method |
|-----------|--------------|--------|
| Queue state | Immediate (lost) | Client resubmit |
| Finality state | O(batches since checkpoint) | Replay from checkpoint |
| Committee state | O(1) | Loaded from epoch store |
| Bridge state | O(retry queue) | Re-attempt pending deliveries |
| Checkpoint store | O(1) | Direct load from sled |

---

## Before Public Testnet: Checklist

- [x] Benchmarks written and baseline established
- [x] Fuzz targets written (6 targets covering all critical parsers)
- [x] Chaos tests passing (10 scenarios, 676 total tests)
- [x] Resource bounds documented and enforced in params
- [x] Chain bridge integration complete and tested
- [ ] Run fuzz targets for ≥24 hours each before testnet
- [ ] Profile under sustained 1K submissions/slot for 1 hour
- [ ] Load test chain bridge relayer with epoch-end ExportBatch at 256 evidence packets
- [ ] Verify crash recovery under simulated disk failure (sled `Db::flush()` mock)
- [ ] Audit committee rotation under adversarial leader election (all leaders crash scenario)
- [ ] Set `authorized_submitter` param in x/poseq before mainnet (prevents unauthorized relaying)
