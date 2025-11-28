package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// MintTokens mints new OMNI tokens with cap enforcement
// CRITICAL: This is the ONLY place where new tokens can be minted
// Must validate against supply cap before executing
func (k Keeper) MintTokens(ctx context.Context, amount math.Int, recipient sdk.AccAddress, reason string) error {
	// P0-CAP-003: Validate against supply cap
	if err := k.ValidateSupplyCap(ctx, amount); err != nil {
		return err
	}

	// P0-MINMAX-004: Reject negative amounts
	if amount.IsNegative() {
		return types.ErrInvalidAmount
	}

	// P0-MINMAX-001: Reject zero amounts
	if amount.IsZero() {
		return types.ErrInvalidAmount
	}

	// Mint coins to module account
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return fmt.Errorf("failed to mint coins: %w", err)
	}

	// Transfer to recipient
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
		// If transfer fails, burn the minted coins to maintain invariant
		_ = k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
		return fmt.Errorf("failed to send minted coins to recipient: %w", err)
	}

	// P0-ACCT-001: Update supply counters
	currentSupply := k.GetCurrentSupply(ctx)
	totalMinted := k.GetTotalMinted(ctx)

	newSupply := currentSupply.Add(amount)
	newMinted := totalMinted.Add(amount)

	if err := k.SetCurrentSupply(ctx, newSupply); err != nil {
		return fmt.Errorf("failed to update current supply: %w", err)
	}

	if err := k.SetTotalMinted(ctx, newMinted); err != nil {
		return fmt.Errorf("failed to update total minted: %w", err)
	}

	// P0-CAP-005: Check for cap warnings (80%, 90%, 95%, 99%)
	k.CheckSupplyCapWarnings(ctx, newSupply)

	// OBS-001: Emit mint event for transparency
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)
	remaining := params.TotalSupplyCap.Sub(newSupply)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"mint_tokens",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("recipient", recipient.String()),
			sdk.NewAttribute("reason", reason),
			sdk.NewAttribute("new_total_supply", newSupply.String()),
			sdk.NewAttribute("new_total_minted", newMinted.String()),
			sdk.NewAttribute("remaining_mintable", remaining.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	k.Logger(ctx).Info("tokens minted",
		"amount", amount.String(),
		"recipient", recipient.String(),
		"reason", reason,
		"new_supply", newSupply.String(),
		"remaining", remaining.String(),
	)

	return nil
}

// CalculateBlockProvisions calculates the tokens to mint per block
func (k Keeper) CalculateBlockProvisions(ctx context.Context) math.LegacyDec {
	params := k.GetParams(ctx)
	currentSupply := k.GetCurrentSupply(ctx)

	// Annual provisions = current_supply × inflation_rate
	annualProvisions := params.InflationRate.MulInt(currentSupply)

	// Block provisions = annual_provisions / blocks_per_year
	// Assuming ~7 second blocks: 365.25 days × 24 hours × 3600 seconds / 7 seconds
	const blocksPerYear int64 = 4500857 // 365.25 * 24 * 3600 / 7

	blockProvisions := annualProvisions.QuoInt64(blocksPerYear)

	return blockProvisions
}

// CalculateAnnualProvisions calculates expected yearly minting
func (k Keeper) CalculateAnnualProvisions(ctx context.Context) math.Int {
	params := k.GetParams(ctx)
	currentSupply := k.GetCurrentSupply(ctx)

	// Annual provisions = current_supply × inflation_rate
	annualProvisions := params.InflationRate.MulInt(currentSupply).TruncateInt()

	return annualProvisions
}
