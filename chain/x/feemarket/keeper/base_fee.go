package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// UpdateBaseFee updates the base fee based on previous block utilization
// Implements EIP-1559 style dynamic pricing
// Called in BeginBlock
func (k Keeper) UpdateBaseFee(ctx context.Context) error {
	params := k.GetParams(ctx)

	// Skip if base fee mechanism is disabled
	if !params.BaseFeeEnabled {
		k.Logger(ctx).Debug("base fee mechanism disabled, skipping update")
		return nil
	}

	currentBaseFee := k.GetCurrentBaseFee(ctx)
	prevUtilization := k.GetPreviousBlockUtilization(ctx)
	targetUtilization := params.TargetBlockUtilization

	k.Logger(ctx).Debug("updating base fee",
		"current_base_fee", currentBaseFee.String(),
		"prev_utilization", prevUtilization.String(),
		"target_utilization", targetUtilization.String(),
	)

	var newBaseFee math.LegacyDec

	// EIP-1559 algorithm:
	// - If utilization > target: increase base fee
	// - If utilization < target: decrease base fee
	// - If utilization == target: keep base fee same

	if prevUtilization.GT(targetUtilization) {
		// Increase base fee (network congested)
		newBaseFee = currentBaseFee.Mul(params.ElasticityMultiplier)
		k.Logger(ctx).Debug("increasing base fee",
			"reason", "utilization above target",
			"multiplier", params.ElasticityMultiplier.String(),
		)
	} else if prevUtilization.LT(targetUtilization) {
		// Decrease base fee (network underutilized)
		newBaseFee = currentBaseFee.Quo(params.ElasticityMultiplier)
		k.Logger(ctx).Debug("decreasing base fee",
			"reason", "utilization below target",
			"divisor", params.ElasticityMultiplier.String(),
		)
	} else {
		// Keep base fee same (perfect utilization)
		newBaseFee = currentBaseFee
		k.Logger(ctx).Debug("maintaining base fee",
			"reason", "utilization at target",
		)
	}

	// Apply minimum gas price floor
	if newBaseFee.LT(params.MinGasPriceFloor) {
		k.Logger(ctx).Debug("base fee below minimum floor, applying floor",
			"calculated", newBaseFee.String(),
			"floor", params.MinGasPriceFloor.String(),
		)
		newBaseFee = params.MinGasPriceFloor
	}

	// Update base fee in state
	if err := k.SetCurrentBaseFee(ctx, newBaseFee); err != nil {
		k.Logger(ctx).Error("failed to set current base fee", "error", err)
		return err
	}

	// Emit base fee update event
	k.EmitBaseFeeUpdateEvent(ctx, currentBaseFee, newBaseFee, prevUtilization)

	k.Logger(ctx).Info("base fee updated",
		"old_base_fee", currentBaseFee.String(),
		"new_base_fee", newBaseFee.String(),
		"utilization", prevUtilization.String(),
	)

	return nil
}

// GetEffectiveGasPrice returns the effective gas price for transactions
// Combines base fee and minimum gas price
func (k Keeper) GetEffectiveGasPrice(ctx context.Context) math.LegacyDec {
	params := k.GetParams(ctx)

	if !params.BaseFeeEnabled {
		// If EIP-1559 is disabled, use static minimum gas price
		return params.MinGasPrice
	}

	// Effective gas price is max(base_fee, min_gas_price)
	baseFee := k.GetCurrentBaseFee(ctx)
	if baseFee.LT(params.MinGasPrice) {
		return params.MinGasPrice
	}

	return baseFee
}

// CalculateEffectiveFee calculates the effective fee for a transaction
// given gas used and optional tip
func (k Keeper) CalculateEffectiveFee(ctx context.Context, gasUsed uint64, tip math.LegacyDec) math.Int {
	// Get effective base gas price
	effectiveGasPrice := k.GetEffectiveGasPrice(ctx)

	// Add tip if provided
	if tip.IsPositive() {
		effectiveGasPrice = effectiveGasPrice.Add(tip)
	}

	// Calculate total fee: effective_gas_price * gas_used
	totalFee := effectiveGasPrice.MulInt64(int64(gasUsed)).TruncateInt()

	return totalFee
}

// EmitBaseFeeUpdateEvent emits an event when base fee is updated
func (k Keeper) EmitBaseFeeUpdateEvent(ctx context.Context, oldBaseFee, newBaseFee, utilization math.LegacyDec) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBaseFeeUpdate,
			sdk.NewAttribute(types.AttributeKeyOldBaseFee, oldBaseFee.String()),
			sdk.NewAttribute(types.AttributeKeyNewBaseFee, newBaseFee.String()),
			sdk.NewAttribute(types.AttributeKeyUtilization, utilization.String()),
		),
	)
}
