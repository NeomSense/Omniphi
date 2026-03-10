#!/bin/bash
# MAINNET REQUIRED: Set PoR security parameters in genesis.json
#
# This script enables three security-critical PoR parameters that MUST be
# active for mainnet. Run this BEFORE starting the chain for the first time.
#
# Parameters enabled:
#   require_leaf_hashes=true      — conclusive double-inclusion fraud proofs
#   require_da_commitment=true    — data availability commitment enforcement
#   require_poseq_commitment=true — PoSeq sequencer commitment enforcement
#
# Without these, fraud verification is best-effort and inconclusive.
#
# Usage:
#   ./set_mainnet_por_params.sh [genesis_path]
#   Default genesis_path: ~/.pos/config/genesis.json

set -e

GENESIS="${1:-$HOME/.pos/config/genesis.json}"

if [ ! -f "$GENESIS" ]; then
    echo "ERROR: Genesis file not found at $GENESIS"
    echo "Usage: $0 [path/to/genesis.json]"
    exit 1
fi

echo "=== PoR Mainnet Security Parameters ==="
echo "Genesis: $GENESIS"
echo ""

# Backup genesis
BACKUP="${GENESIS}.backup.$(date +%s)"
cp "$GENESIS" "$BACKUP"
echo "Backup: $BACKUP"

# Show current values
echo ""
echo "Current PoR params:"
jq '.app_state.por.params | {
  require_leaf_hashes,
  require_da_commitment,
  require_poseq_commitment
}' "$GENESIS" 2>/dev/null || echo "  (no existing PoR params found, will be set)"

# Update PoR security params
echo ""
echo "Setting mainnet security params..."
cat "$GENESIS" | jq '
  .app_state.por.params.require_leaf_hashes = true |
  .app_state.por.params.require_da_commitment = true |
  .app_state.por.params.require_poseq_commitment = true
' > "${GENESIS}.new"

# Validate JSON
if jq empty "${GENESIS}.new" 2>/dev/null; then
    mv "${GENESIS}.new" "$GENESIS"
    echo ""
    echo "Updated PoR params:"
    jq '.app_state.por.params | {
      require_leaf_hashes,
      require_da_commitment,
      require_poseq_commitment,
      max_credits_per_epoch,
      max_credits_per_batch,
      challenge_bond_amount,
      max_challenges_per_address
    }' "$GENESIS"
    echo ""
    echo "SUCCESS: PoR mainnet security params set."
else
    echo "ERROR: Invalid JSON produced. Restoring backup..."
    cp "$BACKUP" "$GENESIS"
    rm -f "${GENESIS}.new"
    exit 1
fi
