#!/bin/bash
# Omniphi Testnet Launch Script
# Launches a multi-validator testnet from configuration files.
set -e

CONFIG_DIR="${CONFIG_DIR:-configs}"
DATA_DIR="${DATA_DIR:-/tmp/omniphi-testnet}"
BINARY="${BINARY:-poseq-node}"
NUM_VALIDATORS="${NUM_VALIDATORS:-4}"
BASE_PORT="${BASE_PORT:-26600}"
METRICS_BASE_PORT="${METRICS_BASE_PORT:-9090}"

echo "=== Omniphi Testnet Launch ==="
echo "Validators: $NUM_VALIDATORS"
echo "Data dir:   $DATA_DIR"
echo ""

# Create data directories
for i in $(seq 1 $NUM_VALIDATORS); do
    mkdir -p "$DATA_DIR/validator-$i"
done

# Generate validator configs
for i in $(seq 1 $NUM_VALIDATORS); do
    PORT=$((BASE_PORT + i - 1))
    METRICS_PORT=$((METRICS_BASE_PORT + i - 1))
    NODE_ID=$(printf '%064d' $i)
    KEY_SEED=$(printf '%064d' $((i + 1000)))

    # Build peer list (all other validators)
    PEERS=""
    for j in $(seq 1 $NUM_VALIDATORS); do
        if [ $j -ne $i ]; then
            PEER_PORT=$((BASE_PORT + j - 1))
            PEERS="$PEERS\"127.0.0.1:$PEER_PORT\","
        fi
    done
    PEERS="[${PEERS%,}]"

    cat > "$DATA_DIR/validator-$i/config.toml" <<EOF
[node]
node_id = "$NODE_ID"
moniker = "validator-$i"
listen_addr = "0.0.0.0:$PORT"

[keys]
key_seed = "$KEY_SEED"

[peers]
addrs = $PEERS

[consensus]
quorum = $((NUM_VALIDATORS * 2 / 3 + 1))
slot_duration_ms = 6000

[metrics]
enabled = true
listen_addr = "0.0.0.0:$METRICS_PORT"

[storage]
data_dir = "$DATA_DIR/validator-$i/data"
EOF

    echo "Generated config for validator-$i (port $PORT, metrics $METRICS_PORT)"
done

echo ""
echo "To start validators:"
for i in $(seq 1 $NUM_VALIDATORS); do
    echo "  $BINARY --config $DATA_DIR/validator-$i/config.toml &"
done

echo ""
echo "To monitor:"
for i in $(seq 1 $NUM_VALIDATORS); do
    METRICS_PORT=$((METRICS_BASE_PORT + i - 1))
    echo "  curl http://localhost:$METRICS_PORT/metrics"
done
