#!/usr/bin/env python3
"""
Generate pre-computed baseline results.
Run this standalone: python generate_baseline.py
"""
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from simulation import simulate_epochs, results_summary
from scenarios import baseline

def main():
    import json
    cfg = baseline()
    results = simulate_epochs(cfg, num_epochs=3650)
    summary = results_summary(results)
    summary["scenario"] = "baseline"

    out_dir = _HERE / "results"
    out_dir.mkdir(parents=True, exist_ok=True)

    # Full epoch data
    from simulation import EpochState
    full = [r.to_dict() for r in results]
    with open(out_dir / "baseline_10yr.json", "w") as f:
        json.dump(full, f, indent=2, default=str)

    # Summary
    with open(out_dir / "baseline_summary.json", "w") as f:
        json.dump(summary, f, indent=2, default=str)

    print(f"Wrote {len(results)} epochs to results/baseline_10yr.json")
    print(f"Summary: results/baseline_summary.json")

    # Print key metrics
    last = results[-1]
    print(f"\n=== 10-Year Baseline Summary ===")
    print(f"Final supply:       {float(last.total_supply):,.0f} OMNI")
    print(f"Total minted:       {float(last.total_minted):,.0f} OMNI")
    print(f"Total burned:       {float(last.total_burned):,.0f} OMNI")
    print(f"Treasury:           {float(last.treasury):,.0f} OMNI")
    print(f"Staked:             {float(last.staked):,.0f} OMNI")
    print(f"Staking ratio:      {float(last.staking_ratio)*100:.1f}%")
    print(f"Staking APY:        {float(last.staking_apy):.2f}%")
    print(f"Inflation rate:     {float(last.inflation_rate)*100:.2f}%")
    print(f"Net inflation:      {float(last.net_inflation):.2f}%")
    print(f"Supply % of cap:    {float(last.supply_pct_of_cap):.1f}%")
    print(f"Gini coefficient:   {float(last.gini_coefficient):.4f}")

if __name__ == "__main__":
    main()
