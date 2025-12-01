#!/bin/bash
# Quick PoC Test
set -e

echo "Quick PoC Test Starting..."

# Setup
killall posd 2>/dev/null || true
rm -rf ~/.pos posd.log posd.pid

# Build
go build -o posd ./cmd/posd
echo "✓ Built"

# Init
./posd init test --chain-id omniphi-1 --default-denom omniphi --home ~/.pos > /dev/null
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' ~/.pos/config/app.toml
echo "✓ Initialized"

# Accounts
./posd keys add alice --keyring-backend test --home ~/.pos 2>/dev/null || true
./posd keys add val1 --keyring-backend test --home ~/.pos 2>/dev/null || true

ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos)
echo "Alice: $ALICE"
echo "Val1: $VAL1"

# Genesis
./posd genesis add-genesis-account $ALICE 10000000000omniphi --home ~/.pos
./posd genesis add-genesis-account $VAL1 20000000000omniphi --home ~/.pos

./posd genesis gentx val1 10000000000omniphi \
  --chain-id omniphi-1 --keyring-backend test \
  --commission-rate="0.10" --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" --home ~/.pos > /dev/null

./posd genesis collect-gentxs --home ~/.pos > /dev/null
echo "✓ Genesis ready"

# Start
./posd start --home ~/.pos > posd.log 2>&1 &
echo $! > posd.pid
echo "✓ Chain starting (PID: $(cat posd.pid))"
sleep 20

# Test
echo ""
echo "=== Testing PoC Submit ==="
./posd tx poc submit-contribution "code" "ipfs://Qm123" "0xABC" \
  --from alice --keyring-backend test --chain-id omniphi-1 \
  --home ~/.pos --gas auto --fees 2000omniphi --yes

echo ""
echo "Waiting 7 seconds..."
sleep 7

echo ""
echo "=== Querying Contribution ==="
./posd query poc contribution 1 --home ~/.pos

echo ""
echo "=== Testing Endorse ==="
./posd tx poc endorse 1 true \
  --from val1 --keyring-backend test --chain-id omniphi-1 \
  --home ~/.pos --gas auto --fees 2000omniphi --yes

sleep 7

echo ""
echo "=== Contribution After Endorse ==="
./posd query poc contribution 1 --home ~/.pos

echo ""
echo "=== Alice Credits ==="
./posd query poc credits $ALICE --home ~/.pos

echo ""
echo "✓ TEST COMPLETE!"
echo "Chain PID: $(cat posd.pid)"
