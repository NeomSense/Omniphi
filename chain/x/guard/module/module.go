package module

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	cli "pos/x/guard/client/cli"
	modulev1 "pos/proto/pos/guard/module/v1"
	"pos/x/guard/keeper"
	"pos/x/guard/types"
)

var (
	_ module.AppModuleBasic      = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasServices         = AppModule{}
	_ module.HasInvariants       = AppModule{}
	_ appmodule.AppModule        = AppModule{}
	_ appmodule.HasBeginBlocker  = AppModule{}
	_ appmodule.HasEndBlocker    = AppModule{}
)

// ConsensusVersion defines the current x/guard module consensus version
const ConsensusVersion = 1

// AppModuleBasic defines the basic application module used by the guard module
type AppModuleBasic struct {
	cdc codec.Codec
}

// Name returns the guard module's name
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the guard module's types on the LegacyAmino codec
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterInterfaces registers the module's interface types
func (a AppModuleBasic) RegisterInterfaces(reg codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis returns default genesis state
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	defaultParams := types.DefaultParams()
	return cdc.MustMarshalJSON(&defaultParams)
}

// ValidateGenesis performs genesis state validation
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var params types.Params
	if err := cdc.UnmarshalJSON(bz, &params); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return params.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the guard module
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// gRPC gateway routes will be registered when grpc-gateway proto generation is configured
}

// GetTxCmd returns the root tx command for the guard module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// GetQueryCmd returns the root query command for the guard module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// AppModule implements the AppModule interface
type AppModule struct {
	AppModuleBasic

	keeper *keeper.Keeper
}

func NewAppModule(cdc codec.Codec, k *keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         k,
	}
}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface
func (am AppModule) IsAppModule() {}

// RegisterInvariants registers the guard module's invariants
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	keeper.RegisterInvariants(ir, *am.keeper)
}

// RegisterServices registers module services
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(*am.keeper))
}

// InitGenesis performs the module's genesis initialization
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) {
	var params types.Params
	cdc.MustUnmarshalJSON(gs, &params)

	if err := am.keeper.SetParams(ctx, params); err != nil {
		panic(err)
	}

	// Store the default linear scoring model so AI evaluation is ready when enabled
	model := types.DefaultLinearModel()
	if err := am.keeper.SetLinearModel(ctx, model); err != nil {
		panic(fmt.Errorf("failed to store default AI model: %w", err))
	}

	metadata := types.AIModelMetadata{
		ModelVersion:      model.ModelVersion,
		WeightsHash:       model.ComputeWeightsHash(),
		FeatureSchemaHash: model.FeatureSchemaHash,
		ActivatedHeight:   ctx.BlockHeight(),
	}
	if err := am.keeper.SetAIModelMetadata(ctx, metadata); err != nil {
		panic(fmt.Errorf("failed to store default AI model metadata: %w", err))
	}
}

// ExportGenesis returns the module's exported genesis state
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	params := am.keeper.GetParams(ctx)
	return cdc.MustMarshalJSON(&params)
}

// ConsensusVersion implements AppModule/ConsensusVersion
func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }

// BeginBlock executes all ABCI BeginBlock logic respective to the guard module
func (am AppModule) BeginBlock(ctx context.Context) error {
	return nil
}

// EndBlock executes all ABCI EndBlock logic respective to the guard module
// This is where we poll for new passed proposals and process the execution queue
func (am AppModule) EndBlock(ctx context.Context) error {
	// Poll for newly passed proposals
	if err := am.keeper.PollGovernanceProposals(ctx); err != nil {
		am.keeper.Logger().Error("failed to poll governance proposals", "error", err)
		// Don't halt chain on polling error
	}

	// Process execution queue
	if err := am.keeper.ProcessQueue(ctx); err != nil {
		am.keeper.Logger().Error("failed to process execution queue", "error", err)
		// Don't halt chain on queue processing error
	}

	return nil
}

// ============================================================================
// App Wiring Setup
// ============================================================================

func init() {
	appmodule.Register(
		&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Cdc          codec.Codec
	StoreService store.KVStoreService
	Logger       log.Logger

	Config        *modulev1.Module
	GovKeeper     *govkeeper.Keeper
	StakingKeeper keeper.StakingKeeper
	BankKeeper    keeper.BankKeeper
	DistrKeeper   keeper.DistrKeeper   `optional:"true"`
	Router        keeper.MessageRouter `optional:"true"`
}

type ModuleOutputs struct {
	depinject.Out

	GuardKeeper *keeper.Keeper
	Module      appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// Default to governance module authority if not specified
	authority := in.Config.Authority
	if authority == "" {
		authority = authtypes.NewModuleAddress("gov").String()
	}

	// Wrap the concrete SDK gov keeper to satisfy our GovKeeper interface
	govAdapter := &govKeeperAdapter{k: in.GovKeeper}

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		authority,
		govAdapter,
		in.StakingKeeper,
		in.BankKeeper,
		in.Logger,
	)

	// Wire optional dependencies via setters (avoids circular deps)
	kp := &k
	if in.DistrKeeper != nil {
		kp.SetDistrKeeper(in.DistrKeeper)
	}
	if in.Router != nil {
		kp.SetRouter(in.Router)
	}
	// InterfaceRegistry is wired via codec
	if protoCodec, ok := in.Cdc.(*codec.ProtoCodec); ok {
		kp.SetInterfaceRegistry(protoCodec.InterfaceRegistry())
	}

	m := NewAppModule(in.Cdc, kp)

	return ModuleOutputs{GuardKeeper: kp, Module: m}
}

// govKeeperAdapter wraps the concrete SDK gov keeper to satisfy the guard
// module's GovKeeper interface. The SDK v0.53+ uses collections-based storage
// instead of the legacy GetProposal/IterateProposals/GetParams methods.
type govKeeperAdapter struct {
	k *govkeeper.Keeper
}

func (a *govKeeperAdapter) GetProposal(ctx context.Context, proposalID uint64) (govtypes.Proposal, error) {
	return a.k.Proposals.Get(ctx, proposalID)
}

func (a *govKeeperAdapter) IterateProposals(ctx context.Context, cb func(proposal govtypes.Proposal) (stop bool)) error {
	return a.k.Proposals.Walk(ctx, nil, func(id uint64, proposal govtypes.Proposal) (bool, error) {
		return cb(proposal), nil
	})
}

func (a *govKeeperAdapter) GetParams(ctx context.Context) (govtypes.Params, error) {
	return a.k.Params.Get(ctx)
}
