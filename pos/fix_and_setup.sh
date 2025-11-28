#!/bin/bash

# Omniphi Fix and Setup Script
# This fixes the sonic build issue and runs setup + tests

set -e

echo "=========================================="
echo "Omniphi Fix, Setup, and Test"
echo "=========================================="
echo ""

cd ~/omniphi/pos

# Step 1: Fix sonic dependency issue
echo "[1/7] Fixing sonic dependency..."
go mod edit -replace github.com/bytedance/sonic=github.com/bytedance/sonic@v1.11.0
go mod tidy > /dev/null 2>&1
echo "✓ Dependencies fixed"
echo ""

# Step 2: Build Linux binary
echo "[2/7] Building Linux binary..."
CGO_ENABLED=0 go build -o build/posd ./cmd/posd
echo "✓ Binary built"
echo ""

# Step 3: Setup chain
echo "[3/7] Setting up chain..."
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
$BINARY genesis add-genesis-account "$ALICE" 1000000000000000$DENOM --home "$HOME_DIR" > /dev/null 2>&1
$BINARY genesis add-genesis-account "$BOB" 1000000000000$DENOM --home "$HOME_DIR" > /dev/null 2>&1

# Create gentx
mkdir -p "$HOME_DIR/config/gentx"
$BINARY genesis gentx alice 500000000000$DENOM --chain-id $CHAIN_ID --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1

# Collect gentxs
$BINARY genesis collect-gentxs --home "$HOME_DIR" > /dev/null 2>&1

# Validate
$BINARY genesis validate --home "$HOME_DIR" > /dev/null 2>&1

echo "✓ Chain setup complete"
echo ""

# Step 4: Run comprehensive tests
echo "[4/7] Running comprehensive tests..."
echo "  This may take a few minutes..."
go test -v ./test/comprehensive/... -timeout 30m 2>&1 | tee test_results.log
echo ""

# Step 5: Check comprehensive test results
echo "[5/7] Analyzing comprehensive test results..."
COMP_PASSED=$(grep -c "^=== RUN" test_results.log 2>/dev/null || echo "0")
COMP_FAILED=$(grep -c "^--- FAIL:" test_results.log 2>/dev/null || echo "0")

echo "  Tests run: $COMP_PASSED"
echo "  Tests failed: $COMP_FAILED"
echo ""

# Step 6: Run unit tests
echo "[6/7] Running unit tests..."
go test ./x/poc/... ./x/tokenomics/... -v -timeout 10m 2>&1 | tee unit_test_results.log
echo ""

# Step 7: Summary
echo "[7/7] Test Summary"
echo "=========================================="

UNIT_PASSED=$(grep -c "^PASS" unit_test_results.log 2>/dev/null || echo "0")
UNIT_FAILED=$(grep -c "^FAIL" unit_test_results.log 2>/dev/null || echo "0")

echo ""
echo "Comprehensive Tests:"
echo "  Total run: $COMP_PASSED"
echo "  Failed: $COMP_FAILED"
echo ""
echo "Unit Tests:"
echo "  Packages passed: $UNIT_PASSED"
echo "  Packages failed: $UNIT_FAILED"
echo ""

if [ "$COMP_FAILED" -eq "0" ] && [ "$UNIT_FAILED" -eq "0" ]; then
    echo "✅ ALL TESTS PASSED!"
else
    echo "⚠️  Some tests failed. Check logs for details."
fi

echo ""
echo "Logs saved:"
echo "  - test_results.log (comprehensive tests)"
echo "  - unit_test_results.log (unit tests)"
echo ""
echo "Chain is ready! To start:"
echo "  $BINARY start --home \"$HOME_DIR\""
echo ""
