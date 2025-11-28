#!/bin/bash
# Complete setup script for Omniphi PoS with omniphi denomination
set -e

CHAIN_ID="omniphi-1"
DENOM="omniphi"
KEYRING="test"
HOME_DIR="$HOME/.pos"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() { echo -e "${YELLOW}➜${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; exit 1; }

echo "================================================"
echo "Omniphi PoS - Complete Setup Script"
echo "Chain ID: $CHAIN_ID"
echo "Denomination: $DENOM"
echo "================================================"
echo ""

# Step 1: Stop any running nodes
info "Step 1: Stopping any running nodes..."
killall posd 2>/dev/null || true
sleep 2
success "Done"

# Step 2: Clean old data
info "Step 2: Cleaning old blockchain data..."
rm -rf $HOME_DIR
success "Done"

# Step 3: Build
info "Step 3: Building posd binary..."
go build -o posd ./cmd/posd || error "Build failed"
success "Binary built successfully"

# Step 4: Initialize
info "Step 4: Initializing blockchain..."
./posd init my-node \
  --chain-id $CHAIN_ID \
  --default-denom $DENOM \
  --home $HOME_DIR > /dev/null 2>&1 || error "Init failed"
success "Chain initialized"

# Step 5: Create accounts
info "Step 5: Creating test accounts..."
./posd keys add alice --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1 || error "Failed to create alice"
./posd keys add bob --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1 || error "Failed to create bob"
./posd keys add val1 --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1 || error "Failed to create val1"

ALICE=$(./posd keys show alice -a --keyring-backend $KEYRING --home $HOME_DIR)
BOB=$(./posd keys show bob -a --keyring-backend $KEYRING --home $HOME_DIR)
VAL1=$(./posd keys show val1 -a --keyring-backend $KEYRING --home $HOME_DIR)

echo "  Alice: $ALICE"
echo "  Bob:   $BOB"
echo "  Val1:  $VAL1"
success "Accounts created"

# Step 6: Add genesis accounts
info "Step 6: Adding accounts to genesis with $DENOM tokens..."
./posd genesis add-genesis-account $ALICE 10000000000$DENOM --home $HOME_DIR || error "Failed to add alice"
./posd genesis add-genesis-account $BOB 10000000000$DENOM --home $HOME_DIR || error "Failed to add bob"
./posd genesis add-genesis-account $VAL1 20000000000$DENOM --home $HOME_DIR || error "Failed to add val1"
success "Genesis accounts added"

# Step 7: Create genesis validator
info "Step 7: Creating genesis validator transaction..."
./posd genesis gentx val1 10000000000$DENOM \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --home $HOME_DIR > /dev/null 2>&1 || error "Genesis validator creation failed"
success "Genesis validator created"

# Step 8: Collect genesis transactions
info "Step 8: Collecting genesis transactions..."
./posd genesis collect-gentxs --home $HOME_DIR > /dev/null 2>&1 || error "Failed to collect gentxs"
success "Genesis finalized"

# Step 9: Start chain
info "Step 9: Starting blockchain..."
./posd start --home $HOME_DIR > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid
success "Chain started (PID: $POSD_PID)"

# Step 10: Wait and verify
info "Step 10: Waiting for chain to produce blocks (15 seconds)..."
sleep 15

if ./posd status --home $HOME_DIR &>/dev/null; then
    BLOCK_HEIGHT=$(./posd status --home $HOME_DIR 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' || echo "0")
    success "Chain is running! Block height: $BLOCK_HEIGHT"

    echo ""
    echo "================================================"
    echo "  ✓ Setup Complete!"
    echo "================================================"
    echo ""
    echo "Account Addresses:"
    echo "  Alice: $ALICE"
    echo "  Bob:   $BOB"
    echo "  Val1:  $VAL1"
    echo ""
    echo "Chain Info:"
    echo "  Chain ID: $CHAIN_ID"
    echo "  Denomination: $DENOM"
    echo "  PID: $POSD_PID (saved to posd.pid)"
    echo "  Logs: posd.log"
    echo "  Home: $HOME_DIR"
    echo ""
    echo "================================================"
    echo "Quick Test Commands"
    echo "================================================"
    echo ""
    echo "# Send tokens:"
    echo "./posd tx bank send $ALICE $VAL1 100000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# Delegate to validator:"
    echo "VALOPER=\$(./posd keys show val1 --bech val -a --keyring-backend test --home $HOME_DIR)"
    echo "./posd tx staking delegate \$VALOPER 250000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# Query balance:"
    echo "./posd query bank balances $VAL1 --home $HOME_DIR"
    echo ""
    echo "# Query delegations:"
    echo "./posd query staking delegations $ALICE --home $HOME_DIR"
    echo ""
    echo "# Submit PoC contribution:"
    echo "./posd tx poc submit-contribution \"code\" \"ipfs://QmTest/code.tar.gz\" \"0xABC\" \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# Validator endorses contribution:"
    echo "./posd tx poc endorse 1 true \\"
    echo "  --from val1 --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# Query PoC credits:"
    echo "./posd query poc credits $ALICE --home $HOME_DIR"
    echo ""
    echo "# Withdraw PoC rewards:"
    echo "./posd tx poc withdraw-poc-rewards \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "================================================"
    echo "Management Commands"
    echo "================================================"
    echo ""
    echo "# View logs:"
    echo "tail -f posd.log"
    echo ""
    echo "# Stop chain:"
    echo "kill \$(cat posd.pid)"
    echo ""
    echo "# Clean up:"
    echo "killall posd; rm -rf $HOME_DIR posd.log posd.pid"
    echo ""
else
    error "Chain failed to start. Check posd.log for errors."
fi
