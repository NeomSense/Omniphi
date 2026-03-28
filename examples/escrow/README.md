# Omniphi Capability-Based Escrow

A reference escrow service demonstrating Omniphi's capability-based authorization model -- a fundamentally safer alternative to ERC-20 token approvals.

## Overview

On Ethereum, interacting with any DeFi contract requires granting token approvals: `token.approve(spender, amount)`. This creates standing permissions that persist indefinitely and can be exploited through re-entrancy, upgrade attacks, or phishing. Billions of dollars have been lost to approval exploits.

Omniphi replaces approvals with **capabilities**: scoped, time-limited, single-use authorizations.

| Property | ERC-20 Approval | Omniphi Capability |
|---|---|---|
| **Scope** | Blanket (any use by spender) | Tied to specific escrow ID |
| **Duration** | Forever (until revoked) | Auto-expires at deadline |
| **Amount** | Often infinite (MAX_UINT256) | Exact amount, bounded |
| **Reuse** | Unlimited calls to transferFrom | Single-use, consumed on exercise |
| **Conditions** | None | Arbiter signature, deadline, custom |
| **Revocation** | Separate transaction required | Automatic on expiry |

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

### Escrow Lifecycle

```
  Buyer                     Omniphi Chain                  Seller
    |                              |                          |
    |-- createEscrow() ----------->|                          |
    |   (funds + capability)       |                          |
    |                              |                          |
    |              Escrow State: FUNDED                       |
    |              Capability: active, conditions pending     |
    |                              |                          |
    |                              |                          |
    |   [Happy path]               |                          |
    |                              |<-- releaseEscrow() ------|
    |                              |    (arbiter signature)   |
    |                              |                          |
    |              Capability EXERCISED:                      |
    |              1. Verify arbiter signature                |
    |              2. Verify deadline not passed              |
    |              3. Transfer funds to seller                |
    |              4. Consume capability (single-use)         |
    |                              |                          |
    |              Escrow State: RELEASED                     |
```

### Capability Model

```
  +----------------------------------+
  | SpendCapability                  |
  |----------------------------------|
  | capabilityId: "cap_escrow_42"    |
  | grantor: "omni1buyer..."         |
  | maxAmount: "5000000000"          |
  | scopedTo: "escrow_42"           |
  | expiresAt: 1711500000            |
  | consumed: false                  |
  |                                  |
  | conditions:                      |
  |   [1] arbiter_signature          |
  |       requiredSigner: omni1arb.. |
  |   [2] deadline_not_passed        |
  |       deadline: 1711500000       |
  +----------------------------------+
```

A capability can only be exercised when ALL conditions are met. Once exercised, it is permanently consumed.

### Dispute Resolution with Guard Module

```
  Dispute Filed
       |
       v
  [VISIBILITY]          -- Dispute logged on-chain, visible to all parties
       |
       v
  [SHOCK_ABSORBER]      -- Cooling-off period (configurable, e.g., 24h)
       |
       v
  [CONDITIONAL_EXEC]    -- Arbiter reviews evidence, submits signed resolution
       |
       v
  [READY]               -- Resolution verified, awaiting execution
       |
       v
  [EXECUTED]             -- Funds directed per arbiter's decision
```

## API Reference

### `OmniphiEscrow.connect(mnemonic)`

Creates a signing escrow client.

### `createEscrow(seller, buyer, amount, arbiter, deadline)`

Creates and funds an escrow atomically. A scoped spend capability is created with conditions: arbiter signature required + deadline enforcement.

### `releaseEscrow(escrowId, arbiterSignature)`

Releases funds to the seller. Exercises the capability with the arbiter's signature.

### `refundEscrow(escrowId, arbiterSignature)`

Refunds funds to the buyer. Same capability mechanism, different payment direction.

### `disputeEscrow(escrowId, reason)`

Files a dispute. Enters the Guard module's 4-gate safety pipeline.

### `createMilestoneEscrow(seller, buyer, milestones, arbiter, deadline)`

Creates a multi-milestone escrow where each milestone has its own independent capability.

### `queryEscrow(escrowId)`

Returns the escrow state including capability status and conditions.

## Security Analysis

### Why Capabilities Are Safer

**Principle of Least Privilege**: Each capability grants exactly the permission needed and nothing more. An approval for an escrow contract grants blanket permission to spend any amount of your tokens at any time.

**No Leftover Permissions**: Capabilities expire and are consumed. After an escrow resolves, there is zero residual attack surface. With approvals, you must remember to revoke every approval you have ever granted.

**Composable Conditions**: Capabilities can require multiple conditions (arbiter signature AND deadline check AND custom verification). Approvals have no condition mechanism at all.

**Transparent Authorization**: Each capability's conditions are visible on-chain. Users can see exactly what they are authorizing. Approvals are opaque: `approve(address, uint256)` tells you nothing about what the spender will do.

### Attacks Prevented

1. **Re-entrancy**: Capabilities are exercised atomically by the chain, not by callback. No contract can re-enter during capability exercise.

2. **Infinite Approval Exploit**: Capabilities have a bounded `maxAmount`. There is no equivalent of `approve(MAX_UINT256)`.

3. **Stale Approval Drain**: Capabilities expire. A capability from a year-old escrow that was never resolved simply ceases to exist.

4. **Upgrade-and-Drain**: Even if a contract is upgraded maliciously, existing capabilities are scoped to specific escrow IDs. The upgraded contract cannot use them for other purposes.

## Chain Configuration

- **Bech32 prefix**: `omni`
- **Native denom**: `omniphi`
- **RPC port**: 26657
- **REST port**: 1317
- **Max escrow duration**: 30 days
- **Arbiter fee**: 1%

## Dependencies

- `@omniphi/sdk` -- Omniphi TypeScript SDK
- `ts-node` -- TypeScript execution runtime
