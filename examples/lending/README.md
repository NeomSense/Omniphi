# Omniphi Intent-Based Lending Protocol

A reference lending protocol built on Omniphi's intent-based execution model. Users declare lending and borrowing intents; the solver network handles pool selection, rate optimization, and cross-pool composability.

## Overview

Traditional DeFi lending (Aave, Compound) requires users to manually select pools, approve tokens, and execute multi-step transactions. Each step is a separate on-chain transaction that can fail independently, leaving users in partially executed states.

Omniphi's intent-based lending collapses these multi-step flows into single atomic operations:

| Operation | Traditional (Aave) | Omniphi Intents |
|---|---|---|
| **Supply** | approve + supply (2 tx) | 1 atomic intent |
| **Borrow** | approve + deposit collateral + borrow (3 tx) | 1 atomic intent |
| **Leveraged Farm** | approve + deposit + borrow + approve + swap + deposit (6+ tx) | 1 atomic intent bundle |
| **Pool Selection** | Manual research | Solver finds best rate |
| **Rebalancing** | Manual monitoring + tx | Solver auto-optimizes |

## How to Run

```bash
chmod +x run.sh
./run.sh

# Or manually:
npm install
npx ts-node src/index.ts
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `OMNIPHI_RPC` | `http://localhost:26657` | Tendermint RPC endpoint |
| `OMNIPHI_REST` | `http://localhost:1317` | REST / gRPC-gateway endpoint |

## Architecture

```
  User declares intent              Solver Network optimizes
  +------------------+          +-------------------------+
  | "Supply 10K USDC |          | Analyze all pools:      |
  |  for best yield" |   --->   |  Pool Prime: 4.2% APY   |
  +------------------+          |  Pool Stable: 4.5% APY  |
                                |  Pool Gamma: 3.9% APY   |
                                |                         |
                                | Optimal split:          |
                                |  60% -> Prime            |
                                |  40% -> Stable           |
                                |  Blended: 4.32% APY     |
                                +------------+------------+
                                             |
                                  Atomic settlement
                                             |
                                  +----------v---------+
                                  | User receives:     |
                                  | - Yield tokens     |
                                  | - 4.32% blended APY|
                                  | - Auto-rebalancing |
                                  +--------------------+
```

### Interest Rate Model

Markets use a kinked interest rate curve based on utilization:

```
  Rate
   ^
   |                    ___________
   |                 __/
   |              __/
   |           __/    <- kink point (optimal utilization)
   |        __/
   |     __/
   |  __/
   | /
   +------------------------------------> Utilization
   0%        optimal       100%
```

Below optimal utilization, rates increase gradually to attract borrowers. Above the kink point, rates increase steeply to attract more suppliers and discourage excessive borrowing.

## API Reference

### `OmniphiLending.connect(mnemonic)`

Creates a signing lending client.

### `supplyIntent(denom, amount)`

Submits a supply intent. The solver finds the best yield across all pools and may split the deposit for optimal returns.

### `borrowIntent(denom, amount, collateral)`

Submits a borrow intent with collateral locking in a single atomic operation. Pre-flight checks ensure the health factor is safe.

### `repayIntent(denom, amount)`

Repays borrowed assets. The solver calculates exact interest owed and handles any excess.

### `withdrawIntent(denom, amount)`

Withdraws supplied assets plus accrued yield. Safety checks prevent withdrawals that would make borrow positions unhealthy.

### `queryMarket(denom)`

Returns real-time market data: supply APY, borrow APR, utilization, collateral factors, and oracle prices.

### `getAccountHealth(address)`

Returns comprehensive account health: collateral value, borrow value, health factor, and per-asset positions.

### `leveragedYieldFarm(supplyDenom, supplyAmount, borrowDenom, yieldDenom, leverageRatio)`

Executes a multi-step leveraged yield farming strategy as a single atomic intent bundle.

## Why Intents Are Better for Lending

### 1. Atomic Multi-Step Operations

Depositing collateral and borrowing against it should be one operation, not three separate transactions. If the borrow step fails after collateral is locked, the user's funds are stuck until they submit another transaction. Intent bundles eliminate this class of failures.

### 2. Cross-Pool Rate Optimization

No human can efficiently monitor and rebalance across dozens of lending pools in real-time. Solver networks can. Supply intents let solvers automatically split deposits across the highest-yielding pools and rebalance as rates change.

### 3. No Token Approvals

ERC-20 approvals are a security anti-pattern. Unlimited approvals expose users to contract exploits. Omniphi's capability-based model (described in the escrow example) eliminates approvals entirely.

### 4. Composable DeFi Primitives

Leveraged yield farming, flash refinancing, and cross-collateral borrowing all become single atomic operations. Users define the strategy; solvers handle the multi-step execution.

## Risk Parameters

| Parameter | Value | Description |
|---|---|---|
| Min Health Factor | 1.1 | Minimum allowed after borrow/withdraw |
| Max LTV | 0.75 | Maximum loan-to-value ratio |
| Liquidation Penalty | 5% | Penalty applied during liquidation |
| Oracle Update | Every block | Price feeds updated each block |

## Chain Configuration

- **Bech32 prefix**: `omni`
- **Native denom**: `omniphi`
- **RPC port**: 26657
- **REST port**: 1317
- **Lending module**: `/pos.contracts.v1.MsgSubmitLendingIntent`

## Dependencies

- `@omniphi/sdk` -- Omniphi TypeScript SDK
- `ts-node` -- TypeScript execution runtime
