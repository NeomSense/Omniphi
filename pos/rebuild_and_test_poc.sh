#!/bin/bash
set -e

cd ~/omniphi/pos

echo "=== Rebuilding Binary ==="
go build -o posd ./cmd/posd

echo ""
echo "=== Testing PoC Query Commands ==="
echo ""

echo "1. Testing params query..."
./posd query poc params --home ~/.pos-new

echo ""
echo "2. Testing contributions query..."
./posd query poc contributions --home ~/.pos-new

echo ""
echo "3. Testing credits query for Alice..."
ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos-new)
./posd query poc credits $ALICE --home ~/.pos-new

echo ""
echo "=== All Tests Complete ==="
