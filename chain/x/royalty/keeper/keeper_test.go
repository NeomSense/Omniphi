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

	"pos/x/royalty/keeper"
	"pos/x/royalty/types"
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
	balances map[string]sdk.Coins
	minted   sdk.Coins
	burned   sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]sdk.Coins),
		minted:   sdk.NewCoins(),
		burned:   sdk.NewCoins(),
	}
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	m.balances[recipientAddr.String()] = m.balances[recipientAddr.String()].Add(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoins(_ context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	m.balances[toAddr.String()] = m.balances[toAddr.String()].Add(amt...)
	return nil
}

func (m *mockBankKeeper) MintCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	m.minted = m.minted.Add(amt...)
	return nil
}

func (m *mockBankKeeper) BurnCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	m.burned = m.burned.Add(amt...)
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	for _, c := range m.balances[addr.String()] {
		if c.Denom == denom {
			return c
		}
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

// Mock account keeper
type mockAccountKeeper struct{}

func (m mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return sdk.AccAddress(moduleName)
}

func (m mockAccountKeeper) GetModuleAccount(_ context.Context, moduleName string) sdk.ModuleAccountI {
	return nil
}

// Mock PoC keeper
type mockPocKeeper struct {
	credits              map[string]math.Int
	rewardedContribs     map[uint64]bool
	pendingRewardsAmount map[string]math.Int
}

func newMockPocKeeper() *mockPocKeeper {
	return &mockPocKeeper{
		credits:              make(map[string]math.Int),
		rewardedContribs:     make(map[uint64]bool),
		pendingRewardsAmount: make(map[string]math.Int),
	}
}

func (m *mockPocKeeper) GetCreditAmount(_ context.Context, addr string) math.Int {
	if amt, ok := m.credits[addr]; ok {
		return amt
	}
	return math.ZeroInt()
}

func (m *mockPocKeeper) IsContributionRewarded(_ context.Context, id uint64) bool {
	return m.rewardedContribs[id]
}

func (m *mockPocKeeper) GetPendingRewardsAmount(_ context.Context, addr string) math.Int {
	if amt, ok := m.pendingRewardsAmount[addr]; ok {
		return amt
	}
	return math.ZeroInt()
}

// Test addresses
var (
	testAddr1 = sdk.AccAddress("addr1_______________").String()
	testAddr2 = sdk.AccAddress("addr2_______________").String()
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
	require.True(t, params.MinRoyaltyShare.Equal(types.DefaultMinRoyaltyShare))
}

func TestParams_SetAndGet(t *testing.T) {
	f := setupTest(t)
	p := types.DefaultParams()
	p.MaxRoyaltyShare = math.LegacyNewDecWithPrec(75, 2) // 0.75
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	got := f.keeper.GetParams(f.ctx)
	require.True(t, got.MaxRoyaltyShare.Equal(math.LegacyNewDecWithPrec(75, 2)))
}

// ========== RoyaltyToken CRUD Tests ==========

func TestRoyaltyToken_SetAndGet(t *testing.T) {
	f := setupTest(t)
	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))

	got, found := f.keeper.GetRoyaltyToken(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, uint64(1), got.TokenID)
	require.Equal(t, testAddr1, got.Owner)
}

func TestRoyaltyToken_NotFound(t *testing.T) {
	f := setupTest(t)
	_, found := f.keeper.GetRoyaltyToken(f.ctx, 999)
	require.False(t, found)
}

func TestRoyaltyToken_GetByOwner(t *testing.T) {
	f := setupTest(t)
	t1 := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	t2 := types.NewRoyaltyToken(2, 101, testAddr1, math.LegacyNewDecWithPrec(20, 2), 50)
	t3 := types.NewRoyaltyToken(3, 102, testAddr2, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t1))
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t2))
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t3))

	owner1Tokens := f.keeper.GetTokensByOwner(f.ctx, testAddr1)
	require.Len(t, owner1Tokens, 2)
}

func TestRoyaltyToken_GetByClaim(t *testing.T) {
	f := setupTest(t)
	t1 := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	t2 := types.NewRoyaltyToken(2, 100, testAddr2, math.LegacyNewDecWithPrec(20, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t1))
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t2))

	claimTokens := f.keeper.GetTokensByClaim(f.ctx, 100)
	require.Len(t, claimTokens, 2)
}

// ========== NextTokenID Tests ==========

func TestNextTokenID(t *testing.T) {
	f := setupTest(t)
	id := f.keeper.GetNextTokenID(f.ctx)
	require.Equal(t, uint64(1), id)

	require.NoError(t, f.keeper.SetNextTokenID(f.ctx, 5))
	require.Equal(t, uint64(5), f.keeper.GetNextTokenID(f.ctx))
}

// ========== AccumulatedRoyalty Tests ==========

func TestAccumulatedRoyalty_SetAndGet(t *testing.T) {
	f := setupTest(t)
	require.NoError(t, f.keeper.SetAccumulatedRoyalty(f.ctx, 1, math.NewInt(500)))

	amt := f.keeper.GetAccumulatedRoyalty(f.ctx, 1)
	require.True(t, amt.Equal(math.NewInt(500)))
}

func TestAccumulatedRoyalty_DefaultZero(t *testing.T) {
	f := setupTest(t)
	amt := f.keeper.GetAccumulatedRoyalty(f.ctx, 999)
	require.True(t, amt.IsZero())
}

// ========== Listing CRUD Tests ==========

func TestListing_SetAndGet(t *testing.T) {
	f := setupTest(t)
	listing := types.Listing{
		TokenID:  1,
		Seller:   testAddr1,
		AskPrice: math.NewInt(1000),
		Denom:    "uomni",
		ListedAt: 100,
	}
	require.NoError(t, f.keeper.SetListing(f.ctx, listing))

	got, found := f.keeper.GetListing(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, testAddr1, got.Seller)
}

func TestListing_Delete(t *testing.T) {
	f := setupTest(t)
	listing := types.Listing{TokenID: 1, Seller: testAddr1, AskPrice: math.NewInt(1000), Denom: "uomni", ListedAt: 100}
	require.NoError(t, f.keeper.SetListing(f.ctx, listing))
	require.NoError(t, f.keeper.DeleteListing(f.ctx, 1))

	_, found := f.keeper.GetListing(f.ctx, 1)
	require.False(t, found)
}

// ========== MsgServer Tests ==========

func TestMsgServer_UpdateParams_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.MaxFractions = 200
	resp, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{Authority: testAuth, Params: p})
	require.NoError(t, err)
	require.NotNil(t, resp)

	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, int64(200), got.MaxFractions)
}

func TestMsgServer_UpdateParams_InvalidAuthority(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{Authority: testAddr1, Params: types.DefaultParams()})
	require.Error(t, err)
}

func TestMsgServer_TokenizeRoyalty_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Mock: contribution 100 is rewarded
	f.pocKeeper.rewardedContribs[100] = true

	resp, err := ms.TokenizeRoyalty(f.ctx, &types.MsgTokenizeRoyalty{
		Creator:      testAddr1,
		ClaimID:      100,
		RoyaltyShare: math.LegacyNewDecWithPrec(10, 2), // 10%
		Metadata:     "test token",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, uint64(1), resp.TokenID)

	// Verify token was stored
	token, found := f.keeper.GetRoyaltyToken(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, testAddr1, token.Owner)
}

func TestMsgServer_TokenizeRoyalty_ModuleDisabled(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = false
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	_, err := ms.TokenizeRoyalty(f.ctx, &types.MsgTokenizeRoyalty{
		Creator:      testAddr1,
		ClaimID:      100,
		RoyaltyShare: math.LegacyNewDecWithPrec(10, 2),
	})
	require.Error(t, err)
}

func TestMsgServer_TokenizeRoyalty_InvalidShare(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))
	f.pocKeeper.rewardedContribs[100] = true

	// Share too high (60% > max 50%)
	_, err := ms.TokenizeRoyalty(f.ctx, &types.MsgTokenizeRoyalty{
		Creator:      testAddr1,
		ClaimID:      100,
		RoyaltyShare: math.LegacyNewDecWithPrec(60, 2),
	})
	require.Error(t, err)
}

func TestMsgServer_TransferToken_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	p.TransferEnabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	// Create a token
	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))
	require.NoError(t, f.keeper.SetNextTokenID(f.ctx, 2))

	resp, err := ms.TransferToken(f.ctx, &types.MsgTransferToken{
		Sender:    testAddr1,
		Recipient: testAddr2,
		TokenID:   1,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify ownership changed
	got, _ := f.keeper.GetRoyaltyToken(f.ctx, 1)
	require.Equal(t, testAddr2, got.Owner)
}

func TestMsgServer_TransferToken_NotOwner(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.TransferEnabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))

	_, err := ms.TransferToken(f.ctx, &types.MsgTransferToken{
		Sender:    testAddr2, // not the owner
		Recipient: testAddr2,
		TokenID:   1,
	})
	require.Error(t, err)
}

func TestMsgServer_ClaimRoyalties_Valid(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	p := types.DefaultParams()
	p.Enabled = true
	require.NoError(t, f.keeper.SetParams(f.ctx, p))

	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))
	require.NoError(t, f.keeper.SetAccumulatedRoyalty(f.ctx, 1, math.NewInt(500)))

	resp, err := ms.ClaimRoyalties(f.ctx, &types.MsgClaimRoyalties{
		Owner:   testAddr1,
		TokenID: 1,
	})
	require.NoError(t, err)
	require.True(t, resp.Amount.Equal(math.NewInt(500)))

	// Accumulated should be reset
	amt := f.keeper.GetAccumulatedRoyalty(f.ctx, 1)
	require.True(t, amt.IsZero())
}

func TestMsgServer_ClaimRoyalties_NoAccumulated(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))

	_, err := ms.ClaimRoyalties(f.ctx, &types.MsgClaimRoyalties{
		Owner:   testAddr1,
		TokenID: 1,
	})
	require.Error(t, err)
}

func TestMsgServer_DelistToken(t *testing.T) {
	f := setupTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	listing := types.Listing{TokenID: 1, Seller: testAddr1, AskPrice: math.NewInt(1000), Denom: "uomni", ListedAt: 100}
	require.NoError(t, f.keeper.SetListing(f.ctx, listing))

	resp, err := ms.DelistToken(f.ctx, &types.MsgDelistToken{Seller: testAddr1, TokenID: 1})
	require.NoError(t, err)
	require.NotNil(t, resp)

	_, found := f.keeper.GetListing(f.ctx, 1)
	require.False(t, found)
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
	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))

	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.Len(t, exported.Tokens, 1)
}

// ========== Query Tests ==========

func TestQuery_Params(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.True(t, resp.Params.MinRoyaltyShare.Equal(types.DefaultMinRoyaltyShare))
}

func TestQuery_Token_Found(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	token := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, token))

	resp, err := qs.RoyaltyToken(f.ctx, &types.QueryRoyaltyTokenRequest{TokenID: 1})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.Token.TokenID)
}

func TestQuery_Token_NotFound(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.RoyaltyToken(f.ctx, &types.QueryRoyaltyTokenRequest{TokenID: 999})
	require.Error(t, err)
}

func TestQuery_TokensByOwner(t *testing.T) {
	f := setupTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	t1 := types.NewRoyaltyToken(1, 100, testAddr1, math.LegacyNewDecWithPrec(10, 2), 50)
	t2 := types.NewRoyaltyToken(2, 101, testAddr1, math.LegacyNewDecWithPrec(20, 2), 50)
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t1))
	require.NoError(t, f.keeper.SetRoyaltyToken(f.ctx, t2))

	resp, err := qs.TokensByOwner(f.ctx, &types.QueryTokensByOwnerRequest{Owner: testAddr1})
	require.NoError(t, err)
	require.Len(t, resp.Tokens, 2)
}
