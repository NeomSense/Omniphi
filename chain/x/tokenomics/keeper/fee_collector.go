package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"pos/x/tokenomics/types"
)

// ProcessBlockFees implements the 90/10 burn/treasury split for all transaction fees
// This is called during EndBlock to process all fees collected in the fee_collector module
//
// Security Requirements:
// - FEE-001: All fees must be processed (no bypass)
// - FEE-002: 90% burned, 10% to treasury (configurable via governance)
// - FEE-003: Treasury must receive exactly treasury_ratio * total_fees
// - FEE-004: Burned amount must decrease total supply
// - FEE-005: All operations atomic (fail together or succeed together)
func (k Keeper) ProcessBlockFees(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Check if fee burning is enabled
	if !params.FeeBurnEnabled {
		k.Logger(ctx).Debug("fee burning disabled, skipping")
		return nil
	}

	// Get fee collector module account
	feeCollectorAddr := k.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	if feeCollectorAddr == nil {
		return fmt.Errorf("fee collector module account not found")
	}

	// Get all collected fees in the fee_collector account
	collectedFees := k.bankKeeper.GetBalance(ctx, feeCollectorAddr, types.BondDenom)

	if collectedFees.Amount.IsZero() {
		// No fees to process
		return nil
	}

	totalFees := collectedFees.Amount

	// Calculate burn and treasury amounts based on governance params
	// ADAPTIVE-BURN MODEL (when enabled):
	//   1. 10% always goes to treasury (fixed)
	//   2. Remaining 90% is split between burn (adaptive 70-95%) and validators
	//   3. Example: If adaptive burn = 80%, then: 10% treasury, 72% burn (80% of 90%), 18% validators
	// FIXED MODEL (when disabled):
	//   1. Use fee_burn_ratio and treasury_fee_ratio from params (must sum to 1.0)

	var burnAmount, treasuryAmount, validatorAmount math.Int

	if params.AdaptiveBurnEnabled && !params.EmergencyBurnOverride {
		// Adaptive burn model: 10% treasury, then adaptive burn of remaining
		treasuryRate := params.TreasuryFeeRatio // Fixed 10%
		treasuryAmount = treasuryRate.MulInt(totalFees).TruncateInt()

		// Remaining for burn/validator split
		remaining := totalFees.Sub(treasuryAmount)

		// Get adaptive burn ratio (applies to remaining amount, not total)
		adaptiveBurnRatio := k.GetCurrentBurnRatio(ctx)
		burnAmount = adaptiveBurnRatio.MulInt(remaining).TruncateInt()

		// Validators get the rest
		validatorAmount = remaining.Sub(burnAmount)

		k.Logger(ctx).Info("using adaptive burn model",
			"total_fees", totalFees.String(),
			"treasury_pct", treasuryRate.String(),
			"adaptive_burn_ratio", adaptiveBurnRatio.String(),
			"treasury_amount", treasuryAmount.String(),
			"burn_amount", burnAmount.String(),
			"validator_amount", validatorAmount.String())
	} else {
		// Fixed model: Use governance-set ratios
		burnRatio := params.FeeBurnRatio       // Default: 0.90
		treasuryRatio := params.TreasuryFeeRatio // Default: 0.10

		// Validate ratios sum to 1.0 (should be enforced in param validation)
		if !burnRatio.Add(treasuryRatio).Equal(math.LegacyOneDec()) {
			return fmt.Errorf("burn ratio + treasury ratio must equal 1.0, got %s + %s",
				burnRatio.String(), treasuryRatio.String())
		}

		burnAmount = burnRatio.MulInt(totalFees).TruncateInt()
		treasuryAmount = treasuryRatio.MulInt(totalFees).TruncateInt()
		validatorAmount = math.ZeroInt()

		// Handle any dust (rounding errors) by adding to burn
		dust := totalFees.Sub(burnAmount).Sub(treasuryAmount)
		if dust.IsPositive() {
			burnAmount = burnAmount.Add(dust)
		}
	}

	// FEE-005: Process atomically
	// Step 1: Transfer fees from fee_collector to tokenomics module for processing
	feesToProcess := sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx,
		authtypes.FeeCollectorName,
		types.ModuleName,
		feesToProcess,
	); err != nil {
		return fmt.Errorf("failed to transfer fees for processing: %w", err)
	}

	// Step 2: Burn the burn portion (FEE-004: decreases supply)
	if burnAmount.IsPositive() {
		burnCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, burnAmount))
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			// Rollback: return fees to fee_collector
			_ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, feesToProcess)
			return fmt.Errorf("failed to burn fee portion: %w", err)
		}
	}

	// Step 3: Send treasury portion to treasury (FEE-003)
	if treasuryAmount.IsPositive() {
		treasuryAddr := k.GetTreasuryAddress(ctx)
		treasuryCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, treasuryAmount))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, treasuryAddr, treasuryCoins); err != nil {
			k.Logger(ctx).Error("failed to send treasury portion", "error", err)
			return fmt.Errorf("failed to send treasury portion: %w", err)
		}

		// Track treasury inflows
		k.IncrementTreasuryInflows(ctx, treasuryAmount, "transaction_fees")

		// Track inflows for treasury redirect mechanism
		// This accumulates until the next redirect execution
		k.IncrementAccumulatedRedirectInflows(ctx, treasuryAmount)
	}

	// Step 3b: Return validator portion to fee_collector (for distribution module)
	// ADAPTIVE-BURN: When enabled, validators get a portion of fees
	if validatorAmount.IsPositive() {
		validatorCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, validatorAmount))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, validatorCoins); err != nil {
			k.Logger(ctx).Error("failed to return validator portion to fee_collector", "error", err)
			return fmt.Errorf("failed to return validator portion: %w", err)
		}

		k.Logger(ctx).Debug("returned validator portion to fee_collector for distribution",
			"amount", validatorAmount.String())
	}

	// Step 4: Update supply counters (must match burn.go logic)
	currentSupply := k.GetCurrentSupply(ctx)
	totalBurned := k.GetTotalBurned(ctx)

	if currentSupply.LT(burnAmount) {
		k.Logger(ctx).Error("fee burn would make supply negative",
			"current_supply", currentSupply.String(),
			"burn_amount", burnAmount.String(),
		)
		return fmt.Errorf("insufficient supply for fee burn")
	}

	newSupply := currentSupply.Sub(burnAmount)
	newTotalBurned := totalBurned.Add(burnAmount)

	if err := k.SetCurrentSupply(ctx, newSupply); err != nil {
		return fmt.Errorf("failed to update current supply: %w", err)
	}

	if err := k.SetTotalBurned(ctx, newTotalBurned); err != nil {
		return fmt.Errorf("failed to update total burned: %w", err)
	}

	// Step 5: Track fee-specific statistics
	k.IncrementTotalFeesBurned(ctx, burnAmount)
	k.IncrementTotalFeesToTreasury(ctx, treasuryAmount)
	k.IncrementBurnsBySource(ctx, types.BurnSource_BURN_SOURCE_POS_GAS, burnAmount)

	// Step 6: Emit detailed events for transparency
	// Calculate effective ratios for event emission
	effectiveBurnRatio := math.LegacyNewDecFromInt(burnAmount).Quo(math.LegacyNewDecFromInt(totalFees))
	effectiveTreasuryRatio := math.LegacyNewDecFromInt(treasuryAmount).Quo(math.LegacyNewDecFromInt(totalFees))

	event := sdk.NewEvent(
		"transaction_fees_processed",
		sdk.NewAttribute("total_fees", totalFees.String()),
		sdk.NewAttribute("burned", burnAmount.String()),
		sdk.NewAttribute("to_treasury", treasuryAmount.String()),
		sdk.NewAttribute("burn_ratio", effectiveBurnRatio.String()),
		sdk.NewAttribute("treasury_ratio", effectiveTreasuryRatio.String()),
		sdk.NewAttribute("new_total_supply", newSupply.String()),
		sdk.NewAttribute("new_total_burned", newTotalBurned.String()),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
	)

	// Add adaptive burn attributes if enabled
	if params.AdaptiveBurnEnabled && !params.EmergencyBurnOverride {
		event = event.AppendAttributes(
			sdk.NewAttribute("adaptive_burn_enabled", "true"),
			sdk.NewAttribute("validator_amount", validatorAmount.String()),
			sdk.NewAttribute("burn_trigger", params.LastBurnTrigger),
		)
	}

	sdkCtx.EventManager().EmitEvent(event)

	k.Logger(ctx).Info("transaction fees processed",
		"total_fees", totalFees.String(),
		"burned", burnAmount.String(),
		"to_treasury", treasuryAmount.String(),
		"to_validators", validatorAmount.String(),
		"burn_pct", effectiveBurnRatio.MulInt64(100).String(),
		"new_supply", newSupply.String(),
		"adaptive_enabled", params.AdaptiveBurnEnabled,
	)

	return nil
}

// IncrementTotalFeesBurned tracks cumulative fees burned
func (k Keeper) IncrementTotalFeesBurned(ctx context.Context, amount math.Int) {
	store := k.storeService.OpenKVStore(ctx)

	var current math.Int
	bz, err := store.Get(types.KeyTotalFeesBurned)
	if err == nil && bz != nil {
		_ = current.Unmarshal(bz)
	} else {
		current = math.ZeroInt()
	}

	newTotal := current.Add(amount)
	bz, err = newTotal.Marshal()
	if err == nil {
		_ = store.Set(types.KeyTotalFeesBurned, bz)
	}
}

// GetTotalFeesBurned returns cumulative fees burned
func (k Keeper) GetTotalFeesBurned(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTotalFeesBurned)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var total math.Int
	if err := total.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return total
}

// IncrementTotalFeesToTreasury tracks cumulative fees sent to treasury
func (k Keeper) IncrementTotalFeesToTreasury(ctx context.Context, amount math.Int) {
	store := k.storeService.OpenKVStore(ctx)

	var current math.Int
	bz, err := store.Get(types.KeyTotalFeesToTreasury)
	if err == nil && bz != nil {
		_ = current.Unmarshal(bz)
	} else {
		current = math.ZeroInt()
	}

	newTotal := current.Add(amount)
	bz, err = newTotal.Marshal()
	if err == nil {
		_ = store.Set(types.KeyTotalFeesToTreasury, bz)
	}
}

// GetTotalFeesToTreasury returns cumulative fees sent to treasury
func (k Keeper) GetTotalFeesToTreasury(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTotalFeesToTreasury)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var total math.Int
	if err := total.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return total
}

// GetAverageFeesBurnedPerBlock calculates average fees burned per block
func (k Keeper) GetAverageFeesBurnedPerBlock(ctx context.Context) math.LegacyDec {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	if currentHeight <= 0 {
		return math.LegacyZeroDec()
	}

	totalBurned := k.GetTotalFeesBurned(ctx)
	if totalBurned.IsZero() {
		return math.LegacyZeroDec()
	}

	return math.LegacyNewDecFromInt(totalBurned).QuoInt64(currentHeight)
}