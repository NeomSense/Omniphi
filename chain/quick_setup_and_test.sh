#!/bin/bash

# Omniphi Quick Setup and Test (Windows/Git Bash)
set -e

cd /c/Users/herna/omniphi/pos

export BINARY=./build/posd.exe
export CHAIN_ID=omniphi-1
export HOME_DIR=/c/Users/herna/.pos
export DENOM=omniphi

echo "========================================"
echo "Omniphi Quick Setup"
echo "========================================"

# Step 1: Clean and init (already done, but including for completeness)
echo "[1/6] Cleaning and initializing..."
rm -rf "$HOME_DIR"
$BINARY init validator-1 --chain-id $CHAIN_ID --home "$HOME_DIR" > /dev/null

# Step 2: Create keys
echo "[2/6] Creating keys..."
printf "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n\n" | \
  $BINARY keys add alice --recover --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1

printf "ability abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n\n" | \
  $BINARY keys add bob --recover --keyring-backend test --home "$HOME_DIR" > /dev/null 2>&1

ALICE=$($BINARY keys show -a alice --keyring-backend test --home "$HOME_DIR")
BOB=$($BINARY keys show -a bob --keyring-backend test --home "$HOME_DIR")

echo "  Alice: $ALICE"
echo "  Bob:   $BOB"

# Step 3: Add genesis accounts
echo "[3/6] Adding genesis accounts..."
$BINARY genesis add-genesis-account "$ALICE" "1000000000000000$DENOM" --home "$HOME_DIR"
$BINARY genesis add-genesis-account "$BOB" "1000000000000$DENOM" --home "$HOME_DIR"

# Step 4: Create gentx
echo "[4/6] Creating gentx..."
mkdir -p "$HOME_DIR/config/gentx"
$BINARY genesis gentx alice "500000000000$DENOM" \
  --chain-id $CHAIN_ID \
  --keyring-backend test \
  --home "$HOME_DIR" > /dev/null

# Step 5: Collect and validate
echo "[5/6] Collecting gentxs and validating..."
$BINARY genesis collect-gentxs --home "$HOME_DIR" > /dev/null
$BINARY genesis validate --home "$HOME_DIR"

echo "Setup complete!"
echo ""

# Step 6: Run quick tests
echo "[6/6] Running quick comprehensive tests..."
echo ""
go test -v ./test/comprehensive/... -run "TestTC00[1-5]" -timeout 10m 2>&1 | tee quick_test_results.txt

# Count results
PASSED=$(grep -c "--- PASS:" quick_test_results.txt || true)
FAILED=$(grep -c "--- FAIL:" quick_test_results.txt || true)

echo ""
echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $PASSED"
echo "Failed: $FAILED"

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "ALL TESTS PASSED!"
  exit 0
else
  echo ""
  echo "SOME TESTS FAILED - Review quick_test_results.txt"
  exit 1
fi
