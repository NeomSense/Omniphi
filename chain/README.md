# Omniphi Blockchain

A production-ready Proof of Stake blockchain built with Cosmos SDK v0.53.3 and CometBFT consensus, featuring custom modules for adaptive fee market, tokenomics, and proof of contribution.

## Chain Specifications

| Parameter | Value |
|-----------|-------|
| **Chain ID** | `omniphi-testnet-1` |
| **Token** | OMNI (1 OMNI = 1,000,000 omniphi) |
| **Total Supply** | 1,500,000,000 OMNI (1.5 Billion) |
| **Block Time** | ~4 seconds |
| **Block Gas Limit** | 60,000,000 |
| **Max Tx Gas** | 2,000,000 |
| **Consensus** | CometBFT (instant finality) |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Omniphi Blockchain Architecture            │
├─────────────────────────────────────────────────────────┤
│  Consensus: CometBFT (Proof of Stake)                   │
│  ├─ Validators stake OMNI to produce blocks             │
│  ├─ Byzantine Fault Tolerance                           │
│  └─ Instant finality (~4 second blocks)                 │
│                                                         │
│  Core Modules (Cosmos SDK v0.53.3):                     │
│  ├─ x/staking      → Validators & Delegations           │
│  ├─ x/bank         → Token transfers                    │
│  ├─ x/gov          → On-chain governance                │
│  ├─ x/distribution → Staking rewards                    │
│  ├─ x/slashing     → Validator penalties                │
│  └─ x/auth         → Accounts & signatures              │
│                                                         │
│  Custom Modules:                                        │
│  ├─ x/feemarket    → EIP-1559 Adaptive Fee Market       │
│  ├─ x/tokenomics   → Token Economics & Burn Mechanics   │
│  └─ x/poc          → Proof of Contribution              │
└─────────────────────────────────────────────────────────┘
```

## Features

### Adaptive Fee Market (EIP-1559)
- Dynamic base fee adjusts based on block utilization
- Tiered burn rates: Cool (10%), Normal (20%), Hot (40%)
- Activity-based multipliers for different transaction types
- Target utilization: 33% (leaves headroom for bursts)

### Anchor Lane Design
- Max tx gas: 2M (prevents single-tx block dominance)
- Requires 30+ transactions to fill a block
- Forces heavy computation to PoSeq layer
- Protects validator resources

### Tokenomics
- 1.5B OMNI total supply cap
- Fee distribution: Burn → Validators (70%) + Treasury (30%)
- Emission splits: Staking (40%), PoC (30%), Sequencer (20%), Treasury (10%)

## Quick Start

### Build from Source

```bash
git clone https://github.com/NeomSense/PoS-PoC.git
cd PoS-PoC
go build -o posd ./cmd/posd
./posd version
```

### Initialize Testnet

```bash
# Initialize node
./posd init my-node --chain-id omniphi-testnet-1 --default-denom omniphi

# Add validator key
./posd keys add validator --keyring-backend test

# Add genesis account
./posd genesis add-genesis-account validator 1000000000000000omniphi --keyring-backend test

# Create validator genesis tx
./posd genesis gentx validator 100000000000000omniphi --chain-id omniphi-testnet-1 --keyring-backend test

# Collect genesis transactions
./posd genesis collect-gentxs

# Start chain
./posd start
```

### Key Commands

```bash
# Check status
posd status

# Query modules
posd query feemarket params
posd query tokenomics params
posd query staking validators

# Send transaction
posd tx bank send <from> <to> 1000000omniphi --fees 100000omniphi --chain-id omniphi-testnet-1
```

## Documentation

| Document | Description |
|----------|-------------|
| [docs/FEE_BURN_MODEL.md](docs/FEE_BURN_MODEL.md) | Single-pass multiplicative burn model |
| [docs/EMISSION_SPLIT.md](docs/EMISSION_SPLIT.md) | Token emission distribution |
| [docs/TREASURY_REDIRECT.md](docs/TREASURY_REDIRECT.md) | Treasury management |
| [docs/GOVERNANCE.md](docs/GOVERNANCE.md) | Governance proposals |
| [docs/VPS_ANCHOR_LANE_UPDATE.md](docs/VPS_ANCHOR_LANE_UPDATE.md) | Validator configuration guide |

## Project Structure

```
.
├── app/                    # Application configuration
├── cmd/posd/               # CLI binary
├── proto/                  # Protobuf definitions
├── x/                      # Custom modules
│   ├── feemarket/          # EIP-1559 adaptive fee market
│   ├── tokenomics/         # Token economics
│   └── poc/                # Proof of contribution
├── scripts/                # Utility scripts
│   ├── init_testnet.sh     # Testnet initialization
│   └── update_anchor_lane_config.sh  # CometBFT configuration
├── proposals/              # Example governance proposals
└── docs/                   # Documentation
```

## System Requirements

- **Go**: 1.23+
- **OS**: Ubuntu 20.04+, Windows 11, macOS
- **RAM**: 4GB minimum, 8GB recommended
- **Disk**: 100GB SSD
- **Network**: 100Mbps+

## Testnet Validators

| Node | IP | Status |
|------|-----|--------|
| omniphi-node | 46.202.179.182 | Active |
| omniphi-node-2 | 148.230.125.9 | Active |

## License

[Add license here]

---

*Built with Cosmos SDK v0.53.3*
