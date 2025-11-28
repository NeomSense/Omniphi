#!/bin/bash
# Final PoC Test Script - Complete setup and test
set -e

CHAIN_ID="omniphi-1"
DENOM="omniphi"
HOME_DIR="$HOME/.pos"

echo "========================================="
echo "Final PoC Test - Complete Setup"
echo "========================================="
echo ""

# 1. Stop and clean
killall posd 2>/dev/null || true
rm -rf $HOME_DIR posd.log posd.pid

# 2. Build
go build -o posd ./cmd/posd

# 3. Initialize
./posd init my-node --chain-id $CHAIN_ID --default-denom $DENOM --home $HOME_DIR > /dev/null

# 4. Set min gas price
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' $HOME_DIR/config/app.toml

# 5. Create accounts (or reuse existing)
./posd keys add alice --keyring-backend test --home $HOME_DIR 2>/dev/null || true
./posd keys add bob --keyring-backend test --home $HOME_DIR 2>/dev/null || true
./posd keys add val1 --keyring-backend test --home $HOME_DIR 2>/dev/null || true

ALICE=$(./posd keys show alice -a --keyring-backend test --home $HOME_DIR)
BOB=$(./posd keys show bob -a --keyring-backend test --home $HOME_DIR)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home $HOME_DIR)

echo "Accounts:"
echo "  Alice: $ALICE"
echo "  Bob:   $BOB"
echo "  Val1:  $VAL1"
echo ""

# 6. Add to genesis
./posd genesis add-genesis-account $ALICE 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $BOB 10000000000$DENOM --home $HOME_DIR
./posd genesis add-genesis-account $VAL1 20000000000$DENOM --home $HOME_DIR

# 7. Create validator
./posd genesis gentx val1 10000000000$DENOM \
    --chain-id $CHAIN_ID \
    --keyring-backend test \
    --commission-rate="0.10" \
    --commission-max-rate="0.20" \
    --commission-max-change-rate="0.01" \
    --home $HOME_DIR > /dev/null

# 8. Collect gentxs
./posd genesis collect-gentxs --home $HOME_DIR > /dev/null

# 9. Start chain
./posd start --home $HOME_DIR > posd.log 2>&1 &
echo $! > posd.pid
echo "Chain started (PID: $(cat posd.pid))"
sleep 15

# 10. Test PoC Module
echo ""
echo "========================================="
echo "Testing PoC Module"
echo "========================================="
echo ""

echo "1. Submitting contribution from Alice..."
./posd tx poc submit-contribution "code" "ipfs://Qm123/project.tar.gz" "0xABCDEF" \
    --from alice --keyring-backend test --chain-id $CHAIN_ID \
    --home $HOME_DIR --gas auto --fees 2000$DENOM --yes > /dev/null
sleep 6
echo "   ✓ Submitted"

echo "2. Querying contribution..."
./posd query poc contribution 1 --home $HOME_DIR
echo ""

echo "3. Validator endorses contribution..."
./posd tx poc endorse 1 true \
    --from val1 --keyring-backend test --chain-id $CHAIN_ID \
    --home $HOME_DIR --gas auto --fees 2000$DENOM --yes > /dev/null
sleep 6
echo "   ✓ Endorsed"

echo "4. Querying contribution after endorsement..."
./posd query poc contribution 1 --home $HOME_DIR
echo ""

echo "5. Checking Alice's PoC credits..."
./posd query poc credits $ALICE --home $HOME_DIR
echo ""

echo "6. Withdrawing PoC rewards..."
./posd tx poc withdraw-poc-rewards \
    --from alice --keyring-backend test --chain-id $CHAIN_ID \
    --home $HOME_DIR --gas auto --fees 2000$DENOM --yes > /dev/null
sleep 6
echo "   ✓ Withdrawn"

echo "7. Checking credits after withdrawal..."
./posd query poc credits $ALICE --home $HOME_DIR
echo ""

echo "========================================="
echo "✓ ALL POC TESTS COMPLETED!"
echo "========================================="
echo ""
echo "Chain is running. PID: $(cat posd.pid)"
echo "Stop: kill $(cat posd.pid)"
