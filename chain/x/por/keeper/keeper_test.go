package keeper_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	"pos/x/por/keeper"
	"pos/x/por/types"
)

type KeeperTestFixture struct {
	ctx           sdk.Context
	keeper        keeper.Keeper
	cdc           codec.Codec
	stakingKeeper *mockStakingKeeper
	bankKeeper    *mockBankKeeper
	pocKeeper     *mockPocKeeper
}

// Mock interfaces for testing

type mockAccountKeeper struct{}

func (m mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return sdk.AccAddress("module_address______")
}

func (m mockAccountKeeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	return nil
}

// mockBankKeeper tracks balances and transfers for testing bond logic
type mockBankKeeper struct {
	balances       map[string]sdk.Coins
	moduleBalances map[string]sdk.Coins
	burned         sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances:       make(map[string]sdk.Coins),
		moduleBalances: make(map[string]sdk.Coins),
		burned:         sdk.NewCoins(),
	}
}

func (m *mockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	coins, ok := m.balances[addr.String()]
	if !ok {
		return sdk.NewCoin(denom, math.ZeroInt())
	}
	return sdk.NewCoin(denom, coins.AmountOf(denom))
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	key := recipientAddr.String()
	existing, ok := m.balances[key]
	if !ok {
		existing = sdk.NewCoins()
	}
	m.balances[key] = existing.Add(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	key := senderAddr.String()
	existing, ok := m.balances[key]
	if !ok {
		return fmt.Errorf("insufficient balance")
	}
	for _, coin := range amt {
		if existing.AmountOf(coin.Denom).LT(coin.Amount) {
			return fmt.Errorf("insufficient balance for %s", coin.Denom)
		}
	}
	m.balances[key], _ = existing.SafeSub(amt...)

	modKey := recipientModule
	modExisting, ok := m.moduleBalances[modKey]
	if !ok {
		modExisting = sdk.NewCoins()
	}
	m.moduleBalances[modKey] = modExisting.Add(amt...)
	return nil
}

func (m *mockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	m.burned = m.burned.Add(amt...)
	return nil
}

// mockStakingKeeper is a stateful mock that tracks registered validators
type mockStakingKeeper struct {
	validators map[string]stakingtypes.Validator // valAddr.String() -> Validator
}

func newMockStakingKeeper() *mockStakingKeeper {
	return &mockStakingKeeper{validators: make(map[string]stakingtypes.Validator)}
}

// RegisterValidator adds a validator to the mock for testing
func (m *mockStakingKeeper) RegisterValidator(accAddr sdk.AccAddress, tokens math.Int, status stakingtypes.BondStatus) {
	valAddr := sdk.ValAddress(accAddr)
	m.validators[valAddr.String()] = stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		Jailed:          false,
		Status:          status,
		Tokens:          tokens,
		DelegatorShares: math.LegacyNewDecFromInt(tokens),
	}
}

func (m *mockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	val, ok := m.validators[addr.String()]
	if !ok {
		return stakingtypes.Validator{}, fmt.Errorf("validator not found: %s", addr.String())
	}
	return val, nil
}

func (m *mockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	vals := make([]stakingtypes.Validator, 0, len(m.validators))
	for _, v := range m.validators {
		vals = append(vals, v)
	}
	return vals, nil
}

func (m *mockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	total := math.ZeroInt()
	for _, v := range m.validators {
		if v.Status == stakingtypes.Bonded {
			total = total.Add(v.Tokens)
		}
	}
	return total, nil
}

func (m *mockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return math.NewInt(1000000)
}

type mockPocKeeper struct {
	credits map[string]math.Int
}

func newMockPocKeeper() *mockPocKeeper {
	return &mockPocKeeper{credits: make(map[string]math.Int)}
}

func (m *mockPocKeeper) AddCreditsWithOverflowCheck(ctx context.Context, addr sdk.AccAddress, amount math.Int) error {
	key := addr.String()
	existing, ok := m.credits[key]
	if !ok {
		existing = math.ZeroInt()
	}
	m.credits[key] = existing.Add(amount)
	return nil
}

func SetupKeeperTest(t *testing.T) *KeeperTestFixture {
	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec

	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	tStoreKey := storetypes.NewTransientStoreKey(types.TStoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(tStoreKey, storetypes.StoreTypeTransient, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, cmtproto.Header{
		Time: time.Now(),
	}, false, log.NewNopLogger())

	authority := sdk.AccAddress("gov_________________").String()

	sk := newMockStakingKeeper()
	bk := newMockBankKeeper()

	porKeeper := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		tStoreKey,
		log.NewNopLogger(),
		authority,
		sk,
		bk,
		mockAccountKeeper{},
	)

	// Set PoC keeper for integration testing
	pocKeeper := newMockPocKeeper()
	porKeeper.SetPocKeeper(pocKeeper)

	params := types.DefaultParams()
	err := porKeeper.SetParams(ctx, params)
	require.NoError(t, err)

	return &KeeperTestFixture{
		ctx:           ctx,
		keeper:        porKeeper,
		cdc:           cdc,
		stakingKeeper: sk,
		bankKeeper:    bk,
		pocKeeper:     pocKeeper,
	}
}
