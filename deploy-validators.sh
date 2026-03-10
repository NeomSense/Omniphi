#!/bin/bash
# Deployment script for OmniPhi validator updates (consensus-breaking)
# Usage: Run this on BOTH validators simultaneously
set -e

echo "=== OmniPhi Validator Update Script ==="
echo "This will update to the new 'omniphi' denom"
echo ""

# Configuration
CHAIN_ID="omniphi-mainnet-1"
MONIKER="${MONIKER:-validator1}"  # Override with: MONIKER=validator2 ./deploy-validators.sh
HOME_DIR="$HOME/.posd"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Step 1: Stopping validator...${NC}"
sudo systemctl stop posd 2>/dev/null || pkill posd || true
sleep 2

echo -e "${YELLOW}Step 2: Updating code...${NC}"
cd ~/omniphi
git fetch origin
git checkout main
git pull origin main

echo -e "${YELLOW}Step 3: Building new binary...${NC}"
cd chain
go build -o ~/go/bin/posd ./cmd/posd
posd version

echo -e "${YELLOW}Step 4: Backing up old data...${NC}"
if [ -d "$HOME_DIR" ]; then
    cp -r "$HOME_DIR" "$HOME_DIR.backup.$(date +%s)"
fi

echo -e "${YELLOW}Step 5: Resetting chain data...${NC}"
posd tendermint unsafe-reset-all --home "$HOME_DIR"

echo -e "${YELLOW}Step 6: Reinitializing chain...${NC}"
rm -rf "$HOME_DIR"
posd init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"

echo -e "${YELLOW}Step 7: Generating fresh genesis...${NC}"
cd ~/omniphi/chain
ignite chain build --skip-proto
ignite chain init --skip-proto

# Copy the fresh genesis
cp ~/.posd/config/genesis.json "$HOME_DIR/config/genesis.json"

echo -e "${GREEN}Genesis generated with omniphi denom${NC}"
echo ""
echo "=== NEXT STEPS ==="
echo "1. On validator1, get the node ID:"
echo "   posd tendermint show-node-id"
echo ""
echo "2. Update config.toml on BOTH validators with persistent_peers"
echo "   Example: <node1_id>@46.202.179.182:26656"
echo ""
echo "3. Start both validators at the SAME time:"
echo "   sudo systemctl start posd"
echo ""
echo "4. Check logs:"
echo "   sudo journalctl -u posd -f"
