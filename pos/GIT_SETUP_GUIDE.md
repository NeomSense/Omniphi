# Git Repository Setup Guide

## Why You Need Git for Testnet/Mainnet

Without version control:
- ❌ **File sync issues** - Like the proto file panic you just experienced
- ❌ **No deployment history** - Can't rollback bad releases
- ❌ **No audit trail** - Can't prove what code is running
- ❌ **Coordination problems** - Multiple validators can't sync code
- ❌ **Production risk** - Manual file transfers are error-prone

With git:
- ✅ **Perfect code sync** - Windows, Ubuntu, testnet, mainnet all identical
- ✅ **Version history** - Track every change
- ✅ **Rollback capability** - Revert to any previous state
- ✅ **Professional deployment** - Industry standard for blockchain projects
- ✅ **Audit compliance** - Required for mainnet security audits

---

## Phase 1: Initialize Local Repository (Windows)

### Step 1: Verify Your Code is Complete

```powershell
# On Windows
cd C:\Users\herna\omniphi\pos

# Check that the fixed proto file exists
Get-Content proto\pos\feemarket\module\v1\module.pb.go | Select-String "ProtoReflect"
# Should show the ProtoReflect() method

# Verify project compiles
go build -o posd.exe .\cmd\posd
# Should build successfully
```

### Step 2: Initialize Git

```powershell
# Initialize repository
git init

# Verify .gitignore exists
cat .gitignore
# Should show comprehensive ignore rules

# Check what files will be committed
git status

# You should see:
# - All *.go source files (green/to be added)
# - All *.proto files (green/to be added)
# - All *.md documentation (green/to be added)
# - go.mod and go.sum (green/to be added)
#
# You should NOT see:
# - .pos/ or .posd/ directories (ignored)
# - *.exe files (ignored)
# - *.log files (ignored)
# - build/ directories (ignored)
```

### Step 3: Stage All Source Files

```powershell
# Add all files (respecting .gitignore)
git add .

# Review what's being added
git status

# Check file count
git status --short | wc -l
# Should show ~200-300 files (source code, protos, docs)
```

### Step 4: Create Initial Commit

```powershell
# Set your identity (if first time using git)
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# Create initial commit
git commit -m "Initial commit: Omniphi POS blockchain with Fee Market v2

Includes:
- POC (Proof of Contribution) module
- Tokenomics module with decaying inflation and adaptive burn
- Fee Market v2 with 3-tier adaptive burn (10%/20%/40%)
- All module implementations, tests, and documentation
- Fixed proto stub for feemarket module (ProtoReflect method)

Chain ID: omniphi-testnet-1
Cosmos SDK: v0.53.3
"

# Verify commit
git log --oneline
# Should show: Initial commit: Omniphi POS blockchain...
```

### Step 5: Verify Commit Contents

```powershell
# Check critical files are in the commit
git ls-files | Select-String "feemarket"

# Should include:
# - proto/pos/feemarket/module/v1/module.pb.go  (CRITICAL!)
# - x/feemarket/keeper/keeper.go
# - x/feemarket/types/*.go
# - All other feemarket module files

# Check stats
git show --stat
# Should show all files committed
```

---

## Phase 2: Create Remote Repository

### Option A: GitHub (Recommended)

#### Step 1: Create Repository on GitHub

1. Go to https://github.com/new
2. Repository name: `omniphi-pos` (or your preferred name)
3. Description: "Omniphi Proof of Stake blockchain with adaptive tokenomics"
4. **Visibility**:
   - **Private** (recommended during development)
   - Public (only if you want open source)
5. **Do NOT** initialize with README, .gitignore, or license (you already have these)
6. Click "Create repository"

#### Step 2: Link Local Repo to GitHub

```powershell
# On Windows
cd C:\Users\herna\omniphi\pos

# Add remote (replace YOUR-USERNAME with your GitHub username)
git remote add origin https://github.com/YOUR-USERNAME/omniphi-pos.git

# Verify remote
git remote -v
# Should show:
# origin  https://github.com/YOUR-USERNAME/omniphi-pos.git (fetch)
# origin  https://github.com/YOUR-USERNAME/omniphi-pos.git (push)
```

#### Step 3: Push to GitHub

```powershell
# Create main branch and push
git branch -M main
git push -u origin main

# You may be prompted for GitHub credentials
# Use a Personal Access Token (PAT) instead of password
# Create PAT at: https://github.com/settings/tokens
```

#### Step 4: Verify on GitHub

Go to your repository URL: `https://github.com/YOUR-USERNAME/omniphi-pos`

You should see:
- ✅ All source code files
- ✅ README.md displayed
- ✅ proto/ directory with feemarket module
- ✅ x/ directory with all modules
- ✅ Documentation files

---

### Option B: GitLab

Similar steps, but use GitLab:

1. Create repository at https://gitlab.com/projects/new
2. Choose "Create blank project"
3. Name: `omniphi-pos`
4. Visibility: Private
5. Create project

Then:
```powershell
git remote add origin https://gitlab.com/YOUR-USERNAME/omniphi-pos.git
git branch -M main
git push -u origin main
```

---

### Option C: Self-Hosted Git (Advanced)

If you have your own git server:

```bash
# On your git server
git init --bare /path/to/omniphi-pos.git

# On Windows
git remote add origin user@your-server:/path/to/omniphi-pos.git
git push -u origin main
```

---

## Phase 3: Clone to Ubuntu

Now that your code is in a git repository, you can easily sync it to Ubuntu.

### Step 1: Backup Existing Ubuntu Code (Optional)

```bash
# On Ubuntu
cd ~
mv omniphi/pos omniphi/pos.backup.$(date +%Y%m%d)
```

### Step 2: Clone Repository

```bash
# Create directory
mkdir -p ~/omniphi
cd ~/omniphi

# Clone from GitHub (replace YOUR-USERNAME)
git clone https://github.com/YOUR-USERNAME/omniphi-pos.git pos

# Or from GitLab
git clone https://gitlab.com/YOUR-USERNAME/omniphi-pos.git pos

# Enter directory
cd pos
```

### Step 3: Verify Files

```bash
# Check proto file is correct
grep "ProtoReflect" proto/pos/feemarket/module/v1/module.pb.go
# Should show the method exists

# Check all modules present
ls -la x/
# Should show: feemarket/ poc/ tokenomics/

# Verify go.mod
cat go.mod | head -5
# Should show module pos and go version
```

### Step 4: Build on Ubuntu

```bash
# Build binary
go build -o posd ./cmd/posd

# Install to system
sudo cp posd /usr/local/bin/posd

# Verify feemarket module
posd query --help | grep feemarket
# Should show: feemarket    Querying commands for the feemarket module
```

### Step 5: Verify Code Integrity

```bash
# Check commit hash matches Windows
git log --oneline -1
# Should show same commit hash as Windows

# Check no uncommitted changes
git status
# Should show: nothing to commit, working tree clean

# Verify file count
git ls-files | wc -l
# Should match Windows file count
```

---

## Phase 4: Development Workflow

### Making Changes

```bash
# On Windows OR Ubuntu (wherever you're working)

# 1. Create a feature branch (optional but recommended)
git checkout -b feature/add-new-feature

# 2. Make your changes
# ... edit files ...

# 3. Check what changed
git status
git diff

# 4. Stage and commit
git add .
git commit -m "Add new feature: description of changes"

# 5. Push to remote
git push origin feature/add-new-feature

# 6. Merge to main (after testing)
git checkout main
git merge feature/add-new-feature
git push origin main
```

### Syncing Between Machines

```bash
# On the machine that needs updates

# Pull latest changes
git pull origin main

# Rebuild binary
go build -o posd ./cmd/posd
sudo cp posd /usr/local/bin/posd

# Verify
posd version
```

### Emergency Rollback

```bash
# If new code breaks, rollback

# See recent commits
git log --oneline

# Rollback to previous commit
git reset --hard <previous-commit-hash>

# Force push (ONLY if you haven't shared the broken commit)
git push -f origin main

# Rebuild
go build -o posd ./cmd/posd
```

---

## Phase 5: Testnet Deployment Workflow

### On Testnet Server:

```bash
# Initial setup
cd /opt/blockchain  # or your preferred location
git clone https://github.com/YOUR-USERNAME/omniphi-pos.git omniphi
cd omniphi

# Build
go build -o posd ./cmd/posd
sudo cp posd /usr/local/bin/posd

# Initialize chain
posd init testnet-node-1 --chain-id omniphi-testnet-1 --home ~/.pos

# Configure from production template
cp TESTNET_GENESIS_TEMPLATE.json ~/.pos/config/genesis.json

# ... complete setup following TESTNET_DEPLOYMENT_GUIDE.md ...

# Start
posd start --home ~/.pos
```

### Deploying Updates to Testnet:

```bash
# On testnet server

# Stop chain
sudo systemctl stop posd
# or: pkill posd

# Pull latest code
cd /opt/blockchain/omniphi
git pull origin main

# Rebuild
go build -o posd ./cmd/posd
sudo cp posd /usr/local/bin/posd

# Restart
sudo systemctl start posd
# or: posd start --home ~/.pos
```

---

## Phase 6: Mainnet Deployment Workflow

### Security Considerations for Mainnet:

1. **Use Release Tags**:
```bash
# On Windows (before mainnet launch)
git tag -a v1.0.0 -m "Mainnet Genesis Release"
git push origin v1.0.0

# On mainnet servers
git clone --branch v1.0.0 https://github.com/YOUR-USERNAME/omniphi-pos.git
```

2. **Verify Checksums**:
```bash
# After building on mainnet server
sha256sum posd > posd.sha256

# All validators should verify same checksum
# This ensures everyone runs identical code
```

3. **Read-Only Access**:
```bash
# Mainnet servers should NOT push code
# Only pull from verified releases
git config --global push.default nothing  # Prevent accidental pushes
```

---

## Best Practices

### Commit Messages

Good:
```
Fix: Resolve feemarket proto panic by adding ProtoReflect method

The module.pb.go stub was missing the ProtoReflect() method required
by depinject framework. Added proper protoimpl implementation.

Fixes #42
```

Bad:
```
fixed stuff
```

### Branching Strategy

- `main` - Production-ready code
- `develop` - Active development (optional)
- `feature/*` - New features
- `fix/*` - Bug fixes
- `release/*` - Release preparation

### Never Commit

❌ Private keys
❌ Mnemonics
❌ .pos/ data directories
❌ Compiled binaries (except releases)
❌ Log files
❌ Test results

### Always Commit

✅ Source code (*.go, *.proto)
✅ Documentation (*.md)
✅ Configuration templates
✅ Build scripts
✅ Test files
✅ go.mod and go.sum

---

## Troubleshooting

### "Permission denied (publickey)"

```bash
# Use HTTPS instead of SSH
git remote set-url origin https://github.com/YOUR-USERNAME/omniphi-pos.git

# Or setup SSH keys: https://docs.github.com/en/authentication/connecting-to-github-with-ssh
```

### "Git is not recognized" (Windows)

```powershell
# Install git from: https://git-scm.com/download/win
# Or use Git Bash that comes with it
```

### "Large files rejected"

```bash
# GitHub has 100MB file size limit
# Check for large files
find . -type f -size +50M

# Add them to .gitignore if they shouldn't be committed
```

### "Merge conflicts"

```bash
# If you edited same file on both machines

# Pull with merge
git pull origin main

# Git will show conflicts in files
# Edit files to resolve conflicts
# Look for:
# <<<<<<< HEAD
# your changes
# =======
# remote changes
# >>>>>>>

# After resolving
git add .
git commit -m "Merge: resolve conflicts"
git push origin main
```

---

## Summary Checklist

- [ ] Local repository initialized on Windows
- [ ] .gitignore configured properly
- [ ] Initial commit created with all source code
- [ ] Remote repository created (GitHub/GitLab)
- [ ] Code pushed to remote
- [ ] Repository cloned to Ubuntu
- [ ] Ubuntu binary built successfully from cloned code
- [ ] Feemarket module works on Ubuntu (no proto panic)
- [ ] Development workflow documented
- [ ] Testnet deployment plan ready
- [ ] Mainnet security considerations understood

---

## Next Steps

1. **Complete Phase 1-3** to sync your Windows code to Ubuntu via git
2. **Test on Ubuntu** - Make sure chain starts without proto panic
3. **Review DEPLOYMENT_WORKFLOW.md** for production deployment procedures
4. **Tag v1.0.0** when ready for mainnet

Your blockchain code is now version-controlled and ready for professional deployment!
