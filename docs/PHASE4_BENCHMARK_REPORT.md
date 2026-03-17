# Phase 4: Benchmark Report

## Benchmark Suites

### Existing Benchmarks (from Phase 1-2)

| Suite | File | Measurements |
|-------|------|-------------|
| Pipeline | `bench_pipeline.rs` | Submit + produce batch (10/100/500 submissions) |
| Ordering | `bench_ordering.rs` | Ordering algorithm performance |
| Batching | `bench_batching.rs` | Batch composition throughput |
| Queue | `bench_queue.rs` | Queue insertion/drain |
| Validation | `bench_validation.rs` | Submission validation |

### Phase 2 Benchmarks (Intent Execution)

| Suite | File | Measurements |
|-------|------|-------------|
| Intent Pipeline | `bench_intent_pipeline.rs` | Pool admission, ordering, commit/reveal, hash verification |

### Load Profiles

| Profile | Intents/Slot | Solvers | Notes |
|---------|-------------|---------|-------|
| Light | 100 | 10 | Development testing |
| Medium | 1,000 | 50 | Testnet baseline |
| Heavy | 10,000 | 100 | Stress test target |

### Run Benchmarks

```bash
cd poseq && cargo bench --bench bench_intent_pipeline
```

### Expected Bottlenecks

1. **Ordering**: O(n × m) where n = intents, m = solvers per intent. At 10k intents × 100 solvers, this is ~1M comparisons.
2. **Hash verification**: SHA256 computation per reveal (3 hashes each). Linear in number of reveals.
3. **Fairness root**: Linear hash over all entries. Not a bottleneck.
4. **DA validation**: Linear scan of bundles × commitments. Bounded by MAX_BUNDLE_STEPS.
5. **Receipt indexing**: BTreeMap insertions, O(log n) per receipt.

### Optimization Opportunities

- **Parallel ordering**: Intent groups are independent; can rank within groups in parallel.
- **Batch hash verification**: Multiple reveals can be verified concurrently.
- **Receipt indexing**: Consider append-only log with periodic index rebuild.
- **Pool admission**: Nonce tracking and rate limiting are O(1) per user — not a bottleneck.

## Capacity Estimates

At 6-second blocks:
- **100 intents/slot**: ~17 intents/sec — well within single-thread capacity
- **1,000 intents/slot**: ~167 intents/sec — requires optimized admission path
- **10,000 intents/slot**: ~1,667 intents/sec — requires parallel processing

Storage growth (per slot, approximate):
- Intents: ~500 bytes × count
- Commitments + reveals: ~2 KB × solver_count
- Receipts: ~1 KB × filled_count
- Fairness log: ~100 bytes × entry_count

At 1,000 intents/slot, 6s blocks:
- ~14,400 slots/day
- ~14.4M intents/day
- ~7.2 GB/day storage (before pruning)
