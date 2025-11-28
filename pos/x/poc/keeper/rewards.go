package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// EnqueueReward calculates and assigns credits for a verified contribution
// SECURITY FIX: CVE-2025-POC-003 - Added overflow protection for credit calculations
func (k Keeper) EnqueueReward(ctx context.Context, c types.Contribution) error {
	params := k.GetParams(ctx)

	// Calculate weight based on contribution type (simple version: all weight = 1)
	weight := k.weightFor(ctx, c)

	// Calculate credits with overflow check
	credits := params.BaseRewardUnit.Mul(weight)

	// SECURITY FIX: Validate result is positive and within safe bounds
	// Use 2^63 - 1 (max int64) as safe limit
	const maxSafeUint64 = uint64(1<<63 - 1)
	maxSafeCredits := math.NewIntFromUint64(maxSafeUint64)
	if credits.IsNegative() {
		return fmt.Errorf("credit calculation resulted in negative value")
	}
	if credits.GTE(maxSafeCredits) {
		return fmt.Errorf("credit amount exceeds maximum safe value: %s >= %s", credits, maxSafeCredits)
	}

	// Add credits to contributor
	contributor, err := sdk.AccAddressFromBech32(c.Contributor)
	if err != nil {
		return err
	}

	// SECURITY FIX: Use safe credit addition with overflow check
	return k.AddCreditsWithOverflowCheck(ctx, contributor, credits)
}

// weightFor calculates the weight multiplier for a contribution
// This can be extended to factor in contribution type, size, etc.
func (k Keeper) weightFor(ctx context.Context, c types.Contribution) math.Int {
	// Simple implementation: uniform weight
	// Future: vary by ctype ("code"=2, "record"=1, "green"=3, etc.)
	return math.NewInt(1)
}

// WithdrawCredits converts PoC credits to coins and sends them to the contributor
// SECURITY FIX: CVE-2025-POC-005 - Prevents re-entrancy by zeroing credits BEFORE sending
func (k Keeper) WithdrawCredits(ctx context.Context, addr sdk.AccAddress) (math.Int, error) {
	// STEP 1: Get current credits
	credits := k.GetCredits(ctx, addr)

	if !credits.IsPositive() {
		return math.ZeroInt(), types.ErrNoCredits
	}

	params := k.GetParams(ctx)
	amount := credits.Amount

	// STEP 2: ZERO CREDITS FIRST (prevents re-entrancy)
	credits.Amount = math.ZeroInt()
	if err := k.SetCredits(ctx, credits); err != nil {
		return math.ZeroInt(), fmt.Errorf("failed to zero credits: %w", err)
	}

	// STEP 3: Verify module balance BEFORE sending
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	moduleBalance := k.bankKeeper.GetBalance(ctx, moduleAddr, params.RewardDenom)

	if moduleBalance.Amount.LT(amount) {
		// RESTORE credits on failure
		credits.Amount = amount
		if restoreErr := k.SetCredits(ctx, credits); restoreErr != nil {
			// Double failure - log critical error
			k.logger.Error("CRITICAL: Failed to restore credits after balance check failure",
				"address", addr.String(),
				"amount", amount.String(),
				"restore_error", restoreErr.Error())
		}
		return math.ZeroInt(), fmt.Errorf(
			"insufficient module balance: have %s, need %s",
			moduleBalance.Amount, amount)
	}

	// STEP 4: Send coins (credits already zeroed, safe from re-entrancy)
	coins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, amount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins); err != nil {
		// Send failed - RESTORE credits
		credits.Amount = amount
		if restoreErr := k.SetCredits(ctx, credits); restoreErr != nil {
			k.logger.Error("CRITICAL: Failed to restore credits after send failure",
				"address", addr.String(),
				"amount", amount.String(),
				"send_error", err.Error(),
				"restore_error", restoreErr.Error())
		}
		return math.ZeroInt(), fmt.Errorf("failed to send coins: %w", err)
	}

	// STEP 5: Success - emit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_withdraw_success",
			sdk.NewAttribute("address", addr.String()),
			sdk.NewAttribute("amount", amount.String()),
		),
	)

	return amount, nil
}

// GetTier returns the tier name based on credit amount
func (k Keeper) GetTier(ctx context.Context, creditAmount math.Int) string {
	params := k.GetParams(ctx)

	tier := "none"
	for _, t := range params.Tiers {
		if creditAmount.GTE(t.Cutoff) {
			tier = t.Name
		} else {
			break
		}
	}

	return tier
}

// GovWeightBoost calculates a governance weight boost based on credits
// This can be used by x/dao or x/gov for contribution-weighted voting
func (k Keeper) GovWeightBoost(ctx context.Context, addr sdk.AccAddress) math.LegacyDec {
	credits := k.GetCredits(ctx, addr)

	if credits.Amount.IsZero() {
		return math.LegacyZeroDec()
	}

	// Simple formula: boost = min(1.0, 0.1 * log10(1 + credits/1000))
	// This gives up to 10% boost for high contributors
	// For production, implement proper logarithm calculation

	// Simplified: give 1% boost per 10,000 credits, max 10%
	boost := math.LegacyNewDecFromInt(credits.Amount).Quo(math.LegacyNewDec(100000))
	maxBoost := math.LegacyMustNewDecFromStr("0.10")

	if boost.GT(maxBoost) {
		return maxBoost
	}

	return boost
}
