# Test 2: Block Production Tests

## Objective
Validate continuous block production, gas limits, proposer rotation, and signature verification.

---

## Test 2.1: Validate 100+ Continuous Blocks

**Purpose**: Verify chain can produce 100+ blocks without issues

**Command (VPS1)**:
```bash
START=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
echo "Starting at block: $START"
TARGET=$((START + 100))
echo "Target block: $TARGET"

while true; do
  CURRENT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
  echo -ne "\rCurrent: $CURRENT / $TARGET"
  if [ $CURRENT -ge $TARGET ]; then
    echo -e "\nPASS: Reached 100+ blocks"
    break
  fi
  sleep 4
done
```

**Expected Result**: Chain reaches target block without stalling

**Pass Criteria**: 100 blocks produced consecutively

---

## Test 2.2: Verify Max Block Gas

**Purpose**: Confirm block gas limit is 60M

**Command (VPS1)**:
```bash
# Query consensus params
posd query consensus params --output json | jq '.params.block'

# Expected output:
# {
#   "max_bytes": "22020096",
#   "max_gas": "60000000"
# }
```

**Expected Result**: `max_gas: "60000000"` (60M)

**Pass Criteria**: max_gas equals 60000000

---

## Test 2.3: Verify Max Transaction Gas

**Purpose**: Confirm max tx gas is 2M

**Command (VPS1)**:
```bash
posd query feemarket params --output json | jq '{max_tx_gas, min_gas_price}'

# Expected output:
# {
#   "max_tx_gas": "2000000",
#   "min_gas_price": "0.025000000000000000"
# }
```

**Expected Result**: `max_tx_gas: "2000000"` (2M)

**Pass Criteria**: max_tx_gas equals 2000000

---

## Test 2.4: Proposer Rotation

**Purpose**: Verify block proposers rotate between validators

**Command (VPS1)**:
```bash
# Get proposer for last 10 blocks
echo "Checking proposer rotation..."
for i in {0..9}; do
  HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
  BLOCK_HEIGHT=$((HEIGHT - i))
  PROPOSER=$(curl -s http://localhost:26657/block?height=$BLOCK_HEIGHT | jq -r '.result.block.header.proposer_address')
  echo "Block $BLOCK_HEIGHT: $PROPOSER"
done

# Count unique proposers
echo ""
echo "Checking for rotation..."
UNIQUE=$(curl -s "http://localhost:26657/block?height=$HEIGHT" | jq -r '.result.block.header.proposer_address')
# Should see both validator addresses rotating
```

**Expected Result**: Multiple proposer addresses appear (rotation happening)

**Pass Criteria**: At least 2 different proposers in 10 blocks

---

## Test 2.5: Block Signature Verification

**Purpose**: Verify blocks have valid signatures from validators

**Command (VPS1)**:
```bash
# Get latest block with signatures
HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
curl -s "http://localhost:26657/block?height=$HEIGHT" | jq '.result.block.last_commit.signatures | length'

# Should return 2 (both validators signed)
```

**Expected Result**: 2 signatures per block

**Pass Criteria**: Each block has signatures from both validators

---

## Test 2.6: Block Time Analysis

**Purpose**: Analyze block time distribution

**Command (VPS1)**:
```bash
# Get timestamps for last 20 blocks
echo "Block Time Analysis"
echo "==================="

CURRENT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
PREV_TIME=""

for i in $(seq 0 19); do
  HEIGHT=$((CURRENT - i))
  TIME=$(curl -s "http://localhost:26657/block?height=$HEIGHT" | jq -r '.result.block.header.time')

  if [ -n "$PREV_TIME" ]; then
    # Calculate difference (simplified - just show times)
    echo "Block $HEIGHT: $TIME"
  fi
  PREV_TIME=$TIME
done
```

**Expected Result**: ~4 second intervals between blocks

**Pass Criteria**: Average interval between 3.5s and 4.5s

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 2.1 | 100+ Continuous Blocks | | |
| 2.2 | Max Block Gas (60M) | | |
| 2.3 | Max Tx Gas (2M) | | |
| 2.4 | Proposer Rotation | | |
| 2.5 | Block Signatures | | |
| 2.6 | Block Time Analysis | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL
