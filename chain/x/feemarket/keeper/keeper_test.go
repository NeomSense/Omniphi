package keeper_test

import (
	"context"
	"errors"
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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	"pos/x/feemarket/keeper"
	"pos/x/feemarket/types"
)

type testFixture struct {
	ctx           sdk.Context
	keeper        keeper.Keeper
	accountKeeper *mockAccountKeeper
	bankKeeper    *mockBankKeeper
}

// setupTest creates a test fixture with a keeper and mock dependencies
func setupTest(t *testing.T) *testFixture {
	t.Helper()

	// Create store key
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	// Create in-memory database
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())

	// Create codec
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	// Create mock keepers
	accountKeeper := newMockAccountKeeper()
	bankKeeper := newMockBankKeeper()

	// Create keeper
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		accountKeeper,
		bankKeeper,
		authority,
	)

	// Create context
	ctx := sdk.NewContext(cms, cmtproto.Header{Height: 1}, false, log.NewNopLogger())

	// Initialize with default genesis
	err := k.InitGenesis(ctx, *types.DefaultGenesisState())
	require.NoError(t, err)

	return &testFixture{
		ctx:           ctx,
		keeper:        k,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

// Mock AccountKeeper
type mockAccountKeeper struct {
	accounts map[string]sdk.AccountI
}

func newMockAccountKeeper() *mockAccountKeeper {
	return &mockAccountKeeper{
		accounts: make(map[string]sdk.AccountI),
	}
}

func (m *mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return authtypes.NewModuleAddress(moduleName)
}

func (m *mockAccountKeeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	addr := authtypes.NewModuleAddress(moduleName)
	if acc, ok := m.accounts[addr.String()]; ok {
		if modAcc, ok := acc.(sdk.ModuleAccountI); ok {
			return modAcc
		}
	}

	// Return a new module account
	baseAcc := authtypes.NewBaseAccountWithAddress(addr)
	modAcc := authtypes.NewModuleAccount(baseAcc, moduleName, authtypes.Burner)
	m.accounts[addr.String()] = modAcc
	return modAcc
}

// Mock BankKeeper
type mockBankKeeper struct {
	balances map[string]sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]sdk.Coins),
	}
}

func (m *mockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if coins, ok := m.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, coins.AmountOf(denom))
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *mockBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	if coins, ok := m.balances[addr.String()]; ok {
		return coins
	}
	return sdk.NewCoins()
}

func (m *mockBankKeeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	senderAddr := authtypes.NewModuleAddress(senderModule).String()
	recipientAddr := authtypes.NewModuleAddress(recipientModule).String()

	senderCoins := m.balances[senderAddr]
	if !senderCoins.IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}

	m.balances[senderAddr] = senderCoins.Sub(amt...)
	recipientCoins := m.balances[recipientAddr]
	m.balances[recipientAddr] = recipientCoins.Add(amt...)

	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	senderAddr := authtypes.NewModuleAddress(senderModule).String()

	senderCoins := m.balances[senderAddr]
	if !senderCoins.IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}

	m.balances[senderAddr] = senderCoins.Sub(amt...)
	recipientCoins := m.balances[recipientAddr.String()]
	m.balances[recipientAddr.String()] = recipientCoins.Add(amt...)

	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	recipientAddr := authtypes.NewModuleAddress(recipientModule).String()

	senderCoins := m.balances[senderAddr.String()]
	if !senderCoins.IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}

	m.balances[senderAddr.String()] = senderCoins.Sub(amt...)
	recipientCoins := m.balances[recipientAddr]
	m.balances[recipientAddr] = recipientCoins.Add(amt...)

	return nil
}

func (m *mockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(moduleName).String()

	moduleCoins := m.balances[moduleAddr]
	if !moduleCoins.IsAllGTE(amt) {
		return errors.New("insufficient funds")
	}

	m.balances[moduleAddr] = moduleCoins.Sub(amt...)
	return nil
}

func (m *mockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(moduleName).String()
	moduleCoins := m.balances[moduleAddr]
	m.balances[moduleAddr] = moduleCoins.Add(amt...)
	return nil
}

func (m *mockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	return m.GetAllBalances(ctx, addr)
}

// Helper to fund an account
func (m *mockBankKeeper) fundAccount(addr sdk.AccAddress, coins sdk.Coins) {
	m.balances[addr.String()] = coins
}

// Test helper functions

func requireDecEqual(t *testing.T, expected, actual math.LegacyDec, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected.String(), actual.String(), msgAndArgs...)
}

func requireDecInRange(t *testing.T, value, min, max math.LegacyDec, msgAndArgs ...interface{}) {
	t.Helper()
	require.True(t, value.GTE(min) && value.LTE(max),
		"Expected %s to be between %s and %s. %v", value, min, max, msgAndArgs)
}
