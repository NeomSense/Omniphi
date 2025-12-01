#!/bin/bash

# Create alice account if it doesn't exist
./posd keys add alice --keyring-backend test 2>/dev/null || true

# Get addresses
VALIDATOR=$(./posd keys show validator -a --keyring-backend test)
ALICE=$(./posd keys show alice -a --keyring-backend test)

echo "Validator: $VALIDATOR"
echo "Alice: $ALICE"

# Send transaction with fees
echo ""
echo "Sending transaction with 5000uomni fees..."
./posd tx bank send $VALIDATOR $ALICE 10000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 5000uomni \
  --yes

# Wait for transaction to be included
echo ""
echo "Waiting for transaction to be included in a block..."
sleep 8

# Check logs for fee processing
echo ""
echo "=== Fee Processing Logs ==="
tail -100 chain.log | grep -E "processing block fees|burned fees|transferred to treasury|fees for validators|no fees collected"

# Query updated fee stats
echo ""
echo "=== Updated Fee Statistics ==="
./posd query feemarket fee-stats

# Query burn tier
echo ""
echo "=== Burn Tier ==="
./posd query feemarket burn-tier
