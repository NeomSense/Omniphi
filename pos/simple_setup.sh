#!/bin/bash
set -e

echo "=== Simple PoC Blockchain Setup ==="
cd ~/omniphi/pos

# 1. Build
echo "1. Building binary..."
go build -o posd ./cmd/posd

# 2. Initialize
echo "2. Initializing chain..."
./posd init my-node --chain-id omniphi-1 --default-denom omniphi --home ~/.pos

# 3. Fix gas price
echo "3. Setting gas price..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' ~/.pos/config/app.toml

# 4. Create accounts
echo "4. Creating accounts..."
./posd keys add alice --keyring-backend test --home ~/.pos 2>/dev/null || echo "Alice already exists"
./posd keys add val1 --keyring-backend test --home ~/.pos 2>/dev/null || echo "Val1 already exists"

ALICE=$(./posd keys show alice -a --keyring-backend test --home ~/.pos)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home ~/.pos)

echo ""
echo "Accounts created:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"

# 5. Add to genesis
echo ""
echo "5. Adding to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000omniphi --home ~/.pos
./posd genesis add-genesis-account $VAL1 20000000000omniphi --home ~/.pos

# 6. Create validator
echo "6. Creating validator..."
./posd genesis gentx val1 10000000000omniphi \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --home ~/.pos

# 7. Collect gentx
echo "7. Collecting genesis txs..."
./posd genesis collect-gentxs --home ~/.pos

echo ""
echo "=== Setup Complete ==="
echo ""
echo "To start the chain, run:"
echo "  ./start_chain.sh"
echo ""
