package module

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"pos/x/contracts/client/cli"
	"pos/x/contracts/keeper"
	"pos/x/contracts/types"
)

var (
	_ module.AppModuleBasic  = AppModule{}
	_ module.HasGenesis      = AppModule{}
	_ appmodule.AppModule    = AppModule{}
	_ appmodule.HasEndBlocker = AppModule{}
)

// AppModule implements the Cosmos SDK AppModule interface for x/contracts.
type AppModule struct {
	keeper *keeper.Keeper
}

// NewAppModule constructs the keeper internally and returns AppModule.
func NewAppModule(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
) AppModule {
	k := keeper.NewKeeper(storeService, logger, authority)
	return AppModule{keeper: &k}
}

// Keeper returns the underlying keeper pointer for cross-module wiring.
func (am AppModule) Keeper() *keeper.Keeper { return am.keeper }

// ── AppModuleBasic ──────────────────────────────────────────────────────────

func (AppModule) Name() string                                                        { return types.ModuleName }
func (AppModule) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)                    {}
func (AppModule) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *gwruntime.ServeMux) {}
func (AppModule) RegisterInterfaces(registry codectypes.InterfaceRegistry)            {}
func (AppModule) IsOnePerModuleType()                                                 {}
func (AppModule) IsAppModule()                                                        {}

// ── Genesis ─────────────────────────────────────────────────────────────────

func (AppModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesis()
	bz, _ := json.Marshal(gs)
	return bz
}

func (AppModule) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	if len(bz) == 0 || string(bz) == "{}" || string(bz) == "null" {
		return nil
	}
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal contracts genesis: %w", err)
	}
	return gs.Validate()
}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		gs = *types.DefaultGenesis()
	} else {
		if err := json.Unmarshal(data, &gs); err != nil {
			panic(fmt.Sprintf("failed to unmarshal contracts genesis: %v", err))
		}
	}
	if err := am.keeper.InitGenesis(ctx, gs); err != nil {
		panic(fmt.Sprintf("failed to init contracts genesis: %v", err))
	}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal contracts genesis: %v", err))
	}
	return bz
}

// ── EndBlocker ──────────────────────────────────────────────────────────────

func (am AppModule) EndBlock(ctx context.Context) error {
	// Process any pending contract validation requests from the bridge
	am.keeper.ProcessValidationRequests(ctx)
	return nil
}

// ── Services ────────────────────────────────────────────────────────────────

func (am AppModule) RegisterServices(cfg module.Configurator) {
	// MsgServer and QueryServer will be registered here when proto types are added
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

// ── CLI ─────────────────────────────────────────────────────────────────────

func (AppModule) GetTxCmd() *cobra.Command    { return cli.GetTxCmd() }
func (AppModule) GetQueryCmd() *cobra.Command  { return cli.GetQueryCmd() }
