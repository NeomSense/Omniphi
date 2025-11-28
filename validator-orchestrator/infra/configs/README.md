# Omniphi Validator Configuration Templates

**Production-ready configuration templates for Omniphi validators.**

---

## Overview

This directory contains configuration templates and guides for setting up and operating Omniphi validator nodes. All templates follow best practices for security, performance, and reliability.

---

## Configuration Files

### Core Configuration Templates

| File | Description | Copy To |
|------|-------------|---------|
| **[config.toml.template](config.toml.template)** | CometBFT consensus configuration | `~/.omniphi/config/config.toml` |
| **[app.toml.template](app.toml.template)** | Cosmos SDK application settings | `~/.omniphi/config/app.toml` |
| **[client.toml.template](client.toml.template)** | CLI client defaults | `~/.omniphi/config/client.toml` |

### Documentation & Guides

| File | Description |
|------|-------------|
| **[PRUNING_STRATEGIES.md](PRUNING_STRATEGIES.md)** | Disk space management strategies |
| **[PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md)** | Monitoring setup with Prometheus & Grafana |

---

## Quick Start

### 1. Initialize Validator Node

```bash
# Initialize node (creates ~/.omniphi directory)
posd init my-validator --chain-id omniphi-1

# This creates default config files:
# ~/.omniphi/config/config.toml
# ~/.omniphi/config/app.toml
# ~/.omniphi/config/client.toml
```

### 2. Apply Production Templates

```bash
# Backup default configs
cp ~/.omniphi/config/config.toml ~/.omniphi/config/config.toml.backup
cp ~/.omniphi/config/app.toml ~/.omniphi/config/app.toml.backup
cp ~/.omniphi/config/client.toml ~/.omniphi/config/client.toml.backup

# Copy production templates
cp config.toml.template ~/.omniphi/config/config.toml
cp app.toml.template ~/.omniphi/config/app.toml
cp client.toml.template ~/.omniphi/config/client.toml
```

### 3. Customize for Your Deployment

**Edit `~/.omniphi/config/config.toml`:**
```bash
nano ~/.omniphi/config/config.toml

# REQUIRED changes:
# - moniker = "CHANGE_ME"  → Your validator name
# - persistent_peers = ""  → Add peer addresses
# - external_address = ""  → Your public IP:26656 (if behind NAT)
```

**Edit `~/.omniphi/config/app.toml`:**
```bash
nano ~/.omniphi/config/app.toml

# Review and adjust:
# - minimum-gas-prices
# - pruning strategy
# - API/gRPC settings
```

**Edit `~/.omniphi/config/client.toml`:**
```bash
nano ~/.omniphi/config/client.toml

# REQUIRED changes:
# - chain-id = "omniphi-1"  → Confirm correct network
# - keyring-backend = "file"  → Choose secure keyring
```

### 4. Start Validator

```bash
# With systemd (recommended):
sudo systemctl start posd

# Or directly:
posd start
```

---

## Configuration Breakdown

### config.toml (CometBFT Settings)

**Key sections:**

#### Network & Peers
```toml
[p2p]
laddr = "tcp://0.0.0.0:26656"        # P2P listen address
external_address = "1.2.3.4:26656"   # Public IP for NAT
persistent_peers = "id@host:port,..."  # Reliable peers
seeds = ""                            # Seed nodes (optional)
```

#### RPC Configuration
```toml
[rpc]
laddr = "tcp://127.0.0.1:26657"      # RPC listen (localhost only for security)
cors_allowed_origins = []             # Disable CORS in production
```

#### Consensus Settings
```toml
[consensus]
timeout_commit = "5s"                # Block time
timeout_propose = "3s"                # Proposal timeout
```

#### Monitoring
```toml
[instrumentation]
prometheus = true                     # Enable Prometheus metrics
prometheus_listen_addr = ":26660"     # Metrics endpoint
```

---

### app.toml (Application Settings)

**Key sections:**

#### Gas & Fees
```toml
minimum-gas-prices = "0.025uomni"    # Minimum fees to accept
```

#### Pruning (Disk Management)
```toml
pruning = "default"                   # Options: default|nothing|everything|custom
pruning-keep-recent = "100"           # Keep last 100 states
pruning-interval = "10"               # Prune every 10 blocks
```

See **[PRUNING_STRATEGIES.md](PRUNING_STRATEGIES.md)** for detailed guide.

#### API & gRPC
```toml
[api]
enable = true
address = "tcp://0.0.0.0:1317"       # REST API

[grpc]
enable = true
address = "0.0.0.0:9090"             # gRPC endpoint
```

#### State Sync Snapshots
```toml
[state-sync]
snapshot-interval = 1000              # Create snapshot every 1000 blocks
snapshot-keep-recent = 2              # Keep last 2 snapshots
```

#### Telemetry
```toml
[telemetry]
enabled = true
prometheus-retention-time = 60        # Prometheus metrics retention
```

---

### client.toml (CLI Defaults)

**Key settings:**

```toml
chain-id = "omniphi-1"                # Network ID
keyring-backend = "file"              # Key storage (file|os|test)
output = "text"                       # Output format (text|json)
node = "tcp://localhost:26657"        # RPC endpoint
broadcast-mode = "sync"               # TX broadcast mode
```

---

## Deployment Scenarios

### Scenario 1: Standard Validator (VPS/Cloud)

**Hardware:** 4 CPU, 16GB RAM, 500GB SSD
**Pruning:** `default`
**Ports:** 26656 (P2P public), 26657 (RPC localhost), 9090 (gRPC public)

**Key config.toml settings:**
```toml
[p2p]
external_address = "YOUR_PUBLIC_IP:26656"
max_num_inbound_peers = 40
max_num_outbound_peers = 10

[rpc]
laddr = "tcp://127.0.0.1:26657"  # Localhost only for security
```

**Key app.toml settings:**
```toml
pruning = "default"
minimum-gas-prices = "0.025uomni"
snapshot-interval = 1000
```

---

### Scenario 2: Disk-Limited Validator (< 200GB)

**Hardware:** 2 CPU, 8GB RAM, 100GB SSD
**Pruning:** `everything` (aggressive)
**Ports:** Same as standard

**Key app.toml settings:**
```toml
pruning = "everything"              # Minimal disk usage
snapshot-interval = 0               # Disable snapshots to save space
```

---

### Scenario 3: Validator + Public RPC Endpoint

**Hardware:** 8 CPU, 32GB RAM, 1TB SSD
**Pruning:** `custom` (keep more history)
**Ports:** All public (behind reverse proxy)

**Key config.toml settings:**
```toml
[rpc]
laddr = "tcp://0.0.0.0:26657"       # Public RPC (use nginx + rate limiting)
max_open_connections = 900
```

**Key app.toml settings:**
```toml
pruning = "custom"
pruning-keep-recent = "362880"      # ~25 days of history at 6s blocks
snapshot-interval = 1000
snapshot-keep-recent = 5            # Keep more snapshots for state sync
```

---

### Scenario 4: Archive Node (Full History)

**Hardware:** 16 CPU, 64GB RAM, 2TB+ SSD
**Pruning:** `nothing` (no pruning)
**Ports:** All public

**Key app.toml settings:**
```toml
pruning = "nothing"                 # Keep all history
min-retain-blocks = 0               # Never prune CometBFT blocks
snapshot-interval = 0               # Disable (archive nodes don't need snapshots)
```

---

## Port Reference

| Port | Service | Expose Publicly? | Notes |
|------|---------|------------------|-------|
| **26656** | P2P networking | ✅ Yes | Required for peer connections |
| **26657** | RPC (CometBFT) | ⚠️ Optional | Localhost recommended; use reverse proxy if public |
| **26660** | Prometheus metrics | ❌ No | Monitoring only, restrict to monitoring server |
| **1317** | REST API | ⚠️ Optional | Localhost recommended; use reverse proxy if public |
| **9090** | gRPC | ⚠️ Optional | Needed for client connections |
| **9091** | gRPC-Web | ⚠️ Optional | Enables web clients |

---

## Security Best Practices

### 1. Firewall Configuration

```bash
# Allow P2P (required)
sudo ufw allow 26656/tcp

# Allow SSH (required for remote access)
sudo ufw allow 22/tcp

# Optional: Allow RPC from specific IP
sudo ufw allow from MONITORING_IP to any port 26657

# Optional: Allow gRPC for clients
sudo ufw allow 9090/tcp

# Enable firewall
sudo ufw enable
```

### 2. Validator Key Security

**Protect consensus private key:**
```bash
chmod 600 ~/.omniphi/config/priv_validator_key.json
```

**Backup key securely:**
```bash
# Encrypt backup
gpg --symmetric --cipher-algo AES256 ~/.omniphi/config/priv_validator_key.json

# Store encrypted backup offline
# NEVER share this file!
```

### 3. RPC/API Security

**Option 1: Localhost only (recommended for validators)**
```toml
[rpc]
laddr = "tcp://127.0.0.1:26657"

[api]
address = "tcp://127.0.0.1:1317"
```

**Option 2: Reverse proxy with rate limiting**
```nginx
# Nginx config
location /rpc {
    limit_req zone=rpc_limit burst=10;
    proxy_pass http://localhost:26657;
}
```

### 4. Sentry Node Architecture (Advanced)

**For high-value validators:**
- Run validator behind sentry nodes
- Validator only connects to trusted sentries
- Sentries handle all public P2P traffic

**Validator config.toml:**
```toml
[p2p]
pex = false                           # Disable peer exchange
persistent_peers = "sentry1,sentry2"  # Only connect to sentries
addr_book_strict = false              # Allow private IPs
```

---

## Monitoring Setup

See **[PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md)** for complete monitoring guide.

**Quick setup:**

1. **Enable metrics** (already enabled in templates)
2. **Install Prometheus**
3. **Install Grafana**
4. **Import Cosmos validator dashboard** (ID: 11036)

**Verify metrics endpoint:**
```bash
curl http://localhost:26660/metrics
```

---

## Common Configuration Issues

### Issue: "Cannot connect to peers"

**Check:**
```bash
# Verify port 26656 is open
sudo ufw status | grep 26656

# Test external connectivity
telnet YOUR_PUBLIC_IP 26656

# Check peer configuration
grep persistent_peers ~/.omniphi/config/config.toml
```

### Issue: "Disk full" errors

**Solution:** Adjust pruning strategy

See **[PRUNING_STRATEGIES.md](PRUNING_STRATEGIES.md)** for details.

**Quick fix:**
```toml
# In app.toml:
pruning = "everything"  # Most aggressive pruning
```

### Issue: "High memory usage"

**Solutions:**

1. Reduce IAVL cache:
```toml
# In app.toml:
iavl-cache-size = 400000  # Reduce from default 781250
```

2. Disable inter-block cache:
```toml
inter-block-cache = false
```

### Issue: "Validator missing blocks"

**Check sync status:**
```bash
posd status | jq '.SyncInfo'
```

**Check peer count:**
```bash
curl localhost:26657/net_info | jq '.result.n_peers'
```

**Review consensus settings:**
```toml
# In config.toml:
[consensus]
timeout_commit = "5s"      # Should match network block time
timeout_propose = "3s"
```

---

## Upgrading Configuration

### When upgrading Omniphi versions:

1. **Backup current configs:**
```bash
cp -r ~/.omniphi/config ~/.omniphi/config.backup-$(date +%Y%m%d)
```

2. **Check for new config options:**
```bash
# Download new templates
wget https://raw.githubusercontent.com/omniphi/validator-orchestrator/main/infra/configs/config.toml.template
wget https://raw.githubusercontent.com/omniphi/validator-orchestrator/main/infra/configs/app.toml.template

# Compare with your current config
diff config.toml.template ~/.omniphi/config/config.toml
diff app.toml.template ~/.omniphi/config/app.toml
```

3. **Merge new options** into your existing config

4. **Test before production:**
```bash
# Validate config
posd start --home ~/.omniphi --dry-run
```

---

## Additional Resources

- **[Traditional Setup Guide](../../docs/TRADITIONAL_SETUP.md)** - Complete validator setup walkthrough
- **[Systemd Service](../systemd/)** - Production service configuration
- **[Omniphi Documentation](https://docs.omniphi.io)** - Official docs
- **[Cosmos SDK Docs](https://docs.cosmos.network)** - Framework documentation
- **[CometBFT Docs](https://docs.cometbft.com)** - Consensus engine docs

---

## Template Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2025-11-20 | 1.0.0 | Initial production templates |

---

**Need help?** Open an issue at: https://github.com/omniphi/validator-orchestrator/issues
