#!/usr/bin/env bash
set -euo pipefail

echo "=== Omniphi Capability-Based Escrow Example ==="
echo ""
echo "Installing dependencies..."
npm install

echo ""
echo "Running escrow example..."
npx ts-node src/index.ts
