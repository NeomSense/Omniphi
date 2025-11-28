#!/bin/bash

# OMNIPHI TESTNET VALIDATOR SETUP SCRIPT
# This script automates the setup of a validator node for omniphi-testnet-1

set -e

echo "=================================================="
echo "OMNIPHI TESTNET VALIDATOR SETUP"
echo "Chain ID: omniphi-testnet-1"
echo "=================================================="
echo ""

# Configuration
CHAIN_ID="omniphi-testnet-1"
MONIKER="${1:-validator-node}"
BINARY_NAME="posd"
HOME_DIR="$HOME/.posd"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    print_error "Please do not run this script as root"
    exit 1
fi

# Step 1: Check prerequisites
echo "Step 1: Checking prerequisites..."

if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
print_success "Go version: $GO_VERSION"

if ! command -v git &> /dev/null; then
    print_error "Git is not installed. Please install git first."
    exit 1
fi
print_success "Git is installed"

if ! command -v jq &> /dev/null; then
    print_warning "jq is not installed. Installing..."
    sudo apt install -y jq
fi
print_success "jq is installed"

# Step 2: Build or check binary
echo ""
echo "Step 2: Setting up posd binary..."

if command -v $BINARY_NAME &> /dev/null; then
    print_success "posd binary found at $(which $BINARY_NAME)"
    $BINARY_NAME version
else
    print_warning "posd not found. Building from source..."

    # Clone repository if not exists
    if [ ! -d "pos" ]; then
        git clone https://github.com/omniphi/pos.git
        cd pos
    else
        cd pos
        git pull
    fi

    # Build binary
    make build

    # Install binary
    sudo cp build/$BINARY_NAME /usr/local/bin/
    sudo chmod +x /usr/local/bin/$BINARY_NAME

    cd ..
    print_success "posd binary built and installed"
fi

# Step 3: Initialize node
echo ""
echo "Step 3: Initializing node..."

if [ -d "$HOME_DIR" ]; then
    print_warning "Node already initialized at $HOME_DIR"
    read -p "Do you want to reset and reinitialize? (yes/no): " RESET
    if [ "$RESET" = "yes" ]; then
        rm -rf $HOME_DIR
        print_success "Old data removed"
    else
        print_warning "Skipping initialization"
    fi
fi

if [ ! -d "$HOME_DIR" ]; then
    $BINARY_NAME init $MONIKER --chain-id $CHAIN_ID
    print_success "Node initialized with moniker: $MONIKER"
fi

# Step 4: Create or import validator key
echo ""
echo "Step 4: Setting up validator key..."

if $BINARY_NAME keys list | grep -q "validator"; then
    print_success "Validator key already exists"
else
    echo "Choose an option:"
    echo "1. Create new validator key"
    echo "2. Import existing key from mnemonic"
    read -p "Enter choice (1 or 2): " KEY_CHOICE

    if [ "$KEY_CHOICE" = "1" ]; then
        $BINARY_NAME keys add validator
        print_success "New validator key created"
        print_warning "IMPORTANT: Save your mnemonic phrase securely!"
    elif [ "$KEY_CHOICE" = "2" ]; then
        $BINARY_NAME keys add validator --recover
        print_success "Validator key imported"
    else
        print_error "Invalid choice"
        exit 1
    fi
fi

VALIDATOR_ADDRESS=$($BINARY_NAME keys show validator -a)
print_success "Validator address: $VALIDATOR_ADDRESS"

# Step 5: Download genesis file
echo ""
echo "Step 5: Downloading genesis file..."

GENESIS_URL="https://raw.githubusercontent.com/omniphi/testnets/main/omniphi-testnet-1/genesis.json"

if [ -f "$HOME_DIR/config/genesis.json" ]; then
    print_warning "Genesis file already exists"
    read -p "Do you want to download the latest? (yes/no): " DOWNLOAD
    if [ "$DOWNLOAD" = "yes" ]; then
        wget -q $GENESIS_URL -O $HOME_DIR/config/genesis.json || {
            print_error "Failed to download genesis. Using manual configuration."
            print_warning "Please manually download genesis.json from coordinator"
        }
    fi
else
    wget -q $GENESIS_URL -O $HOME_DIR/config/genesis.json || {
        print_warning "Genesis file not available yet. You'll need to add it manually."
    }
fi

if [ -f "$HOME_DIR/config/genesis.json" ]; then
    GENESIS_HASH=$(sha256sum $HOME_DIR/config/genesis.json | awk '{print $1}')
    print_success "Genesis file hash: $GENESIS_HASH"
fi

# Step 6: Configure node
echo ""
echo "Step 6: Configuring node..."

# Set minimum gas price
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' $HOME_DIR/config/app.toml
print_success "Set minimum gas price to 0.001uomni"

# Enable API
sed -i 's/enable = false/enable = true/' $HOME_DIR/config/app.toml
print_success "Enabled REST API"

# Enable Prometheus
sed -i 's/prometheus = false/prometheus = true/' $HOME_DIR/config/config.toml
print_success "Enabled Prometheus metrics"

# Configure peers (update these with actual peer addresses)
read -p "Enter persistent peers (comma-separated, or press Enter to skip): " PEERS
if [ ! -z "$PEERS" ]; then
    sed -i "s/persistent_peers = \"\"/persistent_peers = \"$PEERS\"/" $HOME_DIR/config/config.toml
    print_success "Set persistent peers"
fi

# Step 7: Create systemd service
echo ""
echo "Step 7: Creating systemd service..."

SERVICE_FILE="/etc/systemd/system/$BINARY_NAME.service"

if [ -f "$SERVICE_FILE" ]; then
    print_warning "Systemd service already exists"
else
    sudo tee $SERVICE_FILE > /dev/null <<EOF
[Unit]
Description=Omniphi Node
After=network-online.target

[Service]
User=$USER
ExecStart=$(which $BINARY_NAME) start
Restart=always
RestartSec=3
LimitNOFILE=4096
Environment="HOME=$HOME"

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable $BINARY_NAME
    print_success "Systemd service created and enabled"
fi

# Step 8: Genesis transaction (for new validators)
echo ""
echo "Step 8: Genesis transaction (for genesis validators only)..."

read -p "Are you a genesis validator? (yes/no): " IS_GENESIS

if [ "$IS_GENESIS" = "yes" ]; then
    read -p "Enter self-delegation amount in uomni (e.g., 50000000000): " SELF_DELEGATION

    # Add genesis account
    $BINARY_NAME genesis add-genesis-account $VALIDATOR_ADDRESS ${SELF_DELEGATION}uomni

    # Create gentx
    $BINARY_NAME genesis gentx validator ${SELF_DELEGATION}uomni \
        --chain-id $CHAIN_ID \
        --moniker $MONIKER \
        --commission-rate 0.10 \
        --commission-max-rate 0.20 \
        --commission-max-change-rate 0.01 \
        --min-self-delegation 1

    print_success "Genesis transaction created"
    print_warning "Send your gentx file to the coordinator:"
    print_warning "Location: $HOME_DIR/config/gentx/gentx-*.json"
else
    print_success "Skipping genesis transaction (not a genesis validator)"
fi

# Step 9: Final checklist
echo ""
echo "=================================================="
echo "SETUP COMPLETE!"
echo "=================================================="
echo ""
echo "Next steps:"
echo ""
echo "1. If you're a genesis validator:"
echo "   - Send your gentx file to the coordinator"
echo "   - Wait for final genesis.json"
echo "   - Download final genesis.json to $HOME_DIR/config/genesis.json"
echo ""
echo "2. Configure peers in $HOME_DIR/config/config.toml"
echo ""
echo "3. Start your node:"
echo "   sudo systemctl start $BINARY_NAME"
echo ""
echo "4. Check logs:"
echo "   sudo journalctl -u $BINARY_NAME -f"
echo ""
echo "5. Check status:"
echo "   $BINARY_NAME status"
echo ""
echo "6. Verify tokenomics:"
echo "   $BINARY_NAME query tokenomics supply"
echo "   $BINARY_NAME query tokenomics params"
echo ""
echo "=================================================="
echo "Validator address: $VALIDATOR_ADDRESS"
echo "Moniker: $MONIKER"
echo "Home directory: $HOME_DIR"
echo "=================================================="
echo ""
print_success "Setup script completed successfully!"
echo ""
print_warning "IMPORTANT: Backup your validator key!"
echo "Key location: $HOME_DIR/config/priv_validator_key.json"
echo ""
