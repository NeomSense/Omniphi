package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// BurnCalculation contains the results of the unified burn calculation
type BurnCalculation struct {
	BaseBurnRate     math.LegacyDec // Base rate from utilization tier
	ActivityMultiplier math.LegacyDec // Multiplier based on activity type
	EffectiveBurnRate  math.LegacyDec // Final rate after multiplier and cap
	TierName         string         // Name of the utilization tier
	BurnAmount       math.Int       // Actual amount to burn
	ValidatorAmount  math.Int       // Amount for validators
	TreasuryAmount   math.Int       // Amount for treasury
	WasCapped        bool           // True if effective rate was capped
}

// ComputeEffectiveBurn implements the single-pass multiplicative burn model.
// This is the ONLY function that should calculate burn amounts.
//
// Algorithm:
// 1. Get base burn rate from utilization tier (10%, 20%, or 40%)
// 2. Apply activity multiplier (0.5x to 2.0x)
// 3. Cap at MaxBurnRatio (50%)
// 4. Calculate burn, validator, and treasury amounts
//
// This replaces the old additive burn model which caused double-counting.
func (k Keeper) ComputeEffectiveBurn(
	ctx context.Context,
	totalFee math.Int,
	activityType types.ActivityType,
) BurnCalculation {
	params := k.GetParams(ctx)
	
	// Step 1: Get base burn rate from utilization tier
	baseBurnRate, tierName := k.SelectBurnTier(ctx)
	
	// Step 2: Get activity multiplier
	activityMultiplier := params.GetActivityMultiplier(activityType)
	
	// Step 3: Calculate effective burn rate (base * multiplier)
	effectiveBurnRate := baseBurnRate.Mul(activityMultiplier)
	wasCapped := false
	
	// Step 4: Apply max burn cap (PROTOCOL ENFORCED: 50% max)
	if effectiveBurnRate.GT(params.MaxBurnRatio) {
		effectiveBurnRate = params.MaxBurnRatio
		wasCapped = true
		k.Logger(ctx).Debug("burn rate capped",
			"calculated", baseBurnRate.Mul(activityMultiplier).String(),
			"capped_to", effectiveBurnRate.String(),
			"activity", string(activityType),
		)
	}
	
	// Step 5: Calculate amounts
	totalFeeDec := math.LegacyNewDecFromInt(totalFee)
	burnAmount := effectiveBurnRate.Mul(totalFeeDec).TruncateInt()
	if burnAmount.IsNegative() {
		burnAmount = math.ZeroInt()
	}
	
	// Distributable amount is what remains after burn
	distributableAmount := totalFee.Sub(burnAmount)
	if distributableAmount.IsNegative() {
		distributableAmount = math.ZeroInt()
	}
	
	distributableDec := math.LegacyNewDecFromInt(distributableAmount)
	
	// Treasury gets their ratio of post-burn fees
	treasuryAmount := params.TreasuryFeeRatio.Mul(distributableDec).TruncateInt()
	if treasuryAmount.IsNegative() {
		treasuryAmount = math.ZeroInt()
	}
	
	// Validators get the rest
	validatorAmount := distributableAmount.Sub(treasuryAmount)
	if validatorAmount.IsNegative() {
		validatorAmount = math.ZeroInt()
	}
	
	return BurnCalculation{
		BaseBurnRate:       baseBurnRate,
		ActivityMultiplier: activityMultiplier,
		EffectiveBurnRate:  effectiveBurnRate,
		TierName:           tierName,
		BurnAmount:         burnAmount,
		ValidatorAmount:    validatorAmount,
		TreasuryAmount:     treasuryAmount,
		WasCapped:          wasCapped,
	}
}

// EmitBurnCalculationEvent emits a detailed event for the burn calculation
func (k Keeper) EmitBurnCalculationEvent(ctx context.Context, calc BurnCalculation, activityType types.ActivityType) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeUnifiedBurn,
			sdk.NewAttribute(types.AttributeKeyActivityType, string(activityType)),
			sdk.NewAttribute(types.AttributeKeyBurnTier, calc.TierName),
			sdk.NewAttribute(types.AttributeKeyBaseBurnRate, calc.BaseBurnRate.String()),
			sdk.NewAttribute(types.AttributeKeyActivityMultiplier, calc.ActivityMultiplier.String()),
			sdk.NewAttribute(types.AttributeKeyEffectiveBurnRate, calc.EffectiveBurnRate.String()),
			sdk.NewAttribute(types.AttributeKeyBurnAmount, calc.BurnAmount.String()),
			sdk.NewAttribute(types.AttributeKeyValidatorAmount, calc.ValidatorAmount.String()),
			sdk.NewAttribute(types.AttributeKeyTreasuryAmount, calc.TreasuryAmount.String()),
			sdk.NewAttribute(types.AttributeKeyWasCapped, boolToString(calc.WasCapped)),
		),
	)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
