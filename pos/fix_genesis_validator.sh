#!/bin/bash
GENESIS_FILE="/home/funmachine/.pos/config/genesis.json"
VALIDATOR_ADDR="omni1x3w2x9ecmmfep2llu3sekczf2ve0e5uhexeqdx"
# Install jq if not present
if ! command -v jq &> /dev/null; then
    echo "Installing jq..."
    sudo apt-get update && sudo apt-get install -y jq
fi
# Add delegator_address to gentx
jq --arg addr "$VALIDATOR_ADDR" \
  '.app_state.genutil.gen_txs[0].body.messages[0].delegator_address = $addr' \
  $GENESIS_FILE > /tmp/genesis_tmp.json && mv /tmp/genesis_tmp.json $GENESIS_FILE
echo "✅ Fixed genesis - added delegator_address"
# Validate
./posd genesis validate-genesis --home /home/funmachine/.pos

echo "✅ Genesis validated - ready to start!"
