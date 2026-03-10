package module

import (
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	modulev1 "pos/proto/pos/rewardmult/module/v1"
	"pos/x/rewardmult/keeper"
	"pos/x/rewardmult/types"
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

	StakingKeeper  types.StakingKeeper
	SlashingKeeper types.SlashingKeeper
}

type ModuleOutputs struct {
	depinject.Out

	RewardmultKeeper *keeper.Keeper
	Module           appmodule.AppModule
	StakingHooks     stakingtypes.StakingHooksWrapper
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.Logger,
		authority.String(),
		in.StakingKeeper,
		in.SlashingKeeper,
	)

	kp := &k
	m := NewAppModule(kp)

	hooks := keeper.NewStakingHooks(kp)

	return ModuleOutputs{
		RewardmultKeeper: kp,
		Module:           m,
		StakingHooks:     stakingtypes.StakingHooksWrapper{StakingHooks: hooks},
	}
}
