package module

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	modulev1 "pos/proto/pos/timelock/module/v1"
	"pos/x/timelock/keeper"
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
	MsgRouter    baseapp.MessageRouter
}

type ModuleOutputs struct {
	depinject.Out

	TimelockKeeper *keeper.Keeper
	Module         appmodule.AppModule
	GovHooks       govtypes.GovHooksWrapper
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// Default to governance module authority
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		authority.String(),
		in.MsgRouter,
	)

	m := NewAppModule(in.Cdc, k, nil)

	// Create gov hooks for timelock integration
	hooks := keeper.NewGovHooks(k)

	return ModuleOutputs{
		TimelockKeeper: k,
		Module:         m,
		GovHooks:       govtypes.GovHooksWrapper{GovHooks: hooks},
	}
}
