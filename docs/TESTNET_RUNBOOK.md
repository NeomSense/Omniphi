# PoSeq Testnet Runbook — Phase 7D

This runbook covers running, monitoring, and operating the Omniphi PoSeq testnet.
For the devnet (3-node debug build), see [DEVNET_RUNBOOK.md](DEVNET_RUNBOOK.md).

---

## Quick Start

```bash
# Build release binary and launch 5-node testnet
make testnet-start

# Check status
make testnet-status

# Tail logs
make testnet-logs

# Stop all nodes
make testnet-stop
```

---

## Testnet vs Devnet

| Feature | Devnet | Testnet |
|---------|--------|---------|
| Nodes | 3 | 5 |
| Quorum | 2/3 | 3/5 |
| Build | debug | release |
| Slot interval | 2s | 5s |
| Metrics ports | 9091–9093 | 9191–9195 |
| TCP ports | 7001–7003 | 7101–7105 |
| Peer heartbeat | every 3 slots | every 3 slots |
| Reconnect backoff | exp, 1s–60s | exp, 1s–60s |
| Config validation | no | yes |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│  PoSeq Testnet (Rust — fast lane)                                       │
│                                                                         │
│  node1 :7101 ─────── TCP gossip (full mesh) ──────── node2 :7102       │
│      │                       │                              │           │
│   node5 :7105 ─────────── node4 :7104 ─────────── node3 :7103          │
│                                                                         │
│  Quorum: 3/5 (Byzantine fault-tolerant — 1 node can fail)              │
│                                                                         │
│  /metrics :9191–:9195   /healthz :9191–:9195   /status :9191–:9195     │
│                                                                         │
│  Export dir:   /tmp/poseq_testnet/exports/*.json                        │
│  Snapshot dir: /tmp/poseq_testnet/snapshots/*.json                      │
└─────────────────────────────────────────────────────────────────────────┘
           │  ExportBatch JSON
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
| node1 | :7101 | http://127.0.0.1:9191/metrics | http://127.0.0.1:9191/healthz | http://127.0.0.1:9191/status |
| node2 | :7102 | http://127.0.0.1:9192/metrics | http://127.0.0.1:9192/healthz | http://127.0.0.1:9192/status |
| node3 | :7103 | http://127.0.0.1:9193/metrics | http://127.0.0.1:9193/healthz | http://127.0.0.1:9193/status |
| node4 | :7104 | http://127.0.0.1:9194/metrics | http://127.0.0.1:9194/healthz | http://127.0.0.1:9194/status |
| node5 | :7105 | http://127.0.0.1:9195/metrics | http://127.0.0.1:9195/healthz | http://127.0.0.1:9195/status |

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
| `poseq_peers_connected` | Peers in Connected state |
| `poseq_peers_degraded` | Peers in Degraded state (silent but not evicted) |
| `poseq_peers_disconnected` | Peers in Disconnected state (evicted / unresponsive) |
| `poseq_epochs_exported_total` | Epoch exports completed |
| `poseq_export_dedup_hits_total` | Export dedup suppressed replays |
| `poseq_snapshots_imported_total` | Committee snapshots accepted |
| `poseq_snapshots_rejected_total` | Committee snapshots rejected |
| `poseq_node_restarts_total` | Warm restarts (non-zero sled state on boot) |

---

## Peer Lifecycle

### Peer States

```
Disconnected ──(PeerStatus received)──► Connected
Connected    ──(silence > 10 slots)──► Degraded
Degraded     ──(silence > 50 slots)──► Disconnected
Disconnected ──(reconnect attempt)──► Connected
```

### Reconnect Backoff

When a send fails, the peer's `reconnect_backoff_ms` doubles (cap: 60s):

| Failure # | Backoff |
|-----------|---------|
| 0 | 1s (initial) |
| 1–2 | 1s |
| 3 (Degraded) | 2s |
| 4 | 4s |
| 5 | 8s |
| ... | ... |
| 10+ | 60s (capped) |

A successful reconnect resets the backoff to 1s.

### Heartbeat

Every 3 slots (15s at 5s/slot), each node:
1. Runs `tick_health_check` — degrades silent peers
2. Runs `evict_dead_peers` — disconnects long-silent peers
3. Broadcasts `PeerStatus` to all `Disconnected` peers as a reconnect probe

---

## Version Compatibility

Each `PeerStatus` message includes `protocol_version: "1.0.0"`.

The node logs a warning when connecting to a peer with an older minor version.
Major version mismatches cause the connection to be rejected at the application layer.

Current protocol version: **1.0.0**
Minimum compatible: **1.0.0**

---

## Fault Injection

### Kill a node

```bash
# Kill node 3 (simulates crash)
./scripts/testnet.sh kill-node 3

# Observe: remaining 4 nodes continue with quorum 3/4
# Node 3 will appear as Disconnected on other nodes after ~50 slots
```

### Restart a killed node

```bash
# Restart node 3 manually (using its original config)
./.poseq_target/release/poseq-node \
    --id 3333...3333 \
    --addr 127.0.0.1:7103 \
    --peer 127.0.0.1:7101 \
    --peer 127.0.0.1:7102 \
    --peer 127.0.0.1:7104 \
    --peer 127.0.0.1:7105 \
    --quorum 3 \
    --slot-ms 5000 \
    --data-dir /tmp/poseq_testnet/data3 \
    --metrics-addr 127.0.0.1:9193 \
    --state-dump-path /tmp/poseq_testnet/state3.json \
    --export-dir /tmp/poseq_testnet/exports \
    --snapshot-dir /tmp/poseq_testnet/snapshots
```

On restart, the node performs a warm restart:
- Restores `exported_epochs` from sled
- Restores `SnapshotImporter` from persisted snapshots
- Restores `latest_finalized` from sled
- Increments `poseq_node_restarts_total`
- Reconnect probes fire within 3 slots

---

## Config Validation

Before `testnet.sh start` runs the binary, it validates:
- Binary exists at `$POSEQ_BIN`
- All TCP ports are available

The PoSeq binary itself validates at startup:
- All `--peer` addresses are valid `host:port` strings
- No duplicate peer addresses
- `slots_per_epoch` is 1–10000
- `quorum` is 1–100
- `slot_ms` is 100–60000

If validation fails, the node exits immediately with an error message.

---

## Running Tests

```bash
# Network hardening tests (peer lifecycle, config validation, version roundtrip)
make test-network-hardening

# All Rust tests
make test-poseq

# Chaos and failure tests
make test-devnet-chaos

# Full Phase 7D proof
make proof-phase7d
```

---

## Troubleshooting

### Node won't start
- Check port availability: `lsof -i :7101`
- Check binary exists: `ls -la .poseq_target/release/poseq-node`
- Check logs: `tail -50 /tmp/poseq_testnet/logs/node1.log`

### Peer not connecting
- Check `poseq_peers_connected` gauge — should be 4 per node in a healthy testnet
- Look for `READY node_id=` in logs to confirm node started
- Check `poseq_peers_degraded` — if non-zero, heartbeat hasn't reconnected yet (wait up to 3 slots)

### Consensus stalled
- Verify at least 3 nodes are running (`testnet-status`)
- Check `poseq_view_changes_total` — if climbing, leader rotation is happening (expected)
- Check `poseq_peer_count` on each node — must be ≥ 2 for quorum 3

### Exports not appearing
- Verify `--export-dir` is set
- Check `poseq_epochs_exported_total` counter increasing
- Look for `export.failed` in SLOG entries

### Wrong protocol version
- Check `protocol_version` field in `/status` JSON on each node
- Nodes log `WARN: remote version X is older than local Y` for minor version mismatch
- Major version mismatch causes the `PeerStatus` to be silently ignored (peer won't be promoted to Connected)

---

## SLOG Events of Note

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
| `version.incompatible` | WARN | Peer version rejected; peer marked Disconnected |
| `version.warning` | WARN | Peer is on older minor version; accepted with warning |

---

## Phase 8 — State Sync, Catch-Up, and Long-Run Reliability

### Sync Status

Every node tracks sync state internally. Query via `/status` endpoint:

```bash
curl -s http://127.0.0.1:9191/status | jq .sync_status
```

Response fields:

```json
{
  "local_epoch": 42,
  "peer_max_epoch": 42,
  "lag": 0,
  "is_catching_up": false,
  "latest_checkpoint_epoch": 40,
  "bridge_backlog": 0,
  "bridge_acked": 420,
  "bridge_failed": 0
}
```

| Field | Meaning |
|-------|---------|
| `local_epoch` | Node's current epoch |
| `peer_max_epoch` | Highest epoch seen from any peer |
| `lag` | `peer_max_epoch - local_epoch` (0 = in sync) |
| `is_catching_up` | `lag > 0` |
| `latest_checkpoint_epoch` | Most recent snapshot checkpoint epoch |
| `bridge_backlog` | Bridge exports pending delivery |
| `bridge_acked` | Bridge exports acknowledged by chain |
| `bridge_failed` | Bridge exports that exhausted retries |

### Prometheus Sync Metrics

| Metric | Description |
|--------|-------------|
| `poseq_sync_lag_epochs` | Epoch lag behind peer_max_epoch (0 = in sync) |
| `poseq_bridge_backlog` | Bridge batches currently pending delivery |
| `poseq_bridge_failed_total` | Total bridge batches that reached Failed state |

### Checkpoint Lifecycle

Every 10 epochs (default), the node creates a recovery checkpoint:

1. `RecoveryCheckpoint` — epoch-scoped integrity anchor (SHA256 of epoch/slot/batch/committee/bridge state)
2. `PoSeqCheckpoint` — includes finality state and bridge hash
3. Both are stored in-memory in `StateSyncEngine`

Checkpoints are retained for the last 50 epochs (default). Older checkpoints are pruned automatically.

To check checkpoints:
```bash
# Checkpoint info visible in /status sync_status.latest_checkpoint_epoch
curl -s http://127.0.0.1:9191/status | jq .sync_status.latest_checkpoint_epoch
```

### Bridge Delivery Tracking

Each exported epoch is tracked through delivery states:

```
Pending → Exporting → Exported → Acknowledged  (happy path)
                    ↘ Rejected → RetryPending   (retry)
                    ↘ Failed                    (max retries)
```

Bridge state is tracked in the `StateSyncEngine` per-node. On restart, the bridge state is reset (in-memory only). The dedup protection on the Go chain side (`IngestExportBatch`) prevents duplicate delivery effects even if re-exported.

### Late Join / Catch-Up

A new node joining an existing testnet will:
1. Boot with `local_epoch = 0`, see `peer_max_epoch = N` from peer status broadcasts
2. `sync_status.is_catching_up = true`, `lag = N`
3. The node will catch up as it processes finalized batches from peers
4. Once `local_epoch = peer_max_epoch`, `is_catching_up = false`

To verify catch-up progress:
```bash
watch -n 2 'curl -s http://127.0.0.1:9195/status | jq ".sync_status.lag, .sync_status.local_epoch"'
```

### Recovery After Long Downtime

A node restarting after multiple missed epochs:

1. **Warm restart detected** — `poseq_node_restarts_total` increments
2. **State restored from sled** — exported epochs, snapshots, latest finalized batch
3. **Catch-up begins** — node broadcasts PeerStatus, collects peer epochs
4. **Sync lag computed** — visible via `poseq_sync_lag_epochs`
5. **Node rejoins active consensus** once peers respond and it receives catch-up batches

The key invariants that prevent divergence:
- `exported_epochs` dedup set prevents duplicate chain exports
- `SnapshotImporter` dedup prevents duplicate snapshot ingestion
- Bridge delivery state tracking prevents duplicate economic effects

### Running Phase 8 Tests

```bash
# State sync and soak tests
make test-state-sync

# Full Phase 8 proof (all tests)
make proof-phase8
```
