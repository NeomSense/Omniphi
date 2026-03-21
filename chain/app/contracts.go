package app

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	contractsmodule "pos/x/contracts/module"
	contractstypes "pos/x/contracts/types"
)

// registerContractsModule registers the x/contracts module (Intent Contracts).
//
// x/contracts does not participate in depinject because it has no proto v1
// module config. It is wired manually, following the same pattern as x/poseq.
//
// Must be called after appBuilder.Build() and before app.Load().
func (app *App) registerContractsModule() error {
	// Register the contracts KV store key.
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(contractstypes.StoreKey),
	); err != nil {
		return err
	}

	// Governance module address is the authority for x/contracts.
	govModuleAddr, _ := app.AuthKeeper.AddressCodec().BytesToString(
		authtypes.NewModuleAddress(govtypes.ModuleName),
	)

	// Build the AppModule (which constructs the keeper internally).
	contractsMod := contractsmodule.NewAppModule(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(contractstypes.StoreKey)),
		app.Logger(),
		govModuleAddr,
	)

	// Export the keeper so app.go can expose it and wire cross-module calls.
	app.ContractsKeeper = contractsMod.Keeper()

	// Register the module with the module manager.
	return app.RegisterModules(contractsMod)
}
