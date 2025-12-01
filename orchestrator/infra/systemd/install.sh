#!/bin/bash

# Omniphi Validator - Systemd Service Installation Script
# This script installs posd as a systemd service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="posd"
SERVICE_USER="${VALIDATOR_USER:-omniphi}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
BINARY_PATH="${POSD_BINARY:-/usr/local/bin/posd}"
HOME_DIR="${VALIDATOR_HOME:-/home/${SERVICE_USER}}"
DATA_DIR="${HOME_DIR}/.omniphi"

echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}Omniphi Validator Service Installer${NC}"
echo -e "${GREEN}=================================${NC}"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Error: This script must be run as root${NC}"
   exit 1
fi

# Check if binary exists
if [[ ! -f "$BINARY_PATH" ]]; then
    echo -e "${RED}Error: posd binary not found at $BINARY_PATH${NC}"
    echo "Please install posd first or set POSD_BINARY environment variable"
    exit 1
fi

echo -e "${YELLOW}Configuration:${NC}"
echo "  User: $SERVICE_USER"
echo "  Home: $HOME_DIR"
echo "  Data: $DATA_DIR"
echo "  Binary: $BINARY_PATH"
echo ""

# Create service user if doesn't exist
if ! id "$SERVICE_USER" &>/dev/null; then
    echo -e "${YELLOW}Creating service user: $SERVICE_USER${NC}"
    useradd -m -s /bin/bash "$SERVICE_USER"
    echo -e "${GREEN}✓ User created${NC}"
else
    echo -e "${GREEN}✓ User already exists${NC}"
fi

# Ensure data directory exists with correct permissions
if [[ ! -d "$DATA_DIR" ]]; then
    echo -e "${YELLOW}Creating data directory: $DATA_DIR${NC}"
    mkdir -p "$DATA_DIR"
fi

chown -R "${SERVICE_USER}:${SERVICE_USER}" "$DATA_DIR"
chmod 700 "$DATA_DIR"
echo -e "${GREEN}✓ Data directory configured${NC}"

# Create service file
echo -e "${YELLOW}Creating systemd service file...${NC}"
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Omniphi Validator Node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$HOME_DIR
ExecStart=$BINARY_PATH start --home $DATA_DIR
Restart=on-failure
RestartSec=10
LimitNOFILE=65535
LimitNPROC=4096

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=false
ReadWritePaths=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF

echo -e "${GREEN}✓ Service file created${NC}"

# Reload systemd
echo -e "${YELLOW}Reloading systemd daemon...${NC}"
systemctl daemon-reload
echo -e "${GREEN}✓ Systemd reloaded${NC}"

# Enable service
echo -e "${YELLOW}Enabling $SERVICE_NAME service...${NC}"
systemctl enable "$SERVICE_NAME"
echo -e "${GREEN}✓ Service enabled${NC}"

echo ""
echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}Installation Complete!${NC}"
echo -e "${GREEN}=================================${NC}"
echo ""
echo "To start the validator:"
echo "  sudo systemctl start $SERVICE_NAME"
echo ""
echo "To check status:"
echo "  sudo systemctl status $SERVICE_NAME"
echo ""
echo "To view logs:"
echo "  sudo journalctl -u $SERVICE_NAME -f"
echo ""
echo "To stop the validator:"
echo "  sudo systemctl stop $SERVICE_NAME"
echo ""
echo "To restart the validator:"
echo "  sudo systemctl restart $SERVICE_NAME"
echo ""
echo -e "${YELLOW}Important: Make sure the node is initialized before starting!${NC}"
echo "Initialize with: sudo -u $SERVICE_USER $BINARY_PATH init <moniker> --chain-id omniphi-1"
echo ""
