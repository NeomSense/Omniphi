# Validator Genesis Creation - Problem & Solution

## The Problem

When starting the Omniphi chain on Ubuntu, it crashed with:
```
error during handshake: error on replay: validator set is empty after InitGenesis
```

Despite having validators in `app_state.staking.validators`, the chain wouldn't start.

## Root Cause Analysis

After extensive debugging, we discovered **TWO critical issues**:

### Issue 1: Empty genutil.gen_txs Array

The genesis file had:
```json
"genutil": {
  "gen_txs": []
}
```

**Why this matters**: The `genutil` module processes genesis transactions (gentxs) during InitGenesis. If the `gen_txs` array is empty, no validators are created, even if they exist in the staking genesis state.

### Issue 2: Bond Denomination Mismatch

The `collect-gentxs` command **overwrites** the `bond_denom` back to "omniphi" (the default mint denom), but the gentx uses "uomni". This mismatch causes validation failures.

## The Solution

### Correct Setup Sequence

```bash
# 1. Initialize chain
./posd init <moniker> --chain-id <chain-id>

# 2. Add genesis account
./posd genesis add-genesis-account <key> <amount>uomni --keyring-backend test

# 3. Fix bond_denom FIRST TIME (before gentx)
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# 4. Configure gas prices
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001uomni"/' ~/.pos/config/app.toml

# 5. Create gentx (creates validator transaction)
./posd genesis gentx <key> <self-delegation>uomni \
  --chain-id <chain-id> \
  --moniker <moniker> \
  --commission-rate 0.1 \
  --commission-max-rate 0.2 \
  --commission-max-change-rate 0.01 \
  --min-self-delegation 1 \
  --keyring-backend test

# 6. Collect gentxs (adds to genutil.gen_txs)
./posd genesis collect-gentxs

# 7. Fix bond_denom SECOND TIME (collect-gentxs overwrites it!)
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# 8. Validate genesis
./posd genesis validate-genesis

# 9. Start chain
./posd start --minimum-gas-prices 0.001uomni
```

### Key Discovery

The `collect-gentxs` command overwrites `bond_denom` back to the mint module's default ("omniphi"). This happens because collect-gentxs regenerates parts of the staking genesis based on the collected transactions.

**Therefore**: You must fix bond_denom **TWICE**:
1. **Before gentx** - So the gentx transaction uses the correct denomination
2. **After collect-gentxs** - To match what the gentx expects

## What You'll See When It Works

### During Startup

```
INF InitChain chainID=omniphi-1 initialHeight=1 module=baseapp
INF initializing feemarket module from genesis module=server
WRN genesis params are empty or invalid, using defaults module=server
INF feemarket module initialized base_fee=0.050000000000000000 module=server
INF tokenomics genesis initialized allocations=0 genesis_supply=0 height=0
INF Completed ABCI Handshake - CometBFT and App are synced
INF This node is a validator addr=<validator-address> module=consensus pubKey=PubKeyEd25519{...}
```

**Key indicator**: The message **"This node is a validator"** with an address and pubKey

### In Genesis File

```json
"genutil": {
  "gen_txs": [
    {
      "body": {
        "messages": [
          {
            "@type": "/cosmos.staking.v1beta1.MsgCreateValidator",
            ...
          }
        ]
      },
      ...
    }
  ]
},
"staking": {
  "params": {
    "bond_denom": "uomni",  // MUST match gentx
    ...
  },
  "validators": [
    // Will be populated by genutil during InitGenesis
  ],
  ...
}
```

## Automated Setup

We've created `setup_ubuntu_fixed.sh` that handles all of this automatically:

```bash
cd ~/omniphi/pos
chmod +x setup_ubuntu_fixed.sh
./setup_ubuntu_fixed.sh
```

This script:
- ✅ Builds the binary
- ✅ Initializes the chain
- ✅ Creates validator key
- ✅ Fixes bond_denom (twice!)
- ✅ Creates and collects gentx
- ✅ Validates genesis
- ✅ Provides startup instructions

## Testing Verification

### On Windows (Proven Working)

```bash
# Clean start
rm -rf ~/.pos

# Initialize
./posd.exe init test-node --chain-id omniphi-test

# Create key
./posd.exe keys add testval --keyring-backend test

# Add account
./posd.exe genesis add-genesis-account testval 1000000000000000uomni --keyring-backend test

# Fix bond denom
sed -i 's/"bond_denom": "stake"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# Create gentx
./posd.exe genesis gentx testval 100000000000uomni --chain-id omniphi-test --keyring-backend test

# Collect gentxs
./posd.exe genesis collect-gentxs

# Fix bond denom AGAIN
sed -i 's/"bond_denom": "omniphi"/"bond_denom": "uomni"/' ~/.pos/config/genesis.json

# Validate
./posd.exe genesis validate-genesis

# Start
./posd.exe start --minimum-gas-prices 0.001uomni
```

**Result**: ✅ Chain starts with validator successfully

## Other Notes

### FeeMarket Empty Genesis

You may see:
```
WRN genesis params are empty or invalid, using defaults module=server
```

This is **expected and safe**. The feemarket module uses defensive programming to populate defaults when genesis is empty. This prevents crashes during `posd init`.

### Reflection Service Error

You may see at the end of startup:
```
failed to register reflection service: unable to create codec descriptor
```

This is **non-fatal** and doesn't prevent the chain from running. The validator still operates correctly.

## Summary

The "validator set empty" error was caused by:
1. ❌ Missing gentx in genutil.gen_txs array
2. ❌ Bond denomination mismatch after collect-gentxs

The fix:
1. ✅ Create gentx properly
2. ✅ Collect gentxs to populate genutil.gen_txs
3. ✅ Fix bond_denom twice (before gentx and after collect-gentxs)

The chain is now **ready for Ubuntu deployment** using `setup_ubuntu_fixed.sh`.

---

Generated: November 4, 2025
Testing Platform: Windows (PowerShell/Git Bash)
Deployment Target: Ubuntu (via setup_ubuntu_fixed.sh)
