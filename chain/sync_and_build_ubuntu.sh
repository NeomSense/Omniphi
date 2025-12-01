#!/bin/bash
# Sync code to Ubuntu and rebuild with feemarket module fix

echo "========================================"
echo "Rebuilding posd with Fixed Fee Market"
echo "========================================"
echo ""

# 1. Go to project directory
cd ~/omniphi/pos || exit 1

# 2. Pull latest changes (if using git)
echo "📥 Pulling latest code..."
git pull origin main 2>/dev/null || echo "⚠️  Not a git repo or no remote, skipping pull..."

# 3. Clean old build
echo "🧹 Cleaning old build..."
rm -f posd
go clean -cache 2>/dev/null

# 4. Build new binary
echo "🔨 Building posd..."
go build -o posd ./cmd/posd

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"

    # 5. Install to system (optional)
    echo ""
    echo "Installing to /usr/local/bin..."
    sudo cp posd /usr/local/bin/posd 2>/dev/null || cp posd ~/go/bin/posd

    # 6. Verify
    echo ""
    echo "📋 Verifying feemarket module..."
    posd query --help | grep feemarket && echo "✅ Fee market module found!" || echo "❌ Fee market module NOT found"

    echo ""
    echo "========================================"
    echo "✅ Done! Now you can start the chain:"
    echo "   posd start --home ~/.pos --minimum-gas-prices=0.05uomni"
    echo "========================================"
else
    echo "❌ Build failed!"
    exit 1
fi
