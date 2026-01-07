// Package module provides the governance extension module for Omniphi
// This module wraps the standard Cosmos SDK governance module with proposal validation
package module

import (
	"context"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"google.golang.org/grpc"

	"pos/x/gov/keeper"
)

const ModuleName = "govx"

// GovExtension provides governance proposal validation
type GovExtension struct {
	msgServerWrapper govv1.MsgServer
	logger           log.Logger
}

// NewGovExtension creates a new governance extension
func NewGovExtension(
	govKeeper *govkeeper.Keeper,
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
) *GovExtension {
	originalMsgServer := govkeeper.NewMsgServerImpl(govKeeper)
	wrapper := keeper.NewMsgServerWrapper(originalMsgServer, cdc, msgRouter, logger)

	return &GovExtension{
		msgServerWrapper: wrapper,
		logger:           logger.With("module", ModuleName),
	}
}

// RegisterServices registers the governance extension's message server
func (ge *GovExtension) RegisterServices(cfg grpc.ServiceRegistrar) {
	govv1.RegisterMsgServer(cfg, ge.msgServerWrapper)
	ge.logger.Info("registered governance proposal validation wrapper")
}

// GovExtensionInputs defines the inputs for the governance extension
type GovExtensionInputs struct {
	depinject.In

	Config    *ModuleConfig
	Codec     codec.Codec
	Logger    log.Logger
	GovKeeper *govkeeper.Keeper
}

// GovExtensionOutputs defines the outputs from the governance extension
type GovExtensionOutputs struct {
	depinject.Out

	GovExtension *GovExtension
	Module       appmodule.AppModule
}

// ModuleConfig is the configuration for the governance extension module
type ModuleConfig struct {
	// EnableProposalValidation enables proposal validation (default: true)
	EnableProposalValidation bool

	// EnableMessageSimulation enables message simulation during validation (default: true)
	EnableMessageSimulation bool

	// MaxSimulationGas is the maximum gas for message simulation (default: 10M)
	MaxSimulationGas uint64
}

// DefaultModuleConfig returns the default module configuration
func DefaultModuleConfig() *ModuleConfig {
	return &ModuleConfig{
		EnableProposalValidation: true,
		EnableMessageSimulation:  true,
		MaxSimulationGas:         10_000_000,
	}
}

// AppModule implements the appmodule.AppModule interface for the governance extension
type AppModule struct {
	extension *GovExtension
}

// NewAppModule creates a new AppModule
func NewAppModule(extension *GovExtension) AppModule {
	return AppModule{extension: extension}
}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface
func (am AppModule) IsAppModule() {}

// Name returns the module name
func (am AppModule) Name() string {
	return ModuleName
}

// RegisterServices registers the module's services
func (am AppModule) RegisterServices(cfg grpc.ServiceRegistrar) {
	am.extension.RegisterServices(cfg)
}

// ConsensusVersion returns the module's consensus version
func (am AppModule) ConsensusVersion() uint64 {
	return 1
}

// ValidateGenesis validates genesis state (no-op for this extension)
func (am AppModule) ValidateGenesis(cdc codec.JSONCodec, config interface{}, bz []byte) error {
	return nil
}

// InitGenesis initializes genesis state (no-op for this extension)
func (am AppModule) InitGenesis(ctx context.Context, cdc codec.JSONCodec, bz []byte) error {
	return nil
}

// ExportGenesis exports genesis state (no-op for this extension)
func (am AppModule) ExportGenesis(ctx context.Context, cdc codec.JSONCodec) ([]byte, error) {
	return nil, nil
}

// DefaultGenesis returns default genesis state (no-op for this extension)
func (am AppModule) DefaultGenesis(cdc codec.JSONCodec) []byte {
	return nil
}

// GovExtensionSetup is a helper to set up the governance extension after the app is built
// This should be called after the app is built but before serving requests
type GovExtensionSetup struct {
	govKeeper *govkeeper.Keeper
	cdc       codec.Codec
	logger    log.Logger
}

// NewGovExtensionSetup creates a new setup helper
func NewGovExtensionSetup(govKeeper *govkeeper.Keeper, cdc codec.Codec, logger log.Logger) *GovExtensionSetup {
	return &GovExtensionSetup{
		govKeeper: govKeeper,
		cdc:       cdc,
		logger:    logger,
	}
}

// Setup configures the governance extension with the message router
// This must be called after the app is built and the message router is available
func (s *GovExtensionSetup) Setup(app *runtime.App) *GovExtension {
	msgRouter := app.MsgServiceRouter()
	return NewGovExtension(s.govKeeper, s.cdc, msgRouter, s.logger)
}
