# Omniphi Intent-Based DEX

A reference decentralized exchange built on Omniphi's intent-based execution model. Users declare trade outcomes as intents; a competitive solver network handles routing, price optimization, and MEV protection.

## Overview

Traditional DEXes (Uniswap, Osmosis, etc.) require users to construct exact swap instructions: pick a pool, calculate expected output, set slippage, and hope a front-runner does not sandwich the transaction. This couples the user to implementation details they should not need to know.

Omniphi's intent-based DEX inverts the model:

| Aspect | Traditional DEX | Omniphi Intent DEX |
|---|---|---|
| **User specifies** | Pool ID, route, exact amounts | Desired outcome and constraints |
| **Routing** | User must find best route | Solver network finds optimal path |
| **MEV protection** | None (sandwich attacks common) | Built-in (solvers compete for best fill) |
| **Multi-hop** | Multiple transactions | Single atomic intent |
| **Failure mode** | Partial execution possible | All-or-nothing atomic settlement |

## How to Run

```bash
# From this directory:
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
                              Omniphi Chain
                         +-------------------+
                         |                   |
  User submits intent:   |  Intent Mempool   |
  "Swap 1000 OMNI for    |  +-------------+  |
   as much USDC as       |  | SwapIntent  |  |
   possible, max 0.5%    |  | sender: ... |  |
   slippage"             |  | input: OMNI |  |
                         |  | output: USDC|  |
                         |  | slippage:50 |  |
                         |  +------+------+  |
                         |         |         |
                         |         v         |
                         |  Solver Network   |
                         |  +------+------+  |
                         |  | Solver A    |  |  fills at 2.51 USDC/OMNI
                         |  | Solver B    |  |  fills at 2.52 USDC/OMNI  <-- winner
                         |  | Solver C    |  |  fills at 2.49 USDC/OMNI
                         |  +------+------+  |
                         |         |         |
                         |         v         |
                         |  Settlement       |
                         |  (atomic, best    |
                         |   fill wins)      |
                         +-------------------+
```

### Solver Competition

Solvers are off-chain agents that monitor the intent mempool and submit fill proposals. The chain selects the best fill for each intent based on:

1. **Output amount** -- higher is better for the user
2. **Gas efficiency** -- lower gas cost means more surplus for the user
3. **Execution certainty** -- solvers that can guarantee fills are preferred

Solvers earn fees from the spread between their fill price and the user's minimum acceptable output.

## API Reference

### `OmniphiDex.connect(mnemonic: string)`

Creates a signing DEX client connected to the Omniphi chain.

### `createSwapIntent(inputDenom, outputDenom, inputAmount, maxSlippageBps?)`

Submits a swap intent. The solver network finds the best execution path.

- `inputDenom` -- Token to sell (e.g., `"omniphi"`)
- `outputDenom` -- Token to buy (e.g., `"ibc/usdc"`)
- `inputAmount` -- Amount in base units
- `maxSlippageBps` -- Maximum slippage in basis points (default: 50 = 0.5%)

### `queryPool(poolId: number)`

Returns pool details: reserves, fee rate, share token supply.

### `getPrice(denomA, denomB)`

Returns the aggregated price across all liquidity sources, including the optimal route.

### `addLiquidity(poolId, amountA, amountB)`

Adds liquidity to a pool using an atomic intent bundle. The solver layer handles ratio optimization.

### `submitLimitOrder(inputDenom, outputDenom, inputAmount, limitPrice, expirySeconds?)`

Submits a limit order as a swap intent with a specific minimum price and deadline. Solvers fill it when market conditions match.

### `multiLegSwap(legs)`

Executes a multi-leg swap atomically. All legs succeed or all revert.

## Why Intents Are Better for DEXes

### 1. MEV Protection

On Ethereum, sandwich attacks extract value from every swap. The attacker front-runs your transaction, moves the price, and back-runs to capture the difference. With intent-based execution, solvers compete on fill quality -- there is no public mempool to front-run.

### 2. Optimal Routing Without User Effort

Users should not need to know which pools have the best liquidity or which multi-hop path gives the best price. Intent-based execution delegates this to specialized solvers who can analyze all liquidity sources in real-time.

### 3. Atomic Multi-Asset Operations

Rebalancing a portfolio across 5 assets on a traditional DEX requires 5 separate transactions, each of which might fail independently. On Omniphi, you submit a single intent bundle that executes atomically.

### 4. Limit Orders Without Infrastructure

Traditional DEX limit orders require either a centralized order book server or a complex on-chain keeper network. On Omniphi, a limit order is just a swap intent with a deadline and a minimum output -- the standard solver network handles it.

## Chain Configuration

- **Bech32 prefix**: `omni`
- **Native denom**: `omniphi`
- **RPC port**: 26657
- **REST port**: 1317
- **Swap message**: `/pos.contracts.v1.MsgSubmitSwapIntent`

## Dependencies

- `@omniphi/sdk` -- Omniphi TypeScript SDK
- `ts-node` -- TypeScript execution runtime
