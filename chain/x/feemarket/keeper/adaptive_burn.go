package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// SelectBurnTier determines the current burn tier based on block utilization
// Returns the burn percentage and tier name ("cool", "normal", or "hot")
func (k Keeper) SelectBurnTier(ctx context.Context) (math.LegacyDec, string) {
	params := k.GetParams(ctx)
	utilization := k.GetBlockUtilization(ctx)

	// Tier 1: Cool (low utilization < 30%)
	// Burns 10% of collected fees
	if utilization.LT(params.UtilCoolThreshold) {
		return params.BurnCool, "cool"
	}

	// Tier 3: Hot (high utilization >= 70%)
	// Burns 40% of collected fees
	if utilization.GTE(params.UtilHotThreshold) {
		return params.BurnHot, "hot"
	}

	// Tier 2: Normal (medium utilization 30-70%)
	// Burns 20% of collected fees
	return params.BurnNormal, "normal"
}

// GetCurrentBurnRate returns the current burn rate with safety checks
// Ensures burn rate never exceeds the configured maximum
func (k Keeper) GetCurrentBurnRate(ctx context.Context) math.LegacyDec {
	burnRate, _ := k.SelectBurnTier(ctx)
	params := k.GetParams(ctx)

	// Apply maximum burn ratio safety cap (default 50%)
	if burnRate.GT(params.MaxBurnRatio) {
		k.Logger(ctx).Warn("burn rate exceeds maximum, capping",
			"calculated", burnRate.String(),
			"max", params.MaxBurnRatio.String(),
		)
		return params.MaxBurnRatio
	}

	return burnRate
}

// EmitBurnTierEvent emits an event when the burn tier changes
func (k Keeper) EmitBurnTierEvent(ctx context.Context, tier string, burnPct math.LegacyDec, utilization math.LegacyDec) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBurnTierChange,
			sdk.NewAttribute(types.AttributeKeyBurnTier, tier),
			sdk.NewAttribute(types.AttributeKeyBurnPercentage, burnPct.String()),
			sdk.NewAttribute(types.AttributeKeyUtilization, utilization.String()),
		),
	)
}
