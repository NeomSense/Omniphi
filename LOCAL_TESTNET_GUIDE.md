# Local Testnet Testing Guide

**Date**: February 3, 2026
**Purpose**: Test PoC contributions and governance proposals on a local single-node testnet

---

## Quick Start

### 1. Build the Binary

```bash
cd chain
go build -o build/posd ./cmd/posd
```

### 2. Initialize Testnet

```bash
# Create validator key (if not exists)
posd keys add validator --keyring-backend test

# Initialize the testnet
./scripts/init_testnet.sh validator1

# Or use existing script location
cd scripts && bash init_testnet.sh validator1
```

**Expected Output:**
- Chain ID: `omniphi-testnet-2`
- Total Supply: 1.5 Billion OMNI
- Validator allocation: 1B OMNI (500M stake + 500M treasury)
- Voting period: 5 minutes (testnet fast iteration)

### 3. Start the Node

```bash
# From chain directory
posd start
```

**Or run in background:**
```bash
nohup posd start > ~/.pos/node.log 2>&1 &
```

**Check if running:**
```bash
posd status
```

### 4. Run Integration Tests

Open a **new terminal** (keep node running):

```bash
cd chain/scripts
bash test_local_integration.sh
```

---

## What the Integration Test Does

### Test 1: PoC Contribution Submission ⚠️
- **Action**: Submit a test code contribution
- **Expected**: May fail due to PoA (Proof of Authority) requirements
- **Details**:
  - Type: `code`
  - URI: `https://github.com/omniphi/omniphi/pull/42`
  - Hash: SHA256 of URI
- **Note**: PoC module requires C-Score and PoA verification for contributors

### Test 2: PoC Parameter Query ✅
- **Action**: Query PoC module parameters
- **Expected**: Success
- **Shows**:
  - C-Score cap (100,000)
  - Decay rate (0.5% daily)
  - Per-block quota (100)

### Test 3: Governance Proposal Creation ✅
- **Action**: Create a staking parameter update proposal
- **Expected**: Success
- **Details**:
  - Type: Parameter change (staking.unbonding_time)
  - Deposit: 10K OMNI (10,000,000,000 omniphi)
  - Title: "Test Proposal - Staking Parameters"

### Test 4: Governance Proposal Query ✅
- **Action**: Query the submitted proposal
- **Expected**: Success
- **Shows**:
  - Proposal ID
  - Status (VOTING_PERIOD)
  - Voting start/end times

### Test 5: Governance Vote ✅
- **Action**: Vote YES on the proposal
- **Expected**: Success
- **Validates**: Vote is recorded correctly

### Test 6: Governance Tally ✅
- **Action**: Query vote tally results
- **Expected**: Success
- **Shows**: Yes/No/Abstain/NoWithVeto counts

---

## Manual Testing

### Submit PoC Contribution Manually

```bash
# Generate hash
CONTRIB_URI="https://github.com/omniphi/omniphi/pull/123"
CONTRIB_HASH=$(echo -n "$CONTRIB_URI" | sha256sum | cut -d' ' -f1)

# Submit contribution
posd tx poc submit-contribution \
  "code" \
  "$CONTRIB_URI" \
  "$CONTRIB_HASH" \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto \
  --gas-adjustment 1.5 \
  -y
```

**Query contribution:**
```bash
# Get contribution ID from tx logs
posd query tx <TX_HASH> | jq '.events[] | select(.type=="poc_submit")'

# Query specific contribution
posd query poc contribution <CONTRIBUTION_ID>
```

### Create Governance Proposal Manually

Using the existing script:

```bash
cd chain/scripts

# Generate proposal template
bash create_proposal.sh poc \
  --title "Update PoC Parameters" \
  --summary "Increase C-Score cap to 200,000" \
  --output /tmp/poc_proposal.json

# Edit the proposal
nano /tmp/poc_proposal.json

# Submit proposal
posd tx gov submit-proposal /tmp/poc_proposal.json \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas 500000 \
  -y

# Vote on proposal
posd tx gov vote <PROPOSAL_ID> yes \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 50000omniphi \
  -y
```

**Query governance:**
```bash
# List all proposals
posd query gov proposals

# Query specific proposal
posd query gov proposal <PROPOSAL_ID>

# Query vote
posd query gov vote <PROPOSAL_ID> <VOTER_ADDRESS>

# Query tally
posd query gov tally <PROPOSAL_ID>
```

---

## Expected Results

### PoC Contribution
- **If C-Score requirements met**: Contribution created with status "pending"
- **If C-Score requirements NOT met**: Transaction fails with PoA error
- **Normal on fresh testnet**: Validator starts with 0 C-Score, so submission may fail

### Governance Proposal
- **Submission**: Should succeed if deposit ≥ 10K OMNI
- **Voting Period**: 5 minutes (testnet configuration)
- **Passing**: Requires >50% yes votes (with quorum)
- **Execution**: Automatic after voting period ends (if passed)

---

## Troubleshooting

### Node Not Starting
```bash
# Check logs
tail -f ~/.pos/node.log

# Common issues:
# 1. Genesis validation failed
posd genesis validate-genesis

# 2. Port already in use
lsof -i :26657
kill <PID>

# 3. Data corruption - reset
posd comet unsafe-reset-all
bash scripts/init_testnet.sh validator1
```

### PoC Submission Fails
```bash
# Error: "PoA verification failed"
# Cause: Insufficient C-Score or identity requirements

# Solution: Bootstrap C-Score by having validator endorse contributions
# Or adjust PoA requirements in genesis:
# - Lower minimum C-Score threshold
# - Disable identity requirements for testnet
```

### Governance Proposal Fails
```bash
# Error: "insufficient deposit"
# Check balance:
posd query bank balances <ADDRESS>

# Error: "invalid authority"
# Ensure authority is governance module address:
posd query auth module-account gov | grep address
```

### Vote Not Counted
```bash
# Check if validator is bonded
posd query staking validator <VALIDATOR_ADDRESS>

# Status should be "BOND_STATUS_BONDED"
# Only bonded validators' votes count
```

---

## Configuration Files

### Chain Configuration
- **Home directory**: `~/.pos/`
- **Genesis**: `~/.pos/config/genesis.json`
- **Config**: `~/.pos/config/config.toml`
- **App config**: `~/.pos/config/app.toml`

### Key Parameters (from genesis)

**Governance:**
```json
{
  "min_deposit": "10000000000omniphi",  // 10K OMNI
  "voting_period": "300s",               // 5 minutes
  "expedited_voting_period": "60s"       // 1 minute
}
```

**PoC (if exists):**
```json
{
  "cscore_cap": "100000",
  "decay_rate": "0.005",
  "per_block_quota": "100",
  "submission_fee": "100000"
}
```

**Staking:**
```json
{
  "unbonding_time": "1209600s",  // 14 days
  "max_validators": 125,
  "bond_denom": "omniphi"
}
```

---

## Advanced: Multi-Validator Testnet

To test with multiple validators (optional):

### Validator 2 Setup
```bash
# On second machine or different user
export POSD_HOME=$HOME/.pos2

# Initialize
bash scripts/init_testnet.sh validator2

# Copy genesis from validator1
scp validator1:~/.pos/config/genesis.json ~/.pos2/config/

# Update persistent_peers in config.toml
nano ~/.pos2/config/config.toml
# Add: persistent_peers = "validator1_node_id@validator1_ip:26656"

# Start
posd start --home $POSD_HOME
```

### Send Tokens to Validator 2
```bash
# From validator1, get validator2 address
VALIDATOR2_ADDR=$(posd keys show validator2 --keyring-backend test -a)

# Send staking tokens (500M OMNI)
posd tx bank send validator $VALIDATOR2_ADDR 500000000000000omniphi \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  -y
```

### Create Validator 2
```bash
# From validator2 machine
posd tx staking create-validator \
  --amount 50000000000000omniphi \
  --pubkey $(posd comet show-validator --home $POSD_HOME) \
  --moniker "omniphi-node-2" \
  --chain-id omniphi-testnet-2 \
  --commission-rate 0.10 \
  --commission-max-rate 0.20 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --from validator2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --home $POSD_HOME \
  -y
```

---

## Cleanup

### Reset Local Node
```bash
# Stop node
pkill posd

# Remove data (keeps config and keys)
rm -rf ~/.pos/data
rm -f ~/.pos/config/genesis.json

# Re-initialize
bash scripts/init_testnet.sh validator1
```

### Complete Cleanup
```bash
# Stop node
pkill posd

# Remove everything (INCLUDING KEYS - BACKUP FIRST!)
rm -rf ~/.pos

# Start fresh
posd keys add validator --keyring-backend test
bash scripts/init_testnet.sh validator1
```

---

## Next Steps

1. **Run Integration Tests**: Verify PoC and governance work
2. **Deploy to Cloud Testnet**: Test with persistent infrastructure
3. **Multi-Validator Testing**: Test consensus and endorsements
4. **Load Testing**: Test rate limits and throughput
5. **Security Audit**: Third-party review before mainnet

---

## Files Reference

| File | Purpose |
|------|---------|
| [scripts/init_testnet.sh](chain/scripts/init_testnet.sh) | Initialize single-node testnet |
| [scripts/test_local_integration.sh](chain/scripts/test_local_integration.sh) | Run PoC and governance tests |
| [scripts/create_proposal.sh](chain/scripts/create_proposal.sh) | Generate governance proposals |
| [.github/workflows/poc-tests.yml](.github/workflows/poc-tests.yml) | CI/CD for PoC tests |

---

**Generated**: February 3, 2026
**For**: Omniphi Local Testnet Testing
**Status**: Ready for Integration Testing
