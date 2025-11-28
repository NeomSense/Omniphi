#!/bin/bash
# Verify Testnet Network Status

VALIDATOR_HOME="./testnet-2nodes/validator1"

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
echo "  Network Status Check"
echo "======================================"
echo ""

# Check node status
echo "1. Node Status:"
$POSD_BIN status --home "$VALIDATOR_HOME" 2>&1 | grep -E "latest_block_height|catching_up|voting_power" || echo "   Node not responding"

echo ""
echo "2. Peer Connections:"
$POSD_BIN status --home "$VALIDATOR_HOME" 2>&1 | grep -E "n_peers" || echo "   No status available"

echo ""
echo "3. Validators:"
$POSD_BIN query staking validators --home "$VALIDATOR_HOME" --output json 2>&1 | grep -E "moniker|status|tokens" | head -20 || echo "   Cannot query validators"

echo ""
echo "4. Latest Block:"
$POSD_BIN query block --home "$VALIDATOR_HOME" 2>&1 | grep -E "height|time|num_txs" | head -5 || echo "   Cannot query block"

echo ""
echo "======================================"
echo "  Quick Health Check"
echo "======================================"
echo ""

# Get sync info
SYNC_INFO=$($POSD_BIN status --home "$VALIDATOR_HOME" 2>&1)

if echo "$SYNC_INFO" | grep -q "latest_block_height"; then
    HEIGHT=$(echo "$SYNC_INFO" | grep "latest_block_height" | grep -o '[0-9]*' | head -1)
    CATCHING_UP=$(echo "$SYNC_INFO" | grep "catching_up" | grep -o "true\|false")

    echo "✓ Node is running"
    echo "  Block Height: $HEIGHT"
    echo "  Catching Up: $CATCHING_UP"

    if [ "$CATCHING_UP" = "false" ]; then
        echo "  Status: ✓ SYNCED"
    else
        echo "  Status: ⏳ SYNCING"
    fi
else
    echo "✗ Node is not responding"
    echo "  Make sure the validator is running: ./start_validator1.sh"
fi

echo ""
echo "For more detailed information, run:"
echo "  $POSD_BIN status --home $VALIDATOR_HOME"
echo ""
