package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"

	"pos/x/feemarket/types"
)

// Keeper of the feemarket store
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	// Keepers
	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper

	// Authority (typically the governance module account)
	authority string
}

// NewKeeper creates a new feemarket Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		authority:     authority,
	}
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx context.Context) log.Logger {
	return k.logger
}

// GetAuthority returns the module's authority
func (k Keeper) GetAuthority() string {
	return k.authority
}
