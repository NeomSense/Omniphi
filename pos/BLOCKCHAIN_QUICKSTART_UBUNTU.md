# Omniphi Blockchain - Ubuntu Quick Start Guide

**Version:** 1.0 | **Last Updated:** 2025-11-20
**Platform:** Ubuntu 20.04+ / Debian-based Linux

Get your Omniphi validator node running in **5 minutes** or follow the comprehensive guide for production deployment.

---

## ⚠️ Important: Always Rebuild After Code Changes

**If you've made code changes or pulled updates from git, you MUST rebuild the binary:**

```bash
cd ~/omniphi/pos
rm -f posd
go build -o posd ./cmd/posd
```

**Always use the local binary:** `./posd` (not just `posd`)

This ensures you're running the latest code with all fixes applied.

---

## 🚀 5-Minute Quick Start

Perfect for development, testing, or getting familiar with Omniphi.

### Prerequisites Check

```bash
# Check if Go is installed (need 1.21+)
go version

# Check if make is installed
make --version
```

If missing, see [Prerequisites Installation](#prerequisites-installation) below.

### Quick Setup

```bash
# 1. Clone the repository
git clone https://github.com/omniphi/pos.git
cd pos

# 2. Build the binary
make install

# 3. Initialize your node
posd init my-node --chain-id omniphi-testnet-1

# 4. Download genesis file (testnet)
curl -o ~/.posd/config/genesis.json https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json

# 5. Start the node
posd start
```

**That's it!** Your node is now syncing with the testnet.

Press `Ctrl+C` to stop the node.

---

## 📋 Prerequisites Installation

### Install Go (1.21+)

```bash
# Download Go 1.21
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# Remove old installation (if exists)
sudo rm -rf /usr/local/go

# Extract new version
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc for permanent)
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Verify
go version
```

### Install Build Tools

```bash
sudo apt update
sudo apt install -y build-essential git curl wget
```

### Install Optional Tools

```bash
# jq - For parsing JSON output
sudo apt install -y jq

# tmux - For running node in background
sudo apt install -y tmux
```

---

## 📦 Building from Source

### Step 1: Clone Repository

```bash
git clone https://github.com/omniphi/pos.git
cd pos
```

### Step 2: Build Binary

```bash
# Option 1: Build locally in project directory (RECOMMENDED for development)
cd ~/omniphi/pos
go build -o posd ./cmd/posd

# Verify build
./posd version

# Use this binary
./posd start
```

**Alternative: Install to system PATH**
```bash
# Build and install to $GOPATH/bin
make install

# Verify installation
posd version
which posd
```

**Important Notes:**
- **Always rebuild after code changes:** `cd ~/omniphi/pos && go build -o posd ./cmd/posd`
- Local builds (./posd) give you control over which binary you're running
- System installs (make install) are convenient but can be confusing if you make code changes

---

## 🏁 Single-Node Setup (Development)

Perfect for local development and testing.

### 1. Initialize Node

```bash
# Create node configuration
posd init my-validator --chain-id omniphi-testnet-1

# This creates ~/.posd/ with config/ and data/ directories
```

### 2. Configure Node

```bash
# Edit config (optional)
nano ~/.posd/config/config.toml

# Key settings:
# - moniker = "my-validator"
# - minimum-gas-prices = "0.025uomni"
```

### 3. Create Validator Wallet

```bash
# Create new wallet
posd keys add my-wallet

# IMPORTANT: Save the mnemonic phrase!
# Save the address (omni1...)
```

### 4. Add Genesis Account (Local Testnet Only)

```bash
# Add your wallet to genesis with tokens
posd genesis add-genesis-account $(posd keys show my-wallet -a) 100000000000uomni

# Create genesis transaction
posd genesis gentx my-wallet 10000000000uomni \
  --chain-id omniphi-testnet-1 \
  --moniker my-validator

# Collect genesis transactions
posd genesis collect-gentxs
```

### 5. Start Node

```bash
# IMPORTANT: Always use the local binary you built
cd ~/omniphi/pos

# Start in foreground
./posd start

# Or start in background with tmux
tmux new -s validator
cd ~/omniphi/pos && ./posd start
# Press Ctrl+B then D to detach
```

### 6. Check Status

```bash
# Query node status (use local binary)
cd ~/omniphi/pos
./posd status

# Check sync status
./posd status | jq '.SyncInfo.catching_up'

# Check latest block
./posd status | jq '.SyncInfo.latest_block_height'
```

---

## 🌐 Two-Node Testnet Setup

Run a complete testnet with two validators on separate machines or same machine.

### Option A: Two Separate Machines

**See:** [MULTI_NODE_TESTNET_GUIDE.md](MULTI_NODE_TESTNET_GUIDE.md) for comprehensive 2-computer setup.

### Option B: Two Validators on Same Machine

```bash
# 1. Create separate home directories
mkdir -p ~/testnet/{validator1,validator2}

# 2. Initialize both validators
posd init validator1 --chain-id omniphi-testnet-1 --home ~/testnet/validator1
posd init validator2 --chain-id omniphi-testnet-1 --home ~/testnet/validator2

# 3. Create keys for both
posd keys add validator1 --home ~/testnet/validator1
posd keys add validator2 --home ~/testnet/validator2

# 4. Configure different ports for validator2
# Edit ~/testnet/validator2/config/config.toml:
# [rpc]
# laddr = "tcp://127.0.0.1:26757"
# [p2p]
# laddr = "tcp://0.0.0.0:26756"
# [grpc]
# address = "0.0.0.0:9190"

# 5. Add both to genesis
posd genesis add-genesis-account $(posd keys show validator1 -a --home ~/testnet/validator1) 100000000000uomni \
  --home ~/testnet/validator1

posd genesis add-genesis-account $(posd keys show validator2 -a --home ~/testnet/validator2) 100000000000uomni \
  --home ~/testnet/validator1

# 6. Create gentx for both
posd genesis gentx validator1 10000000000uomni \
  --chain-id omniphi-testnet-1 \
  --home ~/testnet/validator1

posd genesis gentx validator2 10000000000uomni \
  --chain-id omniphi-testnet-1 \
  --home ~/testnet/validator2

# 7. Copy validator2's gentx to validator1
cp ~/testnet/validator2/config/gentx/*.json ~/testnet/validator1/config/gentx/

# 8. Collect gentxs
posd genesis collect-gentxs --home ~/testnet/validator1

# 9. Copy genesis to validator2
cp ~/testnet/validator1/config/genesis.json ~/testnet/validator2/config/genesis.json

# 10. Get validator1's node ID for peering
VALIDATOR1_ID=$(posd tendermint show-node-id --home ~/testnet/validator1)
echo "Validator1 ID: $VALIDATOR1_ID"

# 11. Configure validator2 to connect to validator1
# Edit ~/testnet/validator2/config/config.toml:
# persistent_peers = "$VALIDATOR1_ID@127.0.0.1:26656"

# 12. Start both validators (in separate terminals)
# Terminal 1:
posd start --home ~/testnet/validator1

# Terminal 2:
posd start --home ~/testnet/validator2
```

---

## 🔗 Connect to Existing Testnet

Join the official Omniphi testnet.

### 1. Initialize Node

```bash
posd init my-node --chain-id omniphi-testnet-1
```

### 2. Download Genesis

```bash
curl -o ~/.posd/config/genesis.json \
  https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json
```

### 3. Configure Seeds/Peers

```bash
# Edit ~/.posd/config/config.toml
nano ~/.posd/config/config.toml

# Add seed nodes:
seeds = "node1-id@seed1.omniphi.network:26656,node2-id@seed2.omniphi.network:26656"

# Or persistent peers:
persistent_peers = "peer1-id@peer1.omniphi.network:26656"
```

### 4. Set Minimum Gas Prices

```bash
# Edit ~/.posd/config/app.toml
nano ~/.posd/config/app.toml

# Set:
minimum-gas-prices = "0.025uomni"
```

### 5. Start and Sync

```bash
# Use local binary
cd ~/omniphi/pos
./posd start

# Check sync status
./posd status | jq '.SyncInfo.catching_up'
# false = synced, true = still syncing
```

---

## 🛠️ Common Commands

### Node Management

```bash
# IMPORTANT: Always use local binary from project directory
cd ~/omniphi/pos

# Start node
./posd start

# Start with custom home
./posd start --home ~/.custom-node

# Check status
./posd status

# Show node ID
./posd tendermint show-node-id

# Show validator address
./posd tendermint show-validator
```

### Wallet Operations

```bash
# Create wallet
posd keys add my-wallet

# List wallets
posd keys list

# Show wallet address
posd keys show my-wallet -a

# Export private key
posd keys export my-wallet

# Import wallet from mnemonic
posd keys add my-wallet --recover
```

### Query Commands

```bash
# Check balance
posd query bank balances $(posd keys show my-wallet -a)

# Check validator info
posd query staking validator $(posd keys show my-wallet --bech val -a)

# Check delegation
posd query staking delegation $(posd keys show my-wallet -a) $(posd keys show my-wallet --bech val -a)
```

### Transaction Commands

```bash
# Send tokens
posd tx bank send my-wallet omni1recipient1000... 1000000uomni \
  --chain-id omniphi-testnet-1 \
  --fees 5000uomni

# Delegate to validator
posd tx staking delegate $(posd keys show my-wallet --bech val -a) 1000000uomni \
  --from my-wallet \
  --chain-id omniphi-testnet-1 \
  --fees 5000uomni
```

---

## 🔧 Troubleshooting

### Issue: "module 'feemarket' is missing a type URL" panic

**Root Cause:** You're running an old binary that was built before proto file fixes.

**Solution:**
```bash
# Always rebuild after code changes or git pull
cd ~/omniphi/pos
rm -f posd
go build -o posd ./cmd/posd

# Verify it's the new binary
./posd version
ls -lh posd  # Check timestamp is recent
```

### Issue: "command not found: posd"

**Solution:**
```bash
# Use local binary from project directory
cd ~/omniphi/pos
./posd version

# If binary doesn't exist, rebuild it
go build -o posd ./cmd/posd
```

### Issue: "Error: genesis.json not found"

**Solution:**
```bash
# Download genesis file
curl -o ~/.posd/config/genesis.json \
  https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json
```

### Issue: "Error: minimum gas price not set"

**Solution:**
```bash
# Edit app.toml
nano ~/.posd/config/app.toml

# Set minimum gas prices
minimum-gas-prices = "0.025uomni"
```

### Issue: "Node not syncing / No peers"

**Solution:**
```bash
# 1. Check seeds are configured
grep "seeds" ~/.posd/config/config.toml

# 2. Check peer configuration
cd ~/omniphi/pos
./posd tendermint show-node-id

# 3. Restart node
pkill posd
cd ~/omniphi/pos && ./posd start
```

### Issue: "Error: failed to execute message"

**Solution:**
```bash
# Ensure sufficient gas
posd tx ... --gas auto --gas-adjustment 1.5

# Check account has tokens
posd query bank balances $(posd keys show my-wallet -a)
```

---

## 🚀 Running as System Service

For production, run the node as a systemd service.

### Create Service File

```bash
sudo nano /etc/systemd/system/posd.service
```

**Service configuration:**
```ini
[Unit]
Description=Omniphi Node
After=network-online.target

[Service]
User=ubuntu
ExecStart=/home/ubuntu/go/bin/posd start --home /home/ubuntu/.posd
Restart=on-failure
RestartSec=3
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
```

### Enable and Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (start on boot)
sudo systemctl enable posd

# Start service
sudo systemctl start posd

# Check status
sudo systemctl status posd

# View logs
sudo journalctl -u posd -f
```

### Service Commands

```bash
# Stop
sudo systemctl stop posd

# Restart
sudo systemctl restart posd

# Disable (don't start on boot)
sudo systemctl disable posd
```

---

## 📚 Next Steps

### For Development

- **Testing**: [UBUNTU_TESTING_GUIDE.md](UBUNTU_TESTING_GUIDE.md)
- **Module Development**: [x/poc/README.md](x/poc/README.md)

### For Production

- **Production Checklist**: [PRODUCTION_DEPLOYMENT_CHECKLIST.md](PRODUCTION_DEPLOYMENT_CHECKLIST.md)
- **Security Audit**: [SECURITY_AUDIT_REPORT.md](SECURITY_AUDIT_REPORT.md)
- **Monitoring**: [MONITORING.md](MONITORING.md)

### Advanced Topics

- **Multi-Node Setup**: [MULTI_NODE_TESTNET_GUIDE.md](MULTI_NODE_TESTNET_GUIDE.md)
- **Tokenomics**: [TOKENOMICS_FULL_REPORT.md](TOKENOMICS_FULL_REPORT.md)
- **Governance**: [TREASURY_MULTISIG_GUIDE.md](TREASURY_MULTISIG_GUIDE.md)

---

## 🆘 Getting Help

- **Documentation**: [README.md](README.md)
- **Discord**: https://discord.gg/omniphi
- **GitHub Issues**: https://github.com/omniphi/pos/issues
- **Email**: support@omniphi.io

---

**Denomination Reference:**
- `uomni` = 0.000001 OMNI (micro-omni)
- `1 OMNI` = 1,000,000 uomni
- Example: `1000000uomni` = 1 OMNI
