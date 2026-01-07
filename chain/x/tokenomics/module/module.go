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

	"pos/x/tokenomics/client/cli"
	"pos/x/tokenomics/keeper"
	"pos/x/tokenomics/types"
)

var (
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasGenesis     = AppModule{}
	_ appmodule.AppModule   = AppModule{}
)

// AppModuleBasic defines the basic application module used by the tokenomics module
type AppModuleBasic struct {
	cdc codec.Codec
}

// Name returns the tokenomics module's name
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the tokenomics module's types with a legacy Amino codec
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterInterfaces registers the module's interface types
func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// DefaultGenesis returns default genesis state as raw bytes for the tokenomics module
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(keeper.DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation for the tokenomics module
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var data types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return data.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the tokenomics module
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// gRPC gateway routes will be registered via the gRPC gateway proxy
	// For now, this is a no-op as the grpc-gateway will automatically handle it
}

// GetTxCmd returns the root tx command for the tokenomics module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// GetQueryCmd returns the root query command for the tokenomics module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface for the tokenomics module
type AppModule struct {
	AppModuleBasic

	keeper keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(
	cdc codec.Codec,
	keeper keeper.Keeper,
) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         keeper,
	}
}

// RegisterServices registers module services
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))
}

// InitGenesis performs genesis initialization for the tokenomics module
// It returns no validator updates
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)

	if err := am.keeper.InitGenesis(ctx, genesisState); err != nil {
		panic(fmt.Sprintf("failed to initialize tokenomics genesis: %v", err))
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the tokenomics module
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion implements AppModule/ConsensusVersion
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock executes all ABCI BeginBlock logic for the tokenomics module
// P0-IBC-001: Reward distribution happens here every N blocks
func (am AppModule) BeginBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// ADAPTIVE-BURN: Update burn ratio based on network conditions
	// This runs every block to ensure responsive adjustments
	if err := am.keeper.UpdateBurnRatio(ctx); err != nil {
		am.keeper.Logger(ctx).Error("failed to update adaptive burn ratio", "error", err)
		// Don't halt chain - continue with existing ratio
	}

	// Check if it's time to distribute rewards
	if am.keeper.ShouldDistributeRewards(ctx) {
		// Calculate block provisions (tokens to mint this epoch)
		blockProvisions := am.keeper.CalculateBlockProvisions(ctx)

		// Calculate total rewards for this epoch
		params := am.keeper.GetParams(ctx)
		blocksPerEpoch := int64(params.RewardStreamInterval)
		totalRewards := blockProvisions.MulInt64(blocksPerEpoch).TruncateInt()

		// CRITICAL FIX: Skip minting if totalRewards is zero or negative
		// This happens when current supply is zero (at genesis) or very low
		if totalRewards.IsZero() || totalRewards.IsNegative() {
			am.keeper.Logger(ctx).Debug("skipping reward distribution - zero or negative rewards calculated",
				"total_rewards", totalRewards.String(),
				"block_height", sdkCtx.BlockHeight(),
			)
			return nil
		}

		// Mint the rewards
		// Get module address for minting (using treasury address method as template)
		moduleAddr := am.keeper.GetTreasuryAddress(ctx)
		if moduleAddr.Empty() {
			// Fallback: use a placeholder - this should be set in genesis
			am.keeper.Logger(ctx).Warn("treasury address not set, skipping reward distribution")
			return nil
		}

		// Mint to module account for distribution
		if err := am.keeper.MintTokens(ctx, totalRewards, moduleAddr, fmt.Sprintf("Epoch rewards at block %d", sdkCtx.BlockHeight())); err != nil {
			am.keeper.Logger(ctx).Error("failed to mint epoch rewards", "error", err, "height", sdkCtx.BlockHeight())
			return err
		}

		// Calculate reward splits
		recipients := am.keeper.CalculateRewardSplits(ctx, totalRewards)

		// Distribute rewards (local + IBC)
		localDist, ibcDist, packetsSent, err := am.keeper.DistributeRewardsViaIBC(ctx, recipients)
		if err != nil {
			am.keeper.Logger(ctx).Error("failed to distribute rewards", "error", err)
			return err
		}

		am.keeper.Logger(ctx).Info("epoch rewards distributed",
			"total_rewards", totalRewards.String(),
			"local_distributed", localDist.String(),
			"ibc_distributed", ibcDist.String(),
			"ibc_packets_sent", packetsSent,
			"block_height", sdkCtx.BlockHeight(),
		)
	}

	return nil
}

// EndBlock executes all ABCI EndBlock logic for the tokenomics module
// P0-IBC-006: Process IBC acknowledgements
// FEE-001: Process transaction fees (90/10 burn/treasury split)
func (am AppModule) EndBlock(ctx context.Context) error {
	// Process block fees FIRST (90/10 burn/treasury split)
	// This must happen before IBC acknowledgements to ensure all fees from this block are processed
	// Also returns the count of transactions (based on whether fees were processed)
	txCount, err := am.keeper.ProcessBlockFeesWithCount(ctx)
	if err != nil {
		// Log error but don't halt chain - fee processing is important but not critical
		am.keeper.Logger(ctx).Error("failed to process block fees", "error", err)
		// Don't return error to avoid halting the chain
	}

	// Record block transactions for 7-day rolling average (used by adaptive burn)
	// Even empty blocks should be recorded (txCount=0) to maintain accurate averages
	if err := am.keeper.RecordBlockTransactions(ctx, txCount); err != nil {
		am.keeper.Logger(ctx).Error("failed to record block transactions", "error", err)
		// Don't halt chain - this is a metrics tracking feature
	}

	// Process IBC packet acknowledgements
	// This handles failed/timed-out packets and refunds
	if err := am.keeper.ProcessIBCAcknowledgements(ctx); err != nil {
		am.keeper.Logger(ctx).Error("failed to process IBC acknowledgements", "error", err)
		return err
	}

	return nil
}

// Note: IsOnePerModuleType and IsAppModule are implemented in depinject.go
