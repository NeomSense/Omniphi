# Omniphi Blockchain - Complete Ubuntu Deployment Guide

**This is the ONLY guide you need for Ubuntu setup, deployment, and troubleshooting.**

---

## Table of Contents
1. [Quick Start](#quick-start)
2. [Fresh Installation](#fresh-installation)
3. [Manual Setup](#manual-setup)
4. [Troubleshooting](#troubleshooting)
5. [Advanced Configuration](#advanced-configuration)
6. [Verification & Testing](#verification--testing)

---

## Quick Start

### Three Simple Steps

```bash
cd ~/omniphi/pos
git pull origin main

# Step 1: Setup (automated)
chmod +x setup_ubuntu_fixed.sh
./setup_ubuntu_fixed.sh
# Answer 'Y' when prompted to remove old data

# Step 2: Start the chain
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

**That's it!** Within seconds you should see:
```
INF This node is a validator addr=...
INF finalizing commit of block height=1
INF finalized block height=2
```

---

## Fresh Installation

### Prerequisites

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Go 1.23+ (if not installed)
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version

# Install Git (if not installed)
sudo apt install git -y

# Install jq (for JSON parsing - optional but recommended)
sudo apt install jq -y
```

### Clone and Setup

```bash
# Clone the repository
cd ~
git clone https://github.com/NeomSense/PoS-PoC.git omniphi/pos
cd omniphi/pos

# Run automated setup
chmod +x setup_ubuntu_fixed.sh
./setup_ubuntu_fixed.sh
```

### What the Setup Script Does

The `setup_ubuntu_fixed.sh` script performs these steps automatically:

1. ✅ Pulls latest code from GitHub
2. ✅ Builds the `posd` binary
3. ✅ Initializes the chain (creates ~/.pos or ~/.posd)
4. ✅ Creates validator key
5. ✅ Adds genesis account (1,000,000 OMNI)
6. ✅ Fixes bond_denom to "uomni" (first time)
7. ✅ Configures minimum gas prices
8. ✅ **Creates gentx** (genesis transaction for validator)
9. ✅ **Collects gentxs** (adds to genutil.gen_txs)
10. ✅ **Re-fixes bond_denom** (collect-gentxs overwrites it)
11. ✅ Validates final genesis
12. ✅ Verifies all 3 modules are registered

**Critical Step Explained**: The bond_denom must be fixed TWICE because the `collect-gentxs` command overwrites it from "uomni" back to "omniphi". This is a known Cosmos SDK behavior.

### Starting the Chain

```bash
# Foreground (recommended for first test - see output directly)
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false

# Background (for production)
nohup ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false > chain.log 2>&1 &

# Watch logs (if running in background)
tail -f chain.log

# Stop the chain (if in background)
pkill posd
```

**CRITICAL**: You MUST use the `--grpc.enable=false` flag. Without it, the chain will crash with a gRPC reflection service error.

---

## Manual Setup

If you prefer to run each step manually instead of using the automated script, follow these detailed instructions.

### Prerequisites

Ensure you have:
- Go 1.23+ installed (`go version`)
- Git installed (`git --version`)
- jq installed (optional but recommended: `sudo apt install jq`)

### Step-by-Step Manual Setup

#### Step 1: Build the Binary

```bash
cd ~/omniphi/pos

# Pull latest code
git pull origin main

# Build the posd binary
go build -o posd ./cmd/posd

# Verify build
./posd version
ls -lh posd
```

#### Step 2: Clean Previous Data (if any)

```bash
# Remove any existing chain data
rm -rf ~/.pos ~/.posd

# Clean old logs
rm -f chain.log
```

#### Step 3: Initialize the Chain

```bash
# Set your chain variables
CHAIN_ID="omniphi-1"
MONIKER="omniphi-validator"
KEY_NAME="validator"
DENOM="uomni"

# Initialize chain
./posd init $MONIKER --chain-id $CHAIN_ID

# This creates ~/.pos/config/ directory with genesis.json and config files
```

#### Step 4: Create Validator Key

```bash
# Create a new key for your validator
./posd keys add $KEY_NAME --keyring-backend test

# IMPORTANT: Save the mnemonic phrase shown!
# You'll also see your address printed

# View your address anytime:
./posd keys show $KEY_NAME -a --keyring-backend test
```

**Note**: We use `--keyring-backend test` for development. For production, use `--keyring-backend file` or `--keyring-backend os`.

#### Step 5: Add Genesis Account

```bash
# Add your validator account to genesis with initial balance
# 1,000,000,000,000,000 uomni = 1,000,000,000 OMNI
./posd genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test

# Verify it was added
cat ~/.pos/config/genesis.json | jq '.app_state.bank.balances'
```

#### Step 6: Fix Bond Denomination (First Time)

```bash
# CRITICAL: Change bond_denom from "stake" to "uomni"
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# Verify the change
cat ~/.pos/config/genesis.json | jq '.app_state.staking.params.bond_denom'
# Should show: "uomni"
```

**Why?** The default Cosmos SDK uses "stake" as the bond denomination, but our chain uses "uomni".

#### Step 7: Configure Minimum Gas Prices

```bash
# Set minimum gas prices in app.toml
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' ~/.pos/config/app.toml

# Verify the change
grep "minimum-gas-prices" ~/.pos/config/app.toml
# Should show: minimum-gas-prices = "0.001uomni"
```

#### Step 8: Create Genesis Transaction (gentx)

```bash
# Create the genesis transaction that makes you a validator
# Staking 100,000,000,000 uomni = 100,000 OMNI
./posd genesis gentx $KEY_NAME 100000000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test

# Verify gentx was created
ls -lh ~/.pos/config/gentx/
# You should see a gentx-*.json file
```

**What is gentx?** A genesis transaction that:
- Creates your validator at chain launch
- Stakes your initial tokens
- Sets commission rates
- Is included in the genesis block

#### Step 9: Collect Genesis Transactions

```bash
# Collect all gentxs (in our case, just the one we created)
./posd genesis collect-gentxs

# This adds the gentx to genesis.json under app_state.genutil.gen_txs
```

**IMPORTANT**: This step modifies the genesis file!

#### Step 10: Fix Bond Denomination (Second Time)

```bash
# CRITICAL: The collect-gentxs command overwrites bond_denom!
# It changes it from "uomni" back to "omniphi"
# We need to fix it again
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# Verify it's correct
cat ~/.pos/config/genesis.json | jq '.app_state.staking.params.bond_denom'
# Should show: "uomni"
```

**Why twice?** This is a known Cosmos SDK behavior. The `collect-gentxs` command uses the denomination from the gentx file ("omniphi" by default) and overwrites the genesis bond_denom.

#### Step 11: Validate Genesis

```bash
# Validate that the genesis file is correct
./posd genesis validate-genesis

# If successful, you'll see no output (or success message)
# If there are errors, they'll be displayed
```

#### Step 12: Verify Genesis Content

```bash
# Check that everything is set up correctly
echo "=== Genesis Verification ==="

# 1. Check gen_txs array (should have 1 entry)
echo "gen_txs count:"
cat ~/.pos/config/genesis.json | jq '.app_state.genutil.gen_txs | length'

# 2. Check bond_denom
echo "bond_denom:"
cat ~/.pos/config/genesis.json | jq '.app_state.staking.params.bond_denom'

# 3. Check genesis balance
echo "Genesis balances:"
cat ~/.pos/config/genesis.json | jq '.app_state.bank.balances'

# 4. Check all three custom modules are present
echo "Custom modules:"
cat ~/.pos/config/genesis.json | jq 'has("app_state.feemarket"), has("app_state.tokenomics"), has("app_state.poc")'
```

**Expected results:**
- gen_txs count: `1`
- bond_denom: `"uomni"`
- Genesis balances: Should show your validator account
- Custom modules: All should be `true`

#### Step 13: Start the Chain

```bash
# Start in foreground (recommended for first run)
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false

# OR start in background
nohup ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false > chain.log 2>&1 &

# Watch logs (if in background)
tail -f chain.log
```

#### Step 14: Verify Chain is Running

Within a few seconds, you should see:

```
INF This node is a validator addr=0DEB609DE3393FD5084380B7E281B8E944F89460
INF finalizing commit of block height=1
INF finalized block height=2
INF committed state height=2
INF base fee updated module=server new_base_fee=0.039506172839506172
```

**Success indicators:**
- ✅ "This node is a validator" message
- ✅ Block heights increasing (1, 2, 3, ...)
- ✅ "base fee updated" messages (FeeMarket module working)
- ✅ No fatal errors

### Manual Setup Summary

Here's a complete script version of the manual setup for reference:

```bash
#!/bin/bash
# Complete manual setup - all commands in sequence

# Variables
CHAIN_ID="omniphi-1"
MONIKER="omniphi-validator"
KEY_NAME="validator"
DENOM="uomni"

# 1. Build
cd ~/omniphi/pos
git pull origin main
go build -o posd ./cmd/posd

# 2. Clean
rm -rf ~/.pos ~/.posd chain.log

# 3. Initialize
./posd init $MONIKER --chain-id $CHAIN_ID

# 4. Create key
./posd keys add $KEY_NAME --keyring-backend test

# 5. Add genesis account
./posd genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test

# 6. Fix bond_denom (first time)
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# 7. Configure gas prices
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' ~/.pos/config/app.toml

# 8. Create gentx
./posd genesis gentx $KEY_NAME 100000000000$DENOM \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test

# 9. Collect gentxs
./posd genesis collect-gentxs

# 10. Fix bond_denom (second time)
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# 11. Validate
./posd genesis validate-genesis

# 12. Verify
echo "=== Verification ==="
echo "gen_txs count: $(cat ~/.pos/config/genesis.json | jq '.app_state.genutil.gen_txs | length')"
echo "bond_denom: $(cat ~/.pos/config/genesis.json | jq -r '.app_state.staking.params.bond_denom')"

# 13. Start
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

### Understanding Key Concepts

#### Genesis Transaction (gentx)

A genesis transaction is special because:
- It's included in the **genesis block** (block 0)
- It creates validators **before** the chain starts
- It's the only way to bootstrap the validator set
- Without it, the chain cannot start ("validator set is empty" error)

#### Bond Denomination

The bond denomination (`bond_denom`) must:
- Match the denomination used in your genesis accounts
- Match the denomination in your gentx
- Be consistent throughout the genesis file
- For Omniphi: always "uomni"

#### Why Fix bond_denom Twice?

1. **After init**: Cosmos SDK defaults to "stake", we change to "uomni"
2. **After collect-gentxs**: This command reads from the gentx file which has "omniphi" encoded in it, and overwrites the genesis bond_denom
3. **Final fix**: Change it back to "uomni" to match our actual denomination

This is a quirk of Cosmos SDK's gentx workflow.

#### Keyring Backends

- `test`: No password, stores in plaintext - **for development only**
- `file`: Password-protected file storage - **for production**
- `os`: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- `kwallet`: KDE Wallet
- `pass`: Unix password manager

For production validators, always use `file` or `os` backends.

---

## Troubleshooting

### Error: "validator set is empty after InitGenesis"

**Cause**: The genesis transaction (gentx) wasn't created or collected properly.

**Diagnosis**: Run the diagnostic script first:

```bash
chmod +x fix_ubuntu.sh
./fix_ubuntu.sh
```

This will tell you exactly what's wrong.

**Solution**: Complete reset and setup:

```bash
# 1. Stop any running chain
pkill posd

# 2. Remove all chain data
rm -rf ~/.pos ~/.posd

# 3. Clean logs
rm -f chain.log

# 4. Run setup script
./setup_ubuntu_fixed.sh
# ANSWER 'Y' WHEN PROMPTED!

# 5. Start the chain
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

### Error: "cannot execute: required file not found"

**Cause**: Shell scripts have Windows line endings (CRLF) instead of Unix line endings (LF).

**Solution**: This is fixed in the repository with `.gitattributes`. Just pull latest:

```bash
git pull origin main
chmod +x setup_ubuntu_fixed.sh fix_ubuntu.sh
./setup_ubuntu_fixed.sh
```

If you still have issues:

```bash
# Manually fix line endings
sed -i 's/\r$//' setup_ubuntu_fixed.sh fix_ubuntu.sh
chmod +x setup_ubuntu_fixed.sh fix_ubuntu.sh
```

### Error: "failed to register reflection service"

**Cause**: gRPC reflection service has issues with the feemarket proto descriptors.

**Solution**: Always use `--grpc.enable=false` flag when starting:

```bash
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

The REST API still works fine without gRPC.

### Chain Shows Help Output Instead of Starting

**Cause**: Wrong command syntax or missing required parameters.

**Solution**: Use the EXACT command:

```bash
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

Check that:
- You're in the `~/omniphi/pos` directory
- The `posd` binary exists (`ls -lh posd`)
- Genesis was set up properly (`./fix_ubuntu.sh` to verify)

### Warning: "max block gas is zero or negative"

**Impact**: Non-fatal - blocks produce normally

**Cause**: Genesis consensus params not set

**Solution (Optional)**: Run the warning fix script:

```bash
pkill posd
chmod +x fix_genesis_warnings.sh
./fix_genesis_warnings.sh
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

### Warning: "insufficient supply for fee burn"

**Impact**: Non-fatal - only appears on first block

**Cause**: Genesis tokenomics supply is zero initially

**Solution (Optional)**: Same as above - run `fix_genesis_warnings.sh`

### Completely Reset Everything

If you want to start completely fresh:

```bash
# 1. Stop chain
pkill posd

# 2. Remove all data
rm -rf ~/.pos ~/.posd chain.log

# 3. Clean build artifacts (optional)
go clean

# 4. Pull latest code
git pull origin main

# 5. Run setup
./setup_ubuntu_fixed.sh

# 6. Start chain
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

---

## Advanced Configuration

### Custom Chain Parameters

Edit the genesis file BEFORE creating gentx:

```bash
# After 'posd init' but BEFORE 'posd genesis gentx'
nano ~/.pos/config/genesis.json
```

Key parameters:

```json
{
  "consensus": {
    "params": {
      "block": {
        "max_gas": "10000000"  // Max gas per block
      }
    }
  },
  "app_state": {
    "staking": {
      "params": {
        "bond_denom": "uomni"  // Must match your denomination
      }
    },
    "feemarket": {
      "params": {
        "min_gas_price": "0.001000000000000000",
        "target_block_utilization": "0.900000000000000000",
        "elasticity_multiplier": "1.125000000000000000"
      }
    }
  }
}
```

### Network Configuration

Allow external connections (default is localhost only):

```bash
nano ~/.pos/config/config.toml
```

```toml
[rpc]
laddr = "tcp://0.0.0.0:26657"
cors_allowed_origins = ["*"]

[p2p]
laddr = "tcp://0.0.0.0:26656"
```

### Minimum Gas Prices

Set in `app.toml`:

```bash
nano ~/.pos/config/app.toml
```

```toml
minimum-gas-prices = "0.001uomni"
```

Or override on startup:

```bash
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false
```

### Systemd Service (Production)

Create a systemd service for automatic restart:

```bash
sudo nano /etc/systemd/system/omniphi.service
```

```ini
[Unit]
Description=Omniphi Blockchain Node
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/home/YOUR_USERNAME/omniphi/pos
ExecStart=/home/YOUR_USERNAME/omniphi/pos/posd start --minimum-gas-prices 0.001uomni --grpc.enable=false --home /home/YOUR_USERNAME/.pos
Restart=on-failure
RestartSec=10
StandardOutput=append:/home/YOUR_USERNAME/omniphi/pos/chain.log
StandardError=append:/home/YOUR_USERNAME/omniphi/pos/chain.log

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable omniphi
sudo systemctl start omniphi
sudo systemctl status omniphi

# View logs
journalctl -u omniphi -f
```

---

## Verification & Testing

### Check Chain Status

```bash
# Query chain status
./posd status

# Should show block height increasing
./posd status | jq '.sync_info.latest_block_height'
```

### Verify All Modules Loaded

```bash
./posd query --help | grep -E "(feemarket|poc|tokenomics)"

# Expected output:
#   feemarket           Querying commands for the feemarket module
#   poc                 Querying commands for the poc module
#   tokenomics          Querying commands for the tokenomics module
```

### Query Module States

```bash
# FeeMarket state
./posd query feemarket params
./posd query feemarket base-fee
./posd query feemarket block-utilization

# Tokenomics state
./posd query tokenomics params
./posd query tokenomics supply

# POC state
./posd query poc params
```

### Check Validator Status

```bash
# List validators
./posd query staking validators

# Check your validator
./posd query staking validator $(./posd keys show validator --bech val -a --keyring-backend test)

# Check if node is validator (in logs)
grep "This node is a validator" chain.log
```

### Test Transaction

```bash
# Create a test account
./posd keys add test-account --keyring-backend test

# Send tokens
./posd tx bank send validator $(./posd keys show test-account -a --keyring-backend test) 1000uomni \
  --chain-id omniphi-1 \
  --keyring-backend test \
  --fees 100uomni \
  --yes

# Check balance
./posd query bank balances $(./posd keys show test-account -a --keyring-backend test)
```

### Monitor Block Production

```bash
# Watch blocks being produced in real-time
tail -f chain.log | grep "finalized block"

# Should see:
# INF finalized block height=1
# INF finalized block height=2
# INF finalized block height=3
# ...
```

### Check Adaptive Fee Market

```bash
# Watch base fee updates
tail -f chain.log | grep "base fee updated"

# Should see:
# INF base fee updated module=server new_base_fee=0.039506... old_base_fee=0.044444...
```

---

## Success Indicators

### ✅ Your chain is working correctly if you see:

```
INF This node is a validator addr=0DEB609DE3393FD5084380B7E281B8E944F89460
INF finalizing commit of block hash=... height=1
INF finalized block height=2
INF committed state height=2
INF base fee updated module=server new_base_fee=...
```

### ⚠️ Non-critical warnings (safe to ignore):

```
ERR max block gas is zero or negative maxBlockGas=-1
ERR insufficient supply for fee burn
```

These warnings don't prevent block production.

---

## Technical Details

### Chain Configuration

- **Chain ID**: `omniphi-1` (production) / `omniphi-test` (testing)
- **Denomination**: `uomni`
- **Minimum Gas Price**: `0.001uomni`
- **Consensus**: CometBFT (Tendermint)
- **SDK Version**: Cosmos SDK v0.53.3

### Directory Structure

```
~/.pos/                        # Chain home directory (or ~/.posd)
├── config/
│   ├── genesis.json          # Genesis configuration
│   ├── app.toml              # Application config
│   ├── config.toml           # Consensus/networking config
│   └── gentx/                # Genesis transactions
│       └── gentx-*.json      # Your validator gentx
├── data/                      # Blockchain data
└── keyring-test/             # Test keyring (dev only)
```

### Custom Modules

1. **FeeMarket** - Adaptive Fee Market v2
   - EIP-1559 style dynamic base fee
   - Utilization tracking and fee adjustments
   - Configurable elasticity and target utilization

2. **Tokenomics** - Token Economics
   - Fee distribution: burn, treasury, validators
   - Supply tracking and inflation control

3. **POC** - Proof of Contribution
   - Reputation system
   - Contribution tracking and rewards

### Critical Startup Flags

```bash
--minimum-gas-prices 0.001uomni   # Prevents spam transactions
--grpc.enable=false               # Bypasses proto descriptor issue
--home ~/.pos                     # Specify chain home (optional)
```

---

## Scripts Reference

### Automated Setup Scripts

- **setup_ubuntu_fixed.sh** - Complete automated setup with gentx workflow
- **fix_ubuntu.sh** - Diagnostic tool to identify issues
- **fix_genesis_warnings.sh** - Optional script to eliminate warnings

See the [Manual Setup](#manual-setup) section for step-by-step instructions if you prefer to run commands manually.

---

## Common Commands Cheat Sheet

```bash
# Chain Management
./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false  # Start chain
pkill posd                                                         # Stop chain
./posd status                                                      # Check status

# Logs
tail -f chain.log                                                  # Follow logs
grep "validator" chain.log                                         # Search logs

# Keys
./posd keys list --keyring-backend test                           # List keys
./posd keys show validator -a --keyring-backend test              # Show address

# Queries
./posd query bank balances <address>                              # Check balance
./posd query staking validators                                   # List validators
./posd query feemarket base-fee                                   # Current base fee

# Transactions
./posd tx bank send <from> <to> <amount>uomni --fees 100uomni --yes  # Send tokens

# Diagnostics
./fix_ubuntu.sh                                                    # Run diagnostics
./posd version                                                     # Check version
go version                                                         # Check Go version
```

---

## Getting Help

### Documentation
- **This Guide** - Complete Ubuntu deployment reference
- **DEPLOYMENT_SUCCESS.md** - Overall deployment status
- **VALIDATOR_FIX_SUMMARY.md** - Deep dive into validator genesis

### Repository
- **GitHub**: [https://github.com/NeomSense/PoS-PoC](https://github.com/NeomSense/PoS-PoC)

### Common Issues
1. Run `./fix_ubuntu.sh` for diagnostics
2. Check [Troubleshooting](#troubleshooting) section above
3. Ensure you're using `--grpc.enable=false` flag
4. Verify genesis setup completed all 13 steps

---

*Last Updated: 2025-11-05*
*Tested on: Ubuntu 20.04, 22.04*
*Cosmos SDK: v0.53.3*
