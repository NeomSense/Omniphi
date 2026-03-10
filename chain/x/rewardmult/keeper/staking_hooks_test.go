package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/rewardmult/keeper"
	"pos/x/rewardmult/types"
)

// ============================================================================
// StakingHooks: BeforeValidatorSlashed
// ============================================================================

func TestStakingHooks_BeforeValidatorSlashed_Downtime(t *testing.T) {
	f := SetupKeeperTest(t)
	k := f.keeper
	hooks := keeper.NewStakingHooks(&k)

	// Downtime slash fraction: 0.0001 (0.01%) — well below 1% threshold
	fraction := math.LegacyNewDecWithPrec(1, 4) // 0.0001
	// Use raw bytes so valAddr.String() in the hook matches what we query
	rawVal := sdk.ValAddress("val1________________")
	valAddrStr := rawVal.String()

	err := hooks.BeforeValidatorSlashed(f.ctx, rawVal, fraction)
	require.NoError(t, err)

	// Verify it was recorded as downtime
	// epoch = block_height / 100 = 100 / 100 = 1
	decay := k.SlashDecayFractionByType(f.ctx, valAddrStr, 1, 30, types.InfractionDowntime)
	require.True(t, decay.Equal(math.LegacyOneDec()), "expected full decay for just-recorded downtime, got %s", decay)

	// Double-sign decay should be zero (no double-sign events)
	decay = k.SlashDecayFractionByType(f.ctx, valAddrStr, 1, 60, types.InfractionDoubleSign)
	require.True(t, decay.IsZero(), "expected zero double-sign decay, got %s", decay)
}

func TestStakingHooks_BeforeValidatorSlashed_DoubleSign(t *testing.T) {
	f := SetupKeeperTest(t)
	k := f.keeper
	hooks := keeper.NewStakingHooks(&k)

	// Double-sign slash fraction: 0.05 (5%) — above 1% threshold
	fraction := math.LegacyNewDecWithPrec(5, 2) // 0.05
	rawVal := sdk.ValAddress("val1________________")
	valAddrStr := rawVal.String()

	err := hooks.BeforeValidatorSlashed(f.ctx, rawVal, fraction)
	require.NoError(t, err)

	// Verify it was recorded as double-sign
	decay := k.SlashDecayFractionByType(f.ctx, valAddrStr, 1, 60, types.InfractionDoubleSign)
	require.True(t, decay.Equal(math.LegacyOneDec()), "expected full decay for just-recorded double-sign, got %s", decay)
}

func TestStakingHooks_BeforeValidatorSlashed_BoundaryFraction(t *testing.T) {
	f := SetupKeeperTest(t)
	k := f.keeper
	hooks := keeper.NewStakingHooks(&k)

	// Exactly 1% — should classify as double-sign
	fraction := math.LegacyNewDecWithPrec(1, 2) // 0.01
	rawVal := sdk.ValAddress("val1________________")
	valAddrStr := rawVal.String()

	err := hooks.BeforeValidatorSlashed(f.ctx, rawVal, fraction)
	require.NoError(t, err)

	// Should be classified as double-sign (>= 1%)
	decay := k.SlashDecayFractionByType(f.ctx, valAddrStr, 1, 60, types.InfractionDoubleSign)
	require.True(t, decay.Equal(math.LegacyOneDec()), "fraction 0.01 should classify as double-sign")
}

func TestStakingHooks_NoOp_OtherMethods(t *testing.T) {
	f := SetupKeeperTest(t)
	k := f.keeper
	hooks := keeper.NewStakingHooks(&k)

	valAddr := valAddrFromOperator(testVal1)
	consAddr := consAddrFromBytes([]byte("cons________________"))
	accAddr := accAddrFromBytes([]byte("acc_________________"))

	// All other methods should return nil without side effects
	require.NoError(t, hooks.AfterValidatorCreated(f.ctx, valAddr))
	require.NoError(t, hooks.BeforeValidatorModified(f.ctx, valAddr))
	require.NoError(t, hooks.AfterValidatorRemoved(f.ctx, consAddr, valAddr))
	require.NoError(t, hooks.AfterValidatorBonded(f.ctx, consAddr, valAddr))
	require.NoError(t, hooks.AfterValidatorBeginUnbonding(f.ctx, consAddr, valAddr))
	require.NoError(t, hooks.BeforeDelegationCreated(f.ctx, accAddr, valAddr))
	require.NoError(t, hooks.BeforeDelegationSharesModified(f.ctx, accAddr, valAddr))
	require.NoError(t, hooks.BeforeDelegationRemoved(f.ctx, accAddr, valAddr))
	require.NoError(t, hooks.AfterDelegationModified(f.ctx, accAddr, valAddr))
	require.NoError(t, hooks.AfterUnbondingInitiated(f.ctx, 1))

	// Verify no slash events were created
	hasSlash := k.HasSlashInLookback(f.ctx, testVal1, 100, 100)
	require.False(t, hasSlash)
}

// ============================================================================
// SlashDecayFractionByType
// ============================================================================

func TestSlashDecayFractionByType_DowntimeOnly(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record a downtime event at epoch 10
	err := f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 10, types.InfractionDowntime, math.LegacyNewDecWithPrec(1, 4))
	require.NoError(t, err)

	// Downtime decay should be present
	decay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 10, 30, types.InfractionDowntime)
	require.True(t, decay.Equal(math.LegacyOneDec()))

	// Double-sign decay should be zero
	decay = f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 10, 60, types.InfractionDoubleSign)
	require.True(t, decay.IsZero())
}

func TestSlashDecayFractionByType_DoubleSignOnly(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record a double-sign event at epoch 10
	err := f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 10, types.InfractionDoubleSign, math.LegacyNewDecWithPrec(5, 2))
	require.NoError(t, err)

	// Double-sign decay should be present
	decay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 10, 60, types.InfractionDoubleSign)
	require.True(t, decay.Equal(math.LegacyOneDec()))

	// Downtime decay should be zero
	decay = f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 10, 30, types.InfractionDowntime)
	require.True(t, decay.IsZero())
}

func TestSlashDecayFractionByType_LinearDecay(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record downtime at epoch 70
	err := f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 70, types.InfractionDowntime, math.LegacyNewDecWithPrec(1, 4))
	require.NoError(t, err)

	// Half-way through lookback (epoch 85, 15 epochs since slash, lookback=30)
	decay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 85, 30, types.InfractionDowntime)
	require.True(t, decay.Equal(math.LegacyNewDecWithPrec(50, 2)), "expected 0.50, got %s", decay) // 0.5

	// Beyond lookback window
	decay = f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 110, 30, types.InfractionDowntime)
	require.True(t, decay.IsZero())
}

// ============================================================================
// computeSlashPenalty: Infraction-Aware Dual Decay
// ============================================================================

func TestComputeSlashPenalty_DoubleSignDominates(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record both downtime and double-sign at the same epoch
	err := f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 1, types.InfractionDowntime, math.LegacyNewDecWithPrec(1, 4))
	require.NoError(t, err)
	err = f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 1, types.InfractionDoubleSign, math.LegacyNewDecWithPrec(5, 2))
	require.NoError(t, err)

	// Process epoch — double-sign penalty (0.10) should dominate over downtime (0.05)
	err = f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	vm, found := f.keeper.GetValidatorMultiplier(f.ctx, testVal1)
	require.True(t, found)

	// SlashPenalty field should contain the double-sign penalty (0.10 * 1.0 decay = 0.10)
	// which is greater than downtime penalty (0.05 * 1.0 decay = 0.05)
	require.True(t, vm.SlashPenalty.Equal(math.LegacyNewDecWithPrec(10, 2)),
		"expected double-sign penalty 0.10, got %s", vm.SlashPenalty)
}

func TestComputeSlashPenalty_DowntimeDecaysFirst(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record downtime at epoch 1 (lookback = 30 epochs)
	err := f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 1, types.InfractionDowntime, math.LegacyNewDecWithPrec(1, 4))
	require.NoError(t, err)

	// At epoch 31, downtime should have fully decayed (30 epochs elapsed, lookback=30)
	decay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 31, 30, types.InfractionDowntime)
	require.True(t, decay.IsZero(), "downtime should have fully decayed by epoch 31, got %s", decay)

	// Now record double-sign at epoch 1 (lookback = 60 epochs)
	err = f.keeper.RecordSlashEventWithType(f.ctx, testVal1, 1, types.InfractionDoubleSign, math.LegacyNewDecWithPrec(5, 2))
	require.NoError(t, err)

	// At epoch 31, double-sign should still have partial decay (30/60 = 0.5)
	decay = f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 31, 60, types.InfractionDoubleSign)
	expected := math.LegacyNewDec(30).Quo(math.LegacyNewDec(60)) // 0.5
	require.True(t, decay.Equal(expected), "expected double-sign decay 0.50, got %s", decay)
}

func TestComputeSlashPenalty_BackwardCompat(t *testing.T) {
	f := SetupKeeperTest(t)

	// Use the old RecordSlashEvent (no infraction type — records as "unknown")
	err := f.keeper.RecordSlashEvent(f.ctx, testVal1, 1)
	require.NoError(t, err)

	// "unknown" events should appear in both downtime and double-sign queries
	downtimeDecay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 1, 30, types.InfractionDowntime)
	require.True(t, downtimeDecay.Equal(math.LegacyOneDec()), "unknown events should match downtime query, got %s", downtimeDecay)

	doubleSignDecay := f.keeper.SlashDecayFractionByType(f.ctx, testVal1, 1, 60, types.InfractionDoubleSign)
	require.True(t, doubleSignDecay.Equal(math.LegacyOneDec()), "unknown events should match double-sign query, got %s", doubleSignDecay)

	// Original SlashDecayFraction should also still work
	decay := f.keeper.SlashDecayFraction(f.ctx, testVal1, 1, 30)
	require.True(t, decay.Equal(math.LegacyOneDec()), "legacy SlashDecayFraction should still work, got %s", decay)
}

// ============================================================================
// DoubleSign Params Validation
// ============================================================================

func TestDoubleSignParams_Validation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*types.Params)
		wantErr bool
	}{
		{
			name:    "valid default params with double-sign",
			modify:  func(p *types.Params) {},
			wantErr: false,
		},
		{
			name: "negative double-sign penalty",
			modify: func(p *types.Params) {
				p.DoubleSignPenalty = math.LegacyNewDec(-1)
			},
			wantErr: true,
		},
		{
			name: "double-sign penalty exceeds 0.30",
			modify: func(p *types.Params) {
				p.DoubleSignPenalty = math.LegacyNewDecWithPrec(31, 2) // 0.31
			},
			wantErr: true,
		},
		{
			name: "double-sign penalty at max boundary 0.30",
			modify: func(p *types.Params) {
				p.DoubleSignPenalty = math.LegacyNewDecWithPrec(30, 2) // 0.30
			},
			wantErr: false,
		},
		{
			name: "double-sign lookback too low",
			modify: func(p *types.Params) {
				p.DoubleSignLookbackEpochs = 0
			},
			wantErr: true,
		},
		{
			name: "double-sign lookback too high",
			modify: func(p *types.Params) {
				p.DoubleSignLookbackEpochs = 366
			},
			wantErr: true,
		},
		{
			name: "double-sign lookback at max boundary 365",
			modify: func(p *types.Params) {
				p.DoubleSignLookbackEpochs = 365
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := types.DefaultParams()
			tt.modify(&params)
			err := params.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParamsSidecar_DoubleSign_Roundtrip(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set params with custom double-sign values
	params := types.DefaultParams()
	params.DoubleSignPenalty = math.LegacyNewDecWithPrec(15, 2)   // 0.15
	params.DoubleSignLookbackEpochs = 90

	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Read back and verify
	got := f.keeper.GetParams(f.ctx)
	require.True(t, got.DoubleSignPenalty.Equal(math.LegacyNewDecWithPrec(15, 2)),
		"expected DoubleSignPenalty 0.15, got %s", got.DoubleSignPenalty)
	require.Equal(t, int64(90), got.DoubleSignLookbackEpochs)

	// Also verify existing params survived the roundtrip
	require.True(t, got.SlashPenalty.Equal(types.DefaultSlashPenalty))
	require.Equal(t, types.DefaultSlashLookbackEpochs, got.SlashLookbackEpochs)
}

// ============================================================================
// Helpers
// ============================================================================

func valAddrFromOperator(operatorAddr string) sdk.ValAddress {
	return sdk.ValAddress(operatorAddr)
}

func consAddrFromBytes(bz []byte) sdk.ConsAddress {
	return sdk.ConsAddress(bz)
}

func accAddrFromBytes(bz []byte) sdk.AccAddress {
	return sdk.AccAddress(bz)
}
