#!/bin/bash

# Fix Gentx Script for Omniphi Chain
# This script will diagnose and fix the gentx issue

set -e  # Exit on error

echo "=========================================="
echo "Omniphi Gentx Fix Script"
echo "=========================================="
echo ""

# Set variables
export BINARY=./build/posd
export CHAIN_ID=omniphi-1
export HOME_DIR=~/.pos
export DENOM=omniphi
export GAS_PRICES="0.025$DENOM"

echo "Configuration:"
echo "  BINARY: $BINARY"
echo "  CHAIN_ID: $CHAIN_ID"
echo "  HOME_DIR: $HOME_DIR"
echo "  DENOM: $DENOM"
echo ""

# Step 1: Check if binary exists
echo "[Step 1] Checking binary..."
if [ ! -f "$BINARY" ]; then
    echo "ERROR: Binary not found at $BINARY"
    echo "Please build it first: make build"
    exit 1
fi
echo "✓ Binary found"
echo ""

# Step 2: Check if home directory exists
echo "[Step 2] Checking home directory..."
if [ ! -d "$HOME_DIR" ]; then
    echo "ERROR: Home directory not found at $HOME_DIR"
    echo "Please run 'posd init' first"
    exit 1
fi
echo "✓ Home directory exists"
echo ""

# Step 3: Check if alice key exists
echo "[Step 3] Checking alice key..."
if ! $BINARY keys show alice --keyring-backend test --home "$HOME_DIR" &>/dev/null; then
    echo "ERROR: Alice key not found"
    echo "Creating alice key..."
    $BINARY keys add alice --keyring-backend test --home "$HOME_DIR"
fi

ALICE=$($BINARY keys show -a alice --keyring-backend test --home "$HOME_DIR")
echo "✓ Alice address: $ALICE"
echo ""

# Step 4: Check genesis.json exists
echo "[Step 4] Checking genesis.json..."
if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    echo "ERROR: genesis.json not found"
    echo "Please run 'posd init' first"
    exit 1
fi
echo "✓ genesis.json exists"
echo ""

# Step 5: Check chain-id matches
echo "[Step 5] Verifying chain-id..."
GENESIS_CHAIN_ID=$(cat "$HOME_DIR/config/genesis.json" | jq -r '.chain_id')
echo "  Genesis chain-id: $GENESIS_CHAIN_ID"
echo "  Expected chain-id: $CHAIN_ID"

if [ "$GENESIS_CHAIN_ID" != "$CHAIN_ID" ]; then
    echo "WARNING: Chain ID mismatch!"
    echo "Using genesis chain-id: $GENESIS_CHAIN_ID"
    CHAIN_ID=$GENESIS_CHAIN_ID
fi
echo ""

# Step 6: Check if alice is in genesis accounts
echo "[Step 6] Checking alice genesis account..."
ALICE_BALANCE=$(cat "$HOME_DIR/config/genesis.json" | jq -r '.app_state.bank.balances[] | select(.address == "'$ALICE'") | .coins[0].amount // "0"')

if [ "$ALICE_BALANCE" = "0" ] || [ -z "$ALICE_BALANCE" ]; then
    echo "WARNING: Alice not found in genesis or has 0 balance"
    echo "Adding alice to genesis with 1B OMNI (1,000,000,000,000,000 omniphi)..."
    $BINARY genesis add-genesis-account "$ALICE" 1000000000000000$DENOM --home "$HOME_DIR" --append
    ALICE_BALANCE="1000000000000000"
fi

echo "✓ Alice genesis balance: $ALICE_BALANCE $DENOM"
echo ""

# Step 7: Calculate safe gentx amount (50% of genesis balance)
echo "[Step 7] Calculating gentx amount..."
GENTX_AMOUNT=$((ALICE_BALANCE / 2))
echo "  Alice genesis balance: $ALICE_BALANCE"
echo "  Gentx amount (50%): $GENTX_AMOUNT"
echo ""

# Step 8: Create gentx directory if it doesn't exist
echo "[Step 8] Ensuring gentx directory exists..."
mkdir -p "$HOME_DIR/config/gentx"
echo "✓ Gentx directory ready"
echo ""

# Step 9: Remove old gentx files if they exist
echo "[Step 9] Cleaning old gentx files..."
rm -f "$HOME_DIR/config/gentx/"*.json 2>/dev/null || true
echo "✓ Old gentx files removed"
echo ""

# Step 10: Create gentx
echo "[Step 10] Creating gentx..."
echo "Running: $BINARY genesis gentx alice $GENTX_AMOUNT$DENOM --chain-id $CHAIN_ID --keyring-backend test --home \"$HOME_DIR\""
echo ""

$BINARY genesis gentx alice ${GENTX_AMOUNT}${DENOM} \
  --chain-id $CHAIN_ID \
  --keyring-backend test \
  --home "$HOME_DIR"

# Check if gentx was created
GENTX_COUNT=$(ls "$HOME_DIR/config/gentx/"*.json 2>/dev/null | wc -l)
if [ "$GENTX_COUNT" -eq 0 ]; then
    echo ""
    echo "ERROR: Gentx file was not created!"
    echo "The gentx command may have failed silently."
    echo ""
    echo "Please check:"
    echo "1. Alice has sufficient balance in genesis"
    echo "2. Chain ID matches genesis.json"
    echo "3. No errors in the output above"
    exit 1
fi

echo ""
echo "✓ Gentx created successfully!"
ls -lh "$HOME_DIR/config/gentx/"
echo ""

# Step 11: Collect gentxs
echo "[Step 11] Collecting gentxs..."
$BINARY genesis collect-gentxs --home "$HOME_DIR"
echo "✓ Gentxs collected"
echo ""

# Step 12: Validate genesis
echo "[Step 12] Validating genesis..."
$BINARY genesis validate --home "$HOME_DIR"
echo "✓ Genesis validated successfully"
echo ""

echo "=========================================="
echo "✓ GENTX FIX COMPLETE!"
echo "=========================================="
echo ""
echo "Your genesis is now ready. You can start the chain with:"
echo "  $BINARY start --home \"$HOME_DIR\""
echo ""
echo "Genesis summary:"
echo "  Chain ID: $CHAIN_ID"
echo "  Alice address: $ALICE"
echo "  Alice genesis balance: $ALICE_BALANCE $DENOM"
echo "  Alice self-delegation: $GENTX_AMOUNT $DENOM"
echo ""
