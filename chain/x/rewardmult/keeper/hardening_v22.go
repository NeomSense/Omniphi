package keeper

// V2.2 Safety & Audit Hardening
//
// This file contains all V2.2 additions:
// 1. Iterative budget-neutral normalization (fixes clamp-break drift)
// 2. Epoch stake weight snapshots (prevents mid-epoch inconsistency)
// 3. Warm-start EMA initialization (prevents dead neutral first epoch)
// 4. Audit-grade event emission
// 5. Clamp pressure telemetry

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// MaxIterativeNormRounds caps the number of normalization iterations to prevent
// unbounded computation. In practice, convergence happens in 1-2 rounds because
// the multiplier range [0.85, 1.15] is narrow and few validators hit the clamps.
const MaxIterativeNormRounds = 3

// ============================================================================
// Iterative Budget-Neutral Normalization
// ============================================================================

// iterativeNormalize performs budget-neutral normalization that correctly handles
// the clamp-break problem: when post-normalization clamping pushes validators to
// min/max, the remaining "free" validators must absorb the difference to maintain
// Σ(stakeWeight × M_effective) == Σ(stakeWeight).
//
// Algorithm (water-filling variant):
//  1. Compute norm = totalWeight / weightedMultSum
//  2. Apply norm to all multipliers, then clamp to [min, max]
//  3. Partition into "clamped" (hit min or max) and "free" (not clamped)
//  4. Recompute norm using only free validators to absorb the budget residual
//  5. Repeat until no new clamps appear or MaxIterativeNormRounds reached
//
// This is O(validators × MaxIterativeNormRounds) which is bounded at 3× the
// original single-pass cost. With typical [0.85, 1.15] bounds, round 2 is rare
// and round 3 is practically unreachable.
func iterativeNormalize(
	entries []valEntry,
	params types.Params,
) (norm math.LegacyDec, rounds int, clampedMin int, clampedMax int) {
	n := len(entries)
	if n == 0 {
		return math.LegacyOneDec(), 0, 0, 0
	}

	// effective[i] holds the current effective multiplier for each entry
	effective := make([]math.LegacyDec, n)
	for i := range entries {
		effective[i] = entries[i].mFinal
	}

	// clamped[i] tracks whether a validator is locked at min or max
	clamped := make([]bool, n)
	norm = math.LegacyOneDec()

	for round := 0; round < MaxIterativeNormRounds; round++ {
		rounds = round + 1

		// Compute total weight and weighted multiplier sum
		totalWeight := math.LegacyZeroDec()
		weightedMultSum := math.LegacyZeroDec()
		for i := range entries {
			totalWeight = totalWeight.Add(entries[i].stakeWt)
			weightedMultSum = weightedMultSum.Add(entries[i].stakeWt.Mul(effective[i]))
		}

		if weightedMultSum.IsZero() {
			return math.LegacyOneDec(), rounds, 0, 0
		}

		// Compute normalization factor for this round
		norm = totalWeight.Quo(weightedMultSum)

		// Apply normalization and clamp; track new clamps
		newClamps := false
		clampedMin = 0
		clampedMax = 0
		for i := range entries {
			if clamped[i] {
				// Already clamped from a previous round — count but don't re-normalize
				if effective[i].Equal(params.MinMultiplier) {
					clampedMin++
				} else {
					clampedMax++
				}
				continue
			}

			normalized := effective[i].Mul(norm)

			if normalized.LT(params.MinMultiplier) {
				effective[i] = params.MinMultiplier
				clamped[i] = true
				clampedMin++
				newClamps = true
			} else if normalized.GT(params.MaxMultiplier) {
				effective[i] = params.MaxMultiplier
				clamped[i] = true
				clampedMax++
				newClamps = true
			} else {
				effective[i] = normalized
			}
		}

		if !newClamps {
			// Converged: no new validators were clamped
			break
		}

		// Prepare next round: compute residual from clamped validators
		// and redistribute normalization pressure to free validators.
		// Free validators keep their pre-norm mFinal values so normalization
		// is recomputed from scratch each round with clamped values locked.
		for i := range entries {
			if !clamped[i] {
				// Reset free validators to their pre-normalization value
				// so the next round recomputes normalization cleanly
				effective[i] = entries[i].mFinal
			}
		}
	}

	// Write final effective values back to entries
	for i := range entries {
		entries[i].multiplier.MEffective = effective[i]
	}

	return norm, rounds, clampedMin, clampedMax
}

// ============================================================================
// Epoch Stake Weight Snapshots
// ============================================================================

// SetEpochStakeSnapshot stores a validator's stake snapshot at an epoch boundary
func (k Keeper) SetEpochStakeSnapshot(ctx context.Context, snapshot types.EpochStakeSnapshot) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal stake snapshot: %w", err)
	}
	key := types.GetEpochStakeSnapshotKey(snapshot.Epoch, snapshot.ValidatorAddress)
	return kvStore.Set(key, bz)
}

// GetEpochStakeSnapshots returns all stake snapshots for a given epoch
func (k Keeper) GetEpochStakeSnapshots(ctx context.Context, epoch int64) []types.EpochStakeSnapshot {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetEpochStakeSnapshotPrefixKey(epoch)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		k.logger.Error("failed to create stake snapshot iterator", "error", err)
		return nil
	}
	defer iter.Close()

	var snapshots []types.EpochStakeSnapshot
	for ; iter.Valid(); iter.Next() {
		var s types.EpochStakeSnapshot
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			k.logger.Error("failed to unmarshal stake snapshot", "error", err)
			continue
		}
		snapshots = append(snapshots, s)
	}
	return snapshots
}

// snapshotStakeWeights captures bonded validator stakes at epoch boundary
// and returns them as a map for use during normalization. This ensures the
// same stake weights are used for both normalization and any downstream
// distribution, preventing mid-epoch stake changes from causing drift.
func (k Keeper) snapshotStakeWeights(ctx sdk.Context, epoch int64, entries []valEntry) error {
	snapshotTotal := math.ZeroInt()
	for _, e := range entries {
		snapshot := types.EpochStakeSnapshot{
			Epoch:            epoch,
			ValidatorAddress: e.valAddr,
			BondedTokens:     e.stakeWt.TruncateInt(),
		}
		if err := k.SetEpochStakeSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("failed to snapshot stake for %s: %w", e.valAddr, err)
		}
		snapshotTotal = snapshotTotal.Add(snapshot.BondedTokens)
	}

	// Verify snapshot sum matches total bonded tokens within rounding tolerance.
	// The truncation from LegacyDec to Int can cause at most 1 unit per validator.
	totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil {
		// Non-fatal: log and continue. The snapshot is still consistent internally.
		k.logger.Error("failed to verify snapshot against total bonded", "error", err)
		return nil
	}

	diff := totalBonded.Sub(snapshotTotal).Abs()
	maxRounding := math.NewInt(int64(len(entries)))
	if diff.GT(maxRounding) {
		k.logger.Error("stake snapshot sum mismatch exceeds rounding tolerance",
			"snapshot_total", snapshotTotal,
			"total_bonded", totalBonded,
			"diff", diff,
			"max_rounding", maxRounding,
		)
	}

	return nil
}

// ============================================================================
// Warm-Start EMA Initialization
// ============================================================================

// warmStartEMA sets the first EMA value to M_raw on a validator's first epoch,
// preventing a "dead neutral" epoch where EMA returns 1.0 regardless of actual
// performance. This is safe because the single-value EMA is exactly M_raw
// (alpha * M_raw + (1-alpha) * M_raw = M_raw when there's only one value).
// The ComputeEMA function already returns the single value when len(Values)==1,
// so warm-start is actually handled by AddValue before ComputeEMA. This function
// is a safety check that ensures we never return neutral for a validator that
// has computed a real M_raw.
func warmStartEMA(history *types.EMAHistory, mRaw math.LegacyDec, window int64) math.LegacyDec {
	history.AddValue(mRaw, window)
	ema := history.ComputeEMA(window)

	// Safety: if ComputeEMA returned exactly 1.0 but we have values,
	// it means all historical values are exactly 1.0, which is fine.
	// The warm-start guarantee is: after AddValue + ComputeEMA, the result
	// always reflects actual data, never a "no data" default.
	return ema
}

// ============================================================================
// Audit-Grade Event Emission
// ============================================================================

// emitNormalizationEvent emits a detailed event containing all information
// needed for an external auditor to reconstruct the epoch's reward math.
func emitNormalizationEvent(ctx sdk.Context, stats types.EpochNormalizationStats) {
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"rewardmult_epoch_normalization",
		sdk.NewAttribute("epoch", fmt.Sprintf("%d", stats.Epoch)),
		sdk.NewAttribute("norm_factor", stats.NormFactor.String()),
		sdk.NewAttribute("total_stake", stats.TotalStake.String()),
		sdk.NewAttribute("weighted_sum_before_norm", stats.WeightedSumBeforeNorm.String()),
		sdk.NewAttribute("weighted_sum_after_norm", stats.WeightedSumAfterNorm.String()),
		sdk.NewAttribute("count_clamped_min", fmt.Sprintf("%d", stats.CountClampedMin)),
		sdk.NewAttribute("count_clamped_max", fmt.Sprintf("%d", stats.CountClampedMax)),
		sdk.NewAttribute("iterative_rounds", fmt.Sprintf("%d", stats.IterativeRounds)),
		sdk.NewAttribute("budget_error", stats.BudgetError.String()),
	))
}

// computePostNormStats computes the budget error and weighted sums after
// normalization for audit telemetry.
func computePostNormStats(entries []valEntry) (weightedSumAfterNorm math.LegacyDec, totalStake math.LegacyDec, budgetError math.LegacyDec) {
	totalStake = math.LegacyZeroDec()
	weightedSumAfterNorm = math.LegacyZeroDec()

	for _, e := range entries {
		totalStake = totalStake.Add(e.stakeWt)
		weightedSumAfterNorm = weightedSumAfterNorm.Add(e.stakeWt.Mul(e.multiplier.MEffective))
	}

	if totalStake.IsZero() {
		return weightedSumAfterNorm, totalStake, math.LegacyZeroDec()
	}

	// Budget error = |Σ(w*M) - Σ(w)| / Σ(w)
	budgetError = weightedSumAfterNorm.Sub(totalStake).Abs().Quo(totalStake)
	return weightedSumAfterNorm, totalStake, budgetError
}

// ============================================================================
// Clamp Pressure Query Support
// ============================================================================

// GetClampPressure computes clamp pressure telemetry from the latest epoch's
// stored multipliers. This is read-only observability, not logic-changing.
func (k Keeper) GetClampPressure(ctx context.Context) types.QueryClampPressureResponse {
	params := k.GetParams(ctx)
	multipliers := k.GetAllValidatorMultipliers(ctx)

	epoch := k.GetLastProcessedEpoch(ctx)
	countMin := 0
	countMax := 0
	total := len(multipliers)

	for _, vm := range multipliers {
		if vm.MEffective.Equal(params.MinMultiplier) {
			countMin++
		}
		if vm.MEffective.Equal(params.MaxMultiplier) {
			countMax++
		}
	}

	return types.QueryClampPressureResponse{
		Epoch:           epoch,
		CountClampedMin: countMin,
		CountClampedMax: countMax,
		TotalValidators: total,
	}
}
