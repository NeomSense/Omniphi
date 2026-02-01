package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// Test fixture for emissions tests
type emissionsTestSuite struct {
	ctx            context.Context
	keeper         keeper.Keeper
	accountKeeper  *mockAccountKeeperWithBalance
	bankKeeper     *mockBankKeeperWithMinting
	stakingKeeper  *mockStakingKeeperFull
	cdc            codec.Codec
}

// Mock account keeper
type mockAccountKeeperWithBalance struct {
	moduleAddresses map[string]sdk.AccAddress
}

func (m *mockAccountKeeperWithBalance) GetModuleAddress(moduleName string) sdk.AccAddress {
	if addr, ok := m.moduleAddresses[moduleName]; ok {
		return addr
	}
	addr := sdk.AccAddress(moduleName + "_addr")
	m.moduleAddresses[moduleName] = addr
	return addr
}

func (m *mockAccountKeeperWithBalance) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	return nil
}

// Mock bank keeper with minting support
type mockBankKeeperWithMinting struct {
	balances map[string]sdk.Coins
	supply   sdk.Coins
}

func newMockBankKeeper() *mockBankKeeperWithMinting {
	return &mockBankKeeperWithMinting{
		balances: make(map[string]sdk.Coins),
		supply:   sdk.NewCoins(),
	}
}

func (m *mockBankKeeperWithMinting) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if bal, ok := m.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, bal.AmountOf(denom))
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *mockBankKeeperWithMinting) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := sdk.AccAddress(moduleName + "_addr").String()
	if bal, ok := m.balances[moduleAddr]; ok {
		m.balances[moduleAddr] = bal.Add(amt...)
	} else {
		m.balances[moduleAddr] = amt
	}
	m.supply = m.supply.Add(amt...)
	return nil
}

func (m *mockBankKeeperWithMinting) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := sdk.AccAddress(moduleName + "_addr").String()
	bal := m.balances[moduleAddr]
	m.balances[moduleAddr] = bal.Sub(amt...)
	m.supply = m.supply.Sub(amt...)
	return nil
}

func (m *mockBankKeeperWithMinting) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	senderAddr := sdk.AccAddress(senderModule + "_addr").String()

	// Deduct from module
	senderBal := m.balances[senderAddr]
	m.balances[senderAddr] = senderBal.Sub(amt...)

	// Add to recipient
	recipientBal := m.balances[recipientAddr.String()]
	m.balances[recipientAddr.String()] = recipientBal.Add(amt...)

	return nil
}

func (m *mockBankKeeperWithMinting) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	recipientAddr := sdk.AccAddress(recipientModule + "_addr").String()

	// Deduct from sender
	senderBal := m.balances[senderAddr.String()]
	m.balances[senderAddr.String()] = senderBal.Sub(amt...)

	// Add to module
	recipientBal := m.balances[recipientAddr]
	m.balances[recipientAddr] = recipientBal.Add(amt...)

	return nil
}

// Mock staking keeper
type mockStakingKeeperFull struct{}

func (m *mockStakingKeeperFull) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	// Return a valid validator with some power for testing
	val := stakingtypes.Validator{
		OperatorAddress: addr.String(),
		Jailed:          false,
		Status:          stakingtypes.Bonded,
		Tokens:          math.NewInt(100000000), // 100 tokens
		DelegatorShares: math.LegacyNewDec(100000000),
	}
	return val, nil
}

func (m *mockStakingKeeperFull) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	return []stakingtypes.Validator{}, nil
}

func (m *mockStakingKeeperFull) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return math.NewInt(1000000), nil
}

func (m *mockStakingKeeperFull) PowerReduction(ctx context.Context) math.Int {
	return math.NewInt(1000000)
}

func setupKeeperTest(t *testing.T) *emissionsTestSuite {
	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec

	// Register interfaces
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)

	// Create store keys
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	tStoreKey := storetypes.NewTransientStoreKey(types.TStoreKey)

	// Create in-memory database
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(tStoreKey, storetypes.StoreTypeTransient, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	// Create context
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())

	// Create authority address
	authority := sdk.AccAddress("gov_______________").String()

	// Create mock keepers
	accountKeeper := &mockAccountKeeperWithBalance{
		moduleAddresses: make(map[string]sdk.AccAddress),
	}
	bankKeeper := newMockBankKeeper()
	stakingKeeper := &mockStakingKeeperFull{}

	// Create PoC keeper
	pocKeeper := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		tStoreKey,
		log.NewNopLogger(),
		authority,
		stakingKeeper,
		bankKeeper,
		accountKeeper,
	)

	// Initialize PoC params
	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.BaseRewardUnit = math.NewInt(100)
	params.MaxPerBlock = 100
	err := pocKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	return &emissionsTestSuite{
		ctx:            ctx,
		keeper:         pocKeeper,
		accountKeeper:  accountKeeper,
		bankKeeper:     bankKeeper,
		stakingKeeper:  stakingKeeper,
		cdc:            cdc,
	}
}

func TestDistributeEmissions(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Distribute 1000 OMNI emissions
	emissions := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(1000_000000)))
	err := suite.keeper.DistributeEmissions(ctx, emissions)
	require.NoError(t, err, "distribute emissions should succeed")

	// Check module balance equals emission amount (started at zero)
	moduleAddr := suite.accountKeeper.GetModuleAddress(types.ModuleName)
	balanceAfter := suite.bankKeeper.GetBalance(ctx, moduleAddr, "omniphi")
	require.Equal(t, emissions[0].Amount, balanceAfter.Amount,
		"module balance should equal emission amount")
}

func TestDistributeEmissions_WrongDenom(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Try to distribute wrong denom
	emissions := sdk.NewCoins(sdk.NewCoin("wrongdenom", math.NewInt(1000_000000)))
	err := suite.keeper.DistributeEmissions(ctx, emissions)
	require.Error(t, err, "should error on wrong denom")
	require.Contains(t, err.Error(), "invalid emission denomination")
}

func TestProcessPendingRewards_NoContributions(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Process rewards with no contributions
	err := suite.keeper.ProcessPendingRewards(ctx)
	require.NoError(t, err, "should handle no contributions gracefully")
}

func TestProcessPendingRewards_SingleContribution(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Fund PoC module with 1000 OMNI
	_ = suite.accountKeeper.GetModuleAddress(types.ModuleName)
	fundAmount := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(1000_000000)))
	err := suite.bankKeeper.MintCoins(ctx, types.ModuleName, fundAmount)
	require.NoError(t, err)

	// Create and verify a contribution
	contributor := sdk.AccAddress([]byte("contributor1"))
	contribution := types.Contribution{
		Id:          1,
		Contributor: contributor.String(),
		Ctype:       "code",
		Uri:         "https://github.com/example/pr/1",
		Hash:        []byte("hash1"),
		Verified:    true,
		Rewarded:    false,
		BlockHeight: 100,
		BlockTime:   12345,
	}
	err = suite.keeper.SetContribution(ctx, contribution)
	require.NoError(t, err)

	// Get contributor balance before
	balanceBefore := suite.bankKeeper.GetBalance(ctx, contributor, "omniphi")

	// Process rewards
	err = suite.keeper.ProcessPendingRewards(ctx)
	require.NoError(t, err)

	// Check contributor received reward
	balanceAfter := suite.bankKeeper.GetBalance(ctx, contributor, "omniphi")
	require.True(t, balanceAfter.Amount.GT(balanceBefore.Amount),
		"contributor should receive reward")

	// Check contribution marked as rewarded
	c, found := suite.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.True(t, c.Rewarded, "contribution should be marked as rewarded")
}

func TestProcessPendingRewards_MultipleContributions(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Fund PoC module with 10,000 OMNI
	fundAmount := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(10000_000000)))
	err := suite.bankKeeper.MintCoins(ctx, types.ModuleName, fundAmount)
	require.NoError(t, err)

	// Create 3 verified contributions
	contributor1 := sdk.AccAddress([]byte("contributor1"))
	contributor2 := sdk.AccAddress([]byte("contributor2"))
	contributor3 := sdk.AccAddress([]byte("contributor3"))

	contributions := []types.Contribution{
		{
			Id:          1,
			Contributor: contributor1.String(),
			Ctype:       "code",
			Verified:    true,
			Rewarded:    false,
		},
		{
			Id:          2,
			Contributor: contributor2.String(),
			Ctype:       "code",
			Verified:    true,
			Rewarded:    false,
		},
		{
			Id:          3,
			Contributor: contributor3.String(),
			Ctype:       "code",
			Verified:    true,
			Rewarded:    false,
		},
	}

	for _, c := range contributions {
		err = suite.keeper.SetContribution(ctx, c)
		require.NoError(t, err)
	}

	// Process rewards
	err = suite.keeper.ProcessPendingRewards(ctx)
	require.NoError(t, err)

	// Each contributor should receive ~3,333 OMNI (equal weights)
	balance1 := suite.bankKeeper.GetBalance(ctx, contributor1, "omniphi")
	balance2 := suite.bankKeeper.GetBalance(ctx, contributor2, "omniphi")
	balance3 := suite.bankKeeper.GetBalance(ctx, contributor3, "omniphi")

	// All should be positive
	require.True(t, balance1.Amount.IsPositive())
	require.True(t, balance2.Amount.IsPositive())
	require.True(t, balance3.Amount.IsPositive())

	// All should be approximately equal (within rounding)
	diff12 := balance1.Amount.Sub(balance2.Amount).Abs()
	diff23 := balance2.Amount.Sub(balance3.Amount).Abs()
	require.True(t, diff12.LTE(math.NewInt(1)), "balances should be nearly equal")
	require.True(t, diff23.LTE(math.NewInt(1)), "balances should be nearly equal")

	// Total distributed should equal fund amount
	total := balance1.Amount.Add(balance2.Amount).Add(balance3.Amount)
	require.Equal(t, fundAmount[0].Amount, total, "total distributed should equal fund amount")

	// All contributions should be marked as rewarded
	for i := 1; i <= 3; i++ {
		c, found := suite.keeper.GetContribution(ctx, uint64(i))
		require.True(t, found)
		require.True(t, c.Rewarded, "contribution %d should be marked as rewarded", i)
	}
}

func TestProcessPendingRewards_SkipsUnverified(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Fund PoC module
	fundAmount := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(1000_000000)))
	err := suite.bankKeeper.MintCoins(ctx, types.ModuleName, fundAmount)
	require.NoError(t, err)

	// Create unverified contribution
	contributor := sdk.AccAddress([]byte("contributor1"))
	contribution := types.Contribution{
		Id:          1,
		Contributor: contributor.String(),
		Verified:    false, // NOT VERIFIED
		Rewarded:    false,
	}
	err = suite.keeper.SetContribution(ctx, contribution)
	require.NoError(t, err)

	// Process rewards
	err = suite.keeper.ProcessPendingRewards(ctx)
	require.NoError(t, err)

	// Contributor should NOT receive reward
	balance := suite.bankKeeper.GetBalance(ctx, contributor, "omniphi")
	require.True(t, balance.Amount.IsZero(), "unverified contribution should not be rewarded")

	// Contribution should NOT be marked as rewarded
	c, found := suite.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.False(t, c.Rewarded, "unverified contribution should not be marked as rewarded")
}

func TestProcessPendingRewards_SkipsAlreadyRewarded(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Fund PoC module
	fundAmount := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(1000_000000)))
	err := suite.bankKeeper.MintCoins(ctx, types.ModuleName, fundAmount)
	require.NoError(t, err)

	// Create already-rewarded contribution
	contributor := sdk.AccAddress([]byte("contributor1"))
	contribution := types.Contribution{
		Id:          1,
		Contributor: contributor.String(),
		Verified:    true,
		Rewarded:    true, // ALREADY REWARDED
	}
	err = suite.keeper.SetContribution(ctx, contribution)
	require.NoError(t, err)

	balanceBefore := suite.bankKeeper.GetBalance(ctx, contributor, "omniphi")

	// Process rewards
	err = suite.keeper.ProcessPendingRewards(ctx)
	require.NoError(t, err)

	// Contributor should NOT receive additional reward
	balanceAfter := suite.bankKeeper.GetBalance(ctx, contributor, "omniphi")
	require.Equal(t, balanceBefore, balanceAfter, "already rewarded contribution should not be paid again")
}

func TestGetPendingRewardsAmount(t *testing.T) {
	suite := setupKeeperTest(t)
	ctx := suite.ctx

	// Fund PoC module with 3000 OMNI
	fundAmount := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(3000_000000)))
	err := suite.bankKeeper.MintCoins(ctx, types.ModuleName, fundAmount)
	require.NoError(t, err)

	// Create 3 pending contributions (2 from contributor1, 1 from contributor2)
	contributor1 := sdk.AccAddress([]byte("contributor1"))
	contributor2 := sdk.AccAddress([]byte("contributor2"))

	contributions := []types.Contribution{
		{Id: 1, Contributor: contributor1.String(), Verified: true, Rewarded: false},
		{Id: 2, Contributor: contributor1.String(), Verified: true, Rewarded: false},
		{Id: 3, Contributor: contributor2.String(), Verified: true, Rewarded: false},
	}

	for _, c := range contributions {
		err = suite.keeper.SetContribution(ctx, c)
		require.NoError(t, err)
	}

	// Get pending rewards
	pending1 := suite.keeper.GetPendingRewardsAmount(ctx, contributor1)
	pending2 := suite.keeper.GetPendingRewardsAmount(ctx, contributor2)

	// Contributor1 should have ~2000 OMNI pending (2/3 of total)
	// Contributor2 should have ~1000 OMNI pending (1/3 of total)
	require.True(t, pending1.IsPositive())
	require.True(t, pending2.IsPositive())

	// Check ratio (should be 2:1)
	ratio := pending1.Quo(pending2)
	require.Equal(t, int64(2), ratio.Int64(), "pending rewards should be proportional to contribution count")

	// Total pending should equal module balance
	total := pending1.Add(pending2)
	require.Equal(t, fundAmount[0].Amount, total, "total pending should equal module balance")
}
