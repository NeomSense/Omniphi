# Multi-Node Testnet Setup Guide
## Running Omniphi Blockchain on Two Different Computers

This guide will help you set up a 2-validator testnet with validators running on two different computers.

---

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Detailed Setup - Computer 1](#detailed-setup---computer-1)
4. [Detailed Setup - Computer 2](#detailed-setup---computer-2)
5. [Network Configuration](#network-configuration)
6. [Starting the Network](#starting-the-network)
7. [Verification](#verification)
8. [Testing Transactions](#testing-transactions)
9. [Troubleshooting](#troubleshooting)
10. [Advanced Topics](#advanced-topics)

---

## Prerequisites

### Both Computers Need:
- **Go 1.21+** installed ([https://golang.org/dl/](https://golang.org/dl/))
- **Git** for cloning the repository
- **Network connectivity** between the two computers
- **Firewall access** for port 26656 (P2P communication)

### Computer 1 (Coordinator):
- Will generate configuration for both validators
- Needs to package and transfer validator2 config to Computer 2

### Computer 2 (Remote):
- Will receive validator2 configuration package
- Will run the second validator

---

## Quick Start

### On Computer 1:

```bash
# 1. Build the binary
cd ~/omniphi/pos
go build -o posd ./cmd/posd

# 2. Generate 2-node testnet
./setup_2node_testnet.sh    # Linux/Mac
# OR
.\setup_2node_testnet.ps1   # Windows

# 3. Find your IP addresses
ifconfig                     # Linux/Mac
ipconfig                     # Windows

# 4. Update peer IPs
./update_peer_ip.sh 192.168.1.100 192.168.1.101
# Replace with your actual IPs

# 5. Package validator2
./package_validator2.sh      # Linux/Mac
# OR
.\package_validator2.ps1     # Windows

# 6. Transfer validator2-package to Computer 2

# 7. Start validator1
./start_validator1.sh        # Linux/Mac
# OR
.\start_validator1.ps1       # Windows
```

### On Computer 2:

```bash
# 1. Extract the package
tar -xzf validator2-package.tar.gz    # Linux/Mac
# OR
Expand-Archive validator2-package.zip # Windows

# 2. Build/copy the binary
cd ~/omniphi/pos
go build -o posd ./cmd/posd
cp posd /path/to/validator2-package/

# 3. Start validator2
cd validator2-package
./start_validator2.sh        # Linux/Mac
# OR
.\start_validator2.ps1       # Windows
```

---

## Detailed Setup - Computer 1

### Step 1: Build the Binary

```bash
cd ~/omniphi/pos
go build -o posd ./cmd/posd

# Verify build
./posd version
```

### Step 2: Generate Testnet Configuration

The setup script will create a complete 2-node testnet configuration:

**Linux/Mac:**
```bash
chmod +x setup_2node_testnet.sh
./setup_2node_testnet.sh
```

**Windows PowerShell:**
```powershell
.\setup_2node_testnet.ps1
```

This creates:
- `./testnet-2nodes/validator1/` - Configuration for Computer 1
- `./testnet-2nodes/validator2/` - Configuration for Computer 2
- Shared genesis file with both validators
- Keys for both validators (keyring-backend: test)
- Test user account with funds

### Step 3: Find IP Addresses

You need to find the IP addresses of both computers on your network.

**Linux/Mac:**
```bash
ifconfig | grep "inet "
# Look for something like: inet 192.168.1.100
```

**Windows:**
```powershell
ipconfig
# Look for IPv4 Address under your active network adapter
```

**Common IP ranges:**
- Home networks: `192.168.0.x` or `192.168.1.x`
- Office networks: `10.0.x.x` or `172.16.x.x`

**Example:**
- Computer 1: `192.168.1.100`
- Computer 2: `192.168.1.101`

### Step 4: Update Peer Configuration

Use the actual IP addresses you found:

**Linux/Mac:**
```bash
chmod +x update_peer_ip.sh
./update_peer_ip.sh 192.168.1.100 192.168.1.101
```

**Windows:**
```powershell
.\update_peer_ip.ps1 192.168.1.100 192.168.1.101
```

This updates the `persistent_peers` configuration in both validators' config files.

### Step 5: Package Validator 2

**Linux/Mac:**
```bash
chmod +x package_validator2.sh
./package_validator2.sh
```

This creates: `validator2-package.tar.gz`

**Windows:**
```powershell
.\package_validator2.ps1
```

This creates: `validator2-package.zip`

### Step 6: Transfer to Computer 2

Transfer the package using one of these methods:

**USB Drive:**
```bash
cp validator2-package.tar.gz /media/usb/
```

**Network Transfer (scp):**
```bash
scp validator2-package.tar.gz user@192.168.1.101:/home/user/
```

**Windows Network Share:**
```powershell
Copy-Item validator2-package.zip \\Computer2\Share\
```

---

## Detailed Setup - Computer 2

### Step 1: Receive the Package

Ensure you've received the validator2 package from Computer 1.

### Step 2: Extract the Package

**Linux/Mac:**
```bash
tar -xzf validator2-package.tar.gz
cd validator2-package
```

**Windows:**
```powershell
Expand-Archive validator2-package.zip -DestinationPath .\validator2-package
cd validator2-package
```

The package contains:
- `validator2/` - Validator configuration
- `posd` or `posd.exe` - Binary (if included)
- `start_validator2.sh` or `.ps1` - Start script
- `README.txt` - Instructions

### Step 3: Build or Copy Binary

If the binary wasn't included in the package:

```bash
# Clone the repository
cd ~
git clone <repository-url>
cd omniphi/pos

# Build the binary
go build -o posd ./cmd/posd

# Copy to validator package directory
cp posd /path/to/validator2-package/
```

**Windows:**
```powershell
go build -o posd.exe .\cmd\posd
Copy-Item posd.exe \path\to\validator2-package\
```

### Step 4: Configure Firewall

The P2P port (26656) must be accessible from Computer 1.

**Linux (UFW):**
```bash
sudo ufw allow 26656/tcp
sudo ufw status
```

**Windows PowerShell (Run as Administrator):**
```powershell
New-NetFirewallRule -DisplayName "Omniphi P2P" -Direction Inbound -LocalPort 26656 -Protocol TCP -Action Allow
```

**macOS:**
```bash
# System Preferences > Security & Privacy > Firewall > Firewall Options
# Add posd to allowed applications
```

---

## Network Configuration

### Understanding the Configuration

Each validator has configuration files in:
- `config/config.toml` - CometBFT settings (P2P, RPC, consensus)
- `config/app.toml` - Application settings (API, gRPC, gas prices)
- `config/genesis.json` - Initial blockchain state

### Persistent Peers

The `persistent_peers` setting in `config.toml` tells each validator where to find the other:

**Validator 1 (`testnet-2nodes/validator1/config/config.toml`):**
```toml
persistent_peers = "<node2_id>@192.168.1.101:26656"
```

**Validator 2 (`validator2/config/config.toml`):**
```toml
persistent_peers = "<node1_id>@192.168.1.100:26656"
```

### Port Configuration

Both validators use the same default ports (they're on different machines):

| Service | Port | Protocol | Description |
|---------|------|----------|-------------|
| P2P | 26656 | TCP | Peer-to-peer communication |
| RPC | 26657 | TCP | RPC API |
| gRPC | 9090 | TCP | gRPC API |
| API | 1317 | TCP | REST API |
| Prometheus | 26660 | TCP | Metrics |

**Only port 26656 needs to be accessible between computers.**

### Same LAN vs Different Networks

**Same Local Network (192.168.x.x):**
- Use local IP addresses
- No port forwarding needed
- Firewall rules only

**Different Networks (Internet):**
- Use public IP addresses
- Configure port forwarding on router (port 26656 → internal IP)
- May need DDNS if IP changes
- Consider VPN for security

---

## Starting the Network

### Important: Start Order

Both validators should be started within a few seconds of each other to ensure proper network formation.

### On Computer 1:

**Linux/Mac:**
```bash
cd ~/omniphi/pos
chmod +x start_validator1.sh
./start_validator1.sh
```

**Windows:**
```powershell
cd C:\Users\YourName\omniphi\pos
.\start_validator1.ps1
```

You should see output like:
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

... blockchain logs ...
```

### On Computer 2:

**Linux/Mac:**
```bash
cd ~/validator2-package
chmod +x start_validator2.sh
./start_validator2.sh
```

**Windows:**
```powershell
cd C:\Users\YourName\validator2-package
.\start_validator2.ps1
```

### What to Look For

**Good Signs:**
- `module=consensus` messages
- `Executed block` messages
- Block height incrementing
- `Committed state` messages

**Connection Established:**
```
module=p2p Peer is good peer=<node_id>
module=consensus Received proposal
module=consensus Finalizing commit
```

**Warning Signs:**
- `Dialing failed` - Network/firewall issue
- `No peers` - Peer configuration issue
- `Timeout` - Network latency issue

---

## Verification

### Quick Health Check

**On Computer 1:**

**Linux/Mac:**
```bash
./verify_network.sh
```

**Windows:**
```powershell
.\verify_network.ps1
```

Expected output:
```
======================================
  Network Status Check
======================================

1. Node Status:
  "latest_block_height": "42"
  "catching_up": false

2. Peer Connections:
  "n_peers": "1"

3. Validators:
  "moniker": "validator1"
  "status": "BOND_STATUS_BONDED"
  "tokens": "100000000000"

✓ Node is running
  Block Height: 42
  Catching Up: false
  Status: ✓ SYNCED
```

### Manual Verification Commands

**Check node status:**
```bash
posd status --home ./testnet-2nodes/validator1
```

**Check validators:**
```bash
posd query staking validators --home ./testnet-2nodes/validator1
```

**Check balance:**
```bash
posd query bank balances <address> --home ./testnet-2nodes/validator1
```

**View latest block:**
```bash
posd query block --home ./testnet-2nodes/validator1
```

### On Computer 2

Run the same commands with `--home ./validator2`:

```bash
posd status --home ./validator2
posd query staking validators --home ./validator2
```

Both validators should show:
- Same block height
- `catching_up: false`
- 2 validators in the active set

---

## Testing Transactions

### Using the Test Account

The setup script created a test account with 50,000 OMNI (50000000000 uomni).

**Get test account address:**

**Linux/Mac:**
```bash
posd keys show testuser -a --keyring-backend test --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe keys show testuser -a --keyring-backend test --home .\testnet-2nodes\validator1
```

Save this address (looks like: `omni1abc...xyz`)

### Send a Transaction

**Create a new recipient account:**
```bash
posd keys add recipient --keyring-backend test --home ./testnet-2nodes/validator1
```

**Send tokens:**
```bash
posd tx bank send testuser <recipient-address> 1000000uomni \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --home ./testnet-2nodes/validator1 \
  --fees 1000uomni \
  --yes
```

**Check the transaction:**
```bash
posd query tx <tx_hash> --home ./testnet-2nodes/validator1
```

**Verify recipient balance:**
```bash
posd query bank balances <recipient-address> --home ./testnet-2nodes/validator1
```

### Testing the 3-Layer Fee System

The blockchain has a sophisticated 3-layer fee system. You can test it by submitting PoC contributions:

**Check your C-Score (reputation):**
```bash
posd query poc credits <your-address> --home ./testnet-2nodes/validator1
```

**Submit a contribution (fee will be calculated automatically):**
```bash
posd tx poc submit-contribution \
  --contribution-type "code" \
  --title "Test Contribution" \
  --description "Testing the fee system" \
  --hash "abc123def456" \
  --from testuser \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --home ./testnet-2nodes/validator1 \
  --yes
```

**Check fee events:**
```bash
posd query tx <tx_hash> --home ./testnet-2nodes/validator1 | grep "poc_3layer_fee"
```

The event will show:
- `total_fee`: Final fee charged
- `burned`: Amount burned (50%)
- `to_pool`: Amount to reward pool (50%)
- `epoch_multiplier`: Congestion multiplier (0.8x to 5.0x)
- `cscore_discount`: Reputation discount (0% to 90%)

---

## Troubleshooting

### Validators Not Connecting

**Problem:** Validators can't find each other

**Solution:**
1. Verify IP addresses are correct:
   ```bash
   cat testnet-2nodes/validator1/config/config.toml | grep persistent_peers
   ```

2. Check firewall on both computers:
   ```bash
   # Linux
   sudo ufw status
   # Windows
   Get-NetFirewallRule -DisplayName "Omniphi P2P"
   ```

3. Test network connectivity:
   ```bash
   # From Computer 1 to Computer 2
   telnet 192.168.1.101 26656
   # Or
   nc -zv 192.168.1.101 26656
   ```

4. Verify node IDs match:
   ```bash
   posd comet show-node-id --home ./testnet-2nodes/validator1
   posd comet show-node-id --home ./validator2
   ```

### Validator Not Syncing

**Problem:** Block height not increasing

**Check:**
1. Is the other validator running?
2. Are there any errors in the logs?
3. Is catching_up stuck on `true`?

**Solution:**
```bash
# Restart both validators
# On Computer 1:
pkill posd
./start_validator1.sh

# On Computer 2:
pkill posd
./start_validator2.sh
```

### Connection Timeout

**Problem:** `Dialing failed` or `context deadline exceeded`

**Causes:**
- Firewall blocking
- Wrong IP address
- Network latency
- Router configuration

**Debug:**
1. **Ping test:**
   ```bash
   ping 192.168.1.101
   ```

2. **Port test:**
   ```bash
   telnet 192.168.1.101 26656
   ```

3. **Check logs:**
   ```bash
   tail -f testnet-2nodes/validator1/posd.log
   ```

4. **Temporarily disable firewall (for testing):**
   ```bash
   # Linux
   sudo ufw disable
   # Windows
   Set-NetFirewallProfile -Profile Domain,Public,Private -Enabled False
   ```

   **Remember to re-enable after testing!**

### Genesis Mismatch

**Problem:** `AppHash mismatch` or `Genesis mismatch`

**Solution:**
Both validators must have **identical** genesis files.

```bash
# On Computer 1
shasum testnet-2nodes/validator1/config/genesis.json

# On Computer 2
shasum validator2/config/genesis.json

# Hashes must match!
```

If they don't match, re-copy the genesis file from Computer 1 to Computer 2.

### Permission Denied

**Linux/Mac:**
```bash
chmod +x *.sh
chmod +x posd
```

**Windows:** Run PowerShell as Administrator

### Out of Gas

**Problem:** Transaction fails with "out of gas"

**Solution:**
Increase gas limit:
```bash
posd tx bank send ... --gas 300000
```

Or use auto gas estimation:
```bash
posd tx bank send ... --gas auto --gas-adjustment 1.5
```

---

## Advanced Topics

### Adding More Validators

To add a 3rd validator:

1. Run the setup script with `--v 3`
2. Update peer configurations for all 3 nodes
3. Collect gentx from all validators
4. Distribute updated genesis to all

### Changing Validator Stake

```bash
posd tx staking delegate <validator-address> 50000000000uomni \
  --from testuser \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --home ./testnet-2nodes/validator1 \
  --yes
```

### Viewing Logs

**Follow live logs:**
```bash
# Linux/Mac
tail -f testnet-2nodes/validator1/posd.log

# Windows
Get-Content testnet-2nodes\validator1\posd.log -Wait -Tail 50
```

**Filter for errors:**
```bash
grep "ERROR" testnet-2nodes/validator1/posd.log
```

### Governance Proposals

Test governance by creating a parameter change proposal:

```bash
posd tx gov submit-proposal param-change proposal.json \
  --from testuser \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --home ./testnet-2nodes/validator1 \
  --yes
```

### Monitoring with Prometheus

Prometheus metrics are exposed on port 26660:

```bash
curl http://localhost:26660/metrics
```

Set up Grafana for visualization.

### Backup Validator Keys

**Critical:** Backup your validator keys!

```bash
# Validator private key
cp testnet-2nodes/validator1/config/priv_validator_key.json ~/backup/

# Node key
cp testnet-2nodes/validator1/config/node_key.json ~/backup/

# Keyring (for test accounts)
tar -czf keyring-backup.tar.gz testnet-2nodes/validator1/keyring-test/
```

### Resetting the Testnet

To start fresh:

```bash
# Stop validators on both computers
pkill posd

# On Computer 1: Remove old data
rm -rf testnet-2nodes

# Re-run setup
./setup_2node_testnet.sh
```

---

## Network Architectures

### Same LAN (Recommended for Testing)

```
Computer 1 (192.168.1.100)          Computer 2 (192.168.1.101)
┌─────────────────────┐            ┌─────────────────────┐
│   Validator 1       │◄──P2P─────►│   Validator 2       │
│   Port 26656        │            │   Port 26656        │
└─────────────────────┘            └─────────────────────┘
         │                                  │
         └──────────► Router ◄──────────────┘
                   (192.168.1.1)
```

### Different Networks (Production)

```
Computer 1 (Public IP: 1.2.3.4)    Computer 2 (Public IP: 5.6.7.8)
┌─────────────────────┐            ┌─────────────────────┐
│   Validator 1       │            │   Validator 2       │
│   Port 26656        │            │   Port 26656        │
└─────────────────────┘            └─────────────────────┘
         │                                  │
      Router                             Router
   (Port Forward)                    (Port Forward)
         │                                  │
         └──────────► Internet ◄────────────┘
```

---

## Port Reference

### Validator Ports

| Port | Service | External Access? | Description |
|------|---------|------------------|-------------|
| 26656 | P2P | **YES** | Required for validator communication |
| 26657 | RPC | Optional | API access, queries |
| 9090 | gRPC | Optional | gRPC API |
| 1317 | API | Optional | REST API |
| 26660 | Prometheus | No | Metrics |

**Minimum required:** Only port 26656 needs to be accessible between validators.

---

## FAQ

**Q: Do both computers need the same operating system?**
A: No! Computer 1 can be Windows and Computer 2 can be Linux.

**Q: Can I test with both validators on the same computer?**
A: Yes! Use different ports for validator 2 (see single-computer testing guide).

**Q: How do I know if the network is working?**
A: Both validators should show the same block height and have 1 peer connected.

**Q: Can I use this for mainnet?**
A: This is for testing only. Mainnet requires additional security and configuration.

**Q: What's the minimum hardware required?**
A: 2GB RAM, 20GB disk, dual-core CPU minimum for testing.

**Q: How do I add more test accounts?**
A: Use `posd keys add <name> --keyring-backend test --home ./testnet-2nodes/validator1`

**Q: Can I use a VPN?**
A: Yes, but ensure the VPN allows peer-to-peer traffic on port 26656.

---

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review validator logs for errors
3. Verify network connectivity between computers
4. Check firewall configurations

## Additional Resources

- **3-Layer Fee System:** See `x/poc/client/cli/FEE_SYSTEM_GUIDE.md`
- **PoA Access Control:** See `POA_ACCESS_CONTROL_IMPLEMENTATION.md`
- **Complete Implementation:** See `3LAYER_FEE_IMPLEMENTATION_COMPLETE.md`

---

**Happy Testing!** 🚀
