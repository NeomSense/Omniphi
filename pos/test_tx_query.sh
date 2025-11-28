#!/bin/bash

cd ~/omniphi/pos

echo "=== Testing Transaction Query ==="
echo ""

# Get addresses
ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos-new)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos-new)

echo "Alice: $ALICE"
echo "Val1: $VAL1"
echo ""

# Send a transaction
echo "Sending 1000000 omniphi from Alice to Val1..."
TX_RESULT=$(./posd tx bank send $ALICE $VAL1 1000000omniphi \
  --from alice \
  --keyring-backend test \
  --chain-id omniphi-1 \
  --home ~/.pos-new \
  --gas 200000 \
  --fees 2000omniphi \
  --yes \
  --output json)

# Extract transaction hash
TX_HASH=$(echo $TX_RESULT | grep -o '"txhash":"[^"]*"' | cut -d'"' -f4)

echo ""
echo "Transaction sent!"
echo "TX Hash: $TX_HASH"
echo ""
echo "Waiting 6 seconds for block confirmation..."
sleep 6

# Query the transaction
echo ""
echo "=== Querying Transaction ==="
./posd query tx $TX_HASH --home ~/.pos-new

echo ""
echo "=== Query Complete ==="
