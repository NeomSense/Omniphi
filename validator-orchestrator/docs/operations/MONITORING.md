# Comprehensive Monitoring Guide for Omniphi Validators

**Essential guide for monitoring validator health, performance, and preventing downtime.**

---

## Overview

**Why monitoring matters:**
- **Prevent slashing** - Detect downtime before jailing
- **Optimize performance** - Identify bottlenecks
- **Security** - Detect attacks early
- **Compliance** - Track SLA metrics

**Monitoring layers:**
1. **Application** - Validator-specific metrics (block height, missed blocks)
2. **System** - Server resources (CPU, RAM, disk, network)
3. **Network** - Connectivity and peers
4. **External** - Third-party uptime monitoring

---

## Quick Start: Essential Monitoring

### 1. Enable Prometheus Metrics

**Already configured in our templates!**

Verify metrics are enabled:

```bash
# Check CometBFT metrics
curl http://localhost:26660/metrics | head -20

# Should see metrics like:
# cometbft_consensus_height 123456
# cometbft_consensus_validators 100
# cometbft_p2p_peers 15
```

**If not enabled, edit `~/.omniphi/config/config.toml`:**

```toml
[instrumentation]
prometheus = true
prometheus_listen_addr = ":26660"
```

---

### 2. Quick Health Check Script

```bash
#!/bin/bash
# /home/omniphi/scripts/quick-health-check.sh

echo "=== Omniphi Validator Health Check ==="
echo ""

# 1. Process running?
if pgrep -x posd > /dev/null; then
    echo "✓ Process: posd is running"
else
    echo "✗ Process: posd is NOT running"
    exit 1
fi

# 2. Catching up?
CATCHING_UP=$(curl -s localhost:26657/status | jq -r '.result.sync_info.catching_up')
if [ "$CATCHING_UP" = "false" ]; then
    echo "✓ Sync: Caught up with network"
else
    echo "⚠ Sync: Still catching up"
fi

# 3. Block height
LATEST_HEIGHT=$(curl -s localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
echo "ℹ Block Height: $LATEST_HEIGHT"

# 4. Peer count
PEERS=$(curl -s localhost:26657/net_info | jq -r '.result.n_peers')
if [ "$PEERS" -gt 5 ]; then
    echo "✓ Peers: $PEERS connected"
else
    echo "⚠ Peers: Only $PEERS connected (low)"
fi

# 5. Missed blocks
VALIDATOR_ADDR=$(posd tendermint show-validator)
MISSED=$(posd query slashing signing-info $VALIDATOR_ADDR -o json 2>/dev/null | jq -r '.missed_blocks_counter // "unknown"')
if [ "$MISSED" != "unknown" ] && [ "$MISSED" -lt 100 ]; then
    echo "✓ Missed Blocks: $MISSED (good)"
elif [ "$MISSED" != "unknown" ]; then
    echo "⚠ Missed Blocks: $MISSED (monitor closely)"
else
    echo "ℹ Missed Blocks: Unable to query"
fi

# 6. Disk space
DISK_USED=$(df -h ~/.omniphi | tail -1 | awk '{print $5}' | sed 's/%//')
if [ "$DISK_USED" -lt 85 ]; then
    echo "✓ Disk: ${DISK_USED}% used"
else
    echo "⚠ Disk: ${DISK_USED}% used (running low)"
fi

# 7. Memory
MEM_USED=$(free | grep Mem | awk '{printf "%.0f", ($3/$2) * 100.0}')
if [ "$MEM_USED" -lt 90 ]; then
    echo "✓ Memory: ${MEM_USED}% used"
else
    echo "⚠ Memory: ${MEM_USED}% used (high)"
fi

echo ""
echo "=== Health Check Complete ==="
```

**Run manually or via cron:**
```bash
chmod +x ~/scripts/quick-health-check.sh
~/scripts/quick-health-check.sh

# Schedule every 5 minutes
crontab -e
*/5 * * * * /home/omniphi/scripts/quick-health-check.sh >> /var/log/health-check.log 2>&1
```

---

## Prometheus + Grafana Setup

**Full monitoring stack installation:** (See [infra/configs/PROMETHEUS_METRICS.md](../../infra/configs/PROMETHEUS_METRICS.md) for complete guide)

### Quick Prometheus Setup

```bash
# Install Prometheus
cd /tmp
wget https://github.com/prometheus/prometheus/releases/download/v2.48.0/prometheus-2.48.0.linux-amd64.tar.gz
tar xvf prometheus-2.48.0.linux-amd64.tar.gz
sudo cp prometheus-2.48.0.linux-amd64/prometheus /usr/local/bin/
sudo cp prometheus-2.48.0.linux-amd64/promtool /usr/local/bin/

# Create user and directories
sudo useradd --no-create-home --shell /bin/false prometheus
sudo mkdir -p /etc/prometheus /var/lib/prometheus
sudo chown prometheus:prometheus /etc/prometheus /var/lib/prometheus
```

**Create `/etc/prometheus/prometheus.yml`:**
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'cometbft'
    static_configs:
      - targets: ['localhost:26660']
        labels:
          instance: 'my-validator'

  - job_name: 'node_exporter'
    static_configs:
      - targets: ['localhost:9100']
```

**Create systemd service** `/etc/systemd/system/prometheus.service`:
```ini
[Unit]
Description=Prometheus
After=network.target

[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/var/lib/prometheus/ \
  --storage.tsdb.retention.time=30d
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable prometheus
sudo systemctl start prometheus
```

---

## Key Metrics to Monitor

### 1. Validator-Specific Metrics

#### Block Height
```bash
# CLI
posd status | jq '.SyncInfo.latest_block_height'

# Prometheus metric
cometbft_consensus_height
```

**Alert if:** Height not increasing for 5+ minutes

---

#### Missed Blocks
```bash
# CLI
VALIDATOR_ADDR=$(posd tendermint show-validator)
posd query slashing signing-info $VALIDATOR_ADDR | jq '.missed_blocks_counter'

# Monitor in Prometheus (custom exporter needed)
```

**Alert if:** Missed blocks > 1000 (approaching jailing threshold)

---

#### Voting Power
```bash
# CLI
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.tokens'

# Check if in active set
posd query staking validators | jq '.validators[] | select(.operator_address=="YOUR_VALOPER")'
```

**Alert if:** Voting power decreases unexpectedly (delegations withdrawn)

---

#### Jailed Status
```bash
# CLI
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'

# Prometheus query (custom exporter)
validator_jailed{moniker="my-validator"}
```

**Alert if:** `jailed = true`

---

### 2. Consensus Metrics

#### Consensus Round
```promql
cometbft_consensus_rounds
```

**Normal:** Should be 0 most of the time
**Alert if:** Rounds > 0 frequently (indicates network issues)

---

#### Block Interval
```promql
cometbft_consensus_block_interval_seconds
```

**Normal:** ~6 seconds for Omniphi
**Alert if:** Block time > 10 seconds

---

#### Validator Power
```promql
cometbft_consensus_validators_power
```

**Monitor:** Track your validator's voting power over time

---

### 3. Network Metrics

#### Peer Count
```promql
cometbft_p2p_peers
```

**Normal:** 10-50 peers
**Alert if:** < 5 peers (connectivity issues)

---

#### Network Traffic
```promql
rate(cometbft_p2p_peer_receive_bytes_total[5m])
rate(cometbft_p2p_peer_send_bytes_total[5m])
```

**Monitor:** Unusual spikes may indicate attack

---

### 4. System Metrics (via node_exporter)

#### CPU Usage
```promql
100 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

**Normal:** 30-70%
**Alert if:** > 90% sustained

---

#### Memory Usage
```promql
(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100
```

**Normal:** 50-80%
**Alert if:** > 95%

---

#### Disk Usage
```promql
(1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100
```

**Normal:** < 80%
**Alert if:** > 85%

---

#### Disk I/O
```promql
rate(node_disk_io_time_seconds_total[5m])
```

**Monitor:** High I/O may indicate need for better disk (SSD → NVMe)

---

## Alerting Setup

### Prometheus Alertmanager

**Install Alertmanager:**
```bash
cd /tmp
wget https://github.com/prometheus/alertmanager/releases/download/v0.26.0/alertmanager-0.26.0.linux-amd64.tar.gz
tar xvf alertmanager-0.26.0.linux-amd64.tar.gz
sudo cp alertmanager-0.26.0.linux-amd64/alertmanager /usr/local/bin/
```

**Create `/etc/prometheus/alertmanager.yml`:**
```yaml
global:
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@yourdomain.com'
  smtp_auth_username: 'your-email@gmail.com'
  smtp_auth_password: 'your-app-password'

route:
  receiver: 'email-alerts'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h

receivers:
  - name: 'email-alerts'
    email_configs:
      - to: 'your-email@gmail.com'
        headers:
          Subject: 'Validator Alert: {{ .GroupLabels.alertname }}'
```

**Create alert rules** `/etc/prometheus/alerts.yml`:
```yaml
groups:
  - name: validator_alerts
    interval: 30s
    rules:
      # Critical: Validator down
      - alert: ValidatorDown
        expr: up{job="cometbft"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Validator process is down"
          description: "CometBFT metrics endpoint unreachable for 1 minute"

      # Critical: Not catching up
      - alert: ValidatorBehind
        expr: increase(cometbft_consensus_height[5m]) < 10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Validator falling behind"
          description: "Block height increased by less than 10 in 5 minutes"

      # Warning: Low peer count
      - alert: LowPeerCount
        expr: cometbft_p2p_peers < 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low peer count"
          description: "Peer count is {{ $value }}, below threshold of 5"

      # Warning: High memory usage
      - alert: HighMemoryUsage
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }}%"

      # Warning: High disk usage
      - alert: HighDiskUsage
        expr: (1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100 > 85
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High disk usage"
          description: "Disk usage is {{ $value }}% on {{ $labels.device }}"

      # Critical: Disk almost full
      - alert: DiskCritical
        expr: (1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100 > 95
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Disk critically full"
          description: "Disk usage is {{ $value }}% - immediate action required"
```

**Update Prometheus config** to include alerts:
```yaml
# In /etc/prometheus/prometheus.yml
rule_files:
  - "alerts.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['localhost:9093']
```

```bash
# Reload Prometheus
sudo systemctl reload prometheus
```

---

## External Monitoring

### Uptime Robot (Free tier available)

**Setup:**
1. Sign up: https://uptimerobot.com
2. Create monitor:
   - Type: HTTP(s)
   - URL: `http://YOUR_VALIDATOR_IP:26657/health`
   - Interval: 5 minutes
   - Alert contacts: Your email/SMS

---

### Better Uptime (More features)

**Setup:**
1. Sign up: https://betteruptime.com
2. Create heartbeat monitor:
   - Create heartbeat URL
   - Configure validator to ping URL every 5 minutes

**Validator heartbeat script:**
```bash
#!/bin/bash
# /home/omniphi/scripts/heartbeat.sh

HEARTBEAT_URL="https://betteruptime.com/api/v1/heartbeat/YOUR_ID"

# Check if validator is healthy
if pgrep -x posd > /dev/null && \
   [ "$(curl -s localhost:26657/status | jq -r '.result.sync_info.catching_up')" = "false" ]; then
    curl -X POST "$HEARTBEAT_URL"
fi
```

```bash
# Schedule every 5 minutes
crontab -e
*/5 * * * * /home/omniphi/scripts/heartbeat.sh
```

---

## Grafana Dashboards

### Import Pre-Built Dashboard

**Cosmos Validator Dashboard (ID: 11036):**

1. Install Grafana (see [PROMETHEUS_METRICS.md](../../infra/configs/PROMETHEUS_METRICS.md))
2. Open Grafana: `http://your-server:3000`
3. Click **+** → **Import**
4. Enter ID: **11036**
5. Select Prometheus data source
6. Click **Import**

---

### Custom Dashboard Panels

**Panel: Block Height Over Time**
```json
{
  "title": "Block Height",
  "targets": [{
    "expr": "cometbft_consensus_height",
    "legendFormat": "Block Height"
  }],
  "type": "graph"
}
```

**Panel: Peer Count**
```json
{
  "title": "Peer Count",
  "targets": [{
    "expr": "cometbft_p2p_peers",
    "legendFormat": "Peers"
  }],
  "type": "stat"
}
```

**Panel: System Resources**
```json
{
  "title": "CPU & Memory",
  "targets": [
    {
      "expr": "100 - (avg(irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)",
      "legendFormat": "CPU %"
    },
    {
      "expr": "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100",
      "legendFormat": "Memory %"
    }
  ],
  "type": "graph"
}
```

---

## Log Monitoring

### Centralized Logging (Loki + Grafana)

**Install Loki:**
```bash
cd /tmp
wget https://github.com/grafana/loki/releases/download/v2.9.0/loki-linux-amd64.zip
unzip loki-linux-amd64.zip
sudo cp loki-linux-amd64 /usr/local/bin/loki
```

**Install Promtail (log shipper):**
```bash
wget https://github.com/grafana/loki/releases/download/v2.9.0/promtail-linux-amd64.zip
unzip promtail-linux-amd64.zip
sudo cp promtail-linux-amd64 /usr/local/bin/promtail
```

**Configure Promtail** `/etc/promtail/config.yml`:
```yaml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://localhost:3100/loki/api/v1/push

scrape_configs:
  - job_name: validator
    static_configs:
      - targets:
          - localhost
        labels:
          job: validator
          __path__: /var/log/validator.log
```

---

### Real-Time Log Analysis

**Watch for errors:**
```bash
# Real-time error monitoring
sudo journalctl -u posd -f | grep -iE "error|panic|fail"

# Count errors in last hour
sudo journalctl -u posd --since "1 hour ago" | grep -ic error
```

**Alert on critical errors:**
```bash
#!/bin/bash
# /home/omniphi/scripts/log-alert.sh

ERROR_COUNT=$(sudo journalctl -u posd --since "5 minutes ago" | grep -ic "error")

if [ "$ERROR_COUNT" -gt 10 ]; then
    echo "High error rate: $ERROR_COUNT errors in last 5 minutes" | \
      mail -s "Validator Log Alert" your-email@example.com
fi
```

---

## Monitoring Checklist

### Daily Checks
- [ ] Validator signing blocks (check block explorer)
- [ ] No critical alerts triggered
- [ ] Peer count healthy (>10)
- [ ] Disk space < 80%

### Weekly Checks
- [ ] Review Grafana dashboards
- [ ] Check missed blocks counter
- [ ] Review system resource trends
- [ ] Test alerting (trigger test alert)

### Monthly Checks
- [ ] Review alert history and tune thresholds
- [ ] Update monitoring dashboards
- [ ] Test failover procedures
- [ ] Review and optimize metrics retention

---

## Troubleshooting

### Metrics Not Showing in Prometheus

**Check target status:**
```bash
# Open Prometheus UI
http://your-server:9090/targets

# All targets should be "UP"
```

**If DOWN:**
```bash
# Check firewall
sudo ufw status | grep 26660

# Test metrics endpoint
curl http://localhost:26660/metrics
```

---

### Alerts Not Triggering

**Check Alertmanager:**
```bash
# View Alertmanager status
http://your-server:9093

# Test alert manually
amtool alert add alertname="test" severity="warning"
```

---

## Summary

**Essential monitoring setup:**
1. ✅ Prometheus + node_exporter installed
2. ✅ Grafana dashboard configured
3. ✅ Alertmanager for email/SMS alerts
4. ✅ External uptime monitoring
5. ✅ Daily health checks automated

**Key metrics to watch:**
- Block height (always increasing)
- Missed blocks (< 1000)
- Peer count (> 5)
- Disk space (< 85%)
- Memory usage (< 90%)

**Alerting priorities:**
- **Critical:** Validator down, not catching up, disk full
- **Warning:** Low peers, high resource usage, missed blocks increasing

---

**For more operational guides:**
- [STATE_SYNC.md](STATE_SYNC.md) - Fast blockchain sync
- [BACKUPS.md](BACKUPS.md) - Backup and restore
- [../../infra/configs/PROMETHEUS_METRICS.md](../../infra/configs/PROMETHEUS_METRICS.md) - Complete Prometheus setup

---

**Need help?** Ask in Omniphi Discord: https://discord.gg/omniphi
