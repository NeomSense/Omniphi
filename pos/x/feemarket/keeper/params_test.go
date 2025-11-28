package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/feemarket/types"
)

func TestGetSetParams(t *testing.T) {
	f := setupTest(t)

	// Get default params
	params := f.keeper.GetParams(f.ctx)
	require.NotNil(t, params)

	// Verify default values
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.050000000000000000"), params.MinGasPrice)
	require.True(t, params.BaseFeeEnabled)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.050000000000000000"), params.BaseFeeInitial)
	require.Equal(t, uint64(10000000), params.MaxTxGas)

	// Update params
	newParams := params
	newParams.MinGasPrice = math.LegacyMustNewDecFromStr("0.075")
	newParams.BurnHot = math.LegacyMustNewDecFromStr("0.50")

	err := f.keeper.SetParams(f.ctx, newParams)
	require.NoError(t, err)

	// Verify updated params
	params = f.keeper.GetParams(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.075"), params.MinGasPrice)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.50"), params.BurnHot)
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyParam func(*types.FeeMarketParams)
		expectError bool
	}{
		{
			name: "valid default params",
			modifyParam: func(p *types.FeeMarketParams) {
				// Use defaults
			},
			expectError: false,
		},
		{
			name: "negative min_gas_price",
			modifyParam: func(p *types.FeeMarketParams) {
				p.MinGasPrice = math.LegacyMustNewDecFromStr("-0.05")
			},
			expectError: true,
		},
		{
			name: "target_utilization > 1",
			modifyParam: func(p *types.FeeMarketParams) {
				p.TargetBlockUtilization = math.LegacyMustNewDecFromStr("1.5")
			},
			expectError: true,
		},
		{
			name: "burn_hot > max_burn_ratio",
			modifyParam: func(p *types.FeeMarketParams) {
				p.BurnHot = math.LegacyMustNewDecFromStr("0.60")
				p.MaxBurnRatio = math.LegacyMustNewDecFromStr("0.50")
			},
			expectError: true,
		},
		{
			name: "validator_fee_ratio + treasury_fee_ratio != 1",
			modifyParam: func(p *types.FeeMarketParams) {
				p.ValidatorFeeRatio = math.LegacyMustNewDecFromStr("0.60")
				p.TreasuryFeeRatio = math.LegacyMustNewDecFromStr("0.30")
			},
			expectError: true,
		},
		{
			name: "max_tx_gas = 0",
			modifyParam: func(p *types.FeeMarketParams) {
				p.MaxTxGas = 0
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.modifyParam(&params)

			err := params.Validate()

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCumulativeStats(t *testing.T) {
	f := setupTest(t)

	// Initial stats should be zero
	burned := f.keeper.GetCumulativeBurned(f.ctx)
	require.True(t, burned.IsZero())

	toValidators := f.keeper.GetCumulativeToValidators(f.ctx)
	require.True(t, toValidators.IsZero())

	toTreasury := f.keeper.GetCumulativeToTreasury(f.ctx)
	require.True(t, toTreasury.IsZero())

	// Increment burned
	err := f.keeper.IncrementCumulativeBurned(f.ctx, math.NewInt(1000000))
	require.NoError(t, err)

	burned = f.keeper.GetCumulativeBurned(f.ctx)
	require.Equal(t, "1000000", burned.String())

	// Increment to validators
	err = f.keeper.IncrementCumulativeToValidators(f.ctx, math.NewInt(7000000))
	require.NoError(t, err)

	toValidators = f.keeper.GetCumulativeToValidators(f.ctx)
	require.Equal(t, "7000000", toValidators.String())

	// Increment to treasury
	err = f.keeper.IncrementCumulativeToTreasury(f.ctx, math.NewInt(3000000))
	require.NoError(t, err)

	toTreasury = f.keeper.GetCumulativeToTreasury(f.ctx)
	require.Equal(t, "3000000", toTreasury.String())

	// Increment again (cumulative)
	err = f.keeper.IncrementCumulativeBurned(f.ctx, math.NewInt(500000))
	require.NoError(t, err)

	burned = f.keeper.GetCumulativeBurned(f.ctx)
	require.Equal(t, "1500000", burned.String())
}

func TestTreasuryAddress(t *testing.T) {
	f := setupTest(t)

	// Set treasury address
	treasuryAddr := []byte("treasury_address_123")
	err := f.keeper.SetTreasuryAddress(f.ctx, treasuryAddr)
	require.NoError(t, err)

	// Get treasury address
	addr := f.keeper.GetTreasuryAddress(f.ctx)
	require.Equal(t, treasuryAddr, addr)
}

func TestBaseFeeGetSet(t *testing.T) {
	f := setupTest(t)

	// Get initial base fee
	baseFee := f.keeper.GetCurrentBaseFee(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.050000000000000000"), baseFee)

	// Set new base fee
	newBaseFee := math.LegacyMustNewDecFromStr("0.075")
	err := f.keeper.SetCurrentBaseFee(f.ctx, newBaseFee)
	require.NoError(t, err)

	// Verify it was set
	baseFee = f.keeper.GetCurrentBaseFee(f.ctx)
	requireDecEqual(t, newBaseFee, baseFee)
}

func TestPreviousUtilizationGetSet(t *testing.T) {
	f := setupTest(t)

	// Get initial previous utilization
	prevUtil := f.keeper.GetPreviousBlockUtilization(f.ctx)
	require.True(t, prevUtil.IsZero())

	// Set previous utilization
	newUtil := math.LegacyMustNewDecFromStr("0.35")
	err := f.keeper.SetPreviousBlockUtilization(f.ctx, newUtil)
	require.NoError(t, err)

	// Verify it was set
	prevUtil = f.keeper.GetPreviousBlockUtilization(f.ctx)
	requireDecEqual(t, newUtil, prevUtil)
}
