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

// BlocksPerDay is the estimated number of blocks per day (~6 second block time)
const BlocksPerDay = int64(14400)

// RollingWindowDays is the number of days in the rolling average window
const RollingWindowDays = 7

// GetAvgTxPerDay returns the 7-day rolling average of transactions per day
// This provides a smoothed metric for adoption tracking
func (k Keeper) GetAvgTxPerDay(ctx context.Context) math.Int {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// If we're in early blocks, return 0 to trigger adoption incentive
	if sdkCtx.BlockHeight() < BlocksPerDay {
		return math.ZeroInt()
	}

	// Sum up all 7 days of transaction counts
	var totalTx int64
	var daysWithData int64

	for i := uint8(0); i < RollingWindowDays; i++ {
		dayCount := k.getDailyTxCount(ctx, i)
		if dayCount > 0 {
			totalTx += dayCount
			daysWithData++
		}
	}

	// If no data yet, return 0 to trigger adoption incentive
	if daysWithData == 0 {
		return math.ZeroInt()
	}

	// Calculate average (total / days with data gives us average tx per day)
	avgTxPerDay := totalTx / daysWithData
	return math.NewInt(avgTxPerDay)
}

// RecordBlockTransactions should be called in EndBlock to track transaction counts
// It manages the rolling window, rotating days as needed
func (k Keeper) RecordBlockTransactions(ctx context.Context, txCount int64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Get the last rotation height
	lastRotationHeight := k.getLastDayRotationHeight(ctx)

	// Check if we need to rotate to a new day
	blocksSinceRotation := currentHeight - lastRotationHeight
	if blocksSinceRotation >= BlocksPerDay {
		// Time to rotate: save current day's count and move to next day
		currentDayIndex := k.getCurrentDayIndex(ctx)
		currentDayTxCount := k.getCurrentDayTxCount(ctx)

		// Store the completed day's transaction count
		if err := k.setDailyTxCount(ctx, currentDayIndex, currentDayTxCount); err != nil {
			return fmt.Errorf("failed to set daily tx count: %w", err)
		}

		// Move to next day (circular buffer: 0->1->2->...->6->0)
		nextDayIndex := (currentDayIndex + 1) % RollingWindowDays
		if err := k.setCurrentDayIndex(ctx, nextDayIndex); err != nil {
			return fmt.Errorf("failed to set current day index: %w", err)
		}

		// Reset the new day's count to 0
		if err := k.setCurrentDayTxCount(ctx, 0); err != nil {
			return fmt.Errorf("failed to reset current day tx count: %w", err)
		}

		// Update rotation height
		if err := k.setLastDayRotationHeight(ctx, currentHeight); err != nil {
			return fmt.Errorf("failed to set last day rotation height: %w", err)
		}

		k.Logger(ctx).Debug("rotated tx tracking day",
			"completed_day", currentDayIndex,
			"tx_count", currentDayTxCount,
			"new_day", nextDayIndex)
	}

	// Add this block's transaction count to current day
	currentDayTxCount := k.getCurrentDayTxCount(ctx)
	newCount := currentDayTxCount + txCount
	if err := k.setCurrentDayTxCount(ctx, newCount); err != nil {
		return fmt.Errorf("failed to update current day tx count: %w", err)
	}

	return nil
}

// getDailyTxCount returns the transaction count for a specific day in the rolling window
func (k Keeper) getDailyTxCount(ctx context.Context, dayIndex uint8) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.GetDailyTxCountKey(dayIndex))
	if err != nil || len(bz) == 0 {
		return 0
	}
	return int64(bz[0])<<56 | int64(bz[1])<<48 | int64(bz[2])<<40 | int64(bz[3])<<32 |
		int64(bz[4])<<24 | int64(bz[5])<<16 | int64(bz[6])<<8 | int64(bz[7])
}

// setDailyTxCount sets the transaction count for a specific day
func (k Keeper) setDailyTxCount(ctx context.Context, dayIndex uint8, count int64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(count >> 56)
	bz[1] = byte(count >> 48)
	bz[2] = byte(count >> 40)
	bz[3] = byte(count >> 32)
	bz[4] = byte(count >> 24)
	bz[5] = byte(count >> 16)
	bz[6] = byte(count >> 8)
	bz[7] = byte(count)
	return store.Set(types.GetDailyTxCountKey(dayIndex), bz)
}

// getCurrentDayIndex returns the current day index in the rolling window (0-6)
func (k Keeper) getCurrentDayIndex(ctx context.Context) uint8 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyCurrentDayIndex)
	if err != nil || len(bz) == 0 {
		return 0
	}
	return bz[0]
}

// setCurrentDayIndex sets the current day index
func (k Keeper) setCurrentDayIndex(ctx context.Context, dayIndex uint8) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyCurrentDayIndex, []byte{dayIndex})
}

// getLastDayRotationHeight returns the block height of the last day rotation
func (k Keeper) getLastDayRotationHeight(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyLastDayRotationHeight)
	if err != nil || len(bz) == 0 {
		return 0
	}
	return int64(bz[0])<<56 | int64(bz[1])<<48 | int64(bz[2])<<40 | int64(bz[3])<<32 |
		int64(bz[4])<<24 | int64(bz[5])<<16 | int64(bz[6])<<8 | int64(bz[7])
}

// setLastDayRotationHeight sets the last day rotation height
func (k Keeper) setLastDayRotationHeight(ctx context.Context, height int64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(height >> 56)
	bz[1] = byte(height >> 48)
	bz[2] = byte(height >> 40)
	bz[3] = byte(height >> 32)
	bz[4] = byte(height >> 24)
	bz[5] = byte(height >> 16)
	bz[6] = byte(height >> 8)
	bz[7] = byte(height)
	return store.Set(types.KeyLastDayRotationHeight, bz)
}

// getCurrentDayTxCount returns the accumulated transaction count for the current day
func (k Keeper) getCurrentDayTxCount(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyCurrentDayTxCount)
	if err != nil || len(bz) == 0 {
		return 0
	}
	return int64(bz[0])<<56 | int64(bz[1])<<48 | int64(bz[2])<<40 | int64(bz[3])<<32 |
		int64(bz[4])<<24 | int64(bz[5])<<16 | int64(bz[6])<<8 | int64(bz[7])
}

// setCurrentDayTxCount sets the current day's accumulated transaction count
func (k Keeper) setCurrentDayTxCount(ctx context.Context, count int64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(count >> 56)
	bz[1] = byte(count >> 48)
	bz[2] = byte(count >> 40)
	bz[3] = byte(count >> 32)
	bz[4] = byte(count >> 24)
	bz[5] = byte(count >> 16)
	bz[6] = byte(count >> 8)
	bz[7] = byte(count)
	return store.Set(types.KeyCurrentDayTxCount, bz)
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
