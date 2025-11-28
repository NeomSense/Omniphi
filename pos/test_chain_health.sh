#!/bin/bash
################################################################################
# Chain Health Test Script
# Tests basic chain functionality and validator status
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Chain Health Check${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Test 1: Check if posd binary exists
echo -e "${YELLOW}[1/5] Checking posd binary...${NC}"
if [ -f ./posd ]; then
    echo -e "${GREEN}  ✓ posd binary found${NC}"
    ./posd version
else
    echo -e "${RED}  ✗ posd binary not found${NC}"
    echo -e "${YELLOW}  Run: go build -o posd ./cmd/posd${NC}"
    exit 1
fi
echo ""

# Test 2: Check if chain is running
echo -e "${YELLOW}[2/5] Checking if chain is running...${NC}"
if ./posd status &> /dev/null; then
    echo -e "${GREEN}  ✓ Chain is running${NC}"
    HEIGHT=$(./posd status 2>/dev/null | jq -r '.sync_info.latest_block_height')
    echo -e "  Current block height: ${GREEN}$HEIGHT${NC}"
else
    echo -e "${RED}  ✗ Chain is not running${NC}"
    echo -e "${YELLOW}  Start with: ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false${NC}"
    exit 1
fi
echo ""

# Test 3: Check validator status
echo -e "${YELLOW}[3/5] Checking validator status...${NC}"
if [ -f chain.log ]; then
    if grep -q "This node is a validator" chain.log; then
        echo -e "${GREEN}  ✓ Node is a validator${NC}"
        # Extract validator address - use sed instead of grep -P for compatibility
        VALIDATOR_ADDR=$(grep "This node is a validator" chain.log | tail -1 | sed -n 's/.*addr=\([A-F0-9]*\).*/\1/p')
        if [ -n "$VALIDATOR_ADDR" ]; then
            echo -e "  Validator address: ${GREEN}$VALIDATOR_ADDR${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ Node is not a validator${NC}"
    fi
else
    echo -e "${YELLOW}  ⚠ chain.log not found (chain may be running in foreground)${NC}"
fi
echo ""

# Test 4: Check block production
echo -e "${YELLOW}[4/5] Testing block production...${NC}"
INITIAL_HEIGHT=$(./posd status 2>/dev/null | jq -r '.sync_info.latest_block_height')
echo -e "  Initial height: $INITIAL_HEIGHT"
echo -e "  Waiting 10 seconds..."
sleep 10
NEW_HEIGHT=$(./posd status 2>/dev/null | jq -r '.sync_info.latest_block_height')
echo -e "  New height: $NEW_HEIGHT"

if [ "$NEW_HEIGHT" -gt "$INITIAL_HEIGHT" ]; then
    BLOCKS_PRODUCED=$((NEW_HEIGHT - INITIAL_HEIGHT))
    echo -e "${GREEN}  ✓ Blocks are being produced ($BLOCKS_PRODUCED blocks in 10s)${NC}"
else
    echo -e "${RED}  ✗ No blocks produced${NC}"
    exit 1
fi
echo ""

# Test 5: Check module queries
echo -e "${YELLOW}[5/5] Checking module availability...${NC}"
MODULES_OK=true

if ./posd query feemarket base-fee &> /dev/null; then
    echo -e "${GREEN}  ✓ FeeMarket module responding${NC}"
else
    echo -e "${RED}  ✗ FeeMarket module not responding${NC}"
    MODULES_OK=false
fi

if ./posd query tokenomics params &> /dev/null; then
    echo -e "${GREEN}  ✓ Tokenomics module responding${NC}"
else
    echo -e "${RED}  ✗ Tokenomics module not responding${NC}"
    MODULES_OK=false
fi

if ./posd query poc params &> /dev/null; then
    echo -e "${GREEN}  ✓ POC module responding${NC}"
else
    echo -e "${RED}  ✗ POC module not responding${NC}"
    MODULES_OK=false
fi
echo ""

# Summary
echo -e "${CYAN}================================${NC}"
if [ "$MODULES_OK" = true ]; then
    echo -e "${GREEN}✅ All health checks passed!${NC}"
    echo -e "${CYAN}================================${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some checks failed${NC}"
    echo -e "${CYAN}================================${NC}"
    exit 1
fi
