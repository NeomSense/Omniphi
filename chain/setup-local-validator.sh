#!/bin/bash
# Setup Local Validator Node
# This script properly initializes a single-node blockchain for testing

HOME_DIR="$HOME/.pos"
BINARY="./posd"
CHAIN_ID="omniphi-localnet-1"
MONIKER="local-validator"
KEYRING="test"

echo "Setting up local validator node..."

# 1. Clean existing data (optional - uncomment if you want fresh start)
# rm -rf $HOME_DIR

# 2. Initialize node
echo "Initializing node..."
$BINARY init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR

# 3. Create a key for the validator
echo "Creating validator key..."
$BINARY keys add validator --keyring-backend $KEYRING --home $HOME_DIR

# 4. Add genesis account with tokens
echo "Adding genesis account..."
VALIDATOR_ADDR=$($BINARY keys show validator -a --keyring-backend $KEYRING --home $HOME_DIR)
$BINARY genesis add-genesis-account $VALIDATOR_ADDR 1000000000000omniphi --home $HOME_DIR

# 5. Create genesis transaction (gentx)
echo "Creating genesis transaction..."
$BINARY genesis gentx validator 100000000000omniphi --chain-id $CHAIN_ID --keyring-backend $KEYRING --home $HOME_DIR

# 6. Collect genesis transactions (this populates the validators array!)
echo "Collecting genesis transactions..."
$BINARY genesis collect-gentxs --home $HOME_DIR

# 7. Validate genesis
echo "Validating genesis..."
$BINARY genesis validate --home $HOME_DIR

echo ""
echo "=================================================================="
echo "Local validator node setup complete!"
echo "=================================================================="
echo ""
echo "Your validator address: $VALIDATOR_ADDR"
echo ""
echo "To start the node, run:"
echo "  $BINARY start --home $HOME_DIR"
echo ""
