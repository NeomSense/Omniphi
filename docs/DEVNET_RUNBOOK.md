# PoSeq Devnet Runbook — Phase 7C

This runbook covers running, monitoring, and operating the Omniphi PoSeq devnet.

---

## Quick Start

```bash
# Build and launch 3-node devnet
make devnet-start

# Check status
make devnet-status

# Tail logs
make devnet-logs

# Stop all nodes
make devnet-stop
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  PoSeq Layer (Rust — fast lane)                                      │
│                                                                      │
│  node1 :7001  ←──────────── TCP gossip ──────────────►  node2 :7002 │
│      │                                                        │      │
│      └───────────────── node3 :7003 ───────────────────────┘        │
│                                                                      │
│  /metrics :9091     /metrics :9092     /metrics :9093               │
│  /healthz :9091     /healthz :9092     /healthz :9093               │
│  /status  :9091     /status  :9092     /status  :9093               │
│                                                                      │
│  Export dir: /tmp/poseq_devnet/exports/*.json                        │
│  Snapshot dir: /tmp/poseq_devnet/snapshots/*.json                    │
└──────────────────────────────────────────────────────────────────────┘
           │  ExportBatch JSON (epoch data)
           ▼
┌─────────────────────────────────────┐
│  Go Chain Relayer (external)        │
│  Reads exports/*.json               │
│  Writes snapshots/*.json            │
└─────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│  Cosmos Chain (Go — slow lane)      │
│  x/poseq module: IngestExportBatch  │
└─────────────────────────────────────┘
```

---

## Endpoints

| Node | TCP | Prometheus | Health | Status |
|------|-----|-----------|--------|--------|
| node1 | :7001 | http://127.0.0.1:9091/metrics | http://127.0.0.1:9091/healthz | http://127.0.0.1:9091/status |
| node2 | :7002 | http://127.0.0.1:9092/metrics | http://127.0.0.1:9092/healthz | http://127.0.0.1:9092/status |
| node3 | :7003 | http://127.0.0.1:9093/metrics | http://127.0.0.1:9093/healthz | http://127.0.0.1:9093/status |

---

## Key Prometheus Metrics

| Metric | Description |
|--------|-------------|
| `poseq_batches_finalized_total` | Total batches finalized by this node |
| `poseq_view_changes_total` | HotStuff view changes (leader timeouts) |
| `poseq_attestation_latency_ms` | Histogram: proposal → quorum latency |
| `poseq_current_epoch` | Current consensus epoch |
| `poseq_current_slot` | Current slot within epoch |
| `poseq_peer_count` | Number of connected peers |
| `poseq_epochs_exported_total` | Epoch exports completed |
| `poseq_export_dedup_hits_total` | Export dedup suppressed replays |
| `poseq_snapshots_imported_total` | Committee snapshots accepted |
| `poseq_snapshots_rejected_total` | Committee snapshots rejected |
| `poseq_node_restarts_total` | Warm restarts (non-zero sled state on boot) |

---

## Cross-Lane Bridge

### Export Flow (PoSeq → Chain)

When `--export-dir` is configured, each node writes ExportBatch JSON files
to the directory as epochs are completed:

```
/tmp/poseq_devnet/exports/
  0.json   ← epoch 0 export
  1.json   ← epoch 1 export
  ...
```

File format: `ExportBatch` JSON (see `poseq/src/chain_bridge/exporter.rs`).

The Go chain relayer reads these files and calls `IngestExportBatch` on the
Cosmos chain. After ingestion, the file can be archived or deleted.

### Snapshot Flow (Chain → PoSeq)

When `--snapshot-dir` is configured, each node polls the directory for new
`ChainCommitteeSnapshot` JSON files:

```
/tmp/poseq_devnet/snapshots/
  epoch_5.json   ← committee snapshot for epoch 5
  epoch_6.json   ← committee snapshot for epoch 6
  ...
```

The Go chain relayer writes these files after epoch settlement. The PoSeq
node imports them and updates the active committee set.

### Manual Cross-Lane Test

```bash
# 1. Write a test snapshot to the snapshot dir
cat > /tmp/poseq_devnet/snapshots/epoch_99.json << 'EOF'
{
  "epoch": 99,
  "members": [...],
  "snapshot_hash": "...",
  "produced_at_block": 9900
}
EOF

# 2. Check that a node picked it up
curl -s http://127.0.0.1:9091/status | jq .latest_snapshot_epoch
```

---

## Node State Dump

Each node writes a JSON state snapshot every 500ms to its `--state-dump-path`:

```bash
cat /tmp/poseq_devnet/state1.json
{
  "node_id_prefix": "01010101",
  "ready": true,
  "current_epoch": 3,
  "current_slot": 17,
  "in_committee": true,
  "latest_snapshot_epoch": 2,
  "exported_epoch_count": 3,
  "exported_epochs": [0, 1, 2],
  "latest_finalized": "ab12cd34",
  "slog_total": 142,
  "peer_count": 2
}
```

---

## Auto Epoch Advance

By default, `slots_per_epoch = 10`. When `current_slot` crosses a multiple of
`slots_per_epoch`:

1. `current_epoch` increments
2. `ExportEpoch(completed_epoch)` is triggered automatically
3. The exported epoch JSON appears in `--export-dir/`

To adjust: set `slots_per_epoch` in your TOML config or wait for the
configurable default.

---

## Log Format

Each node emits two log streams:

### Legacy event log
```
[poseq-node] EVENT: slot=5 epoch=0 leader="01010101" is_me=true
[poseq-node] EVENT: FINALIZED batch=ab12cd34 approvals=2
```

### Structured SLOG (JSON lines)
```
[poseq-node] SLOG: {"event":"export.completed","epoch":1,"slot":null,"level":"INFO","ts":1742000000,"details":"epoch 1 exported: 0 evidence, 0 escalations"}
[poseq-node] SLOG: {"event":"snapshot.imported","epoch":2,"slot":null,"level":"INFO","ts":1742000010,"details":"committee snapshot accepted for epoch 2"}
```

SLOG events of note:

| Event | Level | Meaning |
|-------|-------|---------|
| `export.completed` | INFO | Epoch successfully exported |
| `export.skipped` | INFO | Duplicate export suppressed |
| `export.failed` | ERROR | Export serialization error |
| `snapshot.imported` | INFO | Snapshot accepted |
| `snapshot.rejected` | WARN | Snapshot rejected (hash/dup) |
| `epoch.advance` | INFO | Epoch boundary crossed |
| `startup.restore.exported_epochs` | INFO | Warm restart: exported set restored |
| `startup.restore.snapshots` | INFO | Warm restart: snapshots restored |

---

## Restart Behavior

The devnet nodes persist state to sled under `--data-dir`. On restart:

1. `exported_epochs` is reconstructed from sled key scan (`export:epoch:*`)
2. `SnapshotImporter` is reconstructed from persisted snapshots (`chain_snapshot:*`)
3. `latest_finalized` is restored from `meta:latest_finalized`
4. `poseq_node_restarts_total` counter is incremented if non-empty state found

This means restarts are safe: no duplicate exports, no duplicate snapshot
ingestion, finalization continues from last known state.

---

## Troubleshooting

### Node won't start
- Check if port is already in use: `lsof -i :7001`
- Check log: `tail -50 /tmp/poseq_devnet/logs/node1.log`

### Metrics not available
- Verify node started: `grep READY /tmp/poseq_devnet/logs/node1.log`
- Check metrics addr binding: `curl http://127.0.0.1:9091/healthz`

### Exports not appearing
- Ensure `--export-dir` is set (check devnet.sh invocation)
- Check for `export.failed` in SLOG entries
- Verify sled write permissions on the data dir

### Snapshot not accepted
- File must be valid `ChainCommitteeSnapshot` JSON
- `snapshot_hash` must match `compute_hash(epoch, sorted_node_ids)`
- Each epoch can only be imported once (dedup by epoch number)

---

## Running Tests

```bash
# All Rust tests (unit + integration + chaos)
make test-poseq

# Chaos and failure tests only
make test-devnet-chaos

# Live cluster tests (requires compiled binary)
make test-live-cluster

# Full Phase 7C proof
make proof-phase7c
```
