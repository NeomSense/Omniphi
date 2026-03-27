#!/bin/bash
# Omniphi Multi-Validator Testnet Launch Script
#
# Usage: ./launch.sh [num_validators] [chain_id]
# Example: ./launch.sh 4 omniphi-testnet-3
#
# This script:
# 1. Builds the posd binary
# 2. Initializes N validator nodes with unique home directories
# 3. Creates genesis with all validators
# 4. Configures persistent peers
# 5. Starts all nodes
# 6. Funds a faucet account

set -euo pipefail

NUM_VALIDATORS=${1:-4}
CHAIN_ID=${2:-"omniphi-testnet-3"}
BASE_DIR="$HOME/.omniphi-testnet"
DENOM="omniphi"
STAKE_DENOM="omniphi"
INITIAL_SUPPLY="1000000000000000" # 1B tokens (with 6 decimals)
VALIDATOR_STAKE="250000000000"     # 250K tokens per validator
FAUCET_AMOUNT="100000000000000"    # 100B for faucet
GOV_VOTING_PERIOD="300s"          # 5 minutes for testnet
BINARY="posd"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[TESTNET]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1" >&2; exit 1; }

# ─── Pre-checks ────────────────────────────────────────────
command -v $BINARY >/dev/null 2>&1 || err "$BINARY not found in PATH. Build with: cd chain && go install ./cmd/posd"
command -v jq >/dev/null 2>&1 || warn "jq not found — genesis modifications will use go script"

log "Launching $NUM_VALIDATORS-validator testnet: $CHAIN_ID"
log "Base directory: $BASE_DIR"

# ─── Clean previous testnet ────────────────────────────────
if [ -d "$BASE_DIR" ]; then
    warn "Removing previous testnet at $BASE_DIR"
    rm -rf "$BASE_DIR"
fi

# ─── Initialize each validator ─────────────────────────────
declare -a NODE_IDS
declare -a VALIDATOR_ADDRS
declare -a VALIDATOR_PUBKEYS

for i in $(seq 1 $NUM_VALIDATORS); do
    NODE_HOME="$BASE_DIR/node$i"
    MONIKER="validator-$i"

    log "Initializing node $i ($MONIKER)..."
    $BINARY init "$MONIKER" --chain-id "$CHAIN_ID" --home "$NODE_HOME" 2>/dev/null

    # Create validator key
    $BINARY keys add "$MONIKER" --keyring-backend test --home "$NODE_HOME" --output json 2>/dev/null > "$NODE_HOME/key.json"
    ADDR=$($BINARY keys show "$MONIKER" --keyring-backend test --home "$NODE_HOME" -a 2>/dev/null)
    VALIDATOR_ADDRS+=("$ADDR")

    # Get node ID
    NODE_ID=$($BINARY tendermint show-node-id --home "$NODE_HOME" 2>/dev/null)
    NODE_IDS+=("$NODE_ID")

    log "  Node $i: $NODE_ID / $ADDR"
done

# ─── Create faucet account ─────────────────────────────────
FAUCET_HOME="$BASE_DIR/node1"
$BINARY keys add faucet --keyring-backend test --home "$FAUCET_HOME" --output json 2>/dev/null > "$FAUCET_HOME/faucet_key.json"
FAUCET_ADDR=$($BINARY keys show faucet --keyring-backend test --home "$FAUCET_HOME" -a 2>/dev/null)
log "Faucet address: $FAUCET_ADDR"

# ─── Build genesis on node 1 ──────────────────────────────
GENESIS="$BASE_DIR/node1/config/genesis.json"
log "Building genesis..."

# Add genesis accounts
for i in $(seq 1 $NUM_VALIDATORS); do
    NODE_HOME="$BASE_DIR/node$i"
    MONIKER="validator-$i"
    ADDR="${VALIDATOR_ADDRS[$((i-1))]}"

    $BINARY genesis add-genesis-account "$ADDR" "${INITIAL_SUPPLY}${DENOM}" \
        --keyring-backend test --home "$BASE_DIR/node1" 2>/dev/null
done

# Add faucet account
$BINARY genesis add-genesis-account "$FAUCET_ADDR" "${FAUCET_AMOUNT}${DENOM}" \
    --keyring-backend test --home "$BASE_DIR/node1" 2>/dev/null

# Create gentx for each validator
for i in $(seq 1 $NUM_VALIDATORS); do
    NODE_HOME="$BASE_DIR/node$i"
    MONIKER="validator-$i"

    # Copy genesis to each node
    if [ $i -gt 1 ]; then
        cp "$GENESIS" "$NODE_HOME/config/genesis.json"
    fi

    $BINARY genesis gentx "$MONIKER" "${VALIDATOR_STAKE}${STAKE_DENOM}" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend test \
        --home "$NODE_HOME" \
        --moniker "$MONIKER" \
        --commission-rate "0.05" \
        --commission-max-rate "0.20" \
        --commission-max-change-rate "0.01" 2>/dev/null

    # Copy gentx to node1
    if [ $i -gt 1 ]; then
        cp "$NODE_HOME/config/gentx/"*.json "$BASE_DIR/node1/config/gentx/"
    fi
done

# Collect all gentxs
$BINARY genesis collect-gentxs --home "$BASE_DIR/node1" 2>/dev/null

# ─── Configure governance & module params ──────────────────
if command -v jq >/dev/null 2>&1; then
    # Fast governance for testnet
    jq ".app_state.gov.params.voting_period = \"$GOV_VOTING_PERIOD\"" "$GENESIS" > "${GENESIS}.tmp" && mv "${GENESIS}.tmp" "$GENESIS"
    jq ".app_state.gov.params.min_deposit[0].amount = \"1000000\"" "$GENESIS" > "${GENESIS}.tmp" && mv "${GENESIS}.tmp" "$GENESIS"

    # Set feemarket treasury
    jq ".app_state.feemarket.params.treasury_address = \"${VALIDATOR_ADDRS[0]}\"" "$GENESIS" > "${GENESIS}.tmp" && mv "${GENESIS}.tmp" "$GENESIS"

    log "Genesis parameters configured via jq"
else
    warn "jq not available — using default genesis params"
fi

# Validate genesis
$BINARY genesis validate --home "$BASE_DIR/node1" 2>/dev/null || err "Genesis validation failed"
log "Genesis validated successfully"

# ─── Distribute genesis and configure peers ────────────────
BASE_P2P_PORT=26656
BASE_RPC_PORT=26657
BASE_API_PORT=1317
BASE_GRPC_PORT=9090

PEERS=""
for i in $(seq 1 $NUM_VALIDATORS); do
    P2P_PORT=$((BASE_P2P_PORT + (i-1) * 10))
    NODE_ID="${NODE_IDS[$((i-1))]}"
    if [ -n "$PEERS" ]; then PEERS="${PEERS},"; fi
    PEERS="${PEERS}${NODE_ID}@127.0.0.1:${P2P_PORT}"
done

for i in $(seq 1 $NUM_VALIDATORS); do
    NODE_HOME="$BASE_DIR/node$i"
    P2P_PORT=$((BASE_P2P_PORT + (i-1) * 10))
    RPC_PORT=$((BASE_RPC_PORT + (i-1) * 10))
    API_PORT=$((BASE_API_PORT + (i-1) * 10))
    GRPC_PORT=$((BASE_GRPC_PORT + (i-1) * 10))

    # Copy final genesis
    cp "$GENESIS" "$NODE_HOME/config/genesis.json"

    # Configure ports (avoid conflicts)
    CONFIG="$NODE_HOME/config/config.toml"
    APP_CONFIG="$NODE_HOME/config/app.toml"

    sed -i "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${P2P_PORT}\"|g" "$CONFIG"
    sed -i "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://127.0.0.1:${RPC_PORT}\"|g" "$CONFIG"
    sed -i "s|persistent_peers = \"\"|persistent_peers = \"${PEERS}\"|g" "$CONFIG"
    sed -i "s|allow_duplicate_ip = false|allow_duplicate_ip = true|g" "$CONFIG"

    # API & gRPC ports
    sed -i "s|address = \"tcp://localhost:1317\"|address = \"tcp://localhost:${API_PORT}\"|g" "$APP_CONFIG"
    sed -i "s|address = \"localhost:9090\"|address = \"localhost:${GRPC_PORT}\"|g" "$APP_CONFIG"

    # Enable API
    sed -i "s|enable = false|enable = true|g" "$APP_CONFIG"

    log "Node $i: P2P=$P2P_PORT RPC=$RPC_PORT API=$API_PORT gRPC=$GRPC_PORT"
done

# ─── Generate start script ────────────────────────────────
START_SCRIPT="$BASE_DIR/start_all.sh"
cat > "$START_SCRIPT" << 'STARTEOF'
#!/bin/bash
BASE_DIR="$HOME/.omniphi-testnet"
BINARY="posd"

for node_dir in "$BASE_DIR"/node*; do
    node_name=$(basename "$node_dir")
    echo "Starting $node_name..."
    $BINARY start --home "$node_dir" \
        --log_level info \
        --minimum-gas-prices "0.001omniphi" \
        > "$node_dir/node.log" 2>&1 &
    echo "$!" > "$node_dir/node.pid"
    echo "  PID: $(cat $node_dir/node.pid)"
done

echo ""
echo "All nodes started. Logs in \$HOME/.omniphi-testnet/nodeN/node.log"
echo "Stop with: $BASE_DIR/stop_all.sh"
STARTEOF
chmod +x "$START_SCRIPT"

# ─── Generate stop script ─────────────────────────────────
STOP_SCRIPT="$BASE_DIR/stop_all.sh"
cat > "$STOP_SCRIPT" << 'STOPEOF'
#!/bin/bash
BASE_DIR="$HOME/.omniphi-testnet"

for node_dir in "$BASE_DIR"/node*; do
    node_name=$(basename "$node_dir")
    if [ -f "$node_dir/node.pid" ]; then
        PID=$(cat "$node_dir/node.pid")
        if kill -0 "$PID" 2>/dev/null; then
            echo "Stopping $node_name (PID $PID)..."
            kill "$PID"
        fi
        rm "$node_dir/node.pid"
    fi
done
echo "All nodes stopped."
STOPEOF
chmod +x "$STOP_SCRIPT"

# ─── Generate status script ───────────────────────────────
STATUS_SCRIPT="$BASE_DIR/status.sh"
cat > "$STATUS_SCRIPT" << 'STATUSEOF'
#!/bin/bash
BASE_DIR="$HOME/.omniphi-testnet"

echo "=== Omniphi Testnet Status ==="
echo ""
for node_dir in "$BASE_DIR"/node*; do
    node_name=$(basename "$node_dir")
    if [ -f "$node_dir/node.pid" ]; then
        PID=$(cat "$node_dir/node.pid")
        if kill -0 "$PID" 2>/dev/null; then
            echo "$node_name: RUNNING (PID $PID)"
        else
            echo "$node_name: DEAD (stale PID $PID)"
        fi
    else
        echo "$node_name: STOPPED"
    fi
done

echo ""
echo "=== Latest Block ==="
curl -s http://127.0.0.1:26657/status 2>/dev/null | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    r=d['result']['sync_info']
    print(f\"Height: {r['latest_block_height']}\")
    print(f\"Time:   {r['latest_block_time']}\")
    print(f\"Catching up: {r['catching_up']}\")
except: print('RPC unavailable')
" 2>/dev/null || echo "RPC unavailable"
STATUSEOF
chmod +x "$STATUS_SCRIPT"

# ─── Summary ──────────────────────────────────────────────
echo ""
log "=========================================="
log " Testnet Ready: $CHAIN_ID"
log "=========================================="
log " Validators: $NUM_VALIDATORS"
log " Chain ID:   $CHAIN_ID"
log " Denom:      $DENOM"
log ""
log " Start:  $START_SCRIPT"
log " Stop:   $STOP_SCRIPT"
log " Status: $STATUS_SCRIPT"
log ""
log " Faucet address: $FAUCET_ADDR"
log ""
log " RPC Endpoints:"
for i in $(seq 1 $NUM_VALIDATORS); do
    RPC_PORT=$((BASE_RPC_PORT + (i-1) * 10))
    log "   Node $i: http://127.0.0.1:${RPC_PORT}"
done
log ""
log " API Endpoints:"
for i in $(seq 1 $NUM_VALIDATORS); do
    API_PORT=$((BASE_API_PORT + (i-1) * 10))
    log "   Node $i: http://127.0.0.1:${API_PORT}"
done
echo ""
