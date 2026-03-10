# Guard Module - Layered Sovereign Governance

The Guard module implements a **3-layer governance firewall** that provides risk evaluation, deterministic AI scoring, and advisory intelligence for governance proposals, protecting the chain from malicious or poorly-designed governance attacks.

## Overview

When a governance proposal passes, it does NOT execute immediately. Instead, it enters the Guard module's **Adaptive Sovereign Timelock (AST)**, which:

1. **Layer 1 — Rules Engine**: Classifies proposal type and computes base risk tier
2. **Layer 2 — Deterministic AI**: Runs integer-only inference for AI risk scoring
3. **Layer 3 — Advisory Intelligence**: Off-chain analysis with on-chain report anchoring
4. **Merge**: Applies `max(rules, AI)` constraint — AI can only increase protections
5. **Execution Gates**: Multi-phase delay with confirmation for CRITICAL proposals

## Architecture

### Layer 1: Rules Engine

The deterministic rules engine classifies proposals into risk tiers:

- **LOW**: Text-only proposals, minimal changes
- **MEDIUM**: Parameter changes, moderate treasury spends
- **HIGH**: Large treasury spends (>10%), economic param changes
- **CRITICAL**: Software upgrades, consensus-critical changes, slashing reductions

Risk evaluation is **deterministic** and based only on:
- Proposal message types
- Treasury spend percentage (if applicable)
- Whether proposal touches consensus-critical parameters

### Layer 2: Deterministic AI Guard

A protocol-native integer-only linear scoring model runs inside consensus on every validator. No external APIs, no floating point, no Python at runtime.

**Feature Schema v1** (11 features):

| Feature | Type | Range |
|---------|------|-------|
| is_upgrade | binary | 0-1 |
| is_param_change | binary | 0-1 |
| is_treasury_spend | binary | 0-1 |
| is_slashing_change | binary | 0-1 |
| is_poc_rule_change | binary | 0-1 |
| is_poseq_rule_change | binary | 0-1 |
| treasury_spend_bps | numeric | 0-10000 |
| modules_touched_count | numeric | 0-50 |
| touches_consensus_critical | binary | 0-1 |
| reduces_slashing | binary | 0-1 |
| changes_validator_rules | binary | 0-1 |

**Inference** (all int32/int64, zero floating point):

```
raw = bias + Σ(weights[i] × features[i])
score = clamp((raw / scale) + 50, 0, 100)
```

**Score-to-Tier**: 0-30 LOW, 31-55 MED, 56-80 HIGH, 81-100 CRITICAL

**Modes**:
- `ai_shadow_mode=true` — AI scores published in RiskReport but not enforced
- `binding_ai_enabled=true` — AI constraints merged via MaxConstraint

**Constitutional Invariant**: AI can only constrain, never relax:

```go
tier_final   = max(tier_rules,      tier_ai)
delay_final  = max(delay_rules,     delay_ai)
thresh_final = max(threshold_rules, threshold_ai)
```

Model weights are stored on-chain and updatable via `MsgUpdateAIModel` governance proposal.

See `ai/governance_model/v1/README.md` for the training pipeline.

### Layer 3: Advisory Intelligence

Off-chain advisory reports (from the gov-copilot service or human analysts) are anchored on-chain via `MsgSubmitAdvisoryLink`:

| Field | Description |
|-------|-------------|
| proposal_id | Governance proposal being analyzed |
| uri | IPFS/HTTPS link to the full report |
| report_hash | SHA256 hex of report bytes (integrity anchor) |
| reporter | Account address of the submitter |

Advisory links are queryable via `QueryAdvisoryLink` and emitted as events. They provide transparency but do not affect execution gates.

### RiskReport Fields

Each proposal's risk report includes:

| Field | Source | Description |
|-------|--------|-------------|
| tier | Layer 1 | Final risk tier (after merge) |
| score | Layer 1 | Rules-based score |
| computed_delay_blocks | Layer 1 | Delay from rules |
| computed_threshold_bps | Layer 1 | Threshold from rules |
| ai_score | Layer 2 | AI model score (0-100) |
| ai_tier | Layer 2 | AI risk tier |
| ai_delay_blocks | Layer 2 | AI-computed delay |
| ai_threshold_bps | Layer 2 | AI-computed threshold |
| ai_model_version | Layer 2 | Model version string |
| feature_schema_hash | Layer 2 | Schema hash for reproducibility |
| ai_features_hash | Layer 2 | SHA256 of feature vector |

### Execution Gates (3-Phase Model)

All proposals go through three execution gates:

#### 1. VISIBILITY Window (Default: 1 day)
- **Purpose**: Transparency and awareness
- **Actions**: Alerts are emitted, community can review
- **Optional**: Validator acknowledgements (future enhancement)

#### 2. SHOCK ABSORBER Window (Default: 2 days)
- **Purpose**: Prevent sudden economic shocks
- **Actions**: Treasury outflow throttles enforced (if enabled)
- **Limits**: Max 10% treasury outflow per day by default

#### 3. CONDITIONAL EXECUTION Window
- **Purpose**: Stability verification before execution
- **Checks**:
  - Validator power churn < 20% (if stability checks enabled)
  - Chain health metrics (future enhancement)
- **Behavior**: If checks fail, delay is extended

#### 4. READY State
- For CRITICAL proposals: **second confirmation required**
  - Confirmation must come from governance module authority
  - Confirmation window: 3 days (configurable)
  - If window expires, delay extends and gates restart
- Final threshold check before execution

### Default Delays

| Tier | Delay | Threshold |
|------|-------|-----------|
| LOW | 1 day | 50% |
| MED | 2 days | 50% |
| HIGH | 7 days | 66.67% |
| CRITICAL | 14 days | 75% |

*Assumes 5-second blocks (~17,280 blocks/day)*

## Lifecycle

```
Proposal Passed (x/gov)
         ↓
Risk Evaluation
         ↓
Queued Execution
         ↓
VISIBILITY → SHOCK_ABSORBER → CONDITIONAL_EXECUTION → READY → EXECUTED
         ↓                                                ↓
     (1 day)                                      (confirmation
         ↓                                         if CRITICAL)
  Treasury throttles
     monitored
```

## Messages

### MsgConfirmExecution

Confirms execution of a CRITICAL proposal. Must be sent by governance module authority.

```json
{
  "authority": "cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn",
  "proposal_id": 42,
  "justification": "Community consensus achieved after extended discussion"
}
```

### MsgUpdateParams

Updates Guard module parameters. Can only be sent by governance authority.

### MsgUpdateAIModel (Layer 2)

Updates the on-chain AI model weights. Must be submitted via governance proposal.

```json
{
  "authority": "cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn",
  "model_version": "linear-v1-trained",
  "weights": [35000, 10000, 5000, 20000, 5000, 5000, 3, 2000, 15000, 10000, 8000],
  "bias": 0,
  "scale": 10000,
  "feature_schema_hash": "ac76dce0...",
  "activated_height": 100000
}
```

The `feature_schema_hash` must match the current on-chain schema — this prevents deploying weights trained against a different feature set. See `ai/governance_model/v1/` for the training pipeline.

### MsgSubmitAdvisoryLink (Layer 3)

Anchors an off-chain advisory report on-chain. Can be submitted by any account.

```json
{
  "reporter": "cosmos1abc...",
  "proposal_id": 42,
  "uri": "ipfs://QmXyz.../report.json",
  "report_hash": "a1b2c3d4e5f6..."
}
```

The `report_hash` must be a 64-character hex SHA256 of the report bytes.

## Queries

### RiskReport

Get risk evaluation results for a proposal (includes both Layer 1 and Layer 2 scores):

```bash
posd query guard risk-report [proposal-id]
```

### QueuedExecution

Get execution queue state for a proposal:

```bash
posd query guard queued [proposal-id]
```

### AdvisoryLink (Layer 3)

Get advisory report link for a proposal:

```bash
posd query guard advisory-link [proposal-id]
```

### Params

Get module parameters:

```bash
posd query guard params
```

## Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `delay_low_blocks` | 17,280 (1 day) | Delay for LOW tier proposals |
| `delay_med_blocks` | 34,560 (2 days) | Delay for MED tier proposals |
| `delay_high_blocks` | 120,960 (7 days) | Delay for HIGH tier proposals |
| `delay_critical_blocks` | 241,920 (14 days) | Delay for CRITICAL tier proposals |
| `visibility_window_blocks` | 17,280 (1 day) | Duration of visibility gate |
| `shock_absorber_window_blocks` | 34,560 (2 days) | Duration of shock absorber gate |
| `threshold_default_bps` | 5000 (50%) | Voting threshold for LOW/MED |
| `threshold_high_bps` | 6667 (66.67%) | Voting threshold for HIGH |
| `threshold_critical_bps` | 7500 (75%) | Voting threshold for CRITICAL |
| `treasury_throttle_enabled` | true | Enable treasury outflow limits |
| `treasury_max_outflow_bps_per_day` | 1000 (10%) | Max treasury outflow per day |
| `enable_stability_checks` | true | Enable validator churn checks |
| `max_validator_churn_bps` | 2000 (20%) | Max allowed validator power change |
| `critical_requires_second_confirm` | true | CRITICAL proposals need confirmation |
| `critical_second_confirm_window_blocks` | 51,840 (3 days) | Confirmation window duration |
| `extension_high_blocks` | 17,280 (1 day) | Extension when HIGH tier fails checks |
| `extension_critical_blocks` | 51,840 (3 days) | Extension when CRITICAL tier fails checks |
| `ai_shadow_mode` | true | Layer 2 AI publishes scores but does not enforce |
| `binding_ai_enabled` | false | Layer 2 AI constraints merged via MaxConstraint |

## Events

| Event | Attributes | Description |
|-------|------------|-------------|
| `guard_proposal_queued` | proposal_id, tier, delay_blocks, earliest_exec_height | Proposal queued for guarded execution |
| `guard_gate_transition` | proposal_id, from, to | Gate state transition |
| `guard_execution_extended` | proposal_id, reason, extension_blocks | Execution delayed due to failed checks |
| `guard_execution_confirm_required` | proposal_id, blocks_remaining | CRITICAL proposal awaiting confirmation |
| `guard_execution_confirmed` | proposal_id, authority, justification | Confirmation received |
| `guard_proposal_executed` | proposal_id | Proposal executed successfully |
| `guard_proposal_aborted` | proposal_id, reason | Execution aborted |
| `guard_ai_model_updated` | model_version, weights_hash, activated_height | AI model weights updated |
| `guard_advisory_link_added` | proposal_id, reporter, uri, report_hash | Advisory report anchored |

## Integration

The Guard module integrates with x/gov using a **polling pattern** in EndBlocker:
1. Detects proposals that transitioned to PASSED status
2. Creates risk reports for new passed proposals
3. Queues them for guarded execution
4. Processes execution queue each block

This design avoids modifying x/gov while ensuring all proposals go through the governance firewall.

## Security Considerations

1. **Timelock bypass**: No mechanism exists to bypass delays - even governance itself must wait
2. **Protected operations**: CRITICAL proposals require explicit confirmation, preventing rushed execution
3. **Threshold enforcement**: Re-checks voting threshold at execution time (prevents gaming via vote withdrawal)
4. **Treasury protection**: Shock absorber window prevents sudden large outflows
5. **Stability gates**: Conditional execution window prevents execution during chain instability

## Future Enhancements

- [x] Layer 2 deterministic AI risk scoring (shadow + binding modes)
- [x] Layer 3 advisory link anchoring
- [ ] Trained model weights from production data (currently hand-tuned)
- [ ] Feature Schema v2 (additional features: historical voting patterns, proposer reputation)
- [ ] Validator acknowledgement tracking during visibility window
- [ ] Advanced treasury throttling with historical baseline
- [ ] Comprehensive stability metrics (validator churn, block time variance, etc.)
- [ ] Proposal dependency chains (proposal A must execute before proposal B)

## License

Copyright © 2025 Omniphi
