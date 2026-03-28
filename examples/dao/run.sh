#!/usr/bin/env bash
set -euo pipefail

echo "=== Omniphi DAO with Reputation-Weighted Governance Example ==="
echo ""
echo "Installing dependencies..."
npm install

echo ""
echo "Running DAO example..."
npx ts-node src/index.ts
