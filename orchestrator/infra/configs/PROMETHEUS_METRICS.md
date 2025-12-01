# Prometheus Metrics Configuration for Omniphi Validators

**Purpose:** Monitor validator health, performance, and network participation using Prometheus and Grafana.

---

## Overview

Omniphi validators expose Prometheus metrics on multiple endpoints:
- **CometBFT metrics** (consensus, networking, blockchain): Port `26660`
- **Application metrics** (transactions, gas, modules): Port `1317/metrics`
- **System metrics** (CPU, RAM, disk): Via `node_exporter` on port `9100`

---

## Quick Start

### 1. Enable Metrics on Validator

**Edit `~/.omniphi/config/config.toml`:**
```toml
#######################################################
###       Instrumentation Configuration Options     ###
#######################################################
[instrumentation]

# Enable Prometheus metrics
prometheus = true

# Address to listen for Prometheus collector(s) connections
prometheus_listen_addr = ":26660"

# Maximum number of simultaneous connections
max_open_connections = 3

# Instrumentation namespace
namespace = "cometbft"
```

**Edit `~/.omniphi/config/app.toml`:**
```toml
###############################################################################
###                         Telemetry Configuration                         ###
###############################################################################
[telemetry]

# Enable application telemetry
enabled = true

# Enable prefixing gauge values with hostname
enable-hostname = false

# Enable adding hostname to labels
enable-hostname-label = false

# Enable adding service to labels
enable-service-label = false

# PrometheusRetentionTime, when positive, enables a Prometheus metrics sink (in seconds)
prometheus-retention-time = 60

# GlobalLabels defines a global set of name/value label tuples applied to all metrics
global-labels = [
  ["chain_id", "omniphi-1"],
  ["moniker", "my-validator"]
]
```

**Restart validator:**
```bash
sudo systemctl restart posd
```

### 2. Verify Metrics Endpoint

```bash
# Test CometBFT metrics
curl http://localhost:26660/metrics

# Should return Prometheus-format metrics like:
# cometbft_consensus_height 12345
# cometbft_consensus_validators 100
# cometbft_p2p_peers 42
```

---

## Installing Prometheus

### On Ubuntu/Debian

```bash
# Create prometheus user
sudo useradd --no-create-home --shell /bin/false prometheus

# Download Prometheus (check for latest version at https://prometheus.io/download/)
cd /tmp
wget https://github.com/prometheus/prometheus/releases/download/v2.48.0/prometheus-2.48.0.linux-amd64.tar.gz
tar xvf prometheus-2.48.0.linux-amd64.tar.gz
cd prometheus-2.48.0.linux-amd64

# Install binaries
sudo cp prometheus /usr/local/bin/
sudo cp promtool /usr/local/bin/
sudo chown prometheus:prometheus /usr/local/bin/prometheus
sudo chown prometheus:prometheus /usr/local/bin/promtool

# Create directories
sudo mkdir -p /etc/prometheus
sudo mkdir -p /var/lib/prometheus
sudo chown prometheus:prometheus /etc/prometheus
sudo chown prometheus:prometheus /var/lib/prometheus

# Copy console files
sudo cp -r consoles /etc/prometheus
sudo cp -r console_libraries /etc/prometheus
sudo chown -R prometheus:prometheus /etc/prometheus/consoles
sudo chown -R prometheus:prometheus /etc/prometheus/console_libraries
```

### Create Prometheus Configuration

**Create `/etc/prometheus/prometheus.yml`:**
```yaml
# Prometheus configuration for Omniphi Validator
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    monitor: 'omniphi-validator'
    chain: 'omniphi-1'

# Scrape configurations
scrape_configs:
  # CometBFT consensus metrics
  - job_name: 'cometbft'
    static_configs:
      - targets: ['localhost:26660']
        labels:
          instance: 'my-validator'
          type: 'consensus'

  # Application metrics (if exposed on API server)
  - job_name: 'cosmos-sdk'
    static_configs:
      - targets: ['localhost:1317']
        labels:
          instance: 'my-validator'
          type: 'application'

  # Node exporter (system metrics)
  - job_name: 'node'
    static_configs:
      - targets: ['localhost:9100']
        labels:
          instance: 'my-validator'
          type: 'system'

  # Prometheus itself
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
```

**Set permissions:**
```bash
sudo chown prometheus:prometheus /etc/prometheus/prometheus.yml
```

### Create Systemd Service

**Create `/etc/systemd/system/prometheus.service`:**
```ini
[Unit]
Description=Prometheus Monitoring System
Wants=network-online.target
After=network-online.target

[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/var/lib/prometheus/ \
  --web.console.templates=/etc/prometheus/consoles \
  --web.console.libraries=/etc/prometheus/console_libraries \
  --web.listen-address=0.0.0.0:9090 \
  --storage.tsdb.retention.time=30d

Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Start Prometheus:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable prometheus
sudo systemctl start prometheus

# Check status
sudo systemctl status prometheus

# View logs
sudo journalctl -u prometheus -f
```

**Verify Prometheus is running:**
```bash
# Check web interface
curl http://localhost:9090/metrics

# Or open in browser:
# http://your-server-ip:9090
```

---

## Installing Node Exporter (System Metrics)

```bash
# Download node_exporter
cd /tmp
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar xvf node_exporter-1.7.0.linux-amd64.tar.gz
cd node_exporter-1.7.0.linux-amd64

# Install binary
sudo cp node_exporter /usr/local/bin/
sudo useradd --no-create-home --shell /bin/false node_exporter
sudo chown node_exporter:node_exporter /usr/local/bin/node_exporter
```

**Create `/etc/systemd/system/node_exporter.service`:**
```ini
[Unit]
Description=Node Exporter
Wants=network-online.target
After=network-online.target

[Service]
User=node_exporter
Group=node_exporter
Type=simple
ExecStart=/usr/local/bin/node_exporter \
  --collector.systemd \
  --collector.processes

Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Start node_exporter:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable node_exporter
sudo systemctl start node_exporter
sudo systemctl status node_exporter

# Test
curl http://localhost:9100/metrics
```

---

## Installing Grafana (Visualization)

### On Ubuntu/Debian

```bash
# Add Grafana GPG key and repository
sudo apt-get install -y software-properties-common
wget -q -O - https://packages.grafana.com/gpg.key | sudo apt-key add -
echo "deb https://packages.grafana.com/oss/deb stable main" | sudo tee /etc/apt/sources.list.d/grafana.list

# Install Grafana
sudo apt-get update
sudo apt-get install -y grafana

# Start Grafana
sudo systemctl daemon-reload
sudo systemctl enable grafana-server
sudo systemctl start grafana-server
sudo systemctl status grafana-server
```

**Access Grafana:**
- URL: `http://your-server-ip:3000`
- Default login: `admin` / `admin` (change on first login)

### Configure Prometheus as Data Source

1. Open Grafana: `http://your-server-ip:3000`
2. Login with `admin` / `admin`
3. Go to **Configuration** → **Data Sources**
4. Click **Add data source**
5. Select **Prometheus**
6. Configure:
   - **URL:** `http://localhost:9090`
   - **Access:** Server (default)
7. Click **Save & Test**

---

## Import Cosmos Validator Dashboard

### Pre-built Dashboard

**Use the official Cosmos Validator dashboard from Grafana:**

1. In Grafana, click **+** → **Import**
2. Enter dashboard ID: **11036** (Cosmos Validator Dashboard)
3. Click **Load**
4. Select your Prometheus data source
5. Click **Import**

**Or manually create custom panels (see below).**

---

## Key Metrics to Monitor

### 1. Consensus & Blockchain Health

**Block Height:**
```promql
cometbft_consensus_height
```

**Blocks per minute:**
```promql
rate(cometbft_consensus_height[1m]) * 60
```

**Validator voting power:**
```promql
cometbft_consensus_validators_power{validator_address="your_address"}
```

**Missing blocks (rounds):**
```promql
cometbft_consensus_missing_validators
```

**Consensus rounds (should be mostly 0):**
```promql
cometbft_consensus_rounds
```

### 2. Networking & Peers

**Number of peers:**
```promql
cometbft_p2p_peers
```

**Peer connectivity over time:**
```promql
avg_over_time(cometbft_p2p_peers[5m])
```

**Inbound/outbound peer count:**
```promql
cometbft_p2p_peer_receive_bytes_total
cometbft_p2p_peer_send_bytes_total
```

### 3. Mempool

**Mempool size (transactions):**
```promql
cometbft_mempool_size
```

**Mempool bytes:**
```promql
cometbft_mempool_size_bytes
```

**Failed transactions:**
```promql
cometbft_mempool_failed_txs
```

### 4. Performance

**Block processing time:**
```promql
cometbft_consensus_block_interval_seconds
```

**State sync status:**
```promql
cometbft_statesync_syncing
```

**Fast sync status:**
```promql
cometbft_consensus_fast_syncing
```

### 5. System Resources

**CPU usage:**
```promql
100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

**Memory usage:**
```promql
(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100
```

**Disk usage:**
```promql
(1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100
```

**Disk I/O:**
```promql
rate(node_disk_io_time_seconds_total[5m])
```

**Network traffic:**
```promql
rate(node_network_receive_bytes_total[5m])
rate(node_network_transmit_bytes_total[5m])
```

---

## Alert Rules

### Create Alert Rules File

**Create `/etc/prometheus/alerts.yml`:**
```yaml
groups:
  - name: omniphi_validator_alerts
    interval: 30s
    rules:
      # Validator is not catching up with the network
      - alert: ValidatorBehind
        expr: increase(cometbft_consensus_height[5m]) < 10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Validator is falling behind"
          description: "Block height increased by less than 10 in the last 5 minutes"

      # Low peer count
      - alert: LowPeerCount
        expr: cometbft_p2p_peers < 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low peer count"
          description: "Peer count is {{ $value }}, below threshold of 5"

      # High memory usage
      - alert: HighMemoryUsage
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }}%"

      # High disk usage
      - alert: HighDiskUsage
        expr: (1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100 > 85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High disk usage"
          description: "Disk usage is {{ $value }}%"

      # Validator stopped producing blocks
      - alert: ValidatorDown
        expr: up{job="cometbft"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Validator is down"
          description: "CometBFT metrics endpoint is unreachable"

      # Mempool growing too large
      - alert: MempoolOverload
        expr: cometbft_mempool_size > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Mempool overload"
          description: "Mempool size is {{ $value }} transactions"
```

**Update `/etc/prometheus/prometheus.yml` to include alerts:**
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

# Load alert rules
rule_files:
  - "alerts.yml"

scrape_configs:
  # ... (existing configs)
```

**Reload Prometheus:**
```bash
sudo systemctl reload prometheus
```

**View alerts in Prometheus UI:**
- Open: `http://your-server-ip:9090/alerts`

---

## Custom Grafana Panels

### Panel: Block Height
```json
{
  "title": "Block Height",
  "targets": [
    {
      "expr": "cometbft_consensus_height",
      "legendFormat": "Block Height"
    }
  ]
}
```

### Panel: Peer Count
```json
{
  "title": "Peer Count",
  "targets": [
    {
      "expr": "cometbft_p2p_peers",
      "legendFormat": "Peers"
    }
  ]
}
```

### Panel: System Resources
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
  ]
}
```

---

## Security Considerations

### Restrict Prometheus Access

**Option 1: Firewall (iptables)**
```bash
# Allow Prometheus only from specific IP (e.g., monitoring server)
sudo iptables -A INPUT -p tcp --dport 9090 -s MONITORING_SERVER_IP -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 9090 -j DROP

# Save rules
sudo netfilter-persistent save
```

**Option 2: SSH Tunnel**
```bash
# From your local machine:
ssh -L 9090:localhost:9090 -L 3000:localhost:3000 user@validator-server

# Access Prometheus: http://localhost:9090
# Access Grafana: http://localhost:3000
```

**Option 3: Nginx Reverse Proxy with Basic Auth**
```bash
# Install nginx and apache2-utils
sudo apt-get install -y nginx apache2-utils

# Create password file
sudo htpasswd -c /etc/nginx/.htpasswd admin

# Configure nginx
sudo nano /etc/nginx/sites-available/prometheus
```

**Nginx config:**
```nginx
server {
    listen 80;
    server_name metrics.your-domain.com;

    location / {
        auth_basic "Prometheus";
        auth_basic_user_file /etc/nginx/.htpasswd;
        proxy_pass http://localhost:9090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/prometheus /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## Troubleshooting

### Prometheus can't scrape validator metrics

**Check metrics endpoint:**
```bash
curl http://localhost:26660/metrics
```

**Check Prometheus targets:**
- Open: `http://your-server-ip:9090/targets`
- All targets should be "UP"

**Check firewall:**
```bash
sudo ufw status
# Ensure ports 26660, 9090, 9100 are allowed
```

### Grafana shows "No Data"

**Check Prometheus data source:**
- Grafana → Configuration → Data Sources → Prometheus
- Click "Test" → should be green checkmark

**Check queries:**
- Use Prometheus UI to test queries first
- Ensure metric names match your configuration

### High cardinality warnings

**Reduce label usage:**
```toml
# In app.toml, remove unnecessary labels:
[telemetry]
enable-hostname-label = false
enable-service-label = false
```

---

## Best Practices

1. **Retention:** Keep 30 days of metrics (`--storage.tsdb.retention.time=30d`)
2. **Scrape interval:** 15 seconds is sufficient for validators
3. **Alerts:** Set up critical alerts (validator down, low peers, disk full)
4. **Backup:** Regularly backup Prometheus data (`/var/lib/prometheus`)
5. **Security:** Restrict access to Prometheus/Grafana endpoints
6. **Resources:** Allocate at least 2GB RAM for Prometheus on busy validators
7. **Monitoring the monitor:** Set up external uptime monitoring for your Prometheus instance

---

## Example Grafana Dashboard JSON

Save this to import a basic Omniphi validator dashboard:

[See full dashboard JSON in the GitHub repository: `infra/grafana/omniphi-validator-dashboard.json`]

---

**Summary:**
- Enable metrics in `config.toml` and `app.toml`
- Install Prometheus, node_exporter, and Grafana
- Configure scrape targets and alert rules
- Import or create custom dashboards
- Secure access with firewalls or reverse proxies

**Recommended monitoring stack:**
- **Prometheus:** Metrics collection (port 9090)
- **Node Exporter:** System metrics (port 9100)
- **Grafana:** Visualization (port 3000)
- **Alertmanager:** Alert routing (optional, port 9093)
