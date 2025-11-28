package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"pos/x/poc/types"
)

// CollectAndBurnSubmissionFee handles the submission fee collection and burn/reward split.
// This is the core function called during contribution submission.
//
// SECURITY: This function is atomic - if any step fails, entire operation reverts
// AUDIT TRAIL: Emits detailed events for transparency
//
// Flow:
// 1. Collect submission_fee from contributor
// 2. Calculate burn amount (fee × submission_burn_ratio)
// 3. Calculate reward amount (fee - burn_amount)
// 4. Burn coins (reduces total supply)
// 5. Keep remainder in module account for rewards
// 6. Update global fee metrics
// 7. Update contributor-specific stats
// 8. Emit events
func (k Keeper) CollectAndBurnSubmissionFee(
	ctx context.Context,
	contributor sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Step 1: Collect fee from contributor
	feeCoins := sdk.NewCoins(params.SubmissionFee)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		contributor,
		types.ModuleName,
		feeCoins,
	); err != nil {
		return fmt.Errorf("%w: failed to collect submission fee from %s: %v", types.ErrInsufficientFee, contributor, err)
	}

	// Step 2: Calculate burn and reward amounts
	feeAmountDec := math.LegacyNewDecFromInt(params.SubmissionFee.Amount)
	burnAmountDec := feeAmountDec.Mul(params.SubmissionBurnRatio)
	burnAmount := burnAmountDec.TruncateInt()
	rewardAmount := params.SubmissionFee.Amount.Sub(burnAmount)

	burnCoins := sdk.NewCoins(sdk.NewCoin(params.SubmissionFee.Denom, burnAmount))
	rewardCoins := sdk.NewCoins(sdk.NewCoin(params.SubmissionFee.Denom, rewardAmount))

	// Step 3: Burn coins (reduces total supply - deflationary mechanism)
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
		return fmt.Errorf("failed to burn submission fee: %w", err)
	}

	// Step 4: Reward amount stays in module account for later distribution
	// The module account acts as the PoC reward pool

	// Step 5: Update global fee metrics (atomic)
	if err := k.UpdateFeeMetrics(ctx, feeCoins, burnCoins, rewardCoins); err != nil {
		return fmt.Errorf("failed to update fee metrics: %w", err)
	}

	// Step 6: Update contributor-specific stats (atomic)
	if err := k.UpdateContributorFeeStats(ctx, contributor, feeCoins, burnCoins); err != nil {
		return fmt.Errorf("failed to update contributor fee stats: %w", err)
	}

	// Step 7: Emit detailed events for transparency and audit trail
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_submission_fee_collected",
			sdk.NewAttribute("contributor", contributor.String()),
			sdk.NewAttribute("fee_paid", feeCoins.String()),
			sdk.NewAttribute("burned", burnCoins.String()),
			sdk.NewAttribute("to_rewards", rewardCoins.String()),
			sdk.NewAttribute("burn_ratio", params.SubmissionBurnRatio.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	})

	return nil
}

// UpdateFeeMetrics updates the global cumulative fee metrics.
// This provides transparency and audit trail for all fee operations.
func (k Keeper) UpdateFeeMetrics(
	ctx context.Context,
	feesCollected, burned, rewardRedirect sdk.Coins,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	metrics := k.GetFeeMetrics(ctx)

	// Add to cumulative totals
	metrics.TotalFeesCollected = metrics.TotalFeesCollected.Add(feesCollected...)
	metrics.TotalBurned = metrics.TotalBurned.Add(burned...)
	metrics.TotalRewardRedirect = metrics.TotalRewardRedirect.Add(rewardRedirect...)
	metrics.LastUpdatedHeight = sdkCtx.BlockHeight()

	k.SetFeeMetrics(ctx, metrics)
	return nil
}

// UpdateContributorFeeStats updates per-contributor fee statistics.
// This enables analytics on contributor behavior and future fee rebate mechanisms.
func (k Keeper) UpdateContributorFeeStats(
	ctx context.Context,
	contributor sdk.AccAddress,
	feesPaid, burned sdk.Coins,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	stats := k.GetContributorFeeStats(ctx, contributor)

	// Initialize if first submission
	if stats.SubmissionCount == 0 {
		stats.Address = contributor.String()
		stats.FirstSubmissionHeight = sdkCtx.BlockHeight()
	}

	// Update cumulative stats
	stats.TotalFeesPaid = stats.TotalFeesPaid.Add(feesPaid...)
	stats.TotalBurned = stats.TotalBurned.Add(burned...)
	stats.SubmissionCount++
	stats.LastSubmissionHeight = sdkCtx.BlockHeight()

	k.SetContributorFeeStats(ctx, stats)
	return nil
}

// GetFeeMetrics retrieves global fee metrics from state.
// Returns empty metrics if not found (first submission case).
func (k Keeper) GetFeeMetrics(ctx context.Context) types.FeeMetrics {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyFeeMetrics)
	if err != nil || bz == nil {
		// Return empty metrics if not found (first time)
		return types.FeeMetrics{
			TotalFeesCollected:  sdk.NewCoins(),
			TotalBurned:         sdk.NewCoins(),
			TotalRewardRedirect: sdk.NewCoins(),
			LastUpdatedHeight:   0,
		}
	}

	var metrics types.FeeMetrics
	k.cdc.MustUnmarshal(bz, &metrics)
	return metrics
}

// SetFeeMetrics stores global fee metrics to state.
func (k Keeper) SetFeeMetrics(ctx context.Context, metrics types.FeeMetrics) {
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&metrics)
	if err := store.Set(types.KeyFeeMetrics, bz); err != nil {
		panic(fmt.Sprintf("failed to set fee metrics: %v", err))
	}
}

// GetContributorFeeStats retrieves contributor-specific fee stats from state.
// Returns empty stats if not found (first submission case).
func (k Keeper) GetContributorFeeStats(ctx context.Context, addr sdk.AccAddress) types.ContributorFeeStats {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetContributorFeeStatsKey(addr)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		// Return empty stats if not found (first time)
		return types.ContributorFeeStats{
			Address:               addr.String(),
			TotalFeesPaid:         sdk.NewCoins(),
			TotalBurned:           sdk.NewCoins(),
			SubmissionCount:       0,
			FirstSubmissionHeight: 0,
			LastSubmissionHeight:  0,
		}
	}

	var stats types.ContributorFeeStats
	k.cdc.MustUnmarshal(bz, &stats)
	return stats
}

// SetContributorFeeStats stores contributor-specific fee stats to state.
func (k Keeper) SetContributorFeeStats(ctx context.Context, stats types.ContributorFeeStats) {
	addr, err := sdk.AccAddressFromBech32(stats.Address)
	if err != nil {
		panic(fmt.Sprintf("invalid contributor address in fee stats: %v", err))
	}

	store := k.storeService.OpenKVStore(ctx)
	key := types.GetContributorFeeStatsKey(addr)
	bz := k.cdc.MustMarshal(&stats)
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set contributor fee stats: %v", err))
	}
}

// GetAllContributorFeeStats retrieves all contributor fee stats (for genesis export).
// This iterates through all stored contributor stats.
func (k Keeper) GetAllContributorFeeStats(ctx context.Context) []types.ContributorFeeStats {
	var allStats []types.ContributorFeeStats
	store := k.storeService.OpenKVStore(ctx)

	// Create end key for iteration (prefix + 1)
	end := storetypes.PrefixEndBytes(types.KeyPrefixContributorFeeStats)

	iterator, err := store.Iterator(types.KeyPrefixContributorFeeStats, end)
	if err != nil {
		panic(fmt.Sprintf("failed to create iterator for contributor fee stats: %v", err))
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var stats types.ContributorFeeStats
		k.cdc.MustUnmarshal(iterator.Value(), &stats)
		allStats = append(allStats, stats)
	}

	return allStats
}
