#!/bin/bash
# Start Validator 2 (Computer 2)

VALIDATOR_HOME="./validator2"

# Check if validator directory exists
if [ ! -d "$VALIDATOR_HOME" ]; then
    echo "Error: Validator directory not found: $VALIDATOR_HOME"
    echo "Please extract the validator2 package first!"
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

echo "======================================"
echo "  Starting Validator 2"
echo "  Omniphi Testnet"
echo "======================================"
echo ""
echo "Home Directory: $VALIDATOR_HOME"
echo "Binary: $POSD_BIN"
echo ""
echo "Ports:"
echo "  P2P: 26656"
echo "  RPC: 26657"
echo "  gRPC: 9090"
echo "  API: 1317"
echo ""
echo "Press Ctrl+C to stop the validator"
echo ""

# Start the validator
$POSD_BIN start \
    --home "$VALIDATOR_HOME" \
    --minimum-gas-prices "0.001uomni"
