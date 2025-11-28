#!/bin/bash
# 2-Node Testnet Setup Script (Computer 1)
# This script generates configuration for 2 validators on different computers

set -e

echo "======================================"
echo "  2-Node Testnet Generator"
echo "  Omniphi Blockchain"
echo "======================================"
echo ""

# Configuration
CHAIN_ID="omniphi-testnet-1"
OUTPUT_DIR="./testnet-2nodes"
VALIDATOR_STAKE="100000000000omniphi"  # 100,000 OMNI per validator
KEYRING_BACKEND="test"

# Check if posd binary exists
if ! command -v ./posd &> /dev/null && ! command -v posd &> /dev/null; then
    echo "Error: posd binary not found!"
    echo "Please build the binary first: make install"
    exit 1
fi

# Use local binary if exists, otherwise use installed
POSD_BIN="./posd"
if [ ! -f "./posd" ]; then
    POSD_BIN="posd"
fi

echo "Using binary: $POSD_BIN"
echo ""

# Clean up old testnet directory if exists
if [ -d "$OUTPUT_DIR" ]; then
    echo "Removing old testnet directory..."
    rm -rf "$OUTPUT_DIR"
fi

echo "Generating 2-node testnet configuration..."
echo "Chain ID: $CHAIN_ID"
echo "Output Directory: $OUTPUT_DIR"
echo ""

# Generate testnet using built-in multi-node command
$POSD_BIN testnet init-files \
    --v 2 \
    --output-dir "$OUTPUT_DIR" \
    --chain-id "$CHAIN_ID" \
    --keyring-backend "$KEYRING_BACKEND" \
    --starting-ip-address "192.168.1.100" 2>/dev/null || {

    # Fallback: manual setup if testnet command doesn't exist
    echo "Built-in testnet command not available. Setting up manually..."

    mkdir -p "$OUTPUT_DIR"

    # Initialize validator 1
    echo "Initializing Validator 1..."
    $POSD_BIN init validator1 --chain-id "$CHAIN_ID" --home "$OUTPUT_DIR/validator1" --overwrite

    # Initialize validator 2
    echo "Initializing Validator 2..."
    $POSD_BIN init validator2 --chain-id "$CHAIN_ID" --home "$OUTPUT_DIR/validator2" --overwrite

    # Create keys for validator 1
    echo "Creating validator1 key..."
    $POSD_BIN keys add validator1 --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator1" 2>&1 | tee "$OUTPUT_DIR/validator1/key_info.txt"

    # Create keys for validator 2
    echo "Creating validator2 key..."
    $POSD_BIN keys add validator2 --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator2" 2>&1 | tee "$OUTPUT_DIR/validator2/key_info.txt"

    # Get validator addresses
    VAL1_ADDR=$($POSD_BIN keys show validator1 -a --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator1")
    VAL2_ADDR=$($POSD_BIN keys show validator2 -a --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator2")

    echo "Validator 1 address: $VAL1_ADDR"
    echo "Validator 2 address: $VAL2_ADDR"

    # Add genesis accounts to validator1's genesis
    $POSD_BIN genesis add-genesis-account $VAL1_ADDR 200000000000omniphi --home "$OUTPUT_DIR/validator1"
    $POSD_BIN genesis add-genesis-account $VAL2_ADDR 200000000000omniphi --home "$OUTPUT_DIR/validator1"

    # Add a test account with funds
    $POSD_BIN keys add testuser --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator1" 2>&1 | tee "$OUTPUT_DIR/testuser_key.txt"
    TEST_ADDR=$($POSD_BIN keys show testuser -a --keyring-backend "$KEYRING_BACKEND" --home "$OUTPUT_DIR/validator1")
    $POSD_BIN genesis add-genesis-account $TEST_ADDR 50000000000omniphi --home "$OUTPUT_DIR/validator1"

    # Create gentx for validator 1
    echo "Creating genesis transaction for validator1..."
    $POSD_BIN genesis gentx validator1 $VALIDATOR_STAKE \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING_BACKEND" \
        --home "$OUTPUT_DIR/validator1"

    # Copy validator1's gentx to validator2's gentx directory
    mkdir -p "$OUTPUT_DIR/validator2/config/gentx"
    cp "$OUTPUT_DIR/validator1/config/gentx"/*.json "$OUTPUT_DIR/validator2/config/gentx/"

    # Create gentx for validator 2
    echo "Creating genesis transaction for validator2..."
    # First, copy the genesis from validator1 to validator2
    cp "$OUTPUT_DIR/validator1/config/genesis.json" "$OUTPUT_DIR/validator2/config/genesis.json"

    $POSD_BIN genesis gentx validator2 $VALIDATOR_STAKE \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING_BACKEND" \
        --home "$OUTPUT_DIR/validator2"

    # Copy validator2's gentx to validator1
    cp "$OUTPUT_DIR/validator2/config/gentx"/*.json "$OUTPUT_DIR/validator1/config/gentx/"

    # Collect all gentxs on validator1
    echo "Collecting genesis transactions..."
    $POSD_BIN genesis collect-gentxs --home "$OUTPUT_DIR/validator1"

    # Copy final genesis to validator2
    cp "$OUTPUT_DIR/validator1/config/genesis.json" "$OUTPUT_DIR/validator2/config/genesis.json"

    # Get node IDs
    NODE1_ID=$($POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator1")
    NODE2_ID=$($POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator2")

    # Configure persistent peers
    # These IPs are placeholders - user will update them
    sed -i.bak "s/persistent_peers = \"\"/persistent_peers = \"$NODE2_ID@VALIDATOR2_IP:26656\"/g" "$OUTPUT_DIR/validator1/config/config.toml"
    sed -i.bak "s/persistent_peers = \"\"/persistent_peers = \"$NODE1_ID@VALIDATOR1_IP:26656\"/g" "$OUTPUT_DIR/validator2/config/config.toml"

    # Enable API and unsafe CORS for testing
    sed -i.bak 's/enable = false/enable = true/g' "$OUTPUT_DIR/validator1/config/app.toml"
    sed -i.bak 's/enable = false/enable = true/g' "$OUTPUT_DIR/validator2/config/app.toml"
    sed -i.bak 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g' "$OUTPUT_DIR/validator1/config/app.toml"
    sed -i.bak 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g' "$OUTPUT_DIR/validator2/config/app.toml"

    # Set minimum gas prices
    sed -i.bak 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/g' "$OUTPUT_DIR/validator1/config/app.toml"
    sed -i.bak 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/g' "$OUTPUT_DIR/validator2/config/app.toml"

    # Allow external RPC connections (bind to 0.0.0.0 instead of 127.0.0.1)
    sed -i.bak 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/g' "$OUTPUT_DIR/validator1/config/config.toml"
    sed -i.bak 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/g' "$OUTPUT_DIR/validator2/config/config.toml"

    # Save node IDs to files
    echo "$NODE1_ID" > "$OUTPUT_DIR/validator1/node_id.txt"
    echo "$NODE2_ID" > "$OUTPUT_DIR/validator2/node_id.txt"
}

echo ""
echo "======================================"
echo "  Testnet Setup Complete!"
echo "======================================"
echo ""

# Get node IDs
NODE1_ID=$(cat "$OUTPUT_DIR/validator1/node_id.txt" 2>/dev/null || $POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator1")
NODE2_ID=$(cat "$OUTPUT_DIR/validator2/node_id.txt" 2>/dev/null || $POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator2")

echo "Configuration created for 2 validators:"
echo ""
echo "Validator 1 (Computer 1 - THIS COMPUTER):"
echo "  Home Directory: $OUTPUT_DIR/validator1"
echo "  Node ID: $NODE1_ID"
echo "  Ports: P2P=26656, RPC=26657, gRPC=9090, API=1317"
echo ""
echo "Validator 2 (Computer 2 - REMOTE COMPUTER):"
echo "  Home Directory: $OUTPUT_DIR/validator2"
echo "  Node ID: $NODE2_ID"
echo "  Ports: P2P=26656, RPC=26657, gRPC=9090, API=1317"
echo ""
echo "======================================"
echo "  NEXT STEPS:"
echo "======================================"
echo ""
echo "1. Find your Computer 1's IP address:"
echo "   Linux/Mac: ifconfig | grep 'inet '"
echo "   Windows: ipconfig"
echo ""
echo "2. Update peer configurations:"
echo "   Run: ./update_peer_ip.sh <COMPUTER1_IP> <COMPUTER2_IP>"
echo "   Example: ./update_peer_ip.sh 192.168.1.100 192.168.1.101"
echo ""
echo "3. Package validator2 for Computer 2:"
echo "   Run: ./package_validator2.sh"
echo ""
echo "4. Transfer the package to Computer 2 and extract it"
echo ""
echo "5. Start validators (on both computers):"
echo "   Computer 1: ./start_validator1.sh"
echo "   Computer 2: ./start_validator2.sh"
echo ""
echo "6. Verify network is running:"
echo "   Run: ./verify_network.sh"
echo ""
echo "See MULTI_NODE_TESTNET_GUIDE.md for detailed instructions!"
echo ""
