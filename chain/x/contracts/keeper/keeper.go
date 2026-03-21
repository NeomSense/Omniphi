package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/contracts/types"
)

// Keeper manages the x/contracts module state.
type Keeper struct {
	storeService store.KVStoreService
	logger       log.Logger
	authority    string
}

func NewKeeper(
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
) Keeper {
	return Keeper{
		storeService: storeService,
		logger:       logger,
		authority:    authority,
	}
}

func (k Keeper) GetAuthority() string { return k.authority }
func (k Keeper) Logger() log.Logger   { return k.logger }

// ── Params ──────────────────────────────────────────────────────────────────

func (k Keeper) GetParams(ctx context.Context) types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyParams)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	if err := json.Unmarshal(bz, &p); err != nil {
		return types.DefaultParams()
	}
	return p
}

func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return kvStore.Set(types.KeyParams, bz)
}

// ── Schema CRUD ─────────────────────────────────────────────────────────────

func (k Keeper) GetSchema(ctx context.Context, schemaID [32]byte) (types.ContractSchema, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetSchemaKey(schemaID))
	if err != nil || bz == nil {
		return types.ContractSchema{}, false
	}
	var schema types.ContractSchema
	if err := json.Unmarshal(bz, &schema); err != nil {
		k.logger.Error("failed to unmarshal contract schema", "error", err)
		return types.ContractSchema{}, false
	}
	return schema, true
}

func (k Keeper) SetSchema(ctx context.Context, schema types.ContractSchema) error {
	schemaID, err := hexToBytes32(schema.SchemaID)
	if err != nil {
		return fmt.Errorf("invalid schema_id hex: %w", err)
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	if err := kvStore.Set(types.GetSchemaKey(schemaID), bz); err != nil {
		return err
	}
	// Index by deployer
	return kvStore.Set(types.GetSchemaByDeployerKey(schema.Deployer, schemaID), []byte{1})
}

// ── Wasm Bytecode ───────────────────────────────────────────────────────────

func (k Keeper) StoreWasm(ctx context.Context, schemaID [32]byte, bytecode []byte) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetWasmKey(schemaID), bytecode)
}

func (k Keeper) GetWasm(ctx context.Context, schemaID [32]byte) ([]byte, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetWasmKey(schemaID))
	if err != nil || bz == nil {
		return nil, false
	}
	return bz, true
}

// ── Instances ───────────────────────────────────────────────────────────────

func (k Keeper) GetNextInstanceID(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetNextInstanceIDKey())
	if err != nil || bz == nil {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) setNextInstanceID(ctx context.Context, id uint64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetNextInstanceIDKey(), sdk.Uint64ToBigEndian(id))
}

func (k Keeper) GetInstance(ctx context.Context, instanceID uint64) (types.ContractInstance, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetInstanceKey(instanceID))
	if err != nil || bz == nil {
		return types.ContractInstance{}, false
	}
	var inst types.ContractInstance
	if err := json.Unmarshal(bz, &inst); err != nil {
		return types.ContractInstance{}, false
	}
	return inst, true
}

func (k Keeper) SetInstance(ctx context.Context, inst types.ContractInstance) error {
	schemaID, err := hexToBytes32(inst.SchemaID)
	if err != nil {
		return fmt.Errorf("invalid schema_id hex: %w", err)
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(inst)
	if err != nil {
		return fmt.Errorf("failed to marshal instance: %w", err)
	}
	if err := kvStore.Set(types.GetInstanceKey(inst.InstanceID), bz); err != nil {
		return err
	}
	// Index by schema
	return kvStore.Set(types.GetInstanceBySchemaKey(schemaID, inst.InstanceID), []byte{1})
}

// ── Deploy Contract ─────────────────────────────────────────────────────────

// DeployContract validates, stores, and indexes a new contract schema.
func (k Keeper) DeployContract(ctx context.Context, msg types.MsgDeployContract) (types.ContractSchema, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Validate Wasm size
	if uint64(len(msg.WasmBytecode)) > params.MaxWasmSize {
		return types.ContractSchema{}, fmt.Errorf(
			"wasm bytecode too large: %d > max %d", len(msg.WasmBytecode), params.MaxWasmSize,
		)
	}

	// Validate max state bytes
	if msg.MaxStateBytes > params.MaxStateBytesHardCap {
		return types.ContractSchema{}, fmt.Errorf(
			"max_state_bytes %d exceeds hard cap %d", msg.MaxStateBytes, params.MaxStateBytesHardCap,
		)
	}

	// Compute schema ID
	schemaID := types.ComputeSchemaID(msg.Deployer, msg.Name, 1)

	// Check for duplicate
	if _, exists := k.GetSchema(ctx, schemaID); exists {
		return types.ContractSchema{}, fmt.Errorf("schema already exists: %s", hex.EncodeToString(schemaID[:]))
	}

	// Compute Wasm hash
	wasmHash := sha256.Sum256(msg.WasmBytecode)

	schema := types.ContractSchema{
		SchemaID:      hex.EncodeToString(schemaID[:]),
		Deployer:      msg.Deployer,
		Version:       1,
		Name:          msg.Name,
		Description:   msg.Description,
		DomainTag:     msg.DomainTag,
		IntentSchemas: msg.IntentSchemas,
		MaxGasPerCall: msg.MaxGasPerCall,
		MaxStateBytes: msg.MaxStateBytes,
		ValidatorHash: hex.EncodeToString(wasmHash[:]),
		WasmSize:      uint64(len(msg.WasmBytecode)),
		Status:        types.ContractStatusActive,
		DeployedAt:    sdkCtx.BlockHeight(),
	}

	// Store schema
	if err := k.SetSchema(ctx, schema); err != nil {
		return types.ContractSchema{}, fmt.Errorf("failed to store schema: %w", err)
	}

	// Store Wasm bytecode
	if err := k.StoreWasm(ctx, schemaID, msg.WasmBytecode); err != nil {
		return types.ContractSchema{}, fmt.Errorf("failed to store wasm: %w", err)
	}

	k.logger.Info("contract deployed",
		"schema_id", schema.SchemaID,
		"name", schema.Name,
		"deployer", schema.Deployer,
		"wasm_size", schema.WasmSize,
		"methods", len(schema.IntentSchemas),
	)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"contract_deployed",
		sdk.NewAttribute("schema_id", schema.SchemaID),
		sdk.NewAttribute("deployer", schema.Deployer),
		sdk.NewAttribute("name", schema.Name),
	))

	return schema, nil
}

// InstantiateContract creates a new instance of an existing schema.
func (k Keeper) InstantiateContract(ctx context.Context, msg types.MsgInstantiateContract) (types.ContractInstance, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	schemaID, err := hexToBytes32(msg.SchemaID)
	if err != nil {
		return types.ContractInstance{}, fmt.Errorf("invalid schema_id: %w", err)
	}

	schema, exists := k.GetSchema(ctx, schemaID)
	if !exists {
		return types.ContractInstance{}, fmt.Errorf("schema not found: %s", msg.SchemaID)
	}
	if schema.Status != types.ContractStatusActive {
		return types.ContractInstance{}, fmt.Errorf("schema is not active: %s", schema.Status)
	}

	instanceID := k.GetNextInstanceID(ctx)
	admin := msg.Admin
	if admin == "" {
		admin = msg.Creator
	}

	inst := types.ContractInstance{
		InstanceID: instanceID,
		SchemaID:   msg.SchemaID,
		Creator:    msg.Creator,
		Admin:      admin,
		Label:      msg.Label,
		CreatedAt:  sdkCtx.BlockHeight(),
	}

	if err := k.SetInstance(ctx, inst); err != nil {
		return types.ContractInstance{}, fmt.Errorf("failed to store instance: %w", err)
	}
	if err := k.setNextInstanceID(ctx, instanceID+1); err != nil {
		return types.ContractInstance{}, err
	}

	k.logger.Info("contract instantiated",
		"instance_id", instanceID,
		"schema_id", msg.SchemaID,
		"creator", msg.Creator,
	)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"contract_instantiated",
		sdk.NewAttribute("instance_id", fmt.Sprintf("%d", instanceID)),
		sdk.NewAttribute("schema_id", msg.SchemaID),
		sdk.NewAttribute("creator", msg.Creator),
	))

	return inst, nil
}

// ── Genesis ─────────────────────────────────────────────────────────────────

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}
	for _, schema := range gs.Schemas {
		if err := k.SetSchema(ctx, schema); err != nil {
			return err
		}
	}
	for _, inst := range gs.Instances {
		if err := k.SetInstance(ctx, inst); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx sdk.Context) types.GenesisState {
	return types.GenesisState{
		Params:    k.GetParams(ctx),
		Schemas:   []types.ContractSchema{},
		Instances: []types.ContractInstance{},
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func hexToBytes32(s string) ([32]byte, error) {
	var result [32]byte
	bz, err := hex.DecodeString(s)
	if err != nil {
		return result, err
	}
	if len(bz) != 32 {
		return result, fmt.Errorf("expected 32 bytes, got %d", len(bz))
	}
	copy(result[:], bz)
	return result, nil
}
