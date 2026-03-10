# Guard Module E2E Tests

End-to-end scenario tests for the `x/guard` governance safeguard module.

These scripts exercise the full guard pipeline against a running localnet, verifying risk evaluation, gate state machine transitions, execution confirmation, stability checks, and veto handling.

## Prerequisites

- **`posd` binary** — built and in `$PATH` (`cd chain && make build && export PATH=$PWD/build:$PATH`)
- **`jq`** — JSON processor (`apt install jq` / `brew install jq`)
- **Running localnet** — single-node testnet initialized via `scripts/init_testnet.sh`
- **Funded validator key** — the `validator` key must have sufficient OMNI for deposits and fees

## Quick Start

```bash
# 1. Build the binary
cd chain && make build

# 2. Initialize and start localnet (if not already running)
./scripts/init_testnet.sh validator1
posd start &

# 3. Wait for the node to produce blocks (~10 seconds)
sleep 15

# 4. Run all guard E2E tests
./scripts/e2e/guard/run_all.sh
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `POSD_NODE` | `tcp://localhost:26657` | RPC endpoint |
| `POSD_HOME` | `$HOME/.pos` | Chain home directory |
| `POSD_CHAIN_ID` | `omniphi-testnet-2` | Chain ID |
| `KEYRING_BACKEND` | `test` | Keyring backend |
| `FEES` | `100000omniphi` | Default tx fees |
| `GAS` | `500000` | Default gas limit |
| `BOND_DENOM` | `omniphi` | Bond denomination |
| `VALIDATOR_KEY` | `validator` | Key name for the validator |
| `BLOCK_TIME` | `4` | Expected seconds per block |
| `GUARD_E2E_SHORT_DELAYS` | `1` | Set guard delays to a few blocks for fast testing |

## Scenarios

| # | Script | What it Tests |
|---|---|---|
| 01 | `01_software_upgrade_critical.sh` | MsgSoftwareUpgrade → CRITICAL tier, max delay, second confirm required |
| 02 | `02_community_pool_spend_treasury_bps.sh` | MsgCommunityPoolSpend → treasury spend BPS computation, tier ordering |
| 03 | `03_gate_transitions.sh` | Full gate state machine: VISIBILITY → SHOCK_ABSORBER → CONDITIONAL → READY |
| 04 | `04_confirm_execution_critical.sh` | CRITICAL proposal stays at READY until MsgConfirmExecution, then EXECUTED |
| 05 | `05_stability_extension_validator_churn.sh` | Validator power churn during CONDITIONAL → earliest_exec_height extension |
| 06 | `06_emergency_veto_abort.sh` | NO_WITH_VETO vote → proposal rejected/aborted |

## Running a Single Scenario

```bash
# Run just scenario 01
bash scripts/e2e/guard/01_software_upgrade_critical.sh

# Run with custom config
POSD_NODE=tcp://192.168.1.100:26657 \
POSD_CHAIN_ID=omniphi-mainnet \
bash scripts/e2e/guard/03_gate_transitions.sh
```

## How It Works

1. **`run_all.sh`** optionally submits a governance proposal to set guard delays to a few blocks (so tests complete in minutes instead of days)
2. Each scenario submits governance proposals, votes on them, and polls guard queries until expected states are observed
3. Assertions verify risk tier, score, delay, gate state, and status notes
4. All scripts time out safely (no infinite loops) and print clear PASS/FAIL

## Debugging Failures

**"Node not reachable"** — Start the localnet: `posd start`

**"Proposal did not pass"** — The validator key needs sufficient balance for deposits (10K OMNI = 10,000,000,000 omniphi). Check: `posd query bank balances $(posd keys show validator -a --keyring-backend test)`

**"No risk report found"** — Guard's EndBlocker may not have processed the proposal yet. Increase `BLOCK_TIME` or check that `x/guard` is registered in `app.go`.

**"Timed out waiting for height"** — The node may have stalled. Check: `posd status`

**"Gate did not reach READY"** — With default params, gate transitions take days. Set `GUARD_E2E_SHORT_DELAYS=1` (default) to use a few-block delays.

**Scenario 05 skipped** — Stability extension requires causing validator power churn. On single-validator localnet, a `user1` key is needed. The script attempts to create and fund one automatically.

## Architecture

```
scripts/e2e/guard/
├── lib.sh                              # Shared helpers (polling, assertions, CLI wrappers)
├── run_all.sh                          # Test runner (executes all scenarios in order)
├── fixtures/                           # Proposal JSON templates
│   ├── software_upgrade.json
│   ├── community_pool_spend_large.json
│   ├── community_pool_spend_small.json
│   ├── param_change_staking.json
│   └── text_only.json
├── 01_software_upgrade_critical.sh
├── 02_community_pool_spend_treasury_bps.sh
├── 03_gate_transitions.sh
├── 04_confirm_execution_critical.sh
├── 05_stability_extension_validator_churn.sh
├── 06_emergency_veto_abort.sh
└── README.md
```
