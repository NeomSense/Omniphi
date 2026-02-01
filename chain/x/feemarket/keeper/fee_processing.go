package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"pos/x/feemarket/types"
	tokenomicstypes "pos/x/tokenomics/types"
)

// ProcessBlockFees processes all fees collected in the current block
// Called in EndBlock to:
// 1. Determine burn tier based on utilization
// 2. Burn the appropriate percentage
// 3. Distribute remaining fees between treasury and validators
func (k Keeper) ProcessBlockFees(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Get fee collector module account
	feeCollectorAddr := k.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	if feeCollectorAddr == nil {
		return fmt.Errorf("fee collector address not found")
	}

	// Get total fees collected in the chain's native denomination
	totalFees := k.bankKeeper.GetBalance(ctx, feeCollectorAddr, tokenomicstypes.BondDenom)
	if totalFees.Amount.IsZero() {
		k.Logger(ctx).Debug("no fees collected in this block")
		return nil
	}

	k.Logger(ctx).Info("processing block fees",
		"total_fees", totalFees.Amount.String(),
	)

	// Select burn tier based on current utilization
	burnRate, tierName := k.SelectBurnTier(ctx)
	utilization := k.GetBlockUtilization(ctx)

	// Apply maximum burn ratio safety cap
	if burnRate.GT(params.MaxBurnRatio) {
		k.Logger(ctx).Warn("burn rate exceeds maximum, capping",
			"tier", tierName,
			"calculated_burn", burnRate.String(),
			"max_burn", params.MaxBurnRatio.String(),
		)
		burnRate = params.MaxBurnRatio
	}

	// Calculate burn amount
	burnAmount := burnRate.MulInt(totalFees.Amount).TruncateInt()
	if burnAmount.IsNegative() {
		burnAmount = math.ZeroInt()
	}

	// Calculate distributable amount (what remains after burn)
	distributableAmount := totalFees.Amount.Sub(burnAmount)
	if distributableAmount.IsNegative() {
		distributableAmount = math.ZeroInt()
	}

	// Calculate treasury and validator amounts from distributable
	treasuryAmount := params.TreasuryFeeRatio.MulInt(distributableAmount).TruncateInt()
	if treasuryAmount.IsNegative() {
		treasuryAmount = math.ZeroInt()
	}

	validatorAmount := distributableAmount.Sub(treasuryAmount)
	if validatorAmount.IsNegative() {
		validatorAmount = math.ZeroInt()
	}

	// 1. Burn tokens
	if burnAmount.IsPositive() {
		burnCoins := sdk.NewCoins(sdk.NewCoin(tokenomicstypes.BondDenom, burnAmount))
		if err := k.bankKeeper.BurnCoins(ctx, authtypes.FeeCollectorName, burnCoins); err != nil {
			k.Logger(ctx).Error("failed to burn fees", "error", err)
			return types.ErrBurnFailed.Wrapf("amount: %s, error: %v", burnAmount.String(), err)
		}

		// Update cumulative burned
		if err := k.IncrementCumulativeBurned(ctx, burnAmount); err != nil {
			k.Logger(ctx).Error("failed to update cumulative burned", "error", err)
		}

		k.Logger(ctx).Info("burned fees",
			"amount", burnAmount.String(),
			"tier", tierName,
			"burn_rate", burnRate.String(),
		)
	}

	// 2. Transfer to treasury
	if treasuryAmount.IsPositive() {
		treasuryAddr := k.GetTreasuryAddress(ctx)
		if len(treasuryAddr) == 0 {
			k.Logger(ctx).Error("treasury address not set")
			return types.ErrTreasuryAddressNotSet
		}

		treasuryCoins := sdk.NewCoins(sdk.NewCoin(tokenomicstypes.BondDenom, treasuryAmount))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, authtypes.FeeCollectorName, treasuryAddr, treasuryCoins); err != nil {
			k.Logger(ctx).Error("failed to transfer to treasury", "error", err)
			return types.ErrTreasuryTransferFailed.Wrapf("amount: %s, error: %v", treasuryAmount.String(), err)
		}

		// Update cumulative to treasury
		if err := k.IncrementCumulativeToTreasury(ctx, treasuryAmount); err != nil {
			k.Logger(ctx).Error("failed to update cumulative to treasury", "error", err)
		}

		k.Logger(ctx).Info("transferred to treasury",
			"amount", treasuryAmount.String(),
			"address", treasuryAddr.String(),
		)

		// Emit treasury transfer event
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeTreasuryTransfer,
				sdk.NewAttribute(types.AttributeKeyTreasuryAmount, treasuryAmount.String()),
			),
		)
	}

	// 3. Validator amount stays in fee_collector for x/distribution to handle
	if validatorAmount.IsPositive() {
		// Update cumulative to validators
		if err := k.IncrementCumulativeToValidators(ctx, validatorAmount); err != nil {
			k.Logger(ctx).Error("failed to update cumulative to validators", "error", err)
		}

		k.Logger(ctx).Info("fees for validators",
			"amount", validatorAmount.String(),
			"note", "remaining in fee_collector for x/distribution",
		)
	}

	// Emit comprehensive fees processed event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeFeesProcessed,
			sdk.NewAttribute(types.AttributeKeyTotalFees, totalFees.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyBurnAmount, burnAmount.String()),
			sdk.NewAttribute(types.AttributeKeyTreasuryAmount, treasuryAmount.String()),
			sdk.NewAttribute(types.AttributeKeyValidatorAmount, validatorAmount.String()),
			sdk.NewAttribute(types.AttributeKeyBurnTier, tierName),
			sdk.NewAttribute(types.AttributeKeyBurnPercentage, burnRate.String()),
			sdk.NewAttribute(types.AttributeKeyUtilization, utilization.String()),
		),
	)

	// Emit burn tier change event
	k.EmitBurnTierEvent(ctx, tierName, burnRate, utilization)

	return nil
}
