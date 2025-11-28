#!/bin/bash
# Complete all-in-one setup script for Ubuntu
# Fixes PoC genesis and sets up the entire chain

set -e

CHAIN_ID="omniphi-1"
DENOM="omniphi"
KEYRING="test"
HOME_DIR="$HOME/.pos"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${YELLOW}➜${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; exit 1; }
section() { echo -e "\n${BLUE}=========================================${NC}\n${BLUE}$1${NC}\n${BLUE}=========================================${NC}"; }

section "Omniphi PoS - Complete Setup for Ubuntu"
echo "Chain ID: $CHAIN_ID"
echo "Denomination: $DENOM"
echo ""

# Step 1: Clean up
info "Step 1: Cleaning up..."
killall posd 2>/dev/null || true
rm -rf $HOME_DIR
rm -f posd.log posd.pid
success "Cleanup complete"

# Step 2: Build
info "Step 2: Building posd for Linux..."
go build -o posd ./cmd/posd || error "Build failed"
success "Binary built"

# Step 3: Initialize
info "Step 3: Initializing blockchain..."
./posd init my-node --chain-id $CHAIN_ID --default-denom $DENOM --home $HOME_DIR > /dev/null 2>&1
success "Chain initialized"

# Step 4: Fix PoC genesis using Node.js or Python
info "Step 4: Fixing PoC module genesis..."

# Create Node.js script inline
cat > /tmp/fix_poc_genesis.js << 'EOFJS'
const fs = require('fs');
const path = require('path');
const os = require('os');

const genesisPath = path.join(os.homedir(), '.pos', 'config', 'genesis.json');
const genesis = JSON.parse(fs.readFileSync(genesisPath, 'utf8'));

genesis.app_state.poc = {
    params: {
        quorum_pct: "0.670000000000000000",
        base_reward_unit: "1000",
        inflation_share: "0.000000000000000000",
        max_per_block: 10,
        tiers: [
            { name: "bronze", cutoff: "1000" },
            { name: "silver", cutoff: "10000" },
            { name: "gold", cutoff: "100000" }
        ],
        reward_denom: "omniphi"
    },
    contributions: [],
    credits: [],
    next_contribution_id: "1"
};

fs.writeFileSync(genesisPath, JSON.stringify(genesis, null, 2));
console.log("PoC genesis updated");
EOFJS

# Try Node.js first
if command -v node &> /dev/null; then
    node /tmp/fix_poc_genesis.js || error "Failed to update genesis with node"
elif command -v python3 &> /dev/null; then
    # Fallback to Python
    python3 << 'EOFPY'
import json
import os

genesis_path = os.path.expanduser("~/.pos/config/genesis.json")
with open(genesis_path, 'r') as f:
    genesis = json.load(f)

genesis['app_state']['poc'] = {
    "params": {
        "quorum_pct": "0.670000000000000000",
        "base_reward_unit": "1000",
        "inflation_share": "0.000000000000000000",
        "max_per_block": 10,
        "tiers": [
            {"name": "bronze", "cutoff": "1000"},
            {"name": "silver", "cutoff": "10000"},
            {"name": "gold", "cutoff": "100000"}
        ],
        "reward_denom": "omniphi"
    },
    "contributions": [],
    "credits": [],
    "next_contribution_id": "1"
}

with open(genesis_path, 'w') as f:
    json.dump(genesis, f, indent=2)
print("PoC genesis updated")
EOFPY
else
    error "Neither node nor python3 found. Please install one of them."
fi

success "PoC genesis configured"

# Step 5: Create accounts
info "Step 5: Creating test accounts..."
./posd keys add alice --keyring-backend $KEYRING --home $HOME_DIR 2>&1 | grep -v "mnemonic"
./posd keys add bob --keyring-backend $KEYRING --home $HOME_DIR 2>&1 | grep -v "mnemonic"
./posd keys add val1 --keyring-backend $KEYRING --home $HOME_DIR 2>&1 | grep -v "mnemonic"

ALICE=$(./posd keys show alice -a --keyring-backend $KEYRING --home $HOME_DIR)
BOB=$(./posd keys show bob -a --keyring-backend $KEYRING --home $HOME_DIR)
VAL1=$(./posd keys show val1 -a --keyring-backend $KEYRING --home $HOME_DIR)

echo "  Alice: $ALICE"
echo "  Bob:   $BOB"
echo "  Val1:  $VAL1"
success "Accounts created"

# Step 6: Add genesis accounts
info "Step 6: Adding accounts to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $BOB 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $VAL1 20000000000$DENOM --home $HOME_DIR
success "Genesis accounts added"

# Step 7: Create genesis validator
info "Step 7: Creating genesis validator..."
./posd genesis gentx val1 10000000000$DENOM \
    --chain-id $CHAIN_ID \
    --keyring-backend $KEYRING \
    --commission-rate="0.10" \
    --commission-max-rate="0.20" \
    --commission-max-change-rate="0.01" \
    --home $HOME_DIR 2>&1 | head -5

if [ $? -eq 0 ]; then
    success "Genesis validator created"
else
    error "Genesis validator creation failed"
fi

# Step 8: Collect gentxs
info "Step 8: Collecting genesis transactions..."
./posd genesis collect-gentxs --home $HOME_DIR > /dev/null 2>&1
success "Genesis finalized"

# Step 9: Start chain
info "Step 9: Starting blockchain..."
./posd start --home $HOME_DIR > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid
success "Chain started (PID: $POSD_PID)"

# Step 10: Wait and verify
info "Step 10: Waiting for chain to start (15 seconds)..."
sleep 15

if ./posd status --home $HOME_DIR &>/dev/null; then
    BLOCK_HEIGHT=$(./posd status --home $HOME_DIR 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' || echo "0")
    success "Chain is running! Current block: $BLOCK_HEIGHT"

    section "Setup Complete!"
    echo ""
    echo "Account Information:"
    echo "  Alice: $ALICE"
    echo "  Bob:   $BOB"
    echo "  Val1:  $VAL1"
    echo ""
    echo "Chain Information:"
    echo "  Chain ID: $CHAIN_ID"
    echo "  Denomination: $DENOM"
    echo "  PID: $POSD_PID (saved to posd.pid)"
    echo "  Home: $HOME_DIR"
    echo "  Logs: posd.log"
    echo ""

    section "Test Commands Ready to Run"
    echo ""
    echo "1. Send tokens (Alice → Val1):"
    echo "./posd tx bank send $ALICE $VAL1 100000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""

    echo "2. Delegate tokens:"
    echo "VALOPER=\$(./posd keys show val1 --bech val -a --keyring-backend test --home $HOME_DIR)"
    echo "./posd tx staking delegate \$VALOPER 250000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""

    echo "3. Check balances:"
    echo "./posd query bank balances $VAL1 --home $HOME_DIR"
    echo "./posd query bank balances $ALICE --home $HOME_DIR"
    echo ""

    echo "4. Check delegations:"
    echo "./posd query staking delegations $ALICE --home $HOME_DIR"
    echo ""

    echo "5. Submit PoC contribution:"
    echo "./posd tx poc submit-contribution \"code\" \"ipfs://Qm123/code.tar.gz\" \"0xABC\" \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""

    echo "6. Endorse contribution (validator only):"
    echo "./posd tx poc endorse 1 true \\"
    echo "  --from val1 --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""

    echo "7. Query PoC credits:"
    echo "./posd query poc credits $ALICE --home $HOME_DIR"
    echo ""

    echo "8. Withdraw PoC rewards:"
    echo "./posd tx poc withdraw-poc-rewards \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""

    section "Management Commands"
    echo ""
    echo "View logs:  tail -f posd.log"
    echo "Stop chain: kill \$(cat posd.pid)"
    echo "Clean up:   killall posd; rm -rf $HOME_DIR posd.log posd.pid"
    echo ""

    success "All ready! Run the test commands above."
else
    error "Chain failed to start. Check posd.log for details."
fi
