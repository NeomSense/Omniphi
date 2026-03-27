"""
Omniphi Tokenomics -- MEV and Sequencer Security Analysis
==========================================================

Maximal Extractable Value (MEV) analysis specific to Omniphi's encrypted
intent architecture and sequencer-based execution model.

The Omniphi sequencer system processes encrypted intents, which in theory
limits MEV.  This module quantifies the residual MEV risk and the economic
incentives around sequencer collusion.

References
----------
- Sequencer emission split: ``chain/x/tokenomics/types/params.go``
  (EmissionSplitSequencer = 20%)
- Sequencer gas burn: ``BurnRateSequencerGas = 15%``
- Fee burn model: ``chain/x/feemarket/keeper/unified_burn.go``
"""

from __future__ import annotations

from decimal import Decimal, getcontext
from typing import Optional

getcontext().prec = 36

D = Decimal


def _dec(v) -> Decimal:
    if isinstance(v, Decimal):
        return v
    return Decimal(str(v))


# ---------------------------------------------------------------------------
# 1. MEV Opportunity Sizing
# ---------------------------------------------------------------------------

def compute_mev_opportunity(
    daily_tx_volume: float | Decimal,
    average_spread_bps: float | Decimal = 50,
    average_latency_ms: float | Decimal = 200,
    block_time_ms: float | Decimal = 7000,
) -> dict:
    """
    Estimate the maximal extractable value available to sequencers.

    Derivation
    ----------
    MEV in intent-based systems comes from three sources:

    1. **Sandwich attacks** on swap intents (limited by encryption).
    2. **Time-priority ordering** within a block (limited by rotation).
    3. **Cross-domain arbitrage** between chains (limited by IBC finality).

    The theoretical maximum extractable value:

        MEV_max = tx_volume * spread_bps / 10000

    The *realizable* MEV is reduced by:
    - **Encrypted mempool**: Sequencer cannot see intent contents until reveal.
    - **Reveal timing**: The fraction of the block window where intents are
      decrypted and re-orderable.

        MEV_realizable = MEV_max * (latency / block_time) * encryption_leakage

    Where encryption_leakage captures metadata leaks (tx size, timing, etc.).

    Parameters
    ----------
    daily_tx_volume : Decimal
        Daily transaction volume in OMNI.
    average_spread_bps : Decimal
        Average bid-ask spread in basis points (1 bp = 0.01%).
    average_latency_ms : Decimal
        Average time between intent reveal and block inclusion (ms).
    block_time_ms : Decimal
        Block time in milliseconds.

    Returns
    -------
    dict
        MEV opportunity analysis with theoretical max and realizable estimates.
    """
    vol = _dec(daily_tx_volume)
    spread = _dec(average_spread_bps)
    latency = _dec(average_latency_ms)
    block = _dec(block_time_ms)

    # Theoretical maximum: full information extraction
    mev_max_daily = vol * spread / D("10000")
    mev_max_annual = mev_max_daily * D("365")

    # Encryption discount: metadata leakage estimated at 5-15%
    encryption_leakage = D("0.10")  # 10% average metadata leakage

    # Timing window: fraction of block where reordering is possible
    timing_window = latency / block if block > 0 else D("0")

    # Realizable MEV
    mev_realizable_daily = mev_max_daily * timing_window * encryption_leakage
    mev_realizable_annual = mev_realizable_daily * D("365")

    # Pct of tx volume
    mev_pct = (mev_realizable_daily / vol * D("100")) if vol > 0 else D("0")

    return {
        "daily_tx_volume": str(vol),
        "theoretical_mev_daily": str(mev_max_daily),
        "theoretical_mev_annual": str(mev_max_annual),
        "encryption_leakage_pct": str(encryption_leakage * D("100")),
        "timing_window_pct": str(timing_window * D("100")),
        "realizable_mev_daily": str(mev_realizable_daily),
        "realizable_mev_annual": str(mev_realizable_annual),
        "mev_as_pct_of_volume": str(mev_pct),
        "protection_effectiveness_pct": str(
            (D("1") - timing_window * encryption_leakage) * D("100")
        ),
    }


# ---------------------------------------------------------------------------
# 2. Encrypted Intent Protection Analysis
# ---------------------------------------------------------------------------

def encrypted_intent_protection_rate(
    reveal_deadline_blocks: int = 1,
    block_time_seconds: float | Decimal = 7,
    encryption_scheme: str = "threshold",
    num_threshold_parties: int = 10,
    corruption_threshold: int = 4,
) -> dict:
    """
    Compute the protection rate of the encrypted intent mempool.

    Derivation
    ----------
    The encrypted intent system protects against MEV through a commit-reveal
    scheme or threshold encryption:

    **Commit-reveal scheme**:
    - Intent is committed (hashed) in block N.
    - Content revealed in block N + reveal_deadline.
    - Protection rate = 1 - (probability of early reveal or inference).

    **Threshold encryption** (Omniphi target architecture):
    - Intent encrypted with threshold key requiring k-of-n sequencers.
    - Decryption only possible after block inclusion.
    - Protection fails only if >= k sequencers collude.

    Protection rate:

        P(protection) = 1 - P(collusion)
        P(collusion) = C(n, k) * p^k * (1-p)^(n-k)

    Where p is the probability any single sequencer is corrupt.
    We use a conservative p = 0.20 (20% corruption assumption).

    Parameters
    ----------
    reveal_deadline_blocks : int
        Number of blocks between commit and reveal.
    block_time_seconds : Decimal
        Block time in seconds.
    encryption_scheme : str
        "commit_reveal" or "threshold".
    num_threshold_parties : int
        n in k-of-n threshold scheme.
    corruption_threshold : int
        k in k-of-n threshold scheme (number needed to decrypt).

    Returns
    -------
    dict
        Protection analysis including rates and timing.
    """
    bt = _dec(block_time_seconds)
    reveal_time = D(str(reveal_deadline_blocks)) * bt

    if encryption_scheme == "commit_reveal":
        # Simple commit-reveal: protection degrades with mempool observation
        # Assume 5% information leakage per block of delay
        leakage_per_block = D("0.05")
        protection = D("1") - leakage_per_block * D(str(reveal_deadline_blocks))
        protection = max(D("0"), min(D("1"), protection))
        collusion_desc = "N/A (commit-reveal)"

    elif encryption_scheme == "threshold":
        # Threshold encryption: protection = 1 - P(k-of-n corrupt)
        # Using binomial probability with p = 0.20 corruption rate
        import math as _math

        n = num_threshold_parties
        k = corruption_threshold
        p = 0.20  # corruption probability per sequencer

        # P(>= k corrupt) = sum_{i=k}^{n} C(n,i) * p^i * (1-p)^(n-i)
        prob_collusion = D("0")
        for i in range(k, n + 1):
            coeff = D(str(_math.comb(n, i)))
            term = coeff * D(str(p)) ** i * D(str(1 - p)) ** (n - i)
            prob_collusion += term

        protection = D("1") - prob_collusion
        collusion_desc = f"{k}-of-{n} threshold (p_corrupt=0.20)"
    else:
        protection = D("0.95")  # conservative default
        collusion_desc = "unknown scheme"

    return {
        "encryption_scheme": encryption_scheme,
        "reveal_deadline_blocks": reveal_deadline_blocks,
        "reveal_time_seconds": str(reveal_time),
        "protection_rate_pct": str(protection * D("100")),
        "information_leakage_pct": str((D("1") - protection) * D("100")),
        "collusion_model": collusion_desc,
    }


# ---------------------------------------------------------------------------
# 3. Sequencer Collusion Threshold
# ---------------------------------------------------------------------------

def sequencer_collusion_threshold(
    num_sequencers: int = 10,
    stake_required_per_sequencer: float | Decimal = 50000,
    slash_rate: float | Decimal = 0.10,
    annual_legitimate_revenue: float | Decimal = 100000,
    mev_extraction_if_colluding: float | Decimal = 500000,
) -> dict:
    """
    Compute the minimum number of colluding sequencers needed and
    whether collusion is economically rational.

    Derivation
    ----------
    In Omniphi's threshold encryption model, ``k`` sequencers must collude
    to decrypt intents early.  The collusion is rational only if:

        MEV_share > legitimate_revenue + expected_slashing_cost

    For each colluding sequencer:

        MEV_share = total_mev / num_colluders
        slashing_cost = stake * slash_rate * detection_probability
        opportunity_cost = legitimate_revenue_lost_if_slashed

    **Minimum colluders**: Determined by the threshold encryption parameter k.

    **Economic rationality**: Collusion is irrational when:

        stake * slash_rate * P(detection) > MEV_share

    Parameters
    ----------
    num_sequencers : int
        Total number of active sequencers.
    stake_required_per_sequencer : Decimal
        Bond each sequencer must post (in OMNI).
    slash_rate : Decimal
        Fraction of bond slashed if caught colluding.
    annual_legitimate_revenue : Decimal
        Annual legitimate revenue per sequencer (in OMNI).
    mev_extraction_if_colluding : Decimal
        Total annual MEV available if all colluders cooperate.

    Returns
    -------
    dict
        Collusion analysis with economic rationality assessment.
    """
    n = num_sequencers
    stake = _dec(stake_required_per_sequencer)
    slash = _dec(slash_rate)
    legit_rev = _dec(annual_legitimate_revenue)
    total_mev = _dec(mev_extraction_if_colluding)

    results = {}

    # Threshold encryption parameter: typically ceil(n/3) + 1
    min_colluders = n // 3 + 1
    results["min_colluders_for_decryption"] = min_colluders
    results["total_sequencers"] = n

    # Detection probabilities for different collusion sizes
    for num_colluders in [min_colluders, n // 2, int(n * 0.75)]:
        if num_colluders > n:
            continue

        mev_per_colluder = total_mev / D(str(num_colluders))

        # Detection probability increases with number of colluders
        # Model: P(detect) = 1 - (1 - p_leak)^num_colluders
        # Where p_leak = 0.15 (probability any single colluder leaks/is caught)
        p_leak = D("0.15")
        p_detect = D("1") - (D("1") - p_leak) ** num_colluders

        expected_slash_cost = stake * slash * p_detect
        expected_legit_loss = legit_rev * p_detect  # lose future revenue if caught

        net_gain = mev_per_colluder - expected_slash_cost - expected_legit_loss
        is_rational = net_gain > 0

        results[f"colluders_{num_colluders}"] = {
            "mev_per_colluder": str(mev_per_colluder),
            "detection_probability_pct": str(p_detect * D("100")),
            "expected_slash_cost": str(expected_slash_cost),
            "expected_revenue_loss": str(expected_legit_loss),
            "net_expected_gain": str(net_gain),
            "economically_rational": is_rational,
            "min_stake_to_deter": str(
                (mev_per_colluder / (slash * p_detect)) if (slash * p_detect) > 0 else D("Infinity")
            ),
        }

    return results


# ---------------------------------------------------------------------------
# 4. Cost of Bias Attack
# ---------------------------------------------------------------------------

def cost_of_bias_attack(
    num_entropy_sources: int = 4,
    penalty_per_detected_bias: float | Decimal = 10000,
    detection_probability: float | Decimal = 0.85,
    expected_profit_from_bias: float | Decimal = 50000,
    hash_strength_bits: int = 256,
) -> dict:
    """
    Compute the cost of biasing the on-chain randomness engine.

    Omniphi uses a deterministic randomness engine with domain separation
    and multiple entropy sources (validator VRF, block hash, previous
    randomness, external beacon).

    Derivation
    ----------
    To bias the randomness output, an attacker must either:

    1. **Control entropy sources**: Compromise >= ceil(n/2) sources to
       influence the combined output.
    2. **Pre-image attack**: Find an input to the hash function that
       produces a desired output (computationally infeasible for SHA-256).
    3. **Last-revealer attack**: The last validator to reveal their VRF
       can choose to withhold, but loses their block reward.

    Expected cost of bias:

        E[cost] = penalty * P(detection) + opportunity_cost_of_withholding
        E[profit] = expected_profit * (1 - P(detection))
        Net EV = E[profit] - E[cost]

    Bias is irrational when Net EV < 0.

    Parameters
    ----------
    num_entropy_sources : int
        Number of independent entropy sources combined.
    penalty_per_detected_bias : Decimal
        Slashing penalty if bias attempt is detected (in OMNI).
    detection_probability : Decimal
        Probability that a bias attempt is detected (0 to 1).
    expected_profit_from_bias : Decimal
        Expected profit if bias succeeds and is undetected.
    hash_strength_bits : int
        Bit strength of the hash function used for domain separation.

    Returns
    -------
    dict
        Bias attack cost analysis.
    """
    n = num_entropy_sources
    penalty = _dec(penalty_per_detected_bias)
    p_detect = _dec(detection_probability)
    profit = _dec(expected_profit_from_bias)
    bits = hash_strength_bits

    # Sources needed to compromise
    sources_needed = n // 2 + 1

    # Expected values
    expected_penalty = penalty * p_detect
    expected_profit = profit * (D("1") - p_detect)
    net_ev = expected_profit - expected_penalty

    # Computational cost of pre-image attack
    preimage_operations = D("2") ** bits
    # At 10^12 hashes/second, time in years
    hash_rate = D("1e12")
    seconds_per_year = D("31557600")
    preimage_years = preimage_operations / hash_rate / seconds_per_year

    # Last-revealer withholding cost
    # Assume block reward ~= epoch_mint / blocks_per_epoch
    # Conservative: 1 OMNI per block
    withholding_cost = D("1")  # OMNI lost by not proposing the block

    return {
        "num_entropy_sources": n,
        "sources_needed_to_compromise": sources_needed,
        "detection_probability_pct": str(p_detect * D("100")),
        "expected_penalty_omni": str(expected_penalty),
        "expected_profit_omni": str(expected_profit),
        "net_expected_value_omni": str(net_ev),
        "bias_is_rational": net_ev > 0,
        "hash_strength_bits": bits,
        "preimage_attack_years": str(preimage_years) if preimage_years < D("1e50") else "infeasible",
        "last_revealer_withholding_cost_omni": str(withholding_cost),
        "min_penalty_to_deter": str(
            (profit * (D("1") - p_detect) / p_detect) if p_detect > 0 else D("Infinity")
        ),
    }


# ---------------------------------------------------------------------------
# 5. Composite MEV Risk Score
# ---------------------------------------------------------------------------

def compute_mev_risk_score(
    daily_tx_volume: float | Decimal,
    num_sequencers: int = 10,
    encryption_scheme: str = "threshold",
    stake_per_sequencer: float | Decimal = 50000,
    slash_rate: float | Decimal = 0.10,
) -> dict:
    """
    Compute a composite MEV risk score (0-100) for the network.

    Combines multiple risk factors into a single headline metric:
    - MEV opportunity size relative to sequencer revenue
    - Protection effectiveness of the encrypted mempool
    - Economic deterrence from slashing

    Score interpretation:
    - 0-20:  Low risk (MEV economically irrational)
    - 20-40: Moderate risk (MEV possible but deterred)
    - 40-60: Elevated risk (review sequencer parameters)
    - 60-80: High risk (protocol changes recommended)
    - 80-100: Critical risk (immediate action needed)
    """
    vol = _dec(daily_tx_volume)

    # Factor 1: MEV opportunity size (0-40 points)
    mev = compute_mev_opportunity(vol)
    mev_annual = _dec(mev["realizable_mev_annual"])
    legit_annual = vol * D("365") * D("0.20") / D(str(num_sequencers))  # 20% sequencer share
    mev_ratio = mev_annual / legit_annual if legit_annual > 0 else D("0")
    opportunity_score = min(D("40"), mev_ratio * D("100"))

    # Factor 2: Protection effectiveness (0-30 points, inverted)
    protection = encrypted_intent_protection_rate(
        encryption_scheme=encryption_scheme,
        num_threshold_parties=num_sequencers,
    )
    protection_pct = _dec(protection["protection_rate_pct"])
    protection_score = (D("100") - protection_pct) * D("30") / D("100")

    # Factor 3: Economic deterrence (0-30 points, inverted)
    collusion = sequencer_collusion_threshold(
        num_sequencers=num_sequencers,
        stake_required_per_sequencer=stake_per_sequencer,
        slash_rate=slash_rate,
    )
    min_c = collusion["min_colluders_for_decryption"]
    key = f"colluders_{min_c}"
    if key in collusion and collusion[key].get("economically_rational"):
        deterrence_score = D("30")  # Max risk: collusion is rational
    else:
        deterrence_score = D("10")  # Collusion is irrational

    total_score = opportunity_score + protection_score + deterrence_score
    total_score = min(D("100"), max(D("0"), total_score))

    if total_score < D("20"):
        risk_level = "LOW"
    elif total_score < D("40"):
        risk_level = "MODERATE"
    elif total_score < D("60"):
        risk_level = "ELEVATED"
    elif total_score < D("80"):
        risk_level = "HIGH"
    else:
        risk_level = "CRITICAL"

    return {
        "composite_risk_score": str(total_score),
        "risk_level": risk_level,
        "opportunity_score": str(opportunity_score),
        "protection_score": str(protection_score),
        "deterrence_score": str(deterrence_score),
        "recommendation": _mev_recommendation(risk_level),
    }


def _mev_recommendation(level: str) -> str:
    recs = {
        "LOW": "No action needed. Current parameters provide adequate MEV protection.",
        "MODERATE": "Monitor sequencer behaviour. Consider increasing slash rate if MEV activity detected.",
        "ELEVATED": "Increase sequencer bond requirements. Review threshold encryption parameters.",
        "HIGH": "Immediate review recommended. Increase slash rate and bond requirements. Consider reducing sequencer count.",
        "CRITICAL": "Protocol intervention needed. Emergency governance proposal to restructure sequencer incentives.",
    }
    return recs.get(level, "Unknown risk level.")
