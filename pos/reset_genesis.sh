#!/bin/bash
set -e

echo "================================"
echo "Resetting Genesis with Fix"
echo "================================"

# Stop the chain
echo "Stopping chain..."
pkill posd || true
sleep 2

# Reset the chain data (keeps your keys)
echo "Resetting chain data..."
./posd comet unsafe-reset-all

# Rebuild the binary with the fix
echo "Rebuilding binary..."
go build -o posd ./cmd/posd

# Reinitialize genesis
echo "Initializing genesis..."
./posd init test-node --chain-id omniphi-1 --default-denom uomni

# Remove old gentx files
echo "Cleaning old gentx files..."
rm -rf ~/.pos/config/gentx/*.json

# Add validator account with initial balance BEFORE gentx
# Give 11M OMNI so after staking 1M, there's 10M spendable
echo "Adding validator account to genesis..."
VALIDATOR_ADDR=$(./posd keys show validator -a --keyring-backend test)
echo "Validator address: $VALIDATOR_ADDR"
./posd genesis add-genesis-account validator 11000000000000uomni --keyring-backend test

# Create validator genesis transaction (stake 1M OMNI = 1000000000000 uomni)
echo "Creating validator gentx..."
./posd genesis gentx validator 1000000000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --moniker "Validator"

# Collect gentx
echo "Collecting gentx..."
./posd genesis collect-gentxs

# Set treasury address in genesis (if exists)
echo "Setting treasury address..."
TREASURY=$(./posd keys show treasury -a --keyring-backend test 2>/dev/null || echo "")
if [ -n "$TREASURY" ]; then
  echo "Treasury address: $TREASURY"
  jq --arg addr "$TREASURY" '.app_state.feemarket.treasury_address = $addr' ~/.pos/config/genesis.json > ~/.pos/config/genesis_tmp.json && mv ~/.pos/config/genesis_tmp.json ~/.pos/config/genesis.json
else
  echo "Warning: Treasury address not found in keyring"
fi

# Start the chain
echo "Starting chain..."
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false > chain.log 2>&1 &

# Wait for chain to start
echo "Waiting for chain to start..."
sleep 5

# Check if chain is running
if pgrep -x "posd" > /dev/null; then
  echo "✓ Chain is running"
  echo ""
  echo "Monitoring block production (Ctrl+C to stop):"
  tail -f chain.log | grep "finalized block"
else
  echo "✗ Chain failed to start. Check chain.log for errors:"
  tail -50 chain.log
fi
