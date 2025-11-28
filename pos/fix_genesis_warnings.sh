#!/bin/bash
################################################################################
# Optional: Fix genesis warnings (max block gas, tokenomics supply)
# Run this AFTER setup_ubuntu_fixed.sh and BEFORE starting the chain
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Detect home directory
if [ -d ~/.pos ]; then
    POSD_HOME=~/.pos
elif [ -d ~/.posd ]; then
    POSD_HOME=~/.posd
else
    echo -e "${RED}No chain data found. Run setup_ubuntu_fixed.sh first.${NC}"
    exit 1
fi

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Fixing Genesis Warnings${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

GENESIS="$POSD_HOME/config/genesis.json"

if [ ! -f "$GENESIS" ]; then
    echo -e "${RED}Genesis file not found at $GENESIS${NC}"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}jq is required but not installed${NC}"
    echo -e "${YELLOW}Install with: sudo apt-get install jq${NC}"
    exit 1
fi

echo -e "${YELLOW}[1/2] Setting max block gas...${NC}"
# Set max block gas to 10 million (common value)
jq '.consensus.params.block.max_gas = "10000000"' "$GENESIS" > "$GENESIS.tmp" && mv "$GENESIS.tmp" "$GENESIS"
echo -e "${GREEN}  ✓ Set max_gas to 10,000,000${NC}"

echo -e "${YELLOW}[2/2] Fixing tokenomics supply...${NC}"
# Get the genesis account balance
GENESIS_BALANCE=$(jq -r '.app_state.bank.balances[0].coins[0].amount' "$GENESIS")

if [ "$GENESIS_BALANCE" != "null" ] && [ ! -z "$GENESIS_BALANCE" ]; then
    # Set tokenomics supply to match genesis balance
    jq ".app_state.tokenomics.supply = \"$GENESIS_BALANCE\"" "$GENESIS" > "$GENESIS.tmp" && mv "$GENESIS.tmp" "$GENESIS"
    echo -e "${GREEN}  ✓ Set tokenomics supply to $GENESIS_BALANCE${NC}"
else
    echo -e "${YELLOW}  ⚠ Could not detect genesis balance, skipping${NC}"
fi

echo ""
echo -e "${GREEN}✅ Genesis warnings fixed!${NC}"
echo ""
echo -e "${CYAN}Verification:${NC}"
MAX_GAS=$(jq -r '.consensus.params.block.max_gas' "$GENESIS")
TOKEN_SUPPLY=$(jq -r '.app_state.tokenomics.supply' "$GENESIS")
echo -e "  Max block gas: ${GREEN}$MAX_GAS${NC}"
echo -e "  Tokenomics supply: ${GREEN}$TOKEN_SUPPLY${NC}"
echo ""
echo -e "${YELLOW}Now start the chain with:${NC}"
echo -e "  ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false"
echo ""
