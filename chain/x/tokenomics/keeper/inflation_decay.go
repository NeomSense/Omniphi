package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// CalculateDecayingInflation calculates the inflation rate based on time elapsed since genesis
// Implements a year-based step decay model:
// Year 1: 3.00%
// Year 2: 2.75%
// Year 3: 2.50%
// Year 4: 2.25%
// Year 5: 2.00%
// Year 6: 1.75%
// Year 7+: Reduce by 0.25%/year until floor = 0.5%
func (k Keeper) CalculateDecayingInflation(ctx context.Context) math.LegacyDec {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Calculate years since genesis
	currentHeight := sdkCtx.BlockHeight()
	genesisHeight := int64(1) // Assuming genesis is height 1

	// Blocks per year (assuming 7-second blocks)
	// 365.25 days * 24 hours * 60 minutes * 60 seconds / 7 seconds per block
	blocksPerYear := int64(4_507_680) // ~4.5M blocks per year

	yearsSinceGenesis := (currentHeight - genesisHeight) / blocksPerYear

	// Get inflation floor from params (default 0.5%)
	minInflation := params.InflationMin

	// Calculate inflation based on year
	var inflationRate math.LegacyDec

	switch {
	case yearsSinceGenesis == 0:
		// Year 1: 3.00%
		inflationRate = math.LegacyMustNewDecFromStr("0.03")
	case yearsSinceGenesis == 1:
		// Year 2: 2.75%
		inflationRate = math.LegacyMustNewDecFromStr("0.0275")
	case yearsSinceGenesis == 2:
		// Year 3: 2.50%
		inflationRate = math.LegacyMustNewDecFromStr("0.025")
	case yearsSinceGenesis == 3:
		// Year 4: 2.25%
		inflationRate = math.LegacyMustNewDecFromStr("0.0225")
	case yearsSinceGenesis == 4:
		// Year 5: 2.00%
		inflationRate = math.LegacyMustNewDecFromStr("0.02")
	case yearsSinceGenesis == 5:
		// Year 6: 1.75%
		inflationRate = math.LegacyMustNewDecFromStr("0.0175")
	default:
		// Year 7+: Reduce by 0.25% per year until floor
		// Starting from 1.75% (year 6), subtract 0.25% for each additional year
		baseRate := math.LegacyMustNewDecFromStr("0.0175")
		decayRate := math.LegacyMustNewDecFromStr("0.0025") // 0.25% per year

		yearsAfterSix := yearsSinceGenesis - 5
		totalDecay := decayRate.MulInt64(yearsAfterSix)

		inflationRate = baseRate.Sub(totalDecay)

		// Enforce minimum floor
		if inflationRate.LT(minInflation) {
			inflationRate = minInflation
		}
	}

	// Enforce params boundaries (min/max from governance)
	if inflationRate.LT(params.InflationMin) {
		inflationRate = params.InflationMin
	}
	if inflationRate.GT(params.InflationMax) {
		inflationRate = params.InflationMax
	}

	return inflationRate
}

// GetCurrentYear returns the current year since genesis (0-indexed)
func (k Keeper) GetCurrentYear(ctx context.Context) int64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()
	genesisHeight := int64(1)
	blocksPerYear := int64(4_507_680)

	return (currentHeight - genesisHeight) / blocksPerYear
}

// GetBlocksPerYear returns the number of blocks per year
// Based on 7-second block time: 365.25 * 24 * 60 * 60 / 7
func (k Keeper) GetBlocksPerYear() int64 {
	return int64(4_507_680)
}

// CalculateDecayingAnnualProvisions calculates the annual provisions based on decaying inflation rate
// and current total supply (renamed to avoid conflict with mint.go method)
func (k Keeper) CalculateDecayingAnnualProvisions(ctx context.Context) math.Int {
	params := k.GetParams(ctx)
	inflationRate := k.CalculateDecayingInflation(ctx)

	// Annual provisions = Current Supply * Inflation Rate
	currentSupply := params.CurrentTotalSupply

	// Convert to LegacyDec for multiplication
	supplyDec := math.LegacyNewDecFromInt(currentSupply)
	annualProvisions := supplyDec.Mul(inflationRate)

	// Convert back to Int, truncating
	return annualProvisions.TruncateInt()
}

// CalculateBlockProvision calculates the provision for a single block using decaying inflation
func (k Keeper) CalculateBlockProvision(ctx context.Context) math.Int {
	annualProvisions := k.CalculateDecayingAnnualProvisions(ctx)
	blocksPerYear := k.GetBlocksPerYear()

	// Block provision = Annual provisions / Blocks per year
	blockProvision := annualProvisions.QuoRaw(blocksPerYear)

	return blockProvision
}

// MintInflation mints inflation for the current block and distributes to recipients
// This should be called in EndBlocker
func (k Keeper) MintInflation(ctx context.Context) error {
	params := k.GetParams(ctx)

	// Check if we've reached supply cap
	if params.CurrentTotalSupply.GTE(params.TotalSupplyCap) {
		k.Logger(ctx).Info("Supply cap reached, no inflation minted",
			"current_supply", params.CurrentTotalSupply.String(),
			"supply_cap", params.TotalSupplyCap.String())
		return nil
	}

	// Calculate block provision
	blockProvision := k.CalculateBlockProvision(ctx)

	if blockProvision.IsZero() {
		return nil
	}

	// Check if minting would exceed cap
	newSupply := params.CurrentTotalSupply.Add(blockProvision)
	if newSupply.GT(params.TotalSupplyCap) {
		// Only mint up to cap
		originalProvision := blockProvision
		blockProvision = params.TotalSupplyCap.Sub(params.CurrentTotalSupply)

		k.Logger(ctx).Warn("Minting limited to reach supply cap exactly",
			"original_provision", originalProvision.String(),
			"adjusted_provision", blockProvision.String(),
			"remaining_mintable", blockProvision.String())
	}

	// Distribute emissions
	if err := k.DistributeEmissions(ctx, blockProvision); err != nil {
		return fmt.Errorf("failed to distribute emissions: %w", err)
	}

	// Update total minted and current supply
	params.TotalMinted = params.TotalMinted.Add(blockProvision)
	params.CurrentTotalSupply = params.CurrentTotalSupply.Add(blockProvision)

	// Update inflation rate in params for queries
	currentInflation := k.CalculateDecayingInflation(ctx)
	params.InflationRate = currentInflation

	if err := k.SetParams(ctx, params); err != nil {
		return fmt.Errorf("failed to update params: %w", err)
	}

	// Emit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMint,
			sdk.NewAttribute(types.AttributeKeyInflationRate, currentInflation.String()),
			sdk.NewAttribute(types.AttributeKeyAnnualProvisions, k.CalculateDecayingAnnualProvisions(ctx).String()),
			sdk.NewAttribute(types.AttributeKeyBlockProvision, blockProvision.String()),
			sdk.NewAttribute(types.AttributeKeyYear, fmt.Sprintf("%d", k.GetCurrentYear(ctx))),
		),
	)

	return nil
}

// DistributeEmissions distributes minted inflation to various recipients
func (k Keeper) DistributeEmissions(ctx context.Context, totalAmount math.Int) error {
	params := k.GetParams(ctx)

	// Calculate distribution amounts
	totalAmountDec := math.LegacyNewDecFromInt(totalAmount)

	stakingAmount := totalAmountDec.Mul(params.EmissionSplitStaking).TruncateInt()
	pocAmount := totalAmountDec.Mul(params.EmissionSplitPoc).TruncateInt()
	sequencerAmount := totalAmountDec.Mul(params.EmissionSplitSequencer).TruncateInt()
	treasuryAmount := totalAmountDec.Mul(params.EmissionSplitTreasury).TruncateInt()

	// Ensure exact distribution (handle rounding)
	distributed := stakingAmount.Add(pocAmount).Add(sequencerAmount).Add(treasuryAmount)
	if distributed.LT(totalAmount) {
		// Add remainder to treasury
		remainder := totalAmount.Sub(distributed)
		treasuryAmount = treasuryAmount.Add(remainder)
	}

	// Mint to staking module (distributed by staking module)
	if stakingAmount.IsPositive() {
		stakingCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, stakingAmount))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, stakingCoins); err != nil {
			return fmt.Errorf("failed to mint staking rewards: %w", err)
		}
		// Transfer to staking module
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "staking", stakingCoins); err != nil {
			return fmt.Errorf("failed to send to staking module: %w", err)
		}
	}

	// Mint to PoC module
	// TODO: Integrate with PoC keeper when available
	if pocAmount.IsPositive() {
		pocCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, pocAmount))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, pocCoins); err != nil {
			return fmt.Errorf("failed to mint PoC rewards: %w", err)
		}
		// For now, keep in tokenomics module or transfer to PoC module when integrated
	}

	// Mint to sequencer module (if exists)
	if sequencerAmount.IsPositive() {
		sequencerCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, sequencerAmount))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, sequencerCoins); err != nil {
			return fmt.Errorf("failed to mint sequencer rewards: %w", err)
		}
		// For now, keep in tokenomics module or transfer to sequencer module when ready
	}

	// Mint to treasury
	if treasuryAmount.IsPositive() {
		treasuryCoins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, treasuryAmount))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, treasuryCoins); err != nil {
			return fmt.Errorf("failed to mint treasury allocation: %w", err)
		}

		// Send to treasury address
		treasuryAddr := k.GetTreasuryAddress(ctx)
		if !treasuryAddr.Empty() {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, treasuryAddr, treasuryCoins); err != nil {
				return fmt.Errorf("failed to send to treasury: %w", err)
			}
		}
	}

	// Record the emission for auditing and transparency
	_, err := k.RecordEmission(ctx, totalAmount, stakingAmount, pocAmount, sequencerAmount, treasuryAmount)
	if err != nil {
		k.Logger(ctx).Error("failed to record emission", "error", err)
		// Don't fail the emission, just log the error
	}

	k.Logger(ctx).Info("Emissions distributed",
		"total", totalAmount.String(),
		"staking", stakingAmount.String(),
		"poc", pocAmount.String(),
		"sequencer", sequencerAmount.String(),
		"treasury", treasuryAmount.String())

	return nil
}

// GetInflationForecast returns inflation forecast for future years
func (k Keeper) GetInflationForecast(ctx context.Context, years int64) []types.InflationForecast {
	forecasts := make([]types.InflationForecast, years)

	params := k.GetParams(ctx)
	currentYear := k.GetCurrentYear(ctx)
	currentSupply := params.CurrentTotalSupply

	for i := int64(0); i < years; i++ {
		year := currentYear + i

		// Calculate inflation rate for this year
		var inflationRate math.LegacyDec
		switch {
		case year == 0:
			inflationRate = math.LegacyMustNewDecFromStr("0.03")
		case year == 1:
			inflationRate = math.LegacyMustNewDecFromStr("0.0275")
		case year == 2:
			inflationRate = math.LegacyMustNewDecFromStr("0.025")
		case year == 3:
			inflationRate = math.LegacyMustNewDecFromStr("0.0225")
		case year == 4:
			inflationRate = math.LegacyMustNewDecFromStr("0.02")
		case year == 5:
			inflationRate = math.LegacyMustNewDecFromStr("0.0175")
		default:
			baseRate := math.LegacyMustNewDecFromStr("0.0175")
			decayRate := math.LegacyMustNewDecFromStr("0.0025")
			yearsAfterSix := year - 5
			totalDecay := decayRate.MulInt64(yearsAfterSix)
			inflationRate = baseRate.Sub(totalDecay)

			if inflationRate.LT(params.InflationMin) {
				inflationRate = params.InflationMin
			}
		}

		// Calculate annual mint
		supplyDec := math.LegacyNewDecFromInt(currentSupply)
		annualMint := supplyDec.Mul(inflationRate).TruncateInt()

		// Check supply cap
		if currentSupply.Add(annualMint).GT(params.TotalSupplyCap) {
			annualMint = params.TotalSupplyCap.Sub(currentSupply)
		}

		forecasts[i] = types.InflationForecast{
			Year:         year,
			InflationRate: inflationRate,
			AnnualMint:   annualMint,
			Supply:       currentSupply,
		}

		// Update supply for next year
		currentSupply = currentSupply.Add(annualMint)

		// Stop if cap reached
		if currentSupply.GTE(params.TotalSupplyCap) {
			forecasts = forecasts[:i+1]
			break
		}
	}

	return forecasts
}
