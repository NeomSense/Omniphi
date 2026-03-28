#!/usr/bin/env bash
set -euo pipefail

echo "=== Omniphi Intent-Based DEX Example ==="
echo ""
echo "Installing dependencies..."
npm install

echo ""
echo "Running DEX example..."
npx ts-node src/index.ts
