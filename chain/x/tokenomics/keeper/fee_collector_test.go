package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TestProcessBlockFees_BasicSplit tests the 90/10 fee split
func TestProcessBlockFees_BasicSplit(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Setup: Add fees to fee_collector
	feeCollectorAddr := suite.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	totalFees := math.NewInt(1_000_000) // 1 OMNI in omniphi

	// Mint coins and send to fee collector
	err := suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)

	// Update CurrentSupply to reflect minted tokens (sync keeper's supply with bank module)
	initialSupply := k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, initialSupply.Add(totalFees))
	require.NoError(t, err)

	// Get initial state
	initialSupply = k.GetCurrentSupply(ctx)
	initialBurned := k.GetTotalBurned(ctx)
	treasuryAddr := k.GetTreasuryAddress(ctx)
	initialTreasuryBalance := suite.BankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom).Amount

	// Execute fee processing
	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Verify 90/10 split
	expectedBurned := math.NewInt(900_000)   // 90%
	expectedTreasury := math.NewInt(100_000) // 10%

	// Check supply decreased by burn amount
	finalSupply := k.GetCurrentSupply(ctx)
	require.Equal(t, initialSupply.Sub(expectedBurned), finalSupply)

	// Check total burned increased
	finalBurned := k.GetTotalBurned(ctx)
	require.Equal(t, initialBurned.Add(expectedBurned), finalBurned)

	// Check treasury received 10%
	finalTreasuryBalance := suite.BankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom).Amount
	require.Equal(t, initialTreasuryBalance.Add(expectedTreasury), finalTreasuryBalance)

	// Check fee_collector is empty
	feeCollectorBalance := suite.BankKeeper.GetBalance(ctx, feeCollectorAddr, types.BondDenom).Amount
	require.True(t, feeCollectorBalance.IsZero())

	// Check statistics were updated
	totalFeesBurned := k.GetTotalFeesBurned(ctx)
	require.Equal(t, expectedBurned, totalFeesBurned)

	totalFeesToTreasury := k.GetTotalFeesToTreasury(ctx)
	require.Equal(t, expectedTreasury, totalFeesToTreasury)
}

// TestProcessBlockFees_ZeroFees tests handling of zero fees
func TestProcessBlockFees_ZeroFees(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	initialSupply := k.GetCurrentSupply(ctx)

	// Execute with no fees in collector
	err := k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Supply should be unchanged
	finalSupply := k.GetCurrentSupply(ctx)
	require.Equal(t, initialSupply, finalSupply)
}

// TestProcessBlockFees_Disabled tests when fee burn is disabled
func TestProcessBlockFees_Disabled(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Disable fee burning
	params := k.GetParams(ctx)
	params.FeeBurnEnabled = false
	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	// Add fees to collector
	totalFees := math.NewInt(1_000_000)
	err = suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)

	initialSupply := k.GetCurrentSupply(ctx)
	feeCollectorAddr := suite.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	initialFeeBalance := suite.BankKeeper.GetBalance(ctx, feeCollectorAddr, types.BondDenom).Amount

	// Execute
	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Nothing should change
	finalSupply := k.GetCurrentSupply(ctx)
	require.Equal(t, initialSupply, finalSupply)

	finalFeeBalance := suite.BankKeeper.GetBalance(ctx, feeCollectorAddr, types.BondDenom).Amount
	require.Equal(t, initialFeeBalance, finalFeeBalance)
}

// TestProcessBlockFees_CustomRatios tests DAO-adjusted ratios
func TestProcessBlockFees_CustomRatios(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Set custom ratios: 80/20 split
	params := k.GetParams(ctx)
	params.FeeBurnRatio = math.LegacyNewDecWithPrec(80, 2)     // 80%
	params.TreasuryFeeRatio = math.LegacyNewDecWithPrec(20, 2) // 20%
	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	// Add fees
	totalFees := math.NewInt(1_000_000)
	err = suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)

	// Update CurrentSupply to reflect minted tokens
	currentSupply := k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, currentSupply.Add(totalFees))
	require.NoError(t, err)

	treasuryAddr := k.GetTreasuryAddress(ctx)
	initialTreasuryBalance := suite.BankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom).Amount
	initialBurned := k.GetTotalBurned(ctx)

	// Execute
	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Verify 80/20 split
	expectedBurned := math.NewInt(800_000)   // 80%
	expectedTreasury := math.NewInt(200_000) // 20%

	finalBurned := k.GetTotalBurned(ctx)
	require.Equal(t, initialBurned.Add(expectedBurned), finalBurned)

	finalTreasuryBalance := suite.BankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom).Amount
	require.Equal(t, initialTreasuryBalance.Add(expectedTreasury), finalTreasuryBalance)
}

// TestProcessBlockFees_RoundingDust tests dust handling
func TestProcessBlockFees_RoundingDust(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Use amount that doesn't divide evenly
	totalFees := math.NewInt(1_000_003) // 3 omniphi dust
	err := suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)

	// Update CurrentSupply to reflect minted tokens
	currentSupply := k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, currentSupply.Add(totalFees))
	require.NoError(t, err)

	// Execute
	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// All fees should be accounted for (dust goes to burn)
	totalFeesBurned := k.GetTotalFeesBurned(ctx)
	totalFeesToTreasury := k.GetTotalFeesToTreasury(ctx)

	// Verify total equals original fees
	require.Equal(t, totalFees, totalFeesBurned.Add(totalFeesToTreasury))

	// Verify dust went to burn (burn should be slightly more than 90%)
	expectedTreasury := math.NewInt(100_000) // Exactly 10%
	require.Equal(t, expectedTreasury, totalFeesToTreasury)
	require.Equal(t, totalFees.Sub(expectedTreasury), totalFeesBurned)
}

// TestParamValidation_FeeRatios tests parameter validation
func TestParamValidation_FeeRatios(t *testing.T) {
	// Valid: sums to 1.0
	params := types.DefaultParams()
	require.NoError(t, params.Validate())

	// Invalid: burn ratio > 1.0
	params = types.DefaultParams()
	params.FeeBurnRatio = math.LegacyNewDecWithPrec(110, 2)
	require.Error(t, params.Validate())

	// Invalid: negative treasury ratio
	params = types.DefaultParams()
	params.TreasuryFeeRatio = math.LegacyNewDecWithPrec(-10, 2)
	require.Error(t, params.Validate())

	// Invalid: ratios don't sum to 1.0
	params = types.DefaultParams()
	params.FeeBurnRatio = math.LegacyNewDecWithPrec(50, 2)
	params.TreasuryFeeRatio = math.LegacyNewDecWithPrec(40, 2) // 50+40=90, not 100
	require.Error(t, params.Validate())
}

// TestProcessBlockFees_MultipleBlocks tests cumulative statistics
func TestProcessBlockFees_MultipleBlocks(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Process fees in block 1
	totalFees1 := math.NewInt(500_000)
	err := suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees1)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees1)))
	require.NoError(t, err)

	// Update CurrentSupply for block 1
	currentSupply := k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, currentSupply.Add(totalFees1))
	require.NoError(t, err)

	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Check block 1 stats
	totalBurned1 := k.GetTotalFeesBurned(ctx)
	require.Equal(t, math.NewInt(450_000), totalBurned1) // 90% of 500k

	// Process fees in block 2
	totalFees2 := math.NewInt(300_000)
	err = suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees2)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees2)))
	require.NoError(t, err)

	// Update CurrentSupply for block 2
	currentSupply = k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, currentSupply.Add(totalFees2))
	require.NoError(t, err)

	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Check cumulative stats
	totalBurned2 := k.GetTotalFeesBurned(ctx)
	require.Equal(t, math.NewInt(720_000), totalBurned2) // 450k + 270k

	totalToTreasury := k.GetTotalFeesToTreasury(ctx)
	require.Equal(t, math.NewInt(80_000), totalToTreasury) // 50k + 30k
}

// TestProcessBlockFees_AverageCalculation tests average fees per block
func TestProcessBlockFees_AverageCalculation(t *testing.T) {
	suite := SetupTestSuite(t)
	ctx := suite.Ctx
	k := suite.Keeper

	// Simulate block height = 10
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx = sdkCtx.WithBlockHeight(10)
	ctx = sdkCtx

	// Total burned = 900k omniphi
	totalFees := math.NewInt(1_000_000)
	err := suite.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)
	err = suite.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(types.BondDenom, totalFees)))
	require.NoError(t, err)

	// Update CurrentSupply to reflect minted tokens
	currentSupply := k.GetCurrentSupply(ctx)
	err = k.SetCurrentSupply(ctx, currentSupply.Add(totalFees))
	require.NoError(t, err)

	err = k.ProcessBlockFees(ctx)
	require.NoError(t, err)

	// Average = 900,000 / 10 = 90,000 omniphi per block
	avgFees := k.GetAverageFeesBurnedPerBlock(ctx)
	expected := math.LegacyNewDec(90_000)
	require.Equal(t, expected, avgFees)
}
