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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/x/repgov/keeper"
	"pos/x/repgov/types"
)

// ========== Test Fixture ==========

type KeeperTestFixture struct {
	ctx       sdk.Context
	keeper    keeper.Keeper
	authority string
	pocKeeper *mockPocKeeper
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
	return math.NewInt(6000000), nil
}

func (m mockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return math.NewInt(1000000)
}

// Mock PoC keeper
type mockPocKeeper struct {
	credits          map[string]math.Int
	reputationScores map[string]math.LegacyDec
	endorsementRates map[string]math.LegacyDec
	originality      map[string]math.LegacyDec
	quality          map[string]math.LegacyDec
}

func newMockPocKeeper() *mockPocKeeper {
	return &mockPocKeeper{
		credits:          make(map[string]math.Int),
		reputationScores: make(map[string]math.LegacyDec),
		endorsementRates: make(map[string]math.LegacyDec),
		originality:      make(map[string]math.LegacyDec),
		quality:          make(map[string]math.LegacyDec),
	}
}

func (m *mockPocKeeper) GetCreditAmount(_ context.Context, addr string) math.Int {
	if amt, ok := m.credits[addr]; ok {
		return amt
	}
	return math.ZeroInt()
}

func (m *mockPocKeeper) GetReputationScoreValue(_ context.Context, addr string) math.LegacyDec {
	if score, ok := m.reputationScores[addr]; ok {
		return score
	}
	return math.LegacyZeroDec()
}

func (m *mockPocKeeper) GetEndorsementParticipationRate(_ context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	if rate, ok := m.endorsementRates[valAddr.String()]; ok {
		return rate, nil
	}
	return math.LegacyZeroDec(), nil
}

func (m *mockPocKeeper) GetValidatorOriginalityMetrics(_ context.Context, valAddr sdk.ValAddress) (math.LegacyDec, math.LegacyDec, error) {
	orig := math.LegacyOneDec()
	qual := math.LegacyNewDecWithPrec(5, 1)
	if o, ok := m.originality[valAddr.String()]; ok {
		orig = o
	}
	if q, ok := m.quality[valAddr.String()]; ok {
		qual = q
	}
	return orig, qual, nil
}

// Test addresses
var (
	testAddr1 = sdk.AccAddress("addr1_______________").String()
	testAddr2 = sdk.AccAddress("addr2_______________").String()
	testAddr3 = sdk.AccAddress("addr3_______________").String()
	testAuth  = sdk.AccAddress("authority___________").String()
)

func setupTest(t *testing.T) *KeeperTestFixture {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, cmtproto.Header{
		Height: 100,
		Time:   time.Now(),
	}, false, log.NewNopLogger())

	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec
	storeService := runtime.NewKVStoreService(storeKey)

	stakingKeeper := mockStakingKeeper{}
	poc := newMockPocKeeper()

	k := keeper.NewKeeper(
		cdc.(codec.BinaryCodec),
		storeService,
		log.NewNopLogger(),
		testAuth,
		stakingKeeper,
	)
	k.SetPocKeeper(poc)

	return &KeeperTestFixture{
		ctx:       ctx,
		keeper:    k,
		authority: testAuth,
		pocKeeper: poc,
	}
}

// ========== Params Tests ==========

func TestParams_DefaultParams(t *testing.T) {
	f := setupTest(t)
	params := f.keeper.GetParams(f.ctx)
	require.True(t, params.MaxVotingWeightCap.Equal(types.DefaultMaxVotingWeightCap))
}

func TestParams_SetAndGet(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.MaxVotingWeightCap = math.LegacyNewDec(3)
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	got := f.keeper.GetParams(f.ctx)
	require.True(t, got.MaxVotingWeightCap.Equal(math.LegacyNewDec(3)))
}

func TestParams_Validate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())

	p.MaxVotingWeightCap = math.LegacyNewDecWithPrec(5, 1) // 0.5 < 1.0
	require.Error(t, p.Validate())
}

// ========== VoterWeight CRUD Tests ==========

func TestVoterWeight_SetAndGet(t *testing.T) {
	f := setupTest(t)
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(2)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	got, found := f.keeper.GetVoterWeight(f.ctx, testAddr1)
	require.True(t, found)
	require.True(t, got.EffectiveWeight.Equal(math.LegacyNewDec(2)))
}

func TestVoterWeight_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetVoterWeight(f.ctx, testAddr1)
	require.False(t, found)
}

func TestVoterWeight_GetAll(t *testing.T) {
	f := setupTest(t)
	vw1 := types.NewVoterWeight(testAddr1, 1)
	vw2 := types.NewVoterWeight(testAddr2, 1)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw1))
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw2))

	all := f.keeper.GetAllVoterWeights(f.ctx)
	require.Len(t, all, 2)
}

func TestVoterWeight_EffectiveWeight_DisabledModule(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	w := f.keeper.GetEffectiveVotingWeight(f.ctx, testAddr1)
	require.True(t, w.Equal(math.LegacyOneDec()))
}

// ========== Delegation CRUD Tests ==========

func TestDelegation_SetAndGet(t *testing.T) {
	f := setupTest(t)
	d := types.DelegatedReputation{
		Delegator: testAddr1,
		Delegatee: testAddr2,
		Amount:    math.LegacyNewDecWithPrec(5, 1),
	}
	require.NoError(t, f.keeper.SetDelegation(f.ctx, d))

	got, found := f.keeper.GetDelegation(f.ctx, testAddr1, testAddr2)
	require.True(t, found)
	require.True(t, got.Amount.Equal(math.LegacyNewDecWithPrec(5, 1)))
}

func TestDelegation_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetDelegation(f.ctx, testAddr1, testAddr2)
	require.False(t, found)
}

func TestDelegation_Delete(t *testing.T) {
	f := setupTest(t)
	d := types.DelegatedReputation{
		Delegator: testAddr1,
		Delegatee: testAddr2,
		Amount:    math.LegacyNewDecWithPrec(5, 1),
	}
	require.NoError(t, f.keeper.SetDelegation(f.ctx, d))
	require.NoError(t, f.keeper.DeleteDelegation(f.ctx, testAddr1, testAddr2))

	_, found := f.keeper.GetDelegation(f.ctx, testAddr1, testAddr2)
	require.False(t, found)
}

func TestDelegation_GetDelegationsFrom(t *testing.T) {
	f := setupTest(t)
	d1 := types.DelegatedReputation{Delegator: testAddr1, Delegatee: testAddr2, Amount: math.LegacyNewDecWithPrec(3, 1)}
	d2 := types.DelegatedReputation{Delegator: testAddr1, Delegatee: testAddr3, Amount: math.LegacyNewDecWithPrec(2, 1)}
	require.NoError(t, f.keeper.SetDelegation(f.ctx, d1))
	require.NoError(t, f.keeper.SetDelegation(f.ctx, d2))

	delegations := f.keeper.GetDelegationsFrom(f.ctx, testAddr1)
	require.Len(t, delegations, 2)
}

// ========== TallyOverride CRUD Tests ==========

func TestTallyOverride_SetAndGet(t *testing.T) {
	f := setupTest(t)
	to := types.TallyOverride{ProposalID: 1, WeightedYes: math.LegacyNewDec(100)}
	require.NoError(t, f.keeper.SetTallyOverride(f.ctx, to))

	got, found := f.keeper.GetTallyOverride(f.ctx, 1)
	require.True(t, found)
	require.True(t, got.WeightedYes.Equal(math.LegacyNewDec(100)))
}

func TestTallyOverride_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetTallyOverride(f.ctx, 999)
	require.False(t, found)
}

// ========== ComputeVoterWeight Tests ==========

func TestComputeVoterWeight_DisabledModule(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	vw := f.keeper.ComputeVoterWeight(f.ctx, testAddr1, 1)
	require.True(t, vw.EffectiveWeight.Equal(math.LegacyOneDec()))
}

func TestComputeVoterWeight_BelowMinThreshold(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Set reputation below threshold (0.10)
	f.pocKeeper.reputationScores[testAddr1] = math.LegacyNewDecWithPrec(5, 2) // 0.05

	vw := f.keeper.ComputeVoterWeight(f.ctx, testAddr1, 1)
	// Below threshold means no bonus: effective weight = 1.0
	require.True(t, vw.EffectiveWeight.Equal(math.LegacyOneDec()))
}

func TestComputeVoterWeight_WithReputation(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Set high reputation and credits
	f.pocKeeper.reputationScores[testAddr1] = math.LegacyNewDecWithPrec(8, 1) // 0.8
	f.pocKeeper.credits[testAddr1] = math.NewInt(5000)                        // 50% normalized

	vw := f.keeper.ComputeVoterWeight(f.ctx, testAddr1, 1)
	// Weight should be > 1.0 (has reputation bonus)
	require.True(t, vw.EffectiveWeight.GT(math.LegacyOneDec()))
	// Weight should be <= MaxVotingWeightCap (5.0)
	require.True(t, vw.EffectiveWeight.LTE(p.MaxVotingWeightCap))
}

// ========== LastComputedEpoch Tests ==========

func TestLastComputedEpoch(t *testing.T) {
	f := setupTest(t)
	require.Equal(t, int64(0), f.keeper.GetLastComputedEpoch(f.ctx))
	require.NoError(t, f.keeper.SetLastComputedEpoch(f.ctx, 42))
	require.Equal(t, int64(42), f.keeper.GetLastComputedEpoch(f.ctx))
}

// ========== MsgServer Tests ==========

func TestMsgServer_UpdateParams_ValidAuthority(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.MaxVotingWeightCap = math.LegacyNewDec(3)
	resp, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: testAuth,
		Params:    p,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	got := f.keeper.GetParams(f.ctx)
	require.True(t, got.MaxVotingWeightCap.Equal(math.LegacyNewDec(3)))
}

func TestMsgServer_UpdateParams_InvalidAuthority(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: testAddr1, // not the authority
		Params:    types.DefaultParams(),
	})
	require.Error(t, err)
}

func TestMsgServer_DelegateReputation_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.DelegationEnabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Register both voters first
	require.NoError(t, f.keeper.EnsureVoterRegistered(f.ctx, testAddr1))

	resp, err := ms.DelegateReputation(f.ctx, &types.MsgDelegateReputation{
		Delegator: testAddr1,
		Delegatee: testAddr2,
		Amount:    math.LegacyNewDecWithPrec(2, 1), // 0.2
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	d, found := f.keeper.GetDelegation(f.ctx, testAddr1, testAddr2)
	require.True(t, found)
	require.True(t, d.Amount.Equal(math.LegacyNewDecWithPrec(2, 1)))
}

func TestMsgServer_UndelegateReputation_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.DelegationEnabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Set a delegation
	d := types.DelegatedReputation{Delegator: testAddr1, Delegatee: testAddr2, Amount: math.LegacyNewDecWithPrec(2, 1)}
	require.NoError(t, f.keeper.SetDelegation(f.ctx, d))

	resp, err := ms.UndelegateReputation(f.ctx, &types.MsgUndelegateReputation{
		Delegator: testAddr1,
		Delegatee: testAddr2,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	_, found := f.keeper.GetDelegation(f.ctx, testAddr1, testAddr2)
	require.False(t, found)
}

func TestMsgServer_UndelegateReputation_NotFound(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.DelegationEnabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	_, err := ms.UndelegateReputation(f.ctx, &types.MsgUndelegateReputation{
		Delegator: testAddr1,
		Delegatee: testAddr2,
	})
	require.Error(t, err)
}

// ========== Genesis Tests ==========

func TestGenesis_InitAndExport(t *testing.T) {
	f := setupTest(t)
	gs := types.DefaultGenesis()
	require.NoError(t, f.keeper.InitGenesis(f.ctx, *gs))

	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.NoError(t, exported.Params.Validate())
}

func TestGenesis_Roundtrip(t *testing.T) {
	f := setupTest(t)

	// Init with default
	gs := types.DefaultGenesis()
	require.NoError(t, f.keeper.InitGenesis(f.ctx, *gs))

	// Add some state
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(2)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	// Export
	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.Len(t, exported.VoterWeights, 1)
	require.Equal(t, testAddr1, exported.VoterWeights[0].Address)
}

// ========== EndBlocker Tests ==========

func TestEndBlocker_DisabledModule(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Should not error even when disabled
	err := f.keeper.EndBlocker(f.ctx)
	require.NoError(t, err)
}

func TestEndBlocker_NonEpochBoundary(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	p.RecomputeInterval = 100
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Block 100 with last epoch at 100 means we're at the same epoch — no recompute
	require.NoError(t, f.keeper.SetLastComputedEpoch(f.ctx, 100))

	err := f.keeper.EndBlocker(f.ctx)
	require.NoError(t, err)
}

// ========== Invariant Tests ==========

func TestInvariant_WeightBounds_Pass(t *testing.T) {
	f := setupTest(t)
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(2)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	msg, broken := keeper.WeightBoundsInvariant(f.keeper)(f.ctx)
	require.False(t, broken, msg)
}

func TestInvariant_WeightBounds_FailBelowOne(t *testing.T) {
	f := setupTest(t)
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDecWithPrec(5, 1) // 0.5 < 1.0
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	_, broken := keeper.WeightBoundsInvariant(f.keeper)(f.ctx)
	require.True(t, broken)
}

func TestInvariant_WeightBounds_FailAboveCap(t *testing.T) {
	f := setupTest(t)
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(10) // 10 > default cap 5
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	_, broken := keeper.WeightBoundsInvariant(f.keeper)(f.ctx)
	require.True(t, broken)
}

// ========== Query Tests ==========

func TestQuery_Params(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.True(t, resp.Params.MaxVotingWeightCap.Equal(types.DefaultMaxVotingWeightCap))
}

func TestQuery_VoterWeight_Found(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(3)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	resp, err := qs.VoterWeight(f.ctx, &types.QueryVoterWeightRequest{Address: testAddr1})
	require.NoError(t, err)
	require.True(t, resp.Weight.EffectiveWeight.Equal(math.LegacyNewDec(3)))
}

func TestQuery_VoterWeight_NotFound(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.VoterWeight(f.ctx, &types.QueryVoterWeightRequest{Address: testAddr1})
	require.Error(t, err)
}

func TestQuery_AllVoterWeights(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, types.NewVoterWeight(testAddr1, 1)))
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, types.NewVoterWeight(testAddr2, 1)))

	resp, err := qs.AllVoterWeights(f.ctx, &types.QueryAllVoterWeightsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Weights, 2)
}

// ========== EnsureVoterRegistered Tests ==========

func TestEnsureVoterRegistered_NewVoter(t *testing.T) {
	f := setupTest(t)
	require.NoError(t, f.keeper.EnsureVoterRegistered(f.ctx, testAddr1))

	_, found := f.keeper.GetVoterWeight(f.ctx, testAddr1)
	require.True(t, found)
}

func TestEnsureVoterRegistered_AlreadyRegistered(t *testing.T) {
	f := setupTest(t)
	vw := types.NewVoterWeight(testAddr1, 1)
	vw.EffectiveWeight = math.LegacyNewDec(3)
	require.NoError(t, f.keeper.SetVoterWeight(f.ctx, vw))

	// Should not overwrite existing
	require.NoError(t, f.keeper.EnsureVoterRegistered(f.ctx, testAddr1))
	got, _ := f.keeper.GetVoterWeight(f.ctx, testAddr1)
	require.True(t, got.EffectiveWeight.Equal(math.LegacyNewDec(3)))
}

// ========== RecordVoteParticipation Tests ==========

func TestRecordVoteParticipation(t *testing.T) {
	f := setupTest(t)
	require.NoError(t, f.keeper.RecordVoteParticipation(f.ctx, testAddr1, 500))

	vw, found := f.keeper.GetVoterWeight(f.ctx, testAddr1)
	require.True(t, found)
	require.Equal(t, int64(500), vw.LastVoteHeight)
}
