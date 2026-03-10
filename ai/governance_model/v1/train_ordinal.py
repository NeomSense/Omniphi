#!/usr/bin/env python3
"""
Train 3 ordinal logistic regression models for 4-tier governance risk classification.

Generates synthetic training data with deterministic tier labels (LOW/MED/HIGH/CRITICAL),
derives 3 binary ordinal labels (>=MED, >=HIGH, >=CRIT), and trains one
LogisticRegression per boundary.

Usage:
    python train_ordinal.py
    python train_ordinal.py --rows 30000 --seed 42 --outdir .
"""

import argparse
import hashlib
import json
import sys
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import accuracy_score, confusion_matrix
from sklearn.model_selection import train_test_split

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

SCHEMA_STRING = (
    "FeatureSchemaV1:is_upgrade,is_param_change,is_treasury_spend,"
    "is_slashing_change,is_poc_rule_change,is_poseq_rule_change,"
    "treasury_spend_bps:0-10000,modules_touched_count:0-50,"
    "touches_consensus_critical:0-1,reduces_slashing:0-1,"
    "changes_validator_rules:0-1"
)

NUM_FEATURES = 11

TIERS = ["LOW", "MED", "HIGH", "CRITICAL"]


def compute_schema_hash() -> str:
    return hashlib.sha256(SCHEMA_STRING.encode()).hexdigest()


# ============================================================================
# Synthetic data generation
# ============================================================================


def assign_tier(row: dict) -> str:
    """
    Deterministic tier assignment based on v1 rules.

    CRITICAL if:
        is_upgrade==1 OR touches_consensus_critical==1
        OR treasury_spend_bps >= 2500
    HIGH if not CRITICAL and:
        reduces_slashing==1 OR changes_validator_rules==1
        OR treasury_spend_bps between 500 and 2499 inclusive
        OR is_poseq_rule_change==1
    MED if not HIGH/CRITICAL and:
        is_param_change==1 OR is_poc_rule_change==1 OR is_slashing_change==1
        OR (is_treasury_spend==1 and treasury_spend_bps between 1 and 499)
    LOW otherwise
    """
    # CRITICAL
    if (row["is_upgrade"] == 1
            or row["touches_consensus_critical"] == 1
            or row["treasury_spend_bps"] >= 2500):
        return "CRITICAL"

    # HIGH
    if (row["reduces_slashing"] == 1
            or row["changes_validator_rules"] == 1
            or (500 <= row["treasury_spend_bps"] <= 2499)
            or row["is_poseq_rule_change"] == 1):
        return "HIGH"

    # MED
    if (row["is_param_change"] == 1
            or row["is_poc_rule_change"] == 1
            or row["is_slashing_change"] == 1
            or (row["is_treasury_spend"] == 1 and 1 <= row["treasury_spend_bps"] <= 499)):
        return "MED"

    return "LOW"


def generate_synthetic_data(n_rows: int, seed: int) -> pd.DataFrame:
    """
    Generate synthetic governance proposals with deterministic 4-tier labels.
    """
    rng = np.random.default_rng(seed)

    # Proposal type: one-hot across indices 0-5
    type_probs = [0.10, 0.30, 0.20, 0.10, 0.15, 0.15]
    proposal_types = rng.choice(6, size=n_rows, p=type_probs)

    rows = []
    for i in range(n_rows):
        ptype = proposal_types[i]
        row = [0] * NUM_FEATURES

        # One-hot proposal type (indices 0-5)
        row[ptype] = 1

        # treasury_spend_bps (index 6)
        if ptype == 2:  # is_treasury_spend
            # Spread across ranges to get all tiers
            r = rng.random()
            if r < 0.30:
                row[6] = int(rng.integers(0, 500))
            elif r < 0.60:
                row[6] = int(rng.integers(500, 2500))
            else:
                row[6] = int(rng.integers(2500, 10001))
        elif rng.random() < 0.05:
            row[6] = int(rng.integers(0, 100))  # noise

        # modules_touched_count (index 7)
        if ptype == 0:  # upgrades touch many
            row[7] = int(min(rng.integers(3, 16), 50))
        elif ptype == 1:  # param changes
            row[7] = int(min(rng.integers(1, 8), 50))
        else:
            row[7] = int(min(rng.integers(1, 6), 50))

        # touches_consensus_critical (index 8)
        if ptype == 0:
            row[8] = 1 if rng.random() < 0.85 else 0
        elif ptype == 1:
            row[8] = 1 if rng.random() < 0.20 else 0
        elif ptype == 3:
            row[8] = 1 if rng.random() < 0.30 else 0
        else:
            row[8] = 1 if rng.random() < 0.05 else 0

        # reduces_slashing (index 9)
        if ptype == 3:
            row[9] = 1 if rng.random() < 0.60 else 0
        else:
            row[9] = 1 if rng.random() < 0.02 else 0

        # changes_validator_rules (index 10)
        if ptype in (0, 3):
            row[10] = 1 if rng.random() < 0.40 else 0
        elif ptype == 1:
            row[10] = 1 if rng.random() < 0.15 else 0
        else:
            row[10] = 1 if rng.random() < 0.03 else 0

        rows.append(row)

    df = pd.DataFrame(rows, columns=FEATURE_ORDER)

    # Deterministic tier assignment
    df["tier"] = df.apply(lambda r: assign_tier(r.to_dict()), axis=1)

    # Ordinal binary labels
    tier_rank = {"LOW": 0, "MED": 1, "HIGH": 2, "CRITICAL": 3}
    df["tier_rank"] = df["tier"].map(tier_rank)
    df["y_med"] = (df["tier_rank"] >= 1).astype(int)   # tier >= MED
    df["y_high"] = (df["tier_rank"] >= 2).astype(int)   # tier >= HIGH
    df["y_crit"] = (df["tier_rank"] >= 3).astype(int)   # tier == CRITICAL

    return df


# ============================================================================
# Training
# ============================================================================


def train_model(
    X_train: np.ndarray,
    X_test: np.ndarray,
    y_train: np.ndarray,
    y_test: np.ndarray,
    name: str,
    seed: int,
) -> tuple[LogisticRegression, dict]:
    """Train one logistic regression model and return it with metrics."""
    model = LogisticRegression(
        max_iter=2000,
        solver="lbfgs",
        class_weight="balanced",
        random_state=seed,
    )
    model.fit(X_train, y_train)

    train_acc = accuracy_score(y_train, model.predict(X_train))
    test_acc = accuracy_score(y_test, model.predict(X_test))
    cm = confusion_matrix(y_test, model.predict(X_test))

    # False negative rate: FN / (FN + TP)
    fn = cm[1, 0] if cm.shape[0] > 1 else 0
    tp = cm[1, 1] if cm.shape[0] > 1 else 0
    fnr = fn / (fn + tp) if (fn + tp) > 0 else 0.0

    print(f"  Model [{name}]")
    print(f"    Train accuracy: {train_acc:.6f}")
    print(f"    Test accuracy:  {test_acc:.6f}")
    print(f"    Confusion matrix:\n      {cm.tolist()}")
    print(f"    False negative rate: {fnr:.6f}")
    if fnr > 0.10:
        print(f"    WARNING: FNR > 10%")
    print()

    metrics = {
        "train_accuracy": round(train_acc, 6),
        "test_accuracy": round(test_acc, 6),
        "confusion_matrix": cm.tolist(),
        "false_negative_rate": round(fnr, 6),
    }
    return model, metrics


def train_all_models(
    df: pd.DataFrame, seed: int
) -> tuple[dict, dict]:
    """Train 3 ordinal models and return them with metrics."""
    X = df[FEATURE_ORDER].values

    X_train, X_test, idx_train, idx_test = train_test_split(
        X, np.arange(len(df)), test_size=0.2, random_state=seed, stratify=df["tier"]
    )

    models = {}
    all_metrics = {}

    print("=" * 60)
    print("ORDINAL MODEL EVALUATION")
    print("=" * 60)

    for label_name, label_col in [("med", "y_med"), ("high", "y_high"), ("crit", "y_crit")]:
        y_train = df[label_col].values[idx_train]
        y_test = df[label_col].values[idx_test]
        model, metrics = train_model(X_train, X_test, y_train, y_test, label_name, seed)
        models[label_name] = model
        all_metrics[label_name] = metrics

    return models, all_metrics


# ============================================================================
# Export
# ============================================================================


def save_outputs(
    models: dict,
    metrics: dict,
    outdir: Path,
    seed: int,
    n_rows: int,
):
    outdir.mkdir(parents=True, exist_ok=True)

    # joblib dump — dict with all 3 models + metadata
    joblib_data = {
        "med": models["med"],
        "high": models["high"],
        "crit": models["crit"],
        "feature_order": FEATURE_ORDER,
        "seed": seed,
    }
    joblib_path = outdir / "ordinal_models_float.joblib"
    joblib.dump(joblib_data, joblib_path)
    print(f"Saved {joblib_path}")

    # JSON dump — weights, intercepts, metrics
    json_data = {
        "feature_order": FEATURE_ORDER,
        "schema_hash": compute_schema_hash(),
        "seed": seed,
        "n_rows": n_rows,
        "models": {},
    }

    for name in ["med", "high", "crit"]:
        model = models[name]
        json_data["models"][name] = {
            "coef": model.coef_[0].tolist(),
            "intercept": float(model.intercept_[0]),
            "metrics": metrics[name],
        }

    json_path = outdir / "ordinal_models_float.json"
    json_path.write_text(json.dumps(json_data, indent=2) + "\n")
    print(f"Saved {json_path}")

    # Print coefficient summary
    for name in ["med", "high", "crit"]:
        coef = models[name].coef_[0]
        intercept = float(models[name].intercept_[0])
        print(f"\n  [{name}] intercept={intercept:.4f}")
        for i, fname in enumerate(FEATURE_ORDER):
            print(f"    {fname:>30s}: {coef[i]:+.4f}")


# ============================================================================
# Main
# ============================================================================


def main():
    parser = argparse.ArgumentParser(
        description="Train 3 ordinal logistic regression models for 4-tier governance risk"
    )
    parser.add_argument("--rows", type=int, default=30000)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--outdir", type=str, default=".")
    args = parser.parse_args()

    print(f"Generating {args.rows} synthetic proposals (seed={args.seed})...")
    df = generate_synthetic_data(args.rows, args.seed)

    # Print tier distribution
    print("\nTier distribution:")
    for tier in TIERS:
        count = (df["tier"] == tier).sum()
        print(f"  {tier:>8s}: {count:>6d} ({count / len(df):.1%})")
    print()

    # Print ordinal label distribution
    print("Ordinal label distribution:")
    print(f"  y_med  (tier>=MED):  {df['y_med'].sum():>6d} ({df['y_med'].mean():.1%})")
    print(f"  y_high (tier>=HIGH): {df['y_high'].sum():>6d} ({df['y_high'].mean():.1%})")
    print(f"  y_crit (tier==CRIT): {df['y_crit'].sum():>6d} ({df['y_crit'].mean():.1%})")
    print()

    models, metrics = train_all_models(df, args.seed)
    save_outputs(models, metrics, Path(args.outdir), args.seed, args.rows)

    return 0


if __name__ == "__main__":
    sys.exit(main())
