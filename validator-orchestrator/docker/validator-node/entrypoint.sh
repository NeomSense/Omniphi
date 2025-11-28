#!/bin/bash
# Omniphi Validator Node Entrypoint Script

set -e

CHAIN_ID="${CHAIN_ID:-omniphi-localnet-1}"
MONIKER="${MONIKER:-validator-node}"
NODE_HOME="${NODE_HOME:-/home/validator/.omniphi}"

echo "=================================================="
echo "Starting Omniphi Validator Node"
echo "Chain ID: $CHAIN_ID"
echo "Moniker: $MONIKER"
echo "Home: $NODE_HOME"
echo "=================================================="

# Initialize node if not already initialized
if [ ! -f "$NODE_HOME/config/genesis.json" ]; then
    echo "Initializing node..."
    posd init "$MONIKER" --chain-id "$CHAIN_ID" --home "$NODE_HOME"

    # If genesis file is provided via mount, use it
    if [ -f "/genesis/genesis.json" ]; then
        echo "Using provided genesis file..."
        cp /genesis/genesis.json "$NODE_HOME/config/genesis.json"
    fi

    # If seeds are provided, configure them
    if [ ! -z "$SEEDS" ]; then
        echo "Configuring seeds: $SEEDS"
        sed -i "s/seeds = \"\"/seeds = \"$SEEDS\"/" "$NODE_HOME/config/config.toml"
    fi

    # If persistent peers are provided, configure them
    if [ ! -z "$PERSISTENT_PEERS" ]; then
        echo "Configuring persistent peers: $PERSISTENT_PEERS"
        sed -i "s/persistent_peers = \"\"/persistent_peers = \"$PERSISTENT_PEERS\"/" "$NODE_HOME/config/config.toml"
    fi

    # Enable RPC
    sed -i 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' "$NODE_HOME/config/config.toml"

    # Enable API
    sed -i 's/enable = false/enable = true/' "$NODE_HOME/config/app.toml"
    sed -i 's/swagger = false/swagger = true/' "$NODE_HOME/config/app.toml"

    echo "Node initialized successfully"
else
    echo "Node already initialized, using existing configuration"
fi

# Display consensus public key
echo "=================================================="
echo "Consensus Public Key:"
posd tendermint show-validator --home "$NODE_HOME"
echo "=================================================="

# Start the node
echo "Starting validator node..."
exec posd start --home "$NODE_HOME" "$@"
