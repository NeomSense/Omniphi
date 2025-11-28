#!/bin/bash
set -e

cd ~/omniphi/pos

echo "=== OmniPhi Setup for Ubuntu ==="
echo ""

# Kill old processes
killall posd 2>/dev/null || true
sleep 2

# Clean
rm -rf ~/.pos-new
rm -f posd.log posd.pid

echo "1. Building binary..."
go build -o posd ./cmd/posd

echo "2. Initializing chain..."
./posd init my-node --chain-id omniphi-1 --default-denom omniphi --home ~/.pos-new

echo "3. Setting gas price..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' ~/.pos-new/config/app.toml

echo "4. Creating accounts..."
./posd keys add alice --keyring-backend test --home ~/.pos-new
./posd keys add val1 --keyring-backend test --home ~/.pos-new

ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos-new)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos-new)

echo ""
echo "✓ Accounts Created:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"
echo ""

echo "5. Adding to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000omniphi --home ~/.pos-new
./posd genesis add-genesis-account $VAL1 20000000000omniphi --home ~/.pos-new

echo "6. Creating validator..."
./posd genesis gentx val1 10000000000omniphi \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --home ~/.pos-new

echo "7. Collecting genesis txs..."
./posd genesis collect-gentxs --home ~/.pos-new

echo "8. Starting chain..."
./posd start --home ~/.pos-new --minimum-gas-prices="0.001omniphi" > posd.log 2>&1 &
echo $! > posd.pid

echo ""
echo "Waiting 15 seconds..."
sleep 15

echo ""
echo "9. Verifying..."
if ./posd status --home ~/.pos-new 2>&1 | grep -q "latest_block_height"; then
    echo "✓ Chain running!"
    ./posd status --home ~/.pos-new 2>&1 | grep -o '"latest_block_height":"[0-9]*"'
    echo ""
    ./posd query bank balance $ALICE omniphi --home ~/.pos-new
else
    echo "⚠ Check logs: tail -20 posd.log"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Your accounts:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"
echo ""
echo "Commands to use:"
echo "  ./posd keys list --keyring-backend test --home ~/.pos-new"
echo "  ./posd status --home ~/.pos-new 2>&1 | grep latest_block_height"
echo ""
