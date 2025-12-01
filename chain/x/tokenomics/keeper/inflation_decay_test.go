package keeper_test

import (
	"cosmossdk.io/math"
)

// TestCalculateDecayingInflation tests the year-based inflation decay schedule
func (suite *KeeperTestSuite) TestCalculateDecayingInflation() {
	// Set inflation_min to 0.5% to test the full decay schedule
	params := suite.keeper.GetParams(suite.ctx)
	params.InflationMin = math.LegacyMustNewDecFromStr("0.005000000000000000") // 0.5%
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	tests := []struct {
		name         string
		year         int64
		expectedRate string
		description  string
	}{
		{
			name:         "Year 0 (Launch)",
			year:         0,
			expectedRate: "0.030000000000000000", // 3.00%
			description:  "Initial inflation rate",
		},
		{
			name:         "Year 1",
			year:         1,
			expectedRate: "0.027500000000000000", // 2.75%
			description:  "First decay step",
		},
		{
			name:         "Year 2",
			year:         2,
			expectedRate: "0.025000000000000000", // 2.50%
			description:  "Second decay step",
		},
		{
			name:         "Year 3",
			year:         3,
			expectedRate: "0.022500000000000000", // 2.25%
			description:  "Third decay step",
		},
		{
			name:         "Year 4",
			year:         4,
			expectedRate: "0.020000000000000000", // 2.00%
			description:  "Fourth decay step",
		},
		{
			name:         "Year 5",
			year:         5,
			expectedRate: "0.017500000000000000", // 1.75%
			description:  "Fifth decay step",
		},
		{
			name:         "Year 6 (Start continuous decay)",
			year:         6,
			expectedRate: "0.015000000000000000", // 1.50%
			description:  "First year of 0.25%/year decay",
		},
		{
			name:         "Year 7",
			year:         7,
			expectedRate: "0.012500000000000000", // 1.25%
			description:  "Second year of continuous decay",
		},
		{
			name:         "Year 8",
			year:         8,
			expectedRate: "0.010000000000000000", // 1.00%
			description:  "Third year of continuous decay",
		},
		{
			name:         "Year 9",
			year:         9,
			expectedRate: "0.007500000000000000", // 0.75%
			description:  "Fourth year of continuous decay",
		},
		{
			name:         "Year 10 (Floor reached)",
			year:         10,
			expectedRate: "0.005000000000000000", // 0.50% (floor)
			description:  "Fifth year - hits floor",
		},
		{
			name:         "Year 11 (Floor maintained)",
			year:         11,
			expectedRate: "0.005000000000000000", // 0.50% (floor)
			description:  "Stays at floor",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			// Set block height to simulate the year
			blocksPerYear := int64(4_507_680)
			targetHeight := int64(1) + (tc.year * blocksPerYear)

			// Create context with specific height
			ctx := suite.ctx.WithBlockHeight(targetHeight)

			// Calculate inflation
			inflationRate := suite.keeper.CalculateDecayingInflation(ctx)

			// Check expected rate
			expectedDec := math.LegacyMustNewDecFromStr(tc.expectedRate)
			suite.Require().True(inflationRate.Equal(expectedDec),
				"Year %d: expected %s, got %s (%s)",
				tc.year, tc.expectedRate, inflationRate.String(), tc.description)
		})
	}
}

// TestCalculateDecayingInflation_EnforceFloor tests that the floor is enforced
func (suite *KeeperTestSuite) TestCalculateDecayingInflation_EnforceFloor() {
	// Set inflation_min to 0.5% for this test
	params := suite.keeper.GetParams(suite.ctx)
	params.InflationMin = math.LegacyMustNewDecFromStr("0.005000000000000000") // 0.5%
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	// Test far future (year 50) - should still be at floor
	blocksPerYear := int64(4_507_680)
	futureHeight := int64(1) + (50 * blocksPerYear)
	ctx := suite.ctx.WithBlockHeight(futureHeight)

	inflationRate := suite.keeper.CalculateDecayingInflation(ctx)

	// Should be at floor (0.5%)
	floor := math.LegacyMustNewDecFromStr("0.005000000000000000")
	suite.Require().True(inflationRate.Equal(floor),
		"Year 50 should still be at floor: expected %s, got %s",
		floor.String(), inflationRate.String())
}

// TestGetCurrentYear tests the year calculation from block height
func (suite *KeeperTestSuite) TestGetCurrentYear() {
	blocksPerYear := int64(4_507_680)

	tests := []struct {
		height       int64
		expectedYear int64
	}{
		{1, 0},                           // Genesis
		{blocksPerYear, 0},               // End of year 0
		{blocksPerYear + 1, 1},           // Start of year 1
		{2 * blocksPerYear, 1},           // End of year 1
		{2*blocksPerYear + 1, 2},         // Start of year 2
		{10*blocksPerYear + 1000, 10},    // Year 10
	}

	for _, tc := range tests {
		ctx := suite.ctx.WithBlockHeight(tc.height)
		year := suite.keeper.GetCurrentYear(ctx)
		suite.Require().Equal(tc.expectedYear, year,
			"Height %d should be year %d, got %d", tc.height, tc.expectedYear, year)
	}
}

// TestCalculateDecayingAnnualProvisions tests annual provisions calculation
func (suite *KeeperTestSuite) TestCalculateDecayingAnnualProvisions() {
	// Set supply to 750M (genesis supply)
	supply := math.NewInt(750_000_000_000_000) // 750M OMNI (6 decimals)
	params := suite.keeper.GetParams(suite.ctx)
	params.CurrentTotalSupply = supply
	params.TotalMinted = supply // Must match current supply for accounting
	params.TotalBurned = math.ZeroInt()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	// At year 0, inflation should be 3%
	ctx := suite.ctx.WithBlockHeight(1)
	annualProvisions := suite.keeper.CalculateDecayingAnnualProvisions(ctx)

	// Expected: 750M * 0.03 = 22.5M OMNI
	expected := math.NewInt(22_500_000_000_000)
	suite.Require().True(annualProvisions.Equal(expected),
		"Annual provisions at 3%% inflation should be 22.5M OMNI: expected %s, got %s",
		expected.String(), annualProvisions.String())
}

// TestCalculateBlockProvision tests per-block provision calculation
func (suite *KeeperTestSuite) TestCalculateBlockProvision() {
	// Set supply to 750M
	supply := math.NewInt(750_000_000_000_000)
	params := suite.keeper.GetParams(suite.ctx)
	params.CurrentTotalSupply = supply
	params.TotalMinted = supply // Must match current supply for accounting
	params.TotalBurned = math.ZeroInt()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	ctx := suite.ctx.WithBlockHeight(1) // Year 0
	blockProvision := suite.keeper.CalculateBlockProvision(ctx)

	// Expected: 22.5M / 4,507,680 blocks = ~4.99 OMNI per block
	// 22,500,000,000,000 / 4,507,680 = 4,992,517 (with 6 decimals)
	expectedMin := math.NewInt(4_900_000) // ~4.9 OMNI (6 decimals)
	expectedMax := math.NewInt(5_100_000) // ~5.1 OMNI (6 decimals)

	suite.Require().True(blockProvision.GT(expectedMin) && blockProvision.LT(expectedMax),
		"Block provision should be ~5 OMNI: got %s (expected between %s and %s)",
		blockProvision.String(), expectedMin.String(), expectedMax.String())
}

// TestMintInflation_SupplyCapEnforcement tests that minting respects supply cap
func (suite *KeeperTestSuite) TestMintInflation_SupplyCapEnforcement() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set supply very close to cap (within 1000 microomni)
	almostCap := params.TotalSupplyCap.Sub(math.NewInt(1000))
	params.CurrentTotalSupply = almostCap
	params.TotalMinted = almostCap // Must match current supply for accounting
	params.TotalBurned = math.ZeroInt()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	// Try to mint inflation
	err = suite.keeper.MintInflation(suite.ctx)
	suite.Require().NoError(err)

	// Verify we didn't exceed cap
	finalParams := suite.keeper.GetParams(suite.ctx)
	suite.Require().True(finalParams.CurrentTotalSupply.LTE(params.TotalSupplyCap),
		"Minting should not exceed supply cap: cap=%s, supply=%s",
		params.TotalSupplyCap.String(), finalParams.CurrentTotalSupply.String())
}

// TestMintInflation_AtCap tests that no inflation is minted when at cap
func (suite *KeeperTestSuite) TestMintInflation_AtCap() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set supply exactly at cap
	params.CurrentTotalSupply = params.TotalSupplyCap
	params.TotalMinted = params.TotalSupplyCap // Must match current supply for accounting
	params.TotalBurned = math.ZeroInt()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	initialSupply := params.CurrentTotalSupply

	// Try to mint inflation
	err = suite.keeper.MintInflation(suite.ctx)
	suite.Require().NoError(err)

	// Verify no change
	finalParams := suite.keeper.GetParams(suite.ctx)
	suite.Require().True(finalParams.CurrentTotalSupply.Equal(initialSupply),
		"No inflation should be minted when at cap")
}

// TestGetInflationForecast tests multi-year inflation projection
func (suite *KeeperTestSuite) TestGetInflationForecast() {
	// Set initial supply
	supply := math.NewInt(750_000_000_000_000)
	params := suite.keeper.GetParams(suite.ctx)
	params.CurrentTotalSupply = supply
	params.TotalMinted = supply // Must match current supply for accounting
	params.TotalBurned = math.ZeroInt()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	// Get 7-year forecast
	ctx := suite.ctx.WithBlockHeight(1) // Start at year 0
	forecasts := suite.keeper.GetInflationForecast(ctx, 7)

	// Verify we got 7 years
	suite.Require().Len(forecasts, 7)

	// Verify year 0 forecast
	suite.Require().Equal(int64(0), forecasts[0].Year)
	expectedRate0 := math.LegacyMustNewDecFromStr("0.03")
	suite.Require().True(forecasts[0].InflationRate.Equal(expectedRate0),
		"Year 0 should be 3%%: got %s", forecasts[0].InflationRate.String())

	// Verify year 6 forecast (continuous decay)
	suite.Require().Equal(int64(6), forecasts[6].Year)
	expectedRate6 := math.LegacyMustNewDecFromStr("0.015")
	suite.Require().True(forecasts[6].InflationRate.Equal(expectedRate6),
		"Year 6 should be 1.5%%: got %s", forecasts[6].InflationRate.String())

	// Verify supply increases each year
	for i := 1; i < len(forecasts); i++ {
		suite.Require().True(forecasts[i].Supply.GT(forecasts[i-1].Supply),
			"Supply should increase year over year")
	}
}
