#!/bin/bash
# Quick Start Script for Ubuntu/WSL
# Run this to initialize and start your Omniphi blockchain

set -e  # Exit on error

echo "=========================================="
echo "Omniphi Blockchain - Quick Start"
echo "=========================================="
echo ""

# Navigate to project directory
cd ~/omniphi/pos

# Step 1: Clean old data
echo "Step 1: Cleaning old chain data..."
rm -rf ~/.posd
echo "✓ Old data removed"
echo ""

# Step 2: Initialize chain
echo "Step 2: Initializing chain..."
./posd init my-validator --chain-id omniphi-1
echo "✓ Chain initialized"
echo ""

# Step 3: Create validator key
echo "Step 3: Creating validator key..."
./posd keys add validator --keyring-backend test
VALIDATOR_ADDRESS=$(./posd keys show validator -a --keyring-backend test)
echo "✓ Validator key created: $VALIDATOR_ADDRESS"
echo ""

# Step 4: Add genesis account
echo "Step 4: Adding genesis account..."
./posd genesis add-genesis-account validator 1000000000000000uomni --keyring-backend test
echo "✓ Genesis account added"
echo ""

# Step 5: Configure minimum gas price
echo "Step 5: Configuring gas prices..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' ~/.posd/config/app.toml
echo "✓ Gas prices configured"
echo ""

# Step 6: Create genesis transaction
echo "Step 6: Creating genesis transaction..."
./posd genesis gentx validator 100000000000uomni \
  --chain-id omniphi-1 \
  --moniker my-validator \
  --keyring-backend test \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1
echo "✓ Genesis transaction created"
echo ""

# Step 7: Collect genesis transactions
echo "Step 7: Collecting genesis transactions..."
./posd genesis collect-gentxs
echo "✓ Genesis transactions collected"
echo ""

# Step 8: Validate genesis
echo "Step 8: Validating genesis..."
./posd genesis validate-genesis
echo "✓ Genesis validated"
echo ""

echo "=========================================="
echo "✓ SETUP COMPLETE!"
echo "=========================================="
echo ""
echo "To start your node:"
echo "  ./posd start"
echo ""
echo "To check status (in another terminal):"
echo "  ./posd status"
echo ""
echo "To query modules:"
echo "  ./posd query poc --help"
echo "  ./posd query tokenomics --help"
echo "  ./posd query feemarket --help"
echo ""
echo "Your validator address: $VALIDATOR_ADDRESS"
echo ""
