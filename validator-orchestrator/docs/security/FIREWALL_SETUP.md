# Validator Firewall Configuration Guide

**Essential security guide for protecting your Omniphi validator with proper firewall rules.**

---

## Overview

A properly configured firewall is your first line of defense against attacks. This guide covers firewall setup for:
- **Linux (UFW)** - Ubuntu/Debian
- **Linux (iptables)** - Manual configuration
- **Cloud providers** - AWS, DigitalOcean, GCP
- **Advanced setups** - Sentry nodes, DDoS protection

---

## Port Reference

### Required Ports (Validator)

| Port | Service | Protocol | Expose | Purpose |
|------|---------|----------|--------|---------|
| **26656** | P2P | TCP | ✅ Public | Peer-to-peer networking (REQUIRED) |
| **22** | SSH | TCP | ⚠️ Restricted | Remote server access |

### Optional Ports (Based on Configuration)

| Port | Service | Protocol | Expose | Purpose |
|------|---------|----------|--------|---------|
| **26657** | RPC | TCP | ❌ Localhost only | CometBFT RPC (queries, monitoring) |
| **26660** | Prometheus | TCP | ⚠️ Monitoring server | Metrics endpoint |
| **1317** | REST API | TCP | ❌ Localhost only | Cosmos SDK REST API |
| **9090** | gRPC | TCP | ⚠️ Optional | Cosmos SDK gRPC |
| **9091** | gRPC-Web | TCP | ❌ No | Web client gRPC |

**Legend:**
- ✅ **Public** - Must be open to 0.0.0.0/0
- ⚠️ **Restricted** - Open only to trusted IPs
- ❌ **Localhost only** - Bind to 127.0.0.1, not exposed

---

## Quick Start: UFW (Ubuntu/Debian)

### 1. Install UFW

```bash
# Install UFW (usually pre-installed on Ubuntu)
sudo apt-get update
sudo apt-get install -y ufw
```

### 2. Configure Default Policies

```bash
# Default: deny all incoming, allow all outgoing
sudo ufw default deny incoming
sudo ufw default allow outgoing
```

### 3. Allow Essential Services

```bash
# Allow SSH (BEFORE enabling firewall!)
# Replace 22 with your custom SSH port if changed
sudo ufw allow 22/tcp comment 'SSH'

# Allow P2P networking (required for validator)
sudo ufw allow 26656/tcp comment 'Omniphi P2P'

# Optional: Allow Prometheus from monitoring server only
sudo ufw allow from MONITORING_SERVER_IP to any port 26660 proto tcp comment 'Prometheus metrics'

# Optional: Allow RPC from specific IP (for your own monitoring)
sudo ufw allow from YOUR_MANAGEMENT_IP to any port 26657 proto tcp comment 'CometBFT RPC'
```

### 4. Enable Firewall

```bash
# Enable UFW
sudo ufw enable

# Verify rules
sudo ufw status verbose
```

**Expected output:**
```
Status: active
Logging: on (low)
Default: deny (incoming), allow (outgoing), disabled (routed)

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW IN    Anywhere                   # SSH
26656/tcp                  ALLOW IN    Anywhere                   # Omniphi P2P
26660/tcp                  ALLOW IN    10.0.1.5                   # Prometheus metrics
26657/tcp                  ALLOW IN    203.0.113.10               # CometBFT RPC
```

---

## Advanced UFW Configuration

### Rate Limiting (Prevent Brute Force)

```bash
# Limit SSH connections (max 6 attempts per 30 seconds)
sudo ufw limit 22/tcp comment 'SSH rate limit'

# This replaces the basic "allow" rule
# Delete old rule first:
sudo ufw delete allow 22/tcp
```

### Allow Specific IP Ranges

```bash
# Allow entire subnet (e.g., your office network)
sudo ufw allow from 192.168.1.0/24 to any port 22 proto tcp comment 'Office network SSH'

# Allow Cloudflare IPs (if using Cloudflare for RPC)
# Get latest IPs: https://www.cloudflare.com/ips/
sudo ufw allow from 173.245.48.0/20 to any port 26657 proto tcp comment 'Cloudflare'
```

### Logging Configuration

```bash
# Enable verbose logging
sudo ufw logging high

# View logs
sudo tail -f /var/log/ufw.log

# Disable logging (if too verbose)
sudo ufw logging off
```

### Delete Rules

```bash
# List rules with numbers
sudo ufw status numbered

# Delete rule by number
sudo ufw delete 3

# Delete rule by specification
sudo ufw delete allow 26657/tcp
```

---

## Manual iptables Configuration

### Save Current Rules (Backup)

```bash
# Backup current iptables rules
sudo iptables-save > ~/iptables-backup-$(date +%Y%m%d).rules

# Restore from backup
sudo iptables-restore < ~/iptables-backup-20251120.rules
```

### Basic iptables Setup

```bash
# Flush existing rules
sudo iptables -F
sudo iptables -X

# Default policies
sudo iptables -P INPUT DROP
sudo iptables -P FORWARD DROP
sudo iptables -P OUTPUT ACCEPT

# Allow loopback
sudo iptables -A INPUT -i lo -j ACCEPT

# Allow established connections
sudo iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# Allow SSH
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# Allow P2P
sudo iptables -A INPUT -p tcp --dport 26656 -j ACCEPT

# Allow Prometheus from specific IP
sudo iptables -A INPUT -p tcp --dport 26660 -s MONITORING_IP -j ACCEPT

# Drop everything else (already default policy)
```

### Rate Limiting with iptables

```bash
# Limit SSH connections (max 3 per minute)
sudo iptables -A INPUT -p tcp --dport 22 -m state --state NEW -m recent --set
sudo iptables -A INPUT -p tcp --dport 22 -m state --state NEW -m recent --update --seconds 60 --hitcount 4 -j DROP

# Limit P2P connections (prevent DDoS)
sudo iptables -A INPUT -p tcp --dport 26656 -m connlimit --connlimit-above 50 -j REJECT
```

### Save iptables Rules (Persist After Reboot)

**Method 1: iptables-persistent (Ubuntu/Debian)**

```bash
# Install iptables-persistent
sudo apt-get install -y iptables-persistent

# Save current rules
sudo netfilter-persistent save

# Rules are saved to:
# /etc/iptables/rules.v4  (IPv4)
# /etc/iptables/rules.v6  (IPv6)

# Reload rules
sudo netfilter-persistent reload
```

**Method 2: Manual save/restore**

```bash
# Save rules
sudo iptables-save > /etc/iptables/rules.v4

# Create systemd service to restore on boot
sudo nano /etc/systemd/system/iptables-restore.service
```

**Service file:**
```ini
[Unit]
Description=Restore iptables rules
Before=network-pre.target
Wants=network-pre.target

[Service]
Type=oneshot
ExecStart=/sbin/iptables-restore /etc/iptables/rules.v4
ExecReload=/sbin/iptables-restore /etc/iptables/rules.v4
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable iptables-restore
sudo systemctl start iptables-restore
```

---

## Cloud Provider Firewall Setup

### AWS Security Groups

**Create security group:**

1. Go to **EC2 Dashboard** → **Security Groups** → **Create security group**

2. **Inbound Rules:**

| Type | Protocol | Port Range | Source | Description |
|------|----------|-----------|--------|-------------|
| SSH | TCP | 22 | Your IP/32 | SSH access |
| Custom TCP | TCP | 26656 | 0.0.0.0/0 | P2P networking |
| Custom TCP | TCP | 26660 | Monitoring SG | Prometheus |

3. **Outbound Rules:**
   - Allow all (default)

4. **Attach to EC2 instance**

**Using AWS CLI:**

```bash
# Create security group
aws ec2 create-security-group \
  --group-name omniphi-validator-sg \
  --description "Omniphi Validator Security Group" \
  --vpc-id vpc-xxxxxxxx

# Allow SSH (from your IP)
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxxxxxxx \
  --protocol tcp \
  --port 22 \
  --cidr YOUR_IP/32

# Allow P2P (from anywhere)
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxxxxxxx \
  --protocol tcp \
  --port 26656 \
  --cidr 0.0.0.0/0

# Allow Prometheus (from monitoring server)
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxxxxxxx \
  --protocol tcp \
  --port 26660 \
  --source-group sg-monitoring-xxxxxxxx
```

---

### DigitalOcean Cloud Firewall

**Create via web interface:**

1. Go to **Networking** → **Firewalls** → **Create Firewall**

2. **Inbound Rules:**
   - **SSH:** TCP, 22, Sources: Your IP
   - **P2P:** TCP, 26656, Sources: All IPv4, All IPv6

3. **Outbound Rules:**
   - All TCP, All Ports, All Destinations
   - All UDP, All Ports, All Destinations

4. **Apply to Droplet**

**Using doctl CLI:**

```bash
# Install doctl
snap install doctl

# Authenticate
doctl auth init

# Create firewall
doctl compute firewall create \
  --name omniphi-validator-fw \
  --inbound-rules "protocol:tcp,ports:22,sources:addresses:YOUR_IP" \
  --inbound-rules "protocol:tcp,ports:26656,sources:addresses:0.0.0.0/0,::/0" \
  --outbound-rules "protocol:tcp,ports:all,destinations:addresses:0.0.0.0/0,::/0" \
  --outbound-rules "protocol:udp,ports:all,destinations:addresses:0.0.0.0/0,::/0" \
  --droplet-ids DROPLET_ID
```

---

### Google Cloud Platform (GCP) Firewall

**Using gcloud CLI:**

```bash
# Allow SSH (from your IP)
gcloud compute firewall-rules create allow-ssh \
  --allow tcp:22 \
  --source-ranges YOUR_IP/32 \
  --target-tags omniphi-validator

# Allow P2P (from anywhere)
gcloud compute firewall-rules create allow-p2p \
  --allow tcp:26656 \
  --source-ranges 0.0.0.0/0 \
  --target-tags omniphi-validator

# Apply tags to instance
gcloud compute instances add-tags INSTANCE_NAME \
  --tags omniphi-validator \
  --zone us-central1-a
```

---

## DDoS Protection

### Connection Rate Limiting

**UFW:**
```bash
# Already covered with:
sudo ufw limit 22/tcp
```

**iptables:**
```bash
# Limit new connections to P2P port
sudo iptables -A INPUT -p tcp --dport 26656 -m state --state NEW -m recent --set
sudo iptables -A INPUT -p tcp --dport 26656 -m state --state NEW -m recent --update --seconds 10 --hitcount 20 -j DROP

# This allows max 20 new connections per 10 seconds
```

### SYN Flood Protection

```bash
# Limit SYN packets
sudo iptables -A INPUT -p tcp --syn -m limit --limit 1/s --limit-burst 3 -j ACCEPT
sudo iptables -A INPUT -p tcp --syn -j DROP

# Protect against SYN floods
sudo iptables -N syn_flood
sudo iptables -A INPUT -p tcp --syn -j syn_flood
sudo iptables -A syn_flood -m limit --limit 1/s --limit-burst 3 -j RETURN
sudo iptables -A syn_flood -j DROP
```

### ICMP (Ping) Flood Protection

```bash
# Limit ping requests
sudo iptables -A INPUT -p icmp --icmp-type echo-request -m limit --limit 1/s -j ACCEPT
sudo iptables -A INPUT -p icmp --icmp-type echo-request -j DROP

# Or block ping entirely (optional)
sudo iptables -A INPUT -p icmp --icmp-type echo-request -j REJECT
```

### Invalid Packet Protection

```bash
# Drop invalid packets
sudo iptables -A INPUT -m conntrack --ctstate INVALID -j DROP

# Drop NULL packets
sudo iptables -A INPUT -p tcp --tcp-flags ALL NONE -j DROP

# Drop XMAS packets
sudo iptables -A INPUT -p tcp --tcp-flags ALL ALL -j DROP
```

---

## Sentry Node Architecture (Advanced)

For high-value validators, use **sentry nodes** to shield your validator from direct internet exposure.

### Architecture

```
                    Internet
                       |
          +------------+------------+
          |            |            |
      Sentry 1     Sentry 2     Sentry 3
       (Public)     (Public)     (Public)
          |            |            |
          +------------+------------+
                       |
                   Validator
                  (Private IP)
```

### Validator Firewall (Private)

```bash
# On VALIDATOR node:

# Only allow P2P from sentry nodes (private IPs)
sudo ufw allow from 10.0.1.10 to any port 26656 proto tcp comment 'Sentry 1'
sudo ufw allow from 10.0.1.11 to any port 26656 proto tcp comment 'Sentry 2'
sudo ufw allow from 10.0.1.12 to any port 26656 proto tcp comment 'Sentry 3'

# Allow SSH from bastion host only
sudo ufw allow from 10.0.1.100 to any port 22 proto tcp comment 'Bastion host'

# Deny all other P2P traffic
sudo ufw deny 26656/tcp

sudo ufw enable
```

### Sentry Firewall (Public)

```bash
# On SENTRY nodes:

# Allow P2P from internet
sudo ufw allow 26656/tcp comment 'P2P public'

# Allow SSH from bastion only
sudo ufw allow from 10.0.1.100 to any port 22 proto tcp comment 'Bastion host'

# Allow connections to validator (outbound - usually allowed by default)

sudo ufw enable
```

### Validator config.toml (Sentry Setup)

```toml
# On VALIDATOR:
[p2p]
pex = false  # Disable peer exchange
persistent_peers = "sentry1_id@10.0.1.10:26656,sentry2_id@10.0.1.11:26656,sentry3_id@10.0.1.12:26656"
private_peer_ids = ""  # Can list sentry IDs here
addr_book_strict = false  # Allow private IPs
```

### Sentry config.toml

```toml
# On SENTRY nodes:
[p2p]
pex = true  # Enable peer exchange
persistent_peers = "validator_id@10.0.1.5:26656,other_sentries..."
private_peer_ids = "validator_node_id"  # Hide validator from gossip
```

**Get node ID:**
```bash
posd tendermint show-node-id
```

---

## Fail2Ban Integration

**Protect against brute-force attacks:**

### Install Fail2Ban

```bash
sudo apt-get install -y fail2ban
```

### Configure for SSH

**Create `/etc/fail2ban/jail.local`:**

```ini
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5
destemail = your-email@example.com
sendername = Fail2Ban
action = %(action_mwl)s

[sshd]
enabled = true
port = 22
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 86400
```

**Start Fail2Ban:**

```bash
sudo systemctl enable fail2ban
sudo systemctl start fail2ban

# Check status
sudo fail2ban-client status sshd

# Unban IP
sudo fail2ban-client set sshd unbanip 1.2.3.4
```

---

## Monitoring & Alerts

### Monitor Firewall Logs

**UFW logs:**
```bash
# View live logs
sudo tail -f /var/log/ufw.log

# Count blocked attempts
sudo grep -c "BLOCK" /var/log/ufw.log

# Top blocked IPs
sudo grep "BLOCK" /var/log/ufw.log | awk '{print $13}' | sort | uniq -c | sort -nr | head -10
```

**iptables logs (if enabled):**
```bash
sudo tail -f /var/log/kern.log | grep iptables
```

### Alert on Suspicious Activity

**Create monitoring script:**

```bash
#!/bin/bash
# ~/monitor-firewall.sh

LOGFILE="/var/log/ufw.log"
THRESHOLD=100  # Alert if more than 100 blocks in 5 minutes

BLOCKS=$(grep "BLOCK" $LOGFILE | grep "$(date +%b\ %e\ %H:%M -d '5 minutes ago')" | wc -l)

if [ $BLOCKS -gt $THRESHOLD ]; then
    echo "ALERT: $BLOCKS firewall blocks in the last 5 minutes" | mail -s "Firewall Alert" your-email@example.com
fi
```

**Run via cron (every 5 minutes):**
```bash
crontab -e
# Add:
*/5 * * * * /home/omniphi/monitor-firewall.sh
```

---

## Testing & Verification

### Test Open Ports

**From external machine:**

```bash
# Test SSH
telnet VALIDATOR_IP 22

# Test P2P
telnet VALIDATOR_IP 26656

# Test RPC (should fail if localhost-only)
telnet VALIDATOR_IP 26657
```

**Using nmap:**
```bash
# Scan validator from external machine
nmap -p 22,26656,26657,9090 VALIDATOR_IP

# Expected output:
# 22/tcp    open   ssh
# 26656/tcp open   unknown
# 26657/tcp closed unknown  (if localhost-only)
```

### Test Firewall Rules

```bash
# List all active rules
sudo ufw status numbered

# Test specific rule
sudo ufw status verbose | grep 26656

# Verify iptables rules
sudo iptables -L -v -n
```

### Verify P2P Connectivity

```bash
# Check peer count
curl -s localhost:26657/net_info | jq '.result.n_peers'

# Should be > 0 if P2P is working

# Check peer details
curl -s localhost:26657/net_info | jq '.result.peers[].remote_ip'
```

---

## Troubleshooting

### Issue: Cannot Connect via SSH After Enabling Firewall

**Solution:**
```bash
# Access via cloud provider console (AWS/DO/GCP)
# Add SSH rule
sudo ufw allow 22/tcp

# Or disable firewall temporarily
sudo ufw disable
```

### Issue: Validator Has No Peers

**Check P2P port:**
```bash
# Verify port 26656 is allowed
sudo ufw status | grep 26656

# Test from external machine
telnet VALIDATOR_IP 26656
```

**Check config.toml:**
```bash
# Ensure P2P is listening on public interface
grep "laddr" ~/.omniphi/config/config.toml
# Should be: laddr = "tcp://0.0.0.0:26656"
```

### Issue: Firewall Rules Not Persisting After Reboot

**Solution:**
```bash
# For UFW (should persist automatically)
sudo ufw enable

# For iptables, install iptables-persistent
sudo apt-get install iptables-persistent
sudo netfilter-persistent save
```

---

## Security Checklist

### ✅ Basic Security
- [ ] UFW or iptables enabled
- [ ] SSH port restricted to your IP (or VPN)
- [ ] P2P port (26656) open to 0.0.0.0/0
- [ ] RPC/API ports bound to localhost only
- [ ] Fail2Ban installed and configured

### ✅ Advanced Security
- [ ] Rate limiting enabled on SSH
- [ ] DDoS protection rules configured
- [ ] Firewall logs monitored
- [ ] Sentry node architecture (for high-value validators)
- [ ] Regular security audits

### ✅ Cloud Security
- [ ] Cloud provider firewall configured
- [ ] Security groups attached to instances
- [ ] Network ACLs reviewed (AWS VPC)
- [ ] Private subnets used for validator (if using sentries)

---

**Remember:** Firewall is just one layer of security. Combine with strong SSH keys, OS hardening, and regular updates.

---

**For more security guides:**
- [KEY_MANAGEMENT.md](KEY_MANAGEMENT.md) - Protecting validator keys
- [SLASHING_PROTECTION.md](SLASHING_PROTECTION.md) - Preventing double-signing
