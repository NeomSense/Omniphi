# Test 5: PoC (Proof of Contribution) Module Tests

## Objective
Test PoC submission, validation, cooldowns, fee deduction, and C-Score functionality.

---

## Test 5.0: Query PoC Parameters

**Purpose**: Check PoC module configuration

**Command (VPS1)**:
```bash
posd query poc params --output json | jq '.'

# Expected fields:
# - submission_fee
# - max_hash_length
# - max_uri_length
# - cooldown_blocks
# - min_stake_to_submit
```

**Expected Result**: PoC parameters are set and queryable

**Pass Criteria**: Parameters returned successfully

---

## Test 5.1: Valid PoC Submission

**Purpose**: Submit a valid proof of contribution

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Submit a valid PoC
posd tx poc submit \
  --hash "sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456" \
  --uri "https://github.com/omniphi/contribution/1" \
  --description "Test contribution for Phase 1 testing" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8

# Query submissions
echo "Querying PoC submissions..."
posd query poc submissions --output json | jq '.'
```

**Expected Result**: Submission recorded on-chain

**Pass Criteria**: Submission appears in query

---

## Test 5.2: Hash Length Enforcement

**Purpose**: Test that invalid hash lengths are rejected

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Try too short hash (should fail)
echo "Testing short hash (should fail)..."
posd tx poc submit \
  --hash "sha256:tooshort" \
  --uri "https://example.com/test" \
  --description "Invalid hash test" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y 2>&1

# Try too long hash (should fail)
echo ""
echo "Testing too-long hash (should fail)..."
posd tx poc submit \
  --hash "sha256:$(head -c 500 /dev/urandom | base64 | tr -d '\n' | head -c 500)" \
  --uri "https://example.com/test" \
  --description "Invalid hash test" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y 2>&1
```

**Expected Result**: Both rejected with hash length error

**Pass Criteria**: Invalid hashes rejected

---

## Test 5.3: URI Length Enforcement

**Purpose**: Test URI length limits

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Try too long URI (should fail)
LONG_URI="https://example.com/$(head -c 2000 /dev/urandom | base64 | tr -d '\n/' | head -c 2000)"
echo "Testing too-long URI (should fail)..."
posd tx poc submit \
  --hash "sha256:validhash1234567890123456789012345678901234567890123456789012" \
  --uri "$LONG_URI" \
  --description "Test" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y 2>&1
```

**Expected Result**: Rejected with URI length error

**Pass Criteria**: Long URIs rejected

---

## Test 5.4: Cooldown Enforcement

**Purpose**: Test submission cooldown period

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# First submission
echo "First submission..."
posd tx poc submit \
  --hash "sha256:cooldowntest123456789012345678901234567890123456789012345678" \
  --uri "https://example.com/cooldown1" \
  --description "Cooldown test 1" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8

# Immediate second submission (should fail if cooldown active)
echo ""
echo "Immediate second submission (testing cooldown)..."
posd tx poc submit \
  --hash "sha256:cooldowntest223456789012345678901234567890123456789012345678" \
  --uri "https://example.com/cooldown2" \
  --description "Cooldown test 2" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y 2>&1

# Check for cooldown error or success (depends on cooldown setting)
```

**Expected Result**: Second submission rejected if within cooldown

**Pass Criteria**: Cooldown enforced correctly

---

## Test 5.5: Fee Deduction

**Purpose**: Verify PoC submission fee is deducted

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Get balance before
BEFORE=$(posd query bank balances $VALIDATOR_ADDR --output json | jq -r '.balances[0].amount')
echo "Balance before: $BEFORE"

# Submit PoC
posd tx poc submit \
  --hash "sha256:feetest12345678901234567890123456789012345678901234567890123" \
  --uri "https://example.com/feetest" \
  --description "Fee deduction test" \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 500000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y

sleep 8

# Get balance after
AFTER=$(posd query bank balances $VALIDATOR_ADDR --output json | jq -r '.balances[0].amount')
echo "Balance after: $AFTER"

DIFF=$((BEFORE - AFTER))
echo "Fee deducted: $DIFF omniphi"
```

**Expected Result**: Balance decreased by submission fee + gas

**Pass Criteria**: Fee correctly deducted

---

## Test 5.6: On-Chain Storage Verification

**Purpose**: Verify PoC data stored correctly

**Command (VPS1)**:
```bash
# Query all submissions
echo "All PoC submissions:"
posd query poc submissions --output json | jq '.submissions[]'

# Query by contributor
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
echo ""
echo "Submissions by $VALIDATOR_ADDR:"
posd query poc submissions-by-contributor $VALIDATOR_ADDR --output json 2>&1 | jq '.'
```

**Expected Result**: Submissions stored with correct data

**Pass Criteria**: All submitted data retrievable

---

## Test 5.7: Batch PoC Submissions (20 valid)

**Purpose**: Test multiple valid submissions

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

echo "Submitting 20 PoC entries..."
for i in {1..20}; do
  HASH="sha256:batch${i}test$(printf '%050d' $i)"
  posd tx poc submit \
    --hash "$HASH" \
    --uri "https://example.com/batch/$i" \
    --description "Batch test $i" \
    --from validator \
    --chain-id omniphi-testnet-1 \
    --keyring-backend test \
    --fees 500000omniphi \
    --gas 200000 \
    -y --broadcast-mode async > /dev/null 2>&1

  echo "Submitted $i/20"
  sleep 1  # Avoid cooldown issues
done

echo "Waiting for confirmations..."
sleep 30

# Count submissions
posd query poc submissions --output json | jq '.submissions | length'
```

**Expected Result**: All 20 submissions recorded

**Pass Criteria**: >= 15 submissions confirmed

---

## Test 5.8: Invalid PoC Submissions (5 invalid)

**Purpose**: Test rejection of invalid submissions

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

echo "Testing invalid submissions..."

# 1. Empty hash
echo "1. Empty hash:"
posd tx poc submit --hash "" --uri "https://x.com" --description "test" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 500000omniphi --gas 200000 -y 2>&1 | grep -i "error\|fail" || echo "Submitted (unexpected)"

# 2. Empty URI
echo "2. Empty URI:"
posd tx poc submit --hash "sha256:valid123456789012345678901234567890123456789012345678901234" --uri "" --description "test" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 500000omniphi --gas 200000 -y 2>&1 | grep -i "error\|fail" || echo "Submitted (unexpected)"

# 3. Duplicate hash (submit same hash twice)
echo "3. Duplicate hash:"
DUPE_HASH="sha256:duplicate12345678901234567890123456789012345678901234567890"
posd tx poc submit --hash "$DUPE_HASH" --uri "https://x.com/1" --description "first" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 500000omniphi --gas 200000 -y > /dev/null 2>&1
sleep 8
posd tx poc submit --hash "$DUPE_HASH" --uri "https://x.com/2" --description "duplicate" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 500000omniphi --gas 200000 -y 2>&1 | grep -i "error\|fail\|already" || echo "Submitted (check if dupe allowed)"

# 4. Zero fee
echo "4. Zero fee:"
posd tx poc submit --hash "sha256:zerofee12345678901234567890123456789012345678901234567890" --uri "https://x.com" --description "test" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 0omniphi --gas 200000 -y 2>&1 | grep -i "error\|fail\|insufficient" || echo "Submitted (unexpected)"

# 5. Malformed hash prefix
echo "5. Malformed hash:"
posd tx poc submit --hash "md5:invalidprefix" --uri "https://x.com" --description "test" \
  --from validator --chain-id omniphi-testnet-1 --keyring-backend test \
  --fees 500000omniphi --gas 200000 -y 2>&1 | grep -i "error\|fail\|invalid" || echo "Submitted (check hash validation)"
```

**Expected Result**: All 5 submissions rejected

**Pass Criteria**: Invalid submissions rejected with appropriate errors

---

## Test 5.9: C-Score Query

**Purpose**: Check contributor scores

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Query C-Score
echo "C-Score for validator:"
posd query poc cscore $VALIDATOR_ADDR --output json 2>&1 | jq '.'

# Query leaderboard
echo ""
echo "PoC Leaderboard:"
posd query poc leaderboard --output json 2>&1 | jq '.'
```

**Expected Result**: C-Score reflects contributions

**Pass Criteria**: C-Score queryable and accurate

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 5.0 | PoC Params | | |
| 5.1 | Valid Submission | | |
| 5.2 | Hash Length | | |
| 5.3 | URI Length | | |
| 5.4 | Cooldown | | |
| 5.5 | Fee Deduction | | |
| 5.6 | On-Chain Storage | | |
| 5.7 | Batch 20 Valid | | |
| 5.8 | 5 Invalid | | |
| 5.9 | C-Score | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL
