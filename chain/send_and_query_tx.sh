#!/bin/bash

cd ~/omniphi/pos

echo "=== Send Transaction and Query It ==="
echo ""

# Get addresses
ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos-new)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos-new)

echo "From (Alice): $ALICE"
echo "To (Val1): $VAL1"
echo ""

# Send transaction
echo "Sending 1,000,000 omniphi..."
echo ""

TX_OUTPUT=$(./posd tx bank send $ALICE $VAL1 1000000omniphi \
  --from alice \
  --keyring-backend test \
  --chain-id omniphi-1 \
  --home ~/.pos-new \
  --gas 200000 \
  --fees 2000omniphi \
  --yes 2>&1)

echo "$TX_OUTPUT"
echo ""

# Extract the transaction hash
TX_HASH=$(echo "$TX_OUTPUT" | grep -oP 'txhash:\s*\K[A-F0-9]{64}' || echo "$TX_OUTPUT" | grep -oP '"txhash":\s*"\K[^"]+')

if [ -z "$TX_HASH" ]; then
    echo "❌ Could not extract transaction hash from output"
    echo ""
    echo "Full output:"
    echo "$TX_OUTPUT"
    exit 1
fi

echo "✓ Transaction Hash: $TX_HASH"
echo ""

# Wait for confirmation
echo "Waiting 6 seconds for block confirmation..."
sleep 6

# Query the transaction
echo ""
echo "=== Querying Transaction ==="
echo ""

./posd query tx $TX_HASH --home ~/.pos-new

echo ""
echo "=== Done ==="
echo ""
echo "To query this transaction again, use:"
echo "./posd query tx $TX_HASH --home ~/.pos-new"
