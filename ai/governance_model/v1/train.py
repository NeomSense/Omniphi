#!/usr/bin/env python3
"""
Train a logistic regression model for governance proposal risk classification.

Generates synthetic training data based on Omniphi governance risk rules,
trains a scikit-learn LogisticRegression, and saves the float weights.

Usage:
    python train.py
    python train.py --rows 20000 --seed 42 --outdir .
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
    "is_upgrade",                # 0  binary
    "is_param_change",           # 1  binary
    "is_treasury_spend",         # 2  binary
    "is_slashing_change",        # 3  binary
    "is_poc_rule_change",        # 4  binary
    "is_poseq_rule_change",      # 5  binary
    "treasury_spend_bps",        # 6  numeric 0-10000
    "modules_touched_count",     # 7  numeric 0-50
    "touches_consensus_critical",# 8  binary
    "reduces_slashing",          # 9  binary
    "changes_validator_rules",   # 10 binary
]

SCHEMA_STRING = (
    "FeatureSchemaV1:is_upgrade,is_param_change,is_treasury_spend,"
    "is_slashing_change,is_poc_rule_change,is_poseq_rule_change,"
    "treasury_spend_bps:0-10000,modules_touched_count:0-50,"
    "touches_consensus_critical:0-1,reduces_slashing:0-1,"
    "changes_validator_rules:0-1"
)

NUM_FEATURES = 11


def compute_schema_hash() -> str:
    return hashlib.sha256(SCHEMA_STRING.encode()).hexdigest()


# ============================================================================
# Synthetic data generation
# ============================================================================


def generate_synthetic_data(n_rows: int, seed: int) -> pd.DataFrame:
    """
    Generate synthetic governance proposals with deterministic labels.

    Label y=1 (high risk) when ANY of:
        - is_upgrade == 1
        - touches_consensus_critical == 1
        - reduces_slashing == 1
        - treasury_spend_bps > 500
    Otherwise y=0 (low risk).
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
            # Many small spends, some large outliers
            if rng.random() < 0.6:
                row[6] = int(rng.integers(0, 500))
            else:
                row[6] = int(rng.integers(500, 10001))
        elif rng.random() < 0.05:
            row[6] = int(rng.integers(0, 100))  # noise

        # modules_touched_count (index 7) — typically 1-15, clamp to 50
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

    # Deterministic labeling
    df["y"] = (
        (df["is_upgrade"] == 1)
        | (df["touches_consensus_critical"] == 1)
        | (df["reduces_slashing"] == 1)
        | (df["treasury_spend_bps"] > 500)
    ).astype(int)

    return df


# ============================================================================
# Training
# ============================================================================


def train_and_evaluate(
    df: pd.DataFrame, seed: int
) -> tuple[LogisticRegression, float, float, np.ndarray]:
    """Train logistic regression and return model + metrics."""
    X = df[FEATURE_ORDER].values
    y = df["y"].values

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=seed, stratify=y
    )

    model = LogisticRegression(
        max_iter=1000,
        solver="lbfgs",
        class_weight="balanced",
        random_state=seed,
    )
    model.fit(X_train, y_train)

    train_acc = accuracy_score(y_train, model.predict(X_train))
    test_acc = accuracy_score(y_test, model.predict(X_test))
    cm = confusion_matrix(y_test, model.predict(X_test))

    # Print metrics
    print("=" * 50)
    print("MODEL EVALUATION")
    print("=" * 50)
    print(f"Train accuracy: {train_acc:.4f}")
    print(f"Test accuracy:  {test_acc:.4f}")
    print(f"Confusion matrix:\n{cm}")

    # False negative rate
    fn = cm[1, 0]
    tp = cm[1, 1]
    fnr = fn / (fn + tp) if (fn + tp) > 0 else 0.0
    print(f"False negative rate: {fnr:.4f}")
    if fnr > 0.10:
        print("WARNING: FNR > 10%")
    print()

    return model, train_acc, test_acc, cm


# ============================================================================
# Export
# ============================================================================


def save_outputs(
    model: LogisticRegression,
    outdir: Path,
    seed: int,
    n_rows: int,
    train_acc: float,
    test_acc: float,
    cm: np.ndarray,
):
    outdir.mkdir(parents=True, exist_ok=True)

    # joblib dump
    joblib_path = outdir / "model_float.joblib"
    joblib.dump(model, joblib_path)
    print(f"Saved {joblib_path}")

    # JSON dump
    coef = model.coef_[0].tolist()
    intercept = float(model.intercept_[0])

    data = {
        "feature_order": FEATURE_ORDER,
        "schema_hash": compute_schema_hash(),
        "coef": coef,
        "intercept": intercept,
        "seed": seed,
        "n_rows": n_rows,
        "train_accuracy": round(train_acc, 6),
        "test_accuracy": round(test_acc, 6),
        "confusion_matrix": cm.tolist(),
    }

    json_path = outdir / "model_float.json"
    json_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Saved {json_path}")
    print(f"  Schema hash: {compute_schema_hash()}")
    print(f"  Coef: {[f'{c:.4f}' for c in coef]}")
    print(f"  Intercept: {intercept:.4f}")


# ============================================================================
# Main
# ============================================================================


def main():
    parser = argparse.ArgumentParser(
        description="Train governance risk logistic regression model"
    )
    parser.add_argument("--rows", type=int, default=20000)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--outdir", type=str, default=".")
    args = parser.parse_args()

    print(f"Generating {args.rows} synthetic proposals (seed={args.seed})...")
    df = generate_synthetic_data(args.rows, args.seed)

    pos = df["y"].sum()
    neg = len(df) - pos
    print(f"  HIGH/CRIT (y=1): {pos} ({pos/len(df):.1%})")
    print(f"  LOW/MED   (y=0): {neg} ({neg/len(df):.1%})")
    print()

    model, train_acc, test_acc, cm = train_and_evaluate(df, args.seed)
    save_outputs(model, Path(args.outdir), args.seed, args.rows, train_acc, test_acc, cm)

    return 0


if __name__ == "__main__":
    sys.exit(main())
