package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// GetSupplyMetrics returns comprehensive supply metrics
func (k Keeper) GetSupplyMetrics(ctx context.Context) types.QuerySupplyResponse {
	params := k.GetParams(ctx)
	currentSupply := k.GetCurrentSupply(ctx)
	totalMinted := k.GetTotalMinted(ctx)
	totalBurned := k.GetTotalBurned(ctx)

	// Calculate remaining mintable
	remainingMintable := params.TotalSupplyCap.Sub(currentSupply)

	// Calculate supply as % of cap
	supplyPct := math.LegacyZeroDec()
	if params.TotalSupplyCap.IsPositive() {
		supplyPct = math.LegacyNewDecFromInt(currentSupply).QuoInt(params.TotalSupplyCap)
	}

	// Calculate net inflation rate (minting - burning)
	netInflationRate := k.CalculateNetInflationRate(ctx)

	return types.QuerySupplyResponse{
		TotalSupplyCap:      params.TotalSupplyCap,
		CurrentTotalSupply:  currentSupply,
		TotalMinted:         totalMinted,
		TotalBurned:         totalBurned,
		RemainingMintable:   remainingMintable,
		SupplyPctOfCap:      supplyPct,
		NetInflationRate:    netInflationRate,
	}
}

// CalculateNetInflationRate calculates the effective growth rate (minting - burning)
func (k Keeper) CalculateNetInflationRate(ctx context.Context) math.LegacyDec {
	params := k.GetParams(ctx)

	// Get average burn rate (weighted by usage)
	// For simplicity, use a conservative estimate based on configured burn rates
	avgBurnRate := params.BurnRatePosGas.
		Add(params.BurnRatePocAnchoring).
		Add(params.BurnRateSequencerGas).
		QuoInt64(3) // Simple average of top 3 sources

	// Net inflation = inflation rate - (inflation rate × avg burn rate)
	// Example: 3% inflation, 50% burn rate → net = 3% × (1 - 0.5) = 1.5%
	netInflation := params.InflationRate.Mul(math.LegacyOneDec().Sub(avgBurnRate))

	return netInflation
}

// CheckSupplyCapWarnings emits warning events if approaching cap
func (k Keeper) CheckSupplyCapWarnings(ctx context.Context, newSupply math.Int) {
	params := k.GetParams(ctx)

	// Calculate supply percentage
	supplyPct := math.LegacyNewDecFromInt(newSupply).QuoInt(params.TotalSupplyCap)

	// Define warning thresholds
	thresholds := []math.LegacyDec{
		math.LegacyNewDecWithPrec(80, 2), // 0.80 = 80%
		math.LegacyNewDecWithPrec(90, 2), // 0.90 = 90%
		math.LegacyNewDecWithPrec(95, 2), // 0.95 = 95%
		math.LegacyNewDecWithPrec(99, 2), // 0.99 = 99%
	}

	// Emit warning if crossed threshold
	for _, threshold := range thresholds {
		if supplyPct.GTE(threshold) {
			k.EmitCapWarning(ctx, threshold, newSupply, params.TotalSupplyCap)
			break // Only emit highest threshold reached
		}
	}
}

// EmitCapWarning emits a warning event when approaching supply cap
func (k Keeper) EmitCapWarning(ctx context.Context, warningLevel math.LegacyDec, currentSupply, cap math.Int) {
	remaining := cap.Sub(currentSupply)

	// Create warning message
	pct := warningLevel.MustFloat64() * 100
	message := fmt.Sprintf(
		"⚠️ Supply cap warning: %.1f%% of cap reached. Remaining mintable: %s OMNI. DAO should review inflation parameters.",
		pct,
		remaining.String(),
	)

	// Log warning
	k.Logger(ctx).Warn(message,
		"warning_level", warningLevel.String(),
		"current_supply", currentSupply.String(),
		"supply_cap", cap.String(),
		"remaining", remaining.String(),
	)

	// P0-CAP-004: Emit SDK event for monitoring
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"supply_cap_warning",
			sdk.NewAttribute("warning_level", fmt.Sprintf("%.1f%%", pct)),
			sdk.NewAttribute("current_supply", currentSupply.String()),
			sdk.NewAttribute("supply_cap", cap.String()),
			sdk.NewAttribute("remaining", remaining.String()),
			sdk.NewAttribute("message", message),
		),
	)
}
