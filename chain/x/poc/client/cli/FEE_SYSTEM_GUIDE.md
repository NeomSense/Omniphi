# 3-Layer Fee System Guide

## Overview

The x/poc module implements a sophisticated 3-layer fee system that combines:
1. **Base Fee Model** - Static foundation fee
2. **Epoch-Adaptive Fee Model** - Dynamic congestion-based multiplier
3. **C-Score Weighted Discount Model** - Reputation-based fee reduction

## Fee Calculation Formula

```
final_fee = max(
    base_fee × epoch_multiplier × (1 - cscore_discount),
    minimum_fee
)
```

### Layer 1: Base Fee Model
- **Parameter**: `base_submission_fee`
- **Default**: 30,000 omniphi (0.03 OMNI)
- **Description**: Static fee charged for every contribution submission
- **Fee Split**: 50% burned (deflationary), 50% to PoC reward pool

### Layer 2: Epoch-Adaptive Fee Model (EIP-1559-like)
- **Parameter**: `target_submissions_per_block`
- **Default**: 5 submissions per block
- **Formula**: `epoch_multiplier = max(0.8, min(5.0, current_submissions / target))`
- **Range**: 0.8x (quiet periods) to 5.0x (extreme congestion)
- **Description**: Dynamically adjusts fees based on network congestion
- **Examples**:
  - 0 submissions: 0.8x multiplier (20% discount)
  - 5 submissions (at target): 1.0x multiplier (base fee)
  - 10 submissions (2x target): 2.0x multiplier (double fee)
  - 25+ submissions: 5.0x multiplier (5x base fee, capped)

### Layer 3: C-Score Weighted Discount Model
- **Parameter**: `max_cscore_discount`
- **Default**: 0.90 (90% maximum discount)
- **Formula**: `cscore_discount = min(max_discount, cscore / 1000)`
- **C-Score Range**: 0-1000
- **Description**: Rewards high-reputation contributors with fee discounts
- **Examples**:
  - C-Score 0: No discount
  - C-Score 100: 10% discount
  - C-Score 500: 50% discount
  - C-Score 1000+: 90% discount (capped)

### Minimum Fee Floor
- **Parameter**: `minimum_submission_fee`
- **Default**: 3,000 omniphi (0.003 OMNI)
- **Description**: Absolute minimum fee after all discounts
- **Purpose**: Ensures economic sustainability even for high-reputation users

## Fee Calculation Examples

### Example 1: New Contributor, Normal Conditions
```
C-Score: 0
Current Submissions: 5 (at target)
Base Fee: 30,000 omniphi

Calculation:
- Epoch Multiplier: 5 / 5 = 1.0
- C-Score Discount: 0 / 1000 = 0%
- Final Fee: 30,000 × 1.0 × (1 - 0) = 30,000 omniphi
```

### Example 2: High-Reputation Contributor, Normal Conditions
```
C-Score: 1000
Current Submissions: 5 (at target)
Base Fee: 30,000 omniphi

Calculation:
- Epoch Multiplier: 5 / 5 = 1.0
- C-Score Discount: min(0.90, 1000/1000) = 90%
- Final Fee: 30,000 × 1.0 × (1 - 0.90) = 3,000 omniphi (minimum fee floor)
```

### Example 3: Medium Contributor, High Congestion
```
C-Score: 500
Current Submissions: 25 (5x target)
Base Fee: 30,000 omniphi

Calculation:
- Epoch Multiplier: min(5.0, 25/5) = 5.0 (capped)
- C-Score Discount: 500 / 1000 = 50%
- Final Fee: 30,000 × 5.0 × (1 - 0.50) = 75,000 omniphi
```

### Example 4: Low Traffic Period
```
C-Score: 0
Current Submissions: 0 (no submissions)
Base Fee: 30,000 omniphi

Calculation:
- Epoch Multiplier: max(0.8, 0/5) = 0.8 (minimum)
- C-Score Discount: 0 / 1000 = 0%
- Final Fee: 30,000 × 0.8 × (1 - 0) = 24,000 omniphi
```

## Fee Split

All collected fees are split as follows:
- **50% Burned**: Permanently removed from circulation (deflationary pressure)
- **50% to PoC Reward Pool**: Used to reward verified contributions

This split balances:
- Economic sustainability (deflation)
- Contributor incentivization (reward pool funding)

## Querying Current Fee

To check what fee you would pay for the next submission:

```bash
# Query current params
posd q poc params

# Relevant fields:
# - base_submission_fee: Base fee amount
# - target_submissions_per_block: Congestion target
# - max_cscore_discount: Maximum discount percentage
# - minimum_submission_fee: Minimum fee floor

# Query your C-Score
posd q poc credits <your-address>

# The amount field is your C-Score for discount calculation
```

## Governance Parameters

All fee parameters are governable via x/gov proposals:

### Adjustable Parameters:
- `base_submission_fee`: Adjust base fee (e.g., for inflation/deflation)
- `target_submissions_per_block`: Tune congestion sensitivity
- `max_cscore_discount`: Control maximum reputation discount
- `minimum_submission_fee`: Set minimum viable fee floor

### Parameter Update Example:
```bash
# Create governance proposal to update base fee
posd tx gov submit-proposal param-change proposal.json \
  --from <key> \
  --chain-id <chain-id>

# Example proposal.json:
{
  "title": "Increase PoC Base Submission Fee",
  "description": "Increase base fee from 30,000 to 50,000 omniphi due to network growth",
  "changes": [{
    "subspace": "poc",
    "key": "base_submission_fee",
    "value": "50000omniphi"
  }]
}
```

## Events

Every fee payment emits a detailed event for transparency:

```
Type: "poc_3layer_fee"
Attributes:
  - contributor: Address of submitter
  - total_fee: Final fee charged
  - burned: Amount burned (50%)
  - to_pool: Amount to reward pool (50%)
  - epoch_multiplier: Congestion multiplier applied
  - cscore_discount: Reputation discount applied
```

## Economic Design Goals

1. **Congestion Management**: Higher fees during high demand encourage strategic timing
2. **Reputation Rewards**: Long-term contributors pay significantly less
3. **Deflationary Pressure**: 50% burn reduces supply over time
4. **Sustainable Incentives**: 50% to reward pool funds future rewards
5. **Predictable Minimums**: Floor prevents extreme discounts
6. **Governance Flexibility**: All parameters adjustable via governance

## Best Practices

### For Contributors:
1. **Build C-Score**: Regular verified contributions earn up to 90% discounts
2. **Monitor Congestion**: Submit during quiet periods for lower multipliers
3. **Check Events**: Review fee events to understand your costs
4. **Plan Timing**: Large batches benefit from low-congestion windows

### For Validators:
1. **Monitor target_submissions_per_block**: Adjust if blocks consistently over/under target
2. **Track Fee Metrics**: Use fee statistics queries to analyze network usage
3. **Governance Proposals**: Propose parameter updates based on network conditions

### For Governance:
1. **Review Quarterly**: Assess if fee levels match network value
2. **Adjust for Growth**: Increase targets as network capacity improves
3. **Balance Incentives**: Ensure rewards pool remains adequately funded
4. **Monitor Burn Rate**: Track deflationary impact on tokenomics

## Technical Details

- **Submission Counter**: Tracked in transient store (auto-resets each block)
- **C-Score Calculation**: Based on accumulated verified contribution credits
- **Precision**: All calculations use `math.LegacyDec` for exact arithmetic
- **Atomicity**: Fee collection occurs before contribution creation
- **Gas Efficiency**: Optimized with validator caching and transient storage

## Troubleshooting

### "Fee below minimum" Error
- Your discounts reduced fee below `minimum_submission_fee`
- Minimum fee will be charged instead
- This is expected behavior for high C-Score contributors

### High Fees During Congestion
- Network is experiencing high submission volume
- Consider waiting for next block if cost-sensitive
- Fee will normalize as submissions decrease

### Fee Calculation Mismatch
- Check your current C-Score: `posd q poc credits <addr>`
- Verify current block submission count (check recent blocks)
- Remember: epoch_multiplier applies to CURRENT block state

## Version History

- **v1.0**: Initial 3-layer fee system implementation
- **Fields 19-22**: Proto definitions for fee parameters
- **JSON Storage**: Hybrid storage approach for compatibility
