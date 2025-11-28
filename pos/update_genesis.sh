#!/usr/bin/env bash
# Update genesis.json with production parameters using sed

GENESIS_FILE="$HOME/.pos/config/genesis.json"
echo "Updating genesis at: $GENESIS_FILE"

# Create backup
cp "$GENESIS_FILE" "$GENESIS_FILE.backup.$(date +%s)"

# Update staking parameters using unique JSON patterns
# max_validators: 100 -> 125 (in staking section)
sed -i 's/"max_validators": 100,/"max_validators": 125,/g' "$GENESIS_FILE"

# min_commission_rate: "0.000000000000000000" -> "0.050000000000000000" (in staking section)
sed -i 's/"min_commission_rate": "0.000000000000000000"/"min_commission_rate": "0.050000000000000000"/g' "$GENESIS_FILE"

# signed_blocks_window: "100" -> "30000" (in slashing section)
sed -i 's/"signed_blocks_window": "100"/"signed_blocks_window": "30000"/g' "$GENESIS_FILE"

# min_signed_per_window: "0.500000000000000000" -> "0.050000000000000000" (in slashing section)
sed -i 's/"min_signed_per_window": "0.500000000000000000"/"min_signed_per_window": "0.050000000000000000"/g' "$GENESIS_FILE"

# slash_fraction_downtime: "0.010000000000000000" -> "0.000100000000000000" (in slashing section)
sed -i 's/"slash_fraction_downtime": "0.010000000000000000"/"slash_fraction_downtime": "0.000100000000000000"/g' "$GENESIS_FILE"

# blocks_per_year: "6311520" -> "5256000" (in mint section)
sed -i 's/"blocks_per_year": "6311520"/"blocks_per_year": "5256000"/g' "$GENESIS_FILE"

# voting_period: "172800s" -> "432000s" (in gov section)
sed -i 's/"voting_period": "172800s"/"voting_period": "432000s"/g' "$GENESIS_FILE"

echo "Genesis parameters updated!"
echo ""
echo "Verifying updates:"
grep -A 5 '"staking"' "$GENESIS_FILE" | grep -E '(max_validators|min_commission_rate)'
grep -A 5 '"slashing"' "$GENESIS_FILE" | grep -E '(signed_blocks_window|min_signed_per_window|slash_fraction_downtime)'
grep -A 5 '"mint"' "$GENESIS_FILE" | grep 'blocks_per_year'
grep -A 5 '"gov"' "$GENESIS_FILE" | grep 'voting_period'
