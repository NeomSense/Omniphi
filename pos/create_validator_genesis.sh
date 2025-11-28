#!/bin/bash
################################################################################
# Create Proper Genesis with Validator
# This script uses gentx + collect-gentxs the RIGHT way
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
POSD_HOME=~/.pos

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Create Genesis with Validator${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Step 1: Clean everything
echo -e "${YELLOW}[1/9] Cleaning old data...${NC}"
if [ -d "$POSD_HOME" ]; then
    rm -rf "$POSD_HOME"
    echo -e "${GREEN}  ✓ Removed $POSD_HOME${NC}"
fi

# Step 2: Initialize chain
echo -e "${YELLOW}[2/9] Initializing chain...${NC}"
./posd init $MONIKER --chain-id $CHAIN_ID --home $POSD_HOME
echo -e "${GREEN}  ✓ Chain initialized${NC}"

# Step 3: Create validator key
echo -e "${YELLOW}[3/9] Creating validator key...${NC}"
./posd keys add $KEY_NAME --keyring-backend test --home $POSD_HOME 2>&1 | grep -v "mnemonic"
ADDRESS=$(./posd keys show $KEY_NAME -a --keyring-backend test --home $POSD_HOME)
echo -e "${GREEN}  ✓ Validator address: $ADDRESS${NC}"

# Step 4: Add genesis account
echo -e "${YELLOW}[4/9] Adding genesis account...${NC}"
./posd genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test --home $POSD_HOME
echo -e "${GREEN}  ✓ Added 1,000,000 OMNI${NC}"

# Step 5: Fix bond_denom BEFORE gentx
echo -e "${YELLOW}[5/9] Fixing staking bond_denom...${NC}"
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' $POSD_HOME/config/genesis.json
echo -e "${GREEN}  ✓ bond_denom set to uomni${NC}"

# Step 6: Configure gas prices
echo -e "${YELLOW}[6/9] Configuring gas prices...${NC}"
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $POSD_HOME/config/app.toml
echo -e "${GREEN}  ✓ Set minimum gas price${NC}"

# Step 7: Create gentx
echo -e "${YELLOW}[7/9] Creating genesis transaction...${NC}"
./posd genesis gentx $KEY_NAME 100000000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test \
  --home $POSD_HOME 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ gentx created in: $POSD_HOME/config/gentx/${NC}"
    ls -lh $POSD_HOME/config/gentx/
else
    echo -e "${RED}  ✗ gentx failed - checking error...${NC}"
    exit 1
fi

# Step 8: Collect gentxs
echo -e "${YELLOW}[8/9] Collecting genesis transactions...${NC}"
./posd genesis collect-gentxs --home $POSD_HOME 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ gentxs collected${NC}"
else
    echo -e "${RED}  ✗ collect-gentxs failed${NC}"
    exit 1
fi

# Step 9: Validate
echo -e "${YELLOW}[9/9] Validating genesis...${NC}"
./posd genesis validate-genesis --home $POSD_HOME 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ Genesis valid${NC}"
else
    echo -e "${RED}  ✗ Validation failed${NC}"
    exit 1
fi

echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${GREEN}✅ SUCCESS${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Verification
echo -e "${YELLOW}Verification:${NC}"
GEN_TXS=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.genutil.gen_txs | length')
VALIDATORS=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.staking.validators | length')

echo -e "  genutil.gen_txs: ${GREEN}$GEN_TXS${NC}"
echo -e "  staking.validators: ${GREEN}$VALIDATORS${NC}"

if [ "$GEN_TXS" -gt 0 ] && [ "$VALIDATORS" -gt 0 ]; then
    echo ""
    echo -e "${GREEN}✅ Genesis has both gen_txs AND validators!${NC}"
    echo ""
    echo -e "${CYAN}Start chain:${NC}"
    echo -e "  ./posd start --minimum-gas-prices 0.001uomni --home $POSD_HOME"
else
    echo -e "${RED}⚠ Something is wrong - check genesis manually${NC}"
fi

echo ""
