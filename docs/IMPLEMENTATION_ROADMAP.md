# Omniphi Dual-Chain Implementation Roadmap

## Overview

This document outlines the implementation steps to build the Omniphi dual-chain ecosystem:
1. **Core Chain** enhancements (bridge + anchor modules)
2. **PoSeQ Chain** development (EVM execution layer)
3. **Bridge Infrastructure** (relayers + contracts)
4. **Wallet & UI** (unified experience)

---

## Phase 1: Core Chain Enhancements

### 1.1 Bridge Module (`x/bridge`)

**Purpose**: Lock/release OMNI for cross-chain transfers

**Files to Create**:
```
chain/x/bridge/
├── keeper/
│   ├── keeper.go           # Core keeper logic
│   ├── deposit.go          # Lock OMNI for PoSeQ
│   ├── withdraw.go         # Release OMNI from PoSeQ
│   ├── relayer.go          # Relayer management
│   ├── escrow.go           # Escrow account management
│   └── genesis.go          # Genesis state
├── types/
│   ├── keys.go             # Store keys
│   ├── params.go           # Module parameters
│   ├── params.pb.go        # Protobuf params
│   ├── tx.pb.go            # Transaction messages
│   ├── query.pb.go         # Query messages
│   ├── events.go           # Event definitions
│   └── errors.go           # Error codes
├── module/
│   ├── module.go           # Module definition
│   └── depinject.go        # Dependency injection
└── client/
    └── cli/
        ├── tx.go           # CLI transaction commands
        └── query.go        # CLI query commands
```

**Key Messages**:
```protobuf
// MsgBridgeDeposit locks OMNI and initiates transfer to PoSeQ
message MsgBridgeDeposit {
  string sender = 1;           // omni1... address
  string recipient = 2;        // 0x... address on PoSeQ
  cosmos.base.v1beta1.Coin amount = 3;
}

// MsgBridgeWithdraw releases OMNI after PoSeQ withdrawal
message MsgBridgeWithdraw {
  string sender = 1;           // Relayer submitting proof
  string recipient = 2;        // omni1... address
  cosmos.base.v1beta1.Coin amount = 3;
  bytes proof = 4;             // Multi-sig attestation
  uint64 poseq_block = 5;      // PoSeQ block number
  bytes poseq_tx_hash = 6;     // PoSeQ transaction hash
}

// MsgRegisterRelayer adds a new bridge relayer
message MsgRegisterRelayer {
  string authority = 1;        // Governance address
  string relayer_address = 2;  // Relayer's omni1... address
  bytes signing_key = 3;       // Relayer's signing public key
}
```

**Parameters**:
```go
type BridgeParams struct {
    // Minimum deposit amount
    MinDepositAmount math.Int
    // Withdrawal finality delay (blocks)
    WithdrawalDelay uint64
    // Required relayer signatures (e.g., 2/3)
    SignatureThreshold uint64
    // Total relayers
    TotalRelayers uint64
    // Bridge fee (percentage)
    BridgeFeeRate math.LegacyDec
    // Max pending deposits
    MaxPendingDeposits uint64
}
```

### 1.2 Anchor Module (`x/anchor`)

**Purpose**: Verify and store PoSeQ checkpoints

**Files to Create**:
```
chain/x/anchor/
├── keeper/
│   ├── keeper.go           # Core keeper logic
│   ├── checkpoint.go       # Checkpoint verification
│   ├── dispute.go          # Fraud proof handling
│   ├── sequencer.go        # Sequencer management
│   └── genesis.go          # Genesis state
├── types/
│   ├── keys.go             # Store keys
│   ├── params.go           # Module parameters
│   ├── checkpoint.pb.go    # Checkpoint structure
│   ├── tx.pb.go            # Transaction messages
│   ├── query.pb.go         # Query messages
│   └── errors.go           # Error codes
├── module/
│   ├── module.go           # Module definition
│   └── depinject.go        # Dependency injection
└── client/
    └── cli/
        ├── tx.go           # CLI commands
        └── query.go        # CLI queries
```

**Key Messages**:
```protobuf
// MsgSubmitCheckpoint submits PoSeQ state root
message MsgSubmitCheckpoint {
  string sequencer = 1;        // Sequencer address
  uint64 poseq_block = 2;      // PoSeQ block number
  bytes state_root = 3;        // State trie root
  bytes transactions_root = 4; // Transactions trie root
  bytes receipts_root = 5;     // Receipts trie root
  uint64 timestamp = 6;        // Block timestamp
  bytes signature = 7;         // Sequencer signature
}

// MsgDisputeCheckpoint challenges an invalid checkpoint
message MsgDisputeCheckpoint {
  string challenger = 1;       // Challenger address
  uint64 checkpoint_id = 2;    // Checkpoint to dispute
  bytes fraud_proof = 3;       // Proof of invalidity
}

// MsgRegisterSequencer adds a sequencer
message MsgRegisterSequencer {
  string authority = 1;        // Governance address
  string sequencer_address = 2;
  bytes signing_key = 3;
  cosmos.base.v1beta1.Coin stake = 4;
}
```

**Checkpoint Storage**:
```go
type Checkpoint struct {
    ID               uint64
    PoSeQBlock       uint64
    StateRoot        []byte
    TransactionsRoot []byte
    ReceiptsRoot     []byte
    Timestamp        uint64
    Sequencer        string
    Status           CheckpointStatus // Pending, Finalized, Disputed
    SubmittedAt      int64  // Core block height
    FinalizedAt      int64  // When finality achieved
}

type CheckpointStatus int32
const (
    CheckpointPending   CheckpointStatus = 0
    CheckpointFinalized CheckpointStatus = 1
    CheckpointDisputed  CheckpointStatus = 2
    CheckpointInvalid   CheckpointStatus = 3
)
```

### 1.3 Integration with Existing Modules

**Tokenomics Integration**:
```go
// In x/tokenomics/keeper/fee_collector.go
// Add bridge fee tracking

func (k Keeper) ProcessBridgeFees(ctx context.Context, fees sdk.Coins) error {
    // Apply standard burn logic to bridge fees
    burnCalc := k.feemarketKeeper.ComputeEffectiveBurn(ctx, fees.AmountOf(types.BondDenom), types.ActivityBridge)

    // Burn portion
    if err := k.BurnTokens(ctx, burnCalc.BurnAmount); err != nil {
        return err
    }

    // Treasury portion
    if err := k.SendToTreasury(ctx, burnCalc.TreasuryAmount); err != nil {
        return err
    }

    return nil
}
```

**FeeMarket Integration**:
```go
// Add bridge activity type
const (
    ActivityBridge ActivityType = "bridge"
)

// In params_extra.go, add multiplier
MultiplierBridge: math.LegacyMustNewDecFromStr("0.75"), // 0.75x multiplier
```

---

## Phase 2: PoSeQ Chain Development

### 2.1 Chain Framework Options

| Option | Pros | Cons |
|--------|------|------|
| **go-ethereum fork** | Full EVM compatibility, proven | Complex consensus swap |
| **OP Stack** | Rollup-ready, battle-tested | Optimism dependency |
| **Polygon CDK** | ZK-ready, modular | Polygon ecosystem |
| **Custom EVM** | Full control | High development cost |

**Recommendation**: Start with **go-ethereum fork** for simplicity, migrate to OP Stack or ZK later if needed.

### 2.2 PoSeQ Directory Structure

```
poseq/
├── cmd/
│   └── poseqd/
│       └── main.go         # Node entrypoint
├── consensus/
│   ├── poseq.go            # PoSeQ consensus engine
│   ├── sequencer.go        # Sequencer selection
│   └── checkpoint.go       # Checkpoint submission
├── bridge/
│   ├── contract/
│   │   └── Bridge.sol      # Bridge contract (Solidity)
│   ├── relayer.go          # Bridge relay logic
│   └── events.go           # Event parsing
├── core/
│   ├── genesis.go          # Genesis configuration
│   ├── chain.go            # Chain configuration
│   └── params.go           # EVM parameters
├── rpc/
│   ├── eth_api.go          # eth_* RPC methods
│   └── omniphi_api.go      # omniphi_* custom methods
└── config/
    ├── config.toml         # Node configuration
    └── genesis.json        # Genesis file
```

### 2.3 Bridge Contract (Solidity)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title OmniphiBridge
 * @notice Bridge contract for OMNI transfers between Core and PoSeQ
 */
contract OmniphiBridge is ERC20, AccessControl, ReentrancyGuard {
    bytes32 public constant RELAYER_ROLE = keccak256("RELAYER_ROLE");

    // Withdrawal tracking
    mapping(bytes32 => bool) public processedWithdrawals;

    // Events
    event Deposit(address indexed from, string indexed coreRecipient, uint256 amount);
    event Withdrawal(address indexed to, uint256 amount, bytes32 indexed txHash);

    constructor() ERC20("Wrapped OMNI", "wOMNI") {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    /**
     * @notice Mint wOMNI when deposit is confirmed from Core
     * @param to Recipient address on PoSeQ
     * @param amount Amount to mint
     * @param coreTxHash Core chain transaction hash
     * @param signatures Relayer signatures
     */
    function mint(
        address to,
        uint256 amount,
        bytes32 coreTxHash,
        bytes[] calldata signatures
    ) external nonReentrant {
        require(!processedWithdrawals[coreTxHash], "Already processed");
        require(_verifySignatures(coreTxHash, amount, to, signatures), "Invalid signatures");

        processedWithdrawals[coreTxHash] = true;
        _mint(to, amount);

        emit Withdrawal(to, amount, coreTxHash);
    }

    /**
     * @notice Burn wOMNI to withdraw to Core chain
     * @param coreRecipient Recipient address on Core (omni1...)
     * @param amount Amount to withdraw
     */
    function withdraw(string calldata coreRecipient, uint256 amount) external nonReentrant {
        require(bytes(coreRecipient).length > 0, "Invalid recipient");
        require(amount > 0, "Amount must be positive");

        _burn(msg.sender, amount);

        emit Deposit(msg.sender, coreRecipient, amount);
    }

    /**
     * @notice Verify relayer signatures meet threshold
     */
    function _verifySignatures(
        bytes32 txHash,
        uint256 amount,
        address to,
        bytes[] calldata signatures
    ) internal view returns (bool) {
        bytes32 message = keccak256(abi.encodePacked(txHash, amount, to));
        uint256 validSignatures = 0;

        for (uint i = 0; i < signatures.length; i++) {
            address signer = _recoverSigner(message, signatures[i]);
            if (hasRole(RELAYER_ROLE, signer)) {
                validSignatures++;
            }
        }

        // Require 2/3 threshold
        return validSignatures * 3 >= signatures.length * 2;
    }

    function _recoverSigner(bytes32 message, bytes memory signature) internal pure returns (address) {
        bytes32 ethSignedMessage = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", message));
        (bytes32 r, bytes32 s, uint8 v) = _splitSignature(signature);
        return ecrecover(ethSignedMessage, v, r, s);
    }

    function _splitSignature(bytes memory sig) internal pure returns (bytes32 r, bytes32 s, uint8 v) {
        require(sig.length == 65, "Invalid signature length");
        assembly {
            r := mload(add(sig, 32))
            s := mload(add(sig, 64))
            v := byte(0, mload(add(sig, 96)))
        }
    }
}
```

### 2.4 PoSeQ Consensus

```go
// poseq/consensus/poseq.go

package consensus

import (
    "github.com/ethereum/go-ethereum/consensus"
    "github.com/ethereum/go-ethereum/core/types"
)

// PoSeQ implements the Proof of Sequenced Execution consensus
type PoSeQ struct {
    config      *Config
    sequencers  []Sequencer
    currentSeq  int
    checkpoint  *CheckpointManager
}

// Config holds PoSeQ consensus parameters
type Config struct {
    BlockTime        uint64 // Target block time in seconds
    CheckpointFreq   uint64 // Checkpoint every N blocks
    SequencerRotation uint64 // Rotate sequencer every N blocks
    CoreChainRPC     string // Core chain RPC endpoint
}

// Sequencer represents a block producer
type Sequencer struct {
    Address    common.Address
    SigningKey *ecdsa.PublicKey
    Stake      *big.Int
    Active     bool
}

// Author returns the address of the sequencer that produced the block
func (p *PoSeQ) Author(header *types.Header) (common.Address, error) {
    return header.Coinbase, nil
}

// Seal generates a new block for the given input block
func (p *PoSeQ) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
    // Only active sequencer can seal
    if !p.isActiveSequencer() {
        return errNotSequencer
    }

    header := block.Header()

    // Sign the block
    sig, err := p.signBlock(header)
    if err != nil {
        return err
    }
    header.Extra = append(header.Extra, sig...)

    // Submit checkpoint if needed
    if header.Number.Uint64() % p.config.CheckpointFreq == 0 {
        go p.submitCheckpoint(block)
    }

    results <- block.WithSeal(header)
    return nil
}

// submitCheckpoint sends checkpoint to Core chain
func (p *PoSeQ) submitCheckpoint(block *types.Block) error {
    checkpoint := &Checkpoint{
        BlockNumber:      block.NumberU64(),
        StateRoot:        block.Root(),
        TransactionsRoot: block.TxHash(),
        ReceiptsRoot:     block.ReceiptHash(),
        Timestamp:        block.Time(),
    }

    return p.checkpoint.Submit(checkpoint)
}
```

---

## Phase 3: Bridge Infrastructure

### 3.1 Relayer Service

```go
// bridge/relayer/main.go

package main

import (
    "context"
    "log"

    "github.com/omniphi/bridge/core"
    "github.com/omniphi/bridge/poseq"
)

type Relayer struct {
    coreClient  *core.Client
    poseqClient *poseq.Client
    signer      *Signer
    db          *Database
}

func (r *Relayer) Start(ctx context.Context) error {
    // Watch Core chain for deposits
    go r.watchCoreDeposits(ctx)

    // Watch PoSeQ chain for withdrawals
    go r.watchPoSeQWithdrawals(ctx)

    // Process pending attestations
    go r.processAttestations(ctx)

    return nil
}

func (r *Relayer) watchCoreDeposits(ctx context.Context) {
    events := r.coreClient.SubscribeEvents("bridge.deposit")

    for {
        select {
        case event := <-events:
            deposit := parseDepositEvent(event)

            // Sign attestation
            sig, err := r.signer.Sign(deposit.Hash())
            if err != nil {
                log.Printf("Failed to sign: %v", err)
                continue
            }

            // Store attestation
            r.db.StoreAttestation(deposit.Hash(), sig)

            // Check if threshold met
            if r.checkThreshold(deposit.Hash()) {
                r.submitToPoSeQ(deposit)
            }

        case <-ctx.Done():
            return
        }
    }
}

func (r *Relayer) watchPoSeQWithdrawals(ctx context.Context) {
    events := r.poseqClient.SubscribeEvents("Deposit") // Withdrawal emits Deposit event

    for {
        select {
        case event := <-events:
            withdrawal := parseWithdrawalEvent(event)

            // Sign attestation
            sig, err := r.signer.Sign(withdrawal.Hash())
            if err != nil {
                log.Printf("Failed to sign: %v", err)
                continue
            }

            // Store attestation
            r.db.StoreAttestation(withdrawal.Hash(), sig)

            // Check if threshold met
            if r.checkThreshold(withdrawal.Hash()) {
                r.submitToCore(withdrawal)
            }

        case <-ctx.Done():
            return
        }
    }
}
```

### 3.2 Relayer Deployment

```yaml
# docker-compose.yml for relayer network
version: '3.8'

services:
  relayer-1:
    image: omniphi/bridge-relayer:latest
    environment:
      - CORE_RPC=https://core.omniphi.network:26657
      - POSEQ_RPC=https://poseq.omniphi.network:8545
      - SIGNER_KEY_FILE=/keys/relayer1.key
    volumes:
      - ./keys/relayer1:/keys:ro
      - relayer1-db:/data
    restart: unless-stopped

  relayer-2:
    image: omniphi/bridge-relayer:latest
    environment:
      - CORE_RPC=https://core.omniphi.network:26657
      - POSEQ_RPC=https://poseq.omniphi.network:8545
      - SIGNER_KEY_FILE=/keys/relayer2.key
    volumes:
      - ./keys/relayer2:/keys:ro
      - relayer2-db:/data
    restart: unless-stopped

  relayer-3:
    image: omniphi/bridge-relayer:latest
    environment:
      - CORE_RPC=https://core.omniphi.network:26657
      - POSEQ_RPC=https://poseq.omniphi.network:8545
      - SIGNER_KEY_FILE=/keys/relayer3.key
    volumes:
      - ./keys/relayer3:/keys:ro
      - relayer3-db:/data
    restart: unless-stopped

volumes:
  relayer1-db:
  relayer2-db:
  relayer3-db:
```

---

## Phase 4: Wallet & UI

### 4.1 Wallet SDK

```typescript
// packages/wallet-sdk/src/index.ts

import { DirectSecp256k1HdWallet } from "@cosmjs/proto-signing";
import { Wallet as EthWallet } from "ethers";

export interface OmniphiWallet {
  mnemonic: string;
  cosmos: {
    address: string;      // omni1...
    wallet: DirectSecp256k1HdWallet;
  };
  evm: {
    address: string;      // 0x...
    displayAddress: string; // 1x...
    wallet: EthWallet;
  };
}

export async function createWallet(): Promise<OmniphiWallet> {
  // Generate mnemonic
  const ethWallet = EthWallet.createRandom();
  const mnemonic = ethWallet.mnemonic!.phrase;

  // Derive Cosmos wallet
  const cosmosWallet = await DirectSecp256k1HdWallet.fromMnemonic(mnemonic, {
    prefix: "omni",
    hdPaths: ["m/44'/118'/0'/0/0"],
  });
  const [cosmosAccount] = await cosmosWallet.getAccounts();

  // Derive EVM wallet (already have it)
  const evmAddress = ethWallet.address;

  return {
    mnemonic,
    cosmos: {
      address: cosmosAccount.address,
      wallet: cosmosWallet,
    },
    evm: {
      address: evmAddress,
      displayAddress: to1x(evmAddress),
      wallet: ethWallet,
    },
  };
}

export function to1x(address: string): string {
  return address.startsWith("0x") ? "1x" + address.slice(2) : address;
}

export function from1x(address: string): string {
  return address.startsWith("1x") ? "0x" + address.slice(2) : address;
}

export async function importWallet(mnemonic: string): Promise<OmniphiWallet> {
  // Derive both wallets from same mnemonic
  const cosmosWallet = await DirectSecp256k1HdWallet.fromMnemonic(mnemonic, {
    prefix: "omni",
    hdPaths: ["m/44'/118'/0'/0/0"],
  });
  const [cosmosAccount] = await cosmosWallet.getAccounts();

  const ethWallet = EthWallet.fromMnemonic(mnemonic);

  return {
    mnemonic,
    cosmos: {
      address: cosmosAccount.address,
      wallet: cosmosWallet,
    },
    evm: {
      address: ethWallet.address,
      displayAddress: to1x(ethWallet.address),
      wallet: ethWallet,
    },
  };
}
```

### 4.2 Bridge UI Component

```typescript
// packages/wallet-ui/src/components/Bridge.tsx

import React, { useState } from 'react';
import { useOmniphiWallet } from '../hooks/useOmniphiWallet';
import { useBridge } from '../hooks/useBridge';

export function Bridge() {
  const { wallet } = useOmniphiWallet();
  const { deposit, withdraw, status } = useBridge();

  const [direction, setDirection] = useState<'to-poseq' | 'to-core'>('to-poseq');
  const [amount, setAmount] = useState('');

  const handleBridge = async () => {
    if (direction === 'to-poseq') {
      // Core → PoSeQ
      await deposit({
        amount,
        from: wallet.cosmos.address,
        to: wallet.evm.address,
      });
    } else {
      // PoSeQ → Core
      await withdraw({
        amount,
        from: wallet.evm.address,
        to: wallet.cosmos.address,
      });
    }
  };

  return (
    <div className="bridge-container">
      <h2>Bridge OMNI</h2>

      <div className="direction-toggle">
        <button
          className={direction === 'to-poseq' ? 'active' : ''}
          onClick={() => setDirection('to-poseq')}
        >
          Core → PoSeQ
        </button>
        <button
          className={direction === 'to-core' ? 'active' : ''}
          onClick={() => setDirection('to-core')}
        >
          PoSeQ → Core
        </button>
      </div>

      <div className="bridge-form">
        <div className="from-chain">
          <label>From</label>
          <div className="chain-info">
            {direction === 'to-poseq' ? (
              <>
                <span className="chain-name">Core Chain</span>
                <span className="address">{wallet.cosmos.address}</span>
              </>
            ) : (
              <>
                <span className="chain-name">PoSeQ Chain</span>
                <span className="address">{wallet.evm.displayAddress}</span>
              </>
            )}
          </div>
        </div>

        <div className="amount-input">
          <input
            type="number"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="Amount"
          />
          <span className="token">OMNI</span>
        </div>

        <div className="to-chain">
          <label>To</label>
          <div className="chain-info">
            {direction === 'to-poseq' ? (
              <>
                <span className="chain-name">PoSeQ Chain</span>
                <span className="address">{wallet.evm.displayAddress}</span>
              </>
            ) : (
              <>
                <span className="chain-name">Core Chain</span>
                <span className="address">{wallet.cosmos.address}</span>
              </>
            )}
          </div>
        </div>

        <div className="bridge-info">
          <p>Estimated time: ~5 minutes</p>
          <p>Bridge fee: 0.1%</p>
        </div>

        <button
          className="bridge-button"
          onClick={handleBridge}
          disabled={status === 'pending'}
        >
          {status === 'pending' ? 'Bridging...' : 'Bridge'}
        </button>
      </div>
    </div>
  );
}
```

---

## Timeline Estimate

| Phase | Components | Complexity |
|-------|------------|------------|
| **Phase 1** | x/bridge, x/anchor modules | Medium |
| **Phase 2** | PoSeQ chain, consensus | High |
| **Phase 3** | Bridge contracts, relayers | Medium |
| **Phase 4** | Wallet SDK, UI | Medium |

---

## Dependencies

### Core Chain
- Cosmos SDK v0.53+
- CometBFT v0.38+
- IBC-Go v10+

### PoSeQ Chain
- go-ethereum v1.13+
- Solidity ^0.8.20
- OpenZeppelin contracts

### Bridge
- gRPC for Core communication
- Web3 for PoSeQ communication
- PostgreSQL for relayer state

### Wallet
- @cosmjs/stargate
- ethers.js v6
- React 18+

---

## Next Immediate Steps

1. **Create `x/bridge` module skeleton**
2. **Create `x/anchor` module skeleton**
3. **Add bridge activity type to feemarket**
4. **Write bridge module tests**
5. **Design PoSeQ genesis configuration**

Ready to start implementation?
