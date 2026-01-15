package module

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"pos/x/timelock/client/cli"
	"pos/x/timelock/keeper"
	"pos/x/timelock/types"
)

var (
	_ module.AppModuleBasic   = (*AppModule)(nil)
	_ appmodule.AppModule     = (*AppModule)(nil)
	_ module.HasGenesis       = (*AppModule)(nil)
	_ module.HasServices      = (*AppModule)(nil)
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic implements the AppModuleBasic interface
type AppModuleBasic struct {
	cdc codec.Codec
}

// NewAppModuleBasic creates a new AppModuleBasic
func NewAppModuleBasic(cdc codec.Codec) AppModuleBasic {
	return AppModuleBasic{cdc: cdc}
}

// Name returns the module's name
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the module's types on the given LegacyAmino codec
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	// Register types for amino
}

// RegisterInterfaces registers the module's interface types
func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	// Register message implementations
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&types.MsgExecuteOperation{},
		&types.MsgCancelOperation{},
		&types.MsgEmergencyExecute{},
		&types.MsgUpdateParams{},
		&types.MsgUpdateGuardian{},
	)
}

// DefaultGenesis returns default genesis state as raw bytes
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetTxCmd returns the root tx command for the timelock module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// GetQueryCmd returns the root query command for the timelock module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface
type AppModule struct {
	AppModuleBasic

	keeper        *keeper.Keeper
	accountKeeper types.AccountKeeper
}

// NewAppModule creates a new AppModule
func NewAppModule(
	cdc codec.Codec,
	keeper *keeper.Keeper,
	accountKeeper types.AccountKeeper,
) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(cdc),
		keeper:         keeper,
		accountKeeper:  accountKeeper,
	}
}

// RegisterServices registers module services
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(*am.keeper))
}

// InitGenesis initializes the module's genesis state
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	cdc.MustUnmarshalJSON(data, &gs)

	if err := am.keeper.InitGenesis(ctx, &gs); err != nil {
		panic(fmt.Sprintf("failed to initialize timelock genesis: %v", err))
	}
}

// ExportGenesis exports the module's genesis state
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs, err := am.keeper.ExportGenesis(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to export timelock genesis: %v", err))
	}
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion returns the module's consensus version
func (AppModule) ConsensusVersion() uint64 { return 1 }

// EndBlock is called at the end of every block
func (am AppModule) EndBlock(ctx context.Context) error {
	// Process pending governance proposals FIRST (before gov module executes them)
	// This must run before the gov module's EndBlocker
	if err := am.keeper.ProcessPendingProposals(ctx); err != nil {
		am.keeper.Logger().Error("failed to process pending proposals", "error", err)
	}

	// Mark expired operations
	if err := am.keeper.MarkExpiredOperations(ctx); err != nil {
		am.keeper.Logger().Error("failed to mark expired operations", "error", err)
	}
	return nil
}
