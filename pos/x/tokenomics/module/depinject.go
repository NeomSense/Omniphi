package module

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	modulev1 "pos/proto/pos/tokenomics/module/v1"
	"pos/x/tokenomics/keeper"
	"pos/x/tokenomics/types"
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

	AccountKeeper types.AccountKeeper
	BankKeeper    types.BankKeeper
	StakingKeeper types.StakingKeeper
	// Note: GovKeeper and IBCKeeper not used yet - will be added when needed
}

type ModuleOutputs struct {
	depinject.Out

	TokenomicsKeeper keeper.Keeper
	Module           appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// Default to governance module authority
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		in.AccountKeeper,
		in.BankKeeper,
		in.StakingKeeper,
		nil, // GovKeeper - not used yet
		nil, // IBCKeeper - not used yet
		authority.String(),
	)

	m := NewAppModule(in.Cdc, k)

	return ModuleOutputs{
		TokenomicsKeeper: k,
		Module:           m,
	}
}
