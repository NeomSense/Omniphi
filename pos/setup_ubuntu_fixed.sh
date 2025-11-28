#!/bin/bash
################################################################################
# Omniphi Chain Setup Script for Ubuntu - FIXED VERSION
# This script uses the proven gentx -> collect-gentxs approach
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

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
echo -e "${YELLOW}[1/13] Pulling latest code from GitHub...${NC}"
git pull origin main
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Code updated${NC}"
else
    echo -e "${RED}  ✗ Failed to pull latest code${NC}"
    exit 1
fi

# Step 2: Build binary
echo -e "${YELLOW}[2/13] Building posd binary...${NC}"
go build -o posd ./cmd/posd
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Build successful${NC}"
    ls -lh posd
else
    echo -e "${RED}  ✗ Build failed${NC}"
    exit 1
fi

# Step 3: Detect home directory
echo -e "${YELLOW}[3/13] Detecting chain home directory...${NC}"
if [ -d ~/.pos ]; then
    POSD_HOME=~/.pos
    echo -e "${GREEN}  ✓ Using ~/.pos${NC}"
elif [ -d ~/.posd ]; then
    POSD_HOME=~/.posd
    echo -e "${GREEN}  ✓ Using ~/.posd${NC}"
else
    POSD_HOME=~/.pos
    echo -e "${GREEN}  ✓ Will create ~/.pos${NC}"
fi

# Step 4: Clean old data
echo -e "${YELLOW}[4/13] Cleaning old chain data...${NC}"
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
echo -e "${YELLOW}[5/13] Initializing chain...${NC}"
./posd init $MONIKER --chain-id $CHAIN_ID --home $POSD_HOME 2>&1 > /dev/null
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Chain initialized${NC}"
else
    echo -e "${RED}  ✗ Initialization failed${NC}"
    exit 1
fi

# Step 6: Create validator key
echo -e "${YELLOW}[6/13] Creating validator key...${NC}"
if ./posd keys show $KEY_NAME --keyring-backend test --home $POSD_HOME >/dev/null 2>&1; then
    echo -e "${YELLOW}  ⚠ Key '$KEY_NAME' already exists${NC}"
    ADDRESS=$(./posd keys show $KEY_NAME -a --keyring-backend test --home $POSD_HOME)
else
    ./posd keys add $KEY_NAME --keyring-backend test --home $POSD_HOME 2>&1 | grep -v "mnemonic"
    ADDRESS=$(./posd keys show $KEY_NAME -a --keyring-backend test --home $POSD_HOME)
fi
echo -e "${GREEN}  ✓ Validator address: $ADDRESS${NC}"

# Step 7: Add genesis account
echo -e "${YELLOW}[7/13] Adding genesis account...${NC}"
./posd genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test --home $POSD_HOME 2>&1 > /dev/null
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Added 1,000,000 OMNI to genesis${NC}"
else
    echo -e "${RED}  ✗ Failed to add genesis account${NC}"
    exit 1
fi

# Step 8: Fix bond_denom BEFORE gentx
echo -e "${YELLOW}[8/13] Fixing staking bond_denom...${NC}"
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' $POSD_HOME/config/genesis.json
echo -e "${GREEN}  ✓ bond_denom set to uomni${NC}"

# Step 9: Configure minimum gas prices
echo -e "${YELLOW}[9/13] Configuring gas prices...${NC}"
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $POSD_HOME/config/app.toml
echo -e "${GREEN}  ✓ Set minimum gas price to 0.001uomni${NC}"

# Step 10: Create gentx (THIS CREATES THE VALIDATOR)
echo -e "${YELLOW}[10/13] Creating genesis transaction...${NC}"
./posd genesis gentx $KEY_NAME 100000000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test \
  --home $POSD_HOME 2>&1 > /dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ gentx created in: $POSD_HOME/config/gentx/${NC}"
    ls -lh $POSD_HOME/config/gentx/ 2>/dev/null | tail -1
else
    echo -e "${RED}  ✗ gentx creation failed${NC}"
    echo -e "${YELLOW}  This is expected if there are issues with custom modules${NC}"
    exit 1
fi

# Step 11: Collect gentxs (adds to genutil.gen_txs array)
echo -e "${YELLOW}[11/13] Collecting genesis transactions...${NC}"
./posd genesis collect-gentxs --home $POSD_HOME 2>&1 > /dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ gentxs collected${NC}"
else
    echo -e "${RED}  ✗ collect-gentxs failed${NC}"
    exit 1
fi

# Step 12: Fix bond_denom AGAIN (collect-gentxs overwrites it)
echo -e "${YELLOW}[12/13] Re-fixing bond_denom...${NC}"
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' $POSD_HOME/config/genesis.json
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ bond_denom fixed to uomni${NC}"
fi

# Step 13: Final validation
echo -e "${YELLOW}[13/13] Validating final genesis...${NC}"
./posd genesis validate-genesis --home $POSD_HOME 2>&1 > /dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Genesis validation passed${NC}"
else
    echo -e "${RED}  ✗ Genesis validation failed${NC}"
    exit 1
fi

echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${GREEN}✅ Chain Ready to Start${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Verification
echo -e "${YELLOW}Verification:${NC}"
if command -v jq &> /dev/null; then
    GEN_TXS=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.genutil.gen_txs | length')
    VALIDATORS=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.staking.validators | length')
    echo -e "  genutil.gen_txs: ${GREEN}$GEN_TXS${NC}"
    echo -e "  staking.validators: ${GREEN}$VALIDATORS${NC}"
fi

echo ""
echo -e "${CYAN}Next Steps:${NC}"
echo ""
echo -e "  ${GREEN}1. Start the chain:${NC}"
echo -e "     ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false --home $POSD_HOME"
echo ""
echo -e "  ${GREEN}2. Or start in background:${NC}"
echo -e "     nohup ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false --home $POSD_HOME > chain.log 2>&1 &"
echo ""
echo -e "  ${GREEN}3. Check status:${NC}"
echo -e "     ./posd status --home $POSD_HOME"
echo ""
echo -e "  ${GREEN}4. View logs (if background):${NC}"
echo -e "     tail -f chain.log"
echo ""
echo -e "  ${GREEN}5. Verify validator:${NC}"
echo -e "     grep 'This node is a validator' chain.log"
echo ""
echo -e "${CYAN}Key Information:${NC}"
echo -e "  Chain ID: ${GREEN}$CHAIN_ID${NC}"
echo -e "  Moniker:  ${GREEN}$MONIKER${NC}"
echo -e "  Home Dir: ${GREEN}$POSD_HOME${NC}"
echo -e "  Address:  ${GREEN}$ADDRESS${NC}"
echo ""
echo -e "${YELLOW}Expected on startup:${NC}"
echo -e "  - 'This node is a validator' message"
echo -e "  - FeeMarket will use defaults if genesis is empty"
echo -e "  - Reflection service error is non-fatal"
echo ""
