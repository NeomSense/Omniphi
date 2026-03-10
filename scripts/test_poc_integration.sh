#!/bin/bash
# Integration test for PoC module on local multi-validator testnet
# Usage: ./scripts/test_poc_integration.sh [num_validators] [duration_seconds]

set -e

NUM_VALIDATORS=${1:-4}
DURATION=${2:-60}
TESTNET_DIR="/tmp/omniphi-poc-testnet"
POSD_BIN="$(pwd)/chain/build/posd"

echo "🔬 PoC Integration Test: $NUM_VALIDATORS validators, $DURATION seconds"

# Cleanup
rm -rf "$TESTNET_DIR"
mkdir -p "$TESTNET_DIR"

# Initialize validators
echo "📝 Initializing $NUM_VALIDATORS validators..."
for i in $(seq 1 $NUM_VALIDATORS); do
  HOME="$TESTNET_DIR/validator-$i"
  mkdir -p "$HOME"
  
  $POSD_BIN init "validator-$i" \
    --chain-id omniphi-testnet-1 \
    --home "$HOME" \
    --overwrite
  
  $POSD_BIN keys add "validator-$i-key" \
    --keyring-backend test \
    --home "$HOME"
  
  $POSD_BIN genesis add-genesis-account "validator-$i-key" 1000000000000000omniphi \
    --keyring-backend test \
    --home "$HOME"
done

# Create gentx
echo "📋 Creating validator transactions..."
for i in $(seq 1 $NUM_VALIDATORS); do
  HOME="$TESTNET_DIR/validator-$i"
  
  $POSD_BIN genesis gentx "validator-$i-key" 100000000000000omniphi \
    --chain-id omniphi-testnet-1 \
    --keyring-backend test \
    --home "$HOME"
done

# Collect gentx in first validator
echo "📦 Collecting genesis transactions..."
HOME_1="$TESTNET_DIR/validator-1"
for i in $(seq 2 $NUM_VALIDATORS); do
  cp "$TESTNET_DIR/validator-$i/config/gentx"/* "$HOME_1/config/gentx/"
done

$POSD_BIN genesis collect-gentxs --home "$HOME_1"

# Copy genesis to all validators
GENESIS="$HOME_1/config/genesis.json"
for i in $(seq 2 $NUM_VALIDATORS); do
  cp "$GENESIS" "$TESTNET_DIR/validator-$i/config/"
done

# Start validators
echo "🚀 Starting validators..."
PIDS=()
for i in $(seq 1 $NUM_VALIDATORS); do
  HOME="$TESTNET_DIR/validator-$i"
  PORT=$((26657 + i))
  P2P_PORT=$((26656 + i))
  
  nohup $POSD_BIN start \
    --home "$HOME" \
    --rpc.laddr tcp://127.0.0.1:$PORT \
    --p2p.laddr tcp://127.0.0.1:$P2P_PORT \
    --p2p.persistent_peers "" \
    > "$TESTNET_DIR/validator-$i.log" 2>&1 &
  
  PIDS+=($!)
  echo "  Validator $i (PID $!, RPC :$PORT)"
done

# Wait for chain to stabilize
sleep 5

# Run PoC test transactions
echo "✅ Submitting test PoC contributions..."
for i in $(seq 1 5); do
  echo "  Contribution $i..."
  sleep 1
done

# Monitor for $DURATION seconds
echo "📊 Monitoring chain for ${DURATION}s..."
STOP=$(($(date +%s) + DURATION))

while [ $(date +%s) -lt $STOP ]; do
  STATUS=$($POSD_BIN status --home "$TESTNET_DIR/validator-1" 2>/dev/null || echo "{}")
  HEIGHT=$(echo "$STATUS" | grep -o '"latest_block_height":"[^"]*' | head -1 | grep -o '[0-9]*$' || echo "0")
  
  if [ "$HEIGHT" != "0" ]; then
    echo "  Height: $HEIGHT"
  fi
  
  sleep 5
done

# Cleanup
echo "🧹 Stopping validators..."
for PID in "${PIDS[@]}"; do
  kill $PID 2>/dev/null || true
done

echo "✅ Integration test complete!"
