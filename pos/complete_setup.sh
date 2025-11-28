#!/bin/bash

# Complete Omniphi Setup Script
# Run this on Ubuntu/WSL

set -e

echo "=========================================="
echo "Omniphi Complete Setup Script"
echo "=========================================="
echo ""

cd ~/omniphi/pos

# Step 1: Build binary (skip if already built)
echo "[1/10] Building binary..."
if [ ! -f "build/posd" ]; then
    sed -i '/^toolchain/d' go.mod 2>/dev/null || true
    go build -tags 'ledger cgo' -o build/posd ./cmd/posd
    echo "✓ Binary built"
else
    echo "✓ Binary already exists, skipping build"
fi
echo ""

# Step 2: Set variables
echo "[2/10] Setting environment variables..."
export BINARY=./build/posd
export CHAIN_ID=omniphi-1
export HOME_DIR=~/.pos
export DENOM=omniphi
export GAS_PRICES="0.025$DENOM"
echo "✓ Variables set"
echo ""

# Step 3: Clean old data
echo "[3/10] Cleaning old data..."
rm -rf "$HOME_DIR"
echo "✓ Old data removed"
echo ""

# Step 4: Initialize chain
echo "[4/10] Initializing chain..."
$BINARY init validator-1 --chain-id $CHAIN_ID --home "$HOME_DIR"
echo "✓ Chain initialized"
echo ""

# Step 5: Create keys
echo "[5/10] Creating keys..."
$BINARY keys add alice --keyring-backend test --home "$HOME_DIR" --output json > /tmp/alice_key.json 2>&1
$BINARY keys add bob --keyring-backend test --home "$HOME_DIR" --output json > /tmp/bob_key.json 2>&1
echo "✓ Keys created"
echo ""

# Step 6: Get addresses
echo "[6/10] Getting addresses..."
ALICE=$($BINARY keys show -a alice --keyring-backend test --home "$HOME_DIR")
BOB=$($BINARY keys show -a bob --keyring-backend test --home "$HOME_DIR")
echo "  Alice: $ALICE"
echo "  Bob: $BOB"
echo ""

# Step 7: Add genesis accounts
echo "[7/10] Adding genesis accounts..."
$BINARY genesis add-genesis-account "$ALICE" 1000000000000000$DENOM --home "$HOME_DIR"
$BINARY genesis add-genesis-account "$BOB" 1000000000000$DENOM --home "$HOME_DIR"
echo "✓ Genesis accounts added"
echo ""

# Step 8: Create gentx
echo "[8/10] Creating gentx..."
mkdir -p "$HOME_DIR/config/gentx"
$BINARY genesis gentx alice 500000000000$DENOM \
  --chain-id $CHAIN_ID \
  --keyring-backend test \
  --home "$HOME_DIR"
echo "✓ Gentx created"
echo ""

# Step 9: Collect gentxs
echo "[9/10] Collecting gentxs..."
$BINARY genesis collect-gentxs --home "$HOME_DIR"
echo "✓ Gentxs collected"
echo ""

# Step 10: Validate genesis
echo "[10/10] Validating genesis..."
$BINARY genesis validate --home "$HOME_DIR"
echo "✓ Genesis validated"
echo ""

echo "=========================================="
echo "✓ SETUP COMPLETE!"
echo "=========================================="
echo ""
echo "Chain Information:"
echo "  Chain ID: $CHAIN_ID"
echo "  Home Dir: $HOME_DIR"
echo "  Binary: $BINARY"
echo ""
echo "Accounts:"
echo "  Alice: $ALICE"
echo "  Bob: $BOB"
echo ""
echo "To start the chain:"
echo "  $BINARY start --home \"$HOME_DIR\""
echo ""
echo "Key files saved to:"
echo "  /tmp/alice_key.json"
echo "  /tmp/bob_key.json"
echo ""
