#!/bin/bash
# Timelock Module Deployment Script
# Run this on your VPS to deploy and test the timelock integration

set -e

echo "=== Timelock Module Deployment ==="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Stop the node
echo -e "${BLUE}Step 1: Stopping posd service...${NC}"
sudo systemctl stop posd
sleep 2

# Step 2: Pull latest code
echo -e "${BLUE}Step 2: Pulling latest code...${NC}"
cd ~/omniphi/chain
git pull origin main

# Step 3: Build
echo -e "${BLUE}Step 3: Building binary...${NC}"
make build

# Step 4: Install
echo -e "${BLUE}Step 4: Installing binary...${NC}"
sudo cp build/posd /usr/local/bin/posd
sudo chmod +x /usr/local/bin/posd

# Step 5: Verify version
echo -e "${BLUE}Step 5: Verifying version...${NC}"
posd version

# Step 6: Start the node
echo -e "${BLUE}Step 6: Starting posd service...${NC}"
sudo systemctl start posd
sleep 5

# Step 7: Check status
echo -e "${BLUE}Step 7: Checking service status...${NC}"
sudo systemctl status posd --no-pager | head -20

# Wait for node to catch up
echo ""
echo -e "${YELLOW}Waiting 10 seconds for node to initialize...${NC}"
sleep 10

# Step 8: Test queries
echo ""
echo -e "${GREEN}=== Phase 1: Verify Module Integration ===${NC}"
echo ""

echo -e "${BLUE}Testing: posd query timelock params${NC}"
posd query timelock params --node tcp://localhost:26657

echo ""
echo -e "${BLUE}Testing: posd query timelock queued${NC}"
posd query timelock queued --node tcp://localhost:26657

echo ""
echo -e "${BLUE}Testing: posd query timelock executable${NC}"
posd query timelock executable --node tcp://localhost:26657

echo ""
echo -e "${GREEN}=== Phase 2: Guardian Setup ===${NC}"
echo ""

# Get validator address for guardian
GUARDIAN=$(posd keys show validator -a 2>/dev/null || echo "")

if [ -z "$GUARDIAN" ]; then
    echo -e "${YELLOW}Warning: validator key not found${NC}"
    echo "Create a guardian address with: posd keys add guardian"
else
    echo -e "${GREEN}Guardian address: $GUARDIAN${NC}"
    echo ""
    echo "To set this as guardian, create a governance proposal:"
    echo ""
    cat <<EOF
cat > guardian-proposal.json <<'PROPOSAL'
{
  "messages": [
    {
      "@type": "/pos.timelock.v1.MsgUpdateGuardian",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700jdv5h4y",
      "new_guardian": "$GUARDIAN"
    }
  ],
  "metadata": "ipfs://QmTimelock",
  "deposit": "10000000uomni",
  "title": "Set Timelock Guardian",
  "summary": "Set the guardian address for timelock emergency operations"
}
PROPOSAL

posd tx gov submit-proposal guardian-proposal.json \\
  --from validator \\
  --chain-id pos \\
  --gas auto \\
  --gas-adjustment 1.5 \\
  --yes
EOF
fi

echo ""
echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""
echo "Next steps:"
echo "1. Submit guardian proposal (see above)"
echo "2. Vote: posd tx gov vote 1 yes --from validator --chain-id pos --yes"
echo "3. Wait for voting period to end"
echo "4. Verify proposal is queued in timelock (not executed immediately)"
echo ""
echo "For full testing guide, see: chain/TIMELOCK_TEST_GUIDE.md"
