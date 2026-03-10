#!/usr/bin/env python3
"""
Quantize 3 ordinal logistic regression models to int32 and generate golden vectors.

Loads ordinal_models_float.joblib (from train_ordinal.py) and feature_schema_v1.json,
then produces:
  - ordinal_model_weights_int.json  (quantized models for on-chain use)
  - golden_vectors_ordinal.json     (50+ deterministic test vectors for Go cross-validation)

Each model produces a score via the same formula:
    raw = bias_int + sum(weights_int[i] * features[i])
    score = clamp((raw // scale) + 50, 0, 100)

Tier derivation from 3 scores:
    if score_crit >= t3_crit_score -> CRITICAL
    else if score_high >= t2_high_score -> HIGH
    else if score_med >= t1_med_score -> MED
    else LOW

Usage:
    python export_ordinal_int_weights.py
    python export_ordinal_int_weights.py --model ordinal_models_float.joblib --scale 10000 --seed 123
"""

import argparse
import hashlib
import json
import random
import struct
import sys
from pathlib import Path

import joblib
import numpy as np

# ============================================================================
# Feature schema — must match FeatureSchemaV1 in x/guard/types/ai_model.go
# ============================================================================

FEATURE_ORDER = [
    "is_upgrade",                 # 0  binary
    "is_param_change",            # 1  binary
    "is_treasury_spend",          # 2  binary
    "is_slashing_change",         # 3  binary
    "is_poc_rule_change",         # 4  binary
    "is_poseq_rule_change",       # 5  binary
    "treasury_spend_bps",         # 6  numeric 0-10000
    "modules_touched_count",      # 7  numeric 0-50
    "touches_consensus_critical", # 8  binary
    "reduces_slashing",           # 9  binary
    "changes_validator_rules",    # 10 binary
]

NUM_FEATURES = 11

# Allowed ranges per feature index
FEATURE_RANGES = {
    0: (0, 1),
    1: (0, 1),
    2: (0, 1),
    3: (0, 1),
    4: (0, 1),
    5: (0, 1),
    6: (0, 10000),
    7: (0, 50),
    8: (0, 1),
    9: (0, 1),
    10: (0, 1),
}

SCHEMA_STRING = (
    "FeatureSchemaV1:is_upgrade,is_param_change,is_treasury_spend,"
    "is_slashing_change,is_poc_rule_change,is_poseq_rule_change,"
    "treasury_spend_bps:0-10000,modules_touched_count:0-50,"
    "touches_consensus_critical:0-1,reduces_slashing:0-1,"
    "changes_validator_rules:0-1"
)

# Ordinal tier thresholds (score boundaries)
T1_MED_SCORE = 55
T2_HIGH_SCORE = 65
T3_CRIT_SCORE = 75


def compute_schema_hash_from_string() -> str:
    return hashlib.sha256(SCHEMA_STRING.encode()).hexdigest()


def compute_schema_hash_from_file(schema_path: Path) -> str:
    data = json.loads(schema_path.read_text())
    canonical = data["schema_string"]
    return hashlib.sha256(canonical.encode()).hexdigest()


def compute_weights_hash(
    weights_med: list[int], bias_med: int,
    weights_high: list[int], bias_high: int,
    weights_crit: list[int], bias_crit: int,
    scale: int,
) -> str:
    """
    sha256 of all 3 models' weights + biases + scale, using big-endian signed int32.
    """
    h = hashlib.sha256()
    for w in weights_med:
        h.update(struct.pack(">i", w))
    h.update(struct.pack(">i", bias_med))
    for w in weights_high:
        h.update(struct.pack(">i", w))
    h.update(struct.pack(">i", bias_high))
    for w in weights_crit:
        h.update(struct.pack(">i", w))
    h.update(struct.pack(">i", bias_crit))
    h.update(struct.pack(">i", scale))
    return h.hexdigest()


# ============================================================================
# Deterministic inference (must match Go InferRiskScore exactly)
# ============================================================================


def infer_raw(weights: list[int], bias: int, features: list[int]) -> int:
    """Compute raw = bias + sum(w[i] * x[i])."""
    raw = bias
    for w, f in zip(weights, features):
        raw += w * f
    return raw


def raw_to_score(raw: int, scale: int) -> int:
    """
    Convert raw to score using Go-compatible integer division.

    Go truncates toward zero; Python // truncates toward negative infinity.
    For negative raw, use: -((-raw) // scale) to match Go.
    """
    if raw >= 0:
        raw_div = raw // scale
    else:
        raw_div = -((-raw) // scale)
    return max(0, min(100, raw_div + 50))


def derive_tier(score_med: int, score_high: int, score_crit: int) -> str:
    """Derive tier from 3 ordinal scores using thresholds."""
    if score_crit >= T3_CRIT_SCORE:
        return "CRITICAL"
    if score_high >= T2_HIGH_SCORE:
        return "HIGH"
    if score_med >= T1_MED_SCORE:
        return "MED"
    return "LOW"


# ============================================================================
# Quantization
# ============================================================================


def quantize_weights(
    float_weights: list[float], float_bias: float, scale: int
) -> tuple[list[int], int]:
    """
    w_int[i] = round(w_float[i] * scale)
    b_int    = round(b_float * scale)
    """
    int_weights = [int(round(w * scale)) for w in float_weights]
    int_bias = int(round(float_bias * scale))
    return int_weights, int_bias


# ============================================================================
# Validation
# ============================================================================


def validate_features(features: list[int]) -> list[int]:
    """Validate and clamp feature vector to allowed ranges."""
    if len(features) != NUM_FEATURES:
        raise ValueError(f"Expected {NUM_FEATURES} features, got {len(features)}")
    clamped = []
    for i, f in enumerate(features):
        lo, hi = FEATURE_RANGES[i]
        clamped.append(max(lo, min(hi, int(f))))
    return clamped


# ============================================================================
# Golden vector generation
# ============================================================================

# Must-have cases that exercise specific tier rules
MUST_HAVE_CASES = [
    # CRITICAL cases
    ("upgrade_only", {"is_upgrade": 1, "modules_touched_count": 5}),
    ("upgrade_consensus", {"is_upgrade": 1, "touches_consensus_critical": 1, "modules_touched_count": 8}),
    ("consensus_critical_only", {"touches_consensus_critical": 1}),
    ("treasury_spend_3000_crit", {"is_treasury_spend": 1, "treasury_spend_bps": 3000}),
    ("treasury_spend_5000_crit", {"is_treasury_spend": 1, "treasury_spend_bps": 5000}),
    ("treasury_spend_10000_crit", {"is_treasury_spend": 1, "treasury_spend_bps": 10000}),
    ("upgrade_full_crit", {"is_upgrade": 1, "touches_consensus_critical": 1, "changes_validator_rules": 1, "modules_touched_count": 10}),

    # HIGH cases
    ("reduces_slashing_high", {"is_slashing_change": 1, "reduces_slashing": 1}),
    ("changes_validator_rules_high", {"changes_validator_rules": 1}),
    ("treasury_spend_600_high", {"is_treasury_spend": 1, "treasury_spend_bps": 600}),
    ("treasury_spend_1500_high", {"is_treasury_spend": 1, "treasury_spend_bps": 1500}),
    ("treasury_spend_2499_high", {"is_treasury_spend": 1, "treasury_spend_bps": 2499}),
    ("poseq_rule_change_high", {"is_poseq_rule_change": 1}),
    ("slashing_full_high", {"is_slashing_change": 1, "reduces_slashing": 1, "changes_validator_rules": 1}),

    # MED cases
    ("param_change_med", {"is_param_change": 1, "modules_touched_count": 1}),
    ("poc_rule_change_med", {"is_poc_rule_change": 1}),
    ("slashing_change_no_reduce_med", {"is_slashing_change": 1}),
    ("treasury_spend_100_med", {"is_treasury_spend": 1, "treasury_spend_bps": 100}),
    ("treasury_spend_499_med", {"is_treasury_spend": 1, "treasury_spend_bps": 499}),

    # LOW cases
    ("harmless_zero", {}),
    ("treasury_spend_0_low", {"is_treasury_spend": 1, "treasury_spend_bps": 0}),
]


def features_dict_to_vector(d: dict) -> list[int]:
    """Convert {feature_name: value} to canonical 11-element vector."""
    name_to_idx = {name: i for i, name in enumerate(FEATURE_ORDER)}
    vec = [0] * NUM_FEATURES
    for name, val in d.items():
        if name not in name_to_idx:
            raise ValueError(f"Unknown feature: {name}")
        vec[name_to_idx[name]] = int(val)
    return vec


def generate_random_vectors(n: int, seed: int) -> list[tuple[str, list[int]]]:
    """Generate n random valid feature vectors with a fixed seed."""
    rng = random.Random(seed)
    vectors = []
    for i in range(n):
        vec = [0] * NUM_FEATURES

        # Pick at most one proposal type (indices 0-5)
        type_idx = rng.randint(0, 6)  # 6 = no type set
        if type_idx < 6:
            vec[type_idx] = 1

        # treasury_spend_bps
        if vec[2] == 1:  # is_treasury_spend
            vec[6] = rng.randint(0, 10000)
        elif rng.random() < 0.1:
            vec[6] = rng.randint(0, 200)

        # modules_touched_count
        vec[7] = rng.randint(0, 15)

        # modifier flags
        vec[8] = 1 if rng.random() < 0.2 else 0   # touches_consensus_critical
        vec[9] = 1 if rng.random() < 0.1 else 0    # reduces_slashing
        vec[10] = 1 if rng.random() < 0.15 else 0   # changes_validator_rules

        vectors.append((f"random_{i:03d}", validate_features(vec)))
    return vectors


def build_golden_vectors(
    w_med: list[int], b_med: int,
    w_high: list[int], b_high: int,
    w_crit: list[int], b_crit: int,
    scale: int,
    seed: int,
    n_random: int,
) -> list[dict]:
    """Build all golden vectors: must-haves + random."""
    all_cases: list[tuple[str, list[int]]] = []

    # Must-have cases
    for name, feat_dict in MUST_HAVE_CASES:
        vec = validate_features(features_dict_to_vector(feat_dict))
        all_cases.append((name, vec))

    # Random cases
    all_cases.extend(generate_random_vectors(n_random, seed))

    vectors = []
    for name, features in all_cases:
        raw_med = infer_raw(w_med, b_med, features)
        score_med = raw_to_score(raw_med, scale)

        raw_high = infer_raw(w_high, b_high, features)
        score_high = raw_to_score(raw_high, scale)

        raw_crit = infer_raw(w_crit, b_crit, features)
        score_crit = raw_to_score(raw_crit, scale)

        tier = derive_tier(score_med, score_high, score_crit)

        vectors.append({
            "name": name,
            "features": features,
            "raw_med": raw_med,
            "score_med": score_med,
            "raw_high": raw_high,
            "score_high": score_high,
            "raw_crit": raw_crit,
            "score_crit": score_crit,
            "tier_expected": tier,
        })

    return vectors


# ============================================================================
# Output
# ============================================================================


def export_model_weights(
    w_med: list[int], b_med: int,
    w_high: list[int], b_high: int,
    w_crit: list[int], b_crit: int,
    scale: int,
    version: str,
    schema_hash: str,
    outdir: Path,
) -> str:
    weights_hash = compute_weights_hash(
        w_med, b_med, w_high, b_high, w_crit, b_crit, scale
    )

    data = {
        "model_version": version,
        "scale": scale,
        "feature_order": FEATURE_ORDER,
        "schema_hash_sha256": schema_hash,
        "weights_int_med": w_med,
        "bias_int_med": b_med,
        "weights_int_high": w_high,
        "bias_int_high": b_high,
        "weights_int_crit": w_crit,
        "bias_int_crit": b_crit,
        "thresholds": {
            "t1_med_score": T1_MED_SCORE,
            "t2_high_score": T2_HIGH_SCORE,
            "t3_crit_score": T3_CRIT_SCORE,
        },
        "score_mapping": "score = clamp((raw // scale) + 50, 0, 100) where raw = bias_int + sum(weights_int[i] * features[i])",
        "tier_derivation": "if score_crit >= t3 -> CRITICAL; else if score_high >= t2 -> HIGH; else if score_med >= t1 -> MED; else LOW",
        "weights_hash": weights_hash,
    }

    out_path = outdir / "ordinal_model_weights_int.json"
    out_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Saved {out_path}")
    print(f"  Weights hash:  {weights_hash}")
    print(f"  Schema hash:   {schema_hash}")
    return weights_hash


def export_golden_vectors(vectors: list[dict], schema_hash: str, outdir: Path):
    data = {
        "description": "Golden test vectors for ordinal 4-tier AI inference. All 3 scores must match Go exactly.",
        "inference_formula": "score = clamp((raw // scale) + 50, 0, 100)",
        "tier_derivation": "if score_crit >= 75 -> CRITICAL; else if score_high >= 65 -> HIGH; else if score_med >= 55 -> MED; else LOW",
        "schema_hash": schema_hash,
        "num_vectors": len(vectors),
        "vectors": vectors,
    }

    out_path = outdir / "golden_vectors_ordinal.json"
    out_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Saved {out_path} ({len(vectors)} vectors)")


def print_summary(
    w_med: list[int], b_med: int,
    w_high: list[int], b_high: int,
    w_crit: list[int], b_crit: int,
    scale: int,
    vectors: list[dict],
):
    print()
    print("=" * 80)
    print("SUMMARY")
    print("=" * 80)
    print(f"  Scale: {scale}")
    print(f"  Thresholds: t1_med={T1_MED_SCORE}, t2_high={T2_HIGH_SCORE}, t3_crit={T3_CRIT_SCORE}")
    print()

    # Tier distribution
    tier_counts = {}
    for v in vectors:
        t = v["tier_expected"]
        tier_counts[t] = tier_counts.get(t, 0) + 1
    print(f"  Tier distribution in golden vectors: {tier_counts}")
    print(f"  Total vectors: {len(vectors)}")
    print()

    # Table header
    fmt = "{:<30s} {:>8s} {:>6s} {:>8s} {:>6s} {:>8s} {:>6s} {:>10s}"
    print(fmt.format("Name", "raw_med", "s_med", "raw_hi", "s_hi", "raw_cr", "s_cr", "tier"))
    print("-" * 88)

    for v in vectors:
        print(fmt.format(
            v["name"],
            str(v["raw_med"]),
            str(v["score_med"]),
            str(v["raw_high"]),
            str(v["score_high"]),
            str(v["raw_crit"]),
            str(v["score_crit"]),
            v["tier_expected"],
        ))


# ============================================================================
# Main
# ============================================================================


def main():
    parser = argparse.ArgumentParser(
        description="Quantize 3 ordinal models and generate golden vectors"
    )
    parser.add_argument(
        "--model", type=str, default="ordinal_models_float.joblib",
        help="Path to joblib model from train_ordinal.py",
    )
    parser.add_argument(
        "--schema", type=str, default="feature_schema_v1.json",
        help="Path to feature_schema_v1.json",
    )
    parser.add_argument("--outdir", type=str, default=".")
    parser.add_argument("--scale", type=int, default=10000)
    parser.add_argument("--seed", type=int, default=123)
    parser.add_argument("--n_random", type=int, default=30)
    parser.add_argument(
        "--version", type=str, default="omniphi-ordinal-l2-v1",
        help="Model version string",
    )
    args = parser.parse_args()

    outdir = Path(args.outdir)
    outdir.mkdir(parents=True, exist_ok=True)
    scale = args.scale

    # ── Load schema hash ──────────────────────────────────────────────
    schema_path = Path(args.schema)
    if schema_path.exists():
        schema_hash = compute_schema_hash_from_file(schema_path)
        print(f"Loaded schema from {schema_path}")
    else:
        schema_hash = compute_schema_hash_from_string()
        print("Schema file not found, using built-in schema string")
    print(f"  Schema hash: {schema_hash}")

    # ── Load models ───────────────────────────────────────────────────
    model_path = Path(args.model)
    if not model_path.exists():
        print(f"ERROR: Model file not found: {model_path}")
        print("  Run train_ordinal.py first, or provide --model path")
        return 1

    bundle = joblib.load(model_path)
    model_med = bundle["med"]
    model_high = bundle["high"]
    model_crit = bundle["crit"]

    print(f"Loaded 3 ordinal models from {model_path}")

    # ── Quantize each model ──────────────────────────────────────────
    models_info = {}
    for name, model in [("med", model_med), ("high", model_high), ("crit", model_crit)]:
        float_weights = model.coef_[0].tolist()
        float_bias = float(model.intercept_[0])

        if len(float_weights) != NUM_FEATURES:
            print(f"ERROR: [{name}] Expected {NUM_FEATURES} weights, got {len(float_weights)}")
            return 1

        int_weights, int_bias = quantize_weights(float_weights, float_bias, scale)

        # Validate int32 range
        for i, w in enumerate(int_weights):
            if w < -(2**31) or w > 2**31 - 1:
                print(f"ERROR: [{name}] weight[{i}]={w} exceeds int32 range")
                return 1
        if int_bias < -(2**31) or int_bias > 2**31 - 1:
            print(f"ERROR: [{name}] bias={int_bias} exceeds int32 range")
            return 1

        models_info[name] = {
            "float_weights": float_weights,
            "float_bias": float_bias,
            "int_weights": int_weights,
            "int_bias": int_bias,
        }

        print(f"\n  [{name}] Quantized (scale={scale}):")
        print(f"    Int weights: {int_weights}")
        print(f"    Int bias:    {int_bias}")

    w_med = models_info["med"]["int_weights"]
    b_med = models_info["med"]["int_bias"]
    w_high = models_info["high"]["int_weights"]
    b_high = models_info["high"]["int_bias"]
    w_crit = models_info["crit"]["int_weights"]
    b_crit = models_info["crit"]["int_bias"]

    # ── Generate golden vectors ───────────────────────────────────────
    vectors = build_golden_vectors(
        w_med, b_med, w_high, b_high, w_crit, b_crit,
        scale, args.seed, args.n_random,
    )

    # ── Validate tier distribution ────────────────────────────────────
    tier_counts = {}
    for v in vectors:
        t = v["tier_expected"]
        tier_counts[t] = tier_counts.get(t, 0) + 1
    print(f"\nGolden vector tier distribution: {tier_counts}")

    # ── Export ────────────────────────────────────────────────────────
    export_model_weights(
        w_med, b_med, w_high, b_high, w_crit, b_crit,
        scale, args.version, schema_hash, outdir,
    )
    export_golden_vectors(vectors, schema_hash, outdir)
    print_summary(
        w_med, b_med, w_high, b_high, w_crit, b_crit,
        scale, vectors,
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
