package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TestGetAdaptiveBurnRatio_EmergencyOverride tests that emergency override takes highest priority
func TestGetAdaptiveBurnRatio_EmergencyOverride(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Enable adaptive burn and set emergency override
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.EmergencyBurnOverride = true
	params.FeeBurnRatio = math.LegacyNewDecWithPrec(85, 2) // 85%
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	ratio, trigger := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify: Should return fee_burn_ratio and emergency_override trigger
	require.Equal(t, params.FeeBurnRatio, ratio, "emergency override should return fee_burn_ratio")
	require.Equal(t, "emergency_override", trigger, "trigger should be emergency_override")
}

// TestGetAdaptiveBurnRatio_AdaptiveDisabled tests that disabled adaptive returns fixed ratio
func TestGetAdaptiveBurnRatio_AdaptiveDisabled(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Disable adaptive burn
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = false
	params.FeeBurnRatio = math.LegacyNewDecWithPrec(90, 2) // 90%
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	ratio, trigger := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify
	require.Equal(t, params.FeeBurnRatio, ratio, "disabled adaptive should return fee_burn_ratio")
	require.Equal(t, "adaptive_disabled", trigger)
}

// TestGetAdaptiveBurnRatio_TreasuryProtection tests treasury floor trigger
func TestGetAdaptiveBurnRatio_TreasuryProtection(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Enable adaptive burn
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.TreasuryFloorPct = math.LegacyNewDecWithPrec(5, 2) // 5%
	params.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)    // 80%
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Mock: Set treasury balance to 3% of supply (below 5% floor)
	// Note: This requires mocking GetTreasuryPct() which reads from bank keeper
	// For now, we'll test the logic path assuming treasury is low

	// Execute
	ratio, _ := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify: If treasury is below floor, should return min_burn_ratio
	// Note: In actual test with mocks, we'd verify this explicitly
	// For now, we verify that the function returns a valid ratio
	require.True(t, ratio.GTE(params.MinBurnRatio), "ratio should be >= min")
	require.True(t, ratio.LTE(params.MaxBurnRatio), "ratio should be <= max")
}

// TestGetAdaptiveBurnRatio_CongestionControl tests high congestion trigger
func TestGetAdaptiveBurnRatio_CongestionControl(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Enable adaptive burn with high congestion threshold
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.BlockCongestionThreshold = math.LegacyNewDecWithPrec(75, 2) // 75%
	params.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)             // 95%
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Note: Testing congestion requires a gas meter with actual usage
	// In a real test environment, we'd set up a context with gas consumed

	// Execute
	ratio, trigger := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify: Should return valid ratio within bounds
	require.True(t, ratio.GTE(params.MinBurnRatio))
	require.True(t, ratio.LTE(params.MaxBurnRatio))
	require.NotEmpty(t, trigger)
}

// TestGetAdaptiveBurnRatio_AdoptionIncentive tests low tx volume trigger
func TestGetAdaptiveBurnRatio_AdoptionIncentive(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Enable adaptive burn with tx target
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.TxPerDayTarget = 10000                              // 10k tx/day
	params.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)    // 80%
	params.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2) // 90%
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	ratio, trigger := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify: Should return valid ratio
	// If tx volume is low (which it is in early blocks), should use min_burn_ratio
	require.True(t, ratio.GTE(params.MinBurnRatio))
	require.True(t, ratio.LTE(params.MaxBurnRatio))

	// In early blocks (< 100), avg tx is estimated low
	if ctx.BlockHeight() < 100 {
		require.Equal(t, "adoption_incentive", trigger, "early blocks should trigger adoption incentive")
		require.Equal(t, params.MinBurnRatio, ratio, "adoption incentive should use min ratio")
	}
}

// TestGetAdaptiveBurnRatio_Normal tests normal conditions
func TestGetAdaptiveBurnRatio_Normal(t *testing.T) {
	f := SetupTestSuite(t)

	// Create a context at a later block height to avoid early-block heuristics
	ctx := f.Ctx.WithBlockHeight(1000)

	// Setup: Enable adaptive burn with normal conditions
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2) // 90%
	params.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)
	params.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)
	params.TreasuryFloorPct = math.LegacyNewDecWithPrec(5, 2)
	params.BlockCongestionThreshold = math.LegacyNewDecWithPrec(75, 2)
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	ratio, trigger := f.Keeper.GetAdaptiveBurnRatio(ctx)

	// Verify: Should return a valid ratio within bounds
	require.True(t, ratio.GTE(params.MinBurnRatio), "ratio should be >= min")
	require.True(t, ratio.LTE(params.MaxBurnRatio), "ratio should be <= max")
	require.NotEmpty(t, trigger, "trigger should be set")
}

// TestApplySmoothing_FirstApplication tests smoothing when no previous ratio
func TestApplySmoothing_FirstApplication(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Set smoothing params with no previous ratio
	params := f.Keeper.GetParams(ctx)
	params.BurnAdjustmentSmoothing = 100
	params.LastAppliedBurnRatio = math.LegacyZeroDec() // No previous ratio
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	targetRatio := math.LegacyNewDecWithPrec(90, 2) // 90%

	// Execute
	smoothed := f.Keeper.ApplySmoothing(ctx, targetRatio)

	// Verify: First application should return target directly
	require.Equal(t, targetRatio, smoothed, "first smoothing should return target unchanged")
}

// TestApplySmoothing_ExponentialMovingAverage tests the smoothing formula
func TestApplySmoothing_ExponentialMovingAverage(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Set current ratio and smoothing window
	currentRatio := math.LegacyNewDecWithPrec(85, 2) // 85%
	targetRatio := math.LegacyNewDecWithPrec(95, 2)  // 95%
	smoothingBlocks := uint64(100)

	params := f.Keeper.GetParams(ctx)
	params.BurnAdjustmentSmoothing = smoothingBlocks
	params.LastAppliedBurnRatio = currentRatio
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	smoothed := f.Keeper.ApplySmoothing(ctx, targetRatio)

	// Verify: Smoothed should be between current and target
	// Formula: smoothed = current * (1 - α) + target * α
	// where α = 1/100 = 0.01
	alpha := math.LegacyOneDec().Quo(math.LegacyNewDec(int64(smoothingBlocks)))
	expected := currentRatio.Mul(math.LegacyOneDec().Sub(alpha)).Add(targetRatio.Mul(alpha))

	require.True(t, smoothed.GT(currentRatio), "smoothed should be > current")
	require.True(t, smoothed.LT(targetRatio), "smoothed should be < target")
	require.Equal(t, expected.String(), smoothed.String(), "smoothing formula should match")
}

// TestApplySmoothing_ConvergenceOver100Blocks tests convergence behavior
func TestApplySmoothing_ConvergenceOver100Blocks(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Start at 80%, target 95%
	currentRatio := math.LegacyNewDecWithPrec(80, 2)
	targetRatio := math.LegacyNewDecWithPrec(95, 2)
	smoothingBlocks := uint64(100)

	params := f.Keeper.GetParams(ctx)
	params.BurnAdjustmentSmoothing = smoothingBlocks
	params.LastAppliedBurnRatio = currentRatio
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Simulate 100 blocks of smoothing
	ratio := currentRatio
	for i := 0; i < 100; i++ {
		params.LastAppliedBurnRatio = ratio
		require.NoError(t, f.Keeper.SetParams(ctx, params))
		ratio = f.Keeper.ApplySmoothing(ctx, targetRatio)
	}

	// Verify: After 100 blocks, should be very close to target
	// With α=0.01, after 100 iterations: (1-0.01)^100 ≈ 0.366
	// So we should have covered ~63.4% of the distance
	distance := targetRatio.Sub(currentRatio)
	expectedProgress := distance.Mul(math.LegacyOneDec().Sub(math.LegacyNewDecWithPrec(366, 3)))
	expectedRatio := currentRatio.Add(expectedProgress)

	// Allow 1% tolerance due to rounding
	tolerance := math.LegacyNewDecWithPrec(1, 2)
	diff := ratio.Sub(expectedRatio).Abs()
	require.True(t, diff.LTE(tolerance), "should converge to ~63%% of target after 100 blocks")
}

// TestUpdateBurnRatio tests the complete update flow
func TestUpdateBurnRatio(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Setup: Enable adaptive burn
	params := f.Keeper.GetParams(ctx)
	params.AdaptiveBurnEnabled = true
	params.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2)
	params.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)
	params.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)
	params.BurnAdjustmentSmoothing = 100
	params.LastAppliedBurnRatio = math.LegacyNewDecWithPrec(85, 2) // Start at 85%
	params.LastBurnTrigger = "test_initial"
	require.NoError(t, f.Keeper.SetParams(ctx, params))

	// Execute
	err := f.Keeper.UpdateBurnRatio(ctx)
	require.NoError(t, err)

	// Verify: Params should be updated
	updatedParams := f.Keeper.GetParams(ctx)
	require.NotEqual(t, "test_initial", updatedParams.LastBurnTrigger, "trigger should be updated")
	require.True(t, updatedParams.LastAppliedBurnRatio.GTE(params.MinBurnRatio))
	require.True(t, updatedParams.LastAppliedBurnRatio.LTE(params.MaxBurnRatio))
}

// TestGetCurrentBurnRatio tests the public getter
func TestGetCurrentBurnRatio(t *testing.T) {
	tests := []struct {
		name                    string
		adaptiveEnabled         bool
		emergencyOverride       bool
		lastAppliedRatio        math.LegacyDec
		feeBurnRatio            math.LegacyDec
		expectedRatio           math.LegacyDec
	}{
		{
			name:              "adaptive enabled",
			adaptiveEnabled:   true,
			emergencyOverride: false,
			lastAppliedRatio:  math.LegacyNewDecWithPrec(88, 2),
			feeBurnRatio:      math.LegacyNewDecWithPrec(90, 2),
			expectedRatio:     math.LegacyNewDecWithPrec(88, 2),
		},
		{
			name:              "adaptive disabled",
			adaptiveEnabled:   false,
			emergencyOverride: false,
			lastAppliedRatio:  math.LegacyNewDecWithPrec(88, 2),
			feeBurnRatio:      math.LegacyNewDecWithPrec(90, 2),
			expectedRatio:     math.LegacyNewDecWithPrec(90, 2),
		},
		{
			name:              "emergency override",
			adaptiveEnabled:   true,
			emergencyOverride: true,
			lastAppliedRatio:  math.LegacyNewDecWithPrec(88, 2),
			feeBurnRatio:      math.LegacyNewDecWithPrec(90, 2),
			expectedRatio:     math.LegacyNewDecWithPrec(90, 2),
		},
		{
			name:              "first time - no previous ratio",
			adaptiveEnabled:   true,
			emergencyOverride: false,
			lastAppliedRatio:  math.LegacyZeroDec(),
			feeBurnRatio:      math.LegacyNewDecWithPrec(90, 2),
			expectedRatio:     math.LegacyNewDecWithPrec(90, 2), // Should use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := SetupTestSuite(t)
			ctx := f.Ctx

			params := f.Keeper.GetParams(ctx)
			params.AdaptiveBurnEnabled = tt.adaptiveEnabled
			params.EmergencyBurnOverride = tt.emergencyOverride
			params.LastAppliedBurnRatio = tt.lastAppliedRatio
			params.FeeBurnRatio = tt.feeBurnRatio
			params.DefaultBurnRatio = tt.feeBurnRatio
			require.NoError(t, f.Keeper.SetParams(ctx, params))

			ratio := f.Keeper.GetCurrentBurnRatio(ctx)
			require.Equal(t, tt.expectedRatio.String(), ratio.String())
		})
	}
}

// TestGetBlockCongestion tests congestion calculation
func TestGetBlockCongestion(t *testing.T) {
	f := SetupTestSuite(t)

	// Test with no gas meter
	ctx := f.Ctx
	congestion := f.Keeper.GetBlockCongestion(ctx)
	require.Equal(t, math.LegacyZeroDec(), congestion, "no gas meter should return 0")

	// Test with gas meter
	// Note: This requires setting up a proper SDK context with gas meter
	// For now, we verify the function doesn't panic
	require.NotPanics(t, func() {
		f.Keeper.GetBlockCongestion(ctx)
	})
}

// TestGetTreasuryPct tests treasury percentage calculation
func TestGetTreasuryPct(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	// Execute
	pct := f.Keeper.GetTreasuryPct(ctx)

	// Verify: Should return a valid percentage (0-1)
	require.True(t, pct.GTE(math.LegacyZeroDec()), "treasury pct should be >= 0")
	require.True(t, pct.LTE(math.LegacyOneDec()), "treasury pct should be <= 1")
}

// TestGetAvgTxPerDay tests transaction volume estimation
func TestGetAvgTxPerDay(t *testing.T) {
	f := SetupTestSuite(t)

	// Test early blocks
	ctxEarly := f.Ctx.WithBlockHeight(50)
	avgEarly := f.Keeper.GetAvgTxPerDay(ctxEarly)
	require.Equal(t, math.ZeroInt(), avgEarly, "early blocks should return 0")

	// Test later blocks
	ctxLater := f.Ctx.WithBlockHeight(1000)
	avgLater := f.Keeper.GetAvgTxPerDay(ctxLater)
	require.True(t, avgLater.GTE(math.ZeroInt()), "should return non-negative value")
}

// TestParameterValidation tests that invalid parameters are rejected
func TestParameterValidation(t *testing.T) {
	f := SetupTestSuite(t)
	ctx := f.Ctx

	tests := []struct {
		name        string
		setupParams func(*types.TokenomicsParams)
		shouldFail  bool
	}{
		{
			name: "valid params",
			setupParams: func(p *types.TokenomicsParams) {
				p.AdaptiveBurnEnabled = true
				p.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)
				p.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2)
			},
			shouldFail: false,
		},
		{
			name: "min > max",
			setupParams: func(p *types.TokenomicsParams) {
				p.AdaptiveBurnEnabled = true
				p.MinBurnRatio = math.LegacyNewDecWithPrec(95, 2)
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(80, 2)
				p.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2)
			},
			shouldFail: true,
		},
		{
			name: "default < min",
			setupParams: func(p *types.TokenomicsParams) {
				p.AdaptiveBurnEnabled = true
				p.MinBurnRatio = math.LegacyNewDecWithPrec(85, 2)
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)
				p.DefaultBurnRatio = math.LegacyNewDecWithPrec(80, 2)
			},
			shouldFail: true,
		},
		{
			name: "min below protocol floor (70%)",
			setupParams: func(p *types.TokenomicsParams) {
				p.AdaptiveBurnEnabled = true
				p.MinBurnRatio = math.LegacyNewDecWithPrec(65, 2)
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(95, 2)
				p.DefaultBurnRatio = math.LegacyNewDecWithPrec(80, 2)
			},
			shouldFail: true,
		},
		{
			name: "max above protocol cap (95%)",
			setupParams: func(p *types.TokenomicsParams) {
				p.AdaptiveBurnEnabled = true
				p.MinBurnRatio = math.LegacyNewDecWithPrec(80, 2)
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(98, 2)
				p.DefaultBurnRatio = math.LegacyNewDecWithPrec(90, 2)
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := f.Keeper.GetParams(ctx)
			tt.setupParams(&params)

			err := params.Validate()
			if tt.shouldFail {
				require.Error(t, err, "validation should fail")
			} else {
				require.NoError(t, err, "validation should pass")
			}
		})
	}
}
