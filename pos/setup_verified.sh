#!/bin/bash
# Verified working setup script - tested step by step
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
error() { echo -e "${RED}✗${NC} $1"; }

echo "========================================"
echo "Omniphi PoS - Verified Setup Script"
echo "========================================"
echo ""

# 1. Cleanup
info "1. Cleaning up..."
killall posd 2>/dev/null || true
sleep 1
rm -rf $HOME_DIR posd.log posd.pid
success "Clean"

# 2. Build
info "2. Building binary..."
go build -o posd ./cmd/posd
if [ ! -f "./posd" ]; then
    error "Build failed - posd not created"
    exit 1
fi
success "Built"

# 3. Initialize
info "3. Initializing chain..."
./posd init my-node --chain-id $CHAIN_ID --default-denom $DENOM --home $HOME_DIR
if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    error "Genesis not created"
    exit 1
fi
success "Initialized"

# 4. Update PoC genesis - critical step
info "4. Updating PoC genesis state..."

# Check if node exists
if command -v node &> /dev/null; then
    info "   Using Node.js to update genesis..."
    node << 'EOFJS'
const fs = require('fs');
const path = require('path');
const os = require('os');

try {
    const genesisPath = path.join(os.homedir(), '.pos', 'config', 'genesis.json');
    const genesis = JSON.parse(fs.readFileSync(genesisPath, 'utf8'));

    // Update PoC with proper structure
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
    console.log("Genesis updated");
    process.exit(0);
} catch (err) {
    console.error("Error:", err.message);
    process.exit(1);
}
EOFJS
    if [ $? -ne 0 ]; then
        error "Failed to update genesis with node"
        exit 1
    fi
elif command -v python3 &> /dev/null; then
    info "   Using Python to update genesis..."
    python3 << 'EOFPY'
import json
import os
try:
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
    print("Genesis updated")
except Exception as e:
    print(f"Error: {e}")
    exit(1)
EOFPY
    if [ $? -ne 0 ]; then
        error "Failed to update genesis with python"
        exit 1
    fi
else
    error "Neither node nor python3 found - cannot update genesis"
    exit 1
fi

# Verify the update worked
if grep -q '"quorum_pct"' $HOME_DIR/config/genesis.json; then
    success "PoC genesis updated"
else
    error "PoC genesis update failed"
    exit 1
fi

# 5. Create accounts
info "5. Creating accounts..."
./posd keys add alice --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1
./posd keys add bob --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1
./posd keys add val1 --keyring-backend $KEYRING --home $HOME_DIR > /dev/null 2>&1

ALICE=$(./posd keys show alice -a --keyring-backend $KEYRING --home $HOME_DIR)
BOB=$(./posd keys show bob -a --keyring-backend $KEYRING --home $HOME_DIR)
VAL1=$(./posd keys show val1 -a --keyring-backend $KEYRING --home $HOME_DIR)

if [ -z "$ALICE" ] || [ -z "$BOB" ] || [ -z "$VAL1" ]; then
    error "Failed to create accounts"
    exit 1
fi

echo "   Alice: $ALICE"
echo "   Bob:   $BOB"
echo "   Val1:  $VAL1"
success "Accounts created"

# 6. Add to genesis
info "6. Adding accounts to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $BOB 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $VAL1 20000000000$DENOM --home $HOME_DIR
success "Accounts added"

# 7. Create validator - THE CRITICAL STEP
info "7. Creating genesis validator..."
./posd genesis gentx val1 10000000000$DENOM \
    --chain-id $CHAIN_ID \
    --keyring-backend $KEYRING \
    --commission-rate="0.10" \
    --commission-max-rate="0.20" \
    --commission-max-change-rate="0.01" \
    --home $HOME_DIR

# Check if gentx was created
if [ ! -d "$HOME_DIR/config/gentx" ] || [ -z "$(ls -A $HOME_DIR/config/gentx)" ]; then
    error "Gentx directory is empty - validator creation failed"
    echo ""
    echo "Debug info:"
    ls -la $HOME_DIR/config/ 2>&1 || true
    exit 1
fi

GENTX_FILE=$(ls $HOME_DIR/config/gentx/*.json 2>/dev/null | head -1)
if [ -f "$GENTX_FILE" ]; then
    success "Validator created: $(basename $GENTX_FILE)"
else
    error "No gentx file found"
    exit 1
fi

# 8. Collect gentxs
info "8. Collecting gentxs..."
./posd genesis collect-gentxs --home $HOME_DIR > /dev/null 2>&1
if [ $? -ne 0 ]; then
    error "Failed to collect gentxs"
    exit 1
fi
success "Genesis finalized"

# 9. Start chain
info "9. Starting blockchain..."
./posd start --home $HOME_DIR > posd.log 2>&1 &
POSD_PID=$!
echo $POSD_PID > posd.pid
success "Started (PID: $POSD_PID)"

# 10. Wait and verify
info "10. Waiting for blocks (15s)..."
sleep 15

if ./posd status --home $HOME_DIR &>/dev/null; then
    BLOCK=$(./posd status --home $HOME_DIR 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' || echo "0")
    success "Chain running at block $BLOCK"

    echo ""
    echo "========================================"
    echo "✓ SETUP COMPLETE!"
    echo "========================================"
    echo ""
    echo "Accounts:"
    echo "  Alice: $ALICE"
    echo "  Bob:   $BOB"
    echo "  Val1:  $VAL1"
    echo ""
    echo "Test Commands:"
    echo ""
    echo "# 1. Send tokens"
    echo "./posd tx bank send $ALICE $VAL1 100000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# 2. Delegate"
    echo "VALOPER=\$(./posd keys show val1 --bech val -a --keyring-backend test --home $HOME_DIR)"
    echo "./posd tx staking delegate \$VALOPER 250000$DENOM \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "# 3. Check balance"
    echo "./posd query bank balances $VAL1 --home $HOME_DIR"
    echo ""
    echo "# 4. Submit PoC"
    echo "./posd tx poc submit-contribution \"code\" \"ipfs://Qm123\" \"0xABC\" \\"
    echo "  --from alice --keyring-backend test --chain-id $CHAIN_ID \\"
    echo "  --home $HOME_DIR --gas auto --fees 2000$DENOM --yes"
    echo ""
    echo "Stop: kill \$(cat posd.pid)"
    echo "Logs: tail -f posd.log"
    echo ""
else
    error "Chain failed to start - check posd.log"
    echo ""
    echo "Last 20 lines of log:"
    tail -20 posd.log
    exit 1
fi
