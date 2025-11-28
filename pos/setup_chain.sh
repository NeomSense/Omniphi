#!/bin/bash
# Simple chain setup for WSL/Ubuntu
# This handles the path issues correctly

echo "=== Omniphi Chain Setup ==="
echo ""

# Go to project directory
cd ~/omniphi/pos || cd /mnt/c/Users/herna/omniphi/pos

# Clear old data
echo "Clearing old chain data..."
rm -rf ~/.posd
rm -rf /c/Users/herna/.posd 2>/dev/null
rm -rf /mnt/c/Users/herna/.posd 2>/dev/null

# Build fresh binary
echo "Building posd..."
go build -o posd ./cmd/posd

# Initialize
echo "Initializing chain..."
./posd init my-validator --chain-id omniphi-1

# Find where config was created
if [ -d ~/.posd ]; then
    POSD_HOME=~/.posd
elif [ -d /c/Users/herna/.posd ]; then
    POSD_HOME=/c/Users/herna/.posd
elif [ -d /mnt/c/Users/herna/.posd ]; then
    POSD_HOME=/mnt/c/Users/herna/.posd
else
    echo "ERROR: Could not find .posd directory!"
    exit 1
fi

echo "Using home: $POSD_HOME"

# Create key
echo "Creating validator key..."
./posd keys add validator --keyring-backend test

# Get address
ADDR=$(./posd keys show validator -a --keyring-backend test)
echo "Validator address: $ADDR"

# Add genesis account
echo "Adding genesis account..."
./posd genesis add-genesis-account validator 1000000000000000uomni --keyring-backend test

# Configure gas price
echo "Configuring gas price..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $POSD_HOME/config/app.toml

# Create gentx
echo "Creating genesis transaction..."
./posd genesis gentx validator 100000000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --moniker my-validator

# Collect gentx
echo "Collecting genesis transactions..."
./posd genesis collect-gentxs

# Validate
echo "Validating genesis..."
./posd genesis validate-genesis

echo ""
echo "=== Setup Complete! ==="
echo ""
echo "To start the chain:"
echo "  ./posd start"
echo ""
echo "Your validator address: $ADDR"
echo ""
