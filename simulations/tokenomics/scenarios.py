"""
Omniphi Tokenomics -- Predefined Scenarios
============================================

Each function returns a ``SimulationConfig`` with parameters tuned to explore
a specific economic or game-theoretic scenario.  All values trace back to
on-chain parameters in ``chain/x/tokenomics/types/params.go`` and the
protocol-enforced safety bounds.

Scenario categories
-------------------
**Economic stress tests** -- baseline, aggressive_burn, low_staking, max_inflation
**Game-theoretic attacks** -- whale_attack, poc_gaming, mev_extraction, validator_cartel
"""

from __future__ import annotations

import sys
from decimal import Decimal as D
from pathlib import Path
from typing import Dict

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from simulation import SimulationConfig

# ---------------------------------------------------------------------------
# Helper
# ---------------------------------------------------------------------------

def _base_config(**overrides) -> SimulationConfig:
    """Return a default config with optional overrides."""
    cfg = SimulationConfig()
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


# ============================= ECONOMIC SCENARIOS ===========================

def baseline() -> SimulationConfig:
    """
    **Baseline** -- Expected normal operating conditions.

    Assumptions
    -----------
    - 60% long-run staking ratio (strong security, moderate liquidity).
    - 5% annualised effective burn rate from organic fee activity.
    - 100 validators with roughly uniform stake distribution.
    - Moderate transaction volume growth (~0.1%/day).
    - No MEV extraction or PoC gaming.

    This is the "happy path" and serves as the reference for comparison.
    """
    return _base_config(
        target_staking_ratio=D("0.60"),
        initial_staking_ratio=D("0.55"),
        effective_burn_rate=D("0.05"),
        num_validators=100,
        initial_validator_distribution="uniform",
        daily_tx_volume_omni=D("500000"),
        tx_volume_growth_rate=D("0.001"),
        mev_extraction_rate=D("0.0"),
        poc_gaming_fraction=D("0.0"),
    )


def aggressive_burn() -> SimulationConfig:
    """
    **Aggressive Burn** -- High fee volume + high burn rates creates
    deflationary pressure.

    Assumptions
    -----------
    - 30% annualised effective burn rate (very high on-chain activity).
    - High transaction volume growing 0.3%/day.
    - Moderate staking (55%).
    - All other parameters at default.

    Key questions
    ~~~~~~~~~~~~~
    - When does supply begin to contract?
    - Does staking APY spike enough to attract more stake?
    - How fast does treasury accumulate from burn redirects?
    """
    return _base_config(
        effective_burn_rate=D("0.30"),
        target_staking_ratio=D("0.55"),
        initial_staking_ratio=D("0.50"),
        daily_tx_volume_omni=D("2000000"),
        tx_volume_growth_rate=D("0.003"),
    )


def low_staking() -> SimulationConfig:
    """
    **Low Staking** -- Only 25% of supply staked, testing security margins.

    Assumptions
    -----------
    - Target staking ratio 25% (weak PoS security).
    - Low effective burn rate (2%) -- little fee activity.
    - Validators are power-law distributed (top-heavy).

    Key questions
    ~~~~~~~~~~~~~
    - What is the cost-to-attack (33% or 67% of stake)?
    - How high does staking APY go to incentivise more staking?
    - Does the mean-reversion model push staking back up?
    """
    return _base_config(
        target_staking_ratio=D("0.25"),
        initial_staking_ratio=D("0.25"),
        effective_burn_rate=D("0.02"),
        num_validators=80,
        initial_validator_distribution="power_law",
        daily_tx_volume_omni=D("100000"),
        tx_volume_growth_rate=D("0.0005"),
    )


def max_inflation() -> SimulationConfig:
    """
    **Max Inflation** -- Year 1 conditions with zero burn activity.

    Assumptions
    -----------
    - 3% inflation (maximum allowed by protocol).
    - 0% effective burn rate (no fee activity).
    - High staking (70%) to capture all emission.
    - All inflation flows into supply with nothing removed.

    Key questions
    ~~~~~~~~~~~~~
    - How fast does supply approach the cap?
    - What is the dilution impact on non-stakers?
    - How does treasury grow from the 10% emission split alone?
    """
    return _base_config(
        effective_burn_rate=D("0.0"),
        target_staking_ratio=D("0.70"),
        initial_staking_ratio=D("0.70"),
        daily_tx_volume_omni=D("10000"),
        tx_volume_growth_rate=D("0.0"),
    )


# ========================= GAME-THEORETIC SCENARIOS =========================

def whale_attack() -> SimulationConfig:
    """
    **Whale Attack** -- A single entity controls 33% of total stake.

    This tests the network's resilience to a Tendermint BFT liveness attack
    (>1/3 stake) and governance capture potential.

    Assumptions
    -----------
    - 1 whale validator holds 33% of all staked tokens.
    - Remaining 99 validators split 67% of stake uniformly.
    - Moderate staking ratio (50%).
    - Normal burn and tx volume.

    Key questions
    ~~~~~~~~~~~~~
    - What is the economic cost for the whale to acquire and maintain 33%?
    - How does the whale's RewardMult multiplier affect total rewards?
    - What is the Gini coefficient of the resulting reward distribution?
    - Does the min commission (5%) provide meaningful whale tax?

    Protocol defences
    ~~~~~~~~~~~~~~~~~
    - MaxSingleRecipientShare = 60% caps any single emission channel.
    - MinStakingShare = 20% ensures validator set is funded.
    - RewardMult budget-neutral normalisation prevents whale bonus accumulation.
    """
    return _base_config(
        target_staking_ratio=D("0.50"),
        initial_staking_ratio=D("0.50"),
        num_validators=100,
        initial_validator_distribution="whale",
        whale_stake_pct=D("0.33"),
        effective_burn_rate=D("0.05"),
        daily_tx_volume_omni=D("500000"),
        tx_volume_growth_rate=D("0.001"),
    )


def poc_gaming() -> SimulationConfig:
    """
    **PoC Gaming** -- Validators submit low-quality contributions to farm
    rewards from the 30% PoC emission split.

    Assumptions
    -----------
    - 40% of PoC submissions are low-quality gaming attempts.
    - PoC quality floor filters some but not all (floor=0.5).
    - Normal staking and burn parameters.

    Key questions
    ~~~~~~~~~~~~~
    - How much reward leaks to gamers vs. honest contributors?
    - Does the RewardMult penalty for low-quality endorsements help?
    - What is the ROI for a gamer vs. an honest contributor?

    Protocol defences
    ~~~~~~~~~~~~~~~~~
    - PoC quality floor (MinQualityForEmission) rejects worst submissions.
    - ARVS peer review catches many gaming attempts.
    - RewardMult penalises validators who endorse low-quality work.
    """
    return _base_config(
        poc_gaming_fraction=D("0.40"),
        poc_quality_floor=D("0.5"),
        effective_burn_rate=D("0.04"),
        daily_tx_volume_omni=D("400000"),
    )


def mev_extraction() -> SimulationConfig:
    """
    **MEV Extraction** -- Sequencers front-run encrypted intents, extracting
    value from the 20% sequencer emission split and transaction flow.

    Assumptions
    -----------
    - 2% of daily tx volume extracted as MEV by colluding sequencers.
    - High tx volume (1M OMNI/day) growing 0.2%/day.
    - Normal staking and burn parameters.

    Key questions
    ~~~~~~~~~~~~~
    - What is the total value extracted per year?
    - How does MEV compare to legitimate sequencer rewards?
    - What sequencer bond is needed to make MEV unprofitable after slashing?

    Protocol defences
    ~~~~~~~~~~~~~~~~~
    - Encrypted intent mempool (reveal-commit scheme).
    - Sequencer rotation and bonding.
    - Slashing for provable front-running.
    """
    return _base_config(
        mev_extraction_rate=D("0.02"),
        daily_tx_volume_omni=D("1000000"),
        tx_volume_growth_rate=D("0.002"),
        effective_burn_rate=D("0.06"),
    )


def validator_cartel() -> SimulationConfig:
    """
    **Validator Cartel** -- 5 validators collude on PoC endorsements,
    boosting each other's RewardMult multipliers.

    Assumptions
    -----------
    - Power-law stake distribution (top 5 hold ~40% of stake).
    - Cartel members endorse each other's PoC submissions exclusively.
    - Other validators participate honestly.
    - The cartel operates within protocol bounds (no slashable offence).

    Key questions
    ~~~~~~~~~~~~~
    - How much extra reward can the cartel extract?
    - Does budget-neutral normalisation limit cartel gains?
    - What is the equilibrium if honest validators leave?
    - How does the Gini coefficient evolve?

    Protocol defences
    ~~~~~~~~~~~~~~~~~
    - RewardMult clamp [0.85, 1.15] limits deviation.
    - Budget-neutral normalisation: if cartel members get >1.0 multiplier,
      others get <1.0, but total rewards are unchanged.
    - Iterative normalisation (V2.2) prevents clamp-break budget drift.
    """
    return _base_config(
        num_validators=100,
        initial_validator_distribution="power_law",
        effective_burn_rate=D("0.05"),
        daily_tx_volume_omni=D("500000"),
        tx_volume_growth_rate=D("0.001"),
    )


# ---------------------------------------------------------------------------
# Registry
# ---------------------------------------------------------------------------

ALL_SCENARIOS: Dict[str, callable] = {
    "baseline": baseline,
    "aggressive_burn": aggressive_burn,
    "low_staking": low_staking,
    "max_inflation": max_inflation,
    "whale_attack": whale_attack,
    "poc_gaming": poc_gaming,
    "mev_extraction": mev_extraction,
    "validator_cartel": validator_cartel,
}


def list_scenarios() -> list[str]:
    """Return sorted list of available scenario names."""
    return sorted(ALL_SCENARIOS.keys())


def get_scenario(name: str) -> SimulationConfig:
    """Return the SimulationConfig for a named scenario."""
    if name not in ALL_SCENARIOS:
        raise ValueError(
            f"Unknown scenario '{name}'. Available: {list_scenarios()}"
        )
    return ALL_SCENARIOS[name]()
