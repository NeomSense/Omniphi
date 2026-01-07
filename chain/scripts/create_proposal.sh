#!/bin/bash
# =============================================================================
# Omniphi Governance Proposal Generator
# =============================================================================
# This script generates properly formatted governance proposals with all
# required parameters pre-filled from the current chain state.
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
CHAIN_ID="${CHAIN_ID:-omniphi-testnet-2}"
OUTPUT_DIR="${OUTPUT_DIR:-/tmp}"
DEFAULT_DEPOSIT="10000000omniphi"

usage() {
    echo -e "${BLUE}Omniphi Governance Proposal Generator${NC}"
    echo ""
    echo "Usage: $0 <proposal-type> [options]"
    echo ""
    echo "Proposal Types:"
    echo "  consensus     - Update consensus parameters (block gas, evidence, etc.)"
    echo "  feemarket     - Update feemarket parameters (min gas price, max tx gas, etc.)"
    echo "  staking       - Update staking parameters (unbonding time, max validators, etc.)"
    echo "  poc           - Update PoC parameters"
    echo "  tokenomics    - Update tokenomics parameters"
    echo ""
    echo "Options:"
    echo "  --title       - Proposal title"
    echo "  --summary     - Proposal summary"
    echo "  --deposit     - Deposit amount (default: $DEFAULT_DEPOSIT)"
    echo "  --output      - Output file path (default: /tmp/<type>_proposal.json)"
    echo ""
    echo "Examples:"
    echo "  $0 consensus --title \"Set Max Block Gas\" --summary \"Set max block gas to 60M\""
    echo "  $0 feemarket --title \"Update Min Gas Price\" --summary \"Increase min gas price\""
    echo ""
}

get_gov_authority() {
    posd query auth module-account gov 2>&1 | grep "address:" | awk '{print $2}'
}

generate_consensus_proposal() {
    local title="$1"
    local summary="$2"
    local deposit="$3"
    local output="$4"

    echo -e "${CYAN}Fetching current consensus parameters...${NC}"

    # Get current params
    local current_params=$(posd query consensus params --output json 2>/dev/null)

    local max_bytes=$(echo "$current_params" | jq -r '.params.block.max_bytes')
    local max_gas=$(echo "$current_params" | jq -r '.params.block.max_gas')
    local max_age_num_blocks=$(echo "$current_params" | jq -r '.params.evidence.max_age_num_blocks')
    local max_age_duration=$(echo "$current_params" | jq -r '.params.evidence.max_age_duration')
    local evidence_max_bytes=$(echo "$current_params" | jq -r '.params.evidence.max_bytes')
    local pub_key_types=$(echo "$current_params" | jq -c '.params.validator.pub_key_types')

    # Convert duration format (48h0m0s -> 172800s)
    local duration_seconds=$(echo "$max_age_duration" | sed 's/h/*3600+/g; s/m/*60+/g; s/s//g; s/+$//' | bc)

    local authority=$(get_gov_authority)

    echo -e "${YELLOW}Current values:${NC}"
    echo "  max_bytes: $max_bytes"
    echo "  max_gas: $max_gas"
    echo "  max_age_num_blocks: $max_age_num_blocks"
    echo "  max_age_duration: ${duration_seconds}s"
    echo "  evidence_max_bytes: $evidence_max_bytes"
    echo "  pub_key_types: $pub_key_types"
    echo ""
    echo -e "${GREEN}Edit the values you want to change in the generated file.${NC}"

    cat > "$output" << EOF
{
  "messages": [
    {
      "@type": "/cosmos.consensus.v1.MsgUpdateParams",
      "authority": "$authority",
      "block": {
        "max_bytes": "$max_bytes",
        "max_gas": "$max_gas"
      },
      "evidence": {
        "max_age_num_blocks": "$max_age_num_blocks",
        "max_age_duration": "${duration_seconds}s",
        "max_bytes": "$evidence_max_bytes"
      },
      "validator": {
        "pub_key_types": $pub_key_types
      },
      "abci": {
        "vote_extensions_enable_height": "0"
      }
    }
  ],
  "metadata": "$summary",
  "deposit": "$deposit",
  "title": "$title",
  "summary": "$summary"
}
EOF

    echo -e "${GREEN}Proposal generated: $output${NC}"
}

generate_feemarket_proposal() {
    local title="$1"
    local summary="$2"
    local deposit="$3"
    local output="$4"

    echo -e "${CYAN}Fetching current feemarket parameters...${NC}"

    # Get current params
    local current_params=$(posd query feemarket params --output json 2>/dev/null)
    local params=$(echo "$current_params" | jq '.params')

    # Get feemarket module authority
    local authority=$(posd query auth module-account feemarket 2>&1 | grep "address:" | awk '{print $2}')
    if [ -z "$authority" ]; then
        authority=$(get_gov_authority)
    fi

    echo -e "${YELLOW}Current values:${NC}"
    echo "$params" | jq '.'
    echo ""
    echo -e "${GREEN}Edit the values you want to change in the generated file.${NC}"

    cat > "$output" << EOF
{
  "messages": [
    {
      "@type": "/omniphi.feemarket.v1.MsgUpdateParams",
      "authority": "$authority",
      "params": $params
    }
  ],
  "metadata": "$summary",
  "deposit": "$deposit",
  "title": "$title",
  "summary": "$summary"
}
EOF

    echo -e "${GREEN}Proposal generated: $output${NC}"
}

generate_staking_proposal() {
    local title="$1"
    local summary="$2"
    local deposit="$3"
    local output="$4"

    echo -e "${CYAN}Fetching current staking parameters...${NC}"

    # Get current params
    local current_params=$(posd query staking params --output json 2>/dev/null)
    local params=$(echo "$current_params" | jq '.params')

    local authority=$(get_gov_authority)

    echo -e "${YELLOW}Current values:${NC}"
    echo "$params" | jq '.'
    echo ""
    echo -e "${GREEN}Edit the values you want to change in the generated file.${NC}"

    cat > "$output" << EOF
{
  "messages": [
    {
      "@type": "/cosmos.staking.v1beta1.MsgUpdateParams",
      "authority": "$authority",
      "params": $params
    }
  ],
  "metadata": "$summary",
  "deposit": "$deposit",
  "title": "$title",
  "summary": "$summary"
}
EOF

    echo -e "${GREEN}Proposal generated: $output${NC}"
}

generate_poc_proposal() {
    local title="$1"
    local summary="$2"
    local deposit="$3"
    local output="$4"

    echo -e "${CYAN}Fetching current PoC parameters...${NC}"

    # Get current params
    local current_params=$(posd query poc params --output json 2>/dev/null)
    local params=$(echo "$current_params" | jq '.params')

    local authority=$(get_gov_authority)

    echo -e "${YELLOW}Current values:${NC}"
    echo "$params" | jq '.'
    echo ""
    echo -e "${GREEN}Edit the values you want to change in the generated file.${NC}"

    cat > "$output" << EOF
{
  "messages": [
    {
      "@type": "/omniphi.poc.v1.MsgUpdateParams",
      "authority": "$authority",
      "params": $params
    }
  ],
  "metadata": "$summary",
  "deposit": "$deposit",
  "title": "$title",
  "summary": "$summary"
}
EOF

    echo -e "${GREEN}Proposal generated: $output${NC}"
}

generate_tokenomics_proposal() {
    local title="$1"
    local summary="$2"
    local deposit="$3"
    local output="$4"

    echo -e "${CYAN}Fetching current tokenomics parameters...${NC}"

    # Get current params - tokenomics returns flat structure
    local params=$(posd query tokenomics params --output json 2>/dev/null)

    local authority=$(get_gov_authority)

    echo -e "${YELLOW}Current values:${NC}"
    echo "$params" | jq '.'
    echo ""
    echo -e "${GREEN}Edit the values you want to change in the generated file.${NC}"

    cat > "$output" << EOF
{
  "messages": [
    {
      "@type": "/omniphi.tokenomics.v1.MsgUpdateParams",
      "authority": "$authority",
      "params": $params
    }
  ],
  "metadata": "$summary",
  "deposit": "$deposit",
  "title": "$title",
  "summary": "$summary"
}
EOF

    echo -e "${GREEN}Proposal generated: $output${NC}"
}

# Parse arguments
PROPOSAL_TYPE=""
TITLE="Parameter Update Proposal"
SUMMARY="Update chain parameters"
DEPOSIT="$DEFAULT_DEPOSIT"
OUTPUT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        consensus|feemarket|staking|poc|tokenomics)
            PROPOSAL_TYPE="$1"
            shift
            ;;
        --title)
            TITLE="$2"
            shift 2
            ;;
        --summary)
            SUMMARY="$2"
            shift 2
            ;;
        --deposit)
            DEPOSIT="$2"
            shift 2
            ;;
        --output)
            OUTPUT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

if [ -z "$PROPOSAL_TYPE" ]; then
    usage
    exit 1
fi

# Set default output if not specified
if [ -z "$OUTPUT" ]; then
    OUTPUT="${OUTPUT_DIR}/${PROPOSAL_TYPE}_proposal.json"
fi

# Generate proposal based on type
case $PROPOSAL_TYPE in
    consensus)
        generate_consensus_proposal "$TITLE" "$SUMMARY" "$DEPOSIT" "$OUTPUT"
        ;;
    feemarket)
        generate_feemarket_proposal "$TITLE" "$SUMMARY" "$DEPOSIT" "$OUTPUT"
        ;;
    staking)
        generate_staking_proposal "$TITLE" "$SUMMARY" "$DEPOSIT" "$OUTPUT"
        ;;
    poc)
        generate_poc_proposal "$TITLE" "$SUMMARY" "$DEPOSIT" "$OUTPUT"
        ;;
    tokenomics)
        generate_tokenomics_proposal "$TITLE" "$SUMMARY" "$DEPOSIT" "$OUTPUT"
        ;;
esac

echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "1. Edit the proposal file to change desired values:"
echo "   nano $OUTPUT"
echo ""
echo "2. Submit the proposal:"
echo "   posd tx gov submit-proposal $OUTPUT \\"
echo "     --from validator \\"
echo "     --chain-id $CHAIN_ID \\"
echo "     --keyring-backend test \\"
echo "     --fees 100000omniphi \\"
echo "     --gas 500000 \\"
echo "     -y"
echo ""
echo "3. Vote on the proposal:"
echo "   posd tx gov vote <proposal-id> yes \\"
echo "     --from validator \\"
echo "     --chain-id $CHAIN_ID \\"
echo "     --keyring-backend test \\"
echo "     --fees 50000omniphi \\"
echo "     -y"
