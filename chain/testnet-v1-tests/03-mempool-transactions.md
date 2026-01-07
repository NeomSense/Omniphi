# Test 3: Mempool & Transaction Tests

## Objective
Test transaction submission, mempool behavior, gas accounting, and various transaction types.

## Prerequisites
- Validator key with funds: `validator`
- Test account with funds (create if needed)

---

## Test 3.0: Setup Test Account

**Purpose**: Create a test account for transactions

**Command (VPS1)**:
```bash
# Create test account
posd keys add testaccount --keyring-backend test 2>&1 | tee testaccount.txt

# Get address
TEST_ADDR=$(posd keys show testaccount -a --keyring-backend test)
echo "Test account: $TEST_ADDR"

# Fund from validator (send 1000 OMNI = 1,000,000,000,000 omniphi)
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
posd tx bank send $VALIDATOR_ADDR $TEST_ADDR 1000000000000omniphi \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

# Wait for confirmation
sleep 8

# Check balance
posd query bank balances $TEST_ADDR
```

---

## Test 3.1: Basic Send Transaction

**Purpose**: Test simple token transfer

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
TEST_ADDR=$(posd keys show testaccount -a --keyring-backend test)

# Send 100 OMNI
posd tx bank send $VALIDATOR_ADDR $TEST_ADDR 100000000omniphi \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

# Check tx result
sleep 8
posd query bank balances $TEST_ADDR
```

**Expected Result**:
- Transaction succeeds
- Balance increases by 100 OMNI

**Pass Criteria**: Transaction confirmed, balance updated

---

## Test 3.2: Delegate Transaction

**Purpose**: Test staking delegation

**Command (VPS1)**:
```bash
# Get validator operator address
VALOPER=$(posd query staking validators --output json | jq -r '.validators[0].operator_address')
echo "Delegating to: $VALOPER"

# Delegate 10 OMNI from test account
posd tx staking delegate $VALOPER 10000000omniphi \
  --from testaccount \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8

# Check delegation
TEST_ADDR=$(posd keys show testaccount -a --keyring-backend test)
posd query staking delegations $TEST_ADDR
```

**Expected Result**: Delegation created successfully

**Pass Criteria**: Delegation shows in query output

---

## Test 3.3: Undelegate Transaction

**Purpose**: Test staking undelegation

**Command (VPS1)**:
```bash
VALOPER=$(posd query staking validators --output json | jq -r '.validators[0].operator_address')

# Undelegate 5 OMNI
posd tx staking unbond $VALOPER 5000000omniphi \
  --from testaccount \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8

# Check unbonding
TEST_ADDR=$(posd keys show testaccount -a --keyring-backend test)
posd query staking unbonding-delegations $TEST_ADDR
```

**Expected Result**: Unbonding entry created

**Pass Criteria**: Unbonding delegation appears in query

---

## Test 3.4: Withdraw Rewards

**Purpose**: Test reward withdrawal

**Command (VPS1)**:
```bash
# Check pending rewards
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
posd query distribution rewards $VALIDATOR_ADDR

# Withdraw all rewards
posd tx distribution withdraw-all-rewards \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8
echo "Rewards withdrawn"
```

**Expected Result**: Rewards withdrawn successfully

**Pass Criteria**: Transaction succeeds

---

## Test 3.5: Batch Transaction Submission (50 txs)

**Purpose**: Test mempool handling under load

**Command (VPS1)**:
```bash
echo "Submitting 50 transactions..."
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
TEST_ADDR=$(posd keys show testaccount -a --keyring-backend test)

for i in {1..50}; do
  posd tx bank send $VALIDATOR_ADDR $TEST_ADDR 1000omniphi \
    --from validator \
    --chain-id omniphi-testnet-1 \
    --keyring-backend test \
    --fees 100000omniphi \
    --gas 100000 \
    -y --broadcast-mode async &

  if [ $((i % 10)) -eq 0 ]; then
    echo "Submitted $i/50 transactions"
    sleep 1
  fi
done

wait
echo "All transactions submitted"

# Check mempool
sleep 2
curl -s http://localhost:26657/num_unconfirmed_txs | jq '.result.n_txs'
```

**Expected Result**:
- All transactions submitted
- Mempool processes without deadlock
- Transactions confirm within a few blocks

**Pass Criteria**: Mempool clears within 30 seconds

---

## Test 3.6: Gas Accounting Verification

**Purpose**: Verify gas is deducted correctly

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Get balance before
BEFORE=$(posd query bank balances $VALIDATOR_ADDR --output json | jq -r '.balances[0].amount')
echo "Balance before: $BEFORE"

# Send tx with known gas
posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1omniphi \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 200000omniphi \
  --gas 100000 \
  -y

sleep 8

# Get balance after
AFTER=$(posd query bank balances $VALIDATOR_ADDR --output json | jq -r '.balances[0].amount')
echo "Balance after: $AFTER"

# Calculate difference (should be ~200000 for fees)
DIFF=$((BEFORE - AFTER))
echo "Difference: $DIFF omniphi (expected: ~200000 for fees)"
```

**Expected Result**: Balance decreased by fee amount

**Pass Criteria**: Fee deduction matches specified amount

---

## Test 3.7: Mempool Status Check

**Purpose**: Verify mempool health

**Command (VPS1)**:
```bash
# Check mempool size
curl -s http://localhost:26657/num_unconfirmed_txs | jq '.result'

# Check mempool contents (if any)
curl -s http://localhost:26657/unconfirmed_txs | jq '.result.n_txs'
```

**Expected Result**:
- `n_txs: 0` (empty when idle)
- No stuck transactions

**Pass Criteria**: Mempool is healthy and clears

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 3.0 | Setup Test Account | | |
| 3.1 | Basic Send | | |
| 3.2 | Delegate | | |
| 3.3 | Undelegate | | |
| 3.4 | Withdraw Rewards | | |
| 3.5 | Batch 50 Txs | | |
| 3.6 | Gas Accounting | | |
| 3.7 | Mempool Status | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL
