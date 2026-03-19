# Omniphi

A dual-lane blockchain combining a Cosmos SDK control plane with a high-throughput Proof of Sequencing (PoSeq) fast lane for intent-based execution.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![Rust Version](https://img.shields.io/badge/Rust-1.75+-orange?logo=rust)](https://www.rust-lang.org/)

## Architecture

Omniphi operates as two coordinated consensus lanes:

- **Slow Lane** (Go/Cosmos SDK) — authoritative registry, governance, staking, economics, and settlement
- **Fast Lane** (Rust/PoSeq) — BFT committee consensus, leader election, proposal finalization, and intent sequencing

The lanes communicate through a file-based bridge: PoSeq exports finalized epoch data, the Go chain ingests it and emits acknowledgments back.

```
omniphi/
├── chain/                # Cosmos SDK blockchain (Go)
│   ├── app/              # Application wiring
│   ├── cmd/              # Binary (posd)
│   └── x/                # Custom modules
│       ├── poseq/        #   PoSeq bridge — export ingestion, ACK, committee snapshots
│       ├── poc/           #   Proof of Contribution
│       ├── por/           #   Proof of Request
│       ├── guard/         #   Governance firewall (4-gate execution pipeline)
│       ├── rewardmult/    #   Reward multiplier (budget-neutral, EMA-smoothed)
│       ├── timelock/      #   Timelock governance enforcement
│       ├── feemarket/     #   Dynamic fee system
│       ├── tokenomics/    #   Emission & burn mechanics
│       └── ...            #   repgov, royalty, uci
│
├── poseq/                # Proof of Sequencing node (Rust)
│   ├── src/
│   │   ├── hotstuff/     #   HotStuff BFT consensus engine
│   │   ├── networking/   #   TCP transport, peer management, signed messaging
│   │   ├── chain_bridge/ #   Export/snapshot/ACK bridge to Go chain
│   │   ├── committee/    #   Committee membership and leader election
│   │   ├── sync/         #   State sync, checkpoints, catch-up detection
│   │   ├── crypto/       #   Ed25519 signing, keystore
│   │   ├── observability/#   Prometheus metrics, HTTP status exporter
│   │   └── ...           #   finality, misbehavior, penalties, fairness, recovery
│   ├── tests/            #   Integration tests (23 test files)
│   └── config/           #   Node TOML configs (devnet)
│
├── runtime/              # Intent execution runtime (Rust)
│   └── src/              #   CRX engine, solver market, settlement, safety kernel
│
├── solver/               # Reference solver SDK (Rust)
│   └── src/              #   Client, strategy framework, metrics
│
├── integration/          # Cross-crate integration layer (Rust)
├── tests/                # Shared test fixtures (JSON)
├── scripts/              # Devnet/testnet launch scripts
├── docs/                 # Architecture and operations documentation
├── orchestrator/         # Validator automation (Python/Docker)
├── wallet/               # Wallet application (React)
└── Makefile              # Build, test, and devnet targets
```

## Building

### Go Chain

```bash
cd chain
go build ./...
```

### PoSeq Node

```bash
cd poseq
cargo build --bin poseq-node
```

### Full Build

```bash
make build-all
```

## Testing

```bash
# Go chain tests
make test-chain

# PoSeq unit + integration tests
make test-poseq

# All tests
make test-all
```

## Running a Devnet

Start a 3-node PoSeq cluster:

```bash
make run-devnet
```

Or manually:

```bash
# Terminal 1
.poseq_target/debug/poseq-node --config poseq/config/node1.toml \
  --export-dir poseq/export_dir --snapshot-dir poseq/snapshot_dir --ack-dir poseq/ack_dir

# Terminal 2-3: same with node2.toml, node3.toml
```

Activate the committee by generating and dropping a snapshot:

```bash
.poseq_target/debug/gen-snapshot --epoch 1 --output poseq/snapshot_dir/snap_e1.json
```

## Cross-Lane Bridge

The dual-lane bridge operates through shared directories:

| Direction | Path | Format |
|-----------|------|--------|
| PoSeq -> Chain | `export_dir/<epoch>.json` | `ExportBatch` JSON |
| Chain -> PoSeq | `snapshot_dir/*.json` | `ChainCommitteeSnapshot` JSON |
| Chain -> PoSeq | `ack_dir/<epoch>.ack.json` | `ExportBatchAck` JSON |

The Go chain ingests exports via `IngestFromDirectory()`, which handles validation, deduplication, and ACK emission.

## Documentation

- [Dual-Chain Architecture](docs/DUAL_CHAIN_ARCHITECTURE.md)
- [PoSeq-Chain Integration](docs/POSEQ_CHAIN_INTEGRATION.md)
- [Bridge Lifecycle](docs/BRIDGE_LIFECYCLE.md)
- [Intent Execution Architecture](docs/INTENT_EXECUTION_ARCHITECTURE.md)
- [Devnet Runbook](docs/DEVNET_RUNBOOK.md)
- [Testnet Runbook](docs/TESTNET_RUNBOOK.md)

## Security

For security vulnerabilities, please email security@omniphichain.org rather than opening a public issue.

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
