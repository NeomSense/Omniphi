# Omniphi Blockchain

A production-grade, multi-consensus blockchain ecosystem featuring Proof of Stake (PoS), Proof of Contribution (PoC), and advanced tokenomics.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Cosmos SDK](https://img.shields.io/badge/Cosmos%20SDK-v0.53-blue)](https://github.com/cosmos/cosmos-sdk)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)

## Architecture

```
omniphi/
├── chain/              # Core blockchain (Cosmos SDK)
│   ├── app/            # Application wiring
│   ├── cmd/            # Binary (posd)
│   ├── x/              # Custom modules
│   │   ├── poc/        # Proof of Contribution
│   │   ├── pos/        # Proof of Stake extensions
│   │   ├── tokenomics/ # Emission & burn mechanics
│   │   ├── feemarket/  # Dynamic fee system
│   │   └── treasury/   # Multi-sig treasury
│   ├── proto/          # Protobuf definitions
│   └── scripts/        # Setup & deployment
│
├── orchestrator/       # Validator automation system
│   ├── backend/        # FastAPI orchestration API
│   ├── frontend/       # Admin dashboard
│   ├── agent/          # Local validator agent (Electron)
│   └── docker/         # Container configurations
│
├── wallet/             # Omniphi Wallet (React)
│   └── src/            # Wallet application
│
├── validator-portal/   # Validator management portal
│
├── dashboards/         # Analytics & monitoring
│   └── tokenomics/     # Tokenomics dashboard
│
├── docs/               # Documentation
│   ├── whitepaper/     # Technical whitepaper
│   ├── architecture/   # System architecture
│   └── guides/         # Developer guides
│
└── contracts/          # Smart contracts (future)
    ├── wasm/           # CosmWasm contracts
    └── evm/            # Solidity contracts
```

## Features

### Consensus Mechanisms
- **Proof of Stake (PoS)**: Tendermint BFT consensus with delegated staking
- **Proof of Contribution (PoC)**: Reward system for network contributions
- **Hybrid Model**: Combined PoS + PoC for security and fairness

### Tokenomics
- **Decaying Inflation**: Adaptive emission schedule
- **Fee Burn**: Deflationary mechanism with 3-layer fee structure
- **Treasury**: Multi-sig governance-controlled treasury
- **Staking Rewards**: Dynamic APR based on network participation

### Infrastructure
- **Validator Orchestrator**: Automated validator deployment & management
- **Multi-Region Support**: Geographic distribution for resilience
- **Snapshot Server**: Fast node synchronization
- **Real-time Monitoring**: WebSocket-based status updates

## Quick Start

### Prerequisites
- Go 1.22+
- Node.js 18+
- Python 3.11+
- Docker (optional)

### Build the Chain
```bash
cd chain
make install
posd version
```

### Start a Local Node
```bash
cd chain
./scripts/setup_and_start.sh
```

### Run the Wallet
```bash
cd wallet
npm install
npm run dev
```

### Run the Orchestrator
```bash
cd orchestrator/backend
python -m venv venv
source venv/bin/activate  # or venv\Scripts\activate on Windows
pip install -r requirements.txt
uvicorn app.main:app --reload
```

## Testnet

| Property | Value |
|----------|-------|
| Chain ID | `omniphi-testnet-1` |
| RPC | `http://46.202.179.182:26657` |
| REST API | `http://46.202.179.182:1317` |
| Explorer | Coming soon |

## Documentation

- [Chain Documentation](chain/docs/)
- [Orchestrator Guide](orchestrator/docs/)
- [Wallet Setup](wallet/README.md)
- [Tokenomics](docs/whitepaper/)

## Contributing

We welcome contributions! Please see our contributing guidelines.

## Security

For security vulnerabilities, please email security@omniphi.io rather than opening a public issue.

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

---

Built with Cosmos SDK | Powered by Tendermint
