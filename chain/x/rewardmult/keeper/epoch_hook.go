package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// valEntry is a package-level type used during epoch processing and normalization.
// It holds the intermediate state for a single bonded validator during multiplier
// computation.
type valEntry struct {
	valAddr    string
	stakeWt    math.LegacyDec
	mFinal     math.LegacyDec // clamped M_ema, before normalization
	multiplier types.ValidatorMultiplier
}

// ProcessEpoch is called at the end of each epoch to recompute all validator multipliers.
// SAFETY: This function never panics. All errors are logged and the chain continues.
//
// V2.2 changes:
//   - Warm-start EMA: first epoch uses M_raw directly instead of returning neutral 1.0
//   - Stake weight snapshots: captured at epoch boundary for normalization consistency
//   - Iterative normalization: up to 3 rounds to handle clamp-break budget drift
//   - Audit events: detailed normalization telemetry for third-party verification
func (k Keeper) ProcessEpoch(ctx sdk.Context, epoch int64) error {
	// Prevent double-processing
	lastProcessed := k.GetLastProcessedEpoch(ctx)
	if epoch <= lastProcessed {
		return nil
	}

	params := k.GetParams(ctx)

	// Get all bonded validators
	validators, err := k.stakingKeeper.GetAllValidators(ctx)
	if err != nil {
		k.logger.Error("failed to get validators for epoch processing", "epoch", epoch, "error", err)
		return nil // don't halt
	}

	totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil || totalBonded.IsZero() {
		k.logger.Error("failed to get total bonded tokens", "epoch", epoch, "error", err)
		return nil
	}

	// Phase 1: Compute M_raw and M_ema for each bonded validator
	var entries []valEntry

	for _, val := range validators {
		if !val.IsBonded() {
			continue
		}

		valAddr := val.GetOperator()
		stakeWeight := val.Tokens.ToLegacyDec()

		// Compute M_raw components
		uptimeBonus := k.computeUptimeBonus(ctx, val, params)
		participationBonus := k.computeParticipationBonus(ctx, valAddr, params)
		slashPenalty := k.computeSlashPenalty(ctx, valAddr, epoch, params)
		fraudPenalty := k.computeFraudPenalty(ctx, valAddr, epoch, params)

		qualityBonus := k.computeQualityBonus(ctx, valAddr, params)

		// M_raw = 1 + UptimeBonus + ParticipationBonus + QualityBonus - SlashPenalty - FraudPenalty
		mRaw := math.LegacyOneDec().
			Add(uptimeBonus).
			Add(participationBonus).
			Add(qualityBonus).
			Sub(slashPenalty).
			Sub(fraudPenalty)

		// V2.2: Warm-start EMA — uses AddValue + ComputeEMA which handles
		// first-epoch correctly (single value → returns M_raw, not neutral 1.0)
		history := k.GetEMAHistory(ctx, valAddr)
		mEma := warmStartEMA(&history, mRaw, params.EMAWindow)

		if err := k.SetEMAHistory(ctx, history); err != nil {
			k.logger.Error("failed to save EMA history", "validator", valAddr, "error", err)
			continue
		}

		// Clamp M_ema to [min, max] — this is the pre-normalization clamp
		mFinal := clamp(mEma, params.MinMultiplier, params.MaxMultiplier)

		vm := types.ValidatorMultiplier{
			ValidatorAddress:   valAddr,
			Epoch:              epoch,
			MRaw:               mRaw,
			MEMA:               mEma,
			MEffective:         mFinal, // will be overwritten after normalization
			UptimeBonus:        uptimeBonus,
			ParticipationBonus: participationBonus,
			QualityBonus:       qualityBonus,
			SlashPenalty:       slashPenalty,
			FraudPenalty:       fraudPenalty,
		}

		entries = append(entries, valEntry{
			valAddr:    valAddr,
			stakeWt:    stakeWeight,
			mFinal:     mFinal,
			multiplier: vm,
		})
	}

	if len(entries) == 0 {
		if err := k.SetLastProcessedEpoch(ctx, epoch); err != nil {
			k.logger.Error("failed to set last processed epoch", "error", err)
		}
		return nil
	}

	// V2.2: Snapshot stake weights at epoch boundary for consistency.
	// This ensures normalization and any downstream distribution use identical weights.
	if err := k.snapshotStakeWeights(ctx, epoch, entries); err != nil {
		k.logger.Error("failed to snapshot stake weights", "epoch", epoch, "error", err)
		// Non-fatal: continue with normalization using in-memory weights
	}

	// Compute pre-normalization weighted sum for audit telemetry
	weightedSumBeforeNorm := math.LegacyZeroDec()
	totalStake := math.LegacyZeroDec()
	for _, e := range entries {
		totalStake = totalStake.Add(e.stakeWt)
		weightedSumBeforeNorm = weightedSumBeforeNorm.Add(e.stakeWt.Mul(e.mFinal))
	}

	if weightedSumBeforeNorm.IsZero() {
		k.logger.Error("weighted multiplier sum is zero, cannot normalize", "epoch", epoch)
		return nil
	}

	// Phase 2+3: V2.2 iterative budget-neutral normalization.
	// This replaces the old single-pass normalize+clamp with an iterative approach
	// that correctly handles validators hitting min/max after normalization.
	norm, rounds, clampedMin, clampedMax := iterativeNormalize(entries, params)

	// Phase 4: Persist all multipliers
	for i := range entries {
		if err := k.SetValidatorMultiplier(ctx, entries[i].multiplier); err != nil {
			k.logger.Error("failed to save validator multiplier",
				"validator", entries[i].valAddr, "error", err)
			continue
		}
	}

	// Record epoch
	if err := k.SetLastProcessedEpoch(ctx, epoch); err != nil {
		k.logger.Error("failed to set last processed epoch", "error", err)
	}

	// V2.2: Compute post-normalization telemetry and emit audit event
	weightedSumAfterNorm, _, budgetError := computePostNormStats(entries)

	stats := types.EpochNormalizationStats{
		Epoch:                 epoch,
		NormFactor:            norm,
		TotalStake:            totalStake,
		WeightedSumBeforeNorm: weightedSumBeforeNorm,
		WeightedSumAfterNorm:  weightedSumAfterNorm,
		CountClampedMin:       clampedMin,
		CountClampedMax:       clampedMax,
		IterativeRounds:       rounds,
		BudgetError:           budgetError,
	}
	emitNormalizationEvent(ctx, stats)

	// Legacy event for backward compatibility
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"rewardmult_epoch_processed",
		sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
		sdk.NewAttribute("validators_processed", fmt.Sprintf("%d", len(entries))),
		sdk.NewAttribute("normalization_factor", norm.String()),
	))

	k.logger.Info("reward multipliers updated",
		"epoch", epoch,
		"validators", len(entries),
		"norm_factor", norm.String(),
		"iterative_rounds", rounds,
		"clamped_min", clampedMin,
		"clamped_max", clampedMax,
		"budget_error", budgetError.String(),
	)

	return nil
}

// computeUptimeBonus computes the uptime bonus for a validator.
// Uses signing info from the slashing keeper to determine uptime.
func (k Keeper) computeUptimeBonus(ctx sdk.Context, val interface{ GetConsensusPower(math.Int) int64; GetConsAddr() ([]byte, error) }, params types.Params) math.LegacyDec {
	if k.slashingKeeper == nil {
		return math.LegacyZeroDec()
	}

	// Safely get consensus address - validators without proper pubkey setup return zero bonus
	var consAddrBytes []byte
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic getting consensus address: %v", r)
			}
		}()
		consAddrBytes, err = val.GetConsAddr()
	}()
	if err != nil || len(consAddrBytes) == 0 {
		return math.LegacyZeroDec()
	}
	consAddr := sdk.ConsAddress(consAddrBytes)

	signingInfo, err := k.slashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
	if err != nil {
		return math.LegacyZeroDec() // missing data → neutral
	}

	// Compute uptime from signing info
	// MissedBlocksCounter / SignedBlocksWindow gives miss rate
	// Uptime = 1 - missRate
	signedBlocksWindow := int64(100) // default if not available from params
	if signingInfo.StartHeight > 0 {
		blocksSinceStart := ctx.BlockHeight() - signingInfo.StartHeight
		if blocksSinceStart > 0 && blocksSinceStart < signedBlocksWindow {
			signedBlocksWindow = blocksSinceStart
		}
	}

	if signedBlocksWindow <= 0 {
		return math.LegacyZeroDec()
	}

	missRate := math.LegacyNewDec(signingInfo.MissedBlocksCounter).Quo(math.LegacyNewDec(signedBlocksWindow))
	uptime := math.LegacyOneDec().Sub(missRate)

	// Clamp uptime to [0, 1]
	if uptime.IsNegative() {
		uptime = math.LegacyZeroDec()
	}
	if uptime.GT(math.LegacyOneDec()) {
		uptime = math.LegacyOneDec()
	}

	// Apply tiered bonus
	if uptime.GTE(params.UptimeThresholdHigh) {
		return params.UptimeBonusHigh
	}
	if uptime.GTE(params.UptimeThresholdMed) {
		return params.UptimeBonusMed
	}

	return math.LegacyZeroDec()
}

// computeParticipationBonus computes the PoV endorsement participation bonus.
// Returns MaxParticipationBonus * participationRate.
// Returns 0 if PoC keeper is not available.
func (k Keeper) computeParticipationBonus(ctx sdk.Context, valAddr string, params types.Params) math.LegacyDec {
	if k.pocKeeper == nil {
		return math.LegacyZeroDec()
	}

	valAddress, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return math.LegacyZeroDec()
	}

	rate, err := k.pocKeeper.GetEndorsementParticipationRate(ctx, valAddress)
	if err != nil {
		return math.LegacyZeroDec()
	}

	// Clamp rate to [0, 1]
	if rate.IsNegative() {
		rate = math.LegacyZeroDec()
	}
	if rate.GT(math.LegacyOneDec()) {
		rate = math.LegacyOneDec()
	}

	return params.MaxParticipationBonus.Mul(rate)
}

// computeSlashPenalty computes the slash penalty with infraction-aware dual decay.
// Downtime and double-sign slashes have separate penalty magnitudes and decay windows.
// The worst penalty dominates (they don't stack).
func (k Keeper) computeSlashPenalty(ctx sdk.Context, valAddr string, currentEpoch int64, params types.Params) math.LegacyDec {
	// Compute downtime slash penalty with decay
	downtimeDecay := k.SlashDecayFractionByType(ctx, valAddr, currentEpoch,
		params.SlashLookbackEpochs, types.InfractionDowntime)
	downtimePenalty := math.LegacyZeroDec()
	if !downtimeDecay.IsZero() {
		downtimePenalty = params.SlashPenalty.Mul(downtimeDecay)
	}

	// Compute double-sign slash penalty with decay (heavier, longer window)
	doubleSignDecay := k.SlashDecayFractionByType(ctx, valAddr, currentEpoch,
		params.DoubleSignLookbackEpochs, types.InfractionDoubleSign)
	doubleSignPenalty := math.LegacyZeroDec()
	if !doubleSignDecay.IsZero() {
		doubleSignPenalty = params.DoubleSignPenalty.Mul(doubleSignDecay)
	}

	// Return the maximum penalty (worst infraction dominates)
	if doubleSignPenalty.GT(downtimePenalty) {
		return doubleSignPenalty
	}
	return downtimePenalty
}

// computeFraudPenalty computes the fraud penalty (stub for PoR integration).
// Returns FraudPenalty if validator has fraudulent attestation in lookback window.
func (k Keeper) computeFraudPenalty(ctx sdk.Context, valAddr string, currentEpoch int64, params types.Params) math.LegacyDec {
	if k.porKeeper == nil {
		return math.LegacyZeroDec() // PoR not yet live
	}

	valAddress, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return math.LegacyZeroDec()
	}

	hasFraud, err := k.porKeeper.HasFraudulentAttestation(ctx, valAddress, params.FraudLookbackEpochs)
	if err != nil || !hasFraud {
		return math.LegacyZeroDec()
	}

	return params.FraudPenalty
}

// computeQualityBonus computes the PoC quality bonus for a validator.
// Uses originality and quality metrics from contributions endorsed by this validator.
// Returns min(MaxQualityBonus, avgQuality/10 * avgOriginality * MaxQualityBonus).
// Returns 0 if PoC keeper is not available or no data exists.
func (k Keeper) computeQualityBonus(ctx sdk.Context, valAddr string, params types.Params) math.LegacyDec {
	if k.pocKeeper == nil {
		return math.LegacyZeroDec()
	}

	// Guard: if MaxQualityBonus is nil or zero, skip computation
	if params.MaxQualityBonus.IsNil() || params.MaxQualityBonus.IsZero() {
		return math.LegacyZeroDec()
	}

	valAddress, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return math.LegacyZeroDec()
	}

	avgOriginality, avgQuality, err := k.pocKeeper.GetValidatorOriginalityMetrics(ctx, valAddress)
	if err != nil {
		return math.LegacyZeroDec()
	}

	// qualityBonus = min(MaxQualityBonus, avgQuality * avgOriginality * MaxQualityBonus)
	// avgQuality is [0, 1], avgOriginality is [0.4, 1.2] (from originality bands)
	bonus := avgQuality.Mul(avgOriginality).Mul(params.MaxQualityBonus)

	// Clamp to [0, MaxQualityBonus]
	if bonus.IsNegative() {
		return math.LegacyZeroDec()
	}
	if bonus.GT(params.MaxQualityBonus) {
		return params.MaxQualityBonus
	}
	return bonus
}

// clamp restricts value to [min, max]
func clamp(value, min, max math.LegacyDec) math.LegacyDec {
	if value.LT(min) {
		return min
	}
	if value.GT(max) {
		return max
	}
	return value
}
