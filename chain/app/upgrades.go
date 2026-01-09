package app

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	timelockmoduletypes "pos/x/timelock/types"
)

// Upgrade names
const (
	// UpgradeNameTimelock is the upgrade name for adding the timelock module
	UpgradeNameTimelock = "v1.1.0-timelock"
)

// RegisterUpgradeHandlers registers upgrade handlers for the app
func (app *App) RegisterUpgradeHandlers() {
	// Register the timelock upgrade handler
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeNameTimelock,
		func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			app.Logger().Info("Running upgrade handler", "upgrade", UpgradeNameTimelock)

			// Run migrations for all modules
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	// Configure store upgrades
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(err)
	}

	if upgradeInfo.Name == UpgradeNameTimelock && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{timelockmoduletypes.StoreKey},
		}

		// Configure store loader for added stores
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
