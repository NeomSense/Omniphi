# Omniphi Blockchain - Windows Quick Start Guide

**Version:** 1.0 | **Last Updated:** 2025-11-20
**Platform:** Windows 10/11 with PowerShell or WSL2

Get your Omniphi validator node running in **5 minutes** or follow the comprehensive guide for production deployment.

---

## 🎯 Choose Your Approach

Windows users have two options:

1. **WSL2 (Recommended)** - Native Linux environment on Windows
2. **Native Windows** - Run directly on Windows with PowerShell

---

## 🚀 Option 1: WSL2 Quick Start (Recommended)

### Prerequisites

**Install WSL2:**
```powershell
# Run in PowerShell as Administrator
wsl --install
```

Restart your computer after installation.

### 5-Minute Setup

```bash
# 1. Open Ubuntu (from Start Menu)

# 2. Update packages
sudo apt update && sudo apt upgrade -y

# 3. Install Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 4. Clone repository
git clone https://github.com/omniphi/pos.git
cd pos

# 5. Build
make install

# 6. Initialize
posd init my-node --chain-id omniphi-testnet-1

# 7. Download genesis
curl -o ~/.posd/config/genesis.json https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json

# 8. Start
posd start
```

**Done!** Your node is syncing.

**Note:** After setup, refer to the [Ubuntu Quick Start Guide](BLOCKCHAIN_QUICKSTART_UBUNTU.md) for all commands - they work identically in WSL2.

---

## 🖥️ Option 2: Native Windows Quick Start

### Prerequisites

**Install Go:**
1. Download: https://go.dev/dl/go1.21.0.windows-amd64.msi
2. Run installer
3. Verify in PowerShell:
   ```powershell
   go version
   ```

**Install Git:**
1. Download: https://git-scm.com/download/win
2. Run installer (use default settings)

**Install Build Tools:**
1. Download: https://visualstudio.microsoft.com/downloads/
2. Install "Desktop development with C++"

### 5-Minute Setup

```powershell
# 1. Clone repository
git clone https://github.com/omniphi/pos.git
cd pos

# 2. Build
go build -o build/posd.exe ./cmd/posd

# 3. Add to PATH (current session)
$env:PATH += ";$PWD\build"

# 4. Initialize
.\build\posd.exe init my-node --chain-id omniphi-testnet-1

# 5. Download genesis
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json" `
  -OutFile "$env:USERPROFILE\.posd\config\genesis.json"

# 6. Start
.\build\posd.exe start
```

**Done!** Your node is syncing.

---

## 📋 Detailed WSL2 Setup

### Install WSL2

```powershell
# PowerShell as Administrator
wsl --install -d Ubuntu

# Restart computer

# After restart, open "Ubuntu" from Start Menu
# Create username and password when prompted
```

### Configure WSL2

```bash
# Inside Ubuntu terminal

# Update system
sudo apt update && sudo apt upgrade -y

# Install essential tools
sudo apt install -y build-essential git curl wget jq
```

### Install Go in WSL2

```bash
# Download Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# Remove old version (if exists)
sudo rm -rf /usr/local/go

# Install new version
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# Add to PATH permanently
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc

# Reload configuration
source ~/.bashrc

# Verify
go version
```

### Build and Run

```bash
# Clone repository
git clone https://github.com/omniphi/pos.git
cd pos

# Build and install
make install

# Initialize node
posd init my-validator --chain-id omniphi-testnet-1

# Download genesis
curl -o ~/.posd/config/genesis.json \
  https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json

# Set minimum gas prices
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.025uomni"/' ~/.posd/config/app.toml

# Start node
posd start
```

---

## 📋 Detailed Native Windows Setup

### Build from Source

```powershell
# Clone repository
git clone https://github.com/omniphi/pos.git
cd pos

# Build binary
go build -o build\posd.exe .\cmd\posd

# Verify build
.\build\posd.exe version
```

### Add to PATH Permanently

**Option 1: PowerShell Profile**
```powershell
# Edit profile
notepad $PROFILE

# Add this line:
$env:PATH += ";C:\Users\YourName\omniphi\pos\build"

# Save and reload
. $PROFILE
```

**Option 2: System Environment Variables**
1. Press `Win + X` → System
2. Advanced System Settings → Environment Variables
3. Under "User variables", edit `Path`
4. Add: `C:\Users\YourName\omniphi\pos\build`
5. Click OK and restart PowerShell

### Initialize Node

```powershell
# Initialize
posd init my-validator --chain-id omniphi-testnet-1

# This creates: C:\Users\YourName\.posd\
```

### Download Genesis File

```powershell
# Download genesis
$genesisUrl = "https://raw.githubusercontent.com/omniphi/networks/main/testnet/genesis.json"
$genesisPath = "$env:USERPROFILE\.posd\config\genesis.json"
Invoke-WebRequest -Uri $genesisUrl -OutFile $genesisPath
```

### Configure Minimum Gas Prices

```powershell
# Edit app.toml
notepad $env:USERPROFILE\.posd\config\app.toml

# Find line:
# minimum-gas-prices = ""

# Change to:
# minimum-gas-prices = "0.025uomni"

# Save and close
```

### Start Node

```powershell
# Start node
posd start

# Or with custom home directory
posd start --home C:\custom\path\.posd
```

---

## 🌐 Two-Node Testnet on Windows

### Using WSL2 (Recommended)

Follow the [Ubuntu Two-Node Setup](BLOCKCHAIN_QUICKSTART_UBUNTU.md#-two-node-testnet-setup) - it works identically in WSL2.

### Using Native Windows

```powershell
# 1. Create directories for two validators
New-Item -ItemType Directory -Path "$env:USERPROFILE\testnet\validator1" -Force
New-Item -ItemType Directory -Path "$env:USERPROFILE\testnet\validator2" -Force

# 2. Initialize both
posd init validator1 --chain-id omniphi-testnet-1 --home "$env:USERPROFILE\testnet\validator1"
posd init validator2 --chain-id omniphi-testnet-1 --home "$env:USERPROFILE\testnet\validator2"

# 3. Create keys
posd keys add validator1 --home "$env:USERPROFILE\testnet\validator1"
posd keys add validator2 --home "$env:USERPROFILE\testnet\validator2"

# 4. Configure validator2 ports
# Edit: $env:USERPROFILE\testnet\validator2\config\config.toml
# Change:
# [rpc]
# laddr = "tcp://127.0.0.1:26757"
# [p2p]
# laddr = "tcp://0.0.0.0:26756"

# 5. Add accounts to genesis
$val1Addr = posd keys show validator1 -a --home "$env:USERPROFILE\testnet\validator1"
$val2Addr = posd keys show validator2 -a --home "$env:USERPROFILE\testnet\validator2"

posd genesis add-genesis-account $val1Addr 100000000000uomni --home "$env:USERPROFILE\testnet\validator1"
posd genesis add-genesis-account $val2Addr 100000000000uomni --home "$env:USERPROFILE\testnet\validator1"

# 6. Create gentx
posd genesis gentx validator1 10000000000uomni --chain-id omniphi-testnet-1 --home "$env:USERPROFILE\testnet\validator1"
posd genesis gentx validator2 10000000000uomni --chain-id omniphi-testnet-1 --home "$env:USERPROFILE\testnet\validator2"

# 7. Copy gentx files
Copy-Item "$env:USERPROFILE\testnet\validator2\config\gentx\*.json" "$env:USERPROFILE\testnet\validator1\config\gentx\"

# 8. Collect gentxs
posd genesis collect-gentxs --home "$env:USERPROFILE\testnet\validator1"

# 9. Copy genesis to validator2
Copy-Item "$env:USERPROFILE\testnet\validator1\config\genesis.json" "$env:USERPROFILE\testnet\validator2\config\genesis.json"

# 10. Get validator1 node ID
$val1NodeId = posd tendermint show-node-id --home "$env:USERPROFILE\testnet\validator1"
Write-Host "Validator1 Node ID: $val1NodeId"

# 11. Configure validator2 persistent peers
# Edit: $env:USERPROFILE\testnet\validator2\config\config.toml
# Set: persistent_peers = "$val1NodeId@127.0.0.1:26656"

# 12. Start validators (two separate PowerShell windows)
# Window 1:
posd start --home "$env:USERPROFILE\testnet\validator1"

# Window 2:
posd start --home "$env:USERPROFILE\testnet\validator2"
```

---

## 🛠️ Common Commands

### Node Management (PowerShell)

```powershell
# Start node
posd start

# With custom home
posd start --home C:\custom\path\.posd

# Check status
posd status

# Show node ID
posd tendermint show-node-id

# Show validator
posd tendermint show-validator
```

### Wallet Operations (PowerShell)

```powershell
# Create wallet
posd keys add my-wallet

# List wallets
posd keys list

# Show address
posd keys show my-wallet -a

# Export private key
posd keys export my-wallet

# Import from mnemonic
posd keys add my-wallet --recover
```

### Query Commands (PowerShell)

```powershell
# Check balance
$addr = posd keys show my-wallet -a
posd query bank balances $addr

# Check validator info
$valAddr = posd keys show my-wallet --bech val -a
posd query staking validator $valAddr
```

### Transaction Commands (PowerShell)

```powershell
# Send tokens
posd tx bank send my-wallet omni1recipient... 1000000uomni `
  --chain-id omniphi-testnet-1 `
  --fees 5000uomni

# Delegate to validator
$valAddr = posd keys show my-wallet --bech val -a
posd tx staking delegate $valAddr 1000000uomni `
  --from my-wallet `
  --chain-id omniphi-testnet-1 `
  --fees 5000uomni
```

---

## 🔧 Troubleshooting

### WSL2 Issues

**Issue: "WSL 2 requires an update to its kernel component"**

**Solution:**
1. Download: https://aka.ms/wsl2kernel
2. Install the update
3. Restart computer

**Issue: "Cannot connect to the Docker daemon"**

**Solution:**
```bash
# Ensure Docker Desktop is running with WSL2 backend
# Or install Docker in WSL:
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

### Native Windows Issues

**Issue: "posd: command not found"**

**Solution:**
```powershell
# Ensure build directory is in PATH
$env:PATH += ";$PWD\build"

# Or use full path
.\build\posd.exe version
```

**Issue: "Error: failed to initialize database"**

**Solution:**
```powershell
# Ensure .posd directory exists
New-Item -ItemType Directory -Path "$env:USERPROFILE\.posd" -Force

# Reinitialize
posd init my-node --chain-id omniphi-testnet-1
```

**Issue: "Error: minimum gas price not set"**

**Solution:**
```powershell
# Edit app.toml
notepad $env:USERPROFILE\.posd\config\app.toml

# Set: minimum-gas-prices = "0.025uomni"
```

**Issue: "Build fails with go errors"**

**Solution:**
```powershell
# Update Go to latest version
# Download from: https://go.dev/dl/

# Clean build cache
go clean -cache -modcache

# Rebuild
go build -o build\posd.exe .\cmd\posd
```

### Path Issues

**Issue: "File paths with spaces cause errors"**

**Solution:**
```powershell
# Use quotes for paths with spaces
posd start --home "C:\Users\My Name\.posd"

# Or avoid spaces in directory names
# Use: C:\Users\MyName instead of C:\Users\My Name
```

---

## 🚀 Running as Windows Service

For production, run the node as a Windows Service.

### Using NSSM (Non-Sucking Service Manager)

**Install NSSM:**
1. Download: https://nssm.cc/download
2. Extract to `C:\nssm`
3. Add to PATH

**Create Service:**
```powershell
# Install service
nssm install Omniphi "C:\Users\YourName\omniphi\pos\build\posd.exe"

# Set arguments
nssm set Omniphi AppParameters start

# Set working directory
nssm set Omniphi AppDirectory "C:\Users\YourName\omniphi\pos"

# Set auto-restart
nssm set Omniphi AppStopMethodSkip 0
nssm set Omniphi AppExit Default Restart

# Start service
nssm start Omniphi

# Check status
nssm status Omniphi
```

**Service Commands:**
```powershell
# Start
nssm start Omniphi

# Stop
nssm stop Omniphi

# Restart
nssm restart Omniphi

# Remove
nssm remove Omniphi confirm
```

---

## 📚 Next Steps

### For Development

- **WSL2 Users**: Follow [UBUNTU_TESTING_GUIDE.md](UBUNTU_TESTING_GUIDE.md)
- **Native Windows**: Follow [WINDOWS_TESTING_GUIDE.md](WINDOWS_TESTING_GUIDE.md)

### For Production

- **Production Checklist**: [PRODUCTION_DEPLOYMENT_CHECKLIST.md](PRODUCTION_DEPLOYMENT_CHECKLIST.md)
- **Security**: [SECURITY_AUDIT_REPORT.md](SECURITY_AUDIT_REPORT.md)
- **Monitoring**: [MONITORING.md](MONITORING.md)

### Advanced Topics

- **Multi-Node**: [MULTI_NODE_TESTNET_GUIDE.md](MULTI_NODE_TESTNET_GUIDE.md)
- **Tokenomics**: [TOKENOMICS_FULL_REPORT.md](TOKENOMICS_FULL_REPORT.md)

---

## 🆘 Getting Help

- **Documentation**: [README.md](README.md)
- **Discord**: https://discord.gg/omniphi
- **GitHub Issues**: https://github.com/omniphi/pos/issues
- **Email**: support@omniphi.io

---

## 💡 Tips for Windows Users

### WSL2 vs Native Windows

| Feature | WSL2 | Native Windows |
|---------|------|----------------|
| **Performance** | ⭐⭐⭐⭐⭐ Better | ⭐⭐⭐ Good |
| **Compatibility** | ⭐⭐⭐⭐⭐ Perfect | ⭐⭐⭐⭐ Excellent |
| **Setup Complexity** | ⭐⭐⭐ Medium | ⭐⭐⭐⭐ Easy |
| **Production Ready** | ✅ Yes | ✅ Yes |
| **Recommended For** | Development, Production | Quick testing |

### File Path Notes

- **WSL2**: Use Linux paths: `/home/user/pos`
- **Native**: Use Windows paths: `C:\Users\user\pos`
- **Access WSL2 files from Windows**: `\\wsl$\Ubuntu\home\user\pos`
- **Access Windows files from WSL2**: `/mnt/c/Users/user/pos`

---

**Denomination Reference:**
- `uomni` = 0.000001 OMNI (micro-omni)
- `1 OMNI` = 1,000,000 uomni
- Example: `1000000uomni` = 1 OMNI
