#!/bin/bash
################################################################################
# Omniphi Ubuntu Diagnostic and Fix Script
# Run this to diagnose and fix the validator genesis issue
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Omniphi Ubuntu Diagnostics${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Detect home directory
if [ -d ~/.pos ]; then
    POSD_HOME=~/.pos
elif [ -d ~/.posd ]; then
    POSD_HOME=~/.posd
else
    echo -e "${RED}No chain data found. Run setup_ubuntu_fixed.sh first.${NC}"
    exit 1
fi

echo -e "${YELLOW}Using home directory: $POSD_HOME${NC}"
echo ""

# Check if binary exists
echo -e "${CYAN}[1/5] Checking binary...${NC}"
if [ -f ./posd ]; then
    echo -e "${GREEN}  ✓ posd binary exists${NC}"
    ./posd version 2>/dev/null || echo -e "${YELLOW}  ⚠ Version check failed${NC}"
else
    echo -e "${RED}  ✗ posd binary not found!${NC}"
    echo -e "${YELLOW}  Building now...${NC}"
    go build -o posd ./cmd/posd
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  ✓ Build successful${NC}"
    else
        echo -e "${RED}  ✗ Build failed${NC}"
        exit 1
    fi
fi
echo ""

# Check genesis file
echo -e "${CYAN}[2/5] Checking genesis.json...${NC}"
if [ -f "$POSD_HOME/config/genesis.json" ]; then
    echo -e "${GREEN}  ✓ genesis.json exists${NC}"

    # Check for gentx
    if command -v jq &> /dev/null; then
        GEN_TXS=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.genutil.gen_txs | length')
        BOND_DENOM=$(cat $POSD_HOME/config/genesis.json | jq -r '.app_state.staking.params.bond_denom')

        echo -e "  genutil.gen_txs count: ${YELLOW}$GEN_TXS${NC}"
        echo -e "  bond_denom: ${YELLOW}$BOND_DENOM${NC}"

        if [ "$GEN_TXS" == "0" ] || [ "$GEN_TXS" == "null" ]; then
            echo -e "${RED}  ✗ PROBLEM: No genesis transactions found!${NC}"
            echo -e "${YELLOW}  This is why you're getting 'validator set is empty' error${NC}"
            NEEDS_RESET=1
        else
            echo -e "${GREEN}  ✓ Genesis transactions present${NC}"
        fi

        if [ "$BOND_DENOM" != "uomni" ]; then
            echo -e "${RED}  ✗ PROBLEM: bond_denom is '$BOND_DENOM' but should be 'uomni'${NC}"
            NEEDS_RESET=1
        else
            echo -e "${GREEN}  ✓ bond_denom is correct${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ jq not installed, cannot validate genesis details${NC}"
        echo -e "${YELLOW}  Install with: sudo apt-get install jq${NC}"
    fi
else
    echo -e "${RED}  ✗ genesis.json not found!${NC}"
    NEEDS_RESET=1
fi
echo ""

# Check for gentx files
echo -e "${CYAN}[3/5] Checking gentx directory...${NC}"
if [ -d "$POSD_HOME/config/gentx" ]; then
    GENTX_COUNT=$(ls -1 $POSD_HOME/config/gentx/*.json 2>/dev/null | wc -l)
    echo -e "  gentx files found: ${YELLOW}$GENTX_COUNT${NC}"

    if [ "$GENTX_COUNT" -eq 0 ]; then
        echo -e "${RED}  ✗ PROBLEM: No gentx files!${NC}"
        NEEDS_RESET=1
    else
        echo -e "${GREEN}  ✓ gentx files present${NC}"
        ls -lh $POSD_HOME/config/gentx/
    fi
else
    echo -e "${RED}  ✗ gentx directory doesn't exist${NC}"
    NEEDS_RESET=1
fi
echo ""

# Check previous startup command
echo -e "${CYAN}[4/5] Checking chain.log for startup issues...${NC}"
if [ -f chain.log ]; then
    # Check if it shows help output (indicating wrong command)
    if grep -q "Available Commands:" chain.log; then
        echo -e "${RED}  ✗ PROBLEM: Last startup showed help output${NC}"
        echo -e "${YELLOW}  This means the startup command was incorrect${NC}"

        # Show last command attempt
        echo -e "${YELLOW}  Last few lines of log:${NC}"
        tail -5 chain.log
    elif grep -q "validator set is empty" chain.log; then
        echo -e "${RED}  ✗ PROBLEM: Validator set is empty${NC}"
        NEEDS_RESET=1
    elif grep -q "This node is a validator" chain.log; then
        echo -e "${GREEN}  ✓ Chain started successfully before${NC}"
        echo -e "${YELLOW}  Check if still running:${NC}"
        ps aux | grep posd | grep -v grep || echo -e "${YELLOW}  Not currently running${NC}"
    fi
else
    echo -e "${YELLOW}  ⚠ No chain.log found${NC}"
fi
echo ""

# Summary and recommendation
echo -e "${CYAN}[5/5] Summary and Recommendation${NC}"
echo ""

if [ "$NEEDS_RESET" == "1" ]; then
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  DIAGNOSIS: Genesis setup incomplete${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}The chain needs to be re-initialized with proper gentx workflow.${NC}"
    echo ""
    echo -e "${GREEN}SOLUTION:${NC}"
    echo ""
    echo -e "  ${CYAN}1. Stop any running chain:${NC}"
    echo -e "     pkill posd"
    echo ""
    echo -e "  ${CYAN}2. Run the setup script:${NC}"
    echo -e "     chmod +x setup_ubuntu_fixed.sh"
    echo -e "     ./setup_ubuntu_fixed.sh"
    echo ""
    echo -e "  ${CYAN}3. When prompted, choose 'y' to remove old data${NC}"
    echo ""
    echo -e "  ${CYAN}4. After setup completes, start the chain:${NC}"
    echo -e "     ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false --home $POSD_HOME"
    echo ""
else
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  DIAGNOSIS: Genesis appears valid${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}The genesis is set up correctly. Try starting the chain:${NC}"
    echo ""
    echo -e "  ${CYAN}1. Stop any running instances:${NC}"
    echo -e "     pkill posd"
    echo ""
    echo -e "  ${CYAN}2. Clear old logs:${NC}"
    echo -e "     rm chain.log"
    echo ""
    echo -e "  ${CYAN}3. Start fresh:${NC}"
    echo -e "     ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false --home $POSD_HOME 2>&1 | tee chain.log"
    echo ""
    echo -e "  ${CYAN}4. Watch for validator message:${NC}"
    echo -e "     Should see: 'This node is a validator'"
    echo ""
fi

echo ""
echo -e "${CYAN}Need help? Check these files:${NC}"
echo -e "  - ${YELLOW}VALIDATOR_FIX_SUMMARY.md${NC} - Complete explanation"
echo -e "  - ${YELLOW}START_HERE.md${NC} - Step-by-step guide"
echo ""
