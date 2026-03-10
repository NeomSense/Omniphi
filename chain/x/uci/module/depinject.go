package module

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	modulev1 "pos/proto/pos/uci/module/v1"
	"pos/x/uci/keeper"
	"pos/x/uci/types"
)

var _ appmodule.AppModule = AppModule{}

func (am AppModule) IsOnePerModuleType() {}
func (am AppModule) IsAppModule()        {}

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

	BankKeeper    types.BankKeeper
	AccountKeeper types.AccountKeeper
}

type ModuleOutputs struct {
	depinject.Out

	UCIKeeper *keeper.Keeper
	Module    appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		authority.String(),
		in.BankKeeper,
		in.AccountKeeper,
	)

	kp := &k
	m := NewAppModule(kp)

	return ModuleOutputs{
		UCIKeeper: kp,
		Module:    m,
	}
}
