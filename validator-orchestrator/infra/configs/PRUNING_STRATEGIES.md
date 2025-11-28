# Pruning Strategies for Omniphi Validators

**Purpose:** Control disk space usage by managing historical blockchain state retention.

---

## Overview

Pruning removes old blockchain state data to save disk space. The right pruning strategy depends on your node's purpose and available disk space.

---

## Pruning Strategies

### 1. Default (Recommended for Most Validators)

**Configuration in `app.toml`:**
```toml
pruning = "default"
```

**What it does:**
- Keeps the last **100 states**
- Keeps every **500th state** for historical queries
- Prunes at **10 block intervals**

**Disk usage:** Moderate (20-50 GB depending on chain activity)

**Best for:**
- Most validator operators
- Nodes that need recent state history
- Balance between disk space and functionality

**Pros:**
- Good balance of disk space and data retention
- Sufficient history for debugging
- Can handle most RPC queries for recent blocks

**Cons:**
- Cannot serve very old historical queries
- Uses more disk than "everything" pruning

---

### 2. Nothing (Archive Node)

**Configuration in `app.toml`:**
```toml
pruning = "nothing"
```

**What it does:**
- Keeps **ALL historical states**
- Never deletes any state data
- Full archival node

**Disk usage:** Very high (100+ GB and growing)

**Best for:**
- Block explorers
- Historical data analysis
- Nodes serving public RPC endpoints
- Research and analytics

**Pros:**
- Can answer any historical query
- Complete blockchain history
- Useful for ecosystem services

**Cons:**
- Requires large disk space (500GB - 1TB+)
- Slower sync times
- Higher I/O requirements

**NOT RECOMMENDED for typical validators** unless you're also running a public RPC service.

---

### 3. Everything (Aggressive Pruning)

**Configuration in `app.toml`:**
```toml
pruning = "everything"
```

**What it does:**
- Keeps **ONLY current and previous state**
- Deletes all other historical states
- Prunes at **10 block intervals**

**Disk usage:** Minimal (5-15 GB)

**Best for:**
- Validators with limited disk space
- Nodes that only need to validate current state
- Cost-sensitive deployments

**Pros:**
- Minimal disk space usage
- Faster sync (less data to process)
- Lower storage costs

**Cons:**
- Cannot answer historical queries
- Limited debugging capabilities
- Not suitable for RPC endpoints

**WARNING:** You won't be able to query any historical state. Only use this if disk space is very limited.

---

### 4. Custom (Advanced Configuration)

**Configuration in `app.toml`:**
```toml
pruning = "custom"
pruning-keep-recent = "100"
pruning-keep-every = "0"
pruning-interval = "10"
```

**Parameters:**

- **`pruning-keep-recent`**: How many recent states to keep
  - Recommended: `100` for validators (keeps ~10 minutes of history at 6s block time)
  - Higher values = more recent history, more disk space

- **`pruning-keep-every`**: Keep every Nth state
  - Recommended: `0` for validators (don't keep specific intervals)
  - Use `500` or `1000` if you need periodic historical snapshots

- **`pruning-interval`**: How often to prune (in blocks)
  - Recommended: `10` (prune every 10 blocks)
  - Lower = more frequent pruning, higher CPU usage
  - Higher = less frequent pruning, more disk usage between prunes

**Best for:**
- Advanced users with specific requirements
- Fine-tuning based on hardware capabilities
- Optimizing for specific use cases

---

## Recommended Settings by Use Case

### Validator Only (Most Common)
```toml
pruning = "default"
# OR custom:
pruning = "custom"
pruning-keep-recent = "100"
pruning-keep-every = "0"
pruning-interval = "10"
```

### Validator + Public RPC Endpoint
```toml
pruning = "custom"
pruning-keep-recent = "362880"  # ~25 days at 6s blocks
pruning-keep-every = "0"
pruning-interval = "10"
```

### Validator with Limited Disk (< 100GB)
```toml
pruning = "everything"
```

### Archive Node (Explorer, Analytics)
```toml
pruning = "nothing"
```

### Validator + State Sync Provider
```toml
pruning = "custom"
pruning-keep-recent = "100000"  # Keep more recent state for snapshots
pruning-keep-every = "0"
pruning-interval = "10"

# Also enable state-sync snapshots in app.toml:
[state-sync]
snapshot-interval = 1000
snapshot-keep-recent = 2
```

---

## CometBFT Block Pruning

**Separate from application state pruning!**

In `app.toml`:
```toml
# Minimum block height to retain in CometBFT
min-retain-blocks = 0
```

**What it does:**
- Controls how many CometBFT blocks to keep
- `0` means keep all blocks (recommended for validators)
- Non-zero values prune blocks older than `current_height - min-retain-blocks`

**Recommendation for validators:** Keep at `0` and let state pruning handle disk management.

---

## State Sync Considerations

If you're using **state sync** to bootstrap new nodes:

**On the state sync source node:**
```toml
pruning = "custom"
pruning-keep-recent = "100000"  # Keep enough state for snapshots
snapshot-interval = 1000
snapshot-keep-recent = 2
```

**On the syncing node (after sync completes):**
```toml
pruning = "default"  # Or your preferred strategy
```

**Important:** The state sync source must have enough historical state to create snapshots.

---

## Monitoring Disk Usage

### Check current disk usage:
```bash
# Total data directory size
du -sh ~/.omniphi

# Application state database size
du -sh ~/.omniphi/data/application.db

# CometBFT block storage size
du -sh ~/.omniphi/data/blockstore.db
```

### Monitor over time:
```bash
# Create monitoring script
cat > ~/check-validator-disk.sh <<'EOF'
#!/bin/bash
echo "=== Omniphi Validator Disk Usage ==="
echo "Date: $(date)"
echo ""
echo "Total data directory:"
du -sh ~/.omniphi
echo ""
echo "Application state:"
du -sh ~/.omniphi/data/application.db
echo ""
echo "Block storage:"
du -sh ~/.omniphi/data/blockstore.db
echo ""
echo "Available disk space:"
df -h ~/.omniphi
EOF

chmod +x ~/check-validator-disk.sh

# Run weekly via cron
crontab -e
# Add: 0 0 * * 0 ~/check-validator-disk.sh >> ~/validator-disk-usage.log
```

---

## Migration Between Pruning Strategies

### Changing from "nothing" to "default"

**WARNING:** This requires stopping the node and rebuilding state!

```bash
# Stop the validator
sudo systemctl stop posd

# Backup current state (IMPORTANT!)
tar -czf ~/omniphi-backup-$(date +%Y%m%d).tar.gz ~/.omniphi

# Update app.toml
nano ~/.omniphi/config/app.toml
# Change: pruning = "default"

# Option 1: Start node and let it prune over time
sudo systemctl start posd
# Pruning happens gradually, disk space reduces over days

# Option 2: Reset and resync (faster, more aggressive)
posd tendermint unsafe-reset-all --home ~/.omniphi
# Download snapshot or use state sync
sudo systemctl start posd
```

### Changing from "everything" to "default"

```bash
# Stop the validator
sudo systemctl stop posd

# Update app.toml
nano ~/.omniphi/config/app.toml
# Change: pruning = "default"

# Restart
sudo systemctl start posd
# Node will start keeping more state going forward
```

**NOTE:** Changing TO a more aggressive pruning strategy ("default" → "everything") takes effect immediately. Changing to LESS aggressive pruning ("everything" → "default") only affects future blocks.

---

## Performance Impact

### CPU Usage
- **More aggressive pruning** = higher CPU usage (more frequent pruning operations)
- **Less aggressive pruning** = lower CPU usage

### Disk I/O
- **Pruning interval** affects I/O spikes
- Lower intervals (e.g., `pruning-interval = 5`) = more frequent I/O
- Higher intervals (e.g., `pruning-interval = 50`) = less frequent but larger I/O spikes

### Recommendations:
- **SSD:** Use any pruning strategy (fast enough for all)
- **HDD:** Use `pruning-interval = 50` or higher to reduce I/O
- **NVMe:** Can handle aggressive pruning without issues

---

## Common Issues

### "Cannot query historical state"

**Cause:** State was pruned
**Solution:**
- Use a less aggressive pruning strategy
- Query a different node (archive node)
- Use block explorers for historical queries

### "Disk full" errors

**Cause:** Not enough pruning
**Solution:**
```bash
# Emergency: switch to aggressive pruning
sudo systemctl stop posd
nano ~/.omniphi/config/app.toml
# Change: pruning = "everything"
sudo systemctl start posd
```

### State sync fails

**Cause:** Source node pruned too aggressively
**Solution:** Use a state sync source with `pruning-keep-recent >= 100000`

---

## Best Practices

1. **Start conservative:** Use "default" pruning initially
2. **Monitor disk usage** for the first week
3. **Adjust based on needs:** Switch to "custom" if needed
4. **Always backup** before changing pruning strategies
5. **Archive nodes:** Consider running a separate archive node if you need historical data
6. **Validator focus:** Validators should prioritize uptime over historical queries

---

## Summary Table

| Strategy | Disk Usage | History | Use Case |
|----------|-----------|---------|----------|
| **default** | Moderate (20-50GB) | Last 100 + every 500th | Most validators |
| **nothing** | High (100GB+) | Complete | Archive/RPC nodes |
| **everything** | Low (5-15GB) | Current + previous | Disk-limited validators |
| **custom** | Variable | Configurable | Advanced/specific needs |

---

**Recommendation for Omniphi validators:** Start with `pruning = "default"` and adjust based on your disk space and query needs.
