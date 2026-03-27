# Omniphi Developer Quickstart

Build your first intent on Omniphi in under 10 minutes.

## Prerequisites

- Node.js 18+ (for TypeScript SDK)
- Rust 1.75+ (for contract toolkit)
- Go 1.22+ (for running a node)

## 1. Install the SDK

```bash
cd sdk/typescript
npm install
npm run build
```

```typescript
import { OmniphiClient, createWallet } from '@omniphi/sdk';

// Create a wallet
const wallet = await createWallet();
console.log('Address:', wallet.address);
console.log('Mnemonic:', wallet.mnemonic);
```

## 2. Connect to Testnet

```typescript
const client = await OmniphiClient.connect('http://localhost:26657');

// Check balance
const balance = await client.getBalance(wallet.address, 'omniphi');
console.log('Balance:', balance);
```

## 3. Send Your First Transfer

```typescript
const result = await client.sendTokens(
  wallet.address,
  'omni1recipient...',
  '1000000',  // 1 OMNI (6 decimals)
  'omniphi',
);
console.log('Tx hash:', result.transactionHash);
```

## 4. Submit an Intent

Intents are Omniphi's native transaction type. Instead of specifying exact execution steps, you declare what you want and solvers compete to fulfill it.

```typescript
const intent = {
  type: 'transfer',
  asset: 'omniphi',
  amount: '1000000',
  recipient: 'omni1recipient...',
  maxFee: '10000',
  deadline: Math.floor(Date.now() / 1000) + 300, // 5 min
};

const result = await client.submitIntent(wallet.address, intent);
console.log('Intent submitted:', result.transactionHash);
```

## 5. Build a Contract

```bash
# Install the contract toolkit
cd contract-toolkit
cargo build --release

# Create a new contract project
./target/release/omniphi-contracts init my-counter

# Edit the contract schema
cat my-counter/contract.yaml
```

```yaml
name: my-counter
version: "1.0.0"
description: A simple counter contract

state:
  - name: count
    type: Uint128
    default: "0"

intents:
  - name: increment
    params:
      - name: amount
        type: Uint128
        required: true
    preconditions: []
    postconditions:
      - type: Custom
        params:
          rule: "count_after == count_before + amount"

  - name: decrement
    params:
      - name: amount
        type: Uint128
        required: true
    preconditions:
      - type: BalanceCheck
        params:
          field: count
          min: amount
    postconditions: []
```

```bash
# Build and validate
./target/release/omniphi-contracts build
./target/release/omniphi-contracts validate
./target/release/omniphi-contracts test
```

## 6. Query Chain State

```typescript
// Get validators
const validators = await client.getValidators();
console.log('Active validators:', validators.length);

// Delegate stake
await client.delegate(
  wallet.address,
  validators[0].operatorAddress,
  '100000000',  // 100 OMNI
);
```

## Architecture Overview

```
                    User / Wallet / dApp
                           |
                    [Intent Submission]
                           |
                    PoSeq (Sequencer)
                    - Orders intents
                    - Anti-front-running
                           |
                    Runtime (Execution)
                    - Solver market
                    - Plan validation
                    - Parallel scheduling
                    - Atomic settlement
                           |
                    Go Chain (Consensus)
                    - PoS + PoC validation
                    - State commitment
                    - IBC bridging
```

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Intent** | A declaration of desired outcome, not exact execution steps |
| **Solver** | A third party that competes to fulfill intents optimally |
| **PoC** | Proof of Contribution — earn rewards for useful work |
| **PoSeq** | Proof of Sequencing — fair ordering, anti-MEV |
| **Capability** | Scoped, time-bound spending permission (replaces approvals) |
| **Sponsorship** | Third party pays gas on your behalf |

## Next Steps

- [Full SDK Reference](../sdk/typescript/README.md)
- [Contract Development Guide](../contract-toolkit/README.md)
- [Bridge Guide](../bridge/ethereum/README.md)
- [Tokenomics Model](../simulations/tokenomics/README.md)
- [Testnet Operations](../testnet/README.md)
