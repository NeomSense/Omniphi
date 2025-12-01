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

type KeeperTestFixture struct {
	ctx    sdk.Context
	keeper keeper.Keeper
	cdc    codec.Codec
}

// Mock interfaces for testing
type mockAccountKeeper struct{}

func (m mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return sdk.AccAddress("module_address____")
}

func (m mockAccountKeeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	return nil
}

type mockBankKeeper struct{}

func (m mockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m mockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

type mockStakingKeeper struct{}

func (m mockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
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

func (m mockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	return []stakingtypes.Validator{}, nil
}

func (m mockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return math.NewInt(1000000000000), nil
}

func (m mockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return math.NewInt(1000000)
}

func SetupKeeperTest(t *testing.T) *KeeperTestFixture {
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
	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())

	// Create authority address
	authority := sdk.AccAddress("gov_______________").String()

	// Create PoC keeper with mocks (order: storeService, tStoreKey, logger, authority, staking, bank, account)
	pocKeeper := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		tStoreKey,
		log.NewNopLogger(),
		authority,
		mockStakingKeeper{},
		mockBankKeeper{},
		mockAccountKeeper{},
	)

	// Initialize PoC params
	params := types.DefaultParams()
	params.RewardDenom = "stake"
	params.BaseRewardUnit = math.NewInt(100)
	params.MaxPerBlock = 100
	err := pocKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	return &KeeperTestFixture{
		ctx:    ctx,
		keeper: pocKeeper,
		cdc:    cdc,
	}
}
