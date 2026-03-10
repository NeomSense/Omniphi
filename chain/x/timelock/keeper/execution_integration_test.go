package keeper

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"pos/x/timelock/types"
)

type stubGuardKeeper struct {
	integrationEnabled bool
	executed           map[uint64]bool
}

func (s stubGuardKeeper) OnTimelockQueued(ctx context.Context, proposalID uint64) error {
	return nil
}

func (s stubGuardKeeper) IsTimelockIntegrationEnabled(ctx context.Context) bool {
	return s.integrationEnabled
}

func (s stubGuardKeeper) HasExecutionMarker(ctx context.Context, proposalID uint64) bool {
	return s.executed[proposalID]
}

type testRouter struct {
	storeKey *storetypes.KVStoreKey
}

func (r testRouter) handler() baseapp.MsgServiceHandler {
	return func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		store := ctx.KVStore(r.storeKey)
		key := []byte("counter")

		if send, ok := req.(*banktypes.MsgSend); ok {
			if len(send.Amount) > 0 && send.Amount[0].Denom == "fail" {
				return nil, errors.New("forced failure")
			}
		}

		var current uint64
		if bz := store.Get(key); bz != nil && len(bz) == 8 {
			current = binary.BigEndian.Uint64(bz)
		}
		current++
		bz := make([]byte, 8)
		binary.BigEndian.PutUint64(bz, current)
		store.Set(key, bz)

		return &sdk.Result{}, nil
	}
}

func (r testRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler {
	return r.handler()
}

func (r testRouter) HandlerByTypeURL(typeURL string) baseapp.MsgServiceHandler {
	return r.handler()
}

func setupTimelockKeeper(t *testing.T, routerFactory func(*storetypes.KVStoreKey) baseapp.MessageRouter) (Keeper, sdk.Context, *storetypes.KVStoreKey) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())

	timelockKey := storetypes.NewKVStoreKey(types.StoreKey)
	testKey := storetypes.NewKVStoreKey("test")

	stateStore.MountStoreWithDB(timelockKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(testKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	ctx := sdk.NewContext(stateStore, tmproto.Header{Height: 1, Time: time.Now()}, false, log.NewNopLogger())

	storeService := runtime.NewKVStoreService(timelockKey)
	authority := sdk.AccAddress("authority_________").String()

	router := routerFactory(testKey)
	k := NewKeeper(cdc, storeService, log.NewNopLogger(), authority, router)
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	require.NoError(t, k.InitDefaultTracks(ctx))

	return *k, ctx, testKey
}

func TestTimelock_AutoExecuteDisabledWhenGuardIntegrationEnabled(t *testing.T) {
	keeper, ctx, testKey := setupTimelockKeeper(t, func(testKey *storetypes.KVStoreKey) baseapp.MessageRouter {
		return testRouter{storeKey: testKey}
	})

	guard := stubGuardKeeper{
		integrationEnabled: true,
		executed:           map[uint64]bool{},
	}
	keeper.SetGuardKeeper(guard)

	msg := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress("from_______________").String(),
		ToAddress:   sdk.AccAddress("to________________").String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("upos", 1)),
	}

	op, err := types.NewQueuedOperation(1, 1, []sdk.Msg{msg}, keeper.GetAuthority(), ctx.BlockTime(), 0, 3600, keeper.cdc)
	require.NoError(t, err)
	require.NoError(t, keeper.SetOperation(ctx, op))

	err = keeper.AutoExecuteReadyOperations(ctx)
	require.NoError(t, err)

	// Ensure operation still queued
	stored, err := keeper.GetOperation(ctx, op.Id)
	require.NoError(t, err)
	require.True(t, stored.IsQueued())

	// Ensure no side effects
	store := ctx.KVStore(testKey)
	require.Nil(t, store.Get([]byte("counter")))
}

func TestTimelock_ExecuteDisabledWhenGuardIntegrationEnabled(t *testing.T) {
	keeper, ctx, _ := setupTimelockKeeper(t, func(testKey *storetypes.KVStoreKey) baseapp.MessageRouter {
		return testRouter{storeKey: testKey}
	})

	guard := stubGuardKeeper{
		integrationEnabled: true,
		executed:           map[uint64]bool{},
	}
	keeper.SetGuardKeeper(guard)

	msg := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress("from_______________").String(),
		ToAddress:   sdk.AccAddress("to________________").String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("upos", 1)),
	}

	op, err := types.NewQueuedOperation(1, 1, []sdk.Msg{msg}, keeper.GetAuthority(), ctx.BlockTime(), 0, 3600, keeper.cdc)
	require.NoError(t, err)
	require.NoError(t, keeper.SetOperation(ctx, op))

	err = keeper.ExecuteOperation(ctx, op.Id, keeper.GetAuthority())
	require.ErrorIs(t, err, types.ErrExecutionDisabled)
}

func TestTimelock_AtomicExecutionRevertsOnFailure(t *testing.T) {
	keeper, ctx, testKey := setupTimelockKeeper(t, func(testKey *storetypes.KVStoreKey) baseapp.MessageRouter {
		return testRouter{storeKey: testKey}
	})

	msg1 := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress("from_______________").String(),
		ToAddress:   sdk.AccAddress("to________________").String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("upos", 1)),
	}
	msg2 := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress("from_______________").String(),
		ToAddress:   sdk.AccAddress("to________________").String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("fail", 1)),
	}

	op, err := types.NewQueuedOperation(1, 1, []sdk.Msg{msg1, msg2}, keeper.GetAuthority(), ctx.BlockTime(), 0, 3600, keeper.cdc)
	require.NoError(t, err)
	require.NoError(t, keeper.SetOperation(ctx, op))

	err = keeper.ExecuteOperation(ctx, op.Id, keeper.GetAuthority())
	require.Error(t, err)

	// Ensure no state committed from first message
	store := ctx.KVStore(testKey)
	require.Nil(t, store.Get([]byte("counter")))
}
