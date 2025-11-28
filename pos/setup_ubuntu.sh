#!/bin/bash
################################################################################
# Omniphi Chain Setup Script for Ubuntu
# Professional deployment script with EIP-1559 FeeMarket + POC + Tokenomics
################################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
CHAIN_ID="omniphi-1"
MONIKER="omniphi-validator"
KEY_NAME="validator"
DENOM="uomni"

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Omniphi Chain Setup - Ubuntu${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Step 1: Pull latest code
echo -e "${YELLOW}[1/11] Pulling latest code from GitHub...${NC}"
git pull origin main
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Code updated${NC}"
else
    echo -e "${RED}  ✗ Failed to pull latest code${NC}"
    exit 1
fi

# Step 2: Build binary
echo -e "${YELLOW}[2/11] Building posd binary...${NC}"
go build -o posd ./cmd/posd
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Build successful${NC}"
    ls -lh posd
else
    echo -e "${RED}  ✗ Build failed${NC}"
    exit 1
fi

# Step 3: Detect home directory (.pos or .posd)
echo -e "${YELLOW}[3/11] Detecting chain home directory...${NC}"
if [ -d ~/.pos ]; then
    POSD_HOME=~/.pos
    echo -e "${GREEN}  ✓ Using ~/.pos (WSL style)${NC}"
elif [ -d ~/.posd ]; then
    POSD_HOME=~/.posd
    echo -e "${GREEN}  ✓ Using ~/.posd (standard)${NC}"
else
    POSD_HOME=~/.pos
    echo -e "${GREEN}  ✓ Will create ~/.pos${NC}"
fi

# Step 4: Clean old data (ask first)
echo -e "${YELLOW}[4/11] Cleaning old chain data...${NC}"
if [ -d "$POSD_HOME" ]; then
    read -p "  Remove existing chain data at $POSD_HOME? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$POSD_HOME"
        echo -e "${GREEN}  ✓ Removed old data${NC}"
    else
        echo -e "${YELLOW}  ⚠ Keeping existing data - this may cause issues!${NC}"
    fi
else
    echo -e "${GREEN}  ✓ No existing data to clean${NC}"
fi

# Step 5: Initialize chain
echo -e "${YELLOW}[5/11] Initializing chain...${NC}"
./posd init $MONIKER --chain-id $CHAIN_ID --home $POSD_HOME
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Chain initialized${NC}"
else
    echo -e "${RED}  ✗ Initialization failed${NC}"
    exit 1
fi

# Step 6: Create validator key
echo -e "${YELLOW}[6/11] Creating validator key...${NC}"
if ./posd keys show $KEY_NAME --keyring-backend test --home $POSD_HOME >/dev/null 2>&1; then
    echo -e "${YELLOW}  ⚠ Key '$KEY_NAME' already exists${NC}"
    ADDRESS=$(./posd keys show $KEY_NAME -a --keyring-backend test --home $POSD_HOME)
else
    ./posd keys add $KEY_NAME --keyring-backend test --home $POSD_HOME
    ADDRESS=$(./posd keys show $KEY_NAME -a --keyring-backend test --home $POSD_HOME)
fi
echo -e "${GREEN}  ✓ Validator address: $ADDRESS${NC}"

# Step 7: Add genesis account
echo -e "${YELLOW}[7/11] Adding genesis account...${NC}"
./posd genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test --home $POSD_HOME
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Added 1,000,000 OMNI to genesis${NC}"
else
    echo -e "${RED}  ✗ Failed to add genesis account${NC}"
    exit 1
fi

# Step 8: Configure minimum gas prices
echo -e "${YELLOW}[8/11] Configuring gas prices...${NC}"
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $POSD_HOME/config/app.toml
echo -e "${GREEN}  ✓ Set minimum gas price to 0.001uomni${NC}"

# Step 9: Verify module registration
echo -e "${YELLOW}[9/11] Verifying module registration...${NC}"
MODULES=$(./posd query --help --home $POSD_HOME | grep -E "^  (feemarket|poc|tokenomics) " | wc -l)
if [ $MODULES -ge 3 ]; then
    echo -e "${GREEN}  ✓ All 3 modules registered:${NC}"
    ./posd query --help --home $POSD_HOME | grep -E "^  (feemarket|poc|tokenomics) " | sed 's/^/    /'
else
    echo -e "${RED}  ✗ Missing modules (found $MODULES/3)${NC}"
    exit 1
fi

# Step 10: Check genesis (FeeMarket should auto-populate on start)
echo -e "${YELLOW}[10/11] Checking genesis file...${NC}"
GENESIS_FILE=$POSD_HOME/config/genesis.json
if grep -q '"feemarket": {}' $GENESIS_FILE; then
    echo -e "${YELLOW}  ⚠ FeeMarket genesis empty (will auto-populate with defaults on startup)${NC}"
elif grep -q '"feemarket":' $GENESIS_FILE; then
    echo -e "${GREEN}  ✓ FeeMarket genesis present${NC}"
fi

# Step 11: Instructions
echo -e "${YELLOW}[11/11] Setup complete!${NC}"
echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${GREEN}✅ Chain Ready to Start${NC}"
echo -e "${CYAN}================================${NC}"
echo ""
echo -e "${CYAN}Next Steps:${NC}"
echo ""
echo -e "  ${GREEN}1. Start the chain:${NC}"
echo -e "     ./posd start --minimum-gas-prices 0.001uomni --home $POSD_HOME"
echo ""
echo -e "  ${GREEN}2. Or start in background:${NC}"
echo -e "     nohup ./posd start --minimum-gas-prices 0.001uomni --home $POSD_HOME > chain.log 2>&1 &"
echo ""
echo -e "  ${GREEN}3. Check status:${NC}"
echo -e "     ./posd status --home $POSD_HOME"
echo ""
echo -e "  ${GREEN}4. View logs (if background):${NC}"
echo -e "     tail -f chain.log"
echo ""
echo -e "  ${GREEN}5. Test modules:${NC}"
echo -e "     ./posd query feemarket params --home $POSD_HOME"
echo -e "     ./posd query tokenomics inflation --home $POSD_HOME"
echo -e "     ./posd query poc --help --home $POSD_HOME"
echo ""
echo -e "${CYAN}Key Information:${NC}"
echo -e "  Chain ID: ${GREEN}$CHAIN_ID${NC}"
echo -e "  Moniker:  ${GREEN}$MONIKER${NC}"
echo -e "  Home Dir: ${GREEN}$POSD_HOME${NC}"
echo -e "  Address:  ${GREEN}$ADDRESS${NC}"
echo ""
echo -e "${YELLOW}Note: FeeMarket uses EIP-1559 dynamic fees with automatic burning${NC}"
echo ""
