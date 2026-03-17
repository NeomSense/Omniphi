# PoSeq Networking — Wire Protocol, Peer Lifecycle, and Devnet Guide

## Overview

PoSeq's networking layer provides real TCP-based node communication for the Proof of Sequencing protocol. All communication uses length-prefixed binary frames over persistent TCP connections. Nodes communicate proposals, attestations, and finalized batches — no simulation layer is involved.

```
poseq/src/networking/
├── messages.rs       — Wire message types and PoSeqMessage enum
├── transport.rs      — TCP framing, connection pool, NodeTransport
├── peer_manager.rs   — Peer state tracking, deduplication
├── node_runner.rs    — NodeState FSM, NetworkedNode event loop
└── devnet_scenarios.rs — Integration scenario tests (cfg(test))
```

---

## Wire Protocol

### Frame Format

Every message is encoded as a length-prefixed frame:

```
[ 4 bytes: payload length, big-endian u32 ][ N bytes: bincode payload ]
```

The payload is `bincode`-serialized `PoSeqMessage`. The 4-byte length header allows receivers to pre-allocate exact buffers with no delimiter scanning.

### Message Types

| Variant | Direction | Purpose |
|---------|-----------|---------|
| `Proposal` | Leader → All | Leader broadcasts a slot proposal with batch root and ordered submission IDs |
| `Attestation` | Attestor → Leader | Vote approving/rejecting a proposal |
| `Finalized` | Leader → All | Quorum reached; batch is final |
| `PeerStatus` | Any → Any | Self-announcement: node ID, listen address, role, epoch |
| `EpochAnnounce` | Leader → All | Epoch transition with new committee membership |
| `SyncRequest` | Any → Any | Request a specific finalized batch by ID |
| `SyncResponse` | Any → Requester | Reply with the requested batch (or None if unknown) |
| `BridgeAck` | Any → Leader | Acknowledge successful export to the runtime bridge |
| `MisbehaviorReport` | Any → All | Report Byzantine behavior with evidence |
| `CheckpointAnnounce` | Any → All | Announce a new durable checkpoint |

### Deduplication

The `PeerManager` maintains a rolling window of message content hashes (SHA-256). Messages seen within the window are silently dropped. The dedup key combines the message kind tag with stable identifying fields (proposal ID, batch ID, etc.) to prevent false positives across message types.

Window size: 512 entries (FIFO eviction).

---

## Leader Election

Leader election is deterministic and clock-free:

```
seed = SHA256(epoch_le ‖ slot_le ‖ sorted_committee_ids)
leader_index = u64::from_le_bytes(seed[0..8]) % committee.len()
leader = sorted_committee[leader_index]
```

All nodes compute the same leader for a given `(epoch, slot, committee)` tuple without coordination. The committee is sorted before hashing to ensure byte-level consistency regardless of insertion order.

### Leader Implicit Attestation

When a leader creates a proposal, it records a self-attestation before broadcasting. This counts toward quorum. With `quorum_threshold = 2` and a 3-node committee, the leader's self-attestation plus one external attestation is sufficient to finalize.

---

## Slot Progression and Finalization

```
Leader:
  advance_slot()
    → compute leader for (epoch, slot)
    → if self is leader:
        build WireProposal
        record self-attestation
        store in pending_proposals
        broadcast to peers

Attestor (on receiving Proposal):
  verify proposal.leader_id == state.current_leader
  record WireAttestation{approve=true}
  broadcast attestation to all peers

Leader (on receiving Attestation):
  count approvals for proposal_id
  if approvals >= quorum_threshold:
    build WireFinalized
    store locally
    broadcast to all peers

Any node (on receiving Finalized):
  store in finalized_batches
  update latest_finalized
```

---

## Peer Manager

`PeerManager` tracks liveness for each known peer:

| State | Meaning |
|-------|---------|
| `Connected` | Recent successful send |
| `Degraded` | ≥ 3 consecutive send failures |
| `Disconnected` | Never seen or explicitly removed |

`update_from_status()` registers/updates a peer from a `WirePeerStatus` message. `record_send_failure()` increments the failure counter and degrades at threshold 3.

---

## Transport

`NodeTransport` wraps:
- A Tokio TCP listener (bound to a configured address or port 0)
- An outbound connection pool (`BTreeMap<String, TcpStream>`)
- An inbound message channel (`mpsc::Sender<(peer_addr, PoSeqMessage)>`)

**`send_to(addr, msg)`** — attempts to reuse a cached connection; on failure (stale socket), evicts and reconnects once before returning the error.

**`broadcast(addrs, msg)`** — best-effort; logs per-peer errors but does not abort.

Inbound connections are accepted in a background Tokio task spawned at `bind()`. Each connection gets its own reader task that forwards decoded messages to the shared inbox channel.

---

## Running a Local Devnet

### Option 1: Single-Process Multi-Node (`poseq-devnet`)

Launches N in-process nodes over real loopback TCP. All nodes run as independent Tokio tasks with separate TCP sockets.

```bash
cargo run --bin poseq-devnet -- --nodes 3 --quorum 2 --slots 5

# Available options:
#   --nodes N       Number of nodes (default: 3)
#   --quorum Q      Quorum threshold (default: 2)
#   --slots S       Number of slots to run (default: 5)
#   --slot-ms MS    Slot duration in milliseconds (default: 500)
#   --scenario S    Scenario: happy_path | leader_crash | attestor_restart (default: happy_path)

# Leader crash scenario (3 nodes, leader crashes at slot 2):
cargo run --bin poseq-devnet -- --nodes 3 --quorum 2 --slots 5 --scenario leader_crash

# Attestor restart (5 nodes, quorum 3, one attestor crashes then rejoins):
cargo run --bin poseq-devnet -- --nodes 5 --quorum 3 --slots 10 --scenario attestor_restart
```

The devnet prints a per-slot state table and a final summary:

```
=== PoSeq Local Devnet ===
Nodes: 3  Quorum: 2  Slots: 5  Slot-ms: 500  Scenario: happy_path

[devnet] ── Slot 1 ─────────────────────────────────────
  Node  1: epoch=1 slot=1 finalized=a1b2c3d4 batches=1 | FINALIZED batch=[a1, b2, c3, d4] approvals=2
  Node  2: epoch=1 slot=1 finalized=a1b2c3d4 batches=1 | RECEIVED FINALIZED batch=[a1, b2, c3, d4]
  Node  3: epoch=1 slot=1 finalized=a1b2c3d4 batches=1 | RECEIVED FINALIZED batch=[a1, b2, c3, d4]
...

[devnet] SUMMARY
Unique finalized batches: 5
  batch=0xa1b2c3d4 seen_by=3 nodes
```

### Option 2: Multi-Process (`poseq-node`)

Run each node in a separate terminal. Nodes discover peers via `--peer` flags and announce via `WirePeerStatus`.

```bash
# Build first
cargo build --bins

# Terminal 1 — first node (no peers yet)
./target/debug/poseq-node \
  --id 0101010101010101010101010101010101010101010101010101010101010101 \
  --addr 127.0.0.1:7001 \
  --quorum 2

# Terminal 2
./target/debug/poseq-node \
  --id 0202020202020202020202020202020202020202020202020202020202020202 \
  --addr 127.0.0.1:7002 \
  --peer 127.0.0.1:7001 \
  --quorum 2

# Terminal 3
./target/debug/poseq-node \
  --id 0303030303030303030303030303030303030303030303030303030303030303 \
  --addr 127.0.0.1:7003 \
  --peer 127.0.0.1:7001 \
  --peer 127.0.0.1:7002 \
  --quorum 2

# Options:
#   --id       32-byte node identity as 64 hex chars (required)
#   --addr     TCP listen address (required)
#   --peer     Peer address (repeatable)
#   --quorum   Approvals needed to finalize a batch (required)
#   --slot-ms  Slot duration in ms (default: 2000)
#   --role     leader | attestor | observer (default: attestor)
```

Each node:
1. Binds the TCP listener
2. Announces itself (`WirePeerStatus`) to all configured peers
3. Advances slots on a timer (`--slot-ms`)
4. Participates in proposals, attestations, and finalization
5. Streams events to stdout

Stop with `Ctrl+C` (graceful shutdown via `NodeControl::Shutdown`).

---

## Scenario Tests

Eight integration scenarios run as `#[tokio::test]` with real TCP sockets:

| Test | What it validates |
|------|-------------------|
| `test_scenario_happy_path_finalization` | 3 nodes, quorum=2 finalize one batch end-to-end |
| `test_scenario_leader_crash_during_proposal` | Crashed leader (quorum=3) → no finalization without all nodes |
| `test_scenario_attestor_restart_and_sync` | Non-leader crash → 2 survivors finalize; rejoined node shows REJOIN log |
| `test_scenario_delayed_attestation_still_finalizes` | Slow network delay doesn't block quorum |
| `test_scenario_duplicate_messages_dropped` | Duplicate `WirePeerStatus` messages deduplicated silently |
| `test_scenario_batch_sync_after_missed_finalization` | Offline node requests missed batch via `SyncRequest/SyncResponse` |
| `test_scenario_stale_node_rejoin_after_epoch` | Node with stale epoch gets updated via `EpochAnnounce` |
| `test_scenario_misbehavior_report_propagated` | `WireMisbehaviorReport` broadcast reaches all peers |

Run all networking tests:

```bash
cargo test networking
```

---

## Invariants

1. **Leader election is stateless** — any node can compute the leader for any `(epoch, slot)` given the committee.
2. **No proposal without quorum** — `WireFinalized` is only broadcast after `approvals >= quorum_threshold`.
3. **Dedup before dispatch** — every inbound message is checked against the seen-hash window before processing.
4. **Self-attestation counts** — the leader records one approval before broadcasting; external attestations add to this count.
5. **Crashed nodes are ignored** — `NodeControl::Crash` sets a flag; `NextSlot`, `Proposal`, and `Attestation` handling are skipped while crashed.
6. **Rejoin re-announces** — `NodeControl::Rejoin` clears the crash flag and broadcasts a fresh `WirePeerStatus`.
