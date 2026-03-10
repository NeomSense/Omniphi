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

	"pos/x/royalty/keeper"
	"pos/x/royalty/types"
)

var (
	_ module.AppModuleBasic = AppModule{}
	_ module.HasGenesis     = AppModule{}
	_ appmodule.AppModule   = AppModule{}
)

type AppModuleBasic struct{}

func (AppModuleBasic) Name() string                                                            { return types.ModuleName }
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)                        { types.RegisterLegacyAminoCodec(cdc) }
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *gwruntime.ServeMux) {}
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry)                { types.RegisterInterfaces(registry) }
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage                     { return []byte("{}") }

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

func (AppModuleBasic) GetTxCmd() *cobra.Command    { return GetTxCmd() }
func (AppModuleBasic) GetQueryCmd() *cobra.Command  { return GetQueryCmd() }

type AppModule struct {
	AppModuleBasic
	keeper *keeper.Keeper
}

func NewAppModule(keeper *keeper.Keeper) AppModule {
	return AppModule{AppModuleBasic: AppModuleBasic{}, keeper: keeper}
}

func (AppModule) Name() string { return types.ModuleName }

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(*am.keeper))
}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if string(data) == "{}" || len(data) == 0 {
		gs = *types.DefaultGenesis()
	} else {
		if err := json.Unmarshal(data, &gs); err != nil {
			panic(fmt.Sprintf("failed to unmarshal royalty genesis state: %v", err))
		}
	}
	if err := am.keeper.InitGenesis(ctx, gs); err != nil {
		panic(fmt.Sprintf("failed to initialize royalty module genesis: %v", err))
	}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal royalty genesis: %v", err))
	}
	return bz
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

// EndBlock — royalty module does not have an EndBlocker.
// Royalty distribution is triggered by PoC via OnContributionRewarded hook.
func (am AppModule) EndBlock(ctx context.Context) error { return nil }
