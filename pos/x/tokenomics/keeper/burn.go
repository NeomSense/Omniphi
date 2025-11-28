package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// BurnTokens burns OMNI tokens with treasury redirect
// P0-BURN-001: Burns decrease circulating supply
// P0-BURN-002: Treasury redirect applied
// Can be executed on any chain, tracked globally
func (k Keeper) BurnTokens(
	ctx context.Context,
	burner sdk.AccAddress,
	amount math.Int,
	source types.BurnSource,
	chainID string,
) (burned math.Int, toTreasury math.Int, err error) {
	// P0-MINMAX-001: Reject zero amounts
	if amount.IsZero() {
		return math.ZeroInt(), math.ZeroInt(), types.ErrInvalidAmount
	}

	// P0-MINMAX-004: Reject negative amounts
	if amount.IsNegative() {
		return math.ZeroInt(), math.ZeroInt(), types.ErrInvalidAmount
	}

	params := k.GetParams(ctx)

	// P0-BURN-002: Calculate treasury redirect (default 10%)
	redirectPct := params.TreasuryBurnRedirect
	redirectAmount := redirectPct.MulInt(amount).TruncateInt()
	burnAmount := amount.Sub(redirectAmount)

	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))

	// P0-BURN-003: Check burner has sufficient balance
	balance := k.bankKeeper.GetBalance(ctx, burner, types.BondDenom)
	if balance.Amount.LT(amount) {
		return math.ZeroInt(), math.ZeroInt(), types.ErrInsufficientBalance
	}

	// Transfer from burner to module (for processing)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, burner, types.ModuleName, coins); err != nil {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("failed to transfer coins for burning: %w", err)
	}

	// Burn the non-redirected portion
	if burnAmount.IsPositive() {
		burnCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, burnAmount))
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			// Restore coins to burner on failure
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, burner, coins)
			return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("failed to burn coins: %w", err)
		}
	}

	// Send redirect to treasury
	if redirectAmount.IsPositive() {
		treasuryAddr := k.GetTreasuryAddress(ctx)
		treasuryCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, redirectAmount))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, treasuryAddr, treasuryCoins); err != nil {
			// On failure, try to restore state (best effort)
			k.Logger(ctx).Error("failed to send treasury redirect", "error", err)
			return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("failed to send treasury redirect: %w", err)
		}

		// Track treasury inflows
		k.IncrementTreasuryInflows(ctx, redirectAmount, "burn_redirect")
	}

	// P0-ACCT-001: Update supply counters
	currentSupply := k.GetCurrentSupply(ctx)
	totalBurned := k.GetTotalBurned(ctx)

	// P0-BURN-004: Prevent supply from going negative
	if currentSupply.LT(burnAmount) {
		k.Logger(ctx).Error("burn would make supply negative",
			"current_supply", currentSupply.String(),
			"burn_amount", burnAmount.String(),
		)
		return math.ZeroInt(), math.ZeroInt(), types.ErrInsufficientSupply
	}

	newSupply := currentSupply.Sub(burnAmount)
	newBurned := totalBurned.Add(burnAmount)

	if err := k.SetCurrentSupply(ctx, newSupply); err != nil {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("failed to update current supply: %w", err)
	}

	if err := k.SetTotalBurned(ctx, newBurned); err != nil {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("failed to update total burned: %w", err)
	}

	// P0-BURN-005: Store burn record for history
	k.StoreBurnRecord(ctx, burner, amount, burnAmount, redirectAmount, source, chainID)

	// OBS-001: Emit burn event for transparency
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"burn_tokens",
			sdk.NewAttribute("burner", burner.String()),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("amount_burned", burnAmount.String()),
			sdk.NewAttribute("amount_to_treasury", redirectAmount.String()),
			sdk.NewAttribute("source", source.String()),
			sdk.NewAttribute("chain_id", chainID),
			sdk.NewAttribute("new_total_supply", newSupply.String()),
			sdk.NewAttribute("new_total_burned", newBurned.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	k.Logger(ctx).Info("tokens burned",
		"burner", burner.String(),
		"amount", amount.String(),
		"burned", burnAmount.String(),
		"to_treasury", redirectAmount.String(),
		"source", source.String(),
		"new_supply", newSupply.String(),
	)

	return burnAmount, redirectAmount, nil
}

// StoreBurnRecord stores a burn event for query history
// P0-BURN-005: Burn records stored correctly
func (k Keeper) StoreBurnRecord(
	ctx context.Context,
	burner sdk.AccAddress,
	total, burned, treasury math.Int,
	source types.BurnSource,
	chainID string,
) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get next burn ID
	burnID := k.IncrementBurnID(ctx)

	record := types.BurnRecord{
		BurnId:           burnID,
		Amount:           total,
		Source:           source,
		ChainId:          chainID,
		BlockHeight:      sdkCtx.BlockHeight(),
		TxHash:           fmt.Sprintf("%X", sdkCtx.TxBytes()),
		Timestamp:        sdkCtx.BlockTime().Unix(),
		BurnerAddress:    burner.String(),
		TreasuryRedirect: treasury,
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&record)
	_ = store.Set(types.GetBurnRecordKey(burnID), bz)

	// P0-BURN-006: Update per-source burn counters
	k.IncrementBurnsBySource(ctx, source, burned)

	// P0-BURN-008: Update per-chain burn counters
	k.IncrementBurnsByChain(ctx, chainID, burned)
}

// IncrementBurnsBySource updates the burn counter for a specific source
func (k Keeper) IncrementBurnsBySource(ctx context.Context, source types.BurnSource, amount math.Int) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBurnBySourceKey(source)

	// Get current total
	var current math.Int
	bz, err := store.Get(key)
	if err == nil && bz != nil {
		_ = current.Unmarshal(bz)
	} else {
		current = math.ZeroInt()
	}

	// Increment
	newTotal := current.Add(amount)

	// Store updated total
	bz, err = newTotal.Marshal()
	if err == nil {
		_ = store.Set(key, bz)
	}
}

// GetBurnsBySource returns the total burns for a specific source
func (k Keeper) GetBurnsBySource(ctx context.Context, source types.BurnSource) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBurnBySourceKey(source)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var total math.Int
	if err := total.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}

	return total
}

// IncrementBurnsByChain updates the burn counter for a specific chain
func (k Keeper) IncrementBurnsByChain(ctx context.Context, chainID string, amount math.Int) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBurnByChainKey(chainID)

	// Get current total
	var current math.Int
	bz, err := store.Get(key)
	if err == nil && bz != nil {
		_ = current.Unmarshal(bz)
	} else {
		current = math.ZeroInt()
	}

	// Increment
	newTotal := current.Add(amount)

	// Store updated total
	bz, err = newTotal.Marshal()
	if err == nil {
		_ = store.Set(key, bz)
	}
}

// GetBurnsByChain returns the total burns for a specific chain
func (k Keeper) GetBurnsByChain(ctx context.Context, chainID string) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBurnByChainKey(chainID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var total math.Int
	if err := total.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}

	return total
}

// IncrementTreasuryInflows tracks treasury deposits
func (k Keeper) IncrementTreasuryInflows(ctx context.Context, amount math.Int, source string) {
	// Track total inflows
	store := k.storeService.OpenKVStore(ctx)

	var current math.Int
	bz, err := store.Get(types.KeyTreasuryInflows)
	if err == nil && bz != nil {
		_ = current.Unmarshal(bz)
	} else {
		current = math.ZeroInt()
	}

	newTotal := current.Add(amount)
	bz, err = newTotal.Marshal()
	if err == nil {
		_ = store.Set(types.KeyTreasuryInflows, bz)
	}

	// Emit treasury deposit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	treasuryAddr := k.GetTreasuryAddress(ctx)
	balance := k.bankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"treasury_deposit",
			sdk.NewAttribute("source", source),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("treasury_address", treasuryAddr.String()),
			sdk.NewAttribute("new_balance", balance.Amount.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)
}

// GetBurnRecord retrieves a burn record by ID
func (k Keeper) GetBurnRecord(ctx context.Context, burnID uint64) (types.BurnRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBurnRecordKey(burnID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.BurnRecord{}, false
	}

	var record types.BurnRecord
	k.cdc.MustUnmarshal(bz, &record)
	return record, true
}
