# Omniphi Blockchain - Production Ready ✅

**A production-ready Proof of Stake blockchain** built with Cosmos SDK v0.53.3 and CometBFT consensus, featuring three custom modules: Adaptive Fee Market, Tokenomics, and Proof of Contribution.

## 🚀 Quick Start

### Ubuntu

```bash
cd ~/omniphi/pos
git pull origin main
chmod +x setup_ubuntu_fixed.sh
./setup_ubuntu_fixed.sh
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

**→ Complete guide**: [UBUNTU_DEPLOYMENT_GUIDE.md](UBUNTU_DEPLOYMENT_GUIDE.md)

### Windows

```bash
cd ~/omniphi/pos
git pull origin main
chmod +x test_windows.sh
./test_windows.sh
```

**→ Complete guide**: [WINDOWS_TEST_SUCCESS.md](WINDOWS_TEST_SUCCESS.md)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│           Omniphi Blockchain Architecture                │
├─────────────────────────────────────────────────────────┤
│  Consensus: CometBFT (Proof of Stake)                   │
│  ├─ Validators stake tokens to produce blocks           │
│  ├─ Byzantine Fault Tolerance                            │
│  └─ Instant finality                                     │
│                                                           │
│  Core Modules (Cosmos SDK v0.53.3):                     │
│  ├─ x/staking      → Validators & Delegations           │
│  ├─ x/bank         → Token transfers                     │
│  ├─ x/gov          → On-chain governance                 │
│  ├─ x/distribution → Staking rewards                     │
│  ├─ x/slashing     → Validator penalties                 │
│  └─ x/auth         → Accounts & signatures               │
│                                                           │
│  Custom Modules:                                         │
│  ├─ x/feemarket    → Adaptive Fee Market (EIP-1559)     │
│  ├─ x/tokenomics   → Token Economics & Distribution     │
│  └─ x/poc          → Proof of Contribution               │
└─────────────────────────────────────────────────────────┘
```

---

## Features

### ⚡ Proof of Stake Consensus
- ✅ **Validator Network**: Secure the chain by running a validator
- ✅ **Staking & Delegation**: Stake tokens to earn block rewards
- ✅ **Slashing Protection**: Auto-penalties for misbehavior
- ✅ **Governance**: On-chain voting for upgrades
- ✅ **IBC Support**: Inter-Blockchain Communication ready

### 💰 Adaptive Fee Market (EIP-1559)
- ✅ **Dynamic Base Fee**: Automatically adjusts based on block utilization
- ✅ **Fee Burning**: Deflationary mechanism via fee burns
- ✅ **Predictable Costs**: Target utilization ensures stable gas prices
- ✅ **Elasticity Multiplier**: Smooth fee adjustments

### 📊 Tokenomics
- ✅ **Fee Distribution**: Split between burn, treasury, and validators
- ✅ **Supply Tracking**: Real-time supply management
- ✅ **Treasury Management**: Funding for ecosystem development
- ✅ **Validator Rewards**: Incentivize network security

### 🏆 Proof of Contribution
- ✅ **Reputation System**: Track and reward contributions
- ✅ **Contribution Scores**: Quantify participant value
- ✅ **Reward Mechanisms**: Incentivize positive behavior

---

## Status: Production Ready ✅

| Platform | Status | Blocks Verified | Documentation |
|----------|--------|-----------------|---------------|
| Ubuntu   | ✅ Working | Height 2+ | [UBUNTU_DEPLOYMENT_GUIDE.md](UBUNTU_DEPLOYMENT_GUIDE.md) |
| Windows  | ✅ Working | Height 28+ | [WINDOWS_TEST_SUCCESS.md](WINDOWS_TEST_SUCCESS.md) |

**Last Tested**: 2025-11-05
**Deployment Status**: [DEPLOYMENT_SUCCESS.md](DEPLOYMENT_SUCCESS.md)

---

## Documentation

### Essential Guides
- **[UBUNTU_DEPLOYMENT_GUIDE.md](UBUNTU_DEPLOYMENT_GUIDE.md)** - Complete Ubuntu setup and troubleshooting
- **[DEPLOYMENT_SUCCESS.md](DEPLOYMENT_SUCCESS.md)** - Overall deployment status and verification
- **[VALIDATOR_FIX_SUMMARY.md](VALIDATOR_FIX_SUMMARY.md)** - Technical details on validator genesis

### Testing & Verification
- **[UBUNTU_TESTING_GUIDE.md](UBUNTU_TESTING_GUIDE.md)** - Complete Ubuntu testing (automated + manual)
- **[WINDOWS_TESTING_GUIDE.md](WINDOWS_TESTING_GUIDE.md)** - Complete Windows testing (automated + manual)
- **[WINDOWS_TEST_SUCCESS.md](WINDOWS_TEST_SUCCESS.md)** - Windows deployment verification results

### Advanced Topics
- **[SECURITY_AUDIT_REPORT.md](SECURITY_AUDIT_REPORT.md)** - Security analysis
- **[TOKENOMICS_FULL_REPORT.md](TOKENOMICS_FULL_REPORT.md)** - Economic model
- **[TREASURY_MULTISIG_GUIDE.md](TREASURY_MULTISIG_GUIDE.md)** - Treasury management

---

## Technical Specifications

### Chain Configuration
- **Chain ID**: `omniphi-1` (production) / `omniphi-test` (testing)
- **Denomination**: `uomni` (micro-OMNI, 1 OMNI = 1,000,000 uomni)
- **Consensus**: CometBFT (Tendermint)
- **Block Time**: ~5 seconds
- **Finality**: Instant (single-block finality)

### System Requirements
- **Go**: 1.23+
- **OS**: Ubuntu 20.04+, Windows 11, macOS
- **RAM**: 4GB minimum, 8GB recommended
- **Disk**: 100GB SSD
- **Network**: 100Mbps+

### Key Commands
```bash
# Start chain
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false

# Check status
./posd status

# Query modules
./posd query feemarket base-fee
./posd query tokenomics supply
./posd query poc params

# Send transaction
./posd tx bank send <from> <to> 1000uomni --fees 100uomni
```

---

## Development

### Build from Source

```bash
# Clone repository
git clone https://github.com/NeomSense/PoS-PoC.git
cd PoS-PoC

# Build binary
go build -o posd ./cmd/posd

# Verify
./posd version
```

### Project Structure

```
.
├── app/                    # Application configuration
│   └── app.go             # Main app setup with depinject
├── cmd/posd/              # CLI binary
├── proto/                 # Protobuf definitions
├── x/                     # Custom modules
│   ├── feemarket/        # Adaptive fee market
│   ├── tokenomics/       # Token economics
│   └── poc/              # Proof of contribution
├── setup_ubuntu_fixed.sh  # Ubuntu automated setup
├── test_windows.sh        # Windows test script
└── fix_ubuntu.sh          # Diagnostic tool
```

---

## Contributing

This is a production blockchain. For contributions:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly (both Ubuntu and Windows)
5. Submit a pull request

---

## Support

### Getting Help
1. Check [UBUNTU_DEPLOYMENT_GUIDE.md](UBUNTU_DEPLOYMENT_GUIDE.md) for Ubuntu issues
2. Run `./fix_ubuntu.sh` for diagnostics
3. Review [DEPLOYMENT_SUCCESS.md](DEPLOYMENT_SUCCESS.md) for known issues

### Common Issues

| Issue | Solution |
|-------|----------|
| "validator set is empty" | Run `./setup_ubuntu_fixed.sh` and answer 'Y' to clean data |
| "cannot execute file not found" | Fixed with `.gitattributes` - run `git pull` |
| "reflection service error" | Use `--grpc.enable=false` flag |

---

## License

[Add your license here]

---

## Repository

**GitHub**: [https://github.com/NeomSense/PoS-PoC](https://github.com/NeomSense/PoS-PoC)

---

*Built with ❤️ using Cosmos SDK v0.53.3*
