# Backup and Restore Guide for Omniphi Validators

**Critical guide for protecting your validator data and enabling disaster recovery.**

---

## Overview

**What to backup:**
1. **Validator consensus key** (`priv_validator_key.json`) - CRITICAL
2. **Validator state** (`priv_validator_state.json`) - Important
3. **Node configuration** (`config.toml`, `app.toml`, `client.toml`) - Important
4. **Wallet keys** (mnemonic phrase) - CRITICAL
5. **Node identity** (`node_key.json`) - Optional

**What NOT to backup:**
- Blockchain data (`data/` directory) - Can re-sync
- Application database - Can rebuild
- Logs - Not critical

---

## Table of Contents

1. [Quick Start: Essential Backups](#quick-start-essential-backups)
2. [Manual Backup Procedures](#manual-backup-procedures)
3. [Automated Backup Scripts](#automated-backup-scripts)
4. [Cloud Backup Solutions](#cloud-backup-solutions)
5. [Restore Procedures](#restore-procedures)
6. [Testing Backups](#testing-backups)
7. [Best Practices](#best-practices)

---

## Quick Start: Essential Backups

### One-Time Backup (Before Starting Validator)

```bash
#!/bin/bash
# Essential one-time backup

BACKUP_DIR="$HOME/omniphi-backup-$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Backup consensus private key (MOST IMPORTANT)
cp ~/.omniphi/config/priv_validator_key.json "$BACKUP_DIR/"

# Backup validator state
cp ~/.omniphi/data/priv_validator_state.json "$BACKUP_DIR/"

# Backup node key
cp ~/.omniphi/config/node_key.json "$BACKUP_DIR/"

# Backup configuration files
cp ~/.omniphi/config/config.toml "$BACKUP_DIR/"
cp ~/.omniphi/config/app.toml "$BACKUP_DIR/"
cp ~/.omniphi/config/client.toml "$BACKUP_DIR/"

# Create encrypted archive
tar -czf "$BACKUP_DIR.tar.gz" "$BACKUP_DIR"
gpg --symmetric --cipher-algo AES256 "$BACKUP_DIR.tar.gz"

# Securely delete unencrypted backup
shred -vfz -n 10 "$BACKUP_DIR.tar.gz"
rm -rf "$BACKUP_DIR"

echo "Encrypted backup created: $BACKUP_DIR.tar.gz.gpg"
echo "Store this file in multiple secure locations!"
```

**Run this immediately after initializing your validator:**
```bash
chmod +x backup-essential.sh
./backup-essential.sh
```

---

## Manual Backup Procedures

### 1. Backing Up Consensus Key

**CRITICAL: This is your validator's identity**

```bash
# Create encrypted backup
gpg --symmetric --cipher-algo AES256 \
  ~/.omniphi/config/priv_validator_key.json

# Output: priv_validator_key.json.gpg

# Store in multiple locations:
# - Encrypted USB drive
# - Password manager (1Password, Bitwarden)
# - Offline hardware wallet (if supported)
# - Secure cloud storage (with additional encryption)
```

**Verification:**
```bash
# Test decryption
gpg --decrypt priv_validator_key.json.gpg > test.json

# Compare with original
diff ~/.omniphi/config/priv_validator_key.json test.json

# Should show no differences
# Securely delete test file
shred -vfz -n 10 test.json
```

---

### 2. Backing Up Validator State

**Important: Prevents double-signing after restore**

```bash
# Backup state file
cp ~/.omniphi/data/priv_validator_state.json \
  ~/priv_validator_state_$(date +%Y%m%d-%H%M%S).json

# Encrypt
gpg --symmetric --cipher-algo AES256 \
  ~/priv_validator_state_$(date +%Y%m%d-%H%M%S).json
```

**⚠️ WARNING:** Always backup state WITH consensus key. Using old state can cause double-signing!

---

### 3. Backing Up Configuration Files

```bash
# Create config backup directory
CONF_BACKUP="$HOME/omniphi-config-$(date +%Y%m%d)"
mkdir -p "$CONF_BACKUP"

# Copy all config files
cp ~/.omniphi/config/config.toml "$CONF_BACKUP/"
cp ~/.omniphi/config/app.toml "$CONF_BACKUP/"
cp ~/.omniphi/config/client.toml "$CONF_BACKUP/"
cp ~/.omniphi/config/genesis.json "$CONF_BACKUP/"

# Create archive
tar -czf "$CONF_BACKUP.tar.gz" "$CONF_BACKUP"

# Optional: Encrypt (contains no sensitive keys, but good practice)
gpg --symmetric --cipher-algo AES256 "$CONF_BACKUP.tar.gz"

# Clean up
rm -rf "$CONF_BACKUP" "$CONF_BACKUP.tar.gz"
```

---

### 4. Backing Up Wallet Keys (Operator Key)

**MOST CRITICAL: This controls your validator and funds**

**If using mnemonic phrase:**
```bash
# DO NOT store mnemonic digitally!
# Write on paper or metal plate
# Store in:
# - Fireproof safe
# - Safe deposit box
# - Split using Shamir's Secret Sharing across multiple locations
```

**If using keyring file:**
```bash
# Backup keyring directory (OS-dependent)

# Linux (file backend):
tar -czf keyring-backup-$(date +%Y%m%d).tar.gz ~/.omniphi/keyring-file/
gpg --symmetric --cipher-algo AES256 keyring-backup-$(date +%Y%m%d).tar.gz

# macOS (Keychain):
# Export from Keychain Access app manually

# Windows (Credential Manager):
# Use Windows Credential Manager export feature
```

---

## Automated Backup Scripts

### Daily Automated Backup Script

```bash
#!/bin/bash
# /home/omniphi/scripts/daily-backup.sh

set -e

# Configuration
BACKUP_DIR="/mnt/backups/omniphi"
RETENTION_DAYS=30
BACKUP_NAME="omniphi-backup-$(date +%Y%m%d-%H%M%S)"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_NAME"
GPG_PASSPHRASE_FILE="/home/omniphi/.backup-passphrase"  # Secure this file!
ALERT_EMAIL="your-email@example.com"

# Create backup directory
mkdir -p "$BACKUP_PATH"

echo "Starting backup: $BACKUP_NAME"

# Backup consensus key and state
cp ~/.omniphi/config/priv_validator_key.json "$BACKUP_PATH/" || {
  echo "ERROR: Failed to backup consensus key" | mail -s "Backup Failed" $ALERT_EMAIL
  exit 1
}

cp ~/.omniphi/data/priv_validator_state.json "$BACKUP_PATH/" || {
  echo "WARNING: Failed to backup validator state"
}

# Backup node key
cp ~/.omniphi/config/node_key.json "$BACKUP_PATH/" 2>/dev/null || true

# Backup configuration
cp ~/.omniphi/config/config.toml "$BACKUP_PATH/"
cp ~/.omniphi/config/app.toml "$BACKUP_PATH/"
cp ~/.omniphi/config/client.toml "$BACKUP_PATH/"
cp ~/.omniphi/config/genesis.json "$BACKUP_PATH/" 2>/dev/null || true

# Create tarball
cd "$BACKUP_DIR"
tar -czf "$BACKUP_NAME.tar.gz" "$BACKUP_NAME"

# Encrypt with GPG
cat "$GPG_PASSPHRASE_FILE" | gpg --batch --yes --passphrase-fd 0 \
  --symmetric --cipher-algo AES256 \
  "$BACKUP_NAME.tar.gz"

# Remove unencrypted files
rm -rf "$BACKUP_NAME" "$BACKUP_NAME.tar.gz"

# Verify encrypted backup exists
if [ -f "$BACKUP_NAME.tar.gz.gpg" ]; then
  BACKUP_SIZE=$(du -h "$BACKUP_NAME.tar.gz.gpg" | cut -f1)
  echo "Backup successful: $BACKUP_NAME.tar.gz.gpg ($BACKUP_SIZE)"
else
  echo "ERROR: Backup file not found!" | mail -s "Backup Failed" $ALERT_EMAIL
  exit 1
fi

# Clean up old backups (keep last 30 days)
find "$BACKUP_DIR" -name "omniphi-backup-*.tar.gz.gpg" -mtime +$RETENTION_DAYS -delete
echo "Old backups cleaned (retention: $RETENTION_DAYS days)"

# Upload to cloud (optional)
# Uncomment if using S3, Backblaze, etc.
# aws s3 cp "$BACKUP_NAME.tar.gz.gpg" s3://my-validator-backups/

echo "Backup complete: $(date)"
```

**Setup:**

```bash
# Create scripts directory
mkdir -p ~/scripts

# Create backup script
nano ~/scripts/daily-backup.sh
# (paste script above)

chmod +x ~/scripts/daily-backup.sh

# Create backup directory
sudo mkdir -p /mnt/backups/omniphi
sudo chown omniphi:omniphi /mnt/backups/omniphi

# Create GPG passphrase file (SECURE THIS!)
echo "YOUR_STRONG_PASSPHRASE_HERE" > ~/.backup-passphrase
chmod 600 ~/.backup-passphrase

# Test backup
~/scripts/daily-backup.sh

# Schedule daily backups (cron)
crontab -e
# Add:
0 2 * * * /home/omniphi/scripts/daily-backup.sh >> /var/log/omniphi-backup.log 2>&1
```

---

### Weekly Full Backup (Including Blockchain Data)

**Use only if you need blockchain data backup:**

```bash
#!/bin/bash
# /home/omniphi/scripts/weekly-full-backup.sh

set -e

BACKUP_DIR="/mnt/large-storage/omniphi-full-backups"
BACKUP_NAME="omniphi-full-$(date +%Y%m%d)"
RETENTION_WEEKS=4

echo "Starting full backup (this may take hours)..."

# Stop validator temporarily
sudo systemctl stop posd

# Create full backup
mkdir -p "$BACKUP_DIR"
tar -czf "$BACKUP_DIR/$BACKUP_NAME.tar.gz" \
  --exclude="*.sock" \
  ~/.omniphi

# Start validator
sudo systemctl start posd

# Encrypt
gpg --symmetric --cipher-algo AES256 "$BACKUP_DIR/$BACKUP_NAME.tar.gz"
rm "$BACKUP_DIR/$BACKUP_NAME.tar.gz"

# Clean old backups
find "$BACKUP_DIR" -name "omniphi-full-*.tar.gz.gpg" -mtime +$((RETENTION_WEEKS * 7)) -delete

echo "Full backup complete: $BACKUP_NAME.tar.gz.gpg"
```

**⚠️ WARNING:** This causes validator downtime. Only use for archive purposes.

---

## Cloud Backup Solutions

### AWS S3 Backup

```bash
#!/bin/bash
# Upload backups to S3

BACKUP_FILE="/mnt/backups/omniphi/omniphi-backup-$(date +%Y%m%d-*).tar.gz.gpg"
S3_BUCKET="s3://my-validator-backups/omniphi/"

# Upload to S3
aws s3 cp $BACKUP_FILE $S3_BUCKET

# Verify upload
aws s3 ls $S3_BUCKET | grep "$(date +%Y%m%d)"
```

**Setup S3:**
```bash
# Install AWS CLI
sudo apt-get install -y awscli

# Configure credentials
aws configure
# Enter:
# - AWS Access Key ID
# - AWS Secret Access Key
# - Default region: us-east-1
# - Default output format: json

# Create S3 bucket
aws s3 mb s3://my-validator-backups

# Enable versioning (keep old versions)
aws s3api put-bucket-versioning \
  --bucket my-validator-backups \
  --versioning-configuration Status=Enabled

# Enable encryption
aws s3api put-bucket-encryption \
  --bucket my-validator-backups \
  --server-side-encryption-configuration '{
    "Rules": [{
      "ApplyServerSideEncryptionByDefault": {
        "SSEAlgorithm": "AES256"
      }
    }]
  }'
```

---

### Backblaze B2 Backup (Cost-Effective)

```bash
#!/bin/bash
# Upload to Backblaze B2

BACKUP_FILE="/mnt/backups/omniphi/omniphi-backup-$(date +%Y%m%d-*).tar.gz.gpg"

# Install B2 CLI
pip3 install --upgrade b2

# Authorize
b2 authorize-account YOUR_KEY_ID YOUR_APPLICATION_KEY

# Upload
b2 upload-file my-backup-bucket $BACKUP_FILE "omniphi/$(basename $BACKUP_FILE)"
```

---

### Automated Cloud Backup (Restic)

**Restic: Modern backup tool with deduplication and encryption**

```bash
# Install Restic
sudo apt-get install -y restic

# Initialize repository (S3 example)
export AWS_ACCESS_KEY_ID="YOUR_KEY"
export AWS_SECRET_ACCESS_KEY="YOUR_SECRET"
export RESTIC_REPOSITORY="s3:s3.amazonaws.com/my-validator-backups"
export RESTIC_PASSWORD="YOUR_STRONG_PASSWORD"

restic init

# Create backup
restic backup ~/.omniphi/config ~/.omniphi/data/priv_validator_state.json

# List snapshots
restic snapshots

# Restore latest snapshot
restic restore latest --target /tmp/restore
```

**Automated restic backup:**

```bash
#!/bin/bash
# /home/omniphi/scripts/restic-backup.sh

export RESTIC_REPOSITORY="s3:s3.amazonaws.com/my-validator-backups"
export RESTIC_PASSWORD_FILE="/home/omniphi/.restic-password"

restic backup \
  ~/.omniphi/config/priv_validator_key.json \
  ~/.omniphi/config/node_key.json \
  ~/.omniphi/data/priv_validator_state.json \
  ~/.omniphi/config/config.toml \
  ~/.omniphi/config/app.toml \
  ~/.omniphi/config/client.toml

# Clean old snapshots (keep last 30 daily, 12 monthly)
restic forget --keep-daily 30 --keep-monthly 12 --prune
```

---

## Restore Procedures

### Scenario 1: Restore Consensus Key (New Server)

```bash
# Copy encrypted backup to new server
scp omniphi-backup-20251120.tar.gz.gpg new-server:~/

# On new server:
# Decrypt
gpg --decrypt omniphi-backup-20251120.tar.gz.gpg > omniphi-backup-20251120.tar.gz

# Extract
tar -xzf omniphi-backup-20251120.tar.gz

# Initialize new node (if not already done)
posd init my-validator --chain-id omniphi-1

# Restore consensus key
cp omniphi-backup-20251120/priv_validator_key.json ~/.omniphi/config/
chmod 600 ~/.omniphi/config/priv_validator_key.json

# Restore validator state
cp omniphi-backup-20251120/priv_validator_state.json ~/.omniphi/data/
chmod 600 ~/.omniphi/data/priv_validator_state.json

# Restore configuration (optional)
cp omniphi-backup-20251120/config.toml ~/.omniphi/config/
cp omniphi-backup-20251120/app.toml ~/.omniphi/config/

# Clean up
shred -vfz -n 10 omniphi-backup-20251120.tar.gz
rm -rf omniphi-backup-20251120

# Start validator
sudo systemctl start posd
```

---

### Scenario 2: Restore After Corruption

```bash
# Stop validator
sudo systemctl stop posd

# Backup corrupted state (for investigation)
mv ~/.omniphi/data ~/.omniphi/data.corrupted

# Restore from backup
tar -xzf omniphi-backup-latest.tar.gz
cp omniphi-backup-latest/priv_validator_state.json ~/.omniphi/data/

# Use state sync to re-download blockchain
# (See STATE_SYNC.md for details)

# Start validator
sudo systemctl start posd
```

---

### Scenario 3: Restore From Cloud (S3)

```bash
# List available backups
aws s3 ls s3://my-validator-backups/omniphi/

# Download latest backup
aws s3 cp s3://my-validator-backups/omniphi/omniphi-backup-20251120.tar.gz.gpg ~/

# Decrypt and restore (see Scenario 1)
```

---

### Scenario 4: Restore Using Restic

```bash
# List available snapshots
export RESTIC_REPOSITORY="s3:s3.amazonaws.com/my-validator-backups"
export RESTIC_PASSWORD_FILE="/home/omniphi/.restic-password"

restic snapshots

# Restore latest snapshot
restic restore latest --target /tmp/restore

# Copy files to validator directory
cp /tmp/restore/home/omniphi/.omniphi/config/priv_validator_key.json ~/.omniphi/config/
cp /tmp/restore/home/omniphi/.omniphi/data/priv_validator_state.json ~/.omniphi/data/

# Set permissions
chmod 600 ~/.omniphi/config/priv_validator_key.json
chmod 600 ~/.omniphi/data/priv_validator_state.json
```

---

## Testing Backups

**CRITICAL: Always test backups before you need them!**

### Monthly Backup Test

```bash
#!/bin/bash
# /home/omniphi/scripts/test-backup.sh

set -e

BACKUP_FILE="/mnt/backups/omniphi/omniphi-backup-latest.tar.gz.gpg"
TEST_DIR="/tmp/backup-test-$(date +%Y%m%d-%H%M%S)"

echo "Testing backup restore..."

# Decrypt
gpg --decrypt "$BACKUP_FILE" > "$TEST_DIR.tar.gz"

# Extract
tar -xzf "$TEST_DIR.tar.gz" -C /tmp/

# Verify critical files exist
[ -f "$TEST_DIR/priv_validator_key.json" ] || { echo "ERROR: Consensus key missing!"; exit 1; }
[ -f "$TEST_DIR/priv_validator_state.json" ] || { echo "WARNING: State file missing!"; }
[ -f "$TEST_DIR/config.toml" ] || { echo "ERROR: Config missing!"; exit 1; }

# Verify key format
cat "$TEST_DIR/priv_validator_key.json" | jq . > /dev/null || {
  echo "ERROR: Consensus key JSON invalid!"
  exit 1
}

# Clean up
shred -vfz -n 10 "$TEST_DIR.tar.gz"
rm -rf "$TEST_DIR"

echo "Backup test PASSED"
```

**Schedule monthly tests:**
```bash
crontab -e
# Add:
0 3 1 * * /home/omniphi/scripts/test-backup.sh >> /var/log/backup-test.log 2>&1
```

---

## Best Practices

### ✅ Backup Dos

- [ ] **Backup immediately** after initializing validator
- [ ] **Test restores monthly** to verify backups work
- [ ] **Encrypt all backups** with strong password
- [ ] **Store in multiple locations** (3-2-1 rule: 3 copies, 2 media types, 1 offsite)
- [ ] **Automate backups** (daily cron job)
- [ ] **Monitor backup jobs** (alert on failures)
- [ ] **Version control configs** (git repository)
- [ ] **Document restore procedures** (this guide + your own notes)

---

### ❌ Backup Don'ts

- [ ] **Don't store unencrypted backups** in cloud
- [ ] **Don't rely on single backup location**
- [ ] **Don't backup without testing restore**
- [ ] **Don't share backup passphrases** via insecure channels
- [ ] **Don't backup blockchain data** (unless necessary - huge size)
- [ ] **Don't forget to backup validator state** with consensus key
- [ ] **Don't use weak encryption** (AES256 minimum)

---

### 3-2-1 Backup Rule

**3 copies:**
1. Original data on validator
2. Encrypted backup on external drive
3. Encrypted backup in cloud (S3, B2)

**2 media types:**
1. SSD/HDD (local)
2. Cloud storage (remote)

**1 offsite:**
- Cloud backup in different geographic region

---

## Backup Checklist

### Daily
- [ ] Automated backup runs successfully
- [ ] Backup files created in expected location
- [ ] No error emails received

### Weekly
- [ ] Verify backup file sizes are reasonable
- [ ] Check backup retention (old files deleted)
- [ ] Review backup logs

### Monthly
- [ ] Test restore procedure
- [ ] Verify cloud backups accessible
- [ ] Update backup documentation
- [ ] Rotate backup encryption passwords (if policy requires)

### Quarterly
- [ ] Full disaster recovery test (restore on new server)
- [ ] Review and update backup strategy
- [ ] Audit backup access permissions

---

## Summary

**Essential backups for validators:**

1. **Consensus private key** (priv_validator_key.json) - CRITICAL
2. **Validator state** (priv_validator_state.json) - Important
3. **Wallet mnemonic** (write on paper/metal) - CRITICAL
4. **Configuration files** (config.toml, app.toml) - Important

**Backup strategy:**
- Daily automated backups (cron)
- Encrypted with GPG (AES256)
- Stored in 3 locations (local, external, cloud)
- Tested monthly

**Recovery time:**
- From backup: 10-30 minutes
- With state sync: 5-30 minutes additional

---

**For more operational guides:**
- [STATE_SYNC.md](STATE_SYNC.md) - Fast blockchain sync
- [MONITORING.md](MONITORING.md) - Monitoring and alerts
- [../security/KEY_MANAGEMENT.md](../security/KEY_MANAGEMENT.md) - Key security

---

**Need help?** Ask in Omniphi Discord: https://discord.gg/omniphi
