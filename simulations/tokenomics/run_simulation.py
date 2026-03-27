#!/usr/bin/env python3
"""
Omniphi Tokenomics Simulation Runner
======================================

CLI tool to run all predefined scenarios and produce:
- JSON results in ``results/``
- CSV summaries in ``results/``
- Console summary table

Usage
-----
    python run_simulation.py                  # Run all scenarios
    python run_simulation.py baseline         # Run single scenario
    python run_simulation.py --list           # List available scenarios
    python run_simulation.py --epochs 7300    # Custom epoch count (20 years)
"""

from __future__ import annotations

import csv
import json
import os
import sys
import time
from decimal import Decimal
from pathlib import Path

# Ensure the package directory is on sys.path for relative imports.
_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from simulation import simulate_epochs, results_summary, EpochState
from scenarios import ALL_SCENARIOS, get_scenario, list_scenarios
from analysis import (
    compute_staking_apy,
    compute_real_staking_apy,
    nash_equilibrium_staking_ratio,
    gini_coefficient,
    compute_security_budget,
    compute_poc_gaming_ev,
    compute_sequencer_revenue,
    years_to_supply_cap,
)
from mev_analysis import (
    compute_mev_opportunity,
    encrypted_intent_protection_rate,
    compute_mev_risk_score,
)

RESULTS_DIR = _HERE / "results"


# ---------------------------------------------------------------------------
# Output helpers
# ---------------------------------------------------------------------------

def _ensure_results_dir():
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)


def _save_json(data: dict | list, filename: str):
    _ensure_results_dir()
    path = RESULTS_DIR / filename
    with open(path, "w") as f:
        json.dump(data, f, indent=2, default=str)
    return path


def _save_csv(epochs: list[EpochState], filename: str):
    """Save epoch-level data as CSV."""
    _ensure_results_dir()
    path = RESULTS_DIR / filename
    if not epochs:
        return path

    # Use the dict keys as column headers
    fieldnames = list(epochs[0].to_dict().keys())
    with open(path, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        for e in epochs:
            writer.writerow(e.to_dict())
    return path


def _fmt(val, decimals=2) -> str:
    """Format a Decimal or number for table display."""
    if isinstance(val, Decimal):
        return f"{float(val):,.{decimals}f}"
    if isinstance(val, (int, float)):
        return f"{val:,.{decimals}f}"
    return str(val)


def _print_separator(width=100):
    print("-" * width)


# ---------------------------------------------------------------------------
# Scenario runner
# ---------------------------------------------------------------------------

def run_scenario(name: str, num_epochs: int = 3650, verbose: bool = True) -> dict:
    """Run a single scenario and return the summary dict."""
    cfg = get_scenario(name)
    if verbose:
        print(f"\n  Running scenario: {name} ({num_epochs} epochs / {num_epochs/365:.0f} years)...")

    t0 = time.time()
    results = simulate_epochs(cfg, num_epochs)
    elapsed = time.time() - t0

    summary = results_summary(results)
    summary["scenario"] = name
    summary["computation_time_seconds"] = round(elapsed, 3)

    # Save outputs
    json_path = _save_json(summary, f"{name}_summary.json")
    csv_path = _save_csv(results, f"{name}_epochs.csv")

    # Full epoch-level JSON for baseline
    if name == "baseline":
        full_path = _save_json(
            [r.to_dict() for r in results],
            "baseline_10yr.json"
        )
        if verbose:
            print(f"    Full results:  {full_path}")

    if verbose:
        print(f"    Summary JSON:  {json_path}")
        print(f"    Epoch CSV:     {csv_path}")
        print(f"    Time: {elapsed:.2f}s")

    return summary


# ---------------------------------------------------------------------------
# Analysis runner
# ---------------------------------------------------------------------------

def run_game_theory_analysis(verbose: bool = True) -> dict:
    """Run supplementary game-theory analyses and return results."""
    analysis = {}

    # 1. Nash equilibrium staking ratio across yield environments
    if verbose:
        print("\n  Computing Nash equilibrium staking ratios...")
    eq_analysis = {}
    for alt_yield in ["0.02", "0.05", "0.08", "0.12"]:
        for burn_rate in ["0.00", "0.05", "0.10", "0.20"]:
            key = f"yield_{alt_yield}_burn_{burn_rate}"
            eq = nash_equilibrium_staking_ratio(
                inflation_rate=0.03,
                effective_burn_rate=Decimal(burn_rate),
                yield_alternatives=Decimal(alt_yield),
            )
            eq_analysis[key] = str(eq)
    analysis["nash_equilibria"] = eq_analysis

    # 2. PoC gaming analysis
    if verbose:
        print("  Computing PoC gaming equilibrium...")
    analysis["poc_gaming"] = compute_poc_gaming_ev(
        gaming_cost=0.05,          # 0.05 OMNI cost per submission
        base_reward=0.50,          # 0.50 OMNI average reward
        gaming_acceptance_rate=0.10,
        honest_acceptance_rate=0.80,
        rewardmult_penalty=0.85,   # Min RewardMult for gaming validators
    )

    # 3. Security budget at various prices
    if verbose:
        print("  Computing security budget at various price points...")
    security = {}
    for price in [0.10, 0.50, 1.00, 5.00, 10.00]:
        staked = Decimal("225000000")  # 225M OMNI staked (60% of 375M)
        security[f"price_usd_{price}"] = compute_security_budget(
            total_staked_value_usd=staked * Decimal(str(price)),
            token_price_usd=price,
            slash_rate=0.05,
            unbonding_days=21,
        )
    analysis["security_budget"] = security

    # 4. Sequencer revenue
    if verbose:
        print("  Computing sequencer revenue model...")
    analysis["sequencer_economics"] = compute_sequencer_revenue(
        daily_tx_volume=500000,
        sequencer_fee_share=0.20,
        num_sequencers=10,
        mev_extraction_rate=0.02,
    )

    # 5. MEV analysis
    if verbose:
        print("  Computing MEV risk assessment...")
    analysis["mev_opportunity"] = compute_mev_opportunity(
        daily_tx_volume=500000,
        average_spread_bps=50,
    )
    analysis["intent_protection"] = encrypted_intent_protection_rate(
        encryption_scheme="threshold",
        num_threshold_parties=10,
        corruption_threshold=4,
    )
    analysis["mev_risk_score"] = compute_mev_risk_score(
        daily_tx_volume=500000,
        num_sequencers=10,
        stake_per_sequencer=50000,
        slash_rate=0.10,
    )

    # 6. Years to supply cap
    if verbose:
        print("  Computing years to supply cap...")
    for burn in ["0.00", "0.05", "0.10", "0.20"]:
        yrs = years_to_supply_cap(
            current_supply=375000000,
            effective_burn_rate=Decimal(burn),
        )
        analysis[f"years_to_cap_burn_{burn}"] = yrs if yrs else "never"

    return analysis


# ---------------------------------------------------------------------------
# Console output
# ---------------------------------------------------------------------------

def print_summary_table(summaries: list[dict]):
    """Print a formatted comparison table of scenario summaries."""
    print("\n")
    print("=" * 110)
    print("  OMNIPHI TOKENOMICS SIMULATION RESULTS -- 10-YEAR PROJECTION")
    print("=" * 110)

    # Header
    headers = [
        "Scenario",
        "Final Supply",
        "Total Burned",
        "Treasury",
        "Stake %",
        "APY %",
        "Net Infl %",
        "Cap %",
        "Gini",
    ]
    fmt = "  {:<20} {:>14} {:>14} {:>12} {:>8} {:>8} {:>10} {:>8} {:>6}"
    print(fmt.format(*headers))
    _print_separator(110)

    for s in summaries:
        def _short(val_str, div=1_000_000):
            """Convert to M (millions) for display."""
            try:
                v = float(Decimal(val_str))
                if abs(v) >= 1e9:
                    return f"{v/1e9:.2f}B"
                return f"{v/1e6:.1f}M"
            except Exception:
                return val_str

        row = [
            s.get("scenario", "?")[:20],
            _short(s.get("final_supply", "0")),
            _short(s.get("total_burned", "0")),
            _short(s.get("final_treasury", "0")),
            _fmt(Decimal(s.get("final_staking_ratio", "0")) * 100, 1),
            _fmt(Decimal(s.get("final_staking_apy", "0")), 1),
            _fmt(Decimal(s.get("final_net_inflation", "0")), 2),
            _fmt(Decimal(s.get("supply_pct_of_cap", "0")), 1),
            _fmt(Decimal(s.get("final_gini", "0")), 3),
        ]
        print(fmt.format(*row))

    _print_separator(110)
    print()


def print_game_theory_summary(analysis: dict):
    """Print key game-theory findings."""
    print("=" * 80)
    print("  GAME-THEORETIC ANALYSIS HIGHLIGHTS")
    print("=" * 80)

    # Nash equilibrium
    print("\n  Nash Equilibrium Staking Ratios (inflation=3%):")
    print("  " + "-" * 60)
    print(f"  {'Alt Yield':<12} {'Burn=0%':>10} {'Burn=5%':>10} {'Burn=10%':>10} {'Burn=20%':>10}")
    eq = analysis.get("nash_equilibria", {})
    for alt in ["0.02", "0.05", "0.08", "0.12"]:
        row = [f"  {float(Decimal(alt))*100:.0f}%".ljust(12)]
        for burn in ["0.00", "0.05", "0.10", "0.20"]:
            key = f"yield_{alt}_burn_{burn}"
            val = eq.get(key, "?")
            row.append(f"{float(Decimal(val))*100:.1f}%".rjust(10))
        print("".join(row))

    # PoC gaming
    poc = analysis.get("poc_gaming", {})
    print(f"\n  PoC Gaming Analysis:")
    print(f"    Honest EV:  {poc.get('honest_ev', '?')} OMNI  (ROI: {poc.get('honest_roi_pct', '?')}%)")
    print(f"    Gaming EV:  {poc.get('gaming_ev', '?')} OMNI  (ROI: {poc.get('gaming_roi_pct', '?')}%)")
    print(f"    Honest dominates: {poc.get('honest_dominates', '?')}")

    # MEV risk
    mev = analysis.get("mev_risk_score", {})
    print(f"\n  MEV Risk Assessment:")
    print(f"    Composite Score: {mev.get('composite_risk_score', '?')}/100")
    print(f"    Risk Level:      {mev.get('risk_level', '?')}")
    print(f"    Recommendation:  {mev.get('recommendation', '?')}")

    # Supply cap timing
    print(f"\n  Years to Supply Cap (from genesis):")
    for burn in ["0.00", "0.05", "0.10", "0.20"]:
        key = f"years_to_cap_burn_{burn}"
        val = analysis.get(key, "?")
        print(f"    Burn {float(Decimal(burn))*100:.0f}%: {val}")

    print()


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    args = sys.argv[1:]

    # Parse flags
    num_epochs = 3650
    scenario_filter = None

    i = 0
    while i < len(args):
        if args[i] == "--list":
            print("Available scenarios:")
            for name in list_scenarios():
                print(f"  - {name}")
            return
        elif args[i] == "--epochs" and i + 1 < len(args):
            num_epochs = int(args[i + 1])
            i += 2
            continue
        elif args[i] == "--help" or args[i] == "-h":
            print(__doc__)
            return
        elif not args[i].startswith("-"):
            scenario_filter = args[i]
        i += 1

    print("=" * 80)
    print("  OMNIPHI TOKENOMICS SIMULATION SUITE")
    print("  Epochs: {} ({:.1f} years)".format(num_epochs, num_epochs / 365))
    print("=" * 80)

    # Determine which scenarios to run
    if scenario_filter:
        scenarios_to_run = [scenario_filter]
    else:
        scenarios_to_run = list(ALL_SCENARIOS.keys())

    # Run simulations
    summaries = []
    t_total = time.time()

    for name in scenarios_to_run:
        try:
            summary = run_scenario(name, num_epochs)
            summaries.append(summary)
        except Exception as e:
            print(f"  ERROR running {name}: {e}")
            import traceback
            traceback.print_exc()

    # Print comparison table
    if summaries:
        print_summary_table(summaries)

    # Run game-theory analysis
    analysis = run_game_theory_analysis()
    analysis_path = _save_json(analysis, "game_theory_analysis.json")
    print(f"  Game theory analysis saved: {analysis_path}")
    print_game_theory_summary(analysis)

    elapsed_total = time.time() - t_total
    print(f"  Total computation time: {elapsed_total:.2f}s")
    print(f"  Results directory: {RESULTS_DIR}")


if __name__ == "__main__":
    main()
