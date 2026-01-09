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

// setupStoreUpgrades configures store loaders for upgrades that add new module stores.
// This MUST be called after Build() but BEFORE Load() to properly handle new stores.
func (app *App) setupStoreUpgrades() {
	// Read upgrade info from disk to check if we're at an upgrade height
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		// No upgrade info file means this is a fresh chain or upgrade already applied
		return
	}

	// Handle timelock module upgrade
	if upgradeInfo.Name == UpgradeNameTimelock && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{timelockmoduletypes.StoreKey},
		}

		// Configure store loader for added stores
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}

// RegisterUpgradeHandlers registers upgrade handlers for the app.
// This is called after Load() to register the actual upgrade logic.
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
}
