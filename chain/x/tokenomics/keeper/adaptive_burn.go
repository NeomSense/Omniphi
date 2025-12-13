package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// DEPRECATED: Burn logic has been centralized in x/feemarket module.
// This file is kept for backwards compatibility but all burn calculations
// should now use the feemarket keeper's ComputeEffectiveBurn function.
//
// The new single-pass burn model:
// 1. Base burn rate from network utilization (10%, 20%, or 40%)
// 2. Activity multiplier (0.5x to 2.0x based on tx type)
// 3. Effective burn = min(base * multiplier, 50%)
//
// This eliminates the old double-counting problem where both
// adaptive burns AND activity-based burns were applied.

// GetBurnStatsByTier returns burn statistics by tier
// DEPRECATED: Use feemarket module for burn statistics
func (k Keeper) GetBurnStatsByTier(ctx context.Context) map[string]math.Int {
	return make(map[string]math.Int)
}

// UpdateSupplyAfterBurn updates the tokenomics supply tracking after a burn
// This is called by feemarket module after executing burns
func (k Keeper) UpdateSupplyAfterBurn(ctx context.Context, burnAmount math.Int) error {
	if burnAmount.IsZero() || burnAmount.IsNegative() {
		return nil
	}

	params := k.GetParams(ctx)
	
	// Update total burned
	params.TotalBurned = params.TotalBurned.Add(burnAmount)
	
	// Update current supply
	params.CurrentTotalSupply = params.CurrentTotalSupply.Sub(burnAmount)
	
	// Validate conservation law
	expected := params.TotalMinted.Sub(params.TotalBurned)
	if !params.CurrentTotalSupply.Equal(expected) {
		k.Logger(ctx).Error("supply conservation violation after burn",
			"current", params.CurrentTotalSupply.String(),
			"expected", expected.String(),
		)
	}
	
	if err := k.SetParams(ctx, params); err != nil {
		return err
	}
	
	// Emit burn event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBurn,
			sdk.NewAttribute(types.AttributeKeyBurnAmount, burnAmount.String()),
			sdk.NewAttribute(types.AttributeKeyBurnSource, "unified_feemarket"),
			sdk.NewAttribute("new_total_burned", params.TotalBurned.String()),
			sdk.NewAttribute("new_supply", params.CurrentTotalSupply.String()),
		),
	)
	
	k.Logger(ctx).Info("supply updated after burn",
		"burn_amount", burnAmount.String(),
		"new_total_burned", params.TotalBurned.String(),
		"current_supply", params.CurrentTotalSupply.String(),
	)
	
	return nil
}

// GetCurrentBurnStats returns current burn statistics
func (k Keeper) GetCurrentBurnStats(ctx context.Context) (totalBurned, currentSupply, totalMinted math.Int) {
	params := k.GetParams(ctx)
	return params.TotalBurned, params.CurrentTotalSupply, params.TotalMinted
}
