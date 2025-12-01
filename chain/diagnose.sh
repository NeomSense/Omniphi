#!/bin/bash
# Diagnostic script to check chain status and logs

echo "========================================="
echo "Omniphi PoS Chain Diagnostics"
echo "========================================="
echo ""

# Check if binary exists
echo "1. Binary Check:"
if [ -f "./posd" ]; then
    echo "   ✓ posd binary exists"
    ls -lh posd
else
    echo "   ✗ posd binary not found"
fi
echo ""

# Check genesis
echo "2. Genesis Check:"
if [ -f "$HOME/.pos/config/genesis.json" ]; then
    echo "   ✓ Genesis exists"
    echo "   Size: $(du -h $HOME/.pos/config/genesis.json | cut -f1)"
    echo "   Chain ID: $(grep -o '"chain_id":"[^"]*"' $HOME/.pos/config/genesis.json | cut -d'"' -f4)"
    echo ""
    echo "   PoC module genesis:"
    grep -A 3 '"poc"' $HOME/.pos/config/genesis.json | head -5
else
    echo "   ✗ Genesis not found"
fi
echo ""

# Check if process is running
echo "3. Process Check:"
if pgrep -f "posd start" > /dev/null; then
    echo "   ✓ posd is running"
    echo "   PID: $(pgrep -f 'posd start')"
else
    echo "   ✗ posd is not running"
fi
echo ""

# Check ports
echo "4. Port Check:"
if command -v netstat &> /dev/null; then
    echo "   Checking if ports are in use:"
    netstat -tuln | grep -E "26656|26657|26658|9090|1317" || echo "   No blockchain ports in use"
elif command -v ss &> /dev/null; then
    echo "   Checking if ports are in use:"
    ss -tuln | grep -E "26656|26657|26658|9090|1317" || echo "   No blockchain ports in use"
else
    echo "   netstat/ss not available"
fi
echo ""

# Check logs
echo "5. Log Check:"
if [ -f "posd.log" ]; then
    echo "   ✓ Log file exists"
    echo "   Size: $(du -h posd.log | cut -f1)"
    echo "   Last modified: $(ls -lh posd.log | awk '{print $6, $7, $8}')"
    echo ""
    echo "   Last 20 lines:"
    echo "   ----------------------------------------"
    tail -20 posd.log
    echo "   ----------------------------------------"
    echo ""
    echo "   Checking for errors:"
    if grep -i "panic\|error\|fail" posd.log | tail -10 | grep -q .; then
        echo "   Found errors/panics:"
        grep -i "panic\|error\|fail" posd.log | tail -10
    else
        echo "   No obvious errors found"
    fi
else
    echo "   ✗ Log file not found"
fi
echo ""

# Check config
echo "6. Config Check:"
if [ -f "$HOME/.pos/config/config.toml" ]; then
    echo "   ✓ config.toml exists"
    echo "   RPC address: $(grep 'laddr = "tcp' $HOME/.pos/config/config.toml | head -1 | cut -d'"' -f2)"
else
    echo "   ✗ config.toml not found"
fi
echo ""

echo "========================================="
echo "Try running the chain manually:"
echo "./posd start --home ~/.pos"
echo "========================================="
