#!/usr/bin/env python3
import json
import sys
import os

home_dir = os.path.expanduser("~/.pos")
genesis_path = f"{home_dir}/config/genesis.json"

print(f"Reading genesis from: {genesis_path}")

try:
    with open(genesis_path, 'r') as f:
        genesis = json.load(f)

    print("Current PoC state:", genesis['app_state']['poc'])

    # Update PoC module genesis
    genesis['app_state']['poc'] = {
        "params": {
            "quorum_pct": "0.670000000000000000",
            "base_reward_unit": "1000",
            "inflation_share": "0.000000000000000000",
            "max_per_block": 10,
            "tiers": [
                {"name": "bronze", "cutoff": "1000"},
                {"name": "silver", "cutoff": "10000"},
                {"name": "gold", "cutoff": "100000"}
            ],
            "reward_denom": "omniphi"
        },
        "contributions": [],
        "credits": [],
        "next_contribution_id": "1"
    }

    # Write back
    with open(genesis_path, 'w') as f:
        json.dump(genesis, f, indent=2)

    print("✓ PoC genesis updated successfully!")
    print("New PoC state:", json.dumps(genesis['app_state']['poc'], indent=2))

except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)
