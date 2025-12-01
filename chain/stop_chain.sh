#!/bin/bash
# Stop the OmniPhi blockchain

cd ~/omniphi/pos

if [ -f posd.pid ]; then
    PID=$(cat posd.pid)
    echo "Stopping blockchain (PID: $PID)..."
    kill $PID 2>/dev/null || true
    sleep 2
    rm posd.pid
    echo "✓ Blockchain stopped"
else
    echo "No PID file found, trying to kill all posd processes..."
    pkill posd 2>/dev/null || true
    echo "✓ Done"
fi
