#!/usr/bin/env python3
"""
Omniphi Anchor Lane Genesis Configuration Updater

This script updates genesis.json with the new anchor lane configuration:
- Max block gas: 60,000,000
- Max block bytes: 10MB
- Feemarket params for anchor lane

Usage:
    python3 update_genesis_anchor_lane.py /path/to/genesis.json

WARNING: This should only be used for:
1. New chain initialization
2. Coordinated chain restart with all validators
3. Testnet resets
"""

import json
import sys
import os
import shutil
from datetime import datetime

# Anchor Lane Configuration Constants
ANCHOR_LANE_CONFIG = {
    "consensus": {
        "block": {
            "max_bytes": "10485760",    # 10 MB
            "max_gas": "60000000"       # 60M gas per block
        }
    },
    "feemarket": {
        "params": {
            "min_gas_price": "0.025000000000000000",
            "base_fee_enabled": True,
            "base_fee_initial": "0.025000000000000000",
            "elasticity_multiplier": "1.125000000000000000",
            "max_tip_ratio": "0.200000000000000000",
            "target_block_utilization": "0.330000000000000000",
            "max_tx_gas": "2000000",     # 2M gas per tx (3.3% of block - no single tx dominance)
            "free_tx_quota": "100",
            "burn_cool": "0.100000000000000000",
            "burn_normal": "0.200000000000000000",
            "burn_hot": "0.400000000000000000",
            "util_cool_threshold": "0.160000000000000000",
            "util_hot_threshold": "0.330000000000000000",
            "validator_fee_ratio": "0.700000000000000000",
            "treasury_fee_ratio": "0.300000000000000000",
            "max_burn_ratio": "0.500000000000000000",
            "min_gas_price_floor": "0.025000000000000000",
            "multiplier_messaging": "0.500000000000000000",
            "multiplier_pos_gas": "1.000000000000000000",
            "multiplier_poc_anchoring": "0.750000000000000000",
            "multiplier_smart_contracts": "1.500000000000000000",
            "multiplier_ai_queries": "1.250000000000000000",
            "multiplier_sequencer": "1.250000000000000000",
            "min_multiplier": "0.250000000000000000",
            "max_multiplier": "2.000000000000000000"
        },
        "base_fee": "0.025000000000000000",
        "current_utilization": "0.000000000000000000",
        "previous_utilization": "0.000000000000000000"
    }
}


def update_nested_dict(base: dict, updates: dict) -> dict:
    """Recursively update nested dictionary."""
    for key, value in updates.items():
        if key in base and isinstance(base[key], dict) and isinstance(value, dict):
            update_nested_dict(base[key], value)
        else:
            base[key] = value
    return base


def update_genesis(genesis_path: str) -> None:
    """Update genesis.json with anchor lane configuration."""

    # Verify file exists
    if not os.path.exists(genesis_path):
        print(f"ERROR: Genesis file not found: {genesis_path}")
        sys.exit(1)

    # Create backup
    backup_path = f"{genesis_path}.backup_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    shutil.copy2(genesis_path, backup_path)
    print(f"Backup created: {backup_path}")

    # Load genesis
    with open(genesis_path, 'r') as f:
        genesis = json.load(f)

    # Update consensus params
    if "consensus" not in genesis:
        genesis["consensus"] = {"params": {"block": {}}}
    if "params" not in genesis["consensus"]:
        genesis["consensus"]["params"] = {"block": {}}
    if "block" not in genesis["consensus"]["params"]:
        genesis["consensus"]["params"]["block"] = {}

    genesis["consensus"]["params"]["block"]["max_bytes"] = ANCHOR_LANE_CONFIG["consensus"]["block"]["max_bytes"]
    genesis["consensus"]["params"]["block"]["max_gas"] = ANCHOR_LANE_CONFIG["consensus"]["block"]["max_gas"]

    print(f"Updated consensus.params.block.max_bytes: {ANCHOR_LANE_CONFIG['consensus']['block']['max_bytes']}")
    print(f"Updated consensus.params.block.max_gas: {ANCHOR_LANE_CONFIG['consensus']['block']['max_gas']}")

    # Update feemarket params in app_state
    if "app_state" not in genesis:
        genesis["app_state"] = {}
    if "feemarket" not in genesis["app_state"]:
        genesis["app_state"]["feemarket"] = {}

    # Update feemarket params
    genesis["app_state"]["feemarket"]["params"] = ANCHOR_LANE_CONFIG["feemarket"]["params"]
    genesis["app_state"]["feemarket"]["base_fee"] = ANCHOR_LANE_CONFIG["feemarket"]["base_fee"]

    print(f"Updated feemarket.params.max_tx_gas: {ANCHOR_LANE_CONFIG['feemarket']['params']['max_tx_gas']}")
    print(f"Updated feemarket.params.min_gas_price: {ANCHOR_LANE_CONFIG['feemarket']['params']['min_gas_price']}")
    print(f"Updated feemarket.params.target_block_utilization: {ANCHOR_LANE_CONFIG['feemarket']['params']['target_block_utilization']}")

    # Write updated genesis
    with open(genesis_path, 'w') as f:
        json.dump(genesis, f, indent=2)

    print(f"\nGenesis updated successfully: {genesis_path}")
    print("\nAnchor Lane Configuration Summary:")
    print("  - Max block gas: 60,000,000 (60M)")
    print("  - Max tx gas: 2,000,000 (2M) - no single tx dominance")
    print("  - Max block bytes: 10,485,760 (10MB)")
    print("  - Target utilization: 33%")
    print("  - Min gas price: 0.025 uomni")
    print("  - Target TPS: ~100 (range: 50-150)")


def main():
    if len(sys.argv) != 2:
        print("Usage: python3 update_genesis_anchor_lane.py /path/to/genesis.json")
        print("\nThis script updates genesis.json with anchor lane configuration.")
        print("WARNING: Only use for new chains or coordinated resets.")
        sys.exit(1)

    genesis_path = sys.argv[1]
    update_genesis(genesis_path)


if __name__ == "__main__":
    main()
