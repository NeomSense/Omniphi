#!/bin/bash
# Production Chain Initialization Script
# This script initializes the POS blockchain with production-ready parameters

set -e

CHAIN_ID="pos-1"
MONIKER="validator1"
KEYRING="test"
DENOM="stake"
HOME_DIR="$HOME/.pos"

echo "========================================="
echo "POS Blockchain Production Initialization"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() {
    echo -e "${YELLOW}➜${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

# Clean previous data
info "Cleaning previous chain data..."
rm -rf $HOME_DIR
success "Clean complete"

# Initialize chain
info "Initializing chain with ID: $CHAIN_ID"
posd init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR
success "Chain initialized"

# Create keys
info "Creating validator keys..."
posd keys add alice --keyring-backend $KEYRING --home $HOME_DIR 2>&1 | grep -E "address|mnemonic" || true
posd keys add bob --keyring-backend $KEYRING --home $HOME_DIR 2>&1 | grep -E "address|mnemonic" || true
success "Keys created"

# Get addresses
ALICE_ADDR=$(posd keys show alice -a --keyring-backend $KEYRING --home $HOME_DIR)
BOB_ADDR=$(posd keys show bob -a --keyring-backend $KEYRING --home $HOME_DIR)

info "Alice address: $ALICE_ADDR"
info "Bob address: $BOB_ADDR"

# Add genesis accounts
info "Adding genesis accounts..."
posd genesis add-genesis-account $ALICE_ADDR 200000000$DENOM --home $HOME_DIR
posd genesis add-genesis-account $BOB_ADDR 100000000$DENOM --home $HOME_DIR
success "Genesis accounts added"

# Generate genesis transaction for validator
info "Creating genesis validator..."
posd genesis gentx alice 100000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --keyring-backend $KEYRING \
  --home $HOME_DIR
success "Genesis transaction created"

# Collect genesis transactions
info "Collecting genesis transactions..."
posd genesis collect-gentxs --home $HOME_DIR
success "Genesis transactions collected"

# Now update genesis.json with production parameters
info "Updating genesis with production parameters..."

GENESIS_FILE="$HOME_DIR/config/genesis.json"

# Backup original
cp $GENESIS_FILE ${GENESIS_FILE}.backup

# Update staking parameters
info "Setting staking parameters..."
cat $GENESIS_FILE | jq '.app_state.staking.params.unbonding_time = "1814400s"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.staking.params.max_validators = 125' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.staking.params.min_commission_rate = "0.050000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
success "Staking parameters set"

# Update slashing parameters
info "Setting slashing parameters..."
cat $GENESIS_FILE | jq '.app_state.slashing.params.signed_blocks_window = "30000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.slashing.params.min_signed_per_window = "0.050000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.slashing.params.downtime_jail_duration = "600s"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.slashing.params.slash_fraction_double_sign = "0.050000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.slashing.params.slash_fraction_downtime = "0.000100000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
success "Slashing parameters set"

# Update governance parameters
info "Setting governance parameters..."
cat $GENESIS_FILE | jq '.app_state.gov.params.min_deposit[0].amount = "10000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.gov.params.voting_period = "432000s"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.gov.params.quorum = "0.334000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.gov.params.threshold = "0.500000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.gov.params.veto_threshold = "0.334000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.gov.params.burn_vote_veto = true' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
success "Governance parameters set"

# Update mint parameters
info "Setting mint parameters..."
cat $GENESIS_FILE | jq '.app_state.mint.params.inflation_rate_change = "0.130000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.mint.params.inflation_max = "0.200000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.mint.params.inflation_min = "0.070000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.mint.params.goal_bonded = "0.670000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.mint.params.blocks_per_year = "5256000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
success "Mint parameters set"

# Update distribution parameters
info "Setting distribution parameters..."
cat $GENESIS_FILE | jq '.app_state.distribution.params.community_tax = "0.020000000000000000"' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
cat $GENESIS_FILE | jq '.app_state.distribution.params.withdraw_addr_enabled = true' > ${GENESIS_FILE}.tmp && mv ${GENESIS_FILE}.tmp $GENESIS_FILE
success "Distribution parameters set"

# Validate genesis
info "Validating genesis..."
posd genesis validate-genesis --home $HOME_DIR
success "Genesis validation passed!"

echo ""
echo "========================================="
echo "✓ Production initialization complete!"
echo "========================================="
echo ""
echo "Chain ID: $CHAIN_ID"
echo "Home: $HOME_DIR"
echo "Validator: $MONIKER"
echo ""
echo "Production Parameters Set:"
echo "  • Staking: min_commission = 5%, unbonding = 21 days, max_validators = 125"
echo "  • Slashing: window = 30000, min_signed = 5%, double_sign_slash = 5%"
echo "  • Governance: deposit = 10M, voting = 5 days, quorum = 33.4%"
echo "  • Mint: inflation 7-20%, goal_bonded = 67%"
echo "  • Distribution: community_tax = 2%"
echo ""
echo "To start the chain:"
echo "  posd start --home $HOME_DIR"
echo ""
echo "Or in another terminal:"
echo "  posd start --home $HOME_DIR > chain.log 2>&1 &"
echo ""
echo "To test:"
echo "  export HOME_DIR=$HOME_DIR"
echo "  posd query staking params --home \$HOME_DIR"
echo ""
