#!/bin/bash
# Omniphi Blockchain Health Check - Ubuntu/Linux/Mac

echo "=== Omniphi Blockchain Health Check ==="
echo ""

# Determine binary name (support both .exe on Windows Git Bash and native binary)
if [ -f "./posd.exe" ]; then
    POSD_BIN="./posd.exe"
elif [ -f "./posd" ]; then
    POSD_BIN="./posd"
elif command -v posd &> /dev/null; then
    POSD_BIN="posd"
else
    echo "✗ posd binary not found!"
    exit 1
fi

# Get status
STATUS=$($POSD_BIN status --home ./testnet-2nodes/validator1 2>&1)

if echo "$STATUS" | grep -q "latest_block_height"; then
    # Use standard grep patterns (compatible with both GNU grep and Git Bash)
    HEIGHT=$(echo "$STATUS" | grep -o '"latest_block_height":"[^"]*"' | cut -d'"' -f4)
    CATCHING=$(echo "$STATUS" | grep -o '"catching_up":[^,}]*' | cut -d':' -f2)
    PEERS=$(echo "$STATUS" | grep -o '"n_peers":"[^"]*"' | cut -d'"' -f4)

    # Default to 0 if peers not found
    if [ -z "$PEERS" ]; then
        PEERS="0"
    fi

    echo "✓ Node is running"
    echo "  Block Height: $HEIGHT"
    echo "  Peers: $PEERS"

    if [ "$CATCHING" = "false" ]; then
        if [ "$HEIGHT" -gt "0" ] 2>/dev/null; then
            echo "  Status: ✓ SYNCED AND PRODUCING BLOCKS"
        else
            echo "  Status: ⏳ WAITING FOR PEER"
        fi
    else
        echo "  Status: ⏳ SYNCING"
    fi
else
    echo "✗ Node is not responding"
    echo "  Make sure the validator is running:"
    echo "  ./start_validator1.sh"
fi

echo ""
