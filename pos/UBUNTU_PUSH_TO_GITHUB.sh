#!/bin/bash
# Run this script on Ubuntu to push your code to GitHub

set -e  # Exit on error

echo "======================================"
echo "Pushing Ubuntu Code to GitHub"
echo "======================================"
echo ""

cd ~/omniphi/pos

# Check if we're in a git repo
if [ ! -d ".git" ]; then
    echo "ERROR: Not a git repository. Initializing..."
    git init
    git remote add origin https://github.com/NeomSense/PoS-PoC.git
fi

# Create .gitignore if it doesn't exist
if [ ! -f ".gitignore" ]; then
    echo "Creating .gitignore..."
    cat > .gitignore << 'EOF'
# Binaries
posd
posd.exe
*.test
*.exe
*.out

# Chain data directories
.posd/
.pos/
config/

# Go build artifacts
build/
*.log
*.pid

# Test results
*test-results*.log
*test_output*.log
quick_test_results.txt

# Backup files
*.backup
*.bak

# Go downloads
go*.linux-amd64.tar.gz
go*.windows-amd64.zip

# Temporary files
nul
debug_*.log
debug_*.dot
download.log
tidy*.log
build*.log
go-mod-tidy.log
posd.log

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Scripts (optional - comment out if you want to include scripts)
*.sh
*.ps1
*.bat
EOF
fi

echo "Checking git status..."
git status

echo ""
echo "Adding all source files to git..."
git add .gitignore
git add proto/
git add x/
git add cmd/
git add app/
git add go.mod go.sum
git add buf.yaml buf.gen.yaml buf.lock
git add Makefile
git add *.md 2>/dev/null || true

echo ""
echo "Files to be committed:"
git status --short

echo ""
read -p "Do you want to continue with the commit? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 1
fi

echo ""
echo "Creating commit..."
git commit -m "Add complete Omniphi blockchain source code

- POC (Proof of Contribution) module with fee burning
- Tokenomics module with inflation and vesting
- Fee Market v2 module (incomplete proto)
- All proto definitions and generated files
- Complete keeper implementations and tests
- App configuration and command definitions

This is the working Ubuntu codebase ready for deployment."

echo ""
echo "Pushing to GitHub..."
git push -u origin main || git push -u origin master

echo ""
echo "======================================"
echo "SUCCESS! Code pushed to GitHub"
echo "======================================"
echo ""
echo "Next steps on Windows:"
echo "1. cd c:\\Users\\herna\\omniphi\\pos"
echo "2. git pull origin main"
echo "3. go build -o posd.exe ./cmd/posd"
echo ""
