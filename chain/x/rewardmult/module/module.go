package module

import (
	"context"
	"encoding/json"
	"fmt"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"pos/x/rewardmult/keeper"
	"pos/x/rewardmult/types"
)

var (
	_ module.AppModuleBasic   = AppModule{}
	_ module.HasGenesis       = AppModule{}
	_ module.HasInvariants    = AppModule{}
	_ appmodule.AppModule     = AppModule{}
	_ appmodule.HasEndBlocker = AppModule{}
)

// AppModuleBasic defines the basic application module used by the rewardmult module.
type AppModuleBasic struct{}

// Name returns the rewardmult module's name.
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the rewardmult module's types on the LegacyAmino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the rewardmult module.
// Gateway handlers will be registered automatically when proto files are
// regenerated with the grpc-gateway plugin. No manual implementation needed.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *gwruntime.ServeMux) {
}

// RegisterInterfaces registers interfaces and implementations of the rewardmult module.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// DefaultGenesis returns default genesis state as raw bytes for the rewardmult module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return []byte("{}")
}

// ValidateGenesis performs genesis state validation for the rewardmult module.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	if string(bz) == "{}" || len(bz) == 0 {
		return nil
	}

	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return gs.Validate()
}

// GetTxCmd returns the root tx command for the rewardmult module.
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return GetTxCmd()
}

// GetQueryCmd returns the root query command for the rewardmult module.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return GetQueryCmd()
}

// ============================================================================
// AppModule
// ============================================================================

// AppModule implements an application module for the rewardmult module.
type AppModule struct {
	AppModuleBasic
	keeper *keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(keeper *keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         keeper,
	}
}

// Name returns the rewardmult module's name.
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterInvariants registers the rewardmult module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, *am.keeper)
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(*am.keeper))
}

// InitGenesis performs genesis initialization for the rewardmult module.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if string(data) == "{}" || len(data) == 0 {
		gs = *types.DefaultGenesis()
	} else {
		if err := json.Unmarshal(data, &gs); err != nil {
			panic(fmt.Sprintf("failed to unmarshal rewardmult genesis state: %v", err))
		}
	}

	if err := am.keeper.InitGenesis(ctx, gs); err != nil {
		panic(fmt.Sprintf("failed to initialize rewardmult module genesis: %v", err))
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the rewardmult module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal rewardmult genesis: %v", err))
	}
	return bz
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// EndBlock returns the end blocker for the rewardmult module.
// SAFETY: This must never panic or halt the chain.
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return am.keeper.EndBlocker(sdkCtx)
}
