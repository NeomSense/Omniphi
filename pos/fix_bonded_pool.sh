#!/bin/bash
GENESIS_FILE="/home/funmachine/.pos/config/genesis.json"
DELEGATION_AMOUNT="100000000000000"
DELEGATOR_ADDR="omni1x3w2x9ecmmfep2llu3sekczf2ve0e5uhexeqdx"
BONDED_POOL_ADDR="omni1fl48vsnmsdzcv85q5d2q4z5ajdha8yu3h60spj"  # Standard Cosmos SDK bonded pool address
echo "Fixing bonded pool..."
# 1. Add bonded pool to bank balances
jq --arg addr "$BONDED_POOL_ADDR" \
   --arg amount "$DELEGATION_AMOUNT" \
   '.app_state.bank.balances += [{
     "address": $addr,
     "coins": [{
       "denom": "uomni",
       "amount": $amount
     }]
   }]' $GENESIS_FILE > /tmp/gen1.json && mv /tmp/gen1.json $GENESIS_FILE
# 2. Subtract delegation amount from delegator balance
jq --arg daddr "$DELEGATOR_ADDR" \
   --arg amount "$DELEGATION_AMOUNT" \
   '(.app_state.bank.balances[] | select(.address == $daddr) | .coins[0].amount) |= 
    (tonumber - ($amount | tonumber) | tostring)' \
   $GENESIS_FILE > /tmp/gen2.json && mv /tmp/gen2.json $GENESIS_FILE
# 3. Update supply to include bonded pool tokens (total stays same)
TOTAL_SUPPLY=$(jq -r '.app_state.bank.supply[0].amount' $GENESIS_FILE)
echo "Total supply: $TOTAL_SUPPLY"
echo "✅ Bonded pool configured!"
# Validate
./posd genesis validate-genesis --home /home/funmachine/.pos
echo ""
echo "Delegator balance:"
jq --arg addr "$DELEGATOR_ADDR" '.app_state.bank.balances[] | select(.address == $addr)' $GENESIS_FILE
echo ""
echo "Bonded pool balance:"
jq --arg addr "$BONDED_POOL_ADDR" '.app_state.bank.balances[] | select(.address == $addr)' $GENESIS_FILE
