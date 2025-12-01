#!/bin/bash
set -e  # Exit on any error

echo "================================"
echo "Complete Chain Reset & Start"
echo "================================"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# 1. Stop any running chain
echo ""
echo "Step 1: Stopping chain..."
pkill posd || true
sleep 2
print_status "Chain stopped"

# 2. Reset chain data
echo ""
echo "Step 2: Resetting chain data..."
./posd comet unsafe-reset-all
print_status "Chain data reset"

# 3. Rebuild binary
echo ""
echo "Step 3: Rebuilding binary..."
go build -o posd ./cmd/posd
if [ ! -f "./posd" ]; then
    print_error "Failed to build posd binary"
    exit 1
fi
print_status "Binary rebuilt"

# 4. Initialize genesis
echo ""
echo "Step 4: Initializing genesis..."
./posd init test-node --chain-id omniphi-1 --default-denom uomni --overwrite
print_status "Genesis initialized"

# 5. Clean old gentx files
echo ""
echo "Step 5: Cleaning old gentx files..."
rm -rf ~/.pos/config/gentx/*.json
print_status "Old gentx files removed"

# 6. Add validator account with balance
echo ""
echo "Step 6: Adding validator account to genesis..."
VALIDATOR_ADDR=$(./posd keys show validator -a --keyring-backend test)
if [ -z "$VALIDATOR_ADDR" ]; then
    print_error "Validator key not found in keyring"
    exit 1
fi
echo "Validator address: $VALIDATOR_ADDR"

# Give 11M OMNI (11000000000000 uomni)
# After staking 1M, validator will have 10M spendable
./posd genesis add-genesis-account validator 11000000000000uomni --keyring-backend test

# Verify it was added
if ! grep -q "$VALIDATOR_ADDR" ~/.pos/config/genesis.json; then
    print_error "Failed to add validator to genesis"
    exit 1
fi
print_status "Validator account added with 11M OMNI"

# 7. Create validator gentx
echo ""
echo "Step 7: Creating validator gentx (staking 1M OMNI)..."
./posd genesis gentx validator 1000000000000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --moniker "Validator"

if [ ! -f ~/.pos/config/gentx/*.json ]; then
    print_error "Failed to create gentx"
    exit 1
fi
print_status "Gentx created"

# 8. Collect gentx
echo ""
echo "Step 8: Collecting gentx..."
./posd genesis collect-gentxs
print_status "Gentx collected"

# 9. Set treasury address in genesis
echo ""
echo "Step 9: Setting treasury address..."
# Create treasury account if it doesn't exist, suppressing mnemonic output
if ./posd keys show treasury --keyring-backend test &>/dev/null; then
    TREASURY_ADDR=$(./posd keys show treasury -a --keyring-backend test)
else
    ./posd keys add treasury --keyring-backend test &>/dev/null
    TREASURY_ADDR=$(./posd keys show treasury -a --keyring-backend test)
fi

echo "Treasury address: $TREASURY_ADDR"

# Set treasury address in genesis using sed with proper escaping
# Use | as delimiter to avoid issues with / in the address
sed -i "s|\"treasury_address\": \"\"|\"treasury_address\": \"$TREASURY_ADDR\"|" ~/.pos/config/genesis.json

# Verify it was set
if grep -q "\"treasury_address\": \"$TREASURY_ADDR\"" ~/.pos/config/genesis.json; then
    print_status "Treasury address set in genesis"
else
    print_error "Failed to set treasury address in genesis"
    exit 1
fi

# 10. Validate genesis
echo ""
echo "Step 10: Validating genesis..."
./posd genesis validate
if [ $? -ne 0 ]; then
    print_error "Genesis validation failed"
    exit 1
fi
print_status "Genesis validated successfully"

# 11. Check genesis balances
echo ""
echo "Step 11: Verifying genesis balances..."
GENESIS_BALANCE=$(grep -A 5 "$VALIDATOR_ADDR" ~/.pos/config/genesis.json | grep "amount" | head -1 | grep -o '[0-9]*')
if [ "$GENESIS_BALANCE" != "11000000000000" ]; then
    print_warning "Genesis balance is $GENESIS_BALANCE instead of 11000000000000"
else
    print_status "Genesis balance correct: 11M OMNI"
fi

# 12. Start chain
echo ""
echo "Step 12: Starting chain..."
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false > chain.log 2>&1 &
CHAIN_PID=$!
print_status "Chain started (PID: $CHAIN_PID)"

# 13. Wait for chain to be ready
echo ""
echo "Step 13: Waiting for chain to be ready..."
for i in {1..10}; do
    sleep 1
    if pgrep -x "posd" > /dev/null; then
        print_status "Chain process running"
        break
    fi
    if [ $i -eq 10 ]; then
        print_error "Chain failed to start"
        tail -50 chain.log
        exit 1
    fi
done

# Wait for first block
echo "Waiting for first block..."
for i in {1..30}; do
    sleep 1
    if grep -q "finalized block" chain.log; then
        print_status "Chain producing blocks"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "Chain not producing blocks after 30 seconds"
        tail -50 chain.log
        exit 1
    fi
done

# 14. Verify chain status
echo ""
echo "Step 14: Verifying chain status..."
sleep 2

# Check if chain is responding
if ./posd status &>/dev/null; then
    print_status "Chain RPC responding"
else
    print_error "Chain RPC not responding"
    exit 1
fi

# 15. Check validator balance
echo ""
echo "Step 15: Checking validator balance..."
BALANCE=$(./posd query bank balances $VALIDATOR_ADDR -o json 2>/dev/null | grep -o '"amount":"[0-9]*"' | grep -o '[0-9]*' | head -1)
if [ -z "$BALANCE" ]; then
    print_warning "Could not query balance (chain may still be starting)"
else
    BALANCE_OMNI=$((BALANCE / 1000000))
    if [ "$BALANCE" = "10000000000000" ]; then
        print_status "Validator balance correct: 10M OMNI (10M spendable after 1M staked)"
    else
        print_warning "Validator balance: ${BALANCE_OMNI}M OMNI (expected 10M)"
    fi
fi

# 15. Summary
echo ""
echo "================================"
echo "Chain Setup Complete!"
echo "================================"
echo ""
echo "Chain Status:"
echo "  • Chain ID: omniphi-1"
echo "  • Validator: $VALIDATOR_ADDR"
echo "  • Genesis Balance: 11M OMNI"
echo "  • Staked: 1M OMNI"
echo "  • Spendable: 10M OMNI"
echo "  • PID: $CHAIN_PID"
echo ""
echo "Next Steps:"
echo "  1. Monitor blocks: tail -f chain.log | grep 'finalized block'"
echo "  2. Check balance: ./posd query bank balances $VALIDATOR_ADDR"
echo "  3. Test fee burning: ./test_fee_burn.sh"
echo ""
echo "To stop chain: pkill posd"
echo ""
