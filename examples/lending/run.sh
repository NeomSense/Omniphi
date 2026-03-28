#!/usr/bin/env bash
set -euo pipefail

echo "=== Omniphi Intent-Based Lending Protocol Example ==="
echo ""
echo "Installing dependencies..."
npm install

echo ""
echo "Running lending protocol example..."
npx ts-node src/index.ts
