package module

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	modulev1 "pos/proto/pos/poc/module/v1"
	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

var _ appmodule.AppModule = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface.
func (am AppModule) IsAppModule() {}

func init() {
	appmodule.Register(&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Cdc          codec.Codec
	StoreService store.KVStoreService
	Logger       log.Logger

	StakingKeeper types.StakingKeeper
	BankKeeper    types.BankKeeper
	AccountKeeper types.AccountKeeper
}

type ModuleOutputs struct {
	depinject.Out

	PocKeeper keeper.Keeper
	Module    appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// Default to governance module authority
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// Create transient store key for per-block submission tracking
	tStoreKey := storetypes.NewTransientStoreKey(types.TStoreKey)

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		tStoreKey,
		in.Logger,
		authority.String(),
		in.StakingKeeper,
		in.BankKeeper,
		in.AccountKeeper,
	)

	m := NewAppModule(k)

	return ModuleOutputs{
		PocKeeper: k,
		Module:    m,
	}
}
