# Omniphi Testnet V1 - Comprehensive Test Suite

**Chain ID**: `omniphi-testnet-1`
**Test Date**: 2026-01-01
**Validators**: 2 (VPS1 + VPS2)

## Test Infrastructure

| Node | IP | Moniker | Stake |
|------|-----|---------|-------|
| VPS1 | 46.202.179.182 | omniphi-node | 100M OMNI |
| VPS2 | 148.230.125.9 | omniphi-node-2 | 50M OMNI |

## Test Categories

| # | Category | Script | Status |
|---|----------|--------|--------|
| 1 | [Chain Stability](01-chain-stability.md) | `01-chain-stability.sh` | Pending |
| 2 | [Block Production](02-block-production.md) | `02-block-production.sh` | Pending |
| 3 | [Mempool & Transactions](03-mempool-transactions.md) | `03-mempool-transactions.sh` | Pending |
| 4 | [Fee Market & Burn](04-feemarket-burn.md) | `04-feemarket-burn.sh` | Pending |
| 5 | [PoC Module](05-poc-module.md) | `05-poc-module.sh` | Pending |
| 6 | [Network Health](06-network-health.md) | `06-network-health.sh` | Pending |
| 7 | [Data Consistency](07-data-consistency.md) | `07-data-consistency.sh` | Pending |
| 8 | [Security & Edge Cases](08-security-edge-cases.md) | `08-security-edge-cases.sh` | Pending |

## Quick Start

```bash
# Run all tests from VPS1
cd ~/testnet-v1-tests
chmod +x *.sh
./run-all-tests.sh

# Run individual test
./01-chain-stability.sh
```

## Pass/Fail Criteria

- **PASS**: Test completes successfully with expected output
- **WARN**: Test completes but with minor deviations
- **FAIL**: Test fails or produces unexpected results

## Final Report

After all tests complete, see [FINAL-REPORT.md](FINAL-REPORT.md) for:
- Overall readiness score (0-100)
- Required patches before Phase 2
- Recommendations
