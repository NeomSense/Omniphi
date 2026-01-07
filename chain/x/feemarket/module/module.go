package module

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"pos/x/feemarket/client/cli"
	"pos/x/feemarket/keeper"
	"pos/x/feemarket/types"
)

var (
	_ module.AppModuleBasic      = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ appmodule.AppModule        = AppModule{}
	_ appmodule.HasBeginBlocker  = AppModule{}
	_ appmodule.HasEndBlocker    = AppModule{}
)

// AppModuleBasic defines the basic application module used by the feemarket module
type AppModuleBasic struct {
	cdc codec.Codec
}

// Name returns the feemarket module's name
func (AppModuleBasic) Name() string { return types.ModuleName }

// RegisterLegacyAminoCodec registers the feemarket module's types on the LegacyAmino codec
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterInterfaces registers the module's interface types
func (AppModuleBasic) RegisterInterfaces(reg codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis returns default genesis state as raw bytes for the feemarket module
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation for the feemarket module
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the feemarket module
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetTxCmd returns the root tx command for the feemarket module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// GetQueryCmd returns the root query command for the feemarket module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// AppModule implements an application module for the feemarket module
type AppModule struct {
	AppModuleBasic

	keeper keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(cdc codec.Codec, keeper keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         keeper,
	}
}

// RegisterServices registers module services
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServer(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServer(am.keeper))
}

// InitGenesis performs genesis initialization for the feemarket module
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genState)

	if err := am.keeper.InitGenesis(ctx, genState); err != nil {
		panic(fmt.Sprintf("failed to initialize feemarket genesis state: %v", err))
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the feemarket module
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genState := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(genState)
}

// ConsensusVersion implements AppModule/ConsensusVersion
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock executes all ABCI BeginBlock logic respective to the feemarket module
// Updates the base fee based on previous block utilization (EIP-1559)
func (am AppModule) BeginBlock(ctx context.Context) error {
	am.keeper.Logger(ctx).Debug("feemarket BeginBlock started")

	// Update base fee based on previous block utilization
	if err := am.keeper.UpdateBaseFee(ctx); err != nil {
		am.keeper.Logger(ctx).Error("failed to update base fee", "error", err)
		return err
	}

	return nil
}

// EndBlock executes all ABCI EndBlock logic respective to the feemarket module
// Processes collected fees: burn, treasury distribution, and validator rewards
func (am AppModule) EndBlock(ctx context.Context) error {
	am.keeper.Logger(ctx).Debug("feemarket EndBlock started")

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get current block utilization before processing fees
	currentUtilization := am.keeper.GetBlockUtilization(ctx)

	// Store block gas metrics for queries
	if sdkCtx.BlockGasMeter() != nil {
		blockGasUsed := int64(sdkCtx.BlockGasMeter().GasConsumed())
		if err := am.keeper.SetPreviousBlockGasUsed(ctx, blockGasUsed); err != nil {
			am.keeper.Logger(ctx).Error("failed to set previous block gas used", "error", err)
		}
	}

	// Store max block gas from consensus params
	if consParams := sdkCtx.ConsensusParams(); consParams.Block != nil {
		maxBlockGas := consParams.Block.MaxGas
		if err := am.keeper.SetMaxBlockGas(ctx, maxBlockGas); err != nil {
			am.keeper.Logger(ctx).Error("failed to set max block gas", "error", err)
		}
	}

	// Process all fees collected in this block
	if err := am.keeper.ProcessBlockFees(ctx); err != nil {
		am.keeper.Logger(ctx).Error("failed to process block fees", "error", err)
		return err
	}

	// Store current utilization for next block's base fee calculation
	if err := am.keeper.SetPreviousBlockUtilization(ctx, currentUtilization); err != nil {
		am.keeper.Logger(ctx).Error("failed to set previous block utilization", "error", err)
		return err
	}

	am.keeper.Logger(ctx).Debug("feemarket EndBlock completed",
		"utilization", currentUtilization.String(),
	)

	return nil
}
