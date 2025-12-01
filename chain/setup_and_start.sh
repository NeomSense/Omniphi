#!/bin/bash
# Complete setup script for Ubuntu - handles PoC genesis fix
set -e

CHAIN_ID="omniphi-1"
KEYRING="test"

echo "================================================"
echo "Omniphi PoS - Complete Setup Script"
echo "================================================"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${YELLOW}➜${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }

# 1. Stop any running nodes
info "Stopping any running nodes..."
killall posd 2>/dev/null || true
success "Done"

# 2. Clean old data
info "Cleaning old blockchain data..."
rm -rf ~/.pos
success "Done"

# 3. Build
info "Building posd binary for Linux..."
go build -o posd ./cmd/posd
success "Binary built"

# 4. Initialize
info "Initializing blockchain..."
./posd init my-node --chain-id $CHAIN_ID > /dev/null
success "Chain initialized"

# 5. Fix PoC module genesis (add default params)
info "Configuring PoC module genesis state..."
cat ~/.pos/config/genesis.json | \
  sed 's/"poc": {}/"poc": {"params":{"quorum_pct":"0.670000000000000000","base_reward_unit":"1000","inflation_share":"0.000000000000000000","max_per_block":10,"tiers":[{"name":"bronze","cutoff":"1000"},{"name":"silver","cutoff":"10000"},{"name":"gold","cutoff":"100000"}],"reward_denom":"stake"},"contributions":[],"credits":[],"next_contribution_id":"1"}/' \
  > ~/.pos/config/genesis_temp.json
mv ~/.pos/config/genesis_temp.json ~/.pos/config/genesis.json
success "PoC genesis configured"

# 6. Create accounts
info "Creating test accounts..."
./posd keys add alice --keyring-backend $KEYRING > /dev/null 2>&1
./posd keys add bob --keyring-backend $KEYRING > /dev/null 2>&1
./posd keys add val1 --keyring-backend $KEYRING > /dev/null 2>&1

ALICE=$(./posd keys show alice -a --keyring-backend $KEYRING)
BOB=$(./posd keys show bob -a --keyring-backend $KEYRING)
VAL1=$(./posd keys show val1 -a --keyring-backend $KEYRING)

echo "  Alice: $ALICE"
echo "  Bob:   $BOB"
echo "  Val1:  $VAL1"
success "Accounts created"

# 7. Add to genesis
info "Adding accounts to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000stake
./posd genesis add-genesis-account $BOB 10000000000stake
./posd genesis add-genesis-account $VAL1 20000000000stake
success "Genesis accounts added"

# 8. Create validator
info "Creating genesis validator..."
./posd genesis gentx val1 10000000000stake \
    --chain-id $CHAIN_ID \
    --keyring-backend $KEYRING \
    --commission-rate="0.10" \
    --commission-max-rate="0.20" \
    --commission-max-change-rate="0.01" > /dev/null 2>&1
success "Validator created"

# 9. Collect gentxs
info "Finalizing genesis..."
./posd genesis collect-gentxs > /dev/null 2>&1
success "Genesis finalized"

# 10. Start chain
info "Starting blockchain..."
./posd start > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid
success "Chain started (PID: $POSD_PID)"

# 11. Wait for chain to be ready
info "Waiting for chain to produce blocks (15 seconds)..."
sleep 15

# 12. Verify
if ./posd status &>/dev/null; then
    BLOCK_HEIGHT=$(./posd status 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' || echo "0")
    success "Chain is running! Block height: $BLOCK_HEIGHT"

    echo ""
    echo "================================================"
    echo "Setup Complete!"
    echo "================================================"
    echo ""
    echo "Account Addresses:"
    echo "  Alice: $ALICE"
    echo "  Bob:   $BOB"
    echo "  Val1:  $VAL1"
    echo ""
    echo "Chain Info:"
    echo "  Chain ID: $CHAIN_ID"
    echo "  PID: $POSD_PID (saved to posd.pid)"
    echo "  Logs: posd.log"
    echo ""
    echo "Quick Test Commands:"
    echo ""
    echo "# Send tokens:"
    echo "./posd tx bank send $ALICE $VAL1 100000stake \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --gas auto --fees 2000stake --yes"
    echo ""
    echo "# Delegate:"
    echo "VALOPER=\$(./posd keys show val1 --bech val -a --keyring-backend test)"
    echo "./posd tx staking delegate \$VALOPER 250000stake \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --gas auto --fees 2000stake --yes"
    echo ""
    echo "# Query balance:"
    echo "./posd query bank balances $ALICE"
    echo ""
    echo "# Submit PoC contribution:"
    echo "./posd tx poc submit-contribution \"code\" \"ipfs://QmTest/code.tar.gz\" \"0xABC\" \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --gas auto --fees 2000stake --yes"
    echo ""
    echo "To stop: kill \$(cat posd.pid)"
    echo "================================================"
else
    echo "❌ Chain failed to start. Check posd.log for errors."
    exit 1
fi
