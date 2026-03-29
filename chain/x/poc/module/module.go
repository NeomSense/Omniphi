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

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

var (
	_ module.AppModuleBasic   = AppModule{}
	_ module.HasGenesis       = AppModule{}
	_ module.HasInvariants    = AppModule{}
	_ appmodule.AppModule     = AppModule{}
	_ appmodule.HasEndBlocker = AppModule{}
)

// AppModuleBasic defines the basic application module used by the poc module.
type AppModuleBasic struct{}

// Name returns the poc module's name.
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the poc module's types on the LegacyAmino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the poc module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *gwruntime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// RegisterInterfaces registers interfaces and implementations of the poc module.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// DefaultGenesis returns default genesis state as raw bytes for the poc module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	// Return empty JSON object - module will initialize with defaults at runtime
	return []byte("{}")
}

// ValidateGenesis performs genesis state validation for the poc module.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	// Accept empty genesis state - will use defaults at runtime
	if string(bz) == "{}" || len(bz) == 0 {
		return nil
	}

	var gs types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return gs.Validate()
}

// GetTxCmd returns the root tx command for the poc module.
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return GetTxCmd()
}

// GetQueryCmd returns no root query command for the poc module.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return GetQueryCmd()
}

// ============================================================================
// AppModule
// ============================================================================

// AppModule implements an application module for the poc module.
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

// Name returns the poc module's name.
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterInvariants registers the poc module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, *am.keeper)
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(*am.keeper))
}

// InitGenesis performs genesis initialization for the poc module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	// If empty genesis, use defaults
	var gs types.GenesisState
	if string(data) == "{}" || len(data) == 0 {
		gs = *types.DefaultGenesis()
	} else {
		cdc.MustUnmarshalJSON(data, &gs)
	}

	if err := am.keeper.InitGenesis(ctx, gs); err != nil {
		// Genesis initialization errors are critical - log detailed error and panic
		panic(fmt.Sprintf("failed to initialize poc module genesis: %v", err))
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the poc module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// EndBlock returns the end blocker for the poc module. It returns no validator updates.
func (am AppModule) EndBlock(ctx context.Context) error {
	// 1. Finalize expired review sessions (Layer 3)
	// Must run before rewards so that newly-finalized contributions can be rewarded
	if err := am.keeper.FinalizeExpiredReviews(ctx); err != nil {
		am.keeper.Logger().Error("failed to finalize expired reviews", "error", err)
	}

	// 2. Process pending rewards for verified contributions (Layer 4)
	if err := am.keeper.ProcessPendingRewards(ctx); err != nil {
		am.keeper.Logger().Error("failed to process pending PoC rewards", "error", err)
	}

	// 3. Process vesting releases (Layer 4) — legacy linear schedules
	if err := am.keeper.ProcessVestingReleases(ctx); err != nil {
		am.keeper.Logger().Error("failed to process vesting releases", "error", err)
	}

	// 3b. Process ARVS multi-stage vesting releases
	if err := am.keeper.ProcessARVSVestingReleases(ctx); err != nil {
		am.keeper.Logger().Error("failed to process ARVS vesting releases", "error", err)
	}

	// 4. Process Layer 5 impact score updates (batch, bounded by EpochBatchSize)
	if err := am.keeper.ProcessImpactUpdates(ctx); err != nil {
		am.keeper.Logger().Error("failed to process impact updates", "error", err)
	}

	// 5. Clear validator cache to prevent stale data
	am.keeper.ClearValidatorCache()

	// Note: PruneRateLimits is intentionally omitted — rate-limit counters live in the
	// transient store and are auto-cleared at the end of each block by the Cosmos SDK.
	return nil
}
