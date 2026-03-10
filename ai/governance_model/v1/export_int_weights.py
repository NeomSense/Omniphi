#!/usr/bin/env python3
"""
Quantize trained logistic regression weights to int32 and generate golden vectors.

Loads model_float.joblib (from train.py) and feature_schema_v1.json, then produces:
  - model_weights_int.json  (quantized model for on-chain use)
  - golden_vectors.json     (30+ deterministic test vectors for Go cross-validation)

The Go inference formula is:
    raw = bias_int + sum(weights_int[i] * features[i])
    score = clamp((raw // scale) + 50, 0, 100)

Usage:
    python export_int_weights.py
    python export_int_weights.py --model model_float.joblib --scale 10000 --seed 123
"""

import argparse
import hashlib
import json
import random
import struct
import sys
from datetime import datetime, timezone
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

# Go schema string (used for schema hash computation)
SCHEMA_STRING = (
    "FeatureSchemaV1:is_upgrade,is_param_change,is_treasury_spend,"
    "is_slashing_change,is_poc_rule_change,is_poseq_rule_change,"
    "treasury_spend_bps:0-10000,modules_touched_count:0-50,"
    "touches_consensus_critical:0-1,reduces_slashing:0-1,"
    "changes_validator_rules:0-1"
)


def compute_schema_hash_from_string() -> str:
    """Compute sha256 of the Go canonical schema string."""
    return hashlib.sha256(SCHEMA_STRING.encode()).hexdigest()


def compute_schema_hash_from_file(schema_path: Path) -> str:
    """Compute sha256 of the canonical schema JSON bytes."""
    data = json.loads(schema_path.read_text())
    canonical = data["schema_string"]
    return hashlib.sha256(canonical.encode()).hexdigest()


def compute_weights_hash(weights: list[int], bias: int, scale: int) -> str:
    """
    sha256(weights ++ bias ++ scale) using big-endian signed int32.
    Must match LinearScoringModel.ComputeWeightsHash() in Go.
    """
    h = hashlib.sha256()
    for w in weights:
        h.update(struct.pack(">i", w))
    h.update(struct.pack(">i", bias))
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


def raw_to_score(raw: int, scale: int) -> tuple[int, int]:
    """
    Convert raw to score using Go-compatible integer division.

    Go truncates toward zero; Python // truncates toward negative infinity.
    For negative raw, use: -((-raw) // scale) to match Go.

    Returns (raw_div, score).
    """
    if raw >= 0:
        raw_div = raw // scale
    else:
        raw_div = -((-raw) // scale)
    score = max(0, min(100, raw_div + 50))
    return raw_div, score


def score_to_tier(score: int) -> str:
    """Match Go ScoreToTier boundaries exactly."""
    if score <= 30:
        return "LOW"
    if score <= 55:
        return "MED"
    if score <= 80:
        return "HIGH"
    return "CRITICAL"


def compute_features_hash(features: list[int]) -> str:
    """sha256 of big-endian int32 serialization. Matches Go ComputeFeaturesHash."""
    buf = b""
    for f in features:
        buf += struct.pack(">i", f)
    return hashlib.sha256(buf).hexdigest()


# ============================================================================
# Quantization
# ============================================================================


def quantize_weights(
    float_weights: list[float], float_bias: float, scale: int
) -> tuple[list[int], int]:
    """
    Quantize float logistic regression coefficients to int32.

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

# Hard-coded must-have cases (name, feature_dict)
MUST_HAVE_CASES = [
    ("upgrade_only", {"is_upgrade": 1}),
    ("upgrade_consensus", {"is_upgrade": 1, "touches_consensus_critical": 1}),
    ("upgrade_full", {"is_upgrade": 1, "touches_consensus_critical": 1, "changes_validator_rules": 1, "modules_touched_count": 10}),
    ("consensus_critical_only", {"touches_consensus_critical": 1}),
    ("reduces_slashing_only", {"reduces_slashing": 1}),
    ("slashing_change_reduces", {"is_slashing_change": 1, "reduces_slashing": 1}),
    ("slashing_full", {"is_slashing_change": 1, "reduces_slashing": 1, "touches_consensus_critical": 1, "changes_validator_rules": 1}),
    ("treasury_spend_bps_0", {"is_treasury_spend": 1, "treasury_spend_bps": 0}),
    ("treasury_spend_bps_300", {"is_treasury_spend": 1, "treasury_spend_bps": 300}),
    ("treasury_spend_bps_600", {"is_treasury_spend": 1, "treasury_spend_bps": 600}),
    ("treasury_spend_bps_2000", {"is_treasury_spend": 1, "treasury_spend_bps": 2000}),
    ("treasury_spend_bps_10000", {"is_treasury_spend": 1, "treasury_spend_bps": 10000}),
    ("harmless_param_change", {"is_param_change": 1, "modules_touched_count": 1}),
    ("param_change_consensus", {"is_param_change": 1, "touches_consensus_critical": 1}),
    ("zero_features", {}),
    ("poc_rule_change", {"is_poc_rule_change": 1}),
    ("poseq_rule_change", {"is_poseq_rule_change": 1}),
    ("validator_rules_only", {"changes_validator_rules": 1}),
    ("max_all_features", {"is_upgrade": 1, "treasury_spend_bps": 10000, "modules_touched_count": 50, "touches_consensus_critical": 1, "reduces_slashing": 1, "changes_validator_rules": 1}),
    ("max_treasury_all_flags", {"is_treasury_spend": 1, "treasury_spend_bps": 10000, "modules_touched_count": 50, "touches_consensus_critical": 1, "reduces_slashing": 1, "changes_validator_rules": 1}),
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
        vec[8] = 1 if rng.random() < 0.2 else 0  # touches_consensus_critical
        vec[9] = 1 if rng.random() < 0.1 else 0   # reduces_slashing
        vec[10] = 1 if rng.random() < 0.15 else 0  # changes_validator_rules

        vectors.append((f"random_{i:03d}", validate_features(vec)))
    return vectors


def build_golden_vectors(
    weights: list[int],
    bias: int,
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
        raw = infer_raw(weights, bias, features)
        raw_div, score = raw_to_score(raw, scale)
        tier = score_to_tier(score)
        high_risk = 1 if tier in ("HIGH", "CRITICAL") else 0
        features_hash = compute_features_hash(features)

        vectors.append({
            "name": name,
            "features": features,
            "raw": raw,
            "raw_div": raw_div,
            "expected_score": score,
            "expected_tier": tier,
            "high_risk": high_risk,
            "features_hash": features_hash,
        })

    return vectors


# ============================================================================
# Output
# ============================================================================


def export_model_weights(
    weights: list[int],
    bias: int,
    scale: int,
    version: str,
    schema_hash: str,
    outdir: Path,
):
    weights_hash = compute_weights_hash(weights, bias, scale)

    data = {
        "model_version": version,
        "scale": scale,
        "feature_order": FEATURE_ORDER,
        "weights_int": weights,
        "bias_int": bias,
        "schema_hash_sha256": schema_hash,
        "score_mapping": "score = clamp((raw // scale) + 50, 0, 100) where raw = bias_int + sum(weights_int[i] * features[i])",
        "tier_thresholds": {
            "LOW": "0-30",
            "MED": "31-55",
            "HIGH": "56-80",
            "CRITICAL": "81-100",
        },
        "created_at": datetime.now(timezone.utc).isoformat(),
        "notes": "Quantized from trained logistic regression. Use with FeatureSchemaV1.",
        # Backward-compat fields for Go cross-validation test
        "weights": weights,
        "bias": bias,
        "feature_names": FEATURE_ORDER,
        "num_features": NUM_FEATURES,
        "feature_schema_hash": schema_hash,
        "weights_hash": weights_hash,
    }

    out_path = outdir / "model_weights_int.json"
    out_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Saved {out_path}")
    print(f"  Weights hash:  {weights_hash}")
    print(f"  Schema hash:   {schema_hash}")
    return weights_hash


def export_golden_vectors(vectors: list[dict], schema_hash: str, outdir: Path):
    data = {
        "description": "Golden test vectors for deterministic AI inference. Scores must match Go InferRiskScore exactly.",
        "inference_formula": "score = clamp((raw // scale) + 50, 0, 100)",
        "schema_hash": schema_hash,
        "num_vectors": len(vectors),
        "vectors": vectors,
    }

    out_path = outdir / "golden_vectors.json"
    out_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Saved {out_path} ({len(vectors)} vectors)")


def print_summary(
    weights: list[int],
    bias: int,
    scale: int,
    schema_hash: str,
    vectors: list[dict],
):
    print()
    print("=" * 64)
    print("SUMMARY")
    print("=" * 64)
    print(f"  Scale:       {scale}")
    print(f"  Bias:        {bias}")
    print(f"  Schema hash: {schema_hash}")
    print(f"  Weights:     {weights}")

    raw_divs = [v["raw_div"] for v in vectors]
    print(f"  raw_div range: [{min(raw_divs)}, {max(raw_divs)}]")

    tier_counts = {}
    for v in vectors:
        t = v["expected_tier"]
        tier_counts[t] = tier_counts.get(t, 0) + 1
    print(f"  Tier distribution: {tier_counts}")
    print(f"  Total vectors: {len(vectors)}")
    print()

    # Table
    print(f"{'Name':<35} {'Raw':>8} {'Div':>5} {'Score':>5} {'Tier':<8} {'HR':>2}")
    print("-" * 68)
    for v in vectors:
        print(
            f"{v['name']:<35} {v['raw']:>8} {v['raw_div']:>5} "
            f"{v['expected_score']:>5} {v['expected_tier']:<8} {v['high_risk']:>2}"
        )


# ============================================================================
# Main
# ============================================================================


def main():
    parser = argparse.ArgumentParser(
        description="Quantize model weights and generate golden vectors"
    )
    parser.add_argument(
        "--model", type=str, default="model_float.joblib",
        help="Path to joblib model from train.py",
    )
    parser.add_argument(
        "--schema", type=str, default="feature_schema_v1.json",
        help="Path to feature_schema_v1.json",
    )
    parser.add_argument("--outdir", type=str, default=".")
    parser.add_argument("--scale", type=int, default=10000)
    parser.add_argument("--seed", type=int, default=123)
    parser.add_argument("--n_random", type=int, default=20)
    parser.add_argument(
        "--version", type=str, default="omniphi-governance-l2-v1",
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
        print(f"Schema file not found, using built-in schema string")
    print(f"  Schema hash: {schema_hash}")

    # ── Load model ────────────────────────────────────────────────────
    model_path = Path(args.model)
    if not model_path.exists():
        print(f"ERROR: Model file not found: {model_path}")
        print("  Run train.py first, or provide --model path")
        return 1

    model = joblib.load(model_path)
    float_weights = model.coef_[0].tolist()
    float_bias = float(model.intercept_[0])

    if len(float_weights) != NUM_FEATURES:
        print(f"ERROR: Expected {NUM_FEATURES} weights, got {len(float_weights)}")
        return 1

    print(f"Loaded model from {model_path}")
    print(f"  Float weights: {[f'{w:.4f}' for w in float_weights]}")
    print(f"  Float bias:    {float_bias:.4f}")

    # ── Quantize ──────────────────────────────────────────────────────
    int_weights, int_bias = quantize_weights(float_weights, float_bias, scale)
    print(f"Quantized (scale={scale}):")
    print(f"  Int weights: {int_weights}")
    print(f"  Int bias:    {int_bias}")

    # Validate int32 range
    for i, w in enumerate(int_weights):
        if w < -(2**31) or w > 2**31 - 1:
            print(f"ERROR: weight[{i}]={w} exceeds int32 range")
            return 1
    if int_bias < -(2**31) or int_bias > 2**31 - 1:
        print(f"ERROR: bias={int_bias} exceeds int32 range")
        return 1

    # ── Generate golden vectors ───────────────────────────────────────
    vectors = build_golden_vectors(int_weights, int_bias, scale, args.seed, args.n_random)

    # ── Export ────────────────────────────────────────────────────────
    export_model_weights(int_weights, int_bias, scale, args.version, schema_hash, outdir)
    export_golden_vectors(vectors, schema_hash, outdir)
    print_summary(int_weights, int_bias, scale, schema_hash, vectors)

    return 0


if __name__ == "__main__":
    sys.exit(main())
