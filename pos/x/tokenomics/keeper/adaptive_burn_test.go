package keeper_test

import (
	"cosmossdk.io/math"
)

// TestCalculateBurnRate tests the tier-based burn rate calculation
func (suite *KeeperTestSuite) TestCalculateBurnRate() {
	tests := []struct {
		name         string
		gasPrice     string
		expectedRate string
		expectedTier string
	}{
		{
			name:         "Very low gas price - low tier",
			gasPrice:     "0.001",
			expectedRate: "0.50",
			expectedTier: "low_fee",
		},
		{
			name:         "Boundary: Exactly 0.01 - mid tier",
			gasPrice:     "0.01",
			expectedRate: "0.75",
			expectedTier: "mid_fee",
		},
		{
			name:         "Mid-range gas price",
			gasPrice:     "0.03",
			expectedRate: "0.75",
			expectedTier: "mid_fee",
		},
		{
			name:         "Boundary: Exactly 0.05 - high tier",
			gasPrice:     "0.05",
			expectedRate: "0.90",
			expectedTier: "high_fee",
		},
		{
			name:         "Very high gas price",
			gasPrice:     "1.00",
			expectedRate: "0.90",
			expectedTier: "high_fee",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			gasPrice := math.LegacyMustNewDecFromStr(tc.gasPrice)
			expectedRate := math.LegacyMustNewDecFromStr(tc.expectedRate)

			burnRate, tier := suite.keeper.CalculateBurnRate(gasPrice)

			suite.Require().True(burnRate.Equal(expectedRate),
				"Expected burn rate %s, got %s", tc.expectedRate, burnRate.String())
			suite.Require().Equal(tc.expectedTier, tier,
				"Expected tier %s, got %s", tc.expectedTier, tier)
		})
	}
}

// TestEstimateBurnForGasPrice_LowFee tests low fee tier allocation
func (suite *KeeperTestSuite) TestEstimateBurnForGasPrice_LowFee() {
	// Low fee: 50% burn, 10% treasury, 40% validators
	gasPrice := math.LegacyMustNewDecFromStr("0.005")
	totalFee := math.NewInt(1_000_000) // 1M microomni

	burnAmount, treasuryAmount, validatorAmount, tier := suite.keeper.EstimateBurnForGasPrice(gasPrice, totalFee)

	// Treasury: 10% of 1M = 100,000
	suite.Require().True(treasuryAmount.Equal(math.NewInt(100_000)),
		"Treasury should be 10%%: expected 100000, got %s", treasuryAmount.String())

	// Burn: 50% of remaining 900,000 = 450,000
	suite.Require().True(burnAmount.Equal(math.NewInt(450_000)),
		"Burn should be 50%% of remainder: expected 450000, got %s", burnAmount.String())

	// Validators: 40% of remainder = 450,000
	suite.Require().True(validatorAmount.Equal(math.NewInt(450_000)),
		"Validators should get remainder: expected 450000, got %s", validatorAmount.String())

	suite.Require().Equal("low_fee", tier)

	// Verify sum equals total
	sum := burnAmount.Add(treasuryAmount).Add(validatorAmount)
	suite.Require().True(sum.Equal(totalFee),
		"Sum should equal total fee: expected %s, got %s", totalFee.String(), sum.String())
}

// TestEstimateBurnForGasPrice_MidFee tests mid fee tier allocation
func (suite *KeeperTestSuite) TestEstimateBurnForGasPrice_MidFee() {
	// Mid fee: 75% burn, 10% treasury, 15% validators
	gasPrice := math.LegacyMustNewDecFromStr("0.03")
	totalFee := math.NewInt(1_000_000)

	burnAmount, treasuryAmount, validatorAmount, tier := suite.keeper.EstimateBurnForGasPrice(gasPrice, totalFee)

	// Treasury: 10% = 100,000
	suite.Require().True(treasuryAmount.Equal(math.NewInt(100_000)))

	// Burn: 75% of 900,000 = 675,000
	suite.Require().True(burnAmount.Equal(math.NewInt(675_000)))

	// Validators: 25% of 900,000 = 225,000
	suite.Require().True(validatorAmount.Equal(math.NewInt(225_000)))

	suite.Require().Equal("mid_fee", tier)
}

// TestEstimateBurnForGasPrice_HighFee tests high fee tier allocation
func (suite *KeeperTestSuite) TestEstimateBurnForGasPrice_HighFee() {
	// High fee: 90% burn, 10% treasury, 0% validators
	gasPrice := math.LegacyMustNewDecFromStr("0.10")
	totalFee := math.NewInt(1_000_000)

	burnAmount, treasuryAmount, validatorAmount, tier := suite.keeper.EstimateBurnForGasPrice(gasPrice, totalFee)

	// Treasury: 10% = 100,000
	suite.Require().True(treasuryAmount.Equal(math.NewInt(100_000)))

	// Burn: 90% of 900,000 = 810,000
	suite.Require().True(burnAmount.Equal(math.NewInt(810_000)))

	// Validators: 10% of 900,000 = 90,000
	suite.Require().True(validatorAmount.Equal(math.NewInt(90_000)))

	suite.Require().Equal("high_fee", tier)
}

// TestGetBurnTiers tests that burn tiers are configured correctly
func (suite *KeeperTestSuite) TestGetBurnTiers() {
	tiers := suite.keeper.GetBurnTiers()

	// Should have exactly 3 tiers
	suite.Require().Len(tiers, 3)

	// Verify tier order and rates
	suite.Require().Equal("low_fee", tiers[0].Name)
	suite.Require().True(tiers[0].BurnRate.Equal(math.LegacyMustNewDecFromStr("0.50")))

	suite.Require().Equal("mid_fee", tiers[1].Name)
	suite.Require().True(tiers[1].BurnRate.Equal(math.LegacyMustNewDecFromStr("0.75")))

	suite.Require().Equal("high_fee", tiers[2].Name)
	suite.Require().True(tiers[2].BurnRate.Equal(math.LegacyMustNewDecFromStr("0.90")))
}

// TestAdaptiveBurn_ScenarioAnalysis tests real-world scenarios
func (suite *KeeperTestSuite) TestAdaptiveBurn_ScenarioAnalysis() {
	scenarios := []struct {
		name         string
		gasPrice     string
		totalFee     int64
		expectedBurn int64
		description  string
	}{
		{
			name:         "Calm network - low fees",
			gasPrice:     "0.005",
			totalFee:     10_000_000, // 10 OMNI
			expectedBurn: 4_500_000,  // 4.5 OMNI (50% of 9 OMNI)
			description:  "Normal conditions, moderate burn",
		},
		{
			name:         "Busy network - medium fees",
			gasPrice:     "0.02",
			totalFee:     50_000_000, // 50 OMNI
			expectedBurn: 33_750_000, // 33.75 OMNI (75% of 45 OMNI)
			description:  "Network congestion, higher burn",
		},
		{
			name:         "Congested network - high fees",
			gasPrice:     "0.08",
			totalFee:     100_000_000, // 100 OMNI
			expectedBurn: 81_000_000,  // 81 OMNI (90% of 90 OMNI)
			description:  "Severe congestion, maximum burn",
		},
	}

	for _, sc := range scenarios {
		suite.Run(sc.name, func() {
			gasPrice := math.LegacyMustNewDecFromStr(sc.gasPrice)
			totalFee := math.NewInt(sc.totalFee)

			burnAmount, _, _, _ := suite.keeper.EstimateBurnForGasPrice(gasPrice, totalFee)

			suite.Require().True(burnAmount.Equal(math.NewInt(sc.expectedBurn)),
				"%s: expected burn %d, got %s", sc.description, sc.expectedBurn, burnAmount.String())
		})
	}
}
