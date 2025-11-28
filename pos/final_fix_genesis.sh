#!/bin/bash
GENESIS_FILE="/home/funmachine/.pos/config/genesis.json"
DELEGATOR_ADDR="omni1x3w2x9ecmmfep2llu3sekczf2ve0e5uhexeqdx"
echo "Fixing genesis - restoring delegator balance..."
# Restore delegator to full 1000000000000000 (we shouldn't have subtracted)
jq --arg addr "$DELEGATOR_ADDR" \
   '.app_state.bank.balances = [
     {
       "address": $addr,
       "coins": [{
         "denom": "uomni",
         "amount": "1000000000000000"
       }]
     }
   ]' $GENESIS_FILE > /tmp/gen_restore.json && mv /tmp/gen_restore.json $GENESIS_FILE
echo "✅ Genesis fixed!"
# Validate
./posd genesis validate-genesis --home /home/funmachine/.pos
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Genesis is valid! Starting chain..."
    nohup ./posd start --minimum-gas-prices 0.001uomni --home /home/funmachine/.pos > chain.log 2>&1 &
    sleep 20
    
    echo ""
    echo "Checking chain status..."
    ./posd status --home /home/funmachine/.pos 2>&1 | head -20
    
    if [ $? -eq 0 ]; then
        echo ""
        echo "🎉 CHAIN IS RUNNING!"
        tail -30 chain.log
    else
        echo ""
        echo "Chain status failed. Checking logs..."
        tail -50 chain.log
    fi
else
    echo "❌ Genesis validation failed"
fi
