#!/bin/bash
################################################################################
# Add Consensus Validators to Genesis
# After collect-gentxs, this script adds the CometBFT validators array
################################################################################

set -e

# Configuration
POSD_HOME=~/.pos
GENESIS_FILE="$POSD_HOME/config/genesis.json"

echo "Adding consensus validators to genesis..."

# Install jq if needed
if ! command -v jq &> /dev/null; then
    echo "Installing jq..."
    sudo apt-get update && sudo apt-get install -y jq
fi

# Extract validator from staking genesis
VALIDATOR_PUBKEY=$(cat $GENESIS_FILE | jq -r '.app_state.genutil.gen_txs[0].body.messages[0].pubkey.key')
VALIDATOR_POWER=$(cat $GENESIS_FILE | jq -r '.app_state.genutil.gen_txs[0].body.messages[0].value.amount | tonumber / 1000000')

if [ -z "$VALIDATOR_PUBKEY" ] || [ "$VALIDATOR_PUBKEY" == "null" ]; then
    echo "Error: No validator found in gen_txs"
    exit 1
fi

echo "Found validator:"
echo "  PubKey: $VALIDATOR_PUBKEY"
echo "  Power: $VALIDATOR_POWER"

# Add to root validators array
cat $GENESIS_FILE | jq --arg pubkey "$VALIDATOR_PUBKEY" --argjson power "$VALIDATOR_POWER" \
  '.validators = [{
    "address": "",
    "pub_key": {
      "@type": "/cosmos.crypto.ed25519.PubKey",
      "key": $pubkey
    },
    "power": ($power | tostring),
    "name": ""
  }]' > /tmp/genesis_with_validators.json

mv /tmp/genesis_with_validators.json $GENESIS_FILE

echo "✅ Added validator to root validators array"

# Validate
./posd genesis validate-genesis --home $POSD_HOME
echo "✅ Genesis validated"
