# Omniphi Blockchain Startup Guide

Complete guide for starting and managing your Omniphi blockchain testnet.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Prerequisites](#prerequisites)
3. [Setup Instructions](#setup-instructions)
4. [Starting the Blockchain](#starting-the-blockchain)
5. [Manual Commands](#manual-commands)
6. [Monitoring](#monitoring)
7. [Stopping the Blockchain](#stopping-the-blockchain)
8. [Troubleshooting](#troubleshooting)
9. [Network Configurations](#network-configurations)

---

## Quick Start

### Single Computer Setup (Testing)

**Ubuntu/Linux/Mac:**
```bash
# 1. Generate testnet
./setup_2node_testnet.sh

# 2. Start validator 1
./start_validator1.sh

# 3. In another terminal, start validator 2
./start_validator2.sh

# 4. Monitor the blockchain
./health_check.sh
```

**Windows:**
```powershell
# 1. Generate testnet
.\setup_2node_testnet.ps1

# 2. Start validator 1
.\start_validator1.ps1

# 3. In another PowerShell window, start validator 2
.\start_validator2.ps1

# 4. Monitor the blockchain
.\health_check.ps1
```

### Two Computer Setup (Distributed)

See [Network Configurations](#network-configurations) section below.

---

## Prerequisites

### System Requirements

**Minimum:**
- 2GB RAM
- 20GB disk space
- Dual-core CPU

**Recommended:**
- 4GB RAM
- 50GB SSD
- Quad-core CPU

### Software Requirements

**All Platforms:**
- Go 1.21+ ([download](https://golang.org/dl/))
- Git

**Ubuntu/Linux:**
```bash
# Install dependencies
sudo apt update
sudo apt install build-essential git

# Install Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

**macOS:**
```bash
# Install with Homebrew
brew install go git

# Verify
go version
```

**Windows:**
- Download Go installer from [golang.org](https://golang.org/dl/)
- Install Git from [git-scm.com](https://git-scm.com/)
- Use PowerShell (not CMD)

### Network Requirements

**For Two-Computer Setup:**
- Both computers on same network OR
- Port 26656 open between computers
- Firewall configured (see below)

---

## Setup Instructions

### Step 1: Build the Binary

**Ubuntu/Linux/Mac:**
```bash
cd ~/omniphi/pos
go build -o posd ./cmd/posd

# Make executable
chmod +x posd

# Verify
./posd version
```

**Windows:**
```powershell
cd C:\Users\YourName\omniphi\pos
go build -o posd.exe .\cmd\posd

# Verify
.\posd.exe version
```

### Step 2: Generate Testnet Configuration

This creates a complete 2-validator testnet setup.

**Ubuntu/Linux/Mac:**
```bash
chmod +x setup_2node_testnet.sh
./setup_2node_testnet.sh
```

**Windows:**
```powershell
.\setup_2node_testnet.ps1
```

**What This Creates:**
- `testnet-2nodes/validator1/` - First validator configuration
- `testnet-2nodes/validator2/` - Second validator configuration
- Shared genesis file with both validators
- Test account with 50,000 OMNI tokens
- Validator keys (keyring-backend: test)

**Configuration Details:**
- Chain ID: `omniphi-testnet-1`
- Denomination: `omniphi` (base unit)
- 1 OMNI = 1,000,000 omniphi
- Each validator: 100,000 OMNI staked
- Test account: 50,000 OMNI

### Step 3: Network Configuration (Two-Computer Setup Only)

Skip this if running both validators on same computer.

**Find IP Addresses:**

Ubuntu/Linux/Mac:
```bash
ifconfig | grep "inet "
# Look for: inet 192.168.x.x
```

Windows:
```powershell
ipconfig
# Look for: IPv4 Address
```

**Update Peer Configuration:**

Ubuntu/Linux/Mac:
```bash
chmod +x update_peer_ip.sh
./update_peer_ip.sh <COMPUTER1_IP> <COMPUTER2_IP>

# Example:
./update_peer_ip.sh 192.168.1.100 192.168.1.101
```

Windows:
```powershell
.\update_peer_ip.ps1 <COMPUTER1_IP> <COMPUTER2_IP>

# Example:
.\update_peer_ip.ps1 192.168.1.100 192.168.1.101
```

### Step 4: Configure Firewall (Two-Computer Setup Only)

**Ubuntu/Linux:**
```bash
# Open P2P port
sudo ufw allow 26656/tcp

# Verify
sudo ufw status
```

**Windows (Run as Administrator):**
```powershell
New-NetFirewallRule -DisplayName "Omniphi P2P" -Direction Inbound -LocalPort 26656 -Protocol TCP -Action Allow

# Verify
Get-NetFirewallRule -DisplayName "Omniphi P2P"
```

**macOS:**
```bash
# System Preferences > Security & Privacy > Firewall > Firewall Options
# Add posd to allowed applications
```

---

## Starting the Blockchain

### Method 1: Using Startup Scripts (Recommended)

**On Computer 1 (or Terminal 1):**

Ubuntu/Linux/Mac:
```bash
chmod +x start_validator1.sh
./start_validator1.sh
```

Windows:
```powershell
.\start_validator1.ps1
```

**On Computer 2 (or Terminal 2):**

Ubuntu/Linux/Mac:
```bash
chmod +x start_validator2.sh
./start_validator2.sh
```

Windows:
```powershell
.\start_validator2.ps1
```

**Expected Output:**
```
======================================
  Starting Validator 1
  Omniphi Testnet
======================================

Home Directory: ./testnet-2nodes/validator1
Binary: ./posd

Ports:
  P2P: 26656
  RPC: 26657
  gRPC: 9090
  API: 1317

Press Ctrl+C to stop the validator

[Blockchain logs start appearing...]
```

**Good Signs:**
- `module=consensus` messages
- `Executed block` with increasing height
- `Committed state`
- No error messages

**Connection Established (when both validators running):**
```
module=p2p Peer is good
module=consensus Received proposal
module=consensus Finalizing commit
module=state Committed state
```

### Method 2: Manual Commands

If you need more control or to run in background.

**Ubuntu/Linux/Mac:**

Validator 1:
```bash
./posd start \
    --home ./testnet-2nodes/validator1 \
    --minimum-gas-prices "0.001omniphi" \
    > validator1.log 2>&1 &
```

Validator 2:
```bash
./posd start \
    --home ./testnet-2nodes/validator2 \
    --minimum-gas-prices "0.001omniphi" \
    --p2p.laddr "tcp://0.0.0.0:26656" \
    --rpc.laddr "tcp://127.0.0.1:26657" \
    --grpc.address "127.0.0.1:9090" \
    --api.address "tcp://127.0.0.1:1317" \
    > validator2.log 2>&1 &
```

**Note:** On same computer, validator2 uses different ports (already configured in testnet setup).

**Windows:**

Validator 1:
```powershell
Start-Process powershell -ArgumentList "-NoExit", "-Command", ".\posd.exe start --home .\testnet-2nodes\validator1 --minimum-gas-prices '0.001omniphi'"
```

Validator 2:
```powershell
Start-Process powershell -ArgumentList "-NoExit", "-Command", ".\posd.exe start --home .\testnet-2nodes\validator2 --minimum-gas-prices '0.001omniphi'"
```

### Method 3: Background Process (Linux/Mac)

**Using systemd (Production Setup):**

Create service file: `/etc/systemd/system/omniphi-validator1.service`
```ini
[Unit]
Description=Omniphi Validator 1
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/home/youruser/omniphi/pos
ExecStart=/home/youruser/omniphi/pos/posd start --home /home/youruser/omniphi/pos/testnet-2nodes/validator1 --minimum-gas-prices 0.001omniphi
Restart=on-failure
RestartSec=10
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

**Start service:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable omniphi-validator1
sudo systemctl start omniphi-validator1

# Check status
sudo systemctl status omniphi-validator1

# View logs
sudo journalctl -u omniphi-validator1 -f
```

### Verification After Startup

Wait 10-20 seconds after starting both validators, then check:

**Ubuntu/Linux/Mac:**
```bash
./health_check.sh
```

**Windows:**
```powershell
.\health_check.ps1
```

**Expected Output:**
```
=== Omniphi Blockchain Health Check ===

✓ Node is running
  Block Height: 42
  Peers: 1
  Status: ✓ SYNCED AND PRODUCING BLOCKS
```

---

## Manual Commands

### Check Node Status

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe status --home .\testnet-2nodes\validator1
```

### Check Block Height

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1 | grep -o '"latest_block_height":"[^"]*"' | cut -d'"' -f4
```

**Windows:**
```powershell
$status = .\posd.exe status --home .\testnet-2nodes\validator1 | ConvertFrom-Json
$status.sync_info.latest_block_height
```

### Check Validators

**Ubuntu/Linux/Mac:**
```bash
./posd query staking validators --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe query staking validators --home .\testnet-2nodes\validator1
```

### Check Account Balance

**Ubuntu/Linux/Mac:**
```bash
# Get testuser address
./posd keys show testuser -a --keyring-backend test --home ./testnet-2nodes/validator1

# Check balance
./posd query bank balances <address> --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
# Get testuser address
.\posd.exe keys show testuser -a --keyring-backend test --home .\testnet-2nodes\validator1

# Check balance
.\posd.exe query bank balances <address> --home .\testnet-2nodes\validator1
```

### Send Transaction

**Ubuntu/Linux/Mac:**
```bash
./posd tx bank send testuser <recipient-address> 1000000omniphi \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --home ./testnet-2nodes/validator1 \
  --fees 1000omniphi \
  --yes
```

**Windows:**
```powershell
.\posd.exe tx bank send testuser <recipient-address> 1000000omniphi `
  --chain-id omniphi-testnet-1 `
  --keyring-backend test `
  --home .\testnet-2nodes\validator1 `
  --fees 1000omniphi `
  --yes
```

---

## Monitoring

### Quick Health Check

**Ubuntu/Linux/Mac:**
```bash
./health_check.sh
```

**Windows:**
```powershell
.\health_check.ps1
```

### Continuous Live Monitoring

**Ubuntu/Linux/Mac:**
```bash
./monitor.sh
```

**Windows:**
```powershell
.\monitor.ps1
```

**Output:**
```
===================================================
       Omniphi Testnet - Live Monitor
===================================================

Block Height:    156
Block Time:      2025-11-18T07:25:42.123456Z
Block Hash:      06E3268DABE4D96...

Sync Status:     ✓ SYNCED
Peers Connected: 1

---------------------------------------------------
Updating every 3 seconds... (Ctrl+C to exit)
```

### View Logs

**Ubuntu/Linux/Mac:**
```bash
# Follow logs in real-time
tail -f ./testnet-2nodes/validator1/posd.log

# Search for errors
grep "ERROR" ./testnet-2nodes/validator1/posd.log
```

**Windows:**
```powershell
# Follow logs in real-time
Get-Content .\testnet-2nodes\validator1\posd.log -Tail 50 -Wait

# Search for errors
Select-String "ERROR" .\testnet-2nodes\validator1\posd.log
```

### Check Process Status

**Ubuntu/Linux/Mac:**
```bash
ps aux | grep posd
```

**Windows:**
```powershell
Get-Process | Where-Object {$_.ProcessName -like "*posd*"}
```

For complete monitoring commands, see [MONITORING.md](MONITORING.md).

---

## Stopping the Blockchain

### Stop Validators Gracefully

**If running in foreground (recommended):**
- Press `Ctrl+C` in the terminal window

**If running in background:**

Ubuntu/Linux/Mac:
```bash
# Find process ID
ps aux | grep posd

# Stop specific validator
pkill -f "posd start --home ./testnet-2nodes/validator1"
pkill -f "posd start --home ./testnet-2nodes/validator2"

# Or stop all posd processes
pkill posd
```

Windows:
```powershell
# Stop all posd processes
Get-Process posd -ErrorAction SilentlyContinue | Stop-Process

# Or stop specific process by ID
Stop-Process -Id <PID>
```

**If using systemd:**
```bash
sudo systemctl stop omniphi-validator1
sudo systemctl stop omniphi-validator2
```

### Restart Validators

Simply run the start commands again:

**Ubuntu/Linux/Mac:**
```bash
./start_validator1.sh
./start_validator2.sh
```

**Windows:**
```powershell
.\start_validator1.ps1
.\start_validator2.ps1
```

### Reset Blockchain Data (Clean Start)

**Warning:** This deletes all blockchain data and transactions!

**Ubuntu/Linux/Mac:**
```bash
# Stop validators first
pkill posd

# Reset validator 1
./posd comet unsafe-reset-all --home ./testnet-2nodes/validator1

# Reset validator 2
./posd comet unsafe-reset-all --home ./testnet-2nodes/validator2

# Restart
./start_validator1.sh
./start_validator2.sh
```

**Windows:**
```powershell
# Stop validators first
Get-Process posd -ErrorAction SilentlyContinue | Stop-Process

# Reset validator 1
.\posd.exe comet unsafe-reset-all --home .\testnet-2nodes\validator1

# Reset validator 2
.\posd.exe comet unsafe-reset-all --home .\testnet-2nodes\validator2

# Restart
.\start_validator1.ps1
.\start_validator2.ps1
```

---

## Troubleshooting

### Blockchain Not Producing Blocks

**Problem:** Block height stuck, not increasing

**Causes:**
- Only one validator running (need both for consensus)
- Validators not connected (peer issue)
- Network connectivity problem

**Solution:**
```bash
# 1. Check both validators are running
ps aux | grep posd  # Linux/Mac
Get-Process posd    # Windows

# 2. Check peer connections
./posd status --home ./testnet-2nodes/validator1 | grep "n_peers"
# Should show: "n_peers":"1"

# 3. Restart both validators
pkill posd && ./start_validator1.sh && ./start_validator2.sh
```

### Validators Not Connecting

**Problem:** Peers: 0, validators can't find each other

**Two-Computer Setup:**

1. **Verify IP addresses:**
```bash
# Check config
cat testnet-2nodes/validator1/config/config.toml | grep persistent_peers
```

2. **Test network connectivity:**
```bash
# Ping the other computer
ping 192.168.1.101

# Test the P2P port
telnet 192.168.1.101 26656
# Or
nc -zv 192.168.1.101 26656
```

3. **Check firewall:**
```bash
# Ubuntu
sudo ufw status

# Windows
Get-NetFirewallRule -DisplayName "Omniphi P2P"
```

4. **Verify node IDs match:**
```bash
# On Computer 1
./posd comet show-node-id --home ./testnet-2nodes/validator1

# Compare with persistent_peers in Computer 2's config
cat testnet-2nodes/validator2/config/config.toml | grep persistent_peers
```

**Same Computer Setup:**

If both on same computer and not connecting, check port configuration:
```bash
# Validator 2 should use different ports
cat testnet-2nodes/validator2/config/config.toml | grep -E "laddr|address"
```

### Port Already in Use

**Problem:** `bind: address already in use`

**Ubuntu/Linux/Mac:**
```bash
# Find what's using port 26656
sudo lsof -i :26656

# Kill the process
sudo kill -9 <PID>
```

**Windows:**
```powershell
# Find what's using port 26656
Get-NetTCPConnection -LocalPort 26656

# Stop the process
Stop-Process -Id <PID> -Force
```

### Genesis Mismatch Error

**Problem:** `AppHash mismatch` or `Genesis mismatch`

**Solution:**
Both validators must have **identical** genesis files.

```bash
# Check genesis hash
shasum testnet-2nodes/validator1/config/genesis.json
shasum testnet-2nodes/validator2/config/genesis.json

# If different, copy from validator1 to validator2
cp testnet-2nodes/validator1/config/genesis.json testnet-2nodes/validator2/config/genesis.json
```

### Permission Denied

**Ubuntu/Linux/Mac:**
```bash
# Make scripts executable
chmod +x *.sh
chmod +x posd

# Check directory permissions
ls -la testnet-2nodes/
```

**Windows:**
Run PowerShell as Administrator.

### Out of Gas Error

**Problem:** Transaction fails with "out of gas"

**Solution:**
```bash
# Use auto gas estimation
posd tx bank send ... --gas auto --gas-adjustment 1.5

# Or specify higher gas
posd tx bank send ... --gas 300000
```

### Validator Crashed or Stopped

**Check logs for errors:**

Ubuntu/Linux/Mac:
```bash
tail -100 testnet-2nodes/validator1/posd.log
```

Windows:
```powershell
Get-Content .\testnet-2nodes\validator1\posd.log -Tail 100
```

**Common errors and solutions:**

| Error | Cause | Solution |
|-------|-------|----------|
| `dial tcp: connection refused` | Other validator not running | Start both validators |
| `context deadline exceeded` | Network/firewall issue | Check firewall, test connectivity |
| `insufficient fees` | Gas price too low | Increase --fees parameter |
| `account sequence mismatch` | Transaction nonce issue | Wait a block and retry |
| `panic` | Critical error | Check logs, report issue |

---

## Network Configurations

### Configuration 1: Same Computer (Testing)

Both validators run on one computer with different ports.

**Ports:**
- Validator 1: 26656, 26657, 9090, 1317 (default)
- Validator 2: 26666, 26667, 9091, 1318 (automatically configured)

**Start:**
```bash
# Terminal 1
./start_validator1.sh

# Terminal 2
./start_validator2.sh
```

**Use Case:** Development, testing, learning

### Configuration 2: Two Computers - Same LAN

Validators on different computers, same local network.

**Network:**
```
Computer 1 (192.168.1.100)  <---LAN--->  Computer 2 (192.168.1.101)
     Validator 1                              Validator 2
```

**Setup:**
```bash
# On Computer 1
./setup_2node_testnet.sh
./update_peer_ip.sh 192.168.1.100 192.168.1.101
./package_validator2.sh
# Transfer validator2-package to Computer 2
./start_validator1.sh

# On Computer 2
tar -xzf validator2-package.tar.gz
cd validator2-package
sudo ufw allow 26656/tcp
./start_validator2.sh
```

**Use Case:** Realistic testing, development network

### Configuration 3: Two Computers - Different Networks

Validators on different networks (Internet).

**Network:**
```
Computer 1 (Public IP: 1.2.3.4)  <---Internet--->  Computer 2 (Public IP: 5.6.7.8)
     Validator 1                                        Validator 2
     (Port forwarding)                                  (Port forwarding)
```

**Additional Setup:**
1. **Get public IPs:**
```bash
curl ifconfig.me
```

2. **Configure router port forwarding:**
   - Forward external port 26656 → internal IP:26656
   - On both routers

3. **Update peer IPs with public IPs:**
```bash
./update_peer_ip.sh 1.2.3.4 5.6.7.8
```

4. **Firewall rules:**
```bash
# Allow from specific IP only (more secure)
sudo ufw allow from 5.6.7.8 to any port 26656
```

**Use Case:** Production-like setup, geographically distributed

### Configuration 4: WSL + Windows

Validator on Windows, validator on WSL (Windows Subsystem for Linux).

**Network:**
```
Windows (192.168.68.71)  <---Bridge--->  WSL (172.x.x.x or 10.x.x.x)
  Validator 1                                  Validator 2
```

**Get WSL IP:**
```bash
# In WSL
ip addr show eth0 | grep "inet "
```

**Get Windows IP from WSL perspective:**
```bash
# In WSL
cat /etc/resolv.conf | grep nameserver | awk '{print $2}'
```

**Setup:**
```powershell
# On Windows
.\setup_2node_testnet.ps1
.\update_peer_ip.ps1 192.168.68.71 172.x.x.x
.\package_validator2.ps1
# Copy validator2-package.zip to WSL
.\start_validator1.ps1
```

```bash
# In WSL
unzip validator2-package.zip
cd validator2-package
chmod +x start_validator2.sh
./start_validator2.sh
```

**Use Case:** Development on Windows with Linux testing

---

## Port Reference

### Default Ports

| Port | Service | Protocol | External Access? | Description |
|------|---------|----------|------------------|-------------|
| 26656 | P2P | TCP | **YES** | Peer-to-peer communication (required) |
| 26657 | RPC | TCP | Optional | RPC API for queries |
| 9090 | gRPC | TCP | Optional | gRPC API |
| 1317 | REST API | TCP | Optional | REST API |
| 26660 | Prometheus | TCP | No | Metrics export |

### Validator 2 Ports (Same Computer)

| Port | Service | Protocol | Description |
|------|---------|----------|-------------|
| 26666 | P2P | TCP | Peer-to-peer |
| 26667 | RPC | TCP | RPC API |
| 9091 | gRPC | TCP | gRPC API |
| 1318 | REST API | TCP | REST API |
| 26670 | Prometheus | TCP | Metrics |

**Minimum Required:** Only port 26656 must be accessible between validators.

---

## Security Notes

⚠️ **This setup is for TESTING only!**

**Insecure for production:**
- Keyring backend: `test` (keys not encrypted)
- APIs fully exposed
- Default generated keys
- No TLS/SSL
- enabled-unsafe-cors

**For production:**
- Use `--keyring-backend file` or `os`
- Disable unsafe CORS
- Generate secure keys offline
- Use TLS for all connections
- Restrict API access
- Implement monitoring
- Regular backups
- Use hardware security modules (HSM) for validators

---

## Additional Resources

- **Monitoring Guide:** [MONITORING.md](MONITORING.md)
- **Multi-Node Setup:** [MULTI_NODE_TESTNET_GUIDE.md](MULTI_NODE_TESTNET_GUIDE.md)
- **Quick Reference:** [TESTNET_QUICK_START.md](TESTNET_QUICK_START.md)
- **3-Layer Fee System:** `x/poc/client/cli/FEE_SYSTEM_GUIDE.md`
- **Implementation Details:** [3LAYER_FEE_IMPLEMENTATION_COMPLETE.md](3LAYER_FEE_IMPLEMENTATION_COMPLETE.md)

---

## Summary

**To start the blockchain:**
1. Generate testnet: `./setup_2node_testnet.sh`
2. Start validator 1: `./start_validator1.sh`
3. Start validator 2: `./start_validator2.sh`
4. Monitor: `./health_check.sh`

**To stop the blockchain:**
- Press `Ctrl+C` in validator terminals

**To monitor:**
- Health check: `./health_check.sh`
- Live monitor: `./monitor.sh`
- See [MONITORING.md](MONITORING.md)

**Need help?**
- Check [Troubleshooting](#troubleshooting) section
- Review validator logs
- Verify network connectivity
- Check firewall configuration

---

**Happy blockchain testing!** 🚀
