#!/bin/bash
set -e

echo "=== Complete Ubuntu Setup ==="

# Use a fresh temp directory to avoid file locks
CHAIN_HOME=~/.pos-new
cd ~/omniphi/pos

# Clean
rm -rf $CHAIN_HOME
rm -f posd.log posd.pid

echo "1. Initializing chain to $CHAIN_HOME..."
./posd init my-node --chain-id omniphi-1 --default-denom omniphi --home $CHAIN_HOME

echo "2. Setting gas price..."
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001omniphi"/' $CHAIN_HOME/config/app.toml

echo "3. Creating accounts..."
./posd keys add alice --keyring-backend test --home $CHAIN_HOME
./posd keys add val1 --keyring-backend test --home $CHAIN_HOME

ALICE=$(./posd keys show alice -a --keyring-backend test --home $CHAIN_HOME)
VAL1=$(./posd keys show val1 -a --keyring-backend test --home $CHAIN_HOME)

echo ""
echo "✓ Accounts:"
echo "  Alice: $ALICE"
echo "  Val1:  $VAL1"

echo ""
echo "4. Adding to genesis..."
./posd genesis add-genesis-account $ALICE 10000000000omniphi --home $CHAIN_HOME
./posd genesis add-genesis-account $VAL1 20000000000omniphi --home $CHAIN_HOME

echo "5. Creating validator..."
./posd genesis gentx val1 10000000000omniphi \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --home $CHAIN_HOME

echo "6. Collecting gentx..."
./posd genesis collect-gentxs --home $CHAIN_HOME

echo "7. Starting chain..."
./posd start --home $CHAIN_HOME --minimum-gas-prices="0.001omniphi" > posd.log 2>&1 &
echo $! > posd.pid
sleep 15

echo ""
echo "8. Verifying..."
if ./posd status --home $CHAIN_HOME 2>&1 | grep -q "latest_block_height"; then
    echo "✓ Chain is running!"
    ./posd status --home $CHAIN_HOME 2>&1 | grep -o '"latest_block_height":"[0-9]*"'
    echo ""
    ./posd query bank balance $ALICE omniphi --home $CHAIN_HOME
else
    echo "⚠ Chain not running. Check: tail -50 posd.log"
fi

echo ""
echo "=== SUCCESS ==="
echo ""
echo "Chain directory: $CHAIN_HOME"
echo "To use commands, add: --home $CHAIN_HOME"
echo ""
echo "Example:"
echo "  ./posd status --home $CHAIN_HOME 2>&1 | grep latest_block_height"
echo ""
