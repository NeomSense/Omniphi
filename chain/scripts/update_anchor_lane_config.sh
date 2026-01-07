#!/bin/bash
# =============================================================================
# Omniphi Anchor Lane Configuration Update Script
# =============================================================================
# This script updates your VPS validator node to use the new anchor lane
# configuration with optimized block time, TPS, and gas limits.
#
# Configuration Summary:
#   - Block time: 4 seconds (down from ~5-6s)
#   - Target TPS: ~100 (sustainable range: 50-150)
#   - Max block gas: 60,000,000
#   - Max tx gas: 5,000,000
#   - Target utilization: 33%
#
# IMPORTANT: This script should be run on each validator node.
# The chain must be stopped before running this script.
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default paths (can be overridden via environment variables)
POSD_HOME="${POSD_HOME:-$HOME/.posd}"
CONFIG_DIR="${POSD_HOME}/config"
CONFIG_TOML="${CONFIG_DIR}/config.toml"
APP_TOML="${CONFIG_DIR}/app.toml"
GENESIS_JSON="${CONFIG_DIR}/genesis.json"

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Omniphi Anchor Lane Configuration Update${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# =============================================================================
# Pre-flight checks
# =============================================================================

echo -e "${YELLOW}[1/6] Running pre-flight checks...${NC}"

# Check if running as appropriate user
if [ "$(id -u)" = "0" ]; then
    echo -e "${YELLOW}Warning: Running as root. Consider using a dedicated validator user.${NC}"
fi

# Check if posd is running
if pgrep -x "posd" > /dev/null 2>&1; then
    echo -e "${RED}ERROR: posd is still running!${NC}"
    echo -e "${RED}Please stop the node first: sudo systemctl stop posd${NC}"
    exit 1
fi

# Check if config files exist
if [ ! -f "$CONFIG_TOML" ]; then
    echo -e "${RED}ERROR: config.toml not found at $CONFIG_TOML${NC}"
    echo -e "${RED}Set POSD_HOME environment variable if using non-default location.${NC}"
    exit 1
fi

if [ ! -f "$GENESIS_JSON" ]; then
    echo -e "${RED}ERROR: genesis.json not found at $GENESIS_JSON${NC}"
    exit 1
fi

echo -e "${GREEN}Pre-flight checks passed.${NC}"

# =============================================================================
# Backup current configuration
# =============================================================================

echo -e "${YELLOW}[2/6] Backing up current configuration...${NC}"

BACKUP_DIR="${POSD_HOME}/config_backup_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"
cp "$CONFIG_TOML" "$BACKUP_DIR/"
cp "$GENESIS_JSON" "$BACKUP_DIR/"
[ -f "$APP_TOML" ] && cp "$APP_TOML" "$BACKUP_DIR/"

echo -e "${GREEN}Backup saved to: $BACKUP_DIR${NC}"

# =============================================================================
# Update CometBFT consensus timeouts (config.toml)
# =============================================================================

echo -e "${YELLOW}[3/6] Updating CometBFT consensus timeouts for 4s block time...${NC}"

# The key changes for 4-second block time:
# - timeout_propose: 1500ms (fast proposal)
# - timeout_propose_delta: 200ms
# - timeout_prevote: 500ms
# - timeout_prevote_delta: 200ms
# - timeout_precommit: 500ms
# - timeout_precommit_delta: 200ms
# - timeout_commit: 1500ms (main delay between blocks)

# Use sed to update consensus timeouts
sed -i.bak \
    -e 's/^timeout_propose = .*/timeout_propose = "1500ms"/' \
    -e 's/^timeout_propose_delta = .*/timeout_propose_delta = "200ms"/' \
    -e 's/^timeout_prevote = .*/timeout_prevote = "500ms"/' \
    -e 's/^timeout_prevote_delta = .*/timeout_prevote_delta = "200ms"/' \
    -e 's/^timeout_precommit = .*/timeout_precommit = "500ms"/' \
    -e 's/^timeout_precommit_delta = .*/timeout_precommit_delta = "200ms"/' \
    -e 's/^timeout_commit = .*/timeout_commit = "1500ms"/' \
    "$CONFIG_TOML"

# Enable prometheus metrics for monitoring
sed -i \
    -e 's/^prometheus = false/prometheus = true/' \
    "$CONFIG_TOML"

echo -e "${GREEN}CometBFT config updated.${NC}"

# =============================================================================
# Update genesis.json consensus params (for new chains only)
# =============================================================================

echo -e "${YELLOW}[4/6] Checking genesis consensus parameters...${NC}"

# Check current max_gas in genesis
CURRENT_MAX_GAS=$(cat "$GENESIS_JSON" | python3 -c "import sys, json; d = json.load(sys.stdin); print(d.get('consensus', {}).get('params', {}).get('block', {}).get('max_gas', 'N/A'))" 2>/dev/null || echo "N/A")

if [ "$CURRENT_MAX_GAS" = "60000000" ]; then
    echo -e "${GREEN}Genesis already configured with correct max_gas (60M).${NC}"
else
    echo -e "${YELLOW}Current max_gas: $CURRENT_MAX_GAS${NC}"
    echo -e "${YELLOW}NOTE: Genesis consensus params can only be changed via governance${NC}"
    echo -e "${YELLOW}      or by coordinated chain restart with new genesis.${NC}"
    echo ""
    echo -e "${BLUE}Required genesis values:${NC}"
    echo "  consensus.params.block.max_bytes: 10485760 (10MB)"
    echo "  consensus.params.block.max_gas: 60000000 (60M)"
fi

# =============================================================================
# Update feemarket parameters (via governance or genesis)
# =============================================================================

echo -e "${YELLOW}[5/6] Displaying required feemarket parameters...${NC}"

echo -e "${BLUE}Required feemarket module params (update via governance proposal):${NC}"
cat << 'EOF'
{
  "min_gas_price": "0.025",
  "base_fee_initial": "0.025",
  "elasticity_multiplier": "1.125",
  "target_block_utilization": "0.33",
  "max_tx_gas": "2000000",
  "burn_cool": "0.10",
  "burn_normal": "0.20",
  "burn_hot": "0.40",
  "util_cool_threshold": "0.16",
  "util_hot_threshold": "0.33",
  "max_burn_ratio": "0.50"
}
EOF

# =============================================================================
# Verify configuration
# =============================================================================

echo ""
echo -e "${YELLOW}[6/6] Verifying configuration...${NC}"

# Extract and display new timeout values
echo -e "${BLUE}New CometBFT consensus timeouts:${NC}"
grep -E "^timeout_(propose|prevote|precommit|commit)" "$CONFIG_TOML" | head -10

echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Configuration Update Complete!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "1. Review the changes above"
echo "2. Restart your node: sudo systemctl start posd"
echo "3. Monitor block times: posd status | jq '.SyncInfo.latest_block_time'"
echo "4. Check metrics at http://localhost:26660/metrics"
echo ""
echo -e "${YELLOW}For multi-validator networks:${NC}"
echo "- Run this script on ALL validator nodes"
echo "- Restart validators one at a time to avoid network disruption"
echo "- Wait for each validator to sync before restarting the next"
echo ""
echo -e "${BLUE}Monitoring anchor lane health:${NC}"
echo "- Target block time: 4.0s (acceptable: 3.5-4.5s)"
echo "- Target utilization: 33% (warning at 70%, critical at 90%)"
echo "- Target TPS: ~100 (range: 50-150)"
echo ""
echo -e "${YELLOW}Backup location: $BACKUP_DIR${NC}"
