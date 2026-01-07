# VPS Anchor Lane Configuration Guide

This guide explains the Omniphi **Anchor Lane** configuration optimized for security and decentralization.

## Configuration Summary

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Block Time | **4.0s** | Fast finality |
| Max Block Gas | **60M** | Conservative for decentralization |
| Max Tx Gas | **2M** | Prevent single-tx dominance (3.3% of block) |
| Target TPS | **~100** | Sustainable for global validators |
| Target Utilization | **33%** | Headroom for traffic spikes |
| Min Gas Price | **0.025** | Low barrier to entry |

## Why 2M Max Tx Gas?

The anchor lane uses a strict 2M gas limit per transaction to ensure:

1. **No Single-Tx Dominance**: 2M = 3.3% of 60M block capacity
2. **Requires 30+ Txs**: To fill a block, preventing griefing
3. **Protects Validators**: Lower-end hardware can participate
4. **Forces Heavy Compute Off-Chain**: Smart contracts run on PoSeq, not anchor

## CometBFT Configuration

For ~4 second blocks, update `~/.pos/config/config.toml`:

```toml
[consensus]
timeout_propose = "1500ms"
timeout_propose_delta = "200ms"
timeout_prevote = "500ms"
timeout_prevote_delta = "200ms"
timeout_precommit = "500ms"
timeout_precommit_delta = "200ms"
timeout_commit = "1500ms"

[instrumentation]
prometheus = true
prometheus_listen_addr = ":26660"
```

Or use the script:
```bash
./scripts/update_anchor_lane_config.sh
```

## Genesis Configuration

These parameters are set in genesis (or via governance):

```json
{
  "consensus": {
    "params": {
      "block": {
        "max_gas": "60000000"
      }
    }
  },
  "app_state": {
    "feemarket": {
      "params": {
        "max_tx_gas": "2000000",
        "min_gas_price": "0.025",
        "target_block_utilization": "0.33",
        "burn_cool": "0.10",
        "burn_normal": "0.20",
        "burn_hot": "0.40"
      }
    }
  }
}
```

## Governance Proposals

To update parameters on a running chain, use governance proposals.

### Example: Update Max Tx Gas

See `proposals/update_max_tx_gas_2m.json` for format.

```bash
posd tx gov submit-proposal proposals/update_max_tx_gas_2m.json \
  --from validator \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --gas auto --gas-adjustment 1.5 \
  --fees 20000omniphi -y
```

### Example: Update Block Max Gas

See `proposals/update_block_max_gas_60m.json` for format.

## Monitoring

### Check Block Times
```bash
watch -n 2 'posd status 2>&1 | jq -r ".sync_info | \"Height: \(.latest_block_height)\""'
```

### Check Gas Parameters
```bash
posd query feemarket params --output json | jq '{max_tx_gas, min_gas_price}'
posd query consensus params --output json | jq '.params.block'
```

### Prometheus Metrics
Access at: `http://your-node-ip:26660/metrics`

| Metric | Target | Warning |
|--------|--------|---------|
| Block interval | 4.0s | >4.5s |
| Mempool size | <1000 | >3000 |
| Block utilization | 33% | >70% |

## Burn Tiers

| Utilization | Tier | Burn Rate |
|-------------|------|-----------|
| 0-16% | Cool | 10% |
| 16-33% | Normal | 20% |
| 33%+ | Hot | 40% |

## Troubleshooting

### Blocks Too Slow
1. Check network latency between validators
2. Verify all validators have updated CometBFT config
3. Check for slow validators in the active set

### High Gas Utilization
1. Burn rate automatically increases (Hot tier)
2. Base fee increases via EIP-1559 mechanism
3. Consider governance proposal for limit changes

### Validator Missing Blocks
```bash
posd query slashing signing-infos
posd query staking validators --output json | jq '.validators[] | {moniker: .description.moniker, status}'
```
