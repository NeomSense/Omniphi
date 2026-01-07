# Test 4: Fee Market & Burn Logic Tests

## Objective
Test EIP-1559 adaptive fee market, burn mechanics, and fee distribution.

---

## Test 4.1: Query Fee Market Parameters

**Purpose**: Verify fee market configuration

**Command (VPS1)**:
```bash
posd query feemarket params --output json | jq '.'

# Expected output:
# {
#   "min_gas_price": "0.025000000000000000",
#   "max_tx_gas": "2000000",
#   "target_block_utilization": "0.330000000000000000",
#   "burn_cool": "0.100000000000000000",
#   "burn_normal": "0.200000000000000000",
#   "burn_hot": "0.400000000000000000",
#   ...
# }
```

**Expected Result**:
- `min_gas_price`: 0.025
- `max_tx_gas`: 2000000
- `target_block_utilization`: 0.33
- Burn tiers: 10%, 20%, 40%

**Pass Criteria**: All parameters match expected values

---

## Test 4.2: Min Gas Price Enforcement

**Purpose**: Verify transactions below min gas price are rejected

**Command (VPS1)**:
```bash
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Try to send with 0 fee (should fail)
echo "Testing 0 fee transaction (should fail)..."
posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1omniphi \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 0omniphi \
  --gas 100000 \
  -y 2>&1

# Try with fee below minimum (0.025 * 100000 = 2500 minimum)
echo ""
echo "Testing below-minimum fee (should fail)..."
posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1omniphi \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --fees 1000omniphi \
  --gas 100000 \
  -y 2>&1
```

**Expected Result**: Both transactions rejected with insufficient fee error

**Pass Criteria**: Low-fee transactions are rejected

---

## Test 4.3: Base Fee Query

**Purpose**: Check current base fee

**Command (VPS1)**:
```bash
# Query current base fee from feemarket state
posd query feemarket state --output json 2>&1 | jq '.'

# If state query not available, estimate from recent blocks
echo "Checking recent transaction fees..."
HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
curl -s "http://localhost:26657/block_results?height=$HEIGHT" | jq '.result.txs_results[0].gas_used // "no txs"'
```

**Expected Result**: Base fee >= min_gas_price (0.025)

**Pass Criteria**: Base fee is queryable and >= 0.025

---

## Test 4.4: Burn Tier Verification

**Purpose**: Verify burn rates based on utilization

**Command (VPS1)**:
```bash
echo "Burn Tier Configuration:"
echo "========================"
echo "Cool (0-16% utilization):   10% burn"
echo "Normal (16-33% utilization): 20% burn"
echo "Hot (33%+ utilization):      40% burn"
echo ""

# Check current block utilization
posd query feemarket state --output json 2>&1 | jq '{
  current_utilization: .utilization,
  current_tier: (if .utilization < 0.16 then "Cool" elif .utilization < 0.33 then "Normal" else "Hot" end)
}' 2>/dev/null || echo "State query not available - check via block analysis"

# Alternative: analyze recent blocks
HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
echo ""
echo "Checking gas used in recent blocks..."
for i in 0 1 2; do
  H=$((HEIGHT - i))
  GAS=$(curl -s "http://localhost:26657/block_results?height=$H" | jq '[.result.txs_results[]?.gas_used // 0] | add // 0')
  UTIL=$(echo "scale=4; $GAS / 60000000 * 100" | bc 2>/dev/null || echo "N/A")
  echo "Block $H: Gas used = $GAS, Utilization = ${UTIL}%"
done
```

**Expected Result**: Burn tier matches current utilization level

**Pass Criteria**: Correct tier applied based on utilization

---

## Test 4.5: Fee Distribution (Burn + Validators)

**Purpose**: Verify fees are split correctly (burn vs rewards)

**Command (VPS1)**:
```bash
# Check tokenomics for fee burn stats
posd query tokenomics params --output json 2>&1 | jq '.'

# Check total burned
posd query tokenomics burned --output json 2>&1 | jq '.' || echo "Query burned tokens"

# Check validator rewards before and after tx
VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)
echo "Checking distribution rewards..."
posd query distribution rewards $VALIDATOR_ADDR --output json | jq '.'
```

**Expected Result**:
- Portion of fees burned (based on tier)
- Remaining fees distributed to validators

**Pass Criteria**: Fee split functioning correctly

---

## Test 4.6: Supply Reduction from Burns

**Purpose**: Verify burned tokens reduce total supply

**Command (VPS1)**:
```bash
# Get current supply
echo "Current total supply:"
posd query bank total --output json | jq '.supply[] | select(.denom=="omniphi")'

# Check burned amount
echo ""
echo "Total burned (from tokenomics):"
posd query tokenomics state --output json 2>&1 | jq '.total_burned // "query not available"'

# Alternative: check burn events in recent blocks
echo ""
echo "Recent burn events:"
HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
curl -s "http://localhost:26657/block_results?height=$HEIGHT" | jq '.result.events[] | select(.type=="burn_tokens")' 2>/dev/null || echo "No burn events in latest block"
```

**Expected Result**: Total supply decreasing as burns occur

**Pass Criteria**: Burns reduce supply correctly

---

## Test 4.7: EIP-1559 Adaptation Test

**Purpose**: Test base fee adjustment under load

**Command (VPS1)**:
```bash
echo "EIP-1559 Adaptation Test"
echo "========================"
echo "Submitting burst of transactions to trigger fee increase..."

VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test)

# Get base fee before
echo "Base fee before burst: (checking feemarket state)"
posd query feemarket state --output json 2>&1 | jq '.base_fee // "N/A"'

# Submit 20 transactions rapidly
for i in {1..20}; do
  posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1omniphi \
    --from validator \
    --chain-id omniphi-testnet-1 \
    --keyring-backend test \
    --fees 500000omniphi \
    --gas 100000 \
    -y --broadcast-mode async > /dev/null 2>&1 &
done
wait

echo "Waiting for transactions to be included..."
sleep 20

# Get base fee after
echo "Base fee after burst:"
posd query feemarket state --output json 2>&1 | jq '.base_fee // "N/A"'

echo ""
echo "If base fee increased, EIP-1559 adaptation is working"
```

**Expected Result**: Base fee increases during high utilization

**Pass Criteria**: Base fee responds to utilization changes

---

## Results Template

| Test | Description | Result | Notes |
|------|-------------|--------|-------|
| 4.1 | Fee Market Params | | |
| 4.2 | Min Gas Price | | |
| 4.3 | Base Fee Query | | |
| 4.4 | Burn Tiers | | |
| 4.5 | Fee Distribution | | |
| 4.6 | Supply Reduction | | |
| 4.7 | EIP-1559 Adaptation | | |

**Overall Status**: [ ] PASS  [ ] WARN  [ ] FAIL
