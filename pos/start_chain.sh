#!/bin/bash
# Start the OmniPhi blockchain in the background

cd ~/omniphi/pos

# Kill any existing posd processes
pkill -9 posd 2>/dev/null || true
sleep 2

# Ensure minimum gas price is set in app.toml
sed -i 's/^minimum-gas-prices = .*/minimum-gas-prices = "0.001omniphi"/' ~/.pos/config/app.toml

# Start the chain with explicit gas price flag
./posd start --home ~/.pos --minimum-gas-prices="0.001omniphi" > posd.log 2>&1 &
PID=$!
echo $PID > posd.pid

echo "✓ Blockchain started (PID: $PID)"
sleep 5

# Check if it's running
if ./posd status --home ~/.pos 2>&1 | grep -q "latest_block_height"; then
    BLOCK=$(./posd status --home ~/.pos 2>&1 | grep -o '"latest_block_height":"[0-9]*"')
    echo "✓ Chain is producing blocks: $BLOCK"
else
    echo "⚠ Chain may not be running properly. Check logs:"
    echo "  tail -50 posd.log"
fi

echo ""
echo "Commands:"
echo "  Status: ./posd status --home ~/.pos 2>&1 | grep -o '\"latest_block_height\":\"[0-9]*\"'"
echo "  Logs: tail -f posd.log"
echo "  Stop: ./stop_chain.sh"
