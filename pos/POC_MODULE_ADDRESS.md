# PoC Module Address - Correct Reference

## The Correct PoC Module Address

**Bech32 Address**: `omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7`

This is the ACTUAL module account address for the PoC module on your chain, as queried from the running blockchain.

## What Went Wrong

The documentation previously showed:
- ❌ **Wrong**: `omni1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsttxc2`
- This address has an **invalid checksum** - it's not a real bech32 address
- Trying to use it results in: `decoding bech32 failed: invalid checksum`

## How to Find Module Addresses

To get any module address from your chain:

```powershell
$env:BINARY = ".\build\posd.exe"
$env:HOME_DIR = "$env:USERPROFILE\.pos"

# Query all module accounts
& $env:BINARY q auth module-accounts --home $env:HOME_DIR --output json
```

Look for the module name in the output:
```json
{
  "type": "/cosmos.auth.v1beta1.ModuleAccount",
  "value": {
    "address": "omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7",
    "name": "poc",
    "permissions": ["minter", "burner"]
  }
}
```

## Correct Commands

### Fund PoC Module

```powershell
$env:BINARY = ".\build\posd.exe"
$env:HOME_DIR = "$env:USERPROFILE\.pos"

& $env:BINARY tx bank send alice omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7 10000000omniphi --from alice --keyring-backend test --chain-id omniphi-1 --gas auto --fees 25000omniphi -y
```

### Check PoC Module Balance

```powershell
& $env:BINARY q bank balances omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7 --home $env:HOME_DIR
```

## All Module Addresses on Your Chain

From the query, here are all module accounts:

| Module | Address | Permissions |
|--------|---------|-------------|
| **poc** | `omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7` | minter, burner |
| tokenomics | `omni14cvnmzrt9cz4qdf5lhs0xn3u0a3gymlag5kyq8` | minter, burner |
| mint | `omni1m3h30wlvsf8llruxtpukdvsy0km2kum8cmkjj5` | minter |
| bonded_tokens_pool | `omni1fl48vsnmsdzcv85q5d2q4z5ajdha8yu393c9vr` | burner, staking |
| not_bonded_tokens_pool | `omni1tygms3xhhs3yv487phx3dw4a95jn7t7l33y56h` | burner, staking |
| distribution | `omni1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8h44kut` | - |
| fee_collector | `omni17xpfvakm2amg962yls6f84z3kell8c5lqnj27f` | - |
| gov | `omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8` | burner |
| nft | `omni1hr93qzcjspaa32px0qqywlh9hf9a8plgljhujn` | - |
| transfer | `omni1yl6hdjhmkf37639730gffanpzndzdpmh748rzh` | minter, burner |
| interchainaccounts | `omni1vlthgax23ca9syk7xgaz347xmf4nunefhz8aey` | - |

## Why Module Addresses Are Important

The PoC module needs to have a balance in order to distribute rewards. When a contribution is verified:

1. Contribution gets `verified: true` and `credits` assigned
2. The `ProcessPendingRewards` function runs in EndBlocker
3. It checks the PoC module balance (in "omniphi" denom)
4. If balance > 0, it sends rewards to contributors
5. Sets `rewarded: true` on the contribution

**Without funding the module, rewards cannot be distributed!**

---

**Updated**: October 25, 2025
**Status**: All scripts and documentation updated with correct address