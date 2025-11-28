#!/bin/bash
# Continuous Blockchain Monitor - Ubuntu/Linux/Mac

# Determine binary name (support both .exe on Windows Git Bash and native binary)
if [ -f "./posd.exe" ]; then
    POSD_BIN="./posd.exe"
elif [ -f "./posd" ]; then
    POSD_BIN="./posd"
elif command -v posd &> /dev/null; then
    POSD_BIN="posd"
else
    echo "Error: posd binary not found!"
    exit 1
fi

echo "Starting continuous monitor..."
echo "Press Ctrl+C to stop"
echo ""
sleep 2

while true; do
    clear
    echo "==================================================="
    echo "       Omniphi Testnet - Live Monitor"
    echo "==================================================="
    echo ""

    # Get status
    STATUS=$($POSD_BIN status --home ./testnet-2nodes/validator1 2>&1)

    if echo "$STATUS" | grep -q "latest_block_height"; then
        # Extract values using standard grep (compatible with both GNU grep and Git Bash)
        HEIGHT=$(echo "$STATUS" | grep -o '"latest_block_height":"[^"]*"' | cut -d'"' -f4)
        TIME=$(echo "$STATUS" | grep -o '"latest_block_time":"[^"]*"' | cut -d'"' -f4)
        CATCHING=$(echo "$STATUS" | grep -o '"catching_up":[^,}]*' | cut -d':' -f2)
        HASH=$(echo "$STATUS" | grep -o '"latest_block_hash":"[^"]*"' | cut -d'"' -f4)

        echo "Block Height:    $HEIGHT"
        echo "Block Time:      $TIME"
        echo "Block Hash:      ${HASH:0:16}..."
        echo ""

        if [ "$CATCHING" = "false" ]; then
            echo "Sync Status:     ✓ SYNCED"
        else
            echo "Sync Status:     ⏳ SYNCING"
        fi

        # Try to get peer count
        PEERS=$(echo "$STATUS" | grep -o '"n_peers":"[^"]*"' | cut -d'"' -f4)
        if [ -z "$PEERS" ]; then
            PEERS="unknown"
        fi
        echo "Peers Connected: $PEERS"

        echo ""
        echo "---------------------------------------------------"
        echo "Updating every 3 seconds... (Ctrl+C to exit)"

    else
        echo "✗ Unable to connect to validator"
        echo ""
        echo "Make sure validator is running:"
        echo "  ./start_validator1.sh"
    fi

    sleep 3
done
