# Governance Risk Model v1

Deterministic AI model for Omniphi governance proposal risk scoring. This model runs inside consensus on every validator node — no external APIs, no floating point, no Python at runtime.

## Architecture

```
ai/governance_model/v1/                    ← Training (Python, offline)
  train.py                                 ← Binary risk logistic regression (legacy)
  export_int_weights.py                    ← Binary quantization (legacy)
  train_ordinal.py                         ← Ordinal 4-tier classification (3 models)
  export_ordinal_int_weights.py            ← Ordinal quantization + golden vectors
  feature_schema_v1.json                   ← Canonical feature definition
  model_weights_int.json                   ← Binary quantized weights (legacy)
  golden_vectors.json                      ← Binary test vectors (legacy)
  ordinal_model_weights_int.json           ← Ordinal quantized weights (3 models)
  golden_vectors_ordinal.json              ← Ordinal test vectors (50+)

chain/x/guard/types/ai_model.go            ← On-chain inference (Go, deterministic)
chain/x/guard/keeper/ai_evaluation.go      ← Feature extraction + merge logic
```

## Feature Schema v1

11 features in canonical order:

| Index | Name | Type | Range | Description |
|-------|------|------|-------|-------------|
| 0 | is_upgrade | binary | 0-1 | Software upgrade proposal |
| 1 | is_param_change | binary | 0-1 | Parameter change proposal |
| 2 | is_treasury_spend | binary | 0-1 | Treasury/community pool spend |
| 3 | is_slashing_change | binary | 0-1 | Slashing parameter change |
| 4 | is_poc_rule_change | binary | 0-1 | Proof-of-Contribution rule change |
| 5 | is_poseq_rule_change | binary | 0-1 | Proof-of-Sequence rule change |
| 6 | treasury_spend_bps | numeric | 0-10000 | Spend as basis points (0-100%) |
| 7 | modules_touched_count | numeric | 0-50 | Modules touched (clamped) |
| 8 | touches_consensus_critical | binary | 0-1 | Touches consensus parameters |
| 9 | reduces_slashing | binary | 0-1 | Reduces slashing fractions |
| 10 | changes_validator_rules | binary | 0-1 | Changes validator rules |

**Invariant**: Feature order must never change. Schema is identified by `sha256(FeatureSchemaV1Name)`.

## Inference Formula

All integer arithmetic, no floats:

```
raw = bias + Σ(weights[i] × features[i])    // int64 accumulator
score = clamp((raw / scale) + 50, 0, 100)   // integer division truncates toward zero
```

## Tier Mapping (Legacy Binary Model)

| Score Range | Tier |
|-------------|------|
| 0-30 | LOW |
| 31-55 | MED |
| 56-80 | HIGH |
| 81-100 | CRITICAL |

## Ordinal 4-Tier Classification

The ordinal model uses 3 binary logistic regression models to classify proposals:

- **Model A (med)**: predicts `tier >= MED`
- **Model B (high)**: predicts `tier >= HIGH`
- **Model C (crit)**: predicts `tier >= CRITICAL`

Each model produces a score via the same integer formula. Tier is derived from threshold comparison:

```
if score_crit >= 75 -> CRITICAL
else if score_high >= 65 -> HIGH
else if score_med >= 55 -> MED
else LOW
```

### Synthetic Tier Rules

| Tier | Condition |
|------|-----------|
| CRITICAL | `is_upgrade==1` OR `touches_consensus_critical==1` OR `treasury_spend_bps >= 2500` |
| HIGH | `reduces_slashing==1` OR `changes_validator_rules==1` OR `500 <= treasury_spend_bps <= 2499` OR `is_poseq_rule_change==1` |
| MED | `is_param_change==1` OR `is_poc_rule_change==1` OR `is_slashing_change==1` OR (`is_treasury_spend==1` AND `1 <= treasury_spend_bps <= 499`) |
| LOW | Everything else |

## Training Pipeline

### Prerequisites

```bash
pip install numpy pandas scikit-learn joblib
```

### Legacy Binary Model

```bash
# Step 1: Train
python train.py --rows 20000 --seed 42 --outdir .

# Step 2: Quantize
python export_int_weights.py --model model_float.joblib --scale 10000 --seed 123

# Step 3: Verify
cd chain && go test ./x/guard/keeper/... -run TestLinearModel_GoldenVectors -v
```

### Ordinal 4-Tier Model

```bash
# Step 1: Train 3 ordinal models
python train_ordinal.py --rows 30000 --seed 42 --outdir .

# Step 2: Quantize + generate golden vectors
python export_ordinal_int_weights.py --model ordinal_models_float.joblib --scale 10000 --seed 123 --n_random 30

# Step 3: Verify Go inference
cd chain && go test ./x/guard/keeper/... -run TestOrdinalModel_GoldenVectors -v
```

Outputs:
- `ordinal_models_float.joblib` — 3 trained scikit-learn models + metadata
- `ordinal_models_float.json` — weights, intercepts, per-model metrics
- `ordinal_model_weights_int.json` — quantized int32 weights for on-chain use
- `golden_vectors_ordinal.json` — 50+ deterministic test vectors with expected tiers

## Model Upgrade Path

1. Train new model with updated `train.py` (new data, hyperparams, etc.)
2. Quantize with `export_int_weights.py`
3. Submit governance proposal via `MsgUpdateAIModel` with:
   - New weights (int32 slice)
   - New bias and scale
   - Feature schema hash (must match current schema)
   - Model version string
4. Proposal goes through Guard module's own risk evaluation
5. On execution, new model replaces old on-chain

**Schema changes** (adding/removing/reordering features) require a coordinated upgrade:
- New feature schema hash
- Code update to `ExtractFeatureVector()`
- New golden vectors
- Software upgrade proposal

## Constitutional Invariant

> **AI can only constrain, never relax.**

When both the rules engine (Layer 1) and AI model (Layer 2) produce results:

```
tier_final = max(tier_rules, tier_ai)
delay_final = max(delay_rules, delay_ai)
threshold_final = max(threshold_rules, threshold_ai)
```

This is enforced in `MergeRulesAndAI()` with a hard error if the invariant is violated. Even a compromised AI model cannot weaken governance protections.

## Modes

| Mode | Parameter | Behavior |
|------|-----------|----------|
| **Off** | Both false | AI not evaluated |
| **Shadow** | `ai_shadow_mode=true` | AI scores published in RiskReport but not enforced |
| **Binding** | `binding_ai_enabled=true` | AI constraints merged with rules via MaxConstraint |

Shadow mode is recommended for initial deployment to build confidence before enabling binding mode.

## Determinism Guarantees

- All arithmetic is int32/int64 — no `float32`, `float64`, or `math.LegacyDec`
- Feature extraction uses only proposal message types (deterministic on all nodes)
- Model weights are stored on-chain (same state on all validators)
- Golden vector tests ensure cross-implementation agreement
- Fuzz tests verify determinism over 1000 random inputs
