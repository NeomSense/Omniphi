# Omniphi Governance Proposals

This directory contains tools and templates for creating governance proposals.

## Quick Start

### Using the Proposal Generator Script

The easiest way to create a proposal is using the generator script:

```bash
# Generate a consensus params proposal (auto-fills current values)
./scripts/create_proposal.sh consensus --title "Set Max Block Gas" --summary "Set max block gas to 60M"

# Generate a feemarket params proposal
./scripts/create_proposal.sh feemarket --title "Update Min Gas Price" --summary "Update minimum gas price"

# Generate other proposal types
./scripts/create_proposal.sh staking --title "Update Staking Params"
./scripts/create_proposal.sh poc --title "Update PoC Params"
./scripts/create_proposal.sh tokenomics --title "Update Tokenomics"
```

The script will:
1. Fetch current parameters from the chain
2. Generate a properly formatted proposal JSON with all required fields
3. Show you what to edit and how to submit

### Manual Proposal Creation

1. Copy the appropriate template from `proposals/templates/`
2. Query current parameters to get the latest values
3. Edit the values you want to change
4. Submit the proposal

## Important Rules

### Consensus Parameters (`MsgUpdateParams`)

**ALL sections are REQUIRED** - even if you only want to change one value:
- `block` (max_bytes, max_gas)
- `evidence` (max_age_num_blocks, max_age_duration, max_bytes)
- `validator` (pub_key_types)
- `abci` (vote_extensions_enable_height)

❌ **Wrong** - Missing sections will cause proposal to fail:
```json
{
  "block": { "max_gas": "60000000" }
}
```

✅ **Correct** - Include all sections:
```json
{
  "block": { "max_bytes": "22020096", "max_gas": "60000000" },
  "evidence": { "max_age_num_blocks": "100000", "max_age_duration": "172800s", "max_bytes": "1048576" },
  "validator": { "pub_key_types": ["ed25519"] },
  "abci": { "vote_extensions_enable_height": "0" }
}
```

### Duration Format

Use seconds format for durations:
- ✅ `"172800s"` (correct)
- ❌ `"48h0m0s"` (may not work)
- ❌ `172800000000000` (nanoseconds won't work)

### Authority Address

The `authority` field must be the gov module address:
```bash
posd query auth module-account gov 2>&1 | grep "address:"
```

For omniphi-testnet-2: `omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8`

## Submitting a Proposal

```bash
# Submit
posd tx gov submit-proposal /path/to/proposal.json \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas 500000 \
  -y

# Check proposal status
posd query gov proposals --output json | jq '.proposals'

# Vote YES
posd tx gov vote <proposal-id> yes \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 50000omniphi \
  -y

# Check tally
posd query gov tally <proposal-id>

# Check when proposal will execute
posd query gov proposal <proposal-id> --output json | jq '.proposal.voting_end_time'
```

## Governance Parameters

Current governance settings:
- **Quorum**: 33.4% of staked tokens must vote
- **Threshold**: 50% YES votes needed to pass
- **Voting Period**: 48 hours
- **Min Deposit**: 10,000,000 omniphi (10 OMNI)

## Templates

- `templates/consensus_params.json` - Update block gas, evidence params
- `templates/feemarket_params.json` - Update gas prices, burn rates, multipliers
