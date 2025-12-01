# Validator Key Management Guide

**Critical security guide for managing Omniphi validator keys.**

⚠️ **WARNING:** Improper key management can result in total loss of funds or slashing penalties. Read this entire guide before operating a validator.

---

## Overview

Omniphi validators use **two types of keys**:

1. **Consensus Private Key** (Validator Key)
   - Used to sign blocks and participate in consensus
   - Generated and stored on the validator node
   - File: `~/.omniphi/config/priv_validator_key.json`
   - **Loss = Validator cannot sign blocks**
   - **Compromise = Slashing risk (double-signing)**

2. **Operator Wallet Key** (Account Key)
   - Used to manage validator account (staking, commission, withdrawals)
   - Stored in keyring (NOT on validator node)
   - Managed by `posd keys` or wallet app (Keplr, Leap)
   - **Loss = Cannot manage validator**
   - **Compromise = Funds can be stolen**

---

## Key Differences

| Property | Consensus Key | Operator Key |
|----------|--------------|-------------|
| **Purpose** | Sign blocks | Manage validator account |
| **Location** | Validator node | Wallet/keyring |
| **Format** | Ed25519 | Secp256k1 (Cosmos standard) |
| **Backup** | Manual file backup | Mnemonic phrase |
| **Recovery** | Restore from file | Restore from mnemonic |
| **Risk if lost** | Cannot sign blocks | Cannot manage validator |
| **Risk if stolen** | Double-signing (slashing) | Funds stolen |
| **Rotation** | Possible (requires validator update) | Possible (requires tx) |

---

## Part 1: Consensus Private Key Management

### Understanding the Consensus Key

**File location:**
```
~/.omniphi/config/priv_validator_key.json
```

**File contents:**
```json
{
  "address": "1234ABCD...",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "base64_encoded_public_key"
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "base64_encoded_PRIVATE_KEY"
  }
}
```

⚠️ **The `priv_key.value` field is HIGHLY SENSITIVE!**

---

### Initial Key Generation

**Method 1: Automatic (during init)**
```bash
# Initialize node - automatically generates consensus key
posd init my-validator --chain-id omniphi-1

# Key is created at: ~/.omniphi/config/priv_validator_key.json
```

**Method 2: Manual generation**
```bash
# Generate key explicitly
posd tendermint show-validator

# This reads from priv_validator_key.json and displays public key
```

---

### Viewing Your Consensus Public Key

**Get consensus public key (safe to share):**
```bash
# Method 1: Direct command
posd tendermint show-validator

# Output example:
# {"@type":"/cosmos.crypto.ed25519.PubKey","key":"ABC123..."}

# Method 2: From file
cat ~/.omniphi/config/priv_validator_key.json | jq '.pub_key'
```

**This public key is used when creating your validator on-chain.**

---

### Backing Up Consensus Key

**CRITICAL: Backup immediately after generation!**

#### Backup Method 1: Encrypted File Backup (Recommended)

```bash
# Create encrypted backup using GPG
gpg --symmetric --cipher-algo AES256 \
  ~/.omniphi/config/priv_validator_key.json

# Creates: priv_validator_key.json.gpg (password-encrypted)
# Store this file in multiple secure locations:
# - Encrypted USB drive
# - Password manager (1Password, Bitwarden)
# - Offline hardware wallet (if supported)
# - Secure cloud storage (encrypted)

# NEVER store unencrypted backups!
```

**Restore from encrypted backup:**
```bash
# Decrypt backup
gpg --decrypt priv_validator_key.json.gpg > priv_validator_key.json

# Copy to validator
cp priv_validator_key.json ~/.omniphi/config/
chmod 600 ~/.omniphi/config/priv_validator_key.json
```

#### Backup Method 2: Print to Paper (Cold Storage)

```bash
# Print key as QR code for offline storage
cat ~/.omniphi/config/priv_validator_key.json | qrencode -o validator-key-qr.png

# Or print as text (less secure)
cat ~/.omniphi/config/priv_validator_key.json

# Store printed copy in:
# - Safe deposit box
# - Fireproof safe
# - Multiple secure physical locations
```

**Recovery from paper:**
```bash
# Scan QR code or manually type JSON
# Verify JSON is valid
cat priv_validator_key.json | jq .

# Copy to validator
cp priv_validator_key.json ~/.omniphi/config/
chmod 600 ~/.omniphi/config/priv_validator_key.json
```

---

### Securing Consensus Key on Validator

**File permissions (CRITICAL):**
```bash
# Set restrictive permissions (owner read/write only)
chmod 600 ~/.omniphi/config/priv_validator_key.json

# Verify
ls -la ~/.omniphi/config/priv_validator_key.json
# Should show: -rw------- (600)

# Set ownership to validator service user
sudo chown omniphi:omniphi ~/.omniphi/config/priv_validator_key.json
```

**Disable root access (optional but recommended):**
```bash
# Use immutable flag to prevent accidental deletion
sudo chattr +i ~/.omniphi/config/priv_validator_key.json

# To modify later (e.g., for key rotation):
sudo chattr -i ~/.omniphi/config/priv_validator_key.json
```

---

### Hardware Security Modules (HSM) - Advanced

For high-value validators, use a hardware security module to store the consensus private key.

**Supported HSM solutions:**

1. **Tendermint KMS (tmkms)**
   - Supports YubiHSM2, Ledger Nano
   - Runs on separate machine
   - Validator connects via secure channel

**Setup example (YubiHSM2):**
```bash
# Install tmkms
cargo install tmkms --features=yubihsm

# Initialize configuration
tmkms init /etc/tmkms

# Configure validator connection
nano /etc/tmkms/tmkms.toml
```

**tmkms.toml example:**
```toml
[[validator]]
chain_id = "omniphi-1"
addr = "tcp://VALIDATOR_IP:26658"  # Private validator RPC
protocol_version = "v0.34"

[[providers.yubihsm]]
adapter = { type = "usb" }
auth = { key = 1, password_file = "/etc/tmkms/password" }
keys = [
    { chain_ids = ["omniphi-1"], key = 1 }
]
```

**Benefits:**
- Private key never leaves HSM
- Protection against server compromise
- Slashing protection built-in

**Drawbacks:**
- Additional hardware cost ($500-1000)
- More complex setup
- Single point of failure (must have backup HSM)

---

### Double-Signing Protection

**What is double-signing?**
- Signing two different blocks at the same height
- Results in **slashing penalty** (loss of staked tokens)
- Usually happens when running two validators with the same key

**Prevention strategies:**

#### 1. Never Copy Running Validator

```bash
# ❌ NEVER DO THIS:
# scp ~/.omniphi/config/priv_validator_key.json backup-validator:/path/

# If you need a backup validator, use:
# - Different consensus key (different validator)
# - Conditional failover (only one active at a time)
# - Horcrux (distributed validator)
```

#### 2. Monitor `priv_validator_state.json`

```bash
# This file tracks the last height/round signed
cat ~/.omniphi/data/priv_validator_state.json
```

**Contents:**
```json
{
  "height": "12345",
  "round": 0,
  "step": 3,
  "signature": "...",
  "signbytes": "..."
}
```

**If you restore a validator from backup:**
```bash
# Ensure state file is also restored (or reset safely)
# Never start with old state file on a different server!
```

#### 3. Use Automatic Slashing Protection

**With tmkms (recommended):**
```toml
# In tmkms.toml:
[[validator]]
chain_id = "omniphi-1"
# tmkms automatically tracks height/round and prevents double-signing
```

**With Horcrux (distributed validator):**
```bash
# Horcrux splits signing across multiple nodes
# Prevents double-signing even if one node is compromised
```

---

### Key Rotation (Advanced)

**When to rotate consensus key:**
- Key potentially compromised
- Migrating to HSM
- Security policy requires periodic rotation

**Steps:**

1. **Generate new key on new validator node:**
```bash
# On NEW validator server:
posd init new-validator --chain-id omniphi-1
# New key created at ~/.omniphi/config/priv_validator_key.json

# Get new consensus public key:
posd tendermint show-validator
```

2. **Update validator on-chain:**
```bash
# From operator wallet:
posd tx staking edit-validator \
  --pubkey='{"@type":"/cosmos.crypto.ed25519.PubKey","key":"NEW_PUBKEY"}' \
  --chain-id=omniphi-1 \
  --from=my-validator-wallet \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni
```

3. **Stop old validator, start new validator:**
```bash
# On OLD validator:
sudo systemctl stop posd

# On NEW validator:
sudo systemctl start posd
```

4. **Verify new validator is signing:**
```bash
# Check recent blocks
posd query block | jq '.block.last_commit.signatures'

# Verify your validator is in the list
```

5. **Securely destroy old key:**
```bash
# On OLD validator (after confirming new validator works):
# Backup first (just in case)
gpg --symmetric ~/.omniphi/config/priv_validator_key.json

# Then securely delete
shred -vfz -n 10 ~/.omniphi/config/priv_validator_key.json
shred -vfz -n 10 ~/.omniphi/data/priv_validator_state.json
```

---

## Part 2: Operator Wallet Key Management

### Understanding the Operator Key

**Purpose:**
- Create validator on-chain
- Edit validator metadata
- Change commission
- Withdraw rewards
- Unjail validator

**Storage:**
- NOT on validator node (security risk)
- In keyring (OS keychain, encrypted file, or hardware wallet)

---

### Creating an Operator Wallet

#### Method 1: Using `posd keys` (CLI)

```bash
# Add new key to keyring
posd keys add my-validator-wallet

# Enter keyring password (if using "file" backend)
# Output:
# - Address: omni1abc123...
# - Public key: omnipub1abc...
# - Mnemonic: word1 word2 word3 ... word24

# ⚠️ WRITE DOWN MNEMONIC IMMEDIATELY!
# This is the ONLY way to recover your wallet.
```

#### Method 2: Using Keplr/Leap Wallet (GUI)

1. Install Keplr browser extension
2. Create new wallet → **Save mnemonic phrase**
3. Add Omniphi network to Keplr
4. Use this address for validator operations

---

### Backing Up Operator Key

**CRITICAL: Backup mnemonic phrase!**

#### Backup Method 1: Physical Storage (Recommended)

```bash
# Write down 24-word mnemonic on paper
# Example:
# 1. abandon  2. ability  3. able  ... 24. zebra

# Store copies in:
# - Fireproof safe at home
# - Safe deposit box
# - Trusted family member (if using Shamir's Secret Sharing)
```

**Best practices:**
- Use metal plates (fireproof, waterproof)
- Never take photos of mnemonic
- Never store in cloud (Google Drive, iCloud, etc.)
- Never email to yourself
- Use Shamir's Secret Sharing to split across multiple locations

#### Backup Method 2: Encrypted Digital Storage

```bash
# Encrypt mnemonic with strong password
echo "word1 word2 ... word24" | gpg --symmetric --cipher-algo AES256 > mnemonic.gpg

# Store encrypted file in password manager
# Also store encryption password separately

# Verify decryption works:
gpg --decrypt mnemonic.gpg
```

---

### Keyring Backends

**Choose keyring backend based on security needs:**

#### 1. OS Keyring (Most Secure)

```bash
# In ~/.omniphi/config/client.toml:
keyring-backend = "os"
```

**Storage locations:**
- **macOS:** Keychain Access
- **Windows:** Windows Credential Manager
- **Linux:** Secret Service API (GNOME Keyring, KDE Wallet)

**Pros:**
- Integrated with OS security
- Hardware-backed on some systems
- No password required on each use (OS handles auth)

**Cons:**
- Harder to backup/restore
- Requires GUI access on Linux

---

#### 2. File Keyring (Balanced)

```bash
# In ~/.omniphi/config/client.toml:
keyring-backend = "file"
```

**Storage:**
- Encrypted files in `~/.omniphi/keyring-file/`
- Password required on each use

**Pros:**
- Easy to backup
- Works on headless servers
- Cross-platform

**Cons:**
- Must enter password for each transaction
- Vulnerable if password is weak

---

#### 3. Test Keyring (INSECURE - Development Only)

```bash
# In ~/.omniphi/config/client.toml:
keyring-backend = "test"
```

**Storage:**
- Unencrypted files
- **NO PASSWORD REQUIRED**

**⚠️ NEVER USE IN PRODUCTION!**
- Anyone with file access can steal keys
- Only for local testing/development

---

### Hardware Wallet Integration (Recommended for Large Validators)

**Supported hardware wallets:**
- Ledger Nano S/S Plus/X

**Setup:**

1. **Install Ledger app:**
   - Install "Cosmos (ATOM)" app on Ledger
   - Enable "Developer Mode" in Ledger Live

2. **Add Ledger key to posd:**
```bash
# Add Ledger key
posd keys add my-ledger-wallet --ledger

# Verify
posd keys list
```

3. **Use Ledger for transactions:**
```bash
# Approve transaction on Ledger device
posd tx staking create-validator \
  --from=my-ledger-wallet \
  --ledger \
  ... (other flags)
```

**Benefits:**
- Private key never leaves device
- Physical confirmation required for transactions
- Protected against malware

**Drawbacks:**
- Must have Ledger physically present
- Slower transaction signing
- Cost ($60-200)

---

### Operator Key Security Best Practices

1. **Never store operator key on validator node**
   ```bash
   # ❌ BAD:
   # posd keys add my-wallet  (on validator server)

   # ✅ GOOD:
   # Use Ledger or separate machine for keys
   ```

2. **Use different machines for different tasks:**
   - **Validator server:** Only consensus key
   - **Management machine:** Operator wallet key
   - **Cold storage:** Backup keys

3. **Enable 2FA on associated accounts:**
   - Cloud provider (AWS, DigitalOcean)
   - GitHub (if keys in repo)
   - Email (for recovery)

4. **Regular security audits:**
   ```bash
   # Check for exposed keys
   grep -r "priv_validator_key" ~/
   grep -r "mnemonic" ~/

   # Review keyring contents
   posd keys list
   ```

---

## Part 3: Key Disaster Recovery

### Scenario 1: Lost Consensus Key (No Backup)

**Impact:**
- Validator cannot sign blocks
- Will be jailed after downtime threshold
- Must create new validator on-chain

**Recovery steps:**

1. **Generate new consensus key:**
```bash
# On new validator server:
posd init new-validator --chain-id omniphi-1
```

2. **Option A: Create new validator**
```bash
# Requires new tokens for self-delegation
posd tx staking create-validator ... (full setup)
```

3. **Option B: Update existing validator (if you have operator key)**
```bash
# Update consensus pubkey (requires governance proposal in some chains)
# Check if Omniphi allows this
```

---

### Scenario 2: Lost Operator Key (No Mnemonic)

**Impact:**
- Cannot manage validator
- Cannot withdraw rewards
- Cannot change commission
- **TOTAL LOSS if no recovery**

**Recovery steps:**

**If you have no mnemonic backup → NO RECOVERY POSSIBLE**

**Prevention:**
- Always backup mnemonic during wallet creation
- Test recovery before using wallet for real funds

---

### Scenario 3: Compromised Consensus Key

**Signs of compromise:**
- Unexpected double-signing (slashing event)
- Validator signing blocks you didn't run
- Unauthorized access to validator server

**Immediate actions:**

1. **Stop validator immediately:**
```bash
sudo systemctl stop posd
```

2. **Rotate consensus key (see Key Rotation section)**

3. **Investigate breach:**
```bash
# Check SSH access logs
sudo grep -i "Accepted" /var/log/auth.log

# Check for malware
sudo rkhunter --check
sudo chkrootkit

# Review firewall rules
sudo ufw status verbose
```

4. **Harden security:**
   - Change all passwords
   - Rotate SSH keys
   - Enable 2FA
   - Audit firewall rules
   - Review access logs

---

### Scenario 4: Compromised Operator Key

**Signs of compromise:**
- Unexpected transactions from your address
- Balance decreased without your actions
- Rewards withdrawn without authorization

**Immediate actions:**

1. **Create new wallet:**
```bash
posd keys add new-validator-wallet
# BACKUP MNEMONIC IMMEDIATELY!
```

2. **Transfer remaining funds:**
```bash
# Move any remaining tokens to new wallet
posd tx bank send \
  old-wallet \
  <new-wallet-address> \
  <amount>uomni \
  --chain-id=omniphi-1
```

3. **Update validator operator address:**
```bash
# This may not be possible on all chains
# Check Omniphi governance for process
```

4. **Report incident:**
   - Notify Omniphi team
   - Report to authorities if large loss
   - Document for insurance (if applicable)

---

## Part 4: Multi-Signature Wallets (Advanced)

For organizations running validators, use multi-sig wallets for operator key.

**Setup multisig:**

```bash
# Create multisig wallet (2-of-3 threshold)
posd keys add key1
posd keys add key2
posd keys add key3

# Create multisig account
posd keys add multisig-wallet \
  --multisig=key1,key2,key3 \
  --multisig-threshold=2
```

**Use multisig for validator operations:**

```bash
# Step 1: Create unsigned tx
posd tx staking create-validator ... \
  --from=multisig-wallet \
  --generate-only > unsigned.json

# Step 2: Sign with first key
posd tx sign unsigned.json \
  --from=key1 \
  --multisig=<multisig-address> \
  --chain-id=omniphi-1 > signed1.json

# Step 3: Sign with second key
posd tx sign unsigned.json \
  --from=key2 \
  --multisig=<multisig-address> \
  --chain-id=omniphi-1 > signed2.json

# Step 4: Combine signatures
posd tx multisign unsigned.json multisig-wallet \
  signed1.json signed2.json > signed.json

# Step 5: Broadcast
posd tx broadcast signed.json
```

**Benefits:**
- No single point of failure
- Requires consensus for validator operations
- Compliance with organizational policies

---

## Summary Checklist

### ✅ Consensus Key Security
- [ ] Consensus key backed up (encrypted)
- [ ] File permissions set to 600
- [ ] Backup stored in 2+ secure locations
- [ ] Double-signing protection enabled
- [ ] Consider HSM for high-value validators

### ✅ Operator Key Security
- [ ] Mnemonic backed up (physical + encrypted digital)
- [ ] Keyring backend set to "file" or "os" (NOT "test")
- [ ] Consider hardware wallet (Ledger)
- [ ] Operator key NOT stored on validator node
- [ ] Test recovery before using for real funds

### ✅ Operational Security
- [ ] Regular backups (automated)
- [ ] Security audits (quarterly)
- [ ] Incident response plan documented
- [ ] Team trained on key management
- [ ] Insurance considered (for large validators)

---

**Remember:** Validators are high-value targets. Invest in security proportional to the value you're securing.

---

**Need help?** Contact Omniphi security team: security@omniphi.io
