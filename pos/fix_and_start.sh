#!/bin/bash
# Fix app.toml and start the chain

set -e

HOME_DIR="$HOME/.pos"
DENOM="omniphi"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${YELLOW}➜${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }

echo "========================================="
echo "Fixing app.toml and starting chain"
echo "========================================="
echo ""

# Fix app.toml - set minimum gas price
info "Setting minimum gas price in app.toml..."

# Update app.toml with minimum gas price
sed -i "s/minimum-gas-prices = \"\"/minimum-gas-prices = \"0.001$DENOM\"/" $HOME_DIR/config/app.toml

# Verify the change
MINGASPRICE=$(grep "^minimum-gas-prices" $HOME_DIR/config/app.toml | cut -d'"' -f2)
echo "   Minimum gas prices: $MINGASPRICE"
success "app.toml updated"

echo ""
info "Starting blockchain..."

# Kill any existing process
killall posd 2>/dev/null || true
sleep 1

# Start the chain
./posd start --home $HOME_DIR > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid

success "Chain started (PID: $POSD_PID)"

# Wait for chain to start
info "Waiting for chain to start (15 seconds)..."
sleep 15

# Check if it's running
if ./posd status --home $HOME_DIR &>/dev/null; then
    BLOCK=$(./posd status --home $HOME_DIR 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
    success "Chain is running at block $BLOCK"

    echo ""
    echo "========================================="
    echo "✓ CHAIN IS RUNNING!"
    echo "========================================="
    echo ""

    # Get account addresses
    ALICE=$(./posd keys show alice -a --keyring-backend test --home $HOME_DIR 2>/dev/null || echo "")
    BOB=$(./posd keys show bob -a --keyring-backend test --home $HOME_DIR 2>/dev/null || echo "")
    VAL1=$(./posd keys show val1 -a --keyring-backend test --home $HOME_DIR 2>/dev/null || echo "")

    if [ -n "$ALICE" ]; then
        echo "Accounts:"
        echo "  Alice: $ALICE"
        echo "  Bob:   $BOB"
        echo "  Val1:  $VAL1"
        echo ""
        echo "Test Commands:"
        echo ""
        echo "# 1. Send tokens:"
        echo "./posd tx bank send $ALICE $VAL1 100000$DENOM \\"
        echo "  --from alice --keyring-backend test --chain-id omniphi-1 \\"
        echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
        echo ""
        echo "# 2. Delegate:"
        echo "VALOPER=\$(./posd keys show val1 --bech val -a --keyring-backend test --home $HOME_DIR)"
        echo "./posd tx staking delegate \$VALOPER 250000$DENOM \\"
        echo "  --from alice --keyring-backend test --chain-id omniphi-1 \\"
        echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
        echo ""
        echo "# 3. Check balance:"
        echo "./posd query bank balances $VAL1 --home $HOME_DIR"
        echo ""
        echo "# 4. Check delegations:"
        echo "./posd query staking delegations $ALICE --home $HOME_DIR"
        echo ""
        echo "# 5. Submit PoC contribution:"
        echo "./posd tx poc submit-contribution \"code\" \"ipfs://Qm123/code.tar.gz\" \"0xABC\" \\"
        echo "  --from alice --keyring-backend test --chain-id omniphi-1 \\"
        echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
        echo ""
        echo "# 6. Endorse contribution:"
        echo "./posd tx poc endorse 1 true \\"
        echo "  --from val1 --keyring-backend test --chain-id omniphi-1 \\"
        echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
        echo ""
        echo "# 7. Check PoC credits:"
        echo "./posd query poc credits $ALICE --home $HOME_DIR"
        echo ""
        echo "# 8. Withdraw PoC rewards:"
        echo "./posd tx poc withdraw-poc-rewards \\"
        echo "  --from alice --keyring-backend test --chain-id omniphi-1 \\"
        echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
        echo ""
    fi

    echo "========================================="
    echo "Management:"
    echo "  View logs: tail -f posd.log"
    echo "  Stop:      kill \$(cat posd.pid)"
    echo "========================================="
    echo ""
else
    echo "✗ Chain failed to start"
    echo ""
    echo "Last 10 lines of log:"
    tail -10 posd.log
    exit 1
fi
