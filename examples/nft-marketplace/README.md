# Omniphi Programmable Ownership Marketplace

A reference marketplace for Omniphi's programmable ownership objects -- digital assets with embedded transfer rules, enforced royalties, and intent-based trading.

## Overview

Omniphi replaces static NFTs (ERC-721) with programmable ownership objects. The key difference: traditional NFTs are just a mapping from token ID to address, with all marketplace logic living in separate contracts. Omniphi ownership objects carry their own rules.

| Feature | ERC-721 NFT | Omniphi Ownership Object |
|---|---|---|
| **Transfer rules** | None (marketplace-dependent) | Embedded in the object |
| **Royalties** | Optional, easily bypassed | Enforced at protocol level |
| **Time-locks** | Requires separate contract | Built-in transfer rule |
| **Fractionalization** | Requires wrapper contract | Native capability |
| **Marketplace approval** | setApprovalForAll (risky) | Not needed (intent-based) |
| **Creator control** | None after mint | Approval gates, allowlists |

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

### Traditional NFT Marketplace Flow

```
  Seller                    Marketplace (Seaport)              Buyer
    |                              |                             |
    |-- setApprovalForAll() ------>|                             |
    |-- sign order (off-chain) --->| stores order on server      |
    |                              |                             |
    |                              |<--- fulfillOrder() ---------|
    |                              |                             |
    |<---- NFT transferred --------|---- payment sent ---------->|
    |                              |                             |
    |  Royalties? Maybe. Depends on marketplace policy.          |
```

### Omniphi Intent-Based Flow

```
  Seller                    Omniphi Chain                     Buyer
    |                              |                             |
    |-- listForSale() ------------>| on-chain listing            |
    |   (no approval needed)       |                             |
    |                              |                             |
    |                              |<--- buyIntent() ------------|
    |                              |     (max price + deadline)  |
    |                              |                             |
    |                       Solver matches                       |
    |                       listing + intent                     |
    |                              |                             |
    |                       Atomic settlement:                   |
    |                       1. Verify transfer rules             |
    |                       2. Check timelock                    |
    |                       3. Transfer object                   |
    |                       4. Pay seller (minus royalty)         |
    |                       5. Pay creator (royalty)              |
    |                              |                             |
    |<---- payment received -------|---- object received ------->|
    |                              |                             |
    |  Royalties: ALWAYS enforced. Object physically cannot      |
    |  transfer without royalty payment.                          |
```

## Transfer Rules

Each ownership object carries transfer rules that are evaluated at settlement time:

### Time-Lock

```typescript
transferRules: {
  timelockEnabled: true,
  timelockDuration: 86400, // must hold 24 hours before transferring
}
```

Prevents pump-and-dump schemes. The chain rejects any transfer intent submitted before the timelock expires.

### Creator Approval

```typescript
transferRules: {
  creatorApprovalRequired: true,
}
```

The original creator must co-sign each transfer. Useful for artists who want control over where their work ends up.

### Recipient Restrictions

```typescript
transferRules: {
  allowedRecipients: ["omni1approved_addr1", "omni1approved_addr2"],
}
```

Transfers are restricted to a whitelist. Useful for membership tokens, compliance-restricted assets, or gated communities.

### Max Transfers

```typescript
transferRules: {
  maxTransfers: 5,
}
```

The object can only change hands N times total. Creates provable scarcity beyond just quantity.

### Enforced Royalties

```typescript
{
  royaltyBps: 500, // 5% of sale price
}
```

Royalty payment is a transfer precondition. The transfer physically cannot happen without the royalty being paid. This is fundamentally different from ERC-721 where royalties are just a suggestion that marketplaces can ignore.

## API Reference

### `OmniphiMarketplace.connect(mnemonic)`

Creates a signing marketplace client.

### `listForSale(objectId, price, duration)`

Lists an ownership object for sale. No approval needed -- the object stays in your wallet until settlement.

### `buyIntent(objectId, maxPrice)`

Submits a purchase intent. Solver matches with the best listing. You pay the asking price, not your max.

### `makeOffer(objectId, price, expiry)`

Makes a conditional offer. Funds are locked in a capability-based escrow (see escrow example).

### `queryListings()`

Returns all active marketplace listings.

### `queryOffers(objectId)`

Returns all active offers on a specific object.

### `queryObject(objectId)`

Returns the ownership object details including transfer rules.

## Why Intents Are Better for Marketplaces

### 1. Enforced Royalties

The single biggest problem in the NFT ecosystem is royalty enforcement. Marketplaces like Blur and Sudoswap allow zero-royalty trading, undermining creators. On Omniphi, royalties are enforced by the object itself -- no marketplace can bypass them.

### 2. No Dangerous Approvals

`setApprovalForAll` is a security anti-pattern that has enabled billions of dollars in theft. If a marketplace contract has a vulnerability, all approved NFTs can be stolen. Omniphi's intent-based model eliminates approvals entirely.

### 3. Decentralized Order Book

OpenSea's order book is centralized. If their servers go down, you cannot trade. Omniphi listings are on-chain, readable by any marketplace frontend.

### 4. Rich Transfer Conditions

Conditional offers, time-locked transfers, and creator approval gates are native features, not bolted-on contracts.

## Chain Configuration

- **Bech32 prefix**: `omni`
- **Native denom**: `omniphi`
- **RPC port**: 26657
- **REST port**: 1317

## Dependencies

- `@omniphi/sdk` -- Omniphi TypeScript SDK
- `ts-node` -- TypeScript execution runtime
