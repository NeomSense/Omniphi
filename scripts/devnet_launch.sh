#!/bin/bash
# Omniphi Devnet Launch Script
# Launches a local 4-node devnet for development and testing.
set -e

BINARY="${BINARY:-poseq-devnet}"
NODES="${NODES:-4}"
QUORUM="${QUORUM:-3}"
SLOTS="${SLOTS:-20}"
SLOT_MS="${SLOT_MS:-2000}"
SCENARIO="${SCENARIO:-happy_path}"

echo "=== Omniphi Devnet Launch ==="
echo "Nodes:    $NODES"
echo "Quorum:   $QUORUM"
echo "Slots:    $SLOTS"
echo "Slot ms:  $SLOT_MS"
echo "Scenario: $SCENARIO"
echo ""

# Build if needed
if ! command -v "$BINARY" &> /dev/null; then
    echo "Building poseq-devnet..."
    cd "$(dirname "$0")/../poseq"
    cargo build --release --bin poseq-devnet
    BINARY="./target/release/poseq-devnet"
    cd ..
fi

echo "Starting devnet..."
$BINARY \
    --nodes "$NODES" \
    --quorum "$QUORUM" \
    --slots "$SLOTS" \
    --slot-ms "$SLOT_MS" \
    --scenario "$SCENARIO"

echo "Devnet finished."
