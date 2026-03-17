package app

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	poseqmodule "pos/x/poseq/module"
	poseqtypes "pos/x/poseq/types"
)

// registerPoseqModule registers the x/poseq module stores and wires the keeper.
//
// x/poseq does not participate in depinject because it has no proto v1 module
// config. It is wired manually, following the same pattern as IBC modules.
//
// Must be called after appBuilder.Build() and before app.Load().
func (app *App) registerPoseqModule() error {
	// Register the poseq KV store key.
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(poseqtypes.StoreKey),
	); err != nil {
		return err
	}

	// Governance module address is the authority for x/poseq.
	govModuleAddr, _ := app.AuthKeeper.AddressCodec().BytesToString(
		authtypes.NewModuleAddress(govtypes.ModuleName),
	)

	// Build the AppModule (which constructs the keeper internally).
	poseqMod := poseqmodule.NewAppModule(
		app.appCodec,
		runtime.NewKVStoreService(app.GetKey(poseqtypes.StoreKey)),
		app.Logger(),
		govModuleAddr,
	)

	// Export the keeper so app.go can expose it and wire cross-module calls.
	app.PoseqKeeper = poseqMod.Keeper()

	// Register the module with the module manager (InitGenesis, EndBlock, etc.).
	return app.RegisterModules(poseqMod)
}
