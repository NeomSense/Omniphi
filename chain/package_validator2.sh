#!/bin/bash
# Package Validator 2 for Transfer to Computer 2

OUTPUT_DIR="./testnet-2nodes"
PACKAGE_NAME="validator2-package.tar.gz"

if [ ! -d "$OUTPUT_DIR/validator2" ]; then
    echo "Error: Validator 2 directory not found!"
    echo "Please run setup_2node_testnet.sh first!"
    exit 1
fi

echo "======================================"
echo "  Packaging Validator 2"
echo "======================================"
echo ""

# Create package with validator2 directory and binary
tar -czf "$PACKAGE_NAME" \
    -C "$OUTPUT_DIR" validator2 \
    --transform 's,^,validator2-package/,'

# Copy binary if it exists locally
if [ -f "./posd" ]; then
    echo "Adding posd binary to package..."
    tar -czf "$PACKAGE_NAME.tmp" \
        -C "$OUTPUT_DIR" validator2 \
        --transform 's,validator2,validator2-package/validator2,' \
        -C .. posd \
        --transform 's,posd,validator2-package/posd,'
    mv "$PACKAGE_NAME.tmp" "$PACKAGE_NAME"
fi

# Add start script
if [ -f "./start_validator2.sh" ]; then
    tar -rzf "$PACKAGE_NAME" \
        start_validator2.sh \
        --transform 's,^,validator2-package/,'
fi

PACKAGE_SIZE=$(du -h "$PACKAGE_NAME" | cut -f1)

echo ""
echo "✓ Package created: $PACKAGE_NAME ($PACKAGE_SIZE)"
echo ""
echo "======================================"
echo "  Transfer Instructions"
echo "======================================"
echo ""
echo "1. Transfer $PACKAGE_NAME to Computer 2 using:"
echo "   - USB drive"
echo "   - scp validator2-package.tar.gz user@computer2:/path/"
echo "   - Network share"
echo ""
echo "2. On Computer 2, extract the package:"
echo "   tar -xzf validator2-package.tar.gz"
echo "   cd validator2-package"
echo ""
echo "3. If posd binary is not included, build it on Computer 2:"
echo "   cd /path/to/pos"
echo "   go build -o posd ./cmd/posd"
echo "   cp posd /path/to/validator2-package/"
echo ""
echo "4. Start validator 2:"
echo "   ./start_validator2.sh"
echo ""
echo "5. Verify it's running:"
echo "   ./posd status --home ./validator2"
echo ""
