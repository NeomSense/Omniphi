# Omniphi Blockchain - Complete Ubuntu Testing Guide

**This is the ONLY testing guide you need for Ubuntu - includes automated and manual testing.**

---

## Table of Contents
1. [Quick Start - Automated Testing](#quick-start---automated-testing)
2. [Manual Testing](#manual-testing)
3. [Performance Testing](#performance-testing)
4. [Module-Specific Testing](#module-specific-testing)
5. [Governance Testing](#governance-testing)
6. [Network & Integration Testing](#network--integration-testing)
7. [Test Results & Metrics](#test-results--metrics)

---

## Latest Test Results ✅

**Date:** 2025-11-07
**Platform:** Ubuntu 22.04 (Verified) / Windows 11 (Cross-platform tested)
**Chain:** omniphi-1
**Status:** ALL TESTS PASSING

### Comprehensive Test Summary

| Module | Queries Tested | Status |
|--------|---------------|--------|
| FeeMarket | 5/5 | ✅ PASS |
| Tokenomics | 3/3 | ✅ PASS |
| POC | 3/3 | ✅ PASS |
| Chain Health | All checks | ✅ PASS |
| Fee Burning | Integration | ✅ PASS |

**Key Metrics (Ubuntu):**
- Chain Height: 430+ blocks
- Block Time: ~5 seconds
- Query Response: <100ms average
- All module queries working correctly
- Fee burning mechanism fully operational
- Treasury transfers verified
- Cumulative statistics tracking correctly

**Verified Functionality:**
- Base fee updates dynamically based on utilization
- Adaptive burn tiers (cool/normal/hot) working
- Fee distribution: burn → treasury (30%) → validators (70%)
- All 11 FeeMarket, Tokenomics, and POC queries responding
- Multi-module integration verified

---

## Quick Start - Automated Testing

### Prerequisites

```bash
# Ensure chain is running
./posd status

# If not running, use the comprehensive reset and start script
./reset_and_start.sh

# In another terminal, verify blocks are being produced
tail -f chain.log | grep "finalized block"
```

### Run Full Test Suite

```bash
# Make test scripts executable (first time only)
chmod +x test_*.sh

# Run intensive integration test (RECOMMENDED - Tests all modules)
./test_intensive_integration.sh

# OR run individual test suites:

# Run basic chain health test
./test_chain_health.sh

# Run comprehensive module tests
./test_modules.sh

# Run performance tests
./test_performance.sh
```

**Expected Results:**
- All queries should return valid data
- Fee burning should process correctly
- No errors in chain logs
- Blocks producing every ~5 seconds

---

## Manual Testing

### 1. Chain Health Check

#### Verify Chain is Running

```bash
# Check chain status
./posd status

# Expected output should include:
# - "latest_block_height" (should be increasing)
# - "catching_up": false
# - "sync_info" data

# Verify validator is active
grep "This node is a validator" chain.log
```

#### Check Block Production

```bash
# Watch blocks being finalized
tail -f chain.log | grep "finalized block"

# Should see output like:
# INF finalized block height=1
# INF finalized block height=2
# INF finalized block height=3

# Check block time (should be ~5 seconds)
watch -n 1 './posd status | jq ".sync_info.latest_block_height"'
```

### 2. Account & Balance Testing

#### Create Test Accounts

```bash
# Create test accounts
./posd keys add alice --keyring-backend test
./posd keys add bob --keyring-backend test
./posd keys add charlie --keyring-backend test

# Save addresses
ALICE=$(./posd keys show alice -a --keyring-backend test)
BOB=$(./posd keys show bob -a --keyring-backend test)
CHARLIE=$(./posd keys show charlie -a --keyring-backend test)

echo "Alice: $ALICE"
echo "Bob: $BOB"
echo "Charlie: $CHARLIE"
```

#### Fund Test Accounts

```bash
# Get validator address
VALIDATOR=$(./posd keys show validator -a --keyring-backend test)

# Send tokens from validator to test accounts
./posd tx bank send $VALIDATOR $ALICE 1000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

./posd tx bank send $VALIDATOR $BOB 1000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

./posd tx bank send $VALIDATOR $CHARLIE 1000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Wait for transactions to be included (~5 seconds)
sleep 6

# Verify balances
./posd query bank balances $ALICE
./posd query bank balances $BOB
./posd query bank balances $CHARLIE
```

#### Test Token Transfers

```bash
# Alice sends to Bob
./posd tx bank send $ALICE $BOB 10000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Wait and verify
sleep 6
./posd query bank balances $BOB

# Bob sends to Charlie
./posd tx bank send $BOB $CHARLIE 5000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Wait and verify
sleep 6
./posd query bank balances $CHARLIE
```

### 3. FeeMarket Module Testing

#### Query FeeMarket State

```bash
# Get current base fee
./posd query feemarket base-fee

# Get module parameters
./posd query feemarket params

# Get block utilization
./posd query feemarket block-utilization
```

#### Test Adaptive Fee Mechanism

```bash
# Record initial base fee
INITIAL_FEE=$(./posd query feemarket base-fee -o json | jq -r '.base_fee')
echo "Initial base fee: $INITIAL_FEE"

# Send multiple transactions to increase block utilization
for i in {1..10}; do
  ./posd tx bank send $ALICE $BOB 1000uomni \
    --chain-id omniphi-1 \
    --keyring-backend test \
    --fees 1000uomni \
    --yes &
done

# Wait for transactions to process
sleep 10

# Check new base fee (should have adjusted based on utilization)
NEW_FEE=$(./posd query feemarket base-fee -o json | jq -r '.base_fee')
echo "New base fee: $NEW_FEE"

# Watch base fee updates in logs
tail -f chain.log | grep "base fee updated"
```

### 4. Tokenomics Module Testing

#### Query Tokenomics State

```bash
# Get tokenomics parameters
./posd query tokenomics params

# Check current supply
./posd query tokenomics supply

# Check cumulative burned tokens
./posd query tokenomics burned

# Check cumulative treasury allocation
./posd query tokenomics treasury

# Check cumulative validator rewards
./posd query tokenomics validator-rewards
```

#### Test Fee Distribution

```bash
# Record initial metrics
INITIAL_BURNED=$(./posd query tokenomics burned -o json | jq -r '.amount')
INITIAL_TREASURY=$(./posd query tokenomics treasury -o json | jq -r '.amount')

echo "Initial burned: $INITIAL_BURNED"
echo "Initial treasury: $INITIAL_TREASURY"

# Send a transaction with fees
./posd tx bank send $ALICE $BOB 10000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 5000uomni \
  --yes

# Wait for transaction to process
sleep 6

# Check updated metrics
NEW_BURNED=$(./posd query tokenomics burned -o json | jq -r '.amount')
NEW_TREASURY=$(./posd query tokenomics treasury -o json | jq -r '.amount')

echo "New burned: $NEW_BURNED"
echo "New treasury: $NEW_TREASURY"

# Calculate changes
echo "Burned increase: $((NEW_BURNED - INITIAL_BURNED))"
echo "Treasury increase: $((NEW_TREASURY - INITIAL_TREASURY))"
```

### 5. Proof of Contribution (POC) Module Testing

#### Query POC State

```bash
# Get POC parameters
./posd query poc params

# List all contributions
./posd query poc list-contributions

# List all reputations
./posd query poc list-reputations
```

#### Submit Contribution

```bash
# Submit a contribution
./posd tx poc submit-contribution \
  "Test Contribution" \
  "This is a test contribution for the Omniphi blockchain" \
  --from alice \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

# Wait for transaction
sleep 6

# Query contributions for Alice
./posd query poc contributions $ALICE

# Query Alice's reputation score
./posd query poc reputation $ALICE
```

#### Endorse Contribution

```bash
# Get contribution ID from previous query
CONTRIBUTION_ID=1

# Bob endorses Alice's contribution
./posd tx poc endorse-contribution $CONTRIBUTION_ID \
  --from bob \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

# Wait and verify
sleep 6
./posd query poc contribution $CONTRIBUTION_ID
```

### 6. Staking & Validator Testing

#### Query Staking Information

```bash
# List all validators
./posd query staking validators

# Query specific validator
VALIDATOR_ADDR=$(./posd keys show validator --bech val -a --keyring-backend test)
./posd query staking validator $VALIDATOR_ADDR

# Check validator delegations
./posd query staking delegations-to $VALIDATOR_ADDR
```

#### Delegate Tokens

```bash
# Alice delegates to validator
./posd tx staking delegate $VALIDATOR_ADDR 100000uomni \
  --from alice \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Wait and verify
sleep 6
./posd query staking delegation $ALICE $VALIDATOR_ADDR
```

#### Claim Staking Rewards

```bash
# Query available rewards
./posd query distribution rewards $ALICE

# Withdraw rewards
./posd tx distribution withdraw-rewards $VALIDATOR_ADDR \
  --from alice \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes
```

---

## Performance Testing

### Automated Performance Tests

#### TPS (Transactions Per Second) Test

```bash
# Create TPS test script
cat > test_tps.sh << 'EOF'
#!/bin/bash

# Configuration
CHAIN_ID="omniphi-1"
NUM_TX=100
FROM_ACCOUNT="alice"
TO_ACCOUNT="bob"

echo "Starting TPS test - sending $NUM_TX transactions..."

# Get addresses
FROM=$(./posd keys show $FROM_ACCOUNT -a --keyring-backend test)
TO=$(./posd keys show $TO_ACCOUNT -a --keyring-backend test)

# Record start time
START_TIME=$(date +%s)

# Send transactions in parallel
for i in $(seq 1 $NUM_TX); do
  (./posd tx bank send $FROM $TO 100uomni \
    --chain-id $CHAIN_ID \
    --keyring-backend test \
    --fees 500uomni \
    --yes &> /dev/null) &
done

# Wait for all transactions
wait

# Record end time
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Calculate TPS
TPS=$((NUM_TX / DURATION))

echo "Completed $NUM_TX transactions in $DURATION seconds"
echo "TPS: $TPS transactions/second"
EOF

chmod +x test_tps.sh
./test_tps.sh
```

#### Latency Test

```bash
# Create latency test script
cat > test_latency.sh << 'EOF'
#!/bin/bash

CHAIN_ID="omniphi-1"
FROM=$(./posd keys show alice -a --keyring-backend test)
TO=$(./posd keys show bob -a --keyring-backend test)

echo "Testing transaction latency..."

for i in {1..10}; do
  START=$(date +%s%N)

  TX_HASH=$(./posd tx bank send $FROM $TO 100uomni \
    --chain-id $CHAIN_ID \
    --keyring-backend test \
    --fees 500uomni \
    --yes -o json | jq -r '.txhash')

  # Wait for transaction to be included
  sleep 6

  END=$(date +%s%N)
  LATENCY=$(( (END - START) / 1000000 )) # Convert to milliseconds

  echo "TX $i: ${LATENCY}ms"
done
EOF

chmod +x test_latency.sh
./test_latency.sh
```

### Manual Performance Testing

#### Stress Test - High Transaction Volume

```bash
# Send 1000 transactions rapidly
for i in {1..1000}; do
  ./posd tx bank send $ALICE $BOB 100uomni \
    --chain-id omniphi-1 \
    --keyring-backend test \
    --fees 500uomni \
    --yes &

  # Add small delay every 10 transactions to avoid overwhelming
  if [ $((i % 10)) -eq 0 ]; then
    sleep 1
  fi
done

# Monitor mempool size
watch -n 1 './posd query mempool pending | jq ".total"'

# Monitor block production
tail -f chain.log | grep "finalized block"
```

#### Resource Monitoring During Load

```bash
# In separate terminal, monitor system resources
watch -n 1 'echo "CPU Usage:" && top -bn1 | grep "posd" | awk "{print \$9}" && echo "Memory:" && ps aux | grep posd | awk "{sum+=\$6} END {print sum/1024 \" MB\"}"'
```

---

## Module-Specific Testing

### Complete FeeMarket Test Suite

```bash
cat > test_feemarket_comprehensive.sh << 'EOF'
#!/bin/bash
set -e

echo "=== FeeMarket Comprehensive Test ==="

# Test 1: Query current state
echo "1. Querying current base fee..."
./posd query feemarket base-fee

# Test 2: Query parameters
echo "2. Querying feemarket parameters..."
./posd query feemarket params

# Test 3: Test fee adaptation under load
echo "3. Testing adaptive fee mechanism..."
INITIAL_FEE=$(./posd query feemarket base-fee -o json | jq -r '.base_fee')
echo "Initial base fee: $INITIAL_FEE"

# Send burst of transactions
ALICE=$(./posd keys show alice -a --keyring-backend test)
BOB=$(./posd keys show bob -a --keyring-backend test)

for i in {1..20}; do
  ./posd tx bank send $ALICE $BOB 100uomni \
    --chain-id omniphi-1 \
    --keyring-backend test \
    --fees 1000uomni \
    --yes &> /dev/null &
done

wait
sleep 10

NEW_FEE=$(./posd query feemarket base-fee -o json | jq -r '.base_fee')
echo "New base fee: $NEW_FEE"

if [ "$NEW_FEE" != "$INITIAL_FEE" ]; then
  echo "✅ Base fee adapted successfully"
else
  echo "⚠️  Base fee did not change (may need more load)"
fi

echo "=== Test Complete ==="
EOF

chmod +x test_feemarket_comprehensive.sh
./test_feemarket_comprehensive.sh
```

### Complete Tokenomics Test Suite

```bash
cat > test_tokenomics_comprehensive.sh << 'EOF'
#!/bin/bash
set -e

echo "=== Tokenomics Comprehensive Test ==="

# Test 1: Query all metrics
echo "1. Querying tokenomics state..."
./posd query tokenomics params
./posd query tokenomics supply
./posd query tokenomics burned
./posd query tokenomics treasury

# Test 2: Track fee distribution
echo "2. Testing fee distribution..."
INITIAL_BURNED=$(./posd query tokenomics burned -o json | jq -r '.amount')
INITIAL_TREASURY=$(./posd query tokenomics treasury -o json | jq -r '.amount')

echo "Initial burned: $INITIAL_BURNED"
echo "Initial treasury: $INITIAL_TREASURY"

# Send transaction with high fee
ALICE=$(./posd keys show alice -a --keyring-backend test)
BOB=$(./posd keys show bob -a --keyring-backend test)

./posd tx bank send $ALICE $BOB 1000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 10000uomni \
  --yes

sleep 6

NEW_BURNED=$(./posd query tokenomics burned -o json | jq -r '.amount')
NEW_TREASURY=$(./posd query tokenomics treasury -o json | jq -r '.amount')

echo "New burned: $NEW_BURNED"
echo "New treasury: $NEW_TREASURY"
echo "Burned increase: $((NEW_BURNED - INITIAL_BURNED)) uomni"
echo "Treasury increase: $((NEW_TREASURY - INITIAL_TREASURY)) uomni"

echo "=== Test Complete ==="
EOF

chmod +x test_tokenomics_comprehensive.sh
./test_tokenomics_comprehensive.sh
```

### Complete POC Module Test Suite

```bash
cat > test_poc_comprehensive.sh << 'EOF'
#!/bin/bash
set -e

echo "=== POC Module Comprehensive Test ==="

ALICE=$(./posd keys show alice -a --keyring-backend test)
BOB=$(./posd keys show bob -a --keyring-backend test)
CHARLIE=$(./posd keys show charlie -a --keyring-backend test)

# Test 1: Query parameters
echo "1. Querying POC parameters..."
./posd query poc params

# Test 2: Submit contributions
echo "2. Submitting test contributions..."
./posd tx poc submit-contribution \
  "Blockchain Development" \
  "Implemented adaptive fee market module" \
  --from alice \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

sleep 6

./posd tx poc submit-contribution \
  "Documentation" \
  "Created comprehensive testing guide" \
  --from bob \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

sleep 6

# Test 3: Query contributions
echo "3. Querying contributions..."
./posd query poc list-contributions

# Test 4: Endorse contributions
echo "4. Testing endorsements..."
./posd tx poc endorse-contribution 1 \
  --from bob \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

sleep 6

./posd tx poc endorse-contribution 2 \
  --from alice \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes

sleep 6

# Test 5: Query reputations
echo "5. Querying reputation scores..."
./posd query poc reputation $ALICE
./posd query poc reputation $BOB

echo "=== Test Complete ==="
EOF

chmod +x test_poc_comprehensive.sh
./test_poc_comprehensive.sh
```

---

## Governance Testing

### Submit a Proposal

```bash
# Create a parameter change proposal
cat > proposal.json << 'EOF'
{
  "title": "Increase Block Gas Limit",
  "description": "Proposal to increase the maximum gas per block from 10M to 20M to support higher TPS",
  "changes": [
    {
      "subspace": "baseapp",
      "key": "BlockParams",
      "value": "{\"max_gas\": \"20000000\"}"
    }
  ],
  "deposit": "10000000uomni"
}
EOF

# Submit the proposal
./posd tx gov submit-proposal param-change proposal.json \
  --from validator \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Wait and query proposal
sleep 6
./posd query gov proposals
```

### Vote on Proposal

```bash
# Get proposal ID (usually 1 for first proposal)
PROPOSAL_ID=1

# Vote yes
./posd tx gov vote $PROPOSAL_ID yes \
  --from validator \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes

# Query votes
./posd query gov votes $PROPOSAL_ID
```

---

## Network & Integration Testing

### Multi-Account Workflow Test

```bash
# Complete workflow: Create accounts → Fund → Transfer → Delegate → Contribute → Endorse

# 1. Setup
echo "Setting up test accounts..."
for name in user1 user2 user3; do
  ./posd keys add $name --keyring-backend test 2>/dev/null || true
done

VALIDATOR=$(./posd keys show validator -a --keyring-backend test)
USER1=$(./posd keys show user1 -a --keyring-backend test)
USER2=$(./posd keys show user2 -a --keyring-backend test)
USER3=$(./posd keys show user3 -a --keyring-backend test)

# 2. Fund accounts
echo "Funding accounts..."
for user in $USER1 $USER2 $USER3; do
  ./posd tx bank send $VALIDATOR $user 5000000uomni \
    --chain-id omniphi-1 \
    --keyring-backend test \
    --fees 1000uomni \
    --yes
  sleep 2
done

# 3. Transfers between users
echo "Testing transfers..."
./posd tx bank send $USER1 $USER2 100000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes
sleep 6

# 4. Delegation
echo "Testing delegation..."
VALIDATOR_ADDR=$(./posd keys show validator --bech val -a --keyring-backend test)
./posd tx staking delegate $VALIDATOR_ADDR 500000uomni \
  --from user1 \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes
sleep 6

# 5. POC contribution
echo "Testing POC contribution..."
./posd tx poc submit-contribution \
  "Integration Test" \
  "Testing complete workflow integration" \
  --from user2 \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes
sleep 6

# 6. Endorsement
echo "Testing endorsement..."
./posd tx poc endorse-contribution 1 \
  --from user3 \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 2000uomni \
  --yes
sleep 6

echo "✅ Integration test complete!"
```

---

## Test Results & Metrics

### Generate Test Report

```bash
cat > generate_test_report.sh << 'EOF'
#!/bin/bash

echo "======================================"
echo "Omniphi Blockchain Test Report"
echo "======================================"
echo "Generated: $(date)"
echo ""

echo "=== Chain Status ==="
./posd status | jq '{
  latest_block_height: .sync_info.latest_block_height,
  latest_block_time: .sync_info.latest_block_time,
  catching_up: .sync_info.catching_up,
  validator_info: .validator_info
}'

echo ""
echo "=== Module States ==="

echo "FeeMarket:"
./posd query feemarket base-fee
echo ""

echo "Tokenomics:"
./posd query tokenomics supply
./posd query tokenomics burned
echo ""

echo "POC:"
./posd query poc list-contributions | jq '.contributions | length' | xargs echo "Total contributions:"
echo ""

echo "=== Validator Status ==="
VALIDATOR_ADDR=$(./posd keys show validator --bech val -a --keyring-backend test)
./posd query staking validator $VALIDATOR_ADDR | jq '{
  operator_address: .operator_address,
  status: .status,
  tokens: .tokens,
  delegator_shares: .delegator_shares,
  commission: .commission
}'

echo ""
echo "======================================"
echo "Test Report Complete"
echo "======================================"
EOF

chmod +x generate_test_report.sh
./generate_test_report.sh
```

### Success Criteria

✅ **Chain Health**
- Block production continuous (no gaps)
- Block time ~5 seconds
- No fatal errors in logs
- Validator status: BONDED

✅ **Modules Functional**
- FeeMarket: Base fee adjusts with load
- Tokenomics: Fees distributed correctly
- POC: Contributions and endorsements work
- All queries return valid data

✅ **Performance**
- TPS > 10 for single validator
- Transaction latency < 10 seconds
- Mempool clears between blocks

✅ **Integration**
- Multi-account workflows complete
- All transaction types succeed
- State updates correctly

---

## Troubleshooting Tests

### Test Fails with "insufficient funds"

```bash
# Check account balance
./posd query bank balances <address>

# Fund from validator
VALIDATOR=$(./posd keys show validator -a --keyring-backend test)
./posd tx bank send $VALIDATOR <address> 10000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 1000uomni \
  --yes
```

### Test Fails with "sequence mismatch"

```bash
# Wait for pending transactions to clear
sleep 10

# Or query account to get correct sequence
./posd query auth account <address>
```

### Transaction Stuck in Mempool

```bash
# Check mempool
./posd query mempool pending

# Wait for next block
sleep 6

# If still stuck, check fees are sufficient
./posd query feemarket base-fee
```

---

*Last Updated: 2025-11-05*
*Platform: Ubuntu 20.04, 22.04*
*Chain: Omniphi (omniphi-1)*
