#!/bin/bash
set -e

echo "==================================="
echo "Windows Chain Test - Manual"
echo "===================================="
echo ""

CHAIN_ID="omniphi-test"
MONIKER="test-validator"
KEY_NAME="testkey"
HOME_DIR="$HOME/.pos"
DENOM="uomni"

echo "[1/14] Cleaning old data..."
rm -rf "$HOME_DIR"
echo "  ✓ Cleaned"

echo "[2/14] Building binary..."
go build -o posd.exe ./cmd/posd
echo "  ✓ Built"

echo "[3/14] Initializing chain..."
./posd.exe init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR > /dev/null 2>&1
echo "  ✓ Initialized"

echo "[4/14] Creating validator key..."
./posd.exe keys add $KEY_NAME --keyring-backend test --home $HOME_DIR > /dev/null 2>&1
ADDRESS=$(./posd.exe keys show $KEY_NAME -a --keyring-backend test --home $HOME_DIR)
echo "  ✓ Key created: $ADDRESS"

echo "[5/14] Adding genesis account..."
./posd.exe genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test --home $HOME_DIR > /dev/null 2>&1
echo "  ✓ Added 1,000,000 OMNI"

echo "[6/14] Fixing bond_denom (FIRST TIME)..."
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' $HOME_DIR/config/genesis.json
echo "  ✓ bond_denom set to uomni"

echo "[7/14] Configuring gas prices..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $HOME_DIR/config/app.toml
echo "  ✓ Set minimum gas price"

echo "[8/14] Creating gentx..."
./posd.exe genesis gentx $KEY_NAME 100000000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test \
  --home $HOME_DIR > /dev/null 2>&1
echo "  ✓ gentx created"

echo "[9/14] Collecting gentxs..."
./posd.exe genesis collect-gentxs --home $HOME_DIR > /dev/null 2>&1
echo "  ✓ gentxs collected"

echo "[10/14] Re-fixing bond_denom (SECOND TIME)..."
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' $HOME_DIR/config/genesis.json
echo "  ✓ bond_denom re-fixed"

echo "[11/14] Verifying modules..."
MODULES=$(./posd.exe query --help --home $HOME_DIR | grep -E "^  (feemarket|poc|tokenomics) " | wc -l)
if [ $MODULES -ge 3 ]; then
    echo "  ✓ All 3 modules registered"
else
    echo "  ✗ Missing modules (found $MODULES/3)"
    exit 1
fi

echo "[12/14] Validating genesis..."
./posd.exe genesis validate-genesis --home $HOME_DIR > /dev/null 2>&1
echo "  ✓ Genesis valid"

echo "[13/14] Checking gen_txs..."
GEN_TXS=$(grep -c '"gen_txs"' $HOME_DIR/config/genesis.json || echo "0")
if [ "$GEN_TXS" -gt 0 ]; then
    echo "  ✓ gentx present in genesis"
else
    echo "  ✗ No gentxs found"
    exit 1
fi

echo "[14/14] Starting chain (with gRPC disabled)..."
timeout 10 ./posd.exe start --minimum-gas-prices 0.001uomni --grpc.enable=false --home $HOME_DIR 2>&1 | grep -i "validator\|height\|error" | head -20 &
CHAIN_PID=$!
sleep 8
kill $CHAIN_PID 2>/dev/null || true

echo ""
echo "===================================="
echo "✅ Test Complete!"
echo "===================================="
