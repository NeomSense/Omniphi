# Slashing Protection Guide

**Critical guide for preventing slashing penalties and protecting your validator stake.**

⚠️ **WARNING:** Slashing can result in permanent loss of staked tokens. This guide is REQUIRED READING for all validator operators.

---

## Table of Contents

1. [What is Slashing?](#what-is-slashing)
2. [Types of Slashing Events](#types-of-slashing-events)
3. [Double-Signing Protection](#double-signing-protection)
4. [Downtime Protection](#downtime-protection)
5. [Monitoring & Alerts](#monitoring--alerts)
6. [Recovery from Slashing](#recovery-from-slashing)
7. [Best Practices](#best-practices)

---

## What is Slashing?

**Slashing** is an on-chain penalty mechanism that punishes validators for malicious or negligent behavior by destroying a portion of their staked tokens.

### Why Slashing Exists

- **Network security:** Discourages attacks (e.g., double-signing)
- **Economic incentive:** Ensures validators maintain high uptime
- **Decentralization:** Penalizes centralized failure points

### Slashing Economics

| Event Type | Penalty | Jailing | Recovery |
|------------|---------|---------|----------|
| **Double-signing** | ~5% of stake | Permanently jailed | Cannot unjail (tombstoned) |
| **Downtime** | ~0.01% of stake | Temporarily jailed | Can unjail after cooldown |

**Example:**
- Validator with 1,000,000 OMNI staked
- Double-sign event: **-50,000 OMNI** (5%) + permanent removal
- Downtime event: **-100 OMNI** (0.01%) + temporary jailing

---

## Types of Slashing Events

### 1. Double-Signing (Equivocation)

**What is it?**
- Signing two different blocks at the same height/round
- Also called "equivocation" or "byzantine behavior"

**Causes:**
1. **Running two validators with same key** (most common)
   - Backup validator starts while primary is still running
   - Accidentally copied `priv_validator_key.json` to second server
   - Poor failover automation

2. **State file desynchronization**
   - `priv_validator_state.json` reset or corrupted
   - Validator restarts with old state

3. **Clock drift**
   - System time significantly out of sync
   - Validator signs same height multiple times

**Penalty:**
- **~5% of bonded stake slashed**
- **Permanently jailed (tombstoned)**
- **Cannot unjail** - must create new validator

**Prevention:** See [Double-Signing Protection](#double-signing-protection)

---

### 2. Downtime (Liveness Fault)

**What is it?**
- Missing too many blocks in a sliding window
- Typically: Missing 50% of last 10,000 blocks (~8-9 hours at 6s block time)

**Causes:**
1. **Server downtime**
   - Hardware failure
   - Network outage
   - OS crashes

2. **Validator not catching up**
   - Falling behind due to poor network
   - State sync issues

3. **Misconfiguration**
   - P2P port blocked
   - Not enough peers
   - Disk full

**Penalty:**
- **~0.01% of bonded stake slashed**
- **Temporarily jailed**
- **Must unjail manually** (requires transaction + unjail fee)

**Prevention:** See [Downtime Protection](#downtime-protection)

---

## Double-Signing Protection

### Understanding the State File

**File:** `~/.omniphi/data/priv_validator_state.json`

**Contents:**
```json
{
  "height": "12345",
  "round": 0,
  "step": 3,
  "signature": "ABC123...",
  "signbytes": "DEF456..."
}
```

**Purpose:**
- Tracks the last height/round/step signed by validator
- Prevents signing older blocks (replay protection)
- **CRITICAL:** Must be in sync with blockchain state

---

### Rule #1: Never Run Two Validators with Same Key

**❌ NEVER DO THIS:**

```bash
# Server A (primary validator):
posd start  # Running with priv_validator_key.json

# Server B (backup):
scp ~/.omniphi/config/priv_validator_key.json server-b:/path/
# Then on server-b:
posd start  # DOUBLE-SIGNING RISK!
```

**Why it causes slashing:**
- Both servers sign blocks independently
- Both sign same height → double-signing detected → slashed

---

### Rule #2: Protect the State File

**Always backup state file with validator key:**

```bash
# WRONG - backing up key without state:
cp ~/.omniphi/config/priv_validator_key.json backup/

# RIGHT - backing up both:
tar -czf validator-backup-$(date +%Y%m%d).tar.gz \
  ~/.omniphi/config/priv_validator_key.json \
  ~/.omniphi/data/priv_validator_state.json
```

**When restoring from backup:**

```bash
# Extract both files
tar -xzf validator-backup-20251120.tar.gz

# Verify state file height is recent
cat ~/.omniphi/data/priv_validator_state.json | jq '.height'

# If height is old (validator has been running elsewhere), DO NOT START!
# Wait for current chain height to exceed backup height by 100+ blocks
```

---

### Rule #3: Use Conditional Failover

**Automated failover is dangerous!** Use these strategies:

#### Strategy 1: Manual Failover (Safest)

```bash
# Primary validator down → Manual intervention required

# On PRIMARY:
sudo systemctl stop posd
# Wait 5 minutes to ensure it's fully stopped

# On BACKUP:
# Copy latest state from primary (if accessible)
rsync -avz primary-server:~/.omniphi/data/priv_validator_state.json ~/.omniphi/data/

# Start backup
sudo systemctl start posd
```

---

#### Strategy 2: Time-Delayed Failover

```bash
#!/bin/bash
# Failover script with safety delay

PRIMARY_HOST="10.0.1.5"
DELAY_SECONDS=300  # 5 minutes

# Check if primary is down
if ! nc -zv $PRIMARY_HOST 26657 &>/dev/null; then
    echo "Primary down, waiting $DELAY_SECONDS seconds before failover..."
    sleep $DELAY_SECONDS

    # Check again
    if ! nc -zv $PRIMARY_HOST 26657 &>/dev/null; then
        echo "Primary still down, starting backup validator"
        sudo systemctl start posd
    else
        echo "Primary recovered, aborting failover"
    fi
else
    echo "Primary is healthy"
fi
```

**Run via cron (every 2 minutes):**
```bash
*/2 * * * * /home/omniphi/failover-check.sh >> /var/log/failover.log 2>&1
```

---

#### Strategy 3: Consensus-Based Failover (Advanced)

Use external consensus to decide which validator should be active.

**Tools:**
- **Horcrux** - Distributed validator signing (recommended)
- **etcd/Consul** - Distributed lock service
- **Raft consensus** - Leadership election

**Example with Horcrux:**

Horcrux splits the consensus private key across 3+ nodes using threshold signing (2-of-3). Only one set of signatures is ever produced, eliminating double-signing risk.

```bash
# Install Horcrux
# See: https://github.com/strangelove-ventures/horcrux

# Each node runs Horcrux, not posd directly
# Horcrux coordinates signing across nodes
# Even if 1 node signs twice, threshold requires 2+ signatures
```

---

### Rule #4: Use Hardware Security Modules (HSM)

**Recommended for validators with >$100k stake.**

#### Tendermint KMS (tmkms)

**Setup:**

```bash
# Install tmkms
cargo install tmkms --features=yubihsm

# Initialize
tmkms init /etc/tmkms

# Configure YubiHSM
nano /etc/tmkms/tmkms.toml
```

**tmkms.toml:**
```toml
[[chain]]
id = "omniphi-1"
key_format = { type = "bech32", account_key_prefix = "omnipub", consensus_key_prefix = "omnivalconspub" }
state_file = "/etc/tmkms/state/omniphi-1-consensus.json"

[[validator]]
chain_id = "omniphi-1"
addr = "tcp://validator-ip:26658"
secret_key = "/etc/tmkms/secrets/validator-secret.key"
protocol_version = "v0.34"
max_height = "none"

[[providers.yubihsm]]
adapter = { type = "usb" }
auth = { key = 1, password_file = "/etc/tmkms/password" }
keys = [{ chain_ids = ["omniphi-1"], key = 1, type = "consensus" }]

[providers.yubihsm.serial_devices]
```

**Double-signing protection:**
- tmkms tracks height/round in state file
- Refuses to sign if height/round is not strictly increasing
- Even if validator crashes and restarts, tmkms prevents replay

**Start tmkms:**
```bash
sudo systemctl start tmkms
sudo journalctl -u tmkms -f
```

---

### Rule #5: Monitor for Double-Sign Alerts

**Set up monitoring:**

```bash
# Check for slashing events
posd query slashing signing-info $(posd tendermint show-validator)

# Expected output:
{
  "address": "omnivalcons1...",
  "start_height": "0",
  "index_offset": "12345",
  "jailed_until": "1970-01-01T00:00:00Z",
  "tombstoned": false,  # ← Should be false
  "missed_blocks_counter": "5"
}
```

**Alert if `tombstoned: true`:**

```bash
#!/bin/bash
# check-slashing.sh

TOMBSTONED=$(posd query slashing signing-info $(posd tendermint show-validator) -o json | jq -r '.tombstoned')

if [ "$TOMBSTONED" = "true" ]; then
    echo "CRITICAL: Validator has been slashed for double-signing!" | mail -s "SLASHING ALERT" your-email@example.com
fi
```

**Run via cron (every 5 minutes):**
```bash
*/5 * * * * /home/omniphi/check-slashing.sh
```

---

## Downtime Protection

### Understanding Downtime Parameters

**On Omniphi, downtime slashing occurs when:**

- **Signed blocks < 50%** of `signed_blocks_window`
- `signed_blocks_window` = typically 10,000 blocks
- At 6s block time: **10,000 blocks ≈ 16.67 hours**
- Missing 5,000+ blocks in this window = slashing

**Check current parameters:**
```bash
posd query slashing params

# Output:
{
  "signed_blocks_window": "10000",
  "min_signed_per_window": "0.500000000000000000",  # 50%
  "downtime_jail_duration": "600s",  # 10 minutes
  "slash_fraction_double_sign": "0.050000000000000000",  # 5%
  "slash_fraction_downtime": "0.000100000000000000"  # 0.01%
}
```

---

### Prevention Strategy 1: High Availability Setup

**Minimize downtime with redundancy:**

#### A. Active-Passive Setup (with manual failover)

```
Primary Validator (Active)
         |
         | (Manual failover only)
         |
Backup Validator (Passive, different key)
```

**Limitations:**
- Cannot use same consensus key (double-signing risk)
- Requires two validators on-chain
- Higher cost

---

#### B. Sentry Node Architecture

```
        Validator (Private)
               |
      +--------+--------+
      |        |        |
  Sentry 1  Sentry 2  Sentry 3
   (Public)  (Public)  (Public)
```

**Benefits:**
- Protects validator from DDoS
- Sentries can be easily replaced if attacked
- Validator stays online even if sentries go down (as long as ≥1 sentry is up)

**Setup:**

See [FIREWALL_SETUP.md](FIREWALL_SETUP.md#sentry-node-architecture-advanced) for configuration details.

---

### Prevention Strategy 2: Monitoring & Alerts

**Monitor validator uptime:**

```bash
# Check signing status
posd query slashing signing-info $(posd tendermint show-validator)

# Output includes:
# missed_blocks_counter: "5"  ← Missed blocks in current window
```

**Alert if missed blocks > 1000:**

```bash
#!/bin/bash
# check-missed-blocks.sh

MISSED=$(posd query slashing signing-info $(posd tendermint show-validator) -o json | jq -r '.missed_blocks_counter')

if [ "$MISSED" -gt 1000 ]; then
    echo "WARNING: Validator has missed $MISSED blocks" | mail -s "Uptime Alert" your-email@example.com
fi
```

**Use external monitoring:**

- **Uptime Robot** - HTTP monitoring (check RPC endpoint)
- **Better Uptime** - Multi-location monitoring
- **Prometheus + Alertmanager** - Metrics-based alerting (recommended)

**Prometheus alert example:**

```yaml
# In /etc/prometheus/alerts.yml
groups:
  - name: validator_uptime
    rules:
      - alert: ValidatorMissingBlocks
        expr: cometbft_consensus_missing_validators > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Validator is missing blocks"

      - alert: ValidatorBehind
        expr: increase(cometbft_consensus_height[5m]) < 10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Validator is not catching up"
```

---

### Prevention Strategy 3: Automated Restarts

**Systemd auto-restart on failure:**

Already configured in our systemd service template:

```ini
[Service]
Restart=on-failure
RestartSec=10
```

**This handles:**
- Process crashes
- Out-of-memory errors
- Panic conditions

**Test auto-restart:**
```bash
# Kill validator process
sudo pkill posd

# Check if it restarts
sudo systemctl status posd
# Should show: Active: active (running) since ...
```

---

### Prevention Strategy 4: Resource Monitoring

**Prevent downtime from resource exhaustion:**

#### Disk Space Monitoring

```bash
# Alert if disk > 85% full
df -h / | grep -vE '^Filesystem|tmpfs|cdrom' | awk '{print $5 " " $1}' | while read output; do
  usep=$(echo $output | awk '{print $1}' | cut -d'%' -f1)
  partition=$(echo $output | awk '{print $2}')
  if [ $usep -ge 85 ]; then
    echo "Disk space critical: $usep% on $partition" | mail -s "Disk Alert" your-email@example.com
  fi
done
```

#### Memory Monitoring

```bash
# Alert if RAM > 90% used
free | grep Mem | awk '{print ($3/$2) * 100.0}' | awk '{if ($1 > 90) print "Memory usage critical: " $1 "%"}'
```

#### Process Monitoring

```bash
# Check if posd is running
if ! pgrep -x posd > /dev/null; then
    echo "posd is not running!" | mail -s "Process Alert" your-email@example.com
    sudo systemctl start posd
fi
```

**Run all checks via cron:**
```bash
*/5 * * * * /home/omniphi/resource-checks.sh
```

---

## Monitoring & Alerts

### Essential Metrics to Track

1. **Block Height**
   ```bash
   curl -s localhost:26657/status | jq '.result.sync_info.latest_block_height'
   ```

2. **Catching Up Status**
   ```bash
   curl -s localhost:26657/status | jq '.result.sync_info.catching_up'
   # Should be: false
   ```

3. **Peer Count**
   ```bash
   curl -s localhost:26657/net_info | jq '.result.n_peers'
   # Should be: > 5
   ```

4. **Missed Blocks**
   ```bash
   posd query slashing signing-info $(posd tendermint show-validator) | jq '.missed_blocks_counter'
   ```

5. **Jailed Status**
   ```bash
   posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'
   # Should be: false
   ```

---

### Complete Monitoring Script

```bash
#!/bin/bash
# /home/omniphi/monitor-validator.sh

# Configuration
VALIDATOR_ADDRESS=$(posd tendermint show-validator)
ALERT_EMAIL="your-email@example.com"

# Check 1: Process running
if ! pgrep -x posd > /dev/null; then
    echo "CRITICAL: posd process not running" | mail -s "Validator Down" $ALERT_EMAIL
    sudo systemctl start posd
    exit 1
fi

# Check 2: Catching up
CATCHING_UP=$(curl -s localhost:26657/status | jq -r '.result.sync_info.catching_up')
if [ "$CATCHING_UP" = "true" ]; then
    echo "WARNING: Validator is catching up to network" | mail -s "Sync Alert" $ALERT_EMAIL
fi

# Check 3: Peer count
PEERS=$(curl -s localhost:26657/net_info | jq -r '.result.n_peers')
if [ "$PEERS" -lt 5 ]; then
    echo "WARNING: Low peer count ($PEERS)" | mail -s "Peer Alert" $ALERT_EMAIL
fi

# Check 4: Missed blocks
MISSED=$(posd query slashing signing-info $VALIDATOR_ADDRESS -o json 2>/dev/null | jq -r '.missed_blocks_counter // "0"')
if [ "$MISSED" -gt 1000 ]; then
    echo "CRITICAL: Validator has missed $MISSED blocks" | mail -s "Uptime Alert" $ALERT_EMAIL
fi

# Check 5: Jailed status
JAILED=$(posd query staking validator $(posd keys show my-validator --bech val -a) -o json 2>/dev/null | jq -r '.jailed // false')
if [ "$JAILED" = "true" ]; then
    echo "CRITICAL: Validator is jailed!" | mail -s "JAILED ALERT" $ALERT_EMAIL
fi

# Check 6: Tombstoned (double-sign)
TOMBSTONED=$(posd query slashing signing-info $VALIDATOR_ADDRESS -o json 2>/dev/null | jq -r '.tombstoned // false')
if [ "$TOMBSTONED" = "true" ]; then
    echo "CRITICAL: Validator TOMBSTONED (double-signed)" | mail -s "SLASHING ALERT" $ALERT_EMAIL
fi

echo "$(date): All checks passed"
```

**Install and run:**
```bash
chmod +x /home/omniphi/monitor-validator.sh

# Run every 5 minutes
crontab -e
*/5 * * * * /home/omniphi/monitor-validator.sh >> /var/log/validator-monitor.log 2>&1
```

---

## Recovery from Slashing

### Recovering from Downtime Slashing

**1. Check if jailed:**
```bash
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'
# If true, continue
```

**2. Wait for unjail period:**
```bash
posd query slashing signing-info $(posd tendermint show-validator) | jq '.jailed_until'
# Must wait until this time passes
```

**3. Unjail validator:**
```bash
posd tx slashing unjail \
  --from=my-validator-wallet \
  --chain-id=omniphi-1 \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni
```

**4. Verify unjailed:**
```bash
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'
# Should be: false
```

---

### Recovering from Double-Signing (Cannot Unjail)

**If tombstoned:**
```bash
posd query slashing signing-info $(posd tendermint show-validator) | jq '.tombstoned'
# If true, validator is permanently removed
```

**Only option: Create new validator**

1. **Stop old validator permanently:**
```bash
sudo systemctl stop posd
sudo systemctl disable posd
```

2. **Generate new consensus key:**
```bash
# On new server or after cleaning old data:
rm -rf ~/.omniphi
posd init new-validator --chain-id omniphi-1
```

3. **Create new validator on-chain:**
```bash
posd tx staking create-validator \
  --amount=1000000000uomni \
  --pubkey=$(posd tendermint show-validator) \
  --moniker="new-validator" \
  --chain-id=omniphi-1 \
  --from=my-wallet \
  ... (full create-validator command)
```

**Note:** You'll lose accumulated reputation, voting power history, and commission from old validator.

---

## Best Practices Summary

### ✅ Double-Signing Prevention
- [ ] Never run two validators with same key
- [ ] Always backup state file with key
- [ ] Use manual or time-delayed failover (never instant)
- [ ] Consider Horcrux for distributed signing
- [ ] Use tmkms with HSM for high-value validators

### ✅ Downtime Prevention
- [ ] Monitor uptime metrics (missed blocks, peer count)
- [ ] Set up alerts for early warning
- [ ] Use systemd auto-restart
- [ ] Implement sentry node architecture
- [ ] Monitor resource usage (disk, RAM, CPU)

### ✅ Monitoring
- [ ] Run monitoring script every 5 minutes
- [ ] Set up Prometheus + Grafana dashboard
- [ ] Configure external uptime monitoring
- [ ] Alert to multiple channels (email, SMS, Telegram)

### ✅ Operational Security
- [ ] Regular backups (key + state file)
- [ ] Document failover procedures
- [ ] Test failover in testnet first
- [ ] Keep emergency contact list
- [ ] Have recovery plan ready

---

**Slashing is permanent and irreversible. Prevention is the only cure.**

For questions, contact Omniphi validators community: https://discord.gg/omniphi
