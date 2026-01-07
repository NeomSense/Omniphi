package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/feemarket/types"
)

func TestInitGenesis(t *testing.T) {
	f := setupTest(t)

	// Create custom genesis state with valid anchor lane parameters
	genesisState := types.GenesisState{
		Params: types.FeeMarketParams{
			MinGasPrice:            math.LegacyMustNewDecFromStr("0.100"),
			BaseFeeEnabled:         true,
			BaseFeeInitial:         math.LegacyMustNewDecFromStr("0.075"),
			ElasticityMultiplier:   math.LegacyMustNewDecFromStr("1.40"), // Max 1.50
			MaxTipRatio:            math.LegacyMustNewDecFromStr("0.25"),
			TargetBlockUtilization: math.LegacyMustNewDecFromStr("0.40"),
			MaxTxGas:               5000000, // Anchor lane max tx gas
			FreeTxQuota:            200,
			BurnCool:               math.LegacyMustNewDecFromStr("0.15"),
			BurnNormal:             math.LegacyMustNewDecFromStr("0.25"),
			BurnHot:                math.LegacyMustNewDecFromStr("0.45"), // Must be <= MaxBurnRatio
			UtilCoolThreshold:      math.LegacyMustNewDecFromStr("0.20"),
			UtilHotThreshold:       math.LegacyMustNewDecFromStr("0.40"),
			ValidatorFeeRatio:      math.LegacyMustNewDecFromStr("0.80"),
			TreasuryFeeRatio:       math.LegacyMustNewDecFromStr("0.20"),
			MaxBurnRatio:           math.LegacyMustNewDecFromStr("0.50"), // Max allowed is 0.50
			MinGasPriceFloor:       math.LegacyMustNewDecFromStr("0.050"),
			// Include activity multipliers from defaults
			MultiplierMessaging:      math.LegacyMustNewDecFromStr("0.50"),
			MultiplierPosGas:         math.LegacyMustNewDecFromStr("1.00"),
			MultiplierPocAnchoring:   math.LegacyMustNewDecFromStr("0.75"),
			MultiplierSmartContracts: math.LegacyMustNewDecFromStr("1.50"),
			MultiplierAiQueries:      math.LegacyMustNewDecFromStr("1.25"),
			MultiplierSequencer:      math.LegacyMustNewDecFromStr("1.25"),
			MinMultiplier:            math.LegacyMustNewDecFromStr("0.25"),
			MaxMultiplier:            math.LegacyMustNewDecFromStr("2.00"),
		},
		CurrentBaseFee:           math.LegacyMustNewDecFromStr("0.080"),
		PreviousBlockUtilization: math.LegacyMustNewDecFromStr("0.30"),
		CumulativeBurned:         math.NewInt(1000000000),
		CumulativeToValidators:   math.NewInt(4000000000),
		CumulativeToTreasury:     math.NewInt(1000000000),
	}

	// Initialize genesis
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	// Verify params were set
	params := f.keeper.GetParams(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.100"), params.MinGasPrice)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.075"), params.BaseFeeInitial)
	require.Equal(t, int64(5000000), params.MaxTxGas)

	// Verify base fee was set
	baseFee := f.keeper.GetCurrentBaseFee(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.080"), baseFee)

	// Verify previous utilization was set
	prevUtil := f.keeper.GetPreviousBlockUtilization(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.30"), prevUtil)

	// Verify cumulative stats
	burned := f.keeper.GetCumulativeBurned(f.ctx)
	require.Equal(t, "1000000000", burned.String())

	toValidators := f.keeper.GetCumulativeToValidators(f.ctx)
	require.Equal(t, "4000000000", toValidators.String())

	toTreasury := f.keeper.GetCumulativeToTreasury(f.ctx)
	require.Equal(t, "1000000000", toTreasury.String())
}

func TestExportGenesis(t *testing.T) {
	f := setupTest(t)

	// Set up state
	params := types.DefaultParams()
	params.MinGasPrice = math.LegacyMustNewDecFromStr("0.075")
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	baseFee := math.LegacyMustNewDecFromStr("0.065")
	err = f.keeper.SetCurrentBaseFee(f.ctx, baseFee)
	require.NoError(t, err)

	prevUtil := math.LegacyMustNewDecFromStr("0.40")
	err = f.keeper.SetPreviousBlockUtilization(f.ctx, prevUtil)
	require.NoError(t, err)

	err = f.keeper.IncrementCumulativeBurned(f.ctx, math.NewInt(500000000))
	require.NoError(t, err)

	err = f.keeper.IncrementCumulativeToValidators(f.ctx, math.NewInt(2000000000))
	require.NoError(t, err)

	err = f.keeper.IncrementCumulativeToTreasury(f.ctx, math.NewInt(500000000))
	require.NoError(t, err)

	// Export genesis
	genesisState := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, genesisState)

	// Verify exported params
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.075"), genesisState.Params.MinGasPrice)

	// Verify exported base fee
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.065"), genesisState.CurrentBaseFee)

	// Verify exported utilization
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.40"), genesisState.PreviousBlockUtilization)

	// Verify exported stats
	require.Equal(t, "500000000", genesisState.CumulativeBurned.String())
	require.Equal(t, "2000000000", genesisState.CumulativeToValidators.String())
	require.Equal(t, "500000000", genesisState.CumulativeToTreasury.String())
}

func TestGenesisRoundTrip(t *testing.T) {
	f := setupTest(t)

	// Create initial genesis state with valid anchor lane parameters
	initialGenesis := types.GenesisState{
		Params: types.FeeMarketParams{
			MinGasPrice:            math.LegacyMustNewDecFromStr("0.080"),
			BaseFeeEnabled:         true,
			BaseFeeInitial:         math.LegacyMustNewDecFromStr("0.060"),
			ElasticityMultiplier:   math.LegacyMustNewDecFromStr("1.25"),
			MaxTipRatio:            math.LegacyMustNewDecFromStr("0.30"),
			TargetBlockUtilization: math.LegacyMustNewDecFromStr("0.35"),
			MaxTxGas:               5000000, // Anchor lane max
			FreeTxQuota:            150,
			BurnCool:               math.LegacyMustNewDecFromStr("0.12"),
			BurnNormal:             math.LegacyMustNewDecFromStr("0.22"),
			BurnHot:                math.LegacyMustNewDecFromStr("0.45"), // <= MaxBurnRatio
			UtilCoolThreshold:      math.LegacyMustNewDecFromStr("0.18"),
			UtilHotThreshold:       math.LegacyMustNewDecFromStr("0.35"),
			ValidatorFeeRatio:      math.LegacyMustNewDecFromStr("0.75"),
			TreasuryFeeRatio:       math.LegacyMustNewDecFromStr("0.25"),
			MaxBurnRatio:           math.LegacyMustNewDecFromStr("0.50"), // Max allowed
			MinGasPriceFloor:       math.LegacyMustNewDecFromStr("0.040"),
			// Include activity multipliers
			MultiplierMessaging:      math.LegacyMustNewDecFromStr("0.50"),
			MultiplierPosGas:         math.LegacyMustNewDecFromStr("1.00"),
			MultiplierPocAnchoring:   math.LegacyMustNewDecFromStr("0.75"),
			MultiplierSmartContracts: math.LegacyMustNewDecFromStr("1.50"),
			MultiplierAiQueries:      math.LegacyMustNewDecFromStr("1.25"),
			MultiplierSequencer:      math.LegacyMustNewDecFromStr("1.25"),
			MinMultiplier:            math.LegacyMustNewDecFromStr("0.25"),
			MaxMultiplier:            math.LegacyMustNewDecFromStr("2.00"),
		},
		CurrentBaseFee:           math.LegacyMustNewDecFromStr("0.070"),
		PreviousBlockUtilization: math.LegacyMustNewDecFromStr("0.38"),
		CumulativeBurned:         math.NewInt(2500000000),
		CumulativeToValidators:   math.NewInt(5000000000),
		CumulativeToTreasury:     math.NewInt(1500000000),
	}

	// Initialize genesis
	err := f.keeper.InitGenesis(f.ctx, initialGenesis)
	require.NoError(t, err)

	// Export genesis
	exportedGenesis := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exportedGenesis)

	// Compare params
	requireDecEqual(t, initialGenesis.Params.MinGasPrice, exportedGenesis.Params.MinGasPrice)
	require.Equal(t, initialGenesis.Params.BaseFeeEnabled, exportedGenesis.Params.BaseFeeEnabled)
	requireDecEqual(t, initialGenesis.Params.BaseFeeInitial, exportedGenesis.Params.BaseFeeInitial)
	requireDecEqual(t, initialGenesis.Params.ElasticityMultiplier, exportedGenesis.Params.ElasticityMultiplier)
	require.Equal(t, initialGenesis.Params.MaxTxGas, exportedGenesis.Params.MaxTxGas)
	require.Equal(t, initialGenesis.Params.FreeTxQuota, exportedGenesis.Params.FreeTxQuota)
	requireDecEqual(t, initialGenesis.Params.BurnCool, exportedGenesis.Params.BurnCool)
	requireDecEqual(t, initialGenesis.Params.BurnNormal, exportedGenesis.Params.BurnNormal)
	requireDecEqual(t, initialGenesis.Params.BurnHot, exportedGenesis.Params.BurnHot)
	requireDecEqual(t, initialGenesis.Params.ValidatorFeeRatio, exportedGenesis.Params.ValidatorFeeRatio)
	requireDecEqual(t, initialGenesis.Params.TreasuryFeeRatio, exportedGenesis.Params.TreasuryFeeRatio)

	// Compare state
	requireDecEqual(t, initialGenesis.CurrentBaseFee, exportedGenesis.CurrentBaseFee)
	requireDecEqual(t, initialGenesis.PreviousBlockUtilization, exportedGenesis.PreviousBlockUtilization)

	// Compare stats
	require.Equal(t, initialGenesis.CumulativeBurned.String(), exportedGenesis.CumulativeBurned.String())
	require.Equal(t, initialGenesis.CumulativeToValidators.String(), exportedGenesis.CumulativeToValidators.String())
	require.Equal(t, initialGenesis.CumulativeToTreasury.String(), exportedGenesis.CumulativeToTreasury.String())
}

func TestDefaultGenesis(t *testing.T) {
	f := setupTest(t)

	// Get default genesis
	defaultGenesis := types.DefaultGenesisState()
	require.NotNil(t, defaultGenesis)

	// Verify defaults (anchor lane: 0.025 gas price)
	require.True(t, defaultGenesis.Params.BaseFeeEnabled)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.025000000000000000"), defaultGenesis.Params.MinGasPrice)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.025000000000000000"), defaultGenesis.CurrentBaseFee)
	require.True(t, defaultGenesis.PreviousBlockUtilization.IsZero())
	require.True(t, defaultGenesis.CumulativeBurned.IsZero())

	// Initialize with defaults
	err := f.keeper.InitGenesis(f.ctx, *defaultGenesis)
	require.NoError(t, err)

	// Verify state was initialized
	params := f.keeper.GetParams(f.ctx)
	require.NotNil(t, params)

	baseFee := f.keeper.GetCurrentBaseFee(f.ctx)
	requireDecEqual(t, math.LegacyMustNewDecFromStr("0.025000000000000000"), baseFee)
}
