# Test 1: Chain Stability Tests

## Objective
Verify the chain maintains consistent block times, consensus health, and can recover from validator restarts.

## Prerequisites
- Both validators running
- RPC endpoints accessible

---

## Test 1.1: Block Time Consistency

**Purpose**: Verify blocks are produced at ~4 second intervals

**Command (VPS1)**:
```bash
# Monitor 20 consecutive blocks
for i in {1..20}; do
  HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
  TIME=$(posd status 2>&1 | jq -r '.sync_info.latest_block_time')
  echo "Block $HEIGHT at $TIME"
  sleep 4
done
```

**Expected Result**:
- Block height increments by ~1 every 4 seconds
- No gaps or stalls

**Pass Criteria**: Average block time between 3.5s and 4.5s

---

## Test 1.2: Consensus Health Check

**Purpose**: Verify both validators are participating in consensus

**Command (VPS1)**:
```bash
# Check validator set
posd query staking validators --output json | jq '.validators[] | {moniker: .description.moniker, status, tokens, jailed}'

# Check signing info (missed blocks)
posd query slashing signing-infos --output json | jq '.info[] | {address, missed_blocks_counter}'
```

**Expected Result**:
- Both validators show `status: BOND_STATUS_BONDED`
- `jailed: false` for both
- `missed_blocks_counter` < 100

**Pass Criteria**: All validators bonded, none jailed

---

## Test 1.3: Block Stall Detection

**Purpose**: Verify chain doesn't stall under normal operation

**Command (VPS1)**:
```bash
# Record height, wait 60 seconds, check progress
START=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
echo "Start height: $START"
sleep 60
END=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
echo "End height: $END"
DIFF=$((END - START))
echo "Blocks produced in 60s: $DIFF (expected: ~15)"

if [ $DIFF -ge 12 ]; then
  echo "PASS: Chain producing blocks normally"
else
  echo "FAIL: Chain stalled or slow"
fi
```

**Expected Result**: 12-18 blocks produced in 60 seconds

**Pass Criteria**: >= 12 blocks in 60 seconds

---

## Test 1.4: Validator Restart & Catchup

**Purpose**: Test validator can restart and sync without issues

**Command (VPS2)**:
```bash
# Stop validator
pkill posd
sleep 5

# Verify stopped
pgrep posd && echo "FAIL: Still running" || echo "Stopped successfully"

# Restart
posd start > ~/.pos/chain.log 2>&1 &
sleep 10

# Check sync status
posd status 2>&1 | jq '{catching_up: .sync_info.catching_up, latest_block_height: .sync_info.latest_block_height}'
```

**Expected Result**:
- Validator restarts successfully
- `catching_up: false` within 30 seconds
- Block height matches VPS1

**Pass Criteria**: Validator syncs within 30 seconds

---

## Test 1.5: Compare Block Heights Across Validators

**Purpose**: Verify both validators are at same height

**Command (any machine with curl)**:
```bash
VPS1_HEIGHT=$(curl -s http://46.202.179.182:26657/status | jq -r '.result.sync_info.latest_block_height')
VPS2_HEIGHT=$(curl -s http://148.230.125.9:26657/status | jq -r '.result.sync_info.latest_block_height')

echo "VPS1: $VPS1_HEIGHT"
echo "VPS2: $VPS2_HEIGHT"
DIFF=$((VPS1_HEIGHT - VPS2_HEIGHT))
ABS_DIFF=${DIFF#-}

if [ $ABS_DIFF -le 2 ]; then
  echo "PASS: Validators in sync (diff: $ABS_DIFF)"
else
  echo "WARN: Validators differ by $ABS_DIFF blocks"
fi
```

**Expected Result**: Height difference <= 2 blocks

**Pass Criteria**: Block height difference <= 2

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 1.1 | Block Time Consistency | | |
| 1.2 | Consensus Health | | |
| 1.3 | Block Stall Detection | | |
| 1.4 | Validator Restart | | |
| 1.5 | Height Sync | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL
