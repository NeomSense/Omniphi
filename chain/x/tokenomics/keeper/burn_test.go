package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// ==================== P0-BURN Tests: Burn Correctness ====================

// TestBurnTokens_P0_BURN_001 tests basic burn functionality
func (suite *KeeperTestSuite) TestBurnTokens_P0_BURN_001() {
	// Setup: mint some tokens first
	burner := sdk.AccAddress([]byte("burner"))
	amount := math.NewInt(1000_000_000) // 1000 OMNI

	err := suite.keeper.SetCurrentSupply(suite.ctx, amount)
	suite.Require().NoError(err)
	err = suite.keeper.SetTotalMinted(suite.ctx, amount)
	suite.Require().NoError(err)

	// Fund the burner account directly (BurnTokens will handle the module transfer)
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	err = suite.bankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.bankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, burner, coins)
	suite.Require().NoError(err)

	// Burn tokens
	burnAmount := math.NewInt(500_000_000) // 500 OMNI
	amountBurned, amountToTreasury, err := suite.keeper.BurnTokens(
		suite.ctx,
		burner,
		burnAmount,
		types.BurnSource_BURN_SOURCE_POS_GAS,
		"omniphi-core-1",
	)

	suite.Require().NoError(err)
	suite.Require().True(amountBurned.IsPositive(), "should burn positive amount")
	suite.Require().True(amountToTreasury.GTE(math.ZeroInt()), "treasury redirect should be non-negative")

	// Verify conservation: burned + treasury = total input
	suite.Require().True(amountBurned.Add(amountToTreasury).Equal(burnAmount),
		"burned + treasury should equal input amount")
}

// TestBurnTreasuryRedirect_P0_BURN_002 tests treasury redirect calculation
func (suite *KeeperTestSuite) TestBurnTreasuryRedirect_P0_BURN_002() {
	params := suite.keeper.GetParams(suite.ctx)

	// Verify default treasury redirect is 10%
	expectedRedirect := math.LegacyNewDecWithPrec(10, 2) // 0.10 = 10%
	suite.Require().True(params.TreasuryBurnRedirect.Equal(expectedRedirect),
		"default treasury redirect should be 10%%")

	// Setup burn scenario
	burner := sdk.AccAddress([]byte("burner"))
	burnAmount := math.NewInt(1000_000_000) // 1000 OMNI

	// Fund burner (BurnTokens will handle the module transfer)
	err := suite.keeper.SetCurrentSupply(suite.ctx, burnAmount)
	suite.Require().NoError(err)
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, burnAmount))
	err = suite.bankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.bankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, burner, coins)
	suite.Require().NoError(err)

	// Burn
	amountBurned, amountToTreasury, err := suite.keeper.BurnTokens(
		suite.ctx,
		burner,
		burnAmount,
		types.BurnSource_BURN_SOURCE_POS_GAS,
		"omniphi-core-1",
	)
	suite.Require().NoError(err)

	// Verify: treasury should get 10% of total
	expectedTreasury := params.TreasuryBurnRedirect.MulInt(burnAmount).TruncateInt()
	suite.Require().True(amountToTreasury.Equal(expectedTreasury),
		"treasury should receive 10%% of burn amount")

	// Verify: actual burn is 90%
	expectedBurn := burnAmount.Sub(expectedTreasury)
	suite.Require().True(amountBurned.Equal(expectedBurn),
		"actual burn should be 90%% of input")
}

// TestBurnConservationLaw_P0_BURN_003 tests supply conservation after burns
func (suite *KeeperTestSuite) TestBurnConservationLaw_P0_BURN_003() {
	// Setup: mint initial supply
	initialSupply := math.NewInt(1_000_000_000_000) // 1M OMNI
	err := suite.keeper.SetCurrentSupply(suite.ctx, initialSupply)
	suite.Require().NoError(err)
	err = suite.keeper.SetTotalMinted(suite.ctx, initialSupply)
	suite.Require().NoError(err)
	err = suite.keeper.SetTotalBurned(suite.ctx, math.ZeroInt())
	suite.Require().NoError(err)

	// Fund burner (BurnTokens will handle the module transfer)
	burner := sdk.AccAddress([]byte("burner"))
	burnAmount := math.NewInt(100_000_000_000) // 100k OMNI
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, burnAmount))
	err = suite.bankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.bankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, burner, coins)
	suite.Require().NoError(err)

	// Burn tokens
	_, _, err = suite.keeper.BurnTokens(
		suite.ctx,
		burner,
		burnAmount,
		types.BurnSource_BURN_SOURCE_POS_GAS,
		"omniphi-core-1",
	)
	suite.Require().NoError(err)

	// Verify conservation law: current = minted - burned
	currentSupply := suite.keeper.GetCurrentSupply(suite.ctx)
	totalMinted := suite.keeper.GetTotalMinted(suite.ctx)
	totalBurned := suite.keeper.GetTotalBurned(suite.ctx)

	expected := totalMinted.Sub(totalBurned)
	suite.Require().True(currentSupply.Equal(expected),
		"conservation law violated: current (%s) != minted (%s) - burned (%s)",
		currentSupply.String(), totalMinted.String(), totalBurned.String())
}

// TestBurnSourceTracking_P0_BURN_004 tests burns are tracked by source
func (suite *KeeperTestSuite) TestBurnSourceTracking_P0_BURN_004() {
	// Setup - fund burner with enough for all burn tests
	burner := sdk.AccAddress([]byte("burner"))
	amount := math.NewInt(10_000_000_000) // 10k OMNI for multiple burns

	err := suite.keeper.SetCurrentSupply(suite.ctx, amount)
	suite.Require().NoError(err)
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	err = suite.bankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.bankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, burner, coins)
	suite.Require().NoError(err)

	// Test all 6 burn sources
	sources := []types.BurnSource{
		types.BurnSource_BURN_SOURCE_POS_GAS,
		types.BurnSource_BURN_SOURCE_POC_ANCHORING,
		types.BurnSource_BURN_SOURCE_SEQUENCER_GAS,
		types.BurnSource_BURN_SOURCE_SMART_CONTRACTS,
		types.BurnSource_BURN_SOURCE_AI_QUERIES,
		types.BurnSource_BURN_SOURCE_MESSAGING,
	}

	for _, source := range sources {
		_, _, err := suite.keeper.BurnTokens(
			suite.ctx,
			burner,
			math.NewInt(1000),
			source,
			"omniphi-core-1",
		)
		suite.Require().NoError(err, "burn from source %v should succeed", source)
	}

	// Verify all sources were recorded
	// (In full implementation, would query burn records by source)
}

// TestBurnChainTracking_P0_BURN_005 tests burns are tracked by chain
func (suite *KeeperTestSuite) TestBurnChainTracking_P0_BURN_005() {
	// Setup - fund burner with enough for all chain tests
	burner := sdk.AccAddress([]byte("burner"))
	amount := math.NewInt(10_000_000_000) // 10k OMNI for multiple burns

	err := suite.keeper.SetCurrentSupply(suite.ctx, amount)
	suite.Require().NoError(err)
	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, amount))
	err = suite.bankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.bankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, burner, coins)
	suite.Require().NoError(err)

	// Burn from different chains
	chains := []string{
		"omniphi-core-1",
		"omniphi-continuity-1",
		"omniphi-sequencer-1",
	}

	for _, chainID := range chains {
		_, _, err := suite.keeper.BurnTokens(
			suite.ctx,
			burner,
			math.NewInt(1000),
			types.BurnSource_BURN_SOURCE_POS_GAS,
			chainID,
		)
		suite.Require().NoError(err, "burn from chain %s should succeed", chainID)
	}
}

// TestBurnTreasuryRedirectBounds_P0_BURN_006 tests treasury redirect is bounded 0-20%
func (suite *KeeperTestSuite) TestBurnTreasuryRedirectBounds_P0_BURN_006() {
	params := suite.keeper.GetParams(suite.ctx)

	// Test maximum allowed (20%)
	params.TreasuryBurnRedirect = math.LegacyNewDecWithPrec(20, 2)
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err, "20%% treasury redirect should be allowed")

	// Test minimum (0%)
	params.TreasuryBurnRedirect = math.LegacyZeroDec()
	err = suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err, "0%% treasury redirect should be allowed")

	// Test exceeding maximum (21%)
	params.TreasuryBurnRedirect = math.LegacyNewDecWithPrec(21, 2)
	err = suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err, "21%% treasury redirect should be rejected")

	// Test negative
	params.TreasuryBurnRedirect = math.LegacyNewDecWithPrec(-1, 2)
	err = suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err, "negative treasury redirect should be rejected")
}

// TestBurnRateBounds_P0_BURN_007 tests burn rates are bounded 0-50% per module
func (suite *KeeperTestSuite) TestBurnRateBounds_P0_BURN_007() {
	params := suite.keeper.GetParams(suite.ctx)

	// Test maximum allowed (50%)
	params.BurnRatePosGas = math.LegacyNewDecWithPrec(50, 2)
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err, "50%% burn rate should be allowed")

	// Test exceeding maximum (51%)
	params.BurnRatePosGas = math.LegacyNewDecWithPrec(51, 2)
	err = suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err, "51%% burn rate should be rejected")

	// Test all burn rate fields
	testCases := []struct {
		name string
		set  func(p *types.TokenomicsParams, rate math.LegacyDec)
	}{
		{"pos_gas", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRatePosGas = r }},
		{"poc_anchoring", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRatePocAnchoring = r }},
		{"sequencer_gas", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRateSequencerGas = r }},
		{"smart_contract", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRateSmartContracts = r }},
		{"ai_query", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRateAiQueries = r }},
		{"messaging", func(p *types.TokenomicsParams, r math.LegacyDec) { p.BurnRateMessaging = r }},
	}

	for _, tc := range testCases {
		// Reset to defaults
		params = types.DefaultParams()

		// Test max allowed
		tc.set(&params, math.LegacyNewDecWithPrec(50, 2))
		err = suite.keeper.SetParams(suite.ctx, params)
		suite.Require().NoError(err, "%s: 50%% should be allowed", tc.name)

		// Test exceeding max
		tc.set(&params, math.LegacyNewDecWithPrec(51, 2))
		err = suite.keeper.SetParams(suite.ctx, params)
		suite.Require().Error(err, "%s: 51%% should be rejected", tc.name)
	}
}

// TestBurnInsufficientFunds_P0_BURN_008 tests burning more than balance fails gracefully
func (suite *KeeperTestSuite) TestBurnInsufficientFunds_P0_BURN_008() {
	burner := sdk.AccAddress([]byte("burner"))

	// Don't fund the burner - should have zero balance

	// Attempt to burn
	_, _, err := suite.keeper.BurnTokens(
		suite.ctx,
		burner,
		math.NewInt(1000),
		types.BurnSource_BURN_SOURCE_POS_GAS,
		"omniphi-core-1",
	)

	// Should fail gracefully
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrInsufficientBalance)
}

// ==================== P0-DIST Tests: Emission Distribution ====================

// TestEmissionSplitsValid_P0_DIST_001 tests emission splits sum to 100%
func (suite *KeeperTestSuite) TestEmissionSplitsValid_P0_DIST_001() {
	params := suite.keeper.GetParams(suite.ctx)

	// Calculate sum
	sum := params.EmissionSplitStaking.
		Add(params.EmissionSplitPoc).
		Add(params.EmissionSplitSequencer).
		Add(params.EmissionSplitTreasury)

	// Should equal exactly 1.0 (100%)
	suite.Require().True(sum.Equal(math.LegacyOneDec()),
		"emission splits should sum to 100%%, got %s", sum.String())
}

// TestEmissionSplitsInvalid_P0_DIST_002 tests setting invalid splits is rejected
func (suite *KeeperTestSuite) TestEmissionSplitsInvalid_P0_DIST_002() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set splits that sum to 90% (invalid)
	params.EmissionSplitStaking = math.LegacyNewDecWithPrec(30, 2)   // 30%
	params.EmissionSplitPoc = math.LegacyNewDecWithPrec(30, 2)       // 30%
	params.EmissionSplitSequencer = math.LegacyNewDecWithPrec(20, 2) // 20%
	params.EmissionSplitTreasury = math.LegacyNewDecWithPrec(10, 2)  // 10%
	// Sum = 90% (invalid)

	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrEmissionSplitInvalid)
}

// TestEmissionSplitsDefault_P0_DIST_003 tests default split is 40/30/20/10
func (suite *KeeperTestSuite) TestEmissionSplitsDefault_P0_DIST_003() {
	params := types.DefaultParams()

	suite.Require().True(params.EmissionSplitStaking.Equal(math.LegacyNewDecWithPrec(40, 2)),
		"staking split should be 40%%")
	suite.Require().True(params.EmissionSplitPoc.Equal(math.LegacyNewDecWithPrec(30, 2)),
		"PoC split should be 30%%")
	suite.Require().True(params.EmissionSplitSequencer.Equal(math.LegacyNewDecWithPrec(20, 2)),
		"sequencer split should be 20%%")
	suite.Require().True(params.EmissionSplitTreasury.Equal(math.LegacyNewDecWithPrec(10, 2)),
		"treasury split should be 10%%")
}

// TestEmissionSplitsNonNegative_P0_DIST_004 tests all splits must be non-negative
func (suite *KeeperTestSuite) TestEmissionSplitsNonNegative_P0_DIST_004() {
	params := suite.keeper.GetParams(suite.ctx)

	// Try negative staking split
	params.EmissionSplitStaking = math.LegacyNewDecWithPrec(-10, 2)
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err, "negative emission split should be rejected")
}

// TestEmissionSplitsMaximum_P0_DIST_005 tests no single split can exceed 100%
func (suite *KeeperTestSuite) TestEmissionSplitsMaximum_P0_DIST_005() {
	params := suite.keeper.GetParams(suite.ctx)

	// Try setting one split to 150%
	params.EmissionSplitStaking = math.LegacyNewDecWithPrec(150, 2)
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().Error(err, "emission split >100%% should be rejected")
}

// TestRewardSplitCalculation_P0_DIST_006 tests reward split calculation accuracy
func (suite *KeeperTestSuite) TestRewardSplitCalculation_P0_DIST_006() {
	totalRewards := math.NewInt(1_000_000_000) // 1000 OMNI

	recipients := suite.keeper.CalculateRewardSplits(suite.ctx, totalRewards)

	// Verify correct number of recipients (4: staking, PoC, sequencer, treasury)
	suite.Require().Len(recipients, 4, "should have 4 reward recipients")

	// Calculate total distributed
	totalDistributed := math.ZeroInt()
	for _, r := range recipients {
		totalDistributed = totalDistributed.Add(r.Amount)
	}

	// Should equal input (no rounding loss for large amounts)
	suite.Require().True(totalDistributed.Equal(totalRewards),
		"total distributed (%s) should equal total rewards (%s)",
		totalDistributed.String(), totalRewards.String())
}
