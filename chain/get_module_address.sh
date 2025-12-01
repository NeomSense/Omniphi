#!/bin/bash
# The bonded pool address is deterministically derived from the module name
# For Cosmos SDK, it's: sha256(hash of "bonded_tokens_pool")
# But we can get it by checking what posd expects
# Let's check the staking module constants
./posd query staking params --home /home/funmachine/.pos --output json 2>/dev/null | jq -r '.bond_denom' || echo "Chain not running"

# Alternative: Let's just use the standard Cosmos SDK derivation
# The module address for bonded_tokens_pool with omni prefix should be:
echo "Calculating bonded pool address for 'omni' prefix..."
# Standard Cosmos SDK bonded pool module name
python3 << 'PYTHON'
import hashlib
# Module name for bonded pool
module_name = "bonded_tokens_pool"
# Hash the module name
module_hash = hashlib.sha256(module_name.encode()).digest()[:20]
# Convert to hex
hex_addr = module_hash.hex()
print(f"Module hash (hex): {hex_addr}")
# For bech32 encoding, you'd need a library, but the hex gives us the raw address
# The actual bech32 address would be: omni1 + bech32_encode(module_hash)
PYTHON
