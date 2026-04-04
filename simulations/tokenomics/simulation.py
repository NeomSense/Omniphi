"""
Omniphi Tokenomics Simulation Engine
=====================================

A mathematically rigorous, deterministic simulation of the Omniphi blockchain's
token economics over a configurable time horizon.  Every formula is derived
directly from the on-chain Go implementation in:

    chain/x/tokenomics/keeper/inflation_decay.go   -- inflation schedule
    chain/x/tokenomics/keeper/mint.go              -- minting + supply cap
    chain/x/tokenomics/keeper/burn.go              -- burn + treasury redirect
    chain/x/tokenomics/types/params.go             -- default parameters
    chain/x/rewardmult/keeper/invariants.go        -- EMA smoothing, budget-neutral invariant
    chain/x/feemarket/keeper/unified_burn.go       -- single-pass fee burn model

All arithmetic uses Python's built-in ``Decimal`` for 18-digit precision,
matching Cosmos SDK ``LegacyDec``.

No external dependencies beyond the Python standard library.
"""

from __future__ import annotations

import copy
import json
import math
from dataclasses import dataclass, field, asdict
from decimal import Decimal, getcontext, ROUND_DOWN
from typing import List, Dict, Optional

# Match Cosmos SDK LegacyDec precision (18 decimal places).
getcontext().prec = 36

# ---------------------------------------------------------------------------
# Utility helpers
# ---------------------------------------------------------------------------

D = Decimal  # shorthand


def _dec(v) -> Decimal:
    """Coerce int / float / str to Decimal deterministically."""
    if isinstance(v, Decimal):
        return v
    return Decimal(str(v))


def _clamp(value: Decimal, lo: Decimal, hi: Decimal) -> Decimal:
    return max(lo, min(hi, value))


# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

@dataclass
class SimulationConfig:
    """
    Complete parameterisation of the Omniphi token economy.

    Default values are taken verbatim from ``chain/x/tokenomics/types/params.go``
    ``DefaultParams()`` and the inflation schedule in
    ``chain/x/tokenomics/keeper/inflation_decay.go``.

    Units
    -----
    All token quantities are in **whole OMNI** (not micro-OMNI / omniphi).
    The on-chain representation uses 6 decimal places; we abstract that away
    because the simulation concerns macro-economic dynamics, not on-chain
    integer encoding.
    """

    # --- Supply ------------------------------------------------------------
    total_supply_cap: Decimal = D("1500000000")
    """Hard cap of 1.5 B OMNI.  Protocol-enforced and immutable."""

    genesis_supply: Decimal = D("375000000")
    """375 M OMNI minted at genesis (25% of cap)."""

    # --- Inflation schedule ------------------------------------------------
    # Year-indexed rates from inflation_decay.go CalculateDecayingInflation.
    # Year 0 = 3%, Year 1 = 2.75%, ..., Year 5 = 1.75%, then -0.25%/yr to floor.
    inflation_schedule: Dict[int, Decimal] = field(default_factory=lambda: {
        0: D("0.03"),
        1: D("0.0275"),
        2: D("0.025"),
        3: D("0.0225"),
        4: D("0.02"),
        5: D("0.0175"),
    })
    """Explicit year -> rate for years 0-5.  Year 6+ decays linearly."""

    inflation_decay_per_year: Decimal = D("0.0025")
    """0.25 pp per year reduction after year 5."""

    inflation_floor: Decimal = D("0.005")
    """Minimum inflation rate (0.5%).  From params InflationMin."""

    inflation_ceiling: Decimal = D("0.03")
    """Protocol hard cap (3%).  From MaxAnnualInflationRateHardCap."""

    # --- Emission split ----------------------------------------------------
    emission_split_staking: Decimal = D("0.40")
    """40% of new emissions to staking rewards."""

    emission_split_poc: Decimal = D("0.30")
    """30% to Proof-of-Contribution rewards."""

    emission_split_sequencer: Decimal = D("0.20")
    """20% to sequencer incentives."""

    emission_split_treasury: Decimal = D("0.10")
    """10% to protocol treasury."""

    # --- Burn parameters ---------------------------------------------------
    fee_burn_ratio: Decimal = D("0.90")
    """90% of transaction fees are burned.  From DefaultParams() FeeBurnRatio."""

    treasury_fee_ratio: Decimal = D("0.10")
    """10% of transaction fees go to treasury.  From DefaultParams() TreasuryFeeRatio."""

    treasury_burn_redirect: Decimal = D("0.10")
    """10% of burn-source amounts redirected to treasury before burning."""

    # Activity-specific burn rates (from DefaultParams).
    burn_rate_pos_gas: Decimal = D("0.20")
    burn_rate_poc_anchoring: Decimal = D("0.25")
    burn_rate_sequencer_gas: Decimal = D("0.15")
    burn_rate_smart_contracts: Decimal = D("0.12")
    burn_rate_ai_queries: Decimal = D("0.10")
    burn_rate_messaging: Decimal = D("0.08")

    # Effective average burn rate used in the simulation's simplified
    # fee-volume model.  Weighted by estimated activity mix.
    effective_burn_rate: Decimal = D("0.05")
    """
    Fraction of *circulating supply* burned per year through fees.
    This is a scenario-level knob.  It combines activity volume and
    per-activity burn rates into a single annual aggregate.

    Baseline assumption: modest activity producing ~5% of circulating supply
    in total fees, of which 90% is burned => ~4.5% net supply reduction from
    fees.  We round to 5% for the default scenario.
    """

    # --- Staking dynamics --------------------------------------------------
    target_staking_ratio: Decimal = D("0.60")
    """Long-run fraction of circulating supply that is staked."""

    staking_ratio_adjustment_speed: Decimal = D("0.05")
    """Per-epoch mean-reversion speed toward target staking ratio."""

    initial_staking_ratio: Decimal = D("0.55")
    """Fraction of genesis supply initially staked."""

    # --- Validator parameters ----------------------------------------------
    max_validators: int = 125
    min_commission: Decimal = D("0.05")
    unbonding_days: int = 21

    # --- RewardMult parameters ---------------------------------------------
    rewardmult_min: Decimal = D("0.85")
    rewardmult_max: Decimal = D("1.15")
    rewardmult_governance_min: Decimal = D("0.50")
    rewardmult_governance_max: Decimal = D("2.00")
    rewardmult_ema_alpha: Decimal = D("0.2222")
    """EMA smoothing alpha = 2/(N+1) with N=8 epochs."""

    # --- Simulation parameters ---------------------------------------------
    epochs_per_year: int = 365
    """1 epoch = 1 day (matching on-chain EndBlocker cadence)."""

    # --- Validator set composition (for game-theory scenarios) --------------
    num_validators: int = 100
    """Number of active validators in the simulation."""

    initial_validator_distribution: str = "uniform"
    """
    How stake is distributed across validators at genesis.
    Options: 'uniform', 'power_law', 'whale' (single entity with whale_stake_pct).
    """

    whale_stake_pct: Decimal = D("0.0")
    """Fraction of total stake held by a single whale entity (for whale scenario)."""

    # --- PoC parameters ----------------------------------------------------
    poc_quality_floor: Decimal = D("0.5")
    """Minimum quality score for PoC contributions to earn rewards."""

    poc_gaming_fraction: Decimal = D("0.0")
    """Fraction of PoC submissions that are low-quality gaming attempts."""

    # --- MEV / Sequencer parameters ----------------------------------------
    daily_tx_volume_omni: Decimal = D("500000")
    """Daily transaction volume in OMNI (fee-generating activity)."""

    tx_volume_growth_rate: Decimal = D("0.001")
    """Daily compounding growth rate of transaction volume."""

    mev_extraction_rate: Decimal = D("0.0")
    """Fraction of tx volume extracted as MEV by sequencers."""

    def get_inflation_rate(self, year: int) -> Decimal:
        """
        Return the annual inflation rate for the given year index.

        Mirrors ``CalculateDecayingInflation`` in
        ``chain/x/tokenomics/keeper/inflation_decay.go``.

        Parameters
        ----------
        year : int
            0-indexed year since genesis.

        Returns
        -------
        Decimal
            The inflation rate for that year, clamped to [floor, ceiling].
        """
        if year in self.inflation_schedule:
            rate = self.inflation_schedule[year]
        else:
            # Year 6+: linear decay from 1.75%
            base = D("0.0175")
            years_after_six = year - 5
            rate = base - self.inflation_decay_per_year * years_after_six
        return _clamp(rate, self.inflation_floor, self.inflation_ceiling)


# ---------------------------------------------------------------------------
# State objects
# ---------------------------------------------------------------------------

@dataclass
class ValidatorState:
    """Per-validator economic state within an epoch."""
    index: int
    stake: Decimal = D("0")
    multiplier: Decimal = D("1.0")
    """Current RewardMult effective multiplier."""
    multiplier_raw: Decimal = D("1.0")
    multiplier_ema: Decimal = D("1.0")
    poc_score: Decimal = D("0")
    rewards_earned: Decimal = D("0")
    commission_rate: Decimal = D("0.05")
    is_whale: bool = False

    def stake_weight(self, total_staked: Decimal) -> Decimal:
        if total_staked == 0:
            return D("0")
        return self.stake / total_staked


@dataclass
class EpochState:
    """Network-wide economic snapshot at the end of an epoch."""
    epoch: int = 0
    year: int = 0

    # --- Supply accounting -------------------------------------------------
    total_supply: Decimal = D("0")
    """current_total_supply = total_minted - total_burned."""
    total_minted: Decimal = D("0")
    total_burned: Decimal = D("0")
    circulating: Decimal = D("0")
    """circulating = total_supply - staked - treasury."""
    staked: Decimal = D("0")
    treasury: Decimal = D("0")

    # --- Inflation ---------------------------------------------------------
    inflation_rate: Decimal = D("0")
    annual_provisions: Decimal = D("0")
    epoch_mint: Decimal = D("0")

    # --- Emission distribution ---------------------------------------------
    epoch_staking_emission: Decimal = D("0")
    epoch_poc_emission: Decimal = D("0")
    epoch_sequencer_emission: Decimal = D("0")
    epoch_treasury_emission: Decimal = D("0")

    # --- Burns -------------------------------------------------------------
    epoch_burn: Decimal = D("0")
    cumulative_burn: Decimal = D("0")

    # --- Derived metrics ---------------------------------------------------
    staking_ratio: Decimal = D("0")
    staking_apy: Decimal = D("0")
    net_inflation: Decimal = D("0")
    """Effective supply growth rate (minting - burning) / supply."""
    supply_pct_of_cap: Decimal = D("0")

    # --- MEV ---------------------------------------------------------------
    daily_tx_volume: Decimal = D("0")
    mev_extracted: Decimal = D("0")

    # --- Validator distribution --------------------------------------------
    gini_coefficient: Decimal = D("0")
    top5_stake_pct: Decimal = D("0")
    validator_count: int = 0

    def to_dict(self) -> dict:
        """Serialise to a JSON-compatible dict (Decimal -> str)."""
        d = {}
        for k, v in asdict(self).items():
            d[k] = str(v) if isinstance(v, Decimal) else v
        return d


# ---------------------------------------------------------------------------
# Simulation engine
# ---------------------------------------------------------------------------

def _init_validators(cfg: SimulationConfig, total_staked: Decimal) -> List[ValidatorState]:
    """Create the initial validator set according to the configured distribution."""
    n = cfg.num_validators
    validators = []

    if cfg.initial_validator_distribution == "whale" and cfg.whale_stake_pct > 0:
        whale_stake = total_staked * cfg.whale_stake_pct
        remaining = total_staked - whale_stake
        per_validator = remaining / max(n - 1, 1)
        for i in range(n):
            if i == 0:
                validators.append(ValidatorState(
                    index=i,
                    stake=whale_stake,
                    is_whale=True,
                    commission_rate=cfg.min_commission,
                ))
            else:
                validators.append(ValidatorState(
                    index=i,
                    stake=per_validator,
                    commission_rate=cfg.min_commission,
                ))
    elif cfg.initial_validator_distribution == "power_law":
        # Zipf-like: stake_i proportional to 1/rank
        ranks = [D(1) / D(i + 1) for i in range(n)]
        total_rank = sum(ranks)
        for i in range(n):
            validators.append(ValidatorState(
                index=i,
                stake=total_staked * (ranks[i] / total_rank),
                commission_rate=cfg.min_commission,
            ))
    else:
        # Uniform
        per_validator = total_staked / n
        for i in range(n):
            validators.append(ValidatorState(
                index=i,
                stake=per_validator,
                commission_rate=cfg.min_commission,
            ))

    return validators


def _compute_gini(stakes: List[Decimal]) -> Decimal:
    """
    Compute the Gini coefficient of a list of stake values.

    The Gini coefficient measures inequality in reward/stake distribution.
    G = 0 means perfect equality; G = 1 means total inequality.

    Formula (unbiased, for a sample):
        G = (2 * sum(i * x_i) - (n+1) * sum(x_i)) / (n * sum(x_i))

    where x_i are sorted ascending.
    """
    if not stakes or all(s == 0 for s in stakes):
        return D("0")
    sorted_stakes = sorted(stakes)
    n = len(sorted_stakes)
    total = sum(sorted_stakes)
    if total == 0:
        return D("0")
    numerator = D("0")
    for i, s in enumerate(sorted_stakes):
        numerator += D(2 * (i + 1) - n - 1) * s
    return numerator / (D(n) * total)


def _apply_rewardmult_ema(
    current_raw: Decimal,
    previous_ema: Decimal,
    alpha: Decimal,
) -> Decimal:
    """
    EMA smoothing as implemented in the RewardMult module.

    Formula: M_EMA(t) = alpha * M_raw(t) + (1 - alpha) * M_EMA(t-1)

    With alpha = 2/(N+1), N=8 => alpha ~= 0.2222.

    Reference: chain/x/rewardmult V2.2 EMA smoothing.
    """
    return alpha * current_raw + (D("1") - alpha) * previous_ema


def _normalize_multipliers(
    validators: List[ValidatorState],
    total_staked: Decimal,
    min_mult: Decimal,
    max_mult: Decimal,
    max_iterations: int = 3,
) -> None:
    """
    Iterative budget-neutral normalisation of RewardMult multipliers.

    After clamping multipliers to [min, max], the stake-weighted sum
    may deviate from the total stake (budget neutrality).  This procedure
    iteratively adjusts unclamped multipliers to restore the invariant:

        sum(stake_i * M_eff_i) = sum(stake_i)

    Matches chain/x/rewardmult V2.2 iterative normalisation (max 3 rounds).

    Reference: chain/x/rewardmult/keeper/invariants.go BudgetNeutralInvariant.
    """
    if total_staked == 0:
        return

    for _ in range(max_iterations):
        # Compute current weighted sum
        weighted_sum = sum(v.stake * v.multiplier for v in validators)
        target = total_staked  # budget-neutral target

        if weighted_sum == 0:
            return

        # Scale factor to restore neutrality
        scale = target / weighted_sum

        # Apply scale, then re-clamp
        any_clamped = False
        for v in validators:
            new_m = v.multiplier * scale
            clamped = _clamp(new_m, min_mult, max_mult)
            if clamped != new_m:
                any_clamped = True
            v.multiplier = clamped

        if not any_clamped:
            break


def simulate_epochs(
    cfg: SimulationConfig,
    num_epochs: int = 3650,
) -> List[EpochState]:
    """
    Run the full tokenomics simulation for ``num_epochs`` epochs.

    This is the primary entry point.  It models:

    1. **Inflation decay** per the on-chain year-based schedule.
    2. **Emission distribution** across staking, PoC, sequencer, and treasury.
    3. **Fee-based burn mechanics** using the effective burn rate.
    4. **Treasury accumulation** from emission splits and burn redirects.
    5. **Supply cap enforcement** -- minting stops when supply = cap.
    6. **Staking ratio dynamics** -- mean-reversion toward target.
    7. **RewardMult EMA smoothing** and budget-neutral normalisation.
    8. **Validator reward distribution** with Gini tracking.
    9. **Transaction volume growth** and MEV extraction modelling.

    Parameters
    ----------
    cfg : SimulationConfig
        All economic parameters.
    num_epochs : int
        Number of daily epochs to simulate.  Default 3650 (10 years).

    Returns
    -------
    List[EpochState]
        One ``EpochState`` snapshot per epoch.
    """
    epochs_per_year = cfg.epochs_per_year

    # Initialise supply state
    total_supply = _dec(cfg.genesis_supply)
    total_minted = _dec(cfg.genesis_supply)
    total_burned = D("0")
    staked = total_supply * cfg.initial_staking_ratio
    treasury = D("0")

    # Initialise validators
    validators = _init_validators(cfg, staked)

    # Transaction volume
    daily_tx_vol = _dec(cfg.daily_tx_volume_omni)

    results: List[EpochState] = []

    for epoch in range(num_epochs):
        year = epoch // epochs_per_year

        # ---------------------------------------------------------------
        # 1. INFLATION
        # ---------------------------------------------------------------
        inflation_rate = cfg.get_inflation_rate(year)
        annual_provisions = total_supply * inflation_rate
        epoch_mint = annual_provisions / epochs_per_year

        # Supply cap enforcement (mirrors MintInflation in inflation_decay.go)
        remaining_mintable = cfg.total_supply_cap - total_supply
        if remaining_mintable <= 0:
            epoch_mint = D("0")
        elif epoch_mint > remaining_mintable:
            epoch_mint = remaining_mintable

        # ---------------------------------------------------------------
        # 2. EMISSION DISTRIBUTION
        # ---------------------------------------------------------------
        staking_emission = epoch_mint * cfg.emission_split_staking
        poc_emission = epoch_mint * cfg.emission_split_poc
        sequencer_emission = epoch_mint * cfg.emission_split_sequencer
        treasury_emission = epoch_mint * cfg.emission_split_treasury

        # Rounding remainder goes to treasury (matches DistributeEmissions)
        distributed = staking_emission + poc_emission + sequencer_emission + treasury_emission
        if distributed < epoch_mint:
            treasury_emission += epoch_mint - distributed

        # ---------------------------------------------------------------
        # 3. BURNS
        # ---------------------------------------------------------------
        # Simplified aggregate model:
        #   burn_base = circulating * (effective_burn_rate / epochs_per_year)
        # Where effective_burn_rate combines fee volume and per-activity rates.
        circulating = total_supply - staked - treasury
        if circulating < 0:
            circulating = D("0")

        epoch_burn_base = circulating * (cfg.effective_burn_rate / epochs_per_year)

        # Treasury redirect on burns (10% default)
        burn_to_treasury = epoch_burn_base * cfg.treasury_burn_redirect
        epoch_burn = epoch_burn_base - burn_to_treasury

        # Cannot burn more than exists
        if epoch_burn > total_supply:
            epoch_burn = total_supply
            burn_to_treasury = D("0")

        # ---------------------------------------------------------------
        # 4. SUPPLY ACCOUNTING
        # ---------------------------------------------------------------
        total_supply = total_supply + epoch_mint - epoch_burn
        total_minted += epoch_mint
        total_burned += epoch_burn

        # Conservation law: total_supply = total_minted - total_burned
        # (Matches params.go Validate: current = minted - burned)

        # Treasury accumulation
        treasury += treasury_emission + burn_to_treasury

        # ---------------------------------------------------------------
        # 5. STAKING DYNAMICS
        # ---------------------------------------------------------------
        # Mean-reversion toward target staking ratio.
        current_staking_ratio = staked / total_supply if total_supply > 0 else D("0")
        ratio_gap = cfg.target_staking_ratio - current_staking_ratio
        staking_adjustment = ratio_gap * cfg.staking_ratio_adjustment_speed

        # New staked amount includes staking emissions + adjustment flow
        staked = staked + staking_emission + (total_supply * staking_adjustment / epochs_per_year)
        # Cannot stake more than total supply minus treasury
        max_stakeable = total_supply - treasury
        if staked > max_stakeable:
            staked = max_stakeable
        if staked < 0:
            staked = D("0")

        # Distribute staking emission across validators proportionally
        total_val_stake = sum(v.stake for v in validators)
        if total_val_stake > 0:
            scale = staked / total_val_stake
            for v in validators:
                v.stake = v.stake * scale
                reward_share = staking_emission * v.stake_weight(staked)
                v.rewards_earned += reward_share

        # ---------------------------------------------------------------
        # 6. REWARDMULT (EMA + normalisation)
        # ---------------------------------------------------------------
        # Simulate small perturbations in raw multiplier based on uptime/PoC
        for v in validators:
            # Slight randomness-free model: validators near 1.0 with small
            # drift based on their index (deterministic).
            # In reality this depends on uptime, slashing, PoC participation.
            poc_bonus = D("0.01") * (D("1") - cfg.poc_gaming_fraction)
            uptime_bonus = D("0.005")  # assume good uptime baseline
            slash_penalty = D("0")

            v.multiplier_raw = D("1") + poc_bonus + uptime_bonus - slash_penalty
            v.multiplier_ema = _apply_rewardmult_ema(
                v.multiplier_raw, v.multiplier_ema, cfg.rewardmult_ema_alpha
            )
            v.multiplier = _clamp(v.multiplier_ema, cfg.rewardmult_min, cfg.rewardmult_max)

        # Budget-neutral normalisation (V2.2)
        _normalize_multipliers(
            validators, staked,
            cfg.rewardmult_min, cfg.rewardmult_max,
        )

        # ---------------------------------------------------------------
        # 7. TRANSACTION VOLUME & MEV
        # ---------------------------------------------------------------
        daily_tx_vol = daily_tx_vol * (D("1") + cfg.tx_volume_growth_rate)
        mev_extracted = daily_tx_vol * cfg.mev_extraction_rate

        # ---------------------------------------------------------------
        # 8. DERIVED METRICS
        # ---------------------------------------------------------------
        circulating = total_supply - staked - treasury
        if circulating < 0:
            circulating = D("0")

        staking_ratio = staked / total_supply if total_supply > 0 else D("0")

        # Staking APY = (annual staking emissions / staked) * 100
        annual_staking_emission = annual_provisions * cfg.emission_split_staking
        staking_apy = (annual_staking_emission / staked * D("100")) if staked > 0 else D("0")

        net_inflation = (
            (epoch_mint - epoch_burn) * epochs_per_year / total_supply * D("100")
            if total_supply > 0 else D("0")
        )

        supply_pct_of_cap = total_supply / cfg.total_supply_cap * D("100") if cfg.total_supply_cap > 0 else D("0")

        stakes = [v.stake for v in validators]
        gini = _compute_gini(stakes)
        sorted_stakes = sorted(stakes, reverse=True)
        top5 = (sum(sorted_stakes[:5]) / staked * D("100")) if staked > 0 else D("0")

        # ---------------------------------------------------------------
        # 9. RECORD EPOCH STATE
        # ---------------------------------------------------------------
        state = EpochState(
            epoch=epoch,
            year=year,
            total_supply=total_supply,
            total_minted=total_minted,
            total_burned=total_burned,
            circulating=circulating,
            staked=staked,
            treasury=treasury,
            inflation_rate=inflation_rate,
            annual_provisions=annual_provisions,
            epoch_mint=epoch_mint,
            epoch_staking_emission=staking_emission,
            epoch_poc_emission=poc_emission,
            epoch_sequencer_emission=sequencer_emission,
            epoch_treasury_emission=treasury_emission,
            epoch_burn=epoch_burn,
            cumulative_burn=total_burned,
            staking_ratio=staking_ratio,
            staking_apy=staking_apy,
            net_inflation=net_inflation,
            supply_pct_of_cap=supply_pct_of_cap,
            daily_tx_volume=daily_tx_vol,
            mev_extracted=mev_extracted,
            gini_coefficient=gini,
            top5_stake_pct=top5,
            validator_count=len(validators),
        )
        results.append(state)

    return results


# ---------------------------------------------------------------------------
# JSON serialisation helper
# ---------------------------------------------------------------------------

def results_to_json(results: List[EpochState]) -> str:
    """Serialise simulation results to a JSON string."""
    return json.dumps(
        [r.to_dict() for r in results],
        indent=2,
    )


def results_summary(results: List[EpochState]) -> dict:
    """
    Produce a compact summary dict suitable for reporting.

    Includes start/end snapshots and key aggregates.
    """
    if not results:
        return {}
    first = results[0]
    last = results[-1]
    yearly_snapshots = [r for r in results if r.epoch % 365 == 364]
    return {
        "simulation_epochs": len(results),
        "simulation_years": len(results) / 365,
        "genesis_supply": str(first.total_supply),
        "final_supply": str(last.total_supply),
        "total_minted": str(last.total_minted),
        "total_burned": str(last.total_burned),
        "final_treasury": str(last.treasury),
        "final_staking_ratio": str(last.staking_ratio),
        "final_staking_apy": str(last.staking_apy),
        "final_inflation_rate": str(last.inflation_rate),
        "final_net_inflation": str(last.net_inflation),
        "supply_pct_of_cap": str(last.supply_pct_of_cap),
        "final_gini": str(last.gini_coefficient),
        "yearly_snapshots": [
            {
                "year": s.year + 1,
                "supply": str(s.total_supply),
                "inflation_rate": str(s.inflation_rate),
                "staking_apy": str(s.staking_apy),
                "staking_ratio": str(s.staking_ratio),
                "cumulative_burn": str(s.cumulative_burn),
                "treasury": str(s.treasury),
                "net_inflation_pct": str(s.net_inflation),
            }
            for s in yearly_snapshots
        ],
    }
