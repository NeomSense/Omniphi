package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"

	"pos/x/contracts/types"
)

// QueryServer implements query endpoints for the contracts module.
type QueryServer struct {
	keeper Keeper
}

func NewQueryServer(keeper Keeper) QueryServer {
	return QueryServer{keeper: keeper}
}

// QuerySchemaRequest is the request for the Schema query.
type QuerySchemaRequest struct {
	SchemaID string `json:"schema_id"`
}

// QuerySchemaResponse is the response for the Schema query.
type QuerySchemaResponse struct {
	Schema types.ContractSchema `json:"schema"`
}

// Schema returns a single contract schema by ID.
func (qs QueryServer) Schema(ctx context.Context, req *QuerySchemaRequest) (*QuerySchemaResponse, error) {
	if req == nil || req.SchemaID == "" {
		return nil, fmt.Errorf("schema_id is required")
	}

	schemaID, err := hexToBytes32(req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("invalid schema_id: %w", err)
	}

	schema, found := qs.keeper.GetSchema(ctx, schemaID)
	if !found {
		return nil, fmt.Errorf("schema not found: %s", req.SchemaID)
	}

	return &QuerySchemaResponse{Schema: schema}, nil
}

// QuerySchemasResponse is the response for the Schemas query.
type QuerySchemasResponse struct {
	Schemas []types.ContractSchema `json:"schemas"`
}

// Schemas returns all deployed contract schemas.
func (qs QueryServer) Schemas(ctx context.Context) (*QuerySchemasResponse, error) {
	kvStore := qs.keeper.storeService.OpenKVStore(ctx)

	iter, err := kvStore.Iterator(types.KeyPrefixSchema, storetypes.PrefixEndBytes(types.KeyPrefixSchema))
	if err != nil {
		return nil, fmt.Errorf("failed to iterate schemas: %w", err)
	}
	defer iter.Close()

	var schemas []types.ContractSchema
	for ; iter.Valid(); iter.Next() {
		var schema types.ContractSchema
		if err := json.Unmarshal(iter.Value(), &schema); err != nil {
			continue
		}
		schemas = append(schemas, schema)
	}

	return &QuerySchemasResponse{Schemas: schemas}, nil
}

// QueryInstanceRequest is the request for the Instance query.
type QueryInstanceRequest struct {
	InstanceID uint64 `json:"instance_id"`
}

// QueryInstanceResponse is the response for the Instance query.
type QueryInstanceResponse struct {
	Instance types.ContractInstance `json:"instance"`
}

// Instance returns a single contract instance by ID.
func (qs QueryServer) Instance(ctx context.Context, req *QueryInstanceRequest) (*QueryInstanceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("instance_id is required")
	}

	inst, found := qs.keeper.GetInstance(ctx, req.InstanceID)
	if !found {
		return nil, fmt.Errorf("instance not found: %d", req.InstanceID)
	}

	return &QueryInstanceResponse{Instance: inst}, nil
}

// QueryInstancesBySchemaRequest lists instances for a schema.
type QueryInstancesBySchemaRequest struct {
	SchemaID string `json:"schema_id"`
}

// QueryInstancesBySchemaResponse is the response.
type QueryInstancesBySchemaResponse struct {
	Instances []types.ContractInstance `json:"instances"`
}

// InstancesBySchema returns all instances of a given schema.
func (qs QueryServer) InstancesBySchema(ctx context.Context, req *QueryInstancesBySchemaRequest) (*QueryInstancesBySchemaResponse, error) {
	if req == nil || req.SchemaID == "" {
		return nil, fmt.Errorf("schema_id is required")
	}

	schemaID, err := hexToBytes32(req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("invalid schema_id: %w", err)
	}

	kvStore := qs.keeper.storeService.OpenKVStore(ctx)
	prefix := types.GetInstanceBySchemaPrefixKey(schemaID)

	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil, fmt.Errorf("failed to iterate instances: %w", err)
	}
	defer iter.Close()

	var instances []types.ContractInstance
	for ; iter.Valid(); iter.Next() {
		// The key contains instance_id as the last 8 bytes
		key := iter.Key()
		if len(key) < 8 {
			continue
		}
		instIDBytes := key[len(key)-8:]
		instID := uint64(0)
		for i := 0; i < 8; i++ {
			instID = (instID << 8) | uint64(instIDBytes[i])
		}

		inst, found := qs.keeper.GetInstance(ctx, instID)
		if found {
			instances = append(instances, inst)
		}
	}

	return &QueryInstancesBySchemaResponse{Instances: instances}, nil
}

// QueryParamsResponse wraps module parameters.
type QueryParamsResponse struct {
	Params types.Params `json:"params"`
}

// Params returns the current module parameters.
func (qs QueryServer) Params(ctx context.Context) (*QueryParamsResponse, error) {
	params := qs.keeper.GetParams(ctx)
	return &QueryParamsResponse{Params: params}, nil
}

// QueryValidatorStatusRequest checks if a schema's Wasm is compiled.
type QueryValidatorStatusRequest struct {
	SchemaID string `json:"schema_id"`
}

// QueryValidatorStatusResponse returns validator compilation status.
type QueryValidatorStatusResponse struct {
	SchemaID      string `json:"schema_id"`
	HasWasm       bool   `json:"has_wasm"`
	ValidatorHash string `json:"validator_hash"`
	WasmSize      uint64 `json:"wasm_size"`
}

// ValidatorStatus checks the Wasm validator status for a schema.
func (qs QueryServer) ValidatorStatus(ctx context.Context, req *QueryValidatorStatusRequest) (*QueryValidatorStatusResponse, error) {
	if req == nil || req.SchemaID == "" {
		return nil, fmt.Errorf("schema_id is required")
	}

	schemaID, err := hexToBytes32(req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("invalid schema_id: %w", err)
	}

	schema, found := qs.keeper.GetSchema(ctx, schemaID)
	if !found {
		return nil, fmt.Errorf("schema not found: %s", req.SchemaID)
	}

	_, hasWasm := qs.keeper.GetWasm(ctx, schemaID)

	return &QueryValidatorStatusResponse{
		SchemaID:      hex.EncodeToString(schemaID[:]),
		HasWasm:       hasWasm,
		ValidatorHash: schema.ValidatorHash,
		WasmSize:      schema.WasmSize,
	}, nil
}
