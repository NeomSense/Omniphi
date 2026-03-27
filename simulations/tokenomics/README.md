# Omniphi Tokenomics Simulation Suite

A mathematically rigorous simulation and game-theoretic analysis of the Omniphi blockchain's token economics. Every formula is derived directly from the on-chain Go implementation.

## Overview

This suite models the complete Omniphi token economy over a configurable time horizon (default: 10 years / 3,650 epochs). It traces through inflation, emission distribution, burn mechanics, staking dynamics, RewardMult normalisation, and validator reward distribution -- matching the exact formulas in the production codebase.

### Source Code References

| Component | On-Chain Source | Simulation Module |
|-----------|----------------|-------------------|
| Inflation decay schedule | `chain/x/tokenomics/keeper/inflation_decay.go` | `simulation.py` |
| Emission distribution | `chain/x/tokenomics/keeper/inflation_decay.go:DistributeEmissions` | `simulation.py` |
| Supply cap enforcement | `chain/x/tokenomics/keeper/inflation_decay.go:MintInflation` | `simulation.py` |
| Burn mechanics | `chain/x/tokenomics/keeper/burn.go` | `simulation.py` |
| Fee burn model | `chain/x/feemarket/keeper/unified_burn.go` | `simulation.py` |
| Default parameters | `chain/x/tokenomics/types/params.go:DefaultParams()` | `simulation.py` |
| RewardMult EMA & normalisation | `chain/x/rewardmult/keeper/invariants.go` | `simulation.py` |
| Adaptive burn controller | `chain/x/tokenomics/keeper/adaptive_burn_controller.go` | `simulation.py` |

## Files

```
simulations/tokenomics/
  simulation.py          -- Core simulation engine
  scenarios.py           -- 8 predefined scenarios
  analysis.py            -- Game-theoretic analysis functions
  mev_analysis.py        -- MEV and sequencer security analysis
  run_simulation.py      -- CLI runner (JSON + CSV + console output)
  generate_baseline.py   -- Generate pre-computed baseline results
  results/
    baseline_10yr.json   -- Pre-computed 10-year baseline projection
```

## Quick Start

```bash
# Run all 8 scenarios with console summary
python simulations/tokenomics/run_simulation.py

# Run a single scenario
python simulations/tokenomics/run_simulation.py baseline

# Run with custom epoch count (20 years)
python simulations/tokenomics/run_simulation.py --epochs 7300

# List available scenarios
python simulations/tokenomics/run_simulation.py --list

# Generate just the baseline pre-computed results
python simulations/tokenomics/generate_baseline.py
```

No external dependencies are required -- only the Python 3.10+ standard library.

## Model Description

### Inflation Schedule

The inflation rate follows a year-based step decay, exactly matching `CalculateDecayingInflation` in `inflation_decay.go`:

| Year | Inflation Rate |
|------|---------------|
| 1    | 3.00%         |
| 2    | 2.75%         |
| 3    | 2.50%         |
| 4    | 2.25%         |
| 5    | 2.00%         |
| 6    | 1.75%         |
| 7    | 1.50%         |
| 8    | 1.25%         |
| 9    | 1.00%         |
| 10   | 0.75%         |
| 11+  | 0.50% (floor) |

**Formula (year >= 6)**: `rate = 0.0175 - 0.0025 * (year - 5)`, clamped to `[0.005, 0.03]`.

### Emission Distribution

Each epoch (1 day), newly minted tokens are distributed per the emission split from `DefaultParams()`:

- **Staking (40%)**: Distributed to validators proportional to stake weight, modified by RewardMult.
- **PoC (30%)**: Distributed to verified Proof-of-Contribution submissions.
- **Sequencer (20%)**: Distributed to sequencer operators.
- **Treasury (10%)**: Accumulated in the protocol treasury.

Rounding remainder goes to treasury (matches `DistributeEmissions` in `inflation_decay.go`).

### Burn Mechanics

Burns occur through fee activity, modelled as an aggregate annual rate:

```
epoch_burn_base = circulating_supply * (effective_burn_rate / epochs_per_year)
burn_to_treasury = epoch_burn_base * treasury_burn_redirect   (10%)
actual_burn = epoch_burn_base - burn_to_treasury
```

The `effective_burn_rate` is a scenario parameter that combines transaction volume and per-activity burn rates. On-chain, individual rates are:

| Activity | Burn Rate |
|----------|-----------|
| PoS Gas | 20% |
| PoC Anchoring | 25% |
| Sequencer Gas | 15% |
| Smart Contracts | 12% |
| AI Queries | 10% |
| Messaging | 8% |

Additionally, 90% of all transaction fees are burned (FeeBurnRatio = 0.90).

### Supply Conservation Law

At every epoch: `current_supply = total_minted - total_burned`

This invariant is enforced both in the simulation and on-chain (`params.go:Validate()`).

### Staking Dynamics

The simulation models staking ratio mean-reversion:

```
ratio_gap = target_ratio - current_ratio
adjustment = ratio_gap * adjustment_speed
staked += staking_emission + (supply * adjustment / epochs_per_year)
```

### RewardMult Smoothing

The RewardMult module applies EMA smoothing with alpha = 2/(N+1), N=8:

```
M_EMA(t) = alpha * M_raw(t) + (1 - alpha) * M_EMA(t-1)
```

Budget-neutral normalisation (V2.2, max 3 iterations) ensures:

```
sum(stake_i * M_effective_i) = sum(stake_i)
```

This prevents total reward inflation/deflation from multiplier drift.

## Parameter Reference

### SimulationConfig Fields

| Parameter | Default | Source |
|-----------|---------|--------|
| `total_supply_cap` | 1,500,000,000 OMNI | `params.go:DefaultParams()` |
| `genesis_supply` | 375,000,000 OMNI | `params.go:DefaultParams()` |
| `inflation_floor` | 0.5% | `params.go:InflationMin` |
| `inflation_ceiling` | 3.0% | `MaxAnnualInflationRateHardCap` |
| `emission_split_staking` | 40% | `params.go:EmissionSplitStaking` |
| `emission_split_poc` | 30% | `params.go:EmissionSplitPoc` |
| `emission_split_sequencer` | 20% | `params.go:EmissionSplitSequencer` |
| `emission_split_treasury` | 10% | `params.go:EmissionSplitTreasury` |
| `fee_burn_ratio` | 90% | `params.go:FeeBurnRatio` |
| `treasury_burn_redirect` | 10% | `params.go:TreasuryBurnRedirect` |
| `rewardmult_min` | 0.85 | RewardMult V2.2 default bounds |
| `rewardmult_max` | 1.15 | RewardMult V2.2 default bounds |
| `max_validators` | 125 | Chain config |
| `min_commission` | 5% | Chain config |
| `unbonding_days` | 21 | Chain config |

### Protocol Safety Bounds (Immutable)

These are hard-coded in `params.go` and cannot be changed by governance:

| Bound | Value | Rationale |
|-------|-------|-----------|
| Max Annual Inflation | 3% | Prevents runaway inflation |
| Max Single Recipient | 60% | Prevents emission centralisation |
| Min Staking Share | 20% | Guarantees PoS security funding |
| Max Burn Rate (per activity) | 50% | Prevents excessive deflation |
| RewardMult governance limits | [0.50, 2.00] | Bounds for governance override |

## Scenarios

### Economic Stress Tests

| Scenario | Burn Rate | Staking | Key Question |
|----------|-----------|---------|--------------|
| `baseline` | 5% | 60% | Normal operating conditions |
| `aggressive_burn` | 30% | 55% | When does deflation dominate? |
| `low_staking` | 2% | 25% | What is the cost to attack? |
| `max_inflation` | 0% | 70% | How fast does supply grow? |

### Game-Theoretic Attacks

| Scenario | Attack Vector | Key Question |
|----------|--------------|--------------|
| `whale_attack` | 33% stake concentration | Is governance capture profitable? |
| `poc_gaming` | Low-quality PoC submissions | Does gaming produce positive ROI? |
| `mev_extraction` | Sequencer front-running | What bond deters MEV extraction? |
| `validator_cartel` | Colluding PoC endorsements | Does budget-neutral normalisation limit cartel gains? |

## Interpretation Guide

### Key Metrics

- **Net Inflation %**: Positive = supply growing, negative = deflationary. The crossover year is when burn > minting.
- **Staking APY**: Nominal return to stakers. Real APY = Nominal APY + abs(net deflation rate).
- **Supply % of Cap**: How close to the 1.5B hard cap. With any meaningful burn rate, the cap is never reached.
- **Gini Coefficient**: 0 = perfect equality, 1 = total inequality. Target < 0.30 for healthy decentralisation.
- **Treasury Balance**: Should grow sustainably from emission split (10%) + burn redirects (10% of burns).

### Key Findings (Baseline)

1. **Deflationary crossover at year 7**: When inflation decays to 1.5% and burn rate holds at ~1.5% of total supply, the network becomes net-deflationary.

2. **Supply cap never reached**: With even modest burn activity (5% effective rate), peak supply is ~392M OMNI (26% of cap). The 1.5B cap serves as an insurance policy, not a realistic ceiling.

3. **Treasury self-sufficiency**: The 10% emission split + 10% burn redirect generates ~13.4M OMNI over 10 years, providing a sustainable funding runway.

4. **Staking APY declines gracefully**: From 2.0% in year 1 to 0.5% in year 10. Real yields are higher because deflation gives a purchasing power bonus. This gradual decline is by design -- early stakers are rewarded more.

5. **Nash equilibrium staking**: With 5% alternative yield and 5% burn, the equilibrium staking ratio is ~40%. With 2% alternatives, it rises to ~75%. The protocol's 60% target falls in a healthy range.

6. **PoC gaming is unprofitable**: With 10% acceptance rate for gaming vs. 80% for honest work, and a 0.85x RewardMult penalty, gaming ROI is deeply negative while honest contribution is profitable.

7. **MEV risk is low with threshold encryption**: The encrypted intent mempool with 4-of-10 threshold encryption reduces realizable MEV to ~0.003% of transaction volume, making MEV extraction uneconomical relative to legitimate sequencer revenue.

## Running Custom Analyses

```python
from simulation import SimulationConfig, simulate_epochs
from analysis import compute_staking_apy, nash_equilibrium_staking_ratio

# Custom scenario
cfg = SimulationConfig()
cfg.effective_burn_rate = Decimal("0.15")
cfg.target_staking_ratio = Decimal("0.45")

results = simulate_epochs(cfg, num_epochs=7300)  # 20 years
last = results[-1]
print(f"Final supply: {last.total_supply}")
print(f"Net inflation: {last.net_inflation}%")

# Game theory
eq_ratio = nash_equilibrium_staking_ratio(
    inflation_rate=0.01,
    effective_burn_rate=0.10,
    yield_alternatives=0.05,
)
print(f"Nash equilibrium staking ratio: {eq_ratio}")
```

## Methodology Notes

### Precision

All arithmetic uses Python's `decimal.Decimal` with 36-digit precision, matching Cosmos SDK's `LegacyDec` (18 decimal places) with comfortable headroom. No floating-point approximation errors.

### Simplifications

1. **Aggregate burn model**: On-chain, burns occur per-transaction with activity-specific rates. The simulation uses a single `effective_burn_rate` that combines activity mix and volume into one annual figure. This is appropriate for macro-economic projections but does not capture intra-day dynamics.

2. **Deterministic RewardMult**: The simulation uses fixed multiplier perturbations rather than simulating real uptime, slashing, and PoC participation. This is sufficient for budget-neutrality validation.

3. **No price dynamics**: Token price is exogenous to the simulation. The analysis module accepts price as a parameter for security budget calculations but does not model price discovery.

4. **No governance changes**: Parameters remain fixed throughout the simulation. In reality, governance can adjust emission splits, burn rates, and other parameters within protocol bounds.
