"""
Omniphi Tokenomics -- Game-Theoretic Analysis
===============================================

This module provides analytical functions for evaluating the economic security,
fairness, and incentive compatibility of the Omniphi token economy.

Every function includes a docstring explaining the underlying game-theoretic
reasoning and the formula derivation.

References
----------
- Chain params: ``chain/x/tokenomics/types/params.go``
- RewardMult:   ``chain/x/rewardmult/keeper/invariants.go``
- Burn model:   ``chain/x/feemarket/keeper/unified_burn.go``
- Inflation:    ``chain/x/tokenomics/keeper/inflation_decay.go``
"""

from __future__ import annotations

import math
import sys
from decimal import Decimal, getcontext
from pathlib import Path
from typing import List, Optional, Tuple

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

getcontext().prec = 36

D = Decimal


def _dec(v) -> Decimal:
    if isinstance(v, Decimal):
        return v
    return Decimal(str(v))


# ---------------------------------------------------------------------------
# 1. Staking Economics
# ---------------------------------------------------------------------------

def compute_staking_apy(
    inflation_rate: float | Decimal,
    staking_ratio: float | Decimal,
    emission_split_staking: float | Decimal = 0.40,
    effective_burn_rate: float | Decimal = 0.05,
) -> Decimal:
    """
    Compute the annualised percentage yield (APY) for stakers.

    Derivation
    ----------
    Stakers receive ``emission_split_staking`` (default 40%) of annual
    inflation.  Their APY depends on how much of the supply is staked:

        APY = (inflation_rate * emission_split_staking) / staking_ratio * 100

    This is a *nominal* APY.  The *real* APY adjusts for supply dilution:

        real_APY = nominal_APY - (inflation_rate * 100) + (effective_burn_rate * 100)

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    - As staking_ratio decreases, APY increases, attracting more stake.
    - As staking_ratio increases, APY decreases, some stake moves to DeFi.
    - This creates a natural **Nash equilibrium** where APY equals the
      opportunity cost of capital (external yield).

    Parameters
    ----------
    inflation_rate : Decimal
        Current annual inflation rate (e.g., 0.03 for 3%).
    staking_ratio : Decimal
        Fraction of total supply that is staked (e.g., 0.60).
    emission_split_staking : Decimal
        Fraction of emissions going to stakers (default 0.40).
    effective_burn_rate : Decimal
        Annual burn rate as fraction of circulating supply.

    Returns
    -------
    Decimal
        Nominal staking APY as a percentage (e.g., 2.0 means 2.0%).
    """
    inf = _dec(inflation_rate)
    sr = _dec(staking_ratio)
    split = _dec(emission_split_staking)

    if sr == 0:
        return D("Infinity")

    nominal_apy = inf * split / sr * D("100")
    return nominal_apy


def compute_real_staking_apy(
    inflation_rate: float | Decimal,
    staking_ratio: float | Decimal,
    emission_split_staking: float | Decimal = 0.40,
    effective_burn_rate: float | Decimal = 0.05,
) -> Decimal:
    """
    Real staking APY adjusted for dilution.

    Formula
    -------
    real_APY = nominal_APY - net_inflation_rate * 100

    Where net_inflation = inflation_rate - effective_burn_rate (if positive).
    """
    nominal = compute_staking_apy(
        inflation_rate, staking_ratio, emission_split_staking
    )
    net_inf = (_dec(inflation_rate) - _dec(effective_burn_rate))
    if net_inf < 0:
        net_inf = D("0")
    return nominal - net_inf * D("100")


# ---------------------------------------------------------------------------
# 2. Proof-of-Contribution Economics
# ---------------------------------------------------------------------------

def compute_poc_roi(
    contribution_cost: float | Decimal,
    expected_reward: float | Decimal,
    quality_acceptance_rate: float | Decimal = 0.80,
    rewardmult_factor: float | Decimal = 1.0,
) -> Decimal:
    """
    Compute the expected ROI for a PoC contributor.

    Derivation
    ----------
    A contributor invests ``contribution_cost`` (time, compute, fees) to
    submit work.  The expected reward depends on:

    1. **Acceptance rate** -- probability the contribution passes peer review.
    2. **RewardMult factor** -- the endorsing validators' average multiplier.
    3. **Base reward** from the PoC emission pool.

    Expected value:

        EV = expected_reward * quality_acceptance_rate * rewardmult_factor
        ROI = (EV - contribution_cost) / contribution_cost * 100

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    - Positive ROI attracts more contributors (increases supply of work).
    - Negative ROI deters low-quality gaming (costs exceed rewards).
    - The quality_acceptance_rate creates a natural filter: gaming attempts
      have lower acceptance and thus lower EV.

    Parameters
    ----------
    contribution_cost : Decimal
        Total cost to produce and submit the contribution (in OMNI).
    expected_reward : Decimal
        Base reward if the contribution is accepted (in OMNI).
    quality_acceptance_rate : Decimal
        Probability of passing quality review (0.0 to 1.0).
    rewardmult_factor : Decimal
        Average RewardMult of endorsing validators (default 1.0).

    Returns
    -------
    Decimal
        ROI as a percentage. Negative means unprofitable.
    """
    cost = _dec(contribution_cost)
    reward = _dec(expected_reward)
    accept = _dec(quality_acceptance_rate)
    mult = _dec(rewardmult_factor)

    if cost == 0:
        return D("Infinity") if reward > 0 else D("0")

    ev = reward * accept * mult
    roi = (ev - cost) / cost * D("100")
    return roi


def compute_poc_gaming_ev(
    gaming_cost: float | Decimal,
    base_reward: float | Decimal,
    gaming_acceptance_rate: float | Decimal = 0.10,
    honest_acceptance_rate: float | Decimal = 0.80,
    rewardmult_penalty: float | Decimal = 0.85,
) -> dict:
    """
    Compare expected value of gaming vs. honest PoC contribution.

    Returns a dict with EVs and ROIs for both strategies, plus the
    honesty premium (how much better honest behaviour is).

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    For a Nash equilibrium where honesty dominates, we need:

        EV(honest) > EV(gaming) for all rational actors

    The protocol achieves this through:
    - Quality floor filtering (low acceptance rate for gaming)
    - RewardMult penalty for validators who endorse low-quality work
    - Submission fees that make high-volume gaming costly
    """
    honest_roi = compute_poc_roi(gaming_cost, base_reward, honest_acceptance_rate, D("1.0"))
    gaming_roi = compute_poc_roi(gaming_cost, base_reward, gaming_acceptance_rate, rewardmult_penalty)

    honest_ev = _dec(base_reward) * _dec(honest_acceptance_rate) * D("1.0")
    gaming_ev = _dec(base_reward) * _dec(gaming_acceptance_rate) * _dec(rewardmult_penalty)

    return {
        "honest_ev": str(honest_ev),
        "gaming_ev": str(gaming_ev),
        "honest_roi_pct": str(honest_roi),
        "gaming_roi_pct": str(gaming_roi),
        "honesty_premium_pct": str(honest_roi - gaming_roi),
        "gaming_is_profitable": gaming_roi > 0,
        "honest_dominates": honest_roi > gaming_roi,
    }


# ---------------------------------------------------------------------------
# 3. Sequencer Economics
# ---------------------------------------------------------------------------

def compute_sequencer_revenue(
    daily_tx_volume: float | Decimal,
    sequencer_fee_share: float | Decimal = 0.20,
    num_sequencers: int = 10,
    mev_extraction_rate: float | Decimal = 0.0,
) -> dict:
    """
    Compute sequencer economics including legitimate and MEV revenue.

    Derivation
    ----------
    Sequencers earn from two sources:

    1. **Emission share**: 20% of block emissions (from emission_split_sequencer).
    2. **Fee share**: A fraction of transaction fees after burns.
    3. **MEV (if extracting)**: Front-running value from transaction ordering.

    Per-sequencer daily revenue:

        legitimate_daily = daily_tx_volume * sequencer_fee_share / num_sequencers
        mev_daily = daily_tx_volume * mev_extraction_rate / num_sequencers
        total_daily = legitimate_daily + mev_daily

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    If MEV revenue is high relative to legitimate revenue, sequencers are
    incentivised to extract MEV.  The protocol must make MEV extraction
    unprofitable through:
    - Encrypted intent mempool (reduces MEV opportunity)
    - Bonding requirements (slashable if caught)
    - Rotation (limits sustained extraction)

    Parameters
    ----------
    daily_tx_volume : Decimal
        Total daily transaction volume in OMNI.
    sequencer_fee_share : Decimal
        Fraction of fees allocated to sequencers.
    num_sequencers : int
        Number of active sequencers sharing rewards.
    mev_extraction_rate : Decimal
        Fraction of tx volume extractable as MEV (0 if no MEV).

    Returns
    -------
    dict
        Revenue breakdown per sequencer.
    """
    vol = _dec(daily_tx_volume)
    share = _dec(sequencer_fee_share)
    mev_rate = _dec(mev_extraction_rate)
    n = D(str(num_sequencers))

    legit_daily = vol * share / n
    mev_daily = vol * mev_rate / n
    total_daily = legit_daily + mev_daily

    return {
        "per_sequencer_daily_legitimate": str(legit_daily),
        "per_sequencer_daily_mev": str(mev_daily),
        "per_sequencer_daily_total": str(total_daily),
        "per_sequencer_annual_legitimate": str(legit_daily * D("365")),
        "per_sequencer_annual_mev": str(mev_daily * D("365")),
        "per_sequencer_annual_total": str(total_daily * D("365")),
        "total_daily_mev_extracted": str(vol * mev_rate),
        "mev_as_pct_of_legitimate": str(
            (mev_daily / legit_daily * D("100")) if legit_daily > 0 else D("0")
        ),
    }


# ---------------------------------------------------------------------------
# 4. Security Budget Analysis
# ---------------------------------------------------------------------------

def compute_security_budget(
    total_staked_value_usd: float | Decimal,
    token_price_usd: float | Decimal,
    slash_rate: float | Decimal = 0.05,
    unbonding_days: int = 21,
) -> dict:
    """
    Compute the economic cost to attack the network.

    Derivation
    ----------
    In Tendermint BFT consensus:
    - **1/3 attack** (liveness): Attacker needs >33.3% of stake to halt the chain.
    - **2/3 attack** (safety): Attacker needs >66.7% of stake to produce
      conflicting blocks.

    The economic security budget is the cost an attacker must bear:

        cost_to_acquire = fraction_needed * total_staked_value
        cost_of_slashing = cost_to_acquire * slash_rate
        opportunity_cost = cost_to_acquire * staking_apy * (unbonding_days / 365)
        total_attack_cost = cost_to_acquire + cost_of_slashing + opportunity_cost

    In practice, acquiring a large stake moves the price (market impact),
    making the actual cost significantly higher.  We report the lower bound.

    Parameters
    ----------
    total_staked_value_usd : Decimal
        Total value of all staked tokens in USD.
    token_price_usd : Decimal
        Current token price in USD.
    slash_rate : Decimal
        Fraction of stake slashed on detection (e.g., 0.05 = 5%).
    unbonding_days : int
        Unbonding period in days.

    Returns
    -------
    dict
        Attack cost analysis for 1/3 and 2/3 thresholds.
    """
    staked = _dec(total_staked_value_usd)
    price = _dec(token_price_usd)
    slash = _dec(slash_rate)
    unbond = D(str(unbonding_days))

    results = {}
    for label, fraction in [("one_third", D("0.334")), ("two_thirds", D("0.667"))]:
        cost_to_acquire = staked * fraction
        tokens_needed = cost_to_acquire / price if price > 0 else D("0")
        slash_cost = cost_to_acquire * slash
        # Conservative 5% opportunity cost per year
        opp_cost = cost_to_acquire * D("0.05") * unbond / D("365")
        total = cost_to_acquire + slash_cost + opp_cost

        results[label] = {
            "stake_fraction_needed": str(fraction),
            "cost_to_acquire_usd": str(cost_to_acquire),
            "tokens_needed": str(tokens_needed),
            "slashing_loss_usd": str(slash_cost),
            "opportunity_cost_usd": str(opp_cost),
            "total_attack_cost_usd": str(total),
            "unbonding_lockup_days": unbonding_days,
        }

    return results


def compute_cost_to_halt(
    staked_omni: float | Decimal,
    token_price_usd: float | Decimal,
    staking_ratio: float | Decimal = 0.60,
    slash_rate: float | Decimal = 0.05,
) -> Decimal:
    """
    Quick estimate: USD cost to halt the chain (>1/3 of stake).

    This is the headline "security budget" metric for investor presentations.

    Formula: cost = staked_omni * (1/3) * token_price * (1 + slash_rate)
    """
    s = _dec(staked_omni)
    p = _dec(token_price_usd)
    sr = _dec(slash_rate)
    return s * D("0.334") * p * (D("1") + sr)


# ---------------------------------------------------------------------------
# 5. Nash Equilibrium Staking Ratio
# ---------------------------------------------------------------------------

def nash_equilibrium_staking_ratio(
    inflation_rate: float | Decimal,
    effective_burn_rate: float | Decimal = 0.05,
    yield_alternatives: float | Decimal = 0.05,
    emission_split_staking: float | Decimal = 0.40,
) -> Decimal:
    """
    Compute the Nash equilibrium staking ratio.

    Derivation
    ----------
    In a one-shot game, each token holder decides whether to stake.

    - **Staking payoff**: nominal APY = inflation * split / ratio
    - **Not staking payoff**: alternative yield + avoided unbonding risk

    At equilibrium, the marginal staker is indifferent:

        inflation * split / ratio_eq = yield_alternatives + inflation - burn_rate

    Solving for ratio_eq:

        ratio_eq = inflation * split / (yield_alternatives + inflation - burn_rate)

    If the denominator is zero or negative (staking always dominates), the
    equilibrium is 100% staked.  If ratio_eq > 1, it means staking is so
    attractive that even at 100% participation the yield exceeds alternatives.

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    This is a **symmetric Nash equilibrium** in a staking game.  No player
    can improve their payoff by unilaterally changing strategy.

    - Higher inflation -> higher equilibrium staking (more reward to capture).
    - Higher burn rate -> higher equilibrium staking (less dilution pressure).
    - Higher yield alternatives -> lower equilibrium staking (competition).

    Parameters
    ----------
    inflation_rate : Decimal
        Annual inflation rate.
    effective_burn_rate : Decimal
        Annual effective burn rate.
    yield_alternatives : Decimal
        Annual yield available outside staking (DeFi, lending, etc.).
    emission_split_staking : Decimal
        Fraction of emissions going to stakers.

    Returns
    -------
    Decimal
        Equilibrium staking ratio (0 to 1).
    """
    inf = _dec(inflation_rate)
    burn = _dec(effective_burn_rate)
    alt = _dec(yield_alternatives)
    split = _dec(emission_split_staking)

    denominator = alt + inf - burn
    if denominator <= 0:
        return D("1.0")  # Staking always dominates

    ratio = inf * split / denominator
    if ratio > D("1.0"):
        return D("1.0")
    if ratio < D("0.0"):
        return D("0.0")
    return ratio


# ---------------------------------------------------------------------------
# 6. Reward Distribution Fairness
# ---------------------------------------------------------------------------

def gini_coefficient(rewards: List[float | Decimal]) -> Decimal:
    """
    Compute the Gini coefficient of a reward distribution.

    The Gini coefficient is the standard measure of inequality, ranging
    from 0 (perfect equality) to 1 (perfect inequality).

    Derivation
    ----------
    For a sorted sample x_1 <= x_2 <= ... <= x_n:

        G = (2 * sum_{i=1}^{n} i * x_i) / (n * sum(x_i)) - (n + 1) / n

    This is equivalent to the ratio of the area between the Lorenz curve
    and the line of equality, to the total area under the line of equality.

    Game-theoretic interpretation
    ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    - G near 0: rewards are evenly distributed -- incentivises participation.
    - G near 1: rewards concentrate on few validators -- cartel risk.
    - Target: G < 0.30 for a healthy PoS network.
    - Omniphi's RewardMult normalisation should keep G low even with
      varied validator sizes, because multipliers are budget-neutral.

    Parameters
    ----------
    rewards : list
        Reward amounts per validator.

    Returns
    -------
    Decimal
        Gini coefficient (0 to 1).
    """
    if not rewards:
        return D("0")

    vals = sorted(_dec(r) for r in rewards)
    n = len(vals)
    total = sum(vals)
    if total == 0:
        return D("0")

    numerator = D("0")
    for i, v in enumerate(vals):
        numerator += D(str(2 * (i + 1) - n - 1)) * v
    return numerator / (D(str(n)) * total)


def herfindahl_hirschman_index(stakes: List[float | Decimal]) -> Decimal:
    """
    Compute the Herfindahl-Hirschman Index (HHI) of stake concentration.

    HHI = sum(s_i^2) where s_i is each validator's share of total stake.

    - HHI near 0: highly competitive (many small validators).
    - HHI near 1: monopoly (single validator).
    - HHI > 0.25: highly concentrated (antitrust concern in traditional markets).
    - For PoS: HHI < 0.10 is healthy.

    Parameters
    ----------
    stakes : list
        Stake amounts per validator.

    Returns
    -------
    Decimal
        HHI value (0 to 1).
    """
    if not stakes:
        return D("0")
    total = sum(_dec(s) for s in stakes)
    if total == 0:
        return D("0")
    return sum((_dec(s) / total) ** 2 for s in stakes)


# ---------------------------------------------------------------------------
# 7. Inflation Dynamics
# ---------------------------------------------------------------------------

def years_to_supply_cap(
    current_supply: float | Decimal,
    supply_cap: float | Decimal = 1_500_000_000,
    effective_burn_rate: float | Decimal = 0.05,
    starting_year: int = 0,
    inflation_floor: float | Decimal = 0.005,
) -> Optional[int]:
    """
    Estimate the number of years until the supply cap is reached.

    Uses the exact inflation schedule from ``inflation_decay.go`` with
    net annual supply change = minting - burning.

    Returns ``None`` if burns exceed minting permanently (deflationary)
    or if cap is never reached within 200 years.

    Parameters
    ----------
    current_supply : Decimal
        Current total supply in OMNI.
    supply_cap : Decimal
        Maximum supply cap.
    effective_burn_rate : Decimal
        Annual burn rate as fraction of circulating supply.
    starting_year : int
        Year index to start from.
    inflation_floor : Decimal
        Minimum inflation rate.

    Returns
    -------
    int or None
        Years until cap is reached, or None if never.
    """
    from simulation import SimulationConfig
    cfg = SimulationConfig()
    supply = _dec(current_supply)
    cap = _dec(supply_cap)
    burn_rate = _dec(effective_burn_rate)

    for y in range(starting_year, 200):
        inf_rate = cfg.get_inflation_rate(y)
        mint = supply * inf_rate
        burn = supply * burn_rate
        net = mint - burn
        supply = supply + net
        if supply >= cap:
            return y - starting_year + 1
    return None


def compute_net_annual_issuance(
    year: int,
    current_supply: float | Decimal,
    effective_burn_rate: float | Decimal = 0.05,
) -> dict:
    """
    Compute net annual issuance for a given year.

    Returns gross minting, gross burning, and net change with rates.
    """
    from simulation import SimulationConfig
    cfg = SimulationConfig()
    supply = _dec(current_supply)
    inf_rate = cfg.get_inflation_rate(year)
    burn = _dec(effective_burn_rate)

    gross_mint = supply * inf_rate
    gross_burn = supply * burn
    net = gross_mint - gross_burn
    net_rate = net / supply if supply > 0 else D("0")

    return {
        "year": year,
        "inflation_rate": str(inf_rate),
        "gross_mint": str(gross_mint),
        "gross_burn": str(gross_burn),
        "net_issuance": str(net),
        "net_issuance_rate": str(net_rate),
        "is_deflationary": net < 0,
    }


# ---------------------------------------------------------------------------
# 8. Treasury Sustainability
# ---------------------------------------------------------------------------

def compute_treasury_runway(
    treasury_balance: float | Decimal,
    annual_inflows: float | Decimal,
    annual_outflows: float | Decimal,
) -> dict:
    """
    Compute how long the treasury can sustain operations.

    Parameters
    ----------
    treasury_balance : Decimal
        Current treasury balance in OMNI.
    annual_inflows : Decimal
        Expected annual treasury income (emission split + burn redirects + fees).
    annual_outflows : Decimal
        Expected annual treasury spending.

    Returns
    -------
    dict
        Runway in years, net annual flow, and sustainability assessment.
    """
    balance = _dec(treasury_balance)
    inflows = _dec(annual_inflows)
    outflows = _dec(annual_outflows)
    net = inflows - outflows

    if net >= 0:
        runway = "infinite"
        assessment = "sustainable"
    elif net < 0 and balance > 0:
        runway = str(balance / abs(net))
        assessment = "depleting"
    else:
        runway = "0"
        assessment = "insolvent"

    return {
        "treasury_balance": str(balance),
        "annual_inflows": str(inflows),
        "annual_outflows": str(outflows),
        "net_annual_flow": str(net),
        "runway_years": runway,
        "assessment": assessment,
    }
