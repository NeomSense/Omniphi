# DAO-Governed Emission Split System

## Overview

The Omniphi Emission Split System provides a **predictable, capped, and DAO-governed** mechanism for distributing new token emissions across network roles. This is separate from fee revenue and operates purely on inflationary token issuance.

## Design Principles

### What This System IS:
- A **predictable emission schedule** with decaying inflation
- A **DAO-governed allocation** with protocol-enforced safety bounds
- A **multi-recipient distribution** ensuring balanced incentives
- A **fully auditable** system with emission records per epoch

### What This System is NOT:
- NOT a fee distribution mechanism (fees are handled separately)
- NOT controlled by any single entity (protocol bounds are immutable)
- NOT subject to arbitrary changes (governance has limits)

## Protocol Safety Bounds (Immutable)

These constants are **hard-coded in the protocol** and **CANNOT be changed by governance**:

```go
const (
    // Maximum annual inflation rate - 3%
    MaxAnnualInflationRateHardCap = "0.03"

    // Maximum share any single recipient can receive - 60%
    MaxSingleRecipientShare = "0.60"

    // Minimum share for staking (PoS security) - 20%
    MinStakingShare = "0.20"
)
```

### Why These Bounds?

| Bound | Value | Rationale |
|-------|-------|-----------|
| Max Inflation | 3% | Prevents runaway inflation, aligns with low-inflation L1s |
| Max Single Recipient | 60% | Prevents centralization of emission allocation |
| Min Staking Share | 20% | Ensures validator security incentives are maintained |

## Emission Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        INFLATION CALCULATION                                 │
│                                                                             │
│  Annual Provisions = Current Supply × Inflation Rate                        │
│  Block Provision = Annual Provisions ÷ Blocks Per Year                     │
│                                                                             │
│  Inflation Schedule (Decaying):                                             │
│    Year 1: 3.00%   Year 4: 2.25%   Year 7+: -0.25%/yr until 0.5% floor     │
│    Year 2: 2.75%   Year 5: 2.00%                                           │
│    Year 3: 2.50%   Year 6: 1.75%                                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         EMISSION SPLIT (Per Epoch)                          │
│                                                                             │
│  Total Emission = Block Provision × Blocks Per Epoch (100 blocks default)  │
│                                                                             │
│         ┌──────────────┬──────────────┬──────────────┬──────────────┐      │
│         │   STAKING    │     PoC      │  SEQUENCER   │   TREASURY   │      │
│         │     40%      │     30%      │     20%      │     10%      │      │
│         │              │              │              │              │      │
│         │  Validator   │  Proof of    │  Ordering    │  DAO         │      │
│         │  Rewards     │  Contribution│  Layer       │  Operations  │      │
│         └──────────────┴──────────────┴──────────────┴──────────────┘      │
│                                                                             │
│  Validation Rules:                                                          │
│    ✓ Sum MUST equal 100%                                                   │
│    ✓ No recipient > 60%                                                    │
│    ✓ Staking ≥ 20%                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      MODULE ACCOUNT DISTRIBUTION                            │
│                                                                             │
│  ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐          │
│  │ staking module  │   │   poc module    │   │sequencer module │          │
│  │                 │   │                 │   │                 │          │
│  │ Distributes to  │   │ Distributes to  │   │ Distributes to  │          │
│  │ validators      │   │ contributors    │   │ sequencers      │          │
│  └─────────────────┘   └─────────────────┘   └─────────────────┘          │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                     tokenomics module (treasury)                   │     │
│  │                                                                    │     │
│  │  → Ecosystem Grants (40%)     → Insurance Fund (20%)              │     │
│  │  → Buy and Burn (30%)         → Research Fund (10%)               │     │
│  └───────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Default Configuration

### Production Values (Mainnet)

```json
{
  "inflation_rate": "0.03",
  "inflation_min": "0.005",
  "inflation_max": "0.03",
  "emission_split_staking": "0.40",
  "emission_split_poc": "0.30",
  "emission_split_sequencer": "0.20",
  "emission_split_treasury": "0.10"
}
```

### Testnet Values

Same splits as mainnet for consistency, with faster epoch intervals for testing.

## Governance Controls

### What DAO CAN Change

| Parameter | Range | Default |
|-----------|-------|---------|
| `emission_split_staking` | 20-60% | 40% |
| `emission_split_poc` | 0-60% | 30% |
| `emission_split_sequencer` | 0-60% | 20% |
| `emission_split_treasury` | 0-60% | 10% |
| `inflation_rate` | 0.5-3% | 3% |
| `inflation_min` | 0-3% | 0.5% |
| `inflation_max` | 0-3% | 3% |

### What DAO CANNOT Change

- Maximum inflation hard cap (3%)
- Maximum single recipient share (60%)
- Minimum staking share (20%)
- Supply cap (1.5B OMNI)

### Parameter Update Rules

1. **Splits must sum to 100%** - Reject any proposal where splits ≠ 1.0
2. **No single recipient > 60%** - Prevents centralization
3. **Staking ≥ 20%** - Ensures PoS security
4. **Changes apply next epoch** - Not retroactive

## Validation Logic

```go
func ValidateEmissionParams(params Params) error {
    // 1. Inflation must be within bounds
    maxCap := math.LegacyMustNewDecFromStr(MaxAnnualInflationRateHardCap)
    if params.InflationMax.GT(maxCap) {
        return ErrInflationExceedsHardCap
    }

    // 2. Emission splits must sum to 100%
    sum := params.EmissionSplitStaking.
        Add(params.EmissionSplitPoc).
        Add(params.EmissionSplitSequencer).
        Add(params.EmissionSplitTreasury)
    if !sum.Equal(math.LegacyOneDec()) {
        return ErrEmissionSplitInvalid
    }

    // 3. No single recipient exceeds 60%
    maxShare := math.LegacyMustNewDecFromStr(MaxSingleRecipientShare)
    splits := []math.LegacyDec{
        params.EmissionSplitStaking,
        params.EmissionSplitPoc,
        params.EmissionSplitSequencer,
        params.EmissionSplitTreasury,
    }
    for _, split := range splits {
        if split.GT(maxShare) {
            return ErrEmissionRecipientExceedsCap
        }
    }

    // 4. Staking meets minimum
    minStaking := math.LegacyMustNewDecFromStr(MinStakingShare)
    if params.EmissionSplitStaking.LT(minStaking) {
        return ErrStakingShareBelowMinimum
    }

    return nil
}
```

## Precision Requirements

### No Floating-Point Math

All calculations use:
- `sdk.Dec` for ratios (18 decimal precision)
- `sdk.Int` for token amounts
- `sdk.Coin` for token transfers

### Rounding Rules

1. **Truncation** for individual allocations
2. **Dust to treasury** - Remainder goes to treasury deterministically
3. **Conservation invariant** - Sum of allocations MUST equal total emission

```go
// Calculate allocations with truncation
stakingAmount := totalDec.Mul(stakingRatio).TruncateInt()
pocAmount := totalDec.Mul(pocRatio).TruncateInt()
sequencerAmount := totalDec.Mul(sequencerRatio).TruncateInt()
treasuryAmount := totalDec.Mul(treasuryRatio).TruncateInt()

// Handle dust (remainder to treasury)
distributed := stakingAmount.Add(pocAmount).Add(sequencerAmount).Add(treasuryAmount)
if distributed.LT(totalAmount) {
    remainder := totalAmount.Sub(distributed)
    treasuryAmount = treasuryAmount.Add(remainder)
}
```

## Emission Records

Every emission event is recorded for auditing:

```proto
message EmissionRecord {
    uint64 emission_id = 1;
    int64 block_height = 2;
    string total_emitted = 3;
    string to_staking = 4;
    string to_poc = 5;
    string to_sequencer = 6;
    string to_treasury = 7;
    int64 timestamp = 8;
}
```

### Query Endpoints

```bash
# Get current emission parameters
posd query tokenomics params

# Get current inflation rate
posd query tokenomics inflation

# Get emission split configuration
posd query tokenomics emissions

# Get recent emission records
posd query tokenomics emission-records --limit 10

# Get emission statistics
posd query tokenomics emission-stats
```

## Economic Analysis

### 10-Year Supply Projection

With 3% initial inflation decaying to 0.5%:

| Year | Inflation | Supply (M OMNI) | Annual Emission |
|------|-----------|-----------------|-----------------|
| 1 | 3.00% | 386.25 | 11.25M |
| 2 | 2.75% | 396.87 | 10.62M |
| 3 | 2.50% | 406.79 | 9.92M |
| 4 | 2.25% | 415.94 | 9.15M |
| 5 | 2.00% | 424.26 | 8.32M |
| 6 | 1.75% | 431.68 | 7.42M |
| 7 | 1.50% | 438.15 | 6.47M |
| 8 | 1.25% | 443.63 | 5.48M |
| 9 | 1.00% | 448.07 | 4.44M |
| 10 | 0.75% | 451.43 | 3.36M |

### Allocation Over 10 Years

| Recipient | Total Received | % of Total |
|-----------|---------------|------------|
| Staking | 30.94M OMNI | 40% |
| PoC | 23.21M OMNI | 30% |
| Sequencer | 15.47M OMNI | 20% |
| Treasury | 7.74M OMNI | 10% |

## Future Extensions

The emission system is designed to support future additions:

### Adding New Recipients

```go
// Future: Add AI incentives module
message EmissionSplit {
    string staking = 1;
    string poc = 2;
    string sequencer = 3;
    string treasury = 4;
    // Future extensions
    string ai_incentives = 5;
    string grants_program = 6;
}
```

### Versioned Migrations

```go
// Migration from V1 to V2 with new recipient
func MigrateEmissionSplitV1ToV2(v1 EmissionSplitV1) EmissionSplitV2 {
    // Reduce existing splits proportionally
    // Add new recipient allocation
}
```

## Comparison with Industry Standards

| Feature | Omniphi | Cosmos Hub | Ethereum PoS |
|---------|---------|------------|--------------|
| Max Inflation | 3% | 20% | ~0.5% |
| Staking Share | 20-60% | ~67% | 100% |
| Multi-recipient | Yes (4) | Yes (2) | No |
| DAO Governed | Yes | Yes | No |
| Decay Schedule | Yes | Dynamic | N/A |

## Audit Checklist

- [x] Emission rate never exceeds 3% annually
- [x] Split always sums to 100%
- [x] No single recipient exceeds 60%
- [x] Staking receives at least 20%
- [x] Minted emissions match sum of allocations
- [x] Governance updates apply next epoch
- [x] Deterministic behavior across all nodes
- [x] No floating-point arithmetic
- [x] Conservation invariant maintained
- [x] Supply cap enforced at 1.5B OMNI
