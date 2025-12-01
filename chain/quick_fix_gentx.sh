#!/bin/bash

# Quick Gentx Fix - Run this on your Ubuntu terminal

echo "Quick Gentx Fix for Omniphi"
echo "=============================="
echo ""

# Set variables
export BINARY=./build/posd
export CHAIN_ID=omniphi-1
export HOME_DIR=~/.pos
export DENOM=omniphi

# Get alice address
ALICE=$($BINARY keys show -a alice --keyring-backend test --home "$HOME_DIR" 2>/dev/null)

if [ -z "$ALICE" ]; then
    echo "ERROR: Alice key not found!"
    echo "Please create alice key first:"
    echo "  $BINARY keys add alice --keyring-backend test --home \"$HOME_DIR\""
    exit 1
fi

echo "Alice address: $ALICE"
echo ""

# Step 1: Create gentx directory
echo "[1/4] Creating gentx directory..."
mkdir -p "$HOME_DIR/config/gentx"
echo "✓ Done"
echo ""

# Step 2: Remove old gentx files
echo "[2/4] Cleaning old gentx files..."
rm -f "$HOME_DIR/config/gentx/"*.json 2>/dev/null || true
echo "✓ Done"
echo ""

# Step 3: Create gentx
echo "[3/4] Creating gentx..."
echo "Running: genesis gentx alice 500000000000$DENOM"
echo ""

$BINARY genesis gentx alice 500000000000${DENOM} \
  --chain-id $CHAIN_ID \
  --keyring-backend test \
  --home "$HOME_DIR"

if [ $? -ne 0 ]; then
    echo ""
    echo "ERROR: Gentx command failed!"
    echo ""
    echo "Common issues:"
    echo "1. Alice not in genesis accounts - run:"
    echo "   $BINARY genesis add-genesis-account \"$ALICE\" 1000000000000000$DENOM --home \"$HOME_DIR\""
    echo ""
    echo "2. Amount too high - try a smaller amount:"
    echo "   $BINARY genesis gentx alice 100000000000$DENOM --chain-id $CHAIN_ID --keyring-backend test --home \"$HOME_DIR\""
    echo ""
    echo "3. Chain ID mismatch - check genesis.json:"
    echo "   cat \"$HOME_DIR/config/genesis.json\" | jq -r '.chain_id'"
    exit 1
fi

echo ""
echo "✓ Gentx created!"
echo ""

# Step 4: Collect gentxs
echo "[4/4] Collecting gentxs..."
$BINARY genesis collect-gentxs --home "$HOME_DIR"

if [ $? -ne 0 ]; then
    echo ""
    echo "ERROR: Collect-gentxs failed!"
    exit 1
fi

echo "✓ Done"
echo ""

echo "=============================="
echo "SUCCESS! Genesis is ready."
echo "=============================="
echo ""
echo "You can now start the chain:"
echo "  $BINARY start --home \"$HOME_DIR\""
echo ""
