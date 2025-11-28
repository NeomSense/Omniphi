package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"pos/x/poc/types"
)

// Note: These tests use mock bank keeper which always succeeds.
// Integration tests with real bank keeper would be needed for full fee collection/burn verification.
// These tests verify the logic flow and state management.

// TestFeeMetrics_StateManagement tests fee metrics state storage and retrieval
func TestFeeMetrics_StateManagement(t *testing.T) {
	f := SetupKeeperTest(t)

	// Initially should return empty metrics
	metrics := f.keeper.GetFeeMetrics(f.ctx)
	require.True(t, metrics.TotalFeesCollected.IsZero())
	require.True(t, metrics.TotalBurned.IsZero())
	require.True(t, metrics.TotalRewardRedirect.IsZero())
	require.Equal(t, int64(0), metrics.LastUpdatedHeight)

	// Update metrics
	feesCollected := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1000)))
	burned := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(750)))
	rewards := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(250)))

	err := f.keeper.UpdateFeeMetrics(f.ctx, feesCollected, burned, rewards)
	require.NoError(t, err)

	// Verify metrics updated
	updatedMetrics := f.keeper.GetFeeMetrics(f.ctx)
	require.Equal(t, feesCollected, updatedMetrics.TotalFeesCollected)
	require.Equal(t, burned, updatedMetrics.TotalBurned)
	require.Equal(t, rewards, updatedMetrics.TotalRewardRedirect)

	// Update again (should accumulate)
	err = f.keeper.UpdateFeeMetrics(f.ctx, feesCollected, burned, rewards)
	require.NoError(t, err)

	finalMetrics := f.keeper.GetFeeMetrics(f.ctx)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(2000))), finalMetrics.TotalFeesCollected)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1500))), finalMetrics.TotalBurned)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(500))), finalMetrics.TotalRewardRedirect)
}

// TestContributorFeeStats_StateManagement tests contributor stats storage and retrieval
func TestContributorFeeStats_StateManagement(t *testing.T) {
	f := SetupKeeperTest(t)

	contributorAddr := sdk.AccAddress("contributor_______")

	// Initially should return empty stats
	stats := f.keeper.GetContributorFeeStats(f.ctx, contributorAddr)
	require.Equal(t, contributorAddr.String(), stats.Address)
	require.True(t, stats.TotalFeesPaid.IsZero())
	require.True(t, stats.TotalBurned.IsZero())
	require.Equal(t, uint64(0), stats.SubmissionCount)

	// Update stats
	feesPaid := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1000)))
	burned := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(750)))

	err := f.keeper.UpdateContributorFeeStats(f.ctx, contributorAddr, feesPaid, burned)
	require.NoError(t, err)

	// Verify stats updated
	updatedStats := f.keeper.GetContributorFeeStats(f.ctx, contributorAddr)
	require.Equal(t, feesPaid, updatedStats.TotalFeesPaid)
	require.Equal(t, burned, updatedStats.TotalBurned)
	require.Equal(t, uint64(1), updatedStats.SubmissionCount)
	// Note: BlockHeight is 0 in mock context, which is fine for testing state management
	require.GreaterOrEqual(t, updatedStats.FirstSubmissionHeight, int64(0))
	require.Equal(t, updatedStats.FirstSubmissionHeight, updatedStats.LastSubmissionHeight)

	// Update again
	err = f.keeper.UpdateContributorFeeStats(f.ctx, contributorAddr, feesPaid, burned)
	require.NoError(t, err)

	finalStats := f.keeper.GetContributorFeeStats(f.ctx, contributorAddr)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(2000))), finalStats.TotalFeesPaid)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1500))), finalStats.TotalBurned)
	require.Equal(t, uint64(2), finalStats.SubmissionCount)
}

// TestGetAllContributorFeeStats tests iteration over all contributor stats
func TestGetAllContributorFeeStats(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create stats for 3 contributors
	for i := 0; i < 3; i++ {
		addr := sdk.AccAddress([]byte("contributor_" + string(rune('a'+i)) + "____"))
		feesPaid := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1000)))
		burned := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(750)))

		err := f.keeper.UpdateContributorFeeStats(f.ctx, addr, feesPaid, burned)
		require.NoError(t, err)
	}

	// Get all stats
	allStats := f.keeper.GetAllContributorFeeStats(f.ctx)
	require.Len(t, allStats, 3)
}

// TestParamValidation_FeeBounds tests parameter validation
func TestParamValidation_FeeBounds(t *testing.T) {
	testCases := []struct {
		name        string
		modifyFunc  func(*types.Params)
		expectError bool
	}{
		{
			name: "fee within bounds - valid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionFee = sdk.NewCoin("uomni", math.NewInt(5000))
				p.MinSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100))
				p.MaxSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100000))
			},
			expectError: false,
		},
		{
			name: "fee below minimum - invalid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionFee = sdk.NewCoin("uomni", math.NewInt(50))
				p.MinSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100))
				p.MaxSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100000))
			},
			expectError: true,
		},
		{
			name: "fee above maximum - invalid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionFee = sdk.NewCoin("uomni", math.NewInt(200000))
				p.MinSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100))
				p.MaxSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100000))
			},
			expectError: true,
		},
		{
			name: "burn ratio below minimum - invalid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionBurnRatio = math.LegacyNewDecWithPrec(40, 2) // 40%
				p.MinBurnRatio = math.LegacyNewDecWithPrec(50, 2)         // 50%
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(90, 2)         // 90%
			},
			expectError: true,
		},
		{
			name: "burn ratio above maximum - invalid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionBurnRatio = math.LegacyNewDecWithPrec(95, 2) // 95%
				p.MinBurnRatio = math.LegacyNewDecWithPrec(50, 2)         // 50%
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(90, 2)         // 90%
			},
			expectError: true,
		},
		{
			name: "burn ratio at minimum - valid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionBurnRatio = math.LegacyNewDecWithPrec(50, 2) // 50%
				p.MinBurnRatio = math.LegacyNewDecWithPrec(50, 2)         // 50%
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(90, 2)         // 90%
			},
			expectError: false,
		},
		{
			name: "burn ratio at maximum - valid",
			modifyFunc: func(p *types.Params) {
				p.SubmissionBurnRatio = math.LegacyNewDecWithPrec(90, 2) // 90%
				p.MinBurnRatio = math.LegacyNewDecWithPrec(50, 2)         // 50%
				p.MaxBurnRatio = math.LegacyNewDecWithPrec(90, 2)         // 90%
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.modifyFunc(&params)
			err := params.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestGenesisImportExport tests fee metrics in genesis
func TestGenesisImportExport(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create some fee metrics
	contributorAddr := sdk.AccAddress("contributor_______")
	feesPaid := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(1000)))
	burned := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(750)))
	rewards := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(250)))

	require.NoError(t, f.keeper.UpdateFeeMetrics(f.ctx, feesPaid, burned, rewards))
	require.NoError(t, f.keeper.UpdateContributorFeeStats(f.ctx, contributorAddr, feesPaid, burned))

	// Export genesis
	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.NotNil(t, exported.FeeMetrics)
	require.Len(t, exported.ContributorFeeStats, 1)
	require.Equal(t, feesPaid, exported.FeeMetrics.TotalFeesCollected)

	// Debug: Print exported params to see what's wrong
	t.Logf("Exported SubmissionFee: %+v", exported.Params.SubmissionFee)
	t.Logf("Exported SubmissionFee.Denom: %s", exported.Params.SubmissionFee.Denom)
	t.Logf("Exported SubmissionFee.Amount: %s", exported.Params.SubmissionFee.Amount)
	t.Logf("Exported SubmissionFee.IsNil(): %v", exported.Params.SubmissionFee.IsNil())
	t.Logf("Exported SubmissionFee.IsValid(): %v", exported.Params.SubmissionFee.IsValid())

	// Validate exported params have fee fields
	require.False(t, exported.Params.SubmissionFee.IsNil(), "SubmissionFee should not be nil")
	require.True(t, exported.Params.SubmissionFee.IsValid(), "SubmissionFee should be valid")
	require.Equal(t, "uomni", exported.Params.SubmissionFee.Denom)

	// Create new keeper and import
	f2 := SetupKeeperTest(t)
	err := f2.keeper.InitGenesis(f2.ctx, *exported)
	require.NoError(t, err)

	// Verify imported correctly
	importedMetrics := f2.keeper.GetFeeMetrics(f2.ctx)
	require.Equal(t, exported.FeeMetrics.TotalFeesCollected, importedMetrics.TotalFeesCollected)
	require.Equal(t, exported.FeeMetrics.TotalBurned, importedMetrics.TotalBurned)

	importedStats := f2.keeper.GetContributorFeeStats(f2.ctx, contributorAddr)
	require.Equal(t, feesPaid, importedStats.TotalFeesPaid)
	require.Equal(t, burned, importedStats.TotalBurned)
}

// TestDefaultParams verifies default fee parameters are set correctly
func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()

	// Verify fee defaults (updated from Adaptive Fee Market v2)
	require.Equal(t, "uomni", params.SubmissionFee.Denom)
	require.Equal(t, math.NewInt(2000), params.SubmissionFee.Amount) // Updated from 1000 to 2000
	require.Equal(t, math.LegacyNewDecWithPrec(50, 2), params.SubmissionBurnRatio) // Updated from 75% to 50%
	require.Equal(t, math.NewInt(100), params.MinSubmissionFee.Amount)
	require.Equal(t, math.NewInt(100000), params.MaxSubmissionFee.Amount)
	require.Equal(t, math.LegacyNewDecWithPrec(50, 2), params.MinBurnRatio)
	require.Equal(t, math.LegacyNewDecWithPrec(90, 2), params.MaxBurnRatio)

	// Verify params validate
	require.NoError(t, params.Validate())
}
