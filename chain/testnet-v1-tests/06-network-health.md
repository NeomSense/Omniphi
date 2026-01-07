# Test 6: Network Health Tests

## Objective
Test peer discovery, validator sync, fault tolerance, and network recovery.

---

## Test 6.1: Peer Discovery

**Purpose**: Verify validators discover and connect to each other

**Command (VPS1)**:
```bash
# Check connected peers
echo "Connected peers on VPS1:"
curl -s http://localhost:26657/net_info | jq '.result.n_peers'
curl -s http://localhost:26657/net_info | jq '.result.peers[] | {id: .node_info.id, moniker: .node_info.moniker, remote_ip: .remote_ip}'
```

**Command (VPS2)**:
```bash
# Check connected peers
echo "Connected peers on VPS2:"
curl -s http://localhost:26657/net_info | jq '.result.n_peers'
curl -s http://localhost:26657/net_info | jq '.result.peers[] | {id: .node_info.id, moniker: .node_info.moniker, remote_ip: .remote_ip}'
```

**Expected Result**:
- VPS1 shows VPS2 as peer (and vice versa)
- `n_peers >= 1`

**Pass Criteria**: Each validator has at least 1 peer

---

## Test 6.2: Validator Restart and Sync

**Purpose**: Test validator can restart and catch up

**Command (VPS2)**:
```bash
# Record current height
BEFORE=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
echo "Height before restart: $BEFORE"

# Stop validator
pkill posd
echo "Validator stopped"
sleep 10

# Restart
posd start > ~/.pos/chain.log 2>&1 &
echo "Validator restarting..."

# Wait and check sync
sleep 15
AFTER=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
CATCHING=$(posd status 2>&1 | jq -r '.sync_info.catching_up')

echo "Height after restart: $AFTER"
echo "Catching up: $CATCHING"

if [ "$CATCHING" = "false" ]; then
  echo "PASS: Validator synced successfully"
else
  echo "WARN: Still catching up (may need more time)"
fi
```

**Expected Result**: Validator syncs within 30 seconds

**Pass Criteria**: `catching_up: false` within 30 seconds

---

## Test 6.3: Single Validator Failure (Panic Test)

**Purpose**: Verify chain continues when one validator goes offline

**Command (VPS1 - monitor while VPS2 is killed)**:
```bash
# Start monitoring block production
echo "Monitoring blocks while VPS2 is killed..."
echo "Current height:"
posd status 2>&1 | jq -r '.sync_info.latest_block_height'

# Note: Have someone kill VPS2 now
# On VPS2: pkill posd

# Continue monitoring for 60 seconds
for i in {1..15}; do
  sleep 4
  HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
  echo "Height: $HEIGHT"
done
```

**On VPS2** (kill command):
```bash
pkill posd
echo "VPS2 validator killed"
# Wait 60 seconds
sleep 60
# Restart
posd start > ~/.pos/chain.log 2>&1 &
```

**Expected Result**:
- With 2 validators (67% + 33% voting power), losing one may halt consensus
- This tests the threshold behavior

**Pass Criteria**:
- If chain halts: Expected with 2-validator setup (need 67% for BFT)
- Document the behavior for production planning

---

## Test 6.4: Both Validators Simultaneous Restart

**Purpose**: Test network recovery after full restart

**Command (coordinate on both VPS)**:

**VPS1**:
```bash
pkill posd
echo "VPS1 stopped at $(date)"
```

**VPS2**:
```bash
pkill posd
echo "VPS2 stopped at $(date)"
```

**Wait 10 seconds, then restart both:**

**VPS1**:
```bash
posd start > ~/.pos/chain.log 2>&1 &
echo "VPS1 starting at $(date)"
```

**VPS2**:
```bash
posd start > ~/.pos/chain.log 2>&1 &
echo "VPS2 starting at $(date)"
```

**Check recovery (VPS1)**:
```bash
sleep 20
echo "Checking recovery..."
posd status 2>&1 | jq '{
  height: .sync_info.latest_block_height,
  catching_up: .sync_info.catching_up,
  latest_block_time: .sync_info.latest_block_time
}'

# Check if blocks are being produced
FIRST=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
sleep 10
SECOND=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
if [ $SECOND -gt $FIRST ]; then
  echo "PASS: Chain recovered, blocks being produced"
else
  echo "FAIL: Chain not producing blocks"
fi
```

**Expected Result**: Chain resumes block production

**Pass Criteria**: Blocks produced within 30 seconds of restart

---

## Test 6.5: Network Latency Check

**Purpose**: Measure latency between validators

**Command (VPS1)**:
```bash
echo "Ping latency to VPS2:"
ping -c 10 148.230.125.9 | tail -1
```

**Command (VPS2)**:
```bash
echo "Ping latency to VPS1:"
ping -c 10 46.202.179.182 | tail -1
```

**Expected Result**: Latency < 200ms

**Pass Criteria**: Average latency < 200ms for stable consensus

---

## Test 6.6: RPC Endpoint Health

**Purpose**: Verify RPC endpoints respond correctly

**Command (from any machine)**:
```bash
echo "VPS1 RPC Health:"
curl -s -w "\nHTTP Status: %{http_code}\nTime: %{time_total}s\n" \
  http://46.202.179.182:26657/health

echo ""
echo "VPS2 RPC Health:"
curl -s -w "\nHTTP Status: %{http_code}\nTime: %{time_total}s\n" \
  http://148.230.125.9:26657/health
```

**Expected Result**: HTTP 200, response time < 1 second

**Pass Criteria**: Both endpoints healthy

---

## Test 6.7: Persistent Peer Configuration

**Purpose**: Verify persistent peers are configured

**Command (VPS1)**:
```bash
grep "persistent_peers" ~/.pos/config/config.toml
```

**Command (VPS2)**:
```bash
grep "persistent_peers" ~/.pos/config/config.toml
```

**Expected Result**: Each node has the other as persistent peer

**Pass Criteria**: Persistent peers configured correctly

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 6.1 | Peer Discovery | | |
| 6.2 | Restart & Sync | | |
| 6.3 | Single Validator Failure | | |
| 6.4 | Both Restart | | |
| 6.5 | Network Latency | | |
| 6.6 | RPC Health | | |
| 6.7 | Persistent Peers | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL

## Notes on Fault Tolerance

With a 2-validator setup:
- **VPS1**: 100M OMNI = ~67% voting power
- **VPS2**: 50M OMNI = ~33% voting power

CometBFT requires >2/3 (67%) voting power for consensus:
- If VPS1 goes down: Chain halts (only 33% available)
- If VPS2 goes down: Chain may continue (67% still online)

**Recommendation**: Add a 3rd validator for production to ensure fault tolerance.
