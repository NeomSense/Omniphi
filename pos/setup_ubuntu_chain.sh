#!/bin/bash
set -e

echo "=== Setting up PoC blockchain on Ubuntu ==="

# 1. Clean everything
echo "1. Cleaning old data..."
killall posd 2>/dev/null || true
sleep 2
rm -rf ~/.pos
rm -f posd.log posd.pid

# 2. Build binary
echo "2. Building posd binary..."
go build -o posd ./cmd/posd

# 3. Initialize chain
echo "3. Initializing chain..."
./posd init my-node --chain-id omniphi-1 --default-denom omniphi --home ~/.pos

# 4. Set minimum gas price
echo "4. Configuring gas price..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' ~/.pos/config/app.toml

# 5. Create accounts
echo "5. Creating accounts..."
./posd keys add alice --keyring-backend test --home ~/.pos
./posd keys add val1 --keyring-backend test --home ~/.pos

# 6. Get addresses
ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos)

echo ""
echo "Created accounts:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"
echo ""

# 7. Add genesis accounts
echo "6. Adding genesis accounts..."
./posd genesis add-genesis-account $ALICE 10000000000omniphi --home ~/.pos
./posd genesis add-genesis-account $VAL1 20000000000omniphi --home ~/.pos

# 8. Create genesis validator
echo "7. Creating genesis validator..."
./posd genesis gentx val1 10000000000omniphi \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --home ~/.pos

# 9. Collect genesis transactions
echo "8. Collecting genesis transactions..."
./posd genesis collect-gentxs --home ~/.pos

# 10. Start chain
echo "9. Starting blockchain..."
nohup ./posd start --home ~/.pos > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid

echo ""
echo "✓ Chain started (PID: $POSD_PID)"
echo ""
echo "Waiting 20 seconds for chain to initialize..."
sleep 20

# 11. Verify chain is running
echo ""
echo "10. Verifying chain status..."
if ./posd status --home ~/.pos 2>&1 | grep -q "latest_block_height"; then
    echo "✓ Chain is running and producing blocks"
    echo ""
    echo "Account balances:"
    ./posd query bank balance $ALICE omniphi --home ~/.pos
    ./posd query bank balance $VAL1 omniphi --home ~/.pos
else
    echo "✗ Chain failed to start. Check posd.log"
    exit 1
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Your accounts:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"
echo ""
echo "To stop the chain:"
echo "  kill \$(cat posd.pid)"
echo ""
echo "To view logs:"
echo "  tail -f posd.log"
echo ""
