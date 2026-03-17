# Phase 5: Economic Calibration Report

## Solver Profit Analysis

### Revenue Model
Solvers earn from the fee spread: `solver_fee_bps` portion of the total fee charged on successful settlements.

At default parameters:
- Typical fee: 30 bps total (15 solver + 15 protocol)
- On a 1,000 OMNI swap: solver earns ~1.5 OMNI per fill

### Break-Even Analysis
- Active solver bond: 50,000 OMNI (mainnet) / 500 OMNI (testnet)
- Commit-without-reveal penalty: 1% of locked bond per occurrence
- At 10,000 OMNI locked per commitment: 100 OMNI penalty per no-reveal
- Break-even: ~67 successful fills to recover one no-reveal penalty

### Profitability Threshold
- Solver must win and successfully settle at least 1 intent per 67 slots to stay profitable after occasional no-reveals
- With 4+ solvers competing, realistic win rate is 25-30%
- Therefore: profitable at ~3+ intents per 67 slots submitted to the pool

## Slashing Incentive Analysis

| Violation | Penalty | Deterrent Analysis |
|-----------|---------|-------------------|
| Commit without reveal | 1% bond | Mild deterrent; sufficient for accidental misses |
| Invalid settlement | 25% bond | Strong deterrent; one violation costs 12,500 OMNI |
| Double fill | 100% bond + ban | Total deterrent; no rational actor would attempt |
| Invalid reveal | 100% bond + ban | Total deterrent; impossible to profit |

### Griefing Analysis
- **Commit-without-reveal spam**: Attacker commits 10 bundles per window (max), each costs 1% = 500 OMNI per window. At 9 blocks/window, griefing costs ~55 OMNI/block. This is expensive but possible.
- **Mitigation**: Violation score escalation means repeated offenders are auto-deactivated at score 9,500. After ~95 no-reveals, the solver is permanently deactivated.
- **Recommendation**: Testnet should use 1% penalty. Mainnet may increase to 3-5% based on observed griefing frequency.

## Spam Economics

### Intent Pool Spam
- Rate limit: 10 intents/block/user → 10 × 6s = ~100/min/user
- Fee floor: 10 bps → minimum intent value must justify the fee
- Pool capacity: 50,000 → would require 5,000 unique users spamming simultaneously to fill
- Eviction: lowest tip first → legitimate high-tip intents survive spam

### Verdict: Spam resistance is adequate for testnet. Monitor pool utilization.

## Validator Reward Sustainability

### Revenue Distribution (per epoch)
- 50% to validators (proportional to uptime)
- 30% to solvers (proportional to fills × quality)
- 10% to treasury
- 10% to insurance fund

### Sustainability Check
- With 4 validators and 100,000 OMNI epoch revenue:
  - Each validator gets ~12,500 OMNI (if equal uptime)
  - At 100,000 OMNI stake: 12.5% return per epoch
- With 21 validators: ~2,380 OMNI each (~2.4% per epoch)

### Recommendation
- Testnet: min_validator_stake = 1,000 OMNI (low barrier for testing)
- Mainnet: calibrate min_stake so validator returns exceed opportunity cost

## Testnet Parameter Recommendations

| Parameter | Testnet Value | Rationale |
|-----------|--------------|-----------|
| min_validator_stake | 1,000 | Low barrier for testing |
| min_solver_bond | 100 | Allow experimentation |
| active_solver_bond | 500 | Meaningful but not prohibitive |
| commit_phase_blocks | 2 | Fast iteration |
| reveal_phase_blocks | 2 | Fast iteration |
| fast_dispute_window | 10 | Quick dispute resolution |
| commit_without_reveal_penalty | 100 bps (1%) | Match mainnet but with lower absolute value |

These values are encoded in `ProtocolParameters::testnet()`.
