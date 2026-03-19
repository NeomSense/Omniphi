// Package module implements the x/poseq Cosmos SDK module.
//
// x/poseq is the Cosmos chain-side accountability layer for PoSeq (Proof of
// Sequencing). It receives epoch-end ExportBatch records from the PoSeq relayer,
// stores evidence packets, governance escalations, checkpoint anchors, and
// committee suspension recommendations for governance and operator queries.
package module

import (
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
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"pos/x/poseq/client/cli"
	"pos/x/poseq/keeper"
	"pos/x/poseq/types"
)

var (
	_ module.AppModuleBasic = AppModule{}
	_ module.HasGenesis     = AppModule{}
	_ module.HasServices    = AppModule{}
	_ appmodule.AppModule   = AppModule{}
)

// ConsensusVersion is the x/poseq module consensus version.
const ConsensusVersion = 1

// AppModule implements the x/poseq Cosmos SDK module.
type AppModule struct {
	cdc    codec.Codec
	keeper keeper.Keeper
}

// NewAppModule constructs the x/poseq AppModule.
func NewAppModule(
	cdc codec.Codec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
) AppModule {
	k := keeper.NewKeeper(cdc, storeService, logger, authority)
	return AppModule{cdc: cdc, keeper: k}
}

// NewAppModuleFromKeeper constructs the x/poseq AppModule from an existing keeper.
func NewAppModuleFromKeeper(cdc codec.Codec, k keeper.Keeper) AppModule {
	return AppModule{cdc: cdc, keeper: k}
}

// Keeper returns the module's keeper (for wiring in app.go).
func (am AppModule) Keeper() keeper.Keeper { return am.keeper }

// ─── AppModuleBasic interface ─────────────────────────────────────────────────

func (AppModule) Name() string { return types.ModuleName }

func (AppModule) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

func (AppModule) RegisterInterfaces(_ codectypes.InterfaceRegistry) {}

func (AppModule) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

func (AppModule) GetTxCmd() *cobra.Command { return cli.GetTxCmd() }

func (AppModule) GetQueryCmd() *cobra.Command { return cli.GetQueryCmd() }

// ─── appmodule.AppModule interface ───────────────────────────────────────────

func (AppModule) IsOnePerModuleType() {}
func (AppModule) IsAppModule()        {}

func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }

// ─── module.HasGenesis interface ──────────────────────────────────────────────

// DefaultGenesis returns the default x/poseq genesis state as JSON.
func (am AppModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesis()
	bz, _ := json.Marshal(gs)
	return bz
}

// ValidateGenesis validates the x/poseq genesis state.
func (am AppModule) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal x/%s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

// InitGenesis initializes x/poseq state from a genesis state.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, bz json.RawMessage) {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		panic(fmt.Errorf("failed to unmarshal x/%s genesis: %w", types.ModuleName, err))
	}
	if err := am.keeper.InitGenesis(ctx, gs); err != nil {
		panic(fmt.Errorf("failed to init x/%s genesis: %w", types.ModuleName, err))
	}
}

// ExportGenesis exports the current x/poseq genesis state.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Errorf("failed to export x/%s genesis: %w", types.ModuleName, err))
	}
	return bz
}

// ─── module.HasServices interface ─────────────────────────────────────────────

// RegisterServices registers the x/poseq message and query servers.
// Full gRPC registration requires proto-generated stubs; this wires the
// keeper's MsgServer for direct handler use in app.go.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	// MsgServer is available via keeper.NewMsgServer(am.keeper) for app.go wiring.
	// Query server registration will be added when proto stubs are generated.
}

// RegisterInvariants is a no-op — PoSeq invariants are enforced in the Rust layer.
func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}

