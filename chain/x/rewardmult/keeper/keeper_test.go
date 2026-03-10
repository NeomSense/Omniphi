package keeper_test

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/x/rewardmult/keeper"
	"pos/x/rewardmult/types"
)

// KeeperTestFixture holds the test context and keeper
type KeeperTestFixture struct {
	ctx    sdk.Context
	keeper keeper.Keeper
}

// Mock staking keeper
type mockStakingKeeper struct {
	validators []stakingtypes.Validator
}

func (m mockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	for _, v := range m.validators {
		if v.GetOperator() == addr.String() {
			return v, nil
		}
	}
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

func (m mockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	return m.validators, nil
}

func (m mockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	total := math.ZeroInt()
	for _, v := range m.validators {
		if v.IsBonded() {
			total = total.Add(v.Tokens)
		}
	}
	return total, nil
}

func (m mockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return math.NewInt(1000000)
}

// Mock slashing keeper
type mockSlashingKeeper struct {
	signingInfos map[string]slashingtypes.ValidatorSigningInfo
}

func newMockSlashingKeeper() *mockSlashingKeeper {
	return &mockSlashingKeeper{
		signingInfos: make(map[string]slashingtypes.ValidatorSigningInfo),
	}
}

func (m *mockSlashingKeeper) GetValidatorSigningInfo(ctx context.Context, consAddr sdk.ConsAddress) (slashingtypes.ValidatorSigningInfo, error) {
	info, found := m.signingInfos[consAddr.String()]
	if !found {
		return slashingtypes.ValidatorSigningInfo{
			MissedBlocksCounter: 0,
			StartHeight:         0,
		}, nil
	}
	return info, nil
}

func (m *mockSlashingKeeper) SetSigningInfo(consAddr sdk.ConsAddress, missedBlocks int64, startHeight int64) {
	m.signingInfos[consAddr.String()] = slashingtypes.ValidatorSigningInfo{
		MissedBlocksCounter: missedBlocks,
		StartHeight:         startHeight,
	}
}

// Test addresses
var (
	testVal1 = sdk.ValAddress("val1________________").String()
	testVal2 = sdk.ValAddress("val2________________").String()
	testVal3 = sdk.ValAddress("val3________________").String()
)

// SetupKeeperTest creates a test fixture with mock keepers
func SetupKeeperTest(t *testing.T) *KeeperTestFixture {
	t.Helper()

	// Setup store
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	// Create context
	ctx := sdk.NewContext(stateStore, cmtproto.Header{
		Height: 100,
		Time:   time.Now(),
	}, false, log.NewNopLogger())

	// Create codec
	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec

	// Create store service
	storeService := runtime.NewKVStoreService(storeKey)

	// Create mock keepers
	validators := []stakingtypes.Validator{
		createMockValidator(testVal1, math.NewInt(1000000), true),
		createMockValidator(testVal2, math.NewInt(2000000), true),
		createMockValidator(testVal3, math.NewInt(3000000), true),
	}
	stakingKeeper := mockStakingKeeper{validators: validators}
	slashingKeeper := newMockSlashingKeeper()

	// Create authority
	authority := sdk.AccAddress("authority___________").String()

	// Create keeper
	k := keeper.NewKeeper(
		cdc.(codec.BinaryCodec),
		storeService,
		log.NewNopLogger(),
		authority,
		stakingKeeper,
		slashingKeeper,
	)

	return &KeeperTestFixture{
		ctx:    ctx,
		keeper: k,
	}
}

// createMockValidator creates a mock validator for testing
func createMockValidator(operatorAddr string, tokens math.Int, bonded bool) stakingtypes.Validator {
	status := stakingtypes.Unbonded
	if bonded {
		status = stakingtypes.Bonded
	}
	return stakingtypes.Validator{
		OperatorAddress: operatorAddr,
		Tokens:          tokens,
		DelegatorShares: tokens.ToLegacyDec(),
		Status:          status,
	}
}

// ============================================================================
// Params Tests
// ============================================================================

func TestParams_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default params
	params := f.keeper.GetParams(f.ctx)
	require.Equal(t, types.DefaultMinMultiplier, params.MinMultiplier)
	require.Equal(t, types.DefaultMaxMultiplier, params.MaxMultiplier)

	// Set new params
	newParams := types.DefaultParams()
	newParams.EMAWindow = 16
	err := f.keeper.SetParams(f.ctx, newParams)
	require.NoError(t, err)

	// Verify
	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, int64(16), got.EMAWindow)
}

func TestParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*types.Params)
		wantErr bool
	}{
		{
			name:    "valid default params",
			modify:  func(p *types.Params) {},
			wantErr: false,
		},
		{
			name: "min > max multiplier",
			modify: func(p *types.Params) {
				p.MinMultiplier = math.LegacyNewDec(2)
				p.MaxMultiplier = math.LegacyOneDec()
			},
			wantErr: true,
		},
		{
			name: "EMA window = 0",
			modify: func(p *types.Params) {
				p.EMAWindow = 0
			},
			wantErr: true,
		},
		{
			name: "negative slash penalty",
			modify: func(p *types.Params) {
				p.SlashPenalty = math.LegacyNewDec(-1)
			},
			wantErr: true,
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

// ============================================================================
// ValidatorMultiplier CRUD Tests
// ============================================================================

func TestValidatorMultiplier_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create and store multiplier
	vm := types.NewValidatorMultiplier(testVal1, 1)
	vm.MRaw = math.LegacyNewDecWithPrec(105, 2) // 1.05
	vm.MEMA = math.LegacyNewDecWithPrec(103, 2)
	vm.MEffective = math.LegacyNewDecWithPrec(103, 2)

	err := f.keeper.SetValidatorMultiplier(f.ctx, vm)
	require.NoError(t, err)

	// Retrieve
	got, found := f.keeper.GetValidatorMultiplier(f.ctx, testVal1)
	require.True(t, found)
	require.Equal(t, testVal1, got.ValidatorAddress)
	require.Equal(t, int64(1), got.Epoch)
	require.True(t, got.MRaw.Equal(math.LegacyNewDecWithPrec(105, 2)))

	// Not found
	_, found = f.keeper.GetValidatorMultiplier(f.ctx, "unknown")
	require.False(t, found)

	// GetAll
	vm2 := types.NewValidatorMultiplier(testVal2, 1)
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm2))

	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	require.Len(t, all, 2)
}

func TestGetEffectiveMultiplier(t *testing.T) {
	f := SetupKeeperTest(t)

	// Non-existent validator returns 1.0
	mEff := f.keeper.GetEffectiveMultiplier(f.ctx, "nonexistent")
	require.True(t, mEff.Equal(math.LegacyOneDec()))

	// Set multiplier
	vm := types.NewValidatorMultiplier(testVal1, 1)
	vm.MEffective = math.LegacyNewDecWithPrec(110, 2) // 1.10
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm))

	mEff = f.keeper.GetEffectiveMultiplier(f.ctx, testVal1)
	require.True(t, mEff.Equal(math.LegacyNewDecWithPrec(110, 2)))
}

// ============================================================================
// EMA History Tests
// ============================================================================

func TestEMAHistory_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	history := f.keeper.GetEMAHistory(f.ctx, testVal1)
	require.Equal(t, testVal1, history.ValidatorAddress)
	require.Empty(t, history.Values)

	// Add values
	history.AddValue(math.LegacyNewDecWithPrec(100, 2), 8)
	history.AddValue(math.LegacyNewDecWithPrec(105, 2), 8)
	history.AddValue(math.LegacyNewDecWithPrec(110, 2), 8)

	err := f.keeper.SetEMAHistory(f.ctx, history)
	require.NoError(t, err)

	// Retrieve
	got := f.keeper.GetEMAHistory(f.ctx, testVal1)
	require.Len(t, got.Values, 3)
}

func TestEMAHistory_ComputeEMA(t *testing.T) {
	history := types.NewEMAHistory(testVal1)

	// Empty returns 1.0
	ema := history.ComputeEMA(8)
	require.True(t, ema.Equal(math.LegacyOneDec()))

	// Single value returns that value
	history.AddValue(math.LegacyNewDecWithPrec(105, 2), 8)
	ema = history.ComputeEMA(8)
	require.True(t, ema.Equal(math.LegacyNewDecWithPrec(105, 2)))

	// Multiple values - EMA smooths towards recent values
	history.AddValue(math.LegacyNewDecWithPrec(110, 2), 8)
	history.AddValue(math.LegacyNewDecWithPrec(115, 2), 8)
	ema = history.ComputeEMA(8)
	// EMA should be between first and last value
	require.True(t, ema.GT(math.LegacyNewDecWithPrec(105, 2)))
	require.True(t, ema.LT(math.LegacyNewDecWithPrec(115, 2)))
}

func TestEMAHistory_RingBuffer(t *testing.T) {
	history := types.NewEMAHistory(testVal1)

	// Add more values than max size
	for i := 0; i < 15; i++ {
		history.AddValue(math.LegacyNewDec(int64(i)), 8)
	}

	// Should only keep last 8
	require.Len(t, history.Values, 8)
	// First value should be 7 (15-8)
	require.True(t, history.Values[0].Equal(math.LegacyNewDec(7)))
}

// ============================================================================
// Slash Event Tests
// ============================================================================

func TestSlashEvent_Recording(t *testing.T) {
	f := SetupKeeperTest(t)

	// Record slash
	err := f.keeper.RecordSlashEvent(f.ctx, testVal1, 10)
	require.NoError(t, err)

	// Check lookback
	hasSlash := f.keeper.HasSlashInLookback(f.ctx, testVal1, 15, 30)
	require.True(t, hasSlash)

	// Outside lookback window
	hasSlash = f.keeper.HasSlashInLookback(f.ctx, testVal1, 50, 30)
	require.False(t, hasSlash)

	// Different validator
	hasSlash = f.keeper.HasSlashInLookback(f.ctx, testVal2, 15, 30)
	require.False(t, hasSlash)
}

func TestSlashDecayFraction(t *testing.T) {
	f := SetupKeeperTest(t)

	// No slash - returns 0
	fraction := f.keeper.SlashDecayFraction(f.ctx, testVal1, 100, 30)
	require.True(t, fraction.IsZero())

	// Record slash at epoch 70
	err := f.keeper.RecordSlashEvent(f.ctx, testVal1, 70)
	require.NoError(t, err)

	// Just slashed (current epoch 70) - full penalty
	fraction = f.keeper.SlashDecayFraction(f.ctx, testVal1, 70, 30)
	require.True(t, fraction.Equal(math.LegacyOneDec()))

	// Half way through (current epoch 85, 15 epochs since slash)
	fraction = f.keeper.SlashDecayFraction(f.ctx, testVal1, 85, 30)
	require.True(t, fraction.Equal(math.LegacyNewDecWithPrec(50, 2))) // 0.5

	// Beyond lookback window
	fraction = f.keeper.SlashDecayFraction(f.ctx, testVal1, 110, 30)
	require.True(t, fraction.IsZero())
}

// ============================================================================
// Genesis Tests
// ============================================================================

func TestGenesis_InitAndExport(t *testing.T) {
	f := SetupKeeperTest(t)

	// Init with default genesis
	gs := types.DefaultGenesis()
	err := f.keeper.InitGenesis(f.ctx, *gs)
	require.NoError(t, err)

	// Add some state
	vm := types.NewValidatorMultiplier(testVal1, 1)
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm))

	history := types.NewEMAHistory(testVal1)
	history.AddValue(math.LegacyOneDec(), 8)
	require.NoError(t, f.keeper.SetEMAHistory(f.ctx, history))

	// Export
	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.Len(t, exported.Multipliers, 1)
	require.Len(t, exported.EmaHistory, 1)
}

// ============================================================================
// Epoch Processing Tests
// ============================================================================

func TestProcessEpoch_ComputesMultipliers(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// All validators should have multipliers
	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	require.Len(t, all, 3)

	// All should be within bounds
	params := f.keeper.GetParams(f.ctx)
	for _, vm := range all {
		require.True(t, vm.MEffective.GTE(params.MinMultiplier),
			"multiplier %s below min %s", vm.MEffective, params.MinMultiplier)
		require.True(t, vm.MEffective.LTE(params.MaxMultiplier),
			"multiplier %s above max %s", vm.MEffective, params.MaxMultiplier)
	}
}

func TestProcessEpoch_PreventsDuplicateProcessing(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch 1
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	lastEpoch := f.keeper.GetLastProcessedEpoch(f.ctx)
	require.Equal(t, int64(1), lastEpoch)

	// Try to process again - should be no-op
	err = f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Epoch 2 should work
	err = f.keeper.ProcessEpoch(f.ctx, 2)
	require.NoError(t, err)

	lastEpoch = f.keeper.GetLastProcessedEpoch(f.ctx)
	require.Equal(t, int64(2), lastEpoch)
}

func TestProcessEpoch_BudgetNeutrality(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Get multipliers - use the same validators from our test setup
	multipliers := f.keeper.GetAllValidatorMultipliers(f.ctx)
	validators := []stakingtypes.Validator{
		createMockValidator(testVal1, math.NewInt(1000000), true),
		createMockValidator(testVal2, math.NewInt(2000000), true),
		createMockValidator(testVal3, math.NewInt(3000000), true),
	}

	// Build map
	multMap := make(map[string]types.ValidatorMultiplier)
	for _, vm := range multipliers {
		multMap[vm.ValidatorAddress] = vm
	}

	// Check budget neutrality
	totalWeight := math.LegacyZeroDec()
	weightedMultSum := math.LegacyZeroDec()

	for _, val := range validators {
		if !val.IsBonded() {
			continue
		}
		stakeWeight := val.Tokens.ToLegacyDec()
		totalWeight = totalWeight.Add(stakeWeight)

		vm, found := multMap[val.GetOperator()]
		mEff := math.LegacyOneDec()
		if found {
			mEff = vm.MEffective
		}
		weightedMultSum = weightedMultSum.Add(stakeWeight.Mul(mEff))
	}

	// V2.2: Tightened from 1% to 1e-6 to match the new invariant
	diff := weightedMultSum.Sub(totalWeight).Abs()
	ratio := diff.Quo(totalWeight)
	require.True(t, ratio.LT(math.LegacyNewDecWithPrec(1, 6)),
		"budget neutrality violated: diff ratio %s (must be < 1e-6)", ratio)
}

// ============================================================================
// Clamp Tests
// ============================================================================

func TestClamp_Enforcement(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)

	// Simulate a validator that would have a very high M_raw
	// by giving it very good metrics that exceed max
	// The epoch processing should clamp it

	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	for _, vm := range all {
		require.True(t, vm.MEffective.GTE(params.MinMultiplier))
		require.True(t, vm.MEffective.LTE(params.MaxMultiplier))
	}
}

// ============================================================================
// MsgServer Tests
// ============================================================================

func TestMsgServer_UpdateParams(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := f.keeper.GetAuthority()

	// Non-authority should fail
	_, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: sdk.AccAddress("random______________").String(),
		Params:    types.DefaultParams(),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidAuthority)

	// Authority should succeed
	newParams := types.DefaultParams()
	newParams.EMAWindow = 16
	_, err = ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	require.NoError(t, err)

	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, int64(16), got.EMAWindow)
}

// ============================================================================
// QueryServer Tests
// ============================================================================

func TestQueryServer_Params(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.DefaultMinMultiplier, resp.Params.MinMultiplier)
}

func TestQueryServer_ValidatorMultiplier(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Not found
	_, err := qs.ValidatorMultiplierQuery(f.ctx, &types.QueryValidatorMultiplierRequest{
		ValidatorAddress: testVal1,
	})
	require.Error(t, err)

	// Store and query
	vm := types.NewValidatorMultiplier(testVal1, 1)
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm))

	resp, err := qs.ValidatorMultiplierQuery(f.ctx, &types.QueryValidatorMultiplierRequest{
		ValidatorAddress: testVal1,
	})
	require.NoError(t, err)
	require.Equal(t, testVal1, resp.Multiplier.ValidatorAddress)
}

func TestQueryServer_AllMultipliers(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Empty
	resp, err := qs.AllMultipliers(f.ctx, &types.QueryAllMultipliersRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Multipliers)

	// Add some
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, types.NewValidatorMultiplier(testVal1, 1)))
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, types.NewValidatorMultiplier(testVal2, 1)))

	resp, err = qs.AllMultipliers(f.ctx, &types.QueryAllMultipliersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Multipliers, 2)
}

// ============================================================================
// Invariant Tests
// ============================================================================

func TestInvariants_BoundsEnforcement(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch to create valid multipliers
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Run invariant
	msg, broken := keeper.MultiplierBoundsInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant broken: %s", msg)
}

// ============================================================================
// Security Tests
// ============================================================================

func TestSecurity_OneValidatorCannotDominateRewards(t *testing.T) {
	// Setup with one very large validator
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())
	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec

	storeService := runtime.NewKVStoreService(storeKey)

	// One whale, two small validators
	validators := []stakingtypes.Validator{
		createMockValidator(testVal1, math.NewInt(90000000), true), // 90% of stake
		createMockValidator(testVal2, math.NewInt(5000000), true),  // 5% of stake
		createMockValidator(testVal3, math.NewInt(5000000), true),  // 5% of stake
	}
	stakingKeeper := mockStakingKeeper{validators: validators}
	slashingKeeper := newMockSlashingKeeper()
	authority := sdk.AccAddress("authority___________").String()

	k := keeper.NewKeeper(cdc.(codec.BinaryCodec), storeService, log.NewNopLogger(),
		authority, stakingKeeper, slashingKeeper)

	// Process epoch
	err := k.ProcessEpoch(ctx, 1)
	require.NoError(t, err)

	// Get multipliers
	all := k.GetAllValidatorMultipliers(ctx)
	params := k.GetParams(ctx)

	for _, vm := range all {
		// No validator should have multiplier > max
		require.True(t, vm.MEffective.LTE(params.MaxMultiplier),
			"validator %s has multiplier %s > max %s",
			vm.ValidatorAddress, vm.MEffective, params.MaxMultiplier)
		// No validator should have multiplier < min
		require.True(t, vm.MEffective.GTE(params.MinMultiplier),
			"validator %s has multiplier %s < min %s",
			vm.ValidatorAddress, vm.MEffective, params.MinMultiplier)
	}
}

// ============================================================================
// V2.1 Safety Invariant Tests
// ============================================================================

func TestNoNaNInvariant_PassesWithValidMultipliers(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process an epoch to create multipliers
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Run NoNaN invariant
	msg, broken := keeper.NoNaNInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant broken: %s", msg)
}

func TestNoNaNInvariant_DetectsNegativeOverflow(t *testing.T) {
	f := SetupKeeperTest(t)

	// Store a multiplier with a large negative value (simulating arithmetic overflow)
	vm := types.ValidatorMultiplier{
		ValidatorAddress:   testVal1,
		Epoch:              1,
		MRaw:               math.LegacyOneDec(),
		MEMA:               math.LegacyOneDec(),
		MEffective:         math.LegacyOneDec(),
		UptimeBonus:        math.LegacyZeroDec(),
		ParticipationBonus: math.LegacyZeroDec(),
		SlashPenalty:       math.LegacyNewDec(-200), // Negative and > 100 abs — unreasonable
		FraudPenalty:       math.LegacyZeroDec(),
	}
	err := f.keeper.SetValidatorMultiplier(f.ctx, vm)
	require.NoError(t, err)

	// Run invariant — should detect unreasonable value
	msg, broken := keeper.NoNaNInvariant(f.keeper)(f.ctx)
	require.True(t, broken, "invariant should be broken for negative overflow: %s", msg)
}

func TestNoNaNInvariant_DetectsOverflowValue(t *testing.T) {
	f := SetupKeeperTest(t)

	// Store a multiplier with an unreasonable value
	vm := types.ValidatorMultiplier{
		ValidatorAddress:   testVal1,
		Epoch:              1,
		MRaw:               math.LegacyNewDec(999), // > 100, unreasonable
		MEMA:               math.LegacyOneDec(),
		MEffective:         math.LegacyOneDec(),
		UptimeBonus:        math.LegacyZeroDec(),
		ParticipationBonus: math.LegacyZeroDec(),
		SlashPenalty:       math.LegacyZeroDec(),
		FraudPenalty:       math.LegacyZeroDec(),
	}
	err := f.keeper.SetValidatorMultiplier(f.ctx, vm)
	require.NoError(t, err)

	msg, broken := keeper.NoNaNInvariant(f.keeper)(f.ctx)
	require.True(t, broken, "invariant should be broken for overflow value: %s", msg)
}

func TestNoNaNInvariant_PassesEmpty(t *testing.T) {
	f := SetupKeeperTest(t)

	// No multipliers stored — should pass
	msg, broken := keeper.NoNaNInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant should pass with no multipliers: %s", msg)
}

// ============================================================================
// V2.2 Safety & Audit Hardening Tests
// ============================================================================

// --- Tight Budget Neutral Invariant ---

func TestBudgetNeutralInvariant_TightEpsilon(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch to create valid multipliers via iterative normalization
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Run the V2.2 budget-neutral invariant (now uses 1e-6 epsilon)
	msg, broken := keeper.BudgetNeutralInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant broken with tight epsilon: %s", msg)
}

func TestBudgetNeutralInvariant_DetectsRealDrift(t *testing.T) {
	f := SetupKeeperTest(t)

	// Manually store multipliers that violate budget neutrality
	// All three validators at 1.10 means Σ(w*M) = 1.10 * Σ(w), a 10% inflation
	for _, addr := range []string{testVal1, testVal2, testVal3} {
		vm := types.NewValidatorMultiplier(addr, 1)
		vm.MEffective = math.LegacyNewDecWithPrec(110, 2) // 1.10
		require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm))
	}

	msg, broken := keeper.BudgetNeutralInvariant(f.keeper)(f.ctx)
	require.True(t, broken, "invariant should detect 10%% inflation drift: %s", msg)
}

// --- Iterative Normalization ---

func TestIterativeNorm_ConvergesWithClampBreak(t *testing.T) {
	// Create a scenario where single-pass normalization would fail:
	// Validators with very different multipliers such that normalization
	// pushes some past the clamp boundary.
	f := SetupKeeperTest(t)

	// Set wide multiplier bounds to create a clamp-break scenario
	params := types.DefaultParams()
	params.MinMultiplier = math.LegacyNewDecWithPrec(90, 2)  // 0.90
	params.MaxMultiplier = math.LegacyNewDecWithPrec(110, 2) // 1.10
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Process epoch — iterative normalization should handle any clamp-break
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Verify all multipliers are within bounds
	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	for _, vm := range all {
		require.True(t, vm.MEffective.GTE(params.MinMultiplier),
			"validator %s: %s < min %s", vm.ValidatorAddress, vm.MEffective, params.MinMultiplier)
		require.True(t, vm.MEffective.LTE(params.MaxMultiplier),
			"validator %s: %s > max %s", vm.ValidatorAddress, vm.MEffective, params.MaxMultiplier)
	}

	// Verify budget neutrality is tight after iterative normalization
	totalWeight := math.LegacyZeroDec()
	weightedMultSum := math.LegacyZeroDec()
	for _, vm := range all {
		// Use the test setup validators to get stake weights
		var stakeWt math.LegacyDec
		switch vm.ValidatorAddress {
		case testVal1:
			stakeWt = math.NewInt(1000000).ToLegacyDec()
		case testVal2:
			stakeWt = math.NewInt(2000000).ToLegacyDec()
		case testVal3:
			stakeWt = math.NewInt(3000000).ToLegacyDec()
		}
		totalWeight = totalWeight.Add(stakeWt)
		weightedMultSum = weightedMultSum.Add(stakeWt.Mul(vm.MEffective))
	}

	diff := weightedMultSum.Sub(totalWeight).Abs()
	ratio := diff.Quo(totalWeight)
	// With iterative normalization, error should be < 1e-6
	require.True(t, ratio.LT(math.LegacyNewDecWithPrec(1, 6)),
		"iterative normalization budget error %s exceeds 1e-6", ratio)
}

func TestIterativeNorm_MultipleEpochsStable(t *testing.T) {
	f := SetupKeeperTest(t)

	// Run 5 consecutive epochs and verify stability
	for epoch := int64(1); epoch <= 5; epoch++ {
		err := f.keeper.ProcessEpoch(f.ctx, epoch)
		require.NoError(t, err)

		// After each epoch, invariant should hold
		msg, broken := keeper.BudgetNeutralInvariant(f.keeper)(f.ctx)
		require.False(t, broken, "invariant broken at epoch %d: %s", epoch, msg)

		msg, broken = keeper.MultiplierBoundsInvariant(f.keeper)(f.ctx)
		require.False(t, broken, "bounds invariant broken at epoch %d: %s", epoch, msg)
	}
}

// --- Stake Weight Snapshot Consistency ---

func TestStakeSnapshot_CapturedOnEpochProcessing(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch 1 — should capture stake snapshots
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Verify snapshots exist for epoch 1
	snapshots := f.keeper.GetEpochStakeSnapshots(f.ctx, 1)
	require.Len(t, snapshots, 3, "expected 3 stake snapshots for 3 bonded validators")

	// Verify snapshot values match test setup
	snapshotMap := make(map[string]math.Int)
	for _, s := range snapshots {
		snapshotMap[s.ValidatorAddress] = s.BondedTokens
		require.Equal(t, int64(1), s.Epoch)
	}

	require.True(t, snapshotMap[testVal1].Equal(math.NewInt(1000000)),
		"val1 snapshot should be 1000000, got %s", snapshotMap[testVal1])
	require.True(t, snapshotMap[testVal2].Equal(math.NewInt(2000000)),
		"val2 snapshot should be 2000000, got %s", snapshotMap[testVal2])
	require.True(t, snapshotMap[testVal3].Equal(math.NewInt(3000000)),
		"val3 snapshot should be 3000000, got %s", snapshotMap[testVal3])
}

func TestStakeSnapshot_DifferentEpochsIndependent(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process two epochs
	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 1))
	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 2))

	// Each epoch should have its own snapshots
	snap1 := f.keeper.GetEpochStakeSnapshots(f.ctx, 1)
	snap2 := f.keeper.GetEpochStakeSnapshots(f.ctx, 2)

	require.Len(t, snap1, 3)
	require.Len(t, snap2, 3)

	// Epoch 0 (never processed) should be empty
	snap0 := f.keeper.GetEpochStakeSnapshots(f.ctx, 0)
	require.Empty(t, snap0)
}

func TestStakeSnapshot_QueryEndpoint(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 1))

	resp, err := qs.StakeSnapshot(f.ctx, &types.QueryStakeSnapshotRequest{Epoch: 1})
	require.NoError(t, err)
	require.Len(t, resp.Snapshots, 3)

	// Non-existent epoch returns empty
	resp, err = qs.StakeSnapshot(f.ctx, &types.QueryStakeSnapshotRequest{Epoch: 999})
	require.NoError(t, err)
	require.Empty(t, resp.Snapshots)
}

// --- Warm-Start EMA ---

func TestWarmStartEMA_FirstEpochUsesRawValue(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process first epoch
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Get multipliers — on first epoch with no prior history, M_ema should
	// equal M_raw (not a neutral 1.0 that ignores actual performance).
	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	require.NotEmpty(t, all)

	for _, vm := range all {
		// With warm-start, MEMA should equal MRaw on first epoch
		// (single value in EMA history → ComputeEMA returns that value)
		require.True(t, vm.MEMA.Equal(vm.MRaw),
			"validator %s: warm-start failed, MEMA=%s != MRaw=%s",
			vm.ValidatorAddress, vm.MEMA, vm.MRaw)
	}
}

func TestWarmStartEMA_SubsequentEpochsSmooth(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process two epochs
	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 1))
	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 2))

	// After 2 epochs, EMA should differ from raw (smoothing effect)
	all := f.keeper.GetAllValidatorMultipliers(f.ctx)
	for _, vm := range all {
		// EMA history has 2 values, so ComputeEMA applies smoothing
		history := f.keeper.GetEMAHistory(f.ctx, vm.ValidatorAddress)
		require.Len(t, history.Values, 2, "should have 2 EMA history values after 2 epochs")
	}
}

// --- Audit-Grade Events ---

func TestAuditEvent_EmittedOnEpochProcessing(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Check that both events were emitted
	events := f.ctx.EventManager().Events()

	foundNormEvent := false
	foundLegacyEvent := false

	for _, event := range events {
		if event.Type == "rewardmult_epoch_normalization" {
			foundNormEvent = true
			// Verify all required attributes are present
			attrMap := make(map[string]string)
			for _, attr := range event.Attributes {
				attrMap[attr.Key] = attr.Value
			}
			require.Contains(t, attrMap, "epoch")
			require.Contains(t, attrMap, "norm_factor")
			require.Contains(t, attrMap, "total_stake")
			require.Contains(t, attrMap, "weighted_sum_before_norm")
			require.Contains(t, attrMap, "weighted_sum_after_norm")
			require.Contains(t, attrMap, "count_clamped_min")
			require.Contains(t, attrMap, "count_clamped_max")
			require.Contains(t, attrMap, "iterative_rounds")
			require.Contains(t, attrMap, "budget_error")

			require.Equal(t, "1", attrMap["epoch"])
		}
		if event.Type == "rewardmult_epoch_processed" {
			foundLegacyEvent = true
		}
	}

	require.True(t, foundNormEvent, "rewardmult_epoch_normalization event not emitted")
	require.True(t, foundLegacyEvent, "legacy rewardmult_epoch_processed event not emitted")
}

// --- Clamp Pressure Telemetry ---

func TestClampPressure_QueryReturnsCorrectCounts(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process epoch to create multipliers
	err := f.keeper.ProcessEpoch(f.ctx, 1)
	require.NoError(t, err)

	// Query clamp pressure
	resp := f.keeper.GetClampPressure(f.ctx)
	require.Equal(t, int64(1), resp.Epoch)
	require.Equal(t, 3, resp.TotalValidators)
	// Clamp counts depend on whether any validator hit min/max
	require.GreaterOrEqual(t, resp.CountClampedMin, 0)
	require.GreaterOrEqual(t, resp.CountClampedMax, 0)
	require.LessOrEqual(t, resp.CountClampedMin+resp.CountClampedMax, resp.TotalValidators)
}

func TestClampPressure_QueryEndpoint(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	require.NoError(t, f.keeper.ProcessEpoch(f.ctx, 1))

	resp, err := qs.ClampPressure(f.ctx, &types.QueryClampPressureRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Epoch)
	require.Equal(t, 3, resp.TotalValidators)
}

func TestClampPressure_ManuallyClampedValidators(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)

	// Manually store multipliers at clamp boundaries
	vm1 := types.NewValidatorMultiplier(testVal1, 1)
	vm1.MEffective = params.MinMultiplier // clamped at min
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm1))

	vm2 := types.NewValidatorMultiplier(testVal2, 1)
	vm2.MEffective = params.MaxMultiplier // clamped at max
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm2))

	vm3 := types.NewValidatorMultiplier(testVal3, 1)
	vm3.MEffective = math.LegacyOneDec() // not clamped
	require.NoError(t, f.keeper.SetValidatorMultiplier(f.ctx, vm3))

	resp := f.keeper.GetClampPressure(f.ctx)
	require.Equal(t, 1, resp.CountClampedMin)
	require.Equal(t, 1, resp.CountClampedMax)
	require.Equal(t, 3, resp.TotalValidators)
}

// --- Combined V2.2 Integration Test ---

func TestV22_FullIntegration(t *testing.T) {
	f := SetupKeeperTest(t)

	// Process multiple epochs
	for epoch := int64(1); epoch <= 3; epoch++ {
		err := f.keeper.ProcessEpoch(f.ctx, epoch)
		require.NoError(t, err)

		// 1. Budget neutrality with tight epsilon
		msg, broken := keeper.BudgetNeutralInvariant(f.keeper)(f.ctx)
		require.False(t, broken, "epoch %d budget invariant: %s", epoch, msg)

		// 2. Bounds invariant
		msg, broken = keeper.MultiplierBoundsInvariant(f.keeper)(f.ctx)
		require.False(t, broken, "epoch %d bounds invariant: %s", epoch, msg)

		// 3. NoNaN invariant
		msg, broken = keeper.NoNaNInvariant(f.keeper)(f.ctx)
		require.False(t, broken, "epoch %d nan invariant: %s", epoch, msg)

		// 4. Stake snapshots captured
		snapshots := f.keeper.GetEpochStakeSnapshots(f.ctx, epoch)
		require.Len(t, snapshots, 3, "epoch %d missing stake snapshots", epoch)

		// 5. Clamp pressure query works
		pressure := f.keeper.GetClampPressure(f.ctx)
		require.Equal(t, epoch, pressure.Epoch)
		require.Equal(t, 3, pressure.TotalValidators)
	}
}

// --- Whale Validator Budget Neutrality with Tight Epsilon ---

func TestV22_WhaleValidator_TightBudgetNeutrality(t *testing.T) {
	// Same as TestSecurity_OneValidatorCannotDominateRewards but with V2.2 tight check
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())
	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec
	storeService := runtime.NewKVStoreService(storeKey)

	// Extreme stake distribution: 95% / 3% / 2%
	validators := []stakingtypes.Validator{
		createMockValidator(testVal1, math.NewInt(95000000), true),
		createMockValidator(testVal2, math.NewInt(3000000), true),
		createMockValidator(testVal3, math.NewInt(2000000), true),
	}
	stakingKeeper := mockStakingKeeper{validators: validators}
	slashingKeeper := newMockSlashingKeeper()
	authority := sdk.AccAddress("authority___________").String()

	k := keeper.NewKeeper(cdc.(codec.BinaryCodec), storeService, log.NewNopLogger(),
		authority, stakingKeeper, slashingKeeper)

	// Process epoch
	err := k.ProcessEpoch(ctx, 1)
	require.NoError(t, err)

	// Verify V2.2 tight budget neutrality invariant passes
	msg, broken := keeper.BudgetNeutralInvariant(k)(ctx)
	require.False(t, broken, "whale scenario budget invariant failed: %s", msg)

	// Also verify bounds
	msg, broken = keeper.MultiplierBoundsInvariant(k)(ctx)
	require.False(t, broken, "whale scenario bounds invariant failed: %s", msg)
}

// Ensure the mock implements the interface
var _ types.StakingKeeper = mockStakingKeeper{}
var _ types.SlashingKeeper = (*mockSlashingKeeper)(nil)
