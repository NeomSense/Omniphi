# Traditional Validator Setup Guide

**Complete guide for running an Omniphi validator node using traditional CLI methods**

**Target Audience:** System administrators, DevOps engineers, and advanced users who prefer full control over their validator setup.

**Estimated Time:** 30-60 minutes

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation Methods](#installation-methods)
3. [Node Initialization](#node-initialization)
4. [Configuration](#configuration)
5. [Starting the Validator](#starting-the-validator)
6. [Creating Your Validator On-Chain](#creating-your-validator-on-chain)
7. [Monitoring & Maintenance](#monitoring--maintenance)
8. [Security Hardening](#security-hardening)

---

## Prerequisites

### Hardware Requirements

**Minimum:**
- CPU: 4 cores
- RAM: 8 GB
- Storage: 500 GB SSD
- Network: 100 Mbps

**Recommended:**
- CPU: 8 cores (3.0+ GHz)
- RAM: 32 GB
- Storage: 2 TB NVMe SSD
- Network: 1 Gbps

### Software Requirements

- **Operating System:** Ubuntu 22.04 LTS (recommended), Debian 11+, or similar
- **Go:** 1.21+ (for building from source)
- **Git:** For cloning repositories
- **Make:** For using Makefiles

### Network Requirements

- Static IP address (recommended for validators)
- Open ports:
  - `26656` - P2P communication
  - `26657` - RPC (optional, can be localhost only)
  - `9090` - gRPC (optional)
  - `26660` - Prometheus metrics (optional)

---

## Installation Methods

### Method 1: Pre-Built Binary (Recommended)

#### Linux (Ubuntu/Debian)

```bash
# Download the latest release
wget https://github.com/omniphi/pos/releases/latest/download/posd-linux-amd64

# Make it executable
chmod +x posd-linux-amd64

# Move to system path
sudo mv posd-linux-amd64 /usr/local/bin/posd

# Verify installation
posd version
```

#### macOS

```bash
# Download for macOS
wget https://github.com/omniphi/pos/releases/latest/download/posd-darwin-amd64

# Make executable
chmod +x posd-darwin-amd64

# Move to path
sudo mv posd-darwin-amd64 /usr/local/bin/posd

# Verify
posd version
```

#### Windows

Download `posd-windows-amd64.exe` from [releases page](https://github.com/omniphi/pos/releases) and add to PATH.

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/omniphi/pos.git
cd pos

# Install dependencies
go mod download

# Build binary
make install

# Or build to specific location
make build
sudo cp build/posd /usr/local/bin/

# Verify
posd version
```

### Method 3: Docker

```bash
# Pull official image
docker pull ghcr.io/omniphi/posd:latest

# Run container
docker run -d \\
  --name omniphi-validator \\
  -p 26656:26656 \\
  -p 26657:26657 \\
  -v $HOME/.omniphi:/root/.omniphi \\
  ghcr.io/omniphi/posd:latest \\
  start
```

---

## Node Initialization

### 1. Initialize Node

```bash
# Initialize with your moniker (validator name)
posd init "my-validator" --chain-id omniphi-1

# This creates ~/.omniphi/ directory with:
# - config/     (configuration files)
# - data/       (blockchain data)
# - keyring-*/ (key storage)
```

### 2. Download Genesis File

```bash
# Download the network's genesis file
curl -o ~/.omniphi/config/genesis.json \\
  https://raw.githubusercontent.com/omniphi/networks/main/mainnet/genesis.json

# Verify genesis file
posd validate-genesis
```

### 3. Configure Seeds and Peers

```bash
# Edit config.toml
nano ~/.omniphi/config/config.toml

# Add seed nodes (for bootstrapping)
seeds = "node1-id@seed1.omniphi.network:26656,node2-id@seed2.omniphi.network:26656"

# Add persistent peers (always connected)
persistent_peers = "peer1-id@peer1.omniphi.network:26656"
```

**Get seed and peer information from:**
- [Omniphi Networks Repository](https://github.com/omniphi/networks)
- [Cosmos Chain Registry](https://github.com/cosmos/chain-registry)

---

## Configuration

### config.toml (CometBFT Configuration)

Key settings to configure:

```toml
# ~/.omniphi/config/config.toml

###############################################################################
###                   Main Base Config Options                              ###
###############################################################################

# A custom human readable name for this node
moniker = "my-validator"

###############################################################################
###                         RPC Server Configuration Options                ###
###############################################################################

[rpc]
# TCP or UNIX socket address for the RPC server to listen on
laddr = "tcp://127.0.0.1:26657"  # localhost only for security

# Maximum number of simultaneous connections
max_open_connections = 900

###############################################################################
###                         P2P Configuration Options                       ###
###############################################################################

[p2p]
# Address to listen for incoming connections
laddr = "tcp://0.0.0.0:26656"

# Address to advertise to peers for them to dial
external_address = "your.public.ip:26656"  # Set your public IP

# Seed nodes
seeds = "node1-id@seed1.omniphi.network:26656"

# Persistent peers
persistent_peers = "peer1-id@peer1.omniphi.network:26656"

# Maximum number of inbound peers
max_num_inbound_peers = 40

# Maximum number of outbound peers
max_num_outbound_peers = 10

###############################################################################
###                      Mempool Configuration Option                       ###
###############################################################################

[mempool]
size = 5000
cache_size = 10000

###############################################################################
###                    Consensus Configuration Options                      ###
###############################################################################

[consensus]
timeout_propose = "3s"
timeout_propose_delta = "500ms"
timeout_prevote = "1s"
timeout_prevote_delta = "500ms"
timeout_precommit = "1s"
timeout_precommit_delta = "500ms"
timeout_commit = "5s"
```

### app.toml (Application Configuration)

```toml
# ~/.omniphi/config/app.toml

###############################################################################
###                           Base Configuration                            ###
###############################################################################

# Minimum gas prices to accept for transactions
minimum-gas-prices = "0.025uomni"

# Pruning configuration
pruning = "default"  # or "nothing", "everything", "custom"

# When pruning is custom:
pruning-keep-recent = "100"
pruning-keep-every = "0"
pruning-interval = "10"

# State sync snapshots
[state-sync]
snapshot-interval = 1000
snapshot-keep-recent = 2

###############################################################################
###                         Telemetry Configuration                         ###
###############################################################################

[telemetry]
service-name = "omniphi-validator"
enabled = true
enable-hostname = true
enable-hostname-label = true
enable-service-label = true

# Prometheus endpoint
prometheus-retention-time = 60

###############################################################################
###                           API Configuration                              ###
###############################################################################

[api]
enable = true
swagger = true
address = "tcp://localhost:1317"  # localhost only
enabled-unsafe-cors = false

###############################################################################
###                           gRPC Configuration                             ###
###############################################################################

[grpc]
enable = true
address = "localhost:9090"  # localhost only

###############################################################################
###                        State Sync Configuration                          ###
###############################################################################

# Enable state sync for fast catch-up
[state-sync]
snapshot-interval = 1000
snapshot-keep-recent = 2
```

### client.toml (Client Configuration)

```toml
# ~/.omniphi/config/client.toml

# Chain ID
chain-id = "omniphi-1"

# Keyring backend (os, file, test)
keyring-backend = "os"  # Most secure

# CLI output format (text, json)
output = "text"

# Node RPC endpoint
node = "tcp://localhost:26657"

# Transaction broadcasting mode
broadcast-mode = "sync"
```

---

## Starting the Validator

### Manual Start (Foreground)

```bash
# Start node in foreground
posd start

# Press Ctrl+C to stop
```

### Background with tmux

```bash
# Start tmux session
tmux new -s validator

# Start validator
posd start

# Detach: Press Ctrl+B, then D
# Reattach: tmux attach -t validator
```

### Systemd Service (Recommended for Production)

See [Systemd Service Setup](../infra/systemd/README.md) for detailed instructions.

Quick setup:

```bash
# Create service file
sudo nano /etc/systemd/system/posd.service

# Add service configuration (see template below)

# Reload systemd
sudo systemctl daemon-reload

# Enable service
sudo systemctl enable posd

# Start service
sudo systemctl start posd

# Check status
sudo systemctl status posd

# View logs
sudo journalctl -u posd -f
```

### Wait for Sync

```bash
# Check sync status
posd status 2>&1 | jq .SyncInfo

# Wait until "catching_up": false
```

---

## Creating Your Validator On-Chain

### 1. Create Wallet

```bash
# Create new wallet (save mnemonic safely!)
posd keys add my-validator-wallet

# Or recover existing wallet
posd keys add my-validator-wallet --recover

# List all keys
posd keys list

# Get wallet address
posd keys show my-validator-wallet -a
```

### 2. Fund Your Wallet

Send tokens to your wallet address (omni1...).

Minimum required:
- Self-delegation: 1,000 OMNI
- Transaction fees: ~1 OMNI

### 3. Get Consensus Public Key

```bash
# Get validator consensus pubkey
posd tendermint show-validator

# Output: {"@type":"/cosmos.crypto.ed25519.PubKey","key":"..."}
```

### 4. Create Validator Transaction

```bash
# Create validator
posd tx staking create-validator \\
  --amount=1000000000uomni \\
  --pubkey=$(posd tendermint show-validator) \\
  --moniker="my-validator" \\
  --chain-id=omniphi-1 \\
  --commission-rate="0.10" \\
  --commission-max-rate="0.20" \\
  --commission-max-change-rate="0.01" \\
  --min-self-delegation="1000000000" \\
  --gas="auto" \\
  --gas-adjustment=1.5 \\
  --gas-prices="0.025uomni" \\
  --from=my-validator-wallet \\
  --website="https://myvalidator.com" \\
  --details="Description of my validator" \\
  --identity="YOUR_KEYBASE_ID" \\
  --security-contact="security@myvalidator.com"

# You'll be prompted to confirm and sign
```

### 5. Verify Validator is Active

```bash
# Check validator info
posd query staking validator $(posd keys show my-validator-wallet --bech val -a)

# Check if validator is in active set
posd query staking validators | grep $(posd tendermint show-address)
```

---

## Monitoring & Maintenance

### Basic Monitoring

```bash
# Check node status
posd status

# Check latest block
posd status | jq .SyncInfo.latest_block_height

# Check peer count
posd status | jq .NodeInfo.n_peers

# Check validator info
posd query staking validator <your-valoper-address>

# Check missed blocks
posd query slashing signing-info $(posd tendermint show-validator)
```

### Prometheus Metrics

Enable in `app.toml`:
```toml
[telemetry]
enabled = true
prometheus-retention-time = 60
```

Metrics available at: `http://localhost:26660/metrics`

### Grafana Dashboards

Import pre-built dashboards:
- [Cosmos Validator Dashboard](https://grafana.com/grafana/dashboards/11036)
- [CometBFT Dashboard](https://grafana.com/grafana/dashboards/12648)

### Log Management

```bash
# View logs (systemd)
sudo journalctl -u posd -f

# View logs (manual)
tail -f ~/.omniphi/posd.log

# Rotate logs to prevent disk fill
sudo logrotate /etc/logrotate.d/posd
```

---

## Security Hardening

### 1. Firewall Configuration

```bash
# Allow P2P port
sudo ufw allow 26656/tcp

# Allow SSH (change default port!)
sudo ufw allow 2222/tcp

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status
```

### 2. Key Management

**CRITICAL:** Back up these files:
- `~/.omniphi/config/priv_validator_key.json` - Consensus private key
- Wallet mnemonic (24 words) - Your validator's operator key

```bash
# Backup consensus key (KEEP OFFLINE!)
cp ~/.omniphi/config/priv_validator_key.json /secure/backup/location/

# Verify permissions
chmod 600 ~/.omniphi/config/priv_validator_key.json
```

### 3. Sentry Node Architecture

For maximum security, use a sentry node architecture:
```
Internet → Sentry Nodes → (Private Network) → Validator Node
```

See [Sentry Node Setup](./security/SENTRY_NODES.md) for details.

### 4. DDoS Protection

```bash
# Install Fail2ban
sudo apt install fail2ban

# Configure for SSH
sudo cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
sudo systemctl enable fail2ban
sudo systemctl start fail2ban
```

### 5. Monitoring & Alerts

Set up alerts for:
- Node going offline
- Missed blocks > 10
- Disk space < 20%
- Memory usage > 80%
- High CPU usage
- P2P peers < 5

---

## Common Operations

### Unjail Validator

```bash
posd tx slashing unjail \\
  --from=my-validator-wallet \\
  --chain-id=omniphi-1 \\
  --gas=auto \\
  --gas-prices=0.025uomni
```

### Edit Validator

```bash
posd tx staking edit-validator \\
  --new-moniker="updated-name" \\
  --website="https://newsite.com" \\
  --details="Updated description" \\
  --from=my-validator-wallet \\
  --chain-id=omniphi-1 \\
  --gas=auto \\
  --gas-prices=0.025uomni
```

### Withdraw Rewards

```bash
# Withdraw all rewards
posd tx distribution withdraw-all-rewards \\
  --from=my-validator-wallet \\
  --chain-id=omniphi-1 \\
  --gas=auto \\
  --gas-prices=0.025uomni
```

### Delegate More Stake

```bash
posd tx staking delegate \\
  $(posd keys show my-validator-wallet --bech val -a) \\
  1000000uomni \\
  --from=my-validator-wallet \\
  --chain-id=omniphi-1 \\
  --gas=auto \\
  --gas-prices=0.025uomni
```

---

## Troubleshooting

### Node Won't Start

```bash
# Check logs for errors
sudo journalctl -u posd -n 100

# Verify genesis hash matches
posd validate-genesis

# Reset data (WARNING: deletes blockchain data)
posd tendermint unsafe-reset-all
```

### Not Syncing

```bash
# Check peers
posd status | jq .NodeInfo.n_peers

# If 0 peers, verify seeds in config.toml
# Try adding more persistent peers
```

### Out of Disk Space

```bash
# Check disk usage
df -h

# Enable pruning in app.toml
pruning = "custom"
pruning-keep-recent = "100"
pruning-interval = "10"

# Or use state sync to quickly catch up with less data
```

### High Memory Usage

```bash
# Restart node to clear memory
sudo systemctl restart posd

# Adjust cache settings in config.toml
```

---

## Next Steps

- **Security:** Read [Security Best Practices](./security/KEY_MANAGEMENT.md)
- **Monitoring:** Set up [Prometheus & Grafana](./operations/MONITORING.md)
- **Backups:** Configure [Automated Backups](./operations/BACKUPS.md)
- **State Sync:** Use [State Sync](./operations/STATE_SYNC.md) for fast catch-up
- **Upgrades:** Learn about [Chain Upgrades](./operations/UPGRADES.md)

---

## Support

- **Documentation:** https://docs.omniphi.io
- **Discord:** https://discord.gg/omniphi
- **GitHub:** https://github.com/omniphi/pos/issues
- **Validator Chat:** Join #validators channel on Discord

---

**Last Updated:** 2025-11-20 | **Version:** 1.0
