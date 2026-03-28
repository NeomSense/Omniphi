# Omniphi Validator Setup Guide

This guide walks you through setting up an Omniphi validator node from a bare Ubuntu server to a fully operational, production-grade validator. It covers the Go consensus chain (`posd`) and the Rust PoSeq sequencer node (`poseq-node`).

**Chain ID:** `omniphi-mainnet-1` (adjust for testnet)
**Bond denom:** `omniphi` (ticker: OMNI, 6 decimal places)
**Address format:** `omni` Bech32 prefix (SDK), `1x` hex prefix (display)
**Bech32 validator prefix:** `omnivaloper`

---

## Table of Contents

1. [Hardware Requirements](#hardware-requirements)
2. [Software Requirements](#software-requirements)
3. [Step-by-Step Setup](#step-by-step-setup)
4. [Systemd Service Files](#systemd-service-files)
5. [Security Hardening](#security-hardening)
6. [Monitoring Setup](#monitoring-setup)
7. [Common Operations](#common-operations)
8. [Staking Economics](#staking-economics)
9. [Troubleshooting](#troubleshooting)

---

## Hardware Requirements

### Minimum Specifications

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 4 cores (x86_64) | 8 cores (x86_64) |
| RAM | 16 GB | 32 GB |
| Storage | 500 GB NVMe SSD | 1 TB NVMe SSD |
| Network | 100 Mbps symmetric | 1 Gbps symmetric |
| OS | Ubuntu 22.04 LTS | Ubuntu 22.04 LTS |

### Cloud Provider Equivalents

| Provider | Minimum | Recommended |
|----------|---------|-------------|
| AWS | `t3.xlarge` (4 vCPU, 16 GB) | `m5.2xlarge` (8 vCPU, 32 GB) |
| GCP | `e2-standard-4` (4 vCPU, 16 GB) | `n2-standard-8` (8 vCPU, 32 GB) |
| Azure | `Standard_D4s_v3` (4 vCPU, 16 GB) | `Standard_D8s_v3` (8 vCPU, 32 GB) |
| Hetzner | `CPX31` (4 vCPU, 8 GB) + upgrade | `CPX51` (8 vCPU, 32 GB) |

**Estimated monthly cost:** $150-$400 depending on provider and region.

### Storage Notes

- Use NVMe SSD exclusively. SATA SSDs and HDDs cause consensus timeouts.
- Plan for ~1 GB/day of chain data growth (pruned node) or ~5 GB/day (archive node).
- The PoSeq node requires an additional ~10 GB for its sled database.
- Enable TRIM/discard for SSD longevity.

---

## Software Requirements

| Software | Version | Purpose |
|----------|---------|---------|
| Ubuntu | 22.04 LTS | Operating system |
| Go | 1.24+ | Build `posd` consensus binary |
| Rust | 1.75+ | Build `poseq-node` sequencer binary |
| CometBFT | 0.38.17 | Consensus engine (bundled with SDK) |
| Git | 2.x | Clone source repository |
| jq | 1.6+ | JSON manipulation for setup scripts |
| make | 4.x | Build orchestration |

---

## Step-by-Step Setup

### Step 1: Install System Dependencies

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install build essentials
sudo apt install -y \
    build-essential \
    curl \
    wget \
    git \
    jq \
    lz4 \
    unzip \
    pkg-config \
    libssl-dev \
    clang \
    cmake \
    ufw \
    fail2ban
```

### Step 2: Install Go

```bash
# Download Go 1.24 (match go.mod: go 1.24.0)
GO_VERSION=1.24.0
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz

# Remove any existing Go installation and extract
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
rm go${GO_VERSION}.linux-amd64.tar.gz

# Add to PATH (append to ~/.bashrc)
cat >> ~/.bashrc << 'GOEOF'
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
GOEOF

source ~/.bashrc

# Verify
go version
# Expected: go version go1.24.0 linux/amd64
```

### Step 3: Install Rust

```bash
# Install Rust via rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
source $HOME/.cargo/env

# Verify
rustc --version
# Expected: rustc 1.75.0 or later
cargo --version
```

### Step 4: Clone and Build from Source

```bash
# Clone the repository
git clone https://github.com/omniphi/omniphi.git ~/omniphi
cd ~/omniphi

# Checkout the latest stable release tag
git fetch --tags
LATEST_TAG=$(git describe --tags --abbrev=0)
git checkout $LATEST_TAG
echo "Building version: $LATEST_TAG"

# Build the Go consensus node (posd)
cd chain
go build -o $GOPATH/bin/posd ./cmd/posd
cd ..

# Verify posd
posd version

# Build the Rust PoSeq node
cd poseq
cargo build --release
sudo cp target/release/poseq-node /usr/local/bin/
cd ..

# Verify poseq-node
poseq-node --help
```

### Step 5: Initialize the Node

```bash
# Set your moniker (validator display name)
MONIKER="my-validator"

# Initialize the node — home directory is ~/.pos (not ~/.posd)
posd init "$MONIKER" --chain-id omniphi-mainnet-1

# This creates:
#   ~/.pos/config/config.toml     — CometBFT configuration
#   ~/.pos/config/app.toml        — Application configuration
#   ~/.pos/config/genesis.json    — Genesis file (placeholder)
#   ~/.pos/config/node_key.json   — Node identity key
#   ~/.pos/config/priv_validator_key.json — Validator signing key
#   ~/.pos/data/                  — Chain data directory
```

### Step 6: Configure config.toml

Edit `~/.pos/config/config.toml` with the following critical settings:

```bash
# Open for editing
nano ~/.pos/config/config.toml
```

Apply these changes:

```toml
#######################################################################
###                      P2P Configuration                          ###
#######################################################################
[p2p]

# Seed nodes — official Omniphi seed nodes
seeds = "seed1-node-id@seed1.omniphi.io:26656,seed2-node-id@seed2.omniphi.io:26656"

# Persistent peers — reliable peers you want to always stay connected to
# Get current peers from https://omniphi.io/peers or the Discord #validators channel
persistent_peers = "peer1-node-id@peer1.omniphi.io:26656,peer2-node-id@peer2.omniphi.io:26656"

# Listen address — bind to all interfaces for P2P
laddr = "tcp://0.0.0.0:26656"

# Maximum number of inbound + outbound peers
max_num_inbound_peers = 40
max_num_outbound_peers = 10

# Peer exchange — enable to discover new peers
pex = true

# Address book — persist peer addresses across restarts
addr_book_strict = true

#######################################################################
###                      RPC Configuration                          ###
#######################################################################
[rpc]

# IMPORTANT: Do NOT expose RPC publicly on a validator.
# Bind to localhost only.
laddr = "tcp://127.0.0.1:26657"

#######################################################################
###                  State Sync Configuration                       ###
#######################################################################
[statesync]

# Enable state sync to catch up quickly instead of replaying all blocks
enable = true

# Get current values from: https://omniphi.io/statesync
# Or query a trusted RPC node:
#   curl -s http://trusted-rpc:26657/block | jq -r '.result.block.header.height'
#   curl -s http://trusted-rpc:26657/block?height=<HEIGHT> | jq -r '.result.block_id.hash'
rpc_servers = "https://rpc1.omniphi.io:443,https://rpc2.omniphi.io:443"
trust_height = 0
trust_hash = ""
trust_period = "168h0m0s"

#######################################################################
###                    Mempool Configuration                        ###
#######################################################################
[mempool]

# Set mempool size appropriate for validator load
max_txs_bytes = 1073741824
max_tx_bytes = 1048576
size = 5000

#######################################################################
###                   Consensus Configuration                       ###
#######################################################################
[consensus]

# Timeout settings — defaults are generally fine
timeout_propose = "3s"
timeout_prevote = "1s"
timeout_precommit = "1s"
timeout_commit = "5s"

#######################################################################
###                  Instrumentation (Prometheus)                    ###
#######################################################################
[instrumentation]

# Enable Prometheus metrics
prometheus = true
prometheus_listen_addr = "127.0.0.1:26660"
```

### Step 7: Configure app.toml

Edit `~/.pos/config/app.toml`:

```bash
nano ~/.pos/config/app.toml
```

Apply these changes:

```toml
#######################################################################
###                      Base Configuration                         ###
#######################################################################

# Minimum gas price — REQUIRED for validators
# Reject transactions below this fee to prevent spam
minimum-gas-prices = "0.01omniphi"

# Pruning — "default" is fine for validators, use "nothing" for archive nodes
pruning = "custom"
pruning-keep-recent = "100"
pruning-interval = "10"

#######################################################################
###                      API Configuration                          ###
#######################################################################
[api]

# REST API — bind to localhost only on validators
enable = true
address = "tcp://127.0.0.1:1317"
swagger = false

# Maximum open connections
max-open-connections = 1000

#######################################################################
###                      gRPC Configuration                         ###
#######################################################################
[grpc]

# gRPC server — bind to localhost only on validators
enable = true
address = "127.0.0.1:9090"

#######################################################################
###                    gRPC-Web Configuration                       ###
#######################################################################
[grpc-web]

# Disable gRPC-web on validators (use on full nodes only)
enable = false
```

### Step 8: Download the Genesis File

```bash
# Download the official genesis file
# Replace URL with the actual genesis file location
wget -O ~/.pos/config/genesis.json https://raw.githubusercontent.com/omniphi/networks/main/omniphi-mainnet-1/genesis.json

# Verify the genesis hash
sha256sum ~/.pos/config/genesis.json
# Expected: <official_genesis_hash>  (check Discord/docs for the official hash)

# Validate the genesis file
posd genesis validate ~/.pos/config/genesis.json
```

**Important genesis notes:**
- The `feemarket` module's `treasury_address` must be set in genesis, or the chain will fail on block 1.
- The `guard` module field `timelock_integration_enabled` defaults to `false` on genesis and requires a governance proposal to enable.

### Step 9: Configure the PoSeq Node

Create the PoSeq configuration file:

```bash
mkdir -p ~/.pos/poseq
```

Create `~/.pos/poseq/poseq.toml`:

```toml
[node]
# 32-byte node identity as 64 hex chars (generate with: openssl rand -hex 32)
id = "YOUR_64_HEX_CHAR_NODE_ID"

# TCP listen address for PoSeq P2P gossip
listen_addr = "0.0.0.0:7001"

# PoSeq peer nodes
peers = ["peer1.omniphi.io:7001", "peer2.omniphi.io:7001"]

# Quorum threshold (number of attestors to finalize a batch)
quorum_threshold = 3

# Slot duration in milliseconds (5 seconds for mainnet)
slot_duration_ms = 5000

# Data directory for durable storage (sled)
data_dir = "/home/validator/.pos/poseq/data"

# Node role: "leader", "attestor", or "observer"
role = "attestor"

# Signing key seed — 64 hex chars. KEEP THIS SECRET.
# Generate with: openssl rand -hex 32
key_seed = "YOUR_64_HEX_CHAR_KEY_SEED"

# Prometheus metrics endpoint
metrics_addr = "127.0.0.1:9191"

# Bootstrap seed peers for peer discovery
seed_peers = ["seed1.omniphi.io:7000"]

# Slots per epoch (10 slots per epoch default)
slots_per_epoch = 10

[policy]
# Ordering, batch, and class policies use defaults if omitted
```

### Step 10: Start the Node Services

Start both services (see [Systemd Service Files](#systemd-service-files) below for production setup):

```bash
# Start the consensus node first
sudo systemctl enable omniphi-node
sudo systemctl start omniphi-node

# Monitor the sync progress
journalctl -u omniphi-node -f

# Wait until the node is fully synced before proceeding
# Check sync status:
posd status 2>&1 | jq '.SyncInfo.catching_up'
# Must return "false" before creating the validator
```

### Step 11: Create the Validator Key

```bash
# Create or recover a key
# Option A: Generate a new key
posd keys add validator --keyring-backend file

# Option B: Recover from mnemonic
posd keys add validator --keyring-backend file --recover

# IMPORTANT: Write down the mnemonic phrase and store it securely offline.
# This is the ONLY way to recover your validator key.

# Verify the key was created
posd keys show validator --keyring-backend file -a
# Output: omni1... (your operator address)

# Fund the account
# Transfer OMNI tokens to the address shown above
```

### Step 12: Submit the Create-Validator Transaction

Wait until the node is fully synced (`catching_up: false`), then:

```bash
# Create the validator
posd tx staking create-validator \
    --amount=1000000000000omniphi \
    --pubkey=$(posd tendermint show-validator) \
    --moniker="My Validator" \
    --website="https://myvalidator.com" \
    --details="Professional validator operator" \
    --security-contact="security@myvalidator.com" \
    --chain-id=omniphi-mainnet-1 \
    --commission-rate="0.10" \
    --commission-max-rate="0.20" \
    --commission-max-change-rate="0.01" \
    --min-self-delegation="1000000" \
    --gas="auto" \
    --gas-adjustment=1.5 \
    --gas-prices="0.01omniphi" \
    --from=validator \
    --keyring-backend=file \
    -y

# Note on amounts: 1 OMNI = 1,000,000 uOMNI (6 decimals)
# The example above stakes 1,000,000 OMNI (1000000 * 10^6 base units)
```

### Step 13: Verify the Validator is Active

```bash
# Check your validator is in the active set
posd query staking validator $(posd keys show validator --keyring-backend file --bech val -a)

# Check your validator's signing status
posd query slashing signing-info $(posd tendermint show-validator)

# Verify you are producing blocks — look for your moniker in the proposer list
posd status 2>&1 | jq '.ValidatorInfo'
```

---

## Systemd Service Files

### omniphi-node.service (Go Consensus Node)

Create `/etc/systemd/system/omniphi-node.service`:

```ini
[Unit]
Description=Omniphi Consensus Node (posd)
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
User=validator
Group=validator
Type=simple
Environment="HOME=/home/validator"
Environment="GOPATH=/home/validator/go"
Environment="PATH=/usr/local/go/bin:/home/validator/go/bin:/usr/local/bin:/usr/bin:/bin"
ExecStart=/home/validator/go/bin/posd start --home /home/validator/.pos
Restart=on-failure
RestartSec=10
LimitNOFILE=65535
LimitNPROC=65535
StandardOutput=journal
StandardError=journal
SyslogIdentifier=omniphi-node

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/validator/.pos
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### omniphi-poseq.service (Rust PoSeq Node)

Create `/etc/systemd/system/omniphi-poseq.service`:

```ini
[Unit]
Description=Omniphi PoSeq Sequencer Node
After=network-online.target omniphi-node.service
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
User=validator
Group=validator
Type=simple
ExecStart=/usr/local/bin/poseq-node --config /home/validator/.pos/poseq/poseq.toml
Restart=on-failure
RestartSec=10
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal
SyslogIdentifier=omniphi-poseq

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/validator/.pos/poseq
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Enable and Start Services

```bash
# Create the validator system user (if not using your own user)
sudo useradd -m -s /bin/bash validator

# Set permissions
sudo chown -R validator:validator /home/validator/.pos

# Reload systemd, enable, and start
sudo systemctl daemon-reload
sudo systemctl enable omniphi-node omniphi-poseq
sudo systemctl start omniphi-node

# Wait for sync, then start PoSeq
sudo systemctl start omniphi-poseq

# Check status
sudo systemctl status omniphi-node
sudo systemctl status omniphi-poseq
```

---

## Security Hardening

### Firewall Configuration (UFW)

```bash
# Reset UFW to defaults
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (change 22 to your custom SSH port if applicable)
sudo ufw allow 22/tcp comment 'SSH'

# Allow CometBFT P2P — this is the ONLY port exposed publicly on a validator
sudo ufw allow 26656/tcp comment 'CometBFT P2P'

# Allow PoSeq P2P (if running PoSeq node)
sudo ufw allow 7001/tcp comment 'PoSeq P2P'

# DO NOT expose these ports publicly on a validator:
# 26657 — CometBFT RPC (should be localhost only)
# 1317  — REST API (should be localhost only)
# 9090  — gRPC (should be localhost only)
# 26660 — Prometheus metrics (should be localhost only)
# 9191  — PoSeq Prometheus metrics (should be localhost only)

# Enable UFW
sudo ufw enable
sudo ufw status verbose
```

### SSH Hardening

Edit `/etc/ssh/sshd_config`:

```bash
sudo nano /etc/ssh/sshd_config
```

Recommended settings:

```
# Disable password authentication — use key-based auth only
PasswordAuthentication no
ChallengeResponseAuthentication no
PubkeyAuthentication yes

# Disable root login
PermitRootLogin no

# Limit to the validator user
AllowUsers validator

# Change default port (optional but recommended)
Port 2222

# Connection limits
MaxAuthTries 3
MaxSessions 2
LoginGraceTime 30

# Idle timeout
ClientAliveInterval 300
ClientAliveCountMax 2
```

```bash
# Restart SSH
sudo systemctl restart sshd
```

### Fail2Ban Configuration

```bash
# Install and enable
sudo apt install -y fail2ban
sudo systemctl enable fail2ban

# Create local config
sudo tee /etc/fail2ban/jail.local << 'EOF'
[sshd]
enabled = true
port = 2222
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 3600
findtime = 600
EOF

sudo systemctl restart fail2ban
```

### Sentry Node Architecture

For production validators, deploy sentry nodes to shield the validator from direct exposure to the public P2P network.

```
                    Public Internet
                         |
              +----------+----------+
              |                     |
        +-----------+        +-----------+
        | Sentry A  |        | Sentry B  |
        | (Full Node|        | (Full Node|
        | Public IP)|        | Public IP)|
        | :26656    |        | :26656    |
        +-----------+        +-----------+
              |                     |
              +----------+----------+
                         |
                   Private Network
                         |
                  +-------------+
                  |  Validator  |
                  | (NO public  |
                  |  P2P port)  |
                  +-------------+
```

**Validator config.toml (behind sentries):**

```toml
[p2p]
# Only connect to your sentry nodes — NO public peers
persistent_peers = "sentry-a-node-id@10.0.1.10:26656,sentry-b-node-id@10.0.1.11:26656"

# Disable peer exchange — validator must not be discoverable
pex = false

# Do not advertise the validator's address
addr_book_strict = false
```

**Sentry node config.toml:**

```toml
[p2p]
# Connect to the validator via private network
persistent_peers = "validator-node-id@10.0.1.5:26656"
private_peer_ids = "validator-node-id"

# Enable peer exchange for public peers
pex = true

# Connect to seeds and public peers normally
seeds = "seed1-node-id@seed1.omniphi.io:26656,seed2-node-id@seed2.omniphi.io:26656"
```

### Key Backup Procedures

```bash
# Critical files to back up (store offline and encrypted):

# 1. Validator signing key — loss means losing your validator
cp ~/.pos/config/priv_validator_key.json /secure-backup/

# 2. Node key — identifies your node on the P2P network
cp ~/.pos/config/node_key.json /secure-backup/

# 3. Keyring — contains your operator account key
cp -r ~/.pos/keyring-file/ /secure-backup/

# 4. PoSeq key seed — from poseq.toml, the key_seed field
# Store this 64-char hex string in your password manager

# Encrypt the backup
tar czf validator-keys-backup.tar.gz /secure-backup/
gpg --symmetric --cipher-algo AES256 validator-keys-backup.tar.gz
# Store the .gpg file in multiple secure locations
rm validator-keys-backup.tar.gz

# WARNING: Never run two nodes with the same priv_validator_key.json simultaneously.
# This WILL result in double-signing and a 5% slash of your staked tokens.
```

---

## Monitoring Setup

### Install Prometheus Node Exporter

```bash
# Download and install node_exporter
NODE_EXPORTER_VERSION="1.7.0"
wget https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz
tar xzf node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz
sudo cp node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter /usr/local/bin/
rm -rf node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64*

# Create systemd service
sudo tee /etc/systemd/system/node-exporter.service << 'EOF'
[Unit]
Description=Prometheus Node Exporter
After=network.target

[Service]
User=validator
Type=simple
ExecStart=/usr/local/bin/node_exporter --web.listen-address="127.0.0.1:9100"
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable node-exporter
sudo systemctl start node-exporter
```

### Prometheus Configuration

On your monitoring server, add these scrape targets to `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  # CometBFT consensus metrics
  - job_name: 'omniphi-consensus'
    static_configs:
      - targets: ['validator-ip:26660']
        labels:
          instance: 'my-validator'
          chain: 'omniphi-mainnet-1'

  # PoSeq node metrics
  - job_name: 'omniphi-poseq'
    static_configs:
      - targets: ['validator-ip:9191']
        labels:
          instance: 'my-validator'
          component: 'poseq'

  # System metrics
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['validator-ip:9100']
        labels:
          instance: 'my-validator'

rule_files:
  - 'omniphi-alerts.yml'
```

### Alert Rules

Create `omniphi-alerts.yml` on your Prometheus server:

```yaml
groups:
  - name: omniphi-validator
    rules:
      # Validator is not signing blocks — CRITICAL
      - alert: ValidatorMissedBlocks
        expr: increase(cometbft_consensus_validator_missed_blocks[5m]) > 10
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Validator missing blocks"
          description: "Validator has missed {{ $value }} blocks in the last 5 minutes. Risk of jailing."

      # Node is catching up (not synced)
      - alert: NodeCatchingUp
        expr: cometbft_consensus_catching_up == 1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Node is catching up"
          description: "Node has been syncing for more than 10 minutes."

      # Node is down
      - alert: NodeDown
        expr: up{job="omniphi-consensus"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Omniphi node is down"
          description: "Cannot reach the consensus node metrics endpoint."

      # Low peer count
      - alert: LowPeerCount
        expr: cometbft_p2p_peers < 3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low P2P peer count"
          description: "Node has only {{ $value }} peers. Minimum recommended: 5."

      # High memory usage
      - alert: HighMemoryUsage
        expr: (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is above 90%."

      # Disk space running low
      - alert: DiskSpaceLow
        expr: (1 - node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) > 0.85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Disk space running low"
          description: "Disk usage is above 85%."

      # PoSeq node down
      - alert: PoSeqNodeDown
        expr: up{job="omniphi-poseq"} == 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "PoSeq node is down"
          description: "Cannot reach the PoSeq metrics endpoint."

      # Consensus round taking too long
      - alert: ConsensusRoundTooLong
        expr: cometbft_consensus_round > 1
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "Consensus stuck in high round"
          description: "Consensus has been in round {{ $value }} for 3+ minutes."
```

### Grafana Dashboard

Import the CometBFT dashboard or create a custom one with these key panels:

```
Dashboard panels:
  Row 1 — Overview:
    - Current block height (cometbft_consensus_height)
    - Number of peers (cometbft_p2p_peers)
    - Catching up status (cometbft_consensus_catching_up)
    - Time since last block

  Row 2 — Validator Performance:
    - Missed blocks (cometbft_consensus_validator_missed_blocks)
    - Validator power (cometbft_consensus_validator_power)
    - Consensus round (cometbft_consensus_round)
    - Block time (rate(cometbft_consensus_height[5m]))

  Row 3 — PoSeq Metrics:
    - PoSeq current epoch
    - Batches finalized (rate)
    - Attestation success rate
    - PoSeq peer count

  Row 4 — System Resources:
    - CPU usage (node_exporter)
    - Memory usage (node_exporter)
    - Disk I/O (node_exporter)
    - Network throughput (node_exporter)
```

---

## Common Operations

### Check Sync Status

```bash
# Quick sync check
posd status 2>&1 | jq '{
    latest_block_height: .SyncInfo.latest_block_height,
    latest_block_time: .SyncInfo.latest_block_time,
    catching_up: .SyncInfo.catching_up
}'

# Detailed node info
posd status 2>&1 | jq '.NodeInfo'
```

### Unjail Validator

If your validator gets jailed for downtime (missing too many blocks):

```bash
# Check if jailed
posd query staking validator $(posd keys show validator --keyring-backend file --bech val -a) \
    | grep jailed

# Wait for the jail duration to pass (typically 10 minutes for downtime)
# Then unjail:
posd tx slashing unjail \
    --from validator \
    --keyring-backend file \
    --chain-id omniphi-mainnet-1 \
    --gas auto \
    --gas-adjustment 1.5 \
    --gas-prices 0.01omniphi \
    -y
```

### Edit Validator Information

```bash
# Update commission rate (limited by commission-max-change-rate)
posd tx staking edit-validator \
    --commission-rate 0.08 \
    --from validator \
    --keyring-backend file \
    --chain-id omniphi-mainnet-1 \
    --gas auto \
    --gas-prices 0.01omniphi \
    -y

# Update validator description
posd tx staking edit-validator \
    --moniker "New Moniker" \
    --website "https://newsite.com" \
    --details "Updated validator description" \
    --security-contact "new-email@example.com" \
    --from validator \
    --keyring-backend file \
    --chain-id omniphi-mainnet-1 \
    --gas auto \
    --gas-prices 0.01omniphi \
    -y
```

### Graceful Upgrade Procedure

When a chain upgrade is scheduled via governance:

```bash
# 1. Monitor for the upgrade height
posd query upgrade plan

# 2. When the node halts at the upgrade height, it will log:
#    "UPGRADE NEEDED at height: XXXX: <plan_name>"

# 3. Build the new binary
cd ~/omniphi
git fetch --tags
git checkout <new-version-tag>
cd chain
go build -o $GOPATH/bin/posd ./cmd/posd
cd ../poseq
cargo build --release
sudo cp target/release/poseq-node /usr/local/bin/

# 4. Verify the new version
posd version

# 5. Restart the service — it will resume from the upgrade height
sudo systemctl restart omniphi-node
sudo systemctl restart omniphi-poseq

# 6. Monitor logs for successful restart
journalctl -u omniphi-node -f
```

### Key Migration Between Machines

```bash
# On the OLD machine — stop the validator first!
sudo systemctl stop omniphi-node omniphi-poseq

# Copy critical files to the new machine
scp ~/.pos/config/priv_validator_key.json new-machine:~/.pos/config/
scp ~/.pos/config/node_key.json new-machine:~/.pos/config/
scp -r ~/.pos/keyring-file/ new-machine:~/.pos/

# WARNING: Ensure the old node is FULLY STOPPED before starting the new one.
# Running two nodes with the same validator key causes double-signing (5% slash).

# On the NEW machine
sudo systemctl start omniphi-node
# Verify it's signing blocks:
journalctl -u omniphi-node -f | grep "committed state"
```

### Emergency: Stop Node and Backup State

```bash
# Graceful stop
sudo systemctl stop omniphi-poseq
sudo systemctl stop omniphi-node

# Backup current state
BACKUP_DIR="/backup/omniphi-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Back up validator keys (critical)
cp ~/.pos/config/priv_validator_key.json "$BACKUP_DIR/"
cp ~/.pos/config/node_key.json "$BACKUP_DIR/"
cp -r ~/.pos/keyring-file/ "$BACKUP_DIR/"

# Back up priv_validator_state.json (tracks last signed block to prevent double-sign)
cp ~/.pos/data/priv_validator_state.json "$BACKUP_DIR/"

# Optional: backup full data directory (large)
# tar czf "$BACKUP_DIR/data.tar.gz" ~/.pos/data/

echo "Backup saved to $BACKUP_DIR"
```

### Query Guard Module Status

```bash
# Check guard module parameters
posd query guard params

# Check risk report for a proposal
posd query guard risk-report <proposal-id>

# Check queued execution status
posd query guard queued-execution <proposal-id>

# Check advisory links for a proposal
posd query guard advisory-link <proposal-id>

# Submit an advisory link (validators only)
posd tx guard submit-advisory-link <proposal-id> <sha256-report-hash> <report-url> \
    --from validator \
    --keyring-backend file \
    --chain-id omniphi-mainnet-1 \
    --gas auto \
    --gas-prices 0.01omniphi \
    -y
```

---

## Staking Economics

### Token Information

| Parameter | Value |
|-----------|-------|
| Token name | OMNI |
| Base denom | `omniphi` |
| Decimals | 6 |
| Total supply cap | 1,500,000,000 OMNI (1.5B) |
| Genesis supply | 375,000,000 OMNI (375M) |
| Maximum inflation | 3% annually (protocol hard cap) |
| Minimum staking emission share | 20% (protocol enforced) |

### Commission

| Parameter | Range |
|-----------|-------|
| Commission rate | 5% - 20% typical |
| Max commission rate | Set at validator creation (immutable) |
| Max commission change rate | Set at validator creation (immutable) |
| Minimum self-delegation | 1 OMNI (1,000,000 base units) |

### Unbonding

| Parameter | Value |
|-----------|-------|
| Unbonding period | 21 days |
| Redelegation | Instant, but cannot redelegate the same tokens for 21 days |

### Slashing

| Infraction | Penalty | Jail Duration |
|------------|---------|---------------|
| Downtime (missed blocks) | 0.01% of stake | 10 minutes |
| Double signing | 5% of stake | Permanent (tombstoned) |

### Reward Sources

Validator rewards come from multiple streams in the Omniphi protocol:

| Source | Share | Description |
|--------|-------|-------------|
| Staking inflation | 40% of emissions | Standard PoS block rewards |
| PoC participation bonus | 30% of emissions | Proof of Contribution rewards |
| Sequencer rewards | 20% of emissions | PoSeq sequencing rewards |
| Treasury | 10% of emissions | Protocol treasury |

### Fee Economics

- 90% of transaction fees are burned (deflationary pressure)
- 10% of transaction fees go to the treasury
- Minimum gas price: 0.01 OMNI
- Adaptive burn controller can adjust burn ratio between 80%-95% based on network congestion

### RewardMult Module

The RewardMult module adjusts each validator's effective reward multiplier based on their participation quality:

| Parameter | Value |
|-----------|-------|
| Multiplier range | 0.85x - 1.15x |
| EMA smoothing window | 8 epochs |
| Uptime bonus (>99.9%) | +0.03 |
| Uptime bonus (>99.5%) | +0.015 |
| Max PoC participation bonus | +0.01 |
| Max quality bonus (PoC metrics) | +0.02 |
| Downtime slash penalty | -0.05 |
| Double-sign penalty | -0.10 |
| Fraud penalty (PoR) | -0.10 |

The multiplier is budget-neutral: the sum of all validators' adjusted rewards equals the total emission allocation. High-performing validators earn more at the expense of underperformers.

---

## Troubleshooting

### "validator not in active set"

**Symptoms:** Your validator was created but is not signing blocks.

**Causes and fixes:**
1. **Insufficient stake:** Your total delegated amount is below the active set threshold. Check the minimum stake in the active set:
   ```bash
   posd query staking validators --limit 200 -o json | \
       jq '[.validators[] | select(.status=="BOND_STATUS_BONDED")] | sort_by(.tokens | tonumber) | .[0].tokens'
   ```
   Delegate more tokens to meet or exceed this threshold.

2. **Validator is jailed:** Check and unjail if applicable:
   ```bash
   posd query staking validator $(posd keys show validator --keyring-backend file --bech val -a) \
       -o json | jq '.jailed'
   ```

3. **Active set is full:** The chain has a maximum validator count. You need more stake than the lowest-staked active validator.

### "node is catching up"

**Symptoms:** `catching_up: true` in `posd status` and the node is not producing or validating blocks.

**Fixes:**
1. **Enable state sync** (fastest method) — see [Step 6](#step-6-configure-configtoml).
2. **Increase peers** — add more persistent peers from the community peer list.
3. **Check network** — ensure port 26656 is accessible and bandwidth is sufficient.
4. **Check disk I/O** — slow disks cause the node to fall behind:
   ```bash
   iostat -x 1 5  # Check %util and await
   ```
5. **Download a snapshot** — if available from community snapshot providers:
   ```bash
   # Stop the node
   sudo systemctl stop omniphi-node
   # Remove old data (keep priv_validator_state.json!)
   cp ~/.pos/data/priv_validator_state.json /tmp/
   rm -rf ~/.pos/data
   # Extract snapshot
   lz4 -d omniphi-snapshot.tar.lz4 | tar xf - -C ~/.pos/
   cp /tmp/priv_validator_state.json ~/.pos/data/
   # Restart
   sudo systemctl start omniphi-node
   ```

### "consensus failure"

**Symptoms:** Node panics or logs `CONSENSUS FAILURE` and halts.

**Fixes:**
1. **Check genesis hash matches:** The genesis file must match exactly:
   ```bash
   sha256sum ~/.pos/config/genesis.json
   # Compare with the official hash
   ```
2. **Check binary version:** Ensure you are running the correct version for the current chain height:
   ```bash
   posd version --long
   ```
3. **Check for upgrade:** If the chain passed an upgrade height, you need the new binary:
   ```bash
   posd query upgrade plan
   ```
4. **Check feemarket treasury_address:** On testnet/devnet, ensure the `treasury_address` is set correctly in genesis. An empty treasury address causes consensus failure on block 1.

### "out of memory"

**Symptoms:** Node crashes with OOM killer or `signal: killed` in logs.

**Fixes:**
1. **Reduce max-open-connections** in `app.toml`:
   ```toml
   [api]
   max-open-connections = 500  # reduce from 1000
   ```
2. **Enable aggressive pruning** in `app.toml`:
   ```toml
   pruning = "custom"
   pruning-keep-recent = "100"
   pruning-interval = "10"
   ```
3. **Reduce mempool size** in `config.toml`:
   ```toml
   [mempool]
   size = 2000  # reduce from 5000
   max_txs_bytes = 536870912  # 512MB instead of 1GB
   ```
4. **Add swap space** as a safety net:
   ```bash
   sudo fallocate -l 8G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
   ```
5. **Upgrade to 32 GB RAM** — if running minimum specs, the recommended 32 GB is needed for long-term operation.

### "AppHash mismatch"

**Symptoms:** Node logs `appHash mismatch` and halts.

**Fixes:**
1. This means your node's state diverged from the network. Usually caused by running a wrong binary version.
2. **Reset and resync:**
   ```bash
   sudo systemctl stop omniphi-node
   cp ~/.pos/data/priv_validator_state.json /tmp/
   posd tendermint unsafe-reset-all --home ~/.pos
   cp /tmp/priv_validator_state.json ~/.pos/data/
   sudo systemctl start omniphi-node
   ```

### PoSeq Node Issues

**"connection refused" on PoSeq peers:**
- Verify firewall allows port 7001 (TCP).
- Verify peer addresses in `poseq.toml` are correct.
- Check that peers are running: `curl http://peer-ip:9191/healthz`

**"quorum not reached" in PoSeq logs:**
- Ensure enough PoSeq nodes are online to meet `quorum_threshold`.
- Check peer connectivity in metrics at `http://127.0.0.1:9191/status`.

---

## Appendix: Port Reference

| Port | Protocol | Service | Exposure |
|------|----------|---------|----------|
| 26656 | TCP | CometBFT P2P | Public (required) |
| 26657 | TCP | CometBFT RPC | Localhost only |
| 26660 | TCP | CometBFT Prometheus | Localhost only |
| 1317 | TCP | REST API | Localhost only |
| 9090 | TCP | gRPC | Localhost only |
| 7001 | TCP | PoSeq P2P | Public (if running PoSeq) |
| 9191 | TCP | PoSeq Prometheus / Health / Status | Localhost only |
| 9100 | TCP | Node Exporter | Localhost only |

---

## Appendix: Useful Aliases

Add these to `~/.bashrc` for convenience:

```bash
# Omniphi validator aliases
alias omni-status='posd status 2>&1 | jq .'
alias omni-sync='posd status 2>&1 | jq ".SyncInfo"'
alias omni-val='posd query staking validator $(posd keys show validator --keyring-backend file --bech val -a)'
alias omni-peers='curl -s http://127.0.0.1:26657/net_info | jq ".result.n_peers"'
alias omni-height='posd status 2>&1 | jq -r ".SyncInfo.latest_block_height"'
alias omni-logs='journalctl -u omniphi-node -f'
alias omni-poseq-logs='journalctl -u omniphi-poseq -f'
alias omni-restart='sudo systemctl restart omniphi-node && sudo systemctl restart omniphi-poseq'
```
