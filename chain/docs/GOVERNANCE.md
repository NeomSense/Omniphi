# Omniphi Governance System

## Overview

Omniphi implements a **community-accessible governance model** designed to:
- Encourage broad participation
- Prevent spam and low-effort proposals
- Avoid plutocratic centralization
- Support DAO-controlled parameter evolution

This document explains the governance deposit mechanism, voting process, and parameter tuning.

## Deposit Structure

### Mainnet Configuration

| Proposal Type | Required Deposit | In uomni |
|---------------|------------------|----------|
| Standard | 1,000 OMNI | 1,000,000,000 |
| Expedited | 5,000 OMNI | 5,000,000,000 |

### Testnet Configuration

| Proposal Type | Required Deposit | In uomni |
|---------------|------------------|----------|
| Standard | 100 OMNI | 100,000,000 |
| Expedited | 500 OMNI | 500,000,000 |

## Deposit Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     PROPOSAL LIFECYCLE                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. SUBMIT PROPOSAL                                             │
│     └─► Requires 10% initial deposit (100 OMNI minimum)        │
│         └─► Opens 2-day deposit period                         │
│                                                                 │
│  2. DEPOSIT PERIOD (2 days)                                     │
│     └─► Community can crowdfund remaining 90%                  │
│     └─► If full deposit reached → Voting begins                │
│     └─► If period expires → Deposits refunded                  │
│                                                                 │
│  3. VOTING PERIOD (5 days mainnet / 2 days testnet)            │
│     └─► Vote options: Yes, No, NoWithVeto, Abstain             │
│     └─► Requires 33.4% quorum                                  │
│     └─► Requires 50% Yes threshold                             │
│                                                                 │
│  4. OUTCOME                                                     │
│     ├─► PASS: 100% deposit refunded                            │
│     ├─► FAIL: 100% deposit refunded                            │
│     └─► VETO (33.4% NoWithVeto): Deposit BURNED                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Deposit Resolution Rules

| Outcome | Deposit Handling | Rationale |
|---------|------------------|-----------|
| Pass | 100% refunded | Reward good-faith proposals |
| Fail (normal) | 100% refunded | Encourage participation |
| Fail (quorum not met) | 100% refunded | Not proposer's fault |
| Deposit period expires | 100% refunded | Proposal never reached vote |
| Vetoed (spam) | 100% burned | Spam deterrent |

### Burn Configuration

```json
{
  "burn_vote_quorum": false,
  "burn_proposal_deposit_prevote": false,
  "burn_vote_veto": true
}
```

- `burn_vote_quorum: false` - Deposits refunded if quorum not met
- `burn_proposal_deposit_prevote: false` - Deposits refunded if deposit period expires
- `burn_vote_veto: true` - Deposits burned if proposal vetoed

## Voting Parameters

### Mainnet

| Parameter | Value | Description |
|-----------|-------|-------------|
| `voting_period` | 5 days | Time for voting |
| `max_deposit_period` | 2 days | Time to collect deposits |
| `quorum` | 33.4% | Minimum participation |
| `threshold` | 50% | Yes votes to pass |
| `veto_threshold` | 33.4% | NoWithVeto to reject |
| `expedited_voting_period` | 1 day | Expedited proposals |
| `expedited_threshold` | 66.7% | Expedited pass threshold |

### Testnet

| Parameter | Value | Description |
|-----------|-------|-------------|
| `voting_period` | 2 days | Time for voting |
| `max_deposit_period` | 1 day | Time to collect deposits |
| `quorum` | 25% | Lower for testing |
| `expedited_voting_period` | 12 hours | Faster iteration |

## Crowdfunding Deposits

Omniphi supports **community-funded proposals**:

1. **Proposer** submits with 10% initial deposit (100 OMNI)
2. **Community members** can contribute to reach full deposit
3. **All depositors** are refunded proportionally when proposal completes

### Example

```
Initial deposit: 100 OMNI (Proposer)
Community adds:  300 OMNI (Supporter A)
Community adds:  600 OMNI (Supporter B)
Total:          1,000 OMNI (Full deposit met)

If proposal passes or fails (non-veto):
  - Proposer receives:    100 OMNI
  - Supporter A receives: 300 OMNI
  - Supporter B receives: 600 OMNI
```

## Submitting Proposals

### Standard Proposal

```bash
posd tx gov submit-proposal proposal.json \
  --from <wallet> \
  --deposit 1000000000uomni \
  --chain-id omniphi-mainnet-1
```

### Expedited Proposal

```bash
posd tx gov submit-proposal proposal.json \
  --from <wallet> \
  --deposit 5000000000uomni \
  --expedited \
  --chain-id omniphi-mainnet-1
```

### proposal.json Example

```json
{
  "@type": "/cosmos.gov.v1.MsgSubmitProposal",
  "messages": [
    {
      "@type": "/cosmos.gov.v1.MsgUpdateParams",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700jvvkrck",
      "params": { ... }
    }
  ],
  "initial_deposit": [
    {"denom": "uomni", "amount": "1000000000"}
  ],
  "proposer": "omni1...",
  "metadata": "ipfs://...",
  "title": "Proposal Title",
  "summary": "Brief description of the proposal"
}
```

## Voting on Proposals

```bash
# Vote Yes
posd tx gov vote <proposal-id> yes --from <wallet>

# Vote No
posd tx gov vote <proposal-id> no --from <wallet>

# Vote Abstain (counts toward quorum)
posd tx gov vote <proposal-id> abstain --from <wallet>

# Vote NoWithVeto (flags as spam)
posd tx gov vote <proposal-id> no_with_veto --from <wallet>
```

## Query Commands

```bash
# List all proposals
posd query gov proposals

# Get specific proposal
posd query gov proposal <proposal-id>

# Check deposits
posd query gov deposits <proposal-id>

# Check votes
posd query gov votes <proposal-id>

# Get governance params
posd query gov params
```

## Design Rationale

### Why 1,000 OMNI Standard Deposit?

1. **Accessible**: ~0.00013% of total supply (750M at genesis)
2. **Not Prohibitive**: Typical holder can afford initial 10%
3. **Spam Resistant**: Still meaningful economic commitment
4. **Industry Aligned**: Similar to Cosmos Hub (512 ATOM), Osmosis (1,600 OSMO)

### Why Refund on Failure?

1. **Encourages Experimentation**: Failed proposals are learning opportunities
2. **Reduces Fear**: Proposers won't lose funds for good-faith attempts
3. **Only Punishes Spam**: Veto burns deposits for clearly malicious proposals

### Why Crowdfunding?

1. **Decentralizes Governance**: Proposals don't require whales
2. **Community Validation**: Popular ideas get funded faster
3. **Signal Before Vote**: Deposit funding indicates support

## Governance Safety Features

### Time-Lock

All parameter changes have a built-in time-lock:
- Proposals must pass voting period
- Changes take effect next block after passage
- No retroactive changes

### Protocol-Enforced Limits

Certain parameters have protocol-level caps that governance cannot exceed:

| Parameter | Limit | Governance Can Change? |
|-----------|-------|------------------------|
| Max inflation | 5% | No (protocol cap) |
| Max burn ratio | 50% | No (protocol cap) |
| Supply cap | 1.5B OMNI | No (immutable) |

### Validator Protection

Governance cannot:
- Reduce validator revenue below 50% of distributable fees
- Set unbonding period below 21 days
- Remove slashing protections

## Comparison with Industry Standards

| Chain | Standard Deposit | Voting Period | Quorum |
|-------|------------------|---------------|--------|
| **Omniphi** | **1,000 OMNI** | **5 days** | **33.4%** |
| Cosmos Hub | 512 ATOM | 14 days | 40% |
| Osmosis | 1,600 OSMO | 3 days | 20% |
| Juno | 1,000 JUNO | 5 days | 33.4% |
| Evmos | 1,600 EVMOS | 5 days | 33.4% |

## Audit Checklist

- [x] No governance lock-out (deposit accessible to regular holders)
- [x] No deposit over-penalization (only burn on veto)
- [x] Clear economic bounds (deposit % of supply documented)
- [x] Crowdfunding supported (community can fund proposals)
- [x] Time-lock on all changes
- [x] Protocol-enforced safety caps
- [x] Industry-standard parameters




