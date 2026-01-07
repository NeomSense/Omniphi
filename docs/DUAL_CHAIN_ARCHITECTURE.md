# Omniphi Dual-Chain Architecture

## Executive Summary

Omniphi operates as a **dual-chain ecosystem** with clear separation of concerns:

| Chain | Purpose | Consensus | Address Format |
|-------|---------|-----------|----------------|
| **Omniphi Core** | Identity, Staking, Governance, PoC | CometBFT (PoS) | `omni1...` |
| **PoSeQ** | Smart Contracts, DeFi, dApps | PoSeQ Consensus | `0x.../1x...` |

These are **separate chains** connected via a **bridge/anchor protocol**, NOT a single chain with embedded EVM.

---

## 1. Architecture Overview

```
                    OMNIPHI ECOSYSTEM
    ════════════════════════════════════════════════════

    ┌─────────────────────────────────────────────────┐
    │              OMNIPHI CORE CHAIN                 │
    │           (Cosmos SDK - CometBFT)               │
    │                                                 │
    │  ┌─────────┐ ┌─────────┐ ┌─────────┐          │
    │  │ Staking │ │   Gov   │ │Treasury │          │
    │  │  (PoS)  │ │  (DAO)  │ │Tokenomics│          │
    │  └─────────┘ └─────────┘ └─────────┘          │
    │                                                 │
    │  ┌─────────┐ ┌─────────┐ ┌─────────┐          │
    │  │   PoC   │ │   IBC   │ │FeeMarket│          │
    │  │ Rewards │ │ Bridge  │ │  Burns  │          │
    │  └─────────┘ └─────────┘ └─────────┘          │
    │                                                 │
    │  Address: omni1...        Wallet: Keplr        │
    └──────────────────┬──────────────────────────────┘
                       │
                       │  BRIDGE / ANCHOR
                       │  (Lock-Mint / Burn-Release)
                       │
    ┌──────────────────┴──────────────────────────────┐
    │              PoSeQ EXECUTION CHAIN              │
    │              (EVM - PoSeQ Consensus)            │
    │                                                 │
    │  ┌─────────┐ ┌─────────┐ ┌─────────┐          │
    │  │Solidity │ │  DeFi   │ │  NFTs   │          │
    │  │Contracts│ │Protocols│ │ Gaming  │          │
    │  └─────────┘ └─────────┘ └─────────┘          │
    │                                                 │
    │  ┌─────────┐ ┌─────────┐ ┌─────────┐          │
    │  │   AI    │ │Sequencer│ │  Data   │          │
    │  │ Queries │ │ Batches │ │ Anchors │          │
    │  └─────────┘ └─────────┘ └─────────┘          │
    │                                                 │
    │  Address: 0x... (1x... display)   Wallet: MetaMask│
    └─────────────────────────────────────────────────┘
```

---

## 2. Chain Specifications

### 2.1 Omniphi Core Chain (Existing)

**Purpose**: Sovereign identity, economic security, governance

| Property | Value |
|----------|-------|
| Framework | Cosmos SDK v0.53 |
| Consensus | CometBFT (Tendermint) |
| Block Time | ~5-6 seconds |
| Finality | Instant (single-slot) |
| Address Format | `omni1...` (Bech32) |
| Key Type | secp256k1 |
| Native Token | OMNI (`omniphi` base denom) |
| Wallets | Keplr, Leap, Cosmostation |

**Modules**:
- `x/staking` - Validator delegation, PoS consensus
- `x/gov` - Governance proposals, voting
- `x/bank` - Native token transfers
- `x/tokenomics` - Inflation, burns, treasury redirect
- `x/feemarket` - Dynamic fees, burn tiers
- `x/poc` - Proof of Contribution rewards
- `x/ibc` - Cross-chain communication
- `x/anchor` - PoSeQ checkpoint verification (NEW)

**Explicitly NOT Included**:
- ❌ `x/evm` (Ethermint)
- ❌ EVM precompiles
- ❌ Solidity execution
- ❌ MetaMask direct signing

### 2.2 PoSeQ Execution Chain (New)

**Purpose**: High-throughput smart contract execution

| Property | Value |
|----------|-------|
| Framework | go-ethereum fork / Custom |
| Consensus | PoSeQ (Proof of Sequenced Execution) |
| Block Time | ~1-2 seconds |
| Finality | Soft (fast) + Hard (anchored to Core) |
| Address Format | `0x...` (EIP-55) |
| Display Format | `1x...` (UI only) |
| Key Type | secp256k1 (Ethereum) |
| Native Token | OMNI (bridged from Core) |
| Wallets | MetaMask, WalletConnect, Rainbow |

**Capabilities**:
- Full EVM bytecode execution
- Solidity/Vyper smart contracts
- ERC-20, ERC-721, ERC-1155 standards
- JSON-RPC compatibility (eth_*)
- Web3.js / Ethers.js support

**Consensus Model (PoSeQ)**:
- Sequencers batch transactions
- Batches anchored to Core chain
- Dispute resolution via Core governance
- Economic security inherited from Core staking

---

## 3. Address System

### 3.1 Key Derivation (Shared)

Both chains use **secp256k1** keys. Users derive addresses from the same seed:

```
BIP-39 Mnemonic
      │
      ▼
Master Key (BIP-32)
      │
      ├─────────────────────────────────────┐
      │                                     │
      ▼                                     ▼
   m/44'/118'/0'/0/0                    m/44'/60'/0'/0/0
   (Cosmos path)                        (Ethereum path)
      │                                     │
      ▼                                     ▼
  SHA256 + RIPEMD160                   Keccak256 (last 20 bytes)
      │                                     │
      ▼                                     ▼
   omni1abc...xyz                       0xAbCdEf...123
   (Core Chain)                         (PoSeQ Chain)
                                            │
                                            ▼
                                        1xAbCdEf...123
                                        (Display Only)
```

### 3.2 Address Formats

| Chain | Format | Example | Usage |
|-------|--------|---------|-------|
| Core | Bech32 | `omni1qwerty789abc...` | All Core transactions |
| Core (Validator) | Bech32 | `omnivaloper1xyz...` | Validator operations |
| PoSeQ (Internal) | EIP-55 | `0xAbCdEf456789...` | Signing, RPC, contracts |
| PoSeQ (Display) | Branded | `1xAbCdEf456789...` | UI/UX only |

### 3.3 The `1x` Branding Rule

**CRITICAL**: `1x` is a **display alias only**. It NEVER appears in:
- Protocol messages
- Transaction signing
- Smart contract calls
- RPC requests/responses
- On-chain storage

**Formatter Functions**:

```typescript
// TypeScript (Wallet UI)
function to1x(address: string): string {
  return address.startsWith("0x") ? "1x" + address.slice(2) : address;
}

function from1x(address: string): string {
  return address.startsWith("1x") ? "0x" + address.slice(2) : address;
}
```

```go
// Go (Explorer/Backend)
func To1x(addr string) string {
    if strings.HasPrefix(addr, "0x") {
        return "1x" + addr[2:]
    }
    return addr
}

func From1x(addr string) string {
    if strings.HasPrefix(addr, "1x") {
        return "0x" + addr[2:]
    }
    return addr
}
```

---

## 4. Bridge Protocol

### 4.1 Bridge Architecture

```
     OMNIPHI CORE                         PoSeQ CHAIN
    ─────────────                        ─────────────

    ┌─────────────┐                    ┌─────────────┐
    │   User      │                    │   User      │
    │ omni1abc... │                    │ 1xDef456... │
    └──────┬──────┘                    └──────┬──────┘
           │                                  │
           │ Lock OMNI                        │ Use OMNI
           ▼                                  ▼
    ┌─────────────┐                    ┌─────────────┐
    │   Bridge    │    Relay Message   │   Bridge    │
    │   Module    │◄──────────────────►│   Contract  │
    │ (x/bridge)  │                    │  (Solidity) │
    └──────┬──────┘                    └──────┬──────┘
           │                                  │
           │ Lock/Burn                        │ Mint/Release
           ▼                                  ▼
    ┌─────────────┐                    ┌─────────────┐
    │   Escrow    │                    │   Wrapped   │
    │   Account   │                    │    OMNI     │
    │(module acct)│                    │  (ERC-20)   │
    └─────────────┘                    └─────────────┘
```

### 4.2 Token Flow: Core → PoSeQ (Deposit)

1. User calls `MsgBridgeDeposit` on Core chain
2. OMNI locked in bridge escrow account
3. Bridge relayers observe and sign
4. Bridge contract on PoSeQ mints wrapped OMNI
5. User receives wOMNI at their `0x` address

```
User: omni1abc... has 1000 OMNI
      │
      │ MsgBridgeDeposit(amount: 500 OMNI, recipient: 0xDef...)
      ▼
Core: Lock 500 OMNI in escrow
      │
      │ Event: BridgeDeposit{from: omni1abc, to: 0xDef, amount: 500}
      ▼
Relayers: Sign attestation (threshold: 2/3 validators)
      │
      │ Submit proof to PoSeQ
      ▼
PoSeQ: Bridge contract verifies signatures
      │
      │ mint(0xDef, 500 wOMNI)
      ▼
User: 0xDef... has 500 wOMNI on PoSeQ
```

### 4.3 Token Flow: PoSeQ → Core (Withdrawal)

1. User calls `withdraw()` on PoSeQ bridge contract
2. wOMNI burned on PoSeQ
3. Bridge relayers observe and sign
4. User claims on Core via `MsgBridgeWithdraw`
5. OMNI released from escrow

```
User: 0xDef... has 500 wOMNI
      │
      │ BridgeContract.withdraw(amount: 500, recipient: "omni1abc...")
      ▼
PoSeQ: Burn 500 wOMNI
      │
      │ Event: BridgeWithdraw{from: 0xDef, to: omni1abc, amount: 500}
      ▼
Relayers: Sign attestation
      │
      │ Submit proof to Core
      ▼
Core: Verify signatures + withdrawal proof
      │
      │ Release 500 OMNI from escrow to omni1abc
      ▼
User: omni1abc... has 500 OMNI on Core
```

### 4.4 Security Model

| Property | Mechanism |
|----------|-----------|
| **Validator Set** | Core validators also validate bridge messages |
| **Threshold** | 2/3 validator signatures required |
| **Delay** | Withdrawals have 1-hour finality delay |
| **Dispute** | Fraud proofs can challenge invalid state |
| **Economic Security** | Bridge funds backed by validator stakes |

---

## 5. Anchor Protocol (PoSeQ → Core)

### 5.1 Purpose

PoSeQ inherits security from Core by **anchoring checkpoints**:

1. PoSeQ sequencers submit state roots to Core
2. Core validators verify and finalize
3. Disputes resolved via Core governance
4. Final settlement on Core chain

### 5.2 Checkpoint Structure

```protobuf
message PoSeQCheckpoint {
  uint64 block_number = 1;        // PoSeQ block height
  bytes state_root = 2;           // Merkle root of PoSeQ state
  bytes transactions_root = 3;    // Merkle root of transactions
  bytes receipts_root = 4;        // Merkle root of receipts
  uint64 timestamp = 5;           // Unix timestamp
  bytes sequencer_signature = 6;  // Sequencer attestation
}
```

### 5.3 Anchor Flow

```
PoSeQ Chain                              Core Chain
───────────                              ──────────

Block N produced
     │
     │ Every 100 blocks
     ▼
Checkpoint created
     │
     │ MsgSubmitCheckpoint
     ▼
                    ────────────────────►  x/anchor module
                                                │
                                                │ Verify signature
                                                │ Store checkpoint
                                                ▼
                                          Checkpoint finalized
                                                │
                                                │ After dispute period
                                                ▼
                                          State considered final
```

---

## 6. Fee & Tokenomics Unification

### 6.1 Single Token Economy

Both chains use OMNI as the native token:

| Chain | Token | Representation |
|-------|-------|----------------|
| Core | OMNI | Native (`omniphi` denom) |
| PoSeQ | wOMNI | ERC-20 (wrapped) |

**1 OMNI on Core = 1 wOMNI on PoSeQ** (always 1:1 via bridge)

### 6.2 Fee Flow

**Core Chain Fees**:
```
Transaction Fee → x/feemarket → Burn + Validators + Treasury
```

**PoSeQ Chain Fees**:
```
Gas Fee (wOMNI) → Sequencer → Periodic settlement to Core
                     │
                     └──► Burns applied on Core settlement
```

### 6.3 Unified Burn Accounting

All burns are **recorded on Core**, regardless of origin:

| Source | Burn Location | Recording |
|--------|--------------|-----------|
| Core tx fees | Core chain | Direct |
| PoSeQ gas fees | PoSeQ chain | Via settlement |
| Bridge fees | Both | Reconciled on Core |

---

## 7. Wallet Integration

### 7.1 Unified Wallet UX

```
┌─────────────────────────────────────────────────────────┐
│                   OMNIPHI WALLET                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   Total Balance: 1,234.56 OMNI                         │
│   ─────────────────────────────────────────            │
│                                                         │
│   ┌─────────────────────────────────────────────────┐  │
│   │  CORE CHAIN                              850 OMNI│  │
│   │  omni1qwerty789...                               │  │
│   │  [Stake] [Vote] [Send]                          │  │
│   └─────────────────────────────────────────────────┘  │
│                                                         │
│   ┌─────────────────────────────────────────────────┐  │
│   │  PoSeQ CHAIN                            384.56 OMNI│  │
│   │  1xAbCdEf456...                                  │  │
│   │  [Swap] [NFTs] [Contracts]                      │  │
│   └─────────────────────────────────────────────────┘  │
│                                                         │
│   ┌─────────────────────────────────────────────────┐  │
│   │  BRIDGE                                          │  │
│   │  [Core → PoSeQ]  [PoSeQ → Core]                 │  │
│   └─────────────────────────────────────────────────┘  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 7.2 Wallet Connections

| Action | Wallet | Chain |
|--------|--------|-------|
| Stake OMNI | Keplr | Core |
| Vote on proposal | Keplr | Core |
| IBC transfer | Keplr | Core |
| Deploy contract | MetaMask | PoSeQ |
| Swap on DEX | MetaMask | PoSeQ |
| Mint NFT | MetaMask | PoSeQ |
| Bridge deposit | Keplr → MetaMask | Core → PoSeQ |
| Bridge withdraw | MetaMask → Keplr | PoSeQ → Core |

### 7.3 Address Input Handling

```typescript
function parseOmniphiAddress(input: string): AddressInfo {
  input = input.trim();

  // Core chain (Bech32)
  if (input.startsWith("omni1")) {
    return {
      chain: "core",
      format: "bech32",
      display: input,
      canonical: input,
      wallet: "keplr"
    };
  }

  // PoSeQ (branded display)
  if (input.startsWith("1x")) {
    const canonical = "0x" + input.slice(2);
    return {
      chain: "poseq",
      format: "evm",
      display: input,
      canonical: canonical,
      wallet: "metamask"
    };
  }

  // PoSeQ (standard EVM)
  if (input.startsWith("0x")) {
    return {
      chain: "poseq",
      format: "evm",
      display: "1x" + input.slice(2),
      canonical: input,
      wallet: "metamask"
    };
  }

  // Validator address
  if (input.startsWith("omnivaloper1")) {
    return {
      chain: "core",
      format: "bech32",
      display: input,
      canonical: input,
      wallet: "keplr"
    };
  }

  throw new Error("Invalid Omniphi address");
}
```

---

## 8. Implementation Phases

### Phase 1: Core Chain Enhancements (Current)
- [x] Tokenomics module
- [x] Fee market with burns
- [x] PoC rewards
- [ ] `x/bridge` module (escrow + relay)
- [ ] `x/anchor` module (checkpoint verification)

### Phase 2: PoSeQ Chain Development
- [ ] Fork go-ethereum or use OP Stack
- [ ] Implement PoSeQ consensus
- [ ] Deploy bridge contract (Solidity)
- [ ] Configure EVM parameters
- [ ] Implement sequencer selection

### Phase 3: Bridge Integration
- [ ] Deploy bridge relayer network
- [ ] Security audits
- [ ] Testnet bridge testing
- [ ] Mainnet bridge launch

### Phase 4: Wallet & UI
- [ ] Unified wallet interface
- [ ] `1x` display formatting
- [ ] Cross-chain transaction UX
- [ ] Explorer integration

---

## 9. Security Considerations

### 9.1 Bridge Security

| Risk | Mitigation |
|------|------------|
| Validator collusion | Slashing + economic penalties |
| Relay manipulation | Multi-sig threshold (2/3) |
| Double-spend | Finality delay + fraud proofs |
| Contract bugs | Multiple audits + formal verification |

### 9.2 PoSeQ Security

| Risk | Mitigation |
|------|------------|
| Sequencer censorship | Forced inclusion via Core |
| Invalid state roots | Dispute resolution + slashing |
| Reorg attacks | Anchoring to Core finality |
| MEV extraction | Sequencer rotation + encryption |

---

## 10. Comparison: Dual-Chain vs Ethermint

| Aspect | Dual-Chain (Chosen) | Ethermint (Rejected) |
|--------|---------------------|----------------------|
| Complexity | Higher (two chains) | Lower (one chain) |
| Scalability | Better (separate execution) | Limited (shared resources) |
| Upgradability | Independent upgrades | Coupled upgrades |
| Security | Isolated failure domains | Single failure domain |
| EVM Compatibility | Full (native EVM) | Good (embedded EVM) |
| Cosmos Compatibility | Full (native Cosmos) | Full |
| Wallet UX | Two connections needed | Single connection |
| Latency | Bridge adds delay | No bridge needed |

**Decision**: Dual-chain chosen for **scalability** and **clean separation of concerns**.

---

## 11. Key Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| EVM location | Separate chain (PoSeQ) | Scalability, isolation |
| Bridge model | Lock-mint / burn-release | Standard, auditable |
| Security anchor | Checkpoint to Core | Inherited security |
| Address display | `1x` for PoSeQ UI | Brand identity |
| Token model | Single OMNI, bridged | Unified economy |
| Fee burns | Recorded on Core | Single source of truth |

---

**Document Status**: CANONICAL ARCHITECTURE SPECIFICATION

**Next Steps**:
1. Implement `x/bridge` module on Core
2. Implement `x/anchor` module on Core
3. Build PoSeQ chain (Phase 2)
4. Deploy bridge contracts
5. Build unified wallet UI

---

*This document defines the authoritative Omniphi dual-chain architecture. All implementation must conform to this specification.*
