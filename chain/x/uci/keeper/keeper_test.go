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
	"github.com/stretchr/testify/require"

	"pos/x/uci/keeper"
	"pos/x/uci/types"
)

// ========== Test Fixture ==========

type KeeperTestFixture struct {
	ctx        sdk.Context
	keeper     keeper.Keeper
	authority  string
	bankKeeper *mockBankKeeper
	pocKeeper  *mockPocKeeper
}

// Mock bank keeper
type mockBankKeeper struct {
	burned sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{burned: sdk.NewCoins()}
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) BurnCoins(_ context.Context, _ string, amt sdk.Coins) error {
	m.burned = m.burned.Add(amt...)
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, math.NewInt(1000000))
}

// Mock account keeper
type mockAccountKeeper struct{}

func (m mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return sdk.AccAddress(moduleName)
}

func (m mockAccountKeeper) GetModuleAccount(_ context.Context, _ string) sdk.ModuleAccountI {
	return nil
}

// Mock PoC keeper
type mockPocKeeper struct {
	nextID uint64
}

func newMockPocKeeper() *mockPocKeeper {
	return &mockPocKeeper{nextID: 1}
}

func (m *mockPocKeeper) SubmitContribution(_ context.Context, _ string, _ string, _ string, _ string) (uint64, error) {
	id := m.nextID
	m.nextID++
	return id, nil
}

// Test addresses
var (
	testAddr1    = sdk.AccAddress("addr1_______________").String()
	testAddr2    = sdk.AccAddress("addr2_______________").String()
	testOracle1  = sdk.AccAddress("oracle1_____________").String()
	testOracle2  = sdk.AccAddress("oracle2_____________").String()
	testAuth     = sdk.AccAddress("authority___________").String()
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

	bank := newMockBankKeeper()
	acct := mockAccountKeeper{}
	poc := newMockPocKeeper()

	k := keeper.NewKeeper(
		cdc.(codec.BinaryCodec),
		storeService,
		log.NewNopLogger(),
		testAuth,
		bank,
		acct,
	)
	k.SetPocKeeper(poc)

	return &KeeperTestFixture{
		ctx:        ctx,
		keeper:     k,
		authority:  testAuth,
		bankKeeper: bank,
		pocKeeper:  poc,
	}
}

// ========== Params Tests ==========

func TestParams_DefaultParams(t *testing.T) {
	f := setupTest(t)
	params := f.keeper.GetParams(f.ctx)
	require.True(t, params.DefaultRewardShare.Equal(types.DefaultDefaultRewardShare))
}

func TestParams_SetAndGet(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.MaxAdapters = 100
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, int64(100), got.MaxAdapters)
}

func TestParams_Validate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())

	p.MaxAdapters = 0
	require.Error(t, p.Validate())
}

// ========== Adapter CRUD Tests ==========

func TestAdapter_SetAndGet(t *testing.T) {
	f := setupTest(t)
	adapter := types.NewAdapter(1, "Helium", testAddr1, "QmTest", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "test adapter")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	got, found := f.keeper.GetAdapter(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, "Helium", got.Name)
	require.Equal(t, testAddr1, got.Owner)
}

func TestAdapter_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetAdapter(f.ctx, 999)
	require.False(t, found)
}

func TestAdapter_GetAll(t *testing.T) {
	f := setupTest(t)
	a1 := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	a2 := types.NewAdapter(2, "Hivemapper", testAddr2, "Qm2", []string{testOracle2}, "hivemapper", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a1))
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a2))

	all := f.keeper.GetAllAdapters(f.ctx)
	require.Len(t, all, 2)
}

func TestAdapter_GetByOwner(t *testing.T) {
	f := setupTest(t)
	a1 := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	a2 := types.NewAdapter(2, "Hivemapper", testAddr1, "Qm2", []string{testOracle2}, "hivemapper", 100, math.LegacyNewDecWithPrec(80, 2), "")
	a3 := types.NewAdapter(3, "DIMO", testAddr2, "Qm3", []string{testOracle1}, "dimo", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a1))
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a2))
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a3))

	owner1 := f.keeper.GetAdaptersByOwner(f.ctx, testAddr1)
	require.Len(t, owner1, 2)
}

// ========== NextAdapterID Tests ==========

func TestNextAdapterID(t *testing.T) {
	f := setupTest(t)
	id := f.keeper.GetNextAdapterID(f.ctx)
	require.Equal(t, uint64(1), id)

	require.NoError(t, f.keeper.SetNextAdapterID(f.ctx, 5))
	require.Equal(t, uint64(5), f.keeper.GetNextAdapterID(f.ctx))
}

// ========== ContributionMapping Tests ==========

func TestContributionMapping_SetAndGet(t *testing.T) {
	f := setupTest(t)
	cm := types.ContributionMapping{
		AdapterID:         1,
		ExternalID:        "ext-123",
		PocContributionID: 42,
		Contributor:       testAddr1,
		MappedAtHeight:    100,
		RewardAmount:      math.ZeroInt(),
		OracleVerified:    false,
	}
	require.NoError(t, f.keeper.SetContributionMapping(f.ctx, cm))

	got, found := f.keeper.GetContributionMapping(f.ctx, 1, "ext-123")
	require.True(t, found)
	require.Equal(t, uint64(42), got.PocContributionID)
}

func TestContributionMapping_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetContributionMapping(f.ctx, 1, "nonexistent")
	require.False(t, found)
}

// ========== AdapterStats Tests ==========

func TestAdapterStats_SetAndGet(t *testing.T) {
	f := setupTest(t)
	stats := types.NewAdapterStats(1)
	stats.TotalSubmissions = 10
	stats.Successful = 8
	require.NoError(t, f.keeper.SetAdapterStats(f.ctx, stats))

	got := f.keeper.GetAdapterStats(f.ctx, 1)
	require.Equal(t, uint64(10), got.TotalSubmissions)
	require.Equal(t, uint64(8), got.Successful)
}

// ========== OracleAttestation Tests ==========

func TestOracleAttestation_SetAndGet(t *testing.T) {
	f := setupTest(t)
	att := types.OracleAttestation{
		AdapterID:         1,
		BatchID:           "batch-1",
		OracleAddress:     testOracle1,
		AttestationHash:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		ContributionCount: 5,
		AttestedAtHeight:  100,
		Signature:         "sig",
	}
	require.NoError(t, f.keeper.SetOracleAttestation(f.ctx, att))
}

// ========== IsOracleAuthorized Tests ==========

func TestIsOracleAuthorized(t *testing.T) {
	f := setupTest(t)
	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1, testOracle2}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	require.True(t, f.keeper.IsOracleAuthorized(adapter, testOracle1))
	require.True(t, f.keeper.IsOracleAuthorized(adapter, testOracle2))
	require.False(t, f.keeper.IsOracleAuthorized(adapter, testAddr1))
}

// ========== ProcessDePINContribution Tests ==========

func TestProcessDePINContribution_Valid(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	pocID, err := f.keeper.ProcessDePINContribution(f.ctx, 1, "ext-1", testAddr2, "hash123", "ipfs://data")
	require.NoError(t, err)
	require.Equal(t, uint64(1), pocID)

	// Verify mapping was created
	cm, found := f.keeper.GetContributionMapping(f.ctx, 1, "ext-1")
	require.True(t, found)
	require.Equal(t, pocID, cm.PocContributionID)
}

func TestProcessDePINContribution_AdapterNotFound(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	_, err := f.keeper.ProcessDePINContribution(f.ctx, 999, "ext-1", testAddr1, "hash", "uri")
	require.Error(t, err)
}

func TestProcessDePINContribution_AdapterSuspended(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	adapter.Status = types.AdapterStatusSuspended
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	_, err := f.keeper.ProcessDePINContribution(f.ctx, 1, "ext-1", testAddr2, "hash", "uri")
	require.Error(t, err)
}

func TestProcessDePINContribution_DuplicateExternalID(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	_, err := f.keeper.ProcessDePINContribution(f.ctx, 1, "ext-1", testAddr2, "hash", "uri")
	require.NoError(t, err)

	// Second submission with same external ID should fail
	_, err = f.keeper.ProcessDePINContribution(f.ctx, 1, "ext-1", testAddr2, "hash2", "uri2")
	require.Error(t, err)
}

func TestProcessDePINContribution_ModuleDisabled(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	_, err := f.keeper.ProcessDePINContribution(f.ctx, 1, "ext-1", testAddr1, "hash", "uri")
	require.Error(t, err)
}

// ========== MsgServer Tests ==========

func TestMsgServer_UpdateParams_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.MaxAdapters = 100
	resp, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{Authority: testAuth, Params: p})
	require.NoError(t, err)
	require.NotNil(t, resp)

	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, int64(100), got.MaxAdapters)
}

func TestMsgServer_UpdateParams_InvalidAuthority(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{Authority: testAddr1, Params: types.DefaultParams()})
	require.Error(t, err)
}

func TestMsgServer_RegisterAdapter_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.AdapterRegistrationFee = math.ZeroInt() // no fee for testing
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	resp, err := ms.RegisterAdapter(f.ctx, &types.MsgRegisterAdapter{
		Owner:           testAddr1,
		Name:            "Helium Adapter",
		SchemaCID:       "QmTestSchema",
		OracleAllowlist: []string{testOracle1},
		NetworkType:     "helium",
		RewardShare:     math.LegacyNewDecWithPrec(80, 2),
		Description:     "Test DePIN adapter",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.AdapterID)

	// Verify adapter stored
	adapter, found := f.keeper.GetAdapter(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, "Helium Adapter", adapter.Name)
}

func TestMsgServer_RegisterAdapter_ModuleDisabled(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	_, err := ms.RegisterAdapter(f.ctx, &types.MsgRegisterAdapter{
		Owner:           testAddr1,
		Name:            "Test",
		SchemaCID:       "Qm",
		OracleAllowlist: []string{testOracle1},
		NetworkType:     "custom",
	})
	require.Error(t, err)
}

func TestMsgServer_RegisterAdapter_MaxAdaptersExceeded(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.MaxAdapters = 1
	p.AdapterRegistrationFee = math.ZeroInt()
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Register first adapter
	_, err := ms.RegisterAdapter(f.ctx, &types.MsgRegisterAdapter{
		Owner:           testAddr1,
		Name:            "Adapter 1",
		SchemaCID:       "Qm1",
		OracleAllowlist: []string{testOracle1},
		NetworkType:     "helium",
		RewardShare:     math.LegacyNewDecWithPrec(80, 2),
	})
	require.NoError(t, err)

	// Second should fail
	_, err = ms.RegisterAdapter(f.ctx, &types.MsgRegisterAdapter{
		Owner:           testAddr2,
		Name:            "Adapter 2",
		SchemaCID:       "Qm2",
		OracleAllowlist: []string{testOracle2},
		NetworkType:     "hivemapper",
		RewardShare:     math.LegacyNewDecWithPrec(80, 2),
	})
	require.Error(t, err)
}

func TestMsgServer_SuspendAdapter_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))
	require.NoError(t, f.keeper.SetNextAdapterID(f.ctx, 2))

	// Owner can suspend
	resp, err := ms.SuspendAdapter(f.ctx, &types.MsgSuspendAdapter{Authority: testAddr1, AdapterID: 1})
	require.NoError(t, err)
	require.NotNil(t, resp)

	got, _ := f.keeper.GetAdapter(f.ctx, 1)
	require.Equal(t, types.AdapterStatusSuspended, got.Status)
}

func TestMsgServer_SuspendAdapter_NotOwner(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	_, err := ms.SuspendAdapter(f.ctx, &types.MsgSuspendAdapter{Authority: testAddr2, AdapterID: 1})
	require.Error(t, err)
}

func TestMsgServer_SubmitDePINContribution_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	resp, err := ms.SubmitDePINContribution(f.ctx, &types.MsgSubmitDePINContribution{
		Submitter:   testAddr1,
		AdapterID:   1,
		ExternalID:  "ext-1",
		Contributor: testAddr2,
		DataHash:    "abc123",
		DataURI:     "ipfs://data",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.PocContributionID > 0)
}

func TestMsgServer_SubmitOracleAttestation_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	resp, err := ms.SubmitOracleAttestation(f.ctx, &types.MsgSubmitOracleAttestation{
		OracleAddress:     testOracle1,
		AdapterID:         1,
		BatchID:           "batch-1",
		AttestationHash:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		ContributionCount: 5,
		Signature:         "sig123",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestMsgServer_SubmitOracleAttestation_NotAuthorized(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	_, err := ms.SubmitOracleAttestation(f.ctx, &types.MsgSubmitOracleAttestation{
		OracleAddress:     testAddr2, // not in allowlist
		AdapterID:         1,
		BatchID:           "batch-1",
		AttestationHash:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		ContributionCount: 5,
		Signature:         "sig123",
	})
	require.Error(t, err)
}

func TestMsgServer_UpdateAdapterConfig_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	resp, err := ms.UpdateAdapterConfig(f.ctx, &types.MsgUpdateAdapterConfig{
		Owner:           testAddr1,
		AdapterID:       1,
		SchemaCID:       "QmNew",
		OracleAllowlist: []string{testOracle1, testOracle2},
		RewardShare:     math.LegacyNewDecWithPrec(90, 2),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	got, _ := f.keeper.GetAdapter(f.ctx, 1)
	require.Equal(t, "QmNew", got.SchemaCID)
	require.Len(t, got.OracleAllowlist, 2)
}

func TestMsgServer_UpdateAdapterConfig_NotOwner(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	_, err := ms.UpdateAdapterConfig(f.ctx, &types.MsgUpdateAdapterConfig{
		Owner:     testAddr2, // not the owner
		AdapterID: 1,
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
	gs := types.DefaultGenesis()
	require.NoError(t, f.keeper.InitGenesis(f.ctx, *gs))

	// Add some state
	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.Len(t, exported.Adapters, 1)
}

// ========== Query Tests ==========

func TestQuery_Params(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.True(t, resp.Params.DefaultRewardShare.Equal(types.DefaultDefaultRewardShare))
}

func TestQuery_Adapter_Found(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	adapter := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, adapter))

	resp, err := qs.Adapter(f.ctx, &types.QueryAdapterRequest{AdapterID: 1})
	require.NoError(t, err)
	require.Equal(t, "Helium", resp.Adapter.Name)
}

func TestQuery_Adapter_NotFound(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.Adapter(f.ctx, &types.QueryAdapterRequest{AdapterID: 999})
	require.Error(t, err)
}

func TestQuery_AllAdapters(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	a1 := types.NewAdapter(1, "Helium", testAddr1, "Qm1", []string{testOracle1}, "helium", 100, math.LegacyNewDecWithPrec(80, 2), "")
	a2 := types.NewAdapter(2, "DIMO", testAddr2, "Qm2", []string{testOracle2}, "dimo", 100, math.LegacyNewDecWithPrec(80, 2), "")
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a1))
	require.NoError(t, f.keeper.SetAdapter(f.ctx, a2))

	resp, err := qs.AllAdapters(f.ctx, &types.QueryAllAdaptersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Adapters, 2)
}

func TestQuery_ContributionMapping_Found(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	cm := types.ContributionMapping{
		AdapterID:         1,
		ExternalID:        "ext-1",
		PocContributionID: 42,
		Contributor:       testAddr1,
		MappedAtHeight:    100,
		RewardAmount:      math.ZeroInt(),
	}
	require.NoError(t, f.keeper.SetContributionMapping(f.ctx, cm))

	resp, err := qs.ContributionMapping(f.ctx, &types.QueryContributionMappingRequest{AdapterID: 1, ExternalID: "ext-1"})
	require.NoError(t, err)
	require.Equal(t, uint64(42), resp.Mapping.PocContributionID)
}

func TestQuery_AdapterStats(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	stats := types.NewAdapterStats(1)
	stats.TotalSubmissions = 25
	require.NoError(t, f.keeper.SetAdapterStats(f.ctx, stats))

	resp, err := qs.AdapterStats(f.ctx, &types.QueryAdapterStatsRequest{AdapterID: 1})
	require.NoError(t, err)
	require.Equal(t, uint64(25), resp.Stats.TotalSubmissions)
}
