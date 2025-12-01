#!/bin/bash
# Update Peer IP Addresses in Config Files

if [ $# -ne 2 ]; then
    echo "Usage: $0 <COMPUTER1_IP> <COMPUTER2_IP>"
    echo "Example: $0 192.168.1.100 192.168.1.101"
    exit 1
fi

COMPUTER1_IP=$1
COMPUTER2_IP=$2
OUTPUT_DIR="./testnet-2nodes"

if [ ! -d "$OUTPUT_DIR" ]; then
    echo "Error: Testnet directory not found: $OUTPUT_DIR"
    echo "Please run setup_2node_testnet.sh first!"
    exit 1
fi

# Check if posd binary exists
if ! command -v ./posd &> /dev/null && ! command -v posd &> /dev/null; then
    echo "Error: posd binary not found!"
    exit 1
fi

POSD_BIN="./posd"
if [ ! -f "./posd" ]; then
    POSD_BIN="posd"
fi

# Get node IDs
NODE1_ID=$($POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator1")
NODE2_ID=$($POSD_BIN comet show-node-id --home "$OUTPUT_DIR/validator2")

echo "======================================"
echo "  Updating Peer IP Addresses"
echo "======================================"
echo ""
echo "Computer 1 IP: $COMPUTER1_IP"
echo "Computer 2 IP: $COMPUTER2_IP"
echo ""
echo "Node 1 ID: $NODE1_ID"
echo "Node 2 ID: $NODE2_ID"
echo ""

# Update validator1 config (peer to validator2)
CONFIG1="$OUTPUT_DIR/validator1/config/config.toml"
sed -i.bak "s/VALIDATOR2_IP/$COMPUTER2_IP/g" "$CONFIG1"
sed -i.bak "s/persistent_peers = \"[^\"]*\"/persistent_peers = \"$NODE2_ID@$COMPUTER2_IP:26656\"/g" "$CONFIG1"

# Update validator2 config (peer to validator1)
CONFIG2="$OUTPUT_DIR/validator2/config/config.toml"
sed -i.bak "s/VALIDATOR1_IP/$COMPUTER1_IP/g" "$CONFIG2"
sed -i.bak "s/persistent_peers = \"[^\"]*\"/persistent_peers = \"$NODE1_ID@$COMPUTER1_IP:26656\"/g" "$CONFIG2"

echo "✓ Updated validator1 persistent peer: $NODE2_ID@$COMPUTER2_IP:26656"
echo "✓ Updated validator2 persistent peer: $NODE1_ID@$COMPUTER1_IP:26656"
echo ""
echo "Peer configuration updated successfully!"
echo ""
echo "Next steps:"
echo "1. Package validator2: ./package_validator2.sh"
echo "2. Transfer to Computer 2"
echo "3. Start both validators"
