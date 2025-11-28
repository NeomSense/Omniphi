package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// GetAdaptiveBurnRatio calculates the optimal burn ratio based on network conditions
// Priority-ordered trigger logic:
// 1. Emergency Override - returns current fee_burn_ratio
// 2. Adaptive Disabled - returns current fee_burn_ratio
// 3. Treasury < Floor - returns min_burn_ratio (protect treasury)
// 4. Congestion > Threshold - returns max_burn_ratio (combat spam)
// 5. Adoption < Target - returns min_burn_ratio (encourage growth)
// 6. Normal Conditions - returns default_burn_ratio
func (k Keeper) GetAdaptiveBurnRatio(ctx context.Context) (math.LegacyDec, string) {
	params := k.GetParams(ctx)

	// Priority 1: Emergency Override
	if params.EmergencyBurnOverride {
		k.Logger(ctx).Warn("adaptive burn in emergency override mode")
		return params.FeeBurnRatio, "emergency_override"
	}

	// Priority 2: Adaptive Disabled
	if !params.AdaptiveBurnEnabled {
		return params.FeeBurnRatio, "adaptive_disabled"
	}

	// Priority 3: Treasury Below Floor
	treasuryPct := k.GetTreasuryPct(ctx)
	if treasuryPct.LT(params.TreasuryFloorPct) {
		k.Logger(ctx).Info("treasury below floor, reducing burn",
			"treasury_pct", treasuryPct.String(),
			"floor", params.TreasuryFloorPct.String(),
			"burn_ratio", params.MinBurnRatio.String())
		return params.MinBurnRatio, "treasury_protection"
	}

	// Priority 4: High Congestion
	congestion := k.GetBlockCongestion(ctx)
	if congestion.GTE(params.BlockCongestionThreshold) {
		k.Logger(ctx).Info("high congestion detected, increasing burn",
			"congestion", congestion.String(),
			"threshold", params.BlockCongestionThreshold.String(),
			"burn_ratio", params.MaxBurnRatio.String())
		return params.MaxBurnRatio, "congestion_control"
	}

	// Priority 5: Low Adoption
	avgTxPerDay := k.GetAvgTxPerDay(ctx)
	txTarget := math.NewInt(int64(params.TxPerDayTarget))
	if avgTxPerDay.LT(txTarget) {
		k.Logger(ctx).Info("low transaction volume, reducing burn to encourage adoption",
			"avg_tx_per_day", avgTxPerDay.String(),
			"target", txTarget.String(),
			"burn_ratio", params.MinBurnRatio.String())
		return params.MinBurnRatio, "adoption_incentive"
	}

	// Priority 6: Normal Conditions
	return params.DefaultBurnRatio, "normal"
}

// GetBlockCongestion returns the current block gas usage as a percentage (0.0-1.0)
// Based on gas used vs gas limit in recent blocks
func (k Keeper) GetBlockCongestion(ctx context.Context) math.LegacyDec {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get the gas meter for the current block
	// Note: This measures instantaneous congestion
	// For production, should average over last N blocks
	gasMeter := sdkCtx.BlockGasMeter()
	if gasMeter == nil {
		return math.LegacyZeroDec()
	}

	gasLimit := gasMeter.Limit()
	gasConsumed := gasMeter.GasConsumed()

	if gasLimit == 0 {
		return math.LegacyZeroDec()
	}

	// Convert to decimal and calculate percentage
	congestion := math.LegacyNewDec(int64(gasConsumed)).Quo(math.LegacyNewDec(int64(gasLimit)))

	// Cap at 1.0 (100%)
	if congestion.GT(math.LegacyOneDec()) {
		return math.LegacyOneDec()
	}

	return congestion
}

// GetTreasuryPct returns the treasury balance as a percentage of total supply
func (k Keeper) GetTreasuryPct(ctx context.Context) math.LegacyDec {
	params := k.GetParams(ctx)
	treasuryAddr := k.GetTreasuryAddress(ctx)

	if treasuryAddr.Empty() {
		k.Logger(ctx).Warn("treasury address not configured")
		return math.LegacyZeroDec()
	}

	// Get treasury balance
	treasuryBalance := k.bankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom)
	treasuryAmount := treasuryBalance.Amount

	// Calculate percentage of current supply
	currentSupply := params.CurrentTotalSupply
	if currentSupply.IsZero() {
		return math.LegacyZeroDec()
	}

	treasuryPct := math.LegacyNewDecFromInt(treasuryAmount).Quo(math.LegacyNewDecFromInt(currentSupply))
	return treasuryPct
}

// GetAvgTxPerDay returns the 7-day rolling average of transactions per day
// This provides a smoothed metric for adoption tracking
func (k Keeper) GetAvgTxPerDay(ctx context.Context) math.Int {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// For initial implementation, estimate from current block
	// In production, this should track historical tx counts in state

	// Estimate: Average block time is ~6 seconds
	// Blocks per day = 86400 / 6 = 14400 blocks
	currentHeight := sdkCtx.BlockHeight()

	// If we're in early blocks, return 0 to trigger adoption incentive
	if currentHeight < 100 {
		return math.ZeroInt()
	}

	// For now, use a simple heuristic:
	// Count txs in current block and multiply by blocks/day
	// TODO: Implement proper 7-day rolling average with state storage
	txCount := len(sdkCtx.TxBytes())
	if txCount == 0 {
		// Check if we have access to tx count in a different way
		// For now, return a conservative estimate
		return math.NewInt(1000) // Assume some baseline activity
	}

	blocksPerDay := int64(14400) // ~6 second blocks
	estimatedTxPerDay := int64(txCount) * blocksPerDay

	return math.NewInt(estimatedTxPerDay)
}

// ApplySmoothing applies exponential smoothing to prevent rapid burn ratio changes
// Formula: new_ratio = (current_ratio * α) + (target_ratio * (1 - α))
// where α = smoothing_factor based on burn_adjustment_smoothing blocks
func (k Keeper) ApplySmoothing(ctx context.Context, targetRatio math.LegacyDec) math.LegacyDec {
	params := k.GetParams(ctx)
	currentRatio := params.LastAppliedBurnRatio

	// If this is the first application or smoothing is disabled (smoothing = 1)
	if currentRatio.IsZero() || params.BurnAdjustmentSmoothing <= 1 {
		return targetRatio
	}

	// Calculate smoothing factor
	// α = 1 / smoothing_blocks
	// This creates an exponential moving average
	smoothingBlocks := math.LegacyNewDec(int64(params.BurnAdjustmentSmoothing))
	alpha := math.LegacyOneDec().Quo(smoothingBlocks)

	// Smoothed ratio = (1 - α) * current + α * target
	oneMinusAlpha := math.LegacyOneDec().Sub(alpha)
	smoothedRatio := currentRatio.Mul(oneMinusAlpha).Add(targetRatio.Mul(alpha))

	k.Logger(ctx).Debug("applying smoothing to burn ratio",
		"current", currentRatio.String(),
		"target", targetRatio.String(),
		"smoothed", smoothedRatio.String(),
		"alpha", alpha.String())

	return smoothedRatio
}

// UpdateBurnRatio updates the current burn ratio with smoothing and state tracking
// This should be called in BeginBlock
func (k Keeper) UpdateBurnRatio(ctx context.Context) error {
	// Get the target ratio based on current conditions
	targetRatio, trigger := k.GetAdaptiveBurnRatio(ctx)

	// Apply smoothing
	smoothedRatio := k.ApplySmoothing(ctx, targetRatio)

	// Update parameters with new values
	params := k.GetParams(ctx)
	oldRatio := params.LastAppliedBurnRatio
	oldTrigger := params.LastBurnTrigger

	params.LastAppliedBurnRatio = smoothedRatio
	params.LastBurnTrigger = trigger

	if err := k.SetParams(ctx, params); err != nil {
		return fmt.Errorf("failed to update burn ratio params: %w", err)
	}

	// Emit event if trigger or ratio changed significantly
	ratioChanged := !smoothedRatio.Equal(oldRatio)
	triggerChanged := trigger != oldTrigger

	if ratioChanged || triggerChanged {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"adaptive_burn_update",
				sdk.NewAttribute("old_ratio", oldRatio.String()),
				sdk.NewAttribute("new_ratio", smoothedRatio.String()),
				sdk.NewAttribute("target_ratio", targetRatio.String()),
				sdk.NewAttribute("old_trigger", oldTrigger),
				sdk.NewAttribute("new_trigger", trigger),
				sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
			),
		)

		k.Logger(ctx).Info("adaptive burn ratio updated",
			"old_ratio", oldRatio.String(),
			"new_ratio", smoothedRatio.String(),
			"target", targetRatio.String(),
			"trigger", trigger)
	}

	return nil
}

// GetCurrentBurnRatio returns the current effective burn ratio
// This is what fee processing should use
func (k Keeper) GetCurrentBurnRatio(ctx context.Context) math.LegacyDec {
	params := k.GetParams(ctx)

	// If adaptive is disabled or emergency override, use fee_burn_ratio
	if !params.AdaptiveBurnEnabled || params.EmergencyBurnOverride {
		return params.FeeBurnRatio
	}

	// Otherwise use the last applied adaptive ratio
	if params.LastAppliedBurnRatio.IsZero() {
		// First time - return default
		return params.DefaultBurnRatio
	}

	return params.LastAppliedBurnRatio
}
