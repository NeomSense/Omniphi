package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestSelectBurnTier(t *testing.T) {
	f := setupTest(t)

	tests := []struct {
		name             string
		utilization      string
		expectedTier     string
		expectedBurnRate string
	}{
		{
			name:             "cool tier - 5% utilization",
			utilization:      "0.05",
			expectedTier:     "cool",
			expectedBurnRate: "0.100000000000000000",
		},
		{
			name:             "cool tier - 15% utilization",
			utilization:      "0.15",
			expectedTier:     "cool",
			expectedBurnRate: "0.100000000000000000",
		},
		{
			name:             "normal tier - 16% utilization (just above cool)",
			utilization:      "0.16",
			expectedTier:     "normal",
			expectedBurnRate: "0.200000000000000000",
		},
		{
			name:             "normal tier - 25% utilization",
			utilization:      "0.25",
			expectedTier:     "normal",
			expectedBurnRate: "0.200000000000000000",
		},
		{
			name:             "normal tier - 32% utilization (just below hot)",
			utilization:      "0.32",
			expectedTier:     "normal",
			expectedBurnRate: "0.200000000000000000",
		},
		{
			name:             "hot tier - 33% utilization (target)",
			utilization:      "0.33",
			expectedTier:     "hot",
			expectedBurnRate: "0.400000000000000000",
		},
		{
			name:             "hot tier - 50% utilization",
			utilization:      "0.50",
			expectedTier:     "hot",
			expectedBurnRate: "0.400000000000000000",
		},
		{
			name:             "hot tier - 100% utilization",
			utilization:      "1.00",
			expectedTier:     "hot",
			expectedBurnRate: "0.400000000000000000",
		},
		{
			name:             "cool tier - 0% utilization (empty block)",
			utilization:      "0.00",
			expectedTier:     "cool",
			expectedBurnRate: "0.100000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set previous utilization to simulate realistic scenario
			util := math.LegacyMustNewDecFromStr(tc.utilization)
			err := f.keeper.SetPreviousBlockUtilization(f.ctx, util)
			require.NoError(t, err)

			// Select burn tier
			burnRate, tier := f.keeper.SelectBurnTier(f.ctx)
			require.Equal(t, tc.expectedTier, tier)
			requireDecEqual(t, math.LegacyMustNewDecFromStr(tc.expectedBurnRate), burnRate)
		})
	}
}

func TestGetCurrentBurnRate(t *testing.T) {
	f := setupTest(t)

	// Set low utilization (cool tier)
	err := f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.10"))
	require.NoError(t, err)

	burnRate := f.keeper.GetCurrentBurnRate(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.10"), burnRate)

	// Set medium utilization (normal tier)
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.25"))
	require.NoError(t, err)

	burnRate = f.keeper.GetCurrentBurnRate(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.20"), burnRate)

	// Set high utilization (hot tier)
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.50"))
	require.NoError(t, err)

	burnRate = f.keeper.GetCurrentBurnRate(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.40"), burnRate)
}

func TestBurnRateMaxCap(t *testing.T) {
	f := setupTest(t)

	// Get params and verify max_burn_ratio
	params := f.keeper.GetParams(f.ctx)
	maxBurn := params.MaxBurnRatio

	// Set utilization that would trigger hot tier
	err := f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.80"))
	require.NoError(t, err)

	burnRate := f.keeper.GetCurrentBurnRate(f.ctx)

	// Burn rate should not exceed max_burn_ratio
	require.True(t, burnRate.LTE(maxBurn),
		"Burn rate %s exceeds max %s", burnRate, maxBurn)
}

func TestBurnTierThresholds(t *testing.T) {
	f := setupTest(t)

	params := f.keeper.GetParams(f.ctx)

	// Test exact threshold boundaries
	testCases := []struct {
		utilization  string
		expectedTier string
	}{
		// Just below cool threshold (0.16)
		{"0.159999999999999999", "cool"},
		// At cool threshold (becomes normal)
		{"0.160000000000000000", "normal"},
		// Just below hot threshold (0.33)
		{"0.329999999999999999", "normal"},
		// At hot threshold
		{"0.330000000000000000", "hot"},
	}

	for _, tc := range testCases {
		t.Run("util_"+tc.utilization, func(t *testing.T) {
			util := math.LegacyMustNewDecFromStr(tc.utilization)
			err := f.keeper.SetPreviousBlockUtilization(f.ctx, util)
			require.NoError(t, err)

			_, tier := f.keeper.SelectBurnTier(f.ctx)
			require.Equal(t, tc.expectedTier, tier,
				"At utilization %s (cool=%s, hot=%s)",
				tc.utilization, params.UtilCoolThreshold, params.UtilHotThreshold)
		})
	}
}

func TestCustomBurnRates(t *testing.T) {
	f := setupTest(t)

	// Update params with custom burn rates
	params := f.keeper.GetParams(f.ctx)
	params.BurnCool = math.LegacyMustNewDecFromStr("0.15")
	params.BurnNormal = math.LegacyMustNewDecFromStr("0.25")
	params.BurnHot = math.LegacyMustNewDecFromStr("0.50")
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Test cool tier with custom rate
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.10"))
	require.NoError(t, err)

	burnRate, tier := f.keeper.SelectBurnTier(f.ctx)
	require.Equal(t, "cool", tier)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.15"), burnRate)

	// Test normal tier with custom rate
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.25"))
	require.NoError(t, err)

	burnRate, tier = f.keeper.SelectBurnTier(f.ctx)
	require.Equal(t, "normal", tier)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.25"), burnRate)

	// Test hot tier with custom rate
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.50"))
	require.NoError(t, err)

	burnRate, tier = f.keeper.SelectBurnTier(f.ctx)
	require.Equal(t, "hot", tier)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.50"), burnRate)
}

func TestBurnTierTransitions(t *testing.T) {
	f := setupTest(t)

	// Simulate increasing utilization across tiers
	utilizationSequence := []struct {
		util         string
		expectedTier string
	}{
		{"0.05", "cool"},
		{"0.10", "cool"},
		{"0.15", "cool"},
		{"0.16", "normal"},
		{"0.20", "normal"},
		{"0.30", "normal"},
		{"0.33", "hot"},
		{"0.50", "hot"},
		{"0.80", "hot"},
		// Coming back down
		{"0.50", "hot"},
		{"0.32", "normal"},
		{"0.20", "normal"},
		{"0.15", "cool"},
		{"0.05", "cool"},
	}

	for i, seq := range utilizationSequence {
		t.Run("step_"+string(rune(i)), func(t *testing.T) {
			util := math.LegacyMustNewDecFromStr(seq.util)
			err := f.keeper.SetPreviousBlockUtilization(f.ctx, util)
			require.NoError(t, err)

			_, tier := f.keeper.SelectBurnTier(f.ctx)
			require.Equal(t, seq.expectedTier, tier,
				"At utilization %s expected tier %s but got %s",
				seq.util, seq.expectedTier, tier)
		})
	}
}
