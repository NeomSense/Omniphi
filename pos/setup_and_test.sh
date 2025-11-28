#!/bin/bash

# Omniphi Setup and Test Script
# Run this DIRECTLY on your Ubuntu terminal (not from Windows)

set -e

echo "=========================================="
echo "Omniphi Setup and Test"
echo "=========================================="
echo ""

cd ~/omniphi/pos

# Step 1: Build Linux binary
echo "[1/6] Building Linux binary..."
go build -o build/posd ./cmd/posd
echo "✓ Binary built"
echo ""

# Step 2: Setup chain
echo "[2/6] Setting up chain..."
rm -rf ~/.pos

export BINARY=./build/posd
export CHAIN_ID=omniphi-1
export HOME_DIR=~/.pos
export DENOM=omniphi

# Init
$BINARY init validator-1 --chain-id $CHAIN_ID --home "$HOME_DIR" > /dev/null 2>&1

# Create keys
$BINARY keys add alice --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1
$BINARY keys add bob --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1

# Get addresses
ALICE=$($BINARY keys show -a alice --keyring-backend test --home "$HOME_DIR")
BOB=$($BINARY keys show -a bob --keyring-backend test --home "$HOME_DIR")

echo "  Alice: $ALICE"
echo "  Bob: $BOB"

# Add genesis accounts
$BINARY genesis add-genesis-account "$ALICE" 1000000000000000$DENOM --home "$HOME_DIR"
$BINARY genesis add-genesis-account "$BOB" 1000000000000$DENOM --home "$HOME_DIR"

# Create gentx
mkdir -p "$HOME_DIR/config/gentx"
$BINARY genesis gentx alice 500000000000$DENOM --chain-id $CHAIN_ID --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1

# Collect gentxs
$BINARY genesis collect-gentxs --home "$HOME_DIR" > /dev/null 2>&1

# Validate
$BINARY genesis validate --home "$HOME_DIR"

echo "✓ Chain setup complete"
echo ""

# Step 3: Run comprehensive tests
echo "[3/6] Running comprehensive tests..."
go test -v ./test/comprehensive/... 2>&1 | tee test_results.log
echo ""

# Step 4: Check test results
echo "[4/6] Analyzing test results..."
PASSED=$(grep -c "PASS:" test_results.log || echo "0")
FAILED=$(grep -c "FAIL:" test_results.log || echo "0")

echo "  Tests passed: $PASSED"
echo "  Tests failed: $FAILED"
echo ""

# Step 5: Run unit tests
echo "[5/6] Running unit tests..."
go test ./x/poc/... ./x/tokenomics/... -v 2>&1 | tee unit_test_results.log
echo ""

# Step 6: Summary
echo "[6/6] Test Summary"
echo "=========================================="

if [ "$FAILED" -eq "0" ]; then
    echo "✓ ALL TESTS PASSED!"
else
    echo "✗ Some tests failed. Check test_results.log for details."
fi

echo ""
echo "Logs saved:"
echo "  - test_results.log"
echo "  - unit_test_results.log"
echo ""
echo "To start the chain:"
echo "  $BINARY start --home \"$HOME_DIR\""
echo ""
