#!/bin/bash
# =============================================================================
# Omniphi Testnet Initialization Script
# =============================================================================
# This script initializes a fresh testnet with correct tokenomics:
#   - Total Supply: 1.5 Billion OMNI = 1,500,000,000,000,000 omniphi
#   - Chain ID: omniphi-testnet-2
#   - Block time: ~4 seconds (CometBFT config)
#   - Voting period: 5 minutes (for testnet fast iteration)
#
# Usage: ./init_testnet.sh [validator1|validator2]
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
CHAIN_ID="omniphi-testnet-2"
BOND_DENOM="omniphi"
DISPLAY_DENOM="OMNI"

# Tokenomics (1 OMNI = 1,000,000 omniphi, 6 decimals)
# Total: 1.5 Billion OMNI = 1,500,000,000 OMNI = 1,500,000,000,000,000 omniphi
TOTAL_SUPPLY="1500000000000000"  # 1.5 quadrillion omniphi = 1.5B OMNI

# Distribution for testnet:
# - Validator 1: 500M OMNI (for staking + operations)
# - Validator 2: 500M OMNI (for staking + operations)
# - Community/Treasury: 500M OMNI (held by validator1 for now)
VALIDATOR1_AMOUNT="1000000000000000"  # 1 quadrillion = 1B OMNI (500M stake + 500M community)
VALIDATOR2_AMOUNT="500000000000000"   # 500 trillion = 500M OMNI

# Staking amounts (use ~10% of each validator's allocation)
VALIDATOR1_STAKE="100000000000000"    # 100T omniphi = 100M OMNI
VALIDATOR2_STAKE="50000000000000"     # 50T omniphi = 50M OMNI

# Governance (testnet: fast voting for iteration)
MIN_DEPOSIT="10000000000"             # 10K OMNI = 10,000,000,000 omniphi
VOTING_PERIOD="300s"                  # 5 minutes for testnet
EXPEDITED_VOTING="60s"                # 1 minute for expedited

# Anchor Lane Parameters
BLOCK_MAX_GAS="60000000"              # 60M gas per block
MAX_TX_GAS="2000000"                  # 2M gas per tx

# Paths
POSD_HOME="${POSD_HOME:-$HOME/.pos}"
CONFIG_DIR="${POSD_HOME}/config"

# Validator info
VALIDATOR_TYPE="${1:-validator1}"

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Omniphi Testnet Initialization${NC}"
echo -e "${BLUE}  Chain ID: ${CHAIN_ID}${NC}"
echo -e "${BLUE}  Validator: ${VALIDATOR_TYPE}${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# =============================================================================
# Step 1: Clean up existing data
# =============================================================================
echo -e "${YELLOW}[1/7] Cleaning up existing chain data...${NC}"

if [ -d "${POSD_HOME}/data" ]; then
    rm -rf "${POSD_HOME}/data"
    echo -e "${GREEN}Removed existing data directory${NC}"
fi

# Keep the keys but remove genesis
if [ -f "${CONFIG_DIR}/genesis.json" ]; then
    rm -f "${CONFIG_DIR}/genesis.json"
    echo -e "${GREEN}Removed existing genesis.json${NC}"
fi

# =============================================================================
# Step 2: Initialize the chain
# =============================================================================
echo -e "${YELLOW}[2/7] Initializing chain...${NC}"

if [ "$VALIDATOR_TYPE" = "validator1" ]; then
    MONIKER="omniphi-node"
else
    MONIKER="omniphi-node-2"
fi

posd init "$MONIKER" --chain-id "$CHAIN_ID" --default-denom "$BOND_DENOM" --overwrite

echo -e "${GREEN}Chain initialized with moniker: ${MONIKER}${NC}"

# =============================================================================
# Step 3: Configure genesis parameters
# =============================================================================
echo -e "${YELLOW}[3/7] Configuring genesis parameters...${NC}"

GENESIS="${CONFIG_DIR}/genesis.json"

# Update consensus params (block gas limit)
cat "$GENESIS" | jq ".consensus.params.block.max_gas = \"${BLOCK_MAX_GAS}\"" > tmp.json && mv tmp.json "$GENESIS"

# Update staking params
cat "$GENESIS" | jq ".app_state.staking.params.unbonding_time = \"1209600s\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.staking.params.max_validators = 125" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.staking.params.bond_denom = \"${BOND_DENOM}\"" > tmp.json && mv tmp.json "$GENESIS"

# Update governance params for testnet (fast voting)
cat "$GENESIS" | jq ".app_state.gov.params.min_deposit[0].denom = \"${BOND_DENOM}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.gov.params.min_deposit[0].amount = \"${MIN_DEPOSIT}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.gov.params.voting_period = \"${VOTING_PERIOD}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.gov.params.expedited_voting_period = \"${EXPEDITED_VOTING}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.gov.params.expedited_min_deposit[0].denom = \"${BOND_DENOM}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.gov.params.expedited_min_deposit[0].amount = \"50000000000\"" > tmp.json && mv tmp.json "$GENESIS"

# Update crisis constant fee
cat "$GENESIS" | jq ".app_state.crisis.constant_fee.denom = \"${BOND_DENOM}\"" > tmp.json && mv tmp.json "$GENESIS"
cat "$GENESIS" | jq ".app_state.crisis.constant_fee.amount = \"1000000000000\"" > tmp.json && mv tmp.json "$GENESIS"

# Update mint params (if exists)
if cat "$GENESIS" | jq -e '.app_state.mint' > /dev/null 2>&1; then
    cat "$GENESIS" | jq ".app_state.mint.params.mint_denom = \"${BOND_DENOM}\"" > tmp.json && mv tmp.json "$GENESIS"
fi

# Add denom metadata
cat "$GENESIS" | jq ".app_state.bank.denom_metadata = [{
    \"description\": \"The native staking token of Omniphi\",
    \"denom_units\": [
        {\"denom\": \"${BOND_DENOM}\", \"exponent\": 0, \"aliases\": [\"microomni\", \"uomni\"]},
        {\"denom\": \"mOMNI\", \"exponent\": 3, \"aliases\": [\"milliomni\"]},
        {\"denom\": \"${DISPLAY_DENOM}\", \"exponent\": 6, \"aliases\": []}
    ],
    \"base\": \"${BOND_DENOM}\",
    \"display\": \"${DISPLAY_DENOM}\",
    \"name\": \"Omniphi\",
    \"symbol\": \"${DISPLAY_DENOM}\"
}]" > tmp.json && mv tmp.json "$GENESIS"

echo -e "${GREEN}Genesis parameters configured${NC}"

# =============================================================================
# Step 4: Add genesis accounts (Validator 1 only)
# =============================================================================
if [ "$VALIDATOR_TYPE" = "validator1" ]; then
    echo -e "${YELLOW}[4/7] Adding genesis accounts...${NC}"

    # Get validator1 address
    VALIDATOR1_ADDR=$(posd keys show validator --keyring-backend test -a 2>/dev/null)
    if [ -z "$VALIDATOR1_ADDR" ]; then
        echo -e "${RED}ERROR: validator key not found. Create it first:${NC}"
        echo "posd keys add validator --keyring-backend test"
        exit 1
    fi

    echo -e "${BLUE}Validator 1 address: ${VALIDATOR1_ADDR}${NC}"
    echo -e "${BLUE}Validator 1 allocation: ${VALIDATOR1_AMOUNT} ${BOND_DENOM} (1B OMNI)${NC}"

    # Add genesis account
    posd genesis add-genesis-account "$VALIDATOR1_ADDR" "${VALIDATOR1_AMOUNT}${BOND_DENOM}"

    echo -e "${GREEN}Genesis account added${NC}"
else
    echo -e "${YELLOW}[4/7] Skipping genesis accounts (validator2 - will receive from validator1)...${NC}"
fi

# =============================================================================
# Step 5: Create gentx (Validator 1 only)
# =============================================================================
if [ "$VALIDATOR_TYPE" = "validator1" ]; then
    echo -e "${YELLOW}[5/7] Creating genesis transaction...${NC}"

    # Create validator.json for gentx
    PUBKEY=$(posd comet show-validator)

    cat > /tmp/validator.json << EOF
{
    "pubkey": ${PUBKEY},
    "amount": "${VALIDATOR1_STAKE}${BOND_DENOM}",
    "moniker": "${MONIKER}",
    "identity": "",
    "website": "https://omniphi.io",
    "security": "",
    "details": "Omniphi Testnet Validator Node 1",
    "commission-rate": "0.10",
    "commission-max-rate": "0.20",
    "commission-max-change-rate": "0.01",
    "min-self-delegation": "1"
}
EOF

    posd genesis gentx validator "${VALIDATOR1_STAKE}${BOND_DENOM}" \
        --chain-id "$CHAIN_ID" \
        --moniker "$MONIKER" \
        --commission-rate "0.10" \
        --commission-max-rate "0.20" \
        --commission-max-change-rate "0.01" \
        --min-self-delegation "1" \
        --keyring-backend test

    echo -e "${GREEN}Genesis transaction created${NC}"
else
    echo -e "${YELLOW}[5/7] Skipping gentx (validator2 - will join after launch)...${NC}"
fi

# =============================================================================
# Step 6: Collect gentxs and finalize genesis (Validator 1 only)
# =============================================================================
if [ "$VALIDATOR_TYPE" = "validator1" ]; then
    echo -e "${YELLOW}[6/7] Collecting genesis transactions...${NC}"

    posd genesis collect-gentxs

    # Validate genesis
    posd genesis validate-genesis

    echo -e "${GREEN}Genesis finalized and validated${NC}"
else
    echo -e "${YELLOW}[6/7] Skipping collect-gentxs (validator2 - will copy genesis from validator1)...${NC}"
fi

# =============================================================================
# Step 7: Configure CometBFT for 4-second blocks
# =============================================================================
echo -e "${YELLOW}[7/7] Configuring CometBFT for 4-second blocks...${NC}"

CONFIG_TOML="${CONFIG_DIR}/config.toml"

# Update consensus timeouts for ~4 second blocks
sed -i 's/^timeout_propose = .*/timeout_propose = "1500ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_propose_delta = .*/timeout_propose_delta = "200ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_prevote = .*/timeout_prevote = "500ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_prevote_delta = .*/timeout_prevote_delta = "200ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_precommit = .*/timeout_precommit = "500ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_precommit_delta = .*/timeout_precommit_delta = "200ms"/' "$CONFIG_TOML"
sed -i 's/^timeout_commit = .*/timeout_commit = "1500ms"/' "$CONFIG_TOML"

# Enable prometheus
sed -i 's/^prometheus = false/prometheus = true/' "$CONFIG_TOML"

echo -e "${GREEN}CometBFT configured for 4-second blocks${NC}"

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Initialization Complete!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo -e "${BLUE}Chain ID:${NC} ${CHAIN_ID}"
echo -e "${BLUE}Moniker:${NC} ${MONIKER}"
echo -e "${BLUE}Token:${NC} ${DISPLAY_DENOM} (base: ${BOND_DENOM}, 6 decimals)"
echo ""
echo -e "${BLUE}Total Supply:${NC} 1,500,000,000 OMNI (1.5 Billion)"
echo -e "${BLUE}Block Gas Limit:${NC} 60,000,000"
echo -e "${BLUE}Voting Period:${NC} 5 minutes (testnet)"
echo ""

if [ "$VALIDATOR_TYPE" = "validator1" ]; then
    echo -e "${YELLOW}Next steps for Validator 1:${NC}"
    echo "1. Start the node: posd start"
    echo "2. Copy genesis.json to Validator 2"
    echo "3. Send tokens to Validator 2 for staking"
    echo ""
    echo -e "${BLUE}Genesis file:${NC} ${CONFIG_DIR}/genesis.json"
else
    echo -e "${YELLOW}Next steps for Validator 2:${NC}"
    echo "1. Copy genesis.json from Validator 1 to ${CONFIG_DIR}/"
    echo "2. Add persistent_peers in config.toml"
    echo "3. Start the node: posd start"
    echo "4. Wait for tokens from Validator 1"
    echo "5. Create validator with staking tx"
fi
