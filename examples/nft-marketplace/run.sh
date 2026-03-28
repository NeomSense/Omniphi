#!/usr/bin/env bash
set -euo pipefail

echo "=== Omniphi Programmable Ownership Marketplace Example ==="
echo ""
echo "Installing dependencies..."
npm install

echo ""
echo "Running marketplace example..."
npx ts-node src/index.ts
