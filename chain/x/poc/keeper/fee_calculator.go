package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// 3-LAYER FEE CALCULATION SYSTEM
//
// This file implements the comprehensive 3-layer fee model:
// Layer 1: Base Fee Model (static base fee)
// Layer 2: Epoch-Adaptive Fee Model (dynamic congestion multiplier)
// Layer 3: C-Score Weighted Discount Model (reputation-based discount)
//
// Fee Calculation Pipeline:
// 1. base_fee = BaseSubmissionFee
// 2. dynamic_fee = base_fee * epoch_multiplier
// 3. final_fee = dynamic_fee * (1 - cscore_discount)
// 4. ensure final_fee >= MinimumSubmissionFee

// SubmissionCounterKey is the transient store key for tracking submissions per block
var SubmissionCounterKey = []byte("submission_counter")

// Calculate3LayerFee computes the final fee using all three layers
//
// Returns:
// - finalFee: The calculated fee after all adjustments
// - epochMultiplier: The congestion multiplier applied (for events)
// - cscoreDiscount: The discount percentage applied (for events)
// - error: Any error during calculation
func (k Keeper) Calculate3LayerFee(ctx context.Context, contributor sdk.AccAddress) (
	finalFee sdk.Coin,
	epochMultiplier math.LegacyDec,
	cscoreDiscount math.LegacyDec,
	err error,
) {
	params := k.GetParams(ctx)

	// LAYER 1: Get base fee
	baseFee := params.BaseSubmissionFee

	// LAYER 2: Calculate epoch multiplier (dynamic congestion fee)
	epochMultiplier, err = k.CalculateEpochMultiplier(ctx)
	if err != nil {
		return sdk.Coin{}, math.LegacyDec{}, math.LegacyDec{}, fmt.Errorf("failed to calculate epoch multiplier: %w", err)
	}

	// Apply epoch multiplier to base fee
	dynamicFeeAmount := math.LegacyNewDecFromInt(baseFee.Amount).Mul(epochMultiplier).TruncateInt()
	dynamicFee := sdk.NewCoin(baseFee.Denom, dynamicFeeAmount)

	// LAYER 3: Calculate C-Score discount
	cscoreDiscount, err = k.CalculateCScoreDiscount(ctx, contributor)
	if err != nil {
		return sdk.Coin{}, math.LegacyDec{}, math.LegacyDec{}, fmt.Errorf("failed to calculate cscore discount: %w", err)
	}

	// Apply discount: final = dynamic * (1 - discount)
	discountMultiplier := math.LegacyOneDec().Sub(cscoreDiscount)
	finalFeeAmount := math.LegacyNewDecFromInt(dynamicFee.Amount).Mul(discountMultiplier).TruncateInt()
	finalFee = sdk.NewCoin(dynamicFee.Denom, finalFeeAmount)

	// LAYER 4: Enforce minimum fee floor
	minimumFee := params.MinimumSubmissionFee
	if finalFee.Amount.LT(minimumFee.Amount) {
		k.Logger().Debug("fee below minimum, applying floor",
			"calculated_fee", finalFee,
			"minimum_fee", minimumFee,
			"contributor", contributor.String(),
		)
		finalFee = minimumFee
	}

	return finalFee, epochMultiplier, cscoreDiscount, nil
}

// CalculateEpochMultiplier computes the dynamic congestion multiplier
//
// Formula: max(0.8, min(5.0, current_submissions / target_submissions))
//
// - If block is quiet (few submissions): multiplier < 1.0 (discount)
// - If block is at target: multiplier = 1.0 (no change)
// - If block is congested: multiplier > 1.0 (premium)
//
// Bounds: [0.8, 5.0]
func (k Keeper) CalculateEpochMultiplier(ctx context.Context) (math.LegacyDec, error) {
	params := k.GetParams(ctx)

	// Get current block submission count from transient store
	currentSubmissions := k.GetCurrentBlockSubmissions(ctx)
	targetSubmissions := params.TargetSubmissionsPerBlock

	if targetSubmissions == 0 {
		return math.LegacyZeroDec(), fmt.Errorf("target_submissions_per_block cannot be zero")
	}

	// Calculate raw multiplier: current / target
	currentDec := math.LegacyNewDec(int64(currentSubmissions))
	targetDec := math.LegacyNewDec(int64(targetSubmissions))
	rawMultiplier := currentDec.Quo(targetDec)

	// Apply bounds: max(0.8, min(5.0, raw_multiplier))
	minMultiplier := math.LegacyMustNewDecFromStr("0.8")
	maxMultiplier := math.LegacyMustNewDecFromStr("5.0")

	// If raw < min, use min
	if rawMultiplier.LT(minMultiplier) {
		return minMultiplier, nil
	}

	// If raw > max, use max
	if rawMultiplier.GT(maxMultiplier) {
		return maxMultiplier, nil
	}

	// Otherwise use raw
	return rawMultiplier, nil
}

// CalculateCScoreDiscount computes the reputation-based discount
//
// Formula: min(MaxCscoreDiscount, CScore / 1000)
//
// C-Score range: 0-1000
// - C-Score 0: 0% discount
// - C-Score 500: 50% discount (if max allows)
// - C-Score 1000: 90% discount (capped by MaxCscoreDiscount)
//
// Returns: discount percentage as LegacyDec (0.0 - 0.9)
func (k Keeper) CalculateCScoreDiscount(ctx context.Context, contributor sdk.AccAddress) (math.LegacyDec, error) {
	params := k.GetParams(ctx)

	// Get contributor's C-Score
	credits := k.GetCredits(ctx, contributor)
	cscore := credits.Amount

	// C-Score is in range 0-1000
	// Calculate discount: cscore / 1000
	cscoreDec := math.LegacyNewDecFromInt(cscore)
	maxCScore := math.LegacyNewDec(1000)
	rawDiscount := cscoreDec.Quo(maxCScore)

	// Cap at MaxCscoreDiscount
	maxDiscount := params.MaxCscoreDiscount
	if rawDiscount.GT(maxDiscount) {
		return maxDiscount, nil
	}

	return rawDiscount, nil
}

// GetCurrentBlockSubmissions retrieves the submission count for the current block
// This is stored in a transient store that resets every block
func (k Keeper) GetCurrentBlockSubmissions(ctx context.Context) uint32 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.TransientStore(k.tStoreKey)

	bz := store.Get(SubmissionCounterKey)
	if bz == nil {
		return 0
	}

	// Decode uint32 from bytes
	if len(bz) != 4 {
		k.Logger().Error("invalid submission counter bytes", "length", len(bz))
		return 0
	}

	count := uint32(bz[0]) | uint32(bz[1])<<8 | uint32(bz[2])<<16 | uint32(bz[3])<<24
	return count
}

// IncrementBlockSubmissions increments the submission counter for the current block
// Called by msg_server when processing a submission
func (k Keeper) IncrementBlockSubmissions(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.TransientStore(k.tStoreKey)

	currentCount := k.GetCurrentBlockSubmissions(ctx)
	newCount := currentCount + 1

	// Encode uint32 to bytes (little-endian)
	bz := make([]byte, 4)
	bz[0] = byte(newCount)
	bz[1] = byte(newCount >> 8)
	bz[2] = byte(newCount >> 16)
	bz[3] = byte(newCount >> 24)

	store.Set(SubmissionCounterKey, bz)

	k.Logger().Debug("incremented block submissions",
		"previous_count", currentCount,
		"new_count", newCount,
		"block_height", sdkCtx.BlockHeight(),
	)
}

// ResetBlockSubmissions resets the submission counter
// Called at the beginning of each block (BeginBlocker)
// Note: Transient store automatically resets, but we call this for logging/debugging
func (k Keeper) ResetBlockSubmissions(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.TransientStore(k.tStoreKey)

	previousCount := k.GetCurrentBlockSubmissions(ctx)

	// Delete the counter (will start at 0 next time)
	store.Delete(SubmissionCounterKey)

	k.Logger().Debug("reset block submissions counter",
		"previous_count", previousCount,
		"block_height", sdkCtx.BlockHeight(),
	)
}

// CollectAndSplit3LayerFee collects the calculated fee and splits it
//
// Split logic:
// - 50% burned (sent to burn module)
// - 50% to PoC reward pool
//
// This replaces the old CollectAndBurnSubmissionFee function
func (k Keeper) CollectAndSplit3LayerFee(
	ctx context.Context,
	contributor sdk.AccAddress,
	fee sdk.Coin,
	epochMultiplier math.LegacyDec,
	cscoreDiscount math.LegacyDec,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Collect fee from contributor
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx,
		contributor,
		types.ModuleName,
		sdk.NewCoins(fee),
	); err != nil {
		return fmt.Errorf("failed to collect fee: %w", err)
	}

	// Calculate split: 50% burn, 50% pool
	burnRatio := math.LegacyMustNewDecFromStr("0.5") // 50%
	burnAmount := math.LegacyNewDecFromInt(fee.Amount).Mul(burnRatio).TruncateInt()
	poolAmount := fee.Amount.Sub(burnAmount) // Remaining goes to pool

	burnCoin := sdk.NewCoin(fee.Denom, burnAmount)
	poolCoin := sdk.NewCoin(fee.Denom, poolAmount)

	// Burn 50%
	if !burnCoin.IsZero() {
		if err := k.bankKeeper.BurnCoins(sdkCtx, types.ModuleName, sdk.NewCoins(burnCoin)); err != nil {
			return fmt.Errorf("failed to burn fee: %w", err)
		}
	}

	// Keep 50% in module account as reward pool
	// (it stays in the module account, no transfer needed)

	// Emit event with fee details
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_3layer_fee",
			sdk.NewAttribute("contributor", contributor.String()),
			sdk.NewAttribute("total_fee", fee.String()),
			sdk.NewAttribute("burned", burnCoin.String()),
			sdk.NewAttribute("to_pool", poolCoin.String()),
			sdk.NewAttribute("epoch_multiplier", epochMultiplier.String()),
			sdk.NewAttribute("cscore_discount", cscoreDiscount.String()),
		),
	})

	k.Logger().Info("collected 3-layer fee",
		"contributor", contributor.String(),
		"total_fee", fee,
		"burned", burnCoin,
		"to_pool", poolCoin,
		"epoch_multiplier", epochMultiplier,
		"cscore_discount", cscoreDiscount,
	)

	return nil
}
