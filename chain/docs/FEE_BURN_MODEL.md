# Omniphi Unified Fee-Burn Model

## Overview

Omniphi implements a **single-pass, multiplicative burn model** that is:
- Economically predictable
- Validator-safe
- Governance-friendly
- Auditor-approved

This document explains the fee-burn mechanism and how to tune it via governance.

## Architecture

### The Problem (Old Model)

The previous system applied burns additively:
1. Adaptive burn based on network utilization
2. **AND** independent burn percentages by activity type

This caused **double-counting**, unpredictable burn rates, and potential violations
of the maximum burn cap.

### The Solution (New Model)

```
effectiveBurnRate = min(baseBurnRate × activityMultiplier, MAX_BURN_CAP)
```

Where:
- `baseBurnRate` comes from network utilization (10%, 20%, or 40%)
- `activityMultiplier` adjusts based on transaction type (0.5x to 2.0x)
- `MAX_BURN_CAP` is protocol-enforced at 50%

## Burn Calculation Steps

### Step 1: Determine Base Burn Rate (Utilization-Driven)

| Network Utilization | Burn Rate | Tier Name |
|---------------------|-----------|-----------|
| < 16%               | 10%       | Cool      |
| 16% - 33%           | 20%       | Normal    |
| > 33%               | 40%       | Hot       |

### Step 2: Apply Activity Multiplier

| Activity Type    | Multiplier | Rationale                          |
|------------------|------------|-------------------------------------|
| Messaging/IBC    | 0.50x      | Encourage cross-chain activity      |
| PoC Anchoring    | 0.75x      | Encourage contributions             |
| PoS Gas (default)| 1.00x      | Baseline for staking/governance     |
| AI Queries       | 1.25x      | Higher computational cost           |
| Sequencer Ops    | 1.25x      | Data availability requirements      |
| Smart Contracts  | 1.50x      | Highest resource consumption        |

### Step 3: Compute Effective Burn

```go
effectiveBurn = min(baseBurn × multiplier, 0.50)
```

**Example: Smart contract during normal utilization**
```
effectiveBurn = min(0.20 × 1.50, 0.50) = min(0.30, 0.50) = 30%
```

**Example: Smart contract during hot utilization**
```
effectiveBurn = min(0.40 × 1.50, 0.50) = min(0.60, 0.50) = 50% (capped)
```

### Step 4: Apply Burn and Distribute Remaining Fees

```
burnAmount    = totalFee × effectiveBurnRate
remaining     = totalFee - burnAmount
treasuryAmt   = remaining × 0.30 (30%)
validatorAmt  = remaining × 0.70 (70%)
```

## Fee Distribution Summary

For a 100 uomni transaction fee at normal utilization (20% base) with PoS activity (1.0x):

| Destination  | Amount | Calculation                     |
|--------------|--------|----------------------------------|
| Burned       | 20     | 100 × 20%                       |
| Treasury     | 24     | (100 - 20) × 30%                |
| Validators   | 56     | (100 - 20) × 70%                |
| **Total**    | 100    |                                  |

## Governance Parameters

All parameters are adjustable via governance proposal:

### Burn Tier Thresholds
| Parameter           | Default | Range       | Description                    |
|---------------------|---------|-------------|--------------------------------|
| burn_cool           | 10%     | 0-20%       | Burn rate at low utilization   |
| burn_normal         | 20%     | 10-30%      | Burn rate at normal utilization|
| burn_hot            | 40%     | 20-50%      | Burn rate at high utilization  |
| util_cool_threshold | 16%     | 5-25%       | Utilization below = cool tier  |
| util_hot_threshold  | 33%     | 25-50%      | Utilization above = hot tier   |

### Activity Multipliers
| Parameter                   | Default | Range      | Description                   |
|-----------------------------|---------|------------|-------------------------------|
| multiplier_messaging        | 0.50x   | 0.25-2.0x  | IBC/messaging transactions    |
| multiplier_pos_gas          | 1.00x   | 0.25-2.0x  | Standard PoS operations       |
| multiplier_poc_anchoring    | 0.75x   | 0.25-2.0x  | PoC contribution submissions  |
| multiplier_smart_contracts  | 1.50x   | 0.25-2.0x  | Smart contract execution      |
| multiplier_ai_queries       | 1.25x   | 0.25-2.0x  | AI inference operations       |
| multiplier_sequencer        | 1.25x   | 0.25-2.0x  | L2 sequencer operations       |

### Safety Limits (Protocol Enforced)
| Parameter        | Value | Governance Can Change? |
|------------------|-------|------------------------|
| max_burn_ratio   | 50%   | No (protocol cap)      |
| min_multiplier   | 0.25x | Yes (within bounds)    |
| max_multiplier   | 2.00x | Yes (within bounds)    |

## Security Invariants

1. **Single Burn Path**: Only ONE burn calculation per transaction
2. **Maximum Burn Cap**: Burn rate NEVER exceeds 50%
3. **Validator Revenue**: Validators receive at least 50% of post-burn fees
4. **Deterministic Outcomes**: Same inputs always produce same outputs
5. **No Retroactive Effects**: Parameter changes take effect next block

## Implementation Location

- Core burn logic: `x/feemarket/keeper/unified_burn.go`
- Parameters: `x/feemarket/types/params_extra.go`
- Tests: `x/feemarket/keeper/unified_burn_test.go`

## Audit Checklist

- [x] No double counting of burns
- [x] Enforced 50% maximum cap
- [x] Deterministic calculation
- [x] All multipliers within governance bounds
- [x] Validator revenue >= 50% of distributable
- [x] Parameter validation on all updates
- [x] Comprehensive test coverage

## Migration Notes

The old `x/tokenomics` burn rates by activity type are **DEPRECATED**:
- `burn_rate_pos_gas`
- `burn_rate_poc_anchoring`
- `burn_rate_sequencer_gas`
- `burn_rate_smart_contracts`
- `burn_rate_ai_queries`
- `burn_rate_messaging`

These are replaced by the `multiplier_*` parameters in `x/feemarket`.
