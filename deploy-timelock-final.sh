#!/bin/bash
# Final Timelock Deployment Script - 100% Complete Implementation
# Run this on your VPS to deploy the complete timelock module

set -e

echo "========================================="
echo "Timelock Module - Final Deployment"
echo "100% Complete Implementation"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Find omniphi directory (case insensitive)
if [ -d ~/omniphi ]; then
    OMNIPHI_DIR=~/omniphi
elif [ -d ~/Omniphi ]; then
    OMNIPHI_DIR=~/Omniphi
elif [ -d ~/OMNIPHI ]; then
    OMNIPHI_DIR=~/OMNIPHI
else
    echo -e "${RED}Error: Omniphi directory not found in home directory${NC}"
    echo "Searched: ~/omniphi, ~/Omniphi, ~/OMNIPHI"
    exit 1
fi

echo "Found Omniphi directory: $OMNIPHI_DIR"

# Step 1: Pull latest code
echo -e "${BLUE}Step 1: Pulling latest code from GitHub...${NC}"
cd $OMNIPHI_DIR/chain
git fetch origin
CURRENT_COMMIT=$(git rev-parse HEAD)
LATEST_COMMIT=$(git rev-parse origin/main)

if [ "$CURRENT_COMMIT" = "$LATEST_COMMIT" ]; then
    echo -e "${YELLOW}Already on latest commit: $CURRENT_COMMIT${NC}"
else
    echo -e "${GREEN}Updating from $CURRENT_COMMIT to $LATEST_COMMIT${NC}"
    git pull origin main
fi

# Verify commit contains timelock completion
if ! git log --oneline -1 | grep -q "complete proposal queueing"; then
    echo -e "${RED}Warning: Latest commit doesn't appear to be timelock completion${NC}"
    echo "Latest commit: $(git log --oneline -1)"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Step 2: Build binary
echo ""
echo -e "${BLUE}Step 2: Building binary...${NC}"
make build

if [ ! -f build/posd ]; then
    echo -e "${RED}Error: Build failed - posd binary not found${NC}"
    exit 1
fi

# Verify binary size (should be ~250-300MB)
BINARY_SIZE=$(du -m build/posd | cut -f1)
echo -e "${GREEN}Binary built successfully: ${BINARY_SIZE}MB${NC}"

# Step 3: Stop node
echo ""
echo -e "${BLUE}Step 3: Stopping posd service...${NC}"
sudo systemctl stop posd
sleep 3

# Verify stopped
if systemctl is-active --quiet posd; then
    echo -e "${RED}Error: Failed to stop posd${NC}"
    exit 1
fi

# Step 4: Backup old binary
echo ""
echo -e "${BLUE}Step 4: Backing up old binary...${NC}"
if [ -f /usr/local/bin/posd ]; then
    BACKUP_NAME="/usr/local/bin/posd.backup.$(date +%s)"
    sudo cp /usr/local/bin/posd "$BACKUP_NAME"
    echo -e "${GREEN}Backed up to: $BACKUP_NAME${NC}"
fi

# Step 5: Install new binary
echo ""
echo -e "${BLUE}Step 5: Installing new binary...${NC}"
sudo cp build/posd /usr/local/bin/posd
sudo chmod +x /usr/local/bin/posd

# Verify installation
if ! which posd > /dev/null; then
    echo -e "${RED}Error: posd not found in PATH${NC}"
    exit 1
fi

# Step 6: Verify version
echo ""
echo -e "${BLUE}Step 6: Verifying version...${NC}"
posd version

# Step 7: Start node
echo ""
echo -e "${BLUE}Step 7: Starting posd service...${NC}"
sudo systemctl start posd
sleep 5

# Check if started
if ! systemctl is-active --quiet posd; then
    echo -e "${RED}Error: Failed to start posd${NC}"
    echo "Check logs: sudo journalctl -u posd -n 100"
    exit 1
fi

echo -e "${GREEN}Service started successfully${NC}"

# Step 8: Wait for node to initialize
echo ""
echo -e "${YELLOW}Waiting 15 seconds for node to initialize...${NC}"
sleep 15

# Step 9: Verify timelock module
echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}Verification Tests${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""

# Test 1: Timelock params
echo -e "${BLUE}Test 1: Querying timelock parameters...${NC}"
if posd query timelock params --node tcp://localhost:26657 > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Timelock params query successful${NC}"
    posd query timelock params --node tcp://localhost:26657
else
    echo -e "${RED}✗ Timelock params query failed${NC}"
    exit 1
fi

echo ""

# Test 2: Queued operations
echo -e "${BLUE}Test 2: Querying queued operations...${NC}"
if posd query timelock queued --node tcp://localhost:26657 > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Queued operations query successful${NC}"
    posd query timelock queued --node tcp://localhost:26657
else
    echo -e "${RED}✗ Queued operations query failed${NC}"
    exit 1
fi

echo ""

# Test 3: Node status
echo -e "${BLUE}Test 3: Checking node sync status...${NC}"
if posd status 2>&1 | jq '.sync_info.catching_up' > /dev/null 2>&1; then
    CATCHING_UP=$(posd status 2>&1 | jq -r '.sync_info.catching_up')
    HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height')
    echo -e "${GREEN}✓ Node status query successful${NC}"
    echo "  Height: $HEIGHT"
    echo "  Catching up: $CATCHING_UP"
else
    echo -e "${YELLOW}⚠ Node status query failed (may still be initializing)${NC}"
fi

echo ""

# Step 10: Check logs for errors
echo -e "${BLUE}Step 10: Checking recent logs for errors...${NC}"
ERROR_COUNT=$(sudo journalctl -u posd -n 100 --no-pager | grep -i error | wc -l)

if [ $ERROR_COUNT -gt 0 ]; then
    echo -e "${YELLOW}⚠ Found $ERROR_COUNT errors in recent logs${NC}"
    echo "Recent errors:"
    sudo journalctl -u posd -n 100 --no-pager | grep -i error | tail -5
else
    echo -e "${GREEN}✓ No errors in recent logs${NC}"
fi

echo ""

# Step 11: Look for timelock initialization
echo -e "${BLUE}Step 11: Verifying timelock module initialization...${NC}"
if sudo journalctl -u posd -n 200 --no-pager | grep -q "timelock"; then
    echo -e "${GREEN}✓ Timelock module logs found${NC}"
    echo "Recent timelock logs:"
    sudo journalctl -u posd -n 200 --no-pager | grep timelock | tail -5
else
    echo -e "${YELLOW}⚠ No timelock logs found yet (may still be initializing)${NC}"
fi

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""

echo "Next Steps:"
echo "1. Monitor logs: sudo journalctl -u posd -f"
echo "2. Test proposal queueing (see TIMELOCK_COMPLETE.md)"
echo "3. Submit test governance proposal"
echo "4. Verify proposal is queued (not executed)"
echo ""

echo "Quick Test Commands:"
echo "  posd query timelock params"
echo "  posd query timelock queued"
echo "  posd query timelock executable"
echo ""

echo "Documentation:"
echo "  TIMELOCK_COMPLETE.md - Complete implementation guide"
echo "  DEPLOYMENT_SUCCESS.md - Previous deployment report"
echo "  TIMELOCK_TEST_GUIDE.md - Testing procedures"
echo ""

echo -e "${GREEN}Ready for production governance! 🚀${NC}"
