# State Sync Guide for Omniphi Validators

**Fast-sync your validator node in minutes instead of days.**

---

## Overview

**State sync** allows a new node to bootstrap by downloading a recent snapshot of the blockchain state instead of replaying all historical blocks from genesis.

### Benefits

| Method | Sync Time | Disk Usage | Historical Data |
|--------|-----------|------------|-----------------|
| **Full Sync** | Days to weeks | High (full history) | Complete |
| **Fast Sync** | Hours to days | Medium | From genesis |
| **State Sync** | 5-30 minutes | Low | From snapshot height only |

**Recommended for:**
- New validators joining the network
- Recovering from corruption
- Testing/development nodes
- Nodes that don't need full history

**Not recommended for:**
- Archive nodes (need full history)
- Block explorers
- Nodes providing public RPC

---

## Quick Start

### Prerequisites

1. **Initialized node** (config files generated)
2. **Trusted RPC endpoints** (2+ recommended)
3. **Recent trust height and hash** (from trusted source)

### Step 1: Get Trust Height and Hash

**From a trusted RPC endpoint:**

```bash
# Get latest block height
LATEST_HEIGHT=$(curl -s https://rpc.omniphi.io/block | jq -r '.result.block.header.height')

# Calculate trust height (go back ~2000 blocks for safety)
TRUST_HEIGHT=$((LATEST_HEIGHT - 2000))

# Get trust hash at that height
TRUST_HASH=$(curl -s "https://rpc.omniphi.io/block?height=$TRUST_HEIGHT" | jq -r '.result.block_id.hash')

echo "Trust Height: $TRUST_HEIGHT"
echo "Trust Hash: $TRUST_HASH"
```

**Example output:**
```
Trust Height: 1234567
Trust Hash: 5A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1D2E3F4A5B6C7D8E9F0A1B
```

---

### Step 2: Configure State Sync

**Edit `~/.omniphi/config/config.toml`:**

```toml
#######################################################
###         State Sync Configuration Options        ###
#######################################################
[statesync]

# Enable state sync
enable = true

# RPC servers for light client verification
# Use 2+ trusted RPC endpoints (comma-separated)
rpc_servers = "https://rpc1.omniphi.io:443,https://rpc2.omniphi.io:443"

# Trust height and hash from Step 1
trust_height = 1234567
trust_hash = "5A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9B0C1D2E3F4A5B6C7D8E9F0A1B"

# Trust period (should be ~2/3 of unbonding period)
# For 21-day unbonding: 336h0m0s (14 days)
trust_period = "168h0m0s"

# Discovery time
discovery_time = "15s"

# Temporary directory for chunks
temp_dir = ""

# Chunk request timeout
chunk_request_timeout = "10s"

# Number of concurrent chunk fetchers
chunk_fetchers = "4"
```

---

### Step 3: Reset Node Data (If Re-Syncing)

**⚠️ WARNING: This deletes all existing blockchain data!**

```bash
# Stop validator if running
sudo systemctl stop posd

# Reset blockchain data (keeps config and keys)
posd tendermint unsafe-reset-all --home ~/.omniphi

# Verify config and keys are intact
ls -la ~/.omniphi/config/priv_validator_key.json  # Should exist
ls -la ~/.omniphi/config/config.toml               # Should exist
ls -la ~/.omniphi/data/                            # Should be empty
```

---

### Step 4: Start State Sync

```bash
# Start validator
sudo systemctl start posd

# Monitor logs
sudo journalctl -u posd -f
```

**Expected log output:**

```
Discovered new snapshot  height=1234567 format=2
Fetching snapshot chunk  height=1234567 chunk=1/50
Fetching snapshot chunk  height=1234567 chunk=2/50
...
Fetching snapshot chunk  height=1234567 chunk=50/50
Applying snapshot chunk  height=1234567 chunk=1/50
...
State sync complete      height=1234567
```

---

### Step 5: Verify State Sync Completed

```bash
# Check sync status
posd status | jq '.SyncInfo'

# Should show:
# {
#   "latest_block_height": "1234600",  # Close to network height
#   "catching_up": false                # Should be false
# }

# Check peer count
curl -s localhost:26657/net_info | jq '.result.n_peers'
# Should be > 5
```

---

## Advanced Configuration

### Using Multiple RPC Servers

**Best practice: Use 3+ geographically distributed RPC endpoints**

```toml
[statesync]
rpc_servers = "https://rpc-us.omniphi.io:443,https://rpc-eu.omniphi.io:443,https://rpc-asia.omniphi.io:443"
```

**Benefits:**
- Redundancy if one RPC is down
- Faster chunk downloads (load balanced)
- Geographic diversity

---

### Optimizing Chunk Fetching

**For fast networks (1Gbps+):**

```toml
[statesync]
chunk_fetchers = "8"          # More parallel downloads
chunk_request_timeout = "5s"  # Shorter timeout
```

**For slow/unreliable networks:**

```toml
[statesync]
chunk_fetchers = "2"          # Fewer parallel connections
chunk_request_timeout = "30s" # Longer timeout
```

---

### Custom Temporary Directory

**If `/tmp` is small, use different directory:**

```toml
[statesync]
temp_dir = "/mnt/large-disk/statesync-tmp"
```

```bash
# Create directory
sudo mkdir -p /mnt/large-disk/statesync-tmp
sudo chown omniphi:omniphi /mnt/large-disk/statesync-tmp
```

---

## Finding Trusted RPC Endpoints

### Official Omniphi RPC Servers

**Mainnet:**
- `https://rpc.omniphi.io:443`
- `https://rpc-us.omniphi.io:443`
- `https://rpc-eu.omniphi.io:443`

**Testnet:**
- `https://rpc-testnet.omniphi.io:443`

---

### Community RPC Servers

**Check Omniphi Discord/docs for community-run RPCs:**
- Public endpoints list: https://docs.omniphi.io/nodes/rpc
- Validator endpoints (if you know trusted validators)

---

### Running Your Own RPC for State Sync

**If you already have a synced node:**

```toml
# On your existing synced node
# Edit ~/.omniphi/config/app.toml

[state-sync]
snapshot-interval = 1000
snapshot-keep-recent = 2
```

```bash
# Restart to enable snapshots
sudo systemctl restart posd

# Wait for snapshots to be created
# Check logs:
sudo journalctl -u posd -f | grep snapshot
```

**Then use your own node as RPC source:**

```toml
# On new node
[statesync]
rpc_servers = "http://YOUR_SYNCED_NODE_IP:26657,https://rpc.omniphi.io:443"
```

---

## Automation Script

**Complete automated state sync script:**

```bash
#!/bin/bash
# state-sync-omniphi.sh

set -e

# Configuration
RPC1="https://rpc.omniphi.io"
RPC2="https://rpc-us.omniphi.io"
OMNIPHI_HOME="$HOME/.omniphi"

echo "=== Omniphi State Sync Setup ==="
echo ""

# Stop validator if running
echo "Stopping validator..."
sudo systemctl stop posd 2>/dev/null || true

# Get trust height and hash
echo "Fetching trust height and hash..."
LATEST_HEIGHT=$(curl -s $RPC1/block | jq -r '.result.block.header.height')
TRUST_HEIGHT=$((LATEST_HEIGHT - 2000))
TRUST_HASH=$(curl -s "$RPC1/block?height=$TRUST_HEIGHT" | jq -r '.result.block_id.hash')

echo "Latest Height: $LATEST_HEIGHT"
echo "Trust Height: $TRUST_HEIGHT"
echo "Trust Hash: $TRUST_HASH"
echo ""

# Backup current state (optional)
echo "Backing up current state..."
tar -czf "$HOME/omniphi-backup-$(date +%Y%m%d-%H%M%S).tar.gz" \
  "$OMNIPHI_HOME/config/priv_validator_key.json" \
  "$OMNIPHI_HOME/data/priv_validator_state.json" \
  2>/dev/null || true

# Reset blockchain data
echo "Resetting blockchain data..."
posd tendermint unsafe-reset-all --home $OMNIPHI_HOME

# Update config.toml
echo "Updating state sync configuration..."
CONFIG_FILE="$OMNIPHI_HOME/config/config.toml"

# Enable state sync
sed -i 's/enable = false/enable = true/g' $CONFIG_FILE

# Set RPC servers
sed -i "s|rpc_servers = \".*\"|rpc_servers = \"$RPC1:443,$RPC2:443\"|g" $CONFIG_FILE

# Set trust height
sed -i "s/trust_height = .*/trust_height = $TRUST_HEIGHT/g" $CONFIG_FILE

# Set trust hash
sed -i "s/trust_hash = \".*\"/trust_hash = \"$TRUST_HASH\"/g" $CONFIG_FILE

# Set trust period (14 days)
sed -i 's/trust_period = ".*"/trust_period = "168h0m0s"/g' $CONFIG_FILE

# Optimize chunk fetching
sed -i 's/chunk_fetchers = ".*"/chunk_fetchers = "4"/g' $CONFIG_FILE

echo ""
echo "=== Configuration Complete ==="
echo "Starting validator..."
sudo systemctl start posd

echo ""
echo "Monitor progress with:"
echo "  sudo journalctl -u posd -f"
echo ""
echo "Check sync status with:"
echo "  posd status | jq '.SyncInfo'"
```

**Usage:**

```bash
chmod +x state-sync-omniphi.sh
./state-sync-omniphi.sh
```

---

## Troubleshooting

### Issue: "No available snapshots"

**Cause:** RPC servers don't have recent snapshots

**Solutions:**

1. **Use different RPC endpoints:**
```toml
rpc_servers = "https://different-rpc.omniphi.io:443,..."
```

2. **Increase discovery time:**
```toml
discovery_time = "30s"  # Give more time to find snapshots
```

3. **Verify RPC servers have snapshots enabled:**
```bash
# Check if RPC server provides snapshots
curl -s https://rpc.omniphi.io/status | jq '.result.sync_info.latest_block_height'
```

---

### Issue: "Chunk fetch timeout"

**Cause:** Network too slow or RPC server overloaded

**Solutions:**

1. **Increase timeout:**
```toml
chunk_request_timeout = "30s"
```

2. **Reduce concurrent fetchers:**
```toml
chunk_fetchers = "2"
```

3. **Use faster RPC endpoints:**
```bash
# Test RPC latency
time curl -s https://rpc.omniphi.io/status > /dev/null
```

---

### Issue: "Trust hash mismatch"

**Cause:** Trust hash is incorrect or chain reorganization occurred

**Solutions:**

1. **Get fresh trust hash:**
```bash
# Re-run trust height/hash fetch
LATEST_HEIGHT=$(curl -s https://rpc.omniphi.io/block | jq -r '.result.block.header.height')
TRUST_HEIGHT=$((LATEST_HEIGHT - 2000))
TRUST_HASH=$(curl -s "https://rpc.omniphi.io/block?height=$TRUST_HEIGHT" | jq -r '.result.block_id.hash')
```

2. **Use larger safety margin:**
```bash
# Go back further (5000 blocks)
TRUST_HEIGHT=$((LATEST_HEIGHT - 5000))
```

---

### Issue: State sync stuck at specific chunk

**Symptoms:**
```
Fetching snapshot chunk  height=1234567 chunk=25/50
Fetching snapshot chunk  height=1234567 chunk=25/50
Fetching snapshot chunk  height=1234567 chunk=25/50
(repeating)
```

**Solutions:**

1. **Restart the process:**
```bash
sudo systemctl stop posd
posd tendermint unsafe-reset-all --home ~/.omniphi
sudo systemctl start posd
```

2. **Use different RPC servers:**
```toml
rpc_servers = "https://alternative-rpc.omniphi.io:443,..."
```

3. **Disable state sync and use fast sync:**
```toml
[statesync]
enable = false

[fastsync]
version = "v0"
```

---

### Issue: "Light client verification failed"

**Cause:** Trust period expired or RPC servers out of sync

**Solutions:**

1. **Update trust height/hash:**
```bash
# Get fresh values (trust height should be recent)
./state-sync-omniphi.sh
```

2. **Adjust trust period:**
```toml
# Increase trust period
trust_period = "336h0m0s"  # 14 days → 21 days
```

3. **Verify RPC servers are in sync:**
```bash
# Check both RPCs are at same height
curl -s https://rpc1.omniphi.io/status | jq '.result.sync_info.latest_block_height'
curl -s https://rpc2.omniphi.io/status | jq '.result.sync_info.latest_block_height'
```

---

## State Sync vs Fast Sync vs Full Sync

### When to Use Each

**State Sync (Fastest):**
- ✅ New validators
- ✅ Recovery from corruption
- ✅ Test/development nodes
- ❌ Archive nodes
- ❌ Public RPC endpoints

**Fast Sync (Middle Ground):**
- ✅ Need some historical data
- ✅ More reliable than state sync
- ❌ Slower than state sync (hours vs minutes)

**Full Sync (Slowest):**
- ✅ Archive nodes (need all history)
- ✅ Block explorers
- ✅ Maximum security (verify all blocks)
- ❌ Very slow (days to weeks)

---

## Post-State Sync Configuration

### Disable State Sync After Initial Sync

**Once synced, disable state sync:**

```bash
# Edit ~/.omniphi/config/config.toml
nano ~/.omniphi/config/config.toml
```

```toml
[statesync]
enable = false  # Change to false
```

```bash
# Restart
sudo systemctl restart posd
```

**Why disable?**
- State sync only needed once
- Prevents accidental re-sync
- Reduces RPC server load

---

### Enable Snapshot Creation (Help Others Sync)

**If you want to provide snapshots for other nodes:**

```bash
# Edit ~/.omniphi/config/app.toml
nano ~/.omniphi/config/app.toml
```

```toml
[state-sync]
snapshot-interval = 1000
snapshot-keep-recent = 2
```

```bash
# Restart
sudo systemctl restart posd

# Verify snapshots are being created
ls -lh ~/.omniphi/data/snapshots/
```

---

## Security Considerations

### Trust Assumptions

**State sync requires trusting:**
1. **RPC servers** - Provide correct snapshots
2. **Trust hash** - Initial state is valid
3. **Network** - >2/3 validators are honest

**Mitigation:**
- Use multiple RPC endpoints from different operators
- Get trust hash from multiple sources
- Verify chain-id matches

---

### Verifying Trust Hash

**Cross-check trust hash from multiple sources:**

```bash
# Source 1: Official RPC
curl -s "https://rpc.omniphi.io/block?height=$TRUST_HEIGHT" | jq -r '.result.block_id.hash'

# Source 2: Community RPC
curl -s "https://community-rpc.omniphi.io/block?height=$TRUST_HEIGHT" | jq -r '.result.block_id.hash'

# Source 3: Block explorer
# Check: https://explorer.omniphi.io/blocks/$TRUST_HEIGHT

# All should match!
```

---

## Best Practices

### ✅ Do's

- Use 2+ RPC endpoints from different operators
- Verify trust hash from multiple sources
- Backup validator keys before resetting data
- Monitor sync progress (logs)
- Disable state sync after initial sync
- Test state sync on testnet first

### ❌ Don'ts

- Don't use untrusted RPC servers
- Don't trust hash from single source
- Don't state sync on archive nodes
- Don't leave state sync enabled after sync completes
- Don't state sync frequently (use fast sync for updates)

---

## Summary

**State sync is the fastest way to bootstrap a new validator.**

**Key steps:**
1. Get trust height/hash from trusted RPC
2. Configure state sync in `config.toml`
3. Reset blockchain data
4. Start validator and monitor logs
5. Disable state sync after completion

**Typical sync time:** 5-30 minutes (vs days for full sync)

**Requirements:**
- Trusted RPC endpoints
- Recent trust height/hash
- Good network connection

---

**For more operational guides:**
- [MONITORING.md](MONITORING.md) - Set up monitoring and alerts
- [BACKUPS.md](BACKUPS.md) - Backup and restore procedures
- [UPGRADES.md](UPGRADES.md) - Upgrade validator software

---

**Need help?** Ask in Omniphi Discord: https://discord.gg/omniphi
