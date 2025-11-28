package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// BurnTier represents a fee threshold and corresponding burn rate
type BurnTier struct {
	MinGasPrice math.LegacyDec
	MaxGasPrice math.LegacyDec
	BurnRate    math.LegacyDec
	Name        string
}

// GetBurnTiers returns the adaptive burn tiers
// Can be governance-controlled via params in the future
func (k Keeper) GetBurnTiers() []BurnTier {
	return []BurnTier{
		{
			MinGasPrice: math.LegacyZeroDec(),
			MaxGasPrice: math.LegacyMustNewDecFromStr("0.01"),
			BurnRate:    math.LegacyMustNewDecFromStr("0.50"), // 50%
			Name:        "low_fee",
		},
		{
			MinGasPrice: math.LegacyMustNewDecFromStr("0.01"),
			MaxGasPrice: math.LegacyMustNewDecFromStr("0.05"),
			BurnRate:    math.LegacyMustNewDecFromStr("0.75"), // 75%
			Name:        "mid_fee",
		},
		{
			MinGasPrice: math.LegacyMustNewDecFromStr("0.05"),
			MaxGasPrice: math.LegacyMustNewDecFromStr("1000000"), // No upper limit
			BurnRate:    math.LegacyMustNewDecFromStr("0.90"), // 90%
			Name:        "high_fee",
		},
	}
}

// CalculateBurnRate determines the burn rate based on gas price
// Returns the burn rate as a decimal (0.50 = 50%)
func (k Keeper) CalculateBurnRate(gasPrice math.LegacyDec) (math.LegacyDec, string) {
	tiers := k.GetBurnTiers()

	for _, tier := range tiers {
		if gasPrice.GTE(tier.MinGasPrice) && gasPrice.LT(tier.MaxGasPrice) {
			return tier.BurnRate, tier.Name
		}
	}

	// Default to highest tier if no match (shouldn't happen)
	return math.LegacyMustNewDecFromStr("0.90"), "high_fee"
}

// ProcessTransactionFees processes transaction fees with adaptive burn logic
// This should be called in the fee deduction ante handler
func (k Keeper) ProcessTransactionFees(ctx context.Context, fees sdk.Coins, gasPrice math.LegacyDec) error {
	if fees.IsZero() {
		return nil
	}

	// Only process the native denom
	feeAmount := fees.AmountOf(types.BondDenom)

	if feeAmount.IsZero() {
		return nil
	}

	// Calculate burn rate based on gas price
	burnRate, tierName := k.CalculateBurnRate(gasPrice)

	// Convert to decimal for calculations
	feeAmountDec := math.LegacyNewDecFromInt(feeAmount)

	// 10% always goes to treasury (before burn calculation)
	treasuryRate := math.LegacyMustNewDecFromStr("0.10")
	treasuryAmount := feeAmountDec.Mul(treasuryRate).TruncateInt()

	// Remaining amount subject to burn
	remainingAmount := feeAmount.Sub(treasuryAmount)
	remainingAmountDec := math.LegacyNewDecFromInt(remainingAmount)

	// Calculate burn amount
	burnAmount := remainingAmountDec.Mul(burnRate).TruncateInt()

	// Remaining goes to validators (or community pool)
	validatorAmount := remainingAmount.Sub(burnAmount)

	k.Logger(ctx).Info("Processing transaction fees",
		"total_fee", feeAmount.String(),
		"gas_price", gasPrice.String(),
		"tier", tierName,
		"burn_rate", burnRate.String(),
		"burn_amount", burnAmount.String(),
		"treasury_amount", treasuryAmount.String(),
		"validator_amount", validatorAmount.String())

	// Execute burns
	if burnAmount.IsPositive() {
		if err := k.BurnFromFees(ctx, burnAmount, tierName); err != nil {
			return fmt.Errorf("failed to burn fees: %w", err)
		}
	}

	// Send to treasury
	if treasuryAmount.IsPositive() {
		if err := k.SendToTreasury(ctx, treasuryAmount); err != nil {
			return fmt.Errorf("failed to send to treasury: %w", err)
		}
	}

	// Note: validator_amount stays in fee collector and is distributed by distribution module
	// No action needed here

	return nil
}

// BurnFromFees burns tokens from collected fees
func (k Keeper) BurnFromFees(ctx context.Context, amount math.Int, tier string) error {
	params := k.GetParams(ctx)
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))

	// Burn from fee collector module
	if err := k.bankKeeper.BurnCoins(ctx, "fee_collector", coins); err != nil {
		return err
	}

	// Update total burned
	params.TotalBurned = params.TotalBurned.Add(amount)
	params.CurrentTotalSupply = params.CurrentTotalSupply.Sub(amount)

	if err := k.SetParams(ctx, params); err != nil {
		return err
	}

	// Emit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBurn,
			sdk.NewAttribute(types.AttributeKeyBurnAmount, amount.String()),
			sdk.NewAttribute(types.AttributeKeyBurnSource, "fee_burn_"+tier),
			sdk.NewAttribute("tier", tier),
		),
	)

	k.Logger(ctx).Info("Burned fees",
		"amount", amount.String(),
		"tier", tier,
		"new_total_burned", params.TotalBurned.String())

	return nil
}

// SendToTreasury sends tokens to the treasury address
func (k Keeper) SendToTreasury(ctx context.Context, amount math.Int) error {
	treasuryAddr := k.GetTreasuryAddress(ctx)

	if treasuryAddr.Empty() {
		// No treasury configured, send to community pool or keep in fee collector
		k.Logger(ctx).Warn("Treasury address not configured, skipping treasury transfer")
		return nil
	}

	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))

	// Send from fee collector to treasury
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, "fee_collector", treasuryAddr, coins); err != nil {
		return err
	}

	k.Logger(ctx).Info("Sent fees to treasury",
		"amount", amount.String(),
		"treasury", treasuryAddr.String())

	return nil
}

// GetBurnStatsByTier returns burn statistics by tier
func (k Keeper) GetBurnStatsByTier(ctx context.Context) map[string]math.Int {
	// This would require tracking burns by tier in state
	// For now, return empty map - can be implemented with additional state tracking
	return make(map[string]math.Int)
}

// EstimateBurnForGasPrice estimates the burn amount for a given gas price and fee
func (k Keeper) EstimateBurnForGasPrice(gasPrice math.LegacyDec, totalFee math.Int) (burnAmount, treasuryAmount, validatorAmount math.Int, tier string) {
	// Calculate burn rate
	burnRate, tierName := k.CalculateBurnRate(gasPrice)

	// Convert to decimal
	totalFeeDec := math.LegacyNewDecFromInt(totalFee)

	// 10% to treasury
	treasuryRate := math.LegacyMustNewDecFromStr("0.10")
	treasuryAmt := totalFeeDec.Mul(treasuryRate).TruncateInt()

	// Remaining for burn/validators
	remaining := totalFee.Sub(treasuryAmt)
	remainingDec := math.LegacyNewDecFromInt(remaining)

	// Burn amount
	burnAmt := remainingDec.Mul(burnRate).TruncateInt()

	// Validator amount
	validatorAmt := remaining.Sub(burnAmt)

	return burnAmt, treasuryAmt, validatorAmt, tierName
}
