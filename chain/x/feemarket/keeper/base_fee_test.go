package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestUpdateBaseFee(t *testing.T) {
	f := setupTest(t)

	params := f.keeper.GetParams(f.ctx)

	tests := []struct {
		name                string
		previousUtilization string
		expectIncrease      bool
		expectDecrease      bool
	}{
		{
			name:                "above target - should increase",
			previousUtilization: "0.50", // 50% > 33% target
			expectIncrease:      true,
		},
		{
			name:                "at target - minimal change",
			previousUtilization: "0.33", // exactly at target
			expectIncrease:      false,
			expectDecrease:      false,
		},
		{
			name:                "below target - should decrease",
			previousUtilization: "0.15", // 15% < 33% target
			expectDecrease:      true,
		},
		{
			name:                "very high utilization - strong increase",
			previousUtilization: "0.90", // 90% >> 33% target
			expectIncrease:      true,
		},
		{
			name:                "very low utilization - strong decrease",
			previousUtilization: "0.05", // 5% << 33% target
			expectDecrease:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset to initial base fee
			initialBaseFee := math.LegacyMustNewDecFromStr("0.050")
			err := f.keeper.SetCurrentBaseFee(f.ctx, initialBaseFee)
			require.NoError(t, err)

			// Set previous utilization
			prevUtil := math.LegacyMustNewDecFromStr(tc.previousUtilization)
			err = f.keeper.SetPreviousBlockUtilization(f.ctx, prevUtil)
			require.NoError(t, err)

			// Update base fee
			err = f.keeper.UpdateBaseFee(f.ctx)
			require.NoError(t, err)

			// Get new base fee
			newBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)

			t.Logf("Previous util: %s, Initial base fee: %s, New base fee: %s",
				tc.previousUtilization, initialBaseFee, newBaseFee)

			if tc.expectIncrease {
				require.True(t, newBaseFee.GT(initialBaseFee),
					"Expected base fee to increase: initial=%s, new=%s", initialBaseFee, newBaseFee)
			}

			if tc.expectDecrease {
				require.True(t, newBaseFee.LT(initialBaseFee),
					"Expected base fee to decrease: initial=%s, new=%s", initialBaseFee, newBaseFee)
			}

			// Verify base fee respects min_gas_price_floor
			require.True(t, newBaseFee.GTE(params.MinGasPriceFloor),
				"Base fee %s should not fall below floor %s", newBaseFee, params.MinGasPriceFloor)
		})
	}
}

func TestBaseFeeFloor(t *testing.T) {
	f := setupTest(t)

	params := f.keeper.GetParams(f.ctx)

	// Set base fee just above floor
	nearFloorFee := params.MinGasPriceFloor.Add(math.LegacyMustNewDecFromStr("0.001"))
	err := f.keeper.SetCurrentBaseFee(f.ctx, nearFloorFee)
	require.NoError(t, err)

	// Set very low utilization to force decrease
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.01"))
	require.NoError(t, err)

	// Update base fee multiple times
	for i := 0; i < 20; i++ {
		err = f.keeper.UpdateBaseFee(f.ctx)
		require.NoError(t, err)
	}

	// Verify base fee did not go below floor
	finalBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)
	require.True(t, finalBaseFee.GTE(params.MinGasPriceFloor),
		"Base fee %s fell below floor %s", finalBaseFee, params.MinGasPriceFloor)

	t.Logf("Floor: %s, Final base fee after 20 decreases: %s",
		params.MinGasPriceFloor, finalBaseFee)
}

func TestGetEffectiveGasPrice(t *testing.T) {
	f := setupTest(t)

	tests := []struct {
		name              string
		baseFee           string
		minGasPrice       string
		expectedEffective string
	}{
		{
			name:              "base fee higher than min",
			baseFee:           "0.100",
			minGasPrice:       "0.050",
			expectedEffective: "0.100000000000000000",
		},
		{
			name:              "min gas price higher than base fee",
			baseFee:           "0.030",
			minGasPrice:       "0.050",
			expectedEffective: "0.050000000000000000",
		},
		{
			name:              "equal values",
			baseFee:           "0.050",
			minGasPrice:       "0.050",
			expectedEffective: "0.050000000000000000",
		},
		{
			name:              "very high base fee",
			baseFee:           "1.000",
			minGasPrice:       "0.050",
			expectedEffective: "1.000000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set base fee
			baseFee := math.LegacyMustNewDecFromStr(tc.baseFee)
			err := f.keeper.SetCurrentBaseFee(f.ctx, baseFee)
			require.NoError(t, err)

			// Update params with new min gas price
			params := f.keeper.GetParams(f.ctx)
			params.MinGasPrice = math.LegacyMustNewDecFromStr(tc.minGasPrice)
			err = f.keeper.SetParams(f.ctx, params)
			require.NoError(t, err)

			// Get effective gas price
			effectiveGasPrice := f.keeper.GetEffectiveGasPrice(f.ctx)

			requireDecEqual(t, math.LegacyMustNewDecFromStr(tc.expectedEffective), effectiveGasPrice)
		})
	}
}

func TestBaseFeeDisabled(t *testing.T) {
	f := setupTest(t)

	// Disable base fee
	params := f.keeper.GetParams(f.ctx)
	params.BaseFeeEnabled = false
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	t.Run("effective gas price uses min_gas_price when disabled", func(t *testing.T) {
		effectiveGasPrice := f.keeper.GetEffectiveGasPrice(f.ctx)
		requireDecEqual(t, params.MinGasPrice, effectiveGasPrice)
	})

	t.Run("base fee does not update when disabled", func(t *testing.T) {
		// Set a specific base fee
		initialBaseFee := math.LegacyMustNewDecFromStr("0.075")
		err := f.keeper.SetCurrentBaseFee(f.ctx, initialBaseFee)
		require.NoError(t, err)

		// Set high utilization
		err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.90"))
		require.NoError(t, err)

		// Update base fee (should be no-op when disabled)
		err = f.keeper.UpdateBaseFee(f.ctx)
		require.NoError(t, err)

		// Verify base fee did not change
		baseFee := f.keeper.GetCurrentBaseFee(f.ctx)
		requireDecEqual(t, initialBaseFee, baseFee)
	})
}

func TestBaseFeeElasticityMultiplier(t *testing.T) {
	f := setupTest(t)

	// Test with default elasticity (1.125)
	t.Run("default elasticity multiplier", func(t *testing.T) {
		initialBaseFee := math.LegacyMustNewDecFromStr("0.050")
		err := f.keeper.SetCurrentBaseFee(f.ctx, initialBaseFee)
		require.NoError(t, err)

		// Set utilization significantly above target
		err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.66"))
		require.NoError(t, err)

		// Update base fee
		err = f.keeper.UpdateBaseFee(f.ctx)
		require.NoError(t, err)

		newBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)

		// Should increase but not excessively
		require.True(t, newBaseFee.GT(initialBaseFee))
		require.True(t, newBaseFee.LT(initialBaseFee.Mul(math.LegacyMustNewDecFromStr("2.0"))))

		t.Logf("Initial: %s, New: %s, Multiplier effect: %.2fx",
			initialBaseFee, newBaseFee, newBaseFee.Quo(initialBaseFee).MustFloat64())
	})

	// Test with higher elasticity
	t.Run("high elasticity multiplier", func(t *testing.T) {
		params := f.keeper.GetParams(f.ctx)
		params.ElasticityMultiplier = math.LegacyMustNewDecFromStr("2.0")
		err := f.keeper.SetParams(f.ctx, params)
		require.NoError(t, err)

		initialBaseFee := math.LegacyMustNewDecFromStr("0.050")
		err = f.keeper.SetCurrentBaseFee(f.ctx, initialBaseFee)
		require.NoError(t, err)

		// Set utilization above target
		err = f.keeper.SetPreviousBlockUtilization(f.ctx, math.LegacyMustNewDecFromStr("0.66"))
		require.NoError(t, err)

		// Update base fee
		err = f.keeper.UpdateBaseFee(f.ctx)
		require.NoError(t, err)

		newBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)

		// Should increase more aggressively with higher elasticity
		require.True(t, newBaseFee.GT(initialBaseFee))

		t.Logf("With 2x elasticity - Initial: %s, New: %s, Effect: %.2fx",
			initialBaseFee, newBaseFee, newBaseFee.Quo(initialBaseFee).MustFloat64())
	})
}

func TestBaseFeeSequentialUpdates(t *testing.T) {
	f := setupTest(t)

	// Simulate a sequence of blocks with varying utilization
	utilizationSequence := []string{
		"0.20", // Below target - decrease
		"0.25", // Below target - decrease
		"0.50", // Above target - increase
		"0.80", // Above target - increase
		"0.40", // Above target - increase
		"0.30", // Below target - decrease
		"0.15", // Below target - decrease
	}

	baseFees := make([]math.LegacyDec, 0, len(utilizationSequence)+1)

	// Record initial base fee
	initialBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)
	baseFees = append(baseFees, initialBaseFee)

	for i, utilStr := range utilizationSequence {
		util := math.LegacyMustNewDecFromStr(utilStr)
		err := f.keeper.SetPreviousBlockUtilization(f.ctx, util)
		require.NoError(t, err)

		err = f.keeper.UpdateBaseFee(f.ctx)
		require.NoError(t, err)

		newBaseFee := f.keeper.GetCurrentBaseFee(f.ctx)
		baseFees = append(baseFees, newBaseFee)

		t.Logf("Block %d: Util=%s, BaseFee=%s", i+1, utilStr, newBaseFee)
	}

	// Verify base fees changed appropriately
	require.NotEqual(t, baseFees[0].String(), baseFees[len(baseFees)-1].String(),
		"Base fee should have changed after sequence of blocks")

	// Verify all base fees are above floor
	params := f.keeper.GetParams(f.ctx)
	for i, fee := range baseFees {
		require.True(t, fee.GTE(params.MinGasPriceFloor),
			"Base fee at block %d (%s) below floor %s", i, fee, params.MinGasPriceFloor)
	}
}
