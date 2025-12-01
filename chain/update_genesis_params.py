#!/usr/bin/env python3
"""Update genesis.json with production parameters without requiring jq"""

import json
import sys
import os
from pathlib import Path

def update_genesis_params(genesis_file):
    """Update genesis file with production parameters"""

    # Read genesis
    with open(genesis_file, 'r') as f:
        genesis = json.load(f)

    print("Updating genesis with production parameters...")

    # Update staking parameters
    print("  • Staking parameters...")
    genesis['app_state']['staking']['params']['unbonding_time'] = "1814400s"
    genesis['app_state']['staking']['params']['max_validators'] = 125
    genesis['app_state']['staking']['params']['min_commission_rate'] = "0.050000000000000000"

    # Update slashing parameters
    print("  • Slashing parameters...")
    genesis['app_state']['slashing']['params']['signed_blocks_window'] = "30000"
    genesis['app_state']['slashing']['params']['min_signed_per_window'] = "0.050000000000000000"
    genesis['app_state']['slashing']['params']['downtime_jail_duration'] = "600s"
    genesis['app_state']['slashing']['params']['slash_fraction_double_sign'] = "0.050000000000000000"
    genesis['app_state']['slashing']['params']['slash_fraction_downtime'] = "0.000100000000000000"

    # Update governance parameters
    print("  • Governance parameters...")
    genesis['app_state']['gov']['params']['min_deposit'][0]['amount'] = "10000000"
    genesis['app_state']['gov']['params']['voting_period'] = "432000s"
    genesis['app_state']['gov']['params']['quorum'] = "0.334000000000000000"
    genesis['app_state']['gov']['params']['threshold'] = "0.500000000000000000"
    genesis['app_state']['gov']['params']['veto_threshold'] = "0.334000000000000000"
    genesis['app_state']['gov']['params']['burn_vote_veto'] = True

    # Update mint parameters
    print("  • Mint parameters...")
    genesis['app_state']['mint']['params']['inflation_rate_change'] = "0.130000000000000000"
    genesis['app_state']['mint']['params']['inflation_max'] = "0.200000000000000000"
    genesis['app_state']['mint']['params']['inflation_min'] = "0.070000000000000000"
    genesis['app_state']['mint']['params']['goal_bonded'] = "0.670000000000000000"
    genesis['app_state']['mint']['params']['blocks_per_year'] = "5256000"

    # Update distribution parameters
    print("  • Distribution parameters...")
    genesis['app_state']['distribution']['params']['community_tax'] = "0.020000000000000000"
    genesis['app_state']['distribution']['params']['withdraw_addr_enabled'] = True

    # Write updated genesis
    with open(genesis_file, 'w') as f:
        json.dump(genesis, f, indent=2)

    print("✓ Genesis parameters updated successfully!")

    return True

if __name__ == "__main__":
    home = os.path.expanduser("~/.pos")
    genesis_file = os.path.join(home, "config", "genesis.json")

    if not os.path.exists(genesis_file):
        print(f"Error: Genesis file not found at {genesis_file}")
        sys.exit(1)

    # Backup original
    backup_file = genesis_file + ".backup"
    print(f"Creating backup at {backup_file}")
    with open(genesis_file, 'r') as src:
        with open(backup_file, 'w') as dst:
            dst.write(src.read())

    # Update parameters
    try:
        update_genesis_params(genesis_file)
        print("\n✓ All production parameters set!")
        print("\nProduction Parameters:")
        print("  • Staking: min_commission = 5%, unbonding = 21 days, max_validators = 125")
        print("  • Slashing: window = 30000, min_signed = 5%, double_sign_slash = 5%")
        print("  • Governance: deposit = 10M, voting = 5 days, quorum = 33.4%")
        print("  • Mint: inflation 7-20%, goal_bonded = 67%")
        print("  • Distribution: community_tax = 2%")
        print("\nReady to start chain with: posd start --home ~/.pos")
    except Exception as e:
        print(f"Error updating genesis: {e}")
        sys.exit(1)
